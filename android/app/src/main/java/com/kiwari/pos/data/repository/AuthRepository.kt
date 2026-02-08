package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.AuthApi
import com.kiwari.pos.data.model.AuthResponse
import com.kiwari.pos.data.model.LoginRequest
import com.kiwari.pos.data.model.PinLoginRequest
import com.kiwari.pos.data.model.RefreshRequest
import com.kiwari.pos.data.model.Result
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Singleton

@Singleton
class AuthRepository @Inject constructor(
    @Named("auth") private val authApi: AuthApi,
    private val tokenRepository: TokenRepository,
    private val gson: Gson
) {
    suspend fun login(email: String, password: String): Result<AuthResponse> {
        return safeApiCall(gson) { authApi.login(LoginRequest(email, password)) }.also { result ->
            if (result is Result.Success) {
                saveTokensFromResponse(result.data)
            }
        }
    }

    suspend fun pinLogin(outletId: String, pin: String): Result<AuthResponse> {
        return safeApiCall(gson) { authApi.pinLogin(PinLoginRequest(outletId, pin)) }.also { result ->
            if (result is Result.Success) {
                saveTokensFromResponse(result.data)
            }
        }
    }

    suspend fun refreshToken(): Result<AuthResponse> {
        val refreshToken = tokenRepository.getRefreshToken()
        if (refreshToken.isNullOrEmpty()) {
            return Result.Error("No refresh token available")
        }

        return safeApiCall(gson) { authApi.refresh(RefreshRequest(refreshToken)) }.also { result ->
            when (result) {
                is Result.Success -> saveTokensFromResponse(result.data)
                is Result.Error -> tokenRepository.clearTokens()
            }
        }
    }

    fun logout() {
        tokenRepository.clearTokens()
    }

    private fun saveTokensFromResponse(authResponse: AuthResponse) {
        tokenRepository.saveTokens(
            accessToken = authResponse.accessToken,
            refreshToken = authResponse.refreshToken,
            userId = authResponse.user.id,
            outletId = authResponse.user.outletId,
            userRole = authResponse.user.role.name,
            userName = authResponse.user.fullName
        )
    }

}
