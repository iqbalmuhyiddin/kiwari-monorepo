package com.kiwari.pos.ui.customers

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.data.model.CustomerOrderResponse
import com.kiwari.pos.data.model.CustomerStatsResponse
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.CustomerRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class CustomerDetailUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val customer: Customer? = null,
    val stats: CustomerStatsResponse? = null,
    val orders: List<CustomerOrderResponse> = emptyList(),
    val showEditDialog: Boolean = false,
    val isUpdating: Boolean = false,
    val updateError: String? = null,
    val showDeleteDialog: Boolean = false,
    val isDeleting: Boolean = false,
    val isDeleted: Boolean = false
)

@HiltViewModel
class CustomerDetailViewModel @Inject constructor(
    private val customerRepository: CustomerRepository,
    savedStateHandle: SavedStateHandle
) : ViewModel() {

    private val customerId: String = checkNotNull(savedStateHandle["customerId"])

    private val _uiState = MutableStateFlow(CustomerDetailUiState())
    val uiState: StateFlow<CustomerDetailUiState> = _uiState.asStateFlow()

    private var loadJob: Job? = null

    init {
        loadCustomer()
    }

    private fun loadCustomer() {
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessage = null) }

            val customerDeferred = async { customerRepository.getCustomer(customerId) }
            val statsDeferred = async { customerRepository.getCustomerStats(customerId) }
            val ordersDeferred = async { customerRepository.getCustomerOrders(customerId) }

            val customerResult = customerDeferred.await()
            val statsResult = statsDeferred.await()
            val ordersResult = ordersDeferred.await()

            when (customerResult) {
                is Result.Success -> {
                    val stats = when (statsResult) {
                        is Result.Success -> statsResult.data
                        is Result.Error -> null
                    }
                    val orders = when (ordersResult) {
                        is Result.Success -> ordersResult.data
                        is Result.Error -> emptyList()
                    }

                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            customer = customerResult.data,
                            stats = stats,
                            orders = orders
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, errorMessage = customerResult.message)
                    }
                }
            }
        }
    }

    fun refresh() {
        loadCustomer()
    }

    fun showEditDialog() {
        _uiState.update { it.copy(showEditDialog = true, updateError = null) }
    }

    fun dismissEditDialog() {
        _uiState.update { it.copy(showEditDialog = false, updateError = null) }
    }

    fun updateCustomer(name: String, phone: String, email: String?, notes: String?) {
        viewModelScope.launch {
            _uiState.update { it.copy(isUpdating = true, updateError = null) }

            when (val result = customerRepository.updateCustomer(customerId, name, phone, email, notes)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isUpdating = false, showEditDialog = false) }
                    loadCustomer()
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isUpdating = false, updateError = result.message)
                    }
                }
            }
        }
    }

    fun showDeleteDialog() {
        _uiState.update { it.copy(showDeleteDialog = true) }
    }

    fun dismissDeleteDialog() {
        _uiState.update { it.copy(showDeleteDialog = false) }
    }

    fun deleteCustomer() {
        viewModelScope.launch {
            _uiState.update { it.copy(isDeleting = true, showDeleteDialog = false) }

            when (customerRepository.deleteCustomer(customerId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isDeleting = false, isDeleted = true) }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isDeleting = false, errorMessage = "Gagal menghapus pelanggan")
                    }
                }
            }
        }
    }
}
