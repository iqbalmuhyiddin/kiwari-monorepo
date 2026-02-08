# Android Order Flow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add save-before-payment order flow, order list, order detail, order editing, bill printing, and receipt sharing to the Android POS app.

**Architecture:** Extends existing Android POS (Kotlin/Jetpack Compose/Hilt/Retrofit) with new screens (Order List, Order Detail), modified screens (Cart, Payment, Catering, Menu), new printing formats (bill), and image sharing. Requires minor Go API changes (multi-status filter + amount_paid in list response).

**Tech Stack:** Kotlin, Jetpack Compose, Hilt, Retrofit, BigDecimal, ESC/POS, Canvas/Bitmap, FileProvider

**Design Doc:** `docs/plans/2026-02-08-android-order-flow-design.md`

---

## Task 1: Go API — Multi-Status Filter + Amount Paid

Add comma-separated multi-status filter and `amount_paid` subquery to the order list endpoint. This is a prerequisite for the Android Order List screen.

**Files:**
- Modify: `api/queries/orders.sql` (lines 60-68 — ListOrders query)
- Modify: `api/internal/handler/orders.go` (lines 348-395 — List handler)
- Regenerate: `api/internal/database/orders.sql.go` (via `make api-sqlc`)
- Test: `api/internal/handler/order_test.go`

**Step 1: Add ListActiveOrders query to orders.sql**

The existing `ListOrders` query uses `sqlc.narg('status')` for a single status. Rather than modifying it (which would break existing consumers like the admin dashboard), add a new dedicated query:

```sql
-- name: ListActiveOrders :many
SELECT o.*,
       COALESCE(
         (SELECT SUM(p.amount) FROM payments p WHERE p.order_id = o.id AND p.status = 'COMPLETED'),
         0
       )::decimal(12,2) AS amount_paid
FROM orders o
WHERE o.outlet_id = $1
  AND (
    o.status IN ('NEW', 'PREPARING', 'READY')
    OR (o.order_type = 'CATERING' AND o.catering_status IN ('BOOKED', 'DP_PAID'))
  )
ORDER BY o.created_at DESC
LIMIT $2 OFFSET $3;
```

This query:
- Returns all active orders (NEW/PREPARING/READY) plus unsettled catering orders (BOOKED/DP_PAID)
- Includes `amount_paid` via correlated subquery (avoids N+1)
- Dedicated endpoint means no risk of breaking existing list behavior

**Step 2: Regenerate sqlc code**

Run: `cd api && ~/go/bin/sqlc generate`

This generates `ListActiveOrders` function and `ListActiveOrdersRow` struct (with `AmountPaid` field).

**Step 3: Add ListActiveOrders handler**

In `api/internal/handler/orders.go`, add a new handler method:

```go
// ListActive handles GET /outlets/{oid}/orders/active.
func (h *OrderHandler) ListActive(w http.ResponseWriter, r *http.Request) {
    outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
    if err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
        return
    }

    claims := middleware.ClaimsFromContext(r.Context())
    if claims == nil {
        writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
        return
    }

    limit := 50
    if s := r.URL.Query().Get("limit"); s != "" {
        if v, err := strconv.Atoi(s); err == nil && v > 0 {
            limit = v
        }
    }
    if limit > 100 {
        limit = 100
    }

    offset := 0
    if s := r.URL.Query().Get("offset"); s != "" {
        if v, err := strconv.Atoi(s); err == nil && v >= 0 {
            offset = v
        }
    }

    rows, err := h.store.ListActiveOrders(r.Context(), database.ListActiveOrdersParams{
        OutletID: outletID,
        Limit:    int32(limit),
        Offset:   int32(offset),
    })
    if err != nil {
        log.Printf("ERROR: list active orders: %v", err)
        writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
        return
    }

    // Build response with amount_paid field
    type activeOrderResponse struct {
        orderResponse
        AmountPaid string `json:"amount_paid"`
    }

    resp := make([]activeOrderResponse, len(rows))
    for i, row := range rows {
        // Convert ListActiveOrdersRow to Order for dbOrderToResponse
        order := database.Order{/* map fields from row */}
        resp[i] = activeOrderResponse{
            orderResponse: dbOrderToResponse(order),
            AmountPaid:    decimalToString(row.AmountPaid),
        }
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "orders": resp,
        "limit":  limit,
        "offset": offset,
    })
}
```

