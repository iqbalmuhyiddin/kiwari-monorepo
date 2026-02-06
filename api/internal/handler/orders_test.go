package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/auth"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/handler"
	"github.com/kiwari-pos/api/internal/middleware"
	"github.com/kiwari-pos/api/internal/service"
)

// --- Mock OrderServicer ---

type mockOrderService struct {
	createFn func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error)
}

func (m *mockOrderService) CreateOrder(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
	return m.createFn(ctx, req)
}

// --- Test helpers ---

const testJWTSecret = "test-secret-for-orders"

func setupOrderRouterWithAuth(svc *mockOrderService, claims *auth.Claims) *chi.Mux {
	h := handler.NewOrderHandler(svc)
	r := chi.NewRouter()
	r.Use(middleware.Authenticate(testJWTSecret))
	r.Route("/outlets/{oid}/orders", h.RegisterRoutes)
	return r
}

func doAuthRequest(t *testing.T, router http.Handler, method, path string, body interface{}, claims *auth.Claims) *httptest.ResponseRecorder {
	t.Helper()

	// Generate a real JWT token from claims
	token, err := auth.GenerateToken(testJWTSecret, claims.UserID, claims.OutletID, claims.Role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

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
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeOrderResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// --- Helpers to build test data ---

func testClaims(outletID uuid.UUID) *auth.Claims {
	return &auth.Claims{
		UserID:   uuid.New(),
		OutletID: outletID,
		Role:     "CASHIER",
	}
}

func testOrderResult(outletID, userID uuid.UUID) *service.CreateOrderResult {
	orderID := uuid.New()
	itemID := uuid.New()
	now := time.Now()

	return &service.CreateOrderResult{
		Order: database.Order{
			ID:          orderID,
			OutletID:    outletID,
			OrderNumber: "KWR-001",
			OrderType:   database.OrderTypeDINEIN,
			Status:      database.OrderStatusNEW,
			Subtotal:    testNumeric("25000.00"),
			DiscountAmount: testNumeric("0.00"),
			TaxAmount:   testNumeric("0.00"),
			TotalAmount: testNumeric("25000.00"),
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Items: []service.OrderItemResult{
			{
				Item: database.OrderItem{
					ID:             itemID,
					OrderID:        orderID,
					ProductID:      uuid.New(),
					Quantity:       2,
					UnitPrice:      testNumeric("12500.00"),
					DiscountAmount: testNumeric("0.00"),
					Subtotal:       testNumeric("25000.00"),
					Status:         database.OrderItemStatusPENDING,
					Station:        database.NullKitchenStation{KitchenStation: database.KitchenStationGRILL, Valid: true},
				},
				Modifiers: nil,
			},
		},
	}
}

// --- Tests ---

func TestOrderCreate_HappyPath(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			if req.OutletID != outletID {
				t.Errorf("outlet_id: got %v, want %v", req.OutletID, outletID)
			}
			if req.CreatedBy != claims.UserID {
				t.Errorf("created_by: got %v, want %v", req.CreatedBy, claims.UserID)
			}
			if req.OrderType != "DINE_IN" {
				t.Errorf("order_type: got %v, want DINE_IN", req.OrderType)
			}
			if len(req.Items) != 1 {
				t.Errorf("items count: got %d, want 1", len(req.Items))
			}
			return testOrderResult(outletID, claims.UserID), nil
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{
				"product_id": uuid.New().String(),
				"quantity":   2,
			},
		},
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["order_number"] != "KWR-001" {
		t.Errorf("order_number: got %v, want KWR-001", resp["order_number"])
	}
	if resp["order_type"] != "DINE_IN" {
		t.Errorf("order_type: got %v, want DINE_IN", resp["order_type"])
	}
	if resp["status"] != "NEW" {
		t.Errorf("status: got %v, want NEW", resp["status"])
	}
	if resp["total_amount"] != "25000.00" {
		t.Errorf("total_amount: got %v, want 25000.00", resp["total_amount"])
	}

	// Verify items are present
	items, ok := resp["items"].([]interface{})
	if !ok {
		t.Fatal("items not present in response")
	}
	if len(items) != 1 {
		t.Fatalf("items count: got %d, want 1", len(items))
	}

	item := items[0].(map[string]interface{})
	if item["quantity"] != float64(2) {
		t.Errorf("item quantity: got %v, want 2", item["quantity"])
	}
	if item["unit_price"] != "12500.00" {
		t.Errorf("item unit_price: got %v, want 12500.00", item["unit_price"])
	}
	if item["status"] != "PENDING" {
		t.Errorf("item status: got %v, want PENDING", item["status"])
	}
	if item["station"] != "GRILL" {
		t.Errorf("item station: got %v, want GRILL", item["station"])
	}
}

func TestOrderCreate_WithModifiers(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	modifierID := uuid.New()

	orderID := uuid.New()
	itemID := uuid.New()
	modResultID := uuid.New()
	now := time.Now()

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			if len(req.Items[0].Modifiers) != 1 {
				t.Errorf("modifiers count: got %d, want 1", len(req.Items[0].Modifiers))
			}
			return &service.CreateOrderResult{
				Order: database.Order{
					ID: orderID, OutletID: outletID, OrderNumber: "KWR-001",
					OrderType: database.OrderTypeDINEIN, Status: database.OrderStatusNEW,
					Subtotal: testNumeric("30000.00"), DiscountAmount: testNumeric("0.00"),
					TaxAmount: testNumeric("0.00"), TotalAmount: testNumeric("30000.00"),
					CreatedBy: claims.UserID, CreatedAt: now, UpdatedAt: now,
				},
				Items: []service.OrderItemResult{
					{
						Item: database.OrderItem{
							ID: itemID, OrderID: orderID, ProductID: uuid.New(),
							Quantity: 1, UnitPrice: testNumeric("25000.00"),
							DiscountAmount: testNumeric("0.00"), Subtotal: testNumeric("30000.00"),
							Status: database.OrderItemStatusPENDING,
						},
						Modifiers: []database.OrderItemModifier{
							{
								ID: modResultID, OrderItemID: itemID,
								ModifierID: modifierID, Quantity: 1,
								UnitPrice: testNumeric("5000.00"),
							},
						},
					},
				},
			}, nil
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{
				"product_id": uuid.New().String(),
				"quantity":   1,
				"modifiers": []map[string]interface{}{
					{"modifier_id": modifierID.String(), "quantity": 1},
				},
			},
		},
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	items := resp["items"].([]interface{})
	item := items[0].(map[string]interface{})
	mods := item["modifiers"].([]interface{})
	if len(mods) != 1 {
		t.Fatalf("modifier count: got %d, want 1", len(mods))
	}
	mod := mods[0].(map[string]interface{})
	if mod["unit_price"] != "5000.00" {
		t.Errorf("modifier unit_price: got %v, want 5000.00", mod["unit_price"])
	}
}

func TestOrderCreate_MissingOrderType(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{}
	router := setupOrderRouterWithAuth(svc, claims)

	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1},
		},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	resp := decodeOrderResponse(t, rr)
	if resp["error"] != "order_type is required" {
		t.Errorf("error: got %v, want 'order_type is required'", resp["error"])
	}
}

