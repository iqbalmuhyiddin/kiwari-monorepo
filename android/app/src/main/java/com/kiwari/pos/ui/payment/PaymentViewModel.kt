package com.kiwari.pos.ui.payment

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.AddPaymentRequest
import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.data.model.CreateOrderItemModifierRequest
import com.kiwari.pos.data.model.CreateOrderItemRequest
import com.kiwari.pos.data.model.CreateOrderRequest
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.CartRepository
import com.kiwari.pos.data.repository.OrderMetadata
import com.kiwari.pos.data.repository.OrderMetadataRepository
import com.kiwari.pos.data.repository.OrderRepository
import com.kiwari.pos.util.coerceAtLeast
import com.kiwari.pos.util.filterDecimalInput
import com.kiwari.pos.util.parseBigDecimal
import com.kiwari.pos.util.printer.PrinterService
import com.kiwari.pos.util.printer.ReceiptData
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.math.BigDecimal
import java.util.UUID
import javax.inject.Inject

enum class PaymentMethod {
    CASH, QRIS, TRANSFER
}

data class PaymentEntry(
    val id: String = UUID.randomUUID().toString(),
    val method: PaymentMethod = PaymentMethod.CASH,
    val amount: String = "",
    val amountReceived: String = "", // Only for CASH
    val referenceNumber: String = "" // Only for QRIS/TRANSFER
)

data class PaymentUiState(
    // Order summary (from metadata + cart)
    val cartItems: List<CartItem> = emptyList(),
    val metadata: OrderMetadata = OrderMetadata(),
    // Payment mode: single (default) vs multi (opt-in via "+ Tambah")
    val isMultiPayment: Boolean = false,
    // Payment entries
    val payments: List<PaymentEntry> = listOf(PaymentEntry()),
    // Calculated totals
    val totalPaid: BigDecimal = BigDecimal.ZERO,
    val remaining: BigDecimal = BigDecimal.ZERO,
    val totalChange: BigDecimal = BigDecimal.ZERO,
    // Submission state
    val isSubmitting: Boolean = false,
    val error: String? = null,
    // Success state
    val isSuccess: Boolean = false,
    val orderNumber: String = ""
)

