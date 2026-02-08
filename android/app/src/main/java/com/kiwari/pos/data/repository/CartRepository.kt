package com.kiwari.pos.data.repository

import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.data.model.SelectedModifier
import com.kiwari.pos.data.model.SelectedVariant
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import java.math.BigDecimal
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Shared cart state that survives navigation between screens.
 * Singleton scoped so Menu, Customization, Cart, and Payment all see the same cart.
 */
@Singleton
class CartRepository @Inject constructor() {

    private val _items = MutableStateFlow<List<CartItem>>(emptyList())
    val items: StateFlow<List<CartItem>> = _items.asStateFlow()

    /**
     * Add a simple product (no variants/modifiers) to cart.
     * If an identical simple product already exists, increment quantity.
     * Returns the created or updated CartItem.
     */
    fun addSimpleProduct(product: Product): CartItem {
        var resultItem: CartItem? = null
        _items.update { currentItems ->
            val existing = currentItems.find { item ->
                item.product.id == product.id
                        && item.selectedVariants.isEmpty()
                        && item.selectedModifiers.isEmpty()
                        && item.notes.isEmpty()
            }
            if (existing != null) {
                currentItems.map { item ->
                    if (item.id == existing.id) {
                        val newQty = item.quantity + 1
                        val updated = item.copy(
                            quantity = newQty,
                            lineTotal = calculateLineTotal(
                                product.basePrice,
                                emptyList(),
                                emptyList(),
                                newQty
                            )
                        )
                        resultItem = updated
                        updated
                    } else item
                }
            } else {
                val newItem = CartItem(
                    id = UUID.randomUUID().toString(),
                    product = product,
                    quantity = 1,
                    lineTotal = BigDecimal(product.basePrice)
                )
                resultItem = newItem
                currentItems + newItem
            }
        }
        return resultItem!!
    }

    /**
     * Add a customized product (with variant/modifiers) to cart.
     * Always creates a new cart item since customizations differ.
     */
    fun addCustomizedProduct(
        product: Product,
        selectedVariants: List<SelectedVariant>,
        selectedModifiers: List<SelectedModifier>,
        quantity: Int = 1,
        notes: String = ""
    ) {
        val lineTotal = calculateLineTotal(
            product.basePrice,
            selectedVariants,
            selectedModifiers,
            quantity
        )
        _items.update { currentItems ->
            currentItems + CartItem(
                id = UUID.randomUUID().toString(),
                product = product,
                selectedVariants = selectedVariants,
                selectedModifiers = selectedModifiers,
                quantity = quantity,
                notes = notes,
                lineTotal = lineTotal
            )
        }
    }

    /**
     * Update quantity of a specific cart item. Removes item if qty <= 0.
     */
    fun updateQuantity(cartItemId: String, newQuantity: Int) {
        if (newQuantity <= 0) {
            removeItem(cartItemId)
            return
        }
        _items.update { currentItems ->
            currentItems.map { item ->
                if (item.id == cartItemId) {
                    item.copy(
                        quantity = newQuantity,
                        lineTotal = calculateLineTotal(
                            item.product.basePrice,
                            item.selectedVariants,
                            item.selectedModifiers,
                            newQuantity
                        )
                    )
                } else item
            }
        }
    }

    /**
     * Update notes on a specific cart item.
     */
    fun updateNotes(cartItemId: String, notes: String) {
        _items.update { currentItems ->
            currentItems.map { item ->
                if (item.id == cartItemId) item.copy(notes = notes)
                else item
            }
        }
    }

    fun removeItem(cartItemId: String) {
        _items.update { currentItems ->
            currentItems.filter { it.id != cartItemId }
        }
    }

    fun clearCart() {
        _items.update { emptyList() }
    }

    /**
     * Get quantity of a product in cart (across all cart items for that product).
     */
    fun getProductQuantity(productId: String): Int {
        return _items.value
            .filter { it.product.id == productId }
            .sumOf { it.quantity }
    }

    /**
     * Find the first cart item for a given product (for quick-edit on simple products).
     */
    fun findCartItemForProduct(productId: String): CartItem? {
        return _items.value.find { it.product.id == productId }
    }

    private fun calculateLineTotal(
        basePriceStr: String,
        variants: List<SelectedVariant>,
        modifiers: List<SelectedModifier>,
        quantity: Int
    ): BigDecimal {
        val basePrice = BigDecimal(basePriceStr)
        val variantAdj = variants.fold(BigDecimal.ZERO) { acc, v -> acc.add(v.priceAdjustment) }
        val modifierTotal = modifiers.fold(BigDecimal.ZERO) { acc, mod -> acc.add(mod.price) }
        val unitPrice = basePrice.add(variantAdj).add(modifierTotal)
        return unitPrice.multiply(BigDecimal(quantity))
    }
}
