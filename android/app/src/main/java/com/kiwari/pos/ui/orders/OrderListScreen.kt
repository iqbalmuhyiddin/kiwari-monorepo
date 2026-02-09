package com.kiwari.pos.ui.orders

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.TabRowDefaults
import androidx.compose.material3.TabRowDefaults.tabIndicatorOffset
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.data.model.ActiveOrderResponse
import com.kiwari.pos.data.model.OrderListItem
import com.kiwari.pos.util.formatPrice
import java.math.BigDecimal

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun OrderListScreen(
    viewModel: OrderListViewModel = hiltViewModel(),
    onOrderClick: (orderId: String) -> Unit = {},
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
            OrderListTopBar(onNavigateBack = onNavigateBack)

            // Tabs (Riwayat only visible for owner/manager)
            if (uiState.isOwnerOrManager) {
                val selectedTabIndex = if (uiState.selectedTab == OrderListTab.AKTIF) 0 else 1
                TabRow(
                    selectedTabIndex = selectedTabIndex,
                    containerColor = MaterialTheme.colorScheme.surface,
                    contentColor = MaterialTheme.colorScheme.primary,
                    indicator = { tabPositions ->
                        if (selectedTabIndex < tabPositions.size) {
                            TabRowDefaults.SecondaryIndicator(
                                modifier = Modifier.tabIndicatorOffset(tabPositions[selectedTabIndex]),
                                color = MaterialTheme.colorScheme.primary
                            )
                        }
                    }
                ) {
                    Tab(
                        selected = uiState.selectedTab == OrderListTab.AKTIF,
                        onClick = { viewModel.onTabSelected(OrderListTab.AKTIF) },
                        text = {
                            Text(
                                text = "Aktif",
                                fontWeight = if (uiState.selectedTab == OrderListTab.AKTIF) FontWeight.Bold else FontWeight.Normal
                            )
                        }
                    )
                    Tab(
                        selected = uiState.selectedTab == OrderListTab.RIWAYAT,
                        onClick = { viewModel.onTabSelected(OrderListTab.RIWAYAT) },
                        text = {
                            Text(
                                text = "Riwayat",
                                fontWeight = if (uiState.selectedTab == OrderListTab.RIWAYAT) FontWeight.Bold else FontWeight.Normal
                            )
                        }
                    )
                }
            }

            // Content based on selected tab
            when (uiState.selectedTab) {
                OrderListTab.AKTIF -> ActiveOrdersContent(
                    uiState = uiState,
                    onFilterSelected = viewModel::onFilterSelected,
                    onRefresh = viewModel::refresh,
                    onRetry = viewModel::retry,
                    onOrderClick = onOrderClick
                )
                OrderListTab.RIWAYAT -> HistoryOrdersContent(
                    uiState = uiState,
                    onSearchChanged = viewModel::onHistorySearchChanged,
                    onDatePresetSelected = viewModel::onHistoryDatePresetSelected,
                    onRefresh = viewModel::refresh,
                    onRetry = viewModel::retry,
                    onOrderClick = onOrderClick
                )
            }
        }
    }
}

@Composable
private fun OrderListTopBar(
    onNavigateBack: () -> Unit
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
            text = "Pesanan",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}

