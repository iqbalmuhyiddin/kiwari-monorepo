# POS Implementation Progress

Tracking execution of the implementation plan (split into three files):
- `docs/plans/2026-02-06-backend-plan.md` — Go API (M1-6 done), Docker (M7 done), Deploy (M10)
- `docs/plans/2026-02-06-android-pos-plan.md` — Android POS (M8) ← PRIORITY
- `docs/plans/2026-02-06-sveltekit-admin-plan.md` — SvelteKit Admin (M9)

## Active Branch

`feature/milestone-8-android-pos` in `.worktrees/milestone-8-android-pos/` (7 commits ahead of main)

## Milestones

### Milestone 1: Project Scaffolding & Database — DONE

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 1.1: Initialize Go API project | Done | `8911b67` | Go module, main.go, /health, config loader |
| 1.2: Docker Compose for local dev | Done | `74df987` | PostgreSQL 16-alpine with healthcheck |
| 1.3: Database migrations — all 14 tables | Done | `4b155de` | 10 enums, 14 tables, indexes, triggers |
| 1.4: Set up sqlc | Done | `d19e435` | sqlc.yaml, outlet queries, generated code |

### Milestone 2: Go API — Auth & Middleware — DONE

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 2.1: JWT token generation & validation | Done | `20c3ff6` | GenerateToken, ValidateToken, 3 tests |
| 2.2: Auth middleware & outlet scoping | Done | `9234879` | Authenticate, RequireOutlet, RequireRole, 7 tests |
| 2.3: Login & PIN login handlers | Done | `71278e8` | login, pin-login, refresh. Chi router. 13 tests |
| 2.4: User CRUD handlers | Done | `e005fdf` | list, create, update, soft-delete. 24 tests |

### Milestone 3: Go API — Menu Management — DONE

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 3.1: Category CRUD | Done | `929648f` | list, create, update, soft-delete. 23 tests |
| 3.2: Product CRUD | Done | `c70283a` | 5 endpoints, decimal price handling, FK validation. 33 tests |
| 3.3: Variant Groups & Variants CRUD | Done | `a6bc06d` | 8 endpoints, cascading ownership verification. 43 tests |
| 3.4: Modifier Groups & Modifiers CRUD | Done | `0e0714e` | 8 endpoints, min/max select constraints. 51 tests |
| 3.5: Combo Items CRUD | Done | `ad913c2` | 3 endpoints (list, create, hard delete). 25 tests |

### Milestone 4: Go API — Orders & Payments — DONE

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 4.1: Order Creation (Atomic) | Done | `b6c111b` | POST /orders, service layer, tx retry, price snapshots. 46 tests (18 handler + 28 service) |
| 4.2: Order Queries & Status Management | Done | `e3587b2` | GET list (filters, pagination), GET detail (nested items/modifiers/payments), PATCH status transitions, DELETE cancel. TOCTOU fix, completed_at on COMPLETED. 32 tests |
| 4.3: Order Item Modifications | Done | `d06f626` | POST add item, PUT update qty/notes, DELETE remove item, PATCH kitchen status. Tx-wrapped writes, order total recalc. 25 tests |
| 4.4: Multi-Payment | Done | `a248215`, `8a95339` | POST add payment, GET list payments. CASH/QRIS/TRANSFER, change calculation, overpayment prevention, auto-complete order on full payment. Catering lifecycle: BOOKED→DP_PAID→SETTLED. TOCTOU fix: SELECT FOR NO KEY UPDATE inside tx. Post-review fix: explicit COMPLETED order guard. 21 tests |

