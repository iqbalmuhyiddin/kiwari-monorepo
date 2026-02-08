package com.kiwari.pos.ui.cart

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.AddOrderItemRequest
import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.data.model.CreateOrderItemModifierRequest
import com.kiwari.pos.data.model.CreateOrderItemRequest
import com.kiwari.pos.data.model.CreateOrderRequest
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.SelectedModifier
import com.kiwari.pos.data.model.SelectedVariant
import com.kiwari.pos.data.model.UpdateOrderItemRequest
import com.kiwari.pos.data.repository.CartRepository
import com.kiwari.pos.data.repository.CustomerRepository
import com.kiwari.pos.data.repository.MenuRepository
import com.kiwari.pos.data.repository.OrderMetadata
import com.kiwari.pos.data.repository.OrderMetadataRepository
import com.kiwari.pos.data.repository.OrderRepository
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
    val editingNotes: String = "",
    // Save order (SIMPAN)
    val isSaving: Boolean = false,
    val savedOrderId: String? = null,
    val saveError: String? = null,
    // Edit mode
    val editingOrderId: String? = null,
    val editingOrderNumber: String? = null,
    val isLoadingOrder: Boolean = false
)

@OptIn(FlowPreview::class)
@HiltViewModel
class CartViewModel @Inject constructor(
    private val cartRepository: CartRepository,
    private val customerRepository: CustomerRepository,
    private val orderMetadataRepository: OrderMetadataRepository,
    private val orderRepository: OrderRepository,
    private val menuRepository: MenuRepository,
    savedStateHandle: SavedStateHandle
) : ViewModel() {

    private val _uiState = MutableStateFlow(CartUiState())
    val uiState: StateFlow<CartUiState> = _uiState.asStateFlow()

    private val _customerSearchQuery = MutableStateFlow("")

    init {
        observeCart()
        observeCustomerSearch()
        // Check if entering edit mode from navigation argument
        savedStateHandle.get<String>("editOrderId")?.let { orderId ->
            loadOrder(orderId)
        }
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

    /**
     * Save order without payment (SIMPAN button).
     * In edit mode, computes a diff and calls add/update/delete item APIs.
     * Otherwise, creates a new order via API.
     */
    fun saveOrder() {
        if (cartRepository.isEditing()) {
            saveOrderEdits()
        } else {
            saveNewOrder()
        }
    }

    private fun saveNewOrder() {
        val state = _uiState.value
        if (state.cartItems.isEmpty() || state.isSaving) return

        // Validate catering requires customer
        if (state.orderType == OrderType.CATERING && state.selectedCustomer == null) {
            _uiState.update { it.copy(cateringCustomerError = true) }
            return
        }

        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null, savedOrderId = null) }

            val request = buildCreateOrderRequest(state)
            when (val result = orderRepository.createOrder(request)) {
                is Result.Success -> {
                    cartRepository.clearCart()
                    orderMetadataRepository.clear()
                    _uiState.update {
                        it.copy(
                            isSaving = false,
                            savedOrderId = result.data.id
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(
                            isSaving = false,
                            saveError = result.message
                        )
                    }
                }
            }
        }
    }

    /**
     * Load an existing order into the cart for editing.
     * Fetches order detail + product data, converts items to CartItems.
     */
    fun loadOrder(orderId: String) {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoadingOrder = true) }

            // 1. Fetch order detail
            val orderResult = orderRepository.getOrder(orderId)
            if (orderResult is Result.Error) {
                _uiState.update {
                    it.copy(isLoadingOrder = false, saveError = orderResult.message)
                }
                return@launch
            }
            val order = (orderResult as Result.Success).data

            // 2. Fetch all products to resolve product data for cart items
            val productsResult = menuRepository.getProducts()
            if (productsResult is Result.Error) {
                _uiState.update {
                    it.copy(isLoadingOrder = false, saveError = productsResult.message)
                }
                return@launch
            }
            val products = (productsResult as Result.Success).data
            val productMap = products.associateBy { it.id }

            // 3. Convert order items to CartItems
            val cartItems = order.items.mapNotNull { orderItem ->
                val product = productMap[orderItem.productId] ?: return@mapNotNull null

                // Calculate modifier total for this item
                val modifierTotal = orderItem.modifiers.fold(BigDecimal.ZERO) { acc, mod ->
                    acc.add(BigDecimal(mod.unitPrice).multiply(BigDecimal(mod.quantity)))
                }

                val selectedVariants = if (orderItem.variantId != null) {
                    // Compute actual variant price adjustment
                    val variantAdjustment = BigDecimal(orderItem.unitPrice)
                        .subtract(BigDecimal(product.basePrice))
                        .subtract(modifierTotal)

                    listOf(
                        SelectedVariant(
                            variantGroupId = "",
                            variantGroupName = "Varian",
                            variantId = orderItem.variantId,
                            variantName = "Varian",
                            priceAdjustment = variantAdjustment
                        )
                    )
                } else {
                    emptyList()
                }

                val selectedModifiers = orderItem.modifiers.map { mod ->
                    SelectedModifier(
                        modifierGroupId = "",
                        modifierGroupName = "Tambahan",
                        modifierId = mod.modifierId,
                        modifierName = "Tambahan",
                        price = BigDecimal(mod.unitPrice)
                    )
                }

                CartItem(
                    id = orderItem.id, // Keep server UUID for diffing
                    product = product,
                    selectedVariants = selectedVariants,
                    selectedModifiers = selectedModifiers,
                    quantity = orderItem.quantity,
                    notes = orderItem.notes ?: "",
                    lineTotal = BigDecimal(orderItem.subtotal)
                )
            }

            // 4. Load items into cart and set edit mode
            cartRepository.setItems(cartItems)
            cartRepository.setEditMode(orderId, order.orderNumber)

            // 5. Set order metadata from the order detail
            val orderType = try {
                OrderType.valueOf(order.orderType)
            } catch (_: IllegalArgumentException) {
                OrderType.DINE_IN
            }
            val discountType = order.discountType?.let {
                try {
                    DiscountType.valueOf(it)
                } catch (_: IllegalArgumentException) {
                    null
                }
            }

            _uiState.update {
                it.copy(
                    orderType = orderType,
                    tableNumber = order.tableNumber ?: "",
                    discountType = discountType,
                    discountValue = order.discountValue ?: "",
                    orderNotes = order.notes ?: "",
                    editingOrderId = orderId,
                    editingOrderNumber = order.orderNumber,
                    isLoadingOrder = false
                )
            }
        }
    }

    /**
     * Compute diff between original items and current cart, then apply changes via API.
     */
    private fun saveOrderEdits() {
        val state = _uiState.value
        val orderId = cartRepository.editingOrderId ?: return
        if (state.isSaving) return

        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null, savedOrderId = null) }

            val originalItems = cartRepository.getOriginalItems()
            val currentItems = cartRepository.items.value

            val originalIds = originalItems.map { it.id }.toSet()
            val currentIds = currentItems.map { it.id }.toSet()

            // Items removed during edit (in original but not in current)
            val removed = originalItems.filter { it.id !in currentIds }
            // Items added during edit (in current but not in original)
            val added = currentItems.filter { it.id !in originalIds }
            // Items that exist in both but may have changed quantity or notes
            val updated = currentItems.filter { current ->
                current.id in originalIds && originalItems.any { orig ->
                    orig.id == current.id &&
                            (orig.quantity != current.quantity || orig.notes != current.notes)
                }
            }

            try {
                // Delete removed items
                for (item in removed) {
                    when (val result = orderRepository.deleteOrderItem(orderId, item.id)) {
                        is Result.Error -> {
                            // Re-fetch order to sync with server after partial changes
                            reloadOrderAfterPartialSave(orderId)
                            _uiState.update {
                                it.copy(isSaving = false, saveError = "Sebagian perubahan tersimpan. ${result.message}")
                            }
                            return@launch
                        }
                        is Result.Success -> { /* continue */ }
                    }
                }

                // Update modified items
                for (item in updated) {
                    val request = UpdateOrderItemRequest(
                        quantity = item.quantity,
                        notes = item.notes.ifBlank { null }
                    )
                    when (val result = orderRepository.updateOrderItem(orderId, item.id, request)) {
                        is Result.Error -> {
                            // Re-fetch order to sync with server after partial changes
                            reloadOrderAfterPartialSave(orderId)
                            _uiState.update {
                                it.copy(isSaving = false, saveError = "Sebagian perubahan tersimpan. ${result.message}")
                            }
                            return@launch
                        }
                        is Result.Success -> { /* continue */ }
                    }
                }

                // Add new items
                for (item in added) {
                    val request = AddOrderItemRequest(
                        productId = item.product.id,
                        variantId = item.selectedVariants.firstOrNull()?.variantId,
                        quantity = item.quantity,
                        notes = item.notes.ifBlank { null },
                        modifiers = item.selectedModifiers.map { mod ->
                            CreateOrderItemModifierRequest(
                                modifierId = mod.modifierId,
                                quantity = 1
                            )
                        }.ifEmpty { null }
                    )
                    when (val result = orderRepository.addOrderItem(orderId, request)) {
                        is Result.Error -> {
                            // Re-fetch order to sync with server after partial changes
                            reloadOrderAfterPartialSave(orderId)
                            _uiState.update {
                                it.copy(isSaving = false, saveError = "Sebagian perubahan tersimpan. ${result.message}")
                            }
                            return@launch
                        }
                        is Result.Success -> { /* continue */ }
                    }
                }

                // All API calls succeeded — clear cart and navigate
                cartRepository.clearCart()
                cartRepository.clearEditMode()
                orderMetadataRepository.clear()
                _uiState.update {
                    it.copy(
                        isSaving = false,
                        savedOrderId = orderId,
                        editingOrderId = null,
                        editingOrderNumber = null
                    )
                }
            } catch (e: Exception) {
                _uiState.update {
                    it.copy(
                        isSaving = false,
                        saveError = e.message ?: "Gagal menyimpan perubahan"
                    )
                }
            }
        }
    }

    /**
     * Re-fetch and reload order after partial save failure to sync UI with server.
     */
    private suspend fun reloadOrderAfterPartialSave(orderId: String) {
        when (val result = orderRepository.getOrder(orderId)) {
            is Result.Success -> {
                val order = result.data
                // Rebuild product map from current cart items (already loaded)
                val productMap = _uiState.value.cartItems.associate { it.product.id to it.product }
                val cartItems = order.items.mapNotNull { orderItem ->
                    val product = productMap[orderItem.productId] ?: return@mapNotNull null
                    val modifierTotal = orderItem.modifiers.fold(BigDecimal.ZERO) { acc, mod ->
                        acc.add(BigDecimal(mod.unitPrice).multiply(BigDecimal(mod.quantity)))
                    }
                    val variantAdjustment = BigDecimal(orderItem.unitPrice)
                        .subtract(BigDecimal(product.basePrice))
                        .subtract(modifierTotal)
                    CartItem(
                        id = orderItem.id,
                        product = product,
                        selectedVariants = if (orderItem.variantId != null) {
                            listOf(SelectedVariant(
                                variantGroupId = "",
                                variantGroupName = "Varian",
                                variantId = orderItem.variantId,
                                variantName = "Varian",
                                priceAdjustment = variantAdjustment
                            ))
                        } else emptyList(),
                        selectedModifiers = orderItem.modifiers.map { mod ->
                            SelectedModifier(
                                modifierGroupId = "",
                                modifierGroupName = "Tambahan",
                                modifierId = mod.modifierId,
                                modifierName = "Tambahan",
                                price = BigDecimal(mod.unitPrice)
                            )
                        },
                        quantity = orderItem.quantity,
                        notes = orderItem.notes ?: "",
                        lineTotal = BigDecimal(orderItem.subtotal)
                    )
                }
                cartRepository.setItems(cartItems)
                cartRepository.setEditMode(orderId, order.orderNumber)
            }
            is Result.Error -> {
                // If we can't even refetch, force exit edit mode
                cartRepository.clearEditMode()
                cartRepository.clearCart()
            }
        }
    }

    /**
     * Exit edit mode without saving — clears cart and edit state.
     */
    fun exitEditMode() {
        cartRepository.clearEditMode()
        cartRepository.clearCart()
        orderMetadataRepository.clear()
        _uiState.update {
            it.copy(editingOrderId = null, editingOrderNumber = null)
        }
    }

    fun clearSavedOrderId() {
        _uiState.update { it.copy(savedOrderId = null) }
    }

    fun clearSaveError() {
        _uiState.update { it.copy(saveError = null) }
    }

    private fun buildCreateOrderRequest(state: CartUiState): CreateOrderRequest {
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
            orderType = state.orderType.name,
            tableNumber = state.tableNumber.ifBlank { null },
            customerId = state.selectedCustomer?.id,
            notes = state.orderNotes.ifBlank { null },
            discountType = state.discountType?.name,
            discountValue = if (state.discountType != null && state.discountValue.isNotBlank()) {
                state.discountValue
            } else null,
            cateringDate = null,
            cateringDpAmount = null,
            deliveryPlatform = null,
            deliveryAddress = null,
            items = items
        )
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
