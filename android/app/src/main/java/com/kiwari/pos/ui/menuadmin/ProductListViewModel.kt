package com.kiwari.pos.ui.menuadmin

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.MenuAdminRepository
import com.kiwari.pos.data.repository.MenuRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class ProductListUiState(
    val isLoading: Boolean = true,
    val isRefreshing: Boolean = false,
    val errorMessage: String? = null,
    val categoryId: String = "",
    val categoryName: String = "",
    val products: List<Product> = emptyList(),
    val variantGroupCounts: Map<String, Int> = emptyMap(),
    val modifierGroupCounts: Map<String, Int> = emptyMap(),
    // Delete state
    val deletingProduct: Product? = null,
    val isDeleting: Boolean = false,
    val deleteError: String? = null
)

@HiltViewModel
class ProductListViewModel @Inject constructor(
    private val menuRepository: MenuRepository,
    private val menuAdminRepository: MenuAdminRepository,
    savedStateHandle: SavedStateHandle
) : ViewModel() {

    private val categoryId: String = checkNotNull(savedStateHandle["categoryId"])
    private val categoryName: String = checkNotNull(savedStateHandle["categoryName"])

    private val _uiState = MutableStateFlow(
        ProductListUiState(
            categoryId = categoryId,
            categoryName = categoryName
        )
    )
    val uiState: StateFlow<ProductListUiState> = _uiState.asStateFlow()

    private var loadJob: Job? = null

    init {
        loadProducts()
    }

    private fun loadProducts(isRefresh: Boolean = false) {
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _uiState.update {
                if (isRefresh) it.copy(isRefreshing = true, errorMessage = null)
                else it.copy(isLoading = true, errorMessage = null)
            }

            when (val result = menuRepository.getProducts()) {
                is Result.Success -> {
                    val filtered = result.data
                        .filter { it.categoryId == categoryId }
                        .sortedBy { it.name.lowercase() }

                    // Load variant and modifier group counts in parallel
                    val variantDeferreds = filtered.map { product ->
                        async { product.id to menuRepository.getVariantGroups(product.id) }
                    }
                    val modifierDeferreds = filtered.map { product ->
                        async { product.id to menuRepository.getModifierGroups(product.id) }
                    }

                    val variantResults = variantDeferreds.awaitAll()
                    val modifierResults = modifierDeferreds.awaitAll()

                    val variantCounts = mutableMapOf<String, Int>()
                    for ((productId, variantResult) in variantResults) {
                        variantCounts[productId] = when (variantResult) {
                            is Result.Success -> variantResult.data.size
                            is Result.Error -> 0
                        }
                    }

                    val modifierCounts = mutableMapOf<String, Int>()
                    for ((productId, modifierResult) in modifierResults) {
                        modifierCounts[productId] = when (modifierResult) {
                            is Result.Success -> modifierResult.data.size
                            is Result.Error -> 0
                        }
                    }

                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            isRefreshing = false,
                            products = filtered,
                            variantGroupCounts = variantCounts,
                            modifierGroupCounts = modifierCounts
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            isRefreshing = false,
                            errorMessage = result.message
                        )
                    }
                }
            }
        }
    }

    fun refresh() {
        loadProducts(isRefresh = true)
    }

    fun retry() {
        loadProducts()
    }

    // ── Delete dialog ──

    fun showDeleteDialog(product: Product) {
        _uiState.update { it.copy(deletingProduct = product, deleteError = null) }
    }

    fun dismissDeleteDialog() {
        _uiState.update { it.copy(deletingProduct = null, deleteError = null) }
    }

    fun deleteProduct() {
        if (_uiState.value.isDeleting) return
        val productId = _uiState.value.deletingProduct?.id ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isDeleting = true, deleteError = null) }

            when (val result = menuAdminRepository.deleteProduct(productId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isDeleting = false, deletingProduct = null) }
                    loadProducts()
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isDeleting = false, deleteError = result.message)
                    }
                }
            }
        }
    }
}