func TestOrderCreate_EmptyItems(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{}
	router := setupOrderRouterWithAuth(svc, claims)

	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items":      []map[string]interface{}{},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	resp := decodeOrderResponse(t, rr)
	if resp["error"] != "items are required" {
		t.Errorf("error: got %v, want 'items are required'", resp["error"])
	}
}

func TestOrderCreate_MissingProductID(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{}
	router := setupOrderRouterWithAuth(svc, claims)

	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{"quantity": 1},
		},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	resp := decodeOrderResponse(t, rr)
	if resp["error"] != "items[0]: product_id is required" {
		t.Errorf("error: got %v, want 'items[0]: product_id is required'", resp["error"])
	}
}

func TestOrderCreate_ZeroQuantity(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{}
	router := setupOrderRouterWithAuth(svc, claims)

	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 0},
		},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	resp := decodeOrderResponse(t, rr)
	if resp["error"] != "items[0]: quantity must be > 0" {
		t.Errorf("error: got %v, want 'items[0]: quantity must be > 0'", resp["error"])
	}
}

func TestOrderCreate_InvalidOutletID(t *testing.T) {
	claims := testClaims(uuid.New())
	svc := &mockOrderService{}
	router := setupOrderRouterWithAuth(svc, claims)

	rr := doAuthRequest(t, router, "POST", "/outlets/not-a-uuid/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1},
		},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderCreate_InvalidBody(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{}
	router := setupOrderRouterWithAuth(svc, claims)

	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", "not json", claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderCreate_NoAuth(t *testing.T) {
	svc := &mockOrderService{}
	h := handler.NewOrderHandler(svc)
	r := chi.NewRouter()
	r.Use(middleware.Authenticate(testJWTSecret))
	r.Route("/outlets/{oid}/orders", h.RegisterRoutes)

	outletID := uuid.New()
	req := httptest.NewRequest("POST", "/outlets/"+outletID.String()+"/orders", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestOrderCreate_ServiceValidationError_InvalidOrderType(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			return nil, service.ErrInvalidOrderType
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "INVALID_TYPE",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1},
		},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	resp := decodeOrderResponse(t, rr)
	if resp["error"] != "invalid order_type" {
		t.Errorf("error: got %v, want 'invalid order_type'", resp["error"])
	}
}

func TestOrderCreate_ServiceValidationError_ProductNotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			return nil, service.ErrProductNotFound
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1},
		},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderCreate_ServiceValidationError_CateringNoDate(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			return nil, service.ErrCateringDate
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "CATERING",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1},
		},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderCreate_ServiceInternalError(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			return nil, context.DeadlineExceeded
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1},
		},
	}, claims)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
}

