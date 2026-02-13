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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

// SalesStore defines the database methods needed by sales handlers.
type SalesStore interface {
	ListAcctSalesDailySummaries(ctx context.Context, arg database.ListAcctSalesDailySummariesParams) ([]database.AcctSalesDailySummary, error)
	GetAcctSalesDailySummary(ctx context.Context, id uuid.UUID) (database.AcctSalesDailySummary, error)
	CreateAcctSalesDailySummary(ctx context.Context, arg database.CreateAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error)
	UpdateAcctSalesDailySummary(ctx context.Context, arg database.UpdateAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error)
	DeleteAcctSalesDailySummary(ctx context.Context, id uuid.UUID) error
	UpsertAcctSalesDailySummary(ctx context.Context, arg database.UpsertAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error)
	ListUnpostedSalesSummaries(ctx context.Context, arg database.ListUnpostedSalesSummariesParams) ([]database.AcctSalesDailySummary, error)
	MarkSalesSummariesPosted(ctx context.Context, arg database.MarkSalesSummariesPostedParams) error
	AggregatePOSSales(ctx context.Context, arg database.AggregatePOSSalesParams) ([]database.AggregatePOSSalesRow, error)
	// For posting to cash_transactions:
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
}

// --- SalesHandler ---

// SalesHandler handles sales summary endpoints.
type SalesHandler struct {
	store SalesStore
}

// NewSalesHandler creates a new SalesHandler.
func NewSalesHandler(store SalesStore) *SalesHandler {
	return &SalesHandler{store: store}
}

// RegisterRoutes registers sales endpoints.
func (h *SalesHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListSalesSummaries)
	r.Post("/", h.CreateSalesSummary)
	r.Put("/{id}", h.UpdateSalesSummary)
	r.Delete("/{id}", h.DeleteSalesSummary)
	r.Post("/sync-pos", h.SyncPOS)
	r.Post("/post", h.PostSales)
}

// Channel mapping: POS order_type → accounting channel name
var orderTypeToChannel = map[string]string{
	"DINE_IN":  "Dine In",
	"TAKEAWAY": "Take Away",
	"CATERING": "Catering",
	"DELIVERY": "Delivery",
}

// --- Request / Response types ---

