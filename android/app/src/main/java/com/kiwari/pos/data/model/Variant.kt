package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class Variant(
    @SerializedName("id")
    val id: String,

    @SerializedName("variant_group_id")
    val variantGroupId: String,

    @SerializedName("name")
    val name: String,

    @SerializedName("price_adjustment")
    val priceAdjustment: String,

    @SerializedName("is_active")
    val isActive: Boolean,

    @SerializedName("sort_order")
    val sortOrder: Int
)