func TestOrderCreate_WithDiscount(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	orderID := uuid.New()
	now := time.Now()

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			if req.DiscountType != "PERCENTAGE" {
				t.Errorf("discount_type: got %v, want PERCENTAGE", req.DiscountType)
			}
			if req.DiscountValue != "10" {
				t.Errorf("discount_value: got %v, want 10", req.DiscountValue)
			}
			return &service.CreateOrderResult{
				Order: database.Order{
					ID: orderID, OutletID: outletID, OrderNumber: "KWR-001",
					OrderType: database.OrderTypeDINEIN, Status: database.OrderStatusNEW,
					Subtotal: testNumeric("50000.00"),
					DiscountType: database.NullDiscountType{DiscountType: database.DiscountTypePERCENTAGE, Valid: true},
					DiscountValue: testNumeric("10.00"),
					DiscountAmount: testNumeric("5000.00"),
					TaxAmount: testNumeric("0.00"),
					TotalAmount: testNumeric("45000.00"),
					CreatedBy: claims.UserID, CreatedAt: now, UpdatedAt: now,
				},
				Items: []service.OrderItemResult{
					{
						Item: database.OrderItem{
							ID: uuid.New(), OrderID: orderID, ProductID: uuid.New(),
							Quantity: 2, UnitPrice: testNumeric("25000.00"),
							DiscountAmount: testNumeric("0.00"), Subtotal: testNumeric("50000.00"),
							Status: database.OrderItemStatusPENDING,
						},
					},
				},
			}, nil
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type":     "DINE_IN",
		"discount_type":  "PERCENTAGE",
		"discount_value": "10",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 2},
		},
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["subtotal"] != "50000.00" {
		t.Errorf("subtotal: got %v, want 50000.00", resp["subtotal"])
	}
	if resp["discount_amount"] != "5000.00" {
		t.Errorf("discount_amount: got %v, want 5000.00", resp["discount_amount"])
	}
	if resp["total_amount"] != "45000.00" {
		t.Errorf("total_amount: got %v, want 45000.00", resp["total_amount"])
	}
	if resp["discount_type"] != "PERCENTAGE" {
		t.Errorf("discount_type: got %v, want PERCENTAGE", resp["discount_type"])
	}
}

func TestOrderCreate_CateringOrder(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	customerID := uuid.New()

	orderID := uuid.New()
	now := time.Now()
	cateringTime := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			if req.OrderType != "CATERING" {
				t.Errorf("order_type: got %v, want CATERING", req.OrderType)
			}
			if req.CustomerID != customerID.String() {
				t.Errorf("customer_id: got %v, want %s", req.CustomerID, customerID.String())
			}
			return &service.CreateOrderResult{
				Order: database.Order{
					ID: orderID, OutletID: outletID, OrderNumber: "KWR-001",
					OrderType: database.OrderTypeCATERING, Status: database.OrderStatusNEW,
					CustomerID: pgtype.UUID{Bytes: customerID, Valid: true},
					Subtotal: testNumeric("500000.00"), DiscountAmount: testNumeric("0.00"),
					TaxAmount: testNumeric("0.00"), TotalAmount: testNumeric("500000.00"),
					CateringDate: pgtype.Timestamptz{Time: cateringTime, Valid: true},
					CateringStatus: database.NullCateringStatus{CateringStatus: database.CateringStatusBOOKED, Valid: true},
					CateringDpAmount: testNumeric("200000.00"),
					CreatedBy: claims.UserID, CreatedAt: now, UpdatedAt: now,
				},
				Items: []service.OrderItemResult{
					{
						Item: database.OrderItem{
							ID: uuid.New(), OrderID: orderID, ProductID: uuid.New(),
							Quantity: 20, UnitPrice: testNumeric("25000.00"),
							DiscountAmount: testNumeric("0.00"), Subtotal: testNumeric("500000.00"),
							Status: database.OrderItemStatusPENDING,
						},
					},
				},
			}, nil
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type":        "CATERING",
		"customer_id":       customerID.String(),
		"catering_date":     "2026-03-01T10:00:00Z",
		"catering_dp_amount": "200000",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 20},
		},
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["order_type"] != "CATERING" {
		t.Errorf("order_type: got %v, want CATERING", resp["order_type"])
	}
	if resp["catering_status"] != "BOOKED" {
		t.Errorf("catering_status: got %v, want BOOKED", resp["catering_status"])
	}
	if resp["customer_id"] != customerID.String() {
		t.Errorf("customer_id: got %v, want %s", resp["customer_id"], customerID.String())
	}
}

