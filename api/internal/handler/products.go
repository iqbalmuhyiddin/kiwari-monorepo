package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// ProductStore defines the database methods needed by product handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type ProductStore interface {
	ListProductsByOutlet(ctx context.Context, outletID uuid.UUID) ([]database.Product, error)
	GetProduct(ctx context.Context, arg database.GetProductParams) (database.Product, error)
	CreateProduct(ctx context.Context, arg database.CreateProductParams) (database.Product, error)
	UpdateProduct(ctx context.Context, arg database.UpdateProductParams) (database.Product, error)
	SoftDeleteProduct(ctx context.Context, arg database.SoftDeleteProductParams) (uuid.UUID, error)
}

// ProductHandler handles product CRUD endpoints.
type ProductHandler struct {
	store ProductStore
}

// NewProductHandler creates a new ProductHandler.
func NewProductHandler(store ProductStore) *ProductHandler {
	return &ProductHandler{store: store}
}

// RegisterRoutes registers product CRUD endpoints on the given Chi router.
// Expected to be mounted inside an outlet-scoped subrouter: /outlets/{oid}/products
func (h *ProductHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
}

// --- Request / Response types ---

type createProductRequest struct {
	CategoryID      string `json:"category_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	BasePrice       string `json:"base_price"`
	ImageURL        string `json:"image_url"`
	Station         string `json:"station"`
	PreparationTime *int32 `json:"preparation_time"`
	IsCombo         bool   `json:"is_combo"`
}

type updateProductRequest struct {
	CategoryID      string `json:"category_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	BasePrice       string `json:"base_price"`
	ImageURL        string `json:"image_url"`
	Station         string `json:"station"`
	PreparationTime *int32 `json:"preparation_time"`
	IsCombo         bool   `json:"is_combo"`
}

