# Kiwari POS Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a multi-outlet POS system with Android cashier app, Go API backend, and SvelteKit web admin.

**Architecture:** Go REST+WebSocket API → PostgreSQL, served behind NPM reverse proxy on Tencent Cloud VPS. Android POS (Kotlin) for cashier operations. SvelteKit for admin/reporting. All in a monorepo.

**Tech Stack:** Go 1.22+ (Chi router, sqlc, golang-migrate), PostgreSQL 16, SvelteKit 2 (Svelte 5), Kotlin (Jetpack Compose, Retrofit, Hilt), Docker Compose.

**Design Doc:** `docs/plans/2026-02-06-pos-system-design.md`

---

## Project Structure

```
pos-superpower/
├── api/                    # Go API server
│   ├── cmd/server/         # main.go entrypoint
│   ├── internal/
│   │   ├── config/         # env config
│   │   ├── database/       # sqlc generated code
│   │   ├── handler/        # HTTP handlers
│   │   ├── middleware/      # auth, outlet scoping
│   │   ├── model/          # domain types
│   │   ├── service/        # business logic
│   │   └── ws/             # WebSocket hub
│   ├── migrations/         # SQL migration files
│   ├── queries/            # sqlc query files
│   ├── sqlc.yaml
│   └── go.mod
├── admin/                  # SvelteKit web admin
│   ├── src/
│   │   ├── lib/
│   │   │   ├── api/        # API client
│   │   │   ├── components/ # shared UI components
│   │   │   └── stores/     # Svelte stores
│   │   └── routes/         # SvelteKit pages
│   ├── package.json
│   └── svelte.config.js
├── android/                # Android POS app (Kotlin)
│   └── (Android Studio project)
├── docker/
│   ├── docker-compose.yml      # production
│   ├── docker-compose.dev.yml  # local dev (just PostgreSQL)
│   ├── Dockerfile.api
│   ├── Dockerfile.admin
│   ├── .env.example
│   └── backup.sh
├── docs/
│   ├── plans/                  # design + implementation plans
│   └── old-references/         # legacy design system files
├── Makefile                    # root task runner (already created)
├── .gitignore                  # covers Go, Node, Android, Docker, IDE (already created)
├── .editorconfig               # Go=tabs, Kotlin=4sp, rest=2sp (already created)
└── CLAUDE.md                   # Claude Code instructions (already created)
```

---

## Milestone 1: Project Scaffolding & Database

> Foundation: get the dev environment running with an empty database.

### Task 1.1: Initialize Go API Project

> Note: `Makefile`, `.gitignore`, `.editorconfig`, and `CLAUDE.md` already exist at project root.

**Files:**
- Create: `api/go.mod`
- Create: `api/cmd/server/main.go`
- Create: `api/internal/config/config.go`

**Step 1: Initialize Go module**

```bash
cd api
go mod init github.com/kiwari-pos/api
```

**Step 2: Create minimal main.go**

```go
// api/cmd/server/main.go
package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8081"
    }

    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"status":"ok"}`))
    })

    log.Printf("Starting server on :%s", port)
    if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
        log.Fatal(err)
    }
}
```

**Step 3: Create config loader**

```go
// api/internal/config/config.go
package config

import "os"

type Config struct {
    Port        string
    DatabaseURL string
    JWTSecret   string
}

