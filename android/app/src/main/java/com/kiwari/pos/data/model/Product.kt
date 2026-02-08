package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class Product(
    @SerializedName("id")
    val id: String,

    @SerializedName("outlet_id")
    val outletId: String,

    @SerializedName("category_id")
    val categoryId: String,

    @SerializedName("name")
    val name: String,

    @SerializedName("description")
    val description: String?,

    @SerializedName("base_price")
    val basePrice: String,

    @SerializedName("image_url")
    val imageUrl: String?,

    @SerializedName("station")
    val station: String?,

    @SerializedName("preparation_time")
    val preparationTime: Int?,

    @SerializedName("is_combo")
    val isCombo: Boolean,

    @SerializedName("is_active")
    val isActive: Boolean,

    @SerializedName("created_at")
    val createdAt: String,

    @SerializedName("updated_at")
    val updatedAt: String
)
