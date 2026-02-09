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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// --- Mock OrderStore ---

type mockOrderStore struct {
	getOrderFn                  func(ctx context.Context, arg database.GetOrderParams) (database.Order, error)
	listOrdersFn                func(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error)
	listActiveOrdersFn          func(ctx context.Context, arg database.ListActiveOrdersParams) ([]database.ListActiveOrdersRow, error)
	listOrderItemsByOrderFn     func(ctx context.Context, orderID uuid.UUID) ([]database.OrderItem, error)
	listOrderItemModifiersFn    func(ctx context.Context, orderItemID uuid.UUID) ([]database.OrderItemModifier, error)
	listPaymentsByOrderFn       func(ctx context.Context, orderID uuid.UUID) ([]database.Payment, error)
	updateOrderStatusFn         func(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error)
	cancelOrderFn               func(ctx context.Context, arg database.CancelOrderParams) (database.Order, error)
	getOrderItemFn              func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error)
	updateOrderItemFn           func(ctx context.Context, arg database.UpdateOrderItemParams) (database.OrderItem, error)
	deleteOrderItemFn           func(ctx context.Context, arg database.DeleteOrderItemParams) error
	updateOrderItemStatusFn     func(ctx context.Context, arg database.UpdateOrderItemStatusParams) (database.OrderItem, error)
	countOrderItemsFn           func(ctx context.Context, orderID uuid.UUID) (int64, error)
	updateOrderTotalsFn         func(ctx context.Context, orderID uuid.UUID) (database.Order, error)
	getProductForOrderFn        func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error)
	getVariantForOrderFn        func(ctx context.Context, variantID uuid.UUID) (database.GetVariantForOrderRow, error)
	getModifierForOrderFn       func(ctx context.Context, modifierID uuid.UUID) (database.GetModifierForOrderRow, error)
	createOrderItemFn           func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error)
	createOrderItemModifierFn   func(ctx context.Context, arg database.CreateOrderItemModifierParams) (database.OrderItemModifier, error)
}

func (m *mockOrderStore) GetOrder(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
	if m.getOrderFn != nil {
		return m.getOrderFn(ctx, arg)
	}
	return database.Order{}, pgx.ErrNoRows
}

func (m *mockOrderStore) ListOrders(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error) {
	if m.listOrdersFn != nil {
		return m.listOrdersFn(ctx, arg)
	}
	return []database.Order{}, nil
}

func (m *mockOrderStore) ListActiveOrders(ctx context.Context, arg database.ListActiveOrdersParams) ([]database.ListActiveOrdersRow, error) {
	if m.listActiveOrdersFn != nil {
		return m.listActiveOrdersFn(ctx, arg)
	}
	return []database.ListActiveOrdersRow{}, nil
}

func (m *mockOrderStore) ListOrderItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]database.OrderItem, error) {
	if m.listOrderItemsByOrderFn != nil {
		return m.listOrderItemsByOrderFn(ctx, orderID)
	}
	return []database.OrderItem{}, nil
}

func (m *mockOrderStore) ListOrderItemModifiersByOrderItem(ctx context.Context, orderItemID uuid.UUID) ([]database.OrderItemModifier, error) {
	if m.listOrderItemModifiersFn != nil {
		return m.listOrderItemModifiersFn(ctx, orderItemID)
	}
	return []database.OrderItemModifier{}, nil
}

func (m *mockOrderStore) ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]database.Payment, error) {
	if m.listPaymentsByOrderFn != nil {
		return m.listPaymentsByOrderFn(ctx, orderID)
	}
	return []database.Payment{}, nil
}

func (m *mockOrderStore) UpdateOrderStatus(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error) {
	if m.updateOrderStatusFn != nil {
		return m.updateOrderStatusFn(ctx, arg)
	}
	return database.Order{}, pgx.ErrNoRows
}

func (m *mockOrderStore) CancelOrder(ctx context.Context, arg database.CancelOrderParams) (database.Order, error) {
	if m.cancelOrderFn != nil {
		return m.cancelOrderFn(ctx, arg)
	}
	return database.Order{}, pgx.ErrNoRows
}

func (m *mockOrderStore) GetOrderItem(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
	if m.getOrderItemFn != nil {
		return m.getOrderItemFn(ctx, arg)
	}
	return database.OrderItem{}, pgx.ErrNoRows
}

func (m *mockOrderStore) UpdateOrderItem(ctx context.Context, arg database.UpdateOrderItemParams) (database.OrderItem, error) {
	if m.updateOrderItemFn != nil {
		return m.updateOrderItemFn(ctx, arg)
	}
	return database.OrderItem{}, pgx.ErrNoRows
}

func (m *mockOrderStore) DeleteOrderItem(ctx context.Context, arg database.DeleteOrderItemParams) error {
	if m.deleteOrderItemFn != nil {
		return m.deleteOrderItemFn(ctx, arg)
	}
	return pgx.ErrNoRows
}

func (m *mockOrderStore) UpdateOrderItemStatus(ctx context.Context, arg database.UpdateOrderItemStatusParams) (database.OrderItem, error) {
	if m.updateOrderItemStatusFn != nil {
		return m.updateOrderItemStatusFn(ctx, arg)
	}
	return database.OrderItem{}, pgx.ErrNoRows
}

func (m *mockOrderStore) CountOrderItems(ctx context.Context, orderID uuid.UUID) (int64, error) {
	if m.countOrderItemsFn != nil {
		return m.countOrderItemsFn(ctx, orderID)
	}
	return 0, nil
}

func (m *mockOrderStore) UpdateOrderTotals(ctx context.Context, orderID uuid.UUID) (database.Order, error) {
	if m.updateOrderTotalsFn != nil {
		return m.updateOrderTotalsFn(ctx, orderID)
	}
	return database.Order{}, pgx.ErrNoRows
}

