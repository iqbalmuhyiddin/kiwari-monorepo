# Android POS — Admin Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add admin capabilities (drawer navigation, reports, CRM, order history, menu CRUD, staff management) to the Android POS app so owners and managers can manage the business from their phone.

**Architecture:** Drawer navigation overlays the existing Menu screen, with role-based filtering. Each admin feature is a standalone screen (or set of screens) accessed via the drawer. All features reuse the existing patterns: Retrofit API interfaces → Repositories (with `safeApiCall`) → ViewModels (StateFlow) → Composable screens. New API interfaces for reports, user admin, and menu admin CRUD. Existing `CustomerApi`, `OrderApi`, and models are extended where needed.

**Tech Stack:** Kotlin, Jetpack Compose, Hilt DI, Retrofit, Material3, existing `safeApiCall`/`Result` pattern.

**Depends on:** Order flow plan (completed — provides OrderListScreen, OrderDetailScreen, OrderApi).

**Base path:** `android/app/src/main/java/com/kiwari/pos/`

---

## Task 1: Role Utility + Drawer Navigation Foundation

**Goal:** Add a role-checking utility and a Material3 modal navigation drawer to the app, replacing the current top-bar-only navigation on MenuScreen.

**Files:**
- Create: `util/RoleAccess.kt`
- Create: `ui/navigation/AppDrawer.kt`
- Modify: `MainActivity.kt`
- Modify: `ui/navigation/NavGraph.kt`
- Modify: `ui/menu/MenuScreen.kt`

**Step 1: Create `util/RoleAccess.kt`**

```kotlin
package com.kiwari.pos.util

import com.kiwari.pos.data.model.UserRole

enum class DrawerFeature {
    PESANAN,
    LAPORAN,
    MENU_ADMIN,
    PELANGGAN,
    PENGGUNA,
    PRINTER
}

fun isFeatureVisible(feature: DrawerFeature, role: UserRole): Boolean {
    return when (feature) {
        DrawerFeature.PESANAN -> role in listOf(UserRole.OWNER, UserRole.MANAGER, UserRole.CASHIER)
        DrawerFeature.LAPORAN -> role in listOf(UserRole.OWNER, UserRole.MANAGER)
        DrawerFeature.MENU_ADMIN -> role in listOf(UserRole.OWNER, UserRole.MANAGER)
        DrawerFeature.PELANGGAN -> role in listOf(UserRole.OWNER, UserRole.MANAGER)
        DrawerFeature.PENGGUNA -> role in listOf(UserRole.OWNER, UserRole.MANAGER)
        DrawerFeature.PRINTER -> role in listOf(UserRole.OWNER, UserRole.MANAGER, UserRole.CASHIER)
    }
}
```

**Step 2: Create `ui/navigation/AppDrawer.kt`**

The drawer composable. Shows user info header + role-filtered menu items.

```kotlin
package com.kiwari.pos.ui.navigation

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.kiwari.pos.data.model.UserRole
import com.kiwari.pos.util.DrawerFeature
import com.kiwari.pos.util.isFeatureVisible

data class DrawerItem(
    val feature: DrawerFeature,
    val label: String,
    val icon: String // emoji for simplicity, or use Icons
)

@Composable
fun AppDrawerContent(
    userName: String,
    userRole: UserRole,
    outletName: String,
    onItemClick: (DrawerFeature) -> Unit,
    onLogout: () -> Unit
) {
    val allItems = listOf(
        DrawerItem(DrawerFeature.PESANAN, "Pesanan", "📋"),
        DrawerItem(DrawerFeature.LAPORAN, "Laporan", "📊"),
        DrawerItem(DrawerFeature.MENU_ADMIN, "Kelola Menu", "🍽️"),
        DrawerItem(DrawerFeature.PELANGGAN, "Pelanggan", "👥"),
        DrawerItem(DrawerFeature.PENGGUNA, "Pengguna", "👤"),
        DrawerItem(DrawerFeature.PRINTER, "Printer", "🖨️")
    )

    val visibleItems = allItems.filter { isFeatureVisible(it.feature, userRole) }

    ModalDrawerSheet(modifier = Modifier.width(280.dp)) {
        // Header: user info
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(MaterialTheme.colorScheme.primary)
                .padding(24.dp)
        ) {
            Text(
                text = "Kiwari POS",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onPrimary
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "$userName (${userRole.name})",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.9f)
            )
            // outletName can be added when we have it stored
        }

        Spacer(modifier = Modifier.height(8.dp))

        // Menu items
        visibleItems.forEach { item ->
            NavigationDrawerItem(
                label = { Text(item.label) },
                selected = false,
                onClick = { onItemClick(item.feature) },
                modifier = Modifier.padding(horizontal = 12.dp)
            )
        }

        Spacer(modifier = Modifier.weight(1f))
        HorizontalDivider(modifier = Modifier.padding(horizontal = 16.dp))

        // Logout
        NavigationDrawerItem(
            label = { Text("Keluar", color = MaterialTheme.colorScheme.error) },
            selected = false,
            onClick = onLogout,
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)
        )
    }
}
```

**Step 3: Modify `MainActivity.kt`**

Move the `Scaffold` + `NavHost` setup to support a `ModalNavigationDrawer` at the top level. The drawer state lives here so it persists across navigation.

In `MainActivity.kt`, replace the `setContent` block:

```kotlin
setContent {
    KiwariTheme {
        val navController = rememberNavController()
        val drawerState = rememberDrawerState(initialValue = DrawerValue.Closed)

        Scaffold(modifier = Modifier.fillMaxSize()) { innerPadding ->
            NavGraph(
                navController = navController,
                tokenRepository = tokenRepository,
                drawerState = drawerState,
                modifier = Modifier.padding(innerPadding)
            )
        }
    }
}
```

Add imports for `rememberDrawerState`, `DrawerValue`, `DrawerState`.

**Step 4: Modify `NavGraph.kt`**

- Add `drawerState: DrawerState` parameter
- Wrap the `NavHost` with `ModalNavigationDrawer`
- Add new routes for admin screens (placeholder composables for now)
- Read user role and name from `tokenRepository`
- Handle drawer item clicks → navigate + close drawer
- Handle logout → `tokenRepository.clearTokens()`

Add new `Screen` entries:

```kotlin
object Reports : Screen("reports")
object MenuAdmin : Screen("menu-admin")
object CustomerList : Screen("customers")
object StaffList : Screen("staff")
```

The drawer wraps the `NavHost`:

```kotlin
val userName = tokenRepository.getUserName() ?: "User"
val userRoleStr = tokenRepository.getUserRole()
val userRole = try {
    UserRole.valueOf(userRoleStr ?: "CASHIER")
} catch (e: Exception) {
    UserRole.CASHIER
}
val scope = rememberCoroutineScope()

ModalNavigationDrawer(
    drawerState = drawerState,
    drawerContent = {
        AppDrawerContent(
            userName = userName,
            userRole = userRole,
            outletName = "", // TODO: store outlet name
            onItemClick = { feature ->
                scope.launch { drawerState.close() }
                when (feature) {
                    DrawerFeature.PESANAN -> navController.navigate(Screen.OrderList.route) { launchSingleTop = true }
                    DrawerFeature.LAPORAN -> navController.navigate(Screen.Reports.route) { launchSingleTop = true }
                    DrawerFeature.MENU_ADMIN -> navController.navigate(Screen.MenuAdmin.route) { launchSingleTop = true }
                    DrawerFeature.PELANGGAN -> navController.navigate(Screen.CustomerList.route) { launchSingleTop = true }
                    DrawerFeature.PENGGUNA -> navController.navigate(Screen.StaffList.route) { launchSingleTop = true }
                    DrawerFeature.PRINTER -> navController.navigate(Screen.Settings.route) { launchSingleTop = true }
                }
            },
            onLogout = {
                scope.launch { drawerState.close() }
                tokenRepository.clearTokens()
            }
        )
    },
    gesturesEnabled = drawerState.isOpen // only allow swipe-to-close when open
) {
    NavHost(...) {
        // existing composables...
        // + placeholder composables for new routes
        composable(Screen.Reports.route) {
            // Placeholder — implemented in Task 3
            Text("Laporan (coming soon)")
        }
        composable(Screen.MenuAdmin.route) {
            Text("Kelola Menu (coming soon)")
        }
        composable(Screen.CustomerList.route) {
            Text("Pelanggan (coming soon)")
        }
        composable(Screen.StaffList.route) {
            Text("Pengguna (coming soon)")
        }
    }
}
```