**Step 4: Register route**

In `api/internal/router/router.go`, add the new route inside the orders group:

```go
r.Get("/active", orderHandler.ListActive)  // Must be BEFORE /{id} to avoid conflict
```

**Step 5: Add OrderStore interface method**

Add `ListActiveOrders` to the `OrderStore` interface in the handler file.

**Step 6: Write tests**

Add test cases for the new endpoint:
- Returns active orders (NEW, PREPARING, READY)
- Returns unsettled catering orders (DP_PAID)
- Excludes COMPLETED and CANCELLED
- Includes `amount_paid` field with correct sum
- Pagination works

**Step 7: Run tests**

Run: `cd api && go test ./internal/handler/ -run TestListActiveOrders -v`

**Step 8: Commit**

```bash
git add api/queries/orders.sql api/internal/database/ api/internal/handler/orders.go api/internal/handler/order_test.go
git commit -m "feat(api): add active orders endpoint with amount_paid"
```

---

## Task 2: Android — Order Detail Response Models

Add data models for the full order detail API response (order + nested items + modifiers + payments). These are needed by both Order List and Order Detail screens.

**Files:**
- Modify: `android/app/src/main/java/com/kiwari/pos/data/model/Order.kt`

**Step 1: Add order detail response models**

The Go API's `GET /orders/:id` returns `orderDetailResponse` which embeds `orderResponse` + `payments`. Add these Kotlin models to `Order.kt`:

```kotlin
// ── Order Detail Response (GET /orders/:id) ──────────────

data class OrderDetailResponse(
    @SerializedName("id") val id: String,
    @SerializedName("outlet_id") val outletId: String,
    @SerializedName("order_number") val orderNumber: String,
    @SerializedName("customer_id") val customerId: String?,
    @SerializedName("order_type") val orderType: String,
    @SerializedName("status") val status: String,
    @SerializedName("table_number") val tableNumber: String?,
    @SerializedName("notes") val notes: String?,
    @SerializedName("subtotal") val subtotal: String,
    @SerializedName("discount_type") val discountType: String?,
    @SerializedName("discount_value") val discountValue: String?,
    @SerializedName("discount_amount") val discountAmount: String,
    @SerializedName("tax_amount") val taxAmount: String,
    @SerializedName("total_amount") val totalAmount: String,
    @SerializedName("catering_date") val cateringDate: String?,
    @SerializedName("catering_status") val cateringStatus: String?,
    @SerializedName("catering_dp_amount") val cateringDpAmount: String?,
    @SerializedName("delivery_platform") val deliveryPlatform: String?,
    @SerializedName("delivery_address") val deliveryAddress: String?,
    @SerializedName("created_by") val createdBy: String,
    @SerializedName("created_at") val createdAt: String,
    @SerializedName("updated_at") val updatedAt: String,
    @SerializedName("items") val items: List<OrderItemResponse>,
    @SerializedName("payments") val payments: List<PaymentDetailResponse>
)

data class OrderItemResponse(
    @SerializedName("id") val id: String,
    @SerializedName("product_id") val productId: String,
    @SerializedName("variant_id") val variantId: String?,
    @SerializedName("quantity") val quantity: Int,
    @SerializedName("unit_price") val unitPrice: String,
    @SerializedName("discount_type") val discountType: String?,
    @SerializedName("discount_value") val discountValue: String?,
    @SerializedName("discount_amount") val discountAmount: String,
    @SerializedName("subtotal") val subtotal: String,
    @SerializedName("notes") val notes: String?,
    @SerializedName("status") val status: String,
    @SerializedName("station") val station: String?,
    @SerializedName("modifiers") val modifiers: List<OrderItemModifierResponse>
)

data class OrderItemModifierResponse(
    @SerializedName("id") val id: String,
    @SerializedName("modifier_id") val modifierId: String,
    @SerializedName("quantity") val quantity: Int,
    @SerializedName("unit_price") val unitPrice: String
)

data class PaymentDetailResponse(
    @SerializedName("id") val id: String,
    @SerializedName("order_id") val orderId: String,
    @SerializedName("payment_method") val paymentMethod: String,
    @SerializedName("amount") val amount: String,
    @SerializedName("status") val status: String,
    @SerializedName("reference_number") val referenceNumber: String?,
    @SerializedName("amount_received") val amountReceived: String?,
    @SerializedName("change_amount") val changeAmount: String?,
    @SerializedName("processed_by") val processedBy: String,
    @SerializedName("processed_at") val processedAt: String
)

// ── Active Orders List Response ──────────────

data class ActiveOrdersResponse(
    @SerializedName("orders") val orders: List<ActiveOrderResponse>,
    @SerializedName("limit") val limit: Int,
    @SerializedName("offset") val offset: Int
)

data class ActiveOrderResponse(
    @SerializedName("id") val id: String,
    @SerializedName("order_number") val orderNumber: String,
    @SerializedName("customer_id") val customerId: String?,
    @SerializedName("order_type") val orderType: String,
    @SerializedName("status") val status: String,
    @SerializedName("table_number") val tableNumber: String?,
    @SerializedName("total_amount") val totalAmount: String,
    @SerializedName("catering_date") val cateringDate: String?,
    @SerializedName("catering_status") val cateringStatus: String?,
    @SerializedName("created_at") val createdAt: String,
    @SerializedName("amount_paid") val amountPaid: String,
    @SerializedName("items") val items: List<OrderItemResponse>
)

// ── Add/Update/Delete Item Requests/Responses ──────────────

data class AddOrderItemRequest(
    @SerializedName("product_id") val productId: String,
    @SerializedName("variant_id") val variantId: String? = null,
    @SerializedName("quantity") val quantity: Int,
    @SerializedName("notes") val notes: String? = null,
    @SerializedName("modifiers") val modifiers: List<CreateOrderItemModifierRequest>? = null
)

data class UpdateOrderItemRequest(
    @SerializedName("quantity") val quantity: Int,
    @SerializedName("notes") val notes: String? = null
)

data class ItemActionResponse(
    @SerializedName("item") val item: OrderItemResponse?,
    @SerializedName("order") val order: OrderResponse,
    @SerializedName("message") val message: String?
)
```

