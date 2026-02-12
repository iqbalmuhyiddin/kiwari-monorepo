package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
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
