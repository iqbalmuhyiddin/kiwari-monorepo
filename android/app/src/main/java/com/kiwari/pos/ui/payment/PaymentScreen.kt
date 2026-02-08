package com.kiwari.pos.ui.payment

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Close
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.data.model.OrderItemResponse
import com.kiwari.pos.util.formatPrice
import java.math.BigDecimal

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PaymentScreen(
    viewModel: PaymentViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit = {},
    onNavigateToMenu: () -> Unit = {},
    onNavigateToOrderDetail: (String) -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }

    // Block back navigation during order submission
    BackHandler(enabled = uiState.isSubmitting) {
        // Block back press during submission
    }

    // Show error as snackbar
    LaunchedEffect(uiState.error) {
        uiState.error?.let {
            snackbarHostState.showSnackbar(it)
            viewModel.onDismissError()
        }
    }

    // Navigate to order detail when existing order payment completes
    LaunchedEffect(uiState.completedOrderId) {
        uiState.completedOrderId?.let { orderId ->
            viewModel.clearCompletedOrderId()
            onNavigateToOrderDetail(orderId)
        }
    }

    // New order success: show success screen then navigate to menu
    if (uiState.isSuccess) {
        SuccessScreen(
            orderNumber = uiState.orderNumber,
            onDone = onNavigateToMenu
        )
        return
    }

    // Loading state for existing order
    if (uiState.isLoadingOrder) {
        Box(
            modifier = Modifier.fillMaxSize(),
            contentAlignment = Alignment.Center
        ) {
            CircularProgressIndicator(color = MaterialTheme.colorScheme.primary)
        }
        return
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "Pembayaran",
                        fontWeight = FontWeight.Bold
                    )
                },
                navigationIcon = {
                    IconButton(
                        onClick = onNavigateBack,
                        enabled = !uiState.isSubmitting
                    ) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                            contentDescription = "Kembali"
                        )
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface,
                    titleContentColor = MaterialTheme.colorScheme.onSurface
                )
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
        bottomBar = {
            PaymentBottomSection(
                totalPaid = uiState.totalPaid,
                remaining = uiState.remaining,
                totalChange = uiState.totalChange,
                orderTotal = uiState.orderTotal,
                isMultiPayment = uiState.isMultiPayment,
                isSubmitting = uiState.isSubmitting,
                canSubmit = uiState.remaining.compareTo(BigDecimal.ZERO) == 0 && !uiState.isSubmitting,
                onSubmit = viewModel::onSubmitOrder
            )
        }
    ) { paddingValues ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(paddingValues)
                .padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // Order summary section
            item {
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = "Ringkasan Pesanan",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }

            // Order items — existing order or cart items
            if (uiState.isExistingOrder) {
                val orderItems = uiState.existingOrder?.items ?: emptyList()
                items(
                    items = orderItems,
                    key = { it.id }
                ) { orderItem ->
                    ExistingOrderSummaryItem(orderItem = orderItem)
                }
            } else {
                items(
                    items = uiState.cartItems,
                    key = { it.id }
                ) { cartItem ->
                    OrderSummaryItem(cartItem = cartItem)
                }
            }

            // Subtotal, discount, total
            item {
                OrderTotalSection(
                    subtotal = uiState.orderSubtotal,
                    discountAmount = uiState.orderDiscountAmount,
                    total = uiState.orderTotal,
                    hasDiscount = uiState.hasDiscount
                )
            }

            // Divider
            item {
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            }

            // Payment entries section
            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "Pembayaran",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    TextButton(onClick = viewModel::onAddPayment) {
                        Icon(
                            imageVector = Icons.Default.Add,
                            contentDescription = null,
                            modifier = Modifier.size(18.dp)
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                        Text("Tambah")
                    }
                }
            }

            // Payment entry cards
            items(
                items = uiState.payments,
                key = { it.id }
            ) { entry ->
                PaymentEntryCard(
                    entry = entry,
                    isMultiPayment = uiState.isMultiPayment,
                    showRemoveButton = uiState.payments.size > 1,
                    orderTotal = uiState.orderTotal,
                    onMethodChanged = { method ->
                        viewModel.onPaymentMethodChanged(entry.id, method)
                    },
                    onAmountChanged = { amount ->
                        viewModel.onPaymentAmountChanged(entry.id, amount)
                    },
                    onAmountReceivedChanged = { received ->
                        viewModel.onAmountReceivedChanged(entry.id, received)
                    },
                    onReferenceChanged = { ref ->
                        viewModel.onReferenceNumberChanged(entry.id, ref)
                    },
                    onRemove = { viewModel.onRemovePayment(entry.id) }
                )
            }

            // Bottom spacer
            item {
                Spacer(modifier = Modifier.height(8.dp))
            }
        }
    }
}

