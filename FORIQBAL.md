# FORIQBAL.md — Kiwari POS

> A plain-language guide to everything in this repo. Written so that future-you, six months from now, can pick this up cold and understand what's going on.

---

## What Is This?

Kiwari POS is a **custom point-of-sale system** built from scratch for your nasi bakar restaurants. It replaces KasirPintar with something you fully own and control.

Think of it as three apps that share one brain:

```
┌─────────────────┐     ┌──────────────────┐
│  Android App    │     │  Web Admin Panel  │
│  (cashier uses) │     │  (you use)        │
└────────┬────────┘     └────────┬──────────┘
         │   REST + WebSocket    │
         └──────────┬────────────┘
              ┌─────▼──────┐
              │  Go API    │  ← the brain
              │  server    │
              └─────┬──────┘
              ┌─────▼──────┐
              │ PostgreSQL │  ← the memory
              └────────────┘
```

The **Android app** is what cashiers tap on to take orders. The **web admin** is where you manage menus, view reports, and keep an eye on things. The **Go API** is the server that does all the actual work — it talks to the database, enforces business rules, and keeps everyone in sync via WebSocket.

---

## The Monorepo Layout

Everything lives in one repo, side by side:

```
pos-superpower/
├── api/            Go backend
├── admin/          SvelteKit web panel
├── android/        Android POS app
├── docker/         Production Docker setup
├── docs/plans/     Design docs & implementation plans
├── .github/        CI/CD workflows
└── Makefile        Glue that ties it all together
```

Each subdirectory is its own world — its own language, its own package manager, its own build system. The root `Makefile` gives you a unified interface so you don't have to remember which tool does what. `make api-test`, `make admin-dev`, `make android-build` — it all just works.

---

## The Tech Stack (and Why)

| Component | Tech | Why This |
|-----------|------|----------|
| API | Go + Chi router | Fast, tiny binaries, great stdlib. Chi is just a mux — no framework magic to debug. |
| Database | PostgreSQL 16 | The only database you'd trust with money. ACID, battle-tested, great with decimals. |
| SQL layer | sqlc | Write SQL, get type-safe Go code. No ORM weirdness, no query builder gotchas. |
| Web Admin | SvelteKit 2 + Svelte 5 | Server-rendered pages with client-side interactivity. Tailwind for styling. Zero UI library dependencies. |
| Android | Kotlin + Jetpack Compose | Google's official UI toolkit. Hilt for DI, Retrofit for HTTP. Standard boring Android. |
| Infra | Docker Compose + Nginx Proxy Manager | Already running on your VPS. No Kubernetes, no orchestrators, no cluster. Just containers. |
| CI/CD | GitHub Actions | Push to main → tests → build → deploy staging. Tag → promote to production. |

The theme here is **boring tech, done well**. Every choice is something with a decade of Stack Overflow answers behind it.

---

## The Database: 21 Tables, No Surprises

The schema is designed around one principle: **data integrity is sacred**.

### Multi-outlet isolation
Every table that matters has an `outlet_id`. A cashier at outlet A can never see, create, or modify anything at outlet B. The API enforces this in middleware before any handler code runs.

### Money handling
All monetary values are `decimal(12,2)` — never floats. In Go, this is `shopspring/decimal`. In Kotlin, `BigDecimal`. In the admin, amounts arrive as strings from the API and get formatted to Rupiah (`Rp 1.250.000`). Floating point arithmetic and money don't mix; this is non-negotiable.

### The tables

**People & places**: `outlets`, `users` (4 roles: OWNER, MANAGER, CASHIER, KITCHEN), `customers`

**Menu structure**: `categories` → `products` → `variant_groups/variants` (pick one, like size) + `modifier_groups/modifiers` (pick many, like extra toppings) + `combo_items` (bundled products)

**Orders**: `orders` → `order_items` → `order_item_modifiers` → `payments`

**Accounting** (added Phase 1-2): `acct_accounts` (chart of accounts), `acct_items` (inventory items for matching), `acct_cash_accounts` (kas/bank accounts), `acct_cash_transactions` (the ledger — every purchase, reimbursement, or sales entry), `acct_reimbursement_requests` (expense claims from staff), `acct_sales_daily_summaries` (daily sales aggregation), `acct_payroll_entries` (salary records)

