# Accounting Phase 3: Reports + Dashboard — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build P&L and Cash Flow report endpoints, a Laporan admin page (two tabs with CSV export), and a Ringkasan accounting dashboard.

**Architecture:** Reports are SQL aggregations over `acct_cash_transactions` — no report tables needed. The Go handler groups flat query rows into structured period-based responses. The dashboard calls 4 independent queries for cash balances, monthly P&L, pending reimbursements, and recent transactions.

**Tech Stack:** Go (Chi, sqlc, pgx/v5, shopspring/decimal), SvelteKit 2 (Svelte 5, Tailwind CSS 4), PostgreSQL 16.

---

## Conventions Reference

All patterns established in Phase 1. Key files:

| Pattern | Reference File |
|---------|---------------|
| sqlc query with optional filters | `api/queries/acct_cash_transactions.sql` (sqlc.narg pattern) |
| Handler + store interface | `api/internal/accounting/handler/purchase.go` |
| Mock store + httptest | `api/internal/accounting/handler/purchase_test.go` |
| pgtype.Numeric → string | `api/internal/accounting/handler/master.go:numericToStringPtr()` |
| Admin page server load | `admin/src/routes/(app)/accounting/purchases/+page.server.ts` |
| Router wiring | `api/internal/router/router.go:70-83` |
| TypeScript API types | `admin/src/lib/types/api.ts` |
| Sidebar nav items | `admin/src/lib/components/Sidebar.svelte:24-27` |
| Money formatting | `admin/src/lib/utils/format.ts:formatRupiah()` |
| Dashboard load pattern | `admin/src/routes/(app)/+page.server.ts` |

**Key conventions:**
- `::text AS alias` on COALESCE+aggregate for sqlc to infer `string` (not `interface{}`)
- Consumer-defines-interface: each handler file defines its own store interface
- Money: `shopspring/decimal` for math, `string` in JSON, `pgtype.Numeric` in DB
- Errors: 400 validation, 404 `pgx.ErrNoRows`, 500 internal
- Tests: `handler_test` package, mock stores, `httptest`, `chi.NewRouter()`
- Admin: `+page.server.ts` load + actions, `use:enhance`, Svelte 5 `$state()`/`$derived()`

---

## Task Overview

| # | Task | Files | Tests |
|---|------|-------|-------|
| 1 | sqlc Queries — Report & Dashboard | 1 query file, regenerate | — |
| 2 | Report Handler — P&L & Cash Flow | 2 Go files | 6+ tests |
| 3 | Dashboard Handler | 2 Go files | 3+ tests |
| 4 | Wire Routes | 1 Go file | — |
| 5 | Admin Types + Sidebar | 2 files | — |
| 6 | Admin Page — Laporan | 2 files | — |
| 7 | Admin Page — Ringkasan | 2 files | — |
| 8 | Build Verification | — | all tests |

---

## Task 1: sqlc Queries — Report & Dashboard

**Files:**
- Create: `api/queries/acct_reports.sql`
- Regenerate: `api/internal/database/` (run `sqlc generate`)

**Step 1: Write query file**

Create `api/queries/acct_reports.sql`:

```sql
-- name: GetProfitAndLossReport :many
-- Returns rows grouped by month, line type, and account for P&L computation.
-- Handler groups by period, sums SALES/COGS/EXPENSE, computes gross profit/margins.
SELECT
    date_trunc('month', ct.transaction_date)::date AS period,
    ct.line_type,
    ct.account_id,
    a.account_code,
    a.account_name,
    SUM(ct.amount)::text AS total_amount
FROM acct_cash_transactions ct
JOIN acct_accounts a ON a.id = ct.account_id
WHERE
    ct.line_type IN ('SALES', 'COGS', 'EXPENSE') AND
    (sqlc.narg('start_date')::date IS NULL OR ct.transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR ct.transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR ct.outlet_id = sqlc.narg('outlet_id'))
GROUP BY 1, 2, 3, 4, 5
ORDER BY 1, 2, 4;

-- name: GetCashFlowReport :many
-- Returns cash in/out per month per cash account for Cash Flow statement.
-- Cash In = SALES + CAPITAL; Cash Out = INVENTORY + EXPENSE + COGS + DRAWING.
SELECT
    date_trunc('month', ct.transaction_date)::date AS period,
    ca.id AS cash_account_id,
    ca.cash_account_code,
    ca.cash_account_name,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('SALES', 'CAPITAL') THEN ct.amount END), 0)::text AS cash_in,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('INVENTORY', 'EXPENSE', 'COGS', 'DRAWING') THEN ct.amount END), 0)::text AS cash_out
FROM acct_cash_transactions ct
JOIN acct_cash_accounts ca ON ca.id = ct.cash_account_id
WHERE
    ct.cash_account_id IS NOT NULL AND
    (sqlc.narg('start_date')::date IS NULL OR ct.transaction_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR ct.transaction_date <= sqlc.narg('end_date')) AND
    (sqlc.narg('outlet_id')::uuid IS NULL OR ct.outlet_id = sqlc.narg('outlet_id'))
GROUP BY 1, 2, 3, 4
ORDER BY 1, 3;

-- name: GetCashBalances :many
-- All-time net cash position per cash account (for dashboard cards).
SELECT
    ca.id AS cash_account_id,
    ca.cash_account_code,
    ca.cash_account_name,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('SALES', 'CAPITAL') THEN ct.amount END), 0)::text AS total_in,
    COALESCE(SUM(CASE WHEN ct.line_type IN ('INVENTORY', 'EXPENSE', 'COGS', 'DRAWING') THEN ct.amount END), 0)::text AS total_out
FROM acct_cash_transactions ct
JOIN acct_cash_accounts ca ON ca.id = ct.cash_account_id
WHERE ct.cash_account_id IS NOT NULL
GROUP BY 1, 2, 3
ORDER BY 2;

-- name: GetMonthlyPnlSummary :one
-- Current month P&L totals (for dashboard mini-summary).
SELECT
    COALESCE(SUM(CASE WHEN line_type = 'SALES' THEN amount END), 0)::text AS net_sales,
    COALESCE(SUM(CASE WHEN line_type = 'COGS' THEN amount END), 0)::text AS cogs,
    COALESCE(SUM(CASE WHEN line_type = 'EXPENSE' THEN amount END), 0)::text AS expenses
FROM acct_cash_transactions
WHERE transaction_date >= sqlc.arg('month_start') AND transaction_date < sqlc.arg('month_end');

-- name: GetPendingReimbursementsSummary :one
-- Count + total of Draft + Ready reimbursements (for dashboard badge).
SELECT
    COUNT(*) AS total_count,
    COALESCE(SUM(amount), 0)::text AS total_amount
FROM acct_reimbursement_requests
WHERE status IN ('Draft', 'Ready');
```