// ── Aktif Tab Content ──────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ActiveOrdersContent(
    uiState: OrderListUiState,
    onFilterSelected: (OrderFilter) -> Unit,
    onRefresh: () -> Unit,
    onRetry: () -> Unit,
    onOrderClick: (String) -> Unit
) {
    // Filter chips
    OrderFilterChips(
        selectedFilter = uiState.selectedFilter,
        onFilterSelected = onFilterSelected
    )

    when {
        uiState.isLoading -> {
            Box(
                modifier = Modifier
                    .fillMaxSize(),
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
                    .fillMaxSize(),
                contentAlignment = Alignment.Center
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        text = uiState.errorMessage ?: "Terjadi kesalahan",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.error
                    )
                    Spacer(modifier = Modifier.height(16.dp))
                    TextButton(onClick = onRetry) {
                        Text("Coba lagi")
                    }
                }
            }
        }

        else -> {
            val filteredOrders = uiState.filteredOrders

            PullToRefreshBox(
                isRefreshing = uiState.isRefreshing,
                onRefresh = onRefresh,
                modifier = Modifier.fillMaxSize()
            ) {
                if (filteredOrders.isEmpty()) {
                    Box(
                        modifier = Modifier.fillMaxSize(),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = when (uiState.selectedFilter) {
                                OrderFilter.ALL -> "Tidak ada pesanan aktif"
                                OrderFilter.UNPAID -> "Tidak ada pesanan belum bayar"
                                OrderFilter.PAID -> "Tidak ada pesanan lunas"
                            },
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
                            items = filteredOrders,
                            key = { it.id }
                        ) { order ->
                            OrderCard(
                                order = order,
                                onClick = { onOrderClick(order.id) }
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun OrderFilterChips(
    selectedFilter: OrderFilter,
    onFilterSelected: (OrderFilter) -> Unit
) {
    Row(
        modifier = Modifier
            .horizontalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        val filters = listOf(
            OrderFilter.ALL to "Semua",
            OrderFilter.UNPAID to "Belum Bayar",
            OrderFilter.PAID to "Lunas"
        )

        filters.forEach { (filter, label) ->
            FilterChip(
                selected = selectedFilter == filter,
                onClick = { onFilterSelected(filter) },
                label = { Text(label) },
                colors = FilterChipDefaults.filterChipColors(
                    selectedContainerColor = MaterialTheme.colorScheme.secondary,
                    selectedLabelColor = MaterialTheme.colorScheme.onSecondary,
                    containerColor = MaterialTheme.colorScheme.surfaceVariant,
                    labelColor = MaterialTheme.colorScheme.onSurfaceVariant
                ),
                shape = MaterialTheme.shapes.extraSmall
            )
        }
    }
}

@Composable
private fun OrderCard(
    order: ActiveOrderResponse,
    onClick: () -> Unit
) {
    Card(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        ),
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(
            modifier = Modifier.padding(16.dp)
        ) {
            // Row 1: Order number + status badge
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

            Spacer(modifier = Modifier.height(4.dp))

            // Row 2: Order type + context
            Text(
                text = formatOrderContext(order),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(8.dp))

            // Row 3: Payment status + total amount
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                PaymentStatusText(order = order)
                Text(
                    text = formatPrice(order.totalAmount.toBigDecimalOrNull() ?: BigDecimal.ZERO),
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }

            Spacer(modifier = Modifier.height(4.dp))

            // Row 4: Timestamp
            Text(
                text = formatOrderTimestamp(order.createdAt),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun PaymentStatusText(order: ActiveOrderResponse) {
    val paid = order.amountPaid.toBigDecimalOrNull() ?: BigDecimal.ZERO
    val total = order.totalAmount.toBigDecimalOrNull() ?: BigDecimal.ZERO

    when {
        paid.compareTo(total) >= 0 -> {
            Text(
                text = "Lunas",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.primary
            )
        }
        paid.compareTo(BigDecimal.ZERO) > 0 -> {
            Text(
                text = "DP ${formatPrice(paid)} / ${formatPrice(total)}",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.secondary
            )
        }
        else -> {
            Text(
                text = "Belum dibayar",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.error
            )
        }
    }
}

private fun formatOrderContext(order: ActiveOrderResponse): String {
    return when (order.orderType.uppercase()) {
        "DINE_IN" -> {
            val table = order.tableNumber
            if (table != null) "Dine-in \u00b7 Meja $table" else "Dine-in"
        }
        "TAKEAWAY" -> "Takeaway"
        "CATERING" -> {
            val date = order.cateringDate
            if (date != null) "Catering \u00b7 $date" else "Catering"
        }
        "DELIVERY" -> "Delivery"
        else -> order.orderType
    }
}

// ── Riwayat Tab Content ──────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun HistoryOrdersContent(
    uiState: OrderListUiState,
    onSearchChanged: (String) -> Unit,
    onDatePresetSelected: (DatePreset) -> Unit,
    onRefresh: () -> Unit,
    onRetry: () -> Unit,
    onOrderClick: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        // Search bar
        val searchQuery = uiState.historySearchQuery
        OutlinedTextField(
            value = searchQuery,
            onValueChange = onSearchChanged,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
            placeholder = { Text("Cari nomor pesanan...") },
            leadingIcon = {
                Icon(
                    imageVector = Icons.Default.Search,
                    contentDescription = "Cari",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            },
            singleLine = true,
            shape = MaterialTheme.shapes.small,
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = MaterialTheme.colorScheme.primary,
                unfocusedBorderColor = MaterialTheme.colorScheme.outline
            )
        )

        // Date preset chips
        HistoryDatePresetChips(
            selectedPreset = uiState.historyDatePreset,
            onPresetSelected = onDatePresetSelected
        )

        when {
            uiState.isLoadingHistory && !uiState.isRefreshingHistory -> {
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

            uiState.historyError != null -> {
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .fillMaxWidth(),
                    contentAlignment = Alignment.Center
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(
                            text = uiState.historyError ?: "Terjadi kesalahan",
                            style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.error
                        )
                        Spacer(modifier = Modifier.height(16.dp))
                        TextButton(onClick = onRetry) {
                            Text("Coba lagi")
                        }
                    }
                }
            }

            else -> {
                val filteredHistory = uiState.filteredHistoryOrders

                PullToRefreshBox(
                    isRefreshing = uiState.isRefreshingHistory,
                    onRefresh = onRefresh,
                    modifier = Modifier.weight(1f)
                ) {
                    if (filteredHistory.isEmpty()) {
                        Box(
                            modifier = Modifier.fillMaxSize(),
                            contentAlignment = Alignment.Center
                        ) {
                            Text(
                                text = if (searchQuery.isNotBlank()) {
                                    "Tidak ada pesanan ditemukan"
                                } else {
                                    "Tidak ada riwayat pesanan"
                                },
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
                                items = filteredHistory,
                                key = { it.id }
                            ) { order ->
                                val orderId = order.id
                                HistoryOrderCard(
                                    order = order,
                                    onClick = { onOrderClick(orderId) }
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun HistoryDatePresetChips(
    selectedPreset: DatePreset,
    onPresetSelected: (DatePreset) -> Unit
) {
    Row(
        modifier = Modifier
            .horizontalScroll(rememberScrollState())
            .padding(horizontal = 16.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        val presets = listOf(
            DatePreset.HARI_INI to "Hari ini",
            DatePreset.KEMARIN to "Kemarin",
            DatePreset.TUJUH_HARI to "7 Hari",
            DatePreset.TIGA_PULUH_HARI to "30 Hari"
        )

        presets.forEach { (preset, label) ->
            FilterChip(
                selected = selectedPreset == preset,
                onClick = { onPresetSelected(preset) },
                label = { Text(label) },
                colors = FilterChipDefaults.filterChipColors(
                    selectedContainerColor = MaterialTheme.colorScheme.secondary,
                    selectedLabelColor = MaterialTheme.colorScheme.onSecondary,
                    containerColor = MaterialTheme.colorScheme.surfaceVariant,
                    labelColor = MaterialTheme.colorScheme.onSurfaceVariant
                ),
                shape = MaterialTheme.shapes.extraSmall
            )
        }
    }
}

@Composable
private fun HistoryOrderCard(
    order: OrderListItem,
    onClick: () -> Unit
) {
    Card(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        ),
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(
            modifier = Modifier.padding(16.dp)
        ) {
            // Row 1: Order number + status badge
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

            Spacer(modifier = Modifier.height(4.dp))

            // Row 2: Order type + context
            Text(
                text = formatHistoryOrderContext(order),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(8.dp))

            // Row 3: Total amount
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.End
            ) {
                Text(
                    text = formatPrice(order.totalAmount.toBigDecimalOrNull() ?: BigDecimal.ZERO),
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }

            Spacer(modifier = Modifier.height(4.dp))

            // Row 4: Timestamp
            Text(
                text = formatOrderTimestamp(order.createdAt),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

private fun formatHistoryOrderContext(order: OrderListItem): String {
    return when (order.orderType.uppercase()) {
        "DINE_IN" -> {
            val table = order.tableNumber
            if (table != null) "Dine-in \u00b7 Meja $table" else "Dine-in"
        }
        "TAKEAWAY" -> "Takeaway"
        "CATERING" -> "Catering"
        "DELIVERY" -> "Delivery"
        else -> order.orderType
    }
}
