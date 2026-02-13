package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock Transaction Store ---

type mockTransactionStore struct {
	transactions        []database.AcctCashTransaction
	nextTransactionCode string
}

func newMockTransactionStore() *mockTransactionStore {
	return &mockTransactionStore{
		transactions:        []database.AcctCashTransaction{},
		nextTransactionCode: "PCS000000",
	}
}

func (m *mockTransactionStore) ListAcctCashTransactions(ctx context.Context, arg database.ListAcctCashTransactionsParams) ([]database.AcctCashTransaction, error) {
	return m.transactions, nil
}

func (m *mockTransactionStore) GetAcctCashTransaction(ctx context.Context, id uuid.UUID) (database.AcctCashTransaction, error) {
	for _, tx := range m.transactions {
		if tx.ID == id {
			return tx, nil
		}
	}
	return database.AcctCashTransaction{}, pgx.ErrNoRows
}

func (m *mockTransactionStore) CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error) {
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
	m.transactions = append(m.transactions, tx)
	return tx, nil
}

func (m *mockTransactionStore) GetNextTransactionCode(ctx context.Context) (string, error) {
	return m.nextTransactionCode, nil
}

// --- Helper functions ---

func setupTransactionRouter(store handler.TransactionStore) *chi.Mux {
	h := handler.NewTransactionHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/transactions", h.RegisterRoutes)
	return r
}

// --- Tests ---

func TestListTransactions(t *testing.T) {
	store := newMockTransactionStore()
	accountID := uuid.New()
	cashAccountID := uuid.New()
	outletID := uuid.New()

	store.transactions = []database.AcctCashTransaction{
		{
			ID:              uuid.New(),
			TransactionCode: "PCS000001",
			TransactionDate: makePgDate(2026, 1, 20),
			Description:     "Bank transfer for equipment",
			Quantity:        makePgNumeric("1.00"),
			UnitPrice:       makePgNumeric("5000000.00"),
			Amount:          makePgNumeric("5000000.00"),
			LineType:        "ASSET",
			AccountID:       accountID,
			CashAccountID:   makePgUUID(cashAccountID),
			OutletID:        makePgUUID(outletID),
			CreatedAt:       time.Now(),
		},
		{
			ID:              uuid.New(),
			TransactionCode: "PCS000002",
			TransactionDate: makePgDate(2026, 1, 21),
			Description:     "Owner drawing",
			Quantity:        makePgNumeric("1.00"),
			UnitPrice:       makePgNumeric("2000000.00"),
			Amount:          makePgNumeric("2000000.00"),
			LineType:        "DRAWING",
			AccountID:       accountID,
			CashAccountID:   makePgUUID(cashAccountID),
			CreatedAt:       time.Now(),
		},
	}

	router := setupTransactionRouter(store)

	req := httptest.NewRequest("GET", "/accounting/transactions/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(resp))
	}

	if resp[0]["transaction_code"] != "PCS000001" {
		t.Errorf("expected transaction_code 'PCS000001', got %v", resp[0]["transaction_code"])
	}
	if resp[0]["line_type"] != "ASSET" {
		t.Errorf("expected line_type 'ASSET', got %v", resp[0]["line_type"])
	}
	if resp[1]["line_type"] != "DRAWING" {
		t.Errorf("expected line_type 'DRAWING', got %v", resp[1]["line_type"])
	}
}

