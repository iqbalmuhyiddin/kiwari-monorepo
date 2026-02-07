package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.Category
import com.kiwari.pos.data.model.Modifier
import com.kiwari.pos.data.model.ModifierGroup
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.data.model.Variant
import com.kiwari.pos.data.model.VariantGroup
import retrofit2.Response
import retrofit2.http.GET
import retrofit2.http.Path

interface MenuApi {
    @GET("outlets/{outletId}/categories")
    suspend fun getCategories(
        @Path("outletId") outletId: String
    ): Response<List<Category>>

    @GET("outlets/{outletId}/products")
    suspend fun getProducts(
        @Path("outletId") outletId: String
    ): Response<List<Product>>

    @GET("outlets/{outletId}/products/{productId}/variant-groups")
    suspend fun getVariantGroups(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String
    ): Response<List<VariantGroup>>

    @GET("outlets/{outletId}/products/{productId}/variant-groups/{variantGroupId}/variants")
    suspend fun getVariants(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("variantGroupId") variantGroupId: String
    ): Response<List<Variant>>

    @GET("outlets/{outletId}/products/{productId}/modifier-groups")
    suspend fun getModifierGroups(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String
    ): Response<List<ModifierGroup>>

    @GET("outlets/{outletId}/products/{productId}/modifier-groups/{modifierGroupId}/modifiers")
    suspend fun getModifiers(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("modifierGroupId") modifierGroupId: String
    ): Response<List<Modifier>>
}
