package com.kiwari.pos.ui.login

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.AuthRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

enum class LoginMode {
    EMAIL,
    PIN
}

data class LoginUiState(
    val email: String = "",
    val password: String = "",
    val outletId: String = "",
    val pin: String = "",
    val loginMode: LoginMode = LoginMode.EMAIL,
    val isLoading: Boolean = false,
    val errorMessage: String? = null,
    val loginSuccess: Boolean = false
)

@HiltViewModel
class LoginViewModel @Inject constructor(
    private val authRepository: AuthRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(LoginUiState())
    val uiState: StateFlow<LoginUiState> = _uiState.asStateFlow()

    fun onEmailChange(email: String) {
        _uiState.update { it.copy(email = email, errorMessage = null) }
    }

    fun onPasswordChange(password: String) {
        _uiState.update { it.copy(password = password, errorMessage = null) }
    }

    fun onOutletIdChange(outletId: String) {
        _uiState.update { it.copy(outletId = outletId, errorMessage = null) }
    }

    fun onPinChange(pin: String) {
        // Only allow digits and max 6 characters
        if (pin.all { it.isDigit() } && pin.length <= 6) {
            _uiState.update { it.copy(pin = pin, errorMessage = null) }
        }
    }

    fun toggleLoginMode() {
        _uiState.update {
            it.copy(
                loginMode = if (it.loginMode == LoginMode.EMAIL) LoginMode.PIN else LoginMode.EMAIL,
                errorMessage = null
            )
        }
    }

    fun login() {
        val currentState = _uiState.value

        // Validate input
        if (currentState.loginMode == LoginMode.EMAIL) {
            if (currentState.email.isBlank()) {
                _uiState.update { it.copy(errorMessage = "Email is required") }
                return
            }
            if (!currentState.email.contains("@")) {
                _uiState.update { it.copy(errorMessage = "Invalid email format") }
                return
            }
            if (currentState.password.isBlank()) {
                _uiState.update { it.copy(errorMessage = "Password is required") }
                return
            }
        } else {
            if (currentState.outletId.isBlank()) {
                _uiState.update { it.copy(errorMessage = "Outlet ID is required") }
                return
            }
            if (currentState.pin.length !in 4..6) {
                _uiState.update { it.copy(errorMessage = "PIN must be 4-6 digits") }
                return
            }
        }

        // Perform login
        _uiState.update { it.copy(isLoading = true, errorMessage = null) }

        viewModelScope.launch {
            val result = if (currentState.loginMode == LoginMode.EMAIL) {
                authRepository.login(currentState.email, currentState.password)
            } else {
                authRepository.pinLogin(currentState.outletId, currentState.pin)
            }

            when (result) {
                is Result.Success -> {
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            loginSuccess = true,
                            errorMessage = null,
                            password = "",
                            pin = ""
                        )
                    }
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            errorMessage = result.message
                        )
                    }
                }
            }
        }
    }

    fun resetLoginSuccess() {
        _uiState.update { it.copy(loginSuccess = false) }
    }

    fun clearError() {
        _uiState.update { it.copy(errorMessage = null) }
    }
}
