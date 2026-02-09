# Remove PostgreSQL Enums — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace all 9 PostgreSQL ENUM types with VARCHAR + CHECK constraints, and replace sqlc-generated enum types with plain Go string constants.

**Architecture:** DB migration converts columns to VARCHAR, adds CHECK constraints for state machines (Group A/C), leaves configurable labels unconstrained (Group B). New `enum` package replaces sqlc-generated types. All handlers/services/tests switch from `database.EnumType` to `enum.Constant` + `string`.

**Tech Stack:** Go (sqlc, golang-migrate), PostgreSQL 16

**Design doc:** `docs/plans/2026-02-08-remove-pg-enums-design.md`

---

### Task 1: Create the migration files

**Files:**
- Create: `api/migrations/000002_remove_enums.up.sql`
- Create: `api/migrations/000002_remove_enums.down.sql`

**Step 1: Create up migration**

```sql
-- api/migrations/000002_remove_enums.up.sql

-- ============================================================
-- Remove PostgreSQL ENUMs → VARCHAR + CHECK constraints
-- ============================================================

-- 1. Convert all enum columns to VARCHAR
ALTER TABLE users ALTER COLUMN role TYPE VARCHAR(20) USING role::text;

ALTER TABLE products ALTER COLUMN station TYPE VARCHAR(20) USING station::text;

ALTER TABLE orders ALTER COLUMN order_type TYPE VARCHAR(20) USING order_type::text;
ALTER TABLE orders ALTER COLUMN status TYPE VARCHAR(20) USING status::text;
ALTER TABLE orders ALTER COLUMN discount_type TYPE VARCHAR(20) USING discount_type::text;
ALTER TABLE orders ALTER COLUMN catering_status TYPE VARCHAR(20) USING catering_status::text;

ALTER TABLE order_items ALTER COLUMN discount_type TYPE VARCHAR(20) USING discount_type::text;
ALTER TABLE order_items ALTER COLUMN status TYPE VARCHAR(20) USING status::text;
ALTER TABLE order_items ALTER COLUMN station TYPE VARCHAR(20) USING station::text;

ALTER TABLE payments ALTER COLUMN payment_method TYPE VARCHAR(20) USING payment_method::text;
ALTER TABLE payments ALTER COLUMN status TYPE VARCHAR(20) USING status::text;

-- 2. Drop all enum types
DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS order_type;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS order_item_status;
DROP TYPE IF EXISTS catering_status;
DROP TYPE IF EXISTS kitchen_station;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS discount_type;

-- 3. Add CHECK constraints for Group A (state machines)
ALTER TABLE orders ADD CONSTRAINT chk_orders_status
  CHECK (status IN ('NEW', 'PREPARING', 'READY', 'COMPLETED', 'CANCELLED'));

ALTER TABLE order_items ADD CONSTRAINT chk_order_items_status
  CHECK (status IN ('PENDING', 'PREPARING', 'READY'));

ALTER TABLE orders ADD CONSTRAINT chk_orders_catering_status
  CHECK (catering_status IS NULL OR catering_status IN ('BOOKED', 'DP_PAID', 'SETTLED', 'CANCELLED'));

ALTER TABLE payments ADD CONSTRAINT chk_payments_status
  CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED'));

-- 4. Add CHECK constraints for Group C (borderline)
ALTER TABLE users ADD CONSTRAINT chk_users_role
  CHECK (role IN ('OWNER', 'MANAGER', 'CASHIER', 'KITCHEN'));

ALTER TABLE orders ADD CONSTRAINT chk_orders_order_type
  CHECK (order_type IN ('DINE_IN', 'TAKEAWAY', 'DELIVERY', 'CATERING'));
```

**Step 2: Create down migration**

