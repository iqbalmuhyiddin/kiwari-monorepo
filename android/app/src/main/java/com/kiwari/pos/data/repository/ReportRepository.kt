package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.ReportApi
import com.kiwari.pos.data.model.DailySalesResponse
import com.kiwari.pos.data.model.HourlySalesResponse
import com.kiwari.pos.data.model.OutletComparisonResponse
import com.kiwari.pos.data.model.PaymentSummaryResponse
import com.kiwari.pos.data.model.ProductSalesResponse
import com.kiwari.pos.data.model.Result
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ReportRepository @Inject constructor(
    private val reportApi: ReportApi,
    private val tokenRepository: TokenRepository,
    private val gson: Gson
) {
    suspend fun getDailySales(startDate: String, endDate: String): Result<List<DailySalesResponse>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { reportApi.getDailySales(outletId, startDate, endDate) }
    }

    suspend fun getProductSales(startDate: String, endDate: String): Result<List<ProductSalesResponse>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { reportApi.getProductSales(outletId, startDate, endDate) }
    }

    suspend fun getPaymentSummary(startDate: String, endDate: String): Result<List<PaymentSummaryResponse>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { reportApi.getPaymentSummary(outletId, startDate, endDate) }
    }

    suspend fun getHourlySales(startDate: String, endDate: String): Result<List<HourlySalesResponse>> {
        val outletId = tokenRepository.getOutletId()
            ?: return Result.Error("No outlet selected")
        return safeApiCall(gson) { reportApi.getHourlySales(outletId, startDate, endDate) }
    }

    suspend fun getOutletComparison(startDate: String, endDate: String): Result<List<OutletComparisonResponse>> {
        return safeApiCall(gson) { reportApi.getOutletComparison(startDate, endDate) }
    }
}
