package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/middleware"
	"github.com/kiwari-pos/api/internal/service"
	"github.com/shopspring/decimal"
)

// OrderServicer defines the service methods needed by order handlers.
// Satisfied by *service.OrderService; narrow interface for testability.
type OrderServicer interface {
	CreateOrder(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error)
}

// OrderStore defines the database methods needed by order read/update handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type OrderStore interface {
	GetOrder(ctx context.Context, arg database.GetOrderParams) (database.Order, error)
	ListOrders(ctx context.Context, arg database.ListOrdersParams) ([]database.Order, error)
	ListOrderItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]database.OrderItem, error)
	ListOrderItemModifiersByOrderItem(ctx context.Context, orderItemID uuid.UUID) ([]database.OrderItemModifier, error)
	ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]database.Payment, error)
	UpdateOrderStatus(ctx context.Context, arg database.UpdateOrderStatusParams) (database.Order, error)
	CancelOrder(ctx context.Context, arg database.CancelOrderParams) (database.Order, error)
	// Item modification methods
	GetOrderItem(ctx context.Context, arg database.GetOrderItemParams) (database.OrderItem, error)
	UpdateOrderItem(ctx context.Context, arg database.UpdateOrderItemParams) (database.OrderItem, error)
	DeleteOrderItem(ctx context.Context, arg database.DeleteOrderItemParams) error
	UpdateOrderItemStatus(ctx context.Context, arg database.UpdateOrderItemStatusParams) (database.OrderItem, error)
	CountOrderItems(ctx context.Context, orderID uuid.UUID) (int64, error)
	UpdateOrderTotals(ctx context.Context, orderID uuid.UUID) (database.Order, error)
	// Product/variant/modifier validation (reused from service layer)
	GetProductForOrder(ctx context.Context, arg database.GetProductForOrderParams) (database.GetProductForOrderRow, error)
	GetVariantForOrder(ctx context.Context, variantID uuid.UUID) (database.GetVariantForOrderRow, error)
	GetModifierForOrder(ctx context.Context, modifierID uuid.UUID) (database.GetModifierForOrderRow, error)
	CreateOrderItem(ctx context.Context, arg database.CreateOrderItemParams) (database.OrderItem, error)
	CreateOrderItemModifier(ctx context.Context, arg database.CreateOrderItemModifierParams) (database.OrderItemModifier, error)
}

// NewOrderStore creates an OrderStore from a DBTX (pool or tx).
// This allows the handler to create store instances from transactions.
type NewOrderStore func(db database.DBTX) OrderStore

// OrderHandler handles order endpoints.
type OrderHandler struct {
	svc      OrderServicer
	store    OrderStore
	pool     service.TxBeginner
	newStore NewOrderStore
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(svc OrderServicer, store OrderStore, pool service.TxBeginner, newStore NewOrderStore) *OrderHandler {
	return &OrderHandler{svc: svc, store: store, pool: pool, newStore: newStore}
}

// RegisterRoutes registers order endpoints on the given Chi router.
// Expected to be mounted inside an outlet-scoped subrouter: /outlets/{oid}/orders
func (h *OrderHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Patch("/status", h.UpdateStatus)
		r.Delete("/", h.Cancel)
		r.Route("/items", func(r chi.Router) {
			r.Post("/", h.AddItem)
			r.Put("/{iid}", h.UpdateItem)
			r.Delete("/{iid}", h.RemoveItem)
			r.Patch("/{iid}/status", h.UpdateItemStatus)
		})
	})
}

// --- Request / Response types ---

type createOrderRequest struct {
	OrderType        string                   `json:"order_type"`
	TableNumber      string                   `json:"table_number"`
	CustomerID       string                   `json:"customer_id"`
	Notes            string                   `json:"notes"`
	DiscountType     string                   `json:"discount_type"`
	DiscountValue    string                   `json:"discount_value"`
	CateringDate     string                   `json:"catering_date"`
	CateringDpAmount string                   `json:"catering_dp_amount"`
	DeliveryPlatform string                   `json:"delivery_platform"`
	DeliveryAddress  string                   `json:"delivery_address"`
	Items            []createOrderItemRequest `json:"items"`
}

type createOrderItemRequest struct {
	ProductID     string                           `json:"product_id"`
	VariantID     string                           `json:"variant_id"`
	Quantity      int32                            `json:"quantity"`
	Notes         string                           `json:"notes"`
	DiscountType  string                           `json:"discount_type"`
	DiscountValue string                           `json:"discount_value"`
	Modifiers     []createOrderItemModifierRequest `json:"modifiers"`
}

type createOrderItemModifierRequest struct {
	ModifierID string `json:"modifier_id"`
	Quantity   int32  `json:"quantity"`
}

