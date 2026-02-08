package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

// ── Create Order Request ──────────────────────

data class CreateOrderRequest(
    @SerializedName("order_type")
    val orderType: String,

    @SerializedName("table_number")
    val tableNumber: String? = null,

    @SerializedName("customer_id")
    val customerId: String? = null,

    @SerializedName("notes")
    val notes: String? = null,

    @SerializedName("discount_type")
    val discountType: String? = null,

    @SerializedName("discount_value")
    val discountValue: String? = null,

    @SerializedName("catering_date")
    val cateringDate: String? = null,

    @SerializedName("catering_dp_amount")
    val cateringDpAmount: String? = null,

    @SerializedName("delivery_platform")
    val deliveryPlatform: String? = null,

    @SerializedName("delivery_address")
    val deliveryAddress: String? = null,

    @SerializedName("items")
    val items: List<CreateOrderItemRequest>
)

data class CreateOrderItemRequest(
    @SerializedName("product_id")
    val productId: String,

    @SerializedName("variant_id")
    val variantId: String? = null,

    @SerializedName("quantity")
    val quantity: Int,

    @SerializedName("notes")
    val notes: String? = null,

    @SerializedName("modifiers")
    val modifiers: List<CreateOrderItemModifierRequest>? = null
)

data class CreateOrderItemModifierRequest(
    @SerializedName("modifier_id")
    val modifierId: String,

    @SerializedName("quantity")
    val quantity: Int = 1
)

// ── Create Order Response ──────────────────────

data class OrderResponse(
    @SerializedName("id")
    val id: String,

    @SerializedName("order_number")
    val orderNumber: String,

    @SerializedName("outlet_id")
    val outletId: String,

    @SerializedName("order_type")
    val orderType: String,

    @SerializedName("status")
    val status: String,

    @SerializedName("total_amount")
    val totalAmount: String,

    @SerializedName("discount_type")
    val discountType: String?,

    @SerializedName("discount_value")
    val discountValue: String?,

    @SerializedName("notes")
    val notes: String?,

    @SerializedName("table_number")
    val tableNumber: String?,

    @SerializedName("customer_id")
    val customerId: String?,

    @SerializedName("created_at")
    val createdAt: String,

    @SerializedName("updated_at")
    val updatedAt: String
)

// ── Add Payment Request ──────────────────────

data class AddPaymentRequest(
    @SerializedName("payment_method")
    val paymentMethod: String,

    @SerializedName("amount")
    val amount: String,

    @SerializedName("amount_received")
    val amountReceived: String? = null,

    @SerializedName("reference_number")
    val referenceNumber: String? = null
)

// ── Add Payment Response ──────────────────────

data class AddPaymentResponse(
    @SerializedName("payment")
    val payment: PaymentResponse,

    @SerializedName("order")
    val order: OrderResponse
)

data class PaymentResponse(
    @SerializedName("id")
    val id: String,

    @SerializedName("order_id")
    val orderId: String,

    @SerializedName("payment_method")
    val paymentMethod: String,

    @SerializedName("amount")
    val amount: String,

    @SerializedName("amount_received")
    val amountReceived: String?,

    @SerializedName("change_amount")
    val changeAmount: String?,

    @SerializedName("reference_number")
    val referenceNumber: String?,

    @SerializedName("created_at")
    val createdAt: String
)
