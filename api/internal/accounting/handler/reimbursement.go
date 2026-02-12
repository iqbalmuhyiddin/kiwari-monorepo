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

// --- Store interface ---

// ReimbursementStore defines the database methods needed by reimbursement handlers.
type ReimbursementStore interface {
	ListAcctReimbursementRequests(ctx context.Context, arg database.ListAcctReimbursementRequestsParams) ([]database.AcctReimbursementRequest, error)
	GetAcctReimbursementRequest(ctx context.Context, id uuid.UUID) (database.AcctReimbursementRequest, error)
	CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error)
	UpdateAcctReimbursementRequest(ctx context.Context, arg database.UpdateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error)
	DeleteAcctReimbursementRequest(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	AssignReimbursementBatch(ctx context.Context, arg database.AssignReimbursementBatchParams) error
	ListReimbursementsByBatch(ctx context.Context, batchID pgtype.Text) ([]database.AcctReimbursementRequest, error)
	PostReimbursementBatch(ctx context.Context, batchID pgtype.Text) error
	CheckBatchPosted(ctx context.Context, batchID pgtype.Text) (bool, error)
	GetNextBatchCode(ctx context.Context) (string, error)
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
}

// --- ReimbursementHandler ---

// ReimbursementHandler handles reimbursement request endpoints.
type ReimbursementHandler struct {
	store ReimbursementStore
}

// NewReimbursementHandler creates a new ReimbursementHandler.
func NewReimbursementHandler(store ReimbursementStore) *ReimbursementHandler {
	return &ReimbursementHandler{
		store: store,
	}
}

// RegisterRoutes registers reimbursement endpoints.
func (h *ReimbursementHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListReimbursements)
	r.Post("/", h.CreateReimbursement)
	r.Get("/{id}", h.GetReimbursement)
	r.Put("/{id}", h.UpdateReimbursement)
	r.Delete("/{id}", h.DeleteReimbursement)
	r.Post("/batch", h.AssignBatch)
	r.Post("/batch/post", h.PostBatch)
}

// --- Request / Response types ---

type createReimbursementRequest struct {
	ExpenseDate string  `json:"expense_date"` // "2026-01-20"
	ItemID      *string `json:"item_id"`      // optional UUID
	Description string  `json:"description"`
	Qty         string  `json:"qty"`       // decimal string
	UnitPrice   string  `json:"unit_price"` // decimal string
	Amount      string  `json:"amount"`     // decimal string
	LineType    string  `json:"line_type"`  // INVENTORY|EXPENSE
	AccountID   string  `json:"account_id"` // UUID
	Status      string  `json:"status"`     // Draft|Ready (defaults to Draft)
	Requester   string  `json:"requester"`
	ReceiptLink *string `json:"receipt_link"` // optional URL
}

type updateReimbursementRequest struct {
	ExpenseDate string  `json:"expense_date"` // "2026-01-20"
	ItemID      *string `json:"item_id"`      // optional UUID
	Description string  `json:"description"`
	Qty         string  `json:"qty"`       // decimal string
	UnitPrice   string  `json:"unit_price"` // decimal string
	Amount      string  `json:"amount"`     // decimal string
	LineType    string  `json:"line_type"`  // INVENTORY|EXPENSE
	AccountID   string  `json:"account_id"` // UUID
	Status      string  `json:"status"`     // Draft|Ready
	ReceiptLink *string `json:"receipt_link"` // optional URL
}