**Step 2: Build to verify compilation**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 3: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/data/model/Order.kt
git commit -m "feat(android): add order detail and active orders response models"
```

---

## Task 3: Android — Extended OrderApi + OrderRepository

Add Retrofit endpoints for order list, detail, cancel, and item manipulation. Wrap them in OrderRepository.

**Files:**
- Modify: `android/app/src/main/java/com/kiwari/pos/data/api/OrderApi.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/data/repository/OrderRepository.kt`

**Step 1: Extend OrderApi with new endpoints**

Current OrderApi has only `createOrder` and `addPayment`. Add:

```kotlin
@GET("outlets/{outletId}/orders/active")
suspend fun listActiveOrders(
    @Path("outletId") outletId: String,
    @Query("limit") limit: Int = 50,
    @Query("offset") offset: Int = 0
): Response<ActiveOrdersResponse>

@GET("outlets/{outletId}/orders/{orderId}")
suspend fun getOrder(
    @Path("outletId") outletId: String,
    @Path("orderId") orderId: String
): Response<OrderDetailResponse>

@DELETE("outlets/{outletId}/orders/{orderId}")
suspend fun cancelOrder(
    @Path("outletId") outletId: String,
    @Path("orderId") orderId: String
): Response<OrderResponse>

@POST("outlets/{outletId}/orders/{orderId}/items")
suspend fun addOrderItem(
    @Path("outletId") outletId: String,
    @Path("orderId") orderId: String,
    @Body request: AddOrderItemRequest
): Response<ItemActionResponse>

@PUT("outlets/{outletId}/orders/{orderId}/items/{itemId}")
suspend fun updateOrderItem(
    @Path("outletId") outletId: String,
    @Path("orderId") orderId: String,
    @Path("itemId") itemId: String,
    @Body request: UpdateOrderItemRequest
): Response<ItemActionResponse>

@DELETE("outlets/{outletId}/orders/{orderId}/items/{itemId}")
suspend fun deleteOrderItem(
    @Path("outletId") outletId: String,
    @Path("orderId") orderId: String,
    @Path("itemId") itemId: String
): Response<ItemActionResponse>
```

**Step 2: Add repository wrapper methods**

Extend `OrderRepository.kt` with `safeApiCall` wrappers for each new endpoint. Use the existing `ApiHelper.safeApiCall` pattern from `data/repository/ApiHelper.kt`.

```kotlin
suspend fun listActiveOrders(outletId: String): Result<ActiveOrdersResponse> =
    safeApiCall { orderApi.listActiveOrders(outletId) }

