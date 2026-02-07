# POS Implementation Progress

Tracking execution of `2026-02-06-pos-implementation-plan.md`.

## Active Branch

`main` (worktree merged and cleaned up)

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

### Milestones 6-10 — NOT STARTED

## Test Count

401 tests passing (3 auth + 303 handler + 28 service + 7 middleware + 8 ws + 52 subtests)

## Resume Prompt

After `/clear`, use:
```
Read PROGRESS.md and docs/plans/2026-02-06-pos-implementation-plan.md, then continue from the next pending task using subagent-driven-development. Working on main branch.
```

## Session Log

- **2026-02-07**: Milestone 3 tasks 3.1–3.4 completed. Each task went through subagent-driven-development: implement → spec review → code quality review → fix → commit. Task 3.5 (Combo Items) pending.
- **2026-02-07**: Session 2 — Completed 3.5 (Combo Items) and 4.1 (Order Creation). Milestone 3 now DONE. Milestone 4 started. Task 4.1 introduced first service layer (`service/order.go`) with transaction handling, price snapshots, discount math, and retry-on-conflict for order numbers. Two review cycles caught: missing service tests (added 28), race condition fix, string-matching error classification replaced with sentinel errors.
- **2026-02-07**: Session 3 — Completed 4.2 (Order Queries & Status) and 4.3 (Order Item Modifications). Each went through full subagent-driven-development cycle. Key review findings fixed: (4.2) TOCTOU race on status transitions → added WHERE status=$current to SQL, completed_at never set → CASE WHEN COMPLETED, inconsistent cancel rules → synced PATCH/DELETE. (4.3) Runtime blocker: updated_at column missing from order_items → removed from SQL, no transaction on AddItem → added TxBeginner to handler, kitchen status on cancelled orders → added status check. 336 tests passing.
- **2026-02-07**: Session 4 — Completed 4.4 (Multi-Payment) and 5.1 (Customer CRUD + Stats). Milestone 4 now DONE. Milestone 5 started. Key review findings fixed: (4.4) Spec review caught TOCTOU race on payment validation → moved GetOrder+SumPayments inside tx with SELECT FOR NO KEY UPDATE row lock. Code quality approved. (5.1) Both reviewers caught missing GET single customer endpoint → added. Code quality reviewer caught stats queries not scoped by outlet_id → added AND o.outlet_id=$2. Also flagged LIKE wildcard injection and soft-delete vs unique constraint — deferred as schema-level concerns. 380 tests passing.
- **2026-02-07**: Session 5 — Merged `feature/milestone-2-auth` worktree into `main` (fast-forward, 15 commits). Worktree and branch cleaned up. Post-merge review of Task 4.4 (Multi-Payment): code quality reviewer caught that COMPLETED orders weren't explicitly blocked from accepting payments (only indirectly via sum check). Fix: added explicit COMPLETED guard + test, also fixed `TestAddPayment_AlreadyFullyPaid` which was silently testing wrong code path after guard addition (changed order status from COMPLETED→NEW). 381 tests passing.
- **2026-02-07**: Session 6 — Completed 5.2 (Reports) and 5.3 (WebSocket). Milestone 5 now DONE. Used worktree `feature/milestone-5-crm-reports-ws`, merged to main (fast-forward, 2 commits). Key review findings fixed: (5.2) Timezone mismatch — `time.Parse` produces UTC but orders are Asia/Jakarta, early morning orders missed → fixed with `time.ParseInLocation("2006-01-02", s, jakartaLoc)`. Untyped sqlc params from `$3 + interval '1 day'` → moved end-of-day to Go. Test package `handler` → `handler_test`. Added date range validation. (5.3) Misleading mutex in broadcast → changed to single write lock covering entire broadcast body. 401 tests passing.
