package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.UserApi
import com.kiwari.pos.data.model.CreateUserRequest
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.StaffMember
import com.kiwari.pos.data.model.UpdateUserRequest
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class UserRepository @Inject constructor(
    private val userApi: UserApi,
    private val tokenRepository: TokenRepository,
    private val gson: Gson
) {
    suspend fun listUsers(): Result<List<StaffMember>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { userApi.listUsers(outletId) }
    }

    suspend fun createUser(request: CreateUserRequest): Result<StaffMember> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { userApi.createUser(outletId, request) }
    }

    suspend fun updateUser(userId: String, request: UpdateUserRequest): Result<StaffMember> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { userApi.updateUser(outletId, userId, request) }
    }

    suspend fun deleteUser(userId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { userApi.deleteUser(outletId, userId) }
    }
}
