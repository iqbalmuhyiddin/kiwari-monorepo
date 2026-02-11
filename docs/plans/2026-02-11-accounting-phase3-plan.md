# Accounting Module Phase 3 — Reports + Dashboard

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build financial reports (P&L and Cash Flow) and a dashboard overview page — providing real-time visibility into Kiwari's financial health, replacing the GSheet formula-based reports.

**Architecture:** New sqlc report queries using SQL aggregations over `acct_cash_transactions` (no report tables — computed on read). New `report.go` and `dashboard.go` handlers. P&L computes net_sales/COGS/expenses by month with optional expense breakdown by account. Cash Flow computes cash_in/cash_out by month by cash_account. Dashboard orchestrates multiple queries (balances, mini P&L, pending reimbursements, recent transactions) into a single response. SvelteKit pages: Laporan (two-tab reports with monthly columns, CSV export) and Ringkasan (dashboard cards + recent transactions).

**Tech Stack:** Go 1.22+ (Chi, sqlc, pgx/v5, shopspring/decimal), PostgreSQL 16, SvelteKit 2 (Svelte 5, Tailwind CSS 4).

**Design Doc:** `docs/plans/2026-02-11-accounting-module-design.md` (Phase 3 section)

**Depends on:** Phase 1 complete (all `acct_*` tables, master data CRUD, purchase handler). Phase 2 complete (reimbursement queries — `CountReimbursementsByStatus` used by dashboard).

---

## Codebase Conventions Reference

Same as Phase 1 and Phase 2 plans. Key notes for Phase 3:

### New Patterns in Phase 3
- **SQL aggregation queries:** `COALESCE(SUM(CASE WHEN ... THEN amount ELSE 0 END), 0)::text AS alias` — the `::text` cast is critical for sqlc to generate `string` return type instead of `interface{}`. See `sqlc-coalesce-type-inference` skill.
- **Multi-query orchestration:** Dashboard handler calls 4 different store methods and assembles a composite response. No DB transactions needed — all reads.
- **Client-side CSV export:** Reports page generates CSV from displayed data in the browser. No server endpoint for CSV.
- **Date formatting in SQL:** `to_char(date_trunc('month', transaction_date), 'YYYY-MM')` returns `"2026-01"` format. sqlc infers `string` type from `to_char()`.

### Commands
```bash
cd api && go test ./internal/accounting/... -v      # All accounting tests
cd api && go test ./internal/accounting/handler/ -v  # Handler tests only
cd api && export PATH=$PATH:~/go/bin && sqlc generate
cd admin && pnpm dev
cd admin && pnpm build
```

---

## Task 1: sqlc Report + Dashboard Queries

**Files:**
- Create: `api/queries/acct_reports.sql`

**Step 1: Write report and dashboard queries**

Create `api/queries/acct_reports.sql`:

```sql
-- ── Profit & Loss ──────────────────────────────

-- name: GetProfitAndLoss :many
SELECT
    to_char(date_trunc('month', transaction_date), 'YYYY-MM') AS period,
    COALESCE(SUM(CASE WHEN line_type = 'SALES' THEN amount ELSE 0 END), 0)::text AS net_sales,
    COALESCE(SUM(CASE WHEN line_type = 'COGS' THEN amount ELSE 0 END), 0)::text AS cogs,
    COALESCE(SUM(CASE WHEN line_type = 'EXPENSE' THEN amount ELSE 0 END), 0)::text AS expenses
FROM acct_cash_transactions
WHERE
    (sqlc.narg('start_date')::date IS NULL OR transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR outlet_id = sqlc.narg('outlet_id'))
GROUP BY date_trunc('month', transaction_date)
ORDER BY date_trunc('month', transaction_date);

-- name: GetExpenseBreakdown :many
SELECT
    to_char(date_trunc('month', ct.transaction_date), 'YYYY-MM') AS period,
    ct.account_id,
    a.account_code,
    a.account_name,
    COALESCE(SUM(ct.amount), 0)::text AS total
FROM acct_cash_transactions ct
JOIN acct_accounts a ON ct.account_id = a.id
WHERE ct.line_type = 'EXPENSE' AND
    (sqlc.narg('start_date')::date IS NULL OR ct.transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR ct.transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR ct.outlet_id = sqlc.narg('outlet_id'))
GROUP BY date_trunc('month', ct.transaction_date), ct.account_id, a.account_code, a.account_name
ORDER BY date_trunc('month', ct.transaction_date), a.account_code;

-- ── Cash Flow ──────────────────────────────────

-- name: GetCashFlow :many
SELECT
    to_char(date_trunc('month', ct.transaction_date), 'YYYY-MM') AS period,
    ct.cash_account_id,
    ca.cash_account_name,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('SALES','CAPITAL') THEN ct.amount ELSE 0 END), 0)::text AS cash_in,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('INVENTORY','EXPENSE','COGS','DRAWING','LIABILITY') THEN ct.amount ELSE 0 END), 0)::text AS cash_out
FROM acct_cash_transactions ct
JOIN acct_cash_accounts ca ON ct.cash_account_id = ca.id
WHERE ct.cash_account_id IS NOT NULL AND
    (sqlc.narg('start_date')::date IS NULL OR ct.transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR ct.transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR ct.outlet_id = sqlc.narg('outlet_id'))
GROUP BY date_trunc('month', ct.transaction_date), ct.cash_account_id, ca.cash_account_name
ORDER BY date_trunc('month', ct.transaction_date), ca.cash_account_name;

-- ── Dashboard ──────────────────────────────────

-- name: GetCashBalances :many
SELECT
    ca.id AS cash_account_id,
    ca.cash_account_code,
    ca.cash_account_name,
    COALESCE(SUM(
        CASE
            WHEN ct.line_type IN ('SALES','CAPITAL') THEN ct.amount
            ELSE -ct.amount
        END
    ), 0)::text AS balance
FROM acct_cash_accounts ca
LEFT JOIN acct_cash_transactions ct ON ct.cash_account_id = ca.id
WHERE ca.is_active = true
GROUP BY ca.id, ca.cash_account_code, ca.cash_account_name
ORDER BY ca.cash_account_code;

-- name: GetPendingReimbursementSummary :one
SELECT
    COUNT(*)::int AS count,
    COALESCE(SUM(amount), 0)::text AS total_amount
FROM acct_reimbursement_requests
WHERE status IN ('Draft', 'Ready');
```

