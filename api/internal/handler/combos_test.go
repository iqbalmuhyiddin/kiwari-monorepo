package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/handler"
)

// --- Mock store ---

type mockComboStore struct {
	products   map[uuid.UUID]database.Product
	comboItems map[uuid.UUID]database.ComboItem
	fkError    bool
}

func newMockComboStore() *mockComboStore {
	return &mockComboStore{
		products:   make(map[uuid.UUID]database.Product),
		comboItems: make(map[uuid.UUID]database.ComboItem),
	}
}

func (m *mockComboStore) GetProduct(_ context.Context, arg database.GetProductParams) (database.Product, error) {
	p, ok := m.products[arg.ID]
	if !ok || p.OutletID != arg.OutletID || !p.IsActive {
		return database.Product{}, pgx.ErrNoRows
	}
	return p, nil
}

func (m *mockComboStore) ListComboItemsByCombo(_ context.Context, comboID uuid.UUID) ([]database.ComboItem, error) {
	var result []database.ComboItem
	for _, ci := range m.comboItems {
		if ci.ComboID == comboID {
			result = append(result, ci)
		}
	}
	return result, nil
}

func (m *mockComboStore) CreateComboItem(_ context.Context, arg database.CreateComboItemParams) (database.ComboItem, error) {
	if m.fkError {
		return database.ComboItem{}, &pgconn.PgError{Code: "23503"}
	}
	ci := database.ComboItem{
		ID:        uuid.New(),
		ComboID:   arg.ComboID,
		ProductID: arg.ProductID,
		Quantity:  arg.Quantity,
		SortOrder: arg.SortOrder,
	}
	m.comboItems[ci.ID] = ci
	return ci, nil
}

func (m *mockComboStore) DeleteComboItem(_ context.Context, arg database.DeleteComboItemParams) (int64, error) {
	ci, ok := m.comboItems[arg.ID]
	if !ok || ci.ComboID != arg.ComboID {
		return 0, nil
	}
	delete(m.comboItems, arg.ID)
	return 1, nil
}

// --- Helpers ---

func setupComboRouter(store *mockComboStore) *chi.Mux {
	h := handler.NewComboHandler(store)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/products/{pid}", h.RegisterRoutes)
	return r
}

func decodeComboResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeComboListResponse(t *testing.T, rr *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// seedComboProduct creates a combo product in the mock store and returns its ID.
func seedComboProduct(store *mockComboStore, outletID uuid.UUID) uuid.UUID {
	prodID := uuid.New()
	store.products[prodID] = database.Product{
		ID:       prodID,
		OutletID: outletID,
		Name:     "Combo Meal",
		IsCombo:  true,
		IsActive: true,
	}
	return prodID
}

// seedNonComboProduct creates a regular (non-combo) product in the mock store and returns its ID.
func seedNonComboProduct(store *mockComboStore, outletID uuid.UUID, name string) uuid.UUID {
	prodID := uuid.New()
	store.products[prodID] = database.Product{
		ID:       prodID,
		OutletID: outletID,
		Name:     name,
		IsCombo:  false,
		IsActive: true,
	}
	return prodID
}

// seedComboItem creates a combo item in the mock store and returns its ID.
func seedComboItem(store *mockComboStore, comboID, childProductID uuid.UUID, quantity, sortOrder int32) uuid.UUID {
	ciID := uuid.New()
	store.comboItems[ciID] = database.ComboItem{
		ID:        ciID,
		ComboID:   comboID,
		ProductID: childProductID,
		Quantity:  quantity,
		SortOrder: sortOrder,
	}
	return ciID
}

// ========================
// Combo Item: List
// ========================

func TestComboItemList_Empty(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeComboListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestComboItemList_ReturnsItems(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	child1 := seedNonComboProduct(store, outletID, "Nasi Bakar")
	child2 := seedNonComboProduct(store, outletID, "Es Teh")
	seedComboItem(store, comboID, child1, 1, 0)
	seedComboItem(store, comboID, child2, 1, 1)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeComboListResponse(t, rr)
	if len(resp) != 2 {
		t.Fatalf("expected 2 combo items, got %d", len(resp))
	}
}

func TestComboItemList_ProductNotFound(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	router := setupComboRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+uuid.New().String()+"/combo-items", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestComboItemList_NotACombo(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	prodID := seedNonComboProduct(store, outletID, "Regular Product")
	router := setupComboRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/combo-items", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "product is not a combo" {
		t.Errorf("error: got %v, want 'product is not a combo'", resp["error"])
	}
}

func TestComboItemList_WrongOutlet(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+wrongOutletID.String()+"/products/"+comboID.String()+"/combo-items", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestComboItemList_InvalidOutletID(t *testing.T) {
	store := newMockComboStore()
	router := setupComboRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/not-a-uuid/products/"+uuid.New().String()+"/combo-items", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestComboItemList_InvalidProductID(t *testing.T) {
	store := newMockComboStore()
	router := setupComboRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+uuid.New().String()+"/products/not-a-uuid/combo-items", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Combo Item: Create
// ========================

func TestComboItemCreate_Valid(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	childID := seedNonComboProduct(store, outletID, "Nasi Bakar")
	router := setupComboRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"product_id": childID.String(),
		"quantity":   2,
		"sort_order": 1,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["combo_id"] != comboID.String() {
		t.Errorf("combo_id: got %v, want %s", resp["combo_id"], comboID.String())
	}
	if resp["product_id"] != childID.String() {
		t.Errorf("product_id: got %v, want %s", resp["product_id"], childID.String())
	}
	if resp["quantity"] != float64(2) {
		t.Errorf("quantity: got %v, want 2", resp["quantity"])
	}
	if resp["sort_order"] != float64(1) {
		t.Errorf("sort_order: got %v, want 1", resp["sort_order"])
	}
}

func TestComboItemCreate_DefaultQuantity(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	childID := seedNonComboProduct(store, outletID, "Nasi Bakar")
	router := setupComboRouter(store)

	// quantity defaults to 1 when not specified (zero value)
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"product_id": childID.String(),
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["quantity"] != float64(1) {
		t.Errorf("quantity: got %v, want 1 (default)", resp["quantity"])
	}
	if resp["sort_order"] != float64(0) {
		t.Errorf("sort_order: got %v, want 0 (default)", resp["sort_order"])
	}
}

func TestComboItemCreate_MissingProductID(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"quantity": 1,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "product_id is required" {
		t.Errorf("error: got %v, want 'product_id is required'", resp["error"])
	}
}

func TestComboItemCreate_InvalidProductID(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"product_id": "not-a-uuid",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "invalid product_id" {
		t.Errorf("error: got %v, want 'invalid product_id'", resp["error"])
	}
}

func TestComboItemCreate_SelfReference(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	// A combo cannot contain itself
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"product_id": comboID.String(),
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "combo cannot contain itself" {
		t.Errorf("error: got %v, want 'combo cannot contain itself'", resp["error"])
	}
}

func TestComboItemCreate_ChildNotFound(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	// Child product does not exist
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"product_id": uuid.New().String(),
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "child product not found in this outlet" {
		t.Errorf("error: got %v, want 'child product not found in this outlet'", resp["error"])
	}
}

func TestComboItemCreate_ChildInDifferentOutlet(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	otherOutletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	childID := seedNonComboProduct(store, otherOutletID, "Other Outlet Product")
	router := setupComboRouter(store)

	// Child product exists but in a different outlet
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"product_id": childID.String(),
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "child product not found in this outlet" {
		t.Errorf("error: got %v, want 'child product not found in this outlet'", resp["error"])
	}
}

func TestComboItemCreate_NotACombo(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	prodID := seedNonComboProduct(store, outletID, "Regular Product")
	childID := seedNonComboProduct(store, outletID, "Child Product")
	router := setupComboRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/combo-items", map[string]interface{}{
		"product_id": childID.String(),
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "product is not a combo" {
		t.Errorf("error: got %v, want 'product is not a combo'", resp["error"])
	}
}

func TestComboItemCreate_ProductNotFound(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	router := setupComboRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+uuid.New().String()+"/combo-items", map[string]interface{}{
		"product_id": uuid.New().String(),
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestComboItemCreate_WrongOutlet(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	childID := seedNonComboProduct(store, outletID, "Nasi Bakar")
	router := setupComboRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+wrongOutletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"product_id": childID.String(),
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestComboItemCreate_InvalidBody(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", "not json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestComboItemCreate_ForeignKeyViolation(t *testing.T) {
	store := newMockComboStore()
	store.fkError = true
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	childID := seedNonComboProduct(store, outletID, "Nasi Bakar")
	router := setupComboRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items", map[string]interface{}{
		"product_id": childID.String(),
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "invalid product_id" {
		t.Errorf("error: got %v, want 'invalid product_id'", resp["error"])
	}
}

// ========================
// Combo Item: Delete
// ========================

func TestComboItemDelete_Valid(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	childID := seedNonComboProduct(store, outletID, "Nasi Bakar")
	ciID := seedComboItem(store, comboID, childID, 1, 0)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items/"+ciID.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	// Verify it was actually deleted (hard delete)
	if _, exists := store.comboItems[ciID]; exists {
		t.Error("expected combo item to be hard deleted, but it still exists")
	}
}

func TestComboItemDelete_NotFound(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestComboItemDelete_WrongCombo(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	otherComboID := seedComboProduct(store, outletID)
	childID := seedNonComboProduct(store, outletID, "Nasi Bakar")
	ciID := seedComboItem(store, comboID, childID, 1, 0)
	router := setupComboRouter(store)

	// Try to delete combo item using a different combo product
	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+otherComboID.String()+"/combo-items/"+ciID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original still exists
	if _, exists := store.comboItems[ciID]; !exists {
		t.Error("combo item should not be affected by wrong-combo delete")
	}
}

func TestComboItemDelete_WrongOutlet(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	childID := seedNonComboProduct(store, outletID, "Nasi Bakar")
	ciID := seedComboItem(store, comboID, childID, 1, 0)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+wrongOutletID.String()+"/products/"+comboID.String()+"/combo-items/"+ciID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original still exists
	if _, exists := store.comboItems[ciID]; !exists {
		t.Error("combo item should not be affected by wrong-outlet delete")
	}
}

func TestComboItemDelete_NotACombo(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	prodID := seedNonComboProduct(store, outletID, "Regular Product")
	router := setupComboRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/combo-items/"+uuid.New().String(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeComboResponse(t, rr)
	if resp["error"] != "product is not a combo" {
		t.Errorf("error: got %v, want 'product is not a combo'", resp["error"])
	}
}

func TestComboItemDelete_InvalidComboItemID(t *testing.T) {
	store := newMockComboStore()
	outletID := uuid.New()
	comboID := seedComboProduct(store, outletID)
	router := setupComboRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+comboID.String()+"/combo-items/not-a-uuid", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
