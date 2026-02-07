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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
)

// CustomerStore defines the database methods needed by customer handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type CustomerStore interface {
	ListCustomersByOutlet(ctx context.Context, arg database.ListCustomersByOutletParams) ([]database.Customer, error)
	GetCustomer(ctx context.Context, arg database.GetCustomerParams) (database.Customer, error)
	CreateCustomer(ctx context.Context, arg database.CreateCustomerParams) (database.Customer, error)
	UpdateCustomer(ctx context.Context, arg database.UpdateCustomerParams) (database.Customer, error)
	SoftDeleteCustomer(ctx context.Context, arg database.SoftDeleteCustomerParams) (uuid.UUID, error)
	GetCustomerStats(ctx context.Context, arg database.GetCustomerStatsParams) (database.GetCustomerStatsRow, error)
	GetCustomerTopItems(ctx context.Context, arg database.GetCustomerTopItemsParams) ([]database.GetCustomerTopItemsRow, error)
	ListCustomerOrders(ctx context.Context, arg database.ListCustomerOrdersParams) ([]database.Order, error)
}

// CustomerHandler handles customer CRUD endpoints.
type CustomerHandler struct {
	store CustomerStore
}

// NewCustomerHandler creates a new CustomerHandler.
func NewCustomerHandler(store CustomerStore) *CustomerHandler {
	return &CustomerHandler{store: store}
}

// RegisterRoutes registers customer CRUD endpoints on the given Chi router.
// Expected to be mounted inside an outlet-scoped subrouter: /outlets/{oid}/customers
func (h *CustomerHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Put("/", h.Update)
		r.Delete("/", h.Delete)
		r.Get("/stats", h.Stats)
		r.Get("/orders", h.Orders)
	})
}

// --- Request / Response types ---

type createCustomerRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
	Notes string `json:"notes"`
}

type updateCustomerRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
	Notes string `json:"notes"`
}

type customerResponse struct {
	ID        uuid.UUID `json:"id"`
	OutletID  uuid.UUID `json:"outlet_id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     *string   `json:"email"`
	Notes     *string   `json:"notes"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type customerStatsResponse struct {
	TotalOrders int64             `json:"total_orders"`
	TotalSpend  string            `json:"total_spend"`
	AvgTicket   string            `json:"avg_ticket"`
	TopItems    []topItemResponse `json:"top_items"`
}

type topItemResponse struct {
	ProductID    uuid.UUID `json:"product_id"`
	ProductName  string    `json:"product_name"`
	TotalQty     int64     `json:"total_qty"`
	TotalRevenue string    `json:"total_revenue"`
}

func toCustomerResponse(c database.Customer) customerResponse {
	resp := customerResponse{
		ID:        c.ID,
		OutletID:  c.OutletID,
		Name:      c.Name,
		Phone:     c.Phone,
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if c.Email.Valid {
		resp.Email = &c.Email.String
	}
	if c.Notes.Valid {
		resp.Notes = &c.Notes.String
	}
	return resp
}

// --- Handlers ---

// List returns all active customers for the given outlet, with optional search.
func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
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

	// Parse search parameter
	var search pgtype.Text
	if s := r.URL.Query().Get("search"); s != "" {
		search = pgtype.Text{String: s, Valid: true}
	}

	customers, err := h.store.ListCustomersByOutlet(r.Context(), database.ListCustomersByOutletParams{
		OutletID: outletID,
		Limit:    int32(limit),
		Offset:   int32(offset),
		Search:   search,
	})
	if err != nil {
		log.Printf("ERROR: list customers: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]customerResponse, len(customers))
	for i, c := range customers {
		resp[i] = toCustomerResponse(c)
	}

	writeJSON(w, http.StatusOK, resp)
}

// Get returns a single customer by ID.
func (h *CustomerHandler) Get(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid customer ID"})
		return
	}

	customer, err := h.store.GetCustomer(r.Context(), database.GetCustomerParams{
		ID:       customerID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "customer not found"})
			return
		}
		log.Printf("ERROR: get customer: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toCustomerResponse(customer))
}

// Create adds a new customer to the given outlet.
func (h *CustomerHandler) Create(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	var req createCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.Phone == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone is required"})
		return
	}

	var email pgtype.Text
	if req.Email != "" {
		email = pgtype.Text{String: req.Email, Valid: true}
	}

	var notes pgtype.Text
	if req.Notes != "" {
		notes = pgtype.Text{String: req.Notes, Valid: true}
	}

	customer, err := h.store.CreateCustomer(r.Context(), database.CreateCustomerParams{
		OutletID: outletID,
		Name:     req.Name,
		Phone:    req.Phone,
		Email:    email,
		Notes:    notes,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "phone already exists for this outlet"})
			return
		}
		log.Printf("ERROR: create customer: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toCustomerResponse(customer))
}

// Update modifies an existing customer in the given outlet.
func (h *CustomerHandler) Update(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid customer ID"})
		return
	}

	var req updateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.Phone == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone is required"})
		return
	}

	var email pgtype.Text
	if req.Email != "" {
		email = pgtype.Text{String: req.Email, Valid: true}
	}

	var notes pgtype.Text
	if req.Notes != "" {
		notes = pgtype.Text{String: req.Notes, Valid: true}
	}

	customer, err := h.store.UpdateCustomer(r.Context(), database.UpdateCustomerParams{
		ID:       customerID,
		Name:     req.Name,
		Phone:    req.Phone,
		Email:    email,
		Notes:    notes,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "customer not found"})
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "phone already exists for this outlet"})
			return
		}
		log.Printf("ERROR: update customer: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toCustomerResponse(customer))
}

