package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

// Category CRUD
data class CreateCategoryRequest(
    @SerializedName("name") val name: String,
    @SerializedName("description") val description: String = "",
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateCategoryRequest(
    @SerializedName("name") val name: String,
    @SerializedName("description") val description: String = "",
    @SerializedName("sort_order") val sortOrder: Int
)

// Product CRUD
data class CreateProductRequest(
    @SerializedName("category_id") val categoryId: String,
    @SerializedName("name") val name: String,
    @SerializedName("description") val description: String = "",
    @SerializedName("base_price") val basePrice: String,
    @SerializedName("image_url") val imageUrl: String = "",
    @SerializedName("station") val station: String = "",
    @SerializedName("preparation_time") val preparationTime: Int? = null,
    @SerializedName("is_combo") val isCombo: Boolean = false
)

data class UpdateProductRequest(
    @SerializedName("category_id") val categoryId: String,
    @SerializedName("name") val name: String,
    @SerializedName("description") val description: String = "",
    @SerializedName("base_price") val basePrice: String,
    @SerializedName("image_url") val imageUrl: String = "",
    @SerializedName("station") val station: String = "",
    @SerializedName("preparation_time") val preparationTime: Int? = null,
    @SerializedName("is_combo") val isCombo: Boolean = false
)

// Variant Group CRUD
data class CreateVariantGroupRequest(
    @SerializedName("name") val name: String,
    @SerializedName("is_required") val isRequired: Boolean,
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateVariantGroupRequest(
    @SerializedName("name") val name: String,
    @SerializedName("is_required") val isRequired: Boolean,
    @SerializedName("sort_order") val sortOrder: Int
)

// Variant CRUD
data class CreateVariantRequest(
    @SerializedName("name") val name: String,
    @SerializedName("price_adjustment") val priceAdjustment: String,
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateVariantRequest(
    @SerializedName("name") val name: String,
    @SerializedName("price_adjustment") val priceAdjustment: String,
    @SerializedName("sort_order") val sortOrder: Int
)

// Modifier Group CRUD
data class CreateModifierGroupRequest(
    @SerializedName("name") val name: String,
    @SerializedName("min_select") val minSelect: Int,
    @SerializedName("max_select") val maxSelect: Int,
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateModifierGroupRequest(
    @SerializedName("name") val name: String,
    @SerializedName("min_select") val minSelect: Int,
    @SerializedName("max_select") val maxSelect: Int,
    @SerializedName("sort_order") val sortOrder: Int
)

// Modifier CRUD
data class CreateModifierRequest(
    @SerializedName("name") val name: String,
    @SerializedName("price") val price: String,
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateModifierRequest(
    @SerializedName("name") val name: String,
    @SerializedName("price") val price: String,
    @SerializedName("sort_order") val sortOrder: Int
)

// Combo Item
data class ComboItem(
    @SerializedName("id") val id: String,
    @SerializedName("combo_id") val comboId: String,
    @SerializedName("product_id") val productId: String,
    @SerializedName("quantity") val quantity: Int,
    @SerializedName("sort_order") val sortOrder: Int
)

data class CreateComboItemRequest(
    @SerializedName("product_id") val productId: String,
    @SerializedName("quantity") val quantity: Int,
    @SerializedName("sort_order") val sortOrder: Int
)
