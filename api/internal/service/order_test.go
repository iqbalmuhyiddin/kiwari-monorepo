package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Mock implementations ---

// mockTx implements pgx.Tx with only the methods we need.
// The unused methods panic so we catch accidental calls.
type mockTx struct {
	commitErr   error
	rollbackErr error
}

func (m *mockTx) Begin(ctx context.Context) (pgx.Tx, error) { panic("not implemented") }
func (m *mockTx) Commit(ctx context.Context) error          { return m.commitErr }
func (m *mockTx) Rollback(ctx context.Context) error        { return m.rollbackErr }
func (m *mockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	panic("not implemented")
}
func (m *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	panic("not implemented")
}
func (m *mockTx) LargeObjects() pgx.LargeObjects { panic("not implemented") }
func (m *mockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	panic("not implemented")
}
func (m *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	panic("not implemented")
}
func (m *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	panic("not implemented")
}
func (m *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	panic("not implemented")
}
func (m *mockTx) Conn() *pgx.Conn { panic("not implemented") }

// mockTxBeginner implements TxBeginner.
type mockTxBeginner struct {
	tx  pgx.Tx
	err error
}

func (m *mockTxBeginner) Begin(ctx context.Context) (pgx.Tx, error) {
	return m.tx, m.err
}

// mockOrderStore implements OrderStore with configurable behavior.
type mockOrderStore struct {
	getNextOrderNumberFn    func(ctx context.Context, outletID uuid.UUID) (int32, error)
	getProductForOrderFn    func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error)
	getVariantForOrderFn    func(ctx context.Context, id uuid.UUID) (database.GetVariantForOrderRow, error)
	getModifierForOrderFn   func(ctx context.Context, id uuid.UUID) (database.GetModifierForOrderRow, error)
	createOrderFn           func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error)
	createOrderItemFn       func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error)
	createOrderItemModFn    func(ctx context.Context, arg database.CreateOrderItemModifierParams) (database.OrderItemModifier, error)
}

func (m *mockOrderStore) GetNextOrderNumber(ctx context.Context, outletID uuid.UUID) (int32, error) {
	return m.getNextOrderNumberFn(ctx, outletID)
}
func (m *mockOrderStore) GetProductForOrder(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
	return m.getProductForOrderFn(ctx, arg)
}
func (m *mockOrderStore) GetVariantForOrder(ctx context.Context, id uuid.UUID) (database.GetVariantForOrderRow, error) {
	return m.getVariantForOrderFn(ctx, id)
}
func (m *mockOrderStore) GetModifierForOrder(ctx context.Context, id uuid.UUID) (database.GetModifierForOrderRow, error) {
	return m.getModifierForOrderFn(ctx, id)
}
func (m *mockOrderStore) CreateOrder(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
	return m.createOrderFn(ctx, arg)
}
func (m *mockOrderStore) CreateOrderItem(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
	return m.createOrderItemFn(ctx, arg)
}
func (m *mockOrderStore) CreateOrderItemModifier(ctx context.Context, arg database.CreateOrderItemModifierParams) (database.OrderItemModifier, error) {
	return m.createOrderItemModFn(ctx, arg)
}

// --- Test helpers ---

func makeNumeric(val string) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(val)
	return n
}

func numericEquals(n pgtype.Numeric, expected string) bool {
	d := numericToDecimal(n)
	exp, _ := decimal.NewFromString(expected)
	return d.Equal(exp)
}

// newTestService creates an OrderService with mocked dependencies.
// store is the mock OrderStore that will be returned by the NewOrderStore factory.
func newTestService(store *mockOrderStore) (*OrderService, *mockTx) {
	tx := &mockTx{}
	pool := &mockTxBeginner{tx: tx}
	newStore := func(db database.DBTX) OrderStore { return store }
	return NewOrderService(pool, newStore), tx
}

