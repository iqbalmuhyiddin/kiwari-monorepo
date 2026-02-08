# Android POS — Order Flow & Order Management

> Design document for save-before-payment flow, order lifecycle management, and printing/sharing.
> Created: 2026-02-08

## 1. Problem

The current Android POS flow forces payment at order creation time:

```
Menu → Cart → Payment (must pay now) → Order created + print
```

This doesn't match real F&B operations. Dine-in customers order first, eat, then pay. The cashier needs to:
1. Save an order (send to kitchen) without collecting payment
2. Find that order later when the customer is ready to pay
3. Edit the order if the customer adds or removes items
4. Print kitchen tickets, bills, and receipts at different stages

## 2. New Flow

```
Menu Screen ──── "Pesanan" ──→ Order List (active orders)
    │                              └── tap → Order Detail
    ▼                                        ├── EDIT → Menu (cart pre-loaded) → Cart → SIMPAN → Order Detail
Cart Screen                                  ├── BAYAR → Payment → auto print receipt → Order Detail (PAID)
├── "SIMPAN" → create order                  ├── Print Kitchen Ticket
│               → Order Detail (UNPAID)      ├── Print Bill / Receipt
│                                            ├── Share Bill / Receipt (image)
├── "BAYAR"  → Payment Screen                └── Batalkan
│               → auto print receipt
│               → Order Detail (PAID)
│
└── "LANJUT BOOKING" (catering, unchanged)
        → Catering Screen → Book + DP → Order Detail (DP_PAID)
```

### Flow Rules

| Cart Action | When | Creates Order? | Payment? | Navigates To |
|---|---|---|---|---|
| SIMPAN | Non-catering orders | Yes (status NEW) | No | Order Detail (unpaid) |
| BAYAR | Non-catering orders | Yes (status NEW) | Yes (immediate) | Order Detail (paid) |
| LANJUT BOOKING | Catering only | Yes (via Catering Screen) | DP only | Order Detail (DP_PAID) |

### Edit Flow

1. Order Detail → "EDIT" → Menu screen (cart pre-loaded from saved order)
2. Cashier modifies items using the normal Menu+Cart UI
3. Cart → "SIMPAN" → diffs changes → API calls to add/remove/update items
4. → Order Detail (refreshed)

The Menu+Cart screens serve as both the order creator and editor. In edit mode:
- Top bar shows "Edit Pesanan #KWR-005"
- Bottom bar shows "✏️ #KWR-005 3 item — LANJUT"
- SIMPAN syncs changes (not creates new order)
- BAYAR syncs changes first, then navigates to Payment

### Catering Lifecycle

```
Cart → LANJUT BOOKING → Catering Screen → Book + DP → Order Detail (DP_PAID)
                                                         ↓ (later)
                                           Order List → Order Detail → BAYAR
                                                         → Payment (remaining balance)
                                                         → auto print receipt
                                                         → Order Detail (SETTLED)
```

## 3. Screen Designs

### 3.1 Cart Screen (Modified)

