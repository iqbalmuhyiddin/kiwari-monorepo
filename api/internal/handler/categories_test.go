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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/handler"
)

// --- Mock store ---

type mockCategoryStore struct {
	categories map[uuid.UUID]database.Category // keyed by category ID
}

func newMockCategoryStore() *mockCategoryStore {
	return &mockCategoryStore{categories: make(map[uuid.UUID]database.Category)}
}

func (m *mockCategoryStore) ListCategoriesByOutlet(_ context.Context, outletID uuid.UUID) ([]database.Category, error) {
	var result []database.Category
	for _, c := range m.categories {
		if c.OutletID == outletID && c.IsActive {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockCategoryStore) CreateCategory(_ context.Context, arg database.CreateCategoryParams) (database.Category, error) {
	c := database.Category{
		ID:          uuid.New(),
		OutletID:    arg.OutletID,
		Name:        arg.Name,
		Description: arg.Description,
		SortOrder:   arg.SortOrder,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	m.categories[c.ID] = c
	return c, nil
}

func (m *mockCategoryStore) UpdateCategory(_ context.Context, arg database.UpdateCategoryParams) (database.Category, error) {
	c, ok := m.categories[arg.ID]
	if !ok || c.OutletID != arg.OutletID || !c.IsActive {
		return database.Category{}, pgx.ErrNoRows
	}
	c.Name = arg.Name
	c.Description = arg.Description
	c.SortOrder = arg.SortOrder
	m.categories[c.ID] = c
	return c, nil
}

func (m *mockCategoryStore) SoftDeleteCategory(_ context.Context, arg database.SoftDeleteCategoryParams) (uuid.UUID, error) {
	c, ok := m.categories[arg.ID]
	if !ok || c.OutletID != arg.OutletID || !c.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	c.IsActive = false
	m.categories[c.ID] = c
	return c.ID, nil
}

// --- Helpers ---

func setupCategoryRouter(store *mockCategoryStore) *chi.Mux {
	h := handler.NewCategoryHandler(store)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/categories", h.RegisterRoutes)
	return r
}

func decodeCategoryResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeCategoryListResponse(t *testing.T, rr *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// --- List tests ---

func TestCategoryList_Empty(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/categories", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeCategoryListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestCategoryList_ReturnsOutletCategories(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()
	otherOutletID := uuid.New()

	catID1 := uuid.New()
	catID2 := uuid.New()
	store.categories[catID1] = database.Category{
		ID: catID1, OutletID: outletID, Name: "Drinks",
		SortOrder: 1, IsActive: true, CreatedAt: time.Now(),
	}
	store.categories[catID2] = database.Category{
		ID: catID2, OutletID: otherOutletID, Name: "Desserts",
		SortOrder: 1, IsActive: true, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/categories", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeCategoryListResponse(t, rr)
	if len(resp) != 1 {
		t.Fatalf("expected 1 category, got %d", len(resp))
	}
	if resp[0]["name"] != "Drinks" {
		t.Errorf("expected Drinks, got %v", resp[0]["name"])
	}
}

func TestCategoryList_ExcludesInactive(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()

	catID := uuid.New()
	store.categories[catID] = database.Category{
		ID: catID, OutletID: outletID, Name: "Deleted",
		SortOrder: 1, IsActive: false, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/categories", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeCategoryListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list (inactive excluded), got %d items", len(resp))
	}
}

func TestCategoryList_InvalidOutletID(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/not-a-uuid/categories", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Create tests ---

func TestCategoryCreate_Valid(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/categories", map[string]interface{}{
		"name":        "Beverages",
		"description": "All drinks",
		"sort_order":  2,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeCategoryResponse(t, rr)
	if resp["name"] != "Beverages" {
		t.Errorf("name: got %v, want Beverages", resp["name"])
	}
	if resp["description"] != "All drinks" {
		t.Errorf("description: got %v, want 'All drinks'", resp["description"])
	}
	// JSON numbers decode as float64
	if resp["sort_order"] != float64(2) {
		t.Errorf("sort_order: got %v, want 2", resp["sort_order"])
	}
	if resp["is_active"] != true {
		t.Errorf("is_active: got %v, want true", resp["is_active"])
	}
	if resp["outlet_id"] != outletID.String() {
		t.Errorf("outlet_id: got %v, want %s", resp["outlet_id"], outletID.String())
	}
}

func TestCategoryCreate_MinimalFields(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()

	// Only name is required; description optional, sort_order defaults to 0
	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/categories", map[string]interface{}{
		"name": "Simple",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeCategoryResponse(t, rr)
	if resp["name"] != "Simple" {
		t.Errorf("name: got %v, want Simple", resp["name"])
	}
	if resp["sort_order"] != float64(0) {
		t.Errorf("sort_order: got %v, want 0", resp["sort_order"])
	}
}

func TestCategoryCreate_MissingName(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/categories", map[string]interface{}{
		"description": "No name",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeCategoryResponse(t, rr)
	if resp["error"] != "name is required" {
		t.Errorf("error: got %v, want 'name is required'", resp["error"])
	}
}

func TestCategoryCreate_EmptyName(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/categories", map[string]interface{}{
		"name": "",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCategoryCreate_InvalidOutletID(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/not-a-uuid/categories", map[string]interface{}{
		"name": "Test",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCategoryCreate_InvalidBody(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/categories", "not json")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Update tests ---

func TestCategoryUpdate_Valid(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()
	catID := uuid.New()

	store.categories[catID] = database.Category{
		ID: catID, OutletID: outletID, Name: "Old Name",
		SortOrder: 0, IsActive: true, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/categories/"+catID.String(), map[string]interface{}{
		"name":        "New Name",
		"description": "Updated desc",
		"sort_order":  5,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeCategoryResponse(t, rr)
	if resp["name"] != "New Name" {
		t.Errorf("name: got %v, want 'New Name'", resp["name"])
	}
	if resp["description"] != "Updated desc" {
		t.Errorf("description: got %v, want 'Updated desc'", resp["description"])
	}
	if resp["sort_order"] != float64(5) {
		t.Errorf("sort_order: got %v, want 5", resp["sort_order"])
	}
}

func TestCategoryUpdate_NotFound(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/categories/"+catID.String(), map[string]interface{}{
		"name": "Whatever",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestCategoryUpdate_WrongOutlet(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	catID := uuid.New()

	store.categories[catID] = database.Category{
		ID: catID, OutletID: outletID, Name: "Food",
		SortOrder: 0, IsActive: true, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+wrongOutletID.String()+"/categories/"+catID.String(), map[string]interface{}{
		"name": "Hacked",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestCategoryUpdate_MissingName(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()
	catID := uuid.New()

	store.categories[catID] = database.Category{
		ID: catID, OutletID: outletID, Name: "Food",
		SortOrder: 0, IsActive: true, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/categories/"+catID.String(), map[string]interface{}{
		"description": "No name",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCategoryUpdate_InvalidCategoryID(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/categories/not-a-uuid", map[string]interface{}{
		"name": "Test",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCategoryUpdate_InvalidOutletID(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	catID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/not-a-uuid/categories/"+catID.String(), map[string]interface{}{
		"name": "Test",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCategoryUpdate_ClearDescription(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()
	catID := uuid.New()

	store.categories[catID] = database.Category{
		ID: catID, OutletID: outletID, Name: "Food",
		Description: pgtype.Text{String: "Old desc", Valid: true},
		SortOrder:   1, IsActive: true, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)

	// Update without description to clear it
	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/categories/"+catID.String(), map[string]interface{}{
		"name":       "Food",
		"sort_order": 1,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeCategoryResponse(t, rr)
	// Description should be null/nil when not provided
	if resp["description"] != nil {
		t.Errorf("description: expected null, got %v", resp["description"])
	}
}

// --- Delete tests ---

func TestCategoryDelete_Valid(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()
	catID := uuid.New()

	store.categories[catID] = database.Category{
		ID: catID, OutletID: outletID, Name: "Delete Me",
		SortOrder: 0, IsActive: true, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/categories/"+catID.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	// Verify the category is soft-deleted
	c := store.categories[catID]
	if c.IsActive {
		t.Error("expected category to be soft-deleted (is_active=false)")
	}
}

func TestCategoryDelete_SoftDeleteDoesNotRemove(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()
	catID := uuid.New()

	store.categories[catID] = database.Category{
		ID: catID, OutletID: outletID, Name: "Soft Delete",
		SortOrder: 0, IsActive: true, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)
	doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/categories/"+catID.String(), nil)

	// Category should still exist in store, just inactive
	c, exists := store.categories[catID]
	if !exists {
		t.Fatal("expected category to still exist in store after soft delete")
	}
	if c.IsActive {
		t.Error("expected is_active=false after soft delete")
	}
}

func TestCategoryDelete_NotFound(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()
	catID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/categories/"+catID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestCategoryDelete_WrongOutlet(t *testing.T) {
	store := newMockCategoryStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	catID := uuid.New()

	store.categories[catID] = database.Category{
		ID: catID, OutletID: outletID, Name: "Wrong Outlet",
		SortOrder: 0, IsActive: true, CreatedAt: time.Now(),
	}

	router := setupCategoryRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+wrongOutletID.String()+"/categories/"+catID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Verify original category is still active
	c := store.categories[catID]
	if !c.IsActive {
		t.Error("category in original outlet should not be affected")
	}
}

func TestCategoryDelete_InvalidOutletID(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	catID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/not-a-uuid/categories/"+catID.String(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCategoryDelete_InvalidCategoryID(t *testing.T) {
	store := newMockCategoryStore()
	router := setupCategoryRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/categories/not-a-uuid", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
