package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.CustomerApi
import com.kiwari.pos.data.model.CreateCustomerRequest
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.data.model.CustomerOrderResponse
import com.kiwari.pos.data.model.CustomerStatsResponse
import com.kiwari.pos.data.model.UpdateCustomerRequest
import com.kiwari.pos.data.model.Result
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class CustomerRepository @Inject constructor(
    private val customerApi: CustomerApi,
    private val tokenRepository: TokenRepository,
    private val gson: Gson
) {
    suspend fun searchCustomers(query: String): Result<List<Customer>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { customerApi.searchCustomers(outletId, query) }
    }

    suspend fun createCustomer(name: String, phone: String): Result<Customer> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) {
            customerApi.createCustomer(outletId, CreateCustomerRequest(name = name, phone = phone))
        }
    }

    suspend fun listCustomers(): Result<List<Customer>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { customerApi.listCustomers(outletId) }
    }

    suspend fun getCustomer(customerId: String): Result<Customer> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { customerApi.getCustomer(outletId, customerId) }
    }

    suspend fun updateCustomer(customerId: String, name: String, phone: String, email: String?, notes: String?): Result<Customer> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) {
            customerApi.updateCustomer(outletId, customerId, UpdateCustomerRequest(name, phone, email, notes))
        }
    }

    suspend fun deleteCustomer(customerId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { customerApi.deleteCustomer(outletId, customerId) }
    }

    suspend fun getCustomerStats(customerId: String): Result<CustomerStatsResponse> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { customerApi.getCustomerStats(outletId, customerId) }
    }

    suspend fun getCustomerOrders(customerId: String): Result<List<CustomerOrderResponse>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { customerApi.getCustomerOrders(outletId, customerId) }
    }
}