suspend fun getOrder(outletId: String, orderId: String): Result<OrderDetailResponse> =
    safeApiCall { orderApi.getOrder(outletId, orderId) }

suspend fun cancelOrder(outletId: String, orderId: String): Result<OrderResponse> =
    safeApiCall { orderApi.cancelOrder(outletId, orderId) }

suspend fun addOrderItem(outletId: String, orderId: String, request: AddOrderItemRequest): Result<ItemActionResponse> =
    safeApiCall { orderApi.addOrderItem(outletId, orderId, request) }

suspend fun updateOrderItem(outletId: String, orderId: String, itemId: String, request: UpdateOrderItemRequest): Result<ItemActionResponse> =
    safeApiCall { orderApi.updateOrderItem(outletId, orderId, itemId, request) }

suspend fun deleteOrderItem(outletId: String, orderId: String, itemId: String): Result<ItemActionResponse> =
    safeApiCall { orderApi.deleteOrderItem(outletId, orderId, itemId, request) }
```

**Step 3: Build to verify**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 4: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/data/api/OrderApi.kt android/app/src/main/java/com/kiwari/pos/data/repository/OrderRepository.kt
git commit -m "feat(android): add order list, detail, cancel, and item API endpoints"
```

---

## Task 4: Android — Order List Screen

Create the Order List screen showing active orders with filter chips.

**Files:**
- Create: `android/app/src/main/java/com/kiwari/pos/ui/orders/OrderListScreen.kt`
- Create: `android/app/src/main/java/com/kiwari/pos/ui/orders/OrderListViewModel.kt`

**Step 1: Create OrderListViewModel**

State: `orders: List<ActiveOrderResponse>`, `isLoading`, `errorMessage`, `selectedFilter` (ALL, UNPAID, PAID).

- `init`: Load active orders from API via `orderRepository.listActiveOrders(outletId)`
- `outletId` from `TokenRepository` (same pattern as `MenuViewModel`)
- Filter logic: ALL shows everything, UNPAID filters where `amountPaid < totalAmount`, PAID filters where `amountPaid >= totalAmount`
- `refresh()` method for pull-to-refresh

**Step 2: Create OrderListScreen**

Composable with:
- Top bar: "Pesanan Aktif" with back arrow
- Filter chips row: [Semua] [Belum Bayar] [Lunas]
- LazyColumn of order cards
- Each card: order number, status badge (colored), payment status text, order type + table/catering date, item count + total, customer name
- Tap navigates to Order Detail via `onOrderClick(orderId: String)`
- Pull-to-refresh support

Card layout uses existing design tokens from `KiwariTheme` (12dp card radius, 1dp border).

**Step 3: Build to verify**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 4: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/orders/
git commit -m "feat(android): add order list screen with filter chips"
```

---

## Task 5: Android — Order Detail Screen

Create the Order Detail screen showing order info, items, payments, and action buttons. Adapts based on payment status (unpaid/paid/catering).

**Files:**
- Create: `android/app/src/main/java/com/kiwari/pos/ui/orders/OrderDetailScreen.kt`
- Create: `android/app/src/main/java/com/kiwari/pos/ui/orders/OrderDetailViewModel.kt`

**Step 1: Create OrderDetailViewModel**

State: `order: OrderDetailResponse?`, `isLoading`, `errorMessage`, `isPaid` (computed from payments sum vs total), `isCancelling`.

- `init(orderId)`: Load order via `orderRepository.getOrder(outletId, orderId)`
- `refresh()`: Reload order detail (called after returning from edit/payment)
- `cancelOrder()`: Call API, navigate back on success
- `isPaid`: `payments.sumOf { BigDecimal(it.amount) } >= BigDecimal(order.totalAmount)`
- `amountPaid`: Sum of payment amounts
- `amountRemaining`: `totalAmount - amountPaid`

**Step 2: Create OrderDetailScreen**

Composable with sections:
1. **Header**: Order number + status badge, order type + table/catering info, customer, timestamp
2. **Items list**: Each item with qty, name, unit price, subtotal. Indented modifiers. Notes.
3. **Totals**: Subtotal, discount, total
4. **Payment section** (paid): Each payment with method + amount + details
5. **Payment section** (unpaid): "Belum dibayar" text
6. **Payment section** (catering DP_PAID): DP amount + remaining
7. **Action row**: Three buttons — Print Kitchen, Print Bill/Receipt, Share
8. **Bottom buttons** (unpaid): EDIT + BAYAR side by side
9. **Cancel link** (unpaid): "Batalkan Pesanan" destructive text button with confirmation dialog

Navigation callbacks:
- `onEdit(orderId)` → Menu screen in edit mode
- `onPay(orderId)` → Payment screen with existing order
- `onBack()` → pop back stack

**Step 3: Wire printing actions (stubs for now)**

The Print Kitchen / Print Bill / Share buttons call ViewModel methods that will be implemented in Task 9. For now, add TODO stubs:

```kotlin
fun printKitchenTicket() { /* TODO: Task 9 */ }
fun printBill() { /* TODO: Task 9 */ }
fun shareReceipt() { /* TODO: Task 9 */ }
```

**Step 4: Build to verify**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 5: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/orders/
git commit -m "feat(android): add order detail screen with unpaid/paid/catering states"
```

