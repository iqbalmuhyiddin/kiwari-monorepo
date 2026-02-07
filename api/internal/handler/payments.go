package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/middleware"
	"github.com/kiwari-pos/api/internal/service"
	"github.com/shopspring/decimal"
)

// PaymentStore defines the database methods needed by payment handlers.
type PaymentStore interface {
	GetOrder(ctx context.Context, arg database.GetOrderParams) (database.Order, error)
	GetOrderForUpdate(ctx context.Context, arg database.GetOrderForUpdateParams) (database.Order, error)
	ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]database.Payment, error)
	CreatePayment(ctx context.Context, arg database.CreatePaymentParams) (database.Payment, error)
	SumPaymentsByOrder(ctx context.Context, orderID uuid.UUID) (pgtype.Numeric, error)
	CompleteOrder(ctx context.Context, id uuid.UUID) (database.Order, error)
	UpdateCateringStatus(ctx context.Context, arg database.UpdateCateringStatusParams) (database.Order, error)
}

// NewPaymentStore creates a PaymentStore from a DBTX (pool or tx).
type NewPaymentStore func(db database.DBTX) PaymentStore

// PaymentHandler handles payment endpoints.
type PaymentHandler struct {
	store    PaymentStore
	pool     service.TxBeginner
	newStore NewPaymentStore
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(store PaymentStore, pool service.TxBeginner, newStore NewPaymentStore) *PaymentHandler {
	return &PaymentHandler{store: store, pool: pool, newStore: newStore}
}

// RegisterRoutes registers payment endpoints on the given Chi router.
// Expected to be mounted at /outlets/{oid}/orders/{id}/payments
func (h *PaymentHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Add)
	r.Get("/", h.List)
}

// --- Request / Response types ---

type addPaymentRequest struct {
	PaymentMethod   string `json:"payment_method"`
	Amount          string `json:"amount"`
	AmountReceived  string `json:"amount_received"`
	ReferenceNumber string `json:"reference_number"`
}

// --- Handlers ---