@HiltViewModel
class PaymentViewModel @Inject constructor(
    private val cartRepository: CartRepository,
    private val orderMetadataRepository: OrderMetadataRepository,
    private val orderRepository: OrderRepository,
    private val printerService: PrinterService
) : ViewModel() {

    private val _uiState = MutableStateFlow(PaymentUiState())
    val uiState: StateFlow<PaymentUiState> = _uiState.asStateFlow()

    init {
        loadOrderData()
    }

    private fun loadOrderData() {
        val metadata = orderMetadataRepository.metadata
        val cartItems = cartRepository.items.value
        _uiState.update {
            it.copy(
                cartItems = cartItems,
                metadata = metadata,
                // Single payment mode: paid = total, remaining = 0
                totalPaid = metadata.total,
                remaining = BigDecimal.ZERO
            )
        }
    }

    fun onAddPayment() {
        _uiState.update { state ->
            val updatedPayments = state.payments + PaymentEntry()
            recalculateTotals(state.copy(payments = updatedPayments, isMultiPayment = true))
        }
    }

    fun onRemovePayment(paymentId: String) {
        _uiState.update { state ->
            val updatedPayments = state.payments.filter { it.id != paymentId }
            // Always keep at least one payment entry
            val finalPayments = if (updatedPayments.isEmpty()) listOf(PaymentEntry()) else updatedPayments
            // Revert to single mode when back to 1 entry
            val isMulti = finalPayments.size > 1
            recalculateTotals(state.copy(payments = finalPayments, isMultiPayment = isMulti))
        }
    }

    fun onPaymentMethodChanged(paymentId: String, method: PaymentMethod) {
        _uiState.update { state ->
            val updatedPayments = state.payments.map { entry ->
                if (entry.id == paymentId) {
                    entry.copy(
                        method = method,
                        amountReceived = if (method == PaymentMethod.CASH) entry.amountReceived else "",
                        referenceNumber = if (method != PaymentMethod.CASH) entry.referenceNumber else ""
                    )
                } else entry
            }
            recalculateTotals(state.copy(payments = updatedPayments))
        }
    }

    fun onPaymentAmountChanged(paymentId: String, amount: String) {
        val filtered = filterDecimalInput(amount)
        _uiState.update { state ->
            val updatedPayments = state.payments.map { entry ->
                if (entry.id == paymentId) entry.copy(amount = filtered) else entry
            }
            recalculateTotals(state.copy(payments = updatedPayments))
        }
    }

    fun onAmountReceivedChanged(paymentId: String, amountReceived: String) {
        val filtered = filterDecimalInput(amountReceived)
        _uiState.update { state ->
            val updatedPayments = state.payments.map { entry ->
                if (entry.id == paymentId) entry.copy(amountReceived = filtered) else entry
            }
            recalculateTotals(state.copy(payments = updatedPayments))
        }
    }

    fun onReferenceNumberChanged(paymentId: String, reference: String) {
        _uiState.update { state ->
            val updatedPayments = state.payments.map { entry ->
                if (entry.id == paymentId) entry.copy(referenceNumber = reference) else entry
            }
            state.copy(payments = updatedPayments)
        }
    }

    fun onDismissError() {
        _uiState.update { it.copy(error = null) }
    }

    fun onSubmitOrder() {
        val state = _uiState.value

        // Validate payments
        val validationError = validatePayments(state)
        if (validationError != null) {
            _uiState.update { it.copy(error = validationError) }
            return
        }

        viewModelScope.launch {
            _uiState.update { it.copy(isSubmitting = true, error = null) }

            // Step 1: Create order
            val orderRequest = buildCreateOrderRequest(state)
            when (val orderResult = orderRepository.createOrder(orderRequest)) {
                is Result.Success -> {
                    val orderId = orderResult.data.id
                    // Step 2: Add payments one by one
                    val paymentError = submitPayments(orderId, state)
                    if (paymentError != null) {
                        _uiState.update {
                            it.copy(isSubmitting = false, error = paymentError)
                        }
                    } else {
                        // Trigger auto-print before clearing cart
                        triggerAutoprint(
                            orderNumber = orderResult.data.orderNumber,
                            cartItems = state.cartItems,
                            metadata = state.metadata,
                            payments = state.payments,
                            totalChange = state.totalChange
                        )

                        // Success - clear cart and metadata
                        cartRepository.clearCart()
                        orderMetadataRepository.clear()
                        _uiState.update {
                            it.copy(
                                isSubmitting = false,
                                isSuccess = true,
                                orderNumber = orderResult.data.orderNumber
                            )
                        }
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isSubmitting = false, error = orderResult.message)
                    }
                }
            }
        }
    }

    private suspend fun submitPayments(orderId: String, state: PaymentUiState): String? {
        for (entry in state.payments) {
            // In single mode, amount is the full order total
            val amount = if (state.isMultiPayment) {
                parseBigDecimal(entry.amount)
            } else {
                state.metadata.total
            }
            if (amount <= BigDecimal.ZERO) continue

            val request = AddPaymentRequest(
                paymentMethod = entry.method.name,
                amount = amount.toPlainString(),
                amountReceived = if (entry.method == PaymentMethod.CASH) {
                    val received = parseBigDecimal(entry.amountReceived)
                    if (received > BigDecimal.ZERO) received.toPlainString() else amount.toPlainString()
                } else null,
                referenceNumber = if (entry.method != PaymentMethod.CASH && entry.referenceNumber.isNotBlank()) {
                    entry.referenceNumber.trim()
                } else null
            )

            when (val result = orderRepository.addPayment(orderId, request)) {
                is Result.Success -> { /* Continue to next payment */ }
                is Result.Error -> return result.message
            }
        }
        return null
    }

    private fun buildCreateOrderRequest(state: PaymentUiState): CreateOrderRequest {
        val metadata = state.metadata
        val items = state.cartItems.map { cartItem ->
            CreateOrderItemRequest(
                productId = cartItem.product.id,
                variantId = cartItem.selectedVariants.firstOrNull()?.variantId,
                quantity = cartItem.quantity,
                notes = cartItem.notes.ifBlank { null },
                modifiers = cartItem.selectedModifiers.map { mod ->
                    CreateOrderItemModifierRequest(
                        modifierId = mod.modifierId,
                        quantity = 1
                    )
                }.ifEmpty { null }
            )
        }

        return CreateOrderRequest(
            orderType = metadata.orderType.name,
            tableNumber = metadata.tableNumber.ifBlank { null },
            customerId = metadata.customer?.id,
            notes = metadata.notes.ifBlank { null },
            discountType = metadata.discountType?.name,
            discountValue = if (metadata.discountType != null && metadata.discountValue.isNotBlank()) {
                metadata.discountValue
            } else null,
            cateringDate = metadata.cateringDate,
            cateringDpAmount = metadata.cateringDpAmount?.toPlainString(),
            deliveryPlatform = null, // TODO: populate from OrderMetadataRepository when delivery is implemented
            deliveryAddress = metadata.deliveryAddress,
            items = items
        )
    }

    private fun validatePayments(state: PaymentUiState): String? {
        if (state.isMultiPayment) {
            // Multi-payment: validate amounts per entry
            val nonEmptyPayments = state.payments.filter {
                parseBigDecimal(it.amount) > BigDecimal.ZERO
            }

            if (nonEmptyPayments.isEmpty()) {
                return "Tambahkan minimal satu pembayaran"
            }

            val totalPaid = nonEmptyPayments.fold(BigDecimal.ZERO) { acc, entry ->
                acc.add(parseBigDecimal(entry.amount))
            }

            if (totalPaid.compareTo(state.metadata.total) > 0) {
                return "Total pembayaran melebihi total pesanan"
            }

            if (totalPaid.compareTo(state.metadata.total) < 0) {
                return "Total pembayaran belum mencukupi"
            }

            // Validate cash entries: amount_received must >= amount
            for (entry in nonEmptyPayments) {
                if (entry.method == PaymentMethod.CASH) {
                    val amount = parseBigDecimal(entry.amount)
                    val received = parseBigDecimal(entry.amountReceived)
                    if (received > BigDecimal.ZERO && received < amount) {
                        return "Uang diterima harus >= jumlah pembayaran tunai"
                    }
                }
            }
        } else {
            // Single payment: amount is the full total, just validate cash received
            val entry = state.payments.first()
            if (entry.method == PaymentMethod.CASH) {
                val received = parseBigDecimal(entry.amountReceived)
                if (received > BigDecimal.ZERO && received < state.metadata.total) {
                    return "Uang diterima harus >= total pesanan"
                }
            }
        }

        return null
    }

    private fun triggerAutoprint(
        orderNumber: String,
        cartItems: List<CartItem>,
        metadata: OrderMetadata,
        payments: List<PaymentEntry>,
        totalChange: BigDecimal
    ) {
        val discountLabel = when {
            metadata.discountType != null && metadata.discountValue.isNotBlank() -> {
                if (metadata.discountType.name == "PERCENTAGE") "${metadata.discountValue}%" else metadata.discountValue
            }
            else -> null
        }

        val receiptData = ReceiptData(
            outletName = "", // Filled from preferences by PrinterService
            orderNumber = orderNumber,
            orderType = metadata.orderType.name,
            tableNumber = metadata.tableNumber,
            cartItems = cartItems,
            subtotal = metadata.subtotal,
            discountLabel = discountLabel,
            discountAmount = metadata.discountAmount,
            total = metadata.total,
            payments = payments,
            changeAmount = totalChange,
            orderNotes = metadata.notes
        )

        viewModelScope.launch {
            printerService.printReceiptIfEnabled(receiptData)
            printerService.printKitchenTicketIfEnabled(receiptData)
        }
    }

    private fun recalculateTotals(state: PaymentUiState): PaymentUiState {
        if (state.isMultiPayment) {
            // Multi-payment: sum amounts from each entry
            val totalPaid = state.payments.fold(BigDecimal.ZERO) { acc, entry ->
                acc.add(parseBigDecimal(entry.amount))
            }
            val remaining = state.metadata.total.subtract(totalPaid).coerceAtLeast(BigDecimal.ZERO)

            val totalChange = state.payments
                .filter { it.method == PaymentMethod.CASH }
                .fold(BigDecimal.ZERO) { acc, entry ->
                    val amount = parseBigDecimal(entry.amount)
                    val received = parseBigDecimal(entry.amountReceived)
                    if (received > amount) {
                        acc.add(received.subtract(amount))
                    } else acc
                }

            return state.copy(
                totalPaid = totalPaid,
                remaining = remaining,
                totalChange = totalChange
            )
        } else {
            // Single payment: amount is always the full total
            val entry = state.payments.first()
            val totalChange = if (entry.method == PaymentMethod.CASH) {
                val received = parseBigDecimal(entry.amountReceived)
                if (received > state.metadata.total) {
                    received.subtract(state.metadata.total)
                } else BigDecimal.ZERO
            } else BigDecimal.ZERO

            return state.copy(
                totalPaid = state.metadata.total,
                remaining = BigDecimal.ZERO,
                totalChange = totalChange
            )
        }
    }
}
