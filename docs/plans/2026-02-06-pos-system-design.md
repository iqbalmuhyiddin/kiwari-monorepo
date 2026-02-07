# Kiwari POS System Design

> Design document for Kiwari's custom POS system.
> Created: 2026-02-06

## 1. Overview

Custom POS system for Kiwari (F&B / Nasi Bakar business) replacing KasirPintar. Multi-outlet, order management with kitchen printing, multi-payment, catering bookings, and simple CRM.

### System Components (v1)

```
Android POS App          Web Admin Panel
(Kotlin native)          (SvelteKit)
- Cashier order entry    - Menu/outlet/user CRUD
- Multi-payment          - Sales reports
- Catering booking/DP    - Customer CRM
- Thermal printing       - Live order monitoring
- Simple CRM lookup      - Catering management
       │                        │
       └────────────┬───────────┘
                    │ REST + WebSocket
           ┌────────▼────────┐
           │  Go API Server  │
           └────────┬────────┘
           ┌────────▼────────┐
           │   PostgreSQL    │
           └─────────────────┘
```

### Target Devices

- **POS**: Android phones (not tablets) — UI optimized for small screens
- **Admin**: Desktop/laptop browser
- **Kitchen**: Thermal printer (Bluetooth) — no KDS in v1

---

## 2. Monorepo Setup

Single monorepo with Makefile as the glue layer across three languages.

```
pos-superpower/
├── api/                        # Go API (own go.mod)
├── admin/                      # SvelteKit (own package.json, pnpm)
├── android/                    # Android POS (own build.gradle.kts)
├── docker/                     # Docker Compose (dev + production)
├── docs/                       # Design docs, plans, old references
├── Makefile                    # Root task runner (aliases for native commands)
├── .gitignore                  # Covers all three projects
├── .editorconfig               # Go=tabs, Kotlin=4spaces, everything else=2spaces
└── CLAUDE.md
```

**Principles:**
- Each project is self-contained — own dependency file, own build system
- No shared code between projects — API contracts are the only coupling
- Makefile provides unified `make <target>` commands across all three
- Android Studio opens `android/` subfolder as project root
- Single `.gitignore` at root covers Go, Node, Android, Docker, IDE files

---

## 3. Brand Identity & Design Tokens