**Step 5: Modify `MenuScreen.kt`**

Replace the Settings icon with a hamburger menu icon. The Settings and OrderList icons move into the drawer.

In `MenuTopBar`, replace the `Row` with icons:

```kotlin
// Add hamburger icon on the LEFT of the title
IconButton(onClick = onMenuClick) {
    Icon(
        imageVector = Icons.Default.Menu,
        contentDescription = "Menu"
    )
}
Text("Menu", ...)
// Keep only search icon on the right
Row(modifier = Modifier.align(Alignment.CenterEnd)) {
    IconButton(onClick = onToggleSearch) {
        Icon(imageVector = Icons.Default.Search, contentDescription = "Cari produk")
    }
}
```

Add `onMenuClick` callback to `MenuScreen` and `MenuTopBar`. In NavGraph, wire it to `scope.launch { drawerState.open() }`.

**Step 6: Build and verify**

Run: `cd android && ./gradlew assembleDebug`
Expected: Build succeeds. Hamburger icon opens drawer. Drawer shows role-filtered items. Tapping an item navigates to placeholder screen. Logout clears auth.

**Step 7: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/util/RoleAccess.kt \
        android/app/src/main/java/com/kiwari/pos/ui/navigation/AppDrawer.kt \
        android/app/src/main/java/com/kiwari/pos/ui/navigation/NavGraph.kt \
        android/app/src/main/java/com/kiwari/pos/MainActivity.kt \
        android/app/src/main/java/com/kiwari/pos/ui/menu/MenuScreen.kt
git commit -m "feat(android): add drawer navigation with role-based access"
```

---

## Task 2: Report Models + API + Repository

**Goal:** Add the data layer for all 5 report endpoints. No UI yet — just models, API interface, and repository.

**Files:**
- Create: `data/model/Report.kt`
- Create: `data/api/ReportApi.kt`
- Create: `data/repository/ReportRepository.kt`
- Modify: `di/NetworkModule.kt`

**Step 1: Create `data/model/Report.kt`**

```kotlin
package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

// GET /outlets/{oid}/reports/daily-sales
data class DailySalesResponse(
    @SerializedName("date") val date: String,
    @SerializedName("order_count") val orderCount: Long,
    @SerializedName("total_revenue") val totalRevenue: String,
    @SerializedName("total_discount") val totalDiscount: String,
    @SerializedName("net_revenue") val netRevenue: String
)

// GET /outlets/{oid}/reports/product-sales
data class ProductSalesResponse(
    @SerializedName("product_id") val productId: String,
    @SerializedName("product_name") val productName: String,
    @SerializedName("quantity_sold") val quantitySold: Long,
    @SerializedName("total_revenue") val totalRevenue: String
)

// GET /outlets/{oid}/reports/payment-summary
data class PaymentSummaryResponse(
    @SerializedName("payment_method") val paymentMethod: String,
    @SerializedName("transaction_count") val transactionCount: Long,
    @SerializedName("total_amount") val totalAmount: String
)

// GET /outlets/{oid}/reports/hourly-sales
data class HourlySalesResponse(
    @SerializedName("hour") val hour: Int,
    @SerializedName("order_count") val orderCount: Long,
    @SerializedName("total_revenue") val totalRevenue: String
)

// GET /reports/outlet-comparison (owner only)
data class OutletComparisonResponse(
    @SerializedName("outlet_id") val outletId: String,
    @SerializedName("outlet_name") val outletName: String,
    @SerializedName("order_count") val orderCount: Long,
    @SerializedName("total_revenue") val totalRevenue: String
)
```

**Step 2: Create `data/api/ReportApi.kt`**

```kotlin
package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.*
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
```

**Step 3: Create `data/repository/ReportRepository.kt`**

```kotlin
package com.kiwari.pos.data.repository

import com.google.gson.Gson
import com.kiwari.pos.data.api.ReportApi
import com.kiwari.pos.data.model.*
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
```

**Step 4: Register `ReportApi` in `di/NetworkModule.kt`**

Add after `provideOrderApi`:

```kotlin
@Provides
@Singleton
fun provideReportApi(retrofit: Retrofit): ReportApi {
    return retrofit.create(ReportApi::class.java)
}
```

**Step 5: Build and verify**

Run: `cd android && ./gradlew assembleDebug`
Expected: Compiles. No UI yet.

**Step 6: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/data/model/Report.kt \
        android/app/src/main/java/com/kiwari/pos/data/api/ReportApi.kt \
        android/app/src/main/java/com/kiwari/pos/data/repository/ReportRepository.kt \
        android/app/src/main/java/com/kiwari/pos/di/NetworkModule.kt
git commit -m "feat(android): add report data layer (models, API, repository)"
```

---

## Task 3: Reports Screen (Laporan)

**Goal:** Build the reports UI with 3 tabs (Penjualan, Produk, Pembayaran) + date range presets + optional OWNER-only Outlet tab.

**Files:**
- Create: `ui/reports/ReportsScreen.kt`
- Create: `ui/reports/ReportsViewModel.kt`
- Modify: `ui/navigation/NavGraph.kt` (replace placeholder)

**Step 1: Create `ui/reports/ReportsViewModel.kt`**

