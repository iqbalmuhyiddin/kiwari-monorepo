package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
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

type mockDashboardStore struct {
	cashBalances      []database.GetCashBalancesRow
	pnlSummary        database.GetMonthlyPnlSummaryRow
	pendingSummary    database.GetPendingReimbursementsSummaryRow
	recentTxs         []database.AcctCashTransaction
	cashBalancesErr   error
	pnlSummaryErr     error
	pendingSummaryErr error
	recentTxsErr      error
}

func (m *mockDashboardStore) GetCashBalances(_ context.Context) ([]database.GetCashBalancesRow, error) {
	return m.cashBalances, m.cashBalancesErr
}

func (m *mockDashboardStore) GetMonthlyPnlSummary(_ context.Context, _ database.GetMonthlyPnlSummaryParams) (database.GetMonthlyPnlSummaryRow, error) {
	return m.pnlSummary, m.pnlSummaryErr
}

func (m *mockDashboardStore) GetPendingReimbursementsSummary(_ context.Context) (database.GetPendingReimbursementsSummaryRow, error) {
	return m.pendingSummary, m.pendingSummaryErr
}

func (m *mockDashboardStore) ListAcctCashTransactions(_ context.Context, _ database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error) {
	return m.recentTxs, m.recentTxsErr
}

func setupDashboardRouter(store handler.DashboardStore) *chi.Mux {
	h := handler.NewDashboardHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/dashboard", h.RegisterRoutes)
	return r
}

func makePgNumeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	n.Scan(s)
	return n
}

// --- Tests ---

func TestGetDashboard_Success(t *testing.T) {
	cashAccountID1 := uuid.New()
	cashAccountID2 := uuid.New()
	txID1 := uuid.New()
	txID2 := uuid.New()

	store := &mockDashboardStore{
		cashBalances: []database.GetCashBalancesRow{
			{
				CashAccountID:   cashAccountID1,
				CashAccountCode: "CA001",
				CashAccountName: "Cash Drawer Outlet A",
				TotalIn:         "10000000.00", // 10M
				TotalOut:        "7000000.00",  // 7M
				// Expected balance: 3M
			},
			{
				CashAccountID:   cashAccountID2,
				CashAccountCode: "CA002",
				CashAccountName: "Bank BCA",
				TotalIn:         "5000000.00", // 5M
				TotalOut:        "2000000.00", // 2M
				// Expected balance: 3M
			},
		},
		pnlSummary: database.GetMonthlyPnlSummaryRow{
			NetSales: "15000000.00",
			Cogs:     "6000000.00",
			Expenses: "3000000.00",
			// Expected gross profit: 9M
			// Expected net profit: 6M
		},
		pendingSummary: database.GetPendingReimbursementsSummaryRow{
			TotalCount:  5,
			TotalAmount: "2500000.00",
		},
		recentTxs: []database.AcctCashTransaction{
			{
				ID:              txID1,
				TransactionCode: "TX001",
				TransactionDate: makePgDate(2026, 2, 12),
				Description:     "Sales revenue",
				Amount:          makePgNumeric("1500000.00"),
				LineType:        "SALES",
			},
			{
				ID:              txID2,
				TransactionCode: "TX002",
				TransactionDate: makePgDate(2026, 2, 11),
				Description:     "Ingredient purchase",
				Amount:          makePgNumeric("500000.00"),
				LineType:        "EXPENSE",
			},
		},
	}

	router := setupDashboardRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/accounting/dashboard/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify cash balances
	cashBalances := resp["cash_balances"].([]interface{})
	if len(cashBalances) != 2 {
		t.Errorf("expected 2 cash balances, got %d", len(cashBalances))
	}
	bal1 := cashBalances[0].(map[string]interface{})
	if bal1["balance"] != "3000000.00" {
		t.Errorf("expected balance 3000000.00 for cash account 1, got %s", bal1["balance"])
	}
	if bal1["cash_account_code"] != "CA001" {
		t.Errorf("expected code CA001, got %s", bal1["cash_account_code"])
	}

	bal2 := cashBalances[1].(map[string]interface{})
	if bal2["balance"] != "3000000.00" {
		t.Errorf("expected balance 3000000.00 for cash account 2, got %s", bal2["balance"])
	}

	// Verify monthly P&L
	monthlyPnl := resp["monthly_pnl"].(map[string]interface{})
	if monthlyPnl["period"] == nil || monthlyPnl["period"] == "" {
		t.Errorf("expected period to be set, got %v", monthlyPnl["period"])
	}
	if monthlyPnl["net_sales"] != "15000000.00" {
		t.Errorf("expected net_sales 15000000.00, got %s", monthlyPnl["net_sales"])
	}
	if monthlyPnl["cogs"] != "6000000.00" {
		t.Errorf("expected cogs 6000000.00, got %s", monthlyPnl["cogs"])
	}
	if monthlyPnl["gross_profit"] != "9000000.00" {
		t.Errorf("expected gross_profit 9000000.00, got %s", monthlyPnl["gross_profit"])
	}
	if monthlyPnl["total_expenses"] != "3000000.00" {
		t.Errorf("expected total_expenses 3000000.00, got %s", monthlyPnl["total_expenses"])
	}
	if monthlyPnl["net_profit"] != "6000000.00" {
		t.Errorf("expected net_profit 6000000.00, got %s", monthlyPnl["net_profit"])
	}

	// Verify pending reimbursements
	pendingReimb := resp["pending_reimbursements"].(map[string]interface{})
	if pendingReimb["count"].(float64) != 5 {
		t.Errorf("expected count 5, got %v", pendingReimb["count"])
	}
	if pendingReimb["total_amount"] != "2500000.00" {
		t.Errorf("expected total_amount 2500000.00, got %s", pendingReimb["total_amount"])
	}

	// Verify recent transactions
	recentTxs := resp["recent_transactions"].([]interface{})
	if len(recentTxs) != 2 {
		t.Errorf("expected 2 recent transactions, got %d", len(recentTxs))
	}
	tx1 := recentTxs[0].(map[string]interface{})
	if tx1["transaction_code"] != "TX001" {
		t.Errorf("expected transaction_code TX001, got %s", tx1["transaction_code"])
	}
	if tx1["amount"] != "1500000.00" {
		t.Errorf("expected amount 1500000.00, got %s", tx1["amount"])
	}
	if tx1["line_type"] != "SALES" {
		t.Errorf("expected line_type SALES, got %s", tx1["line_type"])
	}
}

