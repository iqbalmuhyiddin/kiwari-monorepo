# CLAUDE.md

## Project Overview

Kiwari POS — multi-outlet Point of Sale for F&B (Nasi Bakar). Monorepo: Go API, SvelteKit admin, Android POS.

```
api/          Go REST+WebSocket API (Chi, sqlc, PostgreSQL)
admin/        SvelteKit 2 admin (Svelte 5, Tailwind CSS, pnpm)
android/      Android POS (Kotlin, Jetpack Compose, Hilt, Retrofit)
docker/       Docker Compose (dev + production)
docs/plans/   Design docs and implementation plans
```

All clients → Go API. Data scoped by `outlet_id`. WebSocket for live order updates. Makefile at root glues everything.

## Tech Stack

- **API**: Go 1.22+, Chi router, sqlc, golang-migrate, pgx/v5, JWT (golang-jwt)
- **DB**: PostgreSQL 16, 14 tables, money as `decimal(12,2)` (shopspring/decimal)
- **Admin**: SvelteKit 2, Svelte 5, Tailwind CSS 4, pnpm
- **Android**: Kotlin, Jetpack Compose, Hilt, Retrofit+OkHttp, minSdk 26
- **Infra**: Docker Compose, Nginx Proxy Manager, Tencent Cloud VPS (Ubuntu 24.04)

## Commands

```bash
make db-up / db-down / db-migrate / db-rollback / db-reset
make api-run (port 8081) / api-test / api-lint / api-sqlc
make admin-install / admin-dev (port 5173) / admin-build / admin-test
make android-build / android-test
make docker-up / docker-down / docker-logs
# Single test: cd api && go test ./internal/handler/ -v
# Named test: cd api && go test -run TestOrderCreation -v
```

## Key Design Decisions

- **sqlc only**: SQL in `api/queries/*.sql` → generated to `api/internal/database/`. No raw SQL in handlers.
- **Soft deletes**: FK tables use `is_active=false`. Orders use `status=CANCELLED`. Payments never deleted.
- **Price snapshots**: `order_items.unit_price` frozen at order creation. Menu changes don't affect history.
- **Multi-payment**: Multiple `payments` rows per order (cash + QRIS + transfer splits).
- **Catering**: `order_type=CATERING`, `catering_status` (BOOKED→DP_PAID→SETTLED). DP = first payment.
- **Customization**: Variant groups (pick one/group) vs Modifier groups (pick many, min/max). Separate tables.

## Design Tokens (Bold + Clean)

```
Green: #0c7721  Yellow: #ffd500  Red: #dc2626
Text: #1a1a1a  TextSec: #6b7280  Border: #e5e7eb  Surface: #f8f9fa
Font: Roboto (system)  Theme: Light-only
Shapes: 8dp chips, 10dp buttons, 12dp cards, 16dp sheets
```

## Implementation Patterns

- **Handlers**: Consumer-defines-interface (`AuthStore`, `UserStore`). Mock tests with `httptest`. `RegisterRoutes(r chi.Router)`. Shared `writeJSON`.
- **Validation**: Email requires `@`. PIN 4-6 digits. Dup email → 409 (pgconn `23505`). Soft delete `:one` + `RETURNING id`.
- **PIN storage**: Plaintext by design — low-entropy cashier/kitchen access only.
- **Worktrees**: `.worktrees/` for feature branch isolation.

## Deployment

VPS at `nasibakarkiwari.com`: `pos-api.` (API:8081), `pos.` (admin:3001). NPM + Cloudflare DNS (Full Strict SSL, WS ON).
CI/CD: GitHub Actions → ghcr.io → SSH deploy. Push main → staging, tag v* → production.

## Environment

- Go: `/opt/homebrew/bin/go`. sqlc/migrate: `~/go/bin/` (needs PATH export). Go commands from `api/`.
- Android: CLI-only (no Studio). JDK 17 Homebrew. `cd android && ./gradlew installDebug` via USB. Debug URL = Mac LAN IP, `usesCleartextTraffic=true`.
- **No API version prefix**: routes are `/auth/login`, `/outlets/{oid}/...` — base URL = `http://host:port/`
- **Test creds**: `admin@kiwari.com` / `password123` / PIN `1234` / outlet `17fbe5e3-6dea-4a8e-9036-8a59c345e157`

## Reference Documents

- `docs/plans/2026-02-06-pos-system-design.md` — full system design
- `docs/plans/2026-02-06-backend-plan.md` — backend plan (M1-7 done)
- `docs/plans/2026-02-06-android-pos-plan.md` — Android plan (M8 done)
- `docs/plans/2026-02-06-sveltekit-admin-plan.md` — Admin plan (M9 in progress)
- `docs/plans/2026-02-08-android-order-flow-design.md` — order flow redesign
- `docs/plans/2026-02-08-android-order-flow-plan.md` — order flow impl plan
- `PROGRESS.md` — progress tracker
