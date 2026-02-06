package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/middleware"
	"github.com/kiwari-pos/api/internal/service"
	"github.com/shopspring/decimal"
)

// OrderServicer defines the service methods needed by order handlers.
// Satisfied by *service.OrderService; narrow interface for testability.
type OrderServicer interface {
	CreateOrder(ctx context.Context, req service.CreateOrderRequest) (*service.CreateOrderResult, error)
}

// OrderHandler handles order endpoints.
type OrderHandler struct {
	svc OrderServicer
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(svc OrderServicer) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// RegisterRoutes registers order endpoints on the given Chi router.
// Expected to be mounted inside an outlet-scoped subrouter: /outlets/{oid}/orders
func (h *OrderHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Create)
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
