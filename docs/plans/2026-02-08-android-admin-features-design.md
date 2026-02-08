# Android POS — Admin Features

> Design document for adding admin capabilities to the Android POS app.
> Created: 2026-02-08

## 1. Overview

The Android POS app is currently cashier-only (take orders, pay, print). This design adds admin capabilities so that both the owner (checking in remotely from phone) and outlet managers (on-site, phone may be their only device) can manage the business from the same app.

### Target Users

- **Owner**: Checks sales, reviews customers, manages staff from phone when away from laptop
- **Outlet Managers**: On-site, may share the POS phone with cashiers or have their own device

### Design Constraints

- Phone screen (not tablet) — complex forms need careful layout
- Shared device scenario — manager logs in with own credentials, must not clutter cashier flow
- POS-first — ordering is 90% of usage, admin features are secondary

### What's in scope (Android)

| Feature | Priority |
|---------|----------|
| Drawer navigation with role-based filtering | Foundation |
| Reports (Laporan) — full analytical depth | High |
| CRM (Pelanggan) — list + detail with stats | High |
| Order History (Riwayat) — completed/cancelled orders | High |
| Menu CRUD — full depth (categories → products → variants → modifiers → combos) | High |
| Staff Management (Pengguna) — basic CRUD | Medium |
| Role-based access control | Foundation |

### What stays web admin only

| Feature | Why |
|---------|-----|
| Outlet management | Owner-only, rare, complex |
| CSV/data export | Not useful on phone |
| Complex settings (tax, receipt templates) | Set-once configuration |

---

## 2. Navigation: Drawer (Hamburger Menu)

The cashier ordering flow (Menu screen) remains the home screen. Admin features are accessed via a hamburger drawer that slides out from the left.

### Why drawer over bottom navigation

- POS ordering is the primary use case — it stays front and center with zero extra taps
- Admin features are secondary — accessed intentionally, not always visible
- On a shared device, cashiers see a clean ordering screen without admin clutter
- The hamburger icon already exists in the Menu screen mock

### Drawer Layout

```
┌───────────────────────────┐
│  🟢 Kiwari POS            │
│  Budi (MANAGER)           │
│  Outlet: Nasi Bakar Dago  │
│  ─────────────────────────│
│  📋  Pesanan              │
│  📊  Laporan              │
│  🍽️  Menu                 │
│  👥  Pelanggan            │
│  👤  Pengguna             │
│  🖨️  Printer              │
│  ─────────────────────────│
│  🚪  Keluar               │
└───────────────────────────┘
```

**Header**: Shows logged-in user's name, role badge, and outlet name. Important on shared devices.

**Behavior**: Tapping a drawer item opens a full-screen page. Closing the page (back arrow) returns to the Menu screen. The drawer is an overlay — it doesn't replace the home screen.

### Role-Based Visibility

| Drawer Item | OWNER | MANAGER | CASHIER | KITCHEN |
|-------------|-------|---------|---------|---------|
| Pesanan | Yes | Yes | Yes | No |
| Laporan | Yes | Yes | No | No |
| Menu | Yes | Yes | No | No |
| Pelanggan | Yes | Yes | No | No |
| Pengguna | Yes | Yes | No | No |
| Printer | Yes | Yes | Yes | No |

**Implementation**: A single `isFeatureVisible(feature, role)` utility. The drawer filters items based on the logged-in user's role from `TokenRepository`. Each screen also does its own role check as a safety net.

---

## 3. Reports (Laporan)

Full analytical depth on mobile — not just a dashboard glance. Uses the 5 existing report API endpoints (all support date range filtering).

### Screen Layout