// Add handles POST /outlets/{oid}/orders/{id}/payments.
func (h *PaymentHandler) Add(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req addPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate payment method
	if req.PaymentMethod == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_method is required"})
		return
	}
	paymentMethod := database.PaymentMethod(req.PaymentMethod)
	if !isValidPaymentMethod(paymentMethod) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment_method"})
		return
	}

	// Validate and parse amount
	if req.Amount == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount is required"})
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount must be positive"})
		return
	}

	// For CASH payments, validate amount_received
	var amountReceived pgtype.Numeric
	var changeAmount pgtype.Numeric
	if paymentMethod == database.PaymentMethodCASH {
		if req.AmountReceived == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount_received is required for CASH payments"})
			return
		}
		received, err := decimal.NewFromString(req.AmountReceived)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount_received"})
			return
		}
		if received.LessThan(amount) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount_received must be >= amount"})
			return
		}
		amountReceived = decimalToNumeric(received)
		change := received.Sub(amount)
		changeAmount = decimalToNumeric(change)
	}

	// Optional reference number for QRIS/TRANSFER
	var referenceNumber pgtype.Text
	if req.ReferenceNumber != "" {
		referenceNumber = pgtype.Text{String: req.ReferenceNumber, Valid: true}
	}

	// Begin transaction BEFORE reading order state to prevent TOCTOU races.
	// Two concurrent payments could both pass validation outside a tx, causing overpayment.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Printf("ERROR: begin tx for add payment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	defer tx.Rollback(r.Context())

	txStore := h.newStore(tx)

	// Lock the order row (FOR NO KEY UPDATE) to serialize concurrent payment inserts
	order, err := txStore.GetOrderForUpdate(r.Context(), database.GetOrderForUpdateParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		log.Printf("ERROR: get order for add payment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Cannot add payment to CANCELLED orders
	if order.Status == database.OrderStatusCANCELLED {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot add payment to cancelled order"})
		return
	}

	// Check if order is already fully paid
	totalPaid, err := txStore.SumPaymentsByOrder(r.Context(), orderID)
	if err != nil {
		log.Printf("ERROR: sum payments: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	totalPaidDecimal, _ := numericToDecimal(totalPaid)
	orderTotal, _ := numericToDecimal(order.TotalAmount)

	if totalPaidDecimal.GreaterThanOrEqual(orderTotal) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "order is already fully paid"})
		return
	}

	// Validate that this payment doesn't cause overpayment
	newTotalPaid := totalPaidDecimal.Add(amount)
	if newTotalPaid.GreaterThan(orderTotal) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "payment exceeds remaining balance"})
		return
	}

	// Create payment
	payment, err := txStore.CreatePayment(r.Context(), database.CreatePaymentParams{
		OrderID:         orderID,
		PaymentMethod:   paymentMethod,
		Amount:          decimalToNumeric(amount),
		Status:          database.PaymentStatusCOMPLETED,
		ReferenceNumber: referenceNumber,
		AmountReceived:  amountReceived,
		ChangeAmount:    changeAmount,
		ProcessedBy:     claims.UserID,
	})
	if err != nil {
		log.Printf("ERROR: create payment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Get updated order (will be modified by catering status or completion)
	updatedOrder := order

	// Handle catering lifecycle
	if order.OrderType == database.OrderTypeCATERING {
		// First payment for catering: BOOKED -> DP_PAID
		if order.CateringStatus.Valid && order.CateringStatus.CateringStatus == database.CateringStatusBOOKED {
			// Check if this is the first payment
			if totalPaidDecimal.Equal(decimal.Zero) {
				updatedOrder, err = txStore.UpdateCateringStatus(r.Context(), database.UpdateCateringStatusParams{
					ID: orderID,
					CateringStatus: database.NullCateringStatus{
						CateringStatus: database.CateringStatusDPPAID,
						Valid:          true,
					},
				})
				if err != nil {
					log.Printf("ERROR: update catering status to DP_PAID: %v", err)
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
					return
				}
			}
		}

		// Full payment for catering: -> SETTLED
		if newTotalPaid.GreaterThanOrEqual(orderTotal) {
			updatedOrder, err = txStore.UpdateCateringStatus(r.Context(), database.UpdateCateringStatusParams{
				ID: orderID,
				CateringStatus: database.NullCateringStatus{
					CateringStatus: database.CateringStatusSETTLED,
					Valid:          true,
				},
			})
			if err != nil {
				log.Printf("ERROR: update catering status to SETTLED: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
				return
			}
		}
	}

	// Auto-complete order if fully paid (only if status is NEW, PREPARING, or READY)
	if newTotalPaid.GreaterThanOrEqual(orderTotal) {
		if order.Status == database.OrderStatusNEW ||
			order.Status == database.OrderStatusPREPARING ||
			order.Status == database.OrderStatusREADY {
			updatedOrder, err = txStore.CompleteOrder(r.Context(), orderID)
			if err != nil {
				// If CompleteOrder returns no rows, it means the order was already CANCELLED
				// This shouldn't happen because we checked above, but handle it gracefully
				if !errors.Is(err, pgx.ErrNoRows) {
					log.Printf("ERROR: complete order: %v", err)
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
					return
				}
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("ERROR: commit tx for add payment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Return the created payment with updated order info
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"payment": dbPaymentToResponse(payment),
		"order":   dbOrderToResponse(updatedOrder),
	})
}

// List handles GET /outlets/{oid}/orders/{id}/payments.
func (h *PaymentHandler) List(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order ID"})
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	// Verify order exists and belongs to outlet
	_, err = h.store.GetOrder(r.Context(), database.GetOrderParams{
		ID:       orderID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		log.Printf("ERROR: get order for list payments: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// List payments for the order
	payments, err := h.store.ListPaymentsByOrder(r.Context(), orderID)
	if err != nil {
		log.Printf("ERROR: list payments: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]paymentResponse, len(payments))
	for i, p := range payments {
		resp[i] = dbPaymentToResponse(p)
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Helpers ---

// isValidPaymentMethod checks if the given payment method is valid.
func isValidPaymentMethod(pm database.PaymentMethod) bool {
	switch pm {
	case database.PaymentMethodCASH,
		database.PaymentMethodQRIS,
		database.PaymentMethodTRANSFER:
		return true
	}
	return false
}