### Price snapshots — the most important design decision

When a cashier creates an order, the prices get **frozen** into `order_items.unit_price` and `order_item_modifiers.unit_price`. If you change a menu price tomorrow, yesterday's orders still show the old price. This is how accounting works in the real world — your order history is an immutable ledger.

### Soft deletes — nothing truly dies

Tables with foreign key references use `is_active=false` instead of `DELETE`. Orders use `status=CANCELLED`. Payments are **never** deleted. You always have an audit trail. If a tax auditor shows up, every transaction is traceable.

### Multi-payment — because real life is messy

A customer pays Rp 50k cash and Rp 40k via QRIS? That's two rows in the `payments` table, both linked to the same order. The system tracks exactly how each rupiah was collected.

### Catering lifecycle

Catering orders follow: `BOOKED → DP_PAID → SETTLED`. The down payment (DP) is just the first payment record. Final payment is the second. The `catering_status` field tracks where in this lifecycle the order sits.

---

## The Go API: How It's Built

### Entry point
`api/cmd/server/main.go` — loads config from env vars, connects to PostgreSQL, wires up all handlers, starts listening on port 8081.

### The handler pattern
Every handler owns a **narrow interface** for database access. For example, `AuthHandler` only knows about `GetUserByEmail` and `GetUserByOutletAndPin`. It doesn't import the entire database package. This makes testing trivial — you mock a 2-method interface, not a 50-method god object.

```
api/internal/
├── accounting/
│   ├── handler/     Accounting HTTP handlers (master data, purchases, reimbursements, reports, dashboard)
│   ├── matcher/     Item matching engine (keyword-based, variant filtering)
│   └── parser/      WhatsApp message parser (Indonesian dates, price shortcuts)
├── auth/            JWT token generation & validation
├── config/          Env var loading
├── database/        sqlc-generated code (DON'T EDIT BY HAND)
├── enum/            String constants replacing old PG enums
├── handler/         POS HTTP handlers (one file per domain)
├── middleware/       Auth guard, role checks, outlet scoping
├── router/          Chi router wiring
├── service/         Business logic (order creation, etc.)
└── ws/              WebSocket hub for live updates
```

### SQL queries
All SQL lives in `api/queries/*.sql`. Run `make api-sqlc` and it generates type-safe Go functions in `api/internal/database/`. You never write raw SQL in handler code. If you need a new query, add it to the `.sql` file, regenerate, and you get compile-time type checking for free.

### Auth flow
- **Email + password**: standard bcrypt → JWT (15 min access token + 7 day refresh token)
- **PIN login**: for cashiers doing shift changes. Plaintext by design — it's 4-6 digits, only works within a known outlet, and only grants CASHIER access. The tradeoff is speed over security, and it's acceptable here.

### WebSocket
The API runs a WebSocket hub per outlet at `/ws/outlets/{oid}/orders`. When an order is created, updated, or paid, all connected clients for that outlet get a broadcast. This powers the live orders panel on the admin dashboard and will power the kitchen display.

### Tests
~469 unit tests + 1 integration test that exercises the full lifecycle: login → create order → add items → multi-payment → auto-complete. Every handler file has a corresponding `_test.go` using `httptest`. The accounting module adds its own handler, matcher, and parser tests.

---

## The Android App: What Cashiers See

Built with Kotlin + Jetpack Compose + Hilt DI. Single-activity architecture with Compose Navigation.

### Screens (all complete)
- **Login**: Email/password or PIN. Tokens in `EncryptedSharedPreferences`. OkHttp interceptor auto-refreshes on 401.
- **Menu**: Horizontal category chips, full-width product list. Tap simple product → instant add. Tap product with variants → customization sheet.
- **Customization**: Radio buttons for variants (pick one per group), checkboxes for modifiers (pick many, min/max rules), quantity selector, notes.
- **Cart**: Order type chips (Dine-in/Takeaway/Delivery/Catering), table number, customer search with add-new dialog, discount section (percentage/fixed/none), subtotal/discount/total summary.
- **Payment**: Multi-payment entries (CASH/QRIS/TRANSFER chips), cash change calc, running paid/remaining bar, two-step API flow (create order → add payments). Success screen with order number.
- **Catering**: Customer display, Material3 DatePicker (future-only), delivery address, notes, 50% DP auto-calc, payment retry on orphaned order.
- **Thermal printer**: Bluetooth ESC/POS, paired devices discovery, test print, paper width selection (58/80mm), auto-print toggle, receipt + kitchen ticket formatting.
- **Settings**: Printer config accessible from menu screen gear icon.