type productResponse struct {
	ID              uuid.UUID `json:"id"`
	OutletID        uuid.UUID `json:"outlet_id"`
	CategoryID      uuid.UUID `json:"category_id"`
	Name            string    `json:"name"`
	Description     *string   `json:"description"`
	BasePrice       string    `json:"base_price"`
	ImageURL        *string   `json:"image_url"`
	Station         *string   `json:"station"`
	PreparationTime *int32    `json:"preparation_time"`
	IsCombo         bool      `json:"is_combo"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func toProductResponse(p database.Product) productResponse {
	resp := productResponse{
		ID:         p.ID,
		OutletID:   p.OutletID,
		CategoryID: p.CategoryID,
		Name:       p.Name,
		IsCombo:    p.IsCombo,
		IsActive:   p.IsActive,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
	}

	// Convert pgtype.Numeric to string for base_price.
	// Always format with 2 decimal places for consistent money representation.
	if p.BasePrice.Valid {
		val, err := p.BasePrice.Value()
		if err == nil && val != nil {
			d, err := decimal.NewFromString(val.(string))
			if err == nil {
				resp.BasePrice = d.StringFixed(2)
			}
		}
	}

	if p.Description.Valid {
		resp.Description = &p.Description.String
	}
	if p.ImageUrl.Valid {
		resp.ImageURL = &p.ImageUrl.String
	}
	if p.Station.Valid {
		s := string(p.Station.KitchenStation)
		resp.Station = &s
	}
	if p.PreparationTime.Valid {
		pt := p.PreparationTime.Int32
		resp.PreparationTime = &pt
	}
	return resp
}

// --- Helpers ---

func isValidStation(station string) bool {
	switch database.KitchenStation(station) {
	case database.KitchenStationGRILL, database.KitchenStationBEVERAGE,
		database.KitchenStationRICE, database.KitchenStationDESSERT:
		return true
	}
	return false
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

var errNegativePrice = errors.New("negative price")

func parseBasePrice(s string) (pgtype.Numeric, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return pgtype.Numeric{}, err
	}
	if d.IsNegative() {
		return pgtype.Numeric{}, errNegativePrice
	}
	var n pgtype.Numeric
	if err := n.Scan(d.String()); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// --- Handlers ---

// List returns all active products for the given outlet.
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	products, err := h.store.ListProductsByOutlet(r.Context(), outletID)
	if err != nil {
		log.Printf("ERROR: list products: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]productResponse, len(products))
	for i, p := range products {
		resp[i] = toProductResponse(p)
	}

	writeJSON(w, http.StatusOK, resp)
}

// Get returns a single product by ID.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	prodID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product ID"})
		return
	}

	product, err := h.store.GetProduct(r.Context(), database.GetProductParams{
		ID:       prodID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
			return
		}
		log.Printf("ERROR: get product: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toProductResponse(product))
}

// Create adds a new product to the given outlet.
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	var req createProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.CategoryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category_id is required"})
		return
	}

	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category_id"})
		return
	}

	if req.BasePrice == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "base_price is required"})
		return
	}

	price, err := parseBasePrice(req.BasePrice)
	if err != nil {
		if errors.Is(err, errNegativePrice) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "base_price must be >= 0"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base_price"})
		}
		return
	}

	// Validate optional station
	if req.Station != "" && !isValidStation(req.Station) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid station"})
		return
	}

	// Build params
	desc := pgtype.Text{}
	if req.Description != "" {
		desc = pgtype.Text{String: req.Description, Valid: true}
	}

	imageURL := pgtype.Text{}
	if req.ImageURL != "" {
		imageURL = pgtype.Text{String: req.ImageURL, Valid: true}
	}

	station := database.NullKitchenStation{}
	if req.Station != "" {
		station = database.NullKitchenStation{KitchenStation: database.KitchenStation(req.Station), Valid: true}
	}

	prepTime := pgtype.Int4{}
	if req.PreparationTime != nil {
		prepTime = pgtype.Int4{Int32: *req.PreparationTime, Valid: true}
	}

	product, err := h.store.CreateProduct(r.Context(), database.CreateProductParams{
		OutletID:        outletID,
		CategoryID:      categoryID,
		Name:            req.Name,
		Description:     desc,
		BasePrice:       price,
		ImageUrl:        imageURL,
		Station:         station,
		PreparationTime: prepTime,
		IsCombo:         req.IsCombo,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category_id"})
			return
		}
		log.Printf("ERROR: create product: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toProductResponse(product))
}

// Update modifies an existing product in the given outlet.
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	prodID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product ID"})
		return
	}

	var req updateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.CategoryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category_id is required"})
		return
	}

	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category_id"})
		return
	}

	if req.BasePrice == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "base_price is required"})
		return
	}

	price, err := parseBasePrice(req.BasePrice)
	if err != nil {
		if errors.Is(err, errNegativePrice) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "base_price must be >= 0"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base_price"})
		}
		return
	}

	// Validate optional station
	if req.Station != "" && !isValidStation(req.Station) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid station"})
		return
	}

	// Build params
	desc := pgtype.Text{}
	if req.Description != "" {
		desc = pgtype.Text{String: req.Description, Valid: true}
	}

	imageURL := pgtype.Text{}
	if req.ImageURL != "" {
		imageURL = pgtype.Text{String: req.ImageURL, Valid: true}
	}

	station := database.NullKitchenStation{}
	if req.Station != "" {
		station = database.NullKitchenStation{KitchenStation: database.KitchenStation(req.Station), Valid: true}
	}

	prepTime := pgtype.Int4{}
	if req.PreparationTime != nil {
		prepTime = pgtype.Int4{Int32: *req.PreparationTime, Valid: true}
	}

	product, err := h.store.UpdateProduct(r.Context(), database.UpdateProductParams{
		CategoryID:      categoryID,
		Name:            req.Name,
		Description:     desc,
		BasePrice:       price,
		ImageUrl:        imageURL,
		Station:         station,
		PreparationTime: prepTime,
		IsCombo:         req.IsCombo,
		ID:              prodID,
		OutletID:        outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
			return
		}
		if isForeignKeyViolation(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category_id"})
			return
		}
		log.Printf("ERROR: update product: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toProductResponse(product))
}

// Delete soft-deletes a product by setting is_active=false.
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	prodID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product ID"})
		return
	}

	_, err = h.store.SoftDeleteProduct(r.Context(), database.SoftDeleteProductParams{
		ID:       prodID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
			return
		}
		log.Printf("ERROR: delete product: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
