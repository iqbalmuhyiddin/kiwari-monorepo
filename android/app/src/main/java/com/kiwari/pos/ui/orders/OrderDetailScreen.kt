package com.kiwari.pos.ui.orders

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedCard
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
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.data.model.OrderDetailResponse
import com.kiwari.pos.data.model.OrderItemModifierResponse
import com.kiwari.pos.data.model.OrderItemResponse
import com.kiwari.pos.data.model.PaymentDetailResponse
import com.kiwari.pos.util.formatPrice
import java.math.BigDecimal

@Composable
fun OrderDetailScreen(
    viewModel: OrderDetailViewModel = hiltViewModel(),
    onEdit: (orderId: String) -> Unit = {},
    onPay: (orderId: String) -> Unit = {},
    onBack: () -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // Top bar
            DetailTopBar(onBack = onBack)

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

                uiState.errorMessage != null && uiState.order == null -> {
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

                uiState.order != null -> {
                    val order = uiState.order ?: return@Column
                    val isPaid = uiState.isPaid
                    val amountPaid = uiState.amountPaid
                    val amountRemaining = uiState.amountRemaining

                    Column(
                        modifier = Modifier
                            .weight(1f)
                            .verticalScroll(rememberScrollState())
                            .padding(horizontal = 16.dp, vertical = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        // Section 1: Header info
                        OrderHeaderSection(order = order)

                        // Section 2: Items list
                        ItemsSection(items = order.items)

                        // Section 3: Totals
                        TotalsSection(order = order)

                        // Section 4/5/6: Payment section
                        PaymentSection(
                            order = order,
                            isPaid = isPaid,
                            amountPaid = amountPaid,
                            amountRemaining = amountRemaining
                        )

                        // Section 7: Action row (print/share)
                        ActionRow(
                            isPaid = isPaid,
                            onPrintKitchen = viewModel::printKitchenTicket,
                            onPrintBill = viewModel::printBill,
                            onShare = viewModel::shareReceipt
                        )

                        // Section 8: Bottom buttons (unpaid only)
                        if (!isPaid && order.status.uppercase() != "CANCELLED") {
                            BottomActionButtons(
                                orderId = order.id,
                                onEdit = onEdit,
                                onPay = onPay
                            )
                        }

                        // Section 9: Cancel link (unpaid only)
                        if (!isPaid && order.status.uppercase() != "CANCELLED") {
                            CancelOrderSection(
                                isCancelling = uiState.isCancelling,
                                onCancel = { viewModel.cancelOrder(onSuccess = onBack) }
                            )
                        }

                        Spacer(modifier = Modifier.height(16.dp))
                    }
                }
            }
        }
    }
}

// ── Top Bar ──────────────────────────────────────────

