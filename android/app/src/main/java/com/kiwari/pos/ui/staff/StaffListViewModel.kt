package com.kiwari.pos.ui.staff

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.StaffMember
import com.kiwari.pos.data.repository.TokenRepository
import com.kiwari.pos.data.repository.UserRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class StaffListUiState(
    val isLoading: Boolean = true,
    val isRefreshing: Boolean = false,
    val errorMessage: String? = null,
    val staff: List<StaffMember> = emptyList(),
    val currentUserId: String? = null,
    val showDeleteDialog: Boolean = false,
    val deletingStaff: StaffMember? = null,
    val isDeleting: Boolean = false,
    val deleteError: String? = null
)

@HiltViewModel
class StaffListViewModel @Inject constructor(
    private val userRepository: UserRepository,
    private val tokenRepository: TokenRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(StaffListUiState())
    val uiState: StateFlow<StaffListUiState> = _uiState.asStateFlow()
    private var loadJob: Job? = null

    init {
        _uiState.update { it.copy(currentUserId = tokenRepository.getUserId()) }
        loadStaff()
    }

    private fun loadStaff(isRefresh: Boolean = false) {
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _uiState.update {
                if (isRefresh) it.copy(isRefreshing = true, errorMessage = null)
                else it.copy(isLoading = true, errorMessage = null)
            }
            when (val result = userRepository.listUsers()) {
                is Result.Success -> {
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            isRefreshing = false,
                            staff = result.data.sortedBy { s -> s.fullName.lowercase() }
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isLoading = false, isRefreshing = false, errorMessage = result.message)
                    }
                }
            }
        }
    }

    fun refresh() { loadStaff(isRefresh = true) }
    fun retry() { loadStaff() }

    fun showDeleteDialog(staff: StaffMember) {
        _uiState.update { it.copy(showDeleteDialog = true, deletingStaff = staff) }
    }

    fun dismissDeleteDialog() {
        _uiState.update { it.copy(showDeleteDialog = false, deletingStaff = null, deleteError = null) }
    }

    fun deleteStaff() {
        val staff = _uiState.value.deletingStaff ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isDeleting = true, deleteError = null) }
            when (val result = userRepository.deleteUser(staff.id)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isDeleting = false, showDeleteDialog = false, deletingStaff = null, deleteError = null) }
                    loadStaff()
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isDeleting = false, deleteError = result.message) }
                }
            }
        }
    }
}