```kotlin
package com.kiwari.pos.ui.reports

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.*
import com.kiwari.pos.data.repository.ReportRepository
import com.kiwari.pos.data.repository.TokenRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
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
    // Penjualan tab
    val dailySales: List<DailySalesResponse> = emptyList(),
    val hourlySales: List<HourlySalesResponse> = emptyList(),
    val totalRevenue: String = "0",
    val totalOrders: Long = 0,
    val avgTicket: String = "0",
    // Produk tab
    val productSales: List<ProductSalesResponse> = emptyList(),
    // Pembayaran tab
    val paymentSummary: List<PaymentSummaryResponse> = emptyList(),
    // Outlet tab (owner only)
    val outletComparison: List<OutletComparisonResponse> = emptyList()
)

@HiltViewModel
class ReportsViewModel @Inject constructor(
    private val reportRepository: ReportRepository,
    private val tokenRepository: TokenRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(ReportsUiState())
    val uiState: StateFlow<ReportsUiState> = _uiState.asStateFlow()
    private val dateFormatter = DateTimeFormatter.ofPattern("yyyy-MM-dd")

    init {
        val role = tokenRepository.getUserRole()
        _uiState.value = _uiState.value.copy(isOwner = role == "OWNER")
        loadData()
    }

    fun onTabSelected(tab: ReportTab) {
        _uiState.value = _uiState.value.copy(selectedTab = tab)
        loadData()
    }

    fun onDatePresetSelected(preset: DatePreset) {
        val today = LocalDate.now()
        val (start, end) = when (preset) {
            DatePreset.HARI_INI -> today to today
            DatePreset.KEMARIN -> today.minusDays(1) to today.minusDays(1)
            DatePreset.TUJUH_HARI -> today.minusDays(6) to today
            DatePreset.CUSTOM -> return // handled by date picker
        }
        _uiState.value = _uiState.value.copy(
            selectedDatePreset = preset,
            startDate = start,
            endDate = end
        )
        loadData()
    }

    fun onCustomDateRange(start: LocalDate, end: LocalDate) {
        _uiState.value = _uiState.value.copy(
            selectedDatePreset = DatePreset.CUSTOM,
            startDate = start,
            endDate = end
        )
        loadData()
    }

    private fun loadData() {
        val state = _uiState.value
        val startStr = state.startDate.format(dateFormatter)
        val endStr = state.endDate.format(dateFormatter)

        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(isLoading = true, errorMessage = null)
            when (state.selectedTab) {
                ReportTab.PENJUALAN -> loadSalesData(startStr, endStr)
                ReportTab.PRODUK -> loadProductData(startStr, endStr)
                ReportTab.PEMBAYARAN -> loadPaymentData(startStr, endStr)
                ReportTab.OUTLET -> loadOutletData(startStr, endStr)
            }
        }
    }

    private suspend fun loadSalesData(startDate: String, endDate: String) {
        val dailyResult = reportRepository.getDailySales(startDate, endDate)
        val hourlyResult = reportRepository.getHourlySales(startDate, endDate)

        when {
            dailyResult is Result.Error -> {
                _uiState.value = _uiState.value.copy(isLoading = false, errorMessage = dailyResult.message)
            }
            dailyResult is Result.Success -> {
                val sales = dailyResult.data
                val totalRev = sales.sumOf { it.netRevenue.toBigDecimalOrNull() ?: java.math.BigDecimal.ZERO }
                val totalOrd = sales.sumOf { it.orderCount }
                val avg = if (totalOrd > 0) totalRev.divide(totalOrd.toBigDecimal(), 0, java.math.RoundingMode.HALF_UP) else java.math.BigDecimal.ZERO
                val hourly = if (hourlyResult is Result.Success) hourlyResult.data else emptyList()
                _uiState.value = _uiState.value.copy(
                    isLoading = false,
                    dailySales = sales,
                    hourlySales = hourly,
                    totalRevenue = totalRev.toPlainString(),
                    totalOrders = totalOrd,
                    avgTicket = avg.toPlainString()
                )
            }
        }
    }

    private suspend fun loadProductData(startDate: String, endDate: String) {
        when (val result = reportRepository.getProductSales(startDate, endDate)) {
            is Result.Success -> _uiState.value = _uiState.value.copy(isLoading = false, productSales = result.data)
            is Result.Error -> _uiState.value = _uiState.value.copy(isLoading = false, errorMessage = result.message)
        }
    }

    private suspend fun loadPaymentData(startDate: String, endDate: String) {
        when (val result = reportRepository.getPaymentSummary(startDate, endDate)) {
            is Result.Success -> _uiState.value = _uiState.value.copy(isLoading = false, paymentSummary = result.data)
            is Result.Error -> _uiState.value = _uiState.value.copy(isLoading = false, errorMessage = result.message)
        }
    }

    private suspend fun loadOutletData(startDate: String, endDate: String) {
        when (val result = reportRepository.getOutletComparison(startDate, endDate)) {
            is Result.Success -> _uiState.value = _uiState.value.copy(isLoading = false, outletComparison = result.data)
            is Result.Error -> _uiState.value = _uiState.value.copy(isLoading = false, errorMessage = result.message)
        }
    }
}
```

**Step 2: Create `ui/reports/ReportsScreen.kt`**

Build the full screen:
- Top bar with back arrow + "Laporan" title
- Date preset chips row: `Hari ini`, `Kemarin`, `7 Hari`, `Custom ▼`
- Tab row: `Penjualan`, `Produk`, `Pembayaran`, (+ `Outlet` if owner)
- Content area changes per tab

**Penjualan tab layout:**
- 3 KPI cards in a row: Total Penjualan (Rp formatted), Total Pesanan (count), Rata-rata (Rp)
- Hourly sales section: simple horizontal bar chart using Canvas or basic bars

**Produk tab layout:**
- LazyColumn of product rows: rank number, product name, qty sold, revenue

**Pembayaran tab layout:**
- Cards per payment method: method name, transaction count, total amount, percentage of grand total

**Outlet tab (owner only):**
- Cards per outlet: outlet name, order count, revenue

Use `CurrencyFormatter.formatPrice()` for all money display.

For the date range custom picker, use Material3 `DateRangePicker` or `DatePickerDialog`.

```kotlin
// Core composable signature:
@Composable
fun ReportsScreen(
    viewModel: ReportsViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit = {}
)
```

**Step 3: Wire into NavGraph**

Replace the placeholder `composable(Screen.Reports.route)` with:

```kotlin
composable(Screen.Reports.route) {
    ReportsScreen(onNavigateBack = { navController.popBackStack() })
}
```

**Step 4: Build and verify**

Run: `cd android && ./gradlew assembleDebug`
Expected: Build succeeds. Reports screen shows data from API.

**Step 5: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/reports/
git commit -m "feat(android): add reports screen with sales, products, and payment tabs"
```

---

## Task 4: CRM Models + Extended CustomerApi + Repository

**Goal:** Extend the customer data layer with stats, orders, update, and full list (without search query requirement).

**Files:**
- Create: `data/model/CustomerStats.kt`
- Modify: `data/api/CustomerApi.kt`
- Modify: `data/repository/CustomerRepository.kt`

**Step 1: Create `data/model/CustomerStats.kt`**

```kotlin
package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class CustomerStatsResponse(
    @SerializedName("total_orders") val totalOrders: Long,
    @SerializedName("total_spend") val totalSpend: String,
    @SerializedName("avg_ticket") val avgTicket: String,
    @SerializedName("top_items") val topItems: List<TopItemResponse>
)

data class TopItemResponse(
    @SerializedName("product_id") val productId: String,
    @SerializedName("product_name") val productName: String,
    @SerializedName("total_qty") val totalQty: Long,
    @SerializedName("total_revenue") val totalRevenue: String
)

data class UpdateCustomerRequest(
    @SerializedName("name") val name: String,
    @SerializedName("phone") val phone: String,
    @SerializedName("email") val email: String? = null,
    @SerializedName("notes") val notes: String? = null
)

// Customer orders response — reuses the order types already defined
// GET /customers/{id}/orders returns an array of order summaries
// The Go API returns []orderResponse which maps to OrderDetailResponse
// but without payments in the customer context, use a simpler type
data class CustomerOrderResponse(
    @SerializedName("id") val id: String,
    @SerializedName("order_number") val orderNumber: String,
    @SerializedName("order_type") val orderType: String,
    @SerializedName("status") val status: String,
    @SerializedName("total_amount") val totalAmount: String,
    @SerializedName("created_at") val createdAt: String
)
```

**Step 2: Modify `data/api/CustomerApi.kt`**

Add the new endpoints:

```kotlin
@GET("outlets/{outletId}/customers")
suspend fun listCustomers(
    @Path("outletId") outletId: String,
    @Query("search") search: String? = null,
    @Query("limit") limit: Int = 100,
    @Query("offset") offset: Int = 0
): Response<List<Customer>>

@GET("outlets/{outletId}/customers/{customerId}")
suspend fun getCustomer(
    @Path("outletId") outletId: String,
    @Path("customerId") customerId: String
): Response<Customer>

@PUT("outlets/{outletId}/customers/{customerId}")
suspend fun updateCustomer(
    @Path("outletId") outletId: String,
    @Path("customerId") customerId: String,
    @Body request: UpdateCustomerRequest
): Response<Customer>

@GET("outlets/{outletId}/customers/{customerId}/stats")
suspend fun getCustomerStats(
    @Path("outletId") outletId: String,
    @Path("customerId") customerId: String
): Response<CustomerStatsResponse>

