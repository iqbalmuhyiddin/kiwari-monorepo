# Accounting Module Phase 4 — Sales + Payroll + Ledger

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the accounting module: auto-aggregate POS sales into daily summaries, manual sales entry for non-POS channels, payroll entry with journal posting, and a full ledger view with manual entry for one-off transactions. This retires the GSheet entirely.

**Architecture:** Sales flow is create-then-post: summaries are created (via POS aggregate or manual entry), reviewed, then batch-posted to the journal. Payroll follows the same pattern (create → post). The ledger handler wraps the existing `ListAcctCashTransactions` query with a count for pagination, plus a manual entry endpoint for one-off transactions. POS aggregation joins the existing `orders` + `payments` tables, grouped by date/channel/payment_method. Each prefix (SLS/PYR/JNL) has its own transaction code sequence.

**Tech Stack:** Go 1.22+ (Chi, sqlc, pgx/v5, shopspring/decimal), PostgreSQL 16, SvelteKit 2 (Svelte 5, Tailwind CSS 4).

**Design Doc:** `docs/plans/2026-02-11-accounting-module-design.md` (Phase 4 section)

**Depends on:** Phase 1 complete (all `acct_*` tables, master data CRUD, purchase handler). Phase 3 complete (report handler — dashboard uses report queries).

---

## Codebase Conventions Reference

Same as Phase 1–3 plans. Key additions for Phase 4:

### New Patterns in Phase 4
- **Prefix-specific transaction codes:** Each feature has its own prefix (SLS, PYR, JNL) with independent sequences. Query `WHERE transaction_code LIKE 'SLS%'` instead of global MAX.
- **Create-then-post workflow:** Sales summaries and payroll entries are created first (posted_at=NULL), then batch-posted to the journal (creates cash_transactions, sets posted_at).
- **Upsert for idempotent aggregation:** `ON CONFLICT DO UPDATE` on sales summaries so re-running POS aggregate updates amounts instead of failing on duplicates.
- **Cross-table aggregation:** Sales aggregate JOINs POS `orders` + `payments` tables. The accounting handler accesses POS tables — acceptable coupling in a monorepo with shared `database.Queries`.

### Existing Helpers (from purchase.go, reuse these)
```go
// In api/internal/accounting/handler/purchase.go:
func uuidToPgUUID(id uuid.UUID) pgtype.UUID  // line 291
func writeJSON(w http.ResponseWriter, ...)     // In master.go line 221

// Decimal → pgtype.Numeric pattern:
var pg pgtype.Numeric
pg.Scan(decimal.StringFixed(2))
```

### Commands
```bash
cd api && go test ./internal/accounting/... -v
cd api && export PATH=$PATH:~/go/bin && sqlc generate
cd api && export PATH=$PATH:~/go/bin && migrate -path migrations/ -database "$DATABASE_URL" up
cd admin && pnpm dev
cd admin && pnpm build
```

---

## Task 1: Migration + sqlc Queries

**Files:**
- Create: `api/migrations/NNNNNN_accounting_phase4.up.sql` (check `ls api/migrations/` for next number)
- Create: `api/migrations/NNNNNN_accounting_phase4.down.sql`
- Create: `api/queries/acct_sales.sql`
- Create: `api/queries/acct_payroll.sql`
- Modify: `api/queries/acct_cash_transactions.sql` (add prefix-specific code queries + count)

**Step 1: Write the migration**

The `acct_sales_daily_summaries` table needs a `posted_at` column to track journal posting (parity with `acct_payroll_entries`). Check the next migration number with `ls api/migrations/`.

Up migration:

```sql
-- Phase 4: Add posted_at to sales summaries for create-then-post workflow
ALTER TABLE acct_sales_daily_summaries ADD COLUMN posted_at TIMESTAMPTZ;
```

Down migration:

```sql
ALTER TABLE acct_sales_daily_summaries DROP COLUMN IF EXISTS posted_at;
```

**Step 2: Write sales queries**

Create `api/queries/acct_sales.sql`:

```sql
-- name: ListAcctSalesDailySummaries :many
SELECT * FROM acct_sales_daily_summaries
WHERE
    (sqlc.narg('start_date')::date IS NULL OR sales_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR sales_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id')) AND
    (sqlc.narg('source')::text IS NULL OR source = sqlc.narg('source'))
ORDER BY sales_date DESC, channel, payment_method
LIMIT $1 OFFSET $2;

-- name: GetAcctSalesDailySummary :one
SELECT * FROM acct_sales_daily_summaries WHERE id = $1;

-- name: CreateAcctSalesDailySummary :one
INSERT INTO acct_sales_daily_summaries (
    sales_date, channel, payment_method, gross_sales, discount_amount,
    net_sales, cash_account_id, outlet_id, source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpsertAcctSalesDailySummary :one
INSERT INTO acct_sales_daily_summaries (
    sales_date, channel, payment_method, gross_sales, discount_amount,
    net_sales, cash_account_id, outlet_id, source
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (sales_date, channel, payment_method, outlet_id)
DO UPDATE SET
    gross_sales = EXCLUDED.gross_sales,
    discount_amount = EXCLUDED.discount_amount,
    net_sales = EXCLUDED.net_sales,
    cash_account_id = EXCLUDED.cash_account_id,
    source = EXCLUDED.source
RETURNING *;

-- name: UpdateSalesSummaryPosted :exec
UPDATE acct_sales_daily_summaries
SET posted_at = now()
WHERE id = $1;

-- name: DeleteAcctSalesDailySummary :one
DELETE FROM acct_sales_daily_summaries
WHERE id = $1 AND posted_at IS NULL
RETURNING id;

-- POS order aggregation: joins orders + payments tables
-- name: AggregatePOSSales :many
SELECT
    o.created_at::date AS sales_date,
    o.order_type::text AS channel,
    p.payment_method::text AS payment_method,
    o.outlet_id,
    COALESCE(SUM(p.amount), 0)::text AS net_sales
FROM orders o
JOIN payments p ON p.order_id = o.id
WHERE o.status = 'COMPLETED'
    AND p.status = 'COMPLETED'
    AND o.created_at::date >= sqlc.arg('start_date')
    AND o.created_at::date <= sqlc.arg('end_date')
    AND o.outlet_id = sqlc.arg('outlet_id')
GROUP BY o.created_at::date, o.order_type, p.payment_method, o.outlet_id
ORDER BY o.created_at::date;
```

**Step 3: Write payroll queries**

Create `api/queries/acct_payroll.sql`:

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

-- name: UpdatePayrollEntryPosted :exec
UPDATE acct_payroll_entries
SET posted_at = now()
WHERE id = $1;

