package com.kiwari.pos.data.repository

import com.kiwari.pos.data.model.Product
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Lightweight bridge to pass the selected product from MenuScreen to CustomizationScreen.
 * Avoids re-fetching product data that's already loaded.
 */
@Singleton
class SelectedProductRepository @Inject constructor() {

    @Volatile
    private var product: Product? = null

    fun set(product: Product) {
        this.product = product
    }

    fun get(): Product? = product

    fun clear() {
        product = null
    }
}