@GET("outlets/{outletId}/customers/{customerId}/orders")
suspend fun getCustomerOrders(
    @Path("outletId") outletId: String,
    @Path("customerId") customerId: String,
    @Query("limit") limit: Int = 20,
    @Query("offset") offset: Int = 0
): Response<List<CustomerOrderResponse>>
```

**Step 3: Extend `data/repository/CustomerRepository.kt`**

Add methods:

```kotlin
suspend fun listCustomers(): Result<List<Customer>> {
    val outletId = tokenRepository.getOutletId()
        ?: return Result.Error("No outlet selected")
    return safeApiCall(gson) { customerApi.listCustomers(outletId) }
}

suspend fun getCustomer(customerId: String): Result<Customer> {
    val outletId = tokenRepository.getOutletId()
        ?: return Result.Error("No outlet selected")
    return safeApiCall(gson) { customerApi.getCustomer(outletId, customerId) }
}

suspend fun updateCustomer(customerId: String, name: String, phone: String, email: String?, notes: String?): Result<Customer> {
    val outletId = tokenRepository.getOutletId()
        ?: return Result.Error("No outlet selected")
    return safeApiCall(gson) {
        customerApi.updateCustomer(outletId, customerId, UpdateCustomerRequest(name, phone, email, notes))
    }
}

suspend fun getCustomerStats(customerId: String): Result<CustomerStatsResponse> {
    val outletId = tokenRepository.getOutletId()
        ?: return Result.Error("No outlet selected")
    return safeApiCall(gson) { customerApi.getCustomerStats(outletId, customerId) }
}

suspend fun getCustomerOrders(customerId: String): Result<List<CustomerOrderResponse>> {
    val outletId = tokenRepository.getOutletId()
        ?: return Result.Error("No outlet selected")
    return safeApiCall(gson) { customerApi.getCustomerOrders(outletId, customerId) }
}
```

**Step 4: Build and verify**

Run: `cd android && ./gradlew assembleDebug`
Expected: Compiles.

**Step 5: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/data/model/CustomerStats.kt \
        android/app/src/main/java/com/kiwari/pos/data/api/CustomerApi.kt \
        android/app/src/main/java/com/kiwari/pos/data/repository/CustomerRepository.kt
git commit -m "feat(android): extend customer data layer with stats, orders, and update"
```

---

## Task 5: CRM Screens (Pelanggan)

**Goal:** Build customer list screen (with search + sort chips) and customer detail screen (stats + favorites + order history).

**Files:**
- Create: `ui/customers/CustomerListScreen.kt`
- Create: `ui/customers/CustomerListViewModel.kt`
- Create: `ui/customers/CustomerDetailScreen.kt`
- Create: `ui/customers/CustomerDetailViewModel.kt`
- Modify: `ui/navigation/NavGraph.kt`

**Step 1: Create `ui/customers/CustomerListViewModel.kt`**

```kotlin
package com.kiwari.pos.ui.customers

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.model.*
import com.kiwari.pos.data.repository.CustomerRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

enum class CustomerSort { SEMUA, TERBANYAK, TERBARU }

data class CustomerListUiState(
    val isLoading: Boolean = true,
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

    init { loadCustomers() }

    fun loadCustomers() {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(isLoading = true, errorMessage = null)
            when (val result = customerRepository.listCustomers()) {
                is Result.Success -> {
                    _uiState.value = _uiState.value.copy(isLoading = false, customers = result.data)
                    applyFilters()
                }
                is Result.Error -> _uiState.value = _uiState.value.copy(isLoading = false, errorMessage = result.message)
            }
        }
    }

    fun onSearchQueryChanged(query: String) {
        _uiState.value = _uiState.value.copy(searchQuery = query)
        applyFilters()
    }

    fun onSortSelected(sort: CustomerSort) {
        _uiState.value = _uiState.value.copy(selectedSort = sort)
        applyFilters()
    }

    fun showCreateDialog() {
        _uiState.value = _uiState.value.copy(showCreateDialog = true, createError = null)
    }

    fun dismissCreateDialog() {
        _uiState.value = _uiState.value.copy(showCreateDialog = false, createError = null)
    }

    fun createCustomer(name: String, phone: String) {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(isCreating = true, createError = null)
            when (val result = customerRepository.createCustomer(name, phone)) {
                is Result.Success -> {
                    _uiState.value = _uiState.value.copy(isCreating = false, showCreateDialog = false)
                    loadCustomers()
                }
                is Result.Error -> _uiState.value = _uiState.value.copy(isCreating = false, createError = result.message)
            }
        }
    }

    private fun applyFilters() {
        val state = _uiState.value
        var list = state.customers

        // Search filter
        if (state.searchQuery.isNotBlank()) {
            val q = state.searchQuery.lowercase()
            list = list.filter {
                it.name.lowercase().contains(q) || it.phone.contains(q)
            }
        }

        // Sort
        list = when (state.selectedSort) {
            CustomerSort.SEMUA -> list.sortedBy { it.name.lowercase() }
            CustomerSort.TERBANYAK -> list // client-side — no spend data on list, so same as SEMUA for v1
            CustomerSort.TERBARU -> list.sortedByDescending { it.createdAt }
        }

        _uiState.value = _uiState.value.copy(filteredCustomers = list)
    }
}
```

**Step 2: Create `ui/customers/CustomerListScreen.kt`**

Full-screen composable:
- Top bar: back arrow + "Pelanggan" + [+] FAB
- Search bar: `OutlinedTextField` with search icon
- Sort chips: `Semua`, `Terbanyak`, `Terbaru`
- `LazyColumn` of customer cards: name, phone, order count placeholder
- Tap → navigate to detail
- [+] → show create dialog (name + phone fields)

```kotlin
@Composable
fun CustomerListScreen(
    viewModel: CustomerListViewModel = hiltViewModel(),
    onCustomerClick: (customerId: String) -> Unit = {},
    onNavigateBack: () -> Unit = {}
)
```

**Step 3: Create `ui/customers/CustomerDetailViewModel.kt`**

Loads customer info, stats, and orders in parallel.

```kotlin
data class CustomerDetailUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val customer: Customer? = null,
    val stats: CustomerStatsResponse? = null,
    val orders: List<CustomerOrderResponse> = emptyList(),
    val showEditDialog: Boolean = false,
    val isUpdating: Boolean = false,
    val updateError: String? = null
)
```

Key methods: `loadCustomer(id)`, `showEditDialog()`, `updateCustomer(name, phone, email, notes)`.

Use `SavedStateHandle` to get `customerId` from nav args (same pattern as `OrderDetailViewModel`).

**Step 4: Create `ui/customers/CustomerDetailScreen.kt`**

- Top bar: back arrow + customer name + [edit] icon
- Phone number below name
- 3 KPI stat cards: Pesanan (count), Total (spend), Rata² (avg ticket)
- "Menu Favorit" section: numbered list of top items from stats
- "Riwayat Pesanan" section: LazyColumn of order rows, tap → OrderDetail
- Edit dialog: name, phone, email, notes fields

```kotlin
@Composable
fun CustomerDetailScreen(
    viewModel: CustomerDetailViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit = {},
    onOrderClick: (orderId: String) -> Unit = {}
)
```

**Step 5: Add routes to NavGraph**

```kotlin
object CustomerDetail : Screen("customer/{customerId}") {
    fun createRoute(customerId: String) = "customer/$customerId"
}
```

Replace `CustomerList` placeholder and add `CustomerDetail` composable:

```kotlin
composable(Screen.CustomerList.route) {
    CustomerListScreen(
        onCustomerClick = { id -> navController.navigate(Screen.CustomerDetail.createRoute(id)) },
        onNavigateBack = { navController.popBackStack() }
    )
}
composable(
    route = Screen.CustomerDetail.route,
    arguments = listOf(navArgument("customerId") { type = NavType.StringType })
) {
    CustomerDetailScreen(
        onNavigateBack = { navController.popBackStack() },
        onOrderClick = { orderId -> navController.navigate(Screen.OrderDetail.createRoute(orderId)) }
    )
}
```

**Step 6: Build and verify**

