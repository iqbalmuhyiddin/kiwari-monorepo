package com.kiwari.pos.ui.menu

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.Modifier
import com.kiwari.pos.data.model.ModifierGroup
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.SelectedModifier
import com.kiwari.pos.data.model.SelectedVariant
import com.kiwari.pos.data.model.Variant
import com.kiwari.pos.data.model.VariantGroup
import com.kiwari.pos.data.repository.CartRepository
import com.kiwari.pos.data.repository.MenuRepository
import com.kiwari.pos.data.repository.SelectedProductRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.math.BigDecimal
import javax.inject.Inject

/**
 * Variant group with its loaded variants.
 */
data class VariantGroupWithVariants(
    val group: VariantGroup,
    val variants: List<Variant>
)

/**
 * Modifier group with its loaded modifiers.
 */
data class ModifierGroupWithModifiers(
    val group: ModifierGroup,
    val modifiers: List<Modifier>
)

data class CustomizationUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val product: Product? = null,
    val variantGroups: List<VariantGroupWithVariants> = emptyList(),
    val modifierGroups: List<ModifierGroupWithModifiers> = emptyList(),
    // Selection state: variantGroupId -> selected variantId
    val selectedVariants: Map<String, String> = emptyMap(),
    // Selection state: modifierGroupId -> set of selected modifierIds
    val selectedModifiers: Map<String, Set<String>> = emptyMap(),
    val quantity: Int = 1,
    val notes: String = "",
    // Calculated total for the "ADD TO CART" button
    val calculatedTotal: BigDecimal = BigDecimal.ZERO,
    // Whether the "ADD TO CART" button should be enabled
    val canAddToCart: Boolean = false,
    // True after successfully adding to cart, triggers navigation back
    val addedToCart: Boolean = false
)