@Composable
private fun DetailTopBar(onBack: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(MaterialTheme.colorScheme.surface)
            .padding(horizontal = 4.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        IconButton(onClick = onBack) {
            Icon(
                imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                contentDescription = "Kembali"
            )
        }
        Text(
            text = "Detail Pesanan",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}

// ── Section 1: Header ───────────────────────────────

@Composable
private fun OrderHeaderSection(order: OrderDetailResponse) {
    OutlinedCard(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            // Order number + status badge
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = order.orderNumber,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                OrderStatusBadge(status = order.status)
            }

            Spacer(modifier = Modifier.height(8.dp))

            // Order type + table/catering info
            Text(
                text = formatDetailContext(order),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            // Customer ID
            if (order.customerId != null) {
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = "Pelanggan: ${order.customerId}",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }

            // Notes
            if (!order.notes.isNullOrBlank()) {
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = "Catatan: ${order.notes}",
                    style = MaterialTheme.typography.bodyMedium,
                    fontStyle = FontStyle.Italic,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }

            Spacer(modifier = Modifier.height(4.dp))

            // Timestamp
            Text(
                text = formatOrderTimestamp(order.createdAt),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

private fun formatDetailContext(order: OrderDetailResponse): String {
    return when (order.orderType.uppercase()) {
        "DINE_IN" -> {
            val table = order.tableNumber
            if (table != null) "Dine-in \u00b7 Meja $table" else "Dine-in"
        }
        "TAKEAWAY" -> "Takeaway"
        "CATERING" -> {
            val parts = mutableListOf("Catering")
            order.cateringDate?.let { parts.add(it) }
            order.cateringStatus?.let { parts.add(formatCateringStatus(it)) }
            parts.joinToString(" \u00b7 ")
        }
        "DELIVERY" -> {
            val parts = mutableListOf("Delivery")
            order.deliveryPlatform?.let { parts.add(it) }
            parts.joinToString(" \u00b7 ")
        }
        else -> order.orderType
    }
}

private fun formatCateringStatus(status: String): String = when (status.uppercase()) {
    "BOOKED" -> "Dipesan"
    "DP_PAID" -> "DP Dibayar"
    "SETTLED" -> "Lunas"
    else -> status
}

// ── Section 2: Items ────────────────────────────────

@Composable
private fun ItemsSection(items: List<OrderItemResponse>) {
    OutlinedCard(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "Item Pesanan",
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface
            )

            Spacer(modifier = Modifier.height(8.dp))

            items.forEachIndexed { index, item ->
                if (index > 0) {
                    HorizontalDivider(
                        modifier = Modifier.padding(vertical = 8.dp),
                        color = MaterialTheme.colorScheme.outline.copy(alpha = 0.3f)
                    )
                }
                OrderItemRow(item = item)
            }
        }
    }
}

@Composable
private fun OrderItemRow(item: OrderItemResponse) {
    Column {
        // Qty x Name + subtotal
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                text = "${item.quantity}x Item",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.weight(1f)
            )
            Text(
                text = formatPrice(BigDecimal(item.subtotal)),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
        }

        // Unit price
        Text(
            text = "@ ${formatPrice(BigDecimal(item.unitPrice))}",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        // Modifiers (indented)
        item.modifiers.forEach { modifier ->
            ModifierRow(modifier = modifier)
        }

        // Notes (italic)
        if (!item.notes.isNullOrBlank()) {
            Text(
                text = item.notes,
                style = MaterialTheme.typography.bodySmall,
                fontStyle = FontStyle.Italic,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 2.dp)
            )
        }
    }
}

@Composable
private fun ModifierRow(modifier: OrderItemModifierResponse) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = 16.dp),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = "+ Modifier" + if (modifier.quantity > 1) " x${modifier.quantity}" else "",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.weight(1f)
        )
        Text(
            text = formatPrice(BigDecimal(modifier.unitPrice).multiply(BigDecimal(modifier.quantity))),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

// ── Section 3: Totals ───────────────────────────────

@Composable
private fun TotalsSection(order: OrderDetailResponse) {
    OutlinedCard(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            // Subtotal
            TotalRow(
                label = "Subtotal",
                amount = formatPrice(BigDecimal(order.subtotal))
            )

            // Discount (if any)
            val discountAmount = BigDecimal(order.discountAmount)
            if (discountAmount.compareTo(BigDecimal.ZERO) > 0) {
                Spacer(modifier = Modifier.height(4.dp))
                val discountLabel = when {
                    order.discountType?.uppercase() == "PERCENTAGE" && order.discountValue != null ->
                        "Diskon (${order.discountValue}%)"
                    else -> "Diskon"
                }
                TotalRow(
                    label = discountLabel,
                    amount = "-${formatPrice(discountAmount)}",
                    isDiscount = true
                )
            }

            // Tax (if any)
            val taxAmount = BigDecimal(order.taxAmount)
            if (taxAmount.compareTo(BigDecimal.ZERO) > 0) {
                Spacer(modifier = Modifier.height(4.dp))
                TotalRow(
                    label = "Pajak",
                    amount = formatPrice(taxAmount)
                )
            }

            HorizontalDivider(
                modifier = Modifier.padding(vertical = 8.dp),
                color = MaterialTheme.colorScheme.outline
            )

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
                    text = formatPrice(BigDecimal(order.totalAmount)),
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
        }
    }
}