Run: `cd android && ./gradlew assembleDebug`

**Step 7: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/customers/
git commit -m "feat(android): add customer list and detail screens with stats"
```

---

## Task 6: Order History (Riwayat Tab)

**Goal:** Add a "Riwayat" tab to the existing OrderListScreen showing COMPLETED/CANCELLED orders with date range filtering.

**Files:**
- Modify: `data/api/OrderApi.kt`
- Modify: `data/repository/OrderRepository.kt`
- Modify: `ui/orders/OrderListScreen.kt`
- Modify: `ui/orders/OrderListViewModel.kt`

**Step 1: Extend `OrderApi.kt`**

Add an endpoint for listing orders with status + date filters:

```kotlin
@GET("outlets/{outletId}/orders")
suspend fun listOrders(
    @Path("outletId") outletId: String,
    @Query("status") status: String? = null,
    @Query("start_date") startDate: String? = null,
    @Query("end_date") endDate: String? = null,
    @Query("limit") limit: Int = 50,
    @Query("offset") offset: Int = 0
): Response<OrdersListResponse>
```

Add a response model in `Order.kt`:

```kotlin
data class OrdersListResponse(
    @SerializedName("orders") val orders: List<OrderSummaryResponse>,
    @SerializedName("limit") val limit: Int,
    @SerializedName("offset") val offset: Int
)

data class OrderSummaryResponse(
    @SerializedName("id") val id: String,
    @SerializedName("order_number") val orderNumber: String,
    @SerializedName("customer_id") val customerId: String?,
    @SerializedName("order_type") val orderType: String,
    @SerializedName("status") val status: String,
    @SerializedName("table_number") val tableNumber: String?,
    @SerializedName("total_amount") val totalAmount: String,
    @SerializedName("created_at") val createdAt: String
)
```

**Step 2: Extend `OrderRepository.kt`**

```kotlin
suspend fun listOrders(
    status: String? = null,
    startDate: String? = null,
    endDate: String? = null
): Result<OrdersListResponse> {
    val outletId = tokenRepository.getOutletId()
        ?: return Result.Error("No outlet selected")
    return safeApiCall(gson) {
        orderApi.listOrders(outletId, status, startDate, endDate)
    }
}
```

**Step 3: Modify `OrderListViewModel.kt`**

Add:
- `OrderListTab` enum: `AKTIF`, `RIWAYAT`
- `selectedTab: OrderListTab` to state
- `historyOrders: List<OrderSummaryResponse>` to state
- Date range state: `historyStartDate`, `historyEndDate`, `historyDatePreset`
- Search state: `historySearchQuery`
- `isManager` or `isOwner` flag (from `TokenRepository`) to control tab visibility
- `loadHistory()` method that calls `orderRepository.listOrders(status = "COMPLETED")` + merges CANCELLED

When Riwayat tab selected, load historical orders. Client-side filtering for search and status within loaded data.

**Step 4: Modify `OrderListScreen.kt`**

Add:
- Two-tab row at top: `Aktif` / `Riwayat` (Riwayat only visible for OWNER/MANAGER)
- Below tabs (when Riwayat selected):
  - Search bar for order number/customer
  - Date preset chips (same as reports)
  - LazyColumn of order cards (reuse similar card layout but show COMPLETED/CANCELLED status badges)
- Tap → navigate to OrderDetail (read-only — the existing OrderDetailScreen already hides edit/pay for completed orders)

**Step 5: Build and verify**

Run: `cd android && ./gradlew assembleDebug`

**Step 6: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/data/api/OrderApi.kt \
        android/app/src/main/java/com/kiwari/pos/data/model/Order.kt \
        android/app/src/main/java/com/kiwari/pos/data/repository/OrderRepository.kt \
        android/app/src/main/java/com/kiwari/pos/ui/orders/OrderListScreen.kt \
        android/app/src/main/java/com/kiwari/pos/ui/orders/OrderListViewModel.kt
git commit -m "feat(android): add order history tab with date range filtering"
```

---

## Task 7: Menu Admin — Data Layer (Extended MenuApi + MenuAdminRepository)

**Goal:** Add CRUD endpoints for categories, products, variant groups, variants, modifier groups, modifiers, and combo items.

**Files:**
- Create: `data/api/MenuAdminApi.kt`
- Create: `data/model/MenuAdmin.kt`
- Create: `data/repository/MenuAdminRepository.kt`
- Modify: `di/NetworkModule.kt`

**Step 1: Create `data/model/MenuAdmin.kt`**

Request models for all CRUD operations:

```kotlin
package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

// ── Category CRUD ──
data class CreateCategoryRequest(
    @SerializedName("name") val name: String,
    @SerializedName("description") val description: String = "",
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateCategoryRequest(
    @SerializedName("name") val name: String,
    @SerializedName("description") val description: String = "",
    @SerializedName("sort_order") val sortOrder: Int
)

// ── Product CRUD ──
data class CreateProductRequest(
    @SerializedName("category_id") val categoryId: String,
    @SerializedName("name") val name: String,
    @SerializedName("description") val description: String = "",
    @SerializedName("base_price") val basePrice: String,
    @SerializedName("image_url") val imageUrl: String = "",
    @SerializedName("station") val station: String = "",
    @SerializedName("preparation_time") val preparationTime: Int? = null,
    @SerializedName("is_combo") val isCombo: Boolean = false
)

data class UpdateProductRequest(
    @SerializedName("category_id") val categoryId: String,
    @SerializedName("name") val name: String,
    @SerializedName("description") val description: String = "",
    @SerializedName("base_price") val basePrice: String,
    @SerializedName("image_url") val imageUrl: String = "",
    @SerializedName("station") val station: String = "",
    @SerializedName("preparation_time") val preparationTime: Int? = null,
    @SerializedName("is_combo") val isCombo: Boolean = false
)

// ── Variant Group CRUD ──
data class CreateVariantGroupRequest(
    @SerializedName("name") val name: String,
    @SerializedName("is_required") val isRequired: Boolean,
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateVariantGroupRequest(
    @SerializedName("name") val name: String,
    @SerializedName("is_required") val isRequired: Boolean,
    @SerializedName("sort_order") val sortOrder: Int
)

// ── Variant CRUD ──
data class CreateVariantRequest(
    @SerializedName("name") val name: String,
    @SerializedName("price_adjustment") val priceAdjustment: String,
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateVariantRequest(
    @SerializedName("name") val name: String,
    @SerializedName("price_adjustment") val priceAdjustment: String,
    @SerializedName("sort_order") val sortOrder: Int
)

// ── Modifier Group CRUD ──
data class CreateModifierGroupRequest(
    @SerializedName("name") val name: String,
    @SerializedName("min_select") val minSelect: Int,
    @SerializedName("max_select") val maxSelect: Int,
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateModifierGroupRequest(
    @SerializedName("name") val name: String,
    @SerializedName("min_select") val minSelect: Int,
    @SerializedName("max_select") val maxSelect: Int,
    @SerializedName("sort_order") val sortOrder: Int
)

// ── Modifier CRUD ──
data class CreateModifierRequest(
    @SerializedName("name") val name: String,
    @SerializedName("price") val price: String,
    @SerializedName("sort_order") val sortOrder: Int
)

data class UpdateModifierRequest(
    @SerializedName("name") val name: String,
    @SerializedName("price") val price: String,
    @SerializedName("sort_order") val sortOrder: Int
)

// ── Combo Item CRUD ──
data class ComboItem(
    @SerializedName("id") val id: String,
    @SerializedName("combo_id") val comboId: String,
    @SerializedName("product_id") val productId: String,
    @SerializedName("quantity") val quantity: Int,
    @SerializedName("sort_order") val sortOrder: Int
)

data class CreateComboItemRequest(
    @SerializedName("product_id") val productId: String,
    @SerializedName("quantity") val quantity: Int,
    @SerializedName("sort_order") val sortOrder: Int
)
```