-- name: DeleteAcctPayrollEntry :one
DELETE FROM acct_payroll_entries
WHERE id = $1 AND posted_at IS NULL
RETURNING id;
```

**Step 4: Add transaction code + count queries**

Append to `api/queries/acct_cash_transactions.sql`:

```sql
-- Prefix-specific transaction code sequences
-- name: GetNextSalesCode :one
SELECT COALESCE(MAX(transaction_code), 'SLS000000')::text AS max_code
FROM acct_cash_transactions
WHERE transaction_code LIKE 'SLS%';

-- name: GetNextPayrollCode :one
SELECT COALESCE(MAX(transaction_code), 'PYR000000')::text AS max_code
FROM acct_cash_transactions
WHERE transaction_code LIKE 'PYR%';

-- name: GetNextJournalCode :one
SELECT COALESCE(MAX(transaction_code), 'JNL000000')::text AS max_code
FROM acct_cash_transactions
WHERE transaction_code LIKE 'JNL%';

-- Pagination count (same filters as ListAcctCashTransactions + search)
-- name: CountAcctCashTransactions :one
SELECT COUNT(*)::int AS total
FROM acct_cash_transactions
WHERE
    (sqlc.narg('start_date')::date IS NULL OR transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('line_type')::text IS NULL OR line_type = sqlc.narg('line_type')) AND
    (sqlc.narg('account_id')::uuid IS NULL OR account_id = sqlc.narg('account_id')) AND
    (sqlc.narg('cash_account_id')::uuid IS NULL OR cash_account_id = sqlc.narg('cash_account_id')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id')) AND
    (sqlc.narg('search')::text IS NULL OR description ILIKE '%' || sqlc.narg('search') || '%');
```

**Step 5: Update ListAcctCashTransactions with search filter**

In `api/queries/acct_cash_transactions.sql`, add search filter to `ListAcctCashTransactions` (design doc requires text search on Jurnal page):

```sql
-- Add this line before the ORDER BY in ListAcctCashTransactions:
    (sqlc.narg('search')::text IS NULL OR description ILIKE '%' || sqlc.narg('search') || '%')
```

**Step 5b: Clean up stale GetNextTransactionCode**

Remove or comment out the old `GetNextTransactionCode` query (global MAX without prefix filter). It was used by Phase 1's purchase handler and Phase 2's reimbursement batch posting, both of which should now use `GetNextPurchaseCode` (added in Phase 2 Task 1). Leaving the global query invites accidental misuse.

**Step 6: Run migration + regenerate sqlc**

```bash
cd api && export PATH=$PATH:~/go/bin && migrate -path migrations/ -database "$DATABASE_URL" up
cd api && export PATH=$PATH:~/go/bin && sqlc generate
```

Expected: No errors. New query functions + updated `AcctSalesDailySummary` model with `PostedAt pgtype.Timestamptz`.

**Step 6: Commit**

```bash
git add api/migrations/ api/queries/acct_sales.sql api/queries/acct_payroll.sql api/queries/acct_cash_transactions.sql api/internal/database/
git commit -m "feat(accounting): add Phase 4 migration, sales/payroll/ledger sqlc queries"
```

---

## Task 2: Sales Handler

**Files:**
- Create: `api/internal/accounting/handler/sales.go`
- Create: `api/internal/accounting/handler/sales_test.go`

**Step 1: Write sales handler tests**

Create `api/internal/accounting/handler/sales_test.go`:

```go
package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock sales store ---

type mockSalesStore struct {
	summaries    map[uuid.UUID]database.AcctSalesDailySummary
	posData      []database.AggregatePOSSalesRow
	nextSalesNum int
}

func newMockSalesStore() *mockSalesStore {
	return &mockSalesStore{
		summaries:    make(map[uuid.UUID]database.AcctSalesDailySummary),
		nextSalesNum: 0,
	}
}

func (m *mockSalesStore) ListAcctSalesDailySummaries(ctx context.Context, arg database.ListAcctSalesDailySummariesParams) ([]database.AcctSalesDailySummary, error) {
	var result []database.AcctSalesDailySummary
	for _, s := range m.summaries {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSalesStore) GetAcctSalesDailySummary(ctx context.Context, id uuid.UUID) (database.AcctSalesDailySummary, error) {
	s, ok := m.summaries[id]
	if !ok {
		return database.AcctSalesDailySummary{}, pgx.ErrNoRows
	}
	return s, nil
}

func (m *mockSalesStore) CreateAcctSalesDailySummary(ctx context.Context, arg database.CreateAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error) {
	s := database.AcctSalesDailySummary{
		ID:            uuid.New(),
		SalesDate:     arg.SalesDate,
		Channel:       arg.Channel,
		PaymentMethod: arg.PaymentMethod,
		GrossSales:    arg.GrossSales,
		DiscountAmount: arg.DiscountAmount,
		NetSales:      arg.NetSales,
		CashAccountID: arg.CashAccountID,
		OutletID:      arg.OutletID,
		Source:        arg.Source,
		CreatedAt:     time.Now(),
	}
	m.summaries[s.ID] = s
	return s, nil
}

func (m *mockSalesStore) UpsertAcctSalesDailySummary(ctx context.Context, arg database.UpsertAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error) {
	s := database.AcctSalesDailySummary{
		ID:            uuid.New(),
		SalesDate:     arg.SalesDate,
		Channel:       arg.Channel,
		PaymentMethod: arg.PaymentMethod,
		GrossSales:    arg.GrossSales,
		DiscountAmount: arg.DiscountAmount,
		NetSales:      arg.NetSales,
		CashAccountID: arg.CashAccountID,
		OutletID:      arg.OutletID,
		Source:        arg.Source,
		CreatedAt:     time.Now(),
	}
	m.summaries[s.ID] = s
	return s, nil
}

func (m *mockSalesStore) UpdateSalesSummaryPosted(ctx context.Context, id uuid.UUID) error {
	s, ok := m.summaries[id]
	if !ok {
		return pgx.ErrNoRows
	}
	s.PostedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.summaries[id] = s
	return nil
}

func (m *mockSalesStore) AggregatePOSSales(ctx context.Context, arg database.AggregatePOSSalesParams) ([]database.AggregatePOSSalesRow, error) {
	return m.posData, nil
}

func (m *mockSalesStore) GetNextSalesCode(ctx context.Context) (string, error) {
	code := fmt.Sprintf("SLS%06d", m.nextSalesNum)
	m.nextSalesNum++
	return code, nil
}

func (m *mockSalesStore) CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error) {
	return database.AcctCashTransaction{ID: uuid.New(), TransactionCode: arg.TransactionCode}, nil
}

func setupSalesRouter(store handler.SalesStore) *chi.Mux {
	h := handler.NewSalesHandler(store, nil) // nil pool for non-batch tests
	r := chi.NewRouter()
	r.Route("/accounting/sales", h.RegisterRoutes)
	return r
}

func TestSalesList_Empty(t *testing.T) {
	store := newMockSalesStore()
	router := setupSalesRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/sales?limit=20&offset=0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestSalesCreate_Manual(t *testing.T) {
	store := newMockSalesStore()
	router := setupSalesRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/sales", map[string]interface{}{
		"sales_date":     "2026-01-20",
		"channel":        "GoFood",
		"payment_method": "Transfer",
		"gross_sales":    "500000",
		"discount_amount": "0",
		"net_sales":      "500000",
		"cash_account_id": uuid.New().String(),
		"outlet_id":      uuid.New().String(),
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestSalesCreate_MissingDate(t *testing.T) {
	store := newMockSalesStore()
	router := setupSalesRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/sales", map[string]interface{}{
		"channel":    "GoFood",
		"net_sales":  "500000",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
```

**Note:** The mock pattern follows `master_test.go` from Phase 1.

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: FAIL — `SalesStore` and `NewSalesHandler` not defined.

**Step 3: Write sales handler**

Create `api/internal/accounting/handler/sales.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

type SalesStore interface {
	ListAcctSalesDailySummaries(ctx context.Context, arg database.ListAcctSalesDailySummariesParams) ([]database.AcctSalesDailySummary, error)
	GetAcctSalesDailySummary(ctx context.Context, id uuid.UUID) (database.AcctSalesDailySummary, error)
	CreateAcctSalesDailySummary(ctx context.Context, arg database.CreateAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error)
	UpsertAcctSalesDailySummary(ctx context.Context, arg database.UpsertAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error)
	UpdateSalesSummaryPosted(ctx context.Context, id uuid.UUID) error
	DeleteAcctSalesDailySummary(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	AggregatePOSSales(ctx context.Context, arg database.AggregatePOSSalesParams) ([]database.AggregatePOSSalesRow, error)
	GetNextSalesCode(ctx context.Context) (string, error)
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
}

// --- Handler ---

type SalesHandler struct {
	store SalesStore
	pool  *pgxpool.Pool // for batch posting transaction (same pattern as ReimbursementHandler)
}

func NewSalesHandler(store SalesStore, pool *pgxpool.Pool) *SalesHandler {
	return &SalesHandler{store: store, pool: pool}
}

func (h *SalesHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListSales)
	r.Post("/", h.CreateSales)
	r.Delete("/{id}", h.DeleteSales)
	r.Post("/aggregate", h.AggregatePOS)
	r.Post("/post", h.PostSales)
}

// --- Request / Response types ---

type createSalesRequest struct {
	SalesDate      string  `json:"sales_date"`
	Channel        string  `json:"channel"`
	PaymentMethod  string  `json:"payment_method"`
	GrossSales     string  `json:"gross_sales"`
	DiscountAmount string  `json:"discount_amount"`
	NetSales       string  `json:"net_sales"`
	CashAccountID  string  `json:"cash_account_id"`
	OutletID       *string `json:"outlet_id"`
}

type aggregateRequest struct {
	StartDate          string            `json:"start_date"`
	EndDate            string            `json:"end_date"`
	OutletID           string            `json:"outlet_id"`
	CashAccountMapping map[string]string `json:"cash_account_mapping"` // payment_method → cash_account_id
}

type postSalesRequest struct {
	IDs       []string `json:"ids"`
	AccountID string   `json:"account_id"` // Sales Revenue account
}

type salesSummaryResponse struct {
	ID             string  `json:"id"`
	SalesDate      string  `json:"sales_date"`
	Channel        string  `json:"channel"`
	PaymentMethod  string  `json:"payment_method"`
	GrossSales     string  `json:"gross_sales"`
	DiscountAmount string  `json:"discount_amount"`
	NetSales       string  `json:"net_sales"`
	CashAccountID  string  `json:"cash_account_id"`
	OutletID       *string `json:"outlet_id"`
	Source         string  `json:"source"`
	PostedAt       *string `json:"posted_at"`
	CreatedAt      string  `json:"created_at"`
}

func toSalesSummaryResponse(s database.AcctSalesDailySummary) salesSummaryResponse {
	resp := salesSummaryResponse{
		ID:             s.ID.String(),
		SalesDate:      s.SalesDate.Time.Format("2006-01-02"),
		Channel:        s.Channel,
		PaymentMethod:  s.PaymentMethod,
		GrossSales:     numericToString(s.GrossSales),
		DiscountAmount: numericToString(s.DiscountAmount),
		NetSales:       numericToString(s.NetSales),
		CashAccountID:  s.CashAccountID.String(),
		Source:         s.Source,
		CreatedAt:      s.CreatedAt.Format(time.RFC3339),
	}
	if s.OutletID.Valid {
		oid := uuid.UUID(s.OutletID.Bytes).String()
		resp.OutletID = &oid
	}
	if s.PostedAt.Valid {
		pa := s.PostedAt.Time.Format(time.RFC3339)
		resp.PostedAt = &pa
	}
	return resp
}

// NOTE: Reuse numericToString() from reimbursement.go (Phase 2).
// It's the canonical Numeric→string helper. Do NOT create a duplicate.
// All handlers in this package can call numericToString() directly.

// --- Handlers ---

func (h *SalesHandler) ListSales(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	params := database.ListAcctSalesDailySummariesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}
	// Parse optional filters: start_date, end_date, outlet_id, source
	if s := r.URL.Query().Get("start_date"); s != "" {
		d, _ := time.Parse("2006-01-02", s)
		params.StartDate = pgtype.Date{Time: d, Valid: true}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		d, _ := time.Parse("2006-01-02", s)
		params.EndDate = pgtype.Date{Time: d, Valid: true}
	}
	if s := r.URL.Query().Get("outlet_id"); s != "" {
		id, _ := uuid.Parse(s)
		params.OutletID = uuidToPgUUID(id)
	}
	if s := r.URL.Query().Get("source"); s != "" {
		params.Source = pgtype.Text{String: s, Valid: true}
	}

	rows, err := h.store.ListAcctSalesDailySummaries(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list sales summaries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]salesSummaryResponse, len(rows))
	for i, s := range rows {
		resp[i] = toSalesSummaryResponse(s)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *SalesHandler) CreateSales(w http.ResponseWriter, r *http.Request) {
	var req createSalesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.SalesDate == "" || req.Channel == "" || req.PaymentMethod == "" || req.NetSales == "" || req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sales_date, channel, payment_method, net_sales, cash_account_id are required"})
		return
	}

	date, err := time.Parse("2006-01-02", req.SalesDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sales_date format"})
		return
	}
	cashAcctID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	grossSales, _ := decimal.NewFromString(req.GrossSales)
	discountAmt, _ := decimal.NewFromString(req.DiscountAmount)
	netSales, _ := decimal.NewFromString(req.NetSales)

	var grossPg, discPg, netPg pgtype.Numeric
	grossPg.Scan(grossSales.StringFixed(2))
	discPg.Scan(discountAmt.StringFixed(2))
	netPg.Scan(netSales.StringFixed(2))

	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(id)
	}

	s, err := h.store.CreateAcctSalesDailySummary(r.Context(), database.CreateAcctSalesDailySummaryParams{
		SalesDate:      pgtype.Date{Time: date, Valid: true},
		Channel:        req.Channel,
		PaymentMethod:  req.PaymentMethod,
		GrossSales:     grossPg,
		DiscountAmount: discPg,
		NetSales:       netPg,
		CashAccountID:  cashAcctID,
		OutletID:       outletID,
		Source:         "manual",
	})
	if err != nil {
		log.Printf("ERROR: create sales summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusCreated, toSalesSummaryResponse(s))
}

func (h *SalesHandler) DeleteSales(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ID"})
		return
	}
	_, err = h.store.DeleteAcctSalesDailySummary(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found or already posted"})
			return
		}
		log.Printf("ERROR: delete sales summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AggregatePOS pulls completed POS orders and upserts into sales summaries.
func (h *SalesHandler) AggregatePOS(w http.ResponseWriter, r *http.Request) {
	var req aggregateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.StartDate == "" || req.EndDate == "" || req.OutletID == "" || len(req.CashAccountMapping) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start_date, end_date, outlet_id, cash_account_mapping are required"})
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

	// Validate all cash_account_mapping UUIDs upfront
	cashMapping := make(map[string]uuid.UUID)
	for method, idStr := range req.CashAccountMapping {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid cash_account_id for %s", method)})
			return
		}
		cashMapping[method] = id
	}

	// Query POS data
	rows, err := h.store.AggregatePOSSales(r.Context(), database.AggregatePOSSalesParams{
		StartDate: pgtype.Date{Time: startDate, Valid: true},
		EndDate:   pgtype.Date{Time: endDate, Valid: true},
		OutletID:  outletID,
	})
	if err != nil {
		log.Printf("ERROR: aggregate POS sales: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Upsert each aggregated row
	var summaries []salesSummaryResponse
	for _, row := range rows {
		cashAcctID, ok := cashMapping[row.PaymentMethod]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("no cash_account_id mapping for payment method: %s", row.PaymentMethod),
			})
			return
		}

		netSales, _ := decimal.NewFromString(row.NetSales)
		var netPg pgtype.Numeric
		netPg.Scan(netSales.StringFixed(2))

		// gross_sales = net_sales for POS (discount already applied in order totals)
		zeroPg := pgtype.Numeric{}
		zeroPg.Scan("0.00")

		s, err := h.store.UpsertAcctSalesDailySummary(r.Context(), database.UpsertAcctSalesDailySummaryParams{
			SalesDate:      row.SalesDate,
			Channel:        row.Channel,
			PaymentMethod:  row.PaymentMethod,
			GrossSales:     netPg,
			DiscountAmount: zeroPg,
			NetSales:       netPg,
			CashAccountID:  cashAcctID,
			OutletID:       uuidToPgUUID(outletID),
			Source:         "pos",
		})
		if err != nil {
			log.Printf("ERROR: upsert sales summary: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		summaries = append(summaries, toSalesSummaryResponse(s))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summaries": summaries,
		"count":     len(summaries),
	})
}

// PostSales posts selected sales summaries to the journal as cash_transactions.
func (h *SalesHandler) PostSales(w http.ResponseWriter, r *http.Request) {
	var req postSalesRequest
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

	if h.pool == nil {
		log.Printf("ERROR: pool is nil for sales posting")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Begin DB transaction (same pattern as reimbursement batch posting in Phase 2)
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Printf("ERROR: begin tx: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	defer tx.Rollback(r.Context())

	qtx := database.New(tx)

	// Get next SLS code
	maxCode, err := qtx.GetNextSalesCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next sales code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum, _ := strconv.Atoi(maxCode[3:])
	nextNum++

	posted := 0
	skipped := 0

	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}

		s, err := qtx.GetAcctSalesDailySummary(r.Context(), id)
		if err != nil {
			continue
		}

		// Skip already posted
		if s.PostedAt.Valid {
			skipped++
			continue
		}

		code := fmt.Sprintf("SLS%06d", nextNum)
		nextNum++

		desc := fmt.Sprintf("Sales %s %s %s", s.Channel, s.PaymentMethod, s.SalesDate.Time.Format("2006-01-02"))

		_, err = qtx.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode: code,
			TransactionDate: s.SalesDate,
			Description:     desc,
			Quantity:        mustScanNumeric("1"),
			UnitPrice:       s.NetSales,
			Amount:          s.NetSales,
			LineType:        "SALES",
			AccountID:       accountID,
			CashAccountID:   uuidToPgUUID(s.CashAccountID),
			OutletID:        s.OutletID,
		})
		if err != nil {
			log.Printf("ERROR: create sales cash transaction: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		qtx.UpdateSalesSummaryPosted(r.Context(), id)
		posted++
	}

	// Commit transaction
	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("ERROR: commit sales posting: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"posted":  posted,
		"skipped": skipped,
	})
}

// mustScanNumeric converts a string to pgtype.Numeric. Panics on error (for known-good literals).
func mustScanNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		panic(fmt.Sprintf("mustScanNumeric(%q): %v", s, err))
	}
	return n
}
```

**Step 4: Run tests**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS.

**Step 5: Commit**

```bash
git add api/internal/accounting/handler/sales.go api/internal/accounting/handler/sales_test.go
git commit -m "feat(accounting): add sales handler with manual entry, POS aggregate, and batch posting"
```

---

## Task 3: Payroll Handler

**Files:**
- Create: `api/internal/accounting/handler/payroll.go`
- Create: `api/internal/accounting/handler/payroll_test.go`

**Step 1: Write payroll handler tests**

Create `api/internal/accounting/handler/payroll_test.go`. Key test cases:

1. **List empty** — GET `/accounting/payroll?limit=20&offset=0` → 200 with empty array
2. **Create entry** — POST with valid data → 201 with payroll entry (posted_at=null)
3. **Create missing employee_name** — POST → 400
4. **Post entry** — POST `/accounting/payroll/{id}/post` → 200, creates cash_transaction
5. **Post already posted** — POST `/accounting/payroll/{id}/post` → 409

Follow the same mock store pattern as `sales_test.go`.

**Step 2: Write payroll handler**

Create `api/internal/accounting/handler/payroll.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