```sql
-- api/migrations/000002_remove_enums.down.sql

-- ============================================================
-- Rollback: Recreate PostgreSQL ENUMs from VARCHAR
-- ============================================================

-- 1. Recreate enum types
CREATE TYPE user_role AS ENUM ('OWNER', 'MANAGER', 'CASHIER', 'KITCHEN');
CREATE TYPE order_type AS ENUM ('DINE_IN', 'TAKEAWAY', 'DELIVERY', 'CATERING');
CREATE TYPE order_status AS ENUM ('NEW', 'PREPARING', 'READY', 'COMPLETED', 'CANCELLED');
CREATE TYPE order_item_status AS ENUM ('PENDING', 'PREPARING', 'READY');
CREATE TYPE catering_status AS ENUM ('BOOKED', 'DP_PAID', 'SETTLED', 'CANCELLED');
CREATE TYPE kitchen_station AS ENUM ('GRILL', 'BEVERAGE', 'RICE', 'DESSERT');
CREATE TYPE payment_method AS ENUM ('CASH', 'QRIS', 'TRANSFER');
CREATE TYPE payment_status AS ENUM ('PENDING', 'COMPLETED', 'FAILED');
CREATE TYPE discount_type AS ENUM ('PERCENTAGE', 'FIXED_AMOUNT');

-- 2. Drop CHECK constraints
ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_status;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_catering_status;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_order_type;
ALTER TABLE order_items DROP CONSTRAINT IF EXISTS chk_order_items_status;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS chk_payments_status;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;

-- 3. Convert columns back to enum types
ALTER TABLE users ALTER COLUMN role TYPE user_role USING role::user_role;
ALTER TABLE products ALTER COLUMN station TYPE kitchen_station USING station::kitchen_station;
ALTER TABLE orders ALTER COLUMN order_type TYPE order_type USING order_type::order_type;
ALTER TABLE orders ALTER COLUMN status TYPE order_status USING status::order_status;
ALTER TABLE orders ALTER COLUMN discount_type TYPE discount_type USING discount_type::discount_type;
ALTER TABLE orders ALTER COLUMN catering_status TYPE catering_status USING catering_status::catering_status;
ALTER TABLE order_items ALTER COLUMN discount_type TYPE discount_type USING discount_type::discount_type;
ALTER TABLE order_items ALTER COLUMN status TYPE order_item_status USING status::order_item_status;
ALTER TABLE order_items ALTER COLUMN station TYPE kitchen_station USING station::kitchen_station;
ALTER TABLE payments ALTER COLUMN payment_method TYPE payment_method USING payment_method::payment_method;
ALTER TABLE payments ALTER COLUMN status TYPE payment_status USING status::payment_status;
```

**Step 3: Run migration**

Run: `make db-migrate`
Expected: migration 000002 applied successfully

**Step 4: Commit**

```bash
git add api/migrations/000002_remove_enums.up.sql api/migrations/000002_remove_enums.down.sql
git commit -m "feat(db): migrate enums to VARCHAR + CHECK constraints"
```

---

### Task 2: Create enum constants package

**Files:**
- Create: `api/internal/enum/enum.go`

**Step 1: Create the enum constants file**

```go
// api/internal/enum/enum.go
package enum

// ── Group A: State machines (CHECK constrained in DB) ──

const (
	OrderStatusNew       = "NEW"
	OrderStatusPreparing = "PREPARING"
	OrderStatusReady     = "READY"
	OrderStatusCompleted = "COMPLETED"
	OrderStatusCancelled = "CANCELLED"
)

const (
	OrderItemStatusPending   = "PENDING"
	OrderItemStatusPreparing = "PREPARING"
	OrderItemStatusReady     = "READY"
)

const (
	CateringStatusBooked    = "BOOKED"
	CateringStatusDPPaid    = "DP_PAID"
	CateringStatusSettled   = "SETTLED"
	CateringStatusCancelled = "CANCELLED"
)

const (
	PaymentStatusPending   = "PENDING"
	PaymentStatusCompleted = "COMPLETED"
	PaymentStatusFailed    = "FAILED"
)

// ── Group C: Borderline (CHECK constrained in DB) ──

const (
	UserRoleOwner   = "OWNER"
	UserRoleManager = "MANAGER"
	UserRoleCashier = "CASHIER"
	UserRoleKitchen = "KITCHEN"
)

const (
	OrderTypeDineIn   = "DINE_IN"
	OrderTypeTakeaway = "TAKEAWAY"
	OrderTypeDelivery = "DELIVERY"
	OrderTypeCatering = "CATERING"
)

// ── Group B: Configurable labels (no DB constraint) ──

const (
	StationGrill    = "GRILL"
	StationBeverage = "BEVERAGE"
	StationRice     = "RICE"
	StationDessert  = "DESSERT"
)

const (
	PaymentMethodCash     = "CASH"
	PaymentMethodQRIS     = "QRIS"
	PaymentMethodTransfer = "TRANSFER"
)

const (
	DiscountTypePercentage = "PERCENTAGE"
	DiscountTypeFixed      = "FIXED_AMOUNT"
)
```

