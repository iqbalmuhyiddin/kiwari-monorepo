package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class ModifierGroup(
    @SerializedName("id")
    val id: String,

    @SerializedName("product_id")
    val productId: String,

    @SerializedName("name")
    val name: String,

    @SerializedName("min_select")
    val minSelect: Int,

    @SerializedName("max_select")
    val maxSelect: Int?,

    @SerializedName("is_active")
    val isActive: Boolean,

    @SerializedName("sort_order")
    val sortOrder: Int
)