```
┌───────────────────────────────────────┐
│  ←  Laporan                          │
│  ─────────────────────────────────────│
│  [Hari ini] [Kemarin] [7 Hari] [▼]  │
│  ─────────────────────────────────────│
│  [Penjualan] [Produk] [Pembayaran]  │
│  ─────────────────────────────────────│
│                                       │
│  💰 Total Penjualan                  │
│  Rp 2.450.000                        │
│                                       │
│  📦 Total Pesanan                    │
│  47 pesanan                          │
│                                       │
│  🧾 Rata-rata                        │
│  Rp 52.128                           │
│                                       │
│  ─── Penjualan per Jam ──────────── │
│  ▁▃▅▇█▇▅▃▁                          │
│  10 11 12 13 14 15 16 17 18         │
│                                       │
└───────────────────────────────────────┘
```

### Tabs

1. **Penjualan** (Sales) — KPI cards (revenue, order count, avg ticket) + hourly bar chart. Data: `daily-sales` + `hourly-sales` endpoints.

2. **Produk** (Products) — Sorted list by qty sold or revenue. Each row: product name, qty sold, revenue. Data: `product-sales` endpoint.

3. **Pembayaran** (Payments) — Breakdown by method (CASH/QRIS/TRANSFER): count + total per method, percentage. Data: `payment-summary` endpoint.

### Date Range

Preset chips: Hari ini (Today), Kemarin (Yesterday), 7 Hari (Last 7 Days). [▼] opens a Material3 date range picker for custom ranges. All API calls include `start_date` and `end_date`.

### Owner-Only Addition

If role is OWNER, show a fourth tab **"Outlet"** with `outlet-comparison` data (only relevant with multiple outlets).

### What's NOT on Mobile

No CSV export — that stays on web admin. Phone is for viewing, not exporting spreadsheets.

---

## 4. CRM (Pelanggan)

Two use cases: counter lookup ("has this person ordered before?") and proactive review ("who are my top spenders?").

### Customer List

```
┌───────────────────────────────────────┐
│  ←  Pelanggan                   [+]  │
│  ─────────────────────────────────────│
│  🔍 Cari nama atau telepon...       │
│  ─────────────────────────────────────│
│  [Semua] [Terbanyak] [Terbaru]      │
│  ─────────────────────────────────────│
│  👤 Budi Santoso                     │
│     08123456789 · 12 pesanan         │
│     Total: Rp 1.240.000             │
│  ─────────────────────────────────────│
│  👤 Sari Dewi                        │
│     08198765432 · 8 pesanan          │
│     Total: Rp 890.000               │
│  ─────────────────────────────────────│
│  👤 Pak Ahmad                        │
│     08567891234 · 3 pesanan          │
│     Total: Rp 156.000               │
│  ─────────────────────────────────────│
└───────────────────────────────────────┘
```

**Search**: Filters by name or phone — uses existing `?search=` query param.

**Sort chips**: "Semua" (alphabetical), "Terbanyak" (most spend/orders), "Terbaru" (most recent order). Sorting is client-side for v1 (< 100 customers per outlet).

**[+] button**: Creates new customer (name + phone), same fields as the existing add-customer dialog in Cart but as a standalone action.

### Customer Detail

```
┌───────────────────────────────────────┐
│  ←  Budi Santoso              [✏️]  │
│     08123456789                      │
│  ─────────────────────────────────────│
│  ┌─────────┐ ┌─────────┐ ┌────────┐ │
│  │ 12      │ │Rp1.24jt │ │Rp103rb │ │
│  │ Pesanan │ │ Total   │ │ Rata²  │ │
│  └─────────┘ └─────────┘ └────────┘ │
│  ─────────────────────────────────────│
│  Menu Favorit:                       │
│  1. Nasi Bakar Ayam (8x)            │
│  2. Es Teh Manis (6x)               │
│  3. Kerupuk (5x)                     │
│  ─────────────────────────────────────│
│  Riwayat Pesanan                    │
│  #KWR-045 · 8 Feb · Rp 66.000      │
│  #KWR-038 · 5 Feb · Rp 120.000     │
│  #KWR-029 · 1 Feb · Rp 89.000      │
│  ...                                 │
└───────────────────────────────────────┘
```