**Step 2: Commit**

```bash
git add api/internal/enum/enum.go
git commit -m "feat: add enum constants package to replace sqlc enum types"
```

---

### Task 3: Remove enum casts from SQL queries and regenerate sqlc

**Files:**
- Modify: `api/queries/orders.sql:63-64` — remove `::order_status` and `::order_type` casts

**Step 1: Update orders.sql**

Replace lines 63-64 in `api/queries/orders.sql`:

```sql
-- Before:
  AND (sqlc.narg('status')::order_status IS NULL OR status = sqlc.narg('status')::order_status)
  AND (sqlc.narg('order_type')::order_type IS NULL OR order_type = sqlc.narg('order_type')::order_type)

-- After:
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('order_type')::text IS NULL OR order_type = sqlc.narg('order_type'))
```

**Step 2: Regenerate sqlc**

Run: `make api-sqlc`
Expected: succeeds, `api/internal/database/models.go` no longer has enum types, all `*.sql.go` files use `string` and `sql.NullString`

**Step 3: Verify generated code has no enum types**

Run: `cd api && grep -c "type.*Status\|type.*Role\|type.*Method\|type.*Station\|type.*Type.*string" internal/database/models.go`
Expected: no custom enum type definitions remain (only struct fields with `string` type)

**Step 4: Commit**

```bash
git add api/queries/orders.sql api/internal/database/
git commit -m "refactor: remove enum casts from SQL, regenerate sqlc with string types"
```

---

### Task 4: Update order service (`api/internal/service/order.go`)

**Files:**
- Modify: `api/internal/service/order.go`

**Step 1: Replace imports**

Replace `"github.com/kiwari-pos/api/internal/database"` usage with `enum` package where enum types are referenced. Add the enum import:

```go
import (
    // ... existing imports ...
    "github.com/kiwari-pos/api/internal/enum"
)
```

Keep the `database` import — it's still needed for struct types and query params.

**Step 2: Update `validateOrderType` (line 501-508)**

```go
// Before:
func validateOrderType(s string) (database.OrderType, error) {
	switch database.OrderType(s) {
	case database.OrderTypeDINEIN, database.OrderTypeTAKEAWAY,
		database.OrderTypeDELIVERY, database.OrderTypeCATERING:
		return database.OrderType(s), nil
	}
	return "", ErrInvalidOrderType
}

// After:
func validateOrderType(s string) (string, error) {
	switch s {
	case enum.OrderTypeDineIn, enum.OrderTypeTakeaway,
		enum.OrderTypeDelivery, enum.OrderTypeCatering:
		return s, nil
	}
	return "", ErrInvalidOrderType
}
```

**Step 3: Update `isValidDiscountType` (line 510-516)**

```go
// Before:
func isValidDiscountType(s string) bool {
	switch database.DiscountType(s) {
	case database.DiscountTypePERCENTAGE, database.DiscountTypeFIXEDAMOUNT:
		return true
	}
	return false
}

// After:
func isValidDiscountType(s string) bool {
	switch s {
	case enum.DiscountTypePercentage, enum.DiscountTypeFixed:
		return true
	}
	return false
}
```

**Step 4: Update `createOrderTx` signature (line 191)**

```go
// Before:
func (s *OrderService) createOrderTx(ctx context.Context, req CreateOrderRequest, orderType database.OrderType) (*CreateOrderResult, error) {

// After:
func (s *OrderService) createOrderTx(ctx context.Context, req CreateOrderRequest, orderType string) (*CreateOrderResult, error) {
```

**Step 5: Update catering/delivery comparisons (lines 148, 404, 425)**

```go
// Line 148 — Before:
if orderType == database.OrderTypeCATERING {
// After:
if orderType == enum.OrderTypeCatering {

// Line 404 — Before:
if orderType == database.OrderTypeCATERING {
// After:
if orderType == enum.OrderTypeCatering {

// Line 425 — Before:
if orderType == database.OrderTypeDELIVERY {
// After:
if orderType == enum.OrderTypeDelivery {
```

