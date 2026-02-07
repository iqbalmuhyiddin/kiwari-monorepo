package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class Modifier(
    @SerializedName("id")
    val id: String,

    @SerializedName("modifier_group_id")
    val modifierGroupId: String,

    @SerializedName("name")
    val name: String,

    @SerializedName("price")
    val price: String,

    @SerializedName("is_active")
    val isActive: Boolean,

    @SerializedName("sort_order")
    val sortOrder: Int
)