**Step 2: Regenerate sqlc**

Run: `cd api && export PATH=$PATH:~/go/bin && sqlc generate`

Expected: No errors. New query functions generated in `api/internal/database/`. Expected generated types:

```go
// GetProfitAndLossRow — all strings thanks to ::text casts
type GetProfitAndLossRow struct {
    Period   string `json:"period"`
    NetSales string `json:"net_sales"`
    Cogs     string `json:"cogs"`
    Expenses string `json:"expenses"`
}

// GetExpenseBreakdownRow
type GetExpenseBreakdownRow struct {
    Period      string    `json:"period"`
    AccountID   uuid.UUID `json:"account_id"`
    AccountCode string    `json:"account_code"`
    AccountName string    `json:"account_name"`
    Total       string    `json:"total"`
}

// GetCashFlowRow
type GetCashFlowRow struct {
    Period          string      `json:"period"`
    CashAccountID   pgtype.UUID `json:"cash_account_id"`
    CashAccountName string      `json:"cash_account_name"`
    CashIn          string      `json:"cash_in"`
    CashOut         string      `json:"cash_out"`
}

// GetCashBalancesRow
type GetCashBalancesRow struct {
    CashAccountID   uuid.UUID `json:"cash_account_id"`
    CashAccountCode string    `json:"cash_account_code"`
    CashAccountName string    `json:"cash_account_name"`
    Balance         string    `json:"balance"`
}

// GetPendingReimbursementSummaryRow
type GetPendingReimbursementSummaryRow struct {
    Count       int32  `json:"count"`
    TotalAmount string `json:"total_amount"`
}
```

**Important:** Verify the generated types match the above. If sqlc generates `interface{}` instead of `string` for any COALESCE field, add explicit `::text` cast. If `to_char()` generates something other than `string`, wrap it: `to_char(...)::text AS period`.

**Step 3: Commit**

```bash
git add api/queries/acct_reports.sql api/internal/database/
git commit -m "feat(accounting): add sqlc queries for P&L, cash flow, dashboard"
```

---

## Task 2: Report Handler — P&L + Cash Flow

**Files:**
- Create: `api/internal/accounting/handler/report.go`
- Create: `api/internal/accounting/handler/report_test.go`

**Step 1: Write report handler tests**

Create `api/internal/accounting/handler/report_test.go`:

```go
package handler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock report store ---

type mockReportStore struct {
	pnlRows      []database.GetProfitAndLossRow
	expenseRows  []database.GetExpenseBreakdownRow
	cashFlowRows []database.GetCashFlowRow
}

func newMockReportStore() *mockReportStore {
	return &mockReportStore{}
}

func (m *mockReportStore) GetProfitAndLoss(ctx context.Context, arg database.GetProfitAndLossParams) ([]database.GetProfitAndLossRow, error) {
	return m.pnlRows, nil
}

func (m *mockReportStore) GetExpenseBreakdown(ctx context.Context, arg database.GetExpenseBreakdownParams) ([]database.GetExpenseBreakdownRow, error) {
	return m.expenseRows, nil
}

func (m *mockReportStore) GetCashFlow(ctx context.Context, arg database.GetCashFlowParams) ([]database.GetCashFlowRow, error) {
	return m.cashFlowRows, nil
}

func setupReportRouter(store handler.ReportStore) *chi.Mux {
	h := handler.NewReportHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/reports", h.RegisterRoutes)
	return r
}

// --- P&L tests ---

func TestPnL_Empty(t *testing.T) {
	store := newMockReportStore()
	router := setupReportRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/reports/pnl", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestPnL_WithData(t *testing.T) {
	store := newMockReportStore()
	store.pnlRows = []database.GetProfitAndLossRow{
		{Period: "2026-01", NetSales: "5000000.00", Cogs: "3000000.00", Expenses: "1000000.00"},
	}
	router := setupReportRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/reports/pnl?start_date=2026-01-01&end_date=2026-01-31", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestPnL_InvalidDateFormat(t *testing.T) {
	store := newMockReportStore()
	router := setupReportRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/reports/pnl?start_date=not-a-date", nil)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Cash Flow tests ---

func TestCashFlow_Empty(t *testing.T) {
	store := newMockReportStore()
	router := setupReportRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/reports/cashflow", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestCashFlow_WithData(t *testing.T) {
	store := newMockReportStore()
	cashAcctID := uuid.New()
	store.cashFlowRows = []database.GetCashFlowRow{
		{
			Period:          "2026-01",
			CashAccountID:   pgtype.UUID{Bytes: cashAcctID, Valid: true},
			CashAccountName: "Kas Utama",
			CashIn:          "5000000.00",
			CashOut:         "3000000.00",
		},
	}
	router := setupReportRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/reports/cashflow?start_date=2026-01-01&end_date=2026-01-31", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: FAIL — `ReportStore` and `NewReportHandler` not defined yet.

**Step 3: Write report handler**

Create `api/internal/accounting/handler/report.go`:

```go
package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

