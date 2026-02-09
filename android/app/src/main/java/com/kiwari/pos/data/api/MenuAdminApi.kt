package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.Category
import com.kiwari.pos.data.model.ComboItem
import com.kiwari.pos.data.model.CreateCategoryRequest
import com.kiwari.pos.data.model.CreateComboItemRequest
import com.kiwari.pos.data.model.CreateModifierGroupRequest
import com.kiwari.pos.data.model.CreateModifierRequest
import com.kiwari.pos.data.model.CreateProductRequest
import com.kiwari.pos.data.model.CreateVariantGroupRequest
import com.kiwari.pos.data.model.CreateVariantRequest
import com.kiwari.pos.data.model.Modifier
import com.kiwari.pos.data.model.ModifierGroup
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.data.model.UpdateCategoryRequest
import com.kiwari.pos.data.model.UpdateModifierGroupRequest
import com.kiwari.pos.data.model.UpdateModifierRequest
import com.kiwari.pos.data.model.UpdateProductRequest
import com.kiwari.pos.data.model.UpdateVariantGroupRequest
import com.kiwari.pos.data.model.UpdateVariantRequest
import com.kiwari.pos.data.model.Variant
import com.kiwari.pos.data.model.VariantGroup
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path

interface MenuAdminApi {
    // Categories
    @POST("outlets/{outletId}/categories")
    suspend fun createCategory(
        @Path("outletId") outletId: String,
        @Body request: CreateCategoryRequest
    ): Response<Category>

    @PUT("outlets/{outletId}/categories/{categoryId}")
    suspend fun updateCategory(
        @Path("outletId") outletId: String,
        @Path("categoryId") categoryId: String,
        @Body request: UpdateCategoryRequest
    ): Response<Category>

    @DELETE("outlets/{outletId}/categories/{categoryId}")
    suspend fun deleteCategory(
        @Path("outletId") outletId: String,
        @Path("categoryId") categoryId: String
    ): Response<Void>

    // Products
    @POST("outlets/{outletId}/products")
    suspend fun createProduct(
        @Path("outletId") outletId: String,
        @Body request: CreateProductRequest
    ): Response<Product>

    @PUT("outlets/{outletId}/products/{productId}")
    suspend fun updateProduct(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Body request: UpdateProductRequest
    ): Response<Product>

    @DELETE("outlets/{outletId}/products/{productId}")
    suspend fun deleteProduct(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String
    ): Response<Void>

    // Variant Groups
    @POST("outlets/{outletId}/products/{productId}/variant-groups")
    suspend fun createVariantGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Body request: CreateVariantGroupRequest
    ): Response<VariantGroup>

    @PUT("outlets/{outletId}/products/{productId}/variant-groups/{groupId}")
    suspend fun updateVariantGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Body request: UpdateVariantGroupRequest
    ): Response<VariantGroup>

    @DELETE("outlets/{outletId}/products/{productId}/variant-groups/{groupId}")
    suspend fun deleteVariantGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String
    ): Response<Void>

    // Variants
    @POST("outlets/{outletId}/products/{productId}/variant-groups/{groupId}/variants")
    suspend fun createVariant(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Body request: CreateVariantRequest
    ): Response<Variant>

    @PUT("outlets/{outletId}/products/{productId}/variant-groups/{groupId}/variants/{variantId}")
    suspend fun updateVariant(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Path("variantId") variantId: String,
        @Body request: UpdateVariantRequest
    ): Response<Variant>

    @DELETE("outlets/{outletId}/products/{productId}/variant-groups/{groupId}/variants/{variantId}")
    suspend fun deleteVariant(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Path("variantId") variantId: String
    ): Response<Void>

    // Modifier Groups
    @POST("outlets/{outletId}/products/{productId}/modifier-groups")
    suspend fun createModifierGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Body request: CreateModifierGroupRequest
    ): Response<ModifierGroup>

    @PUT("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}")
    suspend fun updateModifierGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Body request: UpdateModifierGroupRequest
    ): Response<ModifierGroup>

    @DELETE("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}")
    suspend fun deleteModifierGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String
    ): Response<Void>

    // Modifiers
    @POST("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}/modifiers")
    suspend fun createModifier(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Body request: CreateModifierRequest
    ): Response<Modifier>

    @PUT("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}/modifiers/{modifierId}")
    suspend fun updateModifier(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Path("modifierId") modifierId: String,
        @Body request: UpdateModifierRequest
    ): Response<Modifier>

    @DELETE("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}/modifiers/{modifierId}")
    suspend fun deleteModifier(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Path("modifierId") modifierId: String
    ): Response<Void>

    // Combo Items
    @GET("outlets/{outletId}/products/{productId}/combo-items")
    suspend fun getComboItems(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String
    ): Response<List<ComboItem>>

    @POST("outlets/{outletId}/products/{productId}/combo-items")
    suspend fun addComboItem(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Body request: CreateComboItemRequest
    ): Response<ComboItem>

    @DELETE("outlets/{outletId}/products/{productId}/combo-items/{comboItemId}")
    suspend fun removeComboItem(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("comboItemId") comboItemId: String
    ): Response<Void>
}