**Step 2: Run sqlc generate**

```bash
cd api && export PATH=$PATH:~/go/bin && sqlc generate
```

Expected: generates `api/internal/database/acct_reports.sql.go` with these functions:
- `GetProfitAndLossReport` → `[]GetProfitAndLossReportRow`
- `GetCashFlowReport` → `[]GetCashFlowReportRow`
- `GetCashBalances` → `[]GetCashBalancesRow`
- `GetMonthlyPnlSummary` → `GetMonthlyPnlSummaryRow`
- `GetPendingReimbursementsSummary` → `GetPendingReimbursementsSummaryRow`

Verify generated types:
- `GetProfitAndLossReportRow`: Period=`pgtype.Date`, LineType=`string`, AccountID=`uuid.UUID`, AccountCode=`string`, AccountName=`string`, TotalAmount=`string`
- `GetCashFlowReportRow`: Period=`pgtype.Date`, CashAccountID=`uuid.UUID`, CashAccountCode=`string`, CashAccountName=`string`, CashIn=`string`, CashOut=`string`
- `GetCashBalancesRow`: CashAccountID=`uuid.UUID`, CashAccountCode=`string`, CashAccountName=`string`, TotalIn=`string`, TotalOut=`string`
- `GetMonthlyPnlSummaryRow`: NetSales=`string`, Cogs=`string`, Expenses=`string`
- `GetPendingReimbursementsSummaryRow`: TotalCount=`int64`, TotalAmount=`string`

**Step 3: Verify compilation**

```bash
cd api && go build ./...
```

**Step 4: Commit**

```bash
git add api/queries/acct_reports.sql api/internal/database/
git commit -m "feat(accounting): add sqlc queries for P&L, cash flow, and dashboard reports"
```

---

## Task 2: Report Handler — P&L & Cash Flow

**Files:**
- Create: `api/internal/accounting/handler/report.go`
- Create: `api/internal/accounting/handler/report_test.go`
- Modify: `api/internal/accounting/handler/master.go` (add `numericToString` helper)

**Step 1: Add `numericToString` helper to `master.go`**

Add after the existing `numericToStringPtr` function at `master.go:177`:

```go
// numericToString converts pgtype.Numeric to string with 2 decimal places.
// Returns "0.00" for invalid/nil.
func numericToString(n pgtype.Numeric) string {
	s := numericToStringPtr(n)
	if s == nil {
		return "0.00"
	}
	return *s
}
```

**Step 2: Write the failing tests**

Create `api/internal/accounting/handler/report_test.go`:

```go
package handler_test

import (
	"encoding/json"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock store ---

type mockReportStore struct {
	pnlRows      []database.GetProfitAndLossReportRow
	cashFlowRows []database.GetCashFlowReportRow
	pnlErr       error
	cashFlowErr  error
}

func (m *mockReportStore) GetProfitAndLossReport(_ context.Context, _ database.GetProfitAndLossReportParams) ([]database.GetProfitAndLossReportRow, error) {
	return m.pnlRows, m.pnlErr
}

func (m *mockReportStore) GetCashFlowReport(_ context.Context, _ database.GetCashFlowReportParams) ([]database.GetCashFlowReportRow, error) {
	return m.cashFlowRows, m.cashFlowErr
}

func setupReportRouter(store handler.ReportStore) *chi.Mux {
	h := handler.NewReportHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/reports", h.RegisterRoutes)
	return r
}

func makePgDate(year int, month int, day int) pgtype.Date {
	return pgtype.Date{
		Time:  time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

// --- P&L Tests ---

func TestGetProfitAndLoss_Success(t *testing.T) {
	acctID1 := uuid.New()
	acctID2 := uuid.New()
	acctID3 := uuid.New()

	store := &mockReportStore{
		pnlRows: []database.GetProfitAndLossReportRow{
			{Period: makePgDate(2026, 1, 1), LineType: "SALES", AccountID: acctID1, AccountCode: "4000", AccountName: "Sales Revenue", TotalAmount: "5000000.00"},
			{Period: makePgDate(2026, 1, 1), LineType: "COGS", AccountID: acctID2, AccountCode: "5000", AccountName: "Cost of Goods", TotalAmount: "2000000.00"},
			{Period: makePgDate(2026, 1, 1), LineType: "EXPENSE", AccountID: acctID3, AccountCode: "6010", AccountName: "Payroll", TotalAmount: "1500000.00"},
		},
	}
	router := setupReportRouter(store)

	req := httptest.NewRequest("GET", "/accounting/reports/pnl", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Periods []struct {
			Period        string `json:"period"`
			NetSales      string `json:"net_sales"`
			COGS          string `json:"cogs"`
			GrossProfit   string `json:"gross_profit"`
			TotalExpenses string `json:"total_expenses"`
			NetProfit     string `json:"net_profit"`
			Expenses      []struct {
				AccountCode string `json:"account_code"`
				Amount      string `json:"amount"`
			} `json:"expenses"`
		} `json:"periods"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(resp.Periods))
	}
	p := resp.Periods[0]
	if p.Period != "2026-01" {
		t.Errorf("period: got %q, want %q", p.Period, "2026-01")
	}
	if p.NetSales != "5000000.00" {
		t.Errorf("net_sales: got %q", p.NetSales)
	}
	if p.GrossProfit != "3000000.00" {
		t.Errorf("gross_profit: got %q", p.GrossProfit)
	}
	if p.NetProfit != "1500000.00" {
		t.Errorf("net_profit: got %q", p.NetProfit)
	}
	if len(p.Expenses) != 1 {
		t.Fatalf("expected 1 expense, got %d", len(p.Expenses))
	}
}

