package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.MenuAdminApi
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
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.UpdateCategoryRequest
import com.kiwari.pos.data.model.UpdateModifierGroupRequest
import com.kiwari.pos.data.model.UpdateModifierRequest
import com.kiwari.pos.data.model.UpdateProductRequest
import com.kiwari.pos.data.model.UpdateVariantGroupRequest
import com.kiwari.pos.data.model.UpdateVariantRequest
import com.kiwari.pos.data.model.Variant
import com.kiwari.pos.data.model.VariantGroup
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class MenuAdminRepository @Inject constructor(
    private val menuAdminApi: MenuAdminApi,
    private val tokenRepository: TokenRepository,
    private val gson: Gson
) {
    // ── Categories ──

    suspend fun createCategory(name: String, description: String, sortOrder: Int): Result<Category> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) {
            menuAdminApi.createCategory(outletId, CreateCategoryRequest(name, description, sortOrder))
        }
    }

    suspend fun updateCategory(categoryId: String, name: String, description: String, sortOrder: Int): Result<Category> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) {
            menuAdminApi.updateCategory(outletId, categoryId, UpdateCategoryRequest(name, description, sortOrder))
        }
    }

    suspend fun deleteCategory(categoryId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { menuAdminApi.deleteCategory(outletId, categoryId) }
    }

    // ── Products ──

    suspend fun createProduct(request: CreateProductRequest): Result<Product> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.createProduct(outletId, request) }
    }

    suspend fun updateProduct(productId: String, request: UpdateProductRequest): Result<Product> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.updateProduct(outletId, productId, request) }
    }

    suspend fun deleteProduct(productId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { menuAdminApi.deleteProduct(outletId, productId) }
    }

    // ── Variant Groups ──

    suspend fun createVariantGroup(productId: String, request: CreateVariantGroupRequest): Result<VariantGroup> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.createVariantGroup(outletId, productId, request) }
    }

    suspend fun updateVariantGroup(productId: String, groupId: String, request: UpdateVariantGroupRequest): Result<VariantGroup> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.updateVariantGroup(outletId, productId, groupId, request) }
    }

    suspend fun deleteVariantGroup(productId: String, groupId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { menuAdminApi.deleteVariantGroup(outletId, productId, groupId) }
    }

    // ── Variants ──

    suspend fun createVariant(productId: String, groupId: String, request: CreateVariantRequest): Result<Variant> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.createVariant(outletId, productId, groupId, request) }
    }

    suspend fun updateVariant(productId: String, groupId: String, variantId: String, request: UpdateVariantRequest): Result<Variant> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.updateVariant(outletId, productId, groupId, variantId, request) }
    }

    suspend fun deleteVariant(productId: String, groupId: String, variantId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { menuAdminApi.deleteVariant(outletId, productId, groupId, variantId) }
    }

    // ── Modifier Groups ──

    suspend fun createModifierGroup(productId: String, request: CreateModifierGroupRequest): Result<ModifierGroup> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.createModifierGroup(outletId, productId, request) }
    }

    suspend fun updateModifierGroup(productId: String, groupId: String, request: UpdateModifierGroupRequest): Result<ModifierGroup> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.updateModifierGroup(outletId, productId, groupId, request) }
    }

    suspend fun deleteModifierGroup(productId: String, groupId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { menuAdminApi.deleteModifierGroup(outletId, productId, groupId) }
    }

    // ── Modifiers ──

    suspend fun createModifier(productId: String, groupId: String, request: CreateModifierRequest): Result<Modifier> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.createModifier(outletId, productId, groupId, request) }
    }

    suspend fun updateModifier(productId: String, groupId: String, modifierId: String, request: UpdateModifierRequest): Result<Modifier> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.updateModifier(outletId, productId, groupId, modifierId, request) }
    }

    suspend fun deleteModifier(productId: String, groupId: String, modifierId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { menuAdminApi.deleteModifier(outletId, productId, groupId, modifierId) }
    }

    // ── Combo Items ──

    suspend fun getComboItems(productId: String): Result<List<ComboItem>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.getComboItems(outletId, productId) }
    }

    suspend fun addComboItem(productId: String, request: CreateComboItemRequest): Result<ComboItem> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { menuAdminApi.addComboItem(outletId, productId, request) }
    }

    suspend fun removeComboItem(productId: String, comboItemId: String): Result<Unit> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCallNoBody(gson) { menuAdminApi.removeComboItem(outletId, productId, comboItemId) }
    }
}
