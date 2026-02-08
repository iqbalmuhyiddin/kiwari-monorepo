# Seed Data

Realistic example data for development and demos. Nasi Bakar menu with customers, orders, and payments.

## Usage

```bash
# Prerequisite: database running and migrated
make db-up
make db-migrate

# Load seed data (idempotent — safe to re-run)
make db-seed
```

Or run directly:

```bash
psql "$DATABASE_URL" -f api/seed/seed.sql
```

## Test Credentials

All users use password: `password123`

| Name | Email | Role | PIN |
|------|-------|------|-----|
| Iqbal Muhyiddin | admin@kiwari.com | OWNER | 1234 |
| Siti Rahayu | siti@kiwari.com | MANAGER | 5678 |
| Budi Santoso | budi@kiwari.com | CASHIER | 1111 |
| Dewi Lestari | dewi@kiwari.com | KITCHEN | 2222 |

**Outlet ID:** `17fbe5e3-6dea-4a8e-9036-8a59c345e157`

## What's Included

| Table | Count | Notes |
|-------|-------|-------|
| Outlet | 1 | Kiwari Nasi Bakar, Bandung |
| Users | 4 | One per role |
| Categories | 4 | Nasi Bakar, Minuman, Camilan, Paket Hemat |
| Products | 17 | 6 nasi bakar, 5 drinks, 4 sides, 2 combos |
| Variant Groups | 8 | Ukuran (size) for nasi bakar and select drinks |
| Variants | 16 | Reguler/Jumbo, Reguler/Large |
| Modifier Groups | 15 | Level Pedas (6), Tambahan (6), Topping (3) |
| Modifiers | 48 | Spice levels, add-ons, drink toppings |
| Combo Items | 4 | 2 combo products with 2 components each |
| Customers | 5 | Mix of regulars and walk-ins |
| Orders | 10 | All statuses and order types |
| Order Items | 20 | With variant and modifier snapshots |
| Order Item Modifiers | 12 | Spice levels, add-ons, toppings |
| Payments | 7 | CASH, QRIS, TRANSFER, split payment |

## Order Summary

| # | Customer | Type | Status | Total | Payment |
|---|----------|------|--------|-------|---------|
| KWR-001 | Ahmad | DINE_IN | COMPLETED | 38,000 | CASH |
| KWR-002 | Rina | TAKEAWAY | COMPLETED | 26,000 | QRIS |
| KWR-003 | Walk-in | DINE_IN | COMPLETED | 40,000 | CASH |
| KWR-004 | Pak Hendra | TAKEAWAY | COMPLETED | 72,000 | TRANSFER |
| KWR-005 | Dimas | DINE_IN | COMPLETED | 66,000 | CASH + QRIS (split) |
| KWR-006 | Ibu Sari | CATERING | DP_PAID | 600,000 | TRANSFER (DP 250k) |
| KWR-007 | Ahmad | DINE_IN | PREPARING | 53,000 | — |
| KWR-008 | Walk-in | TAKEAWAY | NEW | 28,000 | — |
| KWR-009 | Rina | DINE_IN | READY | 53,000 | — |
| KWR-010 | Dimas | DELIVERY | CANCELLED | 33,000 | — |