func (m *mockOrderStore) GetProductForOrder(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
	if m.getProductForOrderFn != nil {
		return m.getProductForOrderFn(ctx, arg)
	}
	return database.GetProductForOrderRow{}, pgx.ErrNoRows
}

func (m *mockOrderStore) GetVariantForOrder(ctx context.Context, variantID uuid.UUID) (database.GetVariantForOrderRow, error) {
	if m.getVariantForOrderFn != nil {
		return m.getVariantForOrderFn(ctx, variantID)
	}
	return database.GetVariantForOrderRow{}, pgx.ErrNoRows
}

func (m *mockOrderStore) GetModifierForOrder(ctx context.Context, modifierID uuid.UUID) (database.GetModifierForOrderRow, error) {
	if m.getModifierForOrderFn != nil {
		return m.getModifierForOrderFn(ctx, modifierID)
	}
	return database.GetModifierForOrderRow{}, pgx.ErrNoRows
}

func (m *mockOrderStore) CreateOrderItem(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
	if m.createOrderItemFn != nil {
		return m.createOrderItemFn(ctx, arg)
	}
	return database.OrderItem{}, pgx.ErrNoRows
}

func (m *mockOrderStore) CreateOrderItemModifier(ctx context.Context, arg database.CreateOrderItemModifierParams) (database.OrderItemModifier, error) {
	if m.createOrderItemModifierFn != nil {
		return m.createOrderItemModifierFn(ctx, arg)
	}
	return database.OrderItemModifier{}, pgx.ErrNoRows
}

// --- Mock TxBeginner ---

type mockTx struct {
	commitFn   func(ctx context.Context) error
	rollbackFn func(ctx context.Context) error
}

func (m *mockTx) Commit(ctx context.Context) error {
	if m.commitFn != nil {
		return m.commitFn(ctx)
	}
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	if m.rollbackFn != nil {
		return m.rollbackFn(ctx)
	}
	return nil
}

func (m *mockTx) Begin(ctx context.Context) (pgx.Tx, error) {
	return nil, nil
}

func (m *mockTx) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return nil
}