type PayrollStore interface {
	ListAcctPayrollEntries(ctx context.Context, arg database.ListAcctPayrollEntriesParams) ([]database.AcctPayrollEntry, error)
	GetAcctPayrollEntry(ctx context.Context, id uuid.UUID) (database.AcctPayrollEntry, error)
	CreateAcctPayrollEntry(ctx context.Context, arg database.CreateAcctPayrollEntryParams) (database.AcctPayrollEntry, error)
	UpdatePayrollEntryPosted(ctx context.Context, id uuid.UUID) error
	DeleteAcctPayrollEntry(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	GetNextPayrollCode(ctx context.Context) (string, error)
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
}

type PayrollHandler struct {
	store PayrollStore
}

func NewPayrollHandler(store PayrollStore) *PayrollHandler {
	return &PayrollHandler{store: store}
}

func (h *PayrollHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListPayroll)
	r.Post("/", h.CreatePayroll)
	r.Delete("/{id}", h.DeletePayroll)
	r.Post("/{id}/post", h.PostPayroll)
}

// --- Request / Response types ---

type createPayrollRequest struct {
	PayrollDate   string  `json:"payroll_date"`
	PeriodType    string  `json:"period_type"`     // Daily|Weekly|Monthly
	PeriodRef     *string `json:"period_ref"`      // "2026-W03" or "2026-01"
	EmployeeName  string  `json:"employee_name"`
	GrossPay      string  `json:"gross_pay"`
	PaymentMethod string  `json:"payment_method"`
	CashAccountID string  `json:"cash_account_id"`
	OutletID      *string `json:"outlet_id"`
}

