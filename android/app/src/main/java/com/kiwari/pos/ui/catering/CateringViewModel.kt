package com.kiwari.pos.ui.catering

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.AddPaymentRequest
import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.data.model.CreateOrderItemModifierRequest
import com.kiwari.pos.data.model.CreateOrderItemRequest
import com.kiwari.pos.data.model.CreateOrderRequest
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.CartRepository
import com.kiwari.pos.data.repository.OrderMetadata
import com.kiwari.pos.data.repository.OrderMetadataRepository
import com.kiwari.pos.data.repository.OrderRepository
import com.kiwari.pos.ui.payment.PaymentEntry
import com.kiwari.pos.ui.payment.PaymentMethod
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
import java.math.RoundingMode
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import javax.inject.Inject

data class CateringUiState(
    // Order data from metadata + cart
    val cartItems: List<CartItem> = emptyList(),
    val metadata: OrderMetadata = OrderMetadata(),
    val customer: Customer? = null,
    // Catering-specific fields
    val cateringDate: LocalDate? = null,
    val cateringDateDisplay: String = "",
    val deliveryAddress: String = "",
    val notes: String = "",
    // DP calculation
    val dpAmount: BigDecimal = BigDecimal.ZERO,
    // Payment entry
    val paymentMethod: PaymentMethod = PaymentMethod.CASH,
    val paymentAmount: String = "",
    val amountReceived: String = "",
    val referenceNumber: String = "",
    // Submission state
    val isSubmitting: Boolean = false,
    val error: String? = null,
    val createdOrderId: String? = null,
    // Success state
    val isSuccess: Boolean = false,
    val orderNumber: String = ""
)

