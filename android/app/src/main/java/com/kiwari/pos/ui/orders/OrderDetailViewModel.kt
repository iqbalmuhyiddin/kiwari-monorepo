package com.kiwari.pos.ui.orders

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.OrderDetailResponse
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

data class OrderDetailUiState(
    val order: OrderDetailResponse? = null,
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val isCancelling: Boolean = false,
    val isPaid: Boolean = false,
    val amountPaid: BigDecimal = BigDecimal.ZERO,
    val amountRemaining: BigDecimal = BigDecimal.ZERO
)

@HiltViewModel
class OrderDetailViewModel @Inject constructor(
    private val orderRepository: OrderRepository,
    savedStateHandle: SavedStateHandle
) : ViewModel() {

    private val orderId: String = checkNotNull(savedStateHandle["orderId"])

    private val _uiState = MutableStateFlow(OrderDetailUiState())
    val uiState: StateFlow<OrderDetailUiState> = _uiState.asStateFlow()

    init {
        loadOrder()
    }

    private fun loadOrder() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessage = null) }

            when (val result = orderRepository.getOrder(orderId)) {
                is Result.Success -> {
                    val order = result.data
                    val paid = order.payments.fold(BigDecimal.ZERO) { acc, p -> acc.add(BigDecimal(p.amount)) }
                    val total = BigDecimal(order.totalAmount)
                    val remaining = total.subtract(paid).let { if (it.compareTo(BigDecimal.ZERO) > 0) it else BigDecimal.ZERO }
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            order = order,
                            isPaid = paid.compareTo(total) >= 0,
                            amountPaid = paid,
                            amountRemaining = remaining
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, errorMessage = result.message)
                    }
                }
            }
        }
    }

    fun refresh() {
        loadOrder()
    }

    fun cancelOrder(onSuccess: () -> Unit) {
        viewModelScope.launch {
            _uiState.update { it.copy(isCancelling = true) }

            when (val result = orderRepository.cancelOrder(orderId)) {
                is Result.Success -> {
                    _uiState.update {
                        it.copy(isCancelling = false)
                    }
                    onSuccess()
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isCancelling = false, errorMessage = result.message)
                    }
                }
            }
        }
    }

    fun printKitchenTicket() { /* TODO: Task 9 */ }
    fun printBill() { /* TODO: Task 9 */ }
    fun shareReceipt() { /* TODO: Task 9 */ }
}
