package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.ActiveOrdersResponse
import com.kiwari.pos.data.model.AddOrderItemRequest
import com.kiwari.pos.data.model.AddPaymentRequest
import com.kiwari.pos.data.model.AddPaymentResponse
import com.kiwari.pos.data.model.CreateOrderRequest
import com.kiwari.pos.data.model.ItemActionResponse
import com.kiwari.pos.data.model.OrderDetailResponse
import com.kiwari.pos.data.model.OrderResponse
import com.kiwari.pos.data.model.UpdateOrderItemRequest
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

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

    @GET("outlets/{outletId}/orders/active")
    suspend fun listActiveOrders(
        @Path("outletId") outletId: String,
        @Query("limit") limit: Int = 50,
        @Query("offset") offset: Int = 0
    ): Response<ActiveOrdersResponse>

    @GET("outlets/{outletId}/orders/{orderId}")
    suspend fun getOrder(
        @Path("outletId") outletId: String,
        @Path("orderId") orderId: String
    ): Response<OrderDetailResponse>

    @DELETE("outlets/{outletId}/orders/{orderId}")
    suspend fun cancelOrder(
        @Path("outletId") outletId: String,
        @Path("orderId") orderId: String
    ): Response<OrderResponse>

    @POST("outlets/{outletId}/orders/{orderId}/items")
    suspend fun addOrderItem(
        @Path("outletId") outletId: String,
        @Path("orderId") orderId: String,
        @Body request: AddOrderItemRequest
    ): Response<ItemActionResponse>

    @PUT("outlets/{outletId}/orders/{orderId}/items/{itemId}")
    suspend fun updateOrderItem(
        @Path("outletId") outletId: String,
        @Path("orderId") orderId: String,
        @Path("itemId") itemId: String,
        @Body request: UpdateOrderItemRequest
    ): Response<ItemActionResponse>

    @DELETE("outlets/{outletId}/orders/{orderId}/items/{itemId}")
    suspend fun deleteOrderItem(
        @Path("outletId") outletId: String,
        @Path("orderId") orderId: String,
        @Path("itemId") itemId: String
    ): Response<ItemActionResponse>
}
