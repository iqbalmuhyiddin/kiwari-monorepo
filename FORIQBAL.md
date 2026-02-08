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
├── api/            Go backend (the finished part)
├── admin/          SvelteKit web panel (in progress)
├── android/        Android POS app (in progress)
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

## The Database: 14 Tables, No Surprises

The schema is designed around one principle: **data integrity is sacred**.

### Multi-outlet isolation
Every table that matters has an `outlet_id`. A cashier at outlet A can never see, create, or modify anything at outlet B. The API enforces this in middleware before any handler code runs.

### Money handling
All monetary values are `decimal(12,2)` — never floats. In Go, this is `shopspring/decimal`. In Kotlin, `BigDecimal`. In the admin, amounts arrive as strings from the API and get formatted to Rupiah (`Rp 1.250.000`). Floating point arithmetic and money don't mix; this is non-negotiable.

### The tables

**People & places**: `outlets`, `users` (4 roles: OWNER, MANAGER, CASHIER, KITCHEN), `customers`

**Menu structure**: `categories` → `products` → `variant_groups/variants` (pick one, like size) + `modifier_groups/modifiers` (pick many, like extra toppings) + `combo_items` (bundled products)

**Orders**: `orders` → `order_items` → `order_item_modifiers` → `payments`

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
├── auth/        JWT token generation & validation
├── config/      Env var loading
├── database/    sqlc-generated code (DON'T EDIT BY HAND)
├── handler/     HTTP handlers (one file per domain)
├── middleware/   Auth guard, role checks, outlet scoping
├── router/      Chi router wiring
├── service/     Business logic (order creation, etc.)
└── ws/          WebSocket hub for live updates
```

### SQL queries
All SQL lives in `api/queries/*.sql`. Run `make api-sqlc` and it generates type-safe Go functions in `api/internal/database/`. You never write raw SQL in handler code. If you need a new query, add it to the `.sql` file, regenerate, and you get compile-time type checking for free.

### Auth flow
- **Email + password**: standard bcrypt → JWT (15 min access token + 7 day refresh token)
- **PIN login**: for cashiers doing shift changes. Plaintext by design — it's 4-6 digits, only works within a known outlet, and only grants CASHIER access. The tradeoff is speed over security, and it's acceptable here.

### WebSocket
The API runs a WebSocket hub per outlet at `/ws/outlets/{oid}/orders`. When an order is created, updated, or paid, all connected clients for that outlet get a broadcast. This powers the live orders panel on the admin dashboard and will power the kitchen display.

### Tests
401 unit tests + 1 integration test that exercises the full lifecycle: login → create order → add items → multi-payment → auto-complete. Every handler file has a corresponding `_test.go` using `httptest`.

---

## The Android App: What Cashiers See

Built with Kotlin + Jetpack Compose + Hilt DI. Single-activity architecture with Compose Navigation.

### What's working
- **Login**: Email/password or PIN. Tokens stored in `EncryptedSharedPreferences`. OkHttp interceptor auto-refreshes on 401.
- **Menu screen**: Horizontal category chips at top, full-width product list below. Tap a simple product → instant add to cart. Tap a product with required variants → customization sheet slides up.
- **Customization**: Radio buttons for variants (pick one per group), checkboxes for modifiers (pick many, respects min/max rules), quantity selector, notes field.

### What's pending
- Cart screen (order type, discounts, customer lookup)
- Payment screen (multi-payment, cash change calculation)
- Catering booking flow
- Thermal printer (Bluetooth ESC/POS)

### Dev setup quirk
No Android Studio — CLI-only. JDK 17 via Homebrew, `android-commandlinetools` cask. Testing on a physical device over USB. Debug builds point API calls to your Mac's LAN IP (`http://192.168.x.x:8081/`), which requires `usesCleartextTraffic=true` in the manifest.

---

## The Web Admin: Your Control Panel

SvelteKit 2 + Svelte 5 + Tailwind CSS v4. Server-side rendered with client-side hydration.

### What's working

**Authentication**: Login form → Go API `/auth/login` → JWT stored in httpOnly cookies (never accessible to JavaScript). Server hooks parse the JWT on every request, refresh proactively 30 seconds before expiry, and populate `event.locals.user`. Protected routes redirect to `/login` if unauthenticated.

**Dashboard**: Four KPI cards (revenue, orders, avg ticket, payment breakdown) + pure CSS hourly sales bar chart + live orders panel that polls every 10 seconds. All data loaded server-side via `+page.server.ts`, then the live orders component takes over with client-side polling.

**Sidebar**: Navigation with role-based visibility — CASHIER and KITCHEN roles don't see Reports or Settings links.

### What's pending
- Menu management (CRUD for categories, products, variants, modifiers)
- Order list with filters and detail modals
- Customer CRM (search, stats, order history)
- Reports with charts and CSV export
- Settings (tax, receipt template, user management)

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
| 6 | Router assembly & integration test | Done | Full lifecycle test passing |
| 7 | Docker production setup | Done | Compose files, Dockerfiles, backup script |
| 8 | Android POS app | ~69% | Login, menu, customization done. Cart, payment, catering, printer pending. |
| 9 | SvelteKit admin panel | ~33% | Auth, dashboard done. Menu, orders, CRM, reports, settings pending. |
| 10 | Deployment & seed script | Not started | Final production deployment |

**Backend is 100% complete and production-ready.** The two client apps are what's left.

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
make api-test                                    # Run all 401 tests
make docker-up                                   # Full production stack
```

### Key URLs (production)
```
API:    https://pos-api.nasibakarkiwari.com
Admin:  https://pos.nasibakarkiwari.com
```

### Reference docs
```
docs/plans/2026-02-06-pos-system-design.md     # Full system design
docs/plans/2026-02-06-backend-plan.md          # Backend milestones
docs/plans/2026-02-06-android-pos-plan.md      # Android milestones
docs/plans/2026-02-06-sveltekit-admin-plan.md  # Admin milestones
PROGRESS.md                                     # Implementation tracker
```
