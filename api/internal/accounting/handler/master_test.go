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
	"github.com/jackc/pgx/v5"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock AcctAccountStore ---

type mockAcctAccountStore struct {
	accounts map[uuid.UUID]database.AcctAccount
}

func newMockAcctAccountStore() *mockAcctAccountStore {
	return &mockAcctAccountStore{accounts: make(map[uuid.UUID]database.AcctAccount)}
}

func (m *mockAcctAccountStore) ListAcctAccounts(_ context.Context) ([]database.AcctAccount, error) {
	var result []database.AcctAccount
	for _, a := range m.accounts {
		if a.IsActive {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockAcctAccountStore) GetAcctAccount(_ context.Context, id uuid.UUID) (database.AcctAccount, error) {
	a, ok := m.accounts[id]
	if !ok || !a.IsActive {
		return database.AcctAccount{}, pgx.ErrNoRows
	}
	return a, nil
}

func (m *mockAcctAccountStore) CreateAcctAccount(_ context.Context, arg database.CreateAcctAccountParams) (database.AcctAccount, error) {
	a := database.AcctAccount{
		ID:          uuid.New(),
		AccountCode: arg.AccountCode,
		AccountName: arg.AccountName,
		AccountType: arg.AccountType,
		LineType:    arg.LineType,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	m.accounts[a.ID] = a
	return a, nil
}

func (m *mockAcctAccountStore) UpdateAcctAccount(_ context.Context, arg database.UpdateAcctAccountParams) (database.AcctAccount, error) {
	a, ok := m.accounts[arg.ID]
	if !ok || !a.IsActive {
		return database.AcctAccount{}, pgx.ErrNoRows
	}
	a.AccountName = arg.AccountName
	a.AccountType = arg.AccountType
	a.LineType = arg.LineType
	m.accounts[a.ID] = a
	return a, nil
}

func (m *mockAcctAccountStore) SoftDeleteAcctAccount(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	a, ok := m.accounts[id]
	if !ok || !a.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	a.IsActive = false
	m.accounts[a.ID] = a
	return a.ID, nil
}

// --- Mock AcctItemStore ---

type mockAcctItemStore struct {
	items map[uuid.UUID]database.AcctItem
}

func newMockAcctItemStore() *mockAcctItemStore {
	return &mockAcctItemStore{items: make(map[uuid.UUID]database.AcctItem)}
}

func (m *mockAcctItemStore) ListAcctItems(_ context.Context) ([]database.AcctItem, error) {
	var result []database.AcctItem
	for _, i := range m.items {
		if i.IsActive {
			result = append(result, i)
		}
	}
	return result, nil
}

func (m *mockAcctItemStore) GetAcctItem(_ context.Context, id uuid.UUID) (database.AcctItem, error) {
	i, ok := m.items[id]
	if !ok || !i.IsActive {
		return database.AcctItem{}, pgx.ErrNoRows
	}
	return i, nil
}

func (m *mockAcctItemStore) CreateAcctItem(_ context.Context, arg database.CreateAcctItemParams) (database.AcctItem, error) {
	i := database.AcctItem{
		ID:           uuid.New(),
		ItemCode:     arg.ItemCode,
		ItemName:     arg.ItemName,
		ItemCategory: arg.ItemCategory,
		Unit:         arg.Unit,
		IsInventory:  arg.IsInventory,
		AveragePrice: arg.AveragePrice,
		LastPrice:    arg.LastPrice,
		ForHpp:       arg.ForHpp,
		Keywords:     arg.Keywords,
		IsActive:     true,
		CreatedAt:    time.Now(),
	}
	m.items[i.ID] = i
	return i, nil
}

func (m *mockAcctItemStore) UpdateAcctItem(_ context.Context, arg database.UpdateAcctItemParams) (database.AcctItem, error) {
	i, ok := m.items[arg.ID]
	if !ok || !i.IsActive {
		return database.AcctItem{}, pgx.ErrNoRows
	}
	i.ItemName = arg.ItemName
	i.ItemCategory = arg.ItemCategory
	i.Unit = arg.Unit
	i.IsInventory = arg.IsInventory
	i.AveragePrice = arg.AveragePrice
	i.LastPrice = arg.LastPrice
	i.ForHpp = arg.ForHpp
	i.Keywords = arg.Keywords
	m.items[i.ID] = i
	return i, nil
}

func (m *mockAcctItemStore) SoftDeleteAcctItem(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	i, ok := m.items[id]
	if !ok || !i.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	i.IsActive = false
	m.items[i.ID] = i
	return i.ID, nil
}

// --- Mock AcctCashAccountStore ---

type mockAcctCashAccountStore struct {
	cashAccounts map[uuid.UUID]database.AcctCashAccount
}

func newMockAcctCashAccountStore() *mockAcctCashAccountStore {
	return &mockAcctCashAccountStore{cashAccounts: make(map[uuid.UUID]database.AcctCashAccount)}
}

func (m *mockAcctCashAccountStore) ListAcctCashAccounts(_ context.Context) ([]database.AcctCashAccount, error) {
	var result []database.AcctCashAccount
	for _, c := range m.cashAccounts {
		if c.IsActive {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockAcctCashAccountStore) GetAcctCashAccount(_ context.Context, id uuid.UUID) (database.AcctCashAccount, error) {
	c, ok := m.cashAccounts[id]
	if !ok || !c.IsActive {
		return database.AcctCashAccount{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *mockAcctCashAccountStore) CreateAcctCashAccount(_ context.Context, arg database.CreateAcctCashAccountParams) (database.AcctCashAccount, error) {
	c := database.AcctCashAccount{
		ID:              uuid.New(),
		CashAccountCode: arg.CashAccountCode,
		CashAccountName: arg.CashAccountName,
		BankName:        arg.BankName,
		Ownership:       arg.Ownership,
		IsActive:        true,
		CreatedAt:       time.Now(),
	}
	m.cashAccounts[c.ID] = c
	return c, nil
}

func (m *mockAcctCashAccountStore) UpdateAcctCashAccount(_ context.Context, arg database.UpdateAcctCashAccountParams) (database.AcctCashAccount, error) {
	c, ok := m.cashAccounts[arg.ID]
	if !ok || !c.IsActive {
		return database.AcctCashAccount{}, pgx.ErrNoRows
	}
	c.CashAccountName = arg.CashAccountName
	c.BankName = arg.BankName
	c.Ownership = arg.Ownership
	m.cashAccounts[c.ID] = c
	return c, nil
}

func (m *mockAcctCashAccountStore) SoftDeleteAcctCashAccount(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	c, ok := m.cashAccounts[id]
	if !ok || !c.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	c.IsActive = false
	m.cashAccounts[c.ID] = c
	return c.ID, nil
}

// --- Helpers ---

func doRequest(t *testing.T, router http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func setupAccountRouter(store handler.AcctAccountStore) *chi.Mux {
	h := handler.NewMasterHandler(store, nil, nil)
	r := chi.NewRouter()
	r.Route("/accounting/master/accounts", h.RegisterAccountRoutes)
	return r
}

func setupItemRouter(store handler.AcctItemStore) *chi.Mux {
	h := handler.NewMasterHandler(nil, store, nil)
	r := chi.NewRouter()
	r.Route("/accounting/master/items", h.RegisterItemRoutes)
	return r
}

func setupCashAccountRouter(store handler.AcctCashAccountStore) *chi.Mux {
	h := handler.NewMasterHandler(nil, nil, store)
	r := chi.NewRouter()
	r.Route("/accounting/master/cash-accounts", h.RegisterCashAccountRoutes)
	return r
}

// --- Account Tests ---

func TestAccountList_Empty(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)

	rr := doRequest(t, router, "GET", "/accounting/master/accounts", nil)

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

func TestAccountCreate_Valid(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)

	payload := map[string]interface{}{
		"account_code": "1101",
		"account_name": "Cash on Hand",
		"account_type": "Asset",
		"line_type":    "ASSET",
	}

	rr := doRequest(t, router, "POST", "/accounting/master/accounts", payload)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["account_code"] != "1101" {
		t.Errorf("account_code: got %v, want 1101", resp["account_code"])
	}
	if resp["account_name"] != "Cash on Hand" {
		t.Errorf("account_name: got %v, want Cash on Hand", resp["account_name"])
	}
	if resp["is_active"] != true {
		t.Errorf("is_active: got %v, want true", resp["is_active"])
	}
}

func TestAccountCreate_MissingCode(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)

	payload := map[string]interface{}{
		"account_name": "Cash on Hand",
		"account_type": "Asset",
		"line_type":    "ASSET",
	}

	rr := doRequest(t, router, "POST", "/accounting/master/accounts", payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestAccountCreate_InvalidType(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)

	payload := map[string]interface{}{
		"account_code": "1101",
		"account_name": "Cash on Hand",
		"account_type": "InvalidType",
		"line_type":    "ASSET",
	}

	rr := doRequest(t, router, "POST", "/accounting/master/accounts", payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestAccountUpdate_Valid(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)

	// Create account first
	id := uuid.New()
	store.accounts[id] = database.AcctAccount{
		ID:          id,
		AccountCode: "1101",
		AccountName: "Cash on Hand",
		AccountType: "Asset",
		LineType:    "ASSET",
		IsActive:    true,
		CreatedAt:   time.Now(),
	}

	payload := map[string]interface{}{
		"account_name": "Cash in Bank",
		"account_type": "Asset",
		"line_type":    "ASSET",
	}

	rr := doRequest(t, router, "PUT", "/accounting/master/accounts/"+id.String(), payload)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["account_name"] != "Cash in Bank" {
		t.Errorf("account_name: got %v, want Cash in Bank", resp["account_name"])
	}
}

func TestAccountUpdate_NotFound(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)

	payload := map[string]interface{}{
		"account_name": "Cash in Bank",
		"account_type": "Asset",
		"line_type":    "ASSET",
	}

	rr := doRequest(t, router, "PUT", "/accounting/master/accounts/"+uuid.New().String(), payload)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestAccountDelete_Valid(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)

	// Create account first
	id := uuid.New()
	store.accounts[id] = database.AcctAccount{
		ID:          id,
		AccountCode: "1101",
		AccountName: "Cash on Hand",
		AccountType: "Asset",
		LineType:    "ASSET",
		IsActive:    true,
		CreatedAt:   time.Now(),
	}

	rr := doRequest(t, router, "DELETE", "/accounting/master/accounts/"+id.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}
}

func TestAccountDelete_NotFound(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)

	rr := doRequest(t, router, "DELETE", "/accounting/master/accounts/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// --- Item Tests ---

func TestItemList_Empty(t *testing.T) {
	store := newMockAcctItemStore()
	router := setupItemRouter(store)

	rr := doRequest(t, router, "GET", "/accounting/master/items", nil)

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

func TestItemCreate_Valid(t *testing.T) {
	store := newMockAcctItemStore()
	router := setupItemRouter(store)

	payload := map[string]interface{}{
		"item_code":     "RM001",
		"item_name":     "Rice",
		"item_category": "Raw Material",
		"unit":          "kg",
		"is_inventory":  true,
		"average_price": "10000",
		"last_price":    "11000",
		"for_hpp":       "10500",
		"keywords":      "beras nasi",
	}

	rr := doRequest(t, router, "POST", "/accounting/master/items", payload)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["item_code"] != "RM001" {
		t.Errorf("item_code: got %v, want RM001", resp["item_code"])
	}
	if resp["item_name"] != "Rice" {
		t.Errorf("item_name: got %v, want Rice", resp["item_name"])
	}
}

func TestItemCreate_MissingCode(t *testing.T) {
	store := newMockAcctItemStore()
	router := setupItemRouter(store)

	payload := map[string]interface{}{
		"item_name":     "Rice",
		"item_category": "Raw Material",
		"unit":          "kg",
		"is_inventory":  true,
		"keywords":      "beras nasi",
	}

	rr := doRequest(t, router, "POST", "/accounting/master/items", payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestItemDelete_NotFound(t *testing.T) {
	store := newMockAcctItemStore()
	router := setupItemRouter(store)

	rr := doRequest(t, router, "DELETE", "/accounting/master/items/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// --- Cash Account Tests ---

func TestCashAccountList_Empty(t *testing.T) {
	store := newMockAcctCashAccountStore()
	router := setupCashAccountRouter(store)

	rr := doRequest(t, router, "GET", "/accounting/master/cash-accounts", nil)

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

func TestCashAccountCreate_Valid(t *testing.T) {
	store := newMockAcctCashAccountStore()
	router := setupCashAccountRouter(store)

	payload := map[string]interface{}{
		"cash_account_code": "CA001",
		"cash_account_name": "BCA Main Account",
		"bank_name":         "BCA",
		"ownership":         "Business",
	}

	rr := doRequest(t, router, "POST", "/accounting/master/cash-accounts", payload)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["cash_account_code"] != "CA001" {
		t.Errorf("cash_account_code: got %v, want CA001", resp["cash_account_code"])
	}
	if resp["cash_account_name"] != "BCA Main Account" {
		t.Errorf("cash_account_name: got %v, want BCA Main Account", resp["cash_account_name"])
	}
}

func TestCashAccountCreate_MissingCode(t *testing.T) {
	store := newMockAcctCashAccountStore()
	router := setupCashAccountRouter(store)

	payload := map[string]interface{}{
		"cash_account_name": "BCA Main Account",
		"bank_name":         "BCA",
		"ownership":         "Business",
	}

	rr := doRequest(t, router, "POST", "/accounting/master/cash-accounts", payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestCashAccountDelete_NotFound(t *testing.T) {
	store := newMockAcctCashAccountStore()
	router := setupCashAccountRouter(store)

	rr := doRequest(t, router, "DELETE", "/accounting/master/cash-accounts/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}
