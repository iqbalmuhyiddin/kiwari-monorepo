package com.kiwari.pos.ui.orders

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.ActiveOrderResponse
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.OrderRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.math.BigDecimal
import javax.inject.Inject

enum class OrderFilter { ALL, UNPAID, PAID }

data class OrderListUiState(
    val isLoading: Boolean = true,
    val isRefreshing: Boolean = false,
    val errorMessage: String? = null,
    val orders: List<ActiveOrderResponse> = emptyList(),
    val filteredOrders: List<ActiveOrderResponse> = emptyList(),
    val selectedFilter: OrderFilter = OrderFilter.ALL
)

@HiltViewModel
class OrderListViewModel @Inject constructor(
    private val orderRepository: OrderRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(OrderListUiState())
    val uiState: StateFlow<OrderListUiState> = _uiState.asStateFlow()

    init {
        fetchOrders(isRefresh = false)
    }

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
        fetchOrders(isRefresh = true)
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
        fetchOrders(isRefresh = false)
    }

    private fun computeFilteredOrders(
        orders: List<ActiveOrderResponse>,
        filter: OrderFilter
    ): List<ActiveOrderResponse> = when (filter) {
        OrderFilter.ALL -> orders
        OrderFilter.UNPAID -> orders.filter {
            BigDecimal(it.amountPaid).compareTo(BigDecimal(it.totalAmount)) < 0
        }
        OrderFilter.PAID -> orders.filter {
            BigDecimal(it.amountPaid).compareTo(BigDecimal(it.totalAmount)) >= 0
        }
    }
}
