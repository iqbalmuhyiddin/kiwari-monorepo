package com.kiwari.pos.util

import com.kiwari.pos.data.model.UserRole

enum class DrawerFeature {
    PESANAN,
    LAPORAN,
    MENU_ADMIN,
    PELANGGAN,
    PENGGUNA,
    PRINTER
}

fun isFeatureVisible(feature: DrawerFeature, role: UserRole): Boolean {
    return when (feature) {
        DrawerFeature.PESANAN -> role in listOf(UserRole.OWNER, UserRole.MANAGER, UserRole.CASHIER)
        DrawerFeature.LAPORAN -> role in listOf(UserRole.OWNER, UserRole.MANAGER)
        DrawerFeature.MENU_ADMIN -> role in listOf(UserRole.OWNER, UserRole.MANAGER)
        DrawerFeature.PELANGGAN -> role in listOf(UserRole.OWNER, UserRole.MANAGER)
        DrawerFeature.PENGGUNA -> role in listOf(UserRole.OWNER, UserRole.MANAGER)
        DrawerFeature.PRINTER -> role in listOf(UserRole.OWNER, UserRole.MANAGER, UserRole.CASHIER)
    }
}
