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
	"github.com/kiwari-pos/api/internal/auth"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/handler"
	"github.com/kiwari-pos/api/internal/middleware"
	"github.com/shopspring/decimal"
)

// --- Mock PaymentStore ---

type mockPaymentStore struct {
	orders   map[uuid.UUID]database.Order
	payments map[uuid.UUID]database.Payment // keyed by payment ID
}

func newMockPaymentStore() *mockPaymentStore {
	return &mockPaymentStore{
		orders:   make(map[uuid.UUID]database.Order),
		payments: make(map[uuid.UUID]database.Payment),
	}
}

func (m *mockPaymentStore) GetOrder(_ context.Context, arg database.GetOrderParams) (database.Order, error) {
	o, ok := m.orders[arg.ID]
	if !ok || o.OutletID != arg.OutletID {
		return database.Order{}, pgx.ErrNoRows
	}
	return o, nil
}

func (m *mockPaymentStore) GetOrderForUpdate(_ context.Context, arg database.GetOrderForUpdateParams) (database.Order, error) {
	o, ok := m.orders[arg.ID]
	if !ok || o.OutletID != arg.OutletID {
		return database.Order{}, pgx.ErrNoRows
	}
	return o, nil
}

func (m *mockPaymentStore) ListPaymentsByOrder(_ context.Context, orderID uuid.UUID) ([]database.Payment, error) {
	var result []database.Payment
	for _, p := range m.payments {
		if p.OrderID == orderID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPaymentStore) CreatePayment(_ context.Context, arg database.CreatePaymentParams) (database.Payment, error) {
	p := database.Payment{
		ID:              uuid.New(),
		OrderID:         arg.OrderID,
		PaymentMethod:   arg.PaymentMethod,
		Amount:          arg.Amount,
		Status:          arg.Status,
		ReferenceNumber: arg.ReferenceNumber,
		AmountReceived:  arg.AmountReceived,
		ChangeAmount:    arg.ChangeAmount,
		ProcessedBy:     arg.ProcessedBy,
		ProcessedAt:     time.Now(),
	}
	m.payments[p.ID] = p
	return p, nil
}

func (m *mockPaymentStore) SumPaymentsByOrder(_ context.Context, orderID uuid.UUID) (pgtype.Numeric, error) {
	total := decimal.Zero
	for _, p := range m.payments {
		if p.OrderID == orderID && p.Status == database.PaymentStatusCOMPLETED {
			amt, _ := numericToDecimal(p.Amount)
			total = total.Add(amt)
		}
	}
	return decimalToNumeric(total), nil
}

func (m *mockPaymentStore) CompleteOrder(_ context.Context, id uuid.UUID) (database.Order, error) {
	o, ok := m.orders[id]
	if !ok || o.Status == database.OrderStatusCANCELLED {
		return database.Order{}, pgx.ErrNoRows
	}
	o.Status = database.OrderStatusCOMPLETED
	now := time.Now()
	o.CompletedAt = pgtype.Timestamptz{Time: now, Valid: true}
	o.UpdatedAt = now
	m.orders[id] = o
	return o, nil
}

func (m *mockPaymentStore) UpdateCateringStatus(_ context.Context, arg database.UpdateCateringStatusParams) (database.Order, error) {
	o, ok := m.orders[arg.ID]
	if !ok {
		return database.Order{}, pgx.ErrNoRows
	}
	o.CateringStatus = arg.CateringStatus
	o.UpdatedAt = time.Now()
	m.orders[arg.ID] = o
	return o, nil
}

// --- Helpers ---

// numericToDecimal converts pgtype.Numeric to decimal.Decimal (for tests)
func numericToDecimal(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid {
		return decimal.Zero, nil
	}
	val, err := n.Value()
	if err != nil || val == nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(val.(string))
}

// decimalToNumeric converts decimal.Decimal to pgtype.Numeric (for tests)
func decimalToNumeric(d decimal.Decimal) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(d.String())
	return n
}