@Composable
private fun TotalRow(
    label: String,
    amount: String,
    isDiscount: Boolean = false
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = amount,
            style = MaterialTheme.typography.bodyMedium,
            color = if (isDiscount) MaterialTheme.colorScheme.error
                    else MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

// ── Section 4/5/6: Payment ──────────────────────────

@Composable
private fun PaymentSection(
    order: OrderDetailResponse,
    isPaid: Boolean,
    amountPaid: BigDecimal,
    amountRemaining: BigDecimal
) {
    OutlinedCard(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "Pembayaran",
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface
            )

            Spacer(modifier = Modifier.height(8.dp))

            when {
                // Paid: show each payment
                isPaid -> {
                    order.payments.forEachIndexed { index, payment ->
                        if (index > 0) {
                            HorizontalDivider(
                                modifier = Modifier.padding(vertical = 6.dp),
                                color = MaterialTheme.colorScheme.outline.copy(alpha = 0.3f)
                            )
                        }
                        PaymentRow(payment = payment)
                    }
                }

                // Catering DP_PAID: show DP amount + remaining
                order.orderType.uppercase() == "CATERING"
                    && order.cateringStatus?.uppercase() == "DP_PAID" -> {
                    order.payments.forEachIndexed { index, payment ->
                        if (index > 0) {
                            HorizontalDivider(
                                modifier = Modifier.padding(vertical = 6.dp),
                                color = MaterialTheme.colorScheme.outline.copy(alpha = 0.3f)
                            )
                        }
                        PaymentRow(payment = payment)
                    }

                    HorizontalDivider(
                        modifier = Modifier.padding(vertical = 8.dp),
                        color = MaterialTheme.colorScheme.outline
                    )

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Text(
                            text = "DP dibayar",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.secondary
                        )
                        Text(
                            text = formatPrice(amountPaid),
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Medium,
                            color = MaterialTheme.colorScheme.secondary
                        )
                    }
                    Spacer(modifier = Modifier.height(4.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Text(
                            text = "Sisa pembayaran",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.error
                        )
                        Text(
                            text = formatPrice(amountRemaining),
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Medium,
                            color = MaterialTheme.colorScheme.error
                        )
                    }
                }

                // Unpaid: show "Belum dibayar"
                else -> {
                    // Show partial payments if any exist
                    if (order.payments.isNotEmpty()) {
                        order.payments.forEachIndexed { index, payment ->
                            if (index > 0) {
                                HorizontalDivider(
                                    modifier = Modifier.padding(vertical = 6.dp),
                                    color = MaterialTheme.colorScheme.outline.copy(alpha = 0.3f)
                                )
                            }
                            PaymentRow(payment = payment)
                        }
                        HorizontalDivider(
                            modifier = Modifier.padding(vertical = 8.dp),
                            color = MaterialTheme.colorScheme.outline
                        )
                    }

                    Text(
                        text = "Belum dibayar",
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
        }
    }
}

@Composable
private fun PaymentRow(payment: PaymentDetailResponse) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                PaymentMethodBadge(method = payment.paymentMethod)
                Text(
                    text = formatPrice(BigDecimal(payment.amount)),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
            Text(
                text = formatOrderTimestamp(payment.processedAt),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun PaymentMethodBadge(method: String) {
    val label = when (method.uppercase()) {
        "CASH" -> "Tunai"
        "QRIS" -> "QRIS"
        "TRANSFER" -> "Transfer"
        else -> method
    }

    Text(
        text = label,
        style = MaterialTheme.typography.labelSmall,
        fontWeight = FontWeight.Medium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier
            .background(
                color = MaterialTheme.colorScheme.surfaceVariant,
                shape = MaterialTheme.shapes.extraSmall
            )
            .padding(horizontal = 8.dp, vertical = 4.dp)
    )
}

// ── Section 7: Action Row ───────────────────────────

@Composable
private fun ActionRow(
    isPaid: Boolean,
    onPrintKitchen: () -> Unit,
    onPrintBill: () -> Unit,
    onShare: () -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        OutlinedButton(
            onClick = onPrintKitchen,
            modifier = Modifier.weight(1f),
            shape = MaterialTheme.shapes.small
        ) {
            Text(
                text = "Cetak Dapur",
                style = MaterialTheme.typography.labelMedium
            )
        }
        OutlinedButton(
            onClick = onPrintBill,
            modifier = Modifier.weight(1f),
            shape = MaterialTheme.shapes.small
        ) {
            Text(
                text = if (isPaid) "Cetak Struk" else "Cetak Bill",
                style = MaterialTheme.typography.labelMedium
            )
        }
        OutlinedButton(
            onClick = onShare,
            modifier = Modifier.weight(1f),
            shape = MaterialTheme.shapes.small
        ) {
            Text(
                text = "Bagikan",
                style = MaterialTheme.typography.labelMedium
            )
        }
    }
}

// ── Section 8: Bottom Action Buttons ────────────────

@Composable
private fun BottomActionButtons(
    orderId: String,
    onEdit: (orderId: String) -> Unit,
    onPay: (orderId: String) -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        OutlinedButton(
            onClick = { onEdit(orderId) },
            modifier = Modifier.weight(1f),
            shape = MaterialTheme.shapes.small
        ) {
            Text("EDIT")
        }
        Button(
            onClick = { onPay(orderId) },
            modifier = Modifier.weight(1f),
            shape = MaterialTheme.shapes.small,
            colors = ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.primary,
                contentColor = MaterialTheme.colorScheme.onPrimary
            )
        ) {
            Text("BAYAR")
        }
    }
}

// ── Section 9: Cancel Order ─────────────────────────

@Composable
private fun CancelOrderSection(
    isCancelling: Boolean,
    onCancel: () -> Unit
) {
    var showDialog by remember { mutableStateOf(false) }

    Box(
        modifier = Modifier.fillMaxWidth(),
        contentAlignment = Alignment.Center
    ) {
        TextButton(
            onClick = { showDialog = true },
            enabled = !isCancelling
        ) {
            if (isCancelling) {
                CircularProgressIndicator(
                    modifier = Modifier
                        .height(16.dp)
                        .width(16.dp),
                    strokeWidth = 2.dp,
                    color = MaterialTheme.colorScheme.error
                )
                Spacer(modifier = Modifier.width(8.dp))
            }
            Text(
                text = "Batalkan Pesanan",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.error
            )
        }
    }

    if (showDialog) {
        AlertDialog(
            onDismissRequest = { showDialog = false },
            title = {
                Text("Batalkan Pesanan?")
            },
            text = {
                Text("Pesanan yang sudah dibatalkan tidak dapat dikembalikan.")
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        showDialog = false
                        onCancel()
                    }
                ) {
                    Text(
                        text = "Batalkan",
                        color = MaterialTheme.colorScheme.error
                    )
                }
            },
            dismissButton = {
                TextButton(onClick = { showDialog = false }) {
                    Text("Kembali")
                }
            }
        )
    }
}
