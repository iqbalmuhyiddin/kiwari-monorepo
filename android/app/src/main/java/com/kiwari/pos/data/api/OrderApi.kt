package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.AddPaymentRequest
import com.kiwari.pos.data.model.AddPaymentResponse
import com.kiwari.pos.data.model.CreateOrderRequest
import com.kiwari.pos.data.model.OrderResponse
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.POST
import retrofit2.http.Path

interface OrderApi {
    @POST("outlets/{outletId}/orders")
    suspend fun createOrder(
        @Path("outletId") outletId: String,
        @Body request: CreateOrderRequest
    ): Response<OrderResponse>

    @POST("outlets/{outletId}/orders/{orderId}/payments")
    suspend fun addPayment(
        @Path("outletId") outletId: String,
        @Path("orderId") orderId: String,
        @Body request: AddPaymentRequest
    ): Response<AddPaymentResponse>
}
