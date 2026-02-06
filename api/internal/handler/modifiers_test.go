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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/handler"
)

// --- Mock store ---

type mockModifierStore struct {
	products       map[uuid.UUID]database.Product
	modifierGroups map[uuid.UUID]database.ModifierGroup
	modifiers      map[uuid.UUID]database.Modifier
	fkError        bool
}

func newMockModifierStore() *mockModifierStore {
	return &mockModifierStore{
		products:       make(map[uuid.UUID]database.Product),
		modifierGroups: make(map[uuid.UUID]database.ModifierGroup),
		modifiers:      make(map[uuid.UUID]database.Modifier),
	}
}

// Product ownership verification
func (m *mockModifierStore) GetProduct(_ context.Context, arg database.GetProductParams) (database.Product, error) {
	p, ok := m.products[arg.ID]
	if !ok || p.OutletID != arg.OutletID || !p.IsActive {
		return database.Product{}, pgx.ErrNoRows
	}
	return p, nil
}

// Modifier group operations
func (m *mockModifierStore) ListModifierGroupsByProduct(_ context.Context, productID uuid.UUID) ([]database.ModifierGroup, error) {
	var result []database.ModifierGroup
	for _, mg := range m.modifierGroups {
		if mg.ProductID == productID && mg.IsActive {
			result = append(result, mg)
		}
	}
	return result, nil
}

func (m *mockModifierStore) GetModifierGroup(_ context.Context, arg database.GetModifierGroupParams) (database.ModifierGroup, error) {
	mg, ok := m.modifierGroups[arg.ID]
	if !ok || mg.ProductID != arg.ProductID || !mg.IsActive {
		return database.ModifierGroup{}, pgx.ErrNoRows
	}
	return mg, nil
}

func (m *mockModifierStore) CreateModifierGroup(_ context.Context, arg database.CreateModifierGroupParams) (database.ModifierGroup, error) {
	if m.fkError {
		return database.ModifierGroup{}, &pgconn.PgError{Code: "23503"}
	}
	mg := database.ModifierGroup{
		ID:        uuid.New(),
		ProductID: arg.ProductID,
		Name:      arg.Name,
		MinSelect: arg.MinSelect,
		MaxSelect: arg.MaxSelect,
		IsActive:  true,
		SortOrder: arg.SortOrder,
	}
	m.modifierGroups[mg.ID] = mg
	return mg, nil
}

func (m *mockModifierStore) UpdateModifierGroup(_ context.Context, arg database.UpdateModifierGroupParams) (database.ModifierGroup, error) {
	mg, ok := m.modifierGroups[arg.ID]
	if !ok || mg.ProductID != arg.ProductID || !mg.IsActive {
		return database.ModifierGroup{}, pgx.ErrNoRows
	}
	mg.Name = arg.Name
	mg.MinSelect = arg.MinSelect
	mg.MaxSelect = arg.MaxSelect
	mg.SortOrder = arg.SortOrder
	m.modifierGroups[mg.ID] = mg
	return mg, nil
}

func (m *mockModifierStore) SoftDeleteModifierGroup(_ context.Context, arg database.SoftDeleteModifierGroupParams) (uuid.UUID, error) {
	mg, ok := m.modifierGroups[arg.ID]
	if !ok || mg.ProductID != arg.ProductID || !mg.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	mg.IsActive = false
	m.modifierGroups[mg.ID] = mg
	return mg.ID, nil
}

// Modifier operations
func (m *mockModifierStore) ListModifiersByGroup(_ context.Context, modifierGroupID uuid.UUID) ([]database.Modifier, error) {
	var result []database.Modifier
	for _, mod := range m.modifiers {
		if mod.ModifierGroupID == modifierGroupID && mod.IsActive {
			result = append(result, mod)
		}
	}
	return result, nil
}

func (m *mockModifierStore) CreateModifier(_ context.Context, arg database.CreateModifierParams) (database.Modifier, error) {
	if m.fkError {
		return database.Modifier{}, &pgconn.PgError{Code: "23503"}
	}
	mod := database.Modifier{
		ID:              uuid.New(),
		ModifierGroupID: arg.ModifierGroupID,
		Name:            arg.Name,
		Price:           arg.Price,
		IsActive:        true,
		SortOrder:       arg.SortOrder,
	}
	m.modifiers[mod.ID] = mod
	return mod, nil
}