func TestGetProfitAndLoss_Empty(t *testing.T) {
	store := &mockReportStore{pnlRows: []database.GetProfitAndLossReportRow{}}
	router := setupReportRouter(store)

	req := httptest.NewRequest("GET", "/accounting/reports/pnl", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Periods []interface{} `json:"periods"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Periods) != 0 {
		t.Errorf("expected 0 periods, got %d", len(resp.Periods))
	}
}

func TestGetProfitAndLoss_InvalidDate(t *testing.T) {
	store := &mockReportStore{}
	router := setupReportRouter(store)

	req := httptest.NewRequest("GET", "/accounting/reports/pnl?start_date=not-a-date", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Cash Flow Tests ---

func TestGetCashFlow_Success(t *testing.T) {
	caID := uuid.New()
	store := &mockReportStore{
		cashFlowRows: []database.GetCashFlowReportRow{
			{Period: makePgDate(2026, 1, 1), CashAccountID: caID, CashAccountCode: "CASH001", CashAccountName: "Kas Utama", CashIn: "5000000.00", CashOut: "4000000.00"},
		},
	}
	router := setupReportRouter(store)

	req := httptest.NewRequest("GET", "/accounting/reports/cashflow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Periods []struct {
			Period       string `json:"period"`
			TotalCashIn  string `json:"total_cash_in"`
			TotalCashOut string `json:"total_cash_out"`
			TotalNet     string `json:"total_net"`
			Accounts     []struct {
				CashAccountCode string `json:"cash_account_code"`
				CashIn          string `json:"cash_in"`
				CashOut         string `json:"cash_out"`
				Net             string `json:"net"`
			} `json:"accounts"`
		} `json:"periods"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(resp.Periods))
	}
	if resp.Periods[0].TotalNet != "1000000.00" {
		t.Errorf("total_net: got %q, want %q", resp.Periods[0].TotalNet, "1000000.00")
	}
}

