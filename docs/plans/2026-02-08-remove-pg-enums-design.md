# Remove PostgreSQL Enums — Design Document

> Replace PostgreSQL ENUM types with VARCHAR columns for migration flexibility and future extensibility.
> Created: 2026-02-08

## 1. Problem

PostgreSQL `CREATE TYPE ... AS ENUM` is rigid:
- **Adding values** requires `ALTER TYPE ... ADD VALUE` (can't run in transactions pre-PG12, still a migration per value)
- **Removing/renaming values** is effectively impossible without recreating the type
- **sqlc coupling**: generates typed wrappers (`NullOrderStatus`) that add boilerplate

For a v1 POS system that will evolve (new payment methods, kitchen stations, order types), this friction isn't worth the type safety.

## 2. Strategy: Split by Purpose

### Group A — CHECK constraints (state machines)

Values are tightly coupled to Go business logic (transition maps, authorization checks). DB-level constraint prevents garbage data. Adding a value = simple `ALTER TABLE DROP/ADD CONSTRAINT` (works in transactions).

| Enum | Column(s) | Values |
|------|-----------|--------|
| `order_status` | `orders.status` | NEW, PREPARING, READY, COMPLETED, CANCELLED |
| `order_item_status` | `order_items.status` | PENDING, PREPARING, READY |
| `catering_status` | `orders.catering_status` | BOOKED, DP_PAID, SETTLED, CANCELLED |
| `payment_status` | `payments.status` | PENDING, COMPLETED, FAILED |

### Group B — Pure VARCHAR (configurable labels)

Values will grow as the business evolves. No DB constraint — validation in Go constants only.

| Enum | Column(s) | Current Values | Future Examples |
|------|-----------|----------------|-----------------|
| `kitchen_station` | `products.station`, `order_items.station` | GRILL, BEVERAGE, RICE, DESSERT | FRYER, PREP, PACKAGING |
| `payment_method` | `payments.payment_method` | CASH, QRIS, TRANSFER | GOPAY, OVO, DANA, SHOPEEPAY |
| `discount_type` | `orders.discount_type`, `order_items.discount_type` | PERCENTAGE, FIXED_AMOUNT | BUY_X_GET_Y (v2) |

### Group C — CHECK constraints (borderline)

Values control code paths but will occasionally grow. CHECK constraint for safety, easy to update.

| Enum | Column(s) | Values |
|------|-----------|--------|
| `user_role` | `users.role` | OWNER, MANAGER, CASHIER, KITCHEN |
| `order_type` | `orders.order_type` | DINE_IN, TAKEAWAY, DELIVERY, CATERING |

## 3. Migration

### File: `000002_remove_enums.up.sql`

```sql
-- ============================================================
-- Remove PostgreSQL ENUMs → VARCHAR + CHECK constraints
-- ============================================================

-- 1. Convert all enum columns to VARCHAR
-- Using ALTER COLUMN TYPE with USING cast preserves existing data.

-- users
ALTER TABLE users ALTER COLUMN role TYPE VARCHAR(20) USING role::text;

-- products
ALTER TABLE products ALTER COLUMN station TYPE VARCHAR(20) USING station::text;

-- orders
ALTER TABLE orders ALTER COLUMN order_type TYPE VARCHAR(20) USING order_type::text;
ALTER TABLE orders ALTER COLUMN status TYPE VARCHAR(20) USING status::text;
ALTER TABLE orders ALTER COLUMN discount_type TYPE VARCHAR(20) USING discount_type::text;
ALTER TABLE orders ALTER COLUMN catering_status TYPE VARCHAR(20) USING catering_status::text;

-- order_items
ALTER TABLE order_items ALTER COLUMN discount_type TYPE VARCHAR(20) USING discount_type::text;
ALTER TABLE order_items ALTER COLUMN status TYPE VARCHAR(20) USING status::text;
ALTER TABLE order_items ALTER COLUMN station TYPE VARCHAR(20) USING station::text;

-- payments
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

### File: `000002_remove_enums.down.sql`

```sql
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

## 4. Go Code Changes

### New package: `api/internal/enum/enum.go`

Single file defining all constant groups. Replaces sqlc-generated enum types.

```go
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

### Handler changes

Replace all `database.EnumType` references with `enum.Constant` and `string` types:

```go
// Before
func isValidRole(role string) bool {
    switch database.UserRole(role) {
    case database.UserRoleOWNER, database.UserRoleMANAGER,
        database.UserRoleCASHIER, database.UserRoleKITCHEN:
        return true
    }
    return false
}

// After
func isValidRole(role string) bool {
    switch role {
    case enum.UserRoleOwner, enum.UserRoleManager,
        enum.UserRoleCashier, enum.UserRoleKitchen:
        return true
    }
    return false
}
```

### Transition maps

```go
// Before
var allowedTransitions = map[database.OrderStatus][]database.OrderStatus{
    database.OrderStatusNEW: {database.OrderStatusPREPARING, database.OrderStatusCANCELLED},
    ...
}

// After
var allowedTransitions = map[string][]string{
    enum.OrderStatusNew:       {enum.OrderStatusPreparing, enum.OrderStatusCancelled},
    enum.OrderStatusPreparing: {enum.OrderStatusReady, enum.OrderStatusCancelled},
    enum.OrderStatusReady:     {enum.OrderStatusCompleted, enum.OrderStatusCancelled},
}
```

### sqlc queries — remove enum casts

```sql
-- Before
AND (sqlc.narg('status')::order_status IS NULL OR status = sqlc.narg('status')::order_status)

-- After
AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
```

### sqlc generated code — Null wrappers

`NullOrderStatus`, `NullUserRole`, etc. become `sql.NullString`:

```go
// Before
params.Status = database.NullOrderStatus{
    OrderStatus: database.OrderStatus(s), Valid: true,
}

// After
params.Status = sql.NullString{String: s, Valid: true}
```

## 5. sqlc Configuration

No sqlc config changes needed. When enum types are dropped, sqlc automatically generates `string` for VARCHAR columns and `sql.NullString` for nullable ones.

## 6. Files to Modify

### New files
- `api/migrations/000002_remove_enums.up.sql`
- `api/migrations/000002_remove_enums.down.sql`
- `api/internal/enum/enum.go`

### Modified files (after running `make api-sqlc`)
- `api/internal/database/models.go` — all enum types and Null wrappers removed by sqlc
- `api/internal/database/*.sql.go` — generated query functions use `string` instead of enum types

### Handler files (manual updates)
- `api/internal/handler/users.go` — `isValidRole()`, role references
- `api/internal/handler/orders.go` — `isValidOrderStatus()`, `allowedTransitions`, `isValidItemStatus()`, `allowedItemTransitions`
- `api/internal/handler/payments.go` — `isValidPaymentMethod()`, payment method references
- `api/internal/handler/products.go` — `isValidStation()`, station references
- `api/internal/service/order.go` — `validateOrderType()`, `isValidDiscountType()`, order type comparisons

### Android client
- No changes needed. Android already sends/receives string values over JSON.

### Admin (SvelteKit)
- No changes needed. Already uses string values.

## 7. Tradeoffs

| Aspect | Before (PG Enums) | After (VARCHAR + CHECK/constants) |
|--------|-------------------|-----------------------------------|
| **Type safety** | Compile-time via sqlc types | Runtime via Go constants + CHECK |
| **Adding values** | `ALTER TYPE ADD VALUE` migration | Group A/C: `ALTER TABLE DROP/ADD CONSTRAINT`. Group B: just add Go constant |
| **Removing values** | Nearly impossible | Group A/C: update CHECK. Group B: remove constant |
| **DB-level protection** | Enum type enforces | CHECK constraint for state machines, nothing for labels |
| **sqlc boilerplate** | Typed enums + Null wrappers | Plain `string` + `sql.NullString` |
| **Query casts** | `::order_status` needed | No casts needed |
| **Typo risk** | Caught at compile time | Caught at DB level (CHECK) or runtime (validation) |

## 8. Future: Adding a New Payment Method

With this design, adding GOPAY as a payment method:

1. Add `PaymentMethodGopay = "GOPAY"` to `enum/enum.go`
2. Add `"GOPAY"` to `isValidPaymentMethod()` switch
3. Deploy. No migration needed.

Adding a new order status (e.g., `REFUNDED`):

1. Migration: `ALTER TABLE orders DROP CONSTRAINT chk_orders_status; ALTER TABLE orders ADD CONSTRAINT chk_orders_status CHECK (status IN (..., 'REFUNDED'));`
2. Add `OrderStatusRefunded = "REFUNDED"` to `enum/enum.go`
3. Update `allowedTransitions` map
4. Deploy.
