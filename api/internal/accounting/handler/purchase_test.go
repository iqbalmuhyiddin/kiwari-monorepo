package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
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

// --- Mock Purchase Store ---

type mockPurchaseStore struct {
	transactions []database.AcctCashTransaction
	nextCode     string
	lastPrices   map[uuid.UUID]pgtype.Numeric
}

func newMockPurchaseStore() *mockPurchaseStore {
	return &mockPurchaseStore{
		transactions: []database.AcctCashTransaction{},
		nextCode:     "PCS000000",
		lastPrices:   make(map[uuid.UUID]pgtype.Numeric),
	}
}

func (m *mockPurchaseStore) CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error) {
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
	// Track the latest code
	m.nextCode = arg.TransactionCode
	return tx, nil
}

func (m *mockPurchaseStore) GetNextTransactionCode(ctx context.Context) (string, error) {
	return m.nextCode, nil
}

func (m *mockPurchaseStore) UpdateAcctItemLastPrice(ctx context.Context, arg database.UpdateAcctItemLastPriceParams) error {
	m.lastPrices[arg.ID] = arg.LastPrice
	return nil
}

// --- Helper functions ---

func setupPurchaseRouter(store handler.PurchaseStore) *chi.Mux {
	h := handler.NewPurchaseHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/purchases", h.RegisterRoutes)
	return r
}

// --- Tests ---

func TestCreatePurchase_SingleItem(t *testing.T) {
	store := newMockPurchaseStore()
	router := setupPurchaseRouter(store)

	accountID := uuid.New()
	cashAccountID := uuid.New()

	reqBody := map[string]interface{}{
		"transaction_date": "2026-01-20",
		"account_id":       accountID.String(),
		"cash_account_id":  cashAccountID.String(),
		"items": []map[string]interface{}{
			{
				"description": "Minyak Goreng 1L",
				"quantity":    "2.00",
				"unit_price":  "25000.00",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/purchases/", bytes.NewBuffer(body))
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

	transactions, ok := resp["transactions"].([]interface{})
	if !ok || len(transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %v", resp)
	}

	tx := transactions[0].(map[string]interface{})
	if tx["transaction_code"] != "PCS000001" {
		t.Errorf("expected code PCS000001, got %v", tx["transaction_code"])
	}
	if tx["description"] != "Minyak Goreng 1L" {
		t.Errorf("expected description 'Minyak Goreng 1L', got %v", tx["description"])
	}
	if tx["quantity"] != "2.00" {
		t.Errorf("expected quantity '2.00', got %v", tx["quantity"])
	}
	if tx["unit_price"] != "25000.00" {
		t.Errorf("expected unit_price '25000.00', got %v", tx["unit_price"])
	}
	if tx["amount"] != "50000.00" {
		t.Errorf("expected amount '50000.00', got %v", tx["amount"])
	}
	if tx["line_type"] != "INVENTORY" {
		t.Errorf("expected line_type 'INVENTORY', got %v", tx["line_type"])
	}
}

func TestCreatePurchase_MultiItem(t *testing.T) {
	store := newMockPurchaseStore()
	router := setupPurchaseRouter(store)

	accountID := uuid.New()
	cashAccountID := uuid.New()

	reqBody := map[string]interface{}{
		"transaction_date": "2026-01-20",
		"account_id":       accountID.String(),
		"cash_account_id":  cashAccountID.String(),
		"items": []map[string]interface{}{
			{
				"description": "Beras 5kg",
				"quantity":    "3.00",
				"unit_price":  "60000.00",
			},
			{
				"description": "Gula 1kg",
				"quantity":    "5.00",
				"unit_price":  "15000.00",
			},
			{
				"description": "Garam 500g",
				"quantity":    "2.00",
				"unit_price":  "5000.00",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/purchases/", bytes.NewBuffer(body))
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

	transactions, ok := resp["transactions"].([]interface{})
	if !ok || len(transactions) != 3 {
		t.Fatalf("expected 3 transactions, got %v", resp)
	}

	// Check transaction codes
	codes := []string{"PCS000001", "PCS000002", "PCS000003"}
	for i, tx := range transactions {
		txMap := tx.(map[string]interface{})
		if txMap["transaction_code"] != codes[i] {
			t.Errorf("transaction %d: expected code %s, got %v", i, codes[i], txMap["transaction_code"])
		}
	}

	// Check amounts
	expectedAmounts := []string{"180000.00", "75000.00", "10000.00"}
	for i, tx := range transactions {
		txMap := tx.(map[string]interface{})
		if txMap["amount"] != expectedAmounts[i] {
			t.Errorf("transaction %d: expected amount %s, got %v", i, expectedAmounts[i], txMap["amount"])
		}
	}
}

func TestCreatePurchase_MissingDate(t *testing.T) {
	store := newMockPurchaseStore()
	router := setupPurchaseRouter(store)

	accountID := uuid.New()
	cashAccountID := uuid.New()

	reqBody := map[string]interface{}{
		"account_id":      accountID.String(),
		"cash_account_id": cashAccountID.String(),
		"items": []map[string]interface{}{
			{
				"description": "Test Item",
				"quantity":    "1.00",
				"unit_price":  "10000.00",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/purchases/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreatePurchase_MissingCashAccount(t *testing.T) {
	store := newMockPurchaseStore()
	router := setupPurchaseRouter(store)

	accountID := uuid.New()

	reqBody := map[string]interface{}{
		"transaction_date": "2026-01-20",
		"account_id":       accountID.String(),
		"items": []map[string]interface{}{
			{
				"description": "Test Item",
				"quantity":    "1.00",
				"unit_price":  "10000.00",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/purchases/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreatePurchase_EmptyItems(t *testing.T) {
	store := newMockPurchaseStore()
	router := setupPurchaseRouter(store)

	accountID := uuid.New()
	cashAccountID := uuid.New()

	reqBody := map[string]interface{}{
		"transaction_date": "2026-01-20",
		"account_id":       accountID.String(),
		"cash_account_id":  cashAccountID.String(),
		"items":            []map[string]interface{}{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/purchases/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestCreatePurchase_UpdatesLastPrice(t *testing.T) {
	store := newMockPurchaseStore()
	router := setupPurchaseRouter(store)

	accountID := uuid.New()
	cashAccountID := uuid.New()
	itemID := uuid.New()

	reqBody := map[string]interface{}{
		"transaction_date": "2026-01-20",
		"account_id":       accountID.String(),
		"cash_account_id":  cashAccountID.String(),
		"items": []map[string]interface{}{
			{
				"item_id":     itemID.String(),
				"description": "Test Item with ID",
				"quantity":    "2.00",
				"unit_price":  "35000.00",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/purchases/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify last price was updated
	lastPrice, exists := store.lastPrices[itemID]
	if !exists {
		t.Fatal("expected last price to be updated")
	}

	val, err := lastPrice.Value()
	if err != nil {
		t.Fatalf("failed to get last price value: %v", err)
	}

	if val.(string) != "35000.00" {
		t.Errorf("expected last price '35000.00', got %v", val)
	}
}