type orderResponse struct {
	ID               uuid.UUID           `json:"id"`
	OutletID         uuid.UUID           `json:"outlet_id"`
	OrderNumber      string              `json:"order_number"`
	CustomerID       *string             `json:"customer_id"`
	OrderType        string              `json:"order_type"`
	Status           string              `json:"status"`
	TableNumber      *string             `json:"table_number"`
	Notes            *string             `json:"notes"`
	Subtotal         string              `json:"subtotal"`
	DiscountType     *string             `json:"discount_type"`
	DiscountValue    *string             `json:"discount_value"`
	DiscountAmount   string              `json:"discount_amount"`
	TaxAmount        string              `json:"tax_amount"`
	TotalAmount      string              `json:"total_amount"`
	CateringDate     *time.Time          `json:"catering_date"`
	CateringStatus   *string             `json:"catering_status"`
	CateringDpAmount *string             `json:"catering_dp_amount"`
	DeliveryPlatform *string             `json:"delivery_platform"`
	DeliveryAddress  *string             `json:"delivery_address"`
	CreatedBy        uuid.UUID           `json:"created_by"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	Items            []orderItemResponse `json:"items"`
}

type orderItemResponse struct {
	ID             uuid.UUID                   `json:"id"`
	ProductID      uuid.UUID                   `json:"product_id"`
	VariantID      *string                     `json:"variant_id"`
	Quantity       int32                       `json:"quantity"`
	UnitPrice      string                      `json:"unit_price"`
	DiscountType   *string                     `json:"discount_type"`
	DiscountValue  *string                     `json:"discount_value"`
	DiscountAmount string                      `json:"discount_amount"`
	Subtotal       string                      `json:"subtotal"`
	Notes          *string                     `json:"notes"`
	Status         string                      `json:"status"`
	Station        *string                     `json:"station"`
	Modifiers      []orderItemModifierResponse `json:"modifiers"`
}

type orderItemModifierResponse struct {
	ID         uuid.UUID `json:"id"`
	ModifierID uuid.UUID `json:"modifier_id"`
	Quantity   int32     `json:"quantity"`
	UnitPrice  string    `json:"unit_price"`
}

type paymentResponse struct {
	ID              uuid.UUID `json:"id"`
	OrderID         uuid.UUID `json:"order_id"`
	PaymentMethod   string    `json:"payment_method"`
	Amount          string    `json:"amount"`
	Status          string    `json:"status"`
	ReferenceNumber *string   `json:"reference_number"`
	AmountReceived  *string   `json:"amount_received"`
	ChangeAmount    *string   `json:"change_amount"`
	ProcessedBy     uuid.UUID `json:"processed_by"`
	ProcessedAt     time.Time `json:"processed_at"`
}

// orderDetailResponse extends orderResponse with payments for the GET detail endpoint.
type orderDetailResponse struct {
	orderResponse
	Payments []paymentResponse `json:"payments"`
}

// orderListResponse wraps a list of orders with pagination metadata.
type orderListResponse struct {
	Orders []orderResponse `json:"orders"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type addItemRequest struct {
	ProductID     string                           `json:"product_id"`
	VariantID     string                           `json:"variant_id"`
	Quantity      int32                            `json:"quantity"`
	Notes         string                           `json:"notes"`
	DiscountType  string                           `json:"discount_type"`
	DiscountValue string                           `json:"discount_value"`
	Modifiers     []createOrderItemModifierRequest `json:"modifiers"`
}

type updateItemRequest struct {
	Quantity int32  `json:"quantity"`
	Notes    string `json:"notes"`
}

type updateItemStatusRequest struct {
	Status string `json:"status"`
}

// --- Handlers ---

// Create handles POST /outlets/{oid}/orders.
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.OrderType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order_type is required"})
		return
	}

	if len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items are required"})
		return
	}

	for i, item := range req.Items {
		if item.ProductID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": formatItemError(i, "product_id is required"),
			})
			return
		}
		if item.Quantity <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": formatItemError(i, "quantity must be > 0"),
			})
			return
		}
	}

	// Build service request
	svcItems := make([]service.CreateOrderItemRequest, len(req.Items))
	for i, item := range req.Items {
		mods := make([]service.CreateOrderItemModifierRequest, len(item.Modifiers))
		for j, mod := range item.Modifiers {
			mods[j] = service.CreateOrderItemModifierRequest{
				ModifierID: mod.ModifierID,
				Quantity:   mod.Quantity,
			}
		}
		svcItems[i] = service.CreateOrderItemRequest{
			ProductID:     item.ProductID,
			VariantID:     item.VariantID,
			Quantity:      item.Quantity,
			Notes:         item.Notes,
			DiscountType:  item.DiscountType,
			DiscountValue: item.DiscountValue,
			Modifiers:     mods,
		}
	}

	result, err := h.svc.CreateOrder(r.Context(), service.CreateOrderRequest{
		OutletID:         outletID,
		CreatedBy:        claims.UserID,
		OrderType:        req.OrderType,
		TableNumber:      req.TableNumber,
		CustomerID:       req.CustomerID,
		Notes:            req.Notes,
		DiscountType:     req.DiscountType,
		DiscountValue:    req.DiscountValue,
		CateringDate:     req.CateringDate,
		CateringDpAmount: req.CateringDpAmount,
		DeliveryPlatform: req.DeliveryPlatform,
		DeliveryAddress:  req.DeliveryAddress,
		Items:            svcItems,
	})
	if err != nil {
		// Map known service errors to appropriate HTTP status codes.
		if isValidationError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		log.Printf("ERROR: create order: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toOrderResponse(result))
}

// List handles GET /outlets/{oid}/orders.
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	// Parse pagination
	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if s := r.URL.Query().Get("offset"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 0 {
			offset = v
		}
	}

	// Build query params with optional filters
	params := database.ListOrdersParams{
		OutletID: outletID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	}

	if s := r.URL.Query().Get("status"); s != "" {
		params.Status = database.NullOrderStatus{OrderStatus: database.OrderStatus(s), Valid: true}
	}
	if s := r.URL.Query().Get("type"); s != "" {
		params.OrderType = database.NullOrderType{OrderType: database.OrderType(s), Valid: true}
	}
	if s := r.URL.Query().Get("start_date"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date format, use YYYY-MM-DD"})
			return
		}
		params.StartDate = pgtype.Timestamptz{Time: t, Valid: true}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date format, use YYYY-MM-DD"})
			return
		}
		params.EndDate = pgtype.Timestamptz{Time: t, Valid: true}
	}

	orders, err := h.store.ListOrders(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list orders: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = dbOrderToResponse(o)
	}

	writeJSON(w, http.StatusOK, orderListResponse{
		Orders: resp,
		Limit:  limit,
		Offset: offset,
	})
}

