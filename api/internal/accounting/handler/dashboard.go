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

var jakartaLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	jakartaLocation = loc
}

// --- Store interface ---

// DashboardStore defines the database methods needed by dashboard handlers.
type DashboardStore interface {
	GetCashBalances(ctx context.Context) ([]database.GetCashBalancesRow, error)
	GetMonthlyPnlSummary(ctx context.Context, arg database.GetMonthlyPnlSummaryParams) (database.GetMonthlyPnlSummaryRow, error)
	GetPendingReimbursementsSummary(ctx context.Context) (database.GetPendingReimbursementsSummaryRow, error)
	ListAcctCashTransactions(ctx context.Context, arg database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error)
}

// --- DashboardHandler ---

// DashboardHandler handles accounting dashboard endpoints.
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
	CashBalances          []cashBalanceResponse         `json:"cash_balances"`
	MonthlyPnl            monthlyPnlResponse            `json:"monthly_pnl"`
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

// GetDashboard returns dashboard data with cash balances, monthly P&L, pending reimbursements, and recent transactions.
func (h *DashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get cash balances
	cashBalancesRows, err := h.store.GetCashBalances(ctx)
	if err != nil {
		log.Printf("ERROR: get cash balances: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Compute month boundaries in Asia/Jakarta timezone
	now := time.Now().In(jakartaLocation)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	// Get monthly P&L summary
	pnlRow, err := h.store.GetMonthlyPnlSummary(ctx, database.GetMonthlyPnlSummaryParams{
		MonthStart: pgtype.Date{Time: monthStart, Valid: true},
		MonthEnd:   pgtype.Date{Time: monthEnd, Valid: true},
	})
	if err != nil {
		log.Printf("ERROR: get monthly P&L summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Get pending reimbursements summary
	pendingRow, err := h.store.GetPendingReimbursementsSummary(ctx)
	if err != nil {
		log.Printf("ERROR: get pending reimbursements summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Get recent 10 transactions
	recentTxs, err := h.store.ListAcctCashTransactions(ctx, database.ListAcctCashTransactionsParams{
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		log.Printf("ERROR: get recent transactions: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Build response
	resp := dashboardResponse{
		CashBalances:          buildCashBalances(cashBalancesRows),
		MonthlyPnl:            buildMonthlyPnl(pnlRow, now.Format("2006-01")),
		PendingReimbursements: buildPendingReimbursements(pendingRow),
		RecentTransactions:    buildRecentTransactions(recentTxs),
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Response builders ---

func buildCashBalances(rows []database.GetCashBalancesRow) []cashBalanceResponse {
	result := make([]cashBalanceResponse, 0, len(rows))
	for _, row := range rows {
		totalIn, _ := decimal.NewFromString(row.TotalIn)
		totalOut, _ := decimal.NewFromString(row.TotalOut)
		balance := totalIn.Sub(totalOut)

		result = append(result, cashBalanceResponse{
			CashAccountCode: row.CashAccountCode,
			CashAccountName: row.CashAccountName,
			Balance:         balance.StringFixed(2),
		})
	}
	return result
}

func buildMonthlyPnl(row database.GetMonthlyPnlSummaryRow, period string) monthlyPnlResponse {
	netSales, _ := decimal.NewFromString(row.NetSales)
	cogs, _ := decimal.NewFromString(row.Cogs)
	expenses, _ := decimal.NewFromString(row.Expenses)

	grossProfit := netSales.Sub(cogs)
	netProfit := grossProfit.Sub(expenses)

	return monthlyPnlResponse{
		Period:        period,
		NetSales:      netSales.StringFixed(2),
		COGS:          cogs.StringFixed(2),
		GrossProfit:   grossProfit.StringFixed(2),
		TotalExpenses: expenses.StringFixed(2),
		NetProfit:     netProfit.StringFixed(2),
	}
}

func buildPendingReimbursements(row database.GetPendingReimbursementsSummaryRow) pendingReimbursementsResponse {
	return pendingReimbursementsResponse{
		Count:       row.TotalCount,
		TotalAmount: row.TotalAmount,
	}
}

func buildRecentTransactions(txs []database.AcctCashTransaction) []recentTxResponse {
	result := make([]recentTxResponse, 0, len(txs))
	for _, tx := range txs {
		result = append(result, recentTxResponse{
			ID:              tx.ID.String(),
			TransactionCode: tx.TransactionCode,
			TransactionDate: tx.TransactionDate.Time.Format("2006-01-02"),
			Description:     tx.Description,
			Amount:          numericToString(tx.Amount),
			LineType:        tx.LineType,
			CreatedAt:       tx.CreatedAt,
		})
	}
	return result
}