---

## Task 6: Android — Cart Screen SIMPAN Button + Order Save

Modify the Cart screen to show SIMPAN + BAYAR buttons for non-catering orders. SIMPAN creates the order via API and navigates to Order Detail.

**Files:**
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/cart/CartScreen.kt` (lines 546-648 — bottom bar)
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/cart/CartViewModel.kt`

**Step 1: Add saveOrder method to CartViewModel**

New method that:
1. Validates cart (same as `validateForPayment()` minus payment routing)
2. Builds `CreateOrderRequest` from cart items + metadata (reuse logic from `PaymentViewModel.buildOrderRequest()` — extract shared helper)
3. Calls `orderRepository.createOrder(outletId, request)`
4. Clears cart + metadata on success
5. Returns order ID for navigation

State additions: `isSaving: Boolean`, `savedOrderId: String?`

```kotlin
fun saveOrder() {
    if (!validateForSave()) return
    viewModelScope.launch {
        _uiState.update { it.copy(isSaving = true) }
        val request = buildCreateOrderRequest()
        when (val result = orderRepository.createOrder(outletId, request)) {
            is Result.Success -> {
                cartRepository.clearCart()
                orderMetadataRepository.clear()
                _uiState.update { it.copy(isSaving = false, savedOrderId = result.data.id) }
            }
            is Result.Error -> {
                _uiState.update { it.copy(isSaving = false, errorMessage = result.message) }
            }
        }
    }
}
```

**Step 2: Modify Cart bottom bar layout**

Change the single button to two buttons when not catering:

```kotlin
// Bottom buttons
if (uiState.orderType == OrderType.CATERING) {
    // Single full-width LANJUT BOOKING button (unchanged)
    Button(onClick = onNavigateToCatering, ...) { Text("LANJUT BOOKING") }
} else {
    // Two buttons side by side
    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedButton(
            onClick = { viewModel.saveOrder() },
            modifier = Modifier.weight(1f),
            enabled = !uiState.isSaving,
            ...
        ) { Text("SIMPAN") }

        Button(
            onClick = onNavigateToPayment,
            modifier = Modifier.weight(1f),
            ...
        ) { Text("BAYAR ${formatPrice(uiState.total)}") }
    }
}
```

**Step 3: Handle navigation after save**

In NavGraph, observe `savedOrderId` and navigate to Order Detail:

```kotlin
LaunchedEffect(uiState.savedOrderId) {
    uiState.savedOrderId?.let { orderId ->
        viewModel.clearSavedOrderId()
        onNavigateToOrderDetail(orderId)
    }
}
```

