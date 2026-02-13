# Accounting Phase 4: Sales + Payroll + Ledger — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build sales summary management (POS auto-aggregation + manual entry + posting), payroll entry with posting, and a full ledger view with manual entry — completing the GSheet retirement.

**Architecture:** Sales summaries aggregate POS order data or accept manual entry for non-POS channels. Payroll entries record employee pay. Both "post" to `acct_cash_transactions` so they appear in P&L/Cash Flow reports. The Jurnal page provides a filterable view of all cash transactions plus manual entry for one-off items.

**Tech Stack:** Go (Chi, sqlc, pgx/v5, shopspring/decimal), SvelteKit 2 (Svelte 5, Tailwind CSS 4), PostgreSQL 16.

---

## Conventions Reference

All patterns established in Phase 1. Key files:

| Pattern | Reference File |
|---------|---------------|
| sqlc query with optional filters | `api/queries/acct_cash_transactions.sql` (sqlc.narg pattern) |
| Handler + store interface | `api/internal/accounting/handler/purchase.go` (PurchaseStore) |
| Mock store + httptest | `api/internal/accounting/handler/purchase_test.go` |
| pgtype.Numeric ↔ string | `api/internal/accounting/handler/master.go:numericToStringPtr()` |
| Transaction code generation | `api/internal/accounting/handler/purchase.go:150-159` |
| Admin page server load | `admin/src/routes/(app)/accounting/purchases/+page.server.ts` |
| Admin page with forms | `admin/src/routes/(app)/accounting/purchases/+page.svelte` |
| Router wiring | `api/internal/router/router.go:70-83` |
| TypeScript API types | `admin/src/lib/types/api.ts:290-342` |
| Sidebar nav items | `admin/src/lib/components/Sidebar.svelte:24-27` |
| Money formatting | `admin/src/lib/utils/format.ts:formatRupiah()` |

**Key conventions:**
- `::text AS alias` on COALESCE+aggregate for sqlc to infer `string` (not `interface{}`)
- Consumer-defines-interface: each handler file defines its own store interface
- Money: `shopspring/decimal` for math, `string` in JSON, `pgtype.Numeric` in DB
- Nullable: `pgtype.Text`, `pgtype.UUID`, `pgtype.Numeric` for nullable DB fields
- Errors: 400 validation, 404 `pgx.ErrNoRows`, 409 pgconn `23505`, 500 internal
- Tests: `handler_test` package, mock stores, `httptest`, `chi.NewRouter()`
- Admin: `+page.server.ts` load + actions, `use:enhance`, Svelte 5 `$state()`/`$derived()`
- Helpers `writeJSON`, `uuidToPgUUID`, `numericToStringPtr`, `stringToPgNumeric` already exist in `master.go` / `purchase.go`

---

## Task Overview

| # | Task | Files | Tests |
|---|------|-------|-------|
| 1 | DB Migration — add posted_at to sales summaries | 2 migration files | — |
| 2 | sqlc Queries — Sales, Payroll, POS Aggregation, Transaction Search | 2 new query files + 1 modified, regenerate | — |
| 3 | Sales Handler — Sync POS + Manual CRUD + Post | 2 Go files | 8+ tests |
| 4 | Payroll Handler — CRUD + Post | 2 Go files | 6+ tests |
| 5 | Transaction Handler — Ledger List + Manual Entry | 2 Go files | 5+ tests |
| 6 | Wire Routes | 1 Go file | — |
| 7 | Admin Types + Sidebar | 2 files | — |
| 8 | Admin Page — Penjualan (Sales) | 2 files | — |
| 9 | Admin Page — Gaji (Payroll) | 2 files | — |
| 10 | Admin Page — Jurnal (Ledger) | 2 files | — |
| 11 | Build Verification | — | all tests |

---

## Task 1: DB Migration — Add posted_at to Sales Summaries

**Files:**
- Create: `api/migrations/000004_sales_posted_at.up.sql`
- Create: `api/migrations/000004_sales_posted_at.down.sql`

**Context:** The `acct_sales_daily_summaries` table lacks a `posted_at` column. Payroll already has one. Sales posting (creating cash_transactions from summaries) needs this column to track what's been posted and prevent double-posting.

**Step 1: Write the up migration**

Create `api/migrations/000004_sales_posted_at.up.sql`:

```sql
-- Add posted_at to acct_sales_daily_summaries for sales posting workflow.
-- Matches the pattern in acct_payroll_entries.posted_at.
ALTER TABLE acct_sales_daily_summaries ADD COLUMN posted_at TIMESTAMPTZ;
```

**Step 2: Write the down migration**

Create `api/migrations/000004_sales_posted_at.down.sql`:

```sql
ALTER TABLE acct_sales_daily_summaries DROP COLUMN posted_at;
```

**Step 3: Run migration**

```bash
cd api && export PATH=$PATH:~/go/bin && migrate -path migrations/ -database "$DATABASE_URL" up
```

Expected: `4/up` applied successfully.

**Step 4: Commit**

```bash
git add api/migrations/000004_sales_posted_at.up.sql api/migrations/000004_sales_posted_at.down.sql
git commit -m "feat(accounting): add posted_at to sales daily summaries"
```

---

## Task 2: sqlc Queries — Sales, Payroll, POS Aggregation, Transaction Search

**Files:**
- Create: `api/queries/acct_sales_daily_summaries.sql`
- Create: `api/queries/acct_payroll_entries.sql`
- Modify: `api/queries/acct_cash_transactions.sql` (add text search filter)
- Regenerate: `api/internal/database/` (run `sqlc generate`)

**Step 1: Write sales daily summaries query file**

Create `api/queries/acct_sales_daily_summaries.sql`:

```sql
-- name: ListAcctSalesDailySummaries :many
SELECT * FROM acct_sales_daily_summaries
WHERE
    (sqlc.narg('start_date')::date IS NULL OR sales_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR sales_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('channel')::text IS NULL OR channel = sqlc.narg('channel')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
ORDER BY sales_date DESC, channel, payment_method
LIMIT $1 OFFSET $2;

-- name: GetAcctSalesDailySummary :one
SELECT * FROM acct_sales_daily_summaries WHERE id = $1;

-- name: CreateAcctSalesDailySummary :one
INSERT INTO acct_sales_daily_summaries (
    sales_date, channel, payment_method,
    gross_sales, discount_amount, net_sales,
    cash_account_id, outlet_id, source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateAcctSalesDailySummary :one
UPDATE acct_sales_daily_summaries
SET channel = $2, payment_method = $3,
    gross_sales = $4, discount_amount = $5, net_sales = $6,
    cash_account_id = $7
WHERE id = $1 AND source = 'manual' AND posted_at IS NULL
RETURNING *;

-- name: DeleteAcctSalesDailySummary :exec
DELETE FROM acct_sales_daily_summaries
WHERE id = $1 AND source = 'manual' AND posted_at IS NULL;

-- name: UpsertAcctSalesDailySummary :one
-- Used by POS sync — upserts by the UNIQUE(sales_date, channel, payment_method, outlet_id) constraint.
-- Only updates if not yet posted.
INSERT INTO acct_sales_daily_summaries (
    sales_date, channel, payment_method,
    gross_sales, discount_amount, net_sales,
    cash_account_id, outlet_id, source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pos')
ON CONFLICT (sales_date, channel, payment_method, outlet_id)
DO UPDATE SET
    gross_sales = EXCLUDED.gross_sales,
    discount_amount = EXCLUDED.discount_amount,
    net_sales = EXCLUDED.net_sales,
    cash_account_id = EXCLUDED.cash_account_id
WHERE acct_sales_daily_summaries.posted_at IS NULL
RETURNING *;

-- name: MarkSalesSummariesPosted :exec
-- Batch-mark summaries as posted for a given date + outlet.
UPDATE acct_sales_daily_summaries
SET posted_at = now()
WHERE sales_date = $1
    AND (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
    AND posted_at IS NULL;

-- name: ListUnpostedSalesSummaries :many
-- Get unposted summaries for a specific date + optional outlet for posting.
SELECT * FROM acct_sales_daily_summaries
WHERE sales_date = $1
    AND (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
    AND posted_at IS NULL
ORDER BY channel, payment_method;

-- name: AggregatePOSSales :many
-- Cross-domain query: aggregates completed POS orders by date, order_type, payment_method.
-- Handler maps order_type → channel name and payment_method → display name.
SELECT
    o.completed_at::date AS sales_date,
    o.order_type,
    p.payment_method,
    SUM(p.amount)::text AS total_amount
FROM orders o
JOIN payments p ON p.order_id = o.id
WHERE o.status = 'COMPLETED'
    AND p.status = 'COMPLETED'
    AND o.outlet_id = $1
    AND o.completed_at::date >= $2::date
    AND o.completed_at::date <= $3::date
GROUP BY o.completed_at::date, o.order_type, p.payment_method
ORDER BY 1, 2, 3;
```

**Step 2: Write payroll entries query file**

Create `api/queries/acct_payroll_entries.sql`:

```sql
-- name: ListAcctPayrollEntries :many
SELECT * FROM acct_payroll_entries
WHERE
    (sqlc.narg('start_date')::date IS NULL OR payroll_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR payroll_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id')) AND
    (sqlc.narg('period_type')::text IS NULL OR period_type = sqlc.narg('period_type'))
ORDER BY payroll_date DESC, created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAcctPayrollEntry :one
SELECT * FROM acct_payroll_entries WHERE id = $1;

-- name: CreateAcctPayrollEntry :one
INSERT INTO acct_payroll_entries (
    payroll_date, period_type, period_ref, employee_name,
    gross_pay, payment_method, cash_account_id, outlet_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateAcctPayrollEntry :one
UPDATE acct_payroll_entries
SET payroll_date = $2, period_type = $3, period_ref = $4,
    employee_name = $5, gross_pay = $6, payment_method = $7,
    cash_account_id = $8, outlet_id = $9
WHERE id = $1 AND posted_at IS NULL
RETURNING *;

-- name: DeleteAcctPayrollEntry :exec
DELETE FROM acct_payroll_entries WHERE id = $1 AND posted_at IS NULL;

-- name: ListUnpostedPayrollEntries :many
SELECT * FROM acct_payroll_entries
WHERE id = ANY($1::uuid[])
    AND posted_at IS NULL
ORDER BY employee_name;

-- name: MarkPayrollEntriesPosted :exec
UPDATE acct_payroll_entries
SET posted_at = now()
WHERE id = ANY($1::uuid[]) AND posted_at IS NULL;
```

**Step 3: Add text search to cash transactions query**

Modify `api/queries/acct_cash_transactions.sql` — add a `search` filter to `ListAcctCashTransactions`:

Replace the existing query (lines 1-11) with:

```sql
-- name: ListAcctCashTransactions :many
SELECT * FROM acct_cash_transactions
WHERE
    (sqlc.narg('start_date')::date IS NULL OR transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('line_type')::text IS NULL OR line_type = sqlc.narg('line_type')) AND
    (sqlc.narg('account_id')::uuid IS NULL OR account_id = sqlc.narg('account_id')) AND
    (sqlc.narg('cash_account_id')::uuid IS NULL OR cash_account_id = sqlc.narg('cash_account_id')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id')) AND
    (sqlc.narg('search')::text IS NULL OR description ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY transaction_date DESC, created_at DESC
LIMIT $1 OFFSET $2;
```

**Step 4: Regenerate sqlc**

```bash
cd api && export PATH=$PATH:~/go/bin && sqlc generate
```

Expected: generates new functions in `api/internal/database/` for all new queries.

**Step 5: Verify generated code compiles**

```bash
cd api && go build ./...
```

Expected: clean build.

**Step 6: Commit**

```bash
git add api/queries/acct_sales_daily_summaries.sql api/queries/acct_payroll_entries.sql api/queries/acct_cash_transactions.sql api/internal/database/
git commit -m "feat(accounting): add sqlc queries for sales, payroll, POS aggregation, transaction search"
```

---

## Task 3: Sales Handler — Sync POS + Manual CRUD + Post

**Files:**
- Create: `api/internal/accounting/handler/sales.go`
- Create: `api/internal/accounting/handler/sales_test.go`

**Context:** The sales handler manages three flows:
1. **POS Sync**: Aggregates completed POS orders into `acct_sales_daily_summaries` (source='pos')
2. **Manual CRUD**: Create/update/delete summaries for non-POS channels (source='manual')
3. **Post**: Creates `acct_cash_transactions` (line_type='SALES') from unposted summaries

The POS order_type values map to accounting channel names:
- `DINE_IN` → `"Dine In"`, `TAKEAWAY` → `"Take Away"`, `CATERING` → `"Catering"`, `DELIVERY` → `"Delivery"`

**Step 1: Write the sales handler**

Create `api/internal/accounting/handler/sales.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

type SalesStore interface {
	ListAcctSalesDailySummaries(ctx context.Context, arg database.ListAcctSalesDailySummariesParams) ([]database.AcctSalesDailySummary, error)
	GetAcctSalesDailySummary(ctx context.Context, id uuid.UUID) (database.AcctSalesDailySummary, error)
	CreateAcctSalesDailySummary(ctx context.Context, arg database.CreateAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error)
	UpdateAcctSalesDailySummary(ctx context.Context, arg database.UpdateAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error)
	DeleteAcctSalesDailySummary(ctx context.Context, arg database.DeleteAcctSalesDailySummaryParams) error
	UpsertAcctSalesDailySummary(ctx context.Context, arg database.UpsertAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error)
	ListUnpostedSalesSummaries(ctx context.Context, arg database.ListUnpostedSalesSummariesParams) ([]database.AcctSalesDailySummary, error)
	MarkSalesSummariesPosted(ctx context.Context, arg database.MarkSalesSummariesPostedParams) error
	AggregatePOSSales(ctx context.Context, arg database.AggregatePOSSalesParams) ([]database.AggregatePOSSalesRow, error)
	// For posting to cash_transactions:
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
}

// --- SalesHandler ---

type SalesHandler struct {
	store SalesStore
}

func NewSalesHandler(store SalesStore) *SalesHandler {
	return &SalesHandler{store: store}
}

func (h *SalesHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListSalesSummaries)
	r.Post("/", h.CreateSalesSummary)
	r.Put("/{id}", h.UpdateSalesSummary)
	r.Delete("/{id}", h.DeleteSalesSummary)
	r.Post("/sync-pos", h.SyncPOS)
	r.Post("/post", h.PostSales)
}

// --- Request / Response types ---

// Channel mapping: POS order_type → accounting channel name
var orderTypeToChannel = map[string]string{
	"DINE_IN":  "Dine In",
	"TAKEAWAY": "Take Away",
	"CATERING": "Catering",
	"DELIVERY": "Delivery",
}

type salesSummaryResponse struct {
	ID            uuid.UUID  `json:"id"`
	SalesDate     string     `json:"sales_date"`
	Channel       string     `json:"channel"`
	PaymentMethod string     `json:"payment_method"`
	GrossSales    string     `json:"gross_sales"`
	DiscountAmount string   `json:"discount_amount"`
	NetSales      string     `json:"net_sales"`
	CashAccountID string     `json:"cash_account_id"`
	OutletID      *string    `json:"outlet_id"`
	Source        string     `json:"source"`
	PostedAt      *time.Time `json:"posted_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type createSalesSummaryRequest struct {
	SalesDate     string  `json:"sales_date"`
	Channel       string  `json:"channel"`
	PaymentMethod string  `json:"payment_method"`
	GrossSales    string  `json:"gross_sales"`
	DiscountAmount string `json:"discount_amount"`
	NetSales      string  `json:"net_sales"`
	CashAccountID string  `json:"cash_account_id"`
	OutletID      *string `json:"outlet_id"`
}

type updateSalesSummaryRequest struct {
	Channel       string `json:"channel"`
	PaymentMethod string `json:"payment_method"`
	GrossSales    string `json:"gross_sales"`
	DiscountAmount string `json:"discount_amount"`
	NetSales      string `json:"net_sales"`
	CashAccountID string `json:"cash_account_id"`
}

type syncPOSRequest struct {
	StartDate             string            `json:"start_date"`
	EndDate               string            `json:"end_date"`
	OutletID              string            `json:"outlet_id"`
	PaymentMethodAccounts map[string]string `json:"payment_method_accounts"` // e.g. {"CASH":"uuid","QRIS":"uuid"}
}

type syncPOSResponse struct {
	SyncedCount int                    `json:"synced_count"`
	Summaries   []salesSummaryResponse `json:"summaries"`
}

type postSalesRequest struct {
	SalesDate string  `json:"sales_date"`
	OutletID  *string `json:"outlet_id"`
	AccountID string  `json:"account_id"` // Sales Revenue account UUID
}

type postSalesResponse struct {
	PostedCount      int `json:"posted_count"`
	TransactionsCreated int `json:"transactions_created"`
}

// --- Handlers ---