@HiltViewModel
class CustomizationViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val selectedProductRepository: SelectedProductRepository,
    private val menuRepository: MenuRepository,
    private val cartRepository: CartRepository
) : ViewModel() {

    private val productId: String = savedStateHandle.get<String>("productId") ?: ""

    private val _uiState = MutableStateFlow(CustomizationUiState())
    val uiState: StateFlow<CustomizationUiState> = _uiState.asStateFlow()

    init {
        loadCustomizationData()
    }

    private fun loadCustomizationData() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessage = null) }

            // On retry, product is already in state; on first load, read from repository
            val product = _uiState.value.product ?: run {
                val p = selectedProductRepository.get()
                selectedProductRepository.clear()
                p
            }

            if (product == null || product.id != productId) {
                _uiState.update {
                    it.copy(isLoading = false, errorMessage = "Produk tidak ditemukan")
                }
                return@launch
            }

            _uiState.update { it.copy(product = product) }

            // Load variant groups and modifier groups in parallel
            val variantGroupsDeferred = async { menuRepository.getVariantGroups(productId) }
            val modifierGroupsDeferred = async { menuRepository.getModifierGroups(productId) }

            val variantGroupsResult = variantGroupsDeferred.await()
            val modifierGroupsResult = modifierGroupsDeferred.await()

            if (variantGroupsResult is Result.Error) {
                _uiState.update {
                    it.copy(isLoading = false, errorMessage = variantGroupsResult.message)
                }
                return@launch
            }
            if (modifierGroupsResult is Result.Error) {
                _uiState.update {
                    it.copy(isLoading = false, errorMessage = modifierGroupsResult.message)
                }
                return@launch
            }

            val variantGroups = (variantGroupsResult as Result.Success).data
                .filter { it.isActive }
                .sortedBy { it.sortOrder }
            val modifierGroups = (modifierGroupsResult as Result.Success).data
                .filter { it.isActive }
                .sortedBy { it.sortOrder }

            // Load all variants and modifiers in parallel
            val variantGroupsWithVariants = coroutineScope {
                variantGroups.map { vg ->
                    async {
                        val variantsResult = menuRepository.getVariants(productId, vg.id)
                        val variants = when (variantsResult) {
                            is Result.Success -> variantsResult.data
                                .filter { it.isActive }
                                .sortedBy { it.sortOrder }
                            is Result.Error -> emptyList()
                        }
                        VariantGroupWithVariants(group = vg, variants = variants)
                    }
                }.awaitAll()
            }

            val modifierGroupsWithModifiers = coroutineScope {
                modifierGroups.map { mg ->
                    async {
                        val modifiersResult = menuRepository.getModifiers(productId, mg.id)
                        val modifiers = when (modifiersResult) {
                            is Result.Success -> modifiersResult.data
                                .filter { it.isActive }
                                .sortedBy { it.sortOrder }
                            is Result.Error -> emptyList()
                        }
                        ModifierGroupWithModifiers(group = mg, modifiers = modifiers)
                    }
                }.awaitAll()
            }

            // Filter out groups whose sub-fetch failed (returned empty list)
            val loadedVariantGroups = variantGroupsWithVariants.filter { it.variants.isNotEmpty() }
            val loadedModifierGroups = modifierGroupsWithModifiers.filter { it.modifiers.isNotEmpty() }

            if (loadedVariantGroups.size < variantGroupsWithVariants.size ||
                loadedModifierGroups.size < modifierGroupsWithModifiers.size
            ) {
                android.util.Log.w(
                    "CustomizationVM",
                    "Some variant/modifier groups dropped due to fetch failures"
                )
            }

            _uiState.update {
                val updated = it.copy(
                    isLoading = false,
                    variantGroups = loadedVariantGroups,
                    modifierGroups = loadedModifierGroups
                )
                recalculate(updated)
            }
        }
    }

    fun onVariantSelected(variantGroupId: String, variantId: String) {
        _uiState.update {
            val updated = it.copy(
                selectedVariants = it.selectedVariants + (variantGroupId to variantId)
            )
            recalculate(updated)
        }
    }

    fun onModifierToggled(modifierGroupId: String, modifierId: String) {
        _uiState.update { state ->
            val currentSet = state.selectedModifiers[modifierGroupId] ?: emptySet()
            val newSet = if (modifierId in currentSet) {
                currentSet - modifierId
            } else {
                // Enforce max_select: don't add if at max
                val group = state.modifierGroups.find { it.group.id == modifierGroupId }
                val maxSelect = group?.group?.maxSelect
                if (maxSelect != null && currentSet.size >= maxSelect) {
                    return@update state // Already at max, don't toggle on
                }
                currentSet + modifierId
            }
            val updated = state.copy(
                selectedModifiers = state.selectedModifiers + (modifierGroupId to newSet)
            )
            recalculate(updated)
        }
    }

    fun onQuantityChanged(newQuantity: Int) {
        if (newQuantity < 1 || newQuantity > 999) return
        _uiState.update {
            val updated = it.copy(quantity = newQuantity)
            recalculate(updated)
        }
    }

    fun onNotesChanged(newNotes: String) {
        _uiState.update { it.copy(notes = newNotes) }
    }

    fun addToCart() {
        val state = _uiState.value
        val product = state.product ?: return
        if (!state.canAddToCart) return

        val selectedVariants = buildSelectedVariants(state)
        val selectedModifiers = buildSelectedModifiers(state)

        cartRepository.addCustomizedProduct(
            product = product,
            selectedVariants = selectedVariants,
            selectedModifiers = selectedModifiers,
            quantity = state.quantity,
            notes = state.notes.trim()
        )

        _uiState.update { it.copy(addedToCart = true) }
    }

    fun onAddedToCartHandled() {
        _uiState.update { it.copy(addedToCart = false) }
    }

    fun retry() {
        loadCustomizationData()
    }

    // ── Private helpers ──

    private fun buildSelectedVariants(state: CustomizationUiState): List<SelectedVariant> {
        val result = mutableListOf<SelectedVariant>()
        for (vgWithVariants in state.variantGroups) {
            val selectedId = state.selectedVariants[vgWithVariants.group.id] ?: continue
            val variant = vgWithVariants.variants.find { it.id == selectedId } ?: continue
            result.add(
                SelectedVariant(
                    variantGroupId = vgWithVariants.group.id,
                    variantGroupName = vgWithVariants.group.name,
                    variantId = variant.id,
                    variantName = variant.name,
                    priceAdjustment = BigDecimal(variant.priceAdjustment)
                )
            )
        }
        return result
    }

    private fun buildSelectedModifiers(state: CustomizationUiState): List<SelectedModifier> {
        val result = mutableListOf<SelectedModifier>()
        for (mgWithModifiers in state.modifierGroups) {
            val selectedIds = state.selectedModifiers[mgWithModifiers.group.id] ?: continue
            for (modifier in mgWithModifiers.modifiers) {
                if (modifier.id in selectedIds) {
                    result.add(
                        SelectedModifier(
                            modifierGroupId = mgWithModifiers.group.id,
                            modifierGroupName = mgWithModifiers.group.name,
                            modifierId = modifier.id,
                            modifierName = modifier.name,
                            price = BigDecimal(modifier.price)
                        )
                    )
                }
            }
        }
        return result
    }

    /**
     * Recalculate price and validation state.
     * Returns a new state with updated calculatedTotal and canAddToCart.
     */
    private fun recalculate(state: CustomizationUiState): CustomizationUiState {
        val product = state.product ?: return state.copy(
            calculatedTotal = BigDecimal.ZERO,
            canAddToCart = false
        )

        // Base price
        val basePrice = BigDecimal(product.basePrice)

        // Variant adjustment (sum of all selected variants' adjustments)
        var variantAdjustment = BigDecimal.ZERO
        for (vgWithVariants in state.variantGroups) {
            val selectedId = state.selectedVariants[vgWithVariants.group.id] ?: continue
            val variant = vgWithVariants.variants.find { it.id == selectedId } ?: continue
            variantAdjustment = variantAdjustment.add(BigDecimal(variant.priceAdjustment))
        }

        // Modifier total
        var modifierTotal = BigDecimal.ZERO
        for (mgWithModifiers in state.modifierGroups) {
            val selectedIds = state.selectedModifiers[mgWithModifiers.group.id] ?: continue
            for (modifier in mgWithModifiers.modifiers) {
                if (modifier.id in selectedIds) {
                    modifierTotal = modifierTotal.add(BigDecimal(modifier.price))
                }
            }
        }

        val unitPrice = basePrice.add(variantAdjustment).add(modifierTotal)
        val total = unitPrice.multiply(BigDecimal(state.quantity))

        // Validation: all required variant groups must have a selection
        val allRequiredVariantsSelected = state.variantGroups
            .filter { it.group.isRequired }
            .all { vg -> state.selectedVariants.containsKey(vg.group.id) }

        // Validation: all modifier groups with minSelect > 0 must meet minimum
        val allModifierMinsmet = state.modifierGroups.all { mg ->
            val selectedCount = (state.selectedModifiers[mg.group.id] ?: emptySet()).size
            selectedCount >= mg.group.minSelect
        }

        val canAdd = allRequiredVariantsSelected && allModifierMinsmet

        return state.copy(
            calculatedTotal = total,
            canAddToCart = canAdd
        )
    }
}