**Step 6: Update NullDiscountType → sql.NullString (lines 294, 305-308, 353, 361-364)**

Add `"database/sql"` to imports.

```go
// Item discount — Before:
itemDiscountType := database.NullDiscountType{}
// ...
itemDiscountType = database.NullDiscountType{
    DiscountType: database.DiscountType(item.DiscountType),
    Valid:        true,
}

// After:
itemDiscountType := sql.NullString{}
// ...
itemDiscountType = sql.NullString{
    String: item.DiscountType,
    Valid:  true,
}

// Order discount — Before:
orderDiscountType := database.NullDiscountType{}
// ...
orderDiscountType = database.NullDiscountType{
    DiscountType: database.DiscountType(req.DiscountType),
    Valid:        true,
}

// After:
orderDiscountType := sql.NullString{}
// ...
orderDiscountType = sql.NullString{
    String: req.DiscountType,
    Valid:  true,
}
```

**Step 7: Update NullCateringStatus → sql.NullString (lines 402, 410-413)**

```go
// Before:
cateringStatus := database.NullCateringStatus{}
// ...
cateringStatus = database.NullCateringStatus{
    CateringStatus: database.CateringStatusBOOKED,
    Valid:          true,
}

// After:
cateringStatus := sql.NullString{}
// ...
cateringStatus = sql.NullString{
    String: enum.CateringStatusBooked,
    Valid:  true,
}
```

**Step 8: Run tests to verify service compiles**

Run: `cd api && go build ./...`
Expected: compiles (tests will still fail until handlers updated)

**Step 9: Commit**

```bash
git add api/internal/service/order.go
git commit -m "refactor: update order service to use enum constants and string types"
```

---

### Task 5: Update users handler (`api/internal/handler/users.go`)

**Files:**
- Modify: `api/internal/handler/users.go`

**Step 1: Add enum import**

```go
import (
    // ... existing ...
    "github.com/kiwari-pos/api/internal/enum"
)
```

**Step 2: Update `isValidRole` (approx line 306-308)**

```go
// Before:
switch database.UserRole(role) {
case database.UserRoleOWNER, database.UserRoleMANAGER,
    database.UserRoleCASHIER, database.UserRoleKITCHEN:
    return nil
}

// After:
switch role {
case enum.UserRoleOwner, enum.UserRoleManager,
    enum.UserRoleCashier, enum.UserRoleKitchen:
    return nil
}
```

**Step 3: Update role assignments (lines 178, 250)**

The `database.CreateUserParams.Role` field is now `string` (after sqlc regen), so:

```go
// Before:
Role: database.UserRole(req.Role),

// After:
Role: req.Role,
```

**Step 4: Run tests**

Run: `cd api && go test ./internal/handler/ -run TestUser -v`
Expected: FAIL (test file still uses old types — will fix in Task 9)

**Step 5: Commit**

```bash
git add api/internal/handler/users.go
git commit -m "refactor: update users handler to use enum constants"
```

---

### Task 6: Update orders handler (`api/internal/handler/orders.go`)

**Files:**
- Modify: `api/internal/handler/orders.go`

This is the largest file. Add `enum` import, add `"database/sql"` import.

**Step 1: Update `isValidOrderStatus` (line ~1647)**

```go
// Before:
func isValidOrderStatus(s database.OrderStatus) bool {
    switch s {
    case database.OrderStatusNEW, database.OrderStatusPREPARING,
         database.OrderStatusREADY, database.OrderStatusCOMPLETED,
         database.OrderStatusCANCELLED:
        return true
    }
    return false
}

// After:
func isValidOrderStatus(s string) bool {
    switch s {
    case enum.OrderStatusNew, enum.OrderStatusPreparing,
         enum.OrderStatusReady, enum.OrderStatusCompleted,
         enum.OrderStatusCancelled:
        return true
    }
    return false
}
```

**Step 2: Update `allowedTransitions` (line ~1661)**