type postPayrollRequest struct {
	AccountID string `json:"account_id"` // Payroll Expense account (e.g. 6090)
}

type payrollResponse struct {
	ID            string  `json:"id"`
	PayrollDate   string  `json:"payroll_date"`
	PeriodType    string  `json:"period_type"`
	PeriodRef     *string `json:"period_ref"`
	EmployeeName  string  `json:"employee_name"`
	GrossPay      string  `json:"gross_pay"`
	PaymentMethod string  `json:"payment_method"`
	CashAccountID string  `json:"cash_account_id"`
	OutletID      *string `json:"outlet_id"`
	PostedAt      *string `json:"posted_at"`
	CreatedAt     string  `json:"created_at"`
}

func toPayrollResponse(p database.AcctPayrollEntry) payrollResponse {
	resp := payrollResponse{
		ID:            p.ID.String(),
		PayrollDate:   p.PayrollDate.Time.Format("2006-01-02"),
		PeriodType:    p.PeriodType,
		EmployeeName:  p.EmployeeName,
		GrossPay:      numericToString(p.GrossPay),
		PaymentMethod: p.PaymentMethod,
		CashAccountID: p.CashAccountID.String(),
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
	}
	if p.PeriodRef.Valid {
		resp.PeriodRef = &p.PeriodRef.String
	}
	if p.OutletID.Valid {
		oid := uuid.UUID(p.OutletID.Bytes).String()
		resp.OutletID = &oid
	}
	if p.PostedAt.Valid {
		pa := p.PostedAt.Time.Format(time.RFC3339)
		resp.PostedAt = &pa
	}
	return resp
}

