# CI/CD Design — Kiwari POS

**Date:** 2026-02-08
**Status:** Draft

## Overview

Automated CI/CD pipeline for the Kiwari POS monorepo using GitHub Actions, GitHub Container Registry (ghcr.io), and SSH-based deployment to a single Tencent Cloud VPS. Supports two environments: staging and production.

## Decisions

| Decision | Choice |
|----------|--------|
| Environments | Staging + Production on same VPS |
| Trigger model | TBD: push to main → staging, tag v* → production |
| Deploy mechanism | Build in CI (GitHub Actions), push to ghcr.io, SSH to VPS to pull |
| Monorepo handling | Path-filtered separate workflows per service |
| Migrations | Manual (SSH in, run before deploy) |
| Health check | Basic curl after deploy, no auto-rollback |
| Production promotion | Re-tag staging image (no rebuild) |
| Versioning | Unified monorepo version (single tag for all services) |
| Database | Single PostgreSQL instance, separate DBs + users per env |
| Android CI | Deferred |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    GitHub (main branch)                  │
│                                                         │
│  push to main ──► path filter ──► which changed?        │
│                                     │         │         │
│                                  api/**    admin/**      │
│                                     │         │         │
│                                     ▼         ▼         │
│                               api-ci.yml  admin-ci.yml  │
│                               test → build → deploy stg │
│                                                         │
│  tag v1.x.x ──► promote.yml                             │
│                  re-tag :staging → :v1.x.x + :latest    │
│                  deploy to production                    │
└──────────────────┬──────────────────────────────────────┘
                   │
            ┌──────▼──────┐
            │   ghcr.io   │
            │  kiwari-api  │
            │  kiwari-admin│
            └──────┬──────┘
                   │ SSH + docker compose pull
                   ▼
         ┌──────────────────────────────────┐
         │     Tencent Cloud VPS (2GB)      │
         │     Ubuntu 24.04                 │
         │                                  │
         │  postgres (shared)               │
         │    ├── pos_staging DB            │
         │    └── pos_production DB         │
         │                                  │
         │  pos-staging/                    │
         │    ├── pos-api-staging           │
         │    └── pos-admin-staging         │
         │                                  │
         │  pos-production/                 │
         │    ├── pos-api                   │
         │    └── pos-admin                 │
         │                                  │
         │  nginx-proxy-manager             │
         └──────────────────────────────────┘
```

## Domains

| Service | Staging | Production |
|---------|---------|------------|
| API | stg-api.nasibakarkiwari.com | api.nasibakarkiwari.com |
| Admin | stg-admin.nasibakarkiwari.com | admin.nasibakarkiwari.com |

## Pipeline Flows

### Staging Deploy (push to main)

```
1. Developer pushes/merges to main
2. GitHub Actions detects changed paths
3. If api/** changed → api-ci.yml triggers:
   a. Checkout code
   b. Run go test ./... (all 401+ tests)
   c. Build Docker image (multi-stage, same Dockerfile.api)
   d. Push to ghcr.io/<org>/kiwari-api:staging
   e. SSH to VPS
   f. cd ~/docker/pos-staging && docker compose pull && docker compose up -d
   g. Health check: curl https://stg-api.nasibakarkiwari.com/health
4. If admin/** changed → admin-ci.yml triggers (same flow, different image)
```

### Production Deploy (tag)

```
1. Developer creates tag: git tag v1.2.0 && git push --tags
2. promote.yml triggers:
   a. Pull ghcr.io/<org>/kiwari-api:staging
   b. Re-tag as :v1.2.0 and :latest
   c. Push new tags to ghcr.io
   d. Pull ghcr.io/<org>/kiwari-admin:staging
   e. Re-tag as :v1.2.0 and :latest
   f. Push new tags to ghcr.io
   g. SSH to VPS
   h. cd ~/docker/pos-production && docker compose pull && docker compose up -d
   i. Health check: curl https://api.nasibakarkiwari.com/health
```

### Migration Flow (manual)

```
1. Developer adds new migration file to api/migrations/
2. Push to main → staging deploys (new binary has migration files)
3. SSH to VPS
4. Run migration against staging DB:
   migrate -path /path/to/migrations -database "$STAGING_DATABASE_URL" up
5. Test staging
6. Tag release
7. Production deploys
8. Run migration against production DB:
   migrate -path /path/to/migrations -database "$PRODUCTION_DATABASE_URL" up
```

## Workflow Files

### `.github/workflows/api-ci.yml`

Triggers:
- `push` to `main` with paths `api/**`, `docker/Dockerfile.api`

Jobs:
1. **test** — Go test suite on ubuntu-latest
2. **build-and-push** — Docker build + push to ghcr.io (needs: test)
3. **deploy-staging** — SSH to VPS, pull + restart (needs: build-and-push)

### `.github/workflows/admin-ci.yml`

Triggers:
- `push` to `main` with paths `admin/**`, `docker/Dockerfile.admin`

Jobs:
1. **build-and-push** — Docker build + push to ghcr.io
2. **deploy-staging** — SSH to VPS, pull + restart (needs: build-and-push)

### `.github/workflows/promote.yml`

Triggers:
- `push` tags matching `v*`

Jobs:
1. **promote-images** — Re-tag :staging → :version + :latest for both services
2. **deploy-production** — SSH to VPS, pull + restart (needs: promote-images)
3. **health-check** — Verify production endpoints respond (needs: deploy-production)

## VPS Directory Structure

```
/home/iqbal/docker/
├── nginx-proxy-manager/        # existing
├── portainer/                  # existing
├── n8n/                        # existing
├── postgres/                   # NEW — shared PostgreSQL 16
│   ├── docker-compose.yml
│   ├── .env
│   └── init/
│       └── init-databases.sh   # creates both DBs + users on first run
├── pos-staging/                # NEW
│   ├── docker-compose.yml      # pulls :staging images, no DB service
│   └── .env                    # staging env vars
├── pos-production/             # NEW
│   ├── docker-compose.yml      # pulls :latest images, no DB service
│   └── .env                    # production env vars
```

## Docker Compose Files

### `postgres/docker-compose.yml`

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./init:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-ONLY", "pg_isready", "-U", "${POSTGRES_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - proxy

volumes:
  pgdata:

networks:
  proxy:
    external: true
```

### `postgres/init/init-databases.sh`

```bash
#!/bin/bash
set -e

# Create staging database and user
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    CREATE USER pos_stg WITH PASSWORD '${POS_STG_PASSWORD}';
    CREATE DATABASE pos_staging OWNER pos_stg;
    GRANT ALL PRIVILEGES ON DATABASE pos_staging TO pos_stg;

    CREATE USER pos_prod WITH PASSWORD '${POS_PROD_PASSWORD}';
    CREATE DATABASE pos_production OWNER pos_prod;
    GRANT ALL PRIVILEGES ON DATABASE pos_production TO pos_prod;
EOSQL
```

### `pos-staging/docker-compose.yml`

```yaml
services:
  pos-api-staging:
    image: ghcr.io/<org>/kiwari-api:staging
    container_name: pos-api-staging
    restart: unless-stopped
    env_file: .env
    networks:
      - proxy

  pos-admin-staging:
    image: ghcr.io/<org>/kiwari-admin:staging
    container_name: pos-admin-staging
    restart: unless-stopped
    env_file: .env
    networks:
      - proxy

networks:
  proxy:
    external: true
```

### `pos-production/docker-compose.yml`

```yaml
services:
  pos-api:
    image: ghcr.io/<org>/kiwari-api:latest
    container_name: pos-api
    restart: unless-stopped
    env_file: .env
    networks:
      - proxy

  pos-admin:
    image: ghcr.io/<org>/kiwari-admin:latest
    container_name: pos-admin
    restart: unless-stopped
    env_file: .env
    networks:
      - proxy

networks:
  proxy:
    external: true
```

## GitHub Secrets

| Secret | Value | Purpose |
|--------|-------|---------|
| `VPS_HOST` | `43.173.30.193` | SSH target |
| `VPS_USER` | `iqbal` | SSH user |
| `VPS_SSH_KEY` | Ed25519 private key | SSH authentication |
| `GHCR_TOKEN` | PAT with `read:packages` | VPS pulls private images |

Workflows use `GITHUB_TOKEN` (automatic) to push images to ghcr.io.

## One-Time Setup Checklist

### Local (Mac)

- [ ] Generate deploy SSH key: `ssh-keygen -t ed25519 -f ~/.ssh/pos-deploy -C "github-actions-deploy"`
- [ ] Add private key to GitHub repo secret `VPS_SSH_KEY`
- [ ] Create GitHub PAT with `read:packages` scope, add to secret `GHCR_TOKEN`
- [ ] Add remaining GitHub secrets (`VPS_HOST`, `VPS_USER`)

### VPS

- [ ] Add deploy public key to `~/.ssh/authorized_keys`
- [ ] Docker login to ghcr.io: `echo $TOKEN | docker login ghcr.io -u USERNAME --password-stdin`
- [ ] Create `~/docker/postgres/` with compose + init script
- [ ] Start PostgreSQL: `docker compose up -d`
- [ ] Create `~/docker/pos-staging/` with compose + .env
- [ ] Create `~/docker/pos-production/` with compose + .env
- [ ] Add 4 DNS A records in Cloudflare (stg-api, stg-admin, api, admin)
- [ ] Add 4 proxy hosts in NPM with SSL

### Codebase

- [ ] Add `/health` endpoint to Go API
- [ ] Create `.github/workflows/api-ci.yml`
- [ ] Create `.github/workflows/admin-ci.yml`
- [ ] Create `.github/workflows/promote.yml`

## Health Check

Each deploy job ends with:

```bash
sleep 10
curl -f --retry 3 --retry-delay 5 https://<domain>/health
```

API needs a `/health` endpoint:

```go
r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ok"))
})
```

Admin: check HTTP 200 on root `/`.

## NPM Proxy Host Config

| Domain | Container | Port |
|--------|-----------|------|
| stg-api.nasibakarkiwari.com | pos-api-staging | 8081 |
| stg-admin.nasibakarkiwari.com | pos-admin-staging | 3000 |
| api.nasibakarkiwari.com | pos-api | 8081 |
| admin.nasibakarkiwari.com | pos-admin | 3000 |

All with: SSL (Let's Encrypt), Force SSL, HTTP/2.
API hosts need WebSocket config in Advanced tab.
