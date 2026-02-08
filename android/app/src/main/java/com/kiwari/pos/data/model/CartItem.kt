package com.kiwari.pos.data.model

import java.math.BigDecimal

/**
 * Domain model for cart items.
 * Not an API model — managed locally until order submission.
 */
data class CartItem(
    val id: String, // Local UUID to identify this specific cart item
    val product: Product,
    val selectedVariants: List<SelectedVariant> = emptyList(),
    val selectedModifiers: List<SelectedModifier> = emptyList(),
    val quantity: Int,
    val notes: String = "",
    val lineTotal: BigDecimal // Calculated: (basePrice + variantAdjustment + sum(modifierPrices)) * quantity
)

data class SelectedVariant(
    val variantGroupId: String,
    val variantGroupName: String,
    val variantId: String,
    val variantName: String,
    val priceAdjustment: BigDecimal
)

data class SelectedModifier(
    val modifierGroupId: String,
    val modifierGroupName: String,
    val modifierId: String,
    val modifierName: String,
    val price: BigDecimal
)