// --- Handlers ---

func (h *PayrollHandler) ListPayroll(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	params := database.ListAcctPayrollEntriesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}
	if s := r.URL.Query().Get("start_date"); s != "" {
		d, _ := time.Parse("2006-01-02", s)
		params.StartDate = pgtype.Date{Time: d, Valid: true}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		d, _ := time.Parse("2006-01-02", s)
		params.EndDate = pgtype.Date{Time: d, Valid: true}
	}
	if s := r.URL.Query().Get("outlet_id"); s != "" {
		id, _ := uuid.Parse(s)
		params.OutletID = uuidToPgUUID(id)
	}
	if s := r.URL.Query().Get("period_type"); s != "" {
		params.PeriodType = pgtype.Text{String: s, Valid: true}
	}

	rows, err := h.store.ListAcctPayrollEntries(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list payroll entries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	resp := make([]payrollResponse, len(rows))
	for i, p := range rows {
		resp[i] = toPayrollResponse(p)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *PayrollHandler) CreatePayroll(w http.ResponseWriter, r *http.Request) {
	var req createPayrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	validPeriodTypes := map[string]bool{"Daily": true, "Weekly": true, "Monthly": true}
	if req.PayrollDate == "" || req.PeriodType == "" || req.EmployeeName == "" || req.GrossPay == "" || req.PaymentMethod == "" || req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payroll_date, period_type, employee_name, gross_pay, payment_method, cash_account_id are required"})
		return
	}
	if !validPeriodTypes[req.PeriodType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_type must be Daily, Weekly, or Monthly"})
		return
	}

	date, err := time.Parse("2006-01-02", req.PayrollDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payroll_date format"})
		return
	}
	cashAcctID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	grossPay, _ := decimal.NewFromString(req.GrossPay)
	var grossPg pgtype.Numeric
	grossPg.Scan(grossPay.StringFixed(2))

	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, _ := uuid.Parse(*req.OutletID)
		outletID = uuidToPgUUID(id)
	}

	var periodRef pgtype.Text
	if req.PeriodRef != nil && *req.PeriodRef != "" {
		periodRef = pgtype.Text{String: *req.PeriodRef, Valid: true}
	}

	p, err := h.store.CreateAcctPayrollEntry(r.Context(), database.CreateAcctPayrollEntryParams{
		PayrollDate:   pgtype.Date{Time: date, Valid: true},
		PeriodType:    req.PeriodType,
		PeriodRef:     periodRef,
		EmployeeName:  req.EmployeeName,
		GrossPay:      grossPg,
		PaymentMethod: req.PaymentMethod,
		CashAccountID: cashAcctID,
		OutletID:      outletID,
	})
	if err != nil {
		log.Printf("ERROR: create payroll entry: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusCreated, toPayrollResponse(p))
}

func (h *PayrollHandler) DeletePayroll(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ID"})
		return
	}
	_, err = h.store.DeleteAcctPayrollEntry(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found or already posted"})
			return
		}
		log.Printf("ERROR: delete payroll entry: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PostPayroll posts a single payroll entry to the journal.
func (h *PayrollHandler) PostPayroll(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ID"})
		return
	}

	var req postPayrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id is required"})
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	p, err := h.store.GetAcctPayrollEntry(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "payroll entry not found"})
			return
		}
		log.Printf("ERROR: get payroll entry: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if p.PostedAt.Valid {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already posted"})
		return
	}

	maxCode, err := h.store.GetNextPayrollCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next payroll code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum, _ := strconv.Atoi(maxCode[3:])
	nextNum++
	code := fmt.Sprintf("PYR%06d", nextNum)

	periodInfo := ""
	if p.PeriodRef.Valid {
		periodInfo = " " + p.PeriodRef.String
	}
	desc := fmt.Sprintf("Payroll %s%s %s", p.EmployeeName, periodInfo, p.PayrollDate.Time.Format("2006-01-02"))

	_, err = h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
		TransactionCode: code,
		TransactionDate: p.PayrollDate,
		Description:     desc,
		Quantity:        mustScanNumeric("1"),
		UnitPrice:       p.GrossPay,
		Amount:          p.GrossPay,
		LineType:        "EXPENSE",
		AccountID:       accountID,
		CashAccountID:   uuidToPgUUID(p.CashAccountID),
		OutletID:        p.OutletID,
	})
	if err != nil {
		log.Printf("ERROR: create payroll cash transaction: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	h.store.UpdatePayrollEntryPosted(r.Context(), id)

	// Re-fetch to get updated posted_at
	updated, _ := h.store.GetAcctPayrollEntry(r.Context(), id)
	writeJSON(w, http.StatusOK, toPayrollResponse(updated))
}
```

**Step 3: Run tests**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS.

**Step 4: Commit**

```bash
git add api/internal/accounting/handler/payroll.go api/internal/accounting/handler/payroll_test.go
git commit -m "feat(accounting): add payroll handler with create, list, and journal posting"
```

---

## Task 4: Ledger Handler

**Files:**
- Create: `api/internal/accounting/handler/ledger.go`
- Create: `api/internal/accounting/handler/ledger_test.go`

**Step 1: Write ledger handler tests**

Key test cases:
1. **List with pagination** — GET `/accounting/transactions?limit=20&offset=0` → 200 with `{ transactions, total }`
2. **Manual entry** — POST `/accounting/transactions` with valid data → 201
3. **Manual entry missing description** — POST → 400

**Step 2: Write ledger handler**

Create `api/internal/accounting/handler/ledger.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

type LedgerStore interface {
	ListAcctCashTransactions(ctx context.Context, arg database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error)
	CountAcctCashTransactions(ctx context.Context, arg database.CountAcctCashTransactionsParams) (int32, error)
	GetNextJournalCode(ctx context.Context) (string, error)
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
}

type LedgerHandler struct {
	store LedgerStore
}

func NewLedgerHandler(store LedgerStore) *LedgerHandler {
	return &LedgerHandler{store: store}
}

func (h *LedgerHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListTransactions)
	r.Post("/", h.CreateManualEntry)
}