func (m *mockModifierStore) UpdateModifier(_ context.Context, arg database.UpdateModifierParams) (database.Modifier, error) {
	mod, ok := m.modifiers[arg.ID]
	if !ok || mod.ModifierGroupID != arg.ModifierGroupID || !mod.IsActive {
		return database.Modifier{}, pgx.ErrNoRows
	}
	mod.Name = arg.Name
	mod.Price = arg.Price
	mod.SortOrder = arg.SortOrder
	m.modifiers[mod.ID] = mod
	return mod, nil
}

func (m *mockModifierStore) SoftDeleteModifier(_ context.Context, arg database.SoftDeleteModifierParams) (uuid.UUID, error) {
	mod, ok := m.modifiers[arg.ID]
	if !ok || mod.ModifierGroupID != arg.ModifierGroupID || !mod.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	mod.IsActive = false
	m.modifiers[mod.ID] = mod
	return mod.ID, nil
}

// --- Helpers ---

func setupModifierRouter(store *mockModifierStore) *chi.Mux {
	h := handler.NewModifierHandler(store)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/products/{pid}", h.RegisterRoutes)
	return r
}

func decodeModifierResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeModifierListResponse(t *testing.T, rr *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// seedModifierProduct creates a product in the mock store and returns its ID.
func seedModifierProduct(store *mockModifierStore, outletID uuid.UUID) uuid.UUID {
	prodID := uuid.New()
	store.products[prodID] = database.Product{
		ID:        prodID,
		OutletID:  outletID,
		Name:      "Test Product",
		BasePrice: testNumeric("25000"),
		IsActive:  true,
	}
	return prodID
}

// seedModifierGroup creates a modifier group in the mock store and returns its ID.
func seedModifierGroup(store *mockModifierStore, productID uuid.UUID, name string, minSelect int32, maxSelect *int32) uuid.UUID {
	mgID := uuid.New()
	mg := database.ModifierGroup{
		ID:        mgID,
		ProductID: productID,
		Name:      name,
		MinSelect: minSelect,
		IsActive:  true,
		SortOrder: 0,
	}
	if maxSelect != nil {
		mg.MaxSelect = pgtype.Int4{Int32: *maxSelect, Valid: true}
	}
	store.modifierGroups[mgID] = mg
	return mgID
}

// seedModifier creates a modifier in the mock store and returns its ID.
func seedModifier(store *mockModifierStore, mgID uuid.UUID, name string, price string) uuid.UUID {
	mID := uuid.New()
	store.modifiers[mID] = database.Modifier{
		ID:              mID,
		ModifierGroupID: mgID,
		Name:            name,
		Price:           testNumeric(price),
		IsActive:        true,
		SortOrder:       0,
	}
	return mID
}

// int32Ptr returns a pointer to an int32 value.
func int32Ptr(v int32) *int32 {
	return &v
}

// ========================
// Modifier Group: List
// ========================

func TestModifierGroupList_Empty(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeModifierListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestModifierGroupList_ReturnsGroups(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	seedModifierGroup(store, prodID, "Extra Toppings", 0, int32Ptr(3))
	seedModifierGroup(store, prodID, "Sauce", 1, int32Ptr(2))
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeModifierListResponse(t, rr)
	if len(resp) != 2 {
		t.Fatalf("expected 2 modifier groups, got %d", len(resp))
	}
}

func TestModifierGroupList_ExcludesInactive(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Deleted Group", 0, nil)
	mg := store.modifierGroups[mgID]
	mg.IsActive = false
	store.modifierGroups[mgID] = mg
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeModifierListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list (inactive excluded), got %d items", len(resp))
	}
}

func TestModifierGroupList_ProductNotFound(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+uuid.New().String()+"/modifier-groups", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestModifierGroupList_WrongOutlet(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/modifier-groups", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierGroupList_InvalidOutletID(t *testing.T) {
	store := newMockModifierStore()
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/not-a-uuid/products/"+uuid.New().String()+"/modifier-groups", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestModifierGroupList_InvalidProductID(t *testing.T) {
	store := newMockModifierStore()
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+uuid.New().String()+"/products/not-a-uuid/modifier-groups", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Modifier Group: Create
// ========================

func TestModifierGroupCreate_Valid(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", map[string]interface{}{
		"name":       "Extra Toppings",
		"min_select": 0,
		"max_select": 3,
		"sort_order": 1,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["name"] != "Extra Toppings" {
		t.Errorf("name: got %v, want 'Extra Toppings'", resp["name"])
	}
	if resp["min_select"] != float64(0) {
		t.Errorf("min_select: got %v, want 0", resp["min_select"])
	}
	if resp["max_select"] != float64(3) {
		t.Errorf("max_select: got %v, want 3", resp["max_select"])
	}
	if resp["is_active"] != true {
		t.Errorf("is_active: got %v, want true", resp["is_active"])
	}
	if resp["sort_order"] != float64(1) {
		t.Errorf("sort_order: got %v, want 1", resp["sort_order"])
	}
	if resp["product_id"] != prodID.String() {
		t.Errorf("product_id: got %v, want %s", resp["product_id"], prodID.String())
	}
}

func TestModifierGroupCreate_DefaultValues(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	// Only name — min_select defaults to 0, max_select defaults to null, sort_order defaults to 0
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", map[string]interface{}{
		"name": "Extra Toppings",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["name"] != "Extra Toppings" {
		t.Errorf("name: got %v, want 'Extra Toppings'", resp["name"])
	}
	if resp["min_select"] != float64(0) {
		t.Errorf("min_select: got %v, want 0 (default)", resp["min_select"])
	}
	// max_select should be null when not specified (unlimited)
	if resp["max_select"] != nil {
		t.Errorf("max_select: got %v, want nil (unlimited)", resp["max_select"])
	}
	if resp["sort_order"] != float64(0) {
		t.Errorf("sort_order: got %v, want 0 (default)", resp["sort_order"])
	}
}

func TestModifierGroupCreate_WithMinSelectRequired(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	// min_select=1 means customer must pick at least 1 modifier
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", map[string]interface{}{
		"name":       "Required Sauce",
		"min_select": 1,
		"max_select": 2,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["min_select"] != float64(1) {
		t.Errorf("min_select: got %v, want 1", resp["min_select"])
	}
	if resp["max_select"] != float64(2) {
		t.Errorf("max_select: got %v, want 2", resp["max_select"])
	}
}

func TestModifierGroupCreate_NullMaxSelect(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	// Explicitly set max_select to null (unlimited)
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", map[string]interface{}{
		"name":       "Unlimited Toppings",
		"min_select": 0,
		"max_select": nil,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["max_select"] != nil {
		t.Errorf("max_select: got %v, want nil (unlimited)", resp["max_select"])
	}
}

func TestModifierGroupCreate_MissingName(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", map[string]interface{}{
		"min_select": 0,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["error"] != "name is required" {
		t.Errorf("error: got %v, want 'name is required'", resp["error"])
	}
}

func TestModifierGroupCreate_NegativeMinSelect(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", map[string]interface{}{
		"name":       "Bad Group",
		"min_select": -1,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["error"] != "min_select must be >= 0" {
		t.Errorf("error: got %v, want 'min_select must be >= 0'", resp["error"])
	}
}

func TestModifierGroupCreate_MaxSelectLessThanMinSelect(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", map[string]interface{}{
		"name":       "Bad Group",
		"min_select": 3,
		"max_select": 1,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["error"] != "max_select must be >= min_select" {
		t.Errorf("error: got %v, want 'max_select must be >= min_select'", resp["error"])
	}
}

func TestModifierGroupCreate_ProductNotFound(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+uuid.New().String()+"/modifier-groups", map[string]interface{}{
		"name": "Extra Toppings",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestModifierGroupCreate_WrongOutlet(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/modifier-groups", map[string]interface{}{
		"name": "Extra Toppings",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierGroupCreate_InvalidBody(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups", "not json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Modifier Group: Update
// ========================

func TestModifierGroupUpdate_Valid(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Old Name", 0, int32Ptr(3))
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String(), map[string]interface{}{
		"name":       "New Name",
		"min_select": 1,
		"max_select": 5,
		"sort_order": 2,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["name"] != "New Name" {
		t.Errorf("name: got %v, want 'New Name'", resp["name"])
	}
	if resp["min_select"] != float64(1) {
		t.Errorf("min_select: got %v, want 1", resp["min_select"])
	}
	if resp["max_select"] != float64(5) {
		t.Errorf("max_select: got %v, want 5", resp["max_select"])
	}
	if resp["sort_order"] != float64(2) {
		t.Errorf("sort_order: got %v, want 2", resp["sort_order"])
	}
}

func TestModifierGroupUpdate_SetMaxSelectToNull(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, int32Ptr(3))
	router := setupModifierRouter(store)

	// Update to remove max_select (unlimited)
	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String(), map[string]interface{}{
		"name":       "Unlimited Toppings",
		"min_select": 0,
		"max_select": nil,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["max_select"] != nil {
		t.Errorf("max_select: got %v, want nil (unlimited)", resp["max_select"])
	}
}

func TestModifierGroupUpdate_NotFound(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+uuid.New().String(), map[string]interface{}{
		"name":       "Whatever",
		"min_select": 0,
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierGroupUpdate_WrongProduct(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	otherProdID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	// Try to update modifier group using a different product ID
	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+otherProdID.String()+"/modifier-groups/"+mgID.String(), map[string]interface{}{
		"name":       "Hacked",
		"min_select": 0,
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierGroupUpdate_MissingName(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String(), map[string]interface{}{
		"min_select": 0,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestModifierGroupUpdate_InvalidMGID(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/not-a-uuid", map[string]interface{}{
		"name":       "Test",
		"min_select": 0,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestModifierGroupUpdate_MaxSelectLessThanMinSelect(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String(), map[string]interface{}{
		"name":       "Bad Update",
		"min_select": 3,
		"max_select": 1,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	resp := decodeModifierResponse(t, rr)
	if resp["error"] != "max_select must be >= min_select" {
		t.Errorf("error: got %v, want 'max_select must be >= min_select'", resp["error"])
	}
}

// ========================
// Modifier Group: Delete
// ========================

func TestModifierGroupDelete_Valid(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Delete Me", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	mg := store.modifierGroups[mgID]
	if mg.IsActive {
		t.Error("expected modifier group to be soft-deleted (is_active=false)")
	}
}

func TestModifierGroupDelete_NotFound(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierGroupDelete_WrongOutlet(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original is still active
	mg := store.modifierGroups[mgID]
	if !mg.IsActive {
		t.Error("modifier group should not be affected by wrong-outlet delete")
	}
}

// ========================
// Modifier: List
// ========================

func TestModifierList_Empty(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeModifierListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestModifierList_ReturnsModifiers(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	seedModifier(store, mgID, "Extra Cheese", "3000")
	seedModifier(store, mgID, "Extra Egg", "5000")
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeModifierListResponse(t, rr)
	if len(resp) != 2 {
		t.Fatalf("expected 2 modifiers, got %d", len(resp))
	}
}

func TestModifierList_ExcludesInactive(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	mID := seedModifier(store, mgID, "Deleted", "0")
	mod := store.modifiers[mID]
	mod.IsActive = false
	store.modifiers[mID] = mod
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeModifierListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list (inactive excluded), got %d items", len(resp))
	}
}

func TestModifierList_ModifierGroupNotFound(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+uuid.New().String()+"/modifiers", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestModifierList_WrongProduct(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	otherProdID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	seedModifier(store, mgID, "Extra Cheese", "3000")
	router := setupModifierRouter(store)

	// Try to list modifiers using a different product ID
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+otherProdID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// ========================
// Modifier: Create
// ========================

func TestModifierCreate_Valid(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", map[string]interface{}{
		"name":       "Extra Cheese",
		"price":      "3000.00",
		"sort_order": 1,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["name"] != "Extra Cheese" {
		t.Errorf("name: got %v, want 'Extra Cheese'", resp["name"])
	}
	if resp["price"] != "3000.00" {
		t.Errorf("price: got %v, want '3000.00'", resp["price"])
	}
	if resp["is_active"] != true {
		t.Errorf("is_active: got %v, want true", resp["is_active"])
	}
	if resp["sort_order"] != float64(1) {
		t.Errorf("sort_order: got %v, want 1", resp["sort_order"])
	}
	if resp["modifier_group_id"] != mgID.String() {
		t.Errorf("modifier_group_id: got %v, want %s", resp["modifier_group_id"], mgID.String())
	}
}

func TestModifierCreate_DefaultPrice(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	// price defaults to "0.00" when not specified
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", map[string]interface{}{
		"name": "No Extra",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["price"] != "0.00" {
		t.Errorf("price: got %v, want '0.00'", resp["price"])
	}
}

func TestModifierCreate_ZeroPrice(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", map[string]interface{}{
		"name":  "Free Topping",
		"price": "0",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["price"] != "0.00" {
		t.Errorf("price: got %v, want '0.00'", resp["price"])
	}
}

func TestModifierCreate_NegativePrice(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	// Negative price is NOT allowed for modifiers (unlike variant price_adjustment)
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", map[string]interface{}{
		"name":  "Discount Topping",
		"price": "-1000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["error"] != "price must be >= 0" {
		t.Errorf("error: got %v, want 'price must be >= 0'", resp["error"])
	}
}

func TestModifierCreate_MissingName(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", map[string]interface{}{
		"price": "5000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["error"] != "name is required" {
		t.Errorf("error: got %v, want 'name is required'", resp["error"])
	}
}

func TestModifierCreate_InvalidPrice(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", map[string]interface{}{
		"name":  "Bad Price",
		"price": "not-a-number",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["error"] != "invalid price" {
		t.Errorf("error: got %v, want 'invalid price'", resp["error"])
	}
}

func TestModifierCreate_ModifierGroupNotFound(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+uuid.New().String()+"/modifiers", map[string]interface{}{
		"name": "Extra Cheese",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestModifierCreate_WrongOutlet(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", map[string]interface{}{
		"name": "Extra Cheese",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierCreate_InvalidBody(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers", "not json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Modifier: Update
// ========================

func TestModifierUpdate_Valid(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	mID := seedModifier(store, mgID, "Old Name", "1000")
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers/"+mID.String(), map[string]interface{}{
		"name":       "New Name",
		"price":      "7500.50",
		"sort_order": 3,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeModifierResponse(t, rr)
	if resp["name"] != "New Name" {
		t.Errorf("name: got %v, want 'New Name'", resp["name"])
	}
	if resp["price"] != "7500.50" {
		t.Errorf("price: got %v, want '7500.50'", resp["price"])
	}
	if resp["sort_order"] != float64(3) {
		t.Errorf("sort_order: got %v, want 3", resp["sort_order"])
	}
}

func TestModifierUpdate_NotFound(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers/"+uuid.New().String(), map[string]interface{}{
		"name":  "Whatever",
		"price": "0",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierUpdate_WrongGroup(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	otherMgID := seedModifierGroup(store, prodID, "Sauces", 0, nil)
	mID := seedModifier(store, mgID, "Extra Cheese", "3000")
	router := setupModifierRouter(store)

	// Try to update modifier using a different modifier group
	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+otherMgID.String()+"/modifiers/"+mID.String(), map[string]interface{}{
		"name":  "Hacked",
		"price": "0",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierUpdate_MissingName(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	mID := seedModifier(store, mgID, "Extra Cheese", "3000")
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers/"+mID.String(), map[string]interface{}{
		"price": "5000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestModifierUpdate_NegativePrice(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	mID := seedModifier(store, mgID, "Extra Cheese", "3000")
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers/"+mID.String(), map[string]interface{}{
		"name":  "Discount",
		"price": "-500",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	resp := decodeModifierResponse(t, rr)
	if resp["error"] != "price must be >= 0" {
		t.Errorf("error: got %v, want 'price must be >= 0'", resp["error"])
	}
}

func TestModifierUpdate_InvalidModifierID(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers/not-a-uuid", map[string]interface{}{
		"name":  "Test",
		"price": "0",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Modifier: Delete
// ========================

func TestModifierDelete_Valid(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	mID := seedModifier(store, mgID, "Delete Me", "0")
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers/"+mID.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	mod := store.modifiers[mID]
	if mod.IsActive {
		t.Error("expected modifier to be soft-deleted (is_active=false)")
	}
}

func TestModifierDelete_NotFound(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestModifierDelete_WrongGroup(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	otherMgID := seedModifierGroup(store, prodID, "Sauces", 0, nil)
	mID := seedModifier(store, mgID, "Extra Cheese", "3000")
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+otherMgID.String()+"/modifiers/"+mID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original is still active
	mod := store.modifiers[mID]
	if !mod.IsActive {
		t.Error("modifier should not be affected by wrong-group delete")
	}
}

func TestModifierDelete_WrongOutlet(t *testing.T) {
	store := newMockModifierStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedModifierProduct(store, outletID)
	mgID := seedModifierGroup(store, prodID, "Toppings", 0, nil)
	mID := seedModifier(store, mgID, "Extra Cheese", "3000")
	router := setupModifierRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/modifier-groups/"+mgID.String()+"/modifiers/"+mID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original is still active
	mod := store.modifiers[mID]
	if !mod.IsActive {
		t.Error("modifier should not be affected by wrong-outlet delete")
	}
}
