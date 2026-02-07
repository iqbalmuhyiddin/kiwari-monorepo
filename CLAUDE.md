# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kiwari POS — custom multi-outlet Point of Sale system for an F&B business (Nasi Bakar). Monorepo with three codebases: Go API backend, SvelteKit web admin, and Android POS app (Kotlin/Jetpack Compose).

## Architecture

```
api/          Go REST+WebSocket API (Chi router, sqlc, PostgreSQL)
admin/        SvelteKit 2 web admin panel (Svelte 5, Tailwind CSS)
android/      Android POS app (Kotlin, Jetpack Compose, Hilt, Retrofit)
docker/       Docker Compose configs (dev + production)
docs/plans/   Design doc and implementation plan
```

All three clients talk to the Go API. Data scoped by `outlet_id` for multi-outlet isolation. WebSocket used for live order updates (kitchen, dashboard).

## Tech Stack

- **API**: Go 1.22+, Chi router, sqlc (type-safe SQL), golang-migrate, pgx/v5, JWT (golang-jwt)
- **Database**: PostgreSQL 16, 14 tables, all money as `decimal(12,2)` via shopspring/decimal
- **Admin**: SvelteKit 2, Svelte 5, Tailwind CSS, pnpm
- **Android**: Kotlin, Jetpack Compose, Hilt DI, Retrofit + OkHttp, Min SDK 26
- **Infra**: Docker Compose, Nginx Proxy Manager, Tencent Cloud VPS (Ubuntu 24.04)

## Monorepo Tooling

Makefile at root is the glue layer. Each project uses its own native build system underneath.

## Commands

```bash
# ── Database ──────────────────
make db-up              # Start local PostgreSQL (Docker)
make db-down            # Stop local PostgreSQL
make db-migrate         # Run all pending migrations
make db-rollback        # Rollback last migration
make db-reset           # Drop and recreate all tables

# ── Go API ────────────────────
make api-run            # Run API server (port 8081)
make api-test           # Run all Go tests
make api-lint           # Lint Go code (golangci-lint)
make api-sqlc           # Regenerate sqlc code from queries/

# Single test / package test
cd api && go test ./internal/handler/ -v
cd api && go test -run TestOrderCreation -v

# ── SvelteKit Admin ───────────
make admin-install      # Install pnpm dependencies
make admin-dev          # Dev server (port 5173)
make admin-build        # Production build
make admin-test         # Run tests

# ── Android ───────────────────
make android-build      # Build debug APK
make android-test       # Run unit tests

# ── Docker Production ─────────
make docker-up          # Build and start production stack
make docker-down        # Stop production stack
make docker-logs        # Tail production logs

# ── Help ──────────────────────
make help               # List all available targets
```

## Key Design Decisions

- **sqlc for all DB queries**: SQL in `api/queries/*.sql`, Go code generated to `api/internal/database/`. Never write raw SQL in Go handlers.
- **Soft deletes everywhere**: Tables with FK references use `is_active=false`. Orders use `status=CANCELLED`. Payments are never deleted.
- **Price snapshots**: `order_items.unit_price` and `order_item_modifiers.unit_price` freeze at order creation time. Menu price changes don't affect history.
- **Multi-payment**: `payments` table allows multiple rows per order (split cash + QRIS + transfer).
- **Catering lifecycle**: `order_type=CATERING` with `catering_status` (BOOKED→DP_PAID→SETTLED). Down payment is the first payment record.
- **Product customization**: Variant groups (pick one per group) vs Modifier groups (pick many with min/max). Separate concepts, separate tables.

## Brand Design Tokens

```
Primary Green:  #0c7721  (actions, success, primary buttons)
Primary Yellow: #ffd500  (headers, highlights, accents)
Border Yellow:  #ffea60  (active states, subtle borders)
Accent Red:     #d43b0a  (errors, destructive actions)
Dark Grey:      #262626  (text light mode, bg dark mode)
Font:           Inter (Roboto fallback)
```

## Deployment

Production runs on Tencent Cloud VPS at `nasibakarkiwari.com`:
- `pos-api.nasibakarkiwari.com` → Go API (port 8081)
- `pos.nasibakarkiwari.com` → SvelteKit admin (port 3001)
- All behind Nginx Proxy Manager on shared `proxy` Docker network
- Cloudflare DNS with Full (Strict) SSL, WebSockets ON

## Implementation Patterns

- **Handler pattern**: Consumer-defines-interface (`AuthStore`, `UserStore`) for DB access. Mock-based unit tests with `httptest`. Chi router `RegisterRoutes(r chi.Router)`. Shared `writeJSON` helper with error logging.
- **Validation**: Email requires `@`. PIN must be 4-6 digits. Duplicate email → 409 Conflict (catch pgconn error code `23505`). Soft delete with `:one` + `RETURNING id` for proper 404 on non-existent.
- **PIN storage**: Plaintext by design — low-entropy, only grants cashier/kitchen access within known outlet.
- **Worktrees**: Using `.worktrees/` directory for feature branch isolation.

## Environment Notes

- Go installed via Homebrew: `/opt/homebrew/bin/go`
- `sqlc` and `migrate` at `~/go/bin/` — need `export PATH="$HOME/go/bin:$PATH"`
- All `go` commands run from `api/` subdirectory (that's where `go.mod` lives)

## Reference Documents

- `docs/plans/2026-02-06-pos-system-design.md` — full design: data model, API endpoints, screen flows, deployment architecture
- `docs/plans/2026-02-06-backend-plan.md` — backend build plan (M1-7 done), Docker, Deploy
- `docs/plans/2026-02-06-android-pos-plan.md` — Android POS plan (M8, priority)
- `docs/plans/2026-02-06-sveltekit-admin-plan.md` — SvelteKit Admin plan (M9)
- `PROGRESS.md` — implementation progress tracker