func (h *SalesHandler) ListSalesSummaries(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	params := database.ListAcctSalesDailySummariesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	if v := r.URL.Query().Get("start_date"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date"})
			return
		}
		params.StartDate = pgtype.Date{Time: d, Valid: true}
	}
	if v := r.URL.Query().Get("end_date"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date"})
			return
		}
		params.EndDate = pgtype.Date{Time: d, Valid: true}
	}
	if v := r.URL.Query().Get("channel"); v != "" {
		params.Channel = pgtype.Text{String: v, Valid: true}
	}
	if v := r.URL.Query().Get("outlet_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		params.OutletID = uuidToPgUUID(id)
	}

	rows, err := h.store.ListAcctSalesDailySummaries(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list sales summaries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	result := make([]salesSummaryResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, toSalesSummaryResponse(row))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *SalesHandler) CreateSalesSummary(w http.ResponseWriter, r *http.Request) {
	var req createSalesSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.SalesDate == "" || req.Channel == "" || req.PaymentMethod == "" ||
		req.GrossSales == "" || req.NetSales == "" || req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sales_date, channel, payment_method, gross_sales, net_sales, cash_account_id are required"})
		return
	}

	date, err := time.Parse("2006-01-02", req.SalesDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sales_date format, expected YYYY-MM-DD"})
		return
	}

	cashAcctID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	grossSales, discountAmt, netSales, err := parseSalesAmounts(req.GrossSales, req.DiscountAmount, req.NetSales)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(id)
	}

	row, err := h.store.CreateAcctSalesDailySummary(r.Context(), database.CreateAcctSalesDailySummaryParams{
		SalesDate:      pgtype.Date{Time: date, Valid: true},
		Channel:        req.Channel,
		PaymentMethod:  req.PaymentMethod,
		GrossSales:     grossSales,
		DiscountAmount: discountAmt,
		NetSales:       netSales,
		CashAccountID:  cashAcctID,
		OutletID:       outletID,
		Source:         "manual",
	})
	if err != nil {
		if isPgUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "sales summary already exists for this date/channel/payment_method/outlet combination"})
			return
		}
		log.Printf("ERROR: create sales summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toSalesSummaryResponse(row))
}

func (h *SalesHandler) UpdateSalesSummary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req updateSalesSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Channel == "" || req.PaymentMethod == "" ||
		req.GrossSales == "" || req.NetSales == "" || req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel, payment_method, gross_sales, net_sales, cash_account_id are required"})
		return
	}

	cashAcctID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	grossSales, discountAmt, netSales, err := parseSalesAmounts(req.GrossSales, req.DiscountAmount, req.NetSales)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	row, err := h.store.UpdateAcctSalesDailySummary(r.Context(), database.UpdateAcctSalesDailySummaryParams{
		ID:             id,
		Channel:        req.Channel,
		PaymentMethod:  req.PaymentMethod,
		GrossSales:     grossSales,
		DiscountAmount: discountAmt,
		NetSales:       netSales,
		CashAccountID:  cashAcctID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "summary not found, not manual, or already posted"})
			return
		}
		log.Printf("ERROR: update sales summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toSalesSummaryResponse(row))
}

func (h *SalesHandler) DeleteSalesSummary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	err = h.store.DeleteAcctSalesDailySummary(r.Context(), database.DeleteAcctSalesDailySummaryParams{
		ID: id,
	})
	if err != nil {
		log.Printf("ERROR: delete sales summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SalesHandler) SyncPOS(w http.ResponseWriter, r *http.Request) {
	var req syncPOSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.StartDate == "" || req.EndDate == "" || req.OutletID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start_date, end_date, outlet_id are required"})
		return
	}
	if len(req.PaymentMethodAccounts) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_method_accounts mapping is required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date"})
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date"})
		return
	}
	outletID, err := uuid.Parse(req.OutletID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
		return
	}

	// Validate payment method account UUIDs
	cashAccountMap := make(map[string]uuid.UUID, len(req.PaymentMethodAccounts))
	for method, acctIDStr := range req.PaymentMethodAccounts {
		acctID, err := uuid.Parse(acctIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid cash_account_id for payment method %s", method)})
			return
		}
		cashAccountMap[method] = acctID
	}

	// Aggregate POS orders
	rows, err := h.store.AggregatePOSSales(r.Context(), database.AggregatePOSSalesParams{
		OutletID:  outletID,
		Column2:   pgtype.Date{Time: startDate, Valid: true},
		Column3:   pgtype.Date{Time: endDate, Valid: true},
	})
	if err != nil {
		log.Printf("ERROR: aggregate POS sales: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Upsert each aggregated row into sales_daily_summaries
	var summaries []salesSummaryResponse
	for _, row := range rows {
		channel, ok := orderTypeToChannel[row.OrderType]
		if !ok {
			channel = row.OrderType // fallback: use raw value
		}

		cashAcctID, ok := cashAccountMap[row.PaymentMethod]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("no cash_account_id mapping for payment method %s", row.PaymentMethod),
			})
			return
		}

		totalAmt, err := decimal.NewFromString(row.TotalAmount)
		if err != nil {
			log.Printf("ERROR: parse total_amount: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		var grossPg, discountPg, netPg pgtype.Numeric
		amtStr := totalAmt.StringFixed(2)
		grossPg.Scan(amtStr)
		discountPg.Scan("0.00")
		netPg.Scan(amtStr)

		summary, err := h.store.UpsertAcctSalesDailySummary(r.Context(), database.UpsertAcctSalesDailySummaryParams{
			SalesDate:      pgtype.Date{Time: row.SalesDate.Time, Valid: true},
			Channel:        channel,
			PaymentMethod:  row.PaymentMethod,
			GrossSales:     grossPg,
			DiscountAmount: discountPg,
			NetSales:       netPg,
			CashAccountID:  cashAcctID,
			OutletID:       uuidToPgUUID(outletID),
		})
		if err != nil {
			log.Printf("ERROR: upsert sales summary: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		summaries = append(summaries, toSalesSummaryResponse(summary))
	}

	writeJSON(w, http.StatusOK, syncPOSResponse{
		SyncedCount: len(summaries),
		Summaries:   summaries,
	})
}

func (h *SalesHandler) PostSales(w http.ResponseWriter, r *http.Request) {
	var req postSalesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.SalesDate == "" || req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sales_date and account_id are required"})
		return
	}

	date, err := time.Parse("2006-01-02", req.SalesDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sales_date"})
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	var outletFilter pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletFilter = uuidToPgUUID(id)
	}

	// Get unposted summaries for this date
	summaries, err := h.store.ListUnpostedSalesSummaries(r.Context(), database.ListUnpostedSalesSummariesParams{
		SalesDate: pgtype.Date{Time: date, Valid: true},
		OutletID:  outletFilter,
	})
	if err != nil {
		log.Printf("ERROR: list unposted sales: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if len(summaries) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no unposted sales summaries found for this date"})
		return
	}

	// Get next transaction code
	maxCode, err := h.store.GetNextTransactionCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum, err := parseTransactionCodeNum(maxCode)
	if err != nil {
		log.Printf("ERROR: parse transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Create cash_transactions for each summary
	txCount := 0
	for _, s := range summaries {
		transactionCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++

		desc := fmt.Sprintf("Penjualan %s %s %s", s.Channel, s.PaymentMethod, req.SalesDate)

		var onePg pgtype.Numeric
		onePg.Scan("1.00")

		_, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      transactionCode,
			TransactionDate:      s.SalesDate,
			ItemID:               pgtype.UUID{},
			Description:          desc,
			Quantity:             onePg,
			UnitPrice:            s.NetSales,
			Amount:               s.NetSales,
			LineType:             "SALES",
			AccountID:            accountID,
			CashAccountID:        uuidToPgUUID(s.CashAccountID),
			OutletID:             s.OutletID,
			ReimbursementBatchID: pgtype.Text{},
		})
		if err != nil {
			log.Printf("ERROR: create sales cash transaction: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		txCount++
	}

	// Mark summaries as posted
	err = h.store.MarkSalesSummariesPosted(r.Context(), database.MarkSalesSummariesPostedParams{
		SalesDate: pgtype.Date{Time: date, Valid: true},
		OutletID:  outletFilter,
	})
	if err != nil {
		log.Printf("ERROR: mark sales posted: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, postSalesResponse{
		PostedCount:         len(summaries),
		TransactionsCreated: txCount,
	})
}

// --- Helper functions ---

func toSalesSummaryResponse(row database.AcctSalesDailySummary) salesSummaryResponse {
	resp := salesSummaryResponse{
		ID:             row.ID,
		SalesDate:      row.SalesDate.Time.Format("2006-01-02"),
		Channel:        row.Channel,
		PaymentMethod:  row.PaymentMethod,
		GrossSales:     numericToString(row.GrossSales),
		DiscountAmount: numericToString(row.DiscountAmount),
		NetSales:       numericToString(row.NetSales),
		CashAccountID:  row.CashAccountID.String(),
		Source:         row.Source,
		CreatedAt:      row.CreatedAt,
	}
	if row.OutletID.Valid {
		idStr := uuid.UUID(row.OutletID.Bytes).String()
		resp.OutletID = &idStr
	}
	if row.PostedAt.Valid {
		t := row.PostedAt.Time
		resp.PostedAt = &t
	}
	return resp
}

func parseSalesAmounts(grossStr, discountStr, netStr string) (pgtype.Numeric, pgtype.Numeric, pgtype.Numeric, error) {
	var grossPg, discountPg, netPg pgtype.Numeric

	gross, err := decimal.NewFromString(grossStr)
	if err != nil {
		return grossPg, discountPg, netPg, fmt.Errorf("invalid gross_sales format")
	}
	grossPg.Scan(gross.StringFixed(2))

	if discountStr == "" {
		discountPg.Scan("0.00")
	} else {
		disc, err := decimal.NewFromString(discountStr)
		if err != nil {
			return grossPg, discountPg, netPg, fmt.Errorf("invalid discount_amount format")
		}
		discountPg.Scan(disc.StringFixed(2))
	}

	net, err := decimal.NewFromString(netStr)
	if err != nil {
		return grossPg, discountPg, netPg, fmt.Errorf("invalid net_sales format")
	}
	netPg.Scan(net.StringFixed(2))

	return grossPg, discountPg, netPg, nil
}

func parseTransactionCodeNum(maxCode string) (int, error) {
	if len(maxCode) < 4 {
		return 1, nil
	}
	var num int
	_, err := fmt.Sscanf(maxCode[3:], "%d", &num)
	if err != nil {
		return 0, err
	}
	return num + 1, nil
}

// numericToString converts pgtype.Numeric to string with 2 decimal places.
// Returns "0.00" if invalid.
func numericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0.00"
	}
	val, err := n.Value()
	if err != nil || val == nil {
		return "0.00"
	}
	d, err := decimal.NewFromString(val.(string))
	if err != nil {
		return "0.00"
	}
	return d.StringFixed(2)
}