```go
// Before:
var allowedTransitions = map[database.OrderStatus][]database.OrderStatus{
    database.OrderStatusNEW:       {database.OrderStatusPREPARING, database.OrderStatusCANCELLED},
    database.OrderStatusPREPARING: {database.OrderStatusREADY, database.OrderStatusCANCELLED},
    database.OrderStatusREADY:     {database.OrderStatusCOMPLETED, database.OrderStatusCANCELLED},
}

// After:
var allowedTransitions = map[string][]string{
    enum.OrderStatusNew:       {enum.OrderStatusPreparing, enum.OrderStatusCancelled},
    enum.OrderStatusPreparing: {enum.OrderStatusReady, enum.OrderStatusCancelled},
    enum.OrderStatusReady:     {enum.OrderStatusCompleted, enum.OrderStatusCancelled},
}
```

**Step 3: Update `isValidItemStatus` (line ~1682)**

```go
// Before:
func isValidItemStatus(s database.OrderItemStatus) bool {
    switch s {
    case database.OrderItemStatusPENDING, database.OrderItemStatusPREPARING,
         database.OrderItemStatusREADY:
        return true
    }
    return false
}

// After:
func isValidItemStatus(s string) bool {
    switch s {
    case enum.OrderItemStatusPending, enum.OrderItemStatusPreparing,
         enum.OrderItemStatusReady:
        return true
    }
    return false
}
```

**Step 4: Update `allowedItemTransitions` (line ~1693)**

```go
// Before:
var allowedItemTransitions = map[database.OrderItemStatus][]database.OrderItemStatus{
    database.OrderItemStatusPENDING:   {database.OrderItemStatusPREPARING},
    database.OrderItemStatusPREPARING: {database.OrderItemStatusREADY},
}

// After:
var allowedItemTransitions = map[string][]string{
    enum.OrderItemStatusPending:   {enum.OrderItemStatusPreparing},
    enum.OrderItemStatusPreparing: {enum.OrderItemStatusReady},
}
```

**Step 5: Update status comparisons throughout**

All `database.OrderStatusXXX` → `enum.OrderStatusXxx`:
- Line 577: `newStatus := database.OrderStatus(req.Status)` → `newStatus := req.Status`
- Line 667: `database.OrderStatusCOMPLETED` → `enum.OrderStatusCompleted`
- Line 671: `database.OrderStatusCANCELLED` → `enum.OrderStatusCancelled`
- Line 738: `database.OrderStatusNEW` → `enum.OrderStatusNew`
- Line 1005: `database.OrderStatusCANCELLED`, `database.OrderStatusCOMPLETED` → `enum.OrderStatusCancelled`, `enum.OrderStatusCompleted`
- Line 1154: same pattern
- Line 1289: same pattern

**Step 6: Update item status conversion (line 1267)**

```go
// Before:
newStatus := database.OrderItemStatus(req.Status)

// After:
newStatus := req.Status
```

**Step 7: Update NullOrderStatus → sql.NullString for list filtering (line ~371)**

```go
// Before:
params.Status = database.NullOrderStatus{OrderStatus: database.OrderStatus(s), Valid: true}

// After:
params.Status = sql.NullString{String: s, Valid: true}
```

**Step 8: Update NullOrderType → sql.NullString (line ~374)**

```go
// Before:
params.OrderType = database.NullOrderType{OrderType: database.OrderType(s), Valid: true}

// After:
params.OrderType = sql.NullString{String: s, Valid: true}
```

**Step 9: Update NullDiscountType → sql.NullString (line ~850-854)**

```go
// Before:
var discountType database.NullDiscountType
discountType = database.NullDiscountType{DiscountType: database.DiscountType(req.DiscountType), Valid: true}

// After:
var discountType sql.NullString
discountType = sql.NullString{String: req.DiscountType, Valid: true}
```

**Step 10: Update discount comparisons (lines 865, 867, 1046, 1048)**

```go
// Before:
if database.DiscountType(req.DiscountType) == database.DiscountTypePERCENTAGE
if database.DiscountType(req.DiscountType) == database.DiscountTypeFIXEDAMOUNT
if currentItem.DiscountType.DiscountType == database.DiscountTypePERCENTAGE

// After:
if req.DiscountType == enum.DiscountTypePercentage
if req.DiscountType == enum.DiscountTypeFixed
if currentItem.DiscountType.String == enum.DiscountTypePercentage
```

