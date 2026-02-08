package com.kiwari.pos.data.repository

import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.ui.cart.DiscountType
import com.kiwari.pos.ui.cart.OrderType
import java.math.BigDecimal
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Holds order metadata set by CartViewModel, consumed by PaymentViewModel.
 * Singleton so state survives navigation between Cart and Payment screens.
 */
data class OrderMetadata(
    val orderType: OrderType = OrderType.DINE_IN,
    val tableNumber: String = "",
    val customer: Customer? = null,
    val discountType: DiscountType? = null,
    val discountValue: String = "",
    val discountAmount: BigDecimal = BigDecimal.ZERO,
    val notes: String = "",
    val subtotal: BigDecimal = BigDecimal.ZERO,
    val total: BigDecimal = BigDecimal.ZERO,
    val cateringDate: String? = null,
    val cateringDpAmount: BigDecimal? = null,
    val deliveryAddress: String? = null
)

@Singleton
class OrderMetadataRepository @Inject constructor() {

    @Volatile
    private var _metadata: OrderMetadata = OrderMetadata()
    val metadata: OrderMetadata get() = _metadata

    fun setMetadata(metadata: OrderMetadata) {
        _metadata = metadata
    }

    fun clear() {
        _metadata = OrderMetadata()
    }
}