@Composable
private fun OrderSummaryItem(cartItem: CartItem) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Top
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = "${cartItem.quantity}x ${cartItem.product.name}",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            // Show variant info
            cartItem.selectedVariants.forEach { variant ->
                Text(
                    text = "  ${variant.variantGroupName}: ${variant.variantName}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            // Show modifier info
            cartItem.selectedModifiers.forEach { mod ->
                Text(
                    text = "  + ${mod.modifierName}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            // Show notes
            if (cartItem.notes.isNotBlank()) {
                Text(
                    text = "  Catatan: ${cartItem.notes}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
        Text(
            text = formatPrice(cartItem.lineTotal),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
private fun ExistingOrderSummaryItem(orderItem: OrderItemResponse) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Top
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = "${orderItem.quantity}x Item",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            // Unit price
            Text(
                text = "  @ ${formatPrice(BigDecimal(orderItem.unitPrice))}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            // Modifiers
            orderItem.modifiers.forEach { mod ->
                val modPrice = BigDecimal(mod.unitPrice).multiply(BigDecimal(mod.quantity))
                Text(
                    text = "  + Modifier ${if (modPrice.compareTo(BigDecimal.ZERO) > 0) formatPrice(modPrice) else ""}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            // Notes
            if (!orderItem.notes.isNullOrBlank()) {
                Text(
                    text = "  Catatan: ${orderItem.notes}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
        Text(
            text = formatPrice(BigDecimal(orderItem.subtotal)),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
private fun OrderTotalSection(
    subtotal: BigDecimal,
    discountAmount: BigDecimal,
    total: BigDecimal,
    hasDiscount: Boolean
) {
    Column {
        // Subtotal
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                text = "Subtotal",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Text(
                text = formatPrice(subtotal),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
        }

        // Discount
        if (hasDiscount && discountAmount > BigDecimal.ZERO) {
            Spacer(modifier = Modifier.height(4.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = "Diskon",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.error
                )
                Text(
                    text = "-${formatPrice(discountAmount)}",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.error
                )
            }
        }

        Spacer(modifier = Modifier.height(4.dp))
        HorizontalDivider(color = MaterialTheme.colorScheme.outline)
        Spacer(modifier = Modifier.height(4.dp))

        // Total
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                text = "Total",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = formatPrice(total),
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun PaymentEntryCard(
    entry: PaymentEntry,
    isMultiPayment: Boolean,
    showRemoveButton: Boolean,
    orderTotal: BigDecimal,
    onMethodChanged: (PaymentMethod) -> Unit,
    onAmountChanged: (String) -> Unit,
    onAmountReceivedChanged: (String) -> Unit,
    onReferenceChanged: (String) -> Unit,
    onRemove: () -> Unit
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.small,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp)
        ) {
            // Header with remove button
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "Metode Pembayaran",
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                if (showRemoveButton) {
                    IconButton(
                        onClick = onRemove,
                        modifier = Modifier.size(32.dp)
                    ) {
                        Icon(
                            imageVector = Icons.Default.Close,
                            contentDescription = "Hapus pembayaran",
                            modifier = Modifier.size(18.dp),
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            // Method chips
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                PaymentMethod.entries.forEach { method ->
                    val label = when (method) {
                        PaymentMethod.CASH -> "Tunai"
                        PaymentMethod.QRIS -> "QRIS"
                        PaymentMethod.TRANSFER -> "Transfer"
                    }
                    FilterChip(
                        selected = entry.method == method,
                        onClick = { onMethodChanged(method) },
                        label = {
                            Text(
                                text = label,
                                fontWeight = if (entry.method == method) FontWeight.Bold else FontWeight.Normal
                            )
                        },
                        shape = MaterialTheme.shapes.extraSmall,
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = MaterialTheme.colorScheme.secondary,
                            selectedLabelColor = MaterialTheme.colorScheme.onSecondary
                        )
                    )
                }
            }

            // Amount field — only shown in multi-payment mode
            if (isMultiPayment) {
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = entry.amount,
                    onValueChange = onAmountChanged,
                    label = { Text("Jumlah") },
                    prefix = { Text("Rp ") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.extraSmall,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal)
                )
            }

            // Cash-specific: amount received (optional — for change calculation)
            if (entry.method == PaymentMethod.CASH) {
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = entry.amountReceived,
                    onValueChange = onAmountReceivedChanged,
                    label = { Text("Uang Diterima (opsional)") },
                    prefix = { Text("Rp ") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.extraSmall,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal)
                )

                // Show change calculation
                val paymentAmount = if (isMultiPayment) {
                    try { BigDecimal(entry.amount) } catch (_: NumberFormatException) { BigDecimal.ZERO }
                } else {
                    orderTotal
                }
                val received = try { BigDecimal(entry.amountReceived) } catch (_: NumberFormatException) { BigDecimal.ZERO }
                if (received > paymentAmount && paymentAmount > BigDecimal.ZERO) {
                    Spacer(modifier = Modifier.height(4.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Text(
                            text = "Kembalian",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary,
                            fontWeight = FontWeight.Medium
                        )
                        Text(
                            text = formatPrice(received.subtract(paymentAmount)),
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.primary,
                            fontWeight = FontWeight.Bold
                        )
                    }
                }
            }

            // QRIS/Transfer: reference number
            if (entry.method == PaymentMethod.QRIS || entry.method == PaymentMethod.TRANSFER) {
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = entry.referenceNumber,
                    onValueChange = onReferenceChanged,
                    label = { Text("Nomor Referensi (opsional)") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.extraSmall
                )
            }
        }
    }
}

@Composable
private fun PaymentBottomSection(
    totalPaid: BigDecimal,
    remaining: BigDecimal,
    totalChange: BigDecimal,
    orderTotal: BigDecimal,
    isMultiPayment: Boolean,
    isSubmitting: Boolean,
    canSubmit: Boolean,
    onSubmit: () -> Unit
) {
    Surface(
        shadowElevation = 8.dp,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp)
        ) {
            // Total order
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = "Total Pesanan",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = formatPrice(orderTotal),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }

            // Only show paid/remaining breakdown in multi-payment mode
            if (isMultiPayment) {
                Spacer(modifier = Modifier.height(4.dp))

                // Total paid
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(
                        text = "Terbayar",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = formatPrice(totalPaid),
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }

                // Remaining
                if (remaining > BigDecimal.ZERO) {
                    Spacer(modifier = Modifier.height(4.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Text(
                            text = "Sisa",
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.error
                        )
                        Text(
                            text = formatPrice(remaining),
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.error
                        )
                    }
                }
            }

            // Total change
            if (totalChange > BigDecimal.ZERO) {
                Spacer(modifier = Modifier.height(4.dp))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(
                        text = "Kembalian",
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.primary
                    )
                    Text(
                        text = formatPrice(totalChange),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.primary
                    )
                }
            }

            Spacer(modifier = Modifier.height(12.dp))

            // Submit button
            Button(
                onClick = onSubmit,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp),
                enabled = canSubmit,
                shape = MaterialTheme.shapes.small,
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary,
                    contentColor = MaterialTheme.colorScheme.onPrimary
                )
            ) {
                if (isSubmitting) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(24.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onPrimary
                    )
                } else {
                    Text(
                        text = "SELESAI & CETAK",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                }
            }
        }
    }
}

@Composable
private fun SuccessScreen(
    orderNumber: String,
    onDone: () -> Unit
) {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Icon(
                imageVector = Icons.Default.CheckCircle,
                contentDescription = null,
                modifier = Modifier.size(72.dp),
                tint = MaterialTheme.colorScheme.primary
            )
            Text(
                text = "Pesanan Berhasil!",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "No. Pesanan: $orderNumber",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(16.dp))
            Button(
                onClick = onDone,
                modifier = Modifier
                    .fillMaxWidth(0.6f)
                    .height(48.dp),
                shape = MaterialTheme.shapes.small,
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary,
                    contentColor = MaterialTheme.colorScheme.onPrimary
                )
            ) {
                Text(
                    text = "KEMBALI KE MENU",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold
                )
            }
        }
    }
}