func Load() *Config {
    return &Config{
        Port:        getEnv("PORT", "8081"),
        DatabaseURL: getEnv("DATABASE_URL", "postgres://pos:pos@localhost:5432/pos_db?sslmode=disable"),
        JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-in-production"),
    }
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

**Step 4: Create Makefile**

```makefile
# Makefile
.PHONY: api-run api-test db-up db-down db-migrate

api-run:
	cd api && go run ./cmd/server

api-test:
	cd api && go test ./... -v

db-up:
	docker compose -f docker/docker-compose.dev.yml up -d postgres

db-down:
	docker compose -f docker/docker-compose.dev.yml down

db-migrate:
	cd api && go run ./cmd/migrate
```

**Step 5: Verify it runs**

```bash
cd api && go run ./cmd/server
# In another terminal:
curl http://localhost:8081/health
# Expected: {"status":"ok"}
```

**Step 6: Commit**

```bash
git init
git add -A
git commit -m "feat: initialize Go API project with health endpoint"
```

---

### Task 1.2: Docker Compose for Local Development

**Files:**
- Create: `docker/docker-compose.dev.yml`
- Create: `docker/.env.example`

**Step 1: Create dev compose file**

```yaml
# docker/docker-compose.dev.yml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: pos
      POSTGRES_PASSWORD: pos
      POSTGRES_DB: pos_db
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U pos -d pos_db"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  pgdata:
```

**Step 2: Create .env.example**

```env
# docker/.env.example
DATABASE_URL=postgres://pos:pos@localhost:5432/pos_db?sslmode=disable
JWT_SECRET=change-me-in-production
PORT=8081
```

**Step 3: Start database and verify**

```bash
make db-up
# Wait a few seconds
docker exec -it $(docker ps -q --filter name=postgres) psql -U pos -d pos_db -c "SELECT 1"
# Expected: column with value 1
```

**Step 4: Commit**

```bash
git add docker/ Makefile
git commit -m "feat: add Docker Compose for local PostgreSQL"
```

---

### Task 1.3: Database Migrations — All Tables

**Files:**
- Create: `api/migrations/000001_init_schema.up.sql`
- Create: `api/migrations/000001_init_schema.down.sql`

**Step 1: Install golang-migrate**

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

**Step 2: Write UP migration**

Create `api/migrations/000001_init_schema.up.sql` with all 14 tables, enums, and indexes from the design doc. This is a single migration for the initial schema.

```sql
-- Enums
CREATE TYPE user_role AS ENUM ('OWNER', 'MANAGER', 'CASHIER', 'KITCHEN');
CREATE TYPE order_type AS ENUM ('DINE_IN', 'TAKEAWAY', 'DELIVERY', 'CATERING');
CREATE TYPE order_status AS ENUM ('NEW', 'PREPARING', 'READY', 'COMPLETED', 'CANCELLED');
CREATE TYPE order_item_status AS ENUM ('PENDING', 'PREPARING', 'READY');
CREATE TYPE catering_status AS ENUM ('BOOKED', 'DP_PAID', 'SETTLED', 'CANCELLED');
CREATE TYPE kitchen_station AS ENUM ('GRILL', 'BEVERAGE', 'RICE', 'DESSERT');
CREATE TYPE payment_method AS ENUM ('CASH', 'QRIS', 'TRANSFER');
CREATE TYPE payment_status AS ENUM ('PENDING', 'COMPLETED', 'FAILED');
CREATE TYPE discount_type AS ENUM ('PERCENTAGE', 'FIXED_AMOUNT');

-- Enable uuid-ossp
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Outlets
CREATE TABLE outlets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    address TEXT,
    phone VARCHAR(20),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    outlet_id UUID NOT NULL REFERENCES outlets(id),
    email VARCHAR(255) UNIQUE NOT NULL,
    hashed_password VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    role user_role NOT NULL,
    pin VARCHAR(6),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_outlet_role ON users(outlet_id, role);

-- Categories
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    outlet_id UUID NOT NULL REFERENCES outlets(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_categories_outlet_sort ON categories(outlet_id, sort_order);

-- Products
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    outlet_id UUID NOT NULL REFERENCES outlets(id),
    category_id UUID NOT NULL REFERENCES categories(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    base_price DECIMAL(12,2) NOT NULL,
    image_url VARCHAR(500),
    station kitchen_station,
    preparation_time INT,
    is_combo BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_products_outlet_category ON products(outlet_id, category_id);
CREATE INDEX idx_products_active ON products(is_active);

-- Variant Groups
CREATE TABLE variant_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES products(id),
    name VARCHAR(100) NOT NULL,
    is_required BOOLEAN NOT NULL DEFAULT true,
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_variant_groups_product ON variant_groups(product_id);

-- Variants
CREATE TABLE variants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    variant_group_id UUID NOT NULL REFERENCES variant_groups(id),
    name VARCHAR(100) NOT NULL,
    price_adjustment DECIMAL(12,2) NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_variants_group ON variants(variant_group_id);

-- Modifier Groups
CREATE TABLE modifier_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES products(id),
    name VARCHAR(100) NOT NULL,
    min_select INT NOT NULL DEFAULT 0,
    max_select INT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_modifier_groups_product ON modifier_groups(product_id);

-- Modifiers
CREATE TABLE modifiers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    modifier_group_id UUID NOT NULL REFERENCES modifier_groups(id),
    name VARCHAR(100) NOT NULL,
    price DECIMAL(12,2) NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_modifiers_group ON modifiers(modifier_group_id);

-- Combo Items
CREATE TABLE combo_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    combo_id UUID NOT NULL REFERENCES products(id),
    product_id UUID NOT NULL REFERENCES products(id),
    quantity INT NOT NULL DEFAULT 1,
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_combo_items_combo ON combo_items(combo_id);

-- Customers
CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    outlet_id UUID NOT NULL REFERENCES outlets(id),
    name VARCHAR(255) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    email VARCHAR(255),
    notes TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(outlet_id, phone)
);

-- Orders
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    outlet_id UUID NOT NULL REFERENCES outlets(id),
    order_number VARCHAR(20) NOT NULL,
    customer_id UUID REFERENCES customers(id),
    order_type order_type NOT NULL,
    status order_status NOT NULL DEFAULT 'NEW',
    table_number VARCHAR(20),
    notes TEXT,
    subtotal DECIMAL(12,2) NOT NULL,
    discount_type discount_type,
    discount_value DECIMAL(12,2),
    discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_amount DECIMAL(12,2) NOT NULL,
    catering_date TIMESTAMPTZ,
    catering_status catering_status,
    catering_dp_amount DECIMAL(12,2),
    delivery_platform VARCHAR(50),
    delivery_address TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE(outlet_id, order_number)
);
CREATE INDEX idx_orders_outlet_created ON orders(outlet_id, created_at);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_customer ON orders(customer_id);
CREATE INDEX idx_orders_catering ON orders(catering_status);

-- Order Items
CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id),
    variant_id UUID REFERENCES variants(id),
    quantity INT NOT NULL,
    unit_price DECIMAL(12,2) NOT NULL,
    discount_type discount_type,
    discount_value DECIMAL(12,2),
    discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    subtotal DECIMAL(12,2) NOT NULL,
    notes TEXT,
    status order_item_status NOT NULL DEFAULT 'PENDING',
    station kitchen_station
);
CREATE INDEX idx_order_items_order ON order_items(order_id);
CREATE INDEX idx_order_items_status ON order_items(status);

-- Order Item Modifiers
CREATE TABLE order_item_modifiers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
    modifier_id UUID NOT NULL REFERENCES modifiers(id),
    quantity INT NOT NULL DEFAULT 1,
    unit_price DECIMAL(12,2) NOT NULL
);
CREATE INDEX idx_order_item_modifiers_item ON order_item_modifiers(order_item_id);

-- Payments
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES orders(id),
    payment_method payment_method NOT NULL,
    amount DECIMAL(12,2) NOT NULL,
    status payment_status NOT NULL DEFAULT 'COMPLETED',
    reference_number VARCHAR(100),
    amount_received DECIMAL(12,2),
    change_amount DECIMAL(12,2),
    processed_by UUID NOT NULL REFERENCES users(id),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_payments_order ON payments(order_id);
CREATE INDEX idx_payments_method ON payments(payment_method);

-- Updated_at trigger function
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at triggers
CREATE TRIGGER set_updated_at BEFORE UPDATE ON outlets FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE TRIGGER set_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE TRIGGER set_updated_at BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE TRIGGER set_updated_at BEFORE UPDATE ON customers FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE TRIGGER set_updated_at BEFORE UPDATE ON orders FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
```

**Step 3: Write DOWN migration**

```sql
-- api/migrations/000001_init_schema.down.sql
DROP TRIGGER IF EXISTS set_updated_at ON orders;
DROP TRIGGER IF EXISTS set_updated_at ON customers;
DROP TRIGGER IF EXISTS set_updated_at ON products;
DROP TRIGGER IF EXISTS set_updated_at ON users;
DROP TRIGGER IF EXISTS set_updated_at ON outlets;
DROP FUNCTION IF EXISTS trigger_set_updated_at;

DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS order_item_modifiers;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS combo_items;
DROP TABLE IF EXISTS modifiers;
DROP TABLE IF EXISTS modifier_groups;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS variant_groups;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS outlets;

DROP TYPE IF EXISTS discount_type;
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS kitchen_station;
DROP TYPE IF EXISTS catering_status;
DROP TYPE IF EXISTS order_item_status;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS order_type;
DROP TYPE IF EXISTS user_role;
```

**Step 4: Run migration**

```bash
make db-up
migrate -path api/migrations -database "postgres://pos:pos@localhost:5432/pos_db?sslmode=disable" up
# Expected: 000001/u init_schema (Xms)
```

**Step 5: Verify tables exist**

```bash
docker exec -it $(docker ps -q --filter name=postgres) psql -U pos -d pos_db -c "\dt"
# Expected: 14 tables listed
```

**Step 6: Commit**

```bash
git add api/migrations/
git commit -m "feat: add initial database schema with all 14 tables"
```

---

### Task 1.4: Set Up sqlc for Type-Safe Queries

**Files:**
- Create: `api/sqlc.yaml`
- Create: `api/queries/outlets.sql` (starter)

**Step 1: Install sqlc**

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

**Step 2: Create sqlc config**

```yaml
# api/sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "migrations/"
    gen:
      go:
        package: "database"
        out: "internal/database"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "numeric"
            go_type: "github.com/shopspring/decimal.Decimal"
```

**Step 3: Create starter query file**

```sql
-- api/queries/outlets.sql

-- name: GetOutlet :one
SELECT * FROM outlets WHERE id = $1 AND is_active = true;

-- name: ListOutlets :many
SELECT * FROM outlets WHERE is_active = true ORDER BY name;

-- name: CreateOutlet :one
INSERT INTO outlets (name, address, phone)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateOutlet :one
UPDATE outlets SET name = $1, address = $2, phone = $3
WHERE id = $4 AND is_active = true
RETURNING *;

-- name: SoftDeleteOutlet :exec
UPDATE outlets SET is_active = false WHERE id = $1;
```

**Step 4: Install Go dependencies and generate**

```bash
cd api
go get github.com/jackc/pgx/v5
go get github.com/google/uuid
go get github.com/shopspring/decimal
sqlc generate
```

**Step 5: Verify generated code**

```bash
ls api/internal/database/
# Expected: db.go, models.go, outlets.sql.go, querier.go
```

**Step 6: Commit**

```bash
git add api/sqlc.yaml api/queries/ api/internal/database/ api/go.mod api/go.sum
git commit -m "feat: set up sqlc with outlet queries and generated code"
```

---

## Milestone 2: Go API — Auth & Middleware

> JWT authentication, outlet-scoped middleware, user management.

### Task 2.1: Auth — JWT Token Generation & Validation

**Files:**
- Create: `api/internal/auth/jwt.go`
- Create: `api/internal/auth/jwt_test.go`

**Step 1: Write failing test**

```go
// api/internal/auth/jwt_test.go
package auth_test

import (
    "testing"
    "github.com/google/uuid"
    "github.com/kiwari-pos/api/internal/auth"
)

func TestGenerateAndValidateToken(t *testing.T) {
    secret := "test-secret"
    userID := uuid.New()
    outletID := uuid.New()
    role := "CASHIER"

    token, err := auth.GenerateToken(secret, userID, outletID, role)
    if err != nil {
        t.Fatalf("generate token: %v", err)
    }

    claims, err := auth.ValidateToken(secret, token)
    if err != nil {
        t.Fatalf("validate token: %v", err)
    }

    if claims.UserID != userID {
        t.Errorf("user ID: got %v, want %v", claims.UserID, userID)
    }
    if claims.OutletID != outletID {
        t.Errorf("outlet ID: got %v, want %v", claims.OutletID, outletID)
    }
    if claims.Role != role {
        t.Errorf("role: got %v, want %v", claims.Role, role)
    }
}
```

**Step 2: Run test — expect fail**

```bash
cd api && go test ./internal/auth/ -v
# Expected: FAIL — package doesn't exist
```

**Step 3: Implement JWT**

```go
// api/internal/auth/jwt.go
package auth

import (
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

type Claims struct {
    UserID   uuid.UUID `json:"user_id"`
    OutletID uuid.UUID `json:"outlet_id"`
    Role     string    `json:"role"`
    jwt.RegisteredClaims
}

func GenerateToken(secret string, userID, outletID uuid.UUID, role string) (string, error) {
    claims := Claims{
        UserID:   userID,
        OutletID: outletID,
        Role:     role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func GenerateRefreshToken(secret string, userID uuid.UUID) (string, error) {
    claims := jwt.RegisteredClaims{
        Subject:   userID.String(),
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
        IssuedAt:  jwt.NewNumericDate(time.Now()),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func ValidateToken(secret, tokenStr string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return []byte(secret), nil
    })
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }
    return claims, nil
}
```

**Step 4: Run test — expect pass**

```bash
cd api && go test ./internal/auth/ -v
# Expected: PASS
```

**Step 5: Commit**

```bash
git add api/internal/auth/
git commit -m "feat: add JWT token generation and validation"
```

---

### Task 2.2: Auth Middleware & Outlet Scoping Middleware

**Files:**
- Create: `api/internal/middleware/auth.go`
- Create: `api/internal/middleware/auth_test.go`

Implement middleware that:
1. Extracts JWT from `Authorization: Bearer <token>` header
2. Validates token, injects claims into request context
3. Outlet scoping: verifies `:oid` path param matches token's outlet (unless OWNER role)

**Test:** Send request with valid/invalid/missing token, verify 401/403/200 responses.

**Commit:** `feat: add auth and outlet-scoping middleware`

---

### Task 2.3: Login & PIN Login Handlers

**Files:**
- Create: `api/queries/users.sql`
- Create: `api/internal/handler/auth.go`
- Create: `api/internal/handler/auth_test.go`

Implement:
- `POST /auth/login` — email + password → JWT + refresh token
- `POST /auth/pin-login` — outlet_id + pin → JWT + refresh token
- `POST /auth/refresh` — refresh token → new JWT
- Password hashing with bcrypt

**Test:** Login with valid/invalid credentials, PIN login, token refresh.

**Commit:** `feat: add login, PIN login, and token refresh endpoints`

---

### Task 2.4: User CRUD Handlers

**Files:**
- Create: `api/internal/handler/users.go`
- Create: `api/internal/handler/users_test.go`
- Create: `api/queries/users.sql` (extend)

Implement:
- `GET /outlets/:oid/users`
- `POST /outlets/:oid/users`
- `PUT /outlets/:oid/users/:id`
- `DELETE /outlets/:oid/users/:id` (soft delete)

**Commit:** `feat: add user CRUD endpoints`

---

## Milestone 3: Go API — Menu Management

> Full menu CRUD with variants, modifiers, and combos.

### Task 3.1: Category CRUD

**Files:**
- Create: `api/queries/categories.sql`
- Create: `api/internal/handler/categories.go`
- Create: `api/internal/handler/categories_test.go`

Implement all CRUD for categories scoped to outlet. Include sort_order management.

**Commit:** `feat: add category CRUD endpoints`

---

### Task 3.2: Product CRUD

**Files:**
- Create: `api/queries/products.sql`
- Create: `api/internal/handler/products.go`
- Create: `api/internal/handler/products_test.go`

Implement:
- `GET /outlets/:oid/products` — returns full tree (product + variant_groups + variants + modifier_groups + modifiers) in one query using JOINs or separate queries assembled in handler
- `POST /outlets/:oid/products` — create product with basic info
- `PUT /outlets/:oid/products/:id`
- `DELETE /outlets/:oid/products/:id` (soft delete)

**Key:** The GET list endpoint must return nested JSON. Use sqlc queries + manual assembly, or a single query with JSON aggregation.

**Commit:** `feat: add product CRUD with nested variant/modifier response`

---

### Task 3.3: Variant Groups & Variants CRUD

**Files:**
- Create: `api/queries/variants.sql`
- Create: `api/internal/handler/variants.go`
- Create: `api/internal/handler/variants_test.go`

Implement nested CRUD under products.

**Commit:** `feat: add variant group and variant CRUD endpoints`

---

### Task 3.4: Modifier Groups & Modifiers CRUD

**Files:**
- Create: `api/queries/modifiers.sql`
- Create: `api/internal/handler/modifiers.go`
- Create: `api/internal/handler/modifiers_test.go`

Implement nested CRUD under products with min/max select constraints.

**Commit:** `feat: add modifier group and modifier CRUD endpoints`

---

### Task 3.5: Combo Items CRUD

**Files:**
- Create: `api/queries/combos.sql`
- Create: `api/internal/handler/combos.go`
- Create: `api/internal/handler/combos_test.go`

Implement combo_items management (add/remove child products from combo).

**Commit:** `feat: add combo item management endpoints`

---

## Milestone 4: Go API — Orders & Payments

> Order lifecycle, multi-payment, catering bookings.

### Task 4.1: Order Creation (Atomic)

**Files:**
- Create: `api/queries/orders.sql`
- Create: `api/internal/handler/orders.go`
- Create: `api/internal/handler/orders_test.go`
- Create: `api/internal/service/order.go`

Implement `POST /outlets/:oid/orders`:
- Accepts order + items array in single request
- Wraps in database transaction
- Generates sequential order number per outlet (e.g., `KWR-001`)
- Validates product exists, variant belongs to product, modifiers valid
- Calculates subtotals, applies discounts
- Snapshots prices at order time
- For CATERING type: requires catering_date, customer_id

**This is the most complex endpoint. Business logic goes in `service/order.go`.**

**Commit:** `feat: add atomic order creation with price snapshots`

---

### Task 4.2: Order Queries & Status Management

**Files:**
- Modify: `api/internal/handler/orders.go`
- Modify: `api/queries/orders.sql`

Implement:
- `GET /outlets/:oid/orders` — list with filters (status, type, date range, pagination)
- `GET /outlets/:oid/orders/:id` — full detail with items, modifiers, payments
- `PATCH /outlets/:oid/orders/:id/status` — status transitions with validation
- `DELETE /outlets/:oid/orders/:id` — sets status to CANCELLED

Status transition rules:
```
NEW → PREPARING → READY → COMPLETED
NEW → CANCELLED (any time before COMPLETED)
PREPARING → CANCELLED (with reason)
```

**Commit:** `feat: add order listing, detail, and status management`

---

### Task 4.3: Order Item Modifications

**Files:**
- Modify: `api/internal/handler/orders.go`

Implement:
- `POST /outlets/:oid/orders/:id/items` — add item to existing order (only if NEW)
- `PUT /outlets/:oid/orders/:id/items/:iid` — modify item
- `DELETE /outlets/:oid/orders/:id/items/:iid` — remove item
- `PATCH /outlets/:oid/orders/:id/items/:iid/status` — kitchen marks item status

**Commit:** `feat: add order item modification and kitchen status updates`

---

### Task 4.4: Multi-Payment

**Files:**
- Create: `api/queries/payments.sql`
- Create: `api/internal/handler/payments.go`
- Create: `api/internal/handler/payments_test.go`

Implement:
- `POST /outlets/:oid/orders/:id/payments` — add payment
  - Validates total paid doesn't exceed order total
  - For CASH: records amount_received and change_amount
  - For QRIS/TRANSFER: optional reference_number
  - When total paid >= order total, auto-complete order
- `GET /outlets/:oid/orders/:id/payments` — list payments for order

For CATERING orders:
- First payment = down payment, sets catering_status to DP_PAID
- Final payment = settlement, sets catering_status to SETTLED

**Commit:** `feat: add multi-payment with catering DP lifecycle`

---

## Milestone 5: Go API — CRM, Reports, WebSocket

### Task 5.1: Customer CRUD + Stats

**Files:**
- Create: `api/queries/customers.sql`
- Create: `api/internal/handler/customers.go`
- Create: `api/internal/handler/customers_test.go`

Implement:
- CRUD endpoints
- `GET /outlets/:oid/customers/:id/stats` — derived stats (total spend, visits, avg ticket, top items)
- `GET /outlets/:oid/customers/:id/orders` — order history
- Phone search with partial match

**Commit:** `feat: add customer CRUD with derived CRM stats`

---

### Task 5.2: Reports Endpoints

**Files:**
- Create: `api/queries/reports.sql`
- Create: `api/internal/handler/reports.go`
- Create: `api/internal/handler/reports_test.go`

Implement:
- `GET /outlets/:oid/reports/daily-sales` — date range, returns per-day totals
- `GET /outlets/:oid/reports/product-sales` — top sellers by qty and revenue
- `GET /outlets/:oid/reports/payment-summary` — breakdown by payment method
- `GET /outlets/:oid/reports/hourly-sales` — sales per hour (for peak hours)
- `GET /reports/outlet-comparison` — owner only, cross-outlet comparison

All reports accept `start_date` and `end_date` query params.

**Commit:** `feat: add sales and analytics report endpoints`

---

### Task 5.3: WebSocket for Live Order Updates

**Files:**
- Create: `api/internal/ws/hub.go`
- Create: `api/internal/ws/client.go`
- Create: `api/internal/ws/hub_test.go`

Implement:
- WebSocket hub with per-outlet channels
- `WS /ws/outlets/:oid/orders` — authenticated via token query param
- Events: `order.created`, `order.updated`, `item.updated`, `order.paid`
- Hub broadcasts when order handlers modify state

**Commit:** `feat: add WebSocket hub for live order updates`

---

## Milestone 6: Go API — Router Assembly & Integration Test

### Task 6.1: Wire Everything Together

**Files:**
- Modify: `api/cmd/server/main.go`
- Create: `api/internal/router/router.go`

Wire all handlers into Chi router with:
- Auth middleware on protected routes
- Outlet scoping middleware on `/outlets/:oid/*` routes
- Owner-only middleware on admin routes
- CORS configuration for web admin
- WebSocket upgrade route

**Commit:** `feat: wire all routes with middleware chain`

---

### Task 6.2: Integration Tests

**Files:**
- Create: `api/internal/handler/integration_test.go`

End-to-end test:
1. Create outlet
2. Create user (cashier)
3. Login
4. Create category + product + variants + modifiers
5. Create order with items
6. Add multi-payment
7. Verify order completes
8. Check customer stats

Uses test database (Docker).

**Commit:** `feat: add end-to-end integration test`

---

## Milestone 7: Docker Production Setup

### Task 7.1: Production Docker Compose

**Files:**
- Create: `docker/docker-compose.yml`
- Create: `docker/Dockerfile.api`
- Create: `docker/Dockerfile.admin`

```dockerfile
# docker/Dockerfile.api
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ .
RUN CGO_ENABLED=0 go build -o server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/server /server
COPY --from=builder /app/migrations /migrations
EXPOSE 8081
CMD ["/server"]
```

```yaml
# docker/docker-compose.yml
services:
  pos-api:
    build:
      context: ..
      dockerfile: docker/Dockerfile.api
    container_name: pos-api
    env_file: .env
    ports:
      - "8081:8081"
    depends_on:
      pos-db:
        condition: service_healthy
    networks:
      - pos-internal
      - proxy
    restart: unless-stopped

  pos-admin:
    build:
      context: ..
      dockerfile: docker/Dockerfile.admin
    container_name: pos-admin
    environment:
      - API_URL=http://pos-api:8081
    ports:
      - "3001:3000"
    networks:
      - pos-internal
      - proxy
    restart: unless-stopped

  pos-db:
    image: postgres:16-alpine
    container_name: pos-db
    env_file: .env
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U pos -d pos_db"]
      interval: 10s
      timeout: 3s
      retries: 5
    networks:
      - pos-internal
    restart: unless-stopped

volumes:
  pgdata:

networks:
  pos-internal:
  proxy:
    external: true
```

**Commit:** `feat: add production Docker Compose with multi-stage builds`

---

### Task 7.2: Backup Script

**Files:**
- Create: `docker/backup.sh`

```bash
#!/bin/bash
BACKUP_DIR="/home/iqbal/backups/pos"
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR
docker exec pos-db pg_dump -U pos pos_db | gzip > "$BACKUP_DIR/pos_$DATE.sql.gz"
# Retain last 30 days
find $BACKUP_DIR -name "*.sql.gz" -mtime +30 -delete
echo "Backup completed: pos_$DATE.sql.gz"
```

Set up cron: `0 2 * * * /home/iqbal/docker/pos/backup.sh`

**Commit:** `feat: add PostgreSQL backup script with 30-day retention`

---

## Milestone 8: Android POS App — see [android-pos-plan.md](2026-02-06-android-pos-plan.md)

> **PRIORITY.** Cashier-facing Android app. Tasks 8.1–8.8.

---

## Milestone 9: SvelteKit Web Admin — see [sveltekit-admin-plan.md](2026-02-06-sveltekit-admin-plan.md)

> Admin panel for menu management, orders, CRM, reports. Tasks 9.1–9.8.

---

## Milestone 10: Final Integration & Deployment

### Task 10.1: Deploy to VPS

**Steps:**
1. SSH to VPS
2. Clone repo to `~/docker/pos/`
3. Copy `.env.example` to `.env`, fill in production values
4. Build and start: `docker compose up -d --build`
5. Run migration: `docker exec pos-api /server migrate`
6. Add Cloudflare DNS records:
   - `A pos-api.nasibakarkiwari.com → 43.173.30.193` (proxied)
   - `A pos.nasibakarkiwari.com → 43.173.30.193` (proxied)
7. Add NPM proxy hosts:
   - `pos-api.nasibakarkiwari.com` → `pos-api:8081` (with WebSocket config)
   - `pos.nasibakarkiwari.com` → `pos-admin:3001`
8. Set up backup cron
9. Seed initial data: create outlet, create owner user

**Commit:** `feat: add deployment documentation`

---

### Task 10.2: Seed Script

**Files:**
- Create: `api/cmd/seed/main.go`

Create initial data:
- 1 outlet (Kiwari Nasi Bakar - main branch)
- 1 owner user (your login)
- Sample categories and products for testing

**Commit:** `feat: add database seed script`

---

## Execution Order & Dependencies

```
Milestone 1 (Scaffolding + DB) ✅
    ↓
Milestone 2 (Auth + Middleware) ✅
    ↓
Milestone 3 (Menu API) ✅
    ↓
Milestone 4 (Orders + Payments API) ✅
    ↓
Milestone 5 (CRM + Reports + WebSocket) ✅
    ↓
Milestone 6 (Router + Integration Test)
    ↓
Milestone 7 (Docker Production)
    ↓ (API complete, build clients)
    ├── Milestone 8 (Android POS) ← PRIORITY — android-pos-plan.md
    └── Milestone 9 (SvelteKit Admin) — sveltekit-admin-plan.md
    ↓
Milestone 10 (Deploy + Seed)
```

**Android POS (M8) is the priority.** SvelteKit Admin (M9) can run in parallel or after.

---

## Notes for Implementer

- **Go router:** Use `go-chi/chi` — lightweight, stdlib-compatible, good middleware support.
- **sqlc:** All database queries are in `.sql` files, Go code is generated. Never write raw SQL in Go handlers.
- **Decimal handling:** Use `shopspring/decimal` for all money fields. Never use float64.
- **Testing:** Use `testcontainers-go` for integration tests with real PostgreSQL.
- **Android:** Requires Android Studio for build/test. CI can use GitHub Actions with Android emulator.
- **SvelteKit:** Use server-side rendering (SSR) for initial load, then client-side navigation. API calls from server-side `+page.server.ts` to avoid CORS in most cases.
- **Passwords:** bcrypt with cost 12. Never log or return passwords.
- **Order numbers:** Generate in Go service layer using `SELECT COUNT(*) + 1 FROM orders WHERE outlet_id = $1 AND created_at::date = CURRENT_DATE` — simple sequential per outlet per day.