**Stats cards** (3 KPIs) from `GET /customers/:id/stats`.

**Favorite items** from the stats endpoint's `top_items` field.

**Order history** from `GET /customers/:id/orders` — paginated, tap navigates to Order Detail screen.

**[✏️] button** opens edit form for name/phone/notes.

**No delete on mobile** — soft delete stays on web admin to prevent accidental data loss on a small screen.

---

## 5. Order History (Riwayat)

Extends the existing Pesanan screen (from the order flow design) with a second tab for historical orders.

### Screen Layout

```
┌───────────────────────────────────────┐
│  ←  Pesanan                          │
│  ─────────────────────────────────────│
│  [Aktif]              [Riwayat]      │
│  ─────────────────────────────────────│
│                                       │
│  ── RIWAYAT TAB ──                   │
│                                       │
│  🔍 Cari no. pesanan / pelanggan    │
│  [Hari ini] [Kemarin] [7 Hari] [▼]  │
│  ─────────────────────────────────────│
│  ┌─────────────────────────────────┐  │
│  │ #KWR-042  COMPLETED   Lunas   │  │
│  │ Dine-in · 14:20 · 8 Feb       │  │
│  │ 3 item · Rp 89.000            │  │
│  │ 👤 Budi                        │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ #KWR-041  CANCELLED           │  │
│  │ Takeaway · 13:55 · 8 Feb      │  │
│  │ 1 item · Rp 18.000            │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ #KWR-040  COMPLETED   Lunas   │  │
│  │ Dine-in · Meja 2 · 13:30      │  │
│  │ 5 item · Rp 156.000           │  │
│  │ 👤 Sari                        │  │
│  └─────────────────────────────────┘  │
│                                       │
└───────────────────────────────────────┘
```

### Tab Structure

**Aktif tab**: The existing order list design from the order flow plan — active orders with filter chips (Semua/Belum Bayar/Lunas). Visible to all roles.

**Riwayat tab**: Historical orders. Visible to OWNER and MANAGER only.

- **Search** by order number or customer name (client-side filtering for v1)
- **Date range** presets (Today, Yesterday, 7 Days, Custom)
- Shows COMPLETED and CANCELLED orders
- Tap navigates to Order Detail screen (read-only — no Edit/Pay/Cancel buttons)

### API Usage

Uses existing `GET /outlets/:oid/orders` with `?status=COMPLETED` or `?status=CANCELLED` filters. Client-side search in v1; add server-side `?search=` later if volume justifies it.

---

## 6. Menu Management (Menu)

Full CRUD for the complete menu hierarchy: Categories → Products → Variant Groups → Variants → Modifier Groups → Modifiers → Combos.

### Design Approach

**Product Detail as the central hub.** Instead of 4 levels of deep navigation, variants, modifiers, and combos are managed as collapsible sections within the product detail screen. Sub-editing uses bottom sheets to stay on the same page.

### Category List (Entry Point)

```
┌───────────────────────────────────────┐
│  ←  Kelola Menu                 [+]  │
│  ─────────────────────────────────────│
│                                       │
│  ┌─────────────────────────────────┐  │
│  │ 🍚 Nasi Bakar          4 produk│  │
│  │    ≡ drag handle          [✏️] │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ 🥤 Minuman             6 produk│  │
│  │    ≡ drag handle          [✏️] │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ 🍘 Snack               3 produk│  │
│  │    ≡ drag handle          [✏️] │  │
│  └─────────────────────────────────┘  │
│                                       │
└───────────────────────────────────────┘
```

- **Tap category** → drill down to Product List for that category
- **[✏️]** → inline edit dialog (rename, description, toggle active)
- **[+]** → new category dialog (name + description)
- **Drag handles** → reorder categories (updates `sort_order` via API)
- Inactive categories shown with lower opacity + "Nonaktif" badge

### Product List (Within a Category)