Note: `currentItem.DiscountType` is now `sql.NullString`, so access `.String` instead of `.DiscountType`.

**Step 11: Compile check**

Run: `cd api && go build ./...`
Expected: compiles

**Step 12: Commit**

```bash
git add api/internal/handler/orders.go
git commit -m "refactor: update orders handler to use enum constants and string types"
```

---

### Task 7: Update payments handler (`api/internal/handler/payments.go`)

**Files:**
- Modify: `api/internal/handler/payments.go`

Add `enum` import and `"database/sql"` import.

**Step 1: Update `isValidPaymentMethod` (line ~346)**

```go
// Before:
func isValidPaymentMethod(pm database.PaymentMethod) bool {
    switch pm {
    case database.PaymentMethodCASH, database.PaymentMethodQRIS,
         database.PaymentMethodTRANSFER:
        return true
    }
    return false
}

// After:
func isValidPaymentMethod(pm string) bool {
    switch pm {
    case enum.PaymentMethodCash, enum.PaymentMethodQRIS,
         enum.PaymentMethodTransfer:
        return true
    }
    return false
}
```

**Step 2: Update payment method conversion (line 95)**

```go
// Before:
paymentMethod := database.PaymentMethod(req.PaymentMethod)

// After:
paymentMethod := req.PaymentMethod
```

**Step 3: Update all status/type comparisons**

```go
// Line 115 — Before:
if paymentMethod == database.PaymentMethodCASH
// After:
if paymentMethod == enum.PaymentMethodCash

// Line 143 — Before:
Status: database.PaymentStatusCOMPLETED,
// After:
Status: enum.PaymentStatusCompleted,

// Line 168 — Before:
if order.Status == database.OrderStatusCANCELLED
// After:
if order.Status == enum.OrderStatusCancelled

// Line 174 — Before:
if order.Status == database.OrderStatusCOMPLETED
// After:
if order.Status == enum.OrderStatusCompleted

// Line 223 — Before:
if order.OrderType == database.OrderTypeCATERING
// After:
if order.OrderType == enum.OrderTypeCatering
```

**Step 4: Update NullCateringStatus → sql.NullString (lines 225, 230-231, 247-248)**

```go
// Before:
if order.CateringStatus.Valid && order.CateringStatus.CateringStatus == database.CateringStatusBOOKED

// After:
if order.CateringStatus.Valid && order.CateringStatus.String == enum.CateringStatusBooked

// Before:
CateringStatus: database.NullCateringStatus{
    CateringStatus: database.CateringStatusDPPAID,
    Valid: true,
}

// After:
CateringStatus: sql.NullString{
    String: enum.CateringStatusDPPaid,
    Valid:  true,
}

// Before:
CateringStatus: database.NullCateringStatus{
    CateringStatus: database.CateringStatusSETTLED,
    Valid: true,
}

// After:
CateringStatus: sql.NullString{
    String: enum.CateringStatusSettled,
    Valid:  true,
}
```

**Step 5: Update remaining status comparisons (lines 262-264)**

```go
// Before:
if order.Status == database.OrderStatusNEW || order.Status == database.OrderStatusPREPARING ||
   order.Status == database.OrderStatusREADY

// After:
if order.Status == enum.OrderStatusNew || order.Status == enum.OrderStatusPreparing ||
   order.Status == enum.OrderStatusReady
```

**Step 6: Commit**

```bash
git add api/internal/handler/payments.go
git commit -m "refactor: update payments handler to use enum constants"
```

---

### Task 8: Update products handler (`api/internal/handler/products.go`)

**Files:**
- Modify: `api/internal/handler/products.go`

Add `enum` import and `"database/sql"` import.

**Step 1: Update station validation (line ~134)**

```go
// Before:
switch database.KitchenStation(station) {
case database.KitchenStationGRILL, database.KitchenStationBEVERAGE,
     database.KitchenStationRICE, database.KitchenStationDESSERT:
    return nil
}

// After:
switch station {
case enum.StationGrill, enum.StationBeverage,
     enum.StationRice, enum.StationDessert:
    return nil
}
```

**Step 2: Update NullKitchenStation → sql.NullString (lines 283, 285, 386, 388)**