**Step 2: Create `data/api/MenuAdminApi.kt`**

Separate from the read-only `MenuApi` to keep things clean:

```kotlin
package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.*
import retrofit2.Response
import retrofit2.http.*

interface MenuAdminApi {
    // ── Categories ──
    @POST("outlets/{outletId}/categories")
    suspend fun createCategory(
        @Path("outletId") outletId: String,
        @Body request: CreateCategoryRequest
    ): Response<Category>

    @PUT("outlets/{outletId}/categories/{categoryId}")
    suspend fun updateCategory(
        @Path("outletId") outletId: String,
        @Path("categoryId") categoryId: String,
        @Body request: UpdateCategoryRequest
    ): Response<Category>

    @DELETE("outlets/{outletId}/categories/{categoryId}")
    suspend fun deleteCategory(
        @Path("outletId") outletId: String,
        @Path("categoryId") categoryId: String
    ): Response<Unit>

    // ── Products ──
    @POST("outlets/{outletId}/products")
    suspend fun createProduct(
        @Path("outletId") outletId: String,
        @Body request: CreateProductRequest
    ): Response<Product>

    @PUT("outlets/{outletId}/products/{productId}")
    suspend fun updateProduct(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Body request: UpdateProductRequest
    ): Response<Product>

    @DELETE("outlets/{outletId}/products/{productId}")
    suspend fun deleteProduct(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String
    ): Response<Unit>

    // ── Variant Groups ──
    @POST("outlets/{outletId}/products/{productId}/variant-groups")
    suspend fun createVariantGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Body request: CreateVariantGroupRequest
    ): Response<VariantGroup>

    @PUT("outlets/{outletId}/products/{productId}/variant-groups/{groupId}")
    suspend fun updateVariantGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Body request: UpdateVariantGroupRequest
    ): Response<VariantGroup>

    @DELETE("outlets/{outletId}/products/{productId}/variant-groups/{groupId}")
    suspend fun deleteVariantGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String
    ): Response<Unit>

    // ── Variants ──
    @POST("outlets/{outletId}/products/{productId}/variant-groups/{groupId}/variants")
    suspend fun createVariant(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Body request: CreateVariantRequest
    ): Response<Variant>

    @PUT("outlets/{outletId}/products/{productId}/variant-groups/{groupId}/variants/{variantId}")
    suspend fun updateVariant(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Path("variantId") variantId: String,
        @Body request: UpdateVariantRequest
    ): Response<Variant>

    @DELETE("outlets/{outletId}/products/{productId}/variant-groups/{groupId}/variants/{variantId}")
    suspend fun deleteVariant(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Path("variantId") variantId: String
    ): Response<Unit>

    // ── Modifier Groups ──
    @POST("outlets/{outletId}/products/{productId}/modifier-groups")
    suspend fun createModifierGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Body request: CreateModifierGroupRequest
    ): Response<ModifierGroup>

    @PUT("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}")
    suspend fun updateModifierGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Body request: UpdateModifierGroupRequest
    ): Response<ModifierGroup>

    @DELETE("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}")
    suspend fun deleteModifierGroup(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String
    ): Response<Unit>

    // ── Modifiers ──
    @POST("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}/modifiers")
    suspend fun createModifier(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Body request: CreateModifierRequest
    ): Response<Modifier>

    @PUT("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}/modifiers/{modifierId}")
    suspend fun updateModifier(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Path("modifierId") modifierId: String,
        @Body request: UpdateModifierRequest
    ): Response<Modifier>

    @DELETE("outlets/{outletId}/products/{productId}/modifier-groups/{groupId}/modifiers/{modifierId}")
    suspend fun deleteModifier(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("groupId") groupId: String,
        @Path("modifierId") modifierId: String
    ): Response<Unit>

    // ── Combo Items ──
    @GET("outlets/{outletId}/products/{productId}/combo-items")
    suspend fun getComboItems(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String
    ): Response<List<ComboItem>>

    @POST("outlets/{outletId}/products/{productId}/combo-items")
    suspend fun addComboItem(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Body request: CreateComboItemRequest
    ): Response<ComboItem>

    @DELETE("outlets/{outletId}/products/{productId}/combo-items/{comboItemId}")
    suspend fun removeComboItem(
        @Path("outletId") outletId: String,
        @Path("productId") productId: String,
        @Path("comboItemId") comboItemId: String
    ): Response<Unit>
}
```

**Step 3: Create `data/repository/MenuAdminRepository.kt`**

Follows the same pattern as `MenuRepository` but for write operations. Each method extracts `outletId` from `TokenRepository` and calls `safeApiCall`. Group methods by entity (categories, products, variant groups, etc.).

**Step 4: Register in `di/NetworkModule.kt`**

```kotlin
@Provides
@Singleton
fun provideMenuAdminApi(retrofit: Retrofit): MenuAdminApi {
    return retrofit.create(MenuAdminApi::class.java)
}
```

**Step 5: Build and verify**

Run: `cd android && ./gradlew assembleDebug`

**Step 6: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/data/model/MenuAdmin.kt \
        android/app/src/main/java/com/kiwari/pos/data/api/MenuAdminApi.kt \
        android/app/src/main/java/com/kiwari/pos/data/repository/MenuAdminRepository.kt \
        android/app/src/main/java/com/kiwari/pos/di/NetworkModule.kt
git commit -m "feat(android): add menu admin data layer (full CRUD for categories, products, variants, modifiers, combos)"
```

---

## Task 8: Menu Admin — Category List Screen

**Goal:** Build the category list screen with create, edit, reorder, and deactivate.

**Files:**
- Create: `ui/menuadmin/CategoryListScreen.kt`
- Create: `ui/menuadmin/CategoryListViewModel.kt`
- Modify: `ui/navigation/NavGraph.kt`

**Step 1: Create `ui/menuadmin/CategoryListViewModel.kt`**

```kotlin
data class CategoryListUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val categories: List<Category> = emptyList(),
    // Dialog state
    val showCreateDialog: Boolean = false,
    val editingCategory: Category? = null,
    val isSaving: Boolean = false,
    val saveError: String? = null
)
```

Key methods:
- `loadCategories()` — uses existing `MenuRepository.getCategories()`
- `createCategory(name, description)` — uses `MenuAdminRepository`
- `updateCategory(id, name, description, sortOrder)` — uses `MenuAdminRepository`
- `deleteCategory(id)` — soft delete via `MenuAdminRepository`
- `reorderCategories(fromIndex, toIndex)` — update `sort_order` via sequential API calls

**Step 2: Create `ui/menuadmin/CategoryListScreen.kt`**

- Top bar: back arrow + "Kelola Menu" + [+] icon button
- LazyColumn of category cards:
  - Category name + product count (not available from API, show "—" for v1)
  - Drag handle (≡) on left, edit icon on right
  - Inactive categories shown with lower opacity + "Nonaktif" badge
- [+] → dialog with name + description fields
- [edit] → dialog with name + description + toggle active
- Tap category → navigate to ProductListScreen for that category

For drag-to-reorder, use a simple long-press + move approach or a library. For v1, a simpler approach is up/down arrow buttons instead of drag-and-drop (much simpler to implement on phone and avoids needing `org.burnoutcrew.reorderable` dependency).

```kotlin
@Composable
fun CategoryListScreen(
    viewModel: CategoryListViewModel = hiltViewModel(),
    onCategoryClick: (categoryId: String, categoryName: String) -> Unit = { _, _ -> },
    onNavigateBack: () -> Unit = {}
)
```

**Step 3: Wire into NavGraph**

Replace the `MenuAdmin` placeholder:

```kotlin
composable(Screen.MenuAdmin.route) {
    CategoryListScreen(
        onCategoryClick = { id, name ->
            navController.navigate(Screen.ProductList.createRoute(id, name))
        },
        onNavigateBack = { navController.popBackStack() }
    )
}
```

Add new route:

```kotlin
object ProductList : Screen("menu-admin/category/{categoryId}/{categoryName}") {
    fun createRoute(categoryId: String, categoryName: String) =
        "menu-admin/category/$categoryId/${java.net.URLEncoder.encode(categoryName, "UTF-8")}"
}
```

**Step 4: Build and verify**

Run: `cd android && ./gradlew assembleDebug`

**Step 5: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/menuadmin/CategoryListScreen.kt \
        android/app/src/main/java/com/kiwari/pos/ui/menuadmin/CategoryListViewModel.kt \
        android/app/src/main/java/com/kiwari/pos/ui/navigation/NavGraph.kt
git commit -m "feat(android): add category list screen with CRUD"
```