Two buttons replace the single "BAYAR" button for non-catering orders.

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
│  ─────────────────────────────────────│
│ ┌──────────────┐ ┌──────────────────┐ │
│ │    SIMPAN     │ │  BAYAR Rp66.000 │ │
│ │  (outlined)   │ │    (green)      │ │
│ └──────────────┘ └──────────────────┘ │
└───────────────────────────────────────┘
```

**Button layout by context:**

| Context | Left Button | Right Button |
|---|---|---|
| New order (non-catering) | SIMPAN (outlined) | BAYAR Rp 66.000 (green) |
| Editing existing order | SIMPAN (outlined) | BAYAR Rp 66.000 (green) |
| Catering | — (hidden) | LANJUT BOOKING (green, full width) |

**In edit mode**, top bar shows "Edit Pesanan #KWR-005" instead of "Keranjang".

**SIMPAN behavior:**
- New order: `POST /orders` → navigate to Order Detail
- Edit mode: diff cart vs original → `POST/PUT/DELETE /orders/:id/items` as needed → navigate to Order Detail

**BAYAR behavior:**
- New order: navigate to Payment screen (creates order + pays, current behavior)
- Edit mode: sync changes first, then navigate to Payment screen with existing order ID

### 3.2 Order Detail Screen (New)

Central hub for any saved order. Adapts based on payment status.

**Unpaid order:**

```
┌───────────────────────────────────────┐
│  ←  Pesanan #KWR-005          STATUS │
│      Dine-in · Meja 3          [NEW] │
│      👤 Budi (08123456789)           │
│      12:34 · 8 Feb 2026              │
│  ─────────────────────────────────────│
│  1x Ayam Bakar Original      50.000  │
│     L · Hot · +Sambal · Nasi Uduk    │
│  2x Es Teh Manis             16.000  │
│  ─────────────────────────────────────│
│  Subtotal                    66.000  │
│  Diskon                          -0  │
│  Total                    Rp 66.000  │
│  ─────────────────────────────────────│
│  Belum dibayar                       │
│  ─────────────────────────────────────│
│                                       │
│  [🖨 Dapur]  [🧾 Bill]  [📤 Share]  │
│                                       │
│  ┌──────────────┐ ┌──────────────────┐│
│  │  ✏️ EDIT     │ │ BAYAR Rp66.000  ││
│  └──────────────┘ └──────────────────┘│
│            [Batalkan Pesanan]         │
└───────────────────────────────────────┘
```

**Paid order:**

```
┌───────────────────────────────────────┐
│  ←  Pesanan #KWR-005          STATUS │
│      Dine-in · Meja 3    [COMPLETED] │
│      👤 Budi (08123456789)           │
│      12:34 · 8 Feb 2026              │
│  ─────────────────────────────────────│
│  1x Ayam Bakar Original      50.000  │
│     L · Hot · +Sambal · Nasi Uduk    │
│  2x Es Teh Manis             16.000  │
│  ─────────────────────────────────────│
│  Subtotal                    66.000  │
│  Diskon                          -0  │
│  Total                    Rp 66.000  │
│  ─────────────────────────────────────│
│  Pembayaran:                         │
│  CASH          Rp 50.000             │
│    Diterima    Rp 100.000            │
│    Kembalian   Rp 50.000             │
│  QRIS          Rp 16.000             │
│  ─────────────────────────────────────│
│                                       │
│  [🖨 Dapur] [🧾 Receipt] [📤 Share] │
│                                       │
└───────────────────────────────────────┘
```

**Catering order (DP_PAID):**

```
┌───────────────────────────────────────┐
│  ←  Pesanan #KWR-010       [DP_PAID] │
│      CATERING · 15 Feb 2026          │
│      Alamat: Jl. Raya No. 10        │
│      👤 Budi (08123456789)           │
│      Dibuat 8 Feb 2026 · 14:20      │
│  ─────────────────────────────────────│
│  20x Nasi Bakar Ayam        360.000  │
│  20x Es Teh Manis           100.000  │
│  10x Kerupuk                 50.000  │
│  ─────────────────────────────────────│
│  Subtotal                   510.000  │
│  Diskon 2%                  -10.200  │
│  Total                   Rp 499.800  │
│  ─────────────────────────────────────│
│  Pembayaran:                         │
│  DP (TRANSFER)       Rp 249.900     │
│  Sisa belum bayar    Rp 249.900     │
│  ─────────────────────────────────────│
│                                       │
│  [🖨 Dapur]  [🧾 Bill]  [📤 Share]  │
│                                       │
│  ┌──────────────┐ ┌──────────────────┐│
│  │  ✏️ EDIT     │ │BAYAR Rp249.900  ││
│  └──────────────┘ └──────────────────┘│
│            [Batalkan Pesanan]         │
└───────────────────────────────────────┘
```

**Actions by order state:**

| Action | Unpaid | Paid/Settled | Catering (DP_PAID) |
|---|---|---|---|
| Print Kitchen | Yes | Yes | Yes |
| Print Bill | Yes (bill) | — | Yes (bill) |
| Print Receipt | — | Yes (receipt) | — |
| Share | Bill image | Receipt image | Bill image |
| Edit | Yes | No | Yes |
| Bayar | Yes (full amount) | No | Yes (remaining) |
| Batalkan | Yes | No | Yes |

**Data source:** `GET /outlets/:oid/orders/:id` — returns order with nested items, modifiers, and payments in a single call.

### 3.3 Order List Screen (New)

Accessible from Menu screen via "Pesanan" button. Shows active orders only.

```
┌───────────────────────────────────────┐
│  ←  Pesanan Aktif                    │
│  ─────────────────────────────────────│
│  [Semua] [Belum Bayar] [Lunas]       │
│  ─────────────────────────────────────│
│                                       │
│  ┌─────────────────────────────────┐  │
│  │ #KWR-005    NEW   Belum Bayar  │  │
│  │ Dine-in · Meja 3  · 12:34     │  │
│  │ 3 item · Rp 66.000            │  │
│  │ 👤 Budi                        │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ #KWR-010  DP_PAID  Belum Lunas │  │
│  │ CATERING · 15 Feb 2026         │  │
│  │ 3 item · Rp 499.800            │  │
│  │ 👤 Budi                        │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ #KWR-004  PREPARING   Lunas   │  │
│  │ Takeaway · 12:20              │  │
│  │ 1 item · Rp 18.000            │  │
│  └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐  │
│  │ #KWR-003    READY    Lunas    │  │
│  │ Dine-in · Meja 1  · 12:05     │  │
│  │ 5 item · Rp 120.000           │  │
│  │ 👤 Sari                        │  │
│  └─────────────────────────────────┘  │
│                                       │
└───────────────────────────────────────┘
```

**Each order card shows:**
- Order number + status badge (NEW/PREPARING/READY/DP_PAID, color-coded)
- Payment status: "Belum Bayar" (red) or "Lunas" (green) or "Belum Lunas" (orange, for catering partial)
- Order type + table number (dine-in) or catering date (catering) + time
- Item count + total amount
- Customer name (if attached)

**Filter chips:**
- Semua — all active orders
- Belum Bayar — unpaid orders (regular + catering with remaining balance)
- Lunas — paid/settled but still active (PREPARING/READY, not yet COMPLETED)

**Tap → Order Detail screen**

**What counts as "active":**
- Regular orders with status NEW, PREPARING, or READY
- Catering orders with catering_status DP_PAID (regardless of order status)
- Excludes COMPLETED and CANCELLED

### 3.4 Menu Screen (Modified)

One addition: a "Pesanan" button in the top bar to access the Order List.

```
┌───────────────────────────────────────┐
│  ☰  Transaksi     [📋 Pesanan]  ⚙   │
│  🔍  ...                              │
│  [Semua] [Nasi Bakar] [Minuman]       │
│  ...                                  │
```

Optional: badge on the Pesanan button showing count of unpaid orders.

**Edit mode:** When entering from Order Detail → Edit, the bottom bar changes to show the order being edited:

```
│ ┌─────────────────────────────────┐   │
│ │ ✏️ #KWR-005  3 item     LANJUT │   │
│ └─────────────────────────────────┘   │
```

### 3.5 Payment Screen (Modified)

The Payment screen needs to handle two entry points:

| Entry Point | Behavior |
|---|---|
| Cart → BAYAR (new order) | Create order via API, then add payments (current behavior) |
| Order Detail → BAYAR (existing order) | Skip order creation, add payments to existing order ID |

**For existing orders:** Payment screen receives the order ID and remaining balance. Shows the remaining amount as the target. After all payments submitted:
1. Auto-print customer receipt (thermal)
2. Navigate to Order Detail (paid state)

The current success screen inside PaymentScreen is replaced by the Order Detail screen. No redundant screen.

## 4. Printing & Sharing

Three print/share actions on Order Detail, adapting based on payment status. All printing is manual (no auto-print except after payment submission).

### 4.1 Print Kitchen Ticket

Existing logic in `ReceiptFormatter.formatKitchenTicket()`. Shows order number, items with notes, no prices. Available on all orders regardless of payment status.

When editing an order and saving changes, the full kitchen ticket is reprinted (not just deltas).

### 4.2 Print Bill (Unpaid Orders)

New thermal print format for unpaid orders. Like a receipt but without payment details.

```
================================
       KIWARI NASI BAKAR
       Jl. Example No. 1