```
┌───────────────────────────────────────┐
│  ←  Nasi Bakar                  [+]  │
│  ─────────────────────────────────────│
│                                       │
│  ┌─────────────────────────────────┐  │
│  │ Nasi Bakar Ayam     Rp 18.000 │  │
│  │ 🔥 GRILL · 2 varian · 1 modif │  │
│  │ ≡                         [✏️] │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ Nasi Bakar Cumi     Rp 21.000 │  │
│  │ 🔥 GRILL · 1 varian           │  │
│  │ ≡                         [✏️] │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ Nasi Bakar Iga      Rp 28.000  │  │
│  │ 🔥 GRILL                       │  │
│  │ ≡                         [✏️] │  │
│  └─────────────────────────────────┘  │
│                                       │
└───────────────────────────────────────┘
```

- Shows: product name, base price, station badge, variant/modifier count
- **Tap** → Product Detail (hub screen)
- **[✏️]** → quick price edit bottom sheet (just price field for fast adjustments)
- **[+]** → new product form (full Product Detail screen, empty)
- **Drag handles** → reorder products

### Product Detail (Hub Screen)

All variant groups, modifier groups, and combo items managed here via collapsible sections and bottom sheets.

```
┌───────────────────────────────────────┐
│  ←  Nasi Bakar Ayam            [⋮]  │
│  ─────────────────────────────────────│
│  Nama                                │
│  [Nasi Bakar Ayam____________]       │
│  Harga Dasar                         │
│  [18000_____________________]        │
│  Kategori           Station          │
│  [Nasi Bakar ▼]    [GRILL ▼]       │
│  Deskripsi                           │
│  [Nasi bakar ayam original__]        │
│  Waktu Persiapan (menit)             │
│  [15________________________]        │
│  ─────────────────────────────────────│
│                                       │
│  ▼ Varian (1 grup)              [+] │
│  ┌─────────────────────────────────┐  │
│  │ Size (wajib)             [✏️]  │  │
│  │  · Regular     +Rp 0           │  │
│  │  · Large       +Rp 5.000       │  │
│  │                    [+ Varian]   │  │
│  └─────────────────────────────────┘  │
│                                       │
│  ▼ Modifier (1 grup)            [+] │
│  ┌─────────────────────────────────┐  │
│  │ Topping (0-3)            [✏️]  │  │
│  │  · Sambal      Rp 3.000        │  │
│  │  · Keju        Rp 5.000        │  │
│  │  · Telur       Rp 5.000        │  │
│  │                   [+ Modifier]  │  │
│  └─────────────────────────────────┘  │
│                                       │
│  ▼ Combo Items (0)              [+] │
│  (kosong)                            │
│                                       │
│  ─────────────────────────────────────│
│  ┌─────────────────────────────────┐  │
│  │          SIMPAN                 │  │
│  └─────────────────────────────────┘  │
│  [Nonaktifkan Produk]                │
│                                       │
└───────────────────────────────────────┘
```

### Sub-Editing via Bottom Sheets

All child editing (variant groups, individual variants, modifier groups, individual modifiers, combo items) opens as a bottom sheet — keeping the user on the same screen.

**Variant Group Bottom Sheet** (tap [✏️] on group header):

```
┌───────────────────────────────────────┐
│  ─── Edit Grup Varian ───────────── │
│                                       │
│  Nama Grup                           │
│  [Size______________________]        │
│  ☑ Wajib dipilih                     │
│                                       │
│  [HAPUS GRUP]          [SIMPAN]      │
└───────────────────────────────────────┘
```

**Variant Item Bottom Sheet** (tap a variant):

```
┌───────────────────────────────────────┐
│  ─── Edit Varian ─────────────────── │
│                                       │
│  Nama                                │
│  [Large_____________________]        │
│  Harga (+/-)                         │
│  [5000______________________]        │
│                                       │
│  [HAPUS]              [SIMPAN]       │
└───────────────────────────────────────┘
```

