package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.CustomerApi
import com.kiwari.pos.data.model.CreateCustomerRequest
import com.kiwari.pos.data.model.Customer
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
}