**Step 4: Build to verify**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 5: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/cart/
git commit -m "feat(android): add SIMPAN button to save order without payment"
```

---

## Task 7: Android — Cart Edit Mode

Enable loading an existing order's items into the cart for editing. On save, diff the cart against the original and call add/update/delete item APIs.

**Files:**
- Modify: `android/app/src/main/java/com/kiwari/pos/data/repository/CartRepository.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/cart/CartViewModel.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/cart/CartScreen.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/menu/MenuViewModel.kt`

**Step 1: Add loadFromOrder to CartRepository**

New method that converts `OrderDetailResponse` items into `CartItem` list and loads them into the cart:

```kotlin
fun loadFromOrder(order: OrderDetailResponse, products: Map<String, Product>) {
    val items = order.items.map { item ->
        val product = products[item.productId] ?: return@map null
        CartItem(
            id = item.id, // Use API item ID (important for diffing)
            product = product,
            selectedVariants = /* map from item.variantId + product data */,
            selectedModifiers = item.modifiers.map { /* map from modifier data */ },
            quantity = item.quantity,
            notes = item.notes ?: "",
            lineTotal = BigDecimal(item.subtotal)
        )
    }.filterNotNull()
    _items.value = items
}
```

Also add `originalItems` snapshot for diffing:

```kotlin
private var _originalItems: List<CartItem> = emptyList()

fun setOriginalItems(items: List<CartItem>) {
    _originalItems = items.toList()
}

fun getOriginalItems(): List<CartItem> = _originalItems
```

**Step 2: Add edit mode to CartViewModel**

New state: `editingOrderId: String?`, `editingOrderNumber: String?`

New method `loadOrder(orderId: String)`:
1. Fetch order detail via `orderRepository.getOrder()`
2. Fetch product data for each item (need product name, etc. for display)
3. Call `cartRepository.loadFromOrder()` + `cartRepository.setOriginalItems()`
4. Set order metadata (type, table, customer, discount)
5. Set `editingOrderId`

New method `saveOrderEdits()`:
1. Diff current cart vs `originalItems`:
   - New items (in current, not in original by ID) → `addOrderItem` API calls
   - Removed items (in original, not in current) → `deleteOrderItem` API calls
   - Changed items (same ID, different qty/notes) → `updateOrderItem` API calls
2. Call APIs sequentially (order matters for total recalculation)
3. Navigate to Order Detail on success

**Step 3: Update Cart screen for edit mode**

- Top bar: Show "Edit Pesanan #KWR-005" when `editingOrderId != null`
- SIMPAN button calls `saveOrderEdits()` instead of `saveOrder()` in edit mode
- BAYAR button syncs edits first, then navigates to Payment

**Step 4: Update MenuViewModel for edit mode**

When entering Menu from Order Detail → Edit:
- Bottom bar shows "✏️ #KWR-005 3 item — LANJUT"
- Cart is pre-populated (already done in Step 1)

**Step 5: Build to verify**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 6: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/data/repository/CartRepository.kt \
        android/app/src/main/java/com/kiwari/pos/ui/cart/ \
        android/app/src/main/java/com/kiwari/pos/ui/menu/MenuViewModel.kt
git commit -m "feat(android): add cart edit mode with order item diffing"
```

---

## Task 8: Android — Payment Screen Existing Order Support

Modify Payment screen to accept an existing order ID, skip order creation, and navigate to Order Detail after payment.

**Files:**
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/payment/PaymentViewModel.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/payment/PaymentScreen.kt`

**Step 1: Add existing order mode to PaymentViewModel**

New constructor param or init check: if `orderId` is provided (via SavedStateHandle), skip order creation.

```kotlin
private val existingOrderId: String? = savedStateHandle["orderId"]
private val existingOrderTotal: String? = savedStateHandle["orderTotal"]
```

Modify `onSubmitOrder()`:
- If `existingOrderId != null`: Skip `createOrder`, use existing ID directly for `addPayment` calls
- After all payments: don't clear cart (it's already cleared or was never populated in edit-from-detail flow)
- Set `completedOrderId` instead of `isSuccess` for navigation

**Step 2: Replace success screen with Order Detail navigation**

Remove the `SuccessScreen` composable from PaymentScreen. Instead:
- After successful payment + auto-print receipt
- Set `completedOrderId` in state
- NavGraph observes this and navigates to Order Detail (popUpTo Menu)

```kotlin
LaunchedEffect(uiState.completedOrderId) {
    uiState.completedOrderId?.let { orderId ->
        onNavigateToOrderDetail(orderId)
    }
}
```

**Step 3: Build to verify**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 4: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/payment/
git commit -m "feat(android): payment screen supports existing orders, navigates to order detail"
```

---

## Task 9: Android — Bill Formatter + Receipt Image Sharing

Add bill thermal print format (unpaid orders), receipt image generator, and Android share intent.