func parsePagination(r *http.Request) (int, int) {
	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func isPgUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if ok := errors.As(err, &pgErr); ok && pgErr.SQLState() == "23505" {
		return true
	}
	return false
}
```

**Important:** The `isPgUniqueViolation` helper needs `"errors"` in imports. Also check if `parsePagination`, `numericToString`, or `parseTransactionCodeNum` might already exist in other handler files. If `numericToString` already exists in `master.go` as a different signature (`numericToStringPtr`), add this non-pointer variant. If `parsePagination` already exists, reuse it. If not, add it. Check `master.go` and `purchase.go` for existing helpers and reuse or extract shared ones. Do NOT duplicate helpers — if they exist in other files in the same package, just call them directly.

**Step 2: Write sales handler tests**

Create `api/internal/accounting/handler/sales_test.go`:

Tests to write (use the same mock store pattern as `purchase_test.go`):

1. `TestListSalesSummaries` — returns list, tests date filter params
2. `TestCreateSalesSummary` — creates manual entry, validates required fields
3. `TestCreateSalesSummary_MissingFields` — returns 400 for missing required fields
4. `TestCreateSalesSummary_DuplicateConflict` — returns 409 on unique violation
5. `TestUpdateSalesSummary` — updates manual entry, returns updated row
6. `TestUpdateSalesSummary_PostedOrPOS` — returns 404 when trying to update posted/POS entry
7. `TestDeleteSalesSummary` — returns 204
8. `TestSyncPOS` — aggregates POS data and upserts summaries
9. `TestSyncPOS_MissingPaymentMethodMapping` — returns 400 when mapping is incomplete
10. `TestPostSales` — creates cash_transactions from unposted summaries, marks posted
11. `TestPostSales_NoneUnposted` — returns 400 when no unposted summaries found

The mock store should be `mockSalesStore` implementing `SalesStore`. Each test creates the mock, sets up test data, calls the handler via httptest, and asserts response code + body.

**Step 3: Run tests**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestSales
```

Expected: all tests pass.

**Step 4: Commit**

```bash
git add api/internal/accounting/handler/sales.go api/internal/accounting/handler/sales_test.go
git commit -m "feat(accounting): add sales handler — sync POS, manual CRUD, posting"
```

---

## Task 4: Payroll Handler — CRUD + Post

**Files:**
- Create: `api/internal/accounting/handler/payroll.go`
- Create: `api/internal/accounting/handler/payroll_test.go`

**Context:** Payroll is simpler than sales. The owner enters employee pay data, then posts to create cash_transactions (DR Payroll Expense, CR Cash). The `acct_payroll_entries` table already has `posted_at`.

**Step 1: Write the payroll handler**

Create `api/internal/accounting/handler/payroll.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

type PayrollStore interface {
	ListAcctPayrollEntries(ctx context.Context, arg database.ListAcctPayrollEntriesParams) ([]database.AcctPayrollEntry, error)
	GetAcctPayrollEntry(ctx context.Context, id uuid.UUID) (database.AcctPayrollEntry, error)
	CreateAcctPayrollEntry(ctx context.Context, arg database.CreateAcctPayrollEntryParams) (database.AcctPayrollEntry, error)
	UpdateAcctPayrollEntry(ctx context.Context, arg database.UpdateAcctPayrollEntryParams) (database.AcctPayrollEntry, error)
	DeleteAcctPayrollEntry(ctx context.Context, arg database.DeleteAcctPayrollEntryParams) error
	ListUnpostedPayrollEntries(ctx context.Context, ids []uuid.UUID) ([]database.AcctPayrollEntry, error)
	MarkPayrollEntriesPosted(ctx context.Context, ids []uuid.UUID) error
	// For posting:
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
}

// --- PayrollHandler ---

type PayrollHandler struct {
	store PayrollStore
}

func NewPayrollHandler(store PayrollStore) *PayrollHandler {
	return &PayrollHandler{store: store}
}

func (h *PayrollHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListPayrollEntries)
	r.Post("/", h.CreatePayrollBatch)
	r.Put("/{id}", h.UpdatePayrollEntry)
	r.Delete("/{id}", h.DeletePayrollEntry)
	r.Post("/post", h.PostPayroll)
}

// --- Request / Response types ---

var validPeriodTypes = map[string]bool{
	"Daily":   true,
	"Weekly":  true,
	"Monthly": true,
}

type payrollEntryResponse struct {
	ID            uuid.UUID  `json:"id"`
	PayrollDate   string     `json:"payroll_date"`
	PeriodType    string     `json:"period_type"`
	PeriodRef     *string    `json:"period_ref"`
	EmployeeName  string     `json:"employee_name"`
	GrossPay      string     `json:"gross_pay"`
	PaymentMethod string     `json:"payment_method"`
	CashAccountID string     `json:"cash_account_id"`
	OutletID      *string    `json:"outlet_id"`
	PostedAt      *time.Time `json:"posted_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type createPayrollBatchRequest struct {
	PayrollDate   string                  `json:"payroll_date"`
	PeriodType    string                  `json:"period_type"`
	PeriodRef     *string                 `json:"period_ref"`
	CashAccountID string                  `json:"cash_account_id"`
	OutletID      *string                 `json:"outlet_id"`
	Employees     []payrollEmployeeEntry  `json:"employees"`
}

type payrollEmployeeEntry struct {
	EmployeeName  string `json:"employee_name"`
	GrossPay      string `json:"gross_pay"`
	PaymentMethod string `json:"payment_method"`
}

type updatePayrollEntryRequest struct {
	PayrollDate   string  `json:"payroll_date"`
	PeriodType    string  `json:"period_type"`
	PeriodRef     *string `json:"period_ref"`
	EmployeeName  string  `json:"employee_name"`
	GrossPay      string  `json:"gross_pay"`
	PaymentMethod string  `json:"payment_method"`
	CashAccountID string  `json:"cash_account_id"`
	OutletID      *string `json:"outlet_id"`
}

type postPayrollRequest struct {
	IDs       []string `json:"ids"`
	AccountID string   `json:"account_id"` // Payroll Expense account UUID
}

type postPayrollResponse struct {
	PostedCount         int `json:"posted_count"`
	TransactionsCreated int `json:"transactions_created"`
}

// --- Handlers ---

func (h *PayrollHandler) ListPayrollEntries(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	params := database.ListAcctPayrollEntriesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	if v := r.URL.Query().Get("start_date"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date"})
			return
		}
		params.StartDate = pgtype.Date{Time: d, Valid: true}
	}
	if v := r.URL.Query().Get("end_date"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date"})
			return
		}
		params.EndDate = pgtype.Date{Time: d, Valid: true}
	}
	if v := r.URL.Query().Get("outlet_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		params.OutletID = uuidToPgUUID(id)
	}
	if v := r.URL.Query().Get("period_type"); v != "" {
		params.PeriodType = pgtype.Text{String: v, Valid: true}
	}

	rows, err := h.store.ListAcctPayrollEntries(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list payroll entries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	result := make([]payrollEntryResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, toPayrollEntryResponse(row))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *PayrollHandler) CreatePayrollBatch(w http.ResponseWriter, r *http.Request) {
	var req createPayrollBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.PayrollDate == "" || req.PeriodType == "" || req.CashAccountID == "" || len(req.Employees) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payroll_date, period_type, cash_account_id, and employees are required"})
		return
	}

	if !validPeriodTypes[req.PeriodType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_type must be Daily, Weekly, or Monthly"})
		return
	}

	date, err := time.Parse("2006-01-02", req.PayrollDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payroll_date format, expected YYYY-MM-DD"})
		return
	}

	cashAcctID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(id)
	}

	var periodRef pgtype.Text
	if req.PeriodRef != nil && *req.PeriodRef != "" {
		periodRef = pgtype.Text{String: *req.PeriodRef, Valid: true}
	}

	var entries []payrollEntryResponse
	for _, emp := range req.Employees {
		if emp.EmployeeName == "" || emp.GrossPay == "" || emp.PaymentMethod == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "each employee needs employee_name, gross_pay, and payment_method"})
			return
		}

		pay, err := decimal.NewFromString(emp.GrossPay)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid gross_pay for %s", emp.EmployeeName)})
			return
		}

		var payPg pgtype.Numeric
		payPg.Scan(pay.StringFixed(2))

		row, err := h.store.CreateAcctPayrollEntry(r.Context(), database.CreateAcctPayrollEntryParams{
			PayrollDate:   pgtype.Date{Time: date, Valid: true},
			PeriodType:    req.PeriodType,
			PeriodRef:     periodRef,
			EmployeeName:  emp.EmployeeName,
			GrossPay:      payPg,
			PaymentMethod: emp.PaymentMethod,
			CashAccountID: cashAcctID,
			OutletID:      outletID,
		})
		if err != nil {
			log.Printf("ERROR: create payroll entry: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		entries = append(entries, toPayrollEntryResponse(row))
	}

	writeJSON(w, http.StatusCreated, entries)
}

