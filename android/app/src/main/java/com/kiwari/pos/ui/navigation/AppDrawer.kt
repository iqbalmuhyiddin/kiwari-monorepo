package com.kiwari.pos.ui.navigation

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.NavigationDrawerItem
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.kiwari.pos.data.model.UserRole
import com.kiwari.pos.util.DrawerFeature
import com.kiwari.pos.util.isFeatureVisible

data class DrawerItem(
    val feature: DrawerFeature,
    val label: String,
    val icon: String
)

private val allItems = listOf(
    DrawerItem(DrawerFeature.PESANAN, "Pesanan", "\uD83D\uDCCB"),
    DrawerItem(DrawerFeature.LAPORAN, "Laporan", "\uD83D\uDCCA"),
    DrawerItem(DrawerFeature.MENU_ADMIN, "Kelola Menu", "\uD83C\uDF7D\uFE0F"),
    DrawerItem(DrawerFeature.PELANGGAN, "Pelanggan", "\uD83D\uDC65"),
    DrawerItem(DrawerFeature.PENGGUNA, "Pengguna", "\uD83D\uDC64"),
    DrawerItem(DrawerFeature.PRINTER, "Printer", "\uD83D\uDDA8\uFE0F")
)

@Composable
fun AppDrawerContent(
    userName: String,
    userRole: UserRole,
    outletName: String,
    onItemClick: (DrawerFeature) -> Unit,
    onLogout: () -> Unit
) {
    ModalDrawerSheet(modifier = Modifier.width(280.dp)) {
        Column(modifier = Modifier.fillMaxHeight()) {
            // Header
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(MaterialTheme.colorScheme.primary)
                    .padding(horizontal = 16.dp, vertical = 24.dp)
            ) {
                Text(
                    text = "Kiwari POS",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Bold,
                    color = Color.White
                )
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = userName,
                    style = MaterialTheme.typography.bodyMedium,
                    color = Color.White.copy(alpha = 0.9f)
                )
                Text(
                    text = userRole.name,
                    style = MaterialTheme.typography.bodySmall,
                    color = Color.White.copy(alpha = 0.7f)
                )
            }

            Spacer(modifier = Modifier.height(8.dp))

            // Navigation items filtered by role
            val visibleItems = allItems.filter { isFeatureVisible(it.feature, userRole) }
            visibleItems.forEach { item ->
                NavigationDrawerItem(
                    label = {
                        Text(text = "${item.icon}  ${item.label}")
                    },
                    selected = false,
                    onClick = { onItemClick(item.feature) },
                    modifier = Modifier.padding(horizontal = 12.dp)
                )
            }

            Spacer(modifier = Modifier.weight(1f))

            // Logout button
            HorizontalDivider(modifier = Modifier.padding(horizontal = 16.dp))
            TextButton(
                onClick = onLogout,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 8.dp)
            ) {
                Text(
                    text = "Logout",
                    color = MaterialTheme.colorScheme.error,
                    fontWeight = FontWeight.Medium
                )
            }
            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}
