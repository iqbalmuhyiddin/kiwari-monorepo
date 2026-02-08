package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.MenuApi
import com.kiwari.pos.data.model.Category
import com.kiwari.pos.data.model.Modifier
import com.kiwari.pos.data.model.ModifierGroup
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.Variant
import com.kiwari.pos.data.model.VariantGroup
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class MenuRepository @Inject constructor(
    private val menuApi: MenuApi,
    private val tokenRepository: TokenRepository,
    private val gson: Gson
) {
    suspend fun getCategories(): Result<List<Category>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuApi.getCategories(outletId) }
    }

    suspend fun getProducts(): Result<List<Product>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuApi.getProducts(outletId) }
    }

    suspend fun getVariantGroups(productId: String): Result<List<VariantGroup>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuApi.getVariantGroups(outletId, productId) }
    }

    suspend fun getVariants(productId: String, variantGroupId: String): Result<List<Variant>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuApi.getVariants(outletId, productId, variantGroupId) }
    }

    suspend fun getModifierGroups(productId: String): Result<List<ModifierGroup>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuApi.getModifierGroups(outletId, productId) }
    }

    suspend fun getModifiers(productId: String, modifierGroupId: String): Result<List<Modifier>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuApi.getModifiers(outletId, productId, modifierGroupId) }
    }
}
