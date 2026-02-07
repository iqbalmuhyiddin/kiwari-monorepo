package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class VariantGroup(
    @SerializedName("id")
    val id: String,

    @SerializedName("product_id")
    val productId: String,

    @SerializedName("name")
    val name: String,

    @SerializedName("is_required")
    val isRequired: Boolean,

    @SerializedName("is_active")
    val isActive: Boolean,

    @SerializedName("sort_order")
    val sortOrder: Int
)
