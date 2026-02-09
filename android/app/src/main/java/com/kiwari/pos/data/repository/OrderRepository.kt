package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.OrderApi
import com.kiwari.pos.data.model.ActiveOrdersResponse
import com.kiwari.pos.data.model.AddOrderItemRequest
import com.kiwari.pos.data.model.AddPaymentRequest
import com.kiwari.pos.data.model.AddPaymentResponse
import com.kiwari.pos.data.model.CreateOrderRequest
import com.kiwari.pos.data.model.ItemActionResponse
import com.kiwari.pos.data.model.OrderDetailResponse
import com.kiwari.pos.data.model.OrderResponse
import com.kiwari.pos.data.model.OrdersListResponse
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.UpdateOrderItemRequest
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class OrderRepository @Inject constructor(
    private val orderApi: OrderApi,
    private val tokenRepository: TokenRepository,
    private val gson: Gson
) {
    suspend fun createOrder(request: CreateOrderRequest): Result<OrderResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.createOrder(outletId, request) }
    }

    suspend fun addPayment(orderId: String, request: AddPaymentRequest): Result<AddPaymentResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.addPayment(outletId, orderId, request) }
    }

    suspend fun listOrders(
        status: String? = null,
        startDate: String? = null,
        endDate: String? = null,
        limit: Int = 50,
        offset: Int = 0
    ): Result<OrdersListResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.listOrders(outletId, status, null, startDate, endDate, limit, offset) }
    }

    suspend fun listActiveOrders(): Result<ActiveOrdersResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.listActiveOrders(outletId) }
    }

    suspend fun getOrder(orderId: String): Result<OrderDetailResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.getOrder(outletId, orderId) }
    }

    suspend fun cancelOrder(orderId: String): Result<OrderResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.cancelOrder(outletId, orderId) }
    }

    suspend fun addOrderItem(orderId: String, request: AddOrderItemRequest): Result<ItemActionResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.addOrderItem(outletId, orderId, request) }
    }

    suspend fun updateOrderItem(orderId: String, itemId: String, request: UpdateOrderItemRequest): Result<ItemActionResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.updateOrderItem(outletId, orderId, itemId, request) }
    }

    suspend fun deleteOrderItem(orderId: String, itemId: String): Result<ItemActionResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { orderApi.deleteOrderItem(outletId, orderId, itemId) }
    }
}
