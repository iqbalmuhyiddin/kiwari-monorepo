package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/enum"
	"github.com/shopspring/decimal"
)

const maxOrderNumberRetries = 3

// Errors returned by the order service.
var (
	ErrEmptyItems            = errors.New("items are required")
	ErrInvalidOrderType      = errors.New("invalid order_type")
	ErrInvalidQuantity       = errors.New("quantity must be > 0")
	ErrProductNotFound       = errors.New("product not found in outlet")
	ErrVariantNotFound       = errors.New("variant not found")
	ErrVariantMismatch       = errors.New("variant does not belong to product")
	ErrModifierNotFound      = errors.New("modifier not found")
	ErrModifierMismatch      = errors.New("modifier does not belong to product")
	ErrCateringDate          = errors.New("catering_date is required for CATERING orders")
	ErrCateringCustomer      = errors.New("customer_id is required for CATERING orders")
	ErrInvalidDiscount       = errors.New("invalid discount_type")
	ErrInvalidProductID      = errors.New("invalid product_id")
	ErrInvalidVariantID      = errors.New("invalid variant_id")
	ErrInvalidModifierID     = errors.New("invalid modifier_id")
	ErrInvalidDiscountValue  = errors.New("invalid discount_value")
	ErrInvalidCustomerID     = errors.New("invalid customer_id")
	ErrInvalidCateringDate   = errors.New("invalid catering_date")
	ErrInvalidCateringDpAmt  = errors.New("invalid catering_dp_amount")
)

// TxBeginner starts a new database transaction.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// OrderStore defines the DB methods needed to create orders.
// Satisfied by *database.Queries (and its WithTx variant).
type OrderStore interface {
	GetNextOrderNumber(ctx context.Context, outletID uuid.UUID) (int32, error)
	GetProductForOrder(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error)
	GetVariantForOrder(ctx context.Context, id uuid.UUID) (database.GetVariantForOrderRow, error)
	GetModifierForOrder(ctx context.Context, id uuid.UUID) (database.GetModifierForOrderRow, error)
	CreateOrder(ctx context.Context, arg database.CreateOrderParams) (database.Order, error)
	CreateOrderItem(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error)
	CreateOrderItemModifier(ctx context.Context, arg database.CreateOrderItemModifierParams) (database.OrderItemModifier, error)
}

// NewOrderStore creates an OrderStore from a DBTX (pool or tx).
// This allows the service to create store instances from transactions.
type NewOrderStore func(db database.DBTX) OrderStore

// CreateOrderRequest is the validated input for creating an order.
type CreateOrderRequest struct {
	OutletID         uuid.UUID
	CreatedBy        uuid.UUID
	OrderType        string
	TableNumber      string
	CustomerID       string
	Notes            string
	DiscountType     string
	DiscountValue    string
	CateringDate     string // RFC3339
	CateringDpAmount string
	DeliveryPlatform string
	DeliveryAddress  string
	Items            []CreateOrderItemRequest
}

// CreateOrderItemRequest is a single item in the order.
type CreateOrderItemRequest struct {
	ProductID     string
	VariantID     string
	Quantity      int32
	Notes         string
	DiscountType  string
	DiscountValue string
	Modifiers     []CreateOrderItemModifierRequest
}

// CreateOrderItemModifierRequest is a modifier on an order item.
type CreateOrderItemModifierRequest struct {
	ModifierID string
	Quantity   int32
}

// CreateOrderResult is the full created order with items.
type CreateOrderResult struct {
	Order database.Order
	Items []OrderItemResult
}

// OrderItemResult is an item with its modifiers.
type OrderItemResult struct {
	Item      database.OrderItem
	Modifiers []database.OrderItemModifier
}

// OrderService handles order business logic.
type OrderService struct {
	pool     TxBeginner
	newStore NewOrderStore
}

// NewOrderService creates a new OrderService.
func NewOrderService(pool TxBeginner, newStore NewOrderStore) *OrderService {
	return &OrderService{pool: pool, newStore: newStore}
}

// modifierInfo holds data about a modifier to insert.
type modifierInfo struct {
	modifierID uuid.UUID
	quantity   int32
	unitPrice  decimal.Decimal
}

// processedItem holds a prepared order item and its modifiers.
type processedItem struct {
	params    database.CreateOrderItemParams
	modifiers []modifierInfo
}