// defaultStore returns a mockOrderStore with sensible defaults for a basic order.
// Individual tests override the functions they care about.
func defaultStore(outletID, productID uuid.UUID) *mockOrderStore {
	return &mockOrderStore{
		getNextOrderNumberFn: func(ctx context.Context, oid uuid.UUID) (int32, error) {
			return 1, nil
		},
		getProductForOrderFn: func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
			if arg.ID == productID && arg.OutletID == outletID {
				return database.GetProductForOrderRow{
					ID:        productID,
					OutletID:  outletID,
					BasePrice: makeNumeric("25000.00"),
					Station:   database.NullKitchenStation{KitchenStation: database.KitchenStationGRILL, Valid: true},
				}, nil
			}
			return database.GetProductForOrderRow{}, pgx.ErrNoRows
		},
		getVariantForOrderFn: func(ctx context.Context, id uuid.UUID) (database.GetVariantForOrderRow, error) {
			return database.GetVariantForOrderRow{}, pgx.ErrNoRows
		},
		getModifierForOrderFn: func(ctx context.Context, id uuid.UUID) (database.GetModifierForOrderRow, error) {
			return database.GetModifierForOrderRow{}, pgx.ErrNoRows
		},
		createOrderFn: func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
			return database.Order{
				ID:             uuid.New(),
				OutletID:       arg.OutletID,
				OrderNumber:    arg.OrderNumber,
				OrderType:      arg.OrderType,
				Status:         database.OrderStatusNEW,
				Subtotal:       arg.Subtotal,
				DiscountType:   arg.DiscountType,
				DiscountValue:  arg.DiscountValue,
				DiscountAmount: arg.DiscountAmount,
				TaxAmount:      arg.TaxAmount,
				TotalAmount:    arg.TotalAmount,
				CreatedBy:      arg.CreatedBy,
			}, nil
		},
		createOrderItemFn: func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
			return database.OrderItem{
				ID:             uuid.New(),
				OrderID:        arg.OrderID,
				ProductID:      arg.ProductID,
				VariantID:      arg.VariantID,
				Quantity:       arg.Quantity,
				UnitPrice:      arg.UnitPrice,
				DiscountType:   arg.DiscountType,
				DiscountValue:  arg.DiscountValue,
				DiscountAmount: arg.DiscountAmount,
				Subtotal:       arg.Subtotal,
				Notes:          arg.Notes,
				Status:         database.OrderItemStatusPENDING,
				Station:        arg.Station,
			}, nil
		},
		createOrderItemModFn: func(ctx context.Context, arg database.CreateOrderItemModifierParams) (database.OrderItemModifier, error) {
			return database.OrderItemModifier{
				ID:          uuid.New(),
				OrderItemID: arg.OrderItemID,
				ModifierID:  arg.ModifierID,
				Quantity:    arg.Quantity,
				UnitPrice:   arg.UnitPrice,
			}, nil
		},
	}
}

func basicReq(outletID uuid.UUID, productID string) CreateOrderRequest {
	return CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: productID, Quantity: 2},
		},
	}
}

// =====================
// Validation tests
// =====================

func TestCreateOrder_EmptyItems(t *testing.T) {
	store := defaultStore(uuid.New(), uuid.New())
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  uuid.New(),
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items:     nil,
	})
	if !errors.Is(err, ErrEmptyItems) {
		t.Fatalf("expected ErrEmptyItems, got: %v", err)
	}
}

func TestCreateOrder_InvalidOrderType(t *testing.T) {
	store := defaultStore(uuid.New(), uuid.New())
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  uuid.New(),
		CreatedBy: uuid.New(),
		OrderType: "INVALID",
		Items: []CreateOrderItemRequest{
			{ProductID: uuid.New().String(), Quantity: 1},
		},
	})
	if !errors.Is(err, ErrInvalidOrderType) {
		t.Fatalf("expected ErrInvalidOrderType, got: %v", err)
	}
}