@HiltViewModel
class CateringViewModel @Inject constructor(
    private val cartRepository: CartRepository,
    private val orderMetadataRepository: OrderMetadataRepository,
    private val orderRepository: OrderRepository,
    private val printerService: PrinterService
) : ViewModel() {

    private val _uiState = MutableStateFlow(CateringUiState())
    val uiState: StateFlow<CateringUiState> = _uiState.asStateFlow()

    init {
        loadOrderData()
    }

    private fun loadOrderData() {
        val metadata = orderMetadataRepository.metadata
        val cartItems = cartRepository.items.value
        val dpAmount = metadata.total
            .multiply(BigDecimal("0.5"))
            .setScale(0, RoundingMode.HALF_UP)

        _uiState.update {
            it.copy(
                cartItems = cartItems,
                metadata = metadata,
                customer = metadata.customer,
                notes = metadata.notes,
                dpAmount = dpAmount,
                paymentAmount = dpAmount.toPlainString()
            )
        }
    }

    fun onCateringDateSelected(epochMillis: Long) {
        val date = Instant.ofEpochMilli(epochMillis)
            .atZone(ZoneId.systemDefault())
            .toLocalDate()
        val displayFormatter = DateTimeFormatter.ofPattern("dd MMMM yyyy")
        _uiState.update {
            it.copy(
                cateringDate = date,
                cateringDateDisplay = date.format(displayFormatter)
            )
        }
    }

    fun onDeliveryAddressChanged(address: String) {
        _uiState.update { it.copy(deliveryAddress = address) }
    }

    fun onNotesChanged(notes: String) {
        _uiState.update { it.copy(notes = notes) }
    }

    fun onPaymentMethodChanged(method: PaymentMethod) {
        _uiState.update {
            it.copy(
                paymentMethod = method,
                amountReceived = if (method == PaymentMethod.CASH) it.amountReceived else "",
                referenceNumber = if (method != PaymentMethod.CASH) it.referenceNumber else ""
            )
        }
    }

    fun onPaymentAmountChanged(amount: String) {
        val filtered = filterDecimalInput(amount)
        _uiState.update { it.copy(paymentAmount = filtered) }
    }

    fun onAmountReceivedChanged(amountReceived: String) {
        val filtered = filterDecimalInput(amountReceived)
        _uiState.update { it.copy(amountReceived = filtered) }
    }

    fun onReferenceNumberChanged(reference: String) {
        _uiState.update { it.copy(referenceNumber = reference) }
    }

    fun onDismissError() {
        _uiState.update { it.copy(error = null) }
    }

    fun onSubmitBooking() {
        val state = _uiState.value

        // Validate
        val validationError = validate(state)
        if (validationError != null) {
            _uiState.update { it.copy(error = validationError) }
            return
        }

        viewModelScope.launch {
            _uiState.update { it.copy(isSubmitting = true, error = null) }

            // Check if we're retrying payment for an existing order
            val orderId = state.createdOrderId
            val orderNumber: String

            if (orderId != null) {
                // Retry payment only
                orderNumber = state.orderNumber // Saved from first attempt
                val paymentError = submitDpPayment(orderId, state)
                if (paymentError != null) {
                    _uiState.update {
                        it.copy(
                            isSubmitting = false,
                            error = "Pesanan sudah dibuat. Gagal mencatat DP: $paymentError"
                        )
                    }
                    return@launch
                }
            } else {
                // Step 1: Create order with catering fields
                val orderRequest = buildCreateOrderRequest(state)
                when (val orderResult = orderRepository.createOrder(orderRequest)) {
                    is Result.Success -> {
                        val createdId = orderResult.data.id
                        orderNumber = orderResult.data.orderNumber
                        // Save orderId and orderNumber to state BEFORE attempting payment
                        _uiState.update { it.copy(createdOrderId = createdId, orderNumber = orderNumber) }

                        // Step 2: Add DP payment
                        val paymentError = submitDpPayment(createdId, state)
                        if (paymentError != null) {
                            _uiState.update {
                                it.copy(
                                    isSubmitting = false,
                                    error = "Pesanan sudah dibuat (${orderNumber}). Gagal mencatat DP: $paymentError. Klik BOOK lagi untuk coba lagi pembayaran."
                                )
                            }
                            return@launch
                        }
                    }
                    is Result.Error -> {
                        _uiState.update {
                            it.copy(isSubmitting = false, error = orderResult.message)
                        }
                        return@launch
                    }
                }
            }

            // Trigger auto-print before clearing cart
            triggerAutoprint(state, orderNumber)

            // Success — clear cart and metadata
            cartRepository.clearCart()
            orderMetadataRepository.clear()
            _uiState.update {
                it.copy(
                    isSubmitting = false,
                    isSuccess = true,
                    orderNumber = orderNumber
                )
            }
        }
    }

    private fun triggerAutoprint(state: CateringUiState, orderNumber: String) {
        val metadata = state.metadata
        val discountLabel = when {
            metadata.discountType != null && metadata.discountValue.isNotBlank() -> {
                if (metadata.discountType.name == "PERCENTAGE") "${metadata.discountValue}%" else metadata.discountValue
            }
            else -> null
        }

        val receiptData = ReceiptData(
            outletName = "", // Filled from preferences by PrinterService
            orderNumber = orderNumber.ifBlank { "CATERING" },
            orderType = metadata.orderType.name,
            cartItems = state.cartItems,
            subtotal = metadata.subtotal,
            discountLabel = discountLabel,
            discountAmount = metadata.discountAmount,
            total = metadata.total,
            payments = listOf(
                PaymentEntry(
                    method = state.paymentMethod,
                    amount = state.paymentAmount
                )
            ),
            orderNotes = state.notes,
            isCatering = true,
            dpAmount = state.dpAmount
        )

        viewModelScope.launch {
            printerService.printReceiptIfEnabled(receiptData)
            printerService.printKitchenTicketIfEnabled(receiptData)
        }
    }

    private fun validate(state: CateringUiState): String? {
        if (state.customer == null) {
            return "Pelanggan wajib diisi untuk pesanan Catering"
        }
        if (state.cateringDate == null) {
            return "Tanggal catering wajib diisi"
        }
        if (state.cateringDate.isBefore(LocalDate.now()) || state.cateringDate.isEqual(LocalDate.now())) {
            return "Tanggal catering harus di masa depan"
        }
        val paymentAmount = parseBigDecimal(state.paymentAmount)
        if (paymentAmount <= BigDecimal.ZERO) {
            return "Jumlah DP harus lebih dari 0"
        }
        if (paymentAmount < state.dpAmount) {
            return "Jumlah DP minimal ${state.dpAmount.toPlainString()}"
        }
        if (state.paymentMethod == PaymentMethod.CASH) {
            val received = parseBigDecimal(state.amountReceived)
            if (received > BigDecimal.ZERO && received < paymentAmount) {
                return "Uang diterima harus >= jumlah DP"
            }
        }
        if (state.paymentMethod != PaymentMethod.CASH && state.referenceNumber.isBlank()) {
            return "Nomor referensi wajib diisi untuk ${state.paymentMethod.name}"
        }
        return null
    }

    private fun buildCreateOrderRequest(state: CateringUiState): CreateOrderRequest {
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

        // Format catering date as RFC3339 with local timezone
        val cateringDateRfc3339 = state.cateringDate?.let { date ->
            date.atStartOfDay(ZoneId.systemDefault())
                .format(DateTimeFormatter.ISO_OFFSET_DATE_TIME)
        }

        return CreateOrderRequest(
            orderType = metadata.orderType.name,
            tableNumber = null,
            customerId = state.customer?.id,
            notes = state.notes.ifBlank { null },
            discountType = metadata.discountType?.name,
            discountValue = if (metadata.discountType != null && metadata.discountValue.isNotBlank()) {
                metadata.discountValue
            } else null,
            cateringDate = cateringDateRfc3339,
            cateringDpAmount = state.dpAmount.toPlainString(),
            deliveryPlatform = null,
            deliveryAddress = state.deliveryAddress.ifBlank { null },
            items = items
        )
    }

    private suspend fun submitDpPayment(orderId: String, state: CateringUiState): String? {
        val amount = parseBigDecimal(state.paymentAmount)
        if (amount <= BigDecimal.ZERO) return "Jumlah DP tidak valid"

        val request = AddPaymentRequest(
            paymentMethod = state.paymentMethod.name,
            amount = amount.toPlainString(),
            amountReceived = if (state.paymentMethod == PaymentMethod.CASH) {
                val received = parseBigDecimal(state.amountReceived)
                if (received > BigDecimal.ZERO) received.toPlainString() else amount.toPlainString()
            } else null,
            referenceNumber = if (state.paymentMethod != PaymentMethod.CASH && state.referenceNumber.isNotBlank()) {
                state.referenceNumber.trim()
            } else null
        )

        return when (val result = orderRepository.addPayment(orderId, request)) {
            is Result.Success -> null
            is Result.Error -> result.message
        }
    }
}
