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
	ID                   uuid.UUID  `json:"id"`
	TransactionCode      string     `json:"transaction_code"`
	TransactionDate      string     `json:"transaction_date"`
	ItemID               *string    `json:"item_id"`
	Description          string     `json:"description"`
	Quantity             string     `json:"quantity"`
	UnitPrice            string     `json:"unit_price"`
	Amount               string     `json:"amount"`
	LineType             string     `json:"line_type"`
	AccountID            string     `json:"account_id"`
	CashAccountID        *string    `json:"cash_account_id"`
	OutletID             *string    `json:"outlet_id"`
	ReimbursementBatchID *string    `json:"reimbursement_batch_id"`
	CreatedAt            time.Time  `json:"created_at"`
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
	if err := qtyPg.Scan(qty.StringFixed(2)); err != nil {
		log.Printf("ERROR: scan quantity: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if err := pricePg.Scan(price.StringFixed(2)); err != nil {
		log.Printf("ERROR: scan unit_price: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if err := amountPg.Scan(amount.StringFixed(2)); err != nil {
		log.Printf("ERROR: scan amount: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

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
