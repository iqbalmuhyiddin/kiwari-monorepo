package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class StaffMember(
    @SerializedName("id") val id: String,
    @SerializedName("outlet_id") val outletId: String,
    @SerializedName("email") val email: String,
    @SerializedName("full_name") val fullName: String,
    @SerializedName("role") val role: String,
    @SerializedName("pin") val pin: String?,
    @SerializedName("is_active") val isActive: Boolean,
    @SerializedName("created_at") val createdAt: String,
    @SerializedName("updated_at") val updatedAt: String
)

data class CreateUserRequest(
    @SerializedName("email") val email: String,
    @SerializedName("password") val password: String,
    @SerializedName("full_name") val fullName: String,
    @SerializedName("role") val role: String,
    @SerializedName("pin") val pin: String? = null
)

data class UpdateUserRequest(
    @SerializedName("email") val email: String,
    @SerializedName("full_name") val fullName: String,
    @SerializedName("role") val role: String,
    @SerializedName("pin") val pin: String? = null
)