func setupPaymentRouterWithStore(store *mockPaymentStore, claims *auth.Claims) *chi.Mux {
	pool := &mockPool{}
	newStore := func(db database.DBTX) handler.PaymentStore {
		return store
	}
	h := handler.NewPaymentHandler(store, pool, newStore)
	r := chi.NewRouter()
	r.Use(middleware.Authenticate(testJWTSecret))
	r.Route("/outlets/{oid}/orders/{id}/payments", h.RegisterRoutes)
	return r
}

func decodePaymentResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodePaymentListResponse(t *testing.T, rr *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// --- Add Payment Tests ---

func TestAddPayment_Cash_HappyPath(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	// Create a NEW order with total 100000
	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-001",
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method":  "CASH",
			"amount":          "50000",
			"amount_received": "100000",
		}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodePaymentResponse(t, rr)
	payment := resp["payment"].(map[string]interface{})
	if payment["payment_method"] != "CASH" {
		t.Errorf("payment_method: got %v, want CASH", payment["payment_method"])
	}
	if payment["amount"] != "50000.00" {
		t.Errorf("amount: got %v, want 50000.00", payment["amount"])
	}
	if payment["amount_received"] != "100000.00" {
		t.Errorf("amount_received: got %v, want 100000.00", payment["amount_received"])
	}
	if payment["change_amount"] != "50000.00" {
		t.Errorf("change_amount: got %v, want 50000.00", payment["change_amount"])
	}
	if payment["status"] != "COMPLETED" {
		t.Errorf("status: got %v, want COMPLETED", payment["status"])
	}

	// Order should still be NEW (not fully paid)
	order := resp["order"].(map[string]interface{})
	if order["status"] != "NEW" {
		t.Errorf("order status: got %v, want NEW", order["status"])
	}
}

func TestAddPayment_QRIS_WithReference(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-002",
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(75000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method":   "QRIS",
			"amount":           "75000",
			"reference_number": "QRIS-1234567890",
		}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodePaymentResponse(t, rr)
	payment := resp["payment"].(map[string]interface{})
	if payment["payment_method"] != "QRIS" {
		t.Errorf("payment_method: got %v, want QRIS", payment["payment_method"])
	}
	if payment["reference_number"] != "QRIS-1234567890" {
		t.Errorf("reference_number: got %v, want QRIS-1234567890", payment["reference_number"])
	}
}

func TestAddPayment_Transfer(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-003",
		Status:      database.OrderStatusPREPARING,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(200000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method":   "TRANSFER",
			"amount":           "200000",
			"reference_number": "TRF-9876543210",
		}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodePaymentResponse(t, rr)
	payment := resp["payment"].(map[string]interface{})
	if payment["payment_method"] != "TRANSFER" {
		t.Errorf("payment_method: got %v, want TRANSFER", payment["payment_method"])
	}
}

func TestAddPayment_AutoCompleteOrder(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	// Order with total 100000, already has 40000 paid
	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-004",
		Status:      database.OrderStatusREADY,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Existing payment of 40000
	existingPayment := uuid.New()
	store.payments[existingPayment] = database.Payment{
		ID:            existingPayment,
		OrderID:       orderID,
		PaymentMethod: database.PaymentMethodCASH,
		Amount:        decimalToNumeric(decimal.NewFromInt(40000)),
		Status:        database.PaymentStatusCOMPLETED,
		ProcessedBy:   userID,
		ProcessedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	// Add payment of 60000 to complete the order
	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method":  "CASH",
			"amount":          "60000",
			"amount_received": "60000",
		}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodePaymentResponse(t, rr)
	order := resp["order"].(map[string]interface{})
	if order["status"] != "COMPLETED" {
		t.Errorf("order status: got %v, want COMPLETED (auto-completed)", order["status"])
	}
}

func TestAddPayment_PartialPayment(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-005",
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	// Partial payment (30000 of 100000)
	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "QRIS",
			"amount":         "30000",
		}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodePaymentResponse(t, rr)
	order := resp["order"].(map[string]interface{})
	// Order should NOT be completed (partial payment)
	if order["status"] != "NEW" {
		t.Errorf("order status: got %v, want NEW (partial payment)", order["status"])
	}
}

