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