func (h *PayrollHandler) UpdatePayrollEntry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req updatePayrollEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.PayrollDate == "" || req.PeriodType == "" || req.EmployeeName == "" ||
		req.GrossPay == "" || req.PaymentMethod == "" || req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "all fields are required"})
		return
	}

	if !validPeriodTypes[req.PeriodType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_type must be Daily, Weekly, or Monthly"})
		return
	}

	date, err := time.Parse("2006-01-02", req.PayrollDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payroll_date"})
		return
	}

	cashAcctID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	pay, err := decimal.NewFromString(req.GrossPay)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid gross_pay"})
		return
	}

	var payPg pgtype.Numeric
	payPg.Scan(pay.StringFixed(2))

	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		oid, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(oid)
	}

	var periodRef pgtype.Text
	if req.PeriodRef != nil && *req.PeriodRef != "" {
		periodRef = pgtype.Text{String: *req.PeriodRef, Valid: true}
	}

	row, err := h.store.UpdateAcctPayrollEntry(r.Context(), database.UpdateAcctPayrollEntryParams{
		ID:            id,
		PayrollDate:   pgtype.Date{Time: date, Valid: true},
		PeriodType:    req.PeriodType,
		PeriodRef:     periodRef,
		EmployeeName:  req.EmployeeName,
		GrossPay:      payPg,
		PaymentMethod: req.PaymentMethod,
		CashAccountID: cashAcctID,
		OutletID:      outletID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found or already posted"})
			return
		}
		log.Printf("ERROR: update payroll entry: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toPayrollEntryResponse(row))
}

func (h *PayrollHandler) DeletePayrollEntry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	err = h.store.DeleteAcctPayrollEntry(r.Context(), database.DeleteAcctPayrollEntryParams{ID: id})
	if err != nil {
		log.Printf("ERROR: delete payroll entry: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PayrollHandler) PostPayroll(w http.ResponseWriter, r *http.Request) {
	var req postPayrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.IDs) == 0 || req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids and account_id are required"})
		return
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	// Parse UUIDs
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid id: %s", idStr)})
			return
		}
		ids = append(ids, id)
	}

	// Get unposted entries
	entries, err := h.store.ListUnpostedPayrollEntries(r.Context(), ids)
	if err != nil {
		log.Printf("ERROR: list unposted payroll: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if len(entries) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no unposted payroll entries found"})
		return
	}

	// Get next transaction code
	maxCode, err := h.store.GetNextTransactionCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum, err := parseTransactionCodeNum(maxCode)
	if err != nil {
		log.Printf("ERROR: parse transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Create cash_transactions for each entry
	txCount := 0
	for _, e := range entries {
		transactionCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++

		periodRef := ""
		if e.PeriodRef.Valid {
			periodRef = " " + e.PeriodRef.String
		}
		desc := fmt.Sprintf("Gaji %s%s", e.EmployeeName, periodRef)

		var onePg pgtype.Numeric
		onePg.Scan("1.00")

		_, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      transactionCode,
			TransactionDate:      e.PayrollDate,
			ItemID:               pgtype.UUID{},
			Description:          desc,
			Quantity:             onePg,
			UnitPrice:            e.GrossPay,
			Amount:               e.GrossPay,
			LineType:             "EXPENSE",
			AccountID:            accountID,
			CashAccountID:        uuidToPgUUID(e.CashAccountID),
			OutletID:             e.OutletID,
			ReimbursementBatchID: pgtype.Text{},
		})
		if err != nil {
			log.Printf("ERROR: create payroll cash transaction: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		txCount++
	}

	// Mark entries as posted
	err = h.store.MarkPayrollEntriesPosted(r.Context(), ids)
	if err != nil {
		log.Printf("ERROR: mark payroll posted: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, postPayrollResponse{
		PostedCount:         len(entries),
		TransactionsCreated: txCount,
	})
}

// --- Helper functions ---

func toPayrollEntryResponse(row database.AcctPayrollEntry) payrollEntryResponse {
	resp := payrollEntryResponse{
		ID:            row.ID,
		PayrollDate:   row.PayrollDate.Time.Format("2006-01-02"),
		PeriodType:    row.PeriodType,
		EmployeeName:  row.EmployeeName,
		GrossPay:      numericToString(row.GrossPay),
		PaymentMethod: row.PaymentMethod,
		CashAccountID: row.CashAccountID.String(),
		CreatedAt:     row.CreatedAt,
	}
	if row.PeriodRef.Valid {
		resp.PeriodRef = &row.PeriodRef.String
	}
	if row.OutletID.Valid {
		idStr := uuid.UUID(row.OutletID.Bytes).String()
		resp.OutletID = &idStr
	}
	if row.PostedAt.Valid {
		t := row.PostedAt.Time
		resp.PostedAt = &t
	}
	return resp
}
```

**Step 2: Write payroll handler tests**

Create `api/internal/accounting/handler/payroll_test.go`:

Tests to write:
1. `TestListPayrollEntries` — returns list with filter params
2. `TestCreatePayrollBatch` — creates multiple employee entries in one call
3. `TestCreatePayrollBatch_MissingFields` — 400 for missing required fields
4. `TestCreatePayrollBatch_InvalidPeriodType` — 400 for bad period_type
5. `TestUpdatePayrollEntry` — updates unposted entry
6. `TestUpdatePayrollEntry_AlreadyPosted` — 404 when posted_at is set
7. `TestDeletePayrollEntry` — returns 204
8. `TestPostPayroll` — creates cash_transactions and marks posted
9. `TestPostPayroll_NoneUnposted` — 400 when all already posted

**Step 3: Run tests**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestPayroll
```

Expected: all tests pass.

**Step 4: Commit**

```bash
git add api/internal/accounting/handler/payroll.go api/internal/accounting/handler/payroll_test.go
git commit -m "feat(accounting): add payroll handler — batch create, update, delete, posting"
```

---

## Task 5: Transaction Handler — Ledger List + Manual Entry

**Files:**
- Create: `api/internal/accounting/handler/transaction.go`
- Create: `api/internal/accounting/handler/transaction_test.go`

**Context:** The Jurnal (ledger) page needs:
1. List all cash_transactions with all 7 filters (date range, line_type, account, cash_account, outlet, text search) — uses the existing `ListAcctCashTransactions` query (now with search filter added in Task 2)
2. Manual entry — creates a generic cash_transaction for one-off items (bank transfers, owner drawings, equipment purchases)

This handler is thinner than sales/payroll because the query already exists and the create logic is similar to the purchase handler but with user-specified line_type and account.

**Step 1: Write the transaction handler**

Create `api/internal/accounting/handler/transaction.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

type TransactionStore interface {
	ListAcctCashTransactions(ctx context.Context, arg database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error)
	GetAcctCashTransaction(ctx context.Context, id uuid.UUID) (database.AcctCashTransaction, error)
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
}

// --- TransactionHandler ---

type TransactionHandler struct {
	store TransactionStore
}

func NewTransactionHandler(store TransactionStore) *TransactionHandler {
	return &TransactionHandler{store: store}
}

func (h *TransactionHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListTransactions)
	r.Get("/{id}", h.GetTransaction)
	r.Post("/", h.CreateTransaction)
}

// --- Request / Response types ---

var validLineTypes = map[string]bool{
	"ASSET":     true,
	"INVENTORY": true,
	"EXPENSE":   true,
	"SALES":     true,
	"COGS":      true,
	"LIABILITY": true,
	"CAPITAL":   true,
	"DRAWING":   true,
}

type fullTransactionResponse struct {
	ID                    uuid.UUID `json:"id"`
	TransactionCode       string    `json:"transaction_code"`
	TransactionDate       string    `json:"transaction_date"`
	ItemID                *string   `json:"item_id"`
	Description           string    `json:"description"`
	Quantity              string    `json:"quantity"`
	UnitPrice             string    `json:"unit_price"`
	Amount                string    `json:"amount"`
	LineType              string    `json:"line_type"`
	AccountID             string    `json:"account_id"`
	CashAccountID         *string   `json:"cash_account_id"`
	OutletID              *string   `json:"outlet_id"`
	ReimbursementBatchID  *string   `json:"reimbursement_batch_id"`
	CreatedAt             time.Time `json:"created_at"`
}

type createTransactionRequest struct {
	TransactionDate string  `json:"transaction_date"`
	Description     string  `json:"description"`
	Quantity        string  `json:"quantity"`
	UnitPrice       string  `json:"unit_price"`
	LineType        string  `json:"line_type"`
	AccountID       string  `json:"account_id"`
	CashAccountID   *string `json:"cash_account_id"`
	OutletID        *string `json:"outlet_id"`
	ItemID          *string `json:"item_id"`
}

// --- Handlers ---

func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	params := database.ListAcctCashTransactionsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	if v := r.URL.Query().Get("start_date"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date"})
			return
		}
		params.StartDate = pgtype.Date{Time: d, Valid: true}
	}
	if v := r.URL.Query().Get("end_date"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date"})
			return
		}
		params.EndDate = pgtype.Date{Time: d, Valid: true}
	}
	if v := r.URL.Query().Get("line_type"); v != "" {
		params.LineType = pgtype.Text{String: v, Valid: true}
	}
	if v := r.URL.Query().Get("account_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
			return
		}
		params.AccountID = uuidToPgUUID(id)
	}
	if v := r.URL.Query().Get("cash_account_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
			return
		}
		params.CashAccountID = uuidToPgUUID(id)
	}
	if v := r.URL.Query().Get("outlet_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		params.OutletID = uuidToPgUUID(id)
	}
	if v := r.URL.Query().Get("search"); v != "" {
		params.Search = pgtype.Text{String: v, Valid: true}
	}

	rows, err := h.store.ListAcctCashTransactions(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list transactions: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	result := make([]fullTransactionResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, toFullTransactionResponse(row))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	row, err := h.store.GetAcctCashTransaction(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "transaction not found"})
			return
		}
		log.Printf("ERROR: get transaction: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toFullTransactionResponse(row))
}

func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req createTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.TransactionDate == "" || req.Description == "" || req.Quantity == "" ||
		req.UnitPrice == "" || req.LineType == "" || req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "transaction_date, description, quantity, unit_price, line_type, and account_id are required"})
		return
	}

	if !validLineTypes[req.LineType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid line_type"})
		return
	}

	date, err := time.Parse("2006-01-02", req.TransactionDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid transaction_date format, expected YYYY-MM-DD"})
		return
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	qty, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid quantity format"})
		return
	}
	price, err := decimal.NewFromString(req.UnitPrice)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid unit_price format"})
		return
	}
	amount := qty.Mul(price)

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan(qty.StringFixed(2))
	pricePg.Scan(price.StringFixed(2))
	amountPg.Scan(amount.StringFixed(2))

	var cashAccountID pgtype.UUID
	if req.CashAccountID != nil && *req.CashAccountID != "" {
		id, err := uuid.Parse(*req.CashAccountID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
			return
		}
		cashAccountID = uuidToPgUUID(id)
	}

	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(id)
	}

	var itemID pgtype.UUID
	if req.ItemID != nil && *req.ItemID != "" {
		id, err := uuid.Parse(*req.ItemID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item_id"})
			return
		}
		itemID = uuidToPgUUID(id)
	}

	// Get next transaction code
	maxCode, err := h.store.GetNextTransactionCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum, err := parseTransactionCodeNum(maxCode)
	if err != nil {
		log.Printf("ERROR: parse transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	transactionCode := fmt.Sprintf("PCS%06d", nextNum)

	row, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
		TransactionCode:      transactionCode,
		TransactionDate:      pgtype.Date{Time: date, Valid: true},
		ItemID:               itemID,
		Description:          req.Description,
		Quantity:             qtyPg,
		UnitPrice:            pricePg,
		Amount:               amountPg,
		LineType:             req.LineType,
		AccountID:            accountID,
		CashAccountID:        cashAccountID,
		OutletID:             outletID,
		ReimbursementBatchID: pgtype.Text{},
	})
	if err != nil {
		log.Printf("ERROR: create transaction: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toFullTransactionResponse(row))
}

// --- Helper functions ---

func toFullTransactionResponse(row database.AcctCashTransaction) fullTransactionResponse {
	resp := fullTransactionResponse{
		ID:              row.ID,
		TransactionCode: row.TransactionCode,
		TransactionDate: row.TransactionDate.Time.Format("2006-01-02"),
		Description:     row.Description,
		Quantity:        numericToString(row.Quantity),
		UnitPrice:       numericToString(row.UnitPrice),
		Amount:          numericToString(row.Amount),
		LineType:        row.LineType,
		AccountID:       row.AccountID.String(),
		CreatedAt:       row.CreatedAt,
	}
	if row.ItemID.Valid {
		s := uuid.UUID(row.ItemID.Bytes).String()
		resp.ItemID = &s
	}
	if row.CashAccountID.Valid {
		s := uuid.UUID(row.CashAccountID.Bytes).String()
		resp.CashAccountID = &s
	}
	if row.OutletID.Valid {
		s := uuid.UUID(row.OutletID.Bytes).String()
		resp.OutletID = &s
	}
	if row.ReimbursementBatchID.Valid {
		resp.ReimbursementBatchID = &row.ReimbursementBatchID.String
	}
	return resp
}
```

**Step 2: Write transaction handler tests**

Create `api/internal/accounting/handler/transaction_test.go`:

Tests to write:
1. `TestListTransactions` — returns list with default pagination
2. `TestListTransactions_WithFilters` — tests date, line_type, search filters
3. `TestGetTransaction` — returns single transaction by ID
4. `TestGetTransaction_NotFound` — returns 404
5. `TestCreateTransaction` — creates manual entry with all fields
6. `TestCreateTransaction_MissingFields` — 400 for missing required fields
7. `TestCreateTransaction_InvalidLineType` — 400 for bad line_type

**Step 3: Run tests**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestTransaction
```

Expected: all tests pass.

**Step 4: Commit**

```bash
git add api/internal/accounting/handler/transaction.go api/internal/accounting/handler/transaction_test.go
git commit -m "feat(accounting): add transaction handler — ledger list with search, manual entry"
```

---

## Task 6: Wire Routes

**Files:**
- Modify: `api/internal/router/router.go`

**Step 1: Add sales, payroll, and transaction routes**

In `router.go`, inside the existing accounting routes group (after the purchases route, around line 82), add:

```go
			// Sales
			salesHandler := accthandler.NewSalesHandler(queries)
			r.Route("/accounting/sales", salesHandler.RegisterRoutes)

			// Payroll
			payrollHandler := accthandler.NewPayrollHandler(queries)
			r.Route("/accounting/payroll", payrollHandler.RegisterRoutes)

			// Transactions (full ledger)
			transactionHandler := accthandler.NewTransactionHandler(queries)
			r.Route("/accounting/transactions", transactionHandler.RegisterRoutes)
```

**Step 2: Verify build**

```bash
cd api && go build ./...
```

Expected: clean build, no errors.

**Step 3: Commit**

```bash
git add api/internal/router/router.go
git commit -m "feat(accounting): wire sales, payroll, transaction routes (OWNER only)"
```

---

## Task 7: Admin Types + Sidebar

**Files:**
- Modify: `admin/src/lib/types/api.ts`
- Modify: `admin/src/lib/components/Sidebar.svelte`

**Step 1: Add TypeScript types**

Append to `admin/src/lib/types/api.ts` after the existing `AcctCashTransaction` interface:

```typescript
export interface AcctSalesDailySummary {
	id: string;
	sales_date: string;
	channel: string;
	payment_method: string;
	gross_sales: string;
	discount_amount: string;
	net_sales: string;
	cash_account_id: string;
	outlet_id: string | null;
	source: string;         // 'pos' | 'manual'
	posted_at: string | null;
	created_at: string;
}

export interface AcctPayrollEntry {
	id: string;
	payroll_date: string;
	period_type: string;     // 'Daily' | 'Weekly' | 'Monthly'
	period_ref: string | null;
	employee_name: string;
	gross_pay: string;
	payment_method: string;
	cash_account_id: string;
	outlet_id: string | null;
	posted_at: string | null;
	created_at: string;
}

export interface POSSyncRequest {
	start_date: string;
	end_date: string;
	outlet_id: string;
	payment_method_accounts: Record<string, string>;
}

export interface POSSyncResponse {
	synced_count: number;
	summaries: AcctSalesDailySummary[];
}
```

**Step 2: Add Sidebar nav items**

In `admin/src/lib/components/Sidebar.svelte`, update the `keuanganItems` array to add the 3 new pages:

```typescript
const keuanganItems: NavItem[] = [
	{ label: 'Pembelian', href: '/accounting/purchases', icon: '##', roles: ['OWNER'] },
	{ label: 'Penjualan', href: '/accounting/sales', icon: '##', roles: ['OWNER'] },
	{ label: 'Gaji', href: '/accounting/payroll', icon: '##', roles: ['OWNER'] },
	{ label: 'Jurnal', href: '/accounting/transactions', icon: '##', roles: ['OWNER'] },
	{ label: 'Master Data', href: '/accounting/master', icon: '##', roles: ['OWNER'] }
];
```

**Step 3: Verify admin build**

```bash
cd admin && pnpm build
```

Expected: builds with no errors (pre-existing a11y warnings are OK).

**Step 4: Commit**

```bash
git add admin/src/lib/types/api.ts admin/src/lib/components/Sidebar.svelte
git commit -m "feat(accounting): add sales/payroll/transaction types and sidebar nav items"
```

---

## Task 8: Admin Page — Penjualan (Sales)

**Files:**
- Create: `admin/src/routes/(app)/accounting/sales/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/sales/+page.svelte`

**Context:** The Penjualan page shows a monthly list of sales summaries. It has two flows:
1. **POS Sync**: Button to aggregate POS orders into summaries (calls `/accounting/sales/sync-pos`)
2. **Manual Entry**: Form to add non-POS sales (GoFood, ShopeeFood, etc.)
3. **Post**: Button to post unposted summaries to the ledger

Each row shows: date, channel, payment method, amounts, source badge (POS/Manual), posted status. Manual rows can be edited/deleted. POS rows are read-only.

**Step 1: Write the server-side load and actions**

Create `admin/src/routes/(app)/accounting/sales/+page.server.ts`:

```typescript
import type { PageServerLoad, Actions } from './$types';
import { apiRequest } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type {
	AcctSalesDailySummary,
	AcctCashAccount,
	AcctAccount
} from '$lib/types/api';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');

	const accessToken = cookies.get('access_token')!;

	// Default to current month
	const now = new Date();
	const startDate = url.searchParams.get('start_date') ||
		new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10);
	const endDate = url.searchParams.get('end_date') ||
		new Date(now.getFullYear(), now.getMonth() + 1, 0).toISOString().slice(0, 10);

	const [summariesResult, cashAccountsResult, accountsResult] = await Promise.all([
		apiRequest<AcctSalesDailySummary[]>(
			`/accounting/sales?start_date=${startDate}&end_date=${endDate}&limit=500`,
			{ accessToken }
		),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken }),
		apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken })
	]);

	return {
		summaries: summariesResult.ok ? summariesResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : [],
		startDate,
		endDate,
		outletId: user.outlet_id
	};
};