### Milestone 5: Go API — CRM, Reports, WebSocket — DONE

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 5.1: Customer CRUD + Stats | Done | `347fb81` | 7 endpoints: list (search), get, create, update, soft-delete, stats (total_spend/avg_ticket/top_items), order history. Unique phone constraint handling (409). Stats scoped by outlet_id. 24 tests |
| 5.2: Reports Endpoints | Done | `593b81e` | 5 endpoints: daily-sales, product-sales, payment-summary, hourly-sales, outlet-comparison. All with date range filtering (Asia/Jakarta timezone). Owner-only outlet-comparison. Strongly-typed sqlc params (end-of-day computed in Go). 12 tests |
| 5.3: WebSocket for Live Order Updates | Done | `c2026f6` | Hub with per-outlet rooms, Client with ReadPump/WritePump, ServeWS handler with JWT auth via query param. Events: order.created/updated, item.updated, order.paid. gorilla/websocket. Thread-safe broadcast with write lock. 8 tests |

### Milestone 6: Go API — Router Assembly & Integration Test — DONE

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 6.1: Wire Everything Together | Done | `92f4930` | router.go wiring all 11 handler groups into Chi router. Auth middleware on protected routes, RequireOutlet on outlet-scoped, RequireRole("OWNER") on reports. CORS for dev+prod. WebSocket route. main.go with pgxpool, graceful shutdown. Review fix: WriteTimeout 0 for WebSocket compatibility. |
| 6.2: Integration Tests | Done | `8c5adf6` | End-to-end test with real PostgreSQL via testcontainers-go. Full lifecycle: outlet→user→login→category→product→variants→modifiers→order→multi-payment (CASH+QRIS split)→verify auto-complete→customer stats. Price snapshot assertion (57000.00). Build tag `integration`. Review fixes: split payment verification, price assertion, documentation comments. |

### Milestone 7: Docker Production Setup — DONE

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 7.1: Production Docker Compose | Done | `722dc4d` | Dockerfile.api (golang:1.25-alpine multi-stage, non-root user), Dockerfile.admin (placeholder SvelteKit), docker-compose.yml (3 services, internal network isolation, proxy network for NPM, localhost-only ports). .dockerignore added. .env.example with production vars. Review fixes: Go version match, non-root user, 127.0.0.1 port binding, internal:true network, .dockerignore. |
| 7.2: Backup Script | Done | `637c7bb` | docker/backup.sh — pg_dump + gzip, 30-day retention. Review fixes: set -o pipefail (prevents silent pg_dump failures in pipe), -s file check (non-empty), cron log redirection. |

### Milestone 8: Android POS App — IN PROGRESS

| Task | Status | Commit | Notes |
|------|--------|--------|-------|
| 8.1: Scaffold Android Project | Done | `85e3ba7` | Gradle version catalogs, Kotlin 2.1.0, Compose BOM, KSP+Hilt, Retrofit+OkHttp NetworkModule, Kiwari brand theme (7 colors, light/dark, Material 3), project structure. Review fixes: emulator debug URL (10.0.2.2), config cache disabled, shrinkResources, strict Gson. Build fixes: compileSdk 34→35 (androidx.core 1.15 requires it), removed README.md from res/ dirs (resource merger rejects non-resource files). |
| 8.2: Login Screen | Done | `e29d6be` | AuthApi (3 endpoints), LoginScreen (email+PIN modes), LoginViewModel, AuthRepository with safeApiCall helper, TokenRepository (EncryptedSharedPreferences), AuthInterceptor (Bearer header), TokenAuthenticator (auto-refresh on 401 with dual OkHttpClient), NavGraph with auth-state routing. Review fixes: @Named("auth") DI qualifier, logout→login navigation, UserRole UNKNOWN fallback, innerPadding passthrough, clearError after snackbar, security-crypto stable 1.0.0, credential clearing. Build fixes: removed `/api/v1/` prefix from base URL (Go API has no version prefix), added `usesCleartextTraffic=true` for local HTTP dev. **Tested on real device — login works.** |
| 8.3: Menu Screen | Done | `9625dc8` | 15 new files, 3 modified. MenuApi (6 endpoints), MenuRepository, CartRepository (shared singleton), CategoryChips, ProductListItem (letter avatar, qty badge, tap+long-press), CartBottomBar, QuickEditPopup, MenuViewModel (parallel data loading, category/search filtering). Shared ApiHelper extracted (DRY), CurrencyFormatter utility. Spec review fixes: nullable preparationTime/maxSelect, placeholder NavGraph routes. Code quality fixes: trailing slashes, parallel loadVariantInfo, pre-computed filteredProducts, cached NumberFormat. |
| 8.4: Product Customization Bottom Sheet | Done | `417f70d` | 3 new files, 4 modified. CustomizationScreen (full-screen, radio variants, checkbox modifiers with min/max enforcement, qty selector, notes, price calc), CustomizationViewModel (parallel variant+modifier loading, real-time BigDecimal price), SelectedProductRepository (@Volatile bridge). Refactored CartItem.selectedVariant→selectedVariants (List) to support multi-variant-group products. Spec review fix: multi-variant bug (only first group stored). Code quality fixes: retry() after product cleared, filter empty variant/modifier groups, one-shot event pattern, qty cap 999, buildConstraintHint min==max. |
| 8.4b: Bold + Clean Theme Redesign | Done | `9a44467`, `5ef9dee`, `4583e31` | 3 commits, 8 files modified. New color palette (9 tokens, removed 10 old), removed dark theme entirely, added custom Shapes (8-16dp), tightened typography 11-20sp, replaced all hardcoded color imports with MaterialTheme.colorScheme tokens, category chips green→yellow selected state, avatar circle→rounded rect, elevation 8→4dp. Design spec: `docs/plans/2026-02-07-android-theme-redesign.md`. Implementation plan: `docs/plans/2026-02-07-android-theme-implementation.md`. |
| 8.5: Cart Screen | Pending | | |
| 8.6: Payment Screen | Pending | | |
| 8.7: Catering Booking Screen | Pending | | |
| 8.8: Thermal Printer Integration | Pending | | |

