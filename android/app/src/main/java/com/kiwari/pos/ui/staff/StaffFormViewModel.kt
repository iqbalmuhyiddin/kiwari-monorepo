package com.kiwari.pos.ui.staff

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.CreateUserRequest
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.model.UpdateUserRequest
import com.kiwari.pos.data.repository.TokenRepository
import com.kiwari.pos.data.repository.UserRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class StaffFormUiState(
    val isLoading: Boolean = false,
    val isCreateMode: Boolean = true,
    val fullName: String = "",
    val email: String = "",
    val password: String = "",
    val pin: String = "",
    val selectedRole: String = "CASHIER",
    val isSaving: Boolean = false,
    val saveError: String? = null,
    val saveSuccess: Boolean = false,
    val isOwner: Boolean = false
)

@HiltViewModel
class StaffFormViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val userRepository: UserRepository,
    private val tokenRepository: TokenRepository
) : ViewModel() {

    private val userId: String = savedStateHandle.get<String>("userId") ?: "new"
    private val _uiState = MutableStateFlow(StaffFormUiState())
    val uiState: StateFlow<StaffFormUiState> = _uiState.asStateFlow()
    private var loadJob: Job? = null

    init {
        val role = tokenRepository.getUserRole()
        val isCreateMode = userId == "new"
        _uiState.update { it.copy(isCreateMode = isCreateMode, isOwner = role == "OWNER") }
        if (!isCreateMode) {
            loadUser()
        }
    }

    fun onSaveSuccessConsumed() {
        _uiState.update { it.copy(saveSuccess = false) }
    }

    private fun loadUser() {
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true) }
            when (val result = userRepository.listUsers()) {
                is Result.Success -> {
                    val user = result.data.find { it.id == userId }
                    if (user != null) {
                        _uiState.update {
                            it.copy(
                                isLoading = false,
                                fullName = user.fullName,
                                email = user.email,
                                pin = user.pin ?: "",
                                selectedRole = user.role
                            )
                        }
                    } else {
                        _uiState.update { it.copy(isLoading = false, saveError = "Pengguna tidak ditemukan") }
                    }
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isLoading = false, saveError = result.message) }
                }
            }
        }
    }

    fun onFullNameChanged(value: String) { _uiState.update { it.copy(fullName = value, saveError = null) } }
    fun onEmailChanged(value: String) { _uiState.update { it.copy(email = value, saveError = null) } }
    fun onPasswordChanged(value: String) { _uiState.update { it.copy(password = value, saveError = null) } }
    fun onPinChanged(value: String) {
        val filtered = value.filter { it.isDigit() }.take(6)
        _uiState.update { it.copy(pin = filtered, saveError = null) }
    }
    fun onRoleSelected(role: String) { _uiState.update { it.copy(selectedRole = role, saveError = null) } }

    fun saveUser() {
        val state = _uiState.value
        if (state.isSaving) return

        // Validation
        if (state.fullName.isBlank()) {
            _uiState.update { it.copy(saveError = "Nama harus diisi") }
            return
        }
        if (state.email.isBlank() || !state.email.contains("@")) {
            _uiState.update { it.copy(saveError = "Email tidak valid") }
            return
        }
        if (state.isCreateMode && state.password.isBlank()) {
            _uiState.update { it.copy(saveError = "Password harus diisi") }
            return
        }
        if (state.pin.isNotBlank() && (state.pin.length < 4 || state.pin.length > 6)) {
            _uiState.update { it.copy(saveError = "PIN harus 4-6 digit") }
            return
        }

        viewModelScope.launch {
            _uiState.update { it.copy(isSaving = true, saveError = null) }
            val pinValue = state.pin.ifBlank { null }

            val result = if (state.isCreateMode) {
                userRepository.createUser(
                    CreateUserRequest(
                        email = state.email.trim(),
                        password = state.password,
                        fullName = state.fullName.trim(),
                        role = state.selectedRole,
                        pin = pinValue
                    )
                )
            } else {
                userRepository.updateUser(
                    userId = userId,
                    request = UpdateUserRequest(
                        email = state.email.trim(),
                        fullName = state.fullName.trim(),
                        role = state.selectedRole,
                        pin = pinValue
                    )
                )
            }

            when (result) {
                is Result.Success -> {
                    _uiState.update { it.copy(isSaving = false, saveSuccess = true) }
                }
                is Result.Error -> {
                    _uiState.update { it.copy(isSaving = false, saveError = result.message) }
                }
            }
        }
    }
}
