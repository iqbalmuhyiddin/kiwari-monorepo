package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.DailySalesResponse
import com.kiwari.pos.data.model.HourlySalesResponse
import com.kiwari.pos.data.model.OutletComparisonResponse
import com.kiwari.pos.data.model.PaymentSummaryResponse
import com.kiwari.pos.data.model.ProductSalesResponse
import retrofit2.Response
import retrofit2.http.GET
import retrofit2.http.Path
import retrofit2.http.Query

interface ReportApi {
    @GET("outlets/{outletId}/reports/daily-sales")
    suspend fun getDailySales(
        @Path("outletId") outletId: String,
        @Query("start_date") startDate: String,
        @Query("end_date") endDate: String
    ): Response<List<DailySalesResponse>>

    @GET("outlets/{outletId}/reports/product-sales")
    suspend fun getProductSales(
        @Path("outletId") outletId: String,
        @Query("start_date") startDate: String,
        @Query("end_date") endDate: String,
        @Query("limit") limit: Int = 20
    ): Response<List<ProductSalesResponse>>

    @GET("outlets/{outletId}/reports/payment-summary")
    suspend fun getPaymentSummary(
        @Path("outletId") outletId: String,
        @Query("start_date") startDate: String,
        @Query("end_date") endDate: String
    ): Response<List<PaymentSummaryResponse>>

    @GET("outlets/{outletId}/reports/hourly-sales")
    suspend fun getHourlySales(
        @Path("outletId") outletId: String,
        @Query("start_date") startDate: String,
        @Query("end_date") endDate: String
    ): Response<List<HourlySalesResponse>>

    @GET("reports/outlet-comparison")
    suspend fun getOutletComparison(
        @Query("start_date") startDate: String,
        @Query("end_date") endDate: String
    ): Response<List<OutletComparisonResponse>>
}