---

## Task 9: Menu Admin — Product List + Product Detail (Hub Screen)

**Goal:** Build the product list within a category, and the product detail hub screen with collapsible variant/modifier/combo sections and bottom sheet editing.

**Files:**
- Create: `ui/menuadmin/ProductListScreen.kt`
- Create: `ui/menuadmin/ProductListViewModel.kt`
- Create: `ui/menuadmin/ProductDetailScreen.kt`
- Create: `ui/menuadmin/ProductDetailViewModel.kt`
- Modify: `ui/navigation/NavGraph.kt`

**Step 1: Create `ProductListViewModel.kt`**

Uses `SavedStateHandle` to get `categoryId` from nav args.

```kotlin
data class ProductListUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val categoryId: String = "",
    val categoryName: String = "",
    val products: List<Product> = emptyList(),
    val variantGroupCounts: Map<String, Int> = emptyMap(),
    val modifierGroupCounts: Map<String, Int> = emptyMap(),
    // Quick price edit
    val showPriceEditSheet: Boolean = false,
    val editingProduct: Product? = null,
    val isSaving: Boolean = false,
    val saveError: String? = null
)
```

- `loadProducts()` — filter `MenuRepository.getProducts()` by `categoryId`
- For each product, load variant group count and modifier group count (in parallel)
- Quick price edit bottom sheet: just a price field + save button

**Step 2: Create `ProductListScreen.kt`**

- Top bar: back arrow + category name + [+] icon button
- LazyColumn of product cards: name, base price, station badge, variant count badge, modifier count badge
- [edit icon] → quick price edit bottom sheet
- Tap product → navigate to ProductDetail
- [+] → navigate to ProductDetail (create mode, empty)

**Step 3: Create `ProductDetailViewModel.kt`**

The most complex ViewModel — manages the product form state and all child entities.

```kotlin
data class ProductDetailUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val isCreateMode: Boolean = true,
    val productId: String? = null,
    // Form fields
    val name: String = "",
    val basePrice: String = "",
    val categoryId: String = "",
    val station: String = "",
    val description: String = "",
    val preparationTime: String = "",
    val isCombo: Boolean = false,
    val isActive: Boolean = true,
    // Child entities
    val variantGroups: List<VariantGroupWithVariants> = emptyList(),
    val modifierGroups: List<ModifierGroupWithModifiers> = emptyList(),
    val comboItems: List<ComboItem> = emptyList(),
    // Bottom sheet state
    val activeSheet: ProductSheet? = null,
    val isSaving: Boolean = false,
    val saveError: String? = null,
    // Category list for dropdown
    val categories: List<Category> = emptyList(),
    val stations: List<String> = listOf("GRILL", "BEVERAGE", "RICE", "DESSERT")
)

data class VariantGroupWithVariants(
    val group: VariantGroup,
    val variants: List<Variant>
)

data class ModifierGroupWithModifiers(
    val group: ModifierGroup,
    val modifiers: List<Modifier>
)

sealed class ProductSheet {
    data class EditVariantGroup(val group: VariantGroup?) : ProductSheet()
    data class EditVariant(val groupId: String, val variant: Variant?) : ProductSheet()
    data class EditModifierGroup(val group: ModifierGroup?) : ProductSheet()
    data class EditModifier(val groupId: String, val modifier: Modifier?) : ProductSheet()
    object AddComboItem : ProductSheet()
}
```

Key methods:
- `loadProduct(productId)` — load product + all children in parallel
- `saveProduct()` — create or update product
- `deactivateProduct()` — soft delete
- Variant group/variant CRUD methods
- Modifier group/modifier CRUD methods
- Combo item add/remove methods
- Each bottom sheet has save/delete handlers

**Step 4: Create `ProductDetailScreen.kt`**

The hub screen from the design doc:
- Top bar: back arrow + product name (or "Produk Baru") + [⋮] menu (deactivate option)
- Scrollable form:
  - Name field
  - Base price field
  - Category dropdown
  - Station dropdown
  - Description field
  - Preparation time field
  - Collapsible "Varian" section with [+] button
  - Collapsible "Modifier" section with [+] button
  - Collapsible "Combo Items" section with [+] button (only if isCombo)
- Bottom: SIMPAN button + "Nonaktifkan Produk" text button (edit mode only)

Bottom sheets for sub-editing:
- `VariantGroupSheet`: name field + required checkbox + delete button
- `VariantSheet`: name field + price adjustment field + delete button
- `ModifierGroupSheet`: name field + min/max select fields + delete button
- `ModifierSheet`: name field + price field + delete button
- `ComboItemSheet`: product picker (searchable) + quantity

Use `ModalBottomSheet` from Material3.

**Step 5: Wire into NavGraph**

```kotlin
object ProductDetail : Screen("menu-admin/product/{productId}") {
    fun createRoute(productId: String) = "menu-admin/product/$productId"
    fun createNewRoute(categoryId: String) = "menu-admin/product/new?categoryId=$categoryId"
}
```

Add composables for ProductList and ProductDetail routes.

**Step 6: Build and verify**

Run: `cd android && ./gradlew assembleDebug`

**Step 7: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/menuadmin/ProductListScreen.kt \
        android/app/src/main/java/com/kiwari/pos/ui/menuadmin/ProductListViewModel.kt \
        android/app/src/main/java/com/kiwari/pos/ui/menuadmin/ProductDetailScreen.kt \
        android/app/src/main/java/com/kiwari/pos/ui/menuadmin/ProductDetailViewModel.kt \
        android/app/src/main/java/com/kiwari/pos/ui/navigation/NavGraph.kt
git commit -m "feat(android): add product list and detail hub with variant/modifier/combo management"
```

---

## Task 10: Staff Management (Pengguna)

**Goal:** Build staff list and create/edit form screens.

**Files:**
- Create: `data/api/UserApi.kt`
- Create: `data/model/UserAdmin.kt`
- Create: `data/repository/UserRepository.kt`
- Modify: `di/NetworkModule.kt`
- Create: `ui/staff/StaffListScreen.kt`
- Create: `ui/staff/StaffListViewModel.kt`
- Create: `ui/staff/StaffFormScreen.kt`
- Create: `ui/staff/StaffFormViewModel.kt`
- Modify: `ui/navigation/NavGraph.kt`

**Step 1: Create `data/model/UserAdmin.kt`**

```kotlin
package com.kiwari.pos.data.model

import com.google.gson.annotations.SerializedName

data class StaffMember(
    @SerializedName("id") val id: String,
    @SerializedName("outlet_id") val outletId: String,
    @SerializedName("email") val email: String,
    @SerializedName("full_name") val fullName: String,
    @SerializedName("role") val role: String,
    @SerializedName("pin") val pin: String?,
    @SerializedName("is_active") val isActive: Boolean,
    @SerializedName("created_at") val createdAt: String,
    @SerializedName("updated_at") val updatedAt: String
)

data class CreateUserRequest(
    @SerializedName("email") val email: String,
    @SerializedName("password") val password: String,
    @SerializedName("full_name") val fullName: String,
    @SerializedName("role") val role: String,
    @SerializedName("pin") val pin: String
)

