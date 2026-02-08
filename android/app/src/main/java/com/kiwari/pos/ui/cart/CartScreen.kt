package com.kiwari.pos.ui.cart

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
import androidx.compose.material.icons.filled.Clear
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Person
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
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
import androidx.compose.ui.window.PopupProperties
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.ui.cart.components.CartItemCard
import com.kiwari.pos.util.formatPrice
import java.math.BigDecimal

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun CartScreen(
    viewModel: CartViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit = {},
    onNavigateToPayment: () -> Unit = {},
    onNavigateToCatering: () -> Unit = {},
    onNavigateToOrderDetail: (String) -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }

    // Navigate to Order Detail after SIMPAN succeeds
    LaunchedEffect(uiState.savedOrderId) {
        uiState.savedOrderId?.let { orderId ->
            viewModel.clearSavedOrderId()
            onNavigateToOrderDetail(orderId)
        }
    }

    // Show save error as snackbar
    LaunchedEffect(uiState.saveError) {
        uiState.saveError?.let { error ->
            viewModel.clearSaveError()
            snackbarHostState.showSnackbar(error)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "Keranjang",
                        fontWeight = FontWeight.Bold
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onNavigateBack) {
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
            CartBottomSection(
                subtotal = uiState.subtotal,
                discountAmount = uiState.discountAmount,
                total = uiState.total,
                hasDiscount = uiState.discountType != null && uiState.discountValue.isNotBlank(),
                isCartEmpty = uiState.cartItems.isEmpty(),
                isCatering = uiState.orderType == OrderType.CATERING,
                isSaving = uiState.isSaving,
                onSave = { viewModel.saveOrder() },
                onPay = {
                    if (viewModel.validateForPayment()) {
                        if (uiState.orderType == OrderType.CATERING) {
                            onNavigateToCatering()
                        } else {
                            onNavigateToPayment()
                        }
                    }
                }
            )
        }
    ) { paddingValues ->
        if (uiState.cartItems.isEmpty()) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(paddingValues),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = "Keranjang kosong",
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        } else {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(paddingValues)
                    .padding(horizontal = 16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                // Order type section
                item {
                    Spacer(modifier = Modifier.height(4.dp))
                    OrderTypeSection(
                        selectedType = uiState.orderType,
                        onTypeSelected = viewModel::onOrderTypeChanged
                    )
                }

                // Table number (dine-in only)
                if (uiState.orderType == OrderType.DINE_IN) {
                    item {
                        OutlinedTextField(
                            value = uiState.tableNumber,
                            onValueChange = viewModel::onTableNumberChanged,
                            label = { Text("Nomor Meja") },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                            shape = MaterialTheme.shapes.extraSmall,
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number)
                        )
                    }
                }

                // Customer section
                item {
                    CustomerSection(
                        searchQuery = uiState.customerSearchQuery,
                        selectedCustomer = uiState.selectedCustomer,
                        searchResults = uiState.customerSearchResults,
                        isSearching = uiState.isSearchingCustomers,
                        showDropdown = uiState.showCustomerDropdown,
                        hasError = uiState.cateringCustomerError,
                        isCatering = uiState.orderType == OrderType.CATERING,
                        onQueryChanged = viewModel::onCustomerSearchQueryChanged,
                        onCustomerSelected = viewModel::onCustomerSelected,
                        onCustomerCleared = viewModel::onCustomerCleared,
                        onDropdownDismissed = viewModel::onCustomerDropdownDismissed,
                        onAddNewCustomer = viewModel::onShowNewCustomerDialog
                    )
                }

                // Cart items header
                item {
                    Text(
                        text = "Item (${uiState.totalItems})",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }

                // Cart item cards
                items(
                    items = uiState.cartItems,
                    key = { it.id }
                ) { cartItem ->
                    CartItemCard(
                        cartItem = cartItem,
                        onQuantityChanged = { qty ->
                            viewModel.onQuantityChanged(cartItem.id, qty)
                        },
                        onEdit = { viewModel.onEditItem(cartItem.id) },
                        onRemove = { viewModel.onRemoveItem(cartItem.id) }
                    )
                }

                // Discount section
                item {
                    DiscountSection(
                        discountType = uiState.discountType,
                        discountValue = uiState.discountValue,
                        onDiscountTypeChanged = viewModel::onDiscountTypeChanged,
                        onDiscountValueChanged = viewModel::onDiscountValueChanged
                    )
                }

                // Order notes
                item {
                    OutlinedTextField(
                        value = uiState.orderNotes,
                        onValueChange = viewModel::onOrderNotesChanged,
                        label = { Text("Catatan pesanan") },
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.extraSmall,
                        minLines = 2,
                        maxLines = 3
                    )
                }

                // Bottom spacer
                item {
                    Spacer(modifier = Modifier.height(8.dp))
                }
            }
        }
    }

    // New customer dialog
    if (uiState.showNewCustomerDialog) {
        NewCustomerDialog(
            name = uiState.newCustomerName,
            phone = uiState.newCustomerPhone,
            isCreating = uiState.isCreatingCustomer,
            error = uiState.customerError,
            onNameChanged = viewModel::onNewCustomerNameChanged,
            onPhoneChanged = viewModel::onNewCustomerPhoneChanged,
            onConfirm = viewModel::onCreateCustomer,
            onDismiss = viewModel::onDismissNewCustomerDialog
        )
    }

    // Edit item notes dialog
    if (uiState.editingCartItemId != null) {
        EditItemNotesDialog(
            notes = uiState.editingNotes,
            onNotesChanged = viewModel::onEditingNotesChanged,
            onConfirm = viewModel::onSaveItemNotes,
            onDismiss = viewModel::onDismissEditDialog
        )
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun OrderTypeSection(
    selectedType: OrderType,
    onTypeSelected: (OrderType) -> Unit
) {
    Column {
        Text(
            text = "Tipe Pesanan",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )
        Spacer(modifier = Modifier.height(8.dp))
        FlowRow(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            OrderType.entries.forEach { type ->
                val label = when (type) {
                    OrderType.DINE_IN -> "Dine-in"
                    OrderType.TAKEAWAY -> "Takeaway"
                    OrderType.DELIVERY -> "Delivery"
                    OrderType.CATERING -> "Catering"
                }
                FilterChip(
                    selected = selectedType == type,
                    onClick = { onTypeSelected(type) },
                    label = {
                        Text(
                            text = label,
                            fontWeight = if (selectedType == type) FontWeight.Bold else FontWeight.Normal
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
    }
}

@Composable
private fun CustomerSection(
    searchQuery: String,
    selectedCustomer: Customer?,
    searchResults: List<Customer>,
    isSearching: Boolean,
    showDropdown: Boolean,
    hasError: Boolean,
    isCatering: Boolean,
    onQueryChanged: (String) -> Unit,
    onCustomerSelected: (Customer) -> Unit,
    onCustomerCleared: () -> Unit,
    onDropdownDismissed: () -> Unit,
    onAddNewCustomer: () -> Unit
) {
    Column {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(
                text = if (isCatering) "Pelanggan *" else "Pelanggan",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold,
                color = if (hasError) MaterialTheme.colorScheme.error
                    else MaterialTheme.colorScheme.onSurface
            )
            TextButton(onClick = onAddNewCustomer) {
                Icon(
                    imageVector = Icons.Default.Add,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp)
                )
                Spacer(modifier = Modifier.width(4.dp))
                Text("Baru")
            }
        }

        if (hasError) {
            Text(
                text = "Pelanggan wajib diisi untuk pesanan Catering",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error
            )
            Spacer(modifier = Modifier.height(4.dp))
        }

        if (selectedCustomer != null) {
            // Show selected customer as a chip-like card
            Card(
                modifier = Modifier.fillMaxWidth(),
                shape = MaterialTheme.shapes.extraSmall,
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.primaryContainer
                )
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 12.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Icon(
                        imageVector = Icons.Default.Person,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.size(20.dp)
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = selectedCustomer.name,
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onPrimaryContainer
                        )
                        Text(
                            text = selectedCustomer.phone,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onPrimaryContainer
                        )
                    }
                    IconButton(
                        onClick = onCustomerCleared,
                        modifier = Modifier.size(32.dp)
                    ) {
                        Icon(
                            imageVector = Icons.Default.Close,
                            contentDescription = "Hapus pelanggan",
                            modifier = Modifier.size(18.dp)
                        )
                    }
                }
            }
        } else {
            // Search field
            Box {
                OutlinedTextField(
                    value = searchQuery,
                    onValueChange = onQueryChanged,
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("Cari nama atau telepon...") },
                    singleLine = true,
                    shape = MaterialTheme.shapes.extraSmall,
                    trailingIcon = {
                        if (isSearching) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(20.dp),
                                strokeWidth = 2.dp,
                                color = MaterialTheme.colorScheme.primary
                            )
                        } else if (searchQuery.isNotEmpty()) {
                            IconButton(onClick = { onQueryChanged("") }) {
                                Icon(
                                    imageVector = Icons.Default.Clear,
                                    contentDescription = "Bersihkan"
                                )
                            }
                        }
                    },
                    isError = hasError
                )

                // Dropdown results
                if (showDropdown && searchResults.isNotEmpty()) {
                    DropdownMenu(
                        expanded = true,
                        onDismissRequest = onDropdownDismissed,
                        properties = PopupProperties(focusable = false),
                        modifier = Modifier
                            .fillMaxWidth(0.9f)
                    ) {
                        searchResults.forEach { customer ->
                            DropdownMenuItem(
                                text = {
                                    Column {
                                        Text(
                                            text = customer.name,
                                            style = MaterialTheme.typography.bodyMedium,
                                            fontWeight = FontWeight.Bold
                                        )
                                        Text(
                                            text = customer.phone,
                                            style = MaterialTheme.typography.bodySmall,
                                            color = MaterialTheme.colorScheme.onSurfaceVariant
                                        )
                                    }
                                },
                                onClick = { onCustomerSelected(customer) }
                            )
                        }
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun DiscountSection(
    discountType: DiscountType?,
    discountValue: String,
    onDiscountTypeChanged: (DiscountType?) -> Unit,
    onDiscountValueChanged: (String) -> Unit
) {
    Column {
        Text(
            text = "Diskon",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )
        Spacer(modifier = Modifier.height(8.dp))

        FlowRow(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            FilterChip(
                selected = discountType == null,
                onClick = { onDiscountTypeChanged(null) },
                label = { Text("Tanpa Diskon") },
                shape = MaterialTheme.shapes.extraSmall,
                colors = FilterChipDefaults.filterChipColors(
                    selectedContainerColor = MaterialTheme.colorScheme.secondary,
                    selectedLabelColor = MaterialTheme.colorScheme.onSecondary
                )
            )
            FilterChip(
                selected = discountType == DiscountType.PERCENTAGE,
                onClick = { onDiscountTypeChanged(DiscountType.PERCENTAGE) },
                label = { Text("Persen (%)") },
                shape = MaterialTheme.shapes.extraSmall,
                colors = FilterChipDefaults.filterChipColors(
                    selectedContainerColor = MaterialTheme.colorScheme.secondary,
                    selectedLabelColor = MaterialTheme.colorScheme.onSecondary
                )
            )
            FilterChip(
                selected = discountType == DiscountType.FIXED_AMOUNT,
                onClick = { onDiscountTypeChanged(DiscountType.FIXED_AMOUNT) },
                label = { Text("Nominal (Rp)") },
                shape = MaterialTheme.shapes.extraSmall,
                colors = FilterChipDefaults.filterChipColors(
                    selectedContainerColor = MaterialTheme.colorScheme.secondary,
                    selectedLabelColor = MaterialTheme.colorScheme.onSecondary
                )
            )
        }

        if (discountType != null) {
            Spacer(modifier = Modifier.height(8.dp))
            OutlinedTextField(
                value = discountValue,
                onValueChange = onDiscountValueChanged,
                modifier = Modifier.fillMaxWidth(),
                label = {
                    Text(
                        when (discountType) {
                            DiscountType.PERCENTAGE -> "Persen diskon"
                            DiscountType.FIXED_AMOUNT -> "Nominal diskon"
                        }
                    )
                },
                suffix = {
                    Text(
                        when (discountType) {
                            DiscountType.PERCENTAGE -> "%"
                            DiscountType.FIXED_AMOUNT -> "Rp"
                        }
                    )
                },
                singleLine = true,
                shape = MaterialTheme.shapes.extraSmall,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal)
            )
        }
    }
}

@Composable
private fun CartBottomSection(
    subtotal: BigDecimal,
    discountAmount: BigDecimal,
    total: BigDecimal,
    hasDiscount: Boolean,
    isCartEmpty: Boolean,
    isCatering: Boolean = false,
    isSaving: Boolean = false,
    onSave: () -> Unit = {},
    onPay: () -> Unit
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

            // Discount line
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

            Spacer(modifier = Modifier.height(12.dp))

            if (isCatering) {
                // Catering: single LANJUT BOOKING button (unchanged)
                Button(
                    onClick = onPay,
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(52.dp),
                    enabled = !isCartEmpty,
                    shape = MaterialTheme.shapes.small,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = MaterialTheme.colorScheme.primary,
                        contentColor = MaterialTheme.colorScheme.onPrimary
                    )
                ) {
                    Text(
                        text = "LANJUT BOOKING",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                }
            } else {
                // Non-catering: SIMPAN + BAYAR side by side
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    OutlinedButton(
                        onClick = onSave,
                        modifier = Modifier
                            .weight(1f)
                            .height(52.dp),
                        enabled = !isCartEmpty && !isSaving,
                        shape = MaterialTheme.shapes.small
                    ) {
                        if (isSaving) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(20.dp),
                                strokeWidth = 2.dp
                            )
                        } else {
                            Text(
                                text = "SIMPAN",
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.Bold
                            )
                        }
                    }
                    Button(
                        onClick = onPay,
                        modifier = Modifier
                            .weight(1f)
                            .height(52.dp),
                        enabled = !isCartEmpty && !isSaving,
                        shape = MaterialTheme.shapes.small,
                        colors = ButtonDefaults.buttonColors(
                            containerColor = MaterialTheme.colorScheme.primary,
                            contentColor = MaterialTheme.colorScheme.onPrimary
                        )
                    ) {
                        Text(
                            text = "BAYAR",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun NewCustomerDialog(
    name: String,
    phone: String,
    isCreating: Boolean,
    error: String?,
    onNameChanged: (String) -> Unit,
    onPhoneChanged: (String) -> Unit,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit
) {
    AlertDialog(
        onDismissRequest = { if (!isCreating) onDismiss() },
        title = {
            Text(
                text = "Pelanggan Baru",
                fontWeight = FontWeight.Bold
            )
        },
        text = {
            Column {
                OutlinedTextField(
                    value = name,
                    onValueChange = onNameChanged,
                    label = { Text("Nama *") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.extraSmall,
                    enabled = !isCreating
                )
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = phone,
                    onValueChange = onPhoneChanged,
                    label = { Text("Nomor Telepon *") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.extraSmall,
                    enabled = !isCreating,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone)
                )
                if (error != null) {
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        text = error,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
        },
        confirmButton = {
            Button(
                onClick = onConfirm,
                enabled = !isCreating,
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary
                )
            ) {
                if (isCreating) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(18.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onPrimary
                    )
                } else {
                    Text("Simpan")
                }
            }
        },
        dismissButton = {
            TextButton(
                onClick = onDismiss,
                enabled = !isCreating
            ) {
                Text("Batal")
            }
        }
    )
}

@Composable
private fun EditItemNotesDialog(
    notes: String,
    onNotesChanged: (String) -> Unit,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            Text(
                text = "Edit Catatan",
                fontWeight = FontWeight.Bold
            )
        },
        text = {
            OutlinedTextField(
                value = notes,
                onValueChange = onNotesChanged,
                label = { Text("Catatan item") },
                modifier = Modifier.fillMaxWidth(),
                shape = MaterialTheme.shapes.extraSmall,
                minLines = 2,
                maxLines = 4
            )
        },
        confirmButton = {
            Button(
                onClick = onConfirm,
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.primary
                )
            ) {
                Text("Simpan")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Batal")
            }
        }
    )
}
