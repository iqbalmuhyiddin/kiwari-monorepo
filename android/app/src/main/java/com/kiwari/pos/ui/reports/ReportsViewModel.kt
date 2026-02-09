package com.kiwari.pos.ui.reports

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.DailySalesResponse
import com.kiwari.pos.data.model.HourlySalesResponse
import com.kiwari.pos.data.model.OutletComparisonResponse
import com.kiwari.pos.data.model.PaymentSummaryResponse
import com.kiwari.pos.data.model.ProductSalesResponse
import com.kiwari.pos.data.model.Result
import com.kiwari.pos.data.repository.ReportRepository
import com.kiwari.pos.data.repository.TokenRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.math.BigDecimal
import java.math.RoundingMode
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import javax.inject.Inject

enum class ReportTab { PENJUALAN, PRODUK, PEMBAYARAN, OUTLET }

enum class DatePreset { HARI_INI, KEMARIN, TUJUH_HARI, CUSTOM }

data class ReportsUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val selectedTab: ReportTab = ReportTab.PENJUALAN,
    val selectedDatePreset: DatePreset = DatePreset.HARI_INI,
    val startDate: LocalDate = LocalDate.now(),
    val endDate: LocalDate = LocalDate.now(),
    val isOwner: Boolean = false,
    val dailySales: List<DailySalesResponse> = emptyList(),
    val hourlySales: List<HourlySalesResponse> = emptyList(),
    val totalRevenue: String = "0",
    val totalOrders: Long = 0,
    val avgTicket: String = "0",
    val productSales: List<ProductSalesResponse> = emptyList(),
    val paymentSummary: List<PaymentSummaryResponse> = emptyList(),
    val outletComparison: List<OutletComparisonResponse> = emptyList()
)

@HiltViewModel
class ReportsViewModel @Inject constructor(
    private val reportRepository: ReportRepository,
    private val tokenRepository: TokenRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(ReportsUiState())
    val uiState: StateFlow<ReportsUiState> = _uiState.asStateFlow()

    private val dateFormatter: DateTimeFormatter = DateTimeFormatter.ofPattern("yyyy-MM-dd")
    private var loadJob: Job? = null

    init {
        val role = tokenRepository.getUserRole()
        _uiState.update { it.copy(isOwner = role == "OWNER") }
        loadData()
    }

    fun onTabSelected(tab: ReportTab) {
        _uiState.update { it.copy(selectedTab = tab) }
        loadData()
    }

    fun onDatePresetSelected(preset: DatePreset) {
        val today = LocalDate.now()
        val (start, end) = when (preset) {
            DatePreset.HARI_INI -> today to today
            DatePreset.KEMARIN -> today.minusDays(1) to today.minusDays(1)
            DatePreset.TUJUH_HARI -> today.minusDays(6) to today
            DatePreset.CUSTOM -> return // Custom handled by onCustomDateRange
        }
        _uiState.update {
            it.copy(
                selectedDatePreset = preset,
                startDate = start,
                endDate = end
            )
        }
        loadData()
    }

    fun onCustomDateRange(start: LocalDate, end: LocalDate) {
        _uiState.update {
            it.copy(
                selectedDatePreset = DatePreset.CUSTOM,
                startDate = start,
                endDate = end
            )
        }
        loadData()
    }

    private fun loadData() {
        // Capture state before launching coroutine to avoid race conditions
        val state = _uiState.value
        val startStr = state.startDate.format(dateFormatter)
        val endStr = state.endDate.format(dateFormatter)
        val tab = state.selectedTab

        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessage = null) }

            when (tab) {
                ReportTab.PENJUALAN -> loadSalesData(startStr, endStr)
                ReportTab.PRODUK -> loadProductData(startStr, endStr)
                ReportTab.PEMBAYARAN -> loadPaymentData(startStr, endStr)
                ReportTab.OUTLET -> loadOutletData(startStr, endStr)
            }
        }
    }

    private suspend fun loadSalesData(startDate: String, endDate: String) {
        val dailyDeferred = viewModelScope.async { reportRepository.getDailySales(startDate, endDate) }
        val hourlyDeferred = viewModelScope.async { reportRepository.getHourlySales(startDate, endDate) }

        val dailyResult = dailyDeferred.await()
        val hourlyResult = hourlyDeferred.await()

        when (dailyResult) {
            is Result.Success -> {
                val daily = dailyResult.data
                val totalRev = daily.fold(BigDecimal.ZERO) { acc, item ->
                    acc.add(item.netRevenue.toBigDecimalOrNull() ?: BigDecimal.ZERO)
                }
                val totalOrd = daily.sumOf { it.orderCount }
                val avg = if (totalOrd > 0) {
                    totalRev.divide(BigDecimal(totalOrd), 0, RoundingMode.HALF_UP)
                } else {
                    BigDecimal.ZERO
                }

                val hourly = when (hourlyResult) {
                    is Result.Success -> hourlyResult.data
                    is Result.Error -> emptyList()
                }

                _uiState.update {
                    it.copy(
                        isLoading = false,
                        dailySales = daily,
                        hourlySales = hourly,
                        totalRevenue = totalRev.toPlainString(),
                        totalOrders = totalOrd,
                        avgTicket = avg.toPlainString()
                    )
                }
            }
            is Result.Error -> {
                _uiState.update {
                    it.copy(isLoading = false, errorMessage = dailyResult.message)
                }
            }
        }
    }

    private suspend fun loadProductData(startDate: String, endDate: String) {
        when (val result = reportRepository.getProductSales(startDate, endDate)) {
            is Result.Success -> {
                _uiState.update {
                    it.copy(isLoading = false, productSales = result.data)
                }
            }
            is Result.Error -> {
                _uiState.update {
                    it.copy(isLoading = false, errorMessage = result.message)
                }
            }
        }
    }

    private suspend fun loadPaymentData(startDate: String, endDate: String) {
        when (val result = reportRepository.getPaymentSummary(startDate, endDate)) {
            is Result.Success -> {
                _uiState.update {
                    it.copy(isLoading = false, paymentSummary = result.data)
                }
            }
            is Result.Error -> {
                _uiState.update {
                    it.copy(isLoading = false, errorMessage = result.message)
                }
            }
        }
    }

    private suspend fun loadOutletData(startDate: String, endDate: String) {
        when (val result = reportRepository.getOutletComparison(startDate, endDate)) {
            is Result.Success -> {
                _uiState.update {
                    it.copy(isLoading = false, outletComparison = result.data)
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