func TestListTransactions_WithFilters(t *testing.T) {
	store := newMockTransactionStore()
	accountID := uuid.New()
	cashAccountID := uuid.New()

	store.transactions = []database.AcctCashTransaction{
		{
			ID:              uuid.New(),
			TransactionCode: "PCS000001",
			TransactionDate: makePgDate(2026, 1, 20),
			Description:     "Equipment purchase",
			Quantity:        makePgNumeric("1.00"),
			UnitPrice:       makePgNumeric("5000000.00"),
			Amount:          makePgNumeric("5000000.00"),
			LineType:        "ASSET",
			AccountID:       accountID,
			CashAccountID:   makePgUUID(cashAccountID),
			CreatedAt:       time.Now(),
		},
	}

	router := setupTransactionRouter(store)

	// Test with date range filter
	req := httptest.NewRequest("GET", "/accounting/transactions/?start_date=2026-01-20&end_date=2026-01-20", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test with line_type filter
	req = httptest.NewRequest("GET", "/accounting/transactions/?line_type=ASSET", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test with search filter
	req = httptest.NewRequest("GET", "/accounting/transactions/?search=equipment", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(resp))
	}
}

func TestGetTransaction(t *testing.T) {
	store := newMockTransactionStore()
	accountID := uuid.New()
	cashAccountID := uuid.New()
	txID := uuid.New()

	store.transactions = []database.AcctCashTransaction{
		{
			ID:              txID,
			TransactionCode: "PCS000001",
			TransactionDate: makePgDate(2026, 1, 20),
			Description:     "Equipment purchase",
			Quantity:        makePgNumeric("1.00"),
			UnitPrice:       makePgNumeric("5000000.00"),
			Amount:          makePgNumeric("5000000.00"),
			LineType:        "ASSET",
			AccountID:       accountID,
			CashAccountID:   makePgUUID(cashAccountID),
			CreatedAt:       time.Now(),
		},
	}

	router := setupTransactionRouter(store)

	req := httptest.NewRequest("GET", fmt.Sprintf("/accounting/transactions/%s", txID.String()), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["transaction_code"] != "PCS000001" {
		t.Errorf("expected transaction_code 'PCS000001', got %v", resp["transaction_code"])
	}
	if resp["description"] != "Equipment purchase" {
		t.Errorf("expected description 'Equipment purchase', got %v", resp["description"])
	}
	if resp["line_type"] != "ASSET" {
		t.Errorf("expected line_type 'ASSET', got %v", resp["line_type"])
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	store := newMockTransactionStore()
	router := setupTransactionRouter(store)

	txID := uuid.New()

	req := httptest.NewRequest("GET", fmt.Sprintf("/accounting/transactions/%s", txID.String()), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "transaction not found" {
		t.Errorf("expected error 'transaction not found', got %v", resp["error"])
	}
}

func TestCreateTransaction(t *testing.T) {
	store := newMockTransactionStore()
	router := setupTransactionRouter(store)

	accountID := uuid.New()
	cashAccountID := uuid.New()
	outletID := uuid.New()
	itemID := uuid.New()

	reqBody := map[string]interface{}{
		"transaction_date": "2026-01-20",
		"description":      "Bank transfer for equipment",
		"quantity":         "1.00",
		"unit_price":       "5000000.00",
		"line_type":        "ASSET",
		"account_id":       accountID.String(),
		"cash_account_id":  cashAccountID.String(),
		"outlet_id":        outletID.String(),
		"item_id":          itemID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/transactions/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["transaction_code"] != "PCS000001" {
		t.Errorf("expected transaction_code 'PCS000001', got %v", resp["transaction_code"])
	}
	if resp["description"] != "Bank transfer for equipment" {
		t.Errorf("expected description 'Bank transfer for equipment', got %v", resp["description"])
	}
	if resp["line_type"] != "ASSET" {
		t.Errorf("expected line_type 'ASSET', got %v", resp["line_type"])
	}
	if resp["quantity"] != "1.00" {
		t.Errorf("expected quantity '1.00', got %v", resp["quantity"])
	}
	if resp["unit_price"] != "5000000.00" {
		t.Errorf("expected unit_price '5000000.00', got %v", resp["unit_price"])
	}
	if resp["amount"] != "5000000.00" {
		t.Errorf("expected amount '5000000.00', got %v", resp["amount"])
	}

	// Verify all optional fields are present
	if resp["cash_account_id"] == nil {
		t.Error("expected cash_account_id to be present")
	}
	if resp["outlet_id"] == nil {
		t.Error("expected outlet_id to be present")
	}
	if resp["item_id"] == nil {
		t.Error("expected item_id to be present")
	}
}

func TestCreateTransaction_MissingFields(t *testing.T) {
	store := newMockTransactionStore()
	router := setupTransactionRouter(store)

	// Missing required fields
	reqBody := map[string]interface{}{
		"transaction_date": "2026-01-20",
		"description":      "Bank transfer",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/transactions/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	expectedErr := "transaction_date, description, quantity, unit_price, line_type, and account_id are required"
	if resp["error"] != expectedErr {
		t.Errorf("expected error '%s', got %v", expectedErr, resp["error"])
	}
}

func TestCreateTransaction_InvalidLineType(t *testing.T) {
	store := newMockTransactionStore()
	router := setupTransactionRouter(store)

	accountID := uuid.New()

	reqBody := map[string]interface{}{
		"transaction_date": "2026-01-20",
		"description":      "Bank transfer",
		"quantity":         "1.00",
		"unit_price":       "5000000.00",
		"line_type":        "INVALID_TYPE",
		"account_id":       accountID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/transactions/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "invalid line_type" {
		t.Errorf("expected error 'invalid line_type', got %v", resp["error"])
	}
}
