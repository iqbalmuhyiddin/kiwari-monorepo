package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock ReimbursementStore ---

type mockReimbursementStore struct {
	requests   map[uuid.UUID]database.AcctReimbursementRequest
	nextBatch  string
	nextTxCode string
	txns       []database.AcctCashTransaction
}

func newMockReimbursementStore() *mockReimbursementStore {
	return &mockReimbursementStore{
		requests:   make(map[uuid.UUID]database.AcctReimbursementRequest),
		nextBatch:  "RMB000",
		nextTxCode: "PCS000000",
		txns:       []database.AcctCashTransaction{},
	}
}

func (m *mockReimbursementStore) ListAcctReimbursementRequests(_ context.Context, arg database.ListAcctReimbursementRequestsParams) ([]database.AcctReimbursementRequest, error) {
	var result []database.AcctReimbursementRequest
	for _, r := range m.requests {
		// Apply filters
		if arg.Status.Valid && r.Status != arg.Status.String {
			continue
		}
		if arg.Requester.Valid && r.Requester != arg.Requester.String {
			continue
		}
		if arg.BatchID.Valid && (!r.BatchID.Valid || r.BatchID.String != arg.BatchID.String) {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

func (m *mockReimbursementStore) GetAcctReimbursementRequest(_ context.Context, id uuid.UUID) (database.AcctReimbursementRequest, error) {
	r, ok := m.requests[id]
	if !ok {
		return database.AcctReimbursementRequest{}, pgx.ErrNoRows
	}
	return r, nil
}

func (m *mockReimbursementStore) CreateAcctReimbursementRequest(_ context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
	r := database.AcctReimbursementRequest{
		ID:          uuid.New(),
		BatchID:     pgtype.Text{},
		ExpenseDate: arg.ExpenseDate,
		ItemID:      arg.ItemID,
		Description: arg.Description,
		Qty:         arg.Qty,
		UnitPrice:   arg.UnitPrice,
		Amount:      arg.Amount,
		LineType:    arg.LineType,
		AccountID:   arg.AccountID,
		Status:      arg.Status,
		Requester:   arg.Requester,
		ReceiptLink: arg.ReceiptLink,
		PostedAt:    pgtype.Timestamptz{},
		CreatedAt:   time.Now(),
	}
	m.requests[r.ID] = r
	return r, nil
}

func (m *mockReimbursementStore) UpdateAcctReimbursementRequest(_ context.Context, arg database.UpdateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
	r, ok := m.requests[arg.ID]
	if !ok || r.Status == "Posted" {
		return database.AcctReimbursementRequest{}, pgx.ErrNoRows
	}
	r.ExpenseDate = arg.ExpenseDate
	r.ItemID = arg.ItemID
	r.Description = arg.Description
	r.Qty = arg.Qty
	r.UnitPrice = arg.UnitPrice
	r.Amount = arg.Amount
	r.LineType = arg.LineType
	r.AccountID = arg.AccountID
	r.Status = arg.Status
	r.ReceiptLink = arg.ReceiptLink
	m.requests[r.ID] = r
	return r, nil
}

func (m *mockReimbursementStore) DeleteAcctReimbursementRequest(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	r, ok := m.requests[id]
	if !ok || r.Status != "Draft" {
		return uuid.Nil, pgx.ErrNoRows
	}
	delete(m.requests, id)
	return id, nil
}

func (m *mockReimbursementStore) AssignReimbursementBatch(_ context.Context, arg database.AssignReimbursementBatchParams) (int64, error) {
	r, ok := m.requests[arg.ID]
	if !ok || r.Status != "Draft" {
		return 0, nil // :execrows returns 0 rows affected, not an error
	}
	r.BatchID = arg.BatchID
	r.Status = "Ready"
	m.requests[r.ID] = r
	return 1, nil
}

func (m *mockReimbursementStore) ListReimbursementsByBatch(_ context.Context, batchID pgtype.Text) ([]database.AcctReimbursementRequest, error) {
	var result []database.AcctReimbursementRequest
	for _, r := range m.requests {
		if r.BatchID.Valid && r.BatchID.String == batchID.String {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockReimbursementStore) PostReimbursementBatch(_ context.Context, batchID pgtype.Text) error {
	for id, r := range m.requests {
		if r.BatchID.Valid && r.BatchID.String == batchID.String && r.Status == "Ready" {
			r.Status = "Posted"
			r.PostedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			m.requests[id] = r
		}
	}
	return nil
}

func (m *mockReimbursementStore) CheckBatchPosted(_ context.Context, batchID pgtype.Text) (bool, error) {
	for _, r := range m.requests {
		if r.BatchID.Valid && r.BatchID.String == batchID.String && r.Status == "Posted" {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockReimbursementStore) GetNextBatchCode(_ context.Context) (string, error) {
	return m.nextBatch, nil
}

func (m *mockReimbursementStore) CreateAcctCashTransaction(_ context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error) {
	tx := database.AcctCashTransaction{
		ID:                   uuid.New(),
		TransactionCode:      arg.TransactionCode,
		TransactionDate:      arg.TransactionDate,
		ItemID:               arg.ItemID,
		Description:          arg.Description,
		Quantity:             arg.Quantity,
		UnitPrice:            arg.UnitPrice,
		Amount:               arg.Amount,
		LineType:             arg.LineType,
		AccountID:            arg.AccountID,
		CashAccountID:        arg.CashAccountID,
		OutletID:             arg.OutletID,
		ReimbursementBatchID: arg.ReimbursementBatchID,
		CreatedAt:            time.Now(),
	}
	m.txns = append(m.txns, tx)
	return tx, nil
}

func (m *mockReimbursementStore) GetNextTransactionCode(_ context.Context) (string, error) {
	return m.nextTxCode, nil
}

// --- Helpers ---

func setupReimbursementRouter(store handler.ReimbursementStore) *chi.Mux {
	h := handler.NewReimbursementHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/reimbursements", h.RegisterRoutes)
	return r
}

// --- CRUD Tests ---

func TestReimbursementList_Empty(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	rr := doRequest(t, router, "GET", "/accounting/reimbursements", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestReimbursementCreate_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	payload := map[string]interface{}{
		"expense_date": "2026-01-20",
		"description":  "Taxi to supplier",
		"qty":          "1.0",
		"unit_price":   "50000.00",
		"amount":       "50000.00",
		"line_type":    "EXPENSE",
		"account_id":   uuid.New().String(),
		"status":       "Draft",
		"requester":    "John Doe",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements", payload)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())

	if resp["description"] != "Taxi to supplier" {
		t.Errorf("description: got %v, want Taxi to supplier", resp["description"])
	}
	if resp["status"] != "Draft" {
		t.Errorf("status: got %v, want Draft", resp["status"])
	}
	if resp["requester"] != "John Doe" {
		t.Errorf("requester: got %v, want John Doe", resp["requester"])
	}
	if resp["line_type"] != "EXPENSE" {
		t.Errorf("line_type: got %v, want EXPENSE", resp["line_type"])
	}
}

func TestReimbursementCreate_MissingFields(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	tests := []struct {
		name    string
		payload map[string]interface{}
	}{
		{
			name: "missing expense_date",
			payload: map[string]interface{}{
				"description": "Test",
				"qty":         "1.0",
				"unit_price":  "10000.00",
				"amount":      "10000.00",
				"line_type":   "EXPENSE",
				"account_id":  uuid.New().String(),
				"requester":   "John",
			},
		},
		{
			name: "missing description",
			payload: map[string]interface{}{
				"expense_date": "2026-01-20",
				"qty":          "1.0",
				"unit_price":   "10000.00",
				"amount":       "10000.00",
				"line_type":    "EXPENSE",
				"account_id":   uuid.New().String(),
				"requester":    "John",
			},
		},
		{
			name: "missing requester",
			payload: map[string]interface{}{
				"expense_date": "2026-01-20",
				"description":  "Test",
				"qty":          "1.0",
				"unit_price":   "10000.00",
				"amount":       "10000.00",
				"line_type":    "EXPENSE",
				"account_id":   uuid.New().String(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := doRequest(t, router, "POST", "/accounting/reimbursements", tt.payload)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
			}
		})
	}
}

func TestReimbursementUpdate_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	// Create a reimbursement first
	id := uuid.New()
	accountID := uuid.New()
	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan("1.0000")
	pricePg.Scan("50000.00")
	amountPg.Scan("50000.00")

	store.requests[id] = database.AcctReimbursementRequest{
		ID:          id,
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Description: "Original description",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Draft",
		Requester:   "John Doe",
		CreatedAt:   time.Now(),
	}

	payload := map[string]interface{}{
		"expense_date": "2026-01-21",
		"description":  "Updated description",
		"qty":          "2.0",
		"unit_price":   "60000.00",
		"amount":       "120000.00",
		"line_type":    "EXPENSE",
		"account_id":   accountID.String(),
		"status":       "Ready",
	}

	rr := doRequest(t, router, "PUT", "/accounting/reimbursements/"+id.String(), payload)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())

	if resp["description"] != "Updated description" {
		t.Errorf("description: got %v, want Updated description", resp["description"])
	}
	if resp["status"] != "Ready" {
		t.Errorf("status: got %v, want Ready", resp["status"])
	}
	if resp["expense_date"] != "2026-01-21" {
		t.Errorf("expense_date: got %v, want 2026-01-21", resp["expense_date"])
	}
}

func TestReimbursementDelete_DraftOnly(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	// Create a Draft reimbursement
	draftID := uuid.New()
	store.requests[draftID] = database.AcctReimbursementRequest{
		ID:        draftID,
		Status:    "Draft",
		CreatedAt: time.Now(),
	}

	// Create a Ready reimbursement
	readyID := uuid.New()
	store.requests[readyID] = database.AcctReimbursementRequest{
		ID:        readyID,
		Status:    "Ready",
		CreatedAt: time.Now(),
	}

	// DELETE Draft should succeed (204)
	rr := doRequest(t, router, "DELETE", "/accounting/reimbursements/"+draftID.String(), nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE Draft: status got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	// DELETE Ready should fail (404)
	rr = doRequest(t, router, "DELETE", "/accounting/reimbursements/"+readyID.String(), nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("DELETE Ready: status got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestReimbursementGet_NotFound(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	rr := doRequest(t, router, "GET", "/accounting/reimbursements/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// --- Batch Tests ---

func TestBatchAssign_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	// Seed 2 Draft reimbursement requests
	id1 := uuid.New()
	id2 := uuid.New()
	accountID := uuid.New()

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan("5.0000")
	pricePg.Scan("100000.00")
	amountPg.Scan("500000.00")

	store.requests[id1] = database.AcctReimbursementRequest{
		ID:          id1,
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Description: "Request 1",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Draft",
		Requester:   "John Doe",
		CreatedAt:   time.Now(),
	}

	store.requests[id2] = database.AcctReimbursementRequest{
		ID:          id2,
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 21, 0, 0, 0, 0, time.UTC), Valid: true},
		Description: "Request 2",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Draft",
		Requester:   "Jane Doe",
		CreatedAt:   time.Now(),
	}

	// Assign batch
	payload := map[string]interface{}{
		"ids": []string{id1.String(), id2.String()},
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch", payload)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())

	if resp["assigned"] != float64(2) {
		t.Errorf("assigned: got %v, want 2", resp["assigned"])
	}

	batchID, ok := resp["batch_id"].(string)
	if !ok || !strings.HasPrefix(batchID, "RMB") {
		t.Errorf("batch_id: got %v, want RMB prefix", batchID)
	}

	// Verify both items now have status "Ready"
	r1 := store.requests[id1]
	r2 := store.requests[id2]

	if r1.Status != "Ready" {
		t.Errorf("request 1 status: got %v, want Ready", r1.Status)
	}
	if r2.Status != "Ready" {
		t.Errorf("request 2 status: got %v, want Ready", r2.Status)
	}
}

func TestBatchAssign_EmptyIDs(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	payload := map[string]interface{}{
		"ids": []string{},
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch", payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestBatchAssign_NonDraftNotCounted(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	// Seed 1 Draft and 1 Ready item
	draftID := uuid.New()
	readyID := uuid.New()
	accountID := uuid.New()

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan("5.0000")
	pricePg.Scan("100000.00")
	amountPg.Scan("500000.00")

	store.requests[draftID] = database.AcctReimbursementRequest{
		ID:          draftID,
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Description: "Draft request",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Draft",
		Requester:   "John Doe",
		CreatedAt:   time.Now(),
	}

	store.requests[readyID] = database.AcctReimbursementRequest{
		ID:          readyID,
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 21, 0, 0, 0, 0, time.UTC), Valid: true},
		Description: "Ready request",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Ready",
		Requester:   "Jane Doe",
		CreatedAt:   time.Now(),
	}

	// POST batch assign with both IDs
	payload := map[string]interface{}{
		"ids": []string{draftID.String(), readyID.String()},
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch", payload)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())

	// Only the Draft one should be assigned
	if resp["assigned"] != float64(1) {
		t.Errorf("assigned: got %v, want 1 (only Draft item)", resp["assigned"])
	}
}

func TestBatchPost_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	// Seed 1 Ready item with batch_id "RMB001"
	id := uuid.New()
	accountID := uuid.New()
	cashAccountID := uuid.New()
	itemID := uuid.New()

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan("5.0000")
	pricePg.Scan("100000.00")
	amountPg.Scan("500000.00")

	store.requests[id] = database.AcctReimbursementRequest{
		ID:          id,
		BatchID:     pgtype.Text{String: "RMB001", Valid: true},
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		ItemID:      pgtype.UUID{Bytes: itemID, Valid: true},
		Description: "Ready request",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Ready",
		Requester:   "John Doe",
		CreatedAt:   time.Now(),
	}

	// POST batch/post
	payload := map[string]interface{}{
		"batch_id":        "RMB001",
		"payment_date":    "2026-01-25",
		"cash_account_id": cashAccountID.String(),
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch/post", payload)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())

	if resp["posted"] != float64(1) {
		t.Errorf("posted: got %v, want 1", resp["posted"])
	}

	// Verify store item now has status "Posted"
	r := store.requests[id]
	if r.Status != "Posted" {
		t.Errorf("request status: got %v, want Posted", r.Status)
	}
	if !r.PostedAt.Valid {
		t.Error("posted_at should be set")
	}

	// Verify 1 cash transaction was created
	if len(store.txns) != 1 {
		t.Fatalf("txns count: got %d, want 1", len(store.txns))
	}

	tx := store.txns[0]
	if tx.ReimbursementBatchID.String != "RMB001" {
		t.Errorf("txn batch_id: got %v, want RMB001", tx.ReimbursementBatchID.String)
	}

	// Verify cash_account_id
	expectedCashPg := pgtype.UUID{Bytes: cashAccountID, Valid: true}
	if tx.CashAccountID != expectedCashPg {
		t.Errorf("txn cash_account_id: got %v, want %v", tx.CashAccountID, expectedCashPg)
	}

	// Verify transactions array in response
	txns, ok := resp["transactions"].([]interface{})
	if !ok || len(txns) != 1 {
		t.Fatalf("transactions: got %v, want array of length 1", resp["transactions"])
	}
	txn := txns[0].(map[string]interface{})
	if txn["transaction_code"] == nil || txn["transaction_code"] == "" {
		t.Error("transaction_code should be set")
	}
	if txn["amount"] != "500000.00" {
		t.Errorf("transaction amount: got %v, want 500000.00", txn["amount"])
	}
}

func TestBatchPost_AlreadyPosted(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	// Seed 1 Posted item with batch_id "RMB001"
	id := uuid.New()
	accountID := uuid.New()

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan("5.0000")
	pricePg.Scan("100000.00")
	amountPg.Scan("500000.00")

	store.requests[id] = database.AcctReimbursementRequest{
		ID:          id,
		BatchID:     pgtype.Text{String: "RMB001", Valid: true},
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Description: "Posted request",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Posted",
		Requester:   "John Doe",
		PostedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CreatedAt:   time.Now(),
	}

	// POST batch/post with same batch_id
	payload := map[string]interface{}{
		"batch_id":        "RMB001",
		"payment_date":    "2026-01-25",
		"cash_account_id": uuid.New().String(),
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch/post", payload)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestBatchPost_EmptyBatch(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	// POST batch/post with batch_id that doesn't exist
	payload := map[string]interface{}{
		"batch_id":        "NONEXISTENT",
		"payment_date":    "2026-01-25",
		"cash_account_id": uuid.New().String(),
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch/post", payload)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestBatchPost_OnlyProcessesReady(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	// Seed 2 items in same batch: 1 Ready, 1 Draft
	readyID := uuid.New()
	draftID := uuid.New()
	accountID := uuid.New()
	cashAccountID := uuid.New()
	itemID := uuid.New()

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan("5.0000")
	pricePg.Scan("100000.00")
	amountPg.Scan("500000.00")

	store.requests[readyID] = database.AcctReimbursementRequest{
		ID:          readyID,
		BatchID:     pgtype.Text{String: "RMB001", Valid: true},
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		ItemID:      pgtype.UUID{Bytes: itemID, Valid: true},
		Description: "Ready request",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Ready",
		Requester:   "John Doe",
		CreatedAt:   time.Now(),
	}

	store.requests[draftID] = database.AcctReimbursementRequest{
		ID:          draftID,
		BatchID:     pgtype.Text{String: "RMB001", Valid: true},
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 21, 0, 0, 0, 0, time.UTC), Valid: true},
		ItemID:      pgtype.UUID{Bytes: itemID, Valid: true},
		Description: "Draft request (shouldn't be in batch)",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Draft",
		Requester:   "Jane Doe",
		CreatedAt:   time.Now(),
	}

	// POST batch/post
	payload := map[string]interface{}{
		"batch_id":        "RMB001",
		"payment_date":    "2026-01-25",
		"cash_account_id": cashAccountID.String(),
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch/post", payload)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())

	// Only the Ready item should be posted
	if resp["posted"] != float64(1) {
		t.Errorf("posted: got %v, want 1 (only Ready item)", resp["posted"])
	}

	// Verify only 1 cash transaction created
	if len(store.txns) != 1 {
		t.Fatalf("txns count: got %d, want 1", len(store.txns))
	}
}

func TestBatchPost_NoReadyItems(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	draftID := uuid.New()
	accountID := uuid.New()

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan("5.0000")
	pricePg.Scan("100000.00")
	amountPg.Scan("500000.00")

	store.requests[draftID] = database.AcctReimbursementRequest{
		ID:          draftID,
		BatchID:     pgtype.Text{String: "RMB001", Valid: true},
		ExpenseDate: pgtype.Date{Time: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Description: "Draft request in batch",
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    "EXPENSE",
		AccountID:   accountID,
		Status:      "Draft",
		Requester:   "John Doe",
		CreatedAt:   time.Now(),
	}

	payload := map[string]interface{}{
		"batch_id":        "RMB001",
		"payment_date":    "2026-01-25",
		"cash_account_id": uuid.New().String(),
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch/post", payload)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusUnprocessableEntity, rr.Body.String())
	}
}
