package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class CustomerStatsResponse(
    @SerializedName("total_orders")
    val totalOrders: Long,

    @SerializedName("total_spend")
    val totalSpend: String,

    @SerializedName("avg_ticket")
    val avgTicket: String,

    @SerializedName("top_items")
    val topItems: List<TopItemResponse>
)

data class TopItemResponse(
    @SerializedName("product_id")
    val productId: String,

    @SerializedName("product_name")
    val productName: String,

    @SerializedName("total_qty")
    val totalQty: Long,

    @SerializedName("total_revenue")
    val totalRevenue: String
)

data class UpdateCustomerRequest(
    @SerializedName("name")
    val name: String,

    @SerializedName("phone")
    val phone: String,

    @SerializedName("email")
    val email: String? = null,

    @SerializedName("notes")
    val notes: String? = null
)

data class CustomerOrderResponse(
    @SerializedName("id")
    val id: String,

    @SerializedName("order_number")
    val orderNumber: String,

    @SerializedName("order_type")
    val orderType: String,

    @SerializedName("status")
    val status: String,

    @SerializedName("total_amount")
    val totalAmount: String,

    @SerializedName("created_at")
    val createdAt: String
)
