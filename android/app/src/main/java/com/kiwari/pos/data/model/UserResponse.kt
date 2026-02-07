package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class UserResponse(
    @SerializedName("id")
    val id: String,

    @SerializedName("outlet_id")
    val outletId: String,

    @SerializedName("full_name")
    val fullName: String,

    @SerializedName("email")
    val email: String,

    @SerializedName("role")
    val role: UserRole
)

enum class UserRole {
    @SerializedName("OWNER")
    OWNER,

    @SerializedName("MANAGER")
    MANAGER,

    @SerializedName("CASHIER")
    CASHIER,

    @SerializedName("KITCHEN")
    KITCHEN,

    UNKNOWN
}
