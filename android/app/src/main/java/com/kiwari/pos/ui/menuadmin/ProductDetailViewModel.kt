package com.kiwari.pos.ui.menuadmin

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.Category
import com.kiwari.pos.data.model.ComboItem
import com.kiwari.pos.data.model.CreateComboItemRequest
import com.kiwari.pos.data.model.CreateModifierGroupRequest
import com.kiwari.pos.data.model.CreateModifierRequest
import com.kiwari.pos.data.model.CreateProductRequest
import com.kiwari.pos.data.model.CreateVariantGroupRequest
import com.kiwari.pos.data.model.CreateVariantRequest
import com.kiwari.pos.data.model.Modifier
import com.kiwari.pos.data.model.ModifierGroup
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.UpdateModifierGroupRequest
import com.kiwari.pos.data.model.UpdateModifierRequest
import com.kiwari.pos.data.model.UpdateProductRequest
import com.kiwari.pos.data.model.UpdateVariantGroupRequest
import com.kiwari.pos.data.model.UpdateVariantRequest
import com.kiwari.pos.data.model.Variant
import com.kiwari.pos.data.model.VariantGroup
import com.kiwari.pos.data.repository.MenuAdminRepository
import com.kiwari.pos.data.repository.MenuRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class VariantGroupWithVariants(
    val group: VariantGroup,
    val variants: List<Variant>
)

data class ModifierGroupWithModifiers(
    val group: ModifierGroup,
    val modifiers: List<Modifier>
)

data class ComboItemWithProduct(
    val comboItem: ComboItem,
    val productName: String
)

sealed class ProductSheet {
    data class EditVariantGroup(val group: VariantGroup?) : ProductSheet()
    data class EditVariant(val groupId: String, val variant: Variant?) : ProductSheet()
    data class EditModifierGroup(val group: ModifierGroup?) : ProductSheet()
    data class EditModifier(val groupId: String, val modifier: Modifier?) : ProductSheet()
    object AddComboItem : ProductSheet()
}

data class ProductDetailUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val isCreateMode: Boolean = true,
    val productId: String? = null,
    // Form fields
    val name: String = "",
    val basePrice: String = "",
    val categoryId: String = "",
    val station: String = "",
    val description: String = "",
    val preparationTime: String = "",
    val isCombo: Boolean = false,
    val isActive: Boolean = true,
    // Child entities (loaded for edit mode)
    val variantGroups: List<VariantGroupWithVariants> = emptyList(),
    val modifierGroups: List<ModifierGroupWithModifiers> = emptyList(),
    val comboItems: List<ComboItemWithProduct> = emptyList(),
    // Bottom sheet state
    val activeSheet: ProductSheet? = null,
    val isSaving: Boolean = false,
    val saveError: String? = null,
    val savedProductId: String? = null,
    // Category list for dropdown
    val categories: List<Category> = emptyList(),
    // All products for combo picker
    val allProducts: List<Product> = emptyList(),
    // Deactivate
    val showDeactivateDialog: Boolean = false,
    val isDeactivating: Boolean = false
)