type salesSummaryResponse struct {
	ID             uuid.UUID  `json:"id"`
	SalesDate      string     `json:"sales_date"`
	Channel        string     `json:"channel"`
	PaymentMethod  string     `json:"payment_method"`
	GrossSales     string     `json:"gross_sales"`
	DiscountAmount string     `json:"discount_amount"`
	NetSales       string     `json:"net_sales"`
	CashAccountID  string     `json:"cash_account_id"`
	OutletID       *string    `json:"outlet_id"`
	Source         string     `json:"source"`
	PostedAt       *time.Time `json:"posted_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type createSalesSummaryRequest struct {
	SalesDate      string  `json:"sales_date"`
	Channel        string  `json:"channel"`
	PaymentMethod  string  `json:"payment_method"`
	GrossSales     string  `json:"gross_sales"`
	DiscountAmount string  `json:"discount_amount"`
	NetSales       string  `json:"net_sales"`
	CashAccountID  string  `json:"cash_account_id"`
	OutletID       *string `json:"outlet_id"`
}

type updateSalesSummaryRequest struct {
	Channel        string `json:"channel"`
	PaymentMethod  string `json:"payment_method"`
	GrossSales     string `json:"gross_sales"`
	DiscountAmount string `json:"discount_amount"`
	NetSales       string `json:"net_sales"`
	CashAccountID  string `json:"cash_account_id"`
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
	PostedCount         int `json:"posted_count"`
	TransactionsCreated int `json:"transactions_created"`
}

// --- Response converters ---

func toSalesSummaryResponse(row database.AcctSalesDailySummary) salesSummaryResponse {
	resp := salesSummaryResponse{
		ID:             row.ID,
		Channel:        row.Channel,
		PaymentMethod:  row.PaymentMethod,
		GrossSales:     numericToString(row.GrossSales),
		DiscountAmount: numericToString(row.DiscountAmount),
		NetSales:       numericToString(row.NetSales),
		CashAccountID:  row.CashAccountID.String(),
		Source:         row.Source,
		CreatedAt:      row.CreatedAt,
	}

	// Handle SalesDate
	if row.SalesDate.Valid {
		resp.SalesDate = row.SalesDate.Time.Format("2006-01-02")
	}

	// Handle nullable OutletID
	if row.OutletID.Valid {
		outletIDStr := uuid.UUID(row.OutletID.Bytes).String()
		resp.OutletID = &outletIDStr
	}

	// Handle nullable PostedAt
	if row.PostedAt.Valid {
		resp.PostedAt = &row.PostedAt.Time
	}

	return resp
}

// --- Helper functions ---

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

func parseSalesAmounts(grossStr, discountStr, netStr string) (pgtype.Numeric, pgtype.Numeric, pgtype.Numeric, error) {
	gross, err := decimal.NewFromString(grossStr)
	if err != nil {
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Numeric{}, fmt.Errorf("invalid gross_sales format")
	}

	// Default discount to "0.00" if empty
	if discountStr == "" {
		discountStr = "0.00"
	}
	discount, err := decimal.NewFromString(discountStr)
	if err != nil {
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Numeric{}, fmt.Errorf("invalid discount_amount format")
	}

	net, err := decimal.NewFromString(netStr)
	if err != nil {
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Numeric{}, fmt.Errorf("invalid net_sales format")
	}

	var grossPg, discountPg, netPg pgtype.Numeric
	if err := grossPg.Scan(gross.StringFixed(2)); err != nil {
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Numeric{}, fmt.Errorf("scan gross_sales: %w", err)
	}
	if err := discountPg.Scan(discount.StringFixed(2)); err != nil {
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Numeric{}, fmt.Errorf("scan discount_amount: %w", err)
	}
	if err := netPg.Scan(net.StringFixed(2)); err != nil {
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Numeric{}, fmt.Errorf("scan net_sales: %w", err)
	}

	return grossPg, discountPg, netPg, nil
}

func parseTransactionCodeNum(maxCode string) (int, error) {
	if len(maxCode) < 4 {
		return 1, nil
	}
	num, err := strconv.Atoi(maxCode[3:])
	if err != nil {
		return 0, err
	}
	return num + 1, nil
}

// --- Handlers ---

// ListSalesSummaries returns sales summaries with optional filters.
func (h *SalesHandler) ListSalesSummaries(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	params := database.ListAcctSalesDailySummariesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	// Optional filters
	if v := r.URL.Query().Get("start_date"); v != "" {
		date, err := time.Parse("2006-01-02", v)
		if err == nil {
			params.StartDate = pgtype.Date{Time: date, Valid: true}
		}
	}
	if v := r.URL.Query().Get("end_date"); v != "" {
		date, err := time.Parse("2006-01-02", v)
		if err == nil {
			params.EndDate = pgtype.Date{Time: date, Valid: true}
		}
	}
	if v := r.URL.Query().Get("channel"); v != "" {
		params.Channel = pgtype.Text{String: v, Valid: true}
	}
	if v := r.URL.Query().Get("outlet_id"); v != "" {
		id, err := uuid.Parse(v)
		if err == nil {
			params.OutletID = uuidToPgUUID(id)
		}
	}

	summaries, err := h.store.ListAcctSalesDailySummaries(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list sales summaries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]salesSummaryResponse, len(summaries))
	for i, s := range summaries {
		resp[i] = toSalesSummaryResponse(s)
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateSalesSummary creates a manual sales summary entry.
func (h *SalesHandler) CreateSalesSummary(w http.ResponseWriter, r *http.Request) {
	var req createSalesSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.SalesDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sales_date is required"})
		return
	}
	if req.Channel == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel is required"})
		return
	}
	if req.PaymentMethod == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_method is required"})
		return
	}
	if req.GrossSales == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gross_sales is required"})
		return
	}
	if req.NetSales == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "net_sales is required"})
		return
	}
	if req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash_account_id is required"})
		return
	}

	// Parse sales_date
	date, err := time.Parse("2006-01-02", req.SalesDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sales_date format, expected YYYY-MM-DD"})
		return
	}
	pgDate := pgtype.Date{Time: date, Valid: true}

	// Parse cash_account_id
	cashAccountID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	// Parse optional outlet_id
	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(id)
	}

	// Parse amounts
	grossPg, discountPg, netPg, err := parseSalesAmounts(req.GrossSales, req.DiscountAmount, req.NetSales)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	created, err := h.store.CreateAcctSalesDailySummary(r.Context(), database.CreateAcctSalesDailySummaryParams{
		SalesDate:      pgDate,
		Channel:        req.Channel,
		PaymentMethod:  req.PaymentMethod,
		GrossSales:     grossPg,
		DiscountAmount: discountPg,
		NetSales:       netPg,
		CashAccountID:  cashAccountID,
		OutletID:       outletID,
		Source:         "manual",
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "duplicate sales summary for this date, channel, payment method, and outlet"})
			return
		}
		log.Printf("ERROR: create sales summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toSalesSummaryResponse(created))
}

// UpdateSalesSummary updates a manual, unposted sales summary.
func (h *SalesHandler) UpdateSalesSummary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid summary ID"})
		return
	}

	var req updateSalesSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.Channel == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel is required"})
		return
	}
	if req.PaymentMethod == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_method is required"})
		return
	}
	if req.GrossSales == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gross_sales is required"})
		return
	}
	if req.NetSales == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "net_sales is required"})
		return
	}
	if req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash_account_id is required"})
		return
	}

	// Parse cash_account_id
	cashAccountID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	// Parse amounts
	grossPg, discountPg, netPg, err := parseSalesAmounts(req.GrossSales, req.DiscountAmount, req.NetSales)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	updated, err := h.store.UpdateAcctSalesDailySummary(r.Context(), database.UpdateAcctSalesDailySummaryParams{
		ID:             id,
		Channel:        req.Channel,
		PaymentMethod:  req.PaymentMethod,
		GrossSales:     grossPg,
		DiscountAmount: discountPg,
		NetSales:       netPg,
		CashAccountID:  cashAccountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "summary not found, not manual, or already posted"})
			return
		}
		log.Printf("ERROR: update sales summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toSalesSummaryResponse(updated))
}

// DeleteSalesSummary deletes a manual, unposted sales summary.
func (h *SalesHandler) DeleteSalesSummary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid summary ID"})
		return
	}

	err = h.store.DeleteAcctSalesDailySummary(r.Context(), id)
	if err != nil {
		log.Printf("ERROR: delete sales summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SyncPOS aggregates completed POS orders into sales daily summaries.
func (h *SalesHandler) SyncPOS(w http.ResponseWriter, r *http.Request) {
	var req syncPOSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.StartDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start_date is required"})
		return
	}
	if req.EndDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "end_date is required"})
		return
	}
	if req.OutletID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "outlet_id is required"})
		return
	}
	if len(req.PaymentMethodAccounts) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_method_accounts is required"})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date format, expected YYYY-MM-DD"})
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date format, expected YYYY-MM-DD"})
		return
	}

	// Parse outlet_id
	outletID, err := uuid.Parse(req.OutletID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
		return
	}

	// Validate payment_method_accounts UUIDs
	cashAccountMap := make(map[string]uuid.UUID)
	for method, acctStr := range req.PaymentMethodAccounts {
		acctID, err := uuid.Parse(acctStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid cash_account_id for payment method %s", method)})
			return
		}
		cashAccountMap[method] = acctID
	}

	// Aggregate POS sales
	rows, err := h.store.AggregatePOSSales(r.Context(), database.AggregatePOSSalesParams{
		OutletID: outletID,
		Column2:  pgtype.Date{Time: startDate, Valid: true},
		Column3:  pgtype.Date{Time: endDate, Valid: true},
	})
	if err != nil {
		log.Printf("ERROR: aggregate POS sales: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Upsert each aggregated row
	summaries := make([]salesSummaryResponse, 0, len(rows))
	for _, row := range rows {
		// Map order_type → channel name
		channel, ok := orderTypeToChannel[row.OrderType]
		if !ok {
			channel = row.OrderType // fallback to raw value
		}

		// Look up cash account for this payment method
		cashAcctID, ok := cashAccountMap[row.PaymentMethod]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("no cash account mapping for payment method %s", row.PaymentMethod),
			})
			return
		}

		// Parse total_amount to pgtype.Numeric
		totalDec, err := decimal.NewFromString(row.TotalAmount)
		if err != nil {
			log.Printf("ERROR: parse total_amount %s: %v", row.TotalAmount, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		var grossPg pgtype.Numeric
		if err := grossPg.Scan(totalDec.StringFixed(2)); err != nil {
			log.Printf("ERROR: scan gross amount: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		// Discount is 0 for POS sync (discounts already subtracted in payment amounts)
		var discountPg pgtype.Numeric
		if err := discountPg.Scan("0.00"); err != nil {
			log.Printf("ERROR: scan discount: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		// Net = gross for POS sync
		netPg := grossPg

		summary, err := h.store.UpsertAcctSalesDailySummary(r.Context(), database.UpsertAcctSalesDailySummaryParams{
			SalesDate:      row.SalesDate,
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

// PostSales creates cash transactions from unposted sales summaries and marks them posted.
func (h *SalesHandler) PostSales(w http.ResponseWriter, r *http.Request) {
	var req postSalesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.SalesDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sales_date is required"})
		return
	}
	if req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id is required"})
		return
	}

	// Parse sales_date
	date, err := time.Parse("2006-01-02", req.SalesDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sales_date format, expected YYYY-MM-DD"})
		return
	}
	pgDate := pgtype.Date{Time: date, Valid: true}

	// Parse account_id (Sales Revenue account)
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	// Parse optional outlet_id
	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(id)
	}

	// List unposted summaries for the date
	summaries, err := h.store.ListUnpostedSalesSummaries(r.Context(), database.ListUnpostedSalesSummariesParams{
		SalesDate: pgDate,
		OutletID:  outletID,
	})
	if err != nil {
		log.Printf("ERROR: list unposted sales summaries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if len(summaries) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no unposted sales summaries found"})
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

	// TODO: Wrap transaction creation + MarkSalesSummariesPosted in a DB transaction
	// for atomicity. If MarkSalesSummariesPosted fails after transactions are created,
	// a retry would create duplicates. Same pattern as reimbursement handler -- acceptable
	// for single-user accounting module.

	// Create cash transactions for each summary
	txCreated := 0
	for _, summary := range summaries {
		transactionCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++

		desc := fmt.Sprintf("Penjualan %s %s %s", summary.Channel, summary.PaymentMethod, req.SalesDate)

		// Use net_sales as the amount for the transaction
		// Quantity = 1, UnitPrice = net_sales, Amount = net_sales
		var qtyPg pgtype.Numeric
		if err := qtyPg.Scan("1.00"); err != nil {
			log.Printf("ERROR: scan quantity: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		_, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      transactionCode,
			TransactionDate:      pgDate,
			ItemID:               pgtype.UUID{}, // no item for sales
			Description:          desc,
			Quantity:             qtyPg,
			UnitPrice:            summary.NetSales,
			Amount:               summary.NetSales,
			LineType:             "SALES",
			AccountID:            accountID,
			CashAccountID:        uuidToPgUUID(summary.CashAccountID),
			OutletID:             summary.OutletID,
			ReimbursementBatchID: pgtype.Text{}, // empty
		})
		if err != nil {
			log.Printf("ERROR: create cash transaction: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		txCreated++
	}

	// Mark summaries as posted
	err = h.store.MarkSalesSummariesPosted(r.Context(), database.MarkSalesSummariesPostedParams{
		SalesDate: pgDate,
		OutletID:  outletID,
	})
	if err != nil {
		log.Printf("ERROR: mark sales summaries posted: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, postSalesResponse{
		PostedCount:         len(summaries),
		TransactionsCreated: txCreated,
	})
}
