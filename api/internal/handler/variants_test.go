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

type mockVariantStore struct {
	products      map[uuid.UUID]database.Product
	variantGroups map[uuid.UUID]database.VariantGroup
	variants      map[uuid.UUID]database.Variant
	fkError       bool
}

func newMockVariantStore() *mockVariantStore {
	return &mockVariantStore{
		products:      make(map[uuid.UUID]database.Product),
		variantGroups: make(map[uuid.UUID]database.VariantGroup),
		variants:      make(map[uuid.UUID]database.Variant),
	}
}

// Product ownership verification
func (m *mockVariantStore) GetProduct(_ context.Context, arg database.GetProductParams) (database.Product, error) {
	p, ok := m.products[arg.ID]
	if !ok || p.OutletID != arg.OutletID || !p.IsActive {
		return database.Product{}, pgx.ErrNoRows
	}
	return p, nil
}

// Variant group operations
func (m *mockVariantStore) ListVariantGroupsByProduct(_ context.Context, productID uuid.UUID) ([]database.VariantGroup, error) {
	var result []database.VariantGroup
	for _, vg := range m.variantGroups {
		if vg.ProductID == productID && vg.IsActive {
			result = append(result, vg)
		}
	}
	return result, nil
}

func (m *mockVariantStore) GetVariantGroup(_ context.Context, arg database.GetVariantGroupParams) (database.VariantGroup, error) {
	vg, ok := m.variantGroups[arg.ID]
	if !ok || vg.ProductID != arg.ProductID || !vg.IsActive {
		return database.VariantGroup{}, pgx.ErrNoRows
	}
	return vg, nil
}

func (m *mockVariantStore) CreateVariantGroup(_ context.Context, arg database.CreateVariantGroupParams) (database.VariantGroup, error) {
	if m.fkError {
		return database.VariantGroup{}, &pgconn.PgError{Code: "23503"}
	}
	vg := database.VariantGroup{
		ID:         uuid.New(),
		ProductID:  arg.ProductID,
		Name:       arg.Name,
		IsRequired: arg.IsRequired,
		IsActive:   true,
		SortOrder:  arg.SortOrder,
	}
	m.variantGroups[vg.ID] = vg
	return vg, nil
}

func (m *mockVariantStore) UpdateVariantGroup(_ context.Context, arg database.UpdateVariantGroupParams) (database.VariantGroup, error) {
	vg, ok := m.variantGroups[arg.ID]
	if !ok || vg.ProductID != arg.ProductID || !vg.IsActive {
		return database.VariantGroup{}, pgx.ErrNoRows
	}
	vg.Name = arg.Name
	vg.IsRequired = arg.IsRequired
	vg.SortOrder = arg.SortOrder
	m.variantGroups[vg.ID] = vg
	return vg, nil
}

func (m *mockVariantStore) SoftDeleteVariantGroup(_ context.Context, arg database.SoftDeleteVariantGroupParams) (uuid.UUID, error) {
	vg, ok := m.variantGroups[arg.ID]
	if !ok || vg.ProductID != arg.ProductID || !vg.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	vg.IsActive = false
	m.variantGroups[vg.ID] = vg
	return vg.ID, nil
}

// Variant operations
func (m *mockVariantStore) ListVariantsByGroup(_ context.Context, variantGroupID uuid.UUID) ([]database.Variant, error) {
	var result []database.Variant
	for _, v := range m.variants {
		if v.VariantGroupID == variantGroupID && v.IsActive {
			result = append(result, v)
		}
	}
	return result, nil
}

func (m *mockVariantStore) CreateVariant(_ context.Context, arg database.CreateVariantParams) (database.Variant, error) {
	if m.fkError {
		return database.Variant{}, &pgconn.PgError{Code: "23503"}
	}
	v := database.Variant{
		ID:              uuid.New(),
		VariantGroupID:  arg.VariantGroupID,
		Name:            arg.Name,
		PriceAdjustment: arg.PriceAdjustment,
		IsActive:        true,
		SortOrder:       arg.SortOrder,
	}
	m.variants[v.ID] = v
	return v, nil
}

