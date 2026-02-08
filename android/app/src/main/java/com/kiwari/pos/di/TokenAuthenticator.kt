package com.kiwari.pos.di

import com.kiwari.pos.data.api.AuthApi
import com.kiwari.pos.data.model.RefreshRequest
import com.kiwari.pos.data.repository.TokenRepository
import kotlinx.coroutines.runBlocking
import okhttp3.Authenticator
import okhttp3.Request
import okhttp3.Response
import okhttp3.Route
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Singleton

/**
 * TokenAuthenticator handles automatic token refresh on 401 responses.
 *
 * When a request receives a 401 Unauthorized response:
 * 1. Attempts to refresh the token using the refresh token
 * 2. If successful, retries the original request with the new access token
 * 3. If refresh fails, clears all tokens and propagates the 401 to force re-login
 *
 * Thread-safety: Uses synchronized block to prevent concurrent refresh attempts.
 */
@Singleton
class TokenAuthenticator @Inject constructor(
    private val tokenRepository: TokenRepository,
    @Named("auth") private val authApi: AuthApi
) : Authenticator {

    // Mutex to prevent concurrent refresh attempts
    private val refreshLock = Any()

    override fun authenticate(route: Route?, response: Response): Request? {
        // Check if this is a retry attempt - prevent infinite loops
        if (responseCount(response) >= 2) {
            // Already retried once, give up
            return null
        }

        // Get the current refresh token
        val refreshToken = tokenRepository.getRefreshToken()
        if (refreshToken.isNullOrEmpty()) {
            // No refresh token, can't proceed
            tokenRepository.clearTokens()
            return null
        }

        synchronized(refreshLock) {
            // Check if another thread already refreshed while we were waiting
            val currentAccessToken = tokenRepository.getAccessToken()
            val originalAccessToken = response.request.header("Authorization")?.removePrefix("Bearer ")

            if (currentAccessToken != originalAccessToken && !currentAccessToken.isNullOrEmpty()) {
                // Token was already refreshed by another thread, retry with new token
                return response.request.newBuilder()
                    .header("Authorization", "Bearer $currentAccessToken")
                    .build()
            }

            // Perform the refresh
            return runBlocking {
                try {
                    val refreshResponse = authApi.refresh(RefreshRequest(refreshToken))

                    if (refreshResponse.isSuccessful) {
                        val authResponse = refreshResponse.body()
                        if (authResponse != null) {
                            // Save new tokens
                            tokenRepository.saveTokens(
                                accessToken = authResponse.accessToken,
                                refreshToken = authResponse.refreshToken,
                                userId = authResponse.user.id,
                                outletId = authResponse.user.outletId,
                                userRole = authResponse.user.role.name,
                                userName = authResponse.user.fullName
                            )

                            // Retry request with new access token
                            response.request.newBuilder()
                                .header("Authorization", "Bearer ${authResponse.accessToken}")
                                .build()
                        } else {
                            // Empty response, clear tokens
                            tokenRepository.clearTokens()
                            null
                        }
                    } else {
                        // Refresh failed, clear tokens to force re-login
                        tokenRepository.clearTokens()
                        null
                    }
                } catch (e: Exception) {
                    // Refresh failed with exception, clear tokens
                    tokenRepository.clearTokens()
                    null
                }
            }
        }
    }

    /**
     * Counts how many times this response has been retried.
     * Returns 1 for first 401, 2 for second 401, etc.
     */
    private fun responseCount(response: Response): Int {
        var count = 1
        var priorResponse = response.priorResponse
        while (priorResponse != null) {
            count++
            priorResponse = priorResponse.priorResponse
        }
        return count
    }
}
