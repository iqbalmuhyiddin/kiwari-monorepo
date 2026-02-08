# Kiwari POS

Custom multi-outlet Point of Sale system for Nasi Bakar Kiwari, an Indonesian F&B business. Monorepo with three codebases: Go REST API, SvelteKit web admin, and Android POS app.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| API | Go 1.22+, Chi router, sqlc, golang-migrate |
| Database | PostgreSQL 16 |
| Web Admin | SvelteKit 2, Svelte 5, Tailwind CSS v4, TypeScript |
| Android | Kotlin, Jetpack Compose, Hilt, Retrofit + OkHttp |
| Infra | Docker Compose, Nginx Proxy Manager, GitHub Actions |
| Hosting | Tencent Cloud VPS (Ubuntu 24.04), Cloudflare DNS |

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 20+ with pnpm
- Docker & Docker Compose
- JDK 17 and Android SDK (for Android builds)
- `sqlc` and `golang-migrate` in `$PATH`

### Installation

```bash
git clone git@github.com:iqbalmuhyiddin/kiwari-monorepo.git
cd kiwari-monorepo

# Install admin dependencies
make admin-install

# Start local database
make db-up

# Run migrations and seed data
make db-migrate
make db-seed
```

### Running

```bash
# Start API server (port 8081)
make api-run

# Start admin dev server (port 5173)
make admin-dev

# Build Android debug APK
make android-build
```

## Project Structure

```
api/            Go REST + WebSocket API server
admin/          SvelteKit web admin panel
android/        Android POS app (Kotlin/Compose)
docker/         Docker Compose configs (dev + production)
deploy/         VPS deployment compose files (staging + production)
docs/plans/     Design docs and implementation plans
.github/        CI/CD workflows
Makefile        Unified build commands
```

## Usage

### Available Commands

```bash
# Database
make db-up              # Start local PostgreSQL
make db-down            # Stop local PostgreSQL
make db-migrate         # Run pending migrations
make db-rollback        # Rollback last migration
make db-reset           # Drop and recreate tables
make db-seed            # Load seed data

# Go API
make api-run            # Run server (port 8081)
make api-test           # Run all tests
make api-lint           # Lint with golangci-lint
make api-sqlc           # Regenerate sqlc code

# SvelteKit Admin
make admin-install      # Install pnpm dependencies
make admin-dev          # Dev server (port 5173)
make admin-build        # Production build

# Android
make android-build      # Build debug APK
make android-test       # Run unit tests

# Docker Production
make docker-up          # Build and start production stack
make docker-down        # Stop production stack
make docker-logs        # Tail production logs
```

### API Endpoints

All routes are unprefixed (e.g., `/auth/login`, not `/api/v1/auth/login`).

| Group | Base Path | Auth |
|-------|-----------|------|
| Auth | `/auth/login`, `/auth/pin-login`, `/auth/refresh` | Public |
| Users | `/outlets/{oid}/users` | JWT + Outlet |
| Categories | `/outlets/{oid}/categories` | JWT + Outlet |
| Products | `/outlets/{oid}/products` | JWT + Outlet |
| Orders | `/outlets/{oid}/orders` | JWT + Outlet |
| Payments | `/outlets/{oid}/orders/{id}/payments` | JWT + Outlet |
| Customers | `/outlets/{oid}/customers` | JWT + Outlet |
| Reports | `/outlets/{oid}/reports`, `/reports` | JWT + Role |
| WebSocket | `/ws/outlets/{oid}/orders` | JWT (query param) |

### Environment Variables

Copy `docker/.env.example` to `docker/.env` and configure:

| Variable | Description |
|----------|-------------|
| `POSTGRES_USER` | Database user |
| `POSTGRES_PASSWORD` | Database password |
| `POSTGRES_DB` | Database name |
| `DATABASE_URL` | Full connection string |
| `PORT` | API server port (default: 8081) |
| `JWT_SECRET` | Secret for JWT token signing |

## Deployment

Production runs on a Tencent Cloud VPS behind Nginx Proxy Manager with Cloudflare DNS (Full Strict SSL).

| Service | URL | Internal Port |
|---------|-----|---------------|
| API | `pos-api.nasibakarkiwari.com` | 8081 |
| Admin | `pos.nasibakarkiwari.com` | 3001 |

### CI/CD Pipeline

```
Push to main → Tests → Build Docker image → Push to ghcr.io (staging) → Deploy to VPS

git tag v1.x.x → Promote staging → latest tag → Deploy production
```

Daily PostgreSQL backups at 2am via cron, 30-day retention.