// --- Request / Response types ---

type createManualEntryRequest struct {
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

// REUSE: FullTransactionResponse and ToFullTransactionResponse from dashboard.go (Phase 3).
// They are exported specifically for this purpose. Do NOT define a duplicate type.

type ledgerListResponse struct {
	Transactions []FullTransactionResponse `json:"transactions"`
	Total        int32                     `json:"total"`
}

// --- Handlers ---

func (h *LedgerHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	// Build shared filter params
	var startDate, endDate pgtype.Date
	var lineType pgtype.Text
	var accountID, cashAccountID, outletID pgtype.UUID

	if s := r.URL.Query().Get("start_date"); s != "" {
		d, _ := time.Parse("2006-01-02", s)
		startDate = pgtype.Date{Time: d, Valid: true}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		d, _ := time.Parse("2006-01-02", s)
		endDate = pgtype.Date{Time: d, Valid: true}
	}
	if s := r.URL.Query().Get("line_type"); s != "" {
		lineType = pgtype.Text{String: s, Valid: true}
	}
	if s := r.URL.Query().Get("account_id"); s != "" {
		id, _ := uuid.Parse(s)
		accountID = uuidToPgUUID(id)
	}
	if s := r.URL.Query().Get("cash_account_id"); s != "" {
		id, _ := uuid.Parse(s)
		cashAccountID = uuidToPgUUID(id)
	}
	if s := r.URL.Query().Get("outlet_id"); s != "" {
		id, _ := uuid.Parse(s)
		outletID = uuidToPgUUID(id)
	}
	// Search text filter (design doc: "Filterable: ... search text")
	var search pgtype.Text
	if s := r.URL.Query().Get("search"); s != "" {
		search = pgtype.Text{String: s, Valid: true}
	}

	// Run list + count queries
	rows, err := h.store.ListAcctCashTransactions(r.Context(), database.ListAcctCashTransactionsParams{
		StartDate:     startDate,
		EndDate:       endDate,
		LineType:      lineType,
		AccountID:     accountID,
		CashAccountID: cashAccountID,
		OutletID:      outletID,
		Search:        search,
		Limit:         int32(limit),
		Offset:        int32(offset),
	})
	if err != nil {
		log.Printf("ERROR: list transactions: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	total, err := h.store.CountAcctCashTransactions(r.Context(), database.CountAcctCashTransactionsParams{
		StartDate:     startDate,
		EndDate:       endDate,
		LineType:      lineType,
		AccountID:     accountID,
		CashAccountID: cashAccountID,
		OutletID:      outletID,
		Search:        search,
	})
	if err != nil {
		log.Printf("ERROR: count transactions: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	txResp := make([]FullTransactionResponse, len(rows))
	for i, tx := range rows {
		txResp[i] = ToFullTransactionResponse(tx)
	}

	writeJSON(w, http.StatusOK, ledgerListResponse{
		Transactions: txResp,
		Total:        total,
	})
}

func (h *LedgerHandler) CreateManualEntry(w http.ResponseWriter, r *http.Request) {
	var req createManualEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	validLineTypes := map[string]bool{
		"ASSET": true, "INVENTORY": true, "EXPENSE": true, "SALES": true,
		"COGS": true, "LIABILITY": true, "CAPITAL": true, "DRAWING": true,
	}

	if req.TransactionDate == "" || req.Description == "" || req.UnitPrice == "" || req.LineType == "" || req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "transaction_date, description, unit_price, line_type, account_id are required"})
		return
	}
	if !validLineTypes[req.LineType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid line_type"})
		return
	}

	date, err := time.Parse("2006-01-02", req.TransactionDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid transaction_date format"})
		return
	}
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	qty := decimal.NewFromInt(1)
	if req.Quantity != "" {
		qty, _ = decimal.NewFromString(req.Quantity)
	}
	price, _ := decimal.NewFromString(req.UnitPrice)
	amount := qty.Mul(price)

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan(qty.StringFixed(2))
	pricePg.Scan(price.StringFixed(2))
	amountPg.Scan(amount.StringFixed(2))

	var cashAccountID pgtype.UUID
	if req.CashAccountID != nil && *req.CashAccountID != "" {
		id, _ := uuid.Parse(*req.CashAccountID)
		cashAccountID = uuidToPgUUID(id)
	}
	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, _ := uuid.Parse(*req.OutletID)
		outletID = uuidToPgUUID(id)
	}
	var itemID pgtype.UUID
	if req.ItemID != nil && *req.ItemID != "" {
		id, _ := uuid.Parse(*req.ItemID)
		itemID = uuidToPgUUID(id)
	}

	maxCode, err := h.store.GetNextJournalCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next journal code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum, _ := strconv.Atoi(maxCode[3:])
	nextNum++
	code := fmt.Sprintf("JNL%06d", nextNum)

	tx, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
		TransactionCode:      code,
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
		log.Printf("ERROR: create manual transaction: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, ToFullTransactionResponse(tx))
}
```

**Step 3: Run tests, commit**

```bash
cd api && go test ./internal/accounting/handler/ -v
git add api/internal/accounting/handler/ledger.go api/internal/accounting/handler/ledger_test.go
git commit -m "feat(accounting): add ledger handler with filtered list, pagination count, and manual entry"
```

---

## Task 5: Routes + Types + Sidebar

**Files:**
- Modify: `api/internal/router/router.go`
- Modify: `admin/src/lib/types/api.ts`
- Modify: `admin/src/lib/components/Sidebar.svelte`

**Step 1: Add routes to router.go**

In the existing accounting `r.Group` block (after Phase 3 routes), add:

```go
// Sales (pool needed for batch posting transaction)
salesHandler := accthandler.NewSalesHandler(queries, pool)
r.Route("/accounting/sales", salesHandler.RegisterRoutes)