func TestCreateOrder_ZeroQuantity(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), Quantity: 0},
		},
	})
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("expected ErrInvalidQuantity, got: %v", err)
	}
}

func TestCreateOrder_MissingProductID(t *testing.T) {
	store := defaultStore(uuid.New(), uuid.New())
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  uuid.New(),
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: "", Quantity: 1},
		},
	})
	if !errors.Is(err, ErrInvalidProductID) {
		t.Fatalf("expected ErrInvalidProductID, got: %v", err)
	}
}

func TestCreateOrder_CateringWithoutDate(t *testing.T) {
	store := defaultStore(uuid.New(), uuid.New())
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:   uuid.New(),
		CreatedBy:  uuid.New(),
		OrderType:  "CATERING",
		CustomerID: uuid.New().String(),
		Items: []CreateOrderItemRequest{
			{ProductID: uuid.New().String(), Quantity: 1},
		},
	})
	if !errors.Is(err, ErrCateringDate) {
		t.Fatalf("expected ErrCateringDate, got: %v", err)
	}
}

func TestCreateOrder_CateringWithoutCustomer(t *testing.T) {
	store := defaultStore(uuid.New(), uuid.New())
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:     uuid.New(),
		CreatedBy:    uuid.New(),
		OrderType:    "CATERING",
		CateringDate: "2026-03-01T10:00:00Z",
		Items: []CreateOrderItemRequest{
			{ProductID: uuid.New().String(), Quantity: 1},
		},
	})
	if !errors.Is(err, ErrCateringCustomer) {
		t.Fatalf("expected ErrCateringCustomer, got: %v", err)
	}
}

func TestCreateOrder_ProductNotFound(t *testing.T) {
	outletID := uuid.New()
	store := defaultStore(outletID, uuid.New()) // store knows a different product
	svc, _ := newTestService(store)

	unknownProduct := uuid.New()
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: unknownProduct.String(), Quantity: 1},
		},
	})
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got: %v", err)
	}
}

func TestCreateOrder_VariantMismatch(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()
	wrongProductID := uuid.New() // variant belongs to different product

	store := defaultStore(outletID, productID)
	store.getVariantForOrderFn = func(ctx context.Context, id uuid.UUID) (database.GetVariantForOrderRow, error) {
		if id == variantID {
			return database.GetVariantForOrderRow{
				ID:              variantID,
				PriceAdjustment: makeNumeric("5000.00"),
				ProductID:       wrongProductID, // mismatch
			}, nil
		}
		return database.GetVariantForOrderRow{}, pgx.ErrNoRows
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), VariantID: variantID.String(), Quantity: 1},
		},
	})
	if !errors.Is(err, ErrVariantMismatch) {
		t.Fatalf("expected ErrVariantMismatch, got: %v", err)
	}
}

func TestCreateOrder_VariantNotFound(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), VariantID: uuid.New().String(), Quantity: 1},
		},
	})
	if !errors.Is(err, ErrVariantNotFound) {
		t.Fatalf("expected ErrVariantNotFound, got: %v", err)
	}
}

func TestCreateOrder_ModifierMismatch(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	modifierID := uuid.New()
	wrongProductID := uuid.New()

	store := defaultStore(outletID, productID)
	store.getModifierForOrderFn = func(ctx context.Context, id uuid.UUID) (database.GetModifierForOrderRow, error) {
		if id == modifierID {
			return database.GetModifierForOrderRow{
				ID:        modifierID,
				Price:     makeNumeric("3000.00"),
				ProductID: wrongProductID, // mismatch
			}, nil
		}
		return database.GetModifierForOrderRow{}, pgx.ErrNoRows
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{
				ProductID: productID.String(),
				Quantity:  1,
				Modifiers: []CreateOrderItemModifierRequest{
					{ModifierID: modifierID.String(), Quantity: 1},
				},
			},
		},
	})
	if !errors.Is(err, ErrModifierMismatch) {
		t.Fatalf("expected ErrModifierMismatch, got: %v", err)
	}
}