```go
// Before:
station := database.NullKitchenStation{}
station = database.NullKitchenStation{KitchenStation: database.KitchenStation(req.Station), Valid: true}

// After:
station := sql.NullString{}
station = sql.NullString{String: req.Station, Valid: true}
```

**Step 3: Commit**

```bash
git add api/internal/handler/products.go
git commit -m "refactor: update products handler to use enum constants"
```

---

### Task 9: Update all test files (mechanical replacement)

**Files:**
- Modify: `api/internal/handler/orders_test.go` (90 refs)
- Modify: `api/internal/handler/payments_test.go` (39 refs)
- Modify: `api/internal/service/order_test.go` (22 refs)
- Modify: `api/internal/handler/users_test.go` (13 refs)
- Modify: `api/internal/handler/customers_test.go` (4 refs)
- Modify: `api/internal/handler/reports_test.go` (2 refs)
- Modify: `api/internal/handler/products_test.go` (1 ref)
- Modify: `api/internal/handler/auth_test.go` (1 ref)

**Total: 172 replacements across 8 files.**

This is mechanical find-and-replace. The mapping is:

**OrderStatus:**
| Before | After |
|--------|-------|
| `database.OrderStatusNEW` | `enum.OrderStatusNew` |
| `database.OrderStatusPREPARING` | `enum.OrderStatusPreparing` |
| `database.OrderStatusREADY` | `enum.OrderStatusReady` |
| `database.OrderStatusCOMPLETED` | `enum.OrderStatusCompleted` |
| `database.OrderStatusCANCELLED` | `enum.OrderStatusCancelled` |

**OrderItemStatus:**
| Before | After |
|--------|-------|
| `database.OrderItemStatusPENDING` | `enum.OrderItemStatusPending` |
| `database.OrderItemStatusPREPARING` | `enum.OrderItemStatusPreparing` |
| `database.OrderItemStatusREADY` | `enum.OrderItemStatusReady` |

**OrderType:**
| Before | After |
|--------|-------|
| `database.OrderTypeDINEIN` | `enum.OrderTypeDineIn` |
| `database.OrderTypeTAKEAWAY` | `enum.OrderTypeTakeaway` |
| `database.OrderTypeDELIVERY` | `enum.OrderTypeDelivery` |
| `database.OrderTypeCATERING` | `enum.OrderTypeCatering` |

**UserRole:**
| Before | After |
|--------|-------|
| `database.UserRoleOWNER` | `enum.UserRoleOwner` |
| `database.UserRoleMANAGER` | `enum.UserRoleManager` |
| `database.UserRoleCASHIER` | `enum.UserRoleCashier` |
| `database.UserRoleKITCHEN` | `enum.UserRoleKitchen` |

**PaymentMethod:**
| Before | After |
|--------|-------|
| `database.PaymentMethodCASH` | `enum.PaymentMethodCash` |
| `database.PaymentMethodQRIS` | `enum.PaymentMethodQRIS` |
| `database.PaymentMethodTRANSFER` | `enum.PaymentMethodTransfer` |

**PaymentStatus:**
| Before | After |
|--------|-------|
| `database.PaymentStatusCOMPLETED` | `enum.PaymentStatusCompleted` |
| `database.PaymentStatusPENDING` | `enum.PaymentStatusPending` |
| `database.PaymentStatusFAILED` | `enum.PaymentStatusFailed` |

**CateringStatus:**
| Before | After |
|--------|-------|
| `database.CateringStatusBOOKED` | `enum.CateringStatusBooked` |
| `database.CateringStatusDPPAID` | `enum.CateringStatusDPPaid` |
| `database.CateringStatusSETTLED` | `enum.CateringStatusSettled` |
| `database.CateringStatusCANCELLED` | `enum.CateringStatusCancelled` |

**KitchenStation:**
| Before | After |
|--------|-------|
| `database.KitchenStationGRILL` | `enum.StationGrill` |
| `database.KitchenStationBEVERAGE` | `enum.StationBeverage` |
| `database.KitchenStationRICE` | `enum.StationRice` |
| `database.KitchenStationDESSERT` | `enum.StationDessert` |

**DiscountType:**
| Before | After |
|--------|-------|
| `database.DiscountTypePERCENTAGE` | `enum.DiscountTypePercentage` |
| `database.DiscountTypeFIXEDAMOUNT` | `enum.DiscountTypeFixed` |