**Modifier Group Bottom Sheet** (tap [✏️] on group header):

```
┌───────────────────────────────────────┐
│  ─── Edit Grup Modifier ──────────── │
│                                       │
│  Nama Grup                           │
│  [Topping___________________]        │
│  Min Pilih        Max Pilih          │
│  [0_________]     [3_________]       │
│                                       │
│  [HAPUS GRUP]          [SIMPAN]      │
└───────────────────────────────────────┘
```

**Modifier Item Bottom Sheet** (tap a modifier):

```
┌───────────────────────────────────────┐
│  ─── Edit Modifier ──────────────── │
│                                       │
│  Nama                                │
│  [Sambal____________________]        │
│  Harga                               │
│  [3000______________________]        │
│                                       │
│  [HAPUS]              [SIMPAN]       │
└───────────────────────────────────────┘
```

**Combo Item**: [+] opens a product picker (searchable list), each combo item shows product name + quantity, editable inline.

### Key Design Decisions

1. **Hub pattern** — Product Detail is the single place to see and manage everything about a product. No 4-level deep navigation.
2. **Bottom sheets for sub-editing** — keeps context, avoids losing scroll position, familiar Android pattern.
3. **Collapsible sections** — Variants, Modifiers, Combos are collapsed by default, expanded on demand.
4. **Drag-to-reorder** — Categories and products support reordering via drag handles (updates `sort_order`).
5. **Quick price edit** — Product list has a [✏️] shortcut for the most common edit (price change) without opening the full detail screen.
6. **No inline-delete on categories/products** — uses "Nonaktifkan" (soft deactivate) to prevent accidental data loss, consistent with the API's `is_active` pattern.

---

## 7. Staff Management (Pengguna)

Simple list + form for occasional staff management. No over-engineering for < 10 staff per outlet.

### Staff List

```
┌───────────────────────────────────────┐
│  ←  Pengguna                    [+]  │
│  ─────────────────────────────────────│
│  ┌─────────────────────────────────┐  │
│  │ 👤 Siti Rahayu                  │  │
│  │    MANAGER · siti@kiwari.com   │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ 👤 Adi Pratama                  │  │
│  │    CASHIER · adi@kiwari.com    │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ 👤 Budi Hartono                 │  │
│  │    KITCHEN · budi@kiwari.com   │  │
│  └─────────────────────────────────┘  │
│                                       │
└───────────────────────────────────────┘
```

**Tap** → edit form. **[+]** → create form. Same form layout for both.

### Staff Form (Create / Edit)

```
┌───────────────────────────────────────┐
│  ←  Tambah Pengguna                  │
│  ─────────────────────────────────────│
│  Nama Lengkap                        │
│  [________________________]          │
│                                       │
│  Email                               │
│  [________________________]          │
│                                       │
│  Password (create only)              │
│  [________________________]          │
│                                       │
│  PIN (4-6 digit)                     │
│  [______]                            │
│                                       │
│  Role                                │
│  [CASHIER ●] [KITCHEN ○]            │
│  [MANAGER ○] [OWNER ○]              │
│                                       │
│  ┌─────────────────────────────────┐  │
│  │          SIMPAN                 │  │
│  └─────────────────────────────────┘  │
│                                       │
│  (edit only)                         │
│  [Nonaktifkan Pengguna]              │
│                                       │
└───────────────────────────────────────┘
```

### Design Decisions

- **Password**: Only on create form. Not editable (users manage own passwords).
- **PIN**: Editable on both create and edit (covers the "reset PIN" use case).
- **Role chips**: All 4 visible at once. OWNER chip only visible to OWNER role.
- **Deactivate**: Soft delete via "Nonaktifkan" text button with confirmation dialog.
- **No search/pagination**: Outlet has < 10 staff typically.

---

## 8. Role-Based Access Matrix

Complete visibility matrix across all features:

