package com.kiwari.pos.di

import com.kiwari.pos.data.repository.TokenRepository
import okhttp3.Interceptor
import okhttp3.Response
import javax.inject.Inject
import javax.inject.Singleton

/**
 * AuthInterceptor adds Authorization header to all requests (except auth endpoints).
 */
@Singleton
class AuthInterceptor @Inject constructor(
    private val tokenRepository: TokenRepository
) : Interceptor {

    private val authExcludedPaths = listOf(
        "/auth/login",
        "/auth/pin-login",
        "/auth/refresh"
    )

    override fun intercept(chain: Interceptor.Chain): Response {
        val originalRequest = chain.request()
        val requestUrl = originalRequest.url.encodedPath

        // Skip auth header for authentication endpoints
        if (authExcludedPaths.any { requestUrl.endsWith(it) }) {
            return chain.proceed(originalRequest)
        }

        // Add access token to request
        val accessToken = tokenRepository.getAccessToken()
        val request = if (!accessToken.isNullOrEmpty()) {
            originalRequest.newBuilder()
                .header("Authorization", "Bearer $accessToken")
                .build()
        } else {
            originalRequest
        }

        // Execute request
        return chain.proceed(request)
    }
}