### Bold + Clean theme
Redesigned mid-milestone. White backgrounds, tight radii (8-16dp), borders over shadows, green CTAs, yellow category accents. No dark theme. 9 color tokens, all via MaterialTheme.colorScheme.

### Dev setup quirk
No Android Studio — CLI-only. JDK 17 via Homebrew, `android-commandlinetools` cask. Testing on a physical device over USB. Debug builds point API calls to your Mac's LAN IP (`http://192.168.x.x:8081/`), which requires `usesCleartextTraffic=true` in the manifest.

---

## The Web Admin: Your Control Panel

SvelteKit 2 + Svelte 5 + Tailwind CSS v4. Server-side rendered with client-side hydration.

### All pages complete

**Authentication** (Task 9.2): Login form → Go API `/auth/login` → JWT stored in 3 httpOnly cookies (access_token, refresh_token, user_info). Server hooks parse the JWT on every request, refresh proactively 30 seconds before expiry, and populate `event.locals.user`. Protected routes redirect to `/login` if unauthenticated. Open redirect protection on login redirects.

**Dashboard** (Task 9.3): Four KPI cards (revenue, orders, avg ticket, payment method breakdown) + pure CSS hourly sales bar chart (hours 6-22) + live active orders panel that polls every 10 seconds through a SvelteKit server endpoint. All data loaded server-side in parallel via `Promise.all` in `+page.server.ts`. Orders fetched with two parallel calls (one for NEW, one for PREPARING) because the Go API's status filter only accepts one value at a time.

**Menu Management** (Task 9.4): Full CRUD for the entire menu structure. Category tabs at the top with an inline "Kelola Kategori" panel for add/edit/delete. Product grid with search and category filtering. Product detail page with sections for basic info (name, price, category, station, prep time, is_combo, is_active toggle), variant group editor (collapsible groups with inline variant CRUD and price adjustment display), modifier group editor (collapsible groups with min/max rules), and combo items editor (product dropdown with add/remove). 16 SvelteKit form actions in the product detail server file — verbose but follows the colocated-actions convention.

**Orders** (Task 9.5): Order list with tab bar (All Orders / Catering), filter bar (status, type, date range, search by order number), and offset-based pagination. Catering tab shows card layout with DP amount, remaining balance, and upcoming date highlighting. Click any order → slide-in detail panel from the right showing items with product names, variant/modifier info, payment breakdown with method badges (CASH/QRIS/TRANSFER), order summary, and status action buttons for valid transitions. Includes a visual OrderTimeline (4-step: Baru→Diproses→Siap→Selesai) with green checkmarks and a glow effect on the active step. A proxy endpoint enriches order items with product names and customer info (the Go API only returns IDs, so the SvelteKit server resolves them).

**Customer CRM** (Task 9.6): Customer list with search by name/phone, pagination, inline add/edit/delete. Customer detail page with contact card, 3 stats cards (total spend, visits, avg ticket), favorite items (top 5), and order history with pagination. All data loaded in parallel (customer + stats + orders).

**Reports** (Task 9.7): Date range picker (defaults to last 30 days), 4 tabs: Penjualan (daily sales table + bar chart), Produk (product ranking + horizontal bar chart), Pembayaran (payment method summary + stacked bar), Per Outlet (OWNER-only outlet comparison). CSV export per tab with proper RFC 4180 escaping. All charts are pure CSS — no Chart.js or external library.

**Settings & User Management** (Task 9.8): App info section (outlet name, version, coming-soon note for future settings). User CRUD with role dropdown (OWNER/MANAGER/CASHIER/KITCHEN), optional PIN (4-6 digits), inline edit (no password update — API limitation), soft-delete with confirmation. OWNER/MANAGER role guard (server-side redirect). Self-protection: can't delete your own account.

**Sidebar**: Navigation with role-based visibility — CASHIER and KITCHEN roles don't see Reports or Settings links. User info footer with initials avatar, name, and role.
- Outlet selector for Owner role (needs outlets list API endpoint)

### Architecture decisions