### Milestones 9-10 — NOT STARTED

> **Note:** Milestones 9 (SvelteKit) and 10 (Deploy) have separate plan files.
> See plan file references at top of this document.

## Test Count

401 unit tests passing (3 auth + 303 handler + 28 service + 7 middleware + 8 ws + 52 subtests) + 1 integration test (build tag: `integration`)

## Resume Prompt

After `/clear`, use:
```
Read PROGRESS.md and the Android plan file (android-pos-plan.md), then continue from the next pending task (8.5 Cart Screen) using subagent-driven-development. Working in worktree .worktrees/milestone-8-android-pos/ on branch feature/milestone-8-android-pos.
```

## Session Log

- **2026-02-07**: Milestone 3 tasks 3.1–3.4 completed. Each task went through subagent-driven-development: implement → spec review → code quality review → fix → commit. Task 3.5 (Combo Items) pending.
- **2026-02-07**: Session 2 — Completed 3.5 (Combo Items) and 4.1 (Order Creation). Milestone 3 now DONE. Milestone 4 started. Task 4.1 introduced first service layer (`service/order.go`) with transaction handling, price snapshots, discount math, and retry-on-conflict for order numbers. Two review cycles caught: missing service tests (added 28), race condition fix, string-matching error classification replaced with sentinel errors.
- **2026-02-07**: Session 3 — Completed 4.2 (Order Queries & Status) and 4.3 (Order Item Modifications). Each went through full subagent-driven-development cycle. Key review findings fixed: (4.2) TOCTOU race on status transitions → added WHERE status=$current to SQL, completed_at never set → CASE WHEN COMPLETED, inconsistent cancel rules → synced PATCH/DELETE. (4.3) Runtime blocker: updated_at column missing from order_items → removed from SQL, no transaction on AddItem → added TxBeginner to handler, kitchen status on cancelled orders → added status check. 336 tests passing.
- **2026-02-07**: Session 4 — Completed 4.4 (Multi-Payment) and 5.1 (Customer CRUD + Stats). Milestone 4 now DONE. Milestone 5 started. Key review findings fixed: (4.4) Spec review caught TOCTOU race on payment validation → moved GetOrder+SumPayments inside tx with SELECT FOR NO KEY UPDATE row lock. Code quality approved. (5.1) Both reviewers caught missing GET single customer endpoint → added. Code quality reviewer caught stats queries not scoped by outlet_id → added AND o.outlet_id=$2. Also flagged LIKE wildcard injection and soft-delete vs unique constraint — deferred as schema-level concerns. 380 tests passing.
- **2026-02-07**: Session 5 — Merged `feature/milestone-2-auth` worktree into `main` (fast-forward, 15 commits). Worktree and branch cleaned up. Post-merge review of Task 4.4 (Multi-Payment): code quality reviewer caught that COMPLETED orders weren't explicitly blocked from accepting payments (only indirectly via sum check). Fix: added explicit COMPLETED guard + test, also fixed `TestAddPayment_AlreadyFullyPaid` which was silently testing wrong code path after guard addition (changed order status from COMPLETED→NEW). 381 tests passing.
- **2026-02-07**: Session 6 — Completed 5.2 (Reports) and 5.3 (WebSocket). Milestone 5 now DONE. Used worktree `feature/milestone-5-crm-reports-ws`, merged to main (fast-forward, 2 commits). Key review findings fixed: (5.2) Timezone mismatch — `time.Parse` produces UTC but orders are Asia/Jakarta, early morning orders missed → fixed with `time.ParseInLocation("2006-01-02", s, jakartaLoc)`. Untyped sqlc params from `$3 + interval '1 day'` → moved end-of-day to Go. Test package `handler` → `handler_test`. Added date range validation. (5.3) Misleading mutex in broadcast → changed to single write lock covering entire broadcast body. 401 tests passing.
- **2026-02-07**: Session 7 — Completed 6.1 (Router Assembly) and 6.2 (Integration Tests). Milestone 6 now DONE. Used worktree `feature/milestone-6-router-integration`, merged to main (fast-forward, 2 commits). Key review findings: (6.1) Spec reviewer confirmed all 11 handler groups wired correctly with proper middleware ordering. Code quality caught WriteTimeout:15s killing WebSocket connections → fixed to WriteTimeout:0 (WS pumps manage own deadlines). (6.2) Spec reviewer caught single-payment-as-multi and missing price assertion → fixed: split CASH+QRIS with partial payment verification, explicit total_amount==57000.00 assertion. Code quality approved with documentation additions (hub leak note, migration path comment, raw SQL justification). 401 unit tests + 1 integration test.
- **2026-02-07**: Session 10 — First device test. Set up CLI Android build toolchain (Homebrew: openjdk@17, android-commandlinetools, gradle). Build fixes: JAVA_HOME typo in .zshrc (two exports on one line), compileSdk 34→35 (androidx.core 1.15.0 requires SDK 35), README.md files in res/ dirs rejected by resource merger → deleted, API base URL had wrong `/api/v1/` prefix (Go API uses bare paths like `/auth/login`), HTTP blocked by Android cleartext policy → added usesCleartextTraffic=true. Seeded test outlet + user (bcrypt hash via go run). Login tested successfully on real Android device over local WiFi (Mac IP 192.168.1.7:8082). Dev workflow: `./gradlew installDebug` via USB.
- **2026-02-07**: Session 9 — Completed 8.1 (Scaffold Android Project) and 8.2 (Login Screen). Milestone 8 started. Used worktree `feature/milestone-8-android-pos`. (8.1) Full Android project: Gradle version catalogs, Kotlin 2.1.0, Compose BOM 2024.12, KSP+Hilt 2.54, Retrofit+OkHttp NetworkModule, Kiwari brand theme (7 colors, light/dark, M3 typography). Spec review caught missing gradlew + 0-byte font placeholders → fixed (gradlew added, fonts switched to system default). Code quality caught: config cache+Hilt conflict → disabled, debug localhost→10.0.2.2, missing shrinkResources → added, lenient Gson → strict. (8.2) Full auth vertical slice: AuthApi (login/pin-login/refresh), LoginScreen (dual email+PIN modes), TokenRepository (EncryptedSharedPreferences AES256), AuthInterceptor (Bearer header), TokenAuthenticator (auto-refresh on 401 via dual OkHttpClient pattern), NavGraph with auth-state routing. Spec review caught: plain-text DataStore (not encrypted) + missing auto-refresh → both fixed. Code quality caught: AuthRepository using wrong @Named AuthApi → fixed, missing logout→login navigation → added LaunchedEffect, dead isRefreshing field → removed, innerPadding unused → passed through, UserRole enum crash on unknown → UNKNOWN fallback, error snackbar re-fires → clearError(), DRY violation → extracted safeApiCall helper, security-crypto alpha→stable 1.0.0, credentials not cleared after login → cleared. 56 files, 2488 lines added.
- **2026-02-07**: Session 11 — Completed 8.3 (Menu Screen) and 8.4 (Product Customization). Each went through full subagent-driven-development cycle (implement → spec review → fix → code quality review → fix → amend commit). Key findings: (8.3) Spec review caught nullable API field mismatches (preparationTime, maxSelect) and missing NavGraph composable registrations → runtime crash prevention. Code quality caught trailing slashes in MenuApi (potential 404), N+1 sequential variant group fetches → parallelized with async/awaitAll, non-reactive dead properties → removed, duplicated safeApiCall → extracted to ApiHelper.kt, duplicated formatPrice → extracted to CurrencyFormatter.kt with cached NumberFormat. (8.4) Spec review caught critical multi-variant-group bug: CartItem.selectedVariant was singular SelectedVariant?, silently dropping all but first variant group selection → refactored to selectedVariants: List<SelectedVariant> across CartItem, CartRepository, and CustomizationViewModel. Code quality caught broken retry() path (product cleared from SelectedProductRepository before retry), silent empty groups on sub-fetch failure → filtered out, fragile one-shot addedToCart event → reset after handling. 18 new files, 7 modified, ~3400 lines. Worktree 4 commits ahead of main.
- **2026-02-07**: Session 12 — Bold + Clean theme redesign (Task 8.4b). Executed 6-task implementation plan via subagent-driven-development. 3 commits: (1) `9a44467` theme tokens — replaced KiwariColors.kt (9 new colors, removed 10 old: DarkGrey, CreamLight, AccentRed, BorderYellow, SurfaceGrey, Black, LightGrey, MediumGrey, DarkBackground, DarkSurface), KiwariTheme.kt (removed dark theme + dynamic color, added Shapes 8-16dp), KiwariTypography.kt (tightened 57sp→20sp max, all sizes 11-20sp). (2) `5ef9dee` hardcoded colors — CustomizationScreen.kt and ProductListItem.kt replaced PrimaryGreen/PrimaryYellow/White imports with MaterialTheme.colorScheme tokens. Code quality review caught misplaced import + hardcoded RoundedCornerShape(8dp) → fixed to MaterialTheme.shapes.extraSmall. (3) `4583e31` dimensions — CartBottomBar elevation 8→4dp, CategoryChips selected green→yellow + shape=extraSmall, ProductListItem divider indent 72→84dp (matches 56dp avatar). Clean build verified, APK assembled. Worktree 7 commits ahead of main.
- **2026-02-07**: Session 8 — Completed 7.1 (Production Docker Compose) and 7.2 (Backup Script). Milestone 7 now DONE. Used worktree `feature/milestone-7-docker`, merged to main (fast-forward, 2 commits). Key review findings: (7.1) Spec reviewer caught missing `internal: true` on pos-internal network → fixed. Code quality caught: Go version mismatch (golang:1.22 vs go.mod 1.25.7) → updated to golang:1.25-alpine, missing .dockerignore → added, no non-root user → added `app` user in both Dockerfiles, host port exposure → prefixed with 127.0.0.1 (NPM handles public traffic via proxy network), Alpine 3.19 EOL → updated to 3.21. (7.2) Code quality caught silent pg_dump failure in pipe (set -e doesn't catch left side of pipe) → added set -o pipefail, file check -f → -s (non-empty), cron example missing log redirect → added. Backend complete through M7. Next: Android POS (M8, priority).
