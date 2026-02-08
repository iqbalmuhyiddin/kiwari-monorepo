package com.kiwari.pos.ui.menu

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.data.model.Category
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.data.model.Result
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

data class MenuUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val categories: List<Category> = emptyList(),
    val allProducts: List<Product> = emptyList(),
    val selectedCategoryId: String? = null,
    val searchQuery: String = "",
    // Map of productId -> hasRequiredVariants
    val productVariantMap: Map<String, Boolean> = emptyMap(),
    // Cart state (mirrored from CartRepository for reactivity)
    val cartItems: List<CartItem> = emptyList(),
    val cartTotalItems: Int = 0,
    val cartTotalPrice: BigDecimal = BigDecimal.ZERO,
    // Pre-computed filtered products list
    val filteredProducts: List<Product> = emptyList(),
    // Quick edit popup
    val quickEditCartItem: CartItem? = null
)

@HiltViewModel
class MenuViewModel @Inject constructor(
    private val menuRepository: MenuRepository,
    private val cartRepository: CartRepository,
    private val selectedProductRepository: SelectedProductRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(MenuUiState())
    val uiState: StateFlow<MenuUiState> = _uiState.asStateFlow()

    init {
        loadMenu()
        observeCart()
    }

    private fun observeCart() {
        viewModelScope.launch {
            cartRepository.items.collect { items ->
                _uiState.update {
                    it.copy(
                        cartItems = items,
                        cartTotalItems = items.sumOf { item -> item.quantity },
                        cartTotalPrice = items.fold(BigDecimal.ZERO) { acc, item ->
                            acc.add(item.lineTotal)
                        }
                    )
                }
            }
        }
    }

    private fun loadMenu() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessage = null) }

            // Load categories and products in parallel
            val categoriesDeferred = async { menuRepository.getCategories() }
            val productsDeferred = async { menuRepository.getProducts() }
            val categoriesResult = categoriesDeferred.await()
            val productsResult = productsDeferred.await()

            when {
                categoriesResult is Result.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, errorMessage = categoriesResult.message)
                    }
                    return@launch
                }
                productsResult is Result.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, errorMessage = productsResult.message)
                    }
                    return@launch
                }
            }

            val categories = (categoriesResult as Result.Success).data
                .filter { it.isActive }
                .sortedBy { it.sortOrder }
            val products = (productsResult as Result.Success).data
                .filter { it.isActive }

            _uiState.update {
                val updated = it.copy(
                    categories = categories,
                    allProducts = products,
                    isLoading = false
                )
                updated.copy(filteredProducts = computeFilteredProducts(updated))
            }

            // Load variant groups for all products to determine which need customization
            loadVariantInfo(products)
        }
    }

    private suspend fun loadVariantInfo(products: List<Product>) {
        val entries = coroutineScope {
            products.map { product ->
                async {
                    val hasRequired = when (val result = menuRepository.getVariantGroups(product.id)) {
                        is Result.Success -> result.data.any { it.isRequired && it.isActive }
                        is Result.Error -> false // If we can't load variant info, assume no required variants
                    }
                    product.id to hasRequired
                }
            }.awaitAll()
        }
        _uiState.update { it.copy(productVariantMap = entries.toMap()) }
    }

    fun onCategorySelected(categoryId: String?) {
        _uiState.update {
            val updated = it.copy(selectedCategoryId = categoryId)
            updated.copy(filteredProducts = computeFilteredProducts(updated))
        }
    }

    fun onSearchQueryChanged(query: String) {
        _uiState.update {
            val updated = it.copy(searchQuery = query)
            updated.copy(filteredProducts = computeFilteredProducts(updated))
        }
    }

    /**
     * Called when user taps a product.
     * Returns true if product was added to cart (simple product).
     * Returns false if product needs customization (has required variants).
     */
    fun onProductTapped(product: Product): Boolean {
        val hasRequired = _uiState.value.productVariantMap[product.id] == true
        if (hasRequired) {
            selectedProductRepository.set(product)
            return false // Caller should navigate to customization
        }
        cartRepository.addSimpleProduct(product)
        return true
    }

    /**
     * Called when user long-presses a product.
     * Opens quick edit popup if the product is already in cart.
     * If not in cart, add it first then open quick edit.
     */
    fun onProductLongPressed(product: Product) {
        val cartItem = cartRepository.findCartItemForProduct(product.id)
            ?: run {
                // Add to cart first, then show popup
                val hasRequired = _uiState.value.productVariantMap[product.id] == true
                if (hasRequired) {
                    // Can't quick-edit a product that needs customization and isn't in cart
                    return
                }
                cartRepository.addSimpleProduct(product)
            }
        _uiState.update { it.copy(quickEditCartItem = cartItem) }
    }

    fun onQuickEditDismissed() {
        _uiState.update { it.copy(quickEditCartItem = null) }
    }

    fun onQuickEditUpdateQuantity(cartItemId: String, quantity: Int) {
        cartRepository.updateQuantity(cartItemId, quantity)
    }

    fun onQuickEditUpdateNotes(cartItemId: String, notes: String) {
        cartRepository.updateNotes(cartItemId, notes)
    }

    fun onQuickEditRemove(cartItemId: String) {
        cartRepository.removeItem(cartItemId)
        _uiState.update { it.copy(quickEditCartItem = null) }
    }

    fun retry() {
        loadMenu()
    }

    private fun computeFilteredProducts(state: MenuUiState): List<Product> {
        var filtered = state.allProducts
        if (state.selectedCategoryId != null) {
            filtered = filtered.filter { it.categoryId == state.selectedCategoryId }
        }
        if (state.searchQuery.isNotBlank()) {
            val query = state.searchQuery.lowercase()
            filtered = filtered.filter { it.name.lowercase().contains(query) }
        }
        return filtered
    }
}