// Payroll
payrollHandler := accthandler.NewPayrollHandler(queries)
r.Route("/accounting/payroll", payrollHandler.RegisterRoutes)

// Ledger (full transaction list + manual entry)
ledgerHandler := accthandler.NewLedgerHandler(queries)
r.Route("/accounting/transactions", ledgerHandler.RegisterRoutes)
```

**Step 2: Add TypeScript types**

Append to `admin/src/lib/types/api.ts`:

```typescript
// ── Sales types ────────────────────

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
	source: 'pos' | 'manual';
	posted_at: string | null;
	created_at: string;
}

export interface AggregateRequest {
	start_date: string;
	end_date: string;
	outlet_id: string;
	cash_account_mapping: Record<string, string>;
}

export interface PostSalesRequest {
	ids: string[];
	account_id: string;
}

// ── Payroll types ────────────────────

export interface AcctPayrollEntry {
	id: string;
	payroll_date: string;
	period_type: 'Daily' | 'Weekly' | 'Monthly';
	period_ref: string | null;
	employee_name: string;
	gross_pay: string;
	payment_method: string;
	cash_account_id: string;
	outlet_id: string | null;
	posted_at: string | null;
	created_at: string;
}

export interface PostPayrollRequest {
	account_id: string;
}

// ── Ledger types ────────────────────

export interface LedgerListResponse {
	transactions: AcctCashTransaction[];
	total: number;
}

export interface CreateManualEntryRequest {
	transaction_date: string;
	description: string;
	quantity?: string;
	unit_price: string;
	line_type: string;
	account_id: string;
	cash_account_id?: string;
	outlet_id?: string;
	item_id?: string;
}
```

**Step 3: Update sidebar**

In `admin/src/lib/components/Sidebar.svelte`, update `keuanganItems` to include all Phase 4 pages:

```typescript
const keuanganItems: NavItem[] = [
    { label: 'Ringkasan', href: '/accounting', icon: '##', roles: ['OWNER'] },
    { label: 'Pembelian', href: '/accounting/purchases', icon: '##', roles: ['OWNER'] },
    { label: 'Penjualan', href: '/accounting/sales', icon: '##', roles: ['OWNER'] },
    { label: 'Reimburse', href: '/accounting/reimbursements', icon: '##', roles: ['OWNER'] },
    { label: 'Gaji', href: '/accounting/payroll', icon: '##', roles: ['OWNER'] },
    { label: 'Jurnal', href: '/accounting/transactions', icon: '##', roles: ['OWNER'] },
    { label: 'Laporan', href: '/accounting/reports', icon: '##', roles: ['OWNER'] },
    { label: 'Master Data', href: '/accounting/master', icon: '##', roles: ['OWNER'] }
];
```

This matches the navigation structure from the design doc exactly.

**Step 4: Verify compile + commit**

```bash
cd api && go build ./...
git add api/internal/router/router.go admin/src/lib/types/api.ts admin/src/lib/components/Sidebar.svelte
git commit -m "feat(accounting): wire sales/payroll/ledger routes, add types, complete sidebar navigation"
```

---

## Task 6: Admin Page — Penjualan (Sales)

**Files:**
- Create: `admin/src/routes/(app)/accounting/sales/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/sales/+page.svelte`

**Step 1: Write server load + actions**

Create `admin/src/routes/(app)/accounting/sales/+page.server.ts`:

```typescript
import { redirect, fail } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { AcctSalesDailySummary, AcctCashAccount } from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');

	const accessToken = cookies.get('access_token')!;
	const startDate = url.searchParams.get('start_date') ?? '';
	const endDate = url.searchParams.get('end_date') ?? '';

	const params = new URLSearchParams({ limit: '100', offset: '0' });
	if (startDate) params.set('start_date', startDate);
	if (endDate) params.set('end_date', endDate);

	const [salesResult, cashAccountsResult] = await Promise.all([
		apiRequest<AcctSalesDailySummary[]>(`/accounting/sales?${params}`, { accessToken }),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken })
	]);

	return {
		sales: salesResult.ok ? salesResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		startDate,
		endDate
	};
};

export const actions: Actions = {
	create: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const body = JSON.parse(formData.get('sales_data') as string);
		const result = await apiRequest('/accounting/sales', { method: 'POST', body, accessToken });
		if (!result.ok) return fail(result.status, { error: result.message });
		return { success: true };
	},
	aggregate: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const body = JSON.parse(formData.get('aggregate_data') as string);
		const result = await apiRequest('/accounting/sales/aggregate', { method: 'POST', body, accessToken });
		if (!result.ok) return fail(result.status, { error: result.message });
		return { success: true };
	},
	post: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const body = JSON.parse(formData.get('post_data') as string);
		const result = await apiRequest('/accounting/sales/post', { method: 'POST', body, accessToken });
		if (!result.ok) return fail(result.status, { error: result.message });
		return { success: true };
	}
};
```

**Step 2: Write page component**

Create `admin/src/routes/(app)/accounting/sales/+page.svelte`:

Key features (follow existing accounting page patterns):

- **Date filter bar**: Start/end date inputs (GET navigation)
- **"Tarik Data POS" button**: Opens dialog to configure date range, outlet, and cash account mapping per payment method (CASH → select, QRIS → select, TRANSFER → select). Submits aggregate action.
- **Sales summary table**: Date, channel, payment method, gross, discount, net, source badge (POS/Manual), posted status
- **Manual entry form**: For non-POS channels (GoFood, ShopeeFood, catering). Fields: date, channel (dropdown), payment method, amounts, cash account
- **"Post ke Jurnal" button**: Checkboxes on unposted rows. Posts selected to journal. Needs account_id for Sales Revenue (dropdown from accounts list, pre-filtered to Revenue type)
- **Styling**: Follow existing patterns from `purchases/+page.svelte`

**Step 3: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/sales/
git commit -m "feat(accounting): add sales admin page with POS aggregate, manual entry, and batch posting"
```

---

## Task 7: Admin Page — Gaji (Payroll)

**Files:**
- Create: `admin/src/routes/(app)/accounting/payroll/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/payroll/+page.svelte`

**Step 1: Write server load + actions**

Create `admin/src/routes/(app)/accounting/payroll/+page.server.ts`:

