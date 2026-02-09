package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class DailySalesResponse(
    @SerializedName("date") val date: String,
    @SerializedName("order_count") val orderCount: Long,
    @SerializedName("total_revenue") val totalRevenue: String,
    @SerializedName("total_discount") val totalDiscount: String,
    @SerializedName("net_revenue") val netRevenue: String
)

data class ProductSalesResponse(
    @SerializedName("product_id") val productId: String,
    @SerializedName("product_name") val productName: String,
    @SerializedName("quantity_sold") val quantitySold: Long,
    @SerializedName("total_revenue") val totalRevenue: String
)

data class PaymentSummaryResponse(
    @SerializedName("payment_method") val paymentMethod: String,
    @SerializedName("transaction_count") val transactionCount: Long,
    @SerializedName("total_amount") val totalAmount: String
)

data class HourlySalesResponse(
    @SerializedName("hour") val hour: Int,
    @SerializedName("order_count") val orderCount: Long,
    @SerializedName("total_revenue") val totalRevenue: String
)

data class OutletComparisonResponse(
    @SerializedName("outlet_id") val outletId: String,
    @SerializedName("outlet_name") val outletName: String,
    @SerializedName("order_count") val orderCount: Long,
    @SerializedName("total_revenue") val totalRevenue: String
)
