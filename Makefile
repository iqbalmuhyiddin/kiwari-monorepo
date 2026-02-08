.PHONY: help api-run api-test api-lint db-up db-down db-migrate db-rollback db-seed \
        admin-dev admin-build admin-test android-build docker-up docker-down

help:                          ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Database ──────────────────────────────
db-up:                         ## Start local PostgreSQL
	docker compose -f docker/docker-compose.dev.yml up -d

db-down:                       ## Stop local PostgreSQL
	docker compose -f docker/docker-compose.dev.yml down

db-migrate:                    ## Run all pending migrations
	migrate -path api/migrations -database "$${DATABASE_URL}" up

db-rollback:                   ## Rollback last migration
	migrate -path api/migrations -database "$${DATABASE_URL}" down 1

db-reset:                      ## Drop and recreate all tables
	migrate -path api/migrations -database "$${DATABASE_URL}" drop -f
	migrate -path api/migrations -database "$${DATABASE_URL}" up

db-seed:                       ## Load example data for dev/demo
	docker compose -f docker/docker-compose.dev.yml cp api/seed/seed.sql postgres:/tmp/seed.sql
	docker compose -f docker/docker-compose.dev.yml exec -T postgres psql -U pos -d pos_db -f /tmp/seed.sql

# ── Go API ────────────────────────────────
api-run:                       ## Run API server
	cd api && go run ./cmd/server

api-test:                      ## Run API tests
	cd api && go test ./... -v

api-lint:                      ## Lint Go code
	cd api && golangci-lint run

api-sqlc:                      ## Regenerate sqlc code
	cd api && sqlc generate

# ── SvelteKit Admin ───────────────────────
admin-dev:                     ## Start admin dev server
	cd admin && pnpm dev

admin-build:                   ## Build admin for production
	cd admin && pnpm build

admin-test:                    ## Run admin tests
	cd admin && pnpm test

admin-install:                 ## Install admin dependencies
	cd admin && pnpm install

# ── Android ───────────────────────────────
android-build:                 ## Build Android debug APK
	cd android && ./gradlew assembleDebug

android-test:                  ## Run Android unit tests
	cd android && ./gradlew test

# ── Docker Production ─────────────────────
docker-up:                     ## Build and start production stack
	docker compose -f docker/docker-compose.yml up -d --build

docker-down:                   ## Stop production stack
	docker compose -f docker/docker-compose.yml down

docker-logs:                   ## Tail production logs
	docker compose -f docker/docker-compose.yml logs -f
