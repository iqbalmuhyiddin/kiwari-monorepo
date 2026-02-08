package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.AuthResponse
import com.kiwari.pos.data.model.LoginRequest
import com.kiwari.pos.data.model.PinLoginRequest
import com.kiwari.pos.data.model.RefreshRequest
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.POST

interface AuthApi {
    @POST("auth/login")
    suspend fun login(@Body request: LoginRequest): Response<AuthResponse>

    @POST("auth/pin-login")
    suspend fun pinLogin(@Body request: PinLoginRequest): Response<AuthResponse>

    @POST("auth/refresh")
    suspend fun refresh(@Body request: RefreshRequest): Response<AuthResponse>
}