================================
#KWR-005          8 Feb 2026
Dine-in · Meja 3       12:34
--------------------------------
1x Ayam Bakar Original  50.000
   L · Hot · +Sambal
   Nasi Uduk
2x Es Teh Manis         16.000
--------------------------------
Subtotal                66.000
Diskon                       0
                        ------
TOTAL              Rp  66.000
================================
      ** BELUM DIBAYAR **
================================
```

### 4.3 Print Receipt (Paid Orders)

Existing logic in `ReceiptFormatter.formatReceipt()`. Full receipt with payment breakdown and "LUNAS" marking. Auto-printed after payment submission. Can be reprinted from Order Detail.

### 4.4 Share as Image

Generate the bill or receipt as a PNG image, then use Android's share sheet (WhatsApp, Telegram, etc.).

**Approach:**
1. Generate the same text content as thermal print (reuse formatters)
2. Render text onto a `Canvas`/`Bitmap` with monospace font, white background
3. Save to app's cache directory
4. Share via `Intent.ACTION_SEND` + `FileProvider`

Visually consistent with thermal printout — same layout, same content, just as an image.

## 5. API Considerations

### Existing Endpoints (No Changes Needed)

The Go API already supports this flow:

| Endpoint | Used For |
|---|---|
| `POST /outlets/:oid/orders` | Create order (SIMPAN) |
| `GET /outlets/:oid/orders/:id` | Order Detail data |
| `GET /outlets/:oid/orders` | Order List (with status filter) |
| `POST /outlets/:oid/orders/:id/items` | Add item (edit mode) |
| `PUT /outlets/:oid/orders/:id/items/:iid` | Update item qty/notes (edit mode) |
| `DELETE /outlets/:oid/orders/:id/items/:iid` | Remove item (edit mode) |
| `POST /outlets/:oid/orders/:id/payments` | Add payment |
| `DELETE /outlets/:oid/orders/:id` | Cancel order |

### API Gap: Multi-Status Filter

The Order List needs orders with status NEW, PREPARING, or READY, plus catering orders with catering_status DP_PAID. The current API only accepts a single `status` query param.

**Options:**
1. Add comma-separated multi-status support: `?status=NEW,PREPARING,READY`
2. Make 3 parallel API calls from Android (like the SvelteKit dashboard does)
3. Add a dedicated `?active=true` shorthand filter

**Recommendation:** Option 1 (comma-separated) — minimal API change, most flexible.

### API Gap: Payment Status Derivation

The Order List needs to show "Belum Bayar" vs "Lunas". This isn't a stored field — it's derived from `SUM(payments.amount) >= orders.total_amount`. Options:
1. Android fetches payments per order and computes client-side (N+1 problem)
2. API adds a `payment_status` or `amount_paid` field to the order list response
3. API adds a `?paid=false` filter

**Recommendation:** Option 2 — add `amount_paid` to the order list response. Minimal change, avoids N+1.

## 6. Implementation Impact

### New Files

| File | Description |
|---|---|
| `ui/orders/OrderListScreen.kt` | Order List screen |
| `ui/orders/OrderListViewModel.kt` | Order List view model |
| `ui/orders/OrderDetailScreen.kt` | Order Detail screen |
| `ui/orders/OrderDetailViewModel.kt` | Order Detail view model |
| `data/api/OrderApi.kt` | Extended with list/detail/cancel endpoints |
| `util/printer/BillFormatter.kt` | Bill format (unpaid) |
| `util/share/ReceiptImageGenerator.kt` | Render receipt/bill as PNG |
| `util/share/ShareHelper.kt` | Android share intent helper |

### Modified Files

| File | Change |
|---|---|
| `ui/cart/CartScreen.kt` | Add SIMPAN button, edit mode UI |
| `ui/cart/CartViewModel.kt` | Add save-without-payment logic, edit mode diffing |
| `ui/menu/MenuScreen.kt` | Add "Pesanan" button, edit mode bottom bar |
| `ui/menu/MenuViewModel.kt` | Support edit mode (pre-load cart from order) |
| `ui/payment/PaymentScreen.kt` | Accept existing order ID, remove success screen |
| `ui/payment/PaymentViewModel.kt` | Handle existing order payment |
| `data/repository/CartRepository.kt` | Load cart from API order data |
| `NavGraph.kt` | Add Order List and Order Detail routes |

### API Changes (Go)

| File | Change |
|---|---|
| `api/internal/handler/order.go` | Multi-status filter support |
| `api/queries/orders.sql` | Add `amount_paid` subquery to list query |

## 7. Navigation Graph

```
NavGraph additions:

menu ──→ orderList ──→ orderDetail/{orderId}
                            │
                            ├── menu (edit mode, with orderId param)
                            │     └── cart (edit mode) → orderDetail
                            │
                            └── payment (with orderId param)
                                  └── orderDetail

cart ──→ orderDetail/{orderId}  (after SIMPAN)
cart ──→ payment ──→ orderDetail/{orderId}  (after BAYAR)
```

**Key navigation params:**
- `orderDetail/{orderId}` — required, UUID
- `menu?editOrderId={orderId}` — optional, triggers edit mode
- `payment?orderId={orderId}` — optional, skips order creation if present