// ReportStore defines the database methods for report handlers.
type ReportStore interface {
	GetProfitAndLoss(ctx context.Context, arg database.GetProfitAndLossParams) ([]database.GetProfitAndLossRow, error)
	GetExpenseBreakdown(ctx context.Context, arg database.GetExpenseBreakdownParams) ([]database.GetExpenseBreakdownRow, error)
	GetCashFlow(ctx context.Context, arg database.GetCashFlowParams) ([]database.GetCashFlowRow, error)
}

// --- Handler ---

// ReportHandler handles financial report endpoints.
type ReportHandler struct {
	store ReportStore
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(store ReportStore) *ReportHandler {
	return &ReportHandler{store: store}
}

// RegisterRoutes registers report endpoints.
func (h *ReportHandler) RegisterRoutes(r chi.Router) {
	r.Get("/pnl", h.ProfitAndLoss)
	r.Get("/cashflow", h.CashFlow)
}

// --- Response types ---

type pnlSummaryRow struct {
	Period      string `json:"period"`
	NetSales    string `json:"net_sales"`
	COGS        string `json:"cogs"`
	GrossProfit string `json:"gross_profit"`
	Expenses    string `json:"expenses"`
	NetProfit   string `json:"net_profit"`
	GrossMargin string `json:"gross_margin"` // percentage
	NetMargin   string `json:"net_margin"`   // percentage
}

type expenseBreakdownRow struct {
	Period      string `json:"period"`
	AccountID   string `json:"account_id"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	Total       string `json:"total"`
}

type pnlResponse struct {
	Summary          []pnlSummaryRow       `json:"summary"`
	ExpenseBreakdown []expenseBreakdownRow  `json:"expense_breakdown"`
}

type cashFlowRow struct {
	Period          string `json:"period"`
	CashAccountID   string `json:"cash_account_id"`
	CashAccountName string `json:"cash_account_name"`
	CashIn          string `json:"cash_in"`
	CashOut         string `json:"cash_out"`
	NetCashFlow     string `json:"net_cash_flow"`
}

// --- Handlers ---

// ProfitAndLoss returns P&L data grouped by month.
// Query params: start_date, end_date, outlet_id (all optional).
func (h *ReportHandler) ProfitAndLoss(w http.ResponseWriter, r *http.Request) {
	params, err := parseReportFilters(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	pnlParams := database.GetProfitAndLossParams{
		StartDate: params.startDate,
		EndDate:   params.endDate,
		OutletID:  params.outletID,
	}

	rows, err := h.store.GetProfitAndLoss(r.Context(), pnlParams)
	if err != nil {
		log.Printf("ERROR: get P&L: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	summary := make([]pnlSummaryRow, len(rows))
	for i, row := range rows {
		netSales, _ := decimal.NewFromString(row.NetSales)
		cogs, _ := decimal.NewFromString(row.Cogs)
		expenses, _ := decimal.NewFromString(row.Expenses)

		grossProfit := netSales.Sub(cogs)
		netProfit := grossProfit.Sub(expenses)

		grossMargin := decimal.Zero
		netMargin := decimal.Zero
		if !netSales.IsZero() {
			grossMargin = grossProfit.Div(netSales).Mul(decimal.NewFromInt(100))
			netMargin = netProfit.Div(netSales).Mul(decimal.NewFromInt(100))
		}

		summary[i] = pnlSummaryRow{
			Period:      row.Period,
			NetSales:    netSales.StringFixed(2),
			COGS:        cogs.StringFixed(2),
			GrossProfit: grossProfit.StringFixed(2),
			Expenses:    expenses.StringFixed(2),
			NetProfit:   netProfit.StringFixed(2),
			GrossMargin: grossMargin.StringFixed(2),
			NetMargin:   netMargin.StringFixed(2),
		}
	}

	// Get expense breakdown
	expParams := database.GetExpenseBreakdownParams{
		StartDate: params.startDate,
		EndDate:   params.endDate,
		OutletID:  params.outletID,
	}

	expRows, err := h.store.GetExpenseBreakdown(r.Context(), expParams)
	if err != nil {
		log.Printf("ERROR: get expense breakdown: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	breakdown := make([]expenseBreakdownRow, len(expRows))
	for i, row := range expRows {
		breakdown[i] = expenseBreakdownRow{
			Period:      row.Period,
			AccountID:   row.AccountID.String(),
			AccountCode: row.AccountCode,
			AccountName: row.AccountName,
			Total:       row.Total,
		}
	}

	writeJSON(w, http.StatusOK, pnlResponse{
		Summary:          summary,
		ExpenseBreakdown: breakdown,
	})
}

// CashFlow returns cash flow data grouped by month and cash account.
// Query params: start_date, end_date, outlet_id (all optional).
func (h *ReportHandler) CashFlow(w http.ResponseWriter, r *http.Request) {
	params, err := parseReportFilters(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	cfParams := database.GetCashFlowParams{
		StartDate: params.startDate,
		EndDate:   params.endDate,
		OutletID:  params.outletID,
	}

	rows, err := h.store.GetCashFlow(r.Context(), cfParams)
	if err != nil {
		log.Printf("ERROR: get cash flow: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]cashFlowRow, len(rows))
	for i, row := range rows {
		cashIn, _ := decimal.NewFromString(row.CashIn)
		cashOut, _ := decimal.NewFromString(row.CashOut)
		netCashFlow := cashIn.Sub(cashOut)

		cashAccountID := ""
		if row.CashAccountID.Valid {
			cashAccountID = uuid.UUID(row.CashAccountID.Bytes).String()
		}

		resp[i] = cashFlowRow{
			Period:          row.Period,
			CashAccountID:   cashAccountID,
			CashAccountName: row.CashAccountName,
			CashIn:          cashIn.StringFixed(2),
			CashOut:         cashOut.StringFixed(2),
			NetCashFlow:     netCashFlow.StringFixed(2),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Filter parsing helper ---

type reportFilters struct {
	startDate pgtype.Date
	endDate   pgtype.Date
	outletID  pgtype.UUID
}

func parseReportFilters(r *http.Request) (reportFilters, error) {
	var f reportFilters

	if s := r.URL.Query().Get("start_date"); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			return f, fmt.Errorf("invalid start_date format, expected YYYY-MM-DD")
		}
		f.startDate = pgtype.Date{Time: d, Valid: true}
	}

	if s := r.URL.Query().Get("end_date"); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			return f, fmt.Errorf("invalid end_date format, expected YYYY-MM-DD")
		}
		f.endDate = pgtype.Date{Time: d, Valid: true}
	}

	if s := r.URL.Query().Get("outlet_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return f, fmt.Errorf("invalid outlet_id")
		}
		f.outletID = uuidToPgUUID(id)
	}

	return f, nil
}
```

**Note:** The `uuidToPgUUID` and `writeJSON` helpers are already defined in `master.go` / `purchase.go` from Phase 1. The `numericToString` helper is defined in `reimbursement.go` from Phase 2.

**Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS (including existing Phase 1 + Phase 2 tests).

**Step 5: Commit**

```bash
git add api/internal/accounting/handler/report.go api/internal/accounting/handler/report_test.go
git commit -m "feat(accounting): add P&L and cash flow report handlers"
```

---

## Task 3: Dashboard Handler

**Files:**
- Create: `api/internal/accounting/handler/dashboard.go`
- Create: `api/internal/accounting/handler/dashboard_test.go`

**Step 1: Write dashboard handler tests**

Create `api/internal/accounting/handler/dashboard_test.go`:

```go
package handler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock dashboard store ---

type mockDashboardStore struct {
	balances     []database.GetCashBalancesRow
	pnlRows      []database.GetProfitAndLossRow
	pendingReimb database.GetPendingReimbursementSummaryRow
	transactions []database.AcctCashTransaction
}

func newMockDashboardStore() *mockDashboardStore {
	return &mockDashboardStore{
		pendingReimb: database.GetPendingReimbursementSummaryRow{
			Count:       0,
			TotalAmount: "0.00",
		},
	}
}

func (m *mockDashboardStore) GetCashBalances(ctx context.Context) ([]database.GetCashBalancesRow, error) {
	return m.balances, nil
}

func (m *mockDashboardStore) GetProfitAndLoss(ctx context.Context, arg database.GetProfitAndLossParams) ([]database.GetProfitAndLossRow, error) {
	return m.pnlRows, nil
}

func (m *mockDashboardStore) GetPendingReimbursementSummary(ctx context.Context) (database.GetPendingReimbursementSummaryRow, error) {
	return m.pendingReimb, nil
}

func (m *mockDashboardStore) ListAcctCashTransactions(ctx context.Context, arg database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error) {
	return m.transactions, nil
}

func setupDashboardRouter(store handler.DashboardStore) *chi.Mux {
	h := handler.NewDashboardHandler(store)
	r := chi.NewRouter()
	r.Get("/accounting/dashboard", h.Overview)
	return r
}

// --- Dashboard tests ---

func TestDashboard_Empty(t *testing.T) {
	store := newMockDashboardStore()
	router := setupDashboardRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/dashboard", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestDashboard_WithData(t *testing.T) {
	store := newMockDashboardStore()
	store.balances = []database.GetCashBalancesRow{
		{
			CashAccountID:   uuid.New(),
			CashAccountCode: "CASH001",
			CashAccountName: "Kas Utama",
			Balance:         "5000000.00",
		},
	}
	store.pnlRows = []database.GetProfitAndLossRow{
		{Period: "2026-02", NetSales: "5000000.00", Cogs: "3000000.00", Expenses: "1000000.00"},
	}
	store.pendingReimb = database.GetPendingReimbursementSummaryRow{
		Count:       3,
		TotalAmount: "500000.00",
	}

	router := setupDashboardRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/dashboard", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: FAIL — `DashboardStore` and `NewDashboardHandler` not defined.

**Step 3: Write dashboard handler**

Create `api/internal/accounting/handler/dashboard.go`:

```go
package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

// DashboardStore defines the database methods for the dashboard handler.
type DashboardStore interface {
	GetCashBalances(ctx context.Context) ([]database.GetCashBalancesRow, error)
	GetProfitAndLoss(ctx context.Context, arg database.GetProfitAndLossParams) ([]database.GetProfitAndLossRow, error)
	GetPendingReimbursementSummary(ctx context.Context) (database.GetPendingReimbursementSummaryRow, error)
	ListAcctCashTransactions(ctx context.Context, arg database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error)
}

// --- Handler ---

// DashboardHandler handles the accounting dashboard endpoint.
type DashboardHandler struct {
	store DashboardStore
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(store DashboardStore) *DashboardHandler {
	return &DashboardHandler{store: store}
}

// --- Response types ---

type cashBalanceItem struct {
	CashAccountID   string `json:"cash_account_id"`
	CashAccountCode string `json:"cash_account_code"`
	CashAccountName string `json:"cash_account_name"`
	Balance         string `json:"balance"`
}

type miniPnl struct {
	Period      string `json:"period"`
	NetSales    string `json:"net_sales"`
	COGS        string `json:"cogs"`
	GrossProfit string `json:"gross_profit"`
	Expenses    string `json:"expenses"`
	NetProfit   string `json:"net_profit"`
}

type pendingReimbursements struct {
	Count       int32  `json:"count"`
	TotalAmount string `json:"total_amount"`
}

type dashboardResponse struct {
	CashBalances          []cashBalanceItem     `json:"cash_balances"`
	CurrentMonthPnl       *miniPnl              `json:"current_month_pnl"`
	PendingReimbursements pendingReimbursements  `json:"pending_reimbursements"`
	RecentTransactions    []FullTransactionResponse  `json:"recent_transactions"`
}

// FullTransactionResponse is the canonical full-field transaction response.
// EXPORTED so Phase 4's ledger handler can reuse it instead of defining a duplicate.
// Note: purchase.go has a slimmer `transactionResponse` (9 fields, purchase-specific).
// This one has 13 fields (includes cash_account_id, outlet_id, reimbursement_batch_id).
type FullTransactionResponse struct {
	ID                   string  `json:"id"`
	TransactionCode      string  `json:"transaction_code"`
	TransactionDate      string  `json:"transaction_date"`
	ItemID               *string `json:"item_id"`
	Description          string  `json:"description"`
	Quantity             string  `json:"quantity"`
	UnitPrice            string  `json:"unit_price"`
	Amount               string  `json:"amount"`
	LineType             string  `json:"line_type"`
	AccountID            string  `json:"account_id"`
	CashAccountID        *string `json:"cash_account_id"`
	OutletID             *string `json:"outlet_id"`
	ReimbursementBatchID *string `json:"reimbursement_batch_id"`
	CreatedAt            string  `json:"created_at"`
}

func ToFullTransactionResponse(tx database.AcctCashTransaction) FullTransactionResponse {
	resp := FullTransactionResponse{
		ID:              tx.ID.String(),
		TransactionCode: tx.TransactionCode,
		TransactionDate: tx.TransactionDate.Time.Format("2006-01-02"),
		Description:     tx.Description,
		Quantity:        numericToString(tx.Quantity),
		UnitPrice:       numericToString(tx.UnitPrice),
		Amount:          numericToString(tx.Amount),
		LineType:        tx.LineType,
		AccountID:       tx.AccountID.String(),
		CreatedAt:       tx.CreatedAt.Format(time.RFC3339),
	}
	if tx.ItemID.Valid {
		s := uuid.UUID(tx.ItemID.Bytes).String()
		resp.ItemID = &s
	}
	if tx.CashAccountID.Valid {
		s := uuid.UUID(tx.CashAccountID.Bytes).String()
		resp.CashAccountID = &s
	}
	if tx.OutletID.Valid {
		s := uuid.UUID(tx.OutletID.Bytes).String()
		resp.OutletID = &s
	}
	if tx.ReimbursementBatchID.Valid {
		resp.ReimbursementBatchID = &tx.ReimbursementBatchID.String
	}
	return resp
}

// --- Handler ---

// Overview returns the dashboard data: cash balances, current month P&L, pending reimbursements, recent transactions.
func (h *DashboardHandler) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Cash balances
	balances, err := h.store.GetCashBalances(ctx)
	if err != nil {
		log.Printf("ERROR: get cash balances: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	balanceItems := make([]cashBalanceItem, len(balances))
	for i, b := range balances {
		balanceItems[i] = cashBalanceItem{
			CashAccountID:   b.CashAccountID.String(),
			CashAccountCode: b.CashAccountCode,
			CashAccountName: b.CashAccountName,
			Balance:         b.Balance,
		}
	}

	// 2. Current month P&L
	now := time.Now()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	pnlRows, err := h.store.GetProfitAndLoss(ctx, database.GetProfitAndLossParams{
		StartDate: pgtype.Date{Time: firstOfMonth, Valid: true},
		EndDate:   pgtype.Date{Time: now, Valid: true},
	})
	if err != nil {
		log.Printf("ERROR: get current month P&L: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	var currentPnl *miniPnl
	if len(pnlRows) > 0 {
		row := pnlRows[0]
		netSales, _ := decimal.NewFromString(row.NetSales)
		cogs, _ := decimal.NewFromString(row.Cogs)
		expenses, _ := decimal.NewFromString(row.Expenses)
		grossProfit := netSales.Sub(cogs)
		netProfit := grossProfit.Sub(expenses)

		currentPnl = &miniPnl{
			Period:      row.Period,
			NetSales:    netSales.StringFixed(2),
			COGS:        cogs.StringFixed(2),
			GrossProfit: grossProfit.StringFixed(2),
			Expenses:    expenses.StringFixed(2),
			NetProfit:   netProfit.StringFixed(2),
		}
	} else {
		currentPnl = &miniPnl{
			Period:      now.Format("2006-01"),
			NetSales:    "0.00",
			COGS:        "0.00",
			GrossProfit: "0.00",
			Expenses:    "0.00",
			NetProfit:   "0.00",
		}
	}

	// 3. Pending reimbursements
	pendingReimb, err := h.store.GetPendingReimbursementSummary(ctx)
	if err != nil {
		log.Printf("ERROR: get pending reimbursements: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// 4. Recent 10 transactions
	recentTx, err := h.store.ListAcctCashTransactions(ctx, database.ListAcctCashTransactionsParams{
		Limit: 10,
	})
	if err != nil {
		log.Printf("ERROR: get recent transactions: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	txResp := make([]FullTransactionResponse, len(recentTx))
	for i, tx := range recentTx {
		txResp[i] = ToFullTransactionResponse(tx)
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		CashBalances:    balanceItems,
		CurrentMonthPnl: currentPnl,
		PendingReimbursements: pendingReimbursements{
			Count:       pendingReimb.Count,
			TotalAmount: pendingReimb.TotalAmount,
		},
		RecentTransactions: txResp,
	})
}
```

**Note:** Phase 1's `purchase.go` has a slimmer `transactionResponse` (9 fields, purchase-specific — no cash_account_id, outlet_id, reimbursement_batch_id). This dashboard handler defines `FullTransactionResponse` (13 fields, exported) as the canonical full-field version. Phase 4's ledger handler should reuse `FullTransactionResponse` and `ToFullTransactionResponse` instead of defining a duplicate.

**Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS.

**Step 5: Commit**

```bash
git add api/internal/accounting/handler/dashboard.go api/internal/accounting/handler/dashboard_test.go
git commit -m "feat(accounting): add dashboard handler with balances, P&L summary, pending reimbursements"
```

---

## Task 4: Routes + Types + Sidebar

**Files:**
- Modify: `api/internal/router/router.go`
- Modify: `admin/src/lib/types/api.ts`
- Modify: `admin/src/lib/components/Sidebar.svelte`

**Step 1: Add routes to router.go**

In `api/internal/router/router.go`, inside the existing accounting `r.Group` block (after reimbursement routes from Phase 2), add:

```go
// Reports
reportHandler := accthandler.NewReportHandler(queries)
r.Route("/accounting/reports", reportHandler.RegisterRoutes)

// Dashboard
dashboardHandler := accthandler.NewDashboardHandler(queries)
r.Get("/accounting/dashboard", dashboardHandler.Overview)
```

**Step 2: Add types to api.ts**

Append to `admin/src/lib/types/api.ts`:

```typescript
// ── Report types ────────────────────

export interface PnlSummaryRow {
	period: string;
	net_sales: string;
	cogs: string;
	gross_profit: string;
	expenses: string;
	net_profit: string;
	gross_margin: string;
	net_margin: string;
}

export interface ExpenseBreakdownRow {
	period: string;
	account_id: string;
	account_code: string;
	account_name: string;
	total: string;
}

export interface PnlResponse {
	summary: PnlSummaryRow[];
	expense_breakdown: ExpenseBreakdownRow[];
}

export interface CashFlowRow {
	period: string;
	cash_account_id: string;
	cash_account_name: string;
	cash_in: string;
	cash_out: string;
	net_cash_flow: string;
}

// ── Dashboard types ────────────────────

export interface CashBalance {
	cash_account_id: string;
	cash_account_code: string;
	cash_account_name: string;
	balance: string;
}

export interface MiniPnl {
	period: string;
	net_sales: string;
	cogs: string;
	gross_profit: string;
	expenses: string;
	net_profit: string;
}

export interface PendingReimbursements {
	count: number;
	total_amount: string;
}

export interface DashboardResponse {
	cash_balances: CashBalance[];
	current_month_pnl: MiniPnl;
	pending_reimbursements: PendingReimbursements;
	recent_transactions: AcctCashTransaction[];
}
```

**Step 3: Add sidebar items**

In `admin/src/lib/components/Sidebar.svelte`, update the `keuanganItems` array to include Ringkasan and Laporan:

```typescript
const keuanganItems: NavItem[] = [
    { label: 'Ringkasan', href: '/accounting', icon: '##', roles: ['OWNER'] },
    { label: 'Pembelian', href: '/accounting/purchases', icon: '##', roles: ['OWNER'] },
    { label: 'Reimburse', href: '/accounting/reimbursements', icon: '##', roles: ['OWNER'] },
    { label: 'Laporan', href: '/accounting/reports', icon: '##', roles: ['OWNER'] },
    { label: 'Master Data', href: '/accounting/master', icon: '##', roles: ['OWNER'] }
];
```

**Step 4: Verify compile**

Run: `cd api && go build ./...`

Expected: No errors.

**Step 5: Commit**

```bash
git add api/internal/router/router.go admin/src/lib/types/api.ts admin/src/lib/components/Sidebar.svelte
git commit -m "feat(accounting): wire report/dashboard routes, add types, update sidebar"
```

---

## Task 5: Admin Page — Laporan (Reports)

**Files:**
- Create: `admin/src/routes/(app)/accounting/reports/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/reports/+page.svelte`

**Step 1: Write server load**

Create `admin/src/routes/(app)/accounting/reports/+page.server.ts`:

```typescript
import { redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { PnlResponse, CashFlowRow } from '$lib/types/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');

	const accessToken = cookies.get('access_token')!;

	// Default date range: last 12 months
	const now = new Date();
	const startDate = url.searchParams.get('start_date')
		?? new Date(now.getFullYear() - 1, now.getMonth(), 1).toISOString().slice(0, 10);
	const endDate = url.searchParams.get('end_date')
		?? now.toISOString().slice(0, 10);
	const outletId = url.searchParams.get('outlet_id') ?? '';

	const queryParams = new URLSearchParams({ start_date: startDate, end_date: endDate });
	if (outletId) queryParams.set('outlet_id', outletId);

	const [pnlResult, cashFlowResult] = await Promise.all([
		apiRequest<PnlResponse>(`/accounting/reports/pnl?${queryParams}`, { accessToken }),
		apiRequest<CashFlowRow[]>(`/accounting/reports/cashflow?${queryParams}`, { accessToken })
	]);

	return {
		pnl: pnlResult.ok ? pnlResult.data : { summary: [], expense_breakdown: [] },
		cashFlow: cashFlowResult.ok ? cashFlowResult.data : [],
		startDate,
		endDate,
		outletId
	};
};
```

No form actions needed — reports page is read-only (no CRUD). Filters use URL search params for bookmarkability.

**Step 2: Write page component**

Create `admin/src/routes/(app)/accounting/reports/+page.svelte`:

Key features (follow existing accounting page patterns):

- **Two tabs**: Laba Rugi (P&L) and Arus Kas (Cash Flow) — use `$state` for active tab
- **Date range filter**: Start date + end date inputs with form submission via URL params (GET navigation)
- **Optional outlet filter**: Dropdown populated from outlets (if needed; can be deferred)
- **P&L tab**:
  - Summary table with monthly columns: Net Sales, COGS, Gross Profit, Expenses (total), Net Profit, margins
  - Below summary: Expense breakdown table showing per-account totals per month
  - Format money with `formatRupiah()`
- **Cash Flow tab**:
  - Table with monthly columns: per cash account rows showing Cash In, Cash Out, Net Cash Flow
  - Total row at bottom
- **CSV export button**: Client-side download for each tab

CSV export helper (define in `<script>`):
```typescript
function downloadCSV(headers: string[], rows: string[][], filename: string) {
    const csv = [headers.join(','), ...rows.map(r => r.join(','))].join('\n');
    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
}
```

Filter form (uses URL params, not SvelteKit form actions):
```svelte
<form method="GET" class="filter-bar">
    <input type="date" name="start_date" value={data.startDate} />
    <input type="date" name="end_date" value={data.endDate} />
    <button type="submit">Filter</button>
</form>
```

The implementing engineer should follow the established patterns from `purchases/+page.svelte` for styling (scoped CSS with CSS variables, card layouts, responsive grid) and from `master/+page.svelte` for the tab-switching pattern.

**Step 3: Verify it renders**

Run: `cd admin && pnpm dev` — navigate to `/accounting/reports`. Verify both tabs render.

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/reports/
git commit -m "feat(accounting): add reports admin page with P&L and cash flow tabs"
```

---

## Task 6: Admin Page — Ringkasan (Dashboard)

**Files:**
- Create: `admin/src/routes/(app)/accounting/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/+page.svelte`

**Step 1: Write server load**

Create `admin/src/routes/(app)/accounting/+page.server.ts`:

```typescript
import { redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { DashboardResponse } from '$lib/types/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');

	const accessToken = cookies.get('access_token')!;

	const result = await apiRequest<DashboardResponse>('/accounting/dashboard', { accessToken });

	return {
		dashboard: result.ok
			? result.data
			: {
					cash_balances: [],
					current_month_pnl: {
						period: '',
						net_sales: '0.00',
						cogs: '0.00',
						gross_profit: '0.00',
						expenses: '0.00',
						net_profit: '0.00'
					},
					pending_reimbursements: { count: 0, total_amount: '0.00' },
					recent_transactions: []
				}
	};
};
```

No form actions — dashboard is read-only.

**Step 2: Write page component**

Create `admin/src/routes/(app)/accounting/+page.svelte`:

Key features:

- **Cash balance cards**: Horizontal card row, one per cash account. Show account name + formatted balance. Color-code: green for positive, red for negative.
- **This month P&L mini-summary**: A compact card showing Net Sales, COGS, Gross Profit, Expenses, Net Profit with Rupiah formatting.
- **Pending reimbursements badge**: Card showing count and total amount. Links to `/accounting/reimbursements?status=Draft`.
- **Recent 10 transactions**: Simple table with date, code, description, amount, line_type. Clicking goes to `/accounting/transactions` (Phase 4, link can be dead for now).
- **Responsive layout**: Cards use CSS Grid with auto-fill for balance cards, 2-column for P&L + reimbursements.

Layout structure:
```svelte
<div class="dashboard">
    <h1>Ringkasan Keuangan</h1>

    <!-- Cash Balance Cards -->
    <section class="balance-cards">
        {#each data.dashboard.cash_balances as balance}
            <div class="balance-card">
                <span class="card-label">{balance.cash_account_name}</span>
                <span class="card-value">{formatRupiah(balance.balance)}</span>
            </div>
        {/each}
    </section>

    <!-- Summary Row -->
    <div class="summary-row">
        <!-- P&L Mini Card -->
        <section class="pnl-card">
            <h2>Laba Rugi Bulan Ini</h2>
            <!-- net_sales, cogs, gross_profit, expenses, net_profit -->
        </section>

        <!-- Pending Reimbursements Card -->
        <section class="reimb-card">
            <h2>Reimburse Pending</h2>
            <span class="reimb-count">{data.dashboard.pending_reimbursements.count} item</span>
            <span class="reimb-total">{formatRupiah(data.dashboard.pending_reimbursements.total_amount)}</span>
            <a href="/accounting/reimbursements?status=Draft">Lihat Detail</a>
        </section>
    </div>

    <!-- Recent Transactions -->
    <section class="recent-transactions">
        <h2>Transaksi Terakhir</h2>
        <!-- table -->
    </section>
</div>
```

The implementing engineer should follow the card styling from existing admin pages (radius, shadow, padding) and use `formatRupiah` from `$lib/utils/format`.

**Step 3: Verify it renders**

Run: `cd admin && pnpm dev` — navigate to `/accounting`. Verify dashboard shows cards and tables.

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/+page.server.ts admin/src/routes/\(app\)/accounting/+page.svelte
git commit -m "feat(accounting): add accounting dashboard page with balances, P&L summary, and recent transactions"
```

---

## Task 7: Verify Phase 3 Build + Tests

**Step 1: Run Go tests**

Run: `cd api && go test ./... -v`

Expected: All tests pass (existing Phase 1 + Phase 2 + new report + dashboard tests).

**Step 2: Run admin build**

Run: `cd admin && pnpm build`

Expected: No type errors. Build succeeds.

**Step 3: Verify API compiles**

Run: `cd api && go build ./cmd/server/`

Expected: Binary compiles clean.

**Step 4: Commit any fixes**

Fix any issues found, commit.

---

## Phase 3 Checklist

| # | Task | Delivers |
|---|------|----------|
| 1 | sqlc queries | P&L aggregation, expense breakdown, cash flow, cash balances, pending reimbursements |
| 2 | Report handler | `GET /accounting/reports/pnl` + `GET /accounting/reports/cashflow` with date/outlet filters |
| 3 | Dashboard handler | `GET /accounting/dashboard` with balances, mini P&L, pending reimbursements, recent transactions |
| 4 | Router + types + sidebar | Route wiring, TypeScript types, sidebar nav items (Ringkasan + Laporan) |
| 5 | Admin page — Laporan | Two-tab reports with monthly columns, expense breakdown, CSV export |
| 6 | Admin page — Ringkasan | Dashboard cards: cash balances, P&L summary, reimbursement badge, recent transactions |
| 7 | Verify build | Full test suite + build verification |

**NOT in Phase 3:** Sales entry, payroll, full ledger view, manual transaction entry. These are Phase 4.