export const actions: Actions = {
	create: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const body = {
			sales_date: formData.get('sales_date')?.toString() ?? '',
			channel: formData.get('channel')?.toString() ?? '',
			payment_method: formData.get('payment_method')?.toString() ?? '',
			gross_sales: formData.get('gross_sales')?.toString() ?? '',
			discount_amount: formData.get('discount_amount')?.toString() || '0',
			net_sales: formData.get('net_sales')?.toString() ?? '',
			cash_account_id: formData.get('cash_account_id')?.toString() ?? '',
			outlet_id: user.outlet_id || null
		};

		const result = await apiRequest('/accounting/sales', {
			method: 'POST', body, accessToken
		});
		if (!result.ok) {
			if (result.status === 409) return fail(409, { createError: 'Data penjualan sudah ada untuk tanggal/channel/metode ini' });
			return fail(result.status || 400, { createError: result.message });
		}
		return { createSuccess: true };
	},

	update: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		const body = {
			channel: formData.get('channel')?.toString() ?? '',
			payment_method: formData.get('payment_method')?.toString() ?? '',
			gross_sales: formData.get('gross_sales')?.toString() ?? '',
			discount_amount: formData.get('discount_amount')?.toString() || '0',
			net_sales: formData.get('net_sales')?.toString() ?? '',
			cash_account_id: formData.get('cash_account_id')?.toString() ?? ''
		};

		const result = await apiRequest(`/accounting/sales/${id}`, {
			method: 'PUT', body, accessToken
		});
		if (!result.ok) return fail(result.status || 400, { updateError: result.message });
		return { updateSuccess: true };
	},

	delete: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		const result = await apiRequest(`/accounting/sales/${id}`, {
			method: 'DELETE', accessToken
		});
		if (!result.ok) return fail(result.status || 400, { deleteError: result.message });
		return { deleteSuccess: true };
	},

	syncPos: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const body = {
			start_date: formData.get('start_date')?.toString() ?? '',
			end_date: formData.get('end_date')?.toString() ?? '',
			outlet_id: user.outlet_id ?? '',
			payment_method_accounts: JSON.parse(formData.get('payment_method_accounts')?.toString() ?? '{}')
		};

		const result = await apiRequest<{ synced_count: number }>('/accounting/sales/sync-pos', {
			method: 'POST', body, accessToken
		});
		if (!result.ok) return fail(result.status || 400, { syncError: result.message });
		return { syncSuccess: true, syncedCount: result.data.synced_count };
	},

	postSales: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const body = {
			sales_date: formData.get('sales_date')?.toString() ?? '',
			outlet_id: user.outlet_id || null,
			account_id: formData.get('account_id')?.toString() ?? ''
		};

		const result = await apiRequest<{ posted_count: number }>('/accounting/sales/post', {
			method: 'POST', body, accessToken
		});
		if (!result.ok) return fail(result.status || 400, { postError: result.message });
		return { postSuccess: true, postedCount: result.data.posted_count };
	}
};
```

**Step 2: Write the page component**

Create `admin/src/routes/(app)/accounting/sales/+page.svelte`:

Build a page with:
- **Header:** "Penjualan" title, date range picker (defaults to current month)
- **Action buttons row:** "Sinkronkan POS" button (opens sync dialog), "Tambah Manual" button (opens create form)
- **Table:** Date, Channel, Payment Method, Gross Sales, Discount, Net Sales, Source badge (POS green / Manual blue), Posted status (checkmark or empty), Actions (edit/delete for manual unposted only)
- **Sync POS dialog:** Date range inputs, payment method → cash account mapping (one dropdown per unique payment method: Cash, QRIS, Transfer), "Sinkronkan" submit button
- **Create/Edit dialog:** Date, channel dropdown (Dine In, Take Away, GoFood, ShopeeFood, Catering, Delivery), payment method dropdown, gross sales, discount, net sales, cash account dropdown
- **Post dialog:** Select a date, pick sales revenue account, "Posting" submit button — posts all unposted summaries for that date
- **Amounts** formatted with `formatRupiah()`
- Group rows by date visually (date header rows or sticky date labels)

Follow the same Svelte 5 patterns as `purchases/+page.svelte`: `$state()` for form state, `$derived()` for computed values, `use:enhance` for form submissions, scoped `<style>` block using CSS variables.

**Step 3: Verify build**

```bash
cd admin && pnpm build
```

Expected: builds successfully.

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/sales/
git commit -m "feat(accounting): add Penjualan admin page — POS sync, manual entry, posting"
```