type reimbursementResponse struct {
	ID          uuid.UUID  `json:"id"`
	BatchID     *string    `json:"batch_id"`
	ExpenseDate string     `json:"expense_date"` // "2006-01-02" format
	ItemID      *string    `json:"item_id"`
	Description string     `json:"description"`
	Qty         string     `json:"qty"`
	UnitPrice   string     `json:"unit_price"`
	Amount      string     `json:"amount"`
	LineType    string     `json:"line_type"`
	AccountID   string     `json:"account_id"`
	Status      string     `json:"status"`
	Requester   string     `json:"requester"`
	ReceiptLink *string    `json:"receipt_link"`
	PostedAt    *time.Time `json:"posted_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type assignBatchRequest struct {
	IDs []string `json:"ids"` // array of UUIDs
}

type assignBatchResponse struct {
	BatchID  string `json:"batch_id"`
	Assigned int    `json:"assigned"`
}

type postBatchRequest struct {
	BatchID        string `json:"batch_id"`
	PaymentDate    string `json:"payment_date"`    // "2026-01-20"
	CashAccountID  string `json:"cash_account_id"` // UUID
}

type postBatchResponse struct {
	BatchID      string                `json:"batch_id"`
	Posted       int                   `json:"posted"`
	Transactions []transactionResponse `json:"transactions"`
}

// --- Response converters ---

func toReimbursementResponse(r database.AcctReimbursementRequest) reimbursementResponse {
	resp := reimbursementResponse{
		ID:          r.ID,
		Description: r.Description,
		LineType:    r.LineType,
		AccountID:   r.AccountID.String(),
		Status:      r.Status,
		Requester:   r.Requester,
		CreatedAt:   r.CreatedAt,
	}

	// Handle BatchID (pgtype.Text)
	if r.BatchID.Valid {
		resp.BatchID = &r.BatchID.String
	}

	// Handle ExpenseDate (pgtype.Date)
	if r.ExpenseDate.Valid {
		resp.ExpenseDate = r.ExpenseDate.Time.Format("2006-01-02")
	}

	// Handle ItemID (pgtype.UUID)
	if r.ItemID.Valid {
		itemIDStr := uuid.UUID(r.ItemID.Bytes).String()
		resp.ItemID = &itemIDStr
	}

	// Handle ReceiptLink (pgtype.Text)
	if r.ReceiptLink.Valid {
		resp.ReceiptLink = &r.ReceiptLink.String
	}

	// Handle PostedAt (pgtype.Timestamptz)
	if r.PostedAt.Valid {
		resp.PostedAt = &r.PostedAt.Time
	}

	// Convert numeric fields using numericToString
	resp.Qty = numericToString(r.Qty)
	resp.UnitPrice = numericToString(r.UnitPrice)
	resp.Amount = numericToString(r.Amount)

	return resp
}

// numericToString converts pgtype.Numeric to string with 2 decimal places.
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

// --- Handlers ---

// ListReimbursements returns reimbursement requests with optional filters.
func (h *ReimbursementHandler) ListReimbursements(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := int32(50) // default
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err == nil && l > 0 {
			limit = int32(l)
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := int32(0)
	if offsetStr != "" {
		o, err := strconv.Atoi(offsetStr)
		if err == nil && o >= 0 {
			offset = int32(o)
		}
	}

	// Optional filters
	status := r.URL.Query().Get("status")
	requester := r.URL.Query().Get("requester")
	batchID := r.URL.Query().Get("batch_id")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	// Build params
	params := database.ListAcctReimbursementRequestsParams{
		Limit:  limit,
		Offset: offset,
	}

	if status != "" {
		params.Status = pgtype.Text{String: status, Valid: true}
	}
	if requester != "" {
		params.Requester = pgtype.Text{String: requester, Valid: true}
	}
	if batchID != "" {
		params.BatchID = pgtype.Text{String: batchID, Valid: true}
	}
	if startDateStr != "" {
		date, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			params.StartDate = pgtype.Date{Time: date, Valid: true}
		}
	}
	if endDateStr != "" {
		date, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			params.EndDate = pgtype.Date{Time: date, Valid: true}
		}
	}

	// Fetch from store
	requests, err := h.store.ListAcctReimbursementRequests(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list reimbursement requests: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Convert to response
	resp := make([]reimbursementResponse, len(requests))
	for i, req := range requests {
		resp[i] = toReimbursementResponse(req)
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetReimbursement returns a single reimbursement request by ID.
func (h *ReimbursementHandler) GetReimbursement(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reimbursement ID"})
		return
	}

	req, err := h.store.GetAcctReimbursementRequest(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "reimbursement not found"})
			return
		}
		log.Printf("ERROR: get reimbursement request: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toReimbursementResponse(req))
}

// CreateReimbursement creates a new reimbursement request.
func (h *ReimbursementHandler) CreateReimbursement(w http.ResponseWriter, r *http.Request) {
	var req createReimbursementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.ExpenseDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expense_date is required"})
		return
	}
	if req.Description == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "description is required"})
		return
	}
	if req.Qty == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "qty is required"})
		return
	}
	if req.UnitPrice == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unit_price is required"})
		return
	}
	if req.Amount == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount is required"})
		return
	}
	if req.LineType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type is required"})
		return
	}
	if req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id is required"})
		return
	}
	if req.Requester == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "requester is required"})
		return
	}

	// Validate line_type
	if req.LineType != "INVENTORY" && req.LineType != "EXPENSE" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type must be INVENTORY or EXPENSE"})
		return
	}

	// Parse expense_date
	date, err := time.Parse("2006-01-02", req.ExpenseDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expense_date format, expected YYYY-MM-DD"})
		return
	}
	pgDate := pgtype.Date{Time: date, Valid: true}

	// Parse account_id
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	// Parse optional item_id
	var itemID pgtype.UUID
	if req.ItemID != nil && *req.ItemID != "" {
		id, err := uuid.Parse(*req.ItemID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item_id"})
			return
		}
		itemID = uuidToPgUUID(id)
	}

	// Parse qty, unit_price, amount
	qty, err := decimal.NewFromString(req.Qty)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid qty format"})
		return
	}
	price, err := decimal.NewFromString(req.UnitPrice)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid unit_price format"})
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount format"})
		return
	}

	// Convert to pgtype.Numeric
	var qtyPg, pricePg, amountPg pgtype.Numeric
	if err := qtyPg.Scan(qty.StringFixed(4)); err != nil {
		log.Printf("ERROR: scan qty: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if err := pricePg.Scan(price.StringFixed(2)); err != nil {
		log.Printf("ERROR: scan price: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if err := amountPg.Scan(amount.StringFixed(2)); err != nil {
		log.Printf("ERROR: scan amount: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Default status to Draft if not provided or empty
	status := req.Status
	if status == "" {
		status = "Draft"
	}
	// Validate status
	if status != "Draft" && status != "Ready" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be Draft or Ready"})
		return
	}

	// Create reimbursement request
	created, err := h.store.CreateAcctReimbursementRequest(r.Context(), database.CreateAcctReimbursementRequestParams{
		ExpenseDate: pgDate,
		ItemID:      itemID,
		Description: req.Description,
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    req.LineType,
		AccountID:   accountID,
		Status:      status,
		Requester:   req.Requester,
		ReceiptLink: stringToPgText(req.ReceiptLink),
	})
	if err != nil {
		log.Printf("ERROR: create reimbursement request: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toReimbursementResponse(created))
}

// UpdateReimbursement updates an existing reimbursement request.
func (h *ReimbursementHandler) UpdateReimbursement(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reimbursement ID"})
		return
	}

	var req updateReimbursementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.ExpenseDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expense_date is required"})
		return
	}
	if req.Description == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "description is required"})
		return
	}
	if req.Qty == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "qty is required"})
		return
	}
	if req.UnitPrice == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unit_price is required"})
		return
	}
	if req.Amount == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount is required"})
		return
	}
	if req.LineType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type is required"})
		return
	}
	if req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id is required"})
		return
	}
	if req.Status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status is required"})
		return
	}

	// Validate line_type
	if req.LineType != "INVENTORY" && req.LineType != "EXPENSE" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type must be INVENTORY or EXPENSE"})
		return
	}

	// Validate status
	if req.Status != "Draft" && req.Status != "Ready" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be Draft or Ready"})
		return
	}

	// Parse expense_date
	date, err := time.Parse("2006-01-02", req.ExpenseDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expense_date format, expected YYYY-MM-DD"})
		return
	}
	pgDate := pgtype.Date{Time: date, Valid: true}

	// Parse account_id
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	// Parse optional item_id
	var itemID pgtype.UUID
	if req.ItemID != nil && *req.ItemID != "" {
		id, err := uuid.Parse(*req.ItemID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item_id"})
			return
		}
		itemID = uuidToPgUUID(id)
	}

	// Parse qty, unit_price, amount
	qty, err := decimal.NewFromString(req.Qty)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid qty format"})
		return
	}
	price, err := decimal.NewFromString(req.UnitPrice)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid unit_price format"})
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount format"})
		return
	}

	// Convert to pgtype.Numeric
	var qtyPg, pricePg, amountPg pgtype.Numeric
	if err := qtyPg.Scan(qty.StringFixed(4)); err != nil {
		log.Printf("ERROR: scan qty: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if err := pricePg.Scan(price.StringFixed(2)); err != nil {
		log.Printf("ERROR: scan price: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if err := amountPg.Scan(amount.StringFixed(2)); err != nil {
		log.Printf("ERROR: scan amount: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Update reimbursement request
	updated, err := h.store.UpdateAcctReimbursementRequest(r.Context(), database.UpdateAcctReimbursementRequestParams{
		ID:          id,
		ExpenseDate: pgDate,
		ItemID:      itemID,
		Description: req.Description,
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    req.LineType,
		AccountID:   accountID,
		Status:      req.Status,
		ReceiptLink: stringToPgText(req.ReceiptLink),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "reimbursement not found or already posted"})
			return
		}
		log.Printf("ERROR: update reimbursement request: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toReimbursementResponse(updated))
}

// DeleteReimbursement deletes a draft reimbursement request.
func (h *ReimbursementHandler) DeleteReimbursement(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reimbursement ID"})
		return
	}

	_, err = h.store.DeleteAcctReimbursementRequest(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "reimbursement not found or not in Draft status"})
			return
		}
		log.Printf("ERROR: delete reimbursement request: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AssignBatch assigns multiple reimbursement requests to a new batch.
func (h *ReimbursementHandler) AssignBatch(w http.ResponseWriter, r *http.Request) {
	var req assignBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids cannot be empty"})
		return
	}

	// Get next batch code
	maxCode, err := h.store.GetNextBatchCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next batch code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Parse and increment batch code
	// maxCode format: "RMB000" or "RMB123"
	numStr := maxCode[3:] // Extract numeric suffix
	nextNum, err := strconv.Atoi(numStr)
	if err != nil {
		log.Printf("ERROR: parse batch code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum++ // Start from next number

	batchID := fmt.Sprintf("RMB%03d", nextNum)
	pgBatchID := pgtype.Text{String: batchID, Valid: true}

	// Assign each reimbursement to the batch
	assigned := 0
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid id: %s", idStr)})
			return
		}

		err = h.store.AssignReimbursementBatch(r.Context(), database.AssignReimbursementBatchParams{
			BatchID: pgBatchID,
			ID:      id,
		})
		if err != nil {
			log.Printf("ERROR: assign reimbursement batch: %v", err)
			// Continue assigning others (ignore if already assigned or not found)
			continue
		}
		assigned++
	}

	writeJSON(w, http.StatusOK, assignBatchResponse{
		BatchID:  batchID,
		Assigned: assigned,
	})
}

// PostBatch posts a reimbursement batch and creates cash transactions.
func (h *ReimbursementHandler) PostBatch(w http.ResponseWriter, r *http.Request) {
	var req postBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.BatchID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "batch_id is required"})
		return
	}
	if req.PaymentDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_date is required"})
		return
	}
	if req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash_account_id is required"})
		return
	}

	// Parse payment_date
	date, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment_date format, expected YYYY-MM-DD"})
		return
	}
	pgDate := pgtype.Date{Time: date, Valid: true}

	// Parse cash_account_id
	cashAccountID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	pgBatchID := pgtype.Text{String: req.BatchID, Valid: true}

	// Check idempotency: has this batch already been posted?
	isPosted, err := h.store.CheckBatchPosted(r.Context(), pgBatchID)
	if err != nil {
		log.Printf("ERROR: check batch posted: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if isPosted {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "batch already posted"})
		return
	}

	// Get all reimbursements in the batch
	reimbursements, err := h.store.ListReimbursementsByBatch(r.Context(), pgBatchID)
	if err != nil {
		log.Printf("ERROR: list reimbursements by batch: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if len(reimbursements) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "batch not found or empty"})
		return
	}

	// Get next transaction code
	maxCode, err := h.store.GetNextTransactionCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Parse and increment transaction code
	numStr := maxCode[3:] // Extract numeric suffix from "PCS000000"
	nextNum, err := strconv.Atoi(numStr)
	if err != nil {
		log.Printf("ERROR: parse transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum++ // Start from next number

	// Create cash transactions for each Ready reimbursement
	var transactions []transactionResponse
	posted := 0
	for _, reimb := range reimbursements {
		// Only process Ready status
		if reimb.Status != "Ready" {
			continue
		}

		// Generate transaction code
		transactionCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++

		// Create cash transaction
		tx, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      transactionCode,
			TransactionDate:      pgDate,
			ItemID:               reimb.ItemID,
			Description:          reimb.Description,
			Quantity:             reimb.Qty,
			UnitPrice:            reimb.UnitPrice,
			Amount:               reimb.Amount,
			LineType:             reimb.LineType,
			AccountID:            reimb.AccountID,
			CashAccountID:        uuidToPgUUID(cashAccountID),
			OutletID:             pgtype.UUID{}, // empty for reimbursements
			ReimbursementBatchID: pgBatchID,
		})
		if err != nil {
			log.Printf("ERROR: create cash transaction: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		// Build response
		var itemIDPtr *string
		if tx.ItemID.Valid {
			idStr := uuid.UUID(tx.ItemID.Bytes).String()
			itemIDPtr = &idStr
		}

		transactions = append(transactions, transactionResponse{
			ID:              tx.ID,
			TransactionCode: tx.TransactionCode,
			TransactionDate: req.PaymentDate,
			Description:     tx.Description,
			Quantity:        numericToString(tx.Quantity),
			UnitPrice:       numericToString(tx.UnitPrice),
			Amount:          numericToString(tx.Amount),
			LineType:        tx.LineType,
			ItemID:          itemIDPtr,
			CreatedAt:       tx.CreatedAt,
		})
		posted++
	}

	// Mark batch as posted
	err = h.store.PostReimbursementBatch(r.Context(), pgBatchID)
	if err != nil {
		log.Printf("ERROR: post reimbursement batch: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, postBatchResponse{
		BatchID:      req.BatchID,
		Posted:       posted,
		Transactions: transactions,
	})
}