```typescript
import { redirect, fail } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { AcctPayrollEntry, AcctCashAccount, AcctAccount } from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ locals, cookies }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');

	const accessToken = cookies.get('access_token')!;

	const [payrollResult, cashAccountsResult, accountsResult] = await Promise.all([
		apiRequest<AcctPayrollEntry[]>('/accounting/payroll?limit=100&offset=0', { accessToken }),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken }),
		apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken })
	]);

	return {
		entries: payrollResult.ok ? payrollResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : []
	};
};

export const actions: Actions = {
	create: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const body = JSON.parse(formData.get('payroll_data') as string);
		const result = await apiRequest('/accounting/payroll', { method: 'POST', body, accessToken });
		if (!result.ok) return fail(result.status, { error: result.message });
		return { success: true };
	},
	post: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id') as string;
		const body = { account_id: formData.get('account_id') as string };
		const result = await apiRequest(`/accounting/payroll/${id}/post`, { method: 'POST', body, accessToken });
		if (!result.ok) return fail(result.status, { error: result.message });
		return { success: true };
	},
	delete: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id') as string;
		const result = await apiRequest(`/accounting/payroll/${id}`, { method: 'DELETE', accessToken });
		if (!result.ok) return fail(result.status, { error: result.message });
		return { success: true };
	}
};
```

**Step 2: Write page component**

Create `admin/src/routes/(app)/accounting/payroll/+page.svelte`:

Key features:

- **Entry form**: Date, period type (Daily/Weekly/Monthly dropdown), period ref, employee name, gross pay, payment method, cash account, outlet. "Simpan" button.
- **Entry table**: Date, employee, period, gross pay, method, posted status. Unposted rows have "Post" button and "Delete" button.
- **Post button per row**: Clicking opens small inline form to select Expense Account (pre-select 6090 Payroll Expense from accounts dropdown). Submits post action.
- **Batch entry**: Allow adding multiple employee rows before saving (client-side state array, submit all as individual API calls or serialize as JSON)

**Step 3: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/payroll/
git commit -m "feat(accounting): add payroll admin page with entry form and journal posting"
```

---

## Task 8: Admin Page — Jurnal (Ledger)

**Files:**
- Create: `admin/src/routes/(app)/accounting/transactions/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/transactions/+page.svelte`

**Step 1: Write server load + actions**

Create `admin/src/routes/(app)/accounting/transactions/+page.server.ts`:

```typescript
import { redirect, fail } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { LedgerListResponse, AcctAccount, AcctCashAccount } from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');

	const accessToken = cookies.get('access_token')!;

	const params = new URLSearchParams();
	params.set('limit', url.searchParams.get('limit') ?? '50');
	params.set('offset', url.searchParams.get('offset') ?? '0');
	for (const key of ['start_date', 'end_date', 'line_type', 'account_id', 'cash_account_id', 'outlet_id']) {
		const val = url.searchParams.get(key);
		if (val) params.set(key, val);
	}

	const [ledgerResult, accountsResult, cashAccountsResult] = await Promise.all([
		apiRequest<LedgerListResponse>(`/accounting/transactions?${params}`, { accessToken }),
		apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken }),
		apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken })
	]);

	return {
		ledger: ledgerResult.ok ? ledgerResult.data : { transactions: [], total: 0 },
		accounts: accountsResult.ok ? accountsResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		filters: Object.fromEntries(url.searchParams)
	};
};

export const actions: Actions = {
	create: async ({ request, cookies }) => {
		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const body = JSON.parse(formData.get('entry_data') as string);
		const result = await apiRequest('/accounting/transactions', { method: 'POST', body, accessToken });
		if (!result.ok) return fail(result.status, { error: result.message });
		return { success: true };
	}
};
```

**Step 2: Write page component**

Create `admin/src/routes/(app)/accounting/transactions/+page.svelte`:

Key features:

- **Filter bar**: Date range, line_type dropdown (all line types), account dropdown, cash account dropdown, outlet dropdown. All use URL params (GET navigation for bookmarkability).
- **Transaction table**: Code, date, description, qty, unit price, amount, line_type badge, account code. Paginated with prev/next buttons using offset.
- **Pagination**: Shows "Showing X-Y of Z" using `data.ledger.total`. Prev/next update URL `offset` param.
- **Manual entry form**: Expandable section at bottom. Fields: date, description, quantity (default 1), unit price, line_type dropdown, account dropdown, cash account dropdown, outlet dropdown, item dropdown (optional). "Tambah Entry" button.
- **Read-only indicator**: Auto-generated entries (PCS, SLS, PYR, RMB prefixed codes) show a "Auto" badge. Manual entries (JNL prefix) are editable.

**Step 3: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/transactions/
git commit -m "feat(accounting): add ledger admin page with filtered list, pagination, and manual entry"
```

---

## Task 9: Verify Phase 4 Build + Tests

**Step 1: Run Go tests**

Run: `cd api && go test ./... -v`

Expected: All tests pass (Phase 1 + 2 + 3 + 4).

**Step 2: Run admin build**

Run: `cd admin && pnpm build`

Expected: No type errors. Build succeeds.

**Step 3: Verify API compiles**

Run: `cd api && go build ./cmd/server/`

Expected: Binary compiles clean.

**Step 4: Verify all sidebar links work**

Run: `cd admin && pnpm dev` — click through all 8 Keuangan sidebar items. Verify each page loads without errors.

**Step 5: Commit any fixes**

Fix any issues, commit.

---

## Phase 4 Checklist

| # | Task | Delivers |
|---|------|----------|
| 1 | Migration + sqlc queries | `posted_at` column, sales/payroll CRUD, POS aggregate, prefix-specific codes, count |
| 2 | Sales handler | `GET/POST /sales`, `POST /sales/aggregate`, `POST /sales/post` |
| 3 | Payroll handler | `GET/POST /payroll`, `POST /payroll/{id}/post`, `DELETE /payroll/{id}` |
| 4 | Ledger handler | `GET /transactions` (paginated + count), `POST /transactions` (manual entry) |
| 5 | Routes + types + sidebar | Wiring, TypeScript types, complete 8-item sidebar |
| 6 | Admin — Penjualan | POS aggregate, manual entry, batch posting to journal |
| 7 | Admin — Gaji | Payroll entry form, per-row posting to journal |
| 8 | Admin — Jurnal | Full ledger with all filters, pagination, manual entry |
| 9 | Verify build | Full test suite + build + manual verification |

**Phase 4 completes the accounting module. GSheet is fully retired.**

---

## Full Module Summary (All Phases)

| Phase | Tasks | What it builds |
|-------|-------|----------------|
| P1 | 12 | DB tables, master data CRUD, item matcher, purchase entry |
| P2 | 8 | Reimbursement workflow, WhatsApp parser, batch posting |
| P3 | 7 | P&L + Cash Flow reports, dashboard |
| P4 | 9 | Sales (POS + manual), payroll, full ledger |
| **Total** | **36** | **Complete accounting module replacing GSheet** |