// Delete soft-deletes a customer by setting is_active=false.
func (h *CustomerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid customer ID"})
		return
	}

	_, err = h.store.SoftDeleteCustomer(r.Context(), database.SoftDeleteCustomerParams{
		ID:       customerID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "customer not found"})
			return
		}
		log.Printf("ERROR: delete customer: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Stats returns derived CRM statistics for a customer.
func (h *CustomerHandler) Stats(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid customer ID"})
		return
	}

	// Verify customer exists and belongs to outlet
	_, err = h.store.GetCustomer(r.Context(), database.GetCustomerParams{
		ID:       customerID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "customer not found"})
			return
		}
		log.Printf("ERROR: get customer for stats: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Get customer stats (scoped to outlet)
	customerUUID := pgtype.UUID{Bytes: customerID, Valid: true}
	stats, err := h.store.GetCustomerStats(r.Context(), database.GetCustomerStatsParams{
		CustomerID: customerUUID,
		OutletID:   outletID,
	})
	if err != nil {
		log.Printf("ERROR: get customer stats: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Get top items (scoped to outlet)
	topItems, err := h.store.GetCustomerTopItems(r.Context(), database.GetCustomerTopItemsParams{
		CustomerID: customerUUID,
		OutletID:   outletID,
	})
	if err != nil {
		log.Printf("ERROR: get customer top items: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	topItemsResp := make([]topItemResponse, len(topItems))
	for i, item := range topItems {
		topItemsResp[i] = topItemResponse{
			ProductID:    item.ProductID,
			ProductName:  item.ProductName,
			TotalQty:     item.TotalQty,
			TotalRevenue: numericToString(item.TotalRevenue),
		}
	}

	writeJSON(w, http.StatusOK, customerStatsResponse{
		TotalOrders: stats.TotalOrders,
		TotalSpend:  numericToString(stats.TotalSpend),
		AvgTicket:   numericToString(stats.AvgTicket),
		TopItems:    topItemsResp,
	})
}

// Orders returns order history for a customer.
func (h *CustomerHandler) Orders(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid customer ID"})
		return
	}

	// Verify customer exists and belongs to outlet
	_, err = h.store.GetCustomer(r.Context(), database.GetCustomerParams{
		ID:       customerID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "customer not found"})
			return
		}
		log.Printf("ERROR: get customer for orders: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
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

	customerUUID := pgtype.UUID{Bytes: customerID, Valid: true}
	orders, err := h.store.ListCustomerOrders(r.Context(), database.ListCustomerOrdersParams{
		CustomerID: customerUUID,
		OutletID:   outletID,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		log.Printf("ERROR: list customer orders: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = dbOrderToResponse(o)
	}

	writeJSON(w, http.StatusOK, resp)
}
