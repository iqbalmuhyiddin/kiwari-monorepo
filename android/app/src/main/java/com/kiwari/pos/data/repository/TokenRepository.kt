package com.kiwari.pos.data.repository

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class TokenRepository @Inject constructor(
    @ApplicationContext private val context: Context
) {
    private val masterKeyAlias = MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC)

    private val encryptedPrefs: SharedPreferences = EncryptedSharedPreferences.create(
        "kiwari_pos_secure_auth",
        masterKeyAlias,
        context,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
    )

    companion object {
        private const val ACCESS_TOKEN = "access_token"
        private const val REFRESH_TOKEN = "refresh_token"
        private const val USER_ID = "user_id"
        private const val OUTLET_ID = "outlet_id"
        private const val USER_ROLE = "user_role"
        private const val USER_NAME = "user_name"
    }

    // Reactive login state using StateFlow
    private val _isLoggedIn = MutableStateFlow(encryptedPrefs.getString(ACCESS_TOKEN, null)?.isNotEmpty() == true)
    val isLoggedIn: Flow<Boolean> = _isLoggedIn.asStateFlow()

    fun saveTokens(
        accessToken: String,
        refreshToken: String,
        userId: String,
        outletId: String,
        userRole: String,
        userName: String
    ) {
        encryptedPrefs.edit().apply {
            putString(ACCESS_TOKEN, accessToken)
            putString(REFRESH_TOKEN, refreshToken)
            putString(USER_ID, userId)
            putString(OUTLET_ID, outletId)
            putString(USER_ROLE, userRole)
            putString(USER_NAME, userName)
            apply()
        }
        _isLoggedIn.value = true
    }

    fun getAccessToken(): String? {
        return encryptedPrefs.getString(ACCESS_TOKEN, null)
    }

    fun getRefreshToken(): String? {
        return encryptedPrefs.getString(REFRESH_TOKEN, null)
    }

    fun getUserId(): String? {
        return encryptedPrefs.getString(USER_ID, null)
    }

    fun getOutletId(): String? {
        return encryptedPrefs.getString(OUTLET_ID, null)
    }

    fun getUserRole(): String? {
        return encryptedPrefs.getString(USER_ROLE, null)
    }

    fun getUserName(): String? {
        return encryptedPrefs.getString(USER_NAME, null)
    }

    fun clearTokens() {
        encryptedPrefs.edit().clear().apply()
        _isLoggedIn.value = false
    }
}