// CreateOrder validates, calculates prices, and creates an order atomically.
// Retries up to maxOrderNumberRetries times on order_number unique constraint
// violations (race condition where concurrent transactions get the same MAX).
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResult, error) {
	// --- Validate order type ---
	orderType, err := validateOrderType(req.OrderType)
	if err != nil {
		return nil, err
	}

	// --- Validate items non-empty ---
	if len(req.Items) == 0 {
		return nil, ErrEmptyItems
	}

	// --- Validate catering requirements ---
	if orderType == enum.OrderTypeCatering {
		if req.CateringDate == "" {
			return nil, ErrCateringDate
		}
		if req.CustomerID == "" {
			return nil, ErrCateringCustomer
		}
	}

	// --- Validate order-level discount ---
	if req.DiscountType != "" {
		if !isValidDiscountType(req.DiscountType) {
			return nil, ErrInvalidDiscount
		}
	}

	// Retry loop: handles order_number unique constraint race condition.
	var lastErr error
	for attempt := 0; attempt < maxOrderNumberRetries; attempt++ {
		result, err := s.createOrderTx(ctx, req, orderType)
		if err == nil {
			return result, nil
		}
		if isOrderNumberConflict(err) {
			lastErr = err
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

// isOrderNumberConflict checks if the error is a unique constraint violation
// on the order number (pgconn error code 23505).
func isOrderNumberConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && pgErr.ConstraintName == "orders_outlet_id_order_number_key"
	}
	return false
}

// createOrderTx executes the full order creation in a single transaction.
func (s *OrderService) createOrderTx(ctx context.Context, req CreateOrderRequest, orderType string) (*CreateOrderResult, error) {
	// --- Begin transaction ---
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	store := s.newStore(tx)

	// --- Generate order number ---
	nextNum, err := store.GetNextOrderNumber(ctx, req.OutletID)
	if err != nil {
		return nil, fmt.Errorf("get next order number: %w", err)
	}
	orderNumber := fmt.Sprintf("KWR-%03d", nextNum)

	// --- Process items: validate + calculate prices ---
	orderSubtotal := decimal.Zero
	var items []processedItem

	for i, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("item[%d]: %w", i, ErrInvalidQuantity)
		}

		// Parse product ID
		productID, err := uuid.Parse(item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("item[%d]: %w", i, ErrInvalidProductID)
		}

		// Validate product exists in outlet and get base price
		product, err := store.GetProductForOrder(ctx, database.GetProductForOrderParams{
			ID:       productID,
			OutletID: req.OutletID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("item[%d]: %w", i, ErrProductNotFound)
			}
			return nil, fmt.Errorf("item[%d]: get product: %w", i, err)
		}

		// Get base price as decimal
		basePrice := numericToDecimal(product.BasePrice)

		// Validate variant if provided
		variantID := pgtype.UUID{}
		variantAdjustment := decimal.Zero
		if item.VariantID != "" {
			vid, err := uuid.Parse(item.VariantID)
			if err != nil {
				return nil, fmt.Errorf("item[%d]: %w", i, ErrInvalidVariantID)
			}
			variant, err := store.GetVariantForOrder(ctx, vid)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, fmt.Errorf("item[%d]: %w", i, ErrVariantNotFound)
				}
				return nil, fmt.Errorf("item[%d]: get variant: %w", i, err)
			}
			if variant.ProductID != productID {
				return nil, fmt.Errorf("item[%d]: %w", i, ErrVariantMismatch)
			}
			variantID = pgtype.UUID{Bytes: vid, Valid: true}
			variantAdjustment = numericToDecimal(variant.PriceAdjustment)
		}

		// unit_price = base_price + variant adjustment
		unitPrice := basePrice.Add(variantAdjustment)

		// Process modifiers
		modifiersTotal := decimal.Zero
		var itemModifiers []modifierInfo
		for j, mod := range item.Modifiers {
			if mod.Quantity <= 0 {
				return nil, fmt.Errorf("item[%d].modifiers[%d]: %w", i, j, ErrInvalidQuantity)
			}
			modID, err := uuid.Parse(mod.ModifierID)
			if err != nil {
				return nil, fmt.Errorf("item[%d].modifiers[%d]: %w", i, j, ErrInvalidModifierID)
			}
			modifier, err := store.GetModifierForOrder(ctx, modID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, fmt.Errorf("item[%d].modifiers[%d]: %w", i, j, ErrModifierNotFound)
				}
				return nil, fmt.Errorf("item[%d].modifiers[%d]: get modifier: %w", i, j, err)
			}
			if modifier.ProductID != productID {
				return nil, fmt.Errorf("item[%d].modifiers[%d]: %w", i, j, ErrModifierMismatch)
			}
			modPrice := numericToDecimal(modifier.Price)
			modifiersTotal = modifiersTotal.Add(modPrice.Mul(decimal.NewFromInt32(mod.Quantity)))
			itemModifiers = append(itemModifiers, modifierInfo{
				modifierID: modID,
				quantity:   mod.Quantity,
				unitPrice:  modPrice,
			})
		}

		// Calculate item discount
		itemDiscountType := pgtype.Text{}
		itemDiscountValue := pgtype.Numeric{}
		itemDiscountAmount := decimal.Zero
		if item.DiscountType != "" {
			if !isValidDiscountType(item.DiscountType) {
				return nil, fmt.Errorf("item[%d]: %w", i, ErrInvalidDiscount)
			}
			dv, err := decimal.NewFromString(item.DiscountValue)
			if err != nil {
				return nil, fmt.Errorf("item[%d]: %w", i, ErrInvalidDiscountValue)
			}
			itemDiscountType = pgtype.Text{
				String: item.DiscountType,
				Valid:  true,
			}
			itemDiscountValue = decimalToNumeric(dv)

			if item.DiscountType == "PERCENTAGE" {
				// Percentage discount applies to (unit_price * qty) + modifiers_total.
				// This means modifiers are also discounted — intentional business decision.
				lineTotal := unitPrice.Mul(decimal.NewFromInt32(item.Quantity)).Add(modifiersTotal)
				itemDiscountAmount = lineTotal.Mul(dv).Div(decimal.NewFromInt(100))
			} else {
				itemDiscountAmount = dv
			}
		}

		// item subtotal = (unit_price * quantity) + modifiers_total - discount_amount
		itemSubtotal := unitPrice.Mul(decimal.NewFromInt32(item.Quantity)).Add(modifiersTotal).Sub(itemDiscountAmount)
		if itemSubtotal.IsNegative() {
			itemSubtotal = decimal.Zero
		}

		orderSubtotal = orderSubtotal.Add(itemSubtotal)

		// Build item notes
		itemNotes := pgtype.Text{}
		if item.Notes != "" {
			itemNotes = pgtype.Text{String: item.Notes, Valid: true}
		}

		items = append(items, processedItem{
			params: database.CreateOrderItemParams{
				ProductID:      productID,
				VariantID:      variantID,
				Quantity:       item.Quantity,
				UnitPrice:      decimalToNumeric(unitPrice),
				DiscountType:   itemDiscountType,
				DiscountValue:  itemDiscountValue,
				DiscountAmount: decimalToNumeric(itemDiscountAmount),
				Subtotal:       decimalToNumeric(itemSubtotal),
				Notes:          itemNotes,
				Station:        product.Station,
			},
			modifiers: itemModifiers,
		})
	}

	// --- Calculate order-level discount ---
	orderDiscountType := pgtype.Text{}
	orderDiscountValue := pgtype.Numeric{}
	orderDiscountAmount := decimal.Zero
	if req.DiscountType != "" {
		dv, err := decimal.NewFromString(req.DiscountValue)
		if err != nil {
			return nil, ErrInvalidDiscountValue
		}
		orderDiscountType = pgtype.Text{
			String: req.DiscountType,
			Valid:  true,
		}
		orderDiscountValue = decimalToNumeric(dv)

		if req.DiscountType == "PERCENTAGE" {
			orderDiscountAmount = orderSubtotal.Mul(dv).Div(decimal.NewFromInt(100))
		} else {
			orderDiscountAmount = dv
		}
	}

	// --- Calculate total ---
	taxAmount := decimal.Zero // tax = 0 for now
	totalAmount := orderSubtotal.Sub(orderDiscountAmount).Add(taxAmount)
	if totalAmount.IsNegative() {
		totalAmount = decimal.Zero
	}

	// --- Build order params ---
	customerID := pgtype.UUID{}
	if req.CustomerID != "" {
		cid, err := uuid.Parse(req.CustomerID)
		if err != nil {
			return nil, ErrInvalidCustomerID
		}
		customerID = pgtype.UUID{Bytes: cid, Valid: true}
	}

	tableNumber := pgtype.Text{}
	if req.TableNumber != "" {
		tableNumber = pgtype.Text{String: req.TableNumber, Valid: true}
	}

	notes := pgtype.Text{}
	if req.Notes != "" {
		notes = pgtype.Text{String: req.Notes, Valid: true}
	}

	cateringDate := pgtype.Timestamptz{}
	cateringStatus := pgtype.Text{}
	cateringDpAmount := pgtype.Numeric{}
	if orderType == enum.OrderTypeCatering {
		t, err := time.Parse(time.RFC3339, req.CateringDate)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidCateringDate, err)
		}
		cateringDate = pgtype.Timestamptz{Time: t, Valid: true}
		cateringStatus = pgtype.Text{
			String: enum.CateringStatusBooked,
			Valid:  true,
		}
		if req.CateringDpAmount != "" {
			dpAmount, err := decimal.NewFromString(req.CateringDpAmount)
			if err != nil {
				return nil, ErrInvalidCateringDpAmt
			}
			cateringDpAmount = decimalToNumeric(dpAmount)
		}
	}

	deliveryPlatform := pgtype.Text{}
	deliveryAddress := pgtype.Text{}
	if orderType == enum.OrderTypeDelivery {
		if req.DeliveryPlatform != "" {
			deliveryPlatform = pgtype.Text{String: req.DeliveryPlatform, Valid: true}
		}
		if req.DeliveryAddress != "" {
			deliveryAddress = pgtype.Text{String: req.DeliveryAddress, Valid: true}
		}
	}

	// --- Insert order ---
	order, err := store.CreateOrder(ctx, database.CreateOrderParams{
		OutletID:         req.OutletID,
		OrderNumber:      orderNumber,
		CustomerID:       customerID,
		OrderType:        orderType,
		TableNumber:      tableNumber,
		Notes:            notes,
		Subtotal:         decimalToNumeric(orderSubtotal),
		DiscountType:     orderDiscountType,
		DiscountValue:    orderDiscountValue,
		DiscountAmount:   decimalToNumeric(orderDiscountAmount),
		TaxAmount:        decimalToNumeric(taxAmount),
		TotalAmount:      decimalToNumeric(totalAmount),
		CateringDate:     cateringDate,
		CateringStatus:   cateringStatus,
		CateringDpAmount: cateringDpAmount,
		DeliveryPlatform: deliveryPlatform,
		DeliveryAddress:  deliveryAddress,
		CreatedBy:        req.CreatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// --- Insert items ---
	var itemResults []OrderItemResult
	for _, pi := range items {
		pi.params.OrderID = order.ID
		item, err := store.CreateOrderItem(ctx, pi.params)
		if err != nil {
			return nil, fmt.Errorf("create order item: %w", err)
		}

		var modResults []database.OrderItemModifier
		for _, mod := range pi.modifiers {
			oim, err := store.CreateOrderItemModifier(ctx, database.CreateOrderItemModifierParams{
				OrderItemID: item.ID,
				ModifierID:  mod.modifierID,
				Quantity:    mod.quantity,
				UnitPrice:   decimalToNumeric(mod.unitPrice),
			})
			if err != nil {
				return nil, fmt.Errorf("create order item modifier: %w", err)
			}
			modResults = append(modResults, oim)
		}

		itemResults = append(itemResults, OrderItemResult{
			Item:      item,
			Modifiers: modResults,
		})
	}

	// --- Commit ---
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &CreateOrderResult{
		Order: order,
		Items: itemResults,
	}, nil
}

// --- Helpers ---

func validateOrderType(s string) (string, error) {
	switch s {
	case enum.OrderTypeDineIn, enum.OrderTypeTakeaway,
		enum.OrderTypeDelivery, enum.OrderTypeCatering:
		return s, nil
	}
	return "", ErrInvalidOrderType
}

func isValidDiscountType(s string) bool {
	switch s {
	case enum.DiscountTypePercentage, enum.DiscountTypeFixed:
		return true
	}
	return false
}

func numericToDecimal(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid {
		return decimal.Zero
	}
	val, err := n.Value()
	if err != nil || val == nil {
		return decimal.Zero
	}
	d, err := decimal.NewFromString(val.(string))
	if err != nil {
		return decimal.Zero
	}
	return d
}

func decimalToNumeric(d decimal.Decimal) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(d.StringFixed(2))
	return n
}