func TestAddPayment_ExceedsRemainingBalance(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-006",
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Existing payment of 80000
	existingPayment := uuid.New()
	store.payments[existingPayment] = database.Payment{
		ID:            existingPayment,
		OrderID:       orderID,
		PaymentMethod: database.PaymentMethodCASH,
		Amount:        decimalToNumeric(decimal.NewFromInt(80000)),
		Status:        database.PaymentStatusCOMPLETED,
		ProcessedBy:   userID,
		ProcessedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	// Try to pay 30000 (would total 110000, exceeding 100000)
	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "QRIS",
			"amount":         "30000",
		}, claims)

	if rr.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d (overpayment)", rr.Code, http.StatusConflict)
	}

	resp := decodePaymentResponse(t, rr)
	if resp["error"] != "payment exceeds remaining balance" {
		t.Errorf("error: got %v, want 'payment exceeds remaining balance'", resp["error"])
	}
}

func TestAddPayment_CancelledOrder(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-007",
		Status:      database.OrderStatusCANCELLED,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "QRIS",
			"amount":         "100000",
		}, claims)

	if rr.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d (cancelled order)", rr.Code, http.StatusConflict)
	}
}

func TestAddPayment_CompletedOrder(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-007b",
		Status:      database.OrderStatusCOMPLETED,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "QRIS",
			"amount":         "100000",
		}, claims)

	if rr.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d (completed order)", rr.Code, http.StatusConflict)
	}
}

func TestAddPayment_AlreadyFullyPaid(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "ORD-008",
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Existing payment of full amount
	existingPayment := uuid.New()
	store.payments[existingPayment] = database.Payment{
		ID:            existingPayment,
		OrderID:       orderID,
		PaymentMethod: database.PaymentMethodCASH,
		Amount:        decimalToNumeric(decimal.NewFromInt(100000)),
		Status:        database.PaymentStatusCOMPLETED,
		ProcessedBy:   userID,
		ProcessedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "QRIS",
			"amount":         "10000",
		}, claims)

	if rr.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d (already fully paid)", rr.Code, http.StatusConflict)
	}
}

func TestAddPayment_Cash_MissingAmountReceived(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "CASH",
			"amount":         "50000",
		}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAddPayment_Cash_AmountReceivedLessThanAmount(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method":  "CASH",
			"amount":          "100000",
			"amount_received": "50000", // Less than amount!
		}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAddPayment_InvalidPaymentMethod(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "BITCOIN", // Invalid!
			"amount":         "100000",
		}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAddPayment_InvalidAmount(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	// Test negative amount
	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "CASH",
			"amount":         "-100",
		}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d (negative amount)", rr.Code, http.StatusBadRequest)
	}

	// Test zero amount
	rr = doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "CASH",
			"amount":         "0",
		}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d (zero amount)", rr.Code, http.StatusBadRequest)
	}
}

func TestAddPayment_OrderNotFound(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "QRIS",
			"amount":         "100000",
		}, claims)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestAddPayment_InvalidOutletID(t *testing.T) {
	store := newMockPaymentStore()
	orderID := uuid.New()
	userID := uuid.New()
	claims := &auth.Claims{UserID: userID, OutletID: uuid.New(), Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "POST",
		"/outlets/not-a-uuid/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "CASH",
			"amount":         "100000",
		}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAddPayment_MissingAuth(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(100000)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pool := &mockPool{}
	newStore := func(db database.DBTX) handler.PaymentStore { return store }
	h := handler.NewPaymentHandler(store, pool, newStore)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/orders/{id}/payments", h.RegisterRoutes)

	// Don't use doAuthRequest (no auth header)
	rr := doRequest(t, r, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "CASH",
			"amount":         "100000",
		})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAddPayment_Catering_FirstPayment_DpPaid(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	// Catering order with BOOKED status
	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "CAT-001",
		OrderType:   database.OrderTypeCATERING,
		Status:      database.OrderStatusNEW,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(500000)),
		CateringStatus: database.NullCateringStatus{
			CateringStatus: database.CateringStatusBOOKED,
			Valid:          true,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	// First payment (down payment)
	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "TRANSFER",
			"amount":         "100000",
		}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodePaymentResponse(t, rr)
	order := resp["order"].(map[string]interface{})
	if order["catering_status"] != "DP_PAID" {
		t.Errorf("catering_status: got %v, want DP_PAID", order["catering_status"])
	}
}