func TestOrderCreate_ResponseContainsNullFields(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	orderID := uuid.New()
	now := time.Now()

	// Create a minimal order response with no optional fields
	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			return &service.CreateOrderResult{
				Order: database.Order{
					ID: orderID, OutletID: outletID, OrderNumber: "KWR-001",
					OrderType: database.OrderTypeTAKEAWAY, Status: database.OrderStatusNEW,
					Subtotal: testNumeric("15000.00"), DiscountAmount: testNumeric("0.00"),
					TaxAmount: testNumeric("0.00"), TotalAmount: testNumeric("15000.00"),
					CreatedBy: claims.UserID, CreatedAt: now, UpdatedAt: now,
				},
				Items: []service.OrderItemResult{
					{
						Item: database.OrderItem{
							ID: uuid.New(), OrderID: orderID, ProductID: uuid.New(),
							Quantity: 1, UnitPrice: testNumeric("15000.00"),
							DiscountAmount: testNumeric("0.00"), Subtotal: testNumeric("15000.00"),
							Status: database.OrderItemStatusPENDING,
						},
					},
				},
			}, nil
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "TAKEAWAY",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1},
		},
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	// Nullable fields should be nil
	if resp["customer_id"] != nil {
		t.Errorf("customer_id: expected nil, got %v", resp["customer_id"])
	}
	if resp["table_number"] != nil {
		t.Errorf("table_number: expected nil, got %v", resp["table_number"])
	}
	if resp["notes"] != nil {
		t.Errorf("notes: expected nil, got %v", resp["notes"])
	}
	if resp["discount_type"] != nil {
		t.Errorf("discount_type: expected nil, got %v", resp["discount_type"])
	}
	if resp["catering_date"] != nil {
		t.Errorf("catering_date: expected nil, got %v", resp["catering_date"])
	}

	// Items should have empty modifiers array (not null)
	items := resp["items"].([]interface{})
	item := items[0].(map[string]interface{})
	mods := item["modifiers"].([]interface{})
	if mods == nil {
		t.Error("modifiers should be empty array, not null")
	}
	if len(mods) != 0 {
		t.Errorf("modifiers count: got %d, want 0", len(mods))
	}
}

func TestOrderCreate_PassesAllFieldsToService(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	customerID := uuid.New()

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			if req.TableNumber != "A5" {
				t.Errorf("table_number: got %v, want A5", req.TableNumber)
			}
			if req.Notes != "extra spicy" {
				t.Errorf("notes: got %v, want 'extra spicy'", req.Notes)
			}
			if req.CustomerID != customerID.String() {
				t.Errorf("customer_id: got %v, want %s", req.CustomerID, customerID.String())
			}
			if req.DeliveryPlatform != "GrabFood" {
				t.Errorf("delivery_platform: got %v, want GrabFood", req.DeliveryPlatform)
			}
			if req.DeliveryAddress != "Jl. Testing 123" {
				t.Errorf("delivery_address: got %v, want 'Jl. Testing 123'", req.DeliveryAddress)
			}
			if len(req.Items) != 1 {
				t.Fatalf("items count: got %d, want 1", len(req.Items))
			}
			if req.Items[0].Notes != "no onion" {
				t.Errorf("item notes: got %v, want 'no onion'", req.Items[0].Notes)
			}
			if req.Items[0].DiscountType != "FIXED_AMOUNT" {
				t.Errorf("item discount_type: got %v, want FIXED_AMOUNT", req.Items[0].DiscountType)
			}

			return testOrderResult(outletID, claims.UserID), nil
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type":        "DELIVERY",
		"table_number":      "A5",
		"customer_id":       customerID.String(),
		"notes":             "extra spicy",
		"delivery_platform": "GrabFood",
		"delivery_address":  "Jl. Testing 123",
		"items": []map[string]interface{}{
			{
				"product_id":     uuid.New().String(),
				"variant_id":     uuid.New().String(),
				"quantity":       1,
				"notes":          "no onion",
				"discount_type":  "FIXED_AMOUNT",
				"discount_value": "5000",
			},
		},
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestOrderCreate_WrappedServiceError(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	svc := &mockOrderService{
		createFn: func(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error) {
			return nil, newWrappedError(0, service.ErrVariantMismatch)
		},
	}

	router := setupOrderRouterWithAuth(svc, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders", map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1, "variant_id": uuid.New().String()},
		},
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// Helper to simulate fmt.Errorf("item[%d]: %w", idx, err) pattern from service
type wrappedItemError struct {
	idx int
	err error
}

func (e *wrappedItemError) Error() string {
	return "item[" + strconv.Itoa(e.idx) + "]: " + e.err.Error()
}

func (e *wrappedItemError) Unwrap() error {
	return e.err
}

func newWrappedError(idx int, err error) error {
	return &wrappedItemError{idx: idx, err: err}
}