func TestGetCashFlow_Empty(t *testing.T) {
	store := &mockReportStore{cashFlowRows: []database.GetCashFlowReportRow{}}
	router := setupReportRouter(store)

	req := httptest.NewRequest("GET", "/accounting/reports/cashflow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetCashFlow_WithDateFilter(t *testing.T) {
	store := &mockReportStore{cashFlowRows: []database.GetCashFlowReportRow{}}
	router := setupReportRouter(store)

	req := httptest.NewRequest("GET", "/accounting/reports/cashflow?start_date=2026-01-01&end_date=2026-06-30", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
```

**Step 3: Run tests to verify they fail**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestGet
```

Expected: compilation error — `handler.ReportStore` and `handler.NewReportHandler` don't exist yet.

**Step 4: Implement the report handler**

Create `api/internal/accounting/handler/report.go`:

```go
package handler

import (
	"context"
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

// ReportStore defines the database methods needed by report handlers.
type ReportStore interface {
	GetProfitAndLossReport(ctx context.Context, arg database.GetProfitAndLossReportParams) ([]database.GetProfitAndLossReportRow, error)
	GetCashFlowReport(ctx context.Context, arg database.GetCashFlowReportParams) ([]database.GetCashFlowReportRow, error)
}

// --- ReportHandler ---

// ReportHandler handles accounting report endpoints.
type ReportHandler struct {
	store ReportStore
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(store ReportStore) *ReportHandler {
	return &ReportHandler{store: store}
}

// RegisterRoutes registers report endpoints.
func (h *ReportHandler) RegisterRoutes(r chi.Router) {
	r.Get("/pnl", h.GetProfitAndLoss)
	r.Get("/cashflow", h.GetCashFlow)
}

// --- Response types ---

type pnlResponse struct {
	Periods []pnlPeriod `json:"periods"`
}

type pnlPeriod struct {
	Period         string       `json:"period"`
	NetSales       string       `json:"net_sales"`
	COGS           string       `json:"cogs"`
	GrossProfit    string       `json:"gross_profit"`
	Expenses       []expenseRow `json:"expenses"`
	TotalExpenses  string       `json:"total_expenses"`
	NetProfit      string       `json:"net_profit"`
	GrossMarginPct string       `json:"gross_margin_pct"`
	NetMarginPct   string       `json:"net_margin_pct"`
}

type expenseRow struct {
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	Amount      string `json:"amount"`
}

type cashFlowResponse struct {
	Periods []cashFlowPeriod `json:"periods"`
}

type cashFlowPeriod struct {
	Period       string            `json:"period"`
	Accounts     []cashFlowAccount `json:"accounts"`
	TotalCashIn  string            `json:"total_cash_in"`
	TotalCashOut string            `json:"total_cash_out"`
	TotalNet     string            `json:"total_net"`
}

type cashFlowAccount struct {
	CashAccountCode string `json:"cash_account_code"`
	CashAccountName string `json:"cash_account_name"`
	CashIn          string `json:"cash_in"`
	CashOut         string `json:"cash_out"`
	Net             string `json:"net"`
}

// --- Handlers ---

// GetProfitAndLoss returns P&L data grouped by month.
func (h *ReportHandler) GetProfitAndLoss(w http.ResponseWriter, r *http.Request) {
	startDate, err := parseDateParam(r, "start_date")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date format, expected YYYY-MM-DD"})
		return
	}
	endDate, err := parseDateParam(r, "end_date")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date format, expected YYYY-MM-DD"})
		return
	}
	outletID, err := parseOptionalUUIDParam(r, "outlet_id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
		return
	}

	rows, err := h.store.GetProfitAndLossReport(r.Context(), database.GetProfitAndLossReportParams{
		StartDate: startDate,
		EndDate:   endDate,
		OutletID:  outletID,
	})
	if err != nil {
		log.Printf("ERROR: get P&L report: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, buildPnlResponse(rows))
}

// GetCashFlow returns cash flow data grouped by month and cash account.
func (h *ReportHandler) GetCashFlow(w http.ResponseWriter, r *http.Request) {
	startDate, err := parseDateParam(r, "start_date")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date format, expected YYYY-MM-DD"})
		return
	}
	endDate, err := parseDateParam(r, "end_date")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date format, expected YYYY-MM-DD"})
		return
	}
	outletID, err := parseOptionalUUIDParam(r, "outlet_id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
		return
	}

	rows, err := h.store.GetCashFlowReport(r.Context(), database.GetCashFlowReportParams{
		StartDate: startDate,
		EndDate:   endDate,
		OutletID:  outletID,
	})
	if err != nil {
		log.Printf("ERROR: get cash flow report: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, buildCashFlowResponse(rows))
}

// --- Response builders ---

func buildPnlResponse(rows []database.GetProfitAndLossReportRow) pnlResponse {
	// Group rows by period
	type periodData struct {
		sales    decimal.Decimal
		cogs     decimal.Decimal
		expenses []expenseRow
		totalExp decimal.Decimal
	}
	periodMap := make(map[string]*periodData)
	var periodOrder []string

	for _, row := range rows {
		period := row.Period.Time.Format("2006-01")
		pd, ok := periodMap[period]
		if !ok {
			pd = &periodData{}
			periodMap[period] = pd
			periodOrder = append(periodOrder, period)
		}

		amount, _ := decimal.NewFromString(row.TotalAmount)

		switch row.LineType {
		case "SALES":
			pd.sales = pd.sales.Add(amount)
		case "COGS":
			pd.cogs = pd.cogs.Add(amount)
		case "EXPENSE":
			pd.expenses = append(pd.expenses, expenseRow{
				AccountCode: row.AccountCode,
				AccountName: row.AccountName,
				Amount:      amount.StringFixed(2),
			})
			pd.totalExp = pd.totalExp.Add(amount)
		}
	}

	// Build response
	resp := pnlResponse{Periods: make([]pnlPeriod, 0, len(periodOrder))}
	for _, period := range periodOrder {
		pd := periodMap[period]
		grossProfit := pd.sales.Sub(pd.cogs)
		netProfit := grossProfit.Sub(pd.totalExp)

		expenses := pd.expenses
		if expenses == nil {
			expenses = []expenseRow{}
		}

		resp.Periods = append(resp.Periods, pnlPeriod{
			Period:         period,
			NetSales:       pd.sales.StringFixed(2),
			COGS:           pd.cogs.StringFixed(2),
			GrossProfit:    grossProfit.StringFixed(2),
			Expenses:       expenses,
			TotalExpenses:  pd.totalExp.StringFixed(2),
			NetProfit:      netProfit.StringFixed(2),
			GrossMarginPct: calcMarginPct(grossProfit, pd.sales),
			NetMarginPct:   calcMarginPct(netProfit, pd.sales),
		})
	}

	return resp
}

func buildCashFlowResponse(rows []database.GetCashFlowReportRow) cashFlowResponse {
	type periodData struct {
		accounts []cashFlowAccount
		totalIn  decimal.Decimal
		totalOut decimal.Decimal
	}
	periodMap := make(map[string]*periodData)
	var periodOrder []string

	for _, row := range rows {
		period := row.Period.Time.Format("2006-01")
		pd, ok := periodMap[period]
		if !ok {
			pd = &periodData{}
			periodMap[period] = pd
			periodOrder = append(periodOrder, period)
		}

		cashIn, _ := decimal.NewFromString(row.CashIn)
		cashOut, _ := decimal.NewFromString(row.CashOut)
		net := cashIn.Sub(cashOut)

		pd.accounts = append(pd.accounts, cashFlowAccount{
			CashAccountCode: row.CashAccountCode,
			CashAccountName: row.CashAccountName,
			CashIn:          cashIn.StringFixed(2),
			CashOut:         cashOut.StringFixed(2),
			Net:             net.StringFixed(2),
		})
		pd.totalIn = pd.totalIn.Add(cashIn)
		pd.totalOut = pd.totalOut.Add(cashOut)
	}

	resp := cashFlowResponse{Periods: make([]cashFlowPeriod, 0, len(periodOrder))}
	for _, period := range periodOrder {
		pd := periodMap[period]
		resp.Periods = append(resp.Periods, cashFlowPeriod{
			Period:       period,
			Accounts:     pd.accounts,
			TotalCashIn:  pd.totalIn.StringFixed(2),
			TotalCashOut: pd.totalOut.StringFixed(2),
			TotalNet:     pd.totalIn.Sub(pd.totalOut).StringFixed(2),
		})
	}

	return resp
}

// --- Helpers ---

func parseDateParam(r *http.Request, name string) (pgtype.Date, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return pgtype.Date{}, nil // Valid=false → no filter
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func parseOptionalUUIDParam(r *http.Request, name string) (pgtype.UUID, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return pgtype.UUID{}, nil // Valid=false → no filter
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func calcMarginPct(numerator, denominator decimal.Decimal) string {
	if denominator.IsZero() {
		return "0.00"
	}
	return numerator.Div(denominator).Mul(decimal.NewFromInt(100)).StringFixed(2)
}
```

**Step 5: Run tests to verify they pass**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestGet
```

Expected: all 6 tests pass.

**Step 6: Commit**

```bash
git add api/internal/accounting/handler/report.go api/internal/accounting/handler/report_test.go api/internal/accounting/handler/master.go
git commit -m "feat(accounting): add P&L and cash flow report handlers with tests"
```

---

## Task 3: Dashboard Handler

**Files:**
- Create: `api/internal/accounting/handler/dashboard.go`
- Create: `api/internal/accounting/handler/dashboard_test.go`

**Step 1: Write the failing tests**

Create `api/internal/accounting/handler/dashboard_test.go`:

```go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock store ---

type mockDashboardStore struct {
	cashBalances []database.GetCashBalancesRow
	monthlyPnl   database.GetMonthlyPnlSummaryRow
	pendingReimb database.GetPendingReimbursementsSummaryRow
	recentTxns   []database.AcctCashTransaction

	cashBalancesErr error
	monthlyPnlErr   error
	pendingReimbErr error
	recentTxnsErr   error
}

func (m *mockDashboardStore) GetCashBalances(_ context.Context) ([]database.GetCashBalancesRow, error) {
	return m.cashBalances, m.cashBalancesErr
}

func (m *mockDashboardStore) GetMonthlyPnlSummary(_ context.Context, _ database.GetMonthlyPnlSummaryParams) (database.GetMonthlyPnlSummaryRow, error) {
	return m.monthlyPnl, m.monthlyPnlErr
}

func (m *mockDashboardStore) GetPendingReimbursementsSummary(_ context.Context) (database.GetPendingReimbursementsSummaryRow, error) {
	return m.pendingReimb, m.pendingReimbErr
}

func (m *mockDashboardStore) ListAcctCashTransactions(_ context.Context, _ database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error) {
	return m.recentTxns, m.recentTxnsErr
}

func setupDashboardRouter(store handler.DashboardStore) *chi.Mux {
	h := handler.NewDashboardHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/dashboard", h.RegisterRoutes)
	return r
}

// --- Tests ---

func TestGetDashboard_Success(t *testing.T) {
	txID := uuid.New()
	acctID := uuid.New()

	store := &mockDashboardStore{
		cashBalances: []database.GetCashBalancesRow{
			{CashAccountID: uuid.New(), CashAccountCode: "CASH001", CashAccountName: "Kas Utama", TotalIn: "10000000.00", TotalOut: "7000000.00"},
		},
		monthlyPnl: database.GetMonthlyPnlSummaryRow{
			NetSales: "5000000.00",
			Cogs:     "2000000.00",
			Expenses: "1500000.00",
		},
		pendingReimb: database.GetPendingReimbursementsSummaryRow{
			TotalCount:  3,
			TotalAmount: "450000.00",
		},
		recentTxns: []database.AcctCashTransaction{
			{
				ID:              txID,
				TransactionCode: "PCS000001",
				TransactionDate: makePgDate(2026, 1, 20),
				Description:     "Cabe Merah 5kg",
				Amount:          makePgNumeric("500000.00"),
				LineType:        "INVENTORY",
				AccountID:       acctID,
				CreatedAt:       time.Now(),
			},
		},
	}
	router := setupDashboardRouter(store)

	req := httptest.NewRequest("GET", "/accounting/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		CashBalances []struct {
			CashAccountCode string `json:"cash_account_code"`
			Balance         string `json:"balance"`
		} `json:"cash_balances"`
		MonthlyPnl struct {
			NetSales string `json:"net_sales"`
			NetProfit string `json:"net_profit"`
		} `json:"monthly_pnl"`
		PendingReimbursements struct {
			Count       int64  `json:"count"`
			TotalAmount string `json:"total_amount"`
		} `json:"pending_reimbursements"`
		RecentTransactions []struct {
			TransactionCode string `json:"transaction_code"`
		} `json:"recent_transactions"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.CashBalances) != 1 {
		t.Fatalf("expected 1 cash balance, got %d", len(resp.CashBalances))
	}
	if resp.CashBalances[0].Balance != "3000000.00" {
		t.Errorf("balance: got %q, want %q", resp.CashBalances[0].Balance, "3000000.00")
	}
	if resp.PendingReimbursements.Count != 3 {
		t.Errorf("pending count: got %d, want 3", resp.PendingReimbursements.Count)
	}
	if len(resp.RecentTransactions) != 1 {
		t.Fatalf("expected 1 recent tx, got %d", len(resp.RecentTransactions))
	}
}

func TestGetDashboard_Empty(t *testing.T) {
	store := &mockDashboardStore{
		cashBalances: []database.GetCashBalancesRow{},
		monthlyPnl:   database.GetMonthlyPnlSummaryRow{NetSales: "0", Cogs: "0", Expenses: "0"},
		pendingReimb: database.GetPendingReimbursementsSummaryRow{TotalCount: 0, TotalAmount: "0"},
		recentTxns:   []database.AcctCashTransaction{},
	}
	router := setupDashboardRouter(store)

	req := httptest.NewRequest("GET", "/accounting/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetDashboard_StoreError(t *testing.T) {
	store := &mockDashboardStore{
		cashBalancesErr: fmt.Errorf("db error"),
	}
	router := setupDashboardRouter(store)

	req := httptest.NewRequest("GET", "/accounting/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- Helper ---

func makePgNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	n.Scan(s)
	return n
}
```

**Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestGetDashboard
```

Expected: compilation error — `handler.DashboardStore` and `handler.NewDashboardHandler` don't exist.

**Step 3: Implement the dashboard handler**

Create `api/internal/accounting/handler/dashboard.go`:

```go
package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

// DashboardStore defines the database methods needed by the dashboard handler.
type DashboardStore interface {
	GetCashBalances(ctx context.Context) ([]database.GetCashBalancesRow, error)
	GetMonthlyPnlSummary(ctx context.Context, arg database.GetMonthlyPnlSummaryParams) (database.GetMonthlyPnlSummaryRow, error)
	GetPendingReimbursementsSummary(ctx context.Context) (database.GetPendingReimbursementsSummaryRow, error)
	ListAcctCashTransactions(ctx context.Context, arg database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error)
}

// --- DashboardHandler ---

// DashboardHandler handles the accounting dashboard endpoint.
type DashboardHandler struct {
	store DashboardStore
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(store DashboardStore) *DashboardHandler {
	return &DashboardHandler{store: store}
}

// RegisterRoutes registers dashboard endpoints.
func (h *DashboardHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.GetDashboard)
}

// --- Response types ---

type dashboardResponse struct {
	CashBalances          []cashBalanceResponse        `json:"cash_balances"`
	MonthlyPnl            monthlyPnlResponse           `json:"monthly_pnl"`
	PendingReimbursements pendingReimbursementsResponse `json:"pending_reimbursements"`
	RecentTransactions    []recentTxResponse            `json:"recent_transactions"`
}

type cashBalanceResponse struct {
	CashAccountCode string `json:"cash_account_code"`
	CashAccountName string `json:"cash_account_name"`
	Balance         string `json:"balance"`
}

type monthlyPnlResponse struct {
	Period        string `json:"period"`
	NetSales      string `json:"net_sales"`
	COGS          string `json:"cogs"`
	GrossProfit   string `json:"gross_profit"`
	TotalExpenses string `json:"total_expenses"`
	NetProfit     string `json:"net_profit"`
}

type pendingReimbursementsResponse struct {
	Count       int64  `json:"count"`
	TotalAmount string `json:"total_amount"`
}

type recentTxResponse struct {
	ID              string    `json:"id"`
	TransactionCode string    `json:"transaction_code"`
	TransactionDate string    `json:"transaction_date"`
	Description     string    `json:"description"`
	Amount          string    `json:"amount"`
	LineType        string    `json:"line_type"`
	CreatedAt       time.Time `json:"created_at"`
}

// --- Handler ---

// GetDashboard returns the accounting overview (cash balances, monthly P&L, pending reimbursements, recent transactions).
func (h *DashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Determine current month (Asia/Jakarta)
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(jakarta)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	// 1. Cash balances
	balances, err := h.store.GetCashBalances(ctx)
	if err != nil {
		log.Printf("ERROR: get cash balances: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// 2. Monthly P&L
	pnl, err := h.store.GetMonthlyPnlSummary(ctx, database.GetMonthlyPnlSummaryParams{
		MonthStart: pgtype.Date{Time: monthStart, Valid: true},
		MonthEnd:   pgtype.Date{Time: monthEnd, Valid: true},
	})
	if err != nil {
		log.Printf("ERROR: get monthly P&L: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// 3. Pending reimbursements
	reimb, err := h.store.GetPendingReimbursementsSummary(ctx)
	if err != nil {
		log.Printf("ERROR: get pending reimbursements: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// 4. Recent 10 transactions
	txns, err := h.store.ListAcctCashTransactions(ctx, database.ListAcctCashTransactionsParams{
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		log.Printf("ERROR: list recent transactions: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Build response
	cashBalances := make([]cashBalanceResponse, len(balances))
	for i, b := range balances {
		totalIn, _ := decimal.NewFromString(b.TotalIn)
		totalOut, _ := decimal.NewFromString(b.TotalOut)
		balance := totalIn.Sub(totalOut)
		cashBalances[i] = cashBalanceResponse{
			CashAccountCode: b.CashAccountCode,
			CashAccountName: b.CashAccountName,
			Balance:         balance.StringFixed(2),
		}
	}

	netSales, _ := decimal.NewFromString(pnl.NetSales)
	cogs, _ := decimal.NewFromString(pnl.Cogs)
	expenses, _ := decimal.NewFromString(pnl.Expenses)
	grossProfit := netSales.Sub(cogs)
	netProfit := grossProfit.Sub(expenses)

	recentTxs := make([]recentTxResponse, len(txns))
	for i, tx := range txns {
		recentTxs[i] = recentTxResponse{
			ID:              tx.ID.String(),
			TransactionCode: tx.TransactionCode,
			TransactionDate: tx.TransactionDate.Time.Format("2006-01-02"),
			Description:     tx.Description,
			Amount:          numericToString(tx.Amount),
			LineType:        tx.LineType,
			CreatedAt:       tx.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		CashBalances: cashBalances,
		MonthlyPnl: monthlyPnlResponse{
			Period:        now.Format("2006-01"),
			NetSales:      netSales.StringFixed(2),
			COGS:          cogs.StringFixed(2),
			GrossProfit:   grossProfit.StringFixed(2),
			TotalExpenses: expenses.StringFixed(2),
			NetProfit:     netProfit.StringFixed(2),
		},
		PendingReimbursements: pendingReimbursementsResponse{
			Count:       reimb.TotalCount,
			TotalAmount: reimb.TotalAmount,
		},
		RecentTransactions: recentTxs,
	})
}
```

**Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestGetDashboard
```

Expected: all 3 tests pass.

**Step 5: Commit**

```bash
git add api/internal/accounting/handler/dashboard.go api/internal/accounting/handler/dashboard_test.go
git commit -m "feat(accounting): add dashboard handler with cash balances, P&L summary, and recent transactions"
```

---

## Task 4: Wire Routes

**Files:**
- Modify: `api/internal/router/router.go` (lines 70-83)

**Step 1: Add report and dashboard routes**

In `router.go`, inside the existing accounting `r.Group` block (after the purchases route at line 82), add:

```go
			// Reports
			reportHandler := accthandler.NewReportHandler(queries)
			r.Route("/accounting/reports", reportHandler.RegisterRoutes)

			// Dashboard
			dashboardHandler := accthandler.NewDashboardHandler(queries)
			r.Route("/accounting/dashboard", dashboardHandler.RegisterRoutes)
```

**Step 2: Verify compilation**

```bash
cd api && go build ./...
```

**Step 3: Commit**

```bash
git add api/internal/router/router.go
git commit -m "feat(accounting): wire report and dashboard routes"
```

---

## Task 5: Admin Types + Sidebar

**Files:**
- Modify: `admin/src/lib/types/api.ts` (append new types)
- Modify: `admin/src/lib/components/Sidebar.svelte` (add nav items)

**Step 1: Add TypeScript types**

Append to `admin/src/lib/types/api.ts` after the `AcctCashTransaction` interface:

```typescript
// --- Accounting Report Types ---

export interface PnlPeriod {
	period: string;
	net_sales: string;
	cogs: string;
	gross_profit: string;
	expenses: PnlExpenseRow[];
	total_expenses: string;
	net_profit: string;
	gross_margin_pct: string;
	net_margin_pct: string;
}

export interface PnlExpenseRow {
	account_code: string;
	account_name: string;
	amount: string;
}

export interface PnlResponse {
	periods: PnlPeriod[];
}

export interface CashFlowAccount {
	cash_account_code: string;
	cash_account_name: string;
	cash_in: string;
	cash_out: string;
	net: string;
}

export interface CashFlowPeriod {
	period: string;
	accounts: CashFlowAccount[];
	total_cash_in: string;
	total_cash_out: string;
	total_net: string;
}

export interface CashFlowResponse {
	periods: CashFlowPeriod[];
}

export interface CashBalance {
	cash_account_code: string;
	cash_account_name: string;
	balance: string;
}

export interface MonthlyPnlSummary {
	period: string;
	net_sales: string;
	cogs: string;
	gross_profit: string;
	total_expenses: string;
	net_profit: string;
}

export interface PendingReimbursements {
	count: number;
	total_amount: string;
}

export interface RecentTransaction {
	id: string;
	transaction_code: string;
	transaction_date: string;
	description: string;
	amount: string;
	line_type: string;
	created_at: string;
}

export interface DashboardResponse {
	cash_balances: CashBalance[];
	monthly_pnl: MonthlyPnlSummary;
	pending_reimbursements: PendingReimbursements;
	recent_transactions: RecentTransaction[];
}
```

**Step 2: Update sidebar**

In `admin/src/lib/components/Sidebar.svelte`, update the `keuanganItems` array (line 24-27) to add Ringkasan and Laporan:

```typescript
const keuanganItems: NavItem[] = [
	{ label: 'Ringkasan', href: '/accounting', icon: '##', roles: ['OWNER'] },
	{ label: 'Pembelian', href: '/accounting/purchases', icon: '##', roles: ['OWNER'] },
	{ label: 'Laporan', href: '/accounting/reports', icon: '##', roles: ['OWNER'] },
	{ label: 'Master Data', href: '/accounting/master', icon: '##', roles: ['OWNER'] }
];
```

**Important:** The `isActive` function uses `startsWith(href)`. Since `/accounting` would match all `/accounting/*` routes, we need to update the Ringkasan check. Modify the `isActive` function:

```typescript
function isActive(href: string): boolean {
	if (href === '/' || href === '/accounting') return page.url.pathname === href;
	return page.url.pathname.startsWith(href);
}
```

**Step 3: Commit**

```bash
git add admin/src/lib/types/api.ts admin/src/lib/components/Sidebar.svelte
git commit -m "feat(accounting): add report/dashboard types and sidebar nav items"
```

---

## Task 6: Admin Page — Laporan

**Files:**
- Create: `admin/src/routes/(app)/accounting/reports/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/reports/+page.svelte`

**Step 1: Create server load**

Create `admin/src/routes/(app)/accounting/reports/+page.server.ts`:

```typescript
import { redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { PnlResponse, CashFlowResponse } from '$lib/types/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;
	const startDate = url.searchParams.get('start_date') || '';
	const endDate = url.searchParams.get('end_date') || '';

	// Build query string
	const params = new URLSearchParams();
	if (startDate) params.set('start_date', startDate);
	if (endDate) params.set('end_date', endDate);
	const qs = params.toString() ? `?${params.toString()}` : '';

	// Load both P&L and Cash Flow in parallel
	const [pnlResult, cashFlowResult] = await Promise.all([
		apiRequest<PnlResponse>(`/accounting/reports/pnl${qs}`, { accessToken }),
		apiRequest<CashFlowResponse>(`/accounting/reports/cashflow${qs}`, { accessToken })
	]);

	return {
		pnl: pnlResult.ok ? pnlResult.data : { periods: [] },
		cashFlow: cashFlowResult.ok ? cashFlowResult.data : { periods: [] },
		startDate,
		endDate
	};
};
```

**Step 2: Create page component**

Create `admin/src/routes/(app)/accounting/reports/+page.svelte`:

The page has:
- Date range inputs with "Terapkan" (Apply) button
- Two tabs: "Laba Rugi" (P&L) and "Arus Kas" (Cash Flow)
- P&L tab: pivot table with months as columns, line items as rows
- Cash Flow tab: cash in/out by account per month
- CSV export button per tab

Key implementation details:
- Use `$state()` for active tab
- Use `$derived()` to extract unique months from report data
- Date range form submits as GET (same page with query params)
- CSS follows existing design tokens from `app.css`
- `formatRupiah()` from `$lib/utils/format` for monetary display
- CSV export uses `escapeCsvField` pattern from existing reports page
- P&L table: rows = [Pendapatan Bersih, HPP, Laba Kotor, ...expense accounts..., Total Beban, Laba Bersih, Margin Kotor %, Margin Bersih %]
- Cash Flow table: per cash account rows with cash in, cash out, net; totals row
- Mobile responsive: horizontal scroll for tables with many month columns

The page component should be approximately 400-500 lines including styles.

**Step 3: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/reports/
git commit -m "feat(accounting): add Laporan page with P&L and Cash Flow tabs and CSV export"
```

---

## Task 7: Admin Page — Ringkasan (Dashboard)

**Files:**
- Create: `admin/src/routes/(app)/accounting/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/+page.svelte`

**Step 1: Create server load**

Create `admin/src/routes/(app)/accounting/+page.server.ts`:

```typescript
import { redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type { DashboardResponse } from '$lib/types/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals, cookies }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;
	const result = await apiRequest<DashboardResponse>('/accounting/dashboard', { accessToken });

	return {
		dashboard: result.ok
			? result.data
			: {
					cash_balances: [],
					monthly_pnl: {
						period: '',
						net_sales: '0',
						cogs: '0',
						gross_profit: '0',
						total_expenses: '0',
						net_profit: '0'
					},
					pending_reimbursements: { count: 0, total_amount: '0' },
					recent_transactions: []
				}
	};
};
```

**Step 2: Create page component**

Create `admin/src/routes/(app)/accounting/+page.svelte`:

The page displays:
1. **Cash balance cards** — grid of cards, one per cash account, showing account name + formatted balance
2. **Monthly P&L summary card** — shows Net Sales, HPP, Laba Kotor, Beban, Laba Bersih for current month
3. **Pending reimbursements card** — count badge + total amount, links to `/accounting/reimbursements`
4. **Recent 10 transactions table** — code, date, description, amount, line type badge

Key implementation details:
- Use `formatRupiah()` for all monetary values
- Line type badges use color coding (SALES=green, INVENTORY=blue, EXPENSE=red, etc.)
- CSS follows existing dashboard card patterns from POS dashboard
- No form actions needed (read-only page)
- Mobile responsive grid

The page component should be approximately 250-350 lines including styles.

**Step 3: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/+page.server.ts admin/src/routes/\(app\)/accounting/+page.svelte
git commit -m "feat(accounting): add Ringkasan accounting dashboard page"
```

---

## Task 8: Build Verification

**Step 1: Run all Go tests**

```bash
cd api && go test ./... -v
```

Expected: all tests pass (existing 435 unit tests + new report/dashboard tests).

**Step 2: Build admin**

```bash
cd admin && pnpm build
```

Expected: builds successfully (warnings-only acceptable for pre-existing a11y labels).

**Step 3: Build API binary**

```bash
cd api && go build ./cmd/server/
```

Expected: compiles clean.

**Step 4: Commit (if any fixes needed)**

Only if issues found during verification.

---

## Verification

After implementation, manually verify:

1. **API endpoints** (with test creds):
   - `GET /accounting/reports/pnl` — returns P&L with periods array
   - `GET /accounting/reports/pnl?start_date=2026-01-01&end_date=2026-06-30` — filtered
   - `GET /accounting/reports/cashflow` — returns cash flow with periods array
   - `GET /accounting/dashboard` — returns all 4 dashboard sections

2. **Admin pages** (run `pnpm dev`):
   - `/accounting` — Ringkasan dashboard with cash balances, P&L summary, pending reimbursements, recent transactions
   - `/accounting/reports` — two tabs, date range filter, CSV export
   - Sidebar shows: Ringkasan, Pembelian, Laporan, Master Data under KEUANGAN

3. **CSV export**: Click export on Laporan page, verify CSV downloads with correct data
