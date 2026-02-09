package com.kiwari.pos.ui.orders

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.ActiveOrderResponse
import com.kiwari.pos.data.model.OrderListItem
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.OrderRepository
import com.kiwari.pos.data.repository.TokenRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.math.BigDecimal
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import javax.inject.Inject

enum class OrderFilter { ALL, UNPAID, PAID }

enum class OrderListTab { AKTIF, RIWAYAT }

enum class DatePreset { HARI_INI, KEMARIN, TUJUH_HARI, TIGA_PULUH_HARI }

data class OrderListUiState(
    // Active tab
    val selectedTab: OrderListTab = OrderListTab.AKTIF,
    val isOwnerOrManager: Boolean = false,

    // Aktif tab state
    val isLoading: Boolean = true,
    val isRefreshing: Boolean = false,
    val errorMessage: String? = null,
    val orders: List<ActiveOrderResponse> = emptyList(),
    val filteredOrders: List<ActiveOrderResponse> = emptyList(),
    val selectedFilter: OrderFilter = OrderFilter.ALL,

    // Riwayat tab state
    val historyOrders: List<OrderListItem> = emptyList(),
    val filteredHistoryOrders: List<OrderListItem> = emptyList(),
    val historySearchQuery: String = "",
    val historyDatePreset: DatePreset = DatePreset.HARI_INI,
    val historyStartDate: LocalDate = LocalDate.now(),
    val historyEndDate: LocalDate = LocalDate.now(),
    val isLoadingHistory: Boolean = false,
    val isRefreshingHistory: Boolean = false,
    val historyHasLoaded: Boolean = false,
    val historyError: String? = null
)

@HiltViewModel
class OrderListViewModel @Inject constructor(
    private val orderRepository: OrderRepository,
    private val tokenRepository: TokenRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(OrderListUiState())
    val uiState: StateFlow<OrderListUiState> = _uiState.asStateFlow()

    private var historyLoadJob: Job? = null

    private val dateFormatter = DateTimeFormatter.ofPattern("yyyy-MM-dd")

    init {
        val role = tokenRepository.getUserRole()?.uppercase() ?: ""
        val isOwnerOrManager = role == "OWNER" || role == "MANAGER"
        _uiState.update { it.copy(isOwnerOrManager = isOwnerOrManager) }
        fetchOrders(isRefresh = false)
    }

    // ── Tab switching ──────────────────────

    fun onTabSelected(tab: OrderListTab) {
        _uiState.update { it.copy(selectedTab = tab) }
        if (tab == OrderListTab.RIWAYAT && !_uiState.value.historyHasLoaded && !_uiState.value.isLoadingHistory) {
            loadHistory(isRefresh = false)
        }
    }

    // ── Aktif tab ──────────────────────

    private fun fetchOrders(isRefresh: Boolean) {
        viewModelScope.launch {
            _uiState.update {
                if (isRefresh) it.copy(isRefreshing = true, errorMessage = null)
                else it.copy(isLoading = true, errorMessage = null)
            }

            when (val result = orderRepository.listActiveOrders()) {
                is Result.Success -> {
                    val orders = result.data.orders
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            isRefreshing = false,
                            orders = orders,
                            filteredOrders = computeFilteredOrders(orders, it.selectedFilter)
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            isRefreshing = false,
                            errorMessage = result.message
                        )
                    }
                }
            }
        }
    }

    fun refresh() {
        val state = _uiState.value
        if (state.selectedTab == OrderListTab.AKTIF) {
            fetchOrders(isRefresh = true)
        } else {
            loadHistory(isRefresh = true)
        }
    }

    fun onFilterSelected(filter: OrderFilter) {
        _uiState.update {
            it.copy(
                selectedFilter = filter,
                filteredOrders = computeFilteredOrders(it.orders, filter)
            )
        }
    }

    fun retry() {
        val state = _uiState.value
        if (state.selectedTab == OrderListTab.AKTIF) {
            fetchOrders(isRefresh = false)
        } else {
            loadHistory(isRefresh = false)
        }
    }

    private fun computeFilteredOrders(
        orders: List<ActiveOrderResponse>,
        filter: OrderFilter
    ): List<ActiveOrderResponse> = when (filter) {
        OrderFilter.ALL -> orders
        OrderFilter.UNPAID -> orders.filter {
            val paid = it.amountPaid.toBigDecimalOrNull() ?: BigDecimal.ZERO
            val total = it.totalAmount.toBigDecimalOrNull() ?: BigDecimal.ZERO
            paid.compareTo(total) < 0
        }
        OrderFilter.PAID -> orders.filter {
            val paid = it.amountPaid.toBigDecimalOrNull() ?: BigDecimal.ZERO
            val total = it.totalAmount.toBigDecimalOrNull() ?: BigDecimal.ZERO
            paid.compareTo(total) >= 0
        }
    }

    // ── Riwayat tab ──────────────────────

    fun onHistorySearchChanged(query: String) {
        _uiState.update {
            it.copy(
                historySearchQuery = query,
                filteredHistoryOrders = computeFilteredHistoryOrders(it.historyOrders, query)
            )
        }
    }

    fun onHistoryDatePresetSelected(preset: DatePreset) {
        val today = LocalDate.now()
        val (startDate, endDate) = when (preset) {
            DatePreset.HARI_INI -> today to today
            DatePreset.KEMARIN -> today.minusDays(1) to today.minusDays(1)
            DatePreset.TUJUH_HARI -> today.minusDays(6) to today
            DatePreset.TIGA_PULUH_HARI -> today.minusDays(29) to today
        }
        _uiState.update {
            it.copy(
                historyDatePreset = preset,
                historyStartDate = startDate,
                historyEndDate = endDate
            )
        }
        loadHistory(isRefresh = false)
    }

    private fun loadHistory(isRefresh: Boolean) {
        historyLoadJob?.cancel()
        val currentState = _uiState.value
        val startDate = currentState.historyStartDate.format(dateFormatter)
        val endDate = currentState.historyEndDate.format(dateFormatter)
        historyLoadJob = viewModelScope.launch {
            _uiState.update {
                if (isRefresh) it.copy(isRefreshingHistory = true, historyError = null)
                else it.copy(isLoadingHistory = true, historyError = null)
            }

            when (val result = orderRepository.listOrders(startDate = startDate, endDate = endDate)) {
                is Result.Success -> {
                    val orders = result.data.orders
                    val query = _uiState.value.historySearchQuery
                    _uiState.update {
                        it.copy(
                            isLoadingHistory = false,
                            isRefreshingHistory = false,
                            historyHasLoaded = true,
                            historyOrders = orders,
                            filteredHistoryOrders = computeFilteredHistoryOrders(orders, query)
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(
                            isLoadingHistory = false,
                            isRefreshingHistory = false,
                            historyError = result.message
                        )
                    }
                }
            }
        }
    }

    private fun computeFilteredHistoryOrders(
        orders: List<OrderListItem>,
        query: String
    ): List<OrderListItem> {
        if (query.isBlank()) return orders
        val lowerQuery = query.lowercase()
        return orders.filter { it.orderNumber.lowercase().contains(lowerQuery) }
    }
}