**Files:**
- Modify: `android/app/src/main/java/com/kiwari/pos/util/printer/ReceiptFormatter.kt`
- Create: `android/app/src/main/java/com/kiwari/pos/util/share/ReceiptImageGenerator.kt`
- Create: `android/app/src/main/java/com/kiwari/pos/util/share/ShareHelper.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/orders/OrderDetailViewModel.kt`

**Step 1: Add formatBill to ReceiptFormatter**

New method `formatBill(data: ReceiptData): ByteArray` — same as `formatReceipt` but:
- Header says "BILL" instead of "STRUK"
- No payment breakdown section
- Footer says "** BELUM DIBAYAR **" (centered, bold) instead of "Terima Kasih"

Reuse existing ESC/POS helpers (header, twoColumn, separator, etc.)

**Step 2: Create ReceiptImageGenerator**

Renders receipt/bill text as a monospace bitmap:

```kotlin
@Singleton
class ReceiptImageGenerator @Inject constructor() {
    fun generateImage(text: String, context: Context): File {
        val paint = Paint().apply {
            typeface = Typeface.MONOSPACE
            textSize = 28f
            color = Color.BLACK
            isAntiAlias = true
        }

        val lines = text.split("\n")
        val lineHeight = paint.fontSpacing
        val maxWidth = lines.maxOf { paint.measureText(it) }.toInt() + 40
        val height = (lineHeight * lines.size + 40).toInt()

        val bitmap = Bitmap.createBitmap(maxWidth, height, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        canvas.drawColor(Color.WHITE)

        lines.forEachIndexed { index, line ->
            canvas.drawText(line, 20f, 20f + lineHeight * (index + 1), paint)
        }

        val file = File(context.cacheDir, "receipt_${System.currentTimeMillis()}.png")
        file.outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
        return file
    }
}
```

**Step 3: Create ShareHelper**

Uses `FileProvider` + `Intent.ACTION_SEND`:

```kotlin
object ShareHelper {
    fun shareImage(context: Context, file: File, title: String) {
        val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "image/png"
            putExtra(Intent.EXTRA_STREAM, uri)
            putExtra(Intent.EXTRA_SUBJECT, title)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        context.startActivity(Intent.createChooser(intent, "Bagikan"))
    }
}
```

**Step 4: Add FileProvider config**

Add `res/xml/file_paths.xml`:
```xml
<paths>
    <cache-path name="receipts" path="." />
</paths>
```

Register in `AndroidManifest.xml`:
```xml
<provider
    android:name="androidx.core.content.FileProvider"
    android:authorities="${applicationId}.fileprovider"
    android:exported="false"
    android:grantUriPermissions="true">
    <meta-data
        android:name="android.support.FILE_PROVIDER_PATHS"
        android:resource="@xml/file_paths" />
</provider>
```

**Step 5: Wire into OrderDetailViewModel**

Implement the print/share stub methods from Task 5:

```kotlin
fun printKitchenTicket() { /* build ReceiptData from order, call printerService */ }
fun printBill() { /* build ReceiptData, call formatBill via printerService */ }
fun printReceipt() { /* build ReceiptData, call formatReceipt via printerService */ }
fun shareReceipt(context: Context) { /* generate image, call ShareHelper */ }
```

Need to add `formatBillText(data: ReceiptData): String` method to ReceiptFormatter that returns plain text (not ESC/POS bytes) for image generation.

**Step 6: Build to verify**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 7: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/util/ \
        android/app/src/main/java/com/kiwari/pos/ui/orders/OrderDetailViewModel.kt \
        android/app/src/main/res/xml/file_paths.xml \
        android/app/src/main/AndroidManifest.xml
