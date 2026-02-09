package com.kiwari.pos.ui.menu

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.ui.menu.components.CartBottomBar
import com.kiwari.pos.ui.menu.components.CategoryChips
import com.kiwari.pos.ui.menu.components.ProductListItem
import com.kiwari.pos.ui.menu.components.QuickEditPopup

@Composable
fun MenuScreen(
    viewModel: MenuViewModel = hiltViewModel(),
    onNavigateToCart: () -> Unit = {},
    onNavigateToCustomization: (productId: String) -> Unit = {},
    onNavigateToSettings: () -> Unit = {},
    onNavigateToOrderList: () -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()
    var showSearch by remember { mutableStateOf(false) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // Top bar with title, search, and settings
            MenuTopBar(
                showSearch = showSearch,
                searchQuery = uiState.searchQuery,
                onSearchQueryChanged = viewModel::onSearchQueryChanged,
                onToggleSearch = {
                    showSearch = !showSearch
                    if (!showSearch) viewModel.onSearchQueryChanged("")
                },
                onSettingsClick = onNavigateToSettings,
                onOrderListClick = onNavigateToOrderList
            )

            when {
                uiState.isLoading -> {
                    Box(
                        modifier = Modifier
                            .weight(1f)
                            .fillMaxWidth(),
                        contentAlignment = Alignment.Center
                    ) {
                        CircularProgressIndicator(
                            color = MaterialTheme.colorScheme.primary
                        )
                    }
                }

                uiState.errorMessage != null -> {
                    Box(
                        modifier = Modifier
                            .weight(1f)
                            .fillMaxWidth(),
                        contentAlignment = Alignment.Center
                    ) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text(
                                text = uiState.errorMessage ?: "Terjadi kesalahan",
                                style = MaterialTheme.typography.bodyLarge,
                                color = MaterialTheme.colorScheme.error
                            )
                            Spacer(modifier = Modifier.height(16.dp))
                            TextButton(onClick = viewModel::retry) {
                                Text("Coba lagi")
                            }
                        }
                    }
                }

                else -> {
                    // Category chips
                    CategoryChips(
                        categories = uiState.categories,
                        selectedCategoryId = uiState.selectedCategoryId,
                        onCategorySelected = viewModel::onCategorySelected
                    )

                    // Product list
                    val filteredProducts = uiState.filteredProducts
                    if (filteredProducts.isEmpty()) {
                        Box(
                            modifier = Modifier
                                .weight(1f)
                                .fillMaxWidth(),
                            contentAlignment = Alignment.Center
                        ) {
                            Text(
                                text = "Tidak ada produk ditemukan",
                                style = MaterialTheme.typography.bodyLarge,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    } else {
                        LazyColumn(
                            modifier = Modifier.weight(1f)
                        ) {
                            items(
                                items = filteredProducts,
                                key = { it.id }
                            ) { product ->
                                val qtyInCart = uiState.cartItems
                                    .filter { it.product.id == product.id }
                                    .sumOf { it.quantity }
                                val hasRequired =
                                    uiState.productVariantMap[product.id] == true

                                ProductListItem(
                                    product = product,
                                    quantityInCart = qtyInCart,
                                    hasRequiredVariants = hasRequired,
                                    onTap = {
                                        val added = viewModel.onProductTapped(product)
                                        if (!added) {
                                            onNavigateToCustomization(product.id)
                                        }
                                    },
                                    onLongPress = {
                                        viewModel.onProductLongPressed(product)
                                    }
                                )
                            }
                        }
                    }
                }
            }

            // Bottom bar — only show when cart has items, outside the when block
            // but still inside the Column so it stays at the bottom
        }

        // Cart bottom bar (overlays at bottom)
        if (uiState.cartTotalItems > 0 && !uiState.isLoading && uiState.errorMessage == null) {
            CartBottomBar(
                itemCount = uiState.cartTotalItems,
                totalPrice = uiState.cartTotalPrice,
                onContinue = onNavigateToCart,
                modifier = Modifier.align(Alignment.BottomCenter)
            )
        }

        // Quick edit popup
        uiState.quickEditCartItem?.let { cartItem ->
            QuickEditPopup(
                cartItem = cartItem,
                onDismiss = viewModel::onQuickEditDismissed,
                onUpdateQuantity = { qty ->
                    viewModel.onQuickEditUpdateQuantity(cartItem.id, qty)
                },
                onUpdateNotes = { notes ->
                    viewModel.onQuickEditUpdateNotes(cartItem.id, notes)
                },
                onRemove = {
                    viewModel.onQuickEditRemove(cartItem.id)
                }
            )
        }
    }
}

@Composable
private fun MenuTopBar(
    showSearch: Boolean,
    searchQuery: String,
    onSearchQueryChanged: (String) -> Unit,
    onToggleSearch: () -> Unit,
    onSettingsClick: () -> Unit = {},
    onOrderListClick: () -> Unit = {}
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(MaterialTheme.colorScheme.surface)
            .padding(horizontal = 16.dp, vertical = 8.dp)
    ) {
        if (showSearch) {
            OutlinedTextField(
                value = searchQuery,
                onValueChange = onSearchQueryChanged,
                modifier = Modifier.fillMaxWidth(),
                placeholder = { Text("Cari produk...") },
                singleLine = true,
                leadingIcon = {
                    Icon(
                        imageVector = Icons.Default.Search,
                        contentDescription = "Cari"
                    )
                },
                trailingIcon = {
                    IconButton(onClick = onToggleSearch) {
                        Icon(
                            imageVector = Icons.Default.Close,
                            contentDescription = "Tutup pencarian"
                        )
                    }
                }
            )
        } else {
            Box(
                modifier = Modifier.fillMaxWidth(),
                contentAlignment = Alignment.CenterStart
            ) {
                Text(
                    text = "Menu",
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Row(modifier = Modifier.align(Alignment.CenterEnd)) {
                    IconButton(onClick = onToggleSearch) {
                        Icon(
                            imageVector = Icons.Default.Search,
                            contentDescription = "Cari produk"
                        )
                    }
                    IconButton(onClick = onOrderListClick) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Default.List,
                            contentDescription = "Pesanan aktif"
                        )
                    }
                    IconButton(onClick = onSettingsClick) {
                        Icon(
                            imageVector = Icons.Default.Settings,
                            contentDescription = "Pengaturan printer"
                        )
                    }
                }
            }
        }
    }
}
