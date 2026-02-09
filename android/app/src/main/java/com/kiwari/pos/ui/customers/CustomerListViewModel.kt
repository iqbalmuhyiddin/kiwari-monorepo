package com.kiwari.pos.ui.customers

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.Customer
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.CustomerRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

enum class CustomerSort { SEMUA, TERBARU }

data class CustomerListUiState(
    val isLoading: Boolean = true,
    val isRefreshing: Boolean = false,
    val errorMessage: String? = null,
    val customers: List<Customer> = emptyList(),
    val filteredCustomers: List<Customer> = emptyList(),
    val searchQuery: String = "",
    val selectedSort: CustomerSort = CustomerSort.SEMUA,
    val showCreateDialog: Boolean = false,
    val isCreating: Boolean = false,
    val createError: String? = null
)

@HiltViewModel
class CustomerListViewModel @Inject constructor(
    private val customerRepository: CustomerRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(CustomerListUiState())
    val uiState: StateFlow<CustomerListUiState> = _uiState.asStateFlow()

    init {
        loadCustomers()
    }

    private fun loadCustomers(isRefresh: Boolean = false) {
        viewModelScope.launch {
            _uiState.update {
                if (isRefresh) it.copy(isRefreshing = true, errorMessage = null)
                else it.copy(isLoading = true, errorMessage = null)
            }

            when (val result = customerRepository.listCustomers()) {
                is Result.Success -> {
                    val customers = result.data
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            isRefreshing = false,
                            customers = customers,
                            filteredCustomers = applyFilters(customers, it.searchQuery, it.selectedSort)
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
        loadCustomers(isRefresh = true)
    }

    fun onSearchQueryChanged(query: String) {
        _uiState.update {
            it.copy(
                searchQuery = query,
                filteredCustomers = applyFilters(it.customers, query, it.selectedSort)
            )
        }
    }

    fun onSortSelected(sort: CustomerSort) {
        _uiState.update {
            it.copy(
                selectedSort = sort,
                filteredCustomers = applyFilters(it.customers, it.searchQuery, sort)
            )
        }
    }

    fun showCreateDialog() {
        _uiState.update { it.copy(showCreateDialog = true, createError = null) }
    }

    fun dismissCreateDialog() {
        _uiState.update { it.copy(showCreateDialog = false, createError = null) }
    }

    fun createCustomer(name: String, phone: String) {
        viewModelScope.launch {
            _uiState.update { it.copy(isCreating = true, createError = null) }

            when (val result = customerRepository.createCustomer(name, phone)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isCreating = false, showCreateDialog = false) }
                    loadCustomers()
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isCreating = false, createError = result.message)
                    }
                }
            }
        }
    }

    fun retry() {
        loadCustomers()
    }

    private fun applyFilters(
        customers: List<Customer>,
        searchQuery: String,
        sort: CustomerSort
    ): List<Customer> {
        val filtered = if (searchQuery.isBlank()) {
            customers
        } else {
            val query = searchQuery.lowercase()
            customers.filter {
                it.name.lowercase().contains(query) || it.phone.lowercase().contains(query)
            }
        }

        return when (sort) {
            CustomerSort.SEMUA -> filtered.sortedBy { it.name.lowercase() }
            CustomerSort.TERBARU -> filtered.sortedByDescending { it.createdAt }
        }
    }
}
