package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class Category(
    @SerializedName("id")
    val id: String,

    @SerializedName("outlet_id")
    val outletId: String,

    @SerializedName("name")
    val name: String,

    @SerializedName("description")
    val description: String?,

    @SerializedName("sort_order")
    val sortOrder: Int,

    @SerializedName("is_active")
    val isActive: Boolean,

    @SerializedName("created_at")
    val createdAt: String
)
