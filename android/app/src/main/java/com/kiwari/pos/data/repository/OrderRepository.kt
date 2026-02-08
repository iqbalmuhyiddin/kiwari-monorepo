package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.OrderApi
import com.kiwari.pos.data.model.AddPaymentRequest
import com.kiwari.pos.data.model.AddPaymentResponse
import com.kiwari.pos.data.model.CreateOrderRequest
import com.kiwari.pos.data.model.OrderResponse
import com.kiwari.pos.data.model.Result
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
}