data class UpdateUserRequest(
    @SerializedName("email") val email: String,
    @SerializedName("full_name") val fullName: String,
    @SerializedName("role") val role: String,
    @SerializedName("pin") val pin: String
)
```

**Step 2: Create `data/api/UserApi.kt`**

```kotlin
package com.kiwari.pos.data.api

import com.kiwari.pos.data.model.*
import retrofit2.Response
import retrofit2.http.*

interface UserApi {
    @GET("outlets/{outletId}/users")
    suspend fun listUsers(
        @Path("outletId") outletId: String
    ): Response<List<StaffMember>>

    @POST("outlets/{outletId}/users")
    suspend fun createUser(
        @Path("outletId") outletId: String,
        @Body request: CreateUserRequest
    ): Response<StaffMember>

    @PUT("outlets/{outletId}/users/{userId}")
    suspend fun updateUser(
        @Path("outletId") outletId: String,
        @Path("userId") userId: String,
        @Body request: UpdateUserRequest
    ): Response<StaffMember>

    @DELETE("outlets/{outletId}/users/{userId}")
    suspend fun deleteUser(
        @Path("outletId") outletId: String,
        @Path("userId") userId: String
    ): Response<Unit>
}
```

**Step 3: Create `data/repository/UserRepository.kt`**

```kotlin
@Singleton
class UserRepository @Inject constructor(
    private val userApi: UserApi,
    private val tokenRepository: TokenRepository,
    private val gson: Gson
) {
    suspend fun listUsers(): Result<List<StaffMember>> { ... }
    suspend fun createUser(request: CreateUserRequest): Result<StaffMember> { ... }
    suspend fun updateUser(userId: String, request: UpdateUserRequest): Result<StaffMember> { ... }
    suspend fun deleteUser(userId: String): Result<Unit> { ... }
}
```

Same pattern — extract outletId from TokenRepository, safeApiCall wrapper.

**Step 4: Register in `di/NetworkModule.kt`**

```kotlin
@Provides
@Singleton
fun provideUserApi(retrofit: Retrofit): UserApi {
    return retrofit.create(UserApi::class.java)
}
```

**Step 5: Create `ui/staff/StaffListViewModel.kt`**

```kotlin
data class StaffListUiState(
    val isLoading: Boolean = true,
    val errorMessage: String? = null,
    val staff: List<StaffMember> = emptyList(),
    val currentUserRole: UserRole = UserRole.MANAGER
)
```

- `loadStaff()` — list all users
- Navigate to form screen for create/edit

**Step 6: Create `ui/staff/StaffListScreen.kt`**

- Top bar: back arrow + "Pengguna" + [+] button
- LazyColumn of staff cards: name, role badge, email
- Tap → navigate to edit form
- [+] → navigate to create form

**Step 7: Create `ui/staff/StaffFormViewModel.kt`**

```kotlin
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
    val currentUserRole: UserRole = UserRole.MANAGER,
    // For deactivation
    val showDeactivateDialog: Boolean = false,
    val isDeactivating: Boolean = false
)
```

- `loadUser(userId)` — for edit mode (via SavedStateHandle)
- `saveUser()` — create or update
- `deactivateUser()` — soft delete
- Validation: email must contain @, PIN 4-6 digits, name required

**Step 8: Create `ui/staff/StaffFormScreen.kt`**

- Top bar: back arrow + "Tambah Pengguna" or "Edit Pengguna"
- Form fields: Full Name, Email, Password (create only), PIN (4-6 digits)
- Role selector: 4 radio chips (CASHIER, KITCHEN, MANAGER, OWNER — OWNER only visible if current user is OWNER)
- Bottom: SIMPAN button
- (edit only): "Nonaktifkan Pengguna" text button + confirmation dialog

**Step 9: Wire into NavGraph**

```kotlin
object StaffForm : Screen("staff/{userId}") {
    fun createRoute(userId: String) = "staff/$userId"
    fun createNewRoute() = "staff/new"
}
```

Add composables:

```kotlin
composable(Screen.StaffList.route) {
    StaffListScreen(
        onStaffClick = { id -> navController.navigate(Screen.StaffForm.createRoute(id)) },
        onCreateClick = { navController.navigate(Screen.StaffForm.createNewRoute()) },
        onNavigateBack = { navController.popBackStack() }
    )
}
composable(
    route = Screen.StaffForm.route,
    arguments = listOf(navArgument("userId") { type = NavType.StringType })
) {
    StaffFormScreen(
        onNavigateBack = { navController.popBackStack() },
        onSaveSuccess = { navController.popBackStack() }
    )
}
```

**Step 10: Build and verify**

Run: `cd android && ./gradlew assembleDebug`

**Step 11: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/data/model/UserAdmin.kt \
        android/app/src/main/java/com/kiwari/pos/data/api/UserApi.kt \
        android/app/src/main/java/com/kiwari/pos/data/repository/UserRepository.kt \
        android/app/src/main/java/com/kiwari/pos/di/NetworkModule.kt \
        android/app/src/main/java/com/kiwari/pos/ui/staff/ \
        android/app/src/main/java/com/kiwari/pos/ui/navigation/NavGraph.kt
git commit -m "feat(android): add staff management screens (list, create, edit, deactivate)"
```

---

## Task 11: Integration Wiring + Final NavGraph Cleanup

**Goal:** Ensure all screens are properly wired, drawer navigation works end-to-end, and role checks are enforced on each screen.

**Files:**
- Modify: `ui/navigation/NavGraph.kt` (final pass)
- Modify: `ui/navigation/AppDrawer.kt` (highlight active item)

**Step 1: Add role-guard to admin screens**

Each admin screen ViewModel should check `TokenRepository.getUserRole()` in `init` and set an unauthorized state if the role doesn't have access. This is a safety net — the drawer already filters, but direct URL navigation (unlikely but possible) should also be blocked.

**Step 2: Add active-item highlighting to drawer**

Pass current route to `AppDrawerContent` and highlight the matching item using `selected = true` on `NavigationDrawerItem`.

**Step 3: Verify all navigation paths**

- Drawer → each admin screen → back returns to Menu
- Customer detail → order history → order detail → back chain works
- Product detail → bottom sheets → dismiss → back works
- Staff form → save → pops back to list
- Reports → tab switch → date range change → data refreshes

**Step 4: Build final APK and test on device**

Run: `cd android && ./gradlew assembleDebug`

Install on device: `cd android && ./gradlew installDebug`

Test with different roles:
- OWNER account: sees all drawer items, all tabs
- MANAGER account: sees all except Outlet comparison tab
- CASHIER account: sees only Pesanan + Printer

**Step 5: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/navigation/
git commit -m "feat(android): final navigation wiring and role-guard integration"
```

---

## Dependency Graph

```
Task 1 (Drawer + Role) ─┬── Task 2 (Report Data) ── Task 3 (Report Screen)
                         ├── Task 4 (CRM Data) ── Task 5 (CRM Screens)
                         ├── Task 6 (Order History)
                         ├── Task 7 (Menu Data) ── Task 8 (Categories) ── Task 9 (Products)
                         └── Task 10 (Staff)

All ── Task 11 (Integration)
```

Tasks 2-10 can run in parallel after Task 1 (within their chains). Task 11 is the final integration pass.

---

## Estimated File Count

| Group | New Files | Modified Files |
|-------|-----------|---------------|
| Foundation (Task 1) | 2 | 3 |
| Reports (Tasks 2-3) | 4 | 1 |
| CRM (Tasks 4-5) | 5 | 2 |
| Order History (Task 6) | 0 | 4 |
| Menu CRUD (Tasks 7-9) | 8 | 2 |
| Staff (Task 10) | 6 | 2 |
| Integration (Task 11) | 0 | 2 |
| **Total** | **25** | **16** |