func TestCreateOrder_ModifierNotFound(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{
				ProductID: productID.String(),
				Quantity:  1,
				Modifiers: []CreateOrderItemModifierRequest{
					{ModifierID: uuid.New().String(), Quantity: 1},
				},
			},
		},
	})
	if !errors.Is(err, ErrModifierNotFound) {
		t.Fatalf("expected ErrModifierNotFound, got: %v", err)
	}
}

func TestCreateOrder_InvalidDiscount(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)
	svc, _ := newTestService(store)

	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:      outletID,
		CreatedBy:     uuid.New(),
		OrderType:     "DINE_IN",
		DiscountType:  "BOGUS",
		DiscountValue: "10",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), Quantity: 1},
		},
	})
	if !errors.Is(err, ErrInvalidDiscount) {
		t.Fatalf("expected ErrInvalidDiscount, got: %v", err)
	}
}

// =====================
// Price calculation tests
// =====================

func TestCreateOrder_BasicPrice(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	// Capture the CreateOrder params to verify price calculations.
	var captured database.CreateOrderParams
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		captured = arg
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			TaxAmount: arg.TaxAmount, DiscountAmount: arg.DiscountAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	var capturedItem database.CreateOrderItemParams
	store.createOrderItemFn = func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
		capturedItem = arg
		return database.OrderItem{
			ID: uuid.New(), OrderID: arg.OrderID, ProductID: arg.ProductID,
			Quantity: arg.Quantity, UnitPrice: arg.UnitPrice,
			Subtotal: arg.Subtotal, DiscountAmount: arg.DiscountAmount,
			Status: database.OrderItemStatusPENDING, Station: arg.Station,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// unit_price = base_price (25000) + 0 variant = 25000
	if !numericEquals(capturedItem.UnitPrice, "25000.00") {
		t.Errorf("item unit_price: got %v, want 25000.00", numericToDecimal(capturedItem.UnitPrice))
	}
	// subtotal = 25000 * 2 = 50000
	if !numericEquals(capturedItem.Subtotal, "50000.00") {
		t.Errorf("item subtotal: got %v, want 50000.00", numericToDecimal(capturedItem.Subtotal))
	}
	// order subtotal = 50000
	if !numericEquals(captured.Subtotal, "50000.00") {
		t.Errorf("order subtotal: got %v, want 50000.00", numericToDecimal(captured.Subtotal))
	}
	// total = subtotal - 0 discount + 0 tax = 50000
	if !numericEquals(captured.TotalAmount, "50000.00") {
		t.Errorf("order total: got %v, want 50000.00", numericToDecimal(captured.TotalAmount))
	}
}

func TestCreateOrder_WithVariant(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()

	store := defaultStore(outletID, productID)
	store.getVariantForOrderFn = func(ctx context.Context, id uuid.UUID) (database.GetVariantForOrderRow, error) {
		if id == variantID {
			return database.GetVariantForOrderRow{
				ID:              variantID,
				PriceAdjustment: makeNumeric("5000.00"),
				ProductID:       productID,
			}, nil
		}
		return database.GetVariantForOrderRow{}, pgx.ErrNoRows
	}

	var capturedItem database.CreateOrderItemParams
	store.createOrderItemFn = func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
		capturedItem = arg
		return database.OrderItem{
			ID: uuid.New(), OrderID: arg.OrderID, ProductID: arg.ProductID,
			Quantity: arg.Quantity, UnitPrice: arg.UnitPrice,
			Subtotal: arg.Subtotal, Status: database.OrderItemStatusPENDING,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), VariantID: variantID.String(), Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// unit_price = 25000 + 5000 = 30000
	if !numericEquals(capturedItem.UnitPrice, "30000.00") {
		t.Errorf("unit_price with variant: got %v, want 30000.00", numericToDecimal(capturedItem.UnitPrice))
	}
	// subtotal = 30000 * 1 = 30000
	if !numericEquals(capturedItem.Subtotal, "30000.00") {
		t.Errorf("subtotal with variant: got %v, want 30000.00", numericToDecimal(capturedItem.Subtotal))
	}
}

func TestCreateOrder_WithModifiers(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	modifierID := uuid.New()

	store := defaultStore(outletID, productID)
	store.getModifierForOrderFn = func(ctx context.Context, id uuid.UUID) (database.GetModifierForOrderRow, error) {
		if id == modifierID {
			return database.GetModifierForOrderRow{
				ID:        modifierID,
				Price:     makeNumeric("3000.00"),
				ProductID: productID,
			}, nil
		}
		return database.GetModifierForOrderRow{}, pgx.ErrNoRows
	}

	var capturedItem database.CreateOrderItemParams
	store.createOrderItemFn = func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
		capturedItem = arg
		return database.OrderItem{
			ID: uuid.New(), OrderID: arg.OrderID, ProductID: arg.ProductID,
			Quantity: arg.Quantity, UnitPrice: arg.UnitPrice,
			Subtotal: arg.Subtotal, Status: database.OrderItemStatusPENDING,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{
				ProductID: productID.String(),
				Quantity:  2,
				Modifiers: []CreateOrderItemModifierRequest{
					{ModifierID: modifierID.String(), Quantity: 3},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// unit_price = 25000 (no variant)
	if !numericEquals(capturedItem.UnitPrice, "25000.00") {
		t.Errorf("unit_price: got %v, want 25000.00", numericToDecimal(capturedItem.UnitPrice))
	}
	// item subtotal = (25000 * 2) + (3000 * 3) = 50000 + 9000 = 59000
	if !numericEquals(capturedItem.Subtotal, "59000.00") {
		t.Errorf("subtotal with modifiers: got %v, want 59000.00", numericToDecimal(capturedItem.Subtotal))
	}
}

func TestCreateOrder_MultipleItems(t *testing.T) {
	outletID := uuid.New()
	productA := uuid.New()
	productB := uuid.New()

	store := defaultStore(outletID, productA)
	// Override to handle two products.
	store.getProductForOrderFn = func(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error) {
		switch arg.ID {
		case productA:
			return database.GetProductForOrderRow{
				ID: productA, OutletID: outletID,
				BasePrice: makeNumeric("10000.00"),
				Station:   database.NullKitchenStation{KitchenStation: database.KitchenStationGRILL, Valid: true},
			}, nil
		case productB:
			return database.GetProductForOrderRow{
				ID: productB, OutletID: outletID,
				BasePrice: makeNumeric("15000.00"),
				Station:   database.NullKitchenStation{KitchenStation: database.KitchenStationBEVERAGE, Valid: true},
			}, nil
		}
		return database.GetProductForOrderRow{}, pgx.ErrNoRows
	}

	var capturedOrder database.CreateOrderParams
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		capturedOrder = arg
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			TaxAmount: arg.TaxAmount, DiscountAmount: arg.DiscountAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{ProductID: productA.String(), Quantity: 2}, // 10000 * 2 = 20000
			{ProductID: productB.String(), Quantity: 3}, // 15000 * 3 = 45000
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// order subtotal = 20000 + 45000 = 65000
	if !numericEquals(capturedOrder.Subtotal, "65000.00") {
		t.Errorf("order subtotal: got %v, want 65000.00", numericToDecimal(capturedOrder.Subtotal))
	}
	// total = 65000 (no discount, no tax)
	if !numericEquals(capturedOrder.TotalAmount, "65000.00") {
		t.Errorf("order total: got %v, want 65000.00", numericToDecimal(capturedOrder.TotalAmount))
	}
}

// =====================
// Discount calculation tests
// =====================

func TestCreateOrder_ItemPercentageDiscount(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	var capturedItem database.CreateOrderItemParams
	store.createOrderItemFn = func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
		capturedItem = arg
		return database.OrderItem{
			ID: uuid.New(), OrderID: arg.OrderID, ProductID: arg.ProductID,
			Quantity: arg.Quantity, UnitPrice: arg.UnitPrice,
			Subtotal: arg.Subtotal, DiscountAmount: arg.DiscountAmount,
			Status: database.OrderItemStatusPENDING,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{
				ProductID:     productID.String(),
				Quantity:      2,
				DiscountType:  "PERCENTAGE",
				DiscountValue: "10",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// line_total = 25000 * 2 = 50000
	// discount = 50000 * 10 / 100 = 5000
	// subtotal = 50000 - 5000 = 45000
	if !numericEquals(capturedItem.DiscountAmount, "5000.00") {
		t.Errorf("item discount_amount: got %v, want 5000.00", numericToDecimal(capturedItem.DiscountAmount))
	}
	if !numericEquals(capturedItem.Subtotal, "45000.00") {
		t.Errorf("item subtotal: got %v, want 45000.00", numericToDecimal(capturedItem.Subtotal))
	}
}

func TestCreateOrder_ItemFixedDiscount(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	var capturedItem database.CreateOrderItemParams
	store.createOrderItemFn = func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
		capturedItem = arg
		return database.OrderItem{
			ID: uuid.New(), OrderID: arg.OrderID, ProductID: arg.ProductID,
			Quantity: arg.Quantity, UnitPrice: arg.UnitPrice,
			Subtotal: arg.Subtotal, DiscountAmount: arg.DiscountAmount,
			Status: database.OrderItemStatusPENDING,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{
				ProductID:     productID.String(),
				Quantity:      2,
				DiscountType:  "FIXED_AMOUNT",
				DiscountValue: "7000",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// line_total = 25000 * 2 = 50000
	// discount = 7000 (fixed)
	// subtotal = 50000 - 7000 = 43000
	if !numericEquals(capturedItem.DiscountAmount, "7000.00") {
		t.Errorf("item discount_amount: got %v, want 7000.00", numericToDecimal(capturedItem.DiscountAmount))
	}
	if !numericEquals(capturedItem.Subtotal, "43000.00") {
		t.Errorf("item subtotal: got %v, want 43000.00", numericToDecimal(capturedItem.Subtotal))
	}
}

func TestCreateOrder_OrderPercentageDiscount(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	var capturedOrder database.CreateOrderParams
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		capturedOrder = arg
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			DiscountAmount: arg.DiscountAmount, TaxAmount: arg.TaxAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:      outletID,
		CreatedBy:     uuid.New(),
		OrderType:     "DINE_IN",
		DiscountType:  "PERCENTAGE",
		DiscountValue: "20",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), Quantity: 2}, // 25000 * 2 = 50000
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// subtotal = 50000
	// discount = 50000 * 20 / 100 = 10000
	// total = 50000 - 10000 = 40000
	if !numericEquals(capturedOrder.DiscountAmount, "10000.00") {
		t.Errorf("order discount_amount: got %v, want 10000.00", numericToDecimal(capturedOrder.DiscountAmount))
	}
	if !numericEquals(capturedOrder.TotalAmount, "40000.00") {
		t.Errorf("order total: got %v, want 40000.00", numericToDecimal(capturedOrder.TotalAmount))
	}
}

func TestCreateOrder_OrderFixedDiscount(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	var capturedOrder database.CreateOrderParams
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		capturedOrder = arg
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			DiscountAmount: arg.DiscountAmount, TaxAmount: arg.TaxAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:      outletID,
		CreatedBy:     uuid.New(),
		OrderType:     "DINE_IN",
		DiscountType:  "FIXED_AMOUNT",
		DiscountValue: "15000",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), Quantity: 2}, // 25000 * 2 = 50000
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// total = 50000 - 15000 = 35000
	if !numericEquals(capturedOrder.DiscountAmount, "15000.00") {
		t.Errorf("order discount_amount: got %v, want 15000.00", numericToDecimal(capturedOrder.DiscountAmount))
	}
	if !numericEquals(capturedOrder.TotalAmount, "35000.00") {
		t.Errorf("order total: got %v, want 35000.00", numericToDecimal(capturedOrder.TotalAmount))
	}
}

func TestCreateOrder_NegativeSubtotalClampedToZero(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	var capturedOrder database.CreateOrderParams
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		capturedOrder = arg
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			DiscountAmount: arg.DiscountAmount, TaxAmount: arg.TaxAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:      outletID,
		CreatedBy:     uuid.New(),
		OrderType:     "DINE_IN",
		DiscountType:  "FIXED_AMOUNT",
		DiscountValue: "999999", // way more than subtotal
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), Quantity: 1}, // 25000
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// total should be clamped to 0, not negative
	if !numericEquals(capturedOrder.TotalAmount, "0.00") {
		t.Errorf("order total (clamped): got %v, want 0.00", numericToDecimal(capturedOrder.TotalAmount))
	}
}

func TestCreateOrder_ItemNegativeSubtotalClampedToZero(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	var capturedItem database.CreateOrderItemParams
	store.createOrderItemFn = func(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error) {
		capturedItem = arg
		return database.OrderItem{
			ID: uuid.New(), OrderID: arg.OrderID, ProductID: arg.ProductID,
			Quantity: arg.Quantity, UnitPrice: arg.UnitPrice,
			Subtotal: arg.Subtotal, DiscountAmount: arg.DiscountAmount,
			Status: database.OrderItemStatusPENDING,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:  outletID,
		CreatedBy: uuid.New(),
		OrderType: "DINE_IN",
		Items: []CreateOrderItemRequest{
			{
				ProductID:     productID.String(),
				Quantity:      1,
				DiscountType:  "FIXED_AMOUNT",
				DiscountValue: "999999", // way more than item total
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// item subtotal should be clamped to 0
	if !numericEquals(capturedItem.Subtotal, "0.00") {
		t.Errorf("item subtotal (clamped): got %v, want 0.00", numericToDecimal(capturedItem.Subtotal))
	}
}

// =====================
// Order number generation tests
// =====================

func TestCreateOrder_FirstOrderOfDay(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)
	store.getNextOrderNumberFn = func(ctx context.Context, oid uuid.UUID) (int32, error) {
		return 1, nil // first order
	}

	var capturedOrder database.CreateOrderParams
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		capturedOrder = arg
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			TaxAmount: arg.TaxAmount, DiscountAmount: arg.DiscountAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	svc, _ := newTestService(store)
	result, err := svc.CreateOrder(context.Background(), basicReq(outletID, productID.String()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedOrder.OrderNumber != "KWR-001" {
		t.Errorf("order number: got %v, want KWR-001", capturedOrder.OrderNumber)
	}
	if result.Order.OrderNumber != "KWR-001" {
		t.Errorf("result order number: got %v, want KWR-001", result.Order.OrderNumber)
	}
}

func TestCreateOrder_SubsequentOrder(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)
	store.getNextOrderNumberFn = func(ctx context.Context, oid uuid.UUID) (int32, error) {
		return 42, nil // 42nd order of the day
	}

	var capturedOrder database.CreateOrderParams
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		capturedOrder = arg
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			TaxAmount: arg.TaxAmount, DiscountAmount: arg.DiscountAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), basicReq(outletID, productID.String()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedOrder.OrderNumber != "KWR-042" {
		t.Errorf("order number: got %v, want KWR-042", capturedOrder.OrderNumber)
	}
}

// =====================
// Retry on unique constraint violation (race condition fix)
// =====================

func TestCreateOrder_RetryOnUniqueViolation(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	createCallCount := 0
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		createCallCount++
		if createCallCount == 1 {
			// First attempt: unique constraint violation
			return database.Order{}, &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "orders_outlet_id_order_number_key",
			}
		}
		// Second attempt: success
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			TaxAmount: arg.TaxAmount, DiscountAmount: arg.DiscountAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	// GetNextOrderNumber should be called twice (once per attempt)
	orderNumCallCount := 0
	store.getNextOrderNumberFn = func(ctx context.Context, oid uuid.UUID) (int32, error) {
		orderNumCallCount++
		return int32(orderNumCallCount), nil
	}

	svc, _ := newTestService(store)
	result, err := svc.CreateOrder(context.Background(), basicReq(outletID, productID.String()))
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if createCallCount != 2 {
		t.Errorf("expected 2 CreateOrder calls (1 fail + 1 success), got %d", createCallCount)
	}
	if orderNumCallCount != 2 {
		t.Errorf("expected 2 GetNextOrderNumber calls, got %d", orderNumCallCount)
	}
}

func TestCreateOrder_RetryExhausted(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	// Always return unique violation
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		return database.Order{}, &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "orders_outlet_id_order_number_key",
		}
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), basicReq(outletID, productID.String()))
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if !strings.Contains(err.Error(), "create order") {
		t.Errorf("expected 'create order' in error message, got: %v", err)
	}
}

func TestCreateOrder_NonUniqueErrorNotRetried(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	store := defaultStore(outletID, productID)

	callCount := 0
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		callCount++
		return database.Order{}, errors.New("some other DB error")
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), basicReq(outletID, productID.String()))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("non-unique errors should not retry: expected 1 call, got %d", callCount)
	}
}