**Null wrapper replacements in tests:**
| Before | After |
|--------|-------|
| `database.NullOrderStatus{OrderStatus: ..., Valid: true}` | `sql.NullString{String: ..., Valid: true}` |
| `database.NullOrderType{OrderType: ..., Valid: true}` | `sql.NullString{String: ..., Valid: true}` |
| `database.NullCateringStatus{CateringStatus: ..., Valid: true}` | `sql.NullString{String: ..., Valid: true}` |
| `database.NullDiscountType{DiscountType: ..., Valid: true}` | `sql.NullString{String: ..., Valid: true}` |
| `database.NullKitchenStation{KitchenStation: ..., Valid: true}` | `sql.NullString{String: ..., Valid: true}` |
| `database.NullPaymentMethod{PaymentMethod: ..., Valid: true}` | `sql.NullString{String: ..., Valid: true}` |
| `database.NullPaymentStatus{PaymentStatus: ..., Valid: true}` | `sql.NullString{String: ..., Valid: true}` |

Also replace field access patterns:
| Before | After |
|--------|-------|
| `.CateringStatus.CateringStatus` | `.CateringStatus.String` |
| `.DiscountType.DiscountType` | `.DiscountType.String` |
| `.Station.KitchenStation` | `.Station.String` |
| `.Station.Valid` | `.Station.Valid` (unchanged) |

**Step 1: Add enum import to all test files**

Each test file needs:
```go
"github.com/kiwari-pos/api/internal/enum"
```

And if they use Null wrappers, also:
```go
"database/sql"
```

**Step 2: Apply replacements file by file**

Process each file using the mapping tables above. The approach:
1. Replace all `database.EnumConstant` → `enum.NewConstant`
2. Replace all `database.NullXxx{Xxx: ..., Valid: ...}` → `sql.NullString{String: ..., Valid: ...}`
3. Replace all `.FieldName.EnumType` field access → `.FieldName.String`
4. Remove unused `database` imports if the file no longer references `database.`
5. Add `enum` and `"database/sql"` imports as needed

**Step 3: Run full test suite**

Run: `cd api && go test ./... -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add api/internal/handler/*_test.go api/internal/service/*_test.go
git commit -m "refactor: update all test files to use enum constants and sql.NullString"
```

---

### Task 10: Update mock interfaces if needed

After sqlc regeneration, the `OrderStore` interface in `api/internal/service/order.go` uses `database.*Params` structs. These structs now have `string` fields instead of enum types — the interface itself doesn't change, but any mock implementations in tests need to match.

**Step 1: Verify mock implementations compile**

Run: `cd api && go build ./...`
Expected: compiles cleanly

**Step 2: Run full test suite**

Run: `cd api && go test ./... -count=1`
Expected: ALL PASS (the `-count=1` prevents cached results)

**Step 3: Commit (if any fixes were needed)**

```bash
git add -A api/
git commit -m "fix: update mock implementations for new string-typed params"
```

---

### Task 11: Final verification and cleanup

**Step 1: Verify no remaining enum references in non-generated code**

Run: `cd api && grep -rn "database\.\(OrderStatus\|OrderItemStatus\|OrderType\|UserRole\|PaymentMethod\|PaymentStatus\|KitchenStation\|CateringStatus\|DiscountType\)" --include="*.go" --exclude-dir=database`
Expected: zero matches

**Step 2: Verify no remaining Null wrapper references in non-generated code**

Run: `cd api && grep -rn "database\.Null\(OrderStatus\|OrderType\|CateringStatus\|DiscountType\|KitchenStation\|PaymentMethod\|PaymentStatus\|OrderItemStatus\)" --include="*.go" --exclude-dir=database`
Expected: zero matches

**Step 3: Run full test suite one final time**

Run: `cd api && go test ./... -count=1 -v`
Expected: ALL PASS

**Step 4: Verify migration rollback works**

Run: `make db-rollback`
Expected: migration 000002 rolled back, enums recreated

Run: `make db-migrate`
Expected: migration 000002 re-applied cleanly

**Step 5: Final commit if any cleanup**

```bash
git add -A
git commit -m "chore: final cleanup after enum removal"
```
