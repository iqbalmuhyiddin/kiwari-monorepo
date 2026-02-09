package com.kiwari.pos.ui.menuadmin

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedCard
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.util.formatPrice
import java.math.BigDecimal

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ProductListScreen(
    viewModel: ProductListViewModel = hiltViewModel(),
    onProductClick: (productId: String) -> Unit = {},
    onCreateProduct: () -> Unit = {},
    onNavigateBack: () -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // Top bar
            ProductListTopBar(
                categoryName = uiState.categoryName,
                onNavigateBack = onNavigateBack,
                onAdd = onCreateProduct
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

                uiState.errorMessage != null && !uiState.isRefreshing -> {
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
                    PullToRefreshBox(
                        isRefreshing = uiState.isRefreshing,
                        onRefresh = { viewModel.refresh() },
                        modifier = Modifier.weight(1f)
                    ) {
                        if (uiState.products.isEmpty()) {
                            Box(
                                modifier = Modifier.fillMaxSize(),
                                contentAlignment = Alignment.Center
                            ) {
                                Text(
                                    text = "Belum ada produk",
                                    style = MaterialTheme.typography.bodyLarge,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        } else {
                            LazyColumn(
                                modifier = Modifier.fillMaxSize(),
                                verticalArrangement = Arrangement.spacedBy(12.dp),
                                contentPadding = PaddingValues(
                                    horizontal = 16.dp,
                                    vertical = 8.dp
                                )
                            ) {
                                items(
                                    items = uiState.products,
                                    key = { it.id }
                                ) { product ->
                                    ProductCard(
                                        product = product,
                                        variantGroupCount = uiState.variantGroupCounts[product.id] ?: 0,
                                        modifierGroupCount = uiState.modifierGroupCounts[product.id] ?: 0,
                                        onClick = { onProductClick(product.id) },
                                        onDelete = { viewModel.showDeleteDialog(product) }
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // Delete confirmation dialog
    uiState.deletingProduct?.let { product ->
        DeleteProductDialog(
            productName = product.name,
            isDeleting = uiState.isDeleting,
            deleteError = uiState.deleteError,
            onDismiss = viewModel::dismissDeleteDialog,
            onConfirm = viewModel::deleteProduct
        )
    }
}

@Composable
private fun ProductListTopBar(
    categoryName: String,
    onNavigateBack: () -> Unit,
    onAdd: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(MaterialTheme.colorScheme.surface)
            .padding(horizontal = 4.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        IconButton(onClick = onNavigateBack) {
            Icon(
                imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                contentDescription = "Kembali"
            )
        }
        Text(
            text = categoryName,
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface,
            modifier = Modifier.weight(1f)
        )
        IconButton(onClick = onAdd) {
            Icon(
                imageVector = Icons.Default.Add,
                contentDescription = "Tambah Produk"
            )
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ProductCard(
    product: Product,
    variantGroupCount: Int,
    modifierGroupCount: Int,
    onClick: () -> Unit,
    onDelete: () -> Unit
) {
    OutlinedCard(
        onClick = onClick,
        modifier = Modifier
            .fillMaxWidth()
            .alpha(if (product.isActive) 1f else 0.6f),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 16.dp, top = 12.dp, bottom = 12.dp, end = 4.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            // Product info
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = product.name,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    if (!product.isActive) {
                        Spacer(modifier = Modifier.width(8.dp))
                        Text(
                            text = "Nonaktif",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.error
                        )
                    }
                }

                Spacer(modifier = Modifier.height(4.dp))

                Text(
                    text = formatPrice(product.basePrice.toBigDecimalOrNull() ?: BigDecimal.ZERO),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )

                // Badges row
                val hasBadges = product.station != null || variantGroupCount > 0 || modifierGroupCount > 0
                if (hasBadges) {
                    Spacer(modifier = Modifier.height(6.dp))
                    FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        if (product.station != null) {
                            BadgeText(
                                text = product.station,
                                color = MaterialTheme.colorScheme.tertiary
                            )
                        }
                        if (variantGroupCount > 0) {
                            BadgeText(
                                text = "$variantGroupCount varian",
                                color = MaterialTheme.colorScheme.primary
                            )
                        }
                        if (modifierGroupCount > 0) {
                            BadgeText(
                                text = "$modifierGroupCount modifier",
                                color = MaterialTheme.colorScheme.secondary
                            )
                        }
                    }
                }
            }

            // Delete button
            IconButton(
                onClick = onDelete,
                modifier = Modifier.size(40.dp)
            ) {
                Icon(
                    imageVector = Icons.Default.Delete,
                    contentDescription = "Hapus",
                    tint = MaterialTheme.colorScheme.error,
                    modifier = Modifier.size(20.dp)
                )
            }
        }
    }
}

@Composable
private fun BadgeText(
    text: String,
    color: androidx.compose.ui.graphics.Color
) {
    Text(
        text = text,
        style = MaterialTheme.typography.labelSmall,
        color = color,
        modifier = Modifier
            .background(
                color = color.copy(alpha = 0.1f),
                shape = MaterialTheme.shapes.small
            )
            .padding(horizontal = 8.dp, vertical = 2.dp)
    )
}

@Composable
private fun DeleteProductDialog(
    productName: String,
    isDeleting: Boolean,
    deleteError: String?,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit
) {
    AlertDialog(
        onDismissRequest = { if (!isDeleting) onDismiss() },
        title = { Text("Hapus Produk") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Hapus produk \"$productName\"?")
                if (deleteError != null) {
                    Text(
                        text = deleteError,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = onConfirm,
                enabled = !isDeleting
            ) {
                if (isDeleting) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(16.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.error
                    )
                } else {
                    Text(
                        text = "Hapus",
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
        },
        dismissButton = {
            TextButton(
                onClick = onDismiss,
                enabled = !isDeleting
            ) {
                Text("Batal")
            }
        }
    )
}
