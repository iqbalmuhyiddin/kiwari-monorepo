package handler_test

import (
	"encoding/json"
	"context"
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