**No external UI library.** Every component is custom Tailwind. The hourly sales chart is pure CSS — no Chart.js, no Recharts. This means zero bundle bloat and full control over the look.

**Server-side API calls only.** The SvelteKit server acts as a proxy — it reads the JWT from cookies, calls the Go API, and returns data to the page. This means no CORS issues and no tokens exposed to the browser.

**Svelte 5 runes everywhere.** `$state`, `$derived`, `$effect`, `$props` — the new reactive primitives. No legacy `$:` syntax.

### Design tokens

Every color, spacing value, and border radius is defined as a CSS custom property in `src/app.css` and registered with Tailwind's `@theme` directive:

- **Primary Green** `#0c7721` — buttons, selected states, CTAs
- **Primary Yellow** `#ffd500` — category chip highlights
- **Error Red** `#dc2626` — destructive actions
- **Font**: DM Sans (loaded from Google Fonts)
- **Shapes**: 8px chips, 10px buttons, 12px cards, 16px sheets
- **Light-only** — no dark theme (POS environments are brightly lit)

---

## The Accounting Module: Your Financial Backbone

Added in three phases after the core POS was deployed. This is the part where the POS stops being "just a cash register" and starts being a real business management tool.

### What it does

Think of the POS as two halves: the **front of house** (taking orders, printing receipts) and the **back of house** (where did the money go?). The accounting module is the back-of-house half.

```
POS Orders ──→ Sales transactions
WhatsApp ──→ Reimbursement requests
Admin UI ──→ Purchase entries
           ↓
   acct_cash_transactions (the ledger)
           ↓
   P&L Reports / Cash Flow / Dashboard
```

### Phase 1: Master Data & Purchases (done)

The foundation. 7 new `acct_*` tables, CRUD for accounts (chart of accounts), items (inventory for matching), and cash accounts (kas/bank). Purchase entry with auto-sequencing codes (`PCS000001`, `PCS000002`...) and multi-line items.

The **item matching engine** is particularly clever — it takes free-text descriptions (like "cabe merah 5kg") and fuzzy-matches them against your inventory items using keyword scoring. Variant keywords (merah, hijau, tanjung) get 5x weight and trigger hard filtering so "cabe merah" doesn't accidentally match "cabe hijau".

### Phase 2: Reimbursements & WhatsApp (done)

This is the workflow that replaced the "send receipt photo to WhatsApp group" chaos. Staff send a structured WhatsApp message listing what they bought, and the system:

1. **Parses Indonesian text** — dates like "12 Feb" or "12 Februari", prices like "15rb" (15,000) or "1.5jt" (1,500,000), quantities like "5kg" or "3 bungkus"
2. **Matches items** to your inventory using the matching engine
3. **Creates draft reimbursement requests** automatically
4. **Returns a reply** showing what matched (Cocok), what was ambiguous (Ambigu), and what couldn't be found (Tidak cocok)

Then in the admin UI, you review the drafts, batch them together, and post them — which creates the actual cash transactions in the ledger.

The batch workflow: Draft → Ready → Posted. "Buat Batch" assigns selected items to a batch code (BTH000001). "Post Batch" creates the cash transactions and marks them as done. Simple, auditable, reversible-before-posting.

### Phase 3: Reports & Dashboard (in progress)

Financial reporting endpoints:

- **P&L (Laba Rugi)**: Groups `acct_cash_transactions` by month. Computes Net Sales, COGS, Gross Profit, itemized Expenses, Net Profit, and margin percentages. The handler takes flat DB rows grouped by (period, line_type, account) and pivots them into a structured response.
- **Cash Flow (Arus Kas)**: Cash in (SALES + CAPITAL) vs cash out (INVENTORY + EXPENSE + COGS + DRAWING) per cash account per month.
- **Dashboard (Ringkasan)**: Cash balances per account, current month P&L summary, pending reimbursement count, recent 10 transactions.

Admin pages: Laporan (two-tab P&L + Cash Flow with CSV export) and Ringkasan (dashboard with KPI cards and transaction table).

### Accounting architecture decisions