// =====================
// Catering order test
// =====================

func TestCreateOrder_CateringHappyPath(t *testing.T) {
	outletID := uuid.New()
	productID := uuid.New()
	customerID := uuid.New()
	store := defaultStore(outletID, productID)

	var capturedOrder database.CreateOrderParams
	store.createOrderFn = func(ctx context.Context, arg database.CreateOrderParams) (database.Order, error) {
		capturedOrder = arg
		return database.Order{
			ID: uuid.New(), OutletID: arg.OutletID, OrderNumber: arg.OrderNumber,
			OrderType: arg.OrderType, Status: database.OrderStatusNEW,
			Subtotal: arg.Subtotal, TotalAmount: arg.TotalAmount,
			TaxAmount: arg.TaxAmount, DiscountAmount: arg.DiscountAmount,
			CateringDate: arg.CateringDate, CateringStatus: arg.CateringStatus,
			CateringDpAmount: arg.CateringDpAmount,
			CreatedBy: arg.CreatedBy,
		}, nil
	}

	svc, _ := newTestService(store)
	_, err := svc.CreateOrder(context.Background(), CreateOrderRequest{
		OutletID:         outletID,
		CreatedBy:        uuid.New(),
		OrderType:        "CATERING",
		CustomerID:       customerID.String(),
		CateringDate:     "2026-03-15T12:00:00Z",
		CateringDpAmount: "100000",
		Items: []CreateOrderItemRequest{
			{ProductID: productID.String(), Quantity: 10},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedOrder.OrderType != database.OrderTypeCATERING {
		t.Errorf("order_type: got %v, want CATERING", capturedOrder.OrderType)
	}
	if !capturedOrder.CateringDate.Valid {
		t.Error("catering_date should be set")
	}
	if !capturedOrder.CateringStatus.Valid || capturedOrder.CateringStatus.CateringStatus != database.CateringStatusBOOKED {
		t.Errorf("catering_status: got %v, want BOOKED", capturedOrder.CateringStatus)
	}
	if !capturedOrder.CustomerID.Valid {
		t.Error("customer_id should be set for catering")
	}
}