func TestGetDashboard_Empty(t *testing.T) {
	store := &mockDashboardStore{
		cashBalances: []database.GetCashBalancesRow{},
		pnlSummary: database.GetMonthlyPnlSummaryRow{
			NetSales: "0.00",
			Cogs:     "0.00",
			Expenses: "0.00",
		},
		pendingSummary: database.GetPendingReimbursementsSummaryRow{
			TotalCount:  0,
			TotalAmount: "0.00",
		},
		recentTxs: []database.AcctCashTransaction{},
	}

	router := setupDashboardRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/accounting/dashboard/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify empty arrays and zero values
	cashBalances := resp["cash_balances"].([]interface{})
	if len(cashBalances) != 0 {
		t.Errorf("expected 0 cash balances, got %d", len(cashBalances))
	}

	monthlyPnl := resp["monthly_pnl"].(map[string]interface{})
	if monthlyPnl["net_sales"] != "0.00" {
		t.Errorf("expected net_sales 0.00, got %s", monthlyPnl["net_sales"])
	}
	if monthlyPnl["net_profit"] != "0.00" {
		t.Errorf("expected net_profit 0.00, got %s", monthlyPnl["net_profit"])
	}

	pendingReimb := resp["pending_reimbursements"].(map[string]interface{})
	if pendingReimb["count"].(float64) != 0 {
		t.Errorf("expected count 0, got %v", pendingReimb["count"])
	}

	recentTxs := resp["recent_transactions"].([]interface{})
	if len(recentTxs) != 0 {
		t.Errorf("expected 0 recent transactions, got %d", len(recentTxs))
	}
}

func TestGetDashboard_StoreError(t *testing.T) {
	store := &mockDashboardStore{
		cashBalancesErr: fmt.Errorf("database connection failed"),
	}

	router := setupDashboardRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/accounting/dashboard/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != "internal server error" {
		t.Errorf("expected error message 'internal server error', got %s", resp["error"])
	}
}
