package com.kiwari.pos.ui.cart

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.CartRepository
import com.kiwari.pos.data.repository.CustomerRepository
import com.kiwari.pos.data.repository.OrderMetadata
import com.kiwari.pos.data.repository.OrderMetadataRepository
import com.kiwari.pos.util.coerceAtLeast
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.math.BigDecimal
import java.math.RoundingMode
import javax.inject.Inject

enum class OrderType {
    DINE_IN, TAKEAWAY, DELIVERY, CATERING
}

enum class DiscountType {
    PERCENTAGE, FIXED_AMOUNT
}

data class CartUiState(
    val cartItems: List<CartItem> = emptyList(),
    val orderType: OrderType = OrderType.DINE_IN,
    val tableNumber: String = "",
    val selectedCustomer: Customer? = null,
    val customerSearchQuery: String = "",
    val customerSearchResults: List<Customer> = emptyList(),
    val isSearchingCustomers: Boolean = false,
    val showCustomerDropdown: Boolean = false,
    val discountType: DiscountType? = null,
    val discountValue: String = "",
    val orderNotes: String = "",
    val subtotal: BigDecimal = BigDecimal.ZERO,
    val discountAmount: BigDecimal = BigDecimal.ZERO,
    val total: BigDecimal = BigDecimal.ZERO,
    val totalItems: Int = 0,
    // Validation
    val cateringCustomerError: Boolean = false,
    // New customer dialog
    val showNewCustomerDialog: Boolean = false,
    val newCustomerName: String = "",
    val newCustomerPhone: String = "",
    val isCreatingCustomer: Boolean = false,
    val customerError: String? = null,
    // Edit item notes dialog
    val editingCartItemId: String? = null,
    val editingNotes: String = ""
)

