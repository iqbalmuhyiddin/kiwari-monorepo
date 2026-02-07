package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

type mockCustomerStore struct {
	customers map[uuid.UUID]database.Customer // keyed by customer ID
	orders    map[uuid.UUID]database.Order    // keyed by order ID
}

func newMockCustomerStore() *mockCustomerStore {
	return &mockCustomerStore{
		customers: make(map[uuid.UUID]database.Customer),
		orders:    make(map[uuid.UUID]database.Order),
	}
}

func (m *mockCustomerStore) ListCustomersByOutlet(_ context.Context, arg database.ListCustomersByOutletParams) ([]database.Customer, error) {
	var result []database.Customer
	for _, c := range m.customers {
		if c.OutletID == arg.OutletID && c.IsActive {
			// Apply search filter
			if arg.Search.Valid {
				search := strings.ToLower(arg.Search.String)
				if !strings.Contains(strings.ToLower(c.Phone), search) && !strings.Contains(strings.ToLower(c.Name), search) {
					continue
				}
			}
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockCustomerStore) GetCustomer(_ context.Context, arg database.GetCustomerParams) (database.Customer, error) {
	c, ok := m.customers[arg.ID]
	if !ok || c.OutletID != arg.OutletID || !c.IsActive {
		return database.Customer{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *mockCustomerStore) CreateCustomer(_ context.Context, arg database.CreateCustomerParams) (database.Customer, error) {
	// Check for duplicate phone in same outlet
	for _, c := range m.customers {
		if c.OutletID == arg.OutletID && c.Phone == arg.Phone && c.IsActive {
			return database.Customer{}, &pgconn.PgError{Code: "23505"}
		}
	}

	c := database.Customer{
		ID:        uuid.New(),
		OutletID:  arg.OutletID,
		Name:      arg.Name,
		Phone:     arg.Phone,
		Email:     arg.Email,
		Notes:     arg.Notes,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.customers[c.ID] = c
	return c, nil
}

func (m *mockCustomerStore) UpdateCustomer(_ context.Context, arg database.UpdateCustomerParams) (database.Customer, error) {
	c, ok := m.customers[arg.ID]
	if !ok || c.OutletID != arg.OutletID || !c.IsActive {
		return database.Customer{}, pgx.ErrNoRows
	}

	// Check for duplicate phone in same outlet (excluding self)
	for _, existing := range m.customers {
		if existing.ID != arg.ID && existing.OutletID == arg.OutletID && existing.Phone == arg.Phone && existing.IsActive {
			return database.Customer{}, &pgconn.PgError{Code: "23505"}
		}
	}

	c.Name = arg.Name
	c.Phone = arg.Phone
	c.Email = arg.Email
	c.Notes = arg.Notes
	c.UpdatedAt = time.Now()
	m.customers[c.ID] = c
	return c, nil
}

func (m *mockCustomerStore) SoftDeleteCustomer(_ context.Context, arg database.SoftDeleteCustomerParams) (uuid.UUID, error) {
	c, ok := m.customers[arg.ID]
	if !ok || c.OutletID != arg.OutletID || !c.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	c.IsActive = false
	c.UpdatedAt = time.Now()
	m.customers[c.ID] = c
	return c.ID, nil
}

func (m *mockCustomerStore) GetCustomerStats(_ context.Context, arg database.GetCustomerStatsParams) (database.GetCustomerStatsRow, error) {
	// Count orders and calculate stats from mock orders (scoped to outlet)
	var totalOrders int64
	var totalSpend, sumForAvg pgtype.Numeric

	for _, o := range m.orders {
		if o.CustomerID.Valid && o.CustomerID.Bytes == arg.CustomerID.Bytes && o.OutletID == arg.OutletID && o.Status != database.OrderStatusCANCELLED {
			totalOrders++
			if totalSpend.Valid {
				totalSpend = o.TotalAmount
			} else {
				totalSpend = o.TotalAmount
			}
			sumForAvg = o.TotalAmount
		}
	}

	var avgTicket pgtype.Numeric
	if totalOrders > 0 {
		avgTicket = sumForAvg
	}

	return database.GetCustomerStatsRow{
		TotalOrders: totalOrders,
		TotalSpend:  totalSpend,
		AvgTicket:   avgTicket,
	}, nil
}

func (m *mockCustomerStore) GetCustomerTopItems(_ context.Context, arg database.GetCustomerTopItemsParams) ([]database.GetCustomerTopItemsRow, error) {
	// For mock purposes, return empty or preset values
	return []database.GetCustomerTopItemsRow{}, nil
}

func (m *mockCustomerStore) ListCustomerOrders(_ context.Context, arg database.ListCustomerOrdersParams) ([]database.Order, error) {
	var result []database.Order
	for _, o := range m.orders {
		if o.CustomerID.Valid && o.CustomerID.Bytes == arg.CustomerID.Bytes && o.OutletID == arg.OutletID {
			result = append(result, o)
		}
	}
	return result, nil
}

// --- Helpers ---

func setupCustomerRouter(store *mockCustomerStore) *chi.Mux {
	h := handler.NewCustomerHandler(store)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/customers", h.RegisterRoutes)
	return r
}

func decodeCustomerResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeCustomerListResponse(t *testing.T, rr *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// --- Tests ---

func TestCustomerList(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	// Add test customers
	customer1 := database.Customer{
		ID:        uuid.New(),
		OutletID:  outletID,
		Name:      "John Doe",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	customer2 := database.Customer{
		ID:        uuid.New(),
		OutletID:  outletID,
		Name:      "Jane Smith",
		Phone:     "089876543210",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customer1.ID] = customer1
	store.customers[customer2.ID] = customer2

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerListResponse(t, rr)
	if len(resp) != 2 {
		t.Errorf("expected 2 customers, got %d", len(resp))
	}
}

func TestCustomerListWithSearch(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	customer1 := database.Customer{
		ID:        uuid.New(),
		OutletID:  outletID,
		Name:      "John Doe",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	customer2 := database.Customer{
		ID:        uuid.New(),
		OutletID:  outletID,
		Name:      "Jane Smith",
		Phone:     "089876543210",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customer1.ID] = customer1
	store.customers[customer2.ID] = customer2

	// Search by phone
	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers?search=0812", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerListResponse(t, rr)
	if len(resp) != 1 {
		t.Errorf("expected 1 customer, got %d", len(resp))
	}
}

func TestCustomerListWithNameSearch(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	customer1 := database.Customer{
		ID:        uuid.New(),
		OutletID:  outletID,
		Name:      "John Doe",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	customer2 := database.Customer{
		ID:        uuid.New(),
		OutletID:  outletID,
		Name:      "Jane Smith",
		Phone:     "089876543210",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customer1.ID] = customer1
	store.customers[customer2.ID] = customer2

	// Search by name (case insensitive)
	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers?search=jane", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerListResponse(t, rr)
	if len(resp) != 1 {
		t.Errorf("expected 1 customer, got %d", len(resp))
	}
}

func TestCustomerListEmpty(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected 0 customers, got %d", len(resp))
	}
}

func TestCustomerListInvalidOutletID(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/outlets/invalid/customers", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCustomerGet(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customer := database.Customer{
		ID:        uuid.New(),
		OutletID:  outletID,
		Name:      "Test Customer",
		Phone:     "081234567890",
		Email:     pgtype.Text{String: "test@example.com", Valid: true},
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customer.ID] = customer

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers/"+customer.ID.String(), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	resp := decodeCustomerResponse(t, rr)
	if resp["name"] != "Test Customer" {
		t.Errorf("name: got %v, want Test Customer", resp["name"])
	}
	if resp["phone"] != "081234567890" {
		t.Errorf("phone: got %v, want 081234567890", resp["phone"])
	}
	if resp["email"] != "test@example.com" {
		t.Errorf("email: got %v, want test@example.com", resp["email"])
	}
}

func TestCustomerGetNotFound(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers/"+customerID.String(), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestCustomerCreate(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	body := map[string]interface{}{
		"name":  "John Doe",
		"phone": "081234567890",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/outlets/"+outletID.String()+"/customers", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	resp := decodeCustomerResponse(t, rr)
	if resp["name"] != "John Doe" {
		t.Errorf("expected name 'John Doe', got %v", resp["name"])
	}
	if resp["phone"] != "081234567890" {
		t.Errorf("expected phone '081234567890', got %v", resp["phone"])
	}
}

func TestCustomerCreateWithOptionalFields(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	body := map[string]interface{}{
		"name":  "John Doe",
		"phone": "081234567890",
		"email": "john@example.com",
		"notes": "VIP customer",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/outlets/"+outletID.String()+"/customers", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	resp := decodeCustomerResponse(t, rr)
	if resp["email"] != "john@example.com" {
		t.Errorf("expected email 'john@example.com', got %v", resp["email"])
	}
	if resp["notes"] != "VIP customer" {
		t.Errorf("expected notes 'VIP customer', got %v", resp["notes"])
	}
}

func TestCustomerCreateMissingName(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	body := map[string]interface{}{
		"phone": "081234567890",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/outlets/"+outletID.String()+"/customers", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	resp := decodeCustomerResponse(t, rr)
	if !strings.Contains(resp["error"].(string), "name is required") {
		t.Errorf("expected 'name is required' error, got %v", resp["error"])
	}
}

func TestCustomerCreateMissingPhone(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	body := map[string]interface{}{
		"name": "John Doe",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/outlets/"+outletID.String()+"/customers", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	resp := decodeCustomerResponse(t, rr)
	if !strings.Contains(resp["error"].(string), "phone is required") {
		t.Errorf("expected 'phone is required' error, got %v", resp["error"])
	}
}

func TestCustomerCreateDuplicatePhone(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()

	// Create first customer
	existing := database.Customer{
		ID:        uuid.New(),
		OutletID:  outletID,
		Name:      "Existing Customer",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[existing.ID] = existing

	// Try to create duplicate
	body := map[string]interface{}{
		"name":  "John Doe",
		"phone": "081234567890",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/outlets/"+outletID.String()+"/customers", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}

	resp := decodeCustomerResponse(t, rr)
	if !strings.Contains(resp["error"].(string), "phone already exists") {
		t.Errorf("expected 'phone already exists' error, got %v", resp["error"])
	}
}

func TestCustomerUpdate(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	// Create existing customer
	existing := database.Customer{
		ID:        customerID,
		OutletID:  outletID,
		Name:      "Old Name",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customerID] = existing

	body := map[string]interface{}{
		"name":  "New Name",
		"phone": "089999999999",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/outlets/"+outletID.String()+"/customers/"+customerID.String(), bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerResponse(t, rr)
	if resp["name"] != "New Name" {
		t.Errorf("expected name 'New Name', got %v", resp["name"])
	}
	if resp["phone"] != "089999999999" {
		t.Errorf("expected phone '089999999999', got %v", resp["phone"])
	}
}

func TestCustomerUpdateNotFound(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	body := map[string]interface{}{
		"name":  "New Name",
		"phone": "089999999999",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/outlets/"+outletID.String()+"/customers/"+customerID.String(), bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestCustomerUpdateMissingName(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	existing := database.Customer{
		ID:        customerID,
		OutletID:  outletID,
		Name:      "Old Name",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customerID] = existing

	body := map[string]interface{}{
		"phone": "089999999999",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/outlets/"+outletID.String()+"/customers/"+customerID.String(), bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCustomerUpdateDuplicatePhone(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID1 := uuid.New()
	customerID2 := uuid.New()

	customer1 := database.Customer{
		ID:        customerID1,
		OutletID:  outletID,
		Name:      "Customer 1",
		Phone:     "081111111111",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	customer2 := database.Customer{
		ID:        customerID2,
		OutletID:  outletID,
		Name:      "Customer 2",
		Phone:     "082222222222",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customerID1] = customer1
	store.customers[customerID2] = customer2

	// Try to update customer2 to use customer1's phone
	body := map[string]interface{}{
		"name":  "Customer 2",
		"phone": "081111111111",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/outlets/"+outletID.String()+"/customers/"+customerID2.String(), bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", rr.Code)
	}
}

func TestCustomerDelete(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	existing := database.Customer{
		ID:        customerID,
		OutletID:  outletID,
		Name:      "John Doe",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customerID] = existing

	req := httptest.NewRequest(http.MethodDelete, "/outlets/"+outletID.String()+"/customers/"+customerID.String(), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rr.Code)
	}

	// Verify customer is soft deleted
	if store.customers[customerID].IsActive {
		t.Error("expected customer to be soft deleted")
	}
}

func TestCustomerDeleteNotFound(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/outlets/"+outletID.String()+"/customers/"+customerID.String(), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestCustomerStatsWithOrders(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	customer := database.Customer{
		ID:        customerID,
		OutletID:  outletID,
		Name:      "John Doe",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customerID] = customer

	// Add some orders
	var totalAmount pgtype.Numeric
	totalAmount.Scan("150000.00")

	order := database.Order{
		ID:          uuid.New(),
		OutletID:    outletID,
		CustomerID:  pgtype.UUID{Bytes: customerID, Valid: true},
		Status:      database.OrderStatusCOMPLETED,
		TotalAmount: totalAmount,
		CreatedAt:   time.Now(),
	}
	store.orders[order.ID] = order

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers/"+customerID.String()+"/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerResponse(t, rr)
	if resp["total_orders"].(float64) != 1 {
		t.Errorf("expected total_orders 1, got %v", resp["total_orders"])
	}
}

func TestCustomerStatsNoOrders(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	customer := database.Customer{
		ID:        customerID,
		OutletID:  outletID,
		Name:      "John Doe",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customerID] = customer

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers/"+customerID.String()+"/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerResponse(t, rr)
	if resp["total_orders"].(float64) != 0 {
		t.Errorf("expected total_orders 0, got %v", resp["total_orders"])
	}
}

func TestCustomerStatsNotFound(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers/"+customerID.String()+"/stats", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestCustomerOrderHistory(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	customer := database.Customer{
		ID:        customerID,
		OutletID:  outletID,
		Name:      "John Doe",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customerID] = customer

	// Add order
	var totalAmount pgtype.Numeric
	totalAmount.Scan("150000.00")

	order := database.Order{
		ID:          uuid.New(),
		OutletID:    outletID,
		OrderNumber: "ORD-001",
		CustomerID:  pgtype.UUID{Bytes: customerID, Valid: true},
		OrderType:   database.OrderTypeDINEIN,
		Status:      database.OrderStatusCOMPLETED,
		TotalAmount: totalAmount,
		CreatedBy:   uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	store.orders[order.ID] = order

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers/"+customerID.String()+"/orders", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerListResponse(t, rr)
	if len(resp) != 1 {
		t.Errorf("expected 1 order, got %d", len(resp))
	}
}

func TestCustomerOrderHistoryEmpty(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	customer := database.Customer{
		ID:        customerID,
		OutletID:  outletID,
		Name:      "John Doe",
		Phone:     "081234567890",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.customers[customerID] = customer

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers/"+customerID.String()+"/orders", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	resp := decodeCustomerListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected 0 orders, got %d", len(resp))
	}
}

func TestCustomerOrderHistoryNotFound(t *testing.T) {
	store := newMockCustomerStore()
	router := setupCustomerRouter(store)

	outletID := uuid.New()
	customerID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/outlets/"+outletID.String()+"/customers/"+customerID.String()+"/orders", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}
