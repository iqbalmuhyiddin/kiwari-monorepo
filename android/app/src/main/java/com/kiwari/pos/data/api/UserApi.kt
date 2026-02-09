package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.CreateUserRequest
import com.kiwari.pos.data.model.StaffMember
import com.kiwari.pos.data.model.UpdateUserRequest
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path

interface UserApi {
    @GET("outlets/{outletId}/users")
    suspend fun listUsers(
        @Path("outletId") outletId: String
    ): Response<List<StaffMember>>

    @POST("outlets/{outletId}/users")
    suspend fun createUser(
        @Path("outletId") outletId: String,
        @Body request: CreateUserRequest
    ): Response<StaffMember>

    @PUT("outlets/{outletId}/users/{userId}")
    suspend fun updateUser(
        @Path("outletId") outletId: String,
        @Path("userId") userId: String,
        @Body request: UpdateUserRequest
    ): Response<StaffMember>

    @DELETE("outlets/{outletId}/users/{userId}")
    suspend fun deleteUser(
        @Path("outletId") outletId: String,
        @Path("userId") userId: String
    ): Response<Void>
}