// Get handles GET /outlets/{oid}/orders/{id}.
func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	order, err := h.store.GetOrder(r.Context(), database.GetOrderParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		log.Printf("ERROR: get order: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Fetch items
	items, err := h.store.ListOrderItemsByOrder(r.Context(), orderID)
	if err != nil {
		log.Printf("ERROR: list order items: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Fetch modifiers for each item
	itemResponses := make([]orderItemResponse, len(items))
	for i, item := range items {
		mods, err := h.store.ListOrderItemModifiersByOrderItem(r.Context(), item.ID)
		if err != nil {
			log.Printf("ERROR: list order item modifiers: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		itemResponses[i] = dbOrderItemToResponse(item, mods)
	}

	// Fetch payments
	payments, err := h.store.ListPaymentsByOrder(r.Context(), orderID)
	if err != nil {
		log.Printf("ERROR: list payments: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	paymentResps := make([]paymentResponse, len(payments))
	for i, p := range payments {
		paymentResps[i] = dbPaymentToResponse(p)
	}

	orderResp := dbOrderToResponse(order)
	orderResp.Items = itemResponses

	writeJSON(w, http.StatusOK, orderDetailResponse{
		orderResponse: orderResp,
		Payments:      paymentResps,
	})
}

// UpdateStatus handles PATCH /outlets/{oid}/orders/{id}/status.
func (h *OrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	var req updateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status is required"})
		return
	}

	newStatus := database.OrderStatus(req.Status)
	if !isValidOrderStatus(newStatus) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
		return
	}

	// Fetch current order to validate transition
	current, err := h.store.GetOrder(r.Context(), database.GetOrderParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		log.Printf("ERROR: get order for status update: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := validateStatusTransition(current.Status, newStatus); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	updated, err := h.store.UpdateOrderStatus(r.Context(), database.UpdateOrderStatusParams{
		ID:       orderID,
		OutletID: outletID,
		Status:   newStatus,
		Status_2: current.Status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// If no rows were updated, it means the status changed between our read and write (race condition)
			writeJSON(w, http.StatusConflict, map[string]string{"error": "order status changed, please retry"})
			return
		}
		log.Printf("ERROR: update order status: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, dbOrderToResponse(updated))
}

// Cancel handles DELETE /outlets/{oid}/orders/{id}.
func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	// Attempt to cancel the order. The SQL query enforces the precondition atomically:
	// it will only update if the order exists AND is not COMPLETED or CANCELLED.
	cancelled, err := h.store.CancelOrder(r.Context(), database.CancelOrderParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No rows updated means either: order doesn't exist, or already COMPLETED/CANCELLED.
			// Fetch to give a better error message.
			current, fetchErr := h.store.GetOrder(r.Context(), database.GetOrderParams{
				ID:       orderID,
				OutletID: outletID,
			})
			if fetchErr != nil {
				if errors.Is(fetchErr, pgx.ErrNoRows) {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
					return
				}
				log.Printf("ERROR: get order for cancel: %v", fetchErr)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
				return
			}
			// Order exists but couldn't be cancelled due to status
			if current.Status == database.OrderStatusCOMPLETED {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot cancel a completed order"})
				return
			}
			if current.Status == database.OrderStatusCANCELLED {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "order is already cancelled"})
				return
			}
			// Shouldn't reach here, but just in case
			writeJSON(w, http.StatusConflict, map[string]string{"error": "order cannot be cancelled"})
			return
		}
		log.Printf("ERROR: cancel order: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, dbOrderToResponse(cancelled))
}

// AddItem handles POST /outlets/{oid}/orders/{id}/items.
func (h *OrderHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.ProductID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product_id is required"})
		return
	}
	if req.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "quantity must be > 0"})
		return
	}

	// Verify order exists, belongs to outlet, and is NEW
	order, err := h.store.GetOrder(r.Context(), database.GetOrderParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		log.Printf("ERROR: get order for add item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if order.Status != database.OrderStatusNEW {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "can only add items to NEW orders"})
		return
	}

	// Parse and validate product ID
	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
		return
	}

	// Validate product exists and belongs to outlet
	product, err := h.store.GetProductForOrder(r.Context(), database.GetProductForOrderParams{
		ID:       productID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product not found"})
			return
		}
		log.Printf("ERROR: get product for add item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Calculate base price (product base_price + variant adjustment)
	basePrice := product.BasePrice
	var variantID pgtype.UUID
	if req.VariantID != "" {
		vid, err := uuid.Parse(req.VariantID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid variant_id"})
			return
		}
		variant, err := h.store.GetVariantForOrder(r.Context(), vid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "variant not found"})
				return
			}
			log.Printf("ERROR: get variant for add item: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		if variant.ProductID != productID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "variant does not belong to this product"})
			return
		}
		variantID = pgtype.UUID{Bytes: vid, Valid: true}
		// Add variant price adjustment
		if variant.PriceAdjustment.Valid {
			adj, err := numericToDecimal(variant.PriceAdjustment)
			if err == nil {
				base, _ := numericToDecimal(basePrice)
				basePrice = decimalToNumeric(base.Add(adj))
			}
		}
	}

	// Calculate item subtotal: (base_price * qty) + modifier_prices - item_discount
	basePriceDecimal, _ := numericToDecimal(basePrice)
	itemSubtotal := basePriceDecimal.Mul(decimal.NewFromInt32(req.Quantity))

	// Validate and calculate modifier prices
	// Store validated modifiers to reuse when creating modifier records (Issue 3 fix)
	type validatedModifier struct {
		modifierID uuid.UUID
		quantity   int32
		price      pgtype.Numeric
	}
	validatedModifiers := make([]validatedModifier, 0, len(req.Modifiers))
	var modifierSubtotal decimal.Decimal
	for _, mod := range req.Modifiers {
		if mod.ModifierID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "modifier_id is required"})
			return
		}
		modID, err := uuid.Parse(mod.ModifierID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid modifier_id"})
			return
		}
		modifier, err := h.store.GetModifierForOrder(r.Context(), modID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "modifier not found"})
				return
			}
			log.Printf("ERROR: get modifier for add item: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		if modifier.ProductID != productID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "modifier does not belong to this product"})
			return
		}
		modPrice, _ := numericToDecimal(modifier.Price)
		modifierSubtotal = modifierSubtotal.Add(modPrice.Mul(decimal.NewFromInt32(mod.Quantity)))

		// Store validated modifier data for reuse
		validatedModifiers = append(validatedModifiers, validatedModifier{
			modifierID: modID,
			quantity:   mod.Quantity,
			price:      modifier.Price,
		})
	}

	itemSubtotal = itemSubtotal.Add(modifierSubtotal)

	// Apply item-level discount if provided
	var discountType database.NullDiscountType
	var discountValue pgtype.Numeric
	var discountAmount decimal.Decimal
	if req.DiscountType != "" {
		discountType = database.NullDiscountType{DiscountType: database.DiscountType(req.DiscountType), Valid: true}
		if req.DiscountValue == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "discount_value is required when discount_type is set"})
			return
		}
		dv, err := decimal.NewFromString(req.DiscountValue)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid discount_value"})
			return
		}
		discountValue = decimalToNumeric(dv)
		if database.DiscountType(req.DiscountType) == database.DiscountTypePERCENTAGE {
			discountAmount = itemSubtotal.Mul(dv).Div(decimal.NewFromInt(100))
		} else if database.DiscountType(req.DiscountType) == database.DiscountTypeFIXEDAMOUNT {
			discountAmount = dv
			if discountAmount.GreaterThan(itemSubtotal) {
				discountAmount = itemSubtotal
			}
		}
	}

	itemSubtotal = itemSubtotal.Sub(discountAmount)

	// Begin transaction for atomic multi-write operation
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Printf("ERROR: begin tx for add item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	defer tx.Rollback(r.Context())

	txStore := h.newStore(tx)

	// Create order item
	var notes pgtype.Text
	if req.Notes != "" {
		notes = pgtype.Text{String: req.Notes, Valid: true}
	}

	item, err := txStore.CreateOrderItem(r.Context(), database.CreateOrderItemParams{
		OrderID:        orderID,
		ProductID:      productID,
		VariantID:      variantID,
		Quantity:       req.Quantity,
		UnitPrice:      basePrice,
		DiscountType:   discountType,
		DiscountValue:  discountValue,
		DiscountAmount: decimalToNumeric(discountAmount),
		Subtotal:       decimalToNumeric(itemSubtotal),
		Notes:          notes,
		Station:        product.Station,
	})
	if err != nil {
		log.Printf("ERROR: create order item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Create order item modifiers using validated data (no redundant DB calls)
	modifiers := make([]database.OrderItemModifier, 0, len(validatedModifiers))
	for _, mod := range validatedModifiers {
		itemMod, err := txStore.CreateOrderItemModifier(r.Context(), database.CreateOrderItemModifierParams{
			OrderItemID: item.ID,
			ModifierID:  mod.modifierID,
			Quantity:    mod.quantity,
			UnitPrice:   mod.price,
		})
		if err != nil {
			log.Printf("ERROR: create order item modifier: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		modifiers = append(modifiers, itemMod)
	}

	// Recalculate order totals
	updatedOrder, err := txStore.UpdateOrderTotals(r.Context(), orderID)
	if err != nil {
		log.Printf("ERROR: update order totals: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Commit transaction
	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("ERROR: commit tx for add item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Return the added item with modifiers
	itemResp := dbOrderItemToResponse(item, modifiers)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"item":  itemResp,
		"order": dbOrderToResponse(updatedOrder),
	})
}

// UpdateItem handles PUT /outlets/{oid}/orders/{id}/items/{iid}.
func (h *OrderHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	itemID, err := uuid.Parse(chi.URLParam(r, "iid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item ID"})
		return
	}

	var req updateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "quantity must be > 0"})
		return
	}

	// Verify order exists, belongs to outlet, and is NEW
	order, err := h.store.GetOrder(r.Context(), database.GetOrderParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		log.Printf("ERROR: get order for update item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if order.Status != database.OrderStatusNEW {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "can only update items in NEW orders"})
		return
	}

	// Get current item to recalculate subtotal
	currentItem, err := h.store.GetOrderItem(r.Context(), database.GetOrderItemParams{
		ID:      itemID,
		OrderID: orderID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
		log.Printf("ERROR: get order item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Recalculate item subtotal based on new quantity
	// Formula: (unit_price * new_qty) + modifier_prices - item_discount
	unitPrice, _ := numericToDecimal(currentItem.UnitPrice)
	newSubtotalBeforeDiscount := unitPrice.Mul(decimal.NewFromInt32(req.Quantity))

	// Add modifier prices (they don't change with quantity, they're already unit-based)
	modifiers, err := h.store.ListOrderItemModifiersByOrderItem(r.Context(), itemID)
	if err != nil {
		log.Printf("ERROR: list order item modifiers: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	for _, mod := range modifiers {
		modPrice, _ := numericToDecimal(mod.UnitPrice)
		newSubtotalBeforeDiscount = newSubtotalBeforeDiscount.Add(modPrice.Mul(decimal.NewFromInt32(mod.Quantity)))
	}

	// Recalculate discount amount based on new subtotal
	var discountAmount decimal.Decimal
	if currentItem.DiscountType.Valid {
		discountValue, _ := numericToDecimal(currentItem.DiscountValue)
		if currentItem.DiscountType.DiscountType == database.DiscountTypePERCENTAGE {
			discountAmount = newSubtotalBeforeDiscount.Mul(discountValue).Div(decimal.NewFromInt(100))
		} else if currentItem.DiscountType.DiscountType == database.DiscountTypeFIXEDAMOUNT {
			discountAmount = discountValue
			if discountAmount.GreaterThan(newSubtotalBeforeDiscount) {
				discountAmount = newSubtotalBeforeDiscount
			}
		}
	}

	newSubtotal := newSubtotalBeforeDiscount.Sub(discountAmount)

	// Begin transaction for atomic multi-write operation
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Printf("ERROR: begin tx for update item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	defer tx.Rollback(r.Context())

	txStore := h.newStore(tx)

	// Update notes
	var notes pgtype.Text
	if req.Notes != "" {
		notes = pgtype.Text{String: req.Notes, Valid: true}
	}

	// Update item
	updatedItem, err := txStore.UpdateOrderItem(r.Context(), database.UpdateOrderItemParams{
		ID:             itemID,
		OrderID:        orderID,
		Quantity:       req.Quantity,
		Notes:          notes,
		DiscountAmount: decimalToNumeric(discountAmount),
		Subtotal:       decimalToNumeric(newSubtotal),
	})
	if err != nil {
		log.Printf("ERROR: update order item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Recalculate order totals
	updatedOrder, err := txStore.UpdateOrderTotals(r.Context(), orderID)
	if err != nil {
		log.Printf("ERROR: update order totals: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Commit transaction
	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("ERROR: commit tx for update item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Return updated item with modifiers
	itemResp := dbOrderItemToResponse(updatedItem, modifiers)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"item":  itemResp,
		"order": dbOrderToResponse(updatedOrder),
	})
}

// RemoveItem handles DELETE /outlets/{oid}/orders/{id}/items/{iid}.
func (h *OrderHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	itemID, err := uuid.Parse(chi.URLParam(r, "iid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item ID"})
		return
	}

	// Verify order exists, belongs to outlet, and is NEW
	order, err := h.store.GetOrder(r.Context(), database.GetOrderParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		log.Printf("ERROR: get order for remove item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if order.Status != database.OrderStatusNEW {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "can only remove items from NEW orders"})
		return
	}

	// Check if this is the last item - prevent removing it
	itemCount, err := h.store.CountOrderItems(r.Context(), orderID)
	if err != nil {
		log.Printf("ERROR: count order items: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if itemCount <= 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot remove the last item from an order"})
		return
	}

	// Verify item exists and belongs to order
	_, err = h.store.GetOrderItem(r.Context(), database.GetOrderItemParams{
		ID:      itemID,
		OrderID: orderID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
		log.Printf("ERROR: get order item for remove: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Begin transaction for atomic multi-write operation
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Printf("ERROR: begin tx for remove item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	defer tx.Rollback(r.Context())

	txStore := h.newStore(tx)

	// Delete item (CASCADE will handle modifiers)
	err = txStore.DeleteOrderItem(r.Context(), database.DeleteOrderItemParams{
		ID:      itemID,
		OrderID: orderID,
	})
	if err != nil {
		log.Printf("ERROR: delete order item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Recalculate order totals
	updatedOrder, err := txStore.UpdateOrderTotals(r.Context(), orderID)
	if err != nil {
		log.Printf("ERROR: update order totals: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Commit transaction
	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("ERROR: commit tx for remove item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "item removed successfully",
		"order":   dbOrderToResponse(updatedOrder),
	})
}

// UpdateItemStatus handles PATCH /outlets/{oid}/orders/{id}/items/{iid}/status.
func (h *OrderHandler) UpdateItemStatus(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	itemID, err := uuid.Parse(chi.URLParam(r, "iid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item ID"})
		return
	}

	var req updateItemStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status is required"})
		return
	}

	newStatus := database.OrderItemStatus(req.Status)
	if !isValidItemStatus(newStatus) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
		return
	}

	// Verify order exists and belongs to outlet
	order, err := h.store.GetOrder(r.Context(), database.GetOrderParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		log.Printf("ERROR: get order for item status update: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Cannot update items on cancelled or completed orders
	if order.Status == database.OrderStatusCANCELLED || order.Status == database.OrderStatusCOMPLETED {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot update items on a " + string(order.Status) + " order"})
		return
	}

	// Get current item to validate transition
	currentItem, err := h.store.GetOrderItem(r.Context(), database.GetOrderItemParams{
		ID:      itemID,
		OrderID: orderID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
		log.Printf("ERROR: get order item for status update: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Validate status transition
	if err := validateItemStatusTransition(currentItem.Status, newStatus); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	// Update item status
	updatedItem, err := h.store.UpdateOrderItemStatus(r.Context(), database.UpdateOrderItemStatusParams{
		ID:      itemID,
		OrderID: orderID,
		Status:  newStatus,
	})
	if err != nil {
		log.Printf("ERROR: update order item status: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Get modifiers for response
	modifiers, err := h.store.ListOrderItemModifiersByOrderItem(r.Context(), itemID)
	if err != nil {
		log.Printf("ERROR: list order item modifiers: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, dbOrderItemToResponse(updatedItem, modifiers))
}

// --- Helpers ---

func formatItemError(idx int, msg string) string {
	return "items[" + strconv.Itoa(idx) + "]: " + msg
}

// isValidationError checks if the error is a known validation error
// from the service layer that should result in 400 Bad Request.
func isValidationError(err error) bool {
	return errors.Is(err, service.ErrEmptyItems) ||
		errors.Is(err, service.ErrInvalidOrderType) ||
		errors.Is(err, service.ErrInvalidQuantity) ||
		errors.Is(err, service.ErrProductNotFound) ||
		errors.Is(err, service.ErrVariantNotFound) ||
		errors.Is(err, service.ErrVariantMismatch) ||
		errors.Is(err, service.ErrModifierNotFound) ||
		errors.Is(err, service.ErrModifierMismatch) ||
		errors.Is(err, service.ErrCateringDate) ||
		errors.Is(err, service.ErrCateringCustomer) ||
		errors.Is(err, service.ErrInvalidDiscount) ||
		errors.Is(err, service.ErrInvalidProductID) ||
		errors.Is(err, service.ErrInvalidVariantID) ||
		errors.Is(err, service.ErrInvalidModifierID) ||
		errors.Is(err, service.ErrInvalidDiscountValue) ||
		errors.Is(err, service.ErrInvalidCustomerID) ||
		errors.Is(err, service.ErrInvalidCateringDate) ||
		errors.Is(err, service.ErrInvalidCateringDpAmt)
}

func toOrderResponse(result *service.CreateOrderResult) orderResponse {
	o := result.Order
	resp := orderResponse{
		ID:             o.ID,
		OutletID:       o.OutletID,
		OrderNumber:    o.OrderNumber,
		OrderType:      string(o.OrderType),
		Status:         string(o.Status),
		Subtotal:       numericToString(o.Subtotal),
		DiscountAmount: numericToString(o.DiscountAmount),
		TaxAmount:      numericToString(o.TaxAmount),
		TotalAmount:    numericToString(o.TotalAmount),
		CreatedBy:      o.CreatedBy,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}

	if o.CustomerID.Valid {
		s := uuid.UUID(o.CustomerID.Bytes).String()
		resp.CustomerID = &s
	}
	if o.TableNumber.Valid {
		resp.TableNumber = &o.TableNumber.String
	}
	if o.Notes.Valid {
		resp.Notes = &o.Notes.String
	}
	if o.DiscountType.Valid {
		s := string(o.DiscountType.DiscountType)
		resp.DiscountType = &s
	}
	if o.DiscountValue.Valid {
		s := numericToString(o.DiscountValue)
		resp.DiscountValue = &s
	}
	if o.CateringDate.Valid {
		resp.CateringDate = &o.CateringDate.Time
	}
	if o.CateringStatus.Valid {
		s := string(o.CateringStatus.CateringStatus)
		resp.CateringStatus = &s
	}
	if o.CateringDpAmount.Valid {
		s := numericToString(o.CateringDpAmount)
		resp.CateringDpAmount = &s
	}
	if o.DeliveryPlatform.Valid {
		resp.DeliveryPlatform = &o.DeliveryPlatform.String
	}
	if o.DeliveryAddress.Valid {
		resp.DeliveryAddress = &o.DeliveryAddress.String
	}

	resp.Items = make([]orderItemResponse, len(result.Items))
	for i, ir := range result.Items {
		resp.Items[i] = toOrderItemResponse(ir)
	}

	return resp
}

func toOrderItemResponse(ir service.OrderItemResult) orderItemResponse {
	item := ir.Item
	resp := orderItemResponse{
		ID:             item.ID,
		ProductID:      item.ProductID,
		Quantity:       item.Quantity,
		UnitPrice:      numericToString(item.UnitPrice),
		DiscountAmount: numericToString(item.DiscountAmount),
		Subtotal:       numericToString(item.Subtotal),
		Status:         string(item.Status),
	}

	if item.VariantID.Valid {
		s := uuid.UUID(item.VariantID.Bytes).String()
		resp.VariantID = &s
	}
	if item.DiscountType.Valid {
		s := string(item.DiscountType.DiscountType)
		resp.DiscountType = &s
	}
	if item.DiscountValue.Valid {
		s := numericToString(item.DiscountValue)
		resp.DiscountValue = &s
	}
	if item.Notes.Valid {
		resp.Notes = &item.Notes.String
	}
	if item.Station.Valid {
		s := string(item.Station.KitchenStation)
		resp.Station = &s
	}

	resp.Modifiers = make([]orderItemModifierResponse, len(ir.Modifiers))
	for j, mod := range ir.Modifiers {
		resp.Modifiers[j] = orderItemModifierResponse{
			ID:         mod.ID,
			ModifierID: mod.ModifierID,
			Quantity:   mod.Quantity,
			UnitPrice:  numericToString(mod.UnitPrice),
		}
	}

	return resp
}

func numericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0.00"
	}
	val, err := n.Value()
	if err != nil || val == nil {
		return "0.00"
	}
	d, err := decimal.NewFromString(val.(string))
	if err != nil {
		return "0.00"
	}
	return d.StringFixed(2)
}

// dbOrderToResponse converts a database.Order to an orderResponse.
// Unlike toOrderResponse which takes a service.CreateOrderResult, this works
// directly with the DB model for the read endpoints.
func dbOrderToResponse(o database.Order) orderResponse {
	resp := orderResponse{
		ID:             o.ID,
		OutletID:       o.OutletID,
		OrderNumber:    o.OrderNumber,
		OrderType:      string(o.OrderType),
		Status:         string(o.Status),
		Subtotal:       numericToString(o.Subtotal),
		DiscountAmount: numericToString(o.DiscountAmount),
		TaxAmount:      numericToString(o.TaxAmount),
		TotalAmount:    numericToString(o.TotalAmount),
		CreatedBy:      o.CreatedBy,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}

	if o.CustomerID.Valid {
		s := uuid.UUID(o.CustomerID.Bytes).String()
		resp.CustomerID = &s
	}
	if o.TableNumber.Valid {
		resp.TableNumber = &o.TableNumber.String
	}
	if o.Notes.Valid {
		resp.Notes = &o.Notes.String
	}
	if o.DiscountType.Valid {
		s := string(o.DiscountType.DiscountType)
		resp.DiscountType = &s
	}
	if o.DiscountValue.Valid {
		s := numericToString(o.DiscountValue)
		resp.DiscountValue = &s
	}
	if o.CateringDate.Valid {
		resp.CateringDate = &o.CateringDate.Time
	}
	if o.CateringStatus.Valid {
		s := string(o.CateringStatus.CateringStatus)
		resp.CateringStatus = &s
	}
	if o.CateringDpAmount.Valid {
		s := numericToString(o.CateringDpAmount)
		resp.CateringDpAmount = &s
	}
	if o.DeliveryPlatform.Valid {
		resp.DeliveryPlatform = &o.DeliveryPlatform.String
	}
	if o.DeliveryAddress.Valid {
		resp.DeliveryAddress = &o.DeliveryAddress.String
	}

	return resp
}

// dbOrderItemToResponse converts a database.OrderItem and its modifiers to an orderItemResponse.
func dbOrderItemToResponse(item database.OrderItem, mods []database.OrderItemModifier) orderItemResponse {
	resp := orderItemResponse{
		ID:             item.ID,
		ProductID:      item.ProductID,
		Quantity:       item.Quantity,
		UnitPrice:      numericToString(item.UnitPrice),
		DiscountAmount: numericToString(item.DiscountAmount),
		Subtotal:       numericToString(item.Subtotal),
		Status:         string(item.Status),
	}

	if item.VariantID.Valid {
		s := uuid.UUID(item.VariantID.Bytes).String()
		resp.VariantID = &s
	}
	if item.DiscountType.Valid {
		s := string(item.DiscountType.DiscountType)
		resp.DiscountType = &s
	}
	if item.DiscountValue.Valid {
		s := numericToString(item.DiscountValue)
		resp.DiscountValue = &s
	}
	if item.Notes.Valid {
		resp.Notes = &item.Notes.String
	}
	if item.Station.Valid {
		s := string(item.Station.KitchenStation)
		resp.Station = &s
	}

	resp.Modifiers = make([]orderItemModifierResponse, len(mods))
	for j, mod := range mods {
		resp.Modifiers[j] = orderItemModifierResponse{
			ID:         mod.ID,
			ModifierID: mod.ModifierID,
			Quantity:   mod.Quantity,
			UnitPrice:  numericToString(mod.UnitPrice),
		}
	}

	return resp
}

// dbPaymentToResponse converts a database.Payment to a paymentResponse.
func dbPaymentToResponse(p database.Payment) paymentResponse {
	resp := paymentResponse{
		ID:            p.ID,
		OrderID:       p.OrderID,
		PaymentMethod: string(p.PaymentMethod),
		Amount:        numericToString(p.Amount),
		Status:        string(p.Status),
		ProcessedBy:   p.ProcessedBy,
		ProcessedAt:   p.ProcessedAt,
	}
	if p.ReferenceNumber.Valid {
		resp.ReferenceNumber = &p.ReferenceNumber.String
	}
	if p.AmountReceived.Valid {
		s := numericToString(p.AmountReceived)
		resp.AmountReceived = &s
	}
	if p.ChangeAmount.Valid {
		s := numericToString(p.ChangeAmount)
		resp.ChangeAmount = &s
	}
	return resp
}

// isValidOrderStatus checks if the given status is a valid order status.
func isValidOrderStatus(s database.OrderStatus) bool {
	switch s {
	case database.OrderStatusNEW,
		database.OrderStatusPREPARING,
		database.OrderStatusREADY,
		database.OrderStatusCOMPLETED,
		database.OrderStatusCANCELLED:
		return true
	}
	return false
}

// allowedTransitions defines valid status transitions.
// Key is current status, value is the set of statuses it can transition to.
var allowedTransitions = map[database.OrderStatus][]database.OrderStatus{
	database.OrderStatusNEW:       {database.OrderStatusPREPARING, database.OrderStatusCANCELLED},
	database.OrderStatusPREPARING: {database.OrderStatusREADY, database.OrderStatusCANCELLED},
	database.OrderStatusREADY:     {database.OrderStatusCOMPLETED, database.OrderStatusCANCELLED},
}

// validateStatusTransition checks if the transition from current to next is allowed.
func validateStatusTransition(current, next database.OrderStatus) error {
	allowed, ok := allowedTransitions[current]
	if !ok {
		return fmt.Errorf("cannot transition from %s", current)
	}
	for _, s := range allowed {
		if s == next {
			return nil
		}
	}
	return fmt.Errorf("cannot transition from %s to %s", current, next)
}

// isValidItemStatus checks if the given status is a valid order item status.
func isValidItemStatus(s database.OrderItemStatus) bool {
	switch s {
	case database.OrderItemStatusPENDING,
		database.OrderItemStatusPREPARING,
		database.OrderItemStatusREADY:
		return true
	}
	return false
}

// allowedItemTransitions defines valid item status transitions.
var allowedItemTransitions = map[database.OrderItemStatus][]database.OrderItemStatus{
	database.OrderItemStatusPENDING:   {database.OrderItemStatusPREPARING},
	database.OrderItemStatusPREPARING: {database.OrderItemStatusREADY},
	// READY is terminal state for items
}

// validateItemStatusTransition checks if the transition from current to next is allowed.
func validateItemStatusTransition(current, next database.OrderItemStatus) error {
	allowed, ok := allowedItemTransitions[current]
	if !ok {
		return fmt.Errorf("cannot transition from %s", current)
	}
	for _, s := range allowed {
		if s == next {
			return nil
		}
	}
	return fmt.Errorf("cannot transition from %s to %s", current, next)
}

// numericToDecimal converts pgtype.Numeric to decimal.Decimal
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

// decimalToNumeric converts decimal.Decimal to pgtype.Numeric
func decimalToNumeric(d decimal.Decimal) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(d.String())
	return n
}