git commit -m "feat(android): add bill formatter, receipt image generator, and share"
```

---

## Task 10: Android — Navigation Wiring + Menu/Catering Updates

Wire all new screens into NavGraph, add Pesanan button to Menu screen, update Catering to navigate to Order Detail.

**Files:**
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/navigation/NavGraph.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/menu/MenuScreen.kt` (top bar)
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/menu/components/CartBottomBar.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/catering/CateringViewModel.kt`
- Modify: `android/app/src/main/java/com/kiwari/pos/ui/catering/CateringScreen.kt`

**Step 1: Add new routes to NavGraph**

```kotlin
object Screen {
    // ... existing routes ...
    object OrderList : Route { val route = "order-list" }
    object OrderDetail : Route { val route = "order-detail/{orderId}" }
}
```

Add composable destinations:
- `orderList` → `OrderListScreen(onOrderClick, onBack)`
- `orderDetail/{orderId}` → `OrderDetailScreen(onEdit, onPay, onBack)`
- Payment route now accepts optional `?orderId={id}&orderTotal={total}` query params
- Menu route now accepts optional `?editOrderId={id}` query param

**Step 2: Add Pesanan button to Menu screen top bar**

In `MenuScreen.kt`, add a list icon button between the search and settings icons:

```kotlin
// In the top bar Row (line 241)
IconButton(onClick = onNavigateToOrderList) {
    Icon(
        imageVector = Icons.AutoMirrored.Default.List,
        contentDescription = "Pesanan aktif"
    )
}
```

**Step 3: Update CartBottomBar for edit mode**

When `editingOrderNumber` is set, show "✏️ #KWR-005 3 item — LANJUT" instead of the default text.

**Step 4: Update Catering flow**

In `CateringViewModel`: After successful booking, set `completedOrderId` instead of `isSuccess`.

In `CateringScreen`: Remove `CateringSuccessScreen`. Instead, observe `completedOrderId` and navigate to Order Detail:

```kotlin
LaunchedEffect(uiState.completedOrderId) {
    uiState.completedOrderId?.let { orderId ->
        onNavigateToOrderDetail(orderId)
    }
}
```

**Step 5: Build to verify**

Run: `cd android && ./gradlew compileDebugKotlin`

**Step 6: Build APK and test on device**

Run: `cd android && ./gradlew installDebug`

Test flows:
1. Menu → Cart → SIMPAN → Order Detail → back to Menu
2. Menu → Pesanan → Order List → tap → Order Detail → BAYAR → Payment → Order Detail
3. Menu → Pesanan → Order Detail → EDIT → Menu (pre-loaded) → Cart → SIMPAN → Order Detail
4. Menu → Cart → BAYAR → Payment → Order Detail (no more success screen)
5. Catering flow → Order Detail (not success screen)
6. Order Detail → Print Kitchen / Print Bill / Share

**Step 7: Commit**

```bash
git add android/app/src/main/java/com/kiwari/pos/ui/navigation/NavGraph.kt \
        android/app/src/main/java/com/kiwari/pos/ui/menu/MenuScreen.kt \
        android/app/src/main/java/com/kiwari/pos/ui/menu/components/CartBottomBar.kt \
        android/app/src/main/java/com/kiwari/pos/ui/catering/
git commit -m "feat(android): wire navigation, menu pesanan button, catering → order detail"
```

---

## Dependencies

```
Task 1 (Go API) ─────────────────────────────┐
                                              ▼
Task 2 (Models) ──→ Task 3 (API + Repo) ──→ Task 4 (Order List)
                                         ──→ Task 5 (Order Detail) ──→ Task 9 (Print + Share)
                                         ──→ Task 6 (Cart SIMPAN) ──→ Task 7 (Cart Edit)
                                         ──→ Task 8 (Payment Existing)
                                                                   ──→ Task 10 (Navigation)
```

Tasks 4, 5, 6, 8 can be parallelized after Task 3.
Task 7 depends on Tasks 5 + 6.
Task 9 depends on Task 5.
Task 10 depends on all other tasks (final wiring).

## Notes for Implementer

- **Go API testing**: Run `cd api && go test ./internal/handler/ -v` after Task 1. Use `make api-sqlc` to regenerate.
- **Android build**: Run `cd android && ./gradlew compileDebugKotlin` after each task for compilation check. Run `./gradlew installDebug` for device testing.
- **outletId**: Available from `TokenRepository.getOutletId()` — same pattern used in existing ViewModels.
- **BigDecimal**: All money comparisons use `compareTo()`, never `==` (scale mismatch bug caught in M8.6).
- **Existing API shapes**: The Go API's `GET /orders/:id` already returns `orderDetailResponse` with embedded `payments` array (see `orders.go:183-186`). No API changes needed for the detail endpoint.
- **Cart edit diffing**: Compare by cart item ID. Items loaded from API use the API item UUID. New items added during edit use local UUIDs. This makes the diff straightforward: API IDs = existing, local IDs = new.
- **FileProvider**: Required for sharing images. Without it, `Intent.ACTION_SEND` with file URIs fails on Android 7+ due to `FileUriExposedException`.
