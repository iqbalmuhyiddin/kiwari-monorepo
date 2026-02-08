package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.CreateCustomerRequest
import com.kiwari.pos.data.model.Customer
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
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
}