Aesthetic: **"Bold + Clean"** — Bright white backgrounds, strong green CTAs, yellow category accents. Light-only (no dark theme — POS terminal doesn't need it).

> Revised 2026-02-07. Full design spec: `docs/plans/2026-02-07-android-theme-redesign.md`

### Color Palette

| Token | Hex | Usage |
|-------|-----|-------|
| Primary Green | `#0c7721` | CTAs, selected states, checkboxes, primary buttons |
| Primary Yellow | `#ffd500` | Category chips (selected), header accent |
| Error Red | `#dc2626` | Error states, destructive actions |
| Text Primary | `#1a1a1a` | Headings, product names, prices |
| Text Secondary | `#6b7280` | Subtitles, hints, captions |
| Border Color | `#e5e7eb` | Card borders, dividers |
| Surface Color | `#f8f9fa` | Card bg, input bg, avatars |
| White | `#ffffff` | Backgrounds, on-primary text |
| Error Bg Tint | `#fef2f2` | Error field background tint |

### Typography

- **Font**: Roboto (system default, no custom font files)
- Size range: 11–20sp (tight for POS readability)
- Headings: Bold (700), 18–20sp
- Body: Regular (400), 13–15sp
- Labels: Medium (500), 11–13sp

### Shapes

| Element | Radius |
|---------|--------|
| Chips, inputs | 8dp (extraSmall) |
| Buttons | 10dp (small) |
| Cards | 12dp (medium) |
| Bottom sheets | 16dp (large) |

### Component Styles

- **Buttons**: 10dp rounded corners, green primary, 0.38 alpha disabled
- **Cards**: 12dp rounded corners, 1dp border (`#e5e7eb`)
- **Category chips**: Yellow selected, surfaceVariant unselected, 8dp rounded
- **Avatars**: 56dp, 8dp rounded rect, surfaceVariant background
- **Elevation**: Bottom bar 4dp shadow, cards 0dp (border-based)
- **Light-only**: No dark theme

---

## 4. Data Model

### Enums

```
Enum user_role {
  OWNER
  MANAGER
  CASHIER
  KITCHEN
}

Enum order_type {
  DINE_IN
  TAKEAWAY
  DELIVERY
  CATERING
}

Enum order_status {
  NEW
  PREPARING
  READY
  COMPLETED
  CANCELLED
}

Enum order_item_status {
  PENDING
  PREPARING
  READY
}

Enum catering_status {
  BOOKED
  DP_PAID
  SETTLED
  CANCELLED
}

Enum kitchen_station {
  GRILL
  BEVERAGE
  RICE
  DESSERT
}

Enum payment_method {
  CASH
  QRIS
  TRANSFER
}

Enum payment_status {
  PENDING
  COMPLETED
  FAILED
}

Enum discount_type {
  PERCENTAGE
  FIXED_AMOUNT
}
```

### Core Tables

```
Table outlets {
  id uuid [pk]
  name varchar(255) [not null]
  address text
  phone varchar(20)
  is_active boolean [default: true]
  created_at timestamp [default: now()]
  updated_at timestamp [default: now()]
}

Table users {
  id uuid [pk]
  outlet_id uuid [ref: > outlets.id, not null]
  email varchar(255) [unique, not null]
  hashed_password varchar(255) [not null]
  full_name varchar(255) [not null]
  role user_role [not null]
  pin varchar(6)
  is_active boolean [default: true]
  created_at timestamp [default: now()]
  updated_at timestamp [default: now()]

  indexes {
    email
    (outlet_id, role)
  }
}
```

### Menu System

```
Table categories {
  id uuid [pk]
  outlet_id uuid [ref: > outlets.id, not null]
  name varchar(100) [not null]
  description text
  sort_order int [default: 0]
  is_active boolean [default: true]
  created_at timestamp [default: now()]

  indexes {
    (outlet_id, sort_order)
  }
}

Table products {
  id uuid [pk]
  outlet_id uuid [ref: > outlets.id, not null]
  category_id uuid [ref: > categories.id, not null]
  name varchar(255) [not null]
  description text
  base_price decimal(12,2) [not null]
  image_url varchar(500)
  station kitchen_station
  preparation_time int               // minutes, nullable
  is_combo boolean [default: false]
  is_active boolean [default: true]
  created_at timestamp [default: now()]
  updated_at timestamp [default: now()]

  indexes {
    (outlet_id, category_id)
    is_active
  }
}

Table variant_groups {
  id uuid [pk]
  product_id uuid [ref: > products.id, not null]
  name varchar(100) [not null]        // "Size", "Spice Level"
  is_required boolean [default: true]  // must pick one
  is_active boolean [default: true]
  sort_order int [default: 0]

  indexes {
    product_id
  }
}

Table variants {
  id uuid [pk]
  variant_group_id uuid [ref: > variant_groups.id, not null]
  name varchar(100) [not null]        // "S", "M", "L"
  price_adjustment decimal(12,2) [default: 0]
  is_active boolean [default: true]
  sort_order int [default: 0]

  indexes {
    variant_group_id
  }
}

Table modifier_groups {
  id uuid [pk]
  product_id uuid [ref: > products.id, not null]
  name varchar(100) [not null]        // "Toppings", "Side Dish"
  min_select int [default: 0]         // 0 = optional
  max_select int                      // null = unlimited
  is_active boolean [default: true]
  sort_order int [default: 0]

  indexes {
    product_id
  }
}

Table modifiers {
  id uuid [pk]
  modifier_group_id uuid [ref: > modifier_groups.id, not null]
  name varchar(100) [not null]        // "Extra Cheese", "Nasi Putih"
  price decimal(12,2) [default: 0]
  is_active boolean [default: true]
  sort_order int [default: 0]

  indexes {
    modifier_group_id
  }
}

Table combo_items {
  id uuid [pk]
  combo_id uuid [ref: > products.id, not null]     // the combo product
  product_id uuid [ref: > products.id, not null]    // child product
  quantity int [default: 1]
  sort_order int [default: 0]

  indexes {
    combo_id
  }
}
```

### Orders

```
Table orders {
  id uuid [pk]
  outlet_id uuid [ref: > outlets.id, not null]
  order_number varchar(20) [not null]    // daily sequential: "KWR-001"
  customer_id uuid [ref: > customers.id]
  order_type order_type [not null]
  status order_status [not null, default: 'NEW']
  table_number varchar(20)
  notes text

  // Money
  subtotal decimal(12,2) [not null]
  discount_type discount_type            // nullable = no discount
  discount_value decimal(12,2)           // 10 for 10% or 10000 for fixed
  discount_amount decimal(12,2) [default: 0]   // computed final discount
  tax_amount decimal(12,2) [default: 0]
  total_amount decimal(12,2) [not null]

  // Catering-specific (null for non-catering)
  catering_date timestamp
  catering_status catering_status
  catering_dp_amount decimal(12,2)       // required down payment

  // Delivery-specific (null for non-delivery)
  delivery_platform varchar(50)          // "GrabFood", "GoFood", "Self"
  delivery_address text

  // Audit
  created_by uuid [ref: > users.id, not null]
  created_at timestamp [default: now()]
  updated_at timestamp [default: now()]
  completed_at timestamp

  indexes {
    (outlet_id, created_at)
    (outlet_id, order_number) [unique]
    status
    customer_id
    catering_status
  }
}

Table order_items {
  id uuid [pk]
  order_id uuid [ref: > orders.id, not null]
  product_id uuid [ref: > products.id, not null]
  variant_id uuid [ref: > variants.id]
  quantity int [not null]
  unit_price decimal(12,2) [not null]      // snapshot at order time
  discount_type discount_type
  discount_value decimal(12,2)
  discount_amount decimal(12,2) [default: 0]
  subtotal decimal(12,2) [not null]        // (unit_price * qty) - discount
  notes text
  status order_item_status [not null, default: 'PENDING']
  station kitchen_station

  indexes {
    order_id
    status
  }
}

Table order_item_modifiers {
  id uuid [pk]
  order_item_id uuid [ref: > order_items.id, not null]
  modifier_id uuid [ref: > modifiers.id, not null]
  quantity int [default: 1]
  unit_price decimal(12,2) [not null]      // snapshot at order time

  indexes {
    order_item_id
  }
}
```

### Payments (multi-payment per order)

```
Table payments {
  id uuid [pk]
  order_id uuid [ref: > orders.id, not null]
  payment_method payment_method [not null]
  amount decimal(12,2) [not null]
  status payment_status [not null, default: 'COMPLETED']
  reference_number varchar(100)            // for QRIS/transfer ref

  // Cash-specific
  amount_received decimal(12,2)            // null for non-cash
  change_amount decimal(12,2)              // null for non-cash

  processed_by uuid [ref: > users.id, not null]
  processed_at timestamp [default: now()]

  indexes {
    order_id
    payment_method
  }
}
```

### Customers (Simple CRM)

```
Table customers {
  id uuid [pk]
  outlet_id uuid [ref: > outlets.id, not null]
  name varchar(255) [not null]
  phone varchar(20) [not null]
  email varchar(255)
  notes text
  is_active boolean [default: true]
  created_at timestamp [default: now()]
  updated_at timestamp [default: now()]

  indexes {
    (outlet_id, phone) [unique]
  }
}

// CRM data is derived from orders — no separate tables:
// - Total spend:     SUM(orders.total_amount) WHERE customer_id = X
// - Visit count:     COUNT(orders) WHERE customer_id = X
// - Last visit:      MAX(orders.created_at) WHERE customer_id = X
// - Favorite items:  GROUP BY product_id on order_items, ORDER BY count
// - Average ticket:  AVG(orders.total_amount) WHERE customer_id = X
```

### Soft Delete Policy

| Table | Strategy | Reason |
|-------|----------|--------|
| outlets | `is_active=false` | Everything references it |
| users | `is_active=false` | Orders reference `created_by` |
| categories | `is_active=false` | Products reference it |
| products | `is_active=false` | Order items reference it |
| variant_groups | `is_active=false` | Order items reference variants |
| variants | `is_active=false` | Order items reference it |
| modifier_groups | `is_active=false` | Order item modifiers reference it |
| modifiers | `is_active=false` | Order item modifiers reference it |
| customers | `is_active=false` | Orders reference it |
| combo_items | Hard delete OK | Only referenced by combo product |
| orders | Status → CANCELLED | Financial records, never delete |
| order_items | Hard delete OK | Only before submission |
| payments | Never delete | Financial audit trail |

**Total: 14 tables for v1.**

---

## 5. API Design

### Auth

```
POST   /auth/login              // email + password → JWT
POST   /auth/pin-login          // outlet_id + pin → JWT (quick cashier switch)
POST   /auth/refresh            // refresh token
```

### Outlets (Owner/Manager)

```
GET    /outlets
GET    /outlets/:id
POST   /outlets
PUT    /outlets/:id
```

### Users

```
GET    /outlets/:oid/users
POST   /outlets/:oid/users
PUT    /outlets/:oid/users/:id
DELETE /outlets/:oid/users/:id          // soft delete
```

### Menu

```
GET    /outlets/:oid/categories
POST   /outlets/:oid/categories
PUT    /outlets/:oid/categories/:id
DELETE /outlets/:oid/categories/:id

GET    /outlets/:oid/products            // full tree: variants + modifiers nested
GET    /outlets/:oid/products/:id
POST   /outlets/:oid/products
PUT    /outlets/:oid/products/:id
DELETE /outlets/:oid/products/:id

// Variant groups & variants
POST   /outlets/:oid/products/:pid/variant-groups
PUT    /outlets/:oid/products/:pid/variant-groups/:id
DELETE /outlets/:oid/products/:pid/variant-groups/:id
POST   /outlets/:oid/products/:pid/variant-groups/:vgid/variants
PUT    /outlets/:oid/products/:pid/variant-groups/:vgid/variants/:id

// Modifier groups & modifiers
POST   /outlets/:oid/products/:pid/modifier-groups
PUT    /outlets/:oid/products/:pid/modifier-groups/:id
DELETE /outlets/:oid/products/:pid/modifier-groups/:id
POST   /outlets/:oid/products/:pid/modifier-groups/:mgid/modifiers
PUT    /outlets/:oid/products/:pid/modifier-groups/:mgid/modifiers/:id

// Combos
POST   /outlets/:oid/products/:pid/combo-items
DELETE /outlets/:oid/products/:pid/combo-items/:id
```

### Orders

```
GET    /outlets/:oid/orders              // filters: status, type, date range
GET    /outlets/:oid/orders/:id
POST   /outlets/:oid/orders              // create order + items atomically
PUT    /outlets/:oid/orders/:id
PATCH  /outlets/:oid/orders/:id/status
DELETE /outlets/:oid/orders/:id          // cancel

POST   /outlets/:oid/orders/:id/items
PUT    /outlets/:oid/orders/:id/items/:iid
DELETE /outlets/:oid/orders/:id/items/:iid
PATCH  /outlets/:oid/orders/:id/items/:iid/status
```

### Payments

```
POST   /outlets/:oid/orders/:id/payments  // add payment (multi-payment)
GET    /outlets/:oid/orders/:id/payments
```

### Customers (CRM)

```
GET    /outlets/:oid/customers            // search by phone/name
GET    /outlets/:oid/customers/:id
POST   /outlets/:oid/customers
PUT    /outlets/:oid/customers/:id
GET    /outlets/:oid/customers/:id/stats
GET    /outlets/:oid/customers/:id/orders
```

### Reports

```
GET    /outlets/:oid/reports/daily-sales
GET    /outlets/:oid/reports/product-sales
GET    /outlets/:oid/reports/payment-summary
GET    /outlets/:oid/reports/hourly-sales
GET    /reports/outlet-comparison          // owner only
```

### WebSocket

```
WS     /ws/outlets/:oid/orders
// Events:
//   order.created   → new order
//   order.updated   → status change
//   item.updated    → item status change
//   order.paid      → payment completed
```

### Design Principles

- Everything scoped under `/outlets/:oid` — middleware checks JWT outlet claim
- Owner role bypasses outlet scope
- `GET /products` returns full tree in one call (no N+1)
- Order creation is atomic (single POST with items array)
- WebSocket per outlet for live updates

---

## 6. Screen Flows — Android POS

### Login Screen

- Green background (`#0c7721`)
- White card with email + password form
- "Quick PIN" option for shift changes (6-digit PIN)

### Menu Screen (Main)

KasirPintar-style full-width list optimized for phone screens:

```
┌───────────────────────────────────────┐
│  ☰  Transaksi            🔔  ☆  ⋮   │
│  🔍  ➕  📷                           │
│  [Semua] [Nasi Bakar] [Minuman] [>>>] │  ← horizontal scroll categories
│  ─────────────────────────────────────│
│                                       │
│  ┌──┐  Besek                          │
│  │Be│  Rp 5.000                       │
│  └──┘                                 │
│  ┌──┐  Nasi Bakar Pedas        ┌───┐ │
│  │Na│  Rp 18.000               │ 2 │ │ ← qty badge
│  └──┘                          └───┘ │
│  ┌──┐  Ayam Bakar              ┌───┐ │
│  │Ab│  Rp 35.000          ⚙   │ 1 │ │ ← ⚙ = has required variants
│  └──┘                          └───┘ │
│                                       │
│ ┌─────────────────────────────────┐   │
│ │ 🛒 6 Barang              LANJUT ▶ │ ← sticky bottom bar
│ └─────────────────────────────────┘   │
└───────────────────────────────────────┘
```

### Interaction Model

| Action | Simple product (no required options) | Product with required variants/modifiers |
|--------|--------------------------------------|------------------------------------------|
| **Tap** | +1 qty instantly, badge appears | Opens customization bottom sheet |
| **Long press** | Quick popup (qty, add-on, discount, note) | Quick popup (same) |
| **Qty badge** | Shows on right side of list item | Shows on right side |

### Long-Press Quick Popup

```
┌───────────────────────────────────────┐
│  ┌──┐  Nasi Bakar Cumi     Rp 21.000 │
│  │Na│  NBCI                           │
│  └──┘                                 │
│       ┌───┐         ┌───┐            │
│       │ − │    3    │ + │            │
│       └───┘         └───┘            │
│  ┌──────────────┐                     │
│  │  + Add On    │                     │
│  └──────────────┘                     │
│  ☐  Ubah diskon per-item             │
│  ☐  Tambah catatan                   │
│  ┌────────────┐  ┌────────────────┐  │
│  │ EDIT DETAIL │  │      OK       │  │
│  └────────────┘  └────────────────┘  │
└───────────────────────────────────────┘
```

### Product Customization Bottom Sheet

For products with required variants/modifiers, or when tapping "EDIT DETAIL":

```
┌───────────────────────────────────────┐
│  Ayam Bakar Original         Rp 35k  │
│  Size: [Regular] [Large +10k]         │
│  Spice: [Mild] [Medium] [Hot]         │
│  Toppings (max 3):                    │
│  [+Sambal 3k] [+Keju 5k] [+Telur 5k]│
│  Side: [Nasi Putih 5k] [Nasi Uduk 7k]│
│  Qty: [−] 1 [+]   Note: [________]   │
│  [ADD TO CART — Rp 50.000]            │
└───────────────────────────────────────┘
```

### Cart Screen (Separate Page)

```
┌───────────────────────────────────────┐
│  ←  Keranjang                    🗑   │
│  Order Type: [Dine-in ▼]  Table: [3] │
│  Customer: [🔍 search / + add]       │
│  ─────────────────────────────────────│
│  1x Ayam Bakar Original      50.000  │
│     L · Hot · +Sambal · Nasi Uduk    │
│     [edit] [hapus]         [−] 1 [+] │
│  2x Es Teh Manis             16.000  │
│     [edit] [hapus]         [−] 2 [+] │
│  ─────────────────────────────────────│
│  Subtotal                    66.000  │
│  Diskon: [Tidak ada ▼]          -0   │
│  Total                    Rp 66.000  │
│ ┌─────────────────────────────────┐   │
│ │        BAYAR  Rp 66.000        │   │
│ └─────────────────────────────────┘   │
└───────────────────────────────────────┘
```

### Payment Screen

```
┌───────────────────────────────────────┐
│  ←  Pembayaran                        │
│  Total: Rp 66.000                     │
│  ─────────────────────────────────────│
│  Payment 1:                           │
│  [CASH ●] [QRIS ○] [TRANSFER ○]     │
│  Amount: [50.000]                     │
│  Diterima: [100.000]                  │
│  Kembalian: Rp 50.000                 │
│  [+ Tambah pembayaran lain]           │
│  Payment 2:                           │
│  [CASH ○] [QRIS ●] [TRANSFER ○]     │
│  Amount: [16.000]                     │
│  Ref: [___________]                   │
│  ─────────────────────────────────────│
│  Dibayar: Rp 66.000 | Sisa: Rp 0     │
│ ┌─────────────────────────────────┐   │
│ │     ✓ SELESAI & CETAK          │   │
│ └─────────────────────────────────┘   │
└───────────────────────────────────────┘
```

### Catering Booking Screen

Only shown when order type is CATERING:

```
┌───────────────────────────────────────┐
│  Customer: [required for catering]     │
│  Catering Date: [date picker]          │
│  Delivery Address: [____________]      │
│  Notes: [____________]                 │
│  Total: Rp 500.000                     │
│  Min DP (50%): Rp 250.000             │
│  DP Payment:                           │
│  [CASH ○] [QRIS ○] [TRANSFER ●]      │
│  Amount: [250.000]                     │
│  Ref: [TRF-001234]                     │
│  [✓ BOOK & RECORD DP]                 │
└───────────────────────────────────────┘
```

---

## 7. Screen Flows — Web Admin (SvelteKit)

### Sidebar Navigation

| Page | Description |
|------|-------------|
| Dashboard | Today's KPIs, hourly sales chart, live active orders (WebSocket) |
| Pesanan | Order list with filters (status, type, date), catering tab, order detail modal |
| Menu | Category + product CRUD, variant groups, modifier groups, combo management |
| Pelanggan | Customer list, search, stats (spend, visits, favorites), order history |
| Pengguna | Staff CRUD per outlet, role assignment |
| Outlet | Outlet CRUD (Owner only) |
| Laporan | Daily sales, product sales, payment summary, hourly sales, outlet comparison, CSV export |
| Pengaturan | Tax rate, receipt template, printer guidance, order number format |

### Role-Based Access

- **Owner**: All outlets, outlet comparison reports, full user management
- **Manager**: Own outlet only, menu and staff management for that outlet

---

## 8. Deployment Architecture

### Server: Tencent Cloud VPS

- **OS**: Ubuntu 24.04 LTS
- **RAM**: ~2GB
- **Domain**: nasibakarkiwari.com
- **DNS/CDN**: Cloudflare (SSL Full Strict, WebSockets ON)
- **Reverse Proxy**: Nginx Proxy Manager (existing)

### Existing Services (keep as-is)

NPM, Portainer, n8n, NocoDB, WAHA (~430MB RAM used)

### POS Services (~200MB additional)

```
~/docker/pos/
├── docker-compose.yml
├── .env                     # DB creds, JWT secret
├── data/
│   └── postgres/            # persistent volume
└── backups/

Services:
  pos-api      Go binary        :8081   ~20-30MB RAM
  pos-admin    SvelteKit Node   :3001   ~50-80MB RAM
  pos-db       PostgreSQL 16    :5432   ~100MB RAM (internal only)
```

### NPM Proxy Hosts

| Domain | Forward To | Notes |
|--------|-----------|-------|
| `pos-api.nasibakarkiwari.com` | `pos-api:8081` | WebSocket config in Advanced tab |
| `pos.nasibakarkiwari.com` | `pos-admin:3001` | Standard SSL |

### Cloudflare DNS

```
A  pos-api.nasibakarkiwari.com  → 43.173.30.193 (proxied)
A  pos.nasibakarkiwari.com      → 43.173.30.193 (proxied)
```

### Why Separate PostgreSQL (not reusing MariaDB)

- Isolation: POS data separate from other services
- PostgreSQL better for JSON-heavy modifier/variant queries
- Independent backup/restore lifecycle

### Backup Strategy

```bash
# Cron: daily 2am
docker exec pos-db pg_dump -U pos pos_db | gzip > ~/backups/pos/pos_$(date +%Y%m%d).sql.gz
# Retain 30 days, sync to cloud storage
```

---

## 9. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Internet down at outlet | POS can't create orders | v1 accepts this risk. v2: offline-first with local SQLite + sync |
| Printer disconnect | Kitchen misses orders | Bluetooth auto-reconnect + order list on POS as fallback |
| VPS down | Everything stops | Daily backups, monitoring alert, restore to new VPS <1hr |
| JWT token theft | Unauthorized access | Short-lived tokens (15min), refresh rotation, HTTPS only |
| Cashier enters wrong price | Revenue loss | Prices locked from menu. Temp price override deferred to v2 with manager auth |

---

## 10. Roadmap

### v1 — Build Now

| Component | Tech | Scope |
|-----------|------|-------|
| Android POS | Kotlin native | Orders, multi-payment, discounts, catering DP, CRM, thermal print |
| Web Admin | SvelteKit | Dashboard, menu CRUD, orders, CRM, reports, live monitoring |
| API | Go | REST + WebSocket, JWT auth, multi-outlet |
| Database | PostgreSQL 16 | 14 tables |
| Infra | Docker Compose + NPM | Self-hosted on Tencent Cloud VPS |

### v2 — Next Iteration

| Feature | Notes |
|---------|-------|
| KDS app (Android Kotlin) | Kitchen display with order flow |
| Promotions system | Promo codes, scoped (order/product/category), date ranges, usage tracking |
| Temp price override | Manager PIN approval + audit log |
| Delivery platform sync | GrabFood/GoFood/ShopeeFood API integration |
| Accounting module | Web admin — replaces Google Sheets, journal entries |
| Offline mode | Local SQLite + sync for internet outages |

### v2 Promotions Schema (Pre-designed)

```
Enum promotion_type {
  PERCENTAGE
  FIXED_AMOUNT
  BUY_X_GET_Y
}

Enum promotion_scope {
  ORDER
  PRODUCT
  CATEGORY
}

Table promotions {
  id uuid [pk]
  outlet_id uuid [ref: > outlets.id, not null]
  name varchar(255) [not null]
  code varchar(50)
  type promotion_type [not null]
  scope promotion_scope [not null]
  value decimal(12,2) [not null]
  min_purchase decimal(12,2)
  max_discount decimal(12,2)
  start_date timestamp [not null]
  end_date timestamp [not null]
  is_active boolean [default: true]
  usage_limit int
  usage_count int [default: 0]
  created_at timestamp [default: now()]

  indexes {
    (outlet_id, code) [unique]
    (outlet_id, is_active)
  }
}

Table promotion_products {
  id uuid [pk]
  promotion_id uuid [ref: > promotions.id, not null]
  product_id uuid [ref: > products.id, not null]
}

Table promotion_usages {
  id uuid [pk]
  promotion_id uuid [ref: > promotions.id, not null]
  order_id uuid [ref: > orders.id, not null]
  customer_id uuid [ref: > customers.id]
  discount_amount decimal(12,2) [not null]
  used_at timestamp [default: now()]
}
```