@OptIn(FlowPreview::class)
@HiltViewModel
class CartViewModel @Inject constructor(
    private val cartRepository: CartRepository,
    private val customerRepository: CustomerRepository,
    private val orderMetadataRepository: OrderMetadataRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(CartUiState())
    val uiState: StateFlow<CartUiState> = _uiState.asStateFlow()

    private val _customerSearchQuery = MutableStateFlow("")

    init {
        observeCart()
        observeCustomerSearch()
    }

    private fun observeCart() {
        viewModelScope.launch {
            cartRepository.items.collect { items ->
                _uiState.update { state ->
                    val subtotal = items.fold(BigDecimal.ZERO) { acc, item ->
                        acc.add(item.lineTotal)
                    }
                    val discountAmount = calculateDiscountAmount(
                        subtotal, state.discountType, state.discountValue
                    )
                    val total = subtotal.subtract(discountAmount).coerceAtLeast(BigDecimal.ZERO)
                    state.copy(
                        cartItems = items,
                        totalItems = items.sumOf { it.quantity },
                        subtotal = subtotal,
                        discountAmount = discountAmount,
                        total = total
                    )
                }
            }
        }
    }

    private fun observeCustomerSearch() {
        viewModelScope.launch {
            _customerSearchQuery
                .debounce(300)
                .distinctUntilChanged()
                .filter { it.length >= 2 }
                .collectLatest { query ->
                    _uiState.update { it.copy(isSearchingCustomers = true) }
                    when (val result = customerRepository.searchCustomers(query)) {
                        is Result.Success -> {
                            _uiState.update {
                                it.copy(
                                    customerSearchResults = result.data,
                                    isSearchingCustomers = false,
                                    showCustomerDropdown = result.data.isNotEmpty()
                                )
                            }
                        }
                        is Result.Error -> {
                            _uiState.update {
                                it.copy(
                                    customerSearchResults = emptyList(),
                                    isSearchingCustomers = false,
                                    showCustomerDropdown = false
                                )
                            }
                        }
                    }
                }
        }
    }

    fun onOrderTypeChanged(orderType: OrderType) {
        _uiState.update {
            it.copy(
                orderType = orderType,
                cateringCustomerError = false
            )
        }
    }

    fun onTableNumberChanged(tableNumber: String) {
        _uiState.update { it.copy(tableNumber = tableNumber) }
    }

    fun onCustomerSearchQueryChanged(query: String) {
        _uiState.update {
            it.copy(
                customerSearchQuery = query,
                cateringCustomerError = false
            )
        }
        if (query.length < 2) {
            _uiState.update {
                it.copy(
                    customerSearchResults = emptyList(),
                    showCustomerDropdown = false,
                    isSearchingCustomers = false
                )
            }
        }
        _customerSearchQuery.value = query
    }

    fun onCustomerSelected(customer: Customer) {
        _uiState.update {
            it.copy(
                selectedCustomer = customer,
                customerSearchQuery = customer.name,
                showCustomerDropdown = false,
                customerSearchResults = emptyList(),
                cateringCustomerError = false
            )
        }
    }

    fun onCustomerCleared() {
        _uiState.update {
            it.copy(
                selectedCustomer = null,
                customerSearchQuery = "",
                showCustomerDropdown = false,
                customerSearchResults = emptyList()
            )
        }
    }

    fun onCustomerDropdownDismissed() {
        _uiState.update { it.copy(showCustomerDropdown = false) }
    }

    fun onDiscountTypeChanged(type: DiscountType?) {
        _uiState.update { state ->
            val discountValue = if (type == null) "" else state.discountValue
            val discountAmount = calculateDiscountAmount(state.subtotal, type, discountValue)
            val total = state.subtotal.subtract(discountAmount).coerceAtLeast(BigDecimal.ZERO)
            state.copy(
                discountType = type,
                discountValue = discountValue,
                discountAmount = discountAmount,
                total = total
            )
        }
    }

    fun onDiscountValueChanged(value: String) {
        // Only allow digits and at most one decimal point
        val filtered = value.filter { it.isDigit() || it == '.' }
        if (filtered.count { it == '.' } > 1) return

        _uiState.update { state ->
            val discountAmount = calculateDiscountAmount(state.subtotal, state.discountType, filtered)
            val total = state.subtotal.subtract(discountAmount).coerceAtLeast(BigDecimal.ZERO)
            state.copy(
                discountValue = filtered,
                discountAmount = discountAmount,
                total = total
            )
        }
    }

    fun onOrderNotesChanged(notes: String) {
        _uiState.update { it.copy(orderNotes = notes) }
    }

    fun onQuantityChanged(cartItemId: String, newQuantity: Int) {
        cartRepository.updateQuantity(cartItemId, newQuantity)
    }

    fun onRemoveItem(cartItemId: String) {
        cartRepository.removeItem(cartItemId)
    }

    fun onEditItem(cartItemId: String) {
        val item = _uiState.value.cartItems.find { it.id == cartItemId } ?: return
        _uiState.update {
            it.copy(editingCartItemId = cartItemId, editingNotes = item.notes)
        }
    }

    fun onEditingNotesChanged(notes: String) {
        _uiState.update { it.copy(editingNotes = notes) }
    }

    fun onSaveItemNotes() {
        val state = _uiState.value
        val cartItemId = state.editingCartItemId ?: return
        cartRepository.updateNotes(cartItemId, state.editingNotes.trim())
        _uiState.update { it.copy(editingCartItemId = null, editingNotes = "") }
    }

    fun onDismissEditDialog() {
        _uiState.update { it.copy(editingCartItemId = null, editingNotes = "") }
    }

    fun onShowNewCustomerDialog() {
        _uiState.update {
            it.copy(
                showNewCustomerDialog = true,
                newCustomerName = "",
                newCustomerPhone = "",
                customerError = null
            )
        }
    }

    fun onDismissNewCustomerDialog() {
        _uiState.update {
            it.copy(
                showNewCustomerDialog = false,
                newCustomerName = "",
                newCustomerPhone = "",
                customerError = null,
                isCreatingCustomer = false
            )
        }
    }

    fun onNewCustomerNameChanged(name: String) {
        _uiState.update { it.copy(newCustomerName = name, customerError = null) }
    }

    fun onNewCustomerPhoneChanged(phone: String) {
        _uiState.update { it.copy(newCustomerPhone = phone, customerError = null) }
    }

    fun onCreateCustomer() {
        val state = _uiState.value
        if (state.newCustomerName.isBlank()) {
            _uiState.update { it.copy(customerError = "Nama harus diisi") }
            return
        }
        if (state.newCustomerPhone.isBlank()) {
            _uiState.update { it.copy(customerError = "Nomor telepon harus diisi") }
            return
        }

        viewModelScope.launch {
            _uiState.update { it.copy(isCreatingCustomer = true, customerError = null) }
            when (val result = customerRepository.createCustomer(
                state.newCustomerName.trim(),
                state.newCustomerPhone.trim()
            )) {
                is Result.Success -> {
                    _uiState.update {
                        it.copy(
                            selectedCustomer = result.data,
                            customerSearchQuery = result.data.name,
                            showNewCustomerDialog = false,
                            newCustomerName = "",
                            newCustomerPhone = "",
                            isCreatingCustomer = false,
                            cateringCustomerError = false
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(
                            customerError = result.message,
                            isCreatingCustomer = false
                        )
                    }
                }
            }
        }
    }

    /**
     * Validate before proceeding to payment.
     * Returns true if valid, false if there are validation errors.
     * On success, saves order metadata to the shared repository for PaymentViewModel.
     */
    fun validateForPayment(): Boolean {
        val state = _uiState.value
        if (state.orderType == OrderType.CATERING && state.selectedCustomer == null) {
            _uiState.update { it.copy(cateringCustomerError = true) }
            return false
        }
        // Save order metadata for PaymentViewModel
        orderMetadataRepository.setMetadata(
            OrderMetadata(
                orderType = state.orderType,
                tableNumber = state.tableNumber,
                customer = state.selectedCustomer,
                discountType = state.discountType,
                discountValue = state.discountValue,
                discountAmount = state.discountAmount,
                notes = state.orderNotes,
                subtotal = state.subtotal,
                total = state.total
            )
        )
        return true
    }

    private fun calculateDiscountAmount(
        subtotal: BigDecimal,
        discountType: DiscountType?,
        discountValueStr: String
    ): BigDecimal {
        if (discountType == null || discountValueStr.isBlank()) return BigDecimal.ZERO

        val discountValue = try {
            BigDecimal(discountValueStr)
        } catch (e: NumberFormatException) {
            return BigDecimal.ZERO
        }

        return when (discountType) {
            DiscountType.PERCENTAGE -> {
                // Cap percentage at 100%
                val cappedPercent = discountValue.min(BigDecimal(100))
                subtotal.multiply(cappedPercent).divide(
                    BigDecimal(100), 0, RoundingMode.HALF_UP
                )
            }
            DiscountType.FIXED_AMOUNT -> {
                // Cap at subtotal to avoid negative totals
                discountValue.min(subtotal)
            }
        }
    }
}