| Decision | Why |
|----------|-----|
| Separate `acct_*` tables (not extending POS tables) | Clean domain boundary. Accounting is its own world with its own rules. |
| Single `acct_cash_transactions` ledger | One table for all financial entries — purchases, reimbursements, sales. Simplifies reporting queries. |
| Sequential transaction codes (`PCS000001`, `BTH000001`) | Human-readable, grep-friendly, sortable. Generated via `COALESCE(MAX(code), 'XXX000000')` + increment. |
| WhatsApp parser as separate package | Testable in isolation (28 test cases). No HTTP dependencies. |
| Batch posting (not immediate posting) | Owner reviews before anything hits the ledger. Mistake recovery is easy — just don't post. |

---

## Infrastructure & Deployment

### Production stack

Everything runs on a single Tencent Cloud VPS (Ubuntu 24.04, 2GB RAM) that also hosts your other services (n8n, NocoDB, WAHA). The POS services are lightweight:

```
pos-api    → Go binary (~20-30MB RAM)
pos-admin  → Node.js server (~50-80MB RAM)
pos-db     → PostgreSQL 16 (~100MB RAM)
```

All behind Nginx Proxy Manager with Cloudflare DNS (Full Strict SSL, WebSockets ON).

### Docker networking

Two networks: `pos-internal` (private, database can't be reached from outside) and `proxy` (shared with NPM for reverse proxying). The API and admin sit on both networks — they can talk to the database internally and be reached by NPM externally.

### CI/CD pipeline

```
Push to main → GitHub Actions → Run tests → Build Docker image →
Push to ghcr.io (staging tag) → SSH deploy to VPS → Health check

Tag v1.0.0 → Promote staging → latest + versioned tag →
Deploy production → Health check both endpoints
```

### Backups

Daily PostgreSQL dumps at 2am via cron, gzipped, 30-day retention.

---

## Key Decisions & Their Rationale

| Decision | Why | Tradeoff |
|----------|-----|----------|
| sqlc over ORM | Type-safe SQL, no magic, version-controlled queries | Extra codegen step (`make api-sqlc`) |
| Soft deletes everywhere | Audit trail, data integrity, regulatory compliance | Slightly more complex queries |
| Price snapshots on orders | Historical accuracy for accounting | More storage per order |
| Multi-payment table | Real split payments (cash + QRIS is common) | More complex payment reconciliation |
| PIN as plaintext | Fast cashier shifts, outlet-scoped, low-risk | Less secure than hashed (accepted) |
| No dark theme | POS terminals in bright kitchens/counters | Some users prefer dark mode |
| httpOnly cookies for JWT | Tokens can't be stolen via XSS | Need server-side proxy for API calls |
| WebSocket per outlet | Real-time kitchen/dashboard updates | Stateful connections, harder to horizontally scale |
| Monorepo with Makefile | Single source of truth, unified commands | Each project still needs its own tooling |
| CLI-only Android dev | No Android Studio bloat, lighter setup | Harder to debug UI layouts |

---

## Worktree Strategy

Feature branches are developed in git worktrees under `.worktrees/`:

```
pos-superpower/                          ← main branch (stable)
├── .worktrees/
│   ├── milestone-8-android-pos/         ← feature/milestone-8-android-pos
│   └── milestone-9-sveltekit-admin/     ← feature/milestone-9-sveltekit-admin
```

Each worktree is a fully independent checkout. You can have the API running from main while developing the admin in a worktree. No branch switching, no stashing, no conflicts.

---

## Milestone Progress

| # | Milestone | Status | Notes |
|---|-----------|--------|-------|
| 1 | Project scaffold & database | Done | Schema, migrations, seed data |
| 2 | Auth & middleware | Done | JWT, PIN login, role-based access |
| 3 | Menu management API | Done | Full CRUD for categories, products, variants, modifiers, combos |
| 4 | Orders & payments API | Done | Create, update status, cancel, multi-payment |
| 5 | CRM, reports, WebSocket | Done | Customers, 5 report types, live order broadcast |
| 6 | Router assembly & integration test | Done | Full lifecycle test with real PostgreSQL |
| 7 | Docker production setup | Done | Compose files, Dockerfiles, backup script |
| 8 | Android POS app | Done | 8 screens + theme redesign. 100 files, 10,384 lines. |
| 9 | SvelteKit admin panel | Done | 8 tasks: scaffold, auth, dashboard, menu, orders, CRM, reports, settings. ~35 files. |
| 10 | Deployment & seed script | Done | CI/CD pipeline, VPS deploy, seed script. v1.0.0 deployed. |
| — | Android Order Flow | Done | Cart edit mode, payment for existing orders, bill/receipt/share. 10 tasks. |
| — | Remove PG Enums | Done | VARCHAR(20) + CHECK constraints replace 9 enum types. Cleaner migrations. |
| — | Android Admin Features | Done | Reports, CRM, menu admin, staff management, drawer navigation. 11 tasks. |
| — | Accounting Phase 1 | Done | 7 tables, master data CRUD, item matching engine, purchase entry. 12 tasks. |
| — | Accounting Phase 2 | Done | Reimbursement CRUD, WhatsApp parser, batch workflow, admin page. 10 tasks. |
| — | Accounting Phase 3 | **In Progress** | P&L + Cash Flow report handlers done. Dashboard + admin pages pending. |

**Core POS complete and deployed.** Accounting module in active development (Phase 3: reports + dashboard).

---

## Lessons Learned

### The WebSocket NPM bug
Nginx Proxy Manager has a bug where enabling WebSocket support on a proxy host can generate duplicate `proxy_http_version` directives, making the host show "Online" in the UI but return 404 from openresty. The fix is to check the `.conf` file and remove the duplicate. There's a skill for this (`npm-websocket-proxy-config-bug`).

### sqlc and nullable columns
When sqlc generates Go code for nullable columns, it uses `sql.NullString`, `sql.NullInt64`, etc. These are annoying to work with in handlers. The pattern is to convert them to pointer types (`*string`, `*int`) at the handler boundary before sending JSON responses.

### Android debug networking
The debug build talks to your Mac's LAN IP, not `localhost` (that would be the Android device itself). You need `usesCleartextTraffic=true` for HTTP in debug and your Mac's firewall must allow port 8081. Check your current IP with `ipconfig getifaddr en0`.

### Svelte 5 runes
The reactive system changed completely from Svelte 4. No more `$:` reactive declarations. Everything is `$state()`, `$derived()`, `$effect()`. The store pattern is different too — no more `writable()`, just export functions that manipulate `$state`.

### Tailwind v4 + SvelteKit
Tailwind v4 uses `@theme` for design token registration instead of `tailwind.config.js`. The Vite plugin (`@tailwindcss/vite`) replaces PostCSS. Config is now in CSS, not JavaScript. This is cleaner but the docs are still sparse — most Stack Overflow answers reference v3 patterns.

### Price as string, not number
The Go API returns monetary amounts as strings (because `decimal(12,2)` → `shopspring/decimal` → JSON string). Both the admin and Android app must parse these strings, never assume they're numbers. `formatRupiah()` in the admin handles this by parsing to float then formatting with `toLocaleString('id-ID')`.

### JWT payload ≠ login response
The Go API's JWT access token only contains `user_id`, `outlet_id`, `role`, `exp`, `iat`. It does NOT contain `email` or `full_name` — those fields only come in the `/auth/login` response body. The admin works around this with a supplementary `user_info` httpOnly cookie that stores the fields missing from the JWT. If you ever change the JWT claims in Go, check all three places: Go `auth/jwt.go`, Android `TokenRepository`, and SvelteKit `hooks.server.ts`.

### Go API list responses are wrapped
Order list endpoints return `{orders: [...], limit, offset}` — not a bare array. Every SvelteKit page consuming list endpoints needs to unwrap. If you forget, things silently break (an object isn't iterable).

### Go API returns IDs, not names on order items
The order detail endpoint returns `product_id`, `variant_id`, `modifier_id` — but NOT `product_name`, `variant_name`, or `modifier_name`. The Go API was built write-optimized (price snapshots at order time), but the read path doesn't JOIN to resolve human-readable names. The admin works around this with a SvelteKit proxy endpoint (`/api/orders/[id]`) that fetches the order AND the products list in parallel, then enriches items with product names before returning to the client. Same pattern for customer info. This is a pragmatic fix but the real solution is adding JOINs to the Go API's order detail query. Until then, variant and modifier names show generic labels.

### ADMIN vs MANAGER role enum
The Go API and PostgreSQL schema use `MANAGER` as the role enum value, but the SvelteKit admin was written with `ADMIN`. This meant MANAGER users couldn't see Reports or Settings links in the sidebar — the role check `['OWNER', 'ADMIN'].includes(user.role)` always failed because the JWT contains `MANAGER`. The fix was trivial (replace `ADMIN` with `MANAGER` in two files), but the lesson is: always verify your TypeScript enum values against the actual database schema, not what you think they should be. The source of truth is `000001_init_schema.up.sql`.

### Go query params: one value per key
`r.URL.Query().Get("key")` returns only the first value. `?status=NEW&status=PREPARING` won't give you both — only `NEW`. To filter by multiple statuses, make separate API calls and merge results client-side.

### sqlc COALESCE+aggregate needs explicit cast
`COALESCE(MAX(code), 'PCS000000')` generates `interface{}` in sqlc because the type inference can't determine the output type of a COALESCE wrapping an aggregate. The fix: `COALESCE(MAX(code), 'PCS000000')::text AS next_code`. Always add `::text AS alias` on COALESCE+aggregate expressions. This cost hours to debug the first time.

### pgtype.Numeric → string: the non-obvious conversion
`pgtype.Numeric.Int.String()` gives the raw `big.Int` representation — e.g., "1500050" for the decimal value 15000.50. The `Exp` field (-2) tells you where the decimal point goes, but `Int.String()` ignores it entirely. The correct pattern: `n.Value()` → parse with `shopspring/decimal.NewFromString()` → `.StringFixed(2)`. This is wrapped in `numericToStringPtr()` in the accounting handler.

### PostgreSQL DEFAULT values block DROP TYPE
When migrating away from PG enums to VARCHAR, you can't just `DROP TYPE my_enum` — if any column has a DEFAULT using that type, the DROP fails. You must `ALTER COLUMN ... DROP DEFAULT` first, then drop the type, then re-add the default with a string literal. The down migration has the same issue in reverse. Easy to miss, especially with 9 enum types across 14 tables.

### WhatsApp parser: Indonesian date edge cases
Parsing "12 Des" in January means December of last year, not this year. A naive parser assigns the current year, producing a future date that makes no sense. Fix: if the parsed date is more than 30 days in the future, subtract a year. Also: Indonesian has duplicate month abbreviations that look like typos — "Okt" vs "Oktober", "Agt" vs "Agustus". Support both short and long forms.

### shopspring/decimal panics on Div(0)
Unlike Go's built-in float division (which returns Inf or NaN), `decimal.Div(decimal.Zero)` panics. This bit us in the WhatsApp handler when a parsed quantity was zero. Always guard: `if qty.IsZero() { continue }` before any division.

### Batch reimbursement: `:exec` vs `:execrows` in sqlc
An `UPDATE ... WHERE id = ANY($1)` with sqlc's `:exec` returns `nil` error even when zero rows were affected. The handler counted "5 assigned" when really 0 matched. Switch to `:execrows` to get `RowsAffected()` and verify the count. Small sqlc annotation change, big correctness difference.

---

## Quick Reference

### Test credentials (local dev only)
```
Email:     admin@kiwari.com
Password:  password123
PIN:       1234
Outlet:    17fbe5e3-6dea-4a8e-9036-8a59c345e157
```

### Common commands
```bash
make db-up && make db-migrate && make db-seed   # Bootstrap local DB
make api-run                                     # Start API on :8081
make admin-dev                                   # Start admin on :5173
make api-test                                    # Run all ~469 tests
make docker-up                                   # Full production stack
```

### Key URLs (production)
```
API:    https://pos-api.nasibakarkiwari.com
Admin:  https://pos.nasibakarkiwari.com
```

### Reference docs
```
docs/plans/2026-02-06-pos-system-design.md          # Full system design
docs/plans/2026-02-06-backend-plan.md               # Backend milestones
docs/plans/2026-02-06-android-pos-plan.md           # Android milestones
docs/plans/2026-02-06-sveltekit-admin-plan.md       # Admin milestones
docs/plans/2026-02-11-accounting-module-design.md   # Accounting system design
docs/plans/2026-02-11-accounting-phase1-plan.md     # Accounting Phase 1
docs/plans/2026-02-12-accounting-phase2-plan.md     # Accounting Phase 2
docs/plans/2026-02-12-accounting-phase3-plan.md     # Accounting Phase 3 (active)
PROGRESS.md                                          # Implementation tracker
```