| Feature | OWNER | MANAGER | CASHIER | KITCHEN |
|---------|-------|---------|---------|---------|
| **Kasir** (Menu/Cart/Payment) | Yes | Yes | Yes | No |
| **Drawer** — Pesanan | Yes | Yes | Yes | No |
| — Aktif tab | Yes | Yes | Yes | No |
| — Riwayat tab | Yes | Yes | No | No |
| **Drawer** — Laporan | Yes | Yes | No | No |
| — Outlet Comparison tab | Yes | No | No | No |
| **Drawer** — Menu | Yes | Yes | No | No |
| **Drawer** — Pelanggan | Yes | Yes | No | No |
| **Drawer** — Pengguna | Yes | Yes | No | No |
| — Create OWNER role | Yes | No | No | No |
| **Drawer** — Printer | Yes | Yes | Yes | No |

**KITCHEN role**: Not expected to use the Android app in v1 (they receive printed tickets). If they log in, minimal view.

---

## 9. API Considerations

### Existing Endpoints (No Changes Needed)

The Go API (M1-6, 401 tests) already has all endpoints for these features:

| Feature | Endpoints |
|---------|-----------|
| Reports | `GET /outlets/:oid/reports/daily-sales`, `product-sales`, `payment-summary`, `hourly-sales`, `GET /reports/outlet-comparison` |
| CRM | `GET /outlets/:oid/customers` (with search), `GET /:id`, `POST`, `PUT`, `GET /:id/stats`, `GET /:id/orders` |
| Menu | Full CRUD for categories, products, variant groups, variants, modifier groups, modifiers, combo items (M3, 175 tests) |
| Staff | `GET /outlets/:oid/users`, `POST`, `PUT`, `DELETE` (soft) |
| Orders | `GET /outlets/:oid/orders` (with status filter), `GET /:id` |

### Minor API Gaps

1. **Customer sort by spend/visits**: `GET /customers` supports `?search=` but not `?sort=`. For v1, fetch all and sort client-side (< 100 customers per outlet). Add server-side sort later if needed.

2. **Order history search**: `GET /orders` supports `?status=` filter but not `?search=` by order number. For v1, filter client-side from loaded list. Add server-side search later if volume justifies it.

3. **Active orders endpoint** and **`amount_paid` field**: Already designed in the order flow plan (Task 1 of `2026-02-08-android-order-flow-plan.md`).

---

## 10. Implementation Considerations

### New Files (Estimated)

| File Group | Files | Description |
|------------|-------|-------------|
| Navigation | 1-2 | Drawer composable, role visibility utility |
| Reports | 2 | ReportsScreen, ReportsViewModel |
| CRM | 4 | CustomerListScreen, CustomerDetailScreen, 2 ViewModels |
| Menu CRUD | 6-8 | CategoryListScreen, ProductListScreen, ProductDetailScreen, ViewModels, bottom sheet composables |
| Staff | 4 | StaffListScreen, StaffFormScreen, 2 ViewModels |
| API/Repo | 2-4 | Extended APIs (MenuAdminApi, CustomerApi, UserApi), repositories |

### Modified Files

| File | Change |
|------|--------|
| `NavGraph.kt` | Add all new routes |
| `MenuScreen.kt` | Wire hamburger icon to drawer |
| `OrderListScreen.kt` | Add Aktif/Riwayat tabs (from order flow plan, extended) |

### Dependencies

- Drawer and role-based access are prerequisites for all admin screens
- Reports, CRM, Menu CRUD, Staff, and Order History are independent of each other — can be parallelized
- Order History extends the Order List screen from the order flow plan — should be implemented after that plan is complete

### Relationship to Order Flow Plan

The `2026-02-08-android-order-flow-plan.md` should be implemented first. It establishes:
- Order List screen (which this plan extends with Riwayat tab)
- Order Detail screen (which CRM customer history links to)
- Extended OrderApi (which order history reuses)

This admin features plan builds on top of the order flow plan's foundation.