func (m *mockVariantStore) UpdateVariant(_ context.Context, arg database.UpdateVariantParams) (database.Variant, error) {
	v, ok := m.variants[arg.ID]
	if !ok || v.VariantGroupID != arg.VariantGroupID || !v.IsActive {
		return database.Variant{}, pgx.ErrNoRows
	}
	v.Name = arg.Name
	v.PriceAdjustment = arg.PriceAdjustment
	v.SortOrder = arg.SortOrder
	m.variants[v.ID] = v
	return v, nil
}

func (m *mockVariantStore) SoftDeleteVariant(_ context.Context, arg database.SoftDeleteVariantParams) (uuid.UUID, error) {
	v, ok := m.variants[arg.ID]
	if !ok || v.VariantGroupID != arg.VariantGroupID || !v.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	v.IsActive = false
	m.variants[v.ID] = v
	return v.ID, nil
}

// --- Helpers ---

func setupVariantRouter(store *mockVariantStore) *chi.Mux {
	h := handler.NewVariantHandler(store)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/products/{pid}", h.RegisterRoutes)
	return r
}

func decodeVariantResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeVariantListResponse(t *testing.T, rr *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// seedProduct creates a product in the mock store and returns its IDs.
func seedProduct(store *mockVariantStore, outletID uuid.UUID) uuid.UUID {
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

// seedVariantGroup creates a variant group in the mock store and returns its ID.
func seedVariantGroup(store *mockVariantStore, productID uuid.UUID, name string) uuid.UUID {
	vgID := uuid.New()
	store.variantGroups[vgID] = database.VariantGroup{
		ID:         vgID,
		ProductID:  productID,
		Name:       name,
		IsRequired: true,
		IsActive:   true,
		SortOrder:  0,
	}
	return vgID
}

// seedVariant creates a variant in the mock store and returns its ID.
func seedVariant(store *mockVariantStore, vgID uuid.UUID, name string, priceAdj string) uuid.UUID {
	vID := uuid.New()
	store.variants[vID] = database.Variant{
		ID:              vID,
		VariantGroupID:  vgID,
		Name:            name,
		PriceAdjustment: testNumeric(priceAdj),
		IsActive:        true,
		SortOrder:       0,
	}
	return vID
}

// ========================
// Variant Group: List
// ========================

func TestVariantGroupList_Empty(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeVariantListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestVariantGroupList_ReturnsGroups(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	seedVariantGroup(store, prodID, "Size")
	seedVariantGroup(store, prodID, "Spice Level")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeVariantListResponse(t, rr)
	if len(resp) != 2 {
		t.Fatalf("expected 2 variant groups, got %d", len(resp))
	}
}

func TestVariantGroupList_ExcludesInactive(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Deleted Group")
	vg := store.variantGroups[vgID]
	vg.IsActive = false
	store.variantGroups[vgID] = vg
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeVariantListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list (inactive excluded), got %d items", len(resp))
	}
}

func TestVariantGroupList_ProductNotFound(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+uuid.New().String()+"/variant-groups", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestVariantGroupList_WrongOutlet(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedProduct(store, outletID)
	seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/variant-groups", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantGroupList_InvalidOutletID(t *testing.T) {
	store := newMockVariantStore()
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/not-a-uuid/products/"+uuid.New().String()+"/variant-groups", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestVariantGroupList_InvalidProductID(t *testing.T) {
	store := newMockVariantStore()
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+uuid.New().String()+"/products/not-a-uuid/variant-groups", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Variant Group: Create
// ========================

func TestVariantGroupCreate_Valid(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups", map[string]interface{}{
		"name":        "Size",
		"is_required": true,
		"sort_order":  1,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["name"] != "Size" {
		t.Errorf("name: got %v, want 'Size'", resp["name"])
	}
	if resp["is_required"] != true {
		t.Errorf("is_required: got %v, want true", resp["is_required"])
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

func TestVariantGroupCreate_DefaultValues(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	// Only name — is_required defaults to true, sort_order defaults to 0
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups", map[string]interface{}{
		"name": "Size",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["name"] != "Size" {
		t.Errorf("name: got %v, want 'Size'", resp["name"])
	}
	// is_required should default to true when not specified
	if resp["is_required"] != true {
		t.Errorf("is_required: got %v, want true (default)", resp["is_required"])
	}
	if resp["sort_order"] != float64(0) {
		t.Errorf("sort_order: got %v, want 0 (default)", resp["sort_order"])
	}
}

func TestVariantGroupCreate_MissingName(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups", map[string]interface{}{
		"is_required": true,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["error"] != "name is required" {
		t.Errorf("error: got %v, want 'name is required'", resp["error"])
	}
}

func TestVariantGroupCreate_ProductNotFound(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+uuid.New().String()+"/variant-groups", map[string]interface{}{
		"name": "Size",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestVariantGroupCreate_WrongOutlet(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/variant-groups", map[string]interface{}{
		"name": "Size",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantGroupCreate_InvalidBody(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups", "not json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Variant Group: Update
// ========================

func TestVariantGroupUpdate_Valid(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Old Name")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String(), map[string]interface{}{
		"name":        "New Name",
		"is_required": false,
		"sort_order":  2,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["name"] != "New Name" {
		t.Errorf("name: got %v, want 'New Name'", resp["name"])
	}
	if resp["is_required"] != false {
		t.Errorf("is_required: got %v, want false", resp["is_required"])
	}
	if resp["sort_order"] != float64(2) {
		t.Errorf("sort_order: got %v, want 2", resp["sort_order"])
	}
}

func TestVariantGroupUpdate_NotFound(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+uuid.New().String(), map[string]interface{}{
		"name":        "Whatever",
		"is_required": true,
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantGroupUpdate_WrongProduct(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	otherProdID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	// Try to update variant group using a different product ID
	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+otherProdID.String()+"/variant-groups/"+vgID.String(), map[string]interface{}{
		"name":        "Hacked",
		"is_required": true,
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantGroupUpdate_MissingName(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String(), map[string]interface{}{
		"is_required": true,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestVariantGroupUpdate_InvalidVGID(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/not-a-uuid", map[string]interface{}{
		"name":        "Test",
		"is_required": true,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Variant Group: Delete
// ========================

func TestVariantGroupDelete_Valid(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Delete Me")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	vg := store.variantGroups[vgID]
	if vg.IsActive {
		t.Error("expected variant group to be soft-deleted (is_active=false)")
	}
}

func TestVariantGroupDelete_NotFound(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantGroupDelete_WrongOutlet(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original is still active
	vg := store.variantGroups[vgID]
	if !vg.IsActive {
		t.Error("variant group should not be affected by wrong-outlet delete")
	}
}

// ========================
// Variant: List
// ========================

func TestVariantList_Empty(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeVariantListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestVariantList_ReturnsVariants(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	seedVariant(store, vgID, "Small", "0")
	seedVariant(store, vgID, "Large", "5000")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeVariantListResponse(t, rr)
	if len(resp) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(resp))
	}
}

func TestVariantList_ExcludesInactive(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	vID := seedVariant(store, vgID, "Deleted", "0")
	v := store.variants[vID]
	v.IsActive = false
	store.variants[vID] = v
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeVariantListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list (inactive excluded), got %d items", len(resp))
	}
}

func TestVariantList_VariantGroupNotFound(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+uuid.New().String()+"/variants", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestVariantList_WrongProduct(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	otherProdID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	seedVariant(store, vgID, "Small", "0")
	router := setupVariantRouter(store)

	// Try to list variants using a different product ID
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+otherProdID.String()+"/variant-groups/"+vgID.String()+"/variants", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// ========================
// Variant: Create
// ========================

func TestVariantCreate_Valid(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", map[string]interface{}{
		"name":             "Large",
		"price_adjustment": "5000.00",
		"sort_order":       1,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["name"] != "Large" {
		t.Errorf("name: got %v, want 'Large'", resp["name"])
	}
	if resp["price_adjustment"] != "5000.00" {
		t.Errorf("price_adjustment: got %v, want '5000.00'", resp["price_adjustment"])
	}
	if resp["is_active"] != true {
		t.Errorf("is_active: got %v, want true", resp["is_active"])
	}
	if resp["sort_order"] != float64(1) {
		t.Errorf("sort_order: got %v, want 1", resp["sort_order"])
	}
	if resp["variant_group_id"] != vgID.String() {
		t.Errorf("variant_group_id: got %v, want %s", resp["variant_group_id"], vgID.String())
	}
}

func TestVariantCreate_DefaultPriceAdjustment(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	// price_adjustment defaults to "0.00" when not specified
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", map[string]interface{}{
		"name": "Regular",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["price_adjustment"] != "0.00" {
		t.Errorf("price_adjustment: got %v, want '0.00'", resp["price_adjustment"])
	}
}

func TestVariantCreate_NegativePriceAdjustment(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	// Negative price adjustment is allowed (discount)
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", map[string]interface{}{
		"name":             "Small",
		"price_adjustment": "-3000",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["price_adjustment"] != "-3000.00" {
		t.Errorf("price_adjustment: got %v, want '-3000.00'", resp["price_adjustment"])
	}
}

func TestVariantCreate_MissingName(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", map[string]interface{}{
		"price_adjustment": "5000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["error"] != "name is required" {
		t.Errorf("error: got %v, want 'name is required'", resp["error"])
	}
}

func TestVariantCreate_InvalidPriceAdjustment(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", map[string]interface{}{
		"name":             "Large",
		"price_adjustment": "not-a-number",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["error"] != "invalid price_adjustment" {
		t.Errorf("error: got %v, want 'invalid price_adjustment'", resp["error"])
	}
}

func TestVariantCreate_VariantGroupNotFound(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+uuid.New().String()+"/variants", map[string]interface{}{
		"name": "Large",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestVariantCreate_WrongOutlet(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", map[string]interface{}{
		"name": "Large",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantCreate_InvalidBody(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants", "not json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Variant: Update
// ========================

func TestVariantUpdate_Valid(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	vID := seedVariant(store, vgID, "Old Name", "1000")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants/"+vID.String(), map[string]interface{}{
		"name":             "New Name",
		"price_adjustment": "7500.50",
		"sort_order":       3,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeVariantResponse(t, rr)
	if resp["name"] != "New Name" {
		t.Errorf("name: got %v, want 'New Name'", resp["name"])
	}
	if resp["price_adjustment"] != "7500.50" {
		t.Errorf("price_adjustment: got %v, want '7500.50'", resp["price_adjustment"])
	}
	if resp["sort_order"] != float64(3) {
		t.Errorf("sort_order: got %v, want 3", resp["sort_order"])
	}
}

func TestVariantUpdate_NotFound(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants/"+uuid.New().String(), map[string]interface{}{
		"name":             "Whatever",
		"price_adjustment": "0",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantUpdate_WrongGroup(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	otherVgID := seedVariantGroup(store, prodID, "Spice")
	vID := seedVariant(store, vgID, "Large", "5000")
	router := setupVariantRouter(store)

	// Try to update variant using a different variant group
	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+otherVgID.String()+"/variants/"+vID.String(), map[string]interface{}{
		"name":             "Hacked",
		"price_adjustment": "0",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantUpdate_MissingName(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	vID := seedVariant(store, vgID, "Large", "5000")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants/"+vID.String(), map[string]interface{}{
		"price_adjustment": "5000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestVariantUpdate_InvalidVariantID(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants/not-a-uuid", map[string]interface{}{
		"name":             "Test",
		"price_adjustment": "0",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ========================
// Variant: Delete
// ========================

func TestVariantDelete_Valid(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	vID := seedVariant(store, vgID, "Delete Me", "0")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants/"+vID.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	v := store.variants[vID]
	if v.IsActive {
		t.Error("expected variant to be soft-deleted (is_active=false)")
	}
}

func TestVariantDelete_NotFound(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants/"+uuid.New().String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestVariantDelete_WrongGroup(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	otherVgID := seedVariantGroup(store, prodID, "Spice")
	vID := seedVariant(store, vgID, "Large", "5000")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String()+"/variant-groups/"+otherVgID.String()+"/variants/"+vID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original is still active
	v := store.variants[vID]
	if !v.IsActive {
		t.Error("variant should not be affected by wrong-group delete")
	}
}

func TestVariantDelete_WrongOutlet(t *testing.T) {
	store := newMockVariantStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	prodID := seedProduct(store, outletID)
	vgID := seedVariantGroup(store, prodID, "Size")
	vID := seedVariant(store, vgID, "Large", "5000")
	router := setupVariantRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String()+"/variant-groups/"+vgID.String()+"/variants/"+vID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original is still active
	v := store.variants[vID]
	if !v.IsActive {
		t.Error("variant should not be affected by wrong-outlet delete")
	}
}
