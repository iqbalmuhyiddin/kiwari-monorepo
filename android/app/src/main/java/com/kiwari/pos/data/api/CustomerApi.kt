package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.CreateCustomerRequest
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.data.model.CustomerOrderResponse
import com.kiwari.pos.data.model.CustomerStatsResponse
import com.kiwari.pos.data.model.UpdateCustomerRequest
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

interface CustomerApi {
    @GET("outlets/{outletId}/customers")
    suspend fun searchCustomers(
        @Path("outletId") outletId: String,
        @Query("search") search: String,
        @Query("limit") limit: Int = 10
    ): Response<List<Customer>>

    @POST("outlets/{outletId}/customers")
    suspend fun createCustomer(
        @Path("outletId") outletId: String,
        @Body request: CreateCustomerRequest
    ): Response<Customer>

    @GET("outlets/{outletId}/customers")
    suspend fun listCustomers(
        @Path("outletId") outletId: String,
        @Query("search") search: String? = null,
        @Query("limit") limit: Int = 100,
        @Query("offset") offset: Int = 0
    ): Response<List<Customer>>

    @GET("outlets/{outletId}/customers/{customerId}")
    suspend fun getCustomer(
        @Path("outletId") outletId: String,
        @Path("customerId") customerId: String
    ): Response<Customer>

    @PUT("outlets/{outletId}/customers/{customerId}")
    suspend fun updateCustomer(
        @Path("outletId") outletId: String,
        @Path("customerId") customerId: String,
        @Body request: UpdateCustomerRequest
    ): Response<Customer>

    @DELETE("outlets/{outletId}/customers/{customerId}")
    suspend fun deleteCustomer(
        @Path("outletId") outletId: String,
        @Path("customerId") customerId: String
    ): Response<Void>

    @GET("outlets/{outletId}/customers/{customerId}/stats")
    suspend fun getCustomerStats(
        @Path("outletId") outletId: String,
        @Path("customerId") customerId: String
    ): Response<CustomerStatsResponse>

    @GET("outlets/{outletId}/customers/{customerId}/orders")
    suspend fun getCustomerOrders(
        @Path("outletId") outletId: String,
        @Path("customerId") customerId: String,
        @Query("limit") limit: Int = 20,
        @Query("offset") offset: Int = 0
    ): Response<List<CustomerOrderResponse>>
}
