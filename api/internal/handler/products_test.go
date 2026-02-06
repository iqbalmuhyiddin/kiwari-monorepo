package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/handler"
)

// --- Mock store ---

type mockProductStore struct {
	products map[uuid.UUID]database.Product
	fkError  bool // simulate FK violation
}

func newMockProductStore() *mockProductStore {
	return &mockProductStore{products: make(map[uuid.UUID]database.Product)}
}

func (m *mockProductStore) ListProductsByOutlet(_ context.Context, outletID uuid.UUID) ([]database.Product, error) {
	var result []database.Product
	for _, p := range m.products {
		if p.OutletID == outletID && p.IsActive {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockProductStore) GetProduct(_ context.Context, arg database.GetProductParams) (database.Product, error) {
	p, ok := m.products[arg.ID]
	if !ok || p.OutletID != arg.OutletID || !p.IsActive {
		return database.Product{}, pgx.ErrNoRows
	}
	return p, nil
}

func (m *mockProductStore) CreateProduct(_ context.Context, arg database.CreateProductParams) (database.Product, error) {
	if m.fkError {
		return database.Product{}, &pgconn.PgError{Code: "23503"}
	}
	now := time.Now()
	p := database.Product{
		ID:              uuid.New(),
		OutletID:        arg.OutletID,
		CategoryID:      arg.CategoryID,
		Name:            arg.Name,
		Description:     arg.Description,
		BasePrice:       arg.BasePrice,
		ImageUrl:        arg.ImageUrl,
		Station:         arg.Station,
		PreparationTime: arg.PreparationTime,
		IsCombo:         arg.IsCombo,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	m.products[p.ID] = p
	return p, nil
}

func (m *mockProductStore) UpdateProduct(_ context.Context, arg database.UpdateProductParams) (database.Product, error) {
	if m.fkError {
		return database.Product{}, &pgconn.PgError{Code: "23503"}
	}
	p, ok := m.products[arg.ID]
	if !ok || p.OutletID != arg.OutletID || !p.IsActive {
		return database.Product{}, pgx.ErrNoRows
	}
	p.CategoryID = arg.CategoryID
	p.Name = arg.Name
	p.Description = arg.Description
	p.BasePrice = arg.BasePrice
	p.ImageUrl = arg.ImageUrl
	p.Station = arg.Station
	p.PreparationTime = arg.PreparationTime
	p.IsCombo = arg.IsCombo
	p.UpdatedAt = time.Now()
	m.products[p.ID] = p
	return p, nil
}

func (m *mockProductStore) SoftDeleteProduct(_ context.Context, arg database.SoftDeleteProductParams) (uuid.UUID, error) {
	p, ok := m.products[arg.ID]
	if !ok || p.OutletID != arg.OutletID || !p.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	p.IsActive = false
	m.products[p.ID] = p
	return p.ID, nil
}

// --- Helpers ---

func setupProductRouter(store *mockProductStore) *chi.Mux {
	h := handler.NewProductHandler(store)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/products", h.RegisterRoutes)
	return r
}

func decodeProductResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeProductListResponse(t *testing.T, rr *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func testNumeric(val string) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(val)
	return n
}

// --- List tests ---

func TestProductList_Empty(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeProductListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestProductList_ReturnsOutletProducts(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	otherOutletID := uuid.New()
	catID := uuid.New()

	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now()
	store.products[id1] = database.Product{
		ID: id1, OutletID: outletID, CategoryID: catID, Name: "Nasi Bakar",
		BasePrice: testNumeric("25000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	store.products[id2] = database.Product{
		ID: id2, OutletID: otherOutletID, CategoryID: catID, Name: "Es Teh",
		BasePrice: testNumeric("8000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeProductListResponse(t, rr)
	if len(resp) != 1 {
		t.Fatalf("expected 1 product, got %d", len(resp))
	}
	if resp[0]["name"] != "Nasi Bakar" {
		t.Errorf("expected Nasi Bakar, got %v", resp[0]["name"])
	}
}

func TestProductList_ExcludesInactive(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	catID := uuid.New()
	now := time.Now()

	id := uuid.New()
	store.products[id] = database.Product{
		ID: id, OutletID: outletID, CategoryID: catID, Name: "Deleted Product",
		BasePrice: testNumeric("10000"), IsActive: false, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeProductListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list (inactive excluded), got %d items", len(resp))
	}
}

func TestProductList_InvalidOutletID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/not-a-uuid/products", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Get tests ---

func TestProductGet_Valid(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	now := time.Now()

	store.products[prodID] = database.Product{
		ID: prodID, OutletID: outletID, CategoryID: catID, Name: "Nasi Bakar Original",
		Description: pgtype.Text{String: "Signature dish", Valid: true},
		BasePrice:   testNumeric("25000"),
		Station:     database.NullKitchenStation{KitchenStation: database.KitchenStationGRILL, Valid: true},
		PreparationTime: pgtype.Int4{Int32: 15, Valid: true},
		IsCombo: false, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String(), nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["name"] != "Nasi Bakar Original" {
		t.Errorf("name: got %v, want 'Nasi Bakar Original'", resp["name"])
	}
	if resp["description"] != "Signature dish" {
		t.Errorf("description: got %v, want 'Signature dish'", resp["description"])
	}
	if resp["station"] != "GRILL" {
		t.Errorf("station: got %v, want 'GRILL'", resp["station"])
	}
	if resp["preparation_time"] != float64(15) {
		t.Errorf("preparation_time: got %v, want 15", resp["preparation_time"])
	}
	if resp["is_combo"] != false {
		t.Errorf("is_combo: got %v, want false", resp["is_combo"])
	}
	if resp["category_id"] != catID.String() {
		t.Errorf("category_id: got %v, want %s", resp["category_id"], catID.String())
	}
	// Verify base_price is returned as string with 2 decimal places
	if resp["base_price"] != "25000.00" {
		t.Errorf("base_price: got %v, want '25000.00'", resp["base_price"])
	}
}

func TestProductGet_NotFound(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	prodID := uuid.New()

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/"+prodID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProductGet_WrongOutlet(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	now := time.Now()

	store.products[prodID] = database.Product{
		ID: prodID, OutletID: outletID, CategoryID: catID, Name: "Product",
		BasePrice: testNumeric("10000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)
	rr := doRequest(t, router, "GET", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProductGet_InvalidProductID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/products/not-a-uuid", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestProductGet_InvalidOutletID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	prodID := uuid.New()

	rr := doRequest(t, router, "GET", "/outlets/not-a-uuid/products/"+prodID.String(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Create tests ---

func TestProductCreate_Valid(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id":      catID.String(),
		"name":             "Nasi Bakar Ayam",
		"description":      "Grilled rice with chicken",
		"base_price":       "25000.00",
		"image_url":        "https://example.com/nasi.jpg",
		"station":          "GRILL",
		"preparation_time": 15,
		"is_combo":         false,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["name"] != "Nasi Bakar Ayam" {
		t.Errorf("name: got %v, want 'Nasi Bakar Ayam'", resp["name"])
	}
	if resp["description"] != "Grilled rice with chicken" {
		t.Errorf("description: got %v, want 'Grilled rice with chicken'", resp["description"])
	}
	if resp["station"] != "GRILL" {
		t.Errorf("station: got %v, want 'GRILL'", resp["station"])
	}
	if resp["is_active"] != true {
		t.Errorf("is_active: got %v, want true", resp["is_active"])
	}
	if resp["outlet_id"] != outletID.String() {
		t.Errorf("outlet_id: got %v, want %s", resp["outlet_id"], outletID.String())
	}
	if resp["category_id"] != catID.String() {
		t.Errorf("category_id: got %v, want %s", resp["category_id"], catID.String())
	}
	if resp["base_price"] != "25000.00" {
		t.Errorf("base_price: got %v, want '25000.00'", resp["base_price"])
	}
}

func TestProductCreate_MinimalFields(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	// Only required fields: name, category_id, base_price
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Simple Product",
		"base_price":  "10000",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["name"] != "Simple Product" {
		t.Errorf("name: got %v, want 'Simple Product'", resp["name"])
	}
	if resp["is_combo"] != false {
		t.Errorf("is_combo: got %v, want false", resp["is_combo"])
	}
	// Optional fields should be null
	if resp["description"] != nil {
		t.Errorf("description: expected null, got %v", resp["description"])
	}
	if resp["station"] != nil {
		t.Errorf("station: expected null, got %v", resp["station"])
	}
}

func TestProductCreate_MissingName(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": catID.String(),
		"base_price":  "10000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "name is required" {
		t.Errorf("error: got %v, want 'name is required'", resp["error"])
	}
}

func TestProductCreate_MissingCategoryID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"name":       "Product",
		"base_price": "10000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "category_id is required" {
		t.Errorf("error: got %v, want 'category_id is required'", resp["error"])
	}
}

func TestProductCreate_InvalidCategoryID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": "not-a-uuid",
		"name":        "Product",
		"base_price":  "10000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "invalid category_id" {
		t.Errorf("error: got %v, want 'invalid category_id'", resp["error"])
	}
}

func TestProductCreate_MissingBasePrice(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Product",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "base_price is required" {
		t.Errorf("error: got %v, want 'base_price is required'", resp["error"])
	}
}

func TestProductCreate_InvalidBasePrice(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Product",
		"base_price":  "not-a-number",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "invalid base_price" {
		t.Errorf("error: got %v, want 'invalid base_price'", resp["error"])
	}
}

func TestProductCreate_NegativeBasePrice(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Product",
		"base_price":  "-100",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "base_price must be >= 0" {
		t.Errorf("error: got %v, want 'base_price must be >= 0'", resp["error"])
	}
}

func TestProductCreate_InvalidStation(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Product",
		"base_price":  "10000",
		"station":     "INVALID_STATION",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "invalid station" {
		t.Errorf("error: got %v, want 'invalid station'", resp["error"])
	}
}

func TestProductCreate_InvalidOutletID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/not-a-uuid/products", map[string]interface{}{
		"category_id": uuid.New().String(),
		"name":        "Product",
		"base_price":  "10000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestProductCreate_InvalidBody(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", "not json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestProductCreate_ForeignKeyViolation(t *testing.T) {
	store := newMockProductStore()
	store.fkError = true
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Product",
		"base_price":  "10000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "invalid category_id" {
		t.Errorf("error: got %v, want 'invalid category_id'", resp["error"])
	}
}

func TestProductCreate_ZeroBasePrice(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	// base_price of 0 is valid (e.g., free items in combos)
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/products", map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Free Item",
		"base_price":  "0",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

// --- Update tests ---

func TestProductUpdate_Valid(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	catID := uuid.New()
	newCatID := uuid.New()
	prodID := uuid.New()
	now := time.Now()

	store.products[prodID] = database.Product{
		ID: prodID, OutletID: outletID, CategoryID: catID, Name: "Old Name",
		BasePrice: testNumeric("10000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String(), map[string]interface{}{
		"category_id":      newCatID.String(),
		"name":             "New Name",
		"description":      "Updated description",
		"base_price":       "35000.50",
		"station":          "BEVERAGE",
		"preparation_time": 10,
		"is_combo":         true,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["name"] != "New Name" {
		t.Errorf("name: got %v, want 'New Name'", resp["name"])
	}
	if resp["description"] != "Updated description" {
		t.Errorf("description: got %v, want 'Updated description'", resp["description"])
	}
	if resp["category_id"] != newCatID.String() {
		t.Errorf("category_id: got %v, want %s", resp["category_id"], newCatID.String())
	}
	if resp["station"] != "BEVERAGE" {
		t.Errorf("station: got %v, want 'BEVERAGE'", resp["station"])
	}
	if resp["is_combo"] != true {
		t.Errorf("is_combo: got %v, want true", resp["is_combo"])
	}
}

func TestProductUpdate_NotFound(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	prodID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String(), map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Whatever",
		"base_price":  "10000",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProductUpdate_WrongOutlet(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	now := time.Now()

	store.products[prodID] = database.Product{
		ID: prodID, OutletID: outletID, CategoryID: catID, Name: "Product",
		BasePrice: testNumeric("10000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String(), map[string]interface{}{
		"category_id": catID.String(),
		"name":        "Hacked",
		"base_price":  "10000",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProductUpdate_MissingName(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	now := time.Now()

	store.products[prodID] = database.Product{
		ID: prodID, OutletID: outletID, CategoryID: catID, Name: "Product",
		BasePrice: testNumeric("10000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String(), map[string]interface{}{
		"category_id": catID.String(),
		"base_price":  "10000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestProductUpdate_InvalidProductID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/not-a-uuid", map[string]interface{}{
		"category_id": uuid.New().String(),
		"name":        "Test",
		"base_price":  "10000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestProductUpdate_ForeignKeyViolation(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	now := time.Now()

	store.products[prodID] = database.Product{
		ID: prodID, OutletID: outletID, CategoryID: catID, Name: "Product",
		BasePrice: testNumeric("10000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	store.fkError = true

	router := setupProductRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/products/"+prodID.String(), map[string]interface{}{
		"category_id": uuid.New().String(),
		"name":        "Product",
		"base_price":  "10000",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeProductResponse(t, rr)
	if resp["error"] != "invalid category_id" {
		t.Errorf("error: got %v, want 'invalid category_id'", resp["error"])
	}
}

// --- Delete tests ---

func TestProductDelete_Valid(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	now := time.Now()

	store.products[prodID] = database.Product{
		ID: prodID, OutletID: outletID, CategoryID: catID, Name: "Delete Me",
		BasePrice: testNumeric("10000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	// Verify soft-deleted
	p := store.products[prodID]
	if p.IsActive {
		t.Error("expected product to be soft-deleted (is_active=false)")
	}
}

func TestProductDelete_NotFound(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()
	prodID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/"+prodID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestProductDelete_WrongOutlet(t *testing.T) {
	store := newMockProductStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	catID := uuid.New()
	prodID := uuid.New()
	now := time.Now()

	store.products[prodID] = database.Product{
		ID: prodID, OutletID: outletID, CategoryID: catID, Name: "Wrong Outlet",
		BasePrice: testNumeric("10000"), IsActive: true, CreatedAt: now, UpdatedAt: now,
	}

	router := setupProductRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+wrongOutletID.String()+"/products/"+prodID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original product is still active
	p := store.products[prodID]
	if !p.IsActive {
		t.Error("product in original outlet should not be affected")
	}
}

func TestProductDelete_InvalidOutletID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	prodID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/not-a-uuid/products/"+prodID.String(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestProductDelete_InvalidProductID(t *testing.T) {
	store := newMockProductStore()
	router := setupProductRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/products/not-a-uuid", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