@HiltViewModel
class ProductDetailViewModel @Inject constructor(
    private val menuRepository: MenuRepository,
    private val menuAdminRepository: MenuAdminRepository,
    savedStateHandle: SavedStateHandle
) : ViewModel() {

    private val navProductId: String = checkNotNull(savedStateHandle["productId"])
    private val navCategoryId: String? = savedStateHandle["categoryId"]

    private val _uiState = MutableStateFlow(ProductDetailUiState())
    val uiState: StateFlow<ProductDetailUiState> = _uiState.asStateFlow()

    init {
        if (navProductId == "new") {
            _uiState.update {
                it.copy(
                    isCreateMode = true,
                    isLoading = false,
                    categoryId = navCategoryId ?: ""
                )
            }
            loadCategories()
        } else {
            _uiState.update {
                it.copy(
                    isCreateMode = false,
                    productId = navProductId
                )
            }
            loadProduct()
        }
    }

    // ── Loading ──

    private fun loadProduct() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessage = null) }

            val productId = _uiState.value.productId ?: return@launch

            val categoriesDeferred = async { menuRepository.getCategories() }
            val productsDeferred = async { menuRepository.getProducts() }
            val variantGroupsDeferred = async { menuRepository.getVariantGroups(productId) }
            val modifierGroupsDeferred = async { menuRepository.getModifierGroups(productId) }

            val categoriesResult = categoriesDeferred.await()
            val productsResult = productsDeferred.await()
            val variantGroupsResult = variantGroupsDeferred.await()
            val modifierGroupsResult = modifierGroupsDeferred.await()

            // Find the product from the products list
            val allProducts = when (productsResult) {
                is Result.Success -> productsResult.data
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, errorMessage = productsResult.message)
                    }
                    return@launch
                }
            }

            val product = allProducts.find { it.id == productId }
            if (product == null) {
                _uiState.update {
                    it.copy(isLoading = false, errorMessage = "Produk tidak ditemukan")
                }
                return@launch
            }

            val categories = when (categoriesResult) {
                is Result.Success -> categoriesResult.data.sortedBy { it.sortOrder }
                is Result.Error -> emptyList()
            }

            val variantGroups = when (variantGroupsResult) {
                is Result.Success -> variantGroupsResult.data.sortedBy { it.sortOrder }
                is Result.Error -> emptyList()
            }

            val modifierGroups = when (modifierGroupsResult) {
                is Result.Success -> modifierGroupsResult.data.sortedBy { it.sortOrder }
                is Result.Error -> emptyList()
            }

            // Load variants for each variant group in parallel
            val variantGroupsWithVariants = variantGroups.map { group ->
                val variantsResult = menuRepository.getVariants(productId, group.id)
                val variants = when (variantsResult) {
                    is Result.Success -> variantsResult.data.sortedBy { it.sortOrder }
                    is Result.Error -> emptyList()
                }
                VariantGroupWithVariants(group = group, variants = variants)
            }

            // Load modifiers for each modifier group
            val modifierGroupsWithModifiers = modifierGroups.map { group ->
                val modifiersResult = menuRepository.getModifiers(productId, group.id)
                val modifiers = when (modifiersResult) {
                    is Result.Success -> modifiersResult.data.sortedBy { it.sortOrder }
                    is Result.Error -> emptyList()
                }
                ModifierGroupWithModifiers(group = group, modifiers = modifiers)
            }

            // Load combo items if product is a combo
            val comboItems = if (product.isCombo) {
                when (val comboResult = menuAdminRepository.getComboItems(productId)) {
                    is Result.Success -> comboResult.data.map { item ->
                        val itemProduct = allProducts.find { it.id == item.productId }
                        ComboItemWithProduct(
                            comboItem = item,
                            productName = itemProduct?.name ?: "Produk tidak dikenal"
                        )
                    }
                    is Result.Error -> emptyList()
                }
            } else {
                emptyList()
            }

            _uiState.update {
                it.copy(
                    isLoading = false,
                    name = product.name,
                    basePrice = product.basePrice,
                    categoryId = product.categoryId,
                    station = product.station ?: "",
                    description = product.description ?: "",
                    preparationTime = product.preparationTime?.toString() ?: "",
                    isCombo = product.isCombo,
                    isActive = product.isActive,
                    categories = categories,
                    allProducts = allProducts.filter { p -> p.id != productId && !p.isCombo },
                    variantGroups = variantGroupsWithVariants,
                    modifierGroups = modifierGroupsWithModifiers,
                    comboItems = comboItems
                )
            }
        }
    }

    private fun loadCategories() {
        viewModelScope.launch {
            when (val result = menuRepository.getCategories()) {
                is Result.Success -> {
                    _uiState.update {
                        it.copy(categories = result.data.sortedBy { cat -> cat.sortOrder })
                    }
                }
                is Result.Error -> {
                    // Non-fatal: user just can't pick a category
                }
            }
        }
    }

    private fun loadAllProducts() {
        viewModelScope.launch {
            when (val result = menuRepository.getProducts()) {
                is Result.Success -> {
                    val productId = _uiState.value.productId
                    _uiState.update {
                        it.copy(
                            allProducts = result.data.filter { p ->
                                p.id != productId && !p.isCombo
                            }
                        )
                    }
                }
                is Result.Error -> {
                    // Non-fatal
                }
            }
        }
    }

    // ── Form field updaters ──

    fun onNameChanged(name: String) {
        _uiState.update { it.copy(name = name) }
    }

    fun onBasePriceChanged(price: String) {
        _uiState.update { it.copy(basePrice = price) }
    }

    fun onCategorySelected(categoryId: String) {
        _uiState.update { it.copy(categoryId = categoryId) }
    }

    fun onStationSelected(station: String) {
        _uiState.update { it.copy(station = station) }
    }

    fun onDescriptionChanged(desc: String) {
        _uiState.update { it.copy(description = desc) }
    }

    fun onPreparationTimeChanged(time: String) {
        _uiState.update { it.copy(preparationTime = time) }
    }

    fun onIsComboChanged(isCombo: Boolean) {
        _uiState.update { it.copy(isCombo = isCombo) }
        // Load products for combo picker if enabling combo and we don't have them yet
        if (isCombo && _uiState.value.allProducts.isEmpty()) {
            loadAllProducts()
        }
    }

    // ── Save product ──

    fun saveProduct() {
        if (_uiState.value.isSaving) return
        val state = _uiState.value
        if (state.name.isBlank() || state.basePrice.isBlank() || state.categoryId.isBlank()) {
            _uiState.update { it.copy(saveError = "Nama, harga, dan kategori wajib diisi") }
            return
        }

        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            val prepTime = state.preparationTime.toIntOrNull()

            if (state.isCreateMode) {
                val request = CreateProductRequest(
                    categoryId = state.categoryId,
                    name = state.name.trim(),
                    description = state.description.trim(),
                    basePrice = state.basePrice.trim(),
                    station = state.station,
                    preparationTime = prepTime,
                    isCombo = state.isCombo
                )
                when (val result = menuAdminRepository.createProduct(request)) {
                    is Result.Success -> {
                        _uiState.update {
                            it.copy(
                                isSaving = false,
                                savedProductId = result.data.id
                            )
                        }
                    }
                    is Result.Error -> {
                        _uiState.update {
                            it.copy(isSaving = false, saveError = result.message)
                        }
                    }
                }
            } else {
                val productId = state.productId ?: return@launch
                val request = UpdateProductRequest(
                    categoryId = state.categoryId,
                    name = state.name.trim(),
                    description = state.description.trim(),
                    basePrice = state.basePrice.trim(),
                    station = state.station,
                    preparationTime = prepTime,
                    isCombo = state.isCombo
                )
                when (val result = menuAdminRepository.updateProduct(productId, request)) {
                    is Result.Success -> {
                        _uiState.update { it.copy(isSaving = false, saveError = null) }
                    }
                    is Result.Error -> {
                        _uiState.update {
                            it.copy(isSaving = false, saveError = result.message)
                        }
                    }
                }
            }
        }
    }

    fun clearSavedProductId() {
        _uiState.update { it.copy(savedProductId = null) }
    }

    // ── Deactivate product ──

    fun showDeactivateDialog() {
        _uiState.update { it.copy(showDeactivateDialog = true) }
    }

    fun dismissDeactivateDialog() {
        _uiState.update { it.copy(showDeactivateDialog = false) }
    }

    fun deactivateProduct() {
        if (_uiState.value.isDeactivating) return
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isDeactivating = true, showDeactivateDialog = false) }

            when (menuAdminRepository.deleteProduct(productId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isDeactivating = false, isActive = false) }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(
                            isDeactivating = false,
                            errorMessage = "Gagal menonaktifkan produk"
                        )
                    }
                }
            }
        }
    }

    // ── Bottom sheet ──

    fun showSheet(sheet: ProductSheet) {
        _uiState.update { it.copy(activeSheet = sheet, saveError = null) }
    }

    fun dismissSheet() {
        _uiState.update { it.copy(activeSheet = null, saveError = null) }
    }

    // ── Variant Group CRUD ──

    fun saveVariantGroup(name: String, isRequired: Boolean) {
        val productId = _uiState.value.productId ?: return
        val sheet = _uiState.value.activeSheet as? ProductSheet.EditVariantGroup ?: return

        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            val existingGroup = sheet.group
            val result = if (existingGroup != null) {
                val request = UpdateVariantGroupRequest(
                    name = name.trim(),
                    isRequired = isRequired,
                    sortOrder = existingGroup.sortOrder
                )
                menuAdminRepository.updateVariantGroup(productId, existingGroup.id, request)
            } else {
                val nextSort = (_uiState.value.variantGroups.maxOfOrNull { it.group.sortOrder } ?: -1) + 1
                val request = CreateVariantGroupRequest(
                    name = name.trim(),
                    isRequired = isRequired,
                    sortOrder = nextSort
                )
                menuAdminRepository.createVariantGroup(productId, request)
            }

            when (result) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadVariantGroups()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    fun deleteVariantGroup(groupId: String) {
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            when (val result = menuAdminRepository.deleteVariantGroup(productId, groupId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadVariantGroups()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    private fun reloadVariantGroups() {
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            when (val result = menuRepository.getVariantGroups(productId)) {
                is Result.Success -> {
                    val groups = result.data.sortedBy { it.sortOrder }
                    val groupsWithVariants = groups.map { group ->
                        val variantsResult = menuRepository.getVariants(productId, group.id)
                        val variants = when (variantsResult) {
                            is Result.Success -> variantsResult.data.sortedBy { it.sortOrder }
                            is Result.Error -> emptyList()
                        }
                        VariantGroupWithVariants(group = group, variants = variants)
                    }
                    _uiState.update { it.copy(variantGroups = groupsWithVariants) }
                }
                is Result.Error -> {
                    // Keep current state on reload failure
                }
            }
        }
    }

    // ── Variant CRUD ──

    fun saveVariant(groupId: String, name: String, priceAdjustment: String) {
        val productId = _uiState.value.productId ?: return
        val sheet = _uiState.value.activeSheet as? ProductSheet.EditVariant ?: return

        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            val existingVariant = sheet.variant
            val result = if (existingVariant != null) {
                val request = UpdateVariantRequest(
                    name = name.trim(),
                    priceAdjustment = priceAdjustment.trim(),
                    sortOrder = existingVariant.sortOrder
                )
                menuAdminRepository.updateVariant(productId, groupId, existingVariant.id, request)
            } else {
                val currentGroup = _uiState.value.variantGroups.find { it.group.id == groupId }
                val nextSort = (currentGroup?.variants?.maxOfOrNull { it.sortOrder } ?: -1) + 1
                val request = CreateVariantRequest(
                    name = name.trim(),
                    priceAdjustment = priceAdjustment.trim(),
                    sortOrder = nextSort
                )
                menuAdminRepository.createVariant(productId, groupId, request)
            }

            when (result) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadVariantGroups()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    fun deleteVariant(groupId: String, variantId: String) {
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            when (val result = menuAdminRepository.deleteVariant(productId, groupId, variantId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadVariantGroups()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    // ── Modifier Group CRUD ──

    fun saveModifierGroup(name: String, minSelect: Int, maxSelect: Int) {
        val productId = _uiState.value.productId ?: return
        val sheet = _uiState.value.activeSheet as? ProductSheet.EditModifierGroup ?: return

        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            val existingGroup = sheet.group
            val result = if (existingGroup != null) {
                val request = UpdateModifierGroupRequest(
                    name = name.trim(),
                    minSelect = minSelect,
                    maxSelect = maxSelect,
                    sortOrder = existingGroup.sortOrder
                )
                menuAdminRepository.updateModifierGroup(productId, existingGroup.id, request)
            } else {
                val nextSort = (_uiState.value.modifierGroups.maxOfOrNull { it.group.sortOrder } ?: -1) + 1
                val request = CreateModifierGroupRequest(
                    name = name.trim(),
                    minSelect = minSelect,
                    maxSelect = maxSelect,
                    sortOrder = nextSort
                )
                menuAdminRepository.createModifierGroup(productId, request)
            }

            when (result) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadModifierGroups()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    fun deleteModifierGroup(groupId: String) {
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            when (val result = menuAdminRepository.deleteModifierGroup(productId, groupId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadModifierGroups()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    private fun reloadModifierGroups() {
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            when (val result = menuRepository.getModifierGroups(productId)) {
                is Result.Success -> {
                    val groups = result.data.sortedBy { it.sortOrder }
                    val groupsWithModifiers = groups.map { group ->
                        val modifiersResult = menuRepository.getModifiers(productId, group.id)
                        val modifiers = when (modifiersResult) {
                            is Result.Success -> modifiersResult.data.sortedBy { it.sortOrder }
                            is Result.Error -> emptyList()
                        }
                        ModifierGroupWithModifiers(group = group, modifiers = modifiers)
                    }
                    _uiState.update { it.copy(modifierGroups = groupsWithModifiers) }
                }
                is Result.Error -> {
                    // Keep current state on reload failure
                }
            }
        }
    }

    // ── Modifier CRUD ──

    fun saveModifier(groupId: String, name: String, price: String) {
        val productId = _uiState.value.productId ?: return
        val sheet = _uiState.value.activeSheet as? ProductSheet.EditModifier ?: return

        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            val existingModifier = sheet.modifier
            val result = if (existingModifier != null) {
                val request = UpdateModifierRequest(
                    name = name.trim(),
                    price = price.trim(),
                    sortOrder = existingModifier.sortOrder
                )
                menuAdminRepository.updateModifier(productId, groupId, existingModifier.id, request)
            } else {
                val currentGroup = _uiState.value.modifierGroups.find { it.group.id == groupId }
                val nextSort = (currentGroup?.modifiers?.maxOfOrNull { it.sortOrder } ?: -1) + 1
                val request = CreateModifierRequest(
                    name = name.trim(),
                    price = price.trim(),
                    sortOrder = nextSort
                )
                menuAdminRepository.createModifier(productId, groupId, request)
            }

            when (result) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadModifierGroups()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    fun deleteModifier(groupId: String, modifierId: String) {
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            when (val result = menuAdminRepository.deleteModifier(productId, groupId, modifierId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadModifierGroups()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    // ── Combo Items ──

    fun addComboItem(productId: String, quantity: Int) {
        val comboProductId = _uiState.value.productId ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            val nextSort = (_uiState.value.comboItems.maxOfOrNull { it.comboItem.sortOrder } ?: -1) + 1
            val request = CreateComboItemRequest(
                productId = productId,
                quantity = quantity,
                sortOrder = nextSort
            )

            when (val result = menuAdminRepository.addComboItem(comboProductId, request)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, activeSheet = null) }
                    reloadComboItems()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    fun removeComboItem(comboItemId: String) {
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }

            when (val result = menuAdminRepository.removeComboItem(productId, comboItemId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false) }
                    reloadComboItems()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }

    private fun reloadComboItems() {
        val productId = _uiState.value.productId ?: return
        viewModelScope.launch {
            when (val result = menuAdminRepository.getComboItems(productId)) {
                is Result.Success -> {
                    val allProducts = _uiState.value.allProducts
                    val items = result.data.map { item ->
                        val itemProduct = allProducts.find { it.id == item.productId }
                        ComboItemWithProduct(
                            comboItem = item,
                            productName = itemProduct?.name ?: "Produk tidak dikenal"
                        )
                    }
                    _uiState.update { it.copy(comboItems = items) }
                }
                is Result.Error -> {
                    // Keep current state on reload failure
                }
            }
        }
    }
}
