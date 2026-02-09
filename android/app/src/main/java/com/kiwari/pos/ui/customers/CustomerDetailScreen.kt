package com.kiwari.pos.ui.customers

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
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
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedCard
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.data.model.CustomerOrderResponse
import com.kiwari.pos.data.model.CustomerStatsResponse
import com.kiwari.pos.data.model.TopItemResponse
import com.kiwari.pos.ui.orders.OrderStatusBadge
import com.kiwari.pos.util.formatPrice
import androidx.compose.ui.graphics.Color
import java.math.BigDecimal
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

private val timestampFormatter: DateTimeFormatter = DateTimeFormatter.ofPattern("dd MMM yyyy, HH:mm")

private fun formatTimestamp(isoTimestamp: String): String {
    return try {
        val instant = Instant.parse(isoTimestamp)
        val localDateTime = instant.atZone(ZoneId.systemDefault()).toLocalDateTime()
        localDateTime.format(timestampFormatter)
    } catch (_: Exception) {
        isoTimestamp
    }
}

private fun formatOrderType(orderType: String): String {
    return when (orderType.uppercase()) {
        "DINE_IN" -> "Dine-in"
        "TAKEAWAY" -> "Takeaway"
        "CATERING" -> "Catering"
        "DELIVERY" -> "Delivery"
        else -> orderType
    }
}

@Composable
fun CustomerDetailScreen(
    viewModel: CustomerDetailViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit = {},
    onOrderClick: (orderId: String) -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()
    var deleteNavigated by remember { mutableStateOf(false) }

    LaunchedEffect(uiState.isDeleted) {
        if (uiState.isDeleted && !deleteNavigated) {
            deleteNavigated = true
            onNavigateBack()
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // Top bar
            DetailTopBar(
                customerName = uiState.customer?.name ?: "",
                onNavigateBack = onNavigateBack,
                onEdit = viewModel::showEditDialog,
                onDelete = viewModel::showDeleteDialog
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

                uiState.errorMessage != null && uiState.customer == null -> {
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
                            TextButton(onClick = viewModel::refresh) {
                                Text("Coba lagi")
                            }
                        }
                    }
                }

                uiState.customer != null -> {
                    val customer = uiState.customer ?: return@Column
                    val stats = uiState.stats
                    val topItems = stats?.topItems ?: emptyList()

                    LazyColumn(
                        modifier = Modifier
                            .weight(1f)
                            .fillMaxWidth(),
                        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        // Contact info section
                        item {
                            ContactInfoSection(customer = customer)
                        }

                        // KPI stats cards
                        if (stats != null) {
                            item {
                                StatsCardsRow(stats = stats)
                            }
                        }

                        // Menu Favorit section
                        if (topItems.isNotEmpty()) {
                            item {
                                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                            }
                            item {
                                Text(
                                    text = "Menu Favorit",
                                    style = MaterialTheme.typography.titleMedium,
                                    fontWeight = FontWeight.Bold,
                                    color = MaterialTheme.colorScheme.onSurface
                                )
                            }
                            itemsIndexed(
                                items = topItems,
                                key = { _, item -> item.productId }
                            ) { index, item ->
                                TopItemRow(rank = index + 1, item = item)
                            }
                        }

                        // Riwayat Pesanan section
                        val orders = uiState.orders
                        if (orders.isNotEmpty()) {
                            item {
                                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                            }
                            item {
                                Text(
                                    text = "Riwayat Pesanan",
                                    style = MaterialTheme.typography.titleMedium,
                                    fontWeight = FontWeight.Bold,
                                    color = MaterialTheme.colorScheme.onSurface
                                )
                            }
                            items(
                                items = orders,
                                key = { it.id }
                            ) { order ->
                                OrderHistoryCard(
                                    order = order,
                                    onClick = { onOrderClick(order.id) }
                                )
                            }
                        }

                        // Bottom spacing
                        item {
                            Spacer(modifier = Modifier.height(16.dp))
                        }
                    }
                }
            }
        }

        // Deleting overlay — blocks touch input
        if (uiState.isDeleting) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.3f))
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        onClick = {}
                    ),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator(
                    color = MaterialTheme.colorScheme.primary
                )
            }
        }
    }

    // Edit dialog
    val editCustomer = uiState.customer
    if (uiState.showEditDialog && editCustomer != null) {
        EditCustomerDialog(
            customer = editCustomer,
            isUpdating = uiState.isUpdating,
            updateError = uiState.updateError,
            onDismiss = viewModel::dismissEditDialog,
            onSave = viewModel::updateCustomer
        )
    }

    // Delete confirmation dialog
    if (uiState.showDeleteDialog) {
        DeleteCustomerDialog(
            customerName = uiState.customer?.name ?: "",
            onDismiss = viewModel::dismissDeleteDialog,
            onConfirm = viewModel::deleteCustomer
        )
    }
}

// -- Top Bar --