---

## Task 9: Admin Page — Gaji (Payroll)

**Files:**
- Create: `admin/src/routes/(app)/accounting/payroll/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/payroll/+page.svelte`

**Context:** The Gaji page lets the owner enter payroll for employees and post to the ledger.

**Step 1: Write the server-side load and actions**

Create `admin/src/routes/(app)/accounting/payroll/+page.server.ts`:

```typescript
import type { PageServerLoad, Actions } from './$types';
import { apiRequest } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type { AcctPayrollEntry, AcctCashAccount, AcctAccount } from '$lib/types/api';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');
	const accessToken = cookies.get('access_token')!;

	const now = new Date();
	const startDate = url.searchParams.get('start_date') ||
		new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10);
	const endDate = url.searchParams.get('end_date') ||
		new Date(now.getFullYear(), now.getMonth() + 1, 0).toISOString().slice(0, 10);

	const [entriesResult, cashAccountsResult, accountsResult] = await Promise.all([
		apiRequest<AcctPayrollEntry[]>(
			`/accounting/payroll?start_date=${startDate}&end_date=${endDate}&limit=500`,
			{ accessToken }
		),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken }),
		apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken })
	]);

	return {
		entries: entriesResult.ok ? entriesResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : [],
		startDate,
		endDate,
		outletId: user.outlet_id
	};
};

export const actions: Actions = {
	create: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const payrollDataStr = formData.get('payroll_data')?.toString() ?? '';

		let body;
		try {
			body = JSON.parse(payrollDataStr);
		} catch {
			return fail(400, { createError: 'Data tidak valid' });
		}

		// Add outlet_id
		body.outlet_id = user.outlet_id || null;

		const result = await apiRequest('/accounting/payroll', {
			method: 'POST', body, accessToken
		});
		if (!result.ok) return fail(result.status || 400, { createError: result.message });
		return { createSuccess: true };
	},

	update: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';
		const bodyStr = formData.get('entry_data')?.toString() ?? '';

		let body;
		try {
			body = JSON.parse(bodyStr);
		} catch {
			return fail(400, { updateError: 'Data tidak valid' });
		}
		body.outlet_id = user.outlet_id || null;

		const result = await apiRequest(`/accounting/payroll/${id}`, {
			method: 'PUT', body, accessToken
		});
		if (!result.ok) return fail(result.status || 400, { updateError: result.message });
		return { updateSuccess: true };
	},

	delete: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		const result = await apiRequest(`/accounting/payroll/${id}`, {
			method: 'DELETE', accessToken
		});
		if (!result.ok) return fail(result.status || 400, { deleteError: result.message });
		return { deleteSuccess: true };
	},

	postPayroll: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();

		const body = {
			ids: JSON.parse(formData.get('ids')?.toString() ?? '[]'),
			account_id: formData.get('account_id')?.toString() ?? ''
		};

		const result = await apiRequest<{ posted_count: number }>('/accounting/payroll/post', {
			method: 'POST', body, accessToken
		});
		if (!result.ok) return fail(result.status || 400, { postError: result.message });
		return { postSuccess: true, postedCount: result.data.posted_count };
	}
};
```

