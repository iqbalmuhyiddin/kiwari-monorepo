package com.kiwari.pos.ui.menuadmin

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.Category
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.MenuAdminRepository
import com.kiwari.pos.data.repository.MenuRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class CategoryListUiState(
    val isLoading: Boolean = true,
    val isRefreshing: Boolean = false,
    val errorMessage: String? = null,
    val categories: List<Category> = emptyList(),
    // Create dialog state
    val showCreateDialog: Boolean = false,
    val isCreating: Boolean = false,
    val createError: String? = null,
    // Edit dialog state
    val editingCategory: Category? = null,
    val isEditing: Boolean = false,
    val editError: String? = null,
    // Delete state
    val deletingCategory: Category? = null,
    val isDeleting: Boolean = false,
    val deleteError: String? = null,
    // Reorder state
    val isReordering: Boolean = false
)

@HiltViewModel
class CategoryListViewModel @Inject constructor(
    private val menuRepository: MenuRepository,
    private val menuAdminRepository: MenuAdminRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(CategoryListUiState())
    val uiState: StateFlow<CategoryListUiState> = _uiState.asStateFlow()

    init {
        loadCategories()
    }

    private fun loadCategories(isRefresh: Boolean = false) {
        viewModelScope.launch {
            _uiState.update {
                if (isRefresh) it.copy(isRefreshing = true, errorMessage = null)
                else it.copy(isLoading = true, errorMessage = null)
            }

            when (val result = menuRepository.getCategories()) {
                is Result.Success -> {
                    val sorted = result.data.sortedBy { it.sortOrder }
                    _uiState.update {
                        it.copy(
                            isLoading = false,
                            isRefreshing = false,
                            categories = sorted
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
        loadCategories(isRefresh = true)
    }

    fun retry() {
        loadCategories()
    }

    // ── Create dialog ──

    fun showCreateDialog() {
        _uiState.update { it.copy(showCreateDialog = true, createError = null) }
    }

    fun dismissCreateDialog() {
        _uiState.update { it.copy(showCreateDialog = false, createError = null) }
    }

    fun createCategory(name: String, description: String) {
        viewModelScope.launch {
            _uiState.update { it.copy(isCreating = true, createError = null) }

            val nextSortOrder = (_uiState.value.categories.maxOfOrNull { it.sortOrder } ?: -1) + 1

            when (val result = menuAdminRepository.createCategory(name, description, nextSortOrder)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isCreating = false, showCreateDialog = false) }
                    loadCategories()
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isCreating = false, createError = result.message)
                    }
                }
            }
        }
    }

    // ── Edit dialog ──

    fun showEditDialog(category: Category) {
        _uiState.update { it.copy(editingCategory = category, editError = null) }
    }

    fun dismissEditDialog() {
        _uiState.update { it.copy(editingCategory = null, editError = null) }
    }

    fun updateCategory(name: String, description: String, sortOrder: Int) {
        val categoryId = _uiState.value.editingCategory?.id ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isEditing = true, editError = null) }

            when (val result = menuAdminRepository.updateCategory(categoryId, name, description, sortOrder)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isEditing = false, editingCategory = null) }
                    loadCategories()
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isEditing = false, editError = result.message)
                    }
                }
            }
        }
    }

    // ── Delete dialog ──

    fun showDeleteDialog(category: Category) {
        _uiState.update { it.copy(deletingCategory = category, deleteError = null) }
    }

    fun dismissDeleteDialog() {
        _uiState.update { it.copy(deletingCategory = null, deleteError = null) }
    }

    fun deleteCategory() {
        val categoryId = _uiState.value.deletingCategory?.id ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isDeleting = true, deleteError = null) }

            when (val result = menuAdminRepository.deleteCategory(categoryId)) {
                is Result.Success -> {
                    _uiState.update { it.copy(isDeleting = false, deletingCategory = null) }
                    loadCategories()
                }
                is Result.Error -> {
                    _uiState.update {
                        it.copy(isDeleting = false, deleteError = result.message)
                    }
                }
            }
        }
    }

    // ── Reorder ──

    fun moveCategoryUp(category: Category) {
        if (_uiState.value.isReordering) return
        val categories = _uiState.value.categories
        val index = categories.indexOf(category)
        if (index <= 0) return
        swapSortOrders(categories[index - 1], category)
    }

    fun moveCategoryDown(category: Category) {
        if (_uiState.value.isReordering) return
        val categories = _uiState.value.categories
        val index = categories.indexOf(category)
        if (index < 0 || index >= categories.size - 1) return
        swapSortOrders(category, categories[index + 1])
    }

    private fun swapSortOrders(upper: Category, lower: Category) {
        viewModelScope.launch {
            _uiState.update { it.copy(isReordering = true) }

            val result1 = menuAdminRepository.updateCategory(
                upper.id, upper.name, upper.description ?: "", lower.sortOrder
            )
            if (result1 is Result.Error) {
                _uiState.update { it.copy(isReordering = false, errorMessage = "Gagal mengurutkan: ${result1.message}") }
                return@launch
            }

            val result2 = menuAdminRepository.updateCategory(
                lower.id, lower.name, lower.description ?: "", upper.sortOrder
            )
            if (result2 is Result.Error) {
                // Rollback first call
                menuAdminRepository.updateCategory(
                    upper.id, upper.name, upper.description ?: "", upper.sortOrder
                )
                _uiState.update { it.copy(isReordering = false, errorMessage = "Gagal mengurutkan: ${result2.message}") }
                loadCategories()
                return@launch
            }

            _uiState.update { it.copy(isReordering = false) }
            loadCategories()
        }
    }
}