@Composable
private fun DetailTopBar(
    customerName: String,
    onNavigateBack: () -> Unit,
    onEdit: () -> Unit,
    onDelete: () -> Unit
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
            text = customerName,
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f)
        )
        IconButton(onClick = onEdit) {
            Icon(
                imageVector = Icons.Default.Edit,
                contentDescription = "Edit"
            )
        }
        IconButton(onClick = onDelete) {
            Icon(
                imageVector = Icons.Default.Delete,
                contentDescription = "Hapus",
                tint = MaterialTheme.colorScheme.error
            )
        }
    }
}

// -- Contact Info --

@Composable
private fun ContactInfoSection(customer: Customer) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        Text(
            text = customer.phone,
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        if (!customer.email.isNullOrBlank()) {
            Text(
                text = customer.email,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        if (!customer.notes.isNullOrBlank()) {
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = customer.notes,
                style = MaterialTheme.typography.bodyMedium,
                fontStyle = FontStyle.Italic,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

// -- KPI Stats Cards --

@Composable
private fun StatsCardsRow(stats: CustomerStatsResponse) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        KpiCard(
            label = "Pesanan",
            value = stats.totalOrders.toString(),
            modifier = Modifier.weight(1f)
        )
        KpiCard(
            label = "Total",
            value = formatPrice(stats.totalSpend.toBigDecimalOrNull() ?: BigDecimal.ZERO),
            modifier = Modifier.weight(1f)
        )
        KpiCard(
            label = "Rata\u00B2",
            value = formatPrice(stats.avgTicket.toBigDecimalOrNull() ?: BigDecimal.ZERO),
            modifier = Modifier.weight(1f)
        )
    }
}

@Composable
private fun KpiCard(
    label: String,
    value: String,
    modifier: Modifier = Modifier
) {
    OutlinedCard(
        modifier = modifier,
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(
            modifier = Modifier.padding(12.dp)
        ) {
            Text(
                text = label,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = value,
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

// -- Menu Favorit --

@Composable
private fun TopItemRow(rank: Int, item: TopItemResponse) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text(
            text = "$rank",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface,
            modifier = Modifier.width(32.dp),
            textAlign = TextAlign.Center
        )
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = item.productName,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = "${item.totalQty} terjual",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        Text(
            text = formatPrice(item.totalRevenue.toBigDecimalOrNull() ?: BigDecimal.ZERO),
            style = MaterialTheme.typography.bodyLarge,
            fontWeight = FontWeight.Medium,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}

// -- Riwayat Pesanan --

@Composable
private fun OrderHistoryCard(
    order: CustomerOrderResponse,
    onClick: () -> Unit
) {
    OutlinedCard(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = order.orderNumber,
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                OrderStatusBadge(status = order.status)
            }

            Spacer(modifier = Modifier.height(8.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = formatOrderType(order.orderType),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = formatPrice(order.totalAmount.toBigDecimalOrNull() ?: BigDecimal.ZERO),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }

            Spacer(modifier = Modifier.height(4.dp))

            Text(
                text = formatTimestamp(order.createdAt),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

// -- Edit Dialog --

@Composable
private fun EditCustomerDialog(
    customer: Customer,
    isUpdating: Boolean,
    updateError: String?,
    onDismiss: () -> Unit,
    onSave: (name: String, phone: String, email: String?, notes: String?) -> Unit
) {
    var name by remember(customer.updatedAt) { mutableStateOf(customer.name) }
    var phone by remember(customer.updatedAt) { mutableStateOf(customer.phone) }
    var email by remember(customer.updatedAt) { mutableStateOf(customer.email ?: "") }
    var notes by remember(customer.updatedAt) { mutableStateOf(customer.notes ?: "") }

    AlertDialog(
        onDismissRequest = { if (!isUpdating) onDismiss() },
        title = { Text("Edit Pelanggan") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text("Nama") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !isUpdating
                )
                OutlinedTextField(
                    value = phone,
                    onValueChange = { phone = it },
                    label = { Text("No. Telepon") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !isUpdating
                )
                OutlinedTextField(
                    value = email,
                    onValueChange = { email = it },
                    label = { Text("Email") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !isUpdating
                )
                OutlinedTextField(
                    value = notes,
                    onValueChange = { notes = it },
                    label = { Text("Catatan") },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !isUpdating,
                    minLines = 2,
                    maxLines = 4
                )
                if (updateError != null) {
                    Text(
                        text = updateError,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    onSave(
                        name.trim(),
                        phone.trim(),
                        email.trim().ifBlank { null },
                        notes.trim().ifBlank { null }
                    )
                },
                enabled = !isUpdating && name.isNotBlank() && phone.isNotBlank()
            ) {
                if (isUpdating) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(16.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.primary
                    )
                } else {
                    Text("Simpan")
                }
            }
        },
        dismissButton = {
            TextButton(
                onClick = onDismiss,
                enabled = !isUpdating
            ) {
                Text("Batal")
            }
        }
    )
}

// -- Delete Confirmation Dialog --

@Composable
private fun DeleteCustomerDialog(
    customerName: String,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Hapus Pelanggan?") },
        text = {
            Text("Pelanggan \"$customerName\" akan dihapus. Tindakan ini tidak dapat dibatalkan.")
        },
        confirmButton = {
            TextButton(onClick = onConfirm) {
                Text(
                    text = "Hapus",
                    color = MaterialTheme.colorScheme.error
                )
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Batal")
            }
        }
    )
}