func TestAddPayment_Catering_FullPayment_Settled(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	// Catering order with DP_PAID status
	store.orders[orderID] = database.Order{
		ID:          orderID,
		OutletID:    outletID,
		OrderNumber: "CAT-002",
		OrderType:   database.OrderTypeCATERING,
		Status:      database.OrderStatusREADY,
		TotalAmount: decimalToNumeric(decimal.NewFromInt(500000)),
		CateringStatus: database.NullCateringStatus{
			CateringStatus: database.CateringStatusDPPAID,
			Valid:          true,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Existing down payment of 100000
	existingPayment := uuid.New()
	store.payments[existingPayment] = database.Payment{
		ID:            existingPayment,
		OrderID:       orderID,
		PaymentMethod: database.PaymentMethodTRANSFER,
		Amount:        decimalToNumeric(decimal.NewFromInt(100000)),
		Status:        database.PaymentStatusCOMPLETED,
		ProcessedBy:   userID,
		ProcessedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	// Full payment (remaining 400000)
	rr := doAuthRequest(t, router, "POST",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		map[string]interface{}{
			"payment_method": "TRANSFER",
			"amount":         "400000",
		}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodePaymentResponse(t, rr)
	order := resp["order"].(map[string]interface{})
	if order["catering_status"] != "SETTLED" {
		t.Errorf("catering_status: got %v, want SETTLED", order["catering_status"])
	}
	if order["status"] != "COMPLETED" {
		t.Errorf("order status: got %v, want COMPLETED (auto-completed)", order["status"])
	}
}

// --- List Payment Tests ---

func TestListPayments_HappyPath(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:       orderID,
		OutletID: outletID,
		Status:   database.OrderStatusNEW,
	}

	// Add two payments
	p1 := uuid.New()
	store.payments[p1] = database.Payment{
		ID:            p1,
		OrderID:       orderID,
		PaymentMethod: database.PaymentMethodCASH,
		Amount:        decimalToNumeric(decimal.NewFromInt(50000)),
		Status:        database.PaymentStatusCOMPLETED,
		ProcessedBy:   userID,
		ProcessedAt:   time.Now(),
	}

	p2 := uuid.New()
	store.payments[p2] = database.Payment{
		ID:            p2,
		OrderID:       orderID,
		PaymentMethod: database.PaymentMethodQRIS,
		Amount:        decimalToNumeric(decimal.NewFromInt(30000)),
		Status:        database.PaymentStatusCOMPLETED,
		ProcessedBy:   userID,
		ProcessedAt:   time.Now(),
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "GET",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodePaymentListResponse(t, rr)
	if len(resp) != 2 {
		t.Errorf("expected 2 payments, got %d", len(resp))
	}
}

func TestListPayments_Empty(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	store.orders[orderID] = database.Order{
		ID:       orderID,
		OutletID: outletID,
		Status:   database.OrderStatusNEW,
	}

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "GET",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodePaymentListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected 0 payments, got %d", len(resp))
	}
}

func TestListPayments_OrderNotFound(t *testing.T) {
	store := newMockPaymentStore()
	outletID := uuid.New()
	orderID := uuid.New()
	userID := uuid.New()

	claims := &auth.Claims{UserID: userID, OutletID: outletID, Role: "CASHIER"}
	router := setupPaymentRouterWithStore(store, claims)

	rr := doAuthRequest(t, router, "GET",
		"/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/payments",
		nil, claims)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}