func (m *mockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (m *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}

func (m *mockTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (m *mockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (m *mockTx) Conn() *pgx.Conn {
	return nil
}

type mockPool struct {
	beginFn func(ctx context.Context) (pgx.Tx, error)
}

func (m *mockPool) Begin(ctx context.Context) (pgx.Tx, error) {
	if m.beginFn != nil {
		return m.beginFn(ctx)
	}
	// Return a mock transaction that commits successfully
	return &mockTx{}, nil
}

// mockNewStore is a mock factory that returns the same store for transactions.
func mockNewStore(store *mockOrderStore) func(db database.DBTX) handler.OrderStore {
	return func(db database.DBTX) handler.OrderStore {
		return store
	}
}

// --- Test helpers ---

const testJWTSecret = "test-secret-for-orders"

func setupOrderRouterWithAuth(svc *mockOrderService, claims *auth.Claims) *chi.Mux {
	return setupOrderRouterWithStore(svc, nil, claims)
}

func setupOrderRouterWithStore(svc *mockOrderService, store *mockOrderStore, claims *auth.Claims) *chi.Mux {
	pool := &mockPool{}
	newStore := mockNewStore(store)
	h := handler.NewOrderHandler(svc, store, pool, newStore)
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
	store := &mockOrderStore{}
	pool := &mockPool{}
	newStore := mockNewStore(store)
	h := handler.NewOrderHandler(svc, store, pool, newStore)
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

// --- Test data helpers for new endpoints ---

func testDBOrder(outletID uuid.UUID) database.Order {
	return database.Order{
		ID:             uuid.New(),
		OutletID:       outletID,
		OrderNumber:    "KWR-001",
		OrderType:      database.OrderTypeDINEIN,
		Status:         database.OrderStatusNEW,
		Subtotal:       testNumeric("25000.00"),
		DiscountAmount: testNumeric("0.00"),
		TaxAmount:      testNumeric("0.00"),
		TotalAmount:    testNumeric("25000.00"),
		CreatedBy:      uuid.New(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func testDBOrderWithStatus(outletID uuid.UUID, status database.OrderStatus) database.Order {
	o := testDBOrder(outletID)
	o.Status = status
	return o
}

func testDBOrderItem(orderID uuid.UUID) database.OrderItem {
	return database.OrderItem{
		ID:             uuid.New(),
		OrderID:        orderID,
		ProductID:      uuid.New(),
		Quantity:       2,
		UnitPrice:      testNumeric("12500.00"),
		DiscountAmount: testNumeric("0.00"),
		Subtotal:       testNumeric("25000.00"),
		Status:         database.OrderItemStatusPENDING,
	}
}

// --- List endpoint tests ---

func TestOrderList_HappyPath(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order1 := testDBOrder(outletID)
	order2 := testDBOrder(outletID)
	order2.OrderNumber = "KWR-002"

	store := &mockOrderStore{
		listOrdersFn: func(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error) {
			if arg.OutletID != outletID {
				t.Errorf("outlet_id: got %v, want %v", arg.OutletID, outletID)
			}
			if arg.Limit != 20 {
				t.Errorf("limit: got %d, want 20", arg.Limit)
			}
			if arg.Offset != 0 {
				t.Errorf("offset: got %d, want 0", arg.Offset)
			}
			return []database.Order{order1, order2}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	orders, ok := resp["orders"].([]interface{})
	if !ok {
		t.Fatal("orders not present in response")
	}
	if len(orders) != 2 {
		t.Fatalf("orders count: got %d, want 2", len(orders))
	}

	if resp["limit"] != float64(20) {
		t.Errorf("limit: got %v, want 20", resp["limit"])
	}
	if resp["offset"] != float64(0) {
		t.Errorf("offset: got %v, want 0", resp["offset"])
	}
}

func TestOrderList_WithPagination(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		listOrdersFn: func(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error) {
			if arg.Limit != 10 {
				t.Errorf("limit: got %d, want 10", arg.Limit)
			}
			if arg.Offset != 5 {
				t.Errorf("offset: got %d, want 5", arg.Offset)
			}
			return []database.Order{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders?limit=10&offset=5", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["limit"] != float64(10) {
		t.Errorf("limit: got %v, want 10", resp["limit"])
	}
	if resp["offset"] != float64(5) {
		t.Errorf("offset: got %v, want 5", resp["offset"])
	}
}

func TestOrderList_LimitCappedAt100(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		listOrdersFn: func(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error) {
			if arg.Limit != 100 {
				t.Errorf("limit: got %d, want 100 (should be capped)", arg.Limit)
			}
			return []database.Order{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders?limit=999", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestOrderList_WithStatusFilter(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		listOrdersFn: func(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error) {
			if !arg.Status.Valid {
				t.Error("status filter should be set")
			}
			if arg.Status.OrderStatus != database.OrderStatusNEW {
				t.Errorf("status: got %v, want NEW", arg.Status.OrderStatus)
			}
			return []database.Order{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders?status=NEW", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestOrderList_WithTypeFilter(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		listOrdersFn: func(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error) {
			if !arg.OrderType.Valid {
				t.Error("order_type filter should be set")
			}
			if arg.OrderType.OrderType != database.OrderTypeDINEIN {
				t.Errorf("order_type: got %v, want DINE_IN", arg.OrderType.OrderType)
			}
			return []database.Order{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders?type=DINE_IN", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestOrderList_WithDateFilter(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		listOrdersFn: func(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error) {
			if !arg.StartDate.Valid {
				t.Error("start_date filter should be set")
			}
			if !arg.EndDate.Valid {
				t.Error("end_date filter should be set")
			}
			expected := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			if !arg.StartDate.Time.Equal(expected) {
				t.Errorf("start_date: got %v, want %v", arg.StartDate.Time, expected)
			}
			return []database.Order{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders?start_date=2026-01-01&end_date=2026-01-31", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestOrderList_InvalidDateFormat(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{}
	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders?start_date=not-a-date", nil, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderList_Empty(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		listOrdersFn: func(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error) {
			return []database.Order{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	orders := resp["orders"].([]interface{})
	if len(orders) != 0 {
		t.Errorf("orders count: got %d, want 0", len(orders))
	}
}

func TestOrderList_InvalidOutletID(t *testing.T) {
	claims := testClaims(uuid.New())
	store := &mockOrderStore{}
	router := setupOrderRouterWithStore(nil, store, claims)

	rr := doAuthRequest(t, router, "GET", "/outlets/not-a-uuid/orders", nil, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderList_NoAuth(t *testing.T) {
	store := &mockOrderStore{}
	pool := &mockPool{}
	newStore := mockNewStore(store)
	h := handler.NewOrderHandler(nil, store, pool, newStore)
	r := chi.NewRouter()
	r.Use(middleware.Authenticate(testJWTSecret))
	r.Route("/outlets/{oid}/orders", h.RegisterRoutes)

	outletID := uuid.New()
	req := httptest.NewRequest("GET", "/outlets/"+outletID.String()+"/orders", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

// --- Get detail endpoint tests ---

func TestOrderGet_HappyPath(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrder(outletID)
	item := testDBOrderItem(order.ID)
	modID := uuid.New()
	modifier := database.OrderItemModifier{
		ID:          uuid.New(),
		OrderItemID: item.ID,
		ModifierID:  modID,
		Quantity:    1,
		UnitPrice:   testNumeric("5000.00"),
	}

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			if arg.ID != order.ID || arg.OutletID != outletID {
				t.Errorf("get order params mismatch")
			}
			return order, nil
		},
		listOrderItemsByOrderFn: func(ctx context.Context, orderID uuid.UUID) ([]database.OrderItem, error) {
			return []database.OrderItem{item}, nil
		},
		listOrderItemModifiersFn: func(ctx context.Context, orderItemID uuid.UUID) ([]database.OrderItemModifier, error) {
			if orderItemID == item.ID {
				return []database.OrderItemModifier{modifier}, nil
			}
			return []database.OrderItemModifier{}, nil
		},
		listPaymentsByOrderFn: func(ctx context.Context, orderID uuid.UUID) ([]database.Payment, error) {
			return []database.Payment{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders/"+order.ID.String(), nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["order_number"] != "KWR-001" {
		t.Errorf("order_number: got %v, want KWR-001", resp["order_number"])
	}
	if resp["status"] != "NEW" {
		t.Errorf("status: got %v, want NEW", resp["status"])
	}

	items, ok := resp["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("items: expected 1, got %v", items)
	}
	itemResp := items[0].(map[string]interface{})
	mods := itemResp["modifiers"].([]interface{})
	if len(mods) != 1 {
		t.Fatalf("modifiers: expected 1, got %d", len(mods))
	}
	mod := mods[0].(map[string]interface{})
	if mod["unit_price"] != "5000.00" {
		t.Errorf("modifier unit_price: got %v, want 5000.00", mod["unit_price"])
	}

	payments, ok := resp["payments"].([]interface{})
	if !ok {
		t.Fatal("payments not present in response")
	}
	if len(payments) != 0 {
		t.Errorf("payments: expected 0, got %d", len(payments))
	}
}

func TestOrderGet_WithPayments(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	order := testDBOrder(outletID)

	payment := database.Payment{
		ID:            uuid.New(),
		OrderID:       order.ID,
		PaymentMethod: database.PaymentMethodCASH,
		Amount:        testNumeric("25000.00"),
		Status:        database.PaymentStatusCOMPLETED,
		AmountReceived: testNumeric("30000.00"),
		ChangeAmount:   testNumeric("5000.00"),
		ProcessedBy:   claims.UserID,
		ProcessedAt:   time.Now(),
	}

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		listOrderItemsByOrderFn: func(ctx context.Context, orderID uuid.UUID) ([]database.OrderItem, error) {
			return []database.OrderItem{}, nil
		},
		listPaymentsByOrderFn: func(ctx context.Context, orderID uuid.UUID) ([]database.Payment, error) {
			return []database.Payment{payment}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders/"+order.ID.String(), nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	payments := resp["payments"].([]interface{})
	if len(payments) != 1 {
		t.Fatalf("payments: expected 1, got %d", len(payments))
	}
	p := payments[0].(map[string]interface{})
	if p["payment_method"] != "CASH" {
		t.Errorf("payment_method: got %v, want CASH", p["payment_method"])
	}
	if p["amount"] != "25000.00" {
		t.Errorf("amount: got %v, want 25000.00", p["amount"])
	}
	if p["amount_received"] != "30000.00" {
		t.Errorf("amount_received: got %v, want 30000.00", p["amount_received"])
	}
	if p["change_amount"] != "5000.00" {
		t.Errorf("change_amount: got %v, want 5000.00", p["change_amount"])
	}
}

func TestOrderGet_NotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return database.Order{}, pgx.ErrNoRows
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders/"+uuid.New().String(), nil, claims)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestOrderGet_InvalidOrderID(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{}
	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders/not-a-uuid", nil, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// --- Update status endpoint tests ---

func TestOrderUpdateStatus_NewToPreparing(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	updatedOrder := order
	updatedOrder.Status = database.OrderStatusPREPARING

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		updateOrderStatusFn: func(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error) {
			if arg.Status != database.OrderStatusPREPARING {
				t.Errorf("status: got %v, want PREPARING", arg.Status)
			}
			if arg.Status_2 != database.OrderStatusNEW {
				t.Errorf("current status: got %v, want NEW", arg.Status_2)
			}
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "PREPARING"}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["status"] != "PREPARING" {
		t.Errorf("status: got %v, want PREPARING", resp["status"])
	}
}

func TestOrderUpdateStatus_PreparingToReady(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusPREPARING)
	updatedOrder := order
	updatedOrder.Status = database.OrderStatusREADY

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		updateOrderStatusFn: func(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "READY"}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["status"] != "READY" {
		t.Errorf("status: got %v, want READY", resp["status"])
	}
}

func TestOrderUpdateStatus_ReadyToCompleted(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusREADY)
	updatedOrder := order
	updatedOrder.Status = database.OrderStatusCOMPLETED

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		updateOrderStatusFn: func(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "COMPLETED"}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["status"] != "COMPLETED" {
		t.Errorf("status: got %v, want COMPLETED", resp["status"])
	}
}

func TestOrderUpdateStatus_NewToCancelled(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	updatedOrder := order
	updatedOrder.Status = database.OrderStatusCANCELLED

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		updateOrderStatusFn: func(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "CANCELLED"}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestOrderUpdateStatus_PreparingToCancelled(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusPREPARING)
	updatedOrder := order
	updatedOrder.Status = database.OrderStatusCANCELLED

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		updateOrderStatusFn: func(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "CANCELLED"}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestOrderUpdateStatus_ReadyToCancelled(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusREADY)
	updatedOrder := order
	updatedOrder.Status = database.OrderStatusCANCELLED

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		updateOrderStatusFn: func(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error) {
			if arg.Status != database.OrderStatusCANCELLED {
				t.Errorf("status: got %v, want CANCELLED", arg.Status)
			}
			if arg.Status_2 != database.OrderStatusREADY {
				t.Errorf("current status: got %v, want READY", arg.Status_2)
			}
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "CANCELLED"}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["status"] != "CANCELLED" {
		t.Errorf("status: got %v, want CANCELLED", resp["status"])
	}
}

func TestOrderUpdateStatus_InvalidTransition_NewToCompleted(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "COMPLETED"}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestOrderUpdateStatus_InvalidTransition_CompletedToNew(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusCOMPLETED)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "NEW"}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestOrderUpdateStatus_InvalidTransition_ReadyToPreparing(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusREADY)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "PREPARING"}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["error"] == nil {
		t.Error("expected error message")
	}
}

func TestOrderUpdateStatus_InvalidTransition_CancelledToAny(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusCANCELLED)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+order.ID.String()+"/status",
		map[string]string{"status": "NEW"}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestOrderUpdateStatus_MissingStatus(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{}
	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+uuid.New().String()+"/status",
		map[string]string{}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderUpdateStatus_InvalidStatus(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{}
	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+uuid.New().String()+"/status",
		map[string]string{"status": "INVALID"}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderUpdateStatus_NotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return database.Order{}, pgx.ErrNoRows
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+uuid.New().String()+"/status",
		map[string]string{"status": "PREPARING"}, claims)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestOrderUpdateStatus_InvalidOrderID(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{}
	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/not-a-uuid/status",
		map[string]string{"status": "PREPARING"}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// --- Cancel endpoint tests ---

func TestOrderCancel_HappyPath_NewOrder(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	cancelledOrder := order
	cancelledOrder.Status = database.OrderStatusCANCELLED

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		cancelOrderFn: func(ctx context.Context, arg database.CancelOrderParams) (database.Order, error) {
			if arg.ID != order.ID || arg.OutletID != outletID {
				t.Errorf("cancel params mismatch")
			}
			return cancelledOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+order.ID.String(), nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["status"] != "CANCELLED" {
		t.Errorf("status: got %v, want CANCELLED", resp["status"])
	}
}

func TestOrderCancel_HappyPath_PreparingOrder(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusPREPARING)
	cancelledOrder := order
	cancelledOrder.Status = database.OrderStatusCANCELLED

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		cancelOrderFn: func(ctx context.Context, arg database.CancelOrderParams) (database.Order, error) {
			return cancelledOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+order.ID.String(), nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestOrderCancel_CompletedOrder(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusCOMPLETED)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+order.ID.String(), nil, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["error"] != "cannot cancel a completed order" {
		t.Errorf("error: got %v, want 'cannot cancel a completed order'", resp["error"])
	}
}

func TestOrderCancel_AlreadyCancelled(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusCANCELLED)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+order.ID.String(), nil, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["error"] != "order is already cancelled" {
		t.Errorf("error: got %v, want 'order is already cancelled'", resp["error"])
	}
}

func TestOrderCancel_NotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return database.Order{}, pgx.ErrNoRows
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+uuid.New().String(), nil, claims)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestOrderCancel_InvalidOrderID(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{}
	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/not-a-uuid", nil, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestOrderCancel_ReadyOrder(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	order := testDBOrderWithStatus(outletID, database.OrderStatusREADY)
	cancelledOrder := order
	cancelledOrder.Status = database.OrderStatusCANCELLED

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		cancelOrderFn: func(ctx context.Context, arg database.CancelOrderParams) (database.Order, error) {
			return cancelledOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+order.ID.String(), nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

// --- Item modification endpoint tests ---

func TestAddItem_HappyPath(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	productID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	product := database.GetProductForOrderRow{
		ID:        productID,
		OutletID:  outletID,
		BasePrice: testNumeric("25000.00"),
		Station:   database.NullKitchenStation{KitchenStation: database.KitchenStationGRILL, Valid: true},
	}

	createdItem := database.OrderItem{
		ID:             itemID,
		OrderID:        orderID,
		ProductID:      productID,
		Quantity:       2,
		UnitPrice:      testNumeric("25000.00"),
		DiscountAmount: testNumeric("0.00"),
		Subtotal:       testNumeric("50000.00"),
		Status:         database.OrderItemStatusPENDING,
	}

	updatedOrder := order
	updatedOrder.Subtotal = testNumeric("50000.00")
	updatedOrder.TotalAmount = testNumeric("50000.00")

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getProductForOrderFn: func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
			if arg.ID != productID || arg.OutletID != outletID {
				t.Errorf("product params mismatch")
			}
			return product, nil
		},
		createOrderItemFn: func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
			if arg.Quantity != 2 {
				t.Errorf("quantity: got %d, want 2", arg.Quantity)
			}
			return createdItem, nil
		},
		updateOrderTotalsFn: func(ctx context.Context, orderID uuid.UUID) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id": productID.String(),
		"quantity":   2,
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	item := resp["item"].(map[string]interface{})
	if item["quantity"] != float64(2) {
		t.Errorf("item quantity: got %v, want 2", item["quantity"])
	}
}

func TestAddItem_OrderNotNew(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusPREPARING)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id": uuid.New().String(),
		"quantity":   1,
	}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestUpdateItem_HappyPath(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	currentItem := database.OrderItem{
		ID:             itemID,
		OrderID:        orderID,
		ProductID:      uuid.New(),
		Quantity:       2,
		UnitPrice:      testNumeric("25000.00"),
		DiscountAmount: testNumeric("0.00"),
		Subtotal:       testNumeric("50000.00"),
		Status:         database.OrderItemStatusPENDING,
	}

	updatedItem := currentItem
	updatedItem.Quantity = 3
	updatedItem.Subtotal = testNumeric("75000.00")
	updatedItem.Notes = pgtype.Text{String: "extra spicy", Valid: true}

	updatedOrder := order
	updatedOrder.Subtotal = testNumeric("75000.00")
	updatedOrder.TotalAmount = testNumeric("75000.00")

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getOrderItemFn: func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
			if arg.ID == itemID && arg.OrderID == orderID {
				return currentItem, nil
			}
			return database.OrderItem{}, pgx.ErrNoRows
		},
		listOrderItemModifiersFn: func(ctx context.Context, orderItemID uuid.UUID) ([]database.OrderItemModifier, error) {
			return []database.OrderItemModifier{}, nil
		},
		updateOrderItemFn: func(ctx context.Context, arg database.UpdateOrderItemParams) (database.OrderItem, error) {
			if arg.Quantity != 3 {
				t.Errorf("quantity: got %d, want 3", arg.Quantity)
			}
			return updatedItem, nil
		},
		updateOrderTotalsFn: func(ctx context.Context, orderID uuid.UUID) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String(), map[string]interface{}{
		"quantity": 3,
		"notes":    "extra spicy",
	}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	item := resp["item"].(map[string]interface{})
	if item["quantity"] != float64(3) {
		t.Errorf("item quantity: got %v, want 3", item["quantity"])
	}
}

func TestRemoveItem_HappyPath(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	item := database.OrderItem{
		ID:      itemID,
		OrderID: orderID,
	}

	updatedOrder := order

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		countOrderItemsFn: func(ctx context.Context, orderID uuid.UUID) (int64, error) {
			return 2, nil
		},
		getOrderItemFn: func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
			return item, nil
		},
		deleteOrderItemFn: func(ctx context.Context, arg database.DeleteOrderItemParams) error {
			return nil
		},
		updateOrderTotalsFn: func(ctx context.Context, orderID uuid.UUID) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String(), nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestRemoveItem_LastItem(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		countOrderItemsFn: func(ctx context.Context, orderID uuid.UUID) (int64, error) {
			return 1, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String(), nil, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestUpdateItemStatus_PendingToPreparing(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrder(outletID)
	order.ID = orderID

	currentItem := database.OrderItem{
		ID:      itemID,
		OrderID: orderID,
		Status:  database.OrderItemStatusPENDING,
	}

	updatedItem := currentItem
	updatedItem.Status = database.OrderItemStatusPREPARING

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getOrderItemFn: func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
			return currentItem, nil
		},
		updateOrderItemStatusFn: func(ctx context.Context, arg database.UpdateOrderItemStatusParams) (database.OrderItem, error) {
			if arg.Status != database.OrderItemStatusPREPARING {
				t.Errorf("status: got %v, want PREPARING", arg.Status)
			}
			return updatedItem, nil
		},
		listOrderItemModifiersFn: func(ctx context.Context, orderItemID uuid.UUID) ([]database.OrderItemModifier, error) {
			return []database.OrderItemModifier{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String()+"/status", map[string]string{
		"status": "PREPARING",
	}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["status"] != "PREPARING" {
		t.Errorf("status: got %v, want PREPARING", resp["status"])
	}
}

func TestUpdateItemStatus_InvalidTransition(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrder(outletID)
	order.ID = orderID

	currentItem := database.OrderItem{
		ID:      itemID,
		OrderID: orderID,
		Status:  database.OrderItemStatusREADY,
	}

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getOrderItemFn: func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
			return currentItem, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String()+"/status", map[string]string{
		"status": "PREPARING",
	}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

// --- Additional AddItem Tests ---

func TestAddItem_WithVariant(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	product := database.GetProductForOrderRow{
		ID:        productID,
		OutletID:  outletID,
		BasePrice: testNumeric("25000.00"),
		Station:   database.NullKitchenStation{KitchenStation: database.KitchenStationGRILL, Valid: true},
	}

	variant := database.GetVariantForOrderRow{
		ID:              variantID,
		ProductID:       productID,
		PriceAdjustment: testNumeric("5000.00"),
	}

	createdItem := database.OrderItem{
		ID:             itemID,
		OrderID:        orderID,
		ProductID:      productID,
		VariantID:      pgtype.UUID{Bytes: variantID, Valid: true},
		Quantity:       1,
		UnitPrice:      testNumeric("30000.00"), // base + variant adjustment
		DiscountAmount: testNumeric("0.00"),
		Subtotal:       testNumeric("30000.00"),
		Status:         database.OrderItemStatusPENDING,
	}

	updatedOrder := order
	updatedOrder.Subtotal = testNumeric("30000.00")
	updatedOrder.TotalAmount = testNumeric("30000.00")

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getProductForOrderFn: func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
			return product, nil
		},
		getVariantForOrderFn: func(ctx context.Context, vid uuid.UUID) (database.GetVariantForOrderRow, error) {
			if vid != variantID {
				t.Errorf("variant ID mismatch")
			}
			return variant, nil
		},
		createOrderItemFn: func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
			return createdItem, nil
		},
		updateOrderTotalsFn: func(ctx context.Context, orderID uuid.UUID) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id": productID.String(),
		"variant_id": variantID.String(),
		"quantity":   1,
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestAddItem_WithModifiers(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	productID := uuid.New()
	modifierID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	product := database.GetProductForOrderRow{
		ID:        productID,
		OutletID:  outletID,
		BasePrice: testNumeric("25000.00"),
		Station:   database.NullKitchenStation{KitchenStation: database.KitchenStationGRILL, Valid: true},
	}

	modifier := database.GetModifierForOrderRow{
		ID:        modifierID,
		ProductID: productID,
		Price:     testNumeric("5000.00"),
	}

	createdItem := database.OrderItem{
		ID:             itemID,
		OrderID:        orderID,
		ProductID:      productID,
		Quantity:       1,
		UnitPrice:      testNumeric("25000.00"),
		DiscountAmount: testNumeric("0.00"),
		Subtotal:       testNumeric("30000.00"), // base + modifier
		Status:         database.OrderItemStatusPENDING,
	}

	createdModifier := database.OrderItemModifier{
		ID:          uuid.New(),
		OrderItemID: itemID,
		ModifierID:  modifierID,
		Quantity:    1,
		UnitPrice:   testNumeric("5000.00"),
	}

	updatedOrder := order
	updatedOrder.Subtotal = testNumeric("30000.00")
	updatedOrder.TotalAmount = testNumeric("30000.00")

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getProductForOrderFn: func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
			return product, nil
		},
		getModifierForOrderFn: func(ctx context.Context, mid uuid.UUID) (database.GetModifierForOrderRow, error) {
			if mid != modifierID {
				t.Errorf("modifier ID mismatch")
			}
			return modifier, nil
		},
		createOrderItemFn: func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
			return createdItem, nil
		},
		createOrderItemModifierFn: func(ctx context.Context, arg database.CreateOrderItemModifierParams) (database.OrderItemModifier, error) {
			return createdModifier, nil
		},
		updateOrderTotalsFn: func(ctx context.Context, orderID uuid.UUID) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id": productID.String(),
		"quantity":   1,
		"modifiers": []map[string]interface{}{
			{
				"modifier_id": modifierID.String(),
				"quantity":    1,
			},
		},
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestAddItem_WithItemDiscount(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	productID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	product := database.GetProductForOrderRow{
		ID:        productID,
		OutletID:  outletID,
		BasePrice: testNumeric("25000.00"),
		Station:   database.NullKitchenStation{KitchenStation: database.KitchenStationGRILL, Valid: true},
	}

	createdItem := database.OrderItem{
		ID:        itemID,
		OrderID:   orderID,
		ProductID: productID,
		Quantity:  2,
		UnitPrice: testNumeric("25000.00"),
		DiscountType: database.NullDiscountType{
			DiscountType: database.DiscountTypePERCENTAGE,
			Valid:        true,
		},
		DiscountValue:  testNumeric("10.00"),
		DiscountAmount: testNumeric("5000.00"), // 10% of 50000
		Subtotal:       testNumeric("45000.00"),
		Status:         database.OrderItemStatusPENDING,
	}

	updatedOrder := order
	updatedOrder.Subtotal = testNumeric("45000.00")
	updatedOrder.TotalAmount = testNumeric("45000.00")

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getProductForOrderFn: func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
			return product, nil
		},
		createOrderItemFn: func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
			return createdItem, nil
		},
		updateOrderTotalsFn: func(ctx context.Context, orderID uuid.UUID) (database.Order, error) {
			return updatedOrder, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id":     productID.String(),
		"quantity":       2,
		"discount_type":  "PERCENTAGE",
		"discount_value": "10.00",
	}, claims)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestAddItem_ProductNotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	productID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getProductForOrderFn: func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
			return database.GetProductForOrderRow{}, pgx.ErrNoRows
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id": productID.String(),
		"quantity":   1,
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestAddItem_InvalidProductID(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id": "invalid-uuid",
		"quantity":   1,
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestAddItem_MissingProductID(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"quantity": 1,
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestAddItem_ZeroQuantity(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id": uuid.New().String(),
		"quantity":   0,
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestAddItem_OrderNotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return database.Order{}, pgx.ErrNoRows
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "POST", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items", map[string]interface{}{
		"product_id": uuid.New().String(),
		"quantity":   1,
	}, claims)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// --- Additional UpdateItem Tests ---

func TestUpdateItem_OrderNotNew(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusPREPARING)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String(), map[string]interface{}{
		"quantity": 3,
	}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestUpdateItem_ItemNotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getOrderItemFn: func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
			return database.OrderItem{}, pgx.ErrNoRows
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String(), map[string]interface{}{
		"quantity": 3,
	}, claims)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestUpdateItem_ZeroQuantity(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String(), map[string]interface{}{
		"quantity": 0,
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// --- Additional RemoveItem Tests ---

func TestRemoveItem_OrderNotNew(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusPREPARING)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String(), nil, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestRemoveItem_ItemNotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusNEW)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		countOrderItemsFn: func(ctx context.Context, oid uuid.UUID) (int64, error) {
			return 2, nil
		},
		getOrderItemFn: func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
			return database.OrderItem{}, pgx.ErrNoRows
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String(), nil, claims)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// --- Additional UpdateItemStatus Tests ---

func TestUpdateItemStatus_PreparingToReady(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrder(outletID)
	order.ID = orderID

	currentItem := database.OrderItem{
		ID:      itemID,
		OrderID: orderID,
		Status:  database.OrderItemStatusPREPARING,
	}

	updatedItem := currentItem
	updatedItem.Status = database.OrderItemStatusREADY

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getOrderItemFn: func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
			return currentItem, nil
		},
		updateOrderItemStatusFn: func(ctx context.Context, arg database.UpdateOrderItemStatusParams) (database.OrderItem, error) {
			return updatedItem, nil
		},
		listOrderItemModifiersFn: func(ctx context.Context, orderItemID uuid.UUID) ([]database.OrderItemModifier, error) {
			return []database.OrderItemModifier{}, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String()+"/status", map[string]string{
		"status": "READY",
	}, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["status"] != "READY" {
		t.Errorf("status: got %v, want READY", resp["status"])
	}
}

func TestUpdateItemStatus_MissingStatus(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	router := setupOrderRouterWithStore(nil, &mockOrderStore{}, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String()+"/status", map[string]string{}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestUpdateItemStatus_InvalidStatusValue(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	router := setupOrderRouterWithStore(nil, &mockOrderStore{}, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String()+"/status", map[string]string{
		"status": "INVALID_STATUS",
	}, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestUpdateItemStatus_ItemNotFound(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrder(outletID)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
		getOrderItemFn: func(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error) {
			return database.OrderItem{}, pgx.ErrNoRows
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String()+"/status", map[string]string{
		"status": "PREPARING",
	}, claims)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestUpdateItemStatus_CancelledOrder(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusCANCELLED)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String()+"/status", map[string]string{
		"status": "PREPARING",
	}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["error"] == nil {
		t.Fatal("expected error field")
	}
	errMsg := resp["error"].(string)
	if errMsg != "cannot update items on a CANCELLED order" {
		t.Fatalf("error message: got %q, want %q", errMsg, "cannot update items on a CANCELLED order")
	}
}

func TestUpdateItemStatus_CompletedOrder(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	orderID := uuid.New()
	itemID := uuid.New()

	order := testDBOrderWithStatus(outletID, database.OrderStatusCOMPLETED)
	order.ID = orderID

	store := &mockOrderStore{
		getOrderFn: func(ctx context.Context, arg database.GetOrderParams) (database.Order, error) {
			return order, nil
		},
	}

	router := setupOrderRouterWithStore(nil, store, claims)
	rr := doAuthRequest(t, router, "PATCH", "/outlets/"+outletID.String()+"/orders/"+orderID.String()+"/items/"+itemID.String()+"/status", map[string]string{
		"status": "PREPARING",
	}, claims)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["error"] == nil {
		t.Fatal("expected error field")
	}
	errMsg := resp["error"].(string)
	if errMsg != "cannot update items on a COMPLETED order" {
		t.Fatalf("error message: got %q, want %q", errMsg, "cannot update items on a COMPLETED order")
	}
}

// --- ListActive Tests ---

func TestListActive_Success(t *testing.T) {
	outletID := uuid.New()
	order1ID := uuid.New()
	order2ID := uuid.New()
	claims := testClaims(outletID)

	// Create mock rows with all required fields
	var totalAmount1, totalAmount2 pgtype.Numeric
	_ = totalAmount1.Scan("100000.00")
	_ = totalAmount2.Scan("75000.00")

	var amountPaid1, amountPaid2 pgtype.Numeric
	_ = amountPaid1.Scan("50000.00")
	_ = amountPaid2.Scan("75000.00")

	var subtotal1, subtotal2, discountAmt1, discountAmt2, taxAmt1, taxAmt2 pgtype.Numeric
	_ = subtotal1.Scan("100000.00")
	_ = subtotal2.Scan("75000.00")
	_ = discountAmt1.Scan("0.00")
	_ = discountAmt2.Scan("0.00")
	_ = taxAmt1.Scan("0.00")
	_ = taxAmt2.Scan("0.00")

	store := &mockOrderStore{
		listActiveOrdersFn: func(ctx context.Context, arg database.ListActiveOrdersParams) ([]database.ListActiveOrdersRow, error) {
			if arg.OutletID != outletID {
				t.Errorf("OutletID: got %v, want %v", arg.OutletID, outletID)
			}
			if arg.Limit != 50 {
				t.Errorf("Limit: got %d, want %d", arg.Limit, 50)
			}
			if arg.Offset != 0 {
				t.Errorf("Offset: got %d, want %d", arg.Offset, 0)
			}
			return []database.ListActiveOrdersRow{
				{
					ID:             order1ID,
					OutletID:       outletID,
					OrderNumber:    "ORD-001",
					OrderType:      database.OrderTypeDINEIN,
					Status:         database.OrderStatusNEW,
					Subtotal:       subtotal1,
					DiscountAmount: discountAmt1,
					TaxAmount:      taxAmt1,
					TotalAmount:    totalAmount1,
					AmountPaid:     amountPaid1,
					CreatedBy:      uuid.New(),
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
				},
				{
					ID:             order2ID,
					OutletID:       outletID,
					OrderNumber:    "ORD-002",
					OrderType:      database.OrderTypeTAKEAWAY,
					Status:         database.OrderStatusPREPARING,
					Subtotal:       subtotal2,
					DiscountAmount: discountAmt2,
					TaxAmount:      taxAmt2,
					TotalAmount:    totalAmount2,
					AmountPaid:     amountPaid2,
					CreatedBy:      uuid.New(),
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
				},
			}, nil
		},
	}

	svc := &mockOrderService{}
	router := setupOrderRouterWithStore(svc, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders/active", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal("decode response:", err)
	}

	orders := resp["orders"].([]interface{})
	if len(orders) != 2 {
		t.Fatalf("orders count: got %d, want %d", len(orders), 2)
	}

	order1 := orders[0].(map[string]interface{})
	if order1["order_number"] != "ORD-001" {
		t.Errorf("order_number: got %v, want %v", order1["order_number"], "ORD-001")
	}
	if order1["amount_paid"] != "50000.00" {
		t.Errorf("amount_paid: got %v, want %v", order1["amount_paid"], "50000.00")
	}

	order2 := orders[1].(map[string]interface{})
	if order2["order_number"] != "ORD-002" {
		t.Errorf("order_number: got %v, want %v", order2["order_number"], "ORD-002")
	}
	if order2["amount_paid"] != "75000.00" {
		t.Errorf("amount_paid: got %v, want %v", order2["amount_paid"], "75000.00")
	}

	if int(resp["limit"].(float64)) != 50 {
		t.Errorf("limit: got %v, want %d", resp["limit"], 50)
	}
	if int(resp["offset"].(float64)) != 0 {
		t.Errorf("offset: got %v, want %d", resp["offset"], 0)
	}
}

func TestListActive_EmptyList(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		listActiveOrdersFn: func(ctx context.Context, arg database.ListActiveOrdersParams) ([]database.ListActiveOrdersRow, error) {
			return []database.ListActiveOrdersRow{}, nil
		},
	}

	svc := &mockOrderService{}
	router := setupOrderRouterWithStore(svc, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders/active", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal("decode response:", err)
	}

	orders := resp["orders"].([]interface{})
	if len(orders) != 0 {
		t.Fatalf("orders count: got %d, want %d", len(orders), 0)
	}
}

func TestListActive_Pagination(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)

	store := &mockOrderStore{
		listActiveOrdersFn: func(ctx context.Context, arg database.ListActiveOrdersParams) ([]database.ListActiveOrdersRow, error) {
			if arg.Limit != 10 {
				t.Errorf("Limit: got %d, want %d", arg.Limit, 10)
			}
			if arg.Offset != 20 {
				t.Errorf("Offset: got %d, want %d", arg.Offset, 20)
			}
			return []database.ListActiveOrdersRow{}, nil
		},
	}

	svc := &mockOrderService{}
	router := setupOrderRouterWithStore(svc, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/"+outletID.String()+"/orders/active?limit=10&offset=20", nil, claims)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal("decode response:", err)
	}

	if int(resp["limit"].(float64)) != 10 {
		t.Errorf("limit: got %v, want %d", resp["limit"], 10)
	}
	if int(resp["offset"].(float64)) != 20 {
		t.Errorf("offset: got %v, want %d", resp["offset"], 20)
	}
}

func TestListActive_InvalidOutletID(t *testing.T) {
	outletID := uuid.New()
	claims := testClaims(outletID)
	store := &mockOrderStore{}
	svc := &mockOrderService{}
	router := setupOrderRouterWithStore(svc, store, claims)
	rr := doAuthRequest(t, router, "GET", "/outlets/invalid-uuid/orders/active", nil, claims)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["error"] == nil {
		t.Fatal("expected error field")
	}
}

func TestListActive_Unauthenticated(t *testing.T) {
	outletID := uuid.New()
	store := &mockOrderStore{}
	svc := &mockOrderService{}
	router := chi.NewRouter()
	router.Use(middleware.Authenticate(testJWTSecret))
	pool := &mockPool{}
	newStore := mockNewStore(store)
	h := handler.NewOrderHandler(svc, store, pool, newStore)
	router.Route("/outlets/{oid}/orders", h.RegisterRoutes)

	req := httptest.NewRequest("GET", "/outlets/"+outletID.String()+"/orders/active", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}

	resp := decodeOrderResponse(t, rr)
	if resp["error"] == nil {
		t.Fatal("expected error field")
	}
}