**Step 2: Write the page component**

Create `admin/src/routes/(app)/accounting/payroll/+page.svelte`:

Build a page with:
- **Header:** "Gaji" title, date range picker
- **Create button:** Opens multi-employee payroll entry form
- **Create form:** Payroll date, period type chips (Daily/Weekly/Monthly), period ref input, cash account dropdown. Multi-employee entry section: rows with employee name, gross pay, payment method dropdown. Add/remove employee rows. "Simpan" button.
- **Table:** Date, Period, Employee, Gross Pay, Payment Method, Cash Account, Posted status, Actions (edit/delete for unposted)
- **Post button:** Select unposted entries (checkboxes), pick expense account (pre-select 6090 Payroll Expense), "Posting" submit
- Use `$state()` for form state, serialize JSON in `use:enhance` callback (same pattern as purchases page)
- Stable `_key` counter for employee rows in `{#each}` (same pattern as purchase line items)

**Step 3: Verify build**

```bash
cd admin && pnpm build
```

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/payroll/
git commit -m "feat(accounting): add Gaji admin page — batch payroll entry, edit, posting"
```

---

## Task 10: Admin Page — Jurnal (Ledger)

**Files:**
- Create: `admin/src/routes/(app)/accounting/transactions/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/transactions/+page.svelte`

**Context:** The Jurnal page is the full ledger view — every cash transaction from purchases, sales, reimbursements, payroll, and manual entries. It supports comprehensive filtering and manual entry for one-off transactions.

**Step 1: Write the server-side load and actions**

Create `admin/src/routes/(app)/accounting/transactions/+page.server.ts`:

```typescript
import type { PageServerLoad, Actions } from './$types';
import { apiRequest } from '$lib/server/api';
import { fail, redirect } from '@sveltejs/kit';
import type {
	AcctCashTransaction,
	AcctAccount,
	AcctCashAccount,
	AcctItem
} from '$lib/types/api';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');
	const accessToken = cookies.get('access_token')!;

	// Build filter query string from URL params
	const params = new URLSearchParams();
	const startDate = url.searchParams.get('start_date');
	const endDate = url.searchParams.get('end_date');
	const lineType = url.searchParams.get('line_type');
	const accountId = url.searchParams.get('account_id');
	const cashAccountId = url.searchParams.get('cash_account_id');
	const search = url.searchParams.get('search');

	if (startDate) params.set('start_date', startDate);
	if (endDate) params.set('end_date', endDate);
	if (lineType) params.set('line_type', lineType);
	if (accountId) params.set('account_id', accountId);
	if (cashAccountId) params.set('cash_account_id', cashAccountId);
	if (search) params.set('search', search);
	params.set('limit', '100');

	const offset = url.searchParams.get('offset') || '0';
	params.set('offset', offset);

	const queryStr = params.toString();

	const [transactionsResult, accountsResult, cashAccountsResult, itemsResult] = await Promise.all([
		apiRequest<AcctCashTransaction[]>(`/accounting/transactions?${queryStr}`, { accessToken }),
		apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken }),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken }),
		apiRequest<AcctItem[]>('/accounting/master/items', { accessToken })
	]);

	return {
		transactions: transactionsResult.ok ? transactionsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		items: itemsResult.ok ? itemsResult.data : [],
		filters: { startDate, endDate, lineType, accountId, cashAccountId, search },
		offset: parseInt(offset)
	};
};

export const actions: Actions = {
	create: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const txDataStr = formData.get('transaction_data')?.toString() ?? '';

		let body;
		try {
			body = JSON.parse(txDataStr);
		} catch {
			return fail(400, { createError: 'Data tidak valid' });
		}

		const result = await apiRequest('/accounting/transactions', {
			method: 'POST', body, accessToken
		});
		if (!result.ok) return fail(result.status || 400, { createError: result.message });
		return { createSuccess: true };
	}
};
```

**Step 2: Write the page component**

Create `admin/src/routes/(app)/accounting/transactions/+page.svelte`:

Build a page with:
- **Header:** "Jurnal Kas" title
- **Filter bar:** Date range pickers, line_type dropdown (ASSET, INVENTORY, EXPENSE, SALES, COGS, LIABILITY, CAPITAL, DRAWING), account dropdown, cash account dropdown, text search input. Apply filters via URL params (use `goto()` from `$app/navigation`).
- **Tambah Transaksi button:** Opens manual entry form
- **Manual entry form:** Date, description, line_type dropdown, account dropdown, cash account dropdown (optional), outlet selector (optional), item selector (optional, autocomplete), quantity, unit price, auto-calculated amount. Serialize as JSON in `use:enhance` callback.
- **Table:** Code, Date, Description, Line Type badge, Amount (formatted with `formatRupiah`), Account name (lookup from accounts list by account_id), Cash Account name (lookup by cash_account_id)
- **Pagination:** Previous/Next buttons with offset-based navigation
- Look up account_name and cash_account_name by ID from the loaded reference data (build Maps in `$derived()`)
- Line type badges: color-coded (SALES=green, EXPENSE=red, INVENTORY=blue, etc.)

**Step 3: Verify build**

```bash
cd admin && pnpm build
```

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/transactions/
git commit -m "feat(accounting): add Jurnal admin page — filtered ledger view, manual entry"
```

---

## Task 11: Build Verification

**Step 1: Run all Go tests**

```bash
cd api && go test ./... -v
```

Expected: all tests pass (existing 435+ unit tests + new accounting tests from Tasks 3-5).

**Step 2: Run accounting tests specifically**

```bash
cd api && go test ./internal/accounting/... -v
```

Expected: all accounting handler tests pass.

**Step 3: Build Go binary**

```bash
cd api && go build ./cmd/server/
```

Expected: clean build.

**Step 4: Build admin**

```bash
cd admin && pnpm build
```

Expected: builds successfully (pre-existing a11y warnings are OK).

**Step 5: Commit (if any fixes were needed)**

If verification reveals issues, fix them and commit:

```bash
git commit -m "fix(accounting): phase 4 build verification fixes"
```

**Step 6: Merge to main**

If working in a worktree branch:

```bash
git checkout main
git merge feature/accounting-phase4
```

---

## Notes for Implementer

### Shared Helpers

Several helper functions are used across multiple handlers. Before creating them in new files, check if they already exist:
- `writeJSON` — defined in `api/internal/handler/auth.go` (POS handlers) and potentially redefined in `api/internal/accounting/handler/master.go`
- `uuidToPgUUID` — defined in `purchase.go`
- `numericToStringPtr` — defined in `master.go` (pointer variant)
- `numericToString` — new non-pointer variant needed (or reuse existing)
- `parsePagination` — may need to be added if not already in an accounting handler
- `parseTransactionCodeNum` — new helper, add once and reuse
- `isPgUniqueViolation` — may exist in master.go or need to be added

If a helper doesn't exist yet, add it to the first handler that needs it. Subsequent handlers in the same package can call it directly (same package = no import needed).

### sqlc Parameter Names

The `AggregatePOSSales` query uses positional params for the date range (`$2`, `$3`). sqlc may generate these as `Column2`, `Column3` in the params struct. Check the generated code and use the actual field names. If this is confusing, consider using named params: `sqlc.arg('start_date')::date` and `sqlc.arg('end_date')::date`.

### DeleteAcctSalesDailySummary and DeleteAcctPayrollEntry

These queries use `WHERE id = $1 AND source = 'manual' AND posted_at IS NULL` (sales) and `WHERE id = $1 AND posted_at IS NULL` (payroll). sqlc generates `:exec` return type for DELETE queries. The handler won't know if the row was actually deleted (0 rows affected returns no error for `:exec`). If you need to detect "not found or already posted", change to `:one` with `RETURNING id` — then `pgx.ErrNoRows` indicates the row doesn't exist or can't be deleted.

### POS Sync — Outlet ID

The `AggregatePOSSales` query requires `outlet_id` because POS orders are outlet-scoped. The admin user's `outlet_id` from their JWT can be used. If the owner manages multiple outlets, the frontend should offer an outlet selector.
