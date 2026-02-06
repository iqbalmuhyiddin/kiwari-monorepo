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
	"github.com/shopspring/decimal"
)

// VariantStore defines the database methods needed by variant handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type VariantStore interface {
	// Product ownership verification
	GetProduct(ctx context.Context, arg database.GetProductParams) (database.Product, error)

	// Variant groups
	ListVariantGroupsByProduct(ctx context.Context, productID uuid.UUID) ([]database.VariantGroup, error)
	GetVariantGroup(ctx context.Context, arg database.GetVariantGroupParams) (database.VariantGroup, error)
	CreateVariantGroup(ctx context.Context, arg database.CreateVariantGroupParams) (database.VariantGroup, error)
	UpdateVariantGroup(ctx context.Context, arg database.UpdateVariantGroupParams) (database.VariantGroup, error)
	SoftDeleteVariantGroup(ctx context.Context, arg database.SoftDeleteVariantGroupParams) (uuid.UUID, error)

	// Variants
	ListVariantsByGroup(ctx context.Context, variantGroupID uuid.UUID) ([]database.Variant, error)
	CreateVariant(ctx context.Context, arg database.CreateVariantParams) (database.Variant, error)
	UpdateVariant(ctx context.Context, arg database.UpdateVariantParams) (database.Variant, error)
	SoftDeleteVariant(ctx context.Context, arg database.SoftDeleteVariantParams) (uuid.UUID, error)
}

// VariantHandler handles variant group and variant CRUD endpoints.
type VariantHandler struct {
	store VariantStore
}

// NewVariantHandler creates a new VariantHandler.
func NewVariantHandler(store VariantStore) *VariantHandler {
	return &VariantHandler{store: store}
}

// RegisterRoutes registers variant group and variant endpoints on the given Chi router.
// Expected to be mounted at /outlets/{oid}/products/{pid}
func (h *VariantHandler) RegisterRoutes(r chi.Router) {
	r.Get("/variant-groups", h.ListGroups)
	r.Post("/variant-groups", h.CreateGroup)
	r.Put("/variant-groups/{vgid}", h.UpdateGroup)
	r.Delete("/variant-groups/{vgid}", h.DeleteGroup)

	r.Get("/variant-groups/{vgid}/variants", h.ListVariants)
	r.Post("/variant-groups/{vgid}/variants", h.CreateVariant)
	r.Put("/variant-groups/{vgid}/variants/{vid}", h.UpdateVariant)
	r.Delete("/variant-groups/{vgid}/variants/{vid}", h.DeleteVariant)
}

// --- Request / Response types ---

type createVariantGroupRequest struct {
	Name       string `json:"name"`
	IsRequired *bool  `json:"is_required"`
	SortOrder  int32  `json:"sort_order"`
}

type updateVariantGroupRequest struct {
	Name       string `json:"name"`
	IsRequired *bool  `json:"is_required"`
	SortOrder  int32  `json:"sort_order"`
}

type variantGroupResponse struct {
	ID         uuid.UUID `json:"id"`
	ProductID  uuid.UUID `json:"product_id"`
	Name       string    `json:"name"`
	IsRequired bool      `json:"is_required"`
	IsActive   bool      `json:"is_active"`
	SortOrder  int32     `json:"sort_order"`
}

func toVariantGroupResponse(vg database.VariantGroup) variantGroupResponse {
	return variantGroupResponse{
		ID:         vg.ID,
		ProductID:  vg.ProductID,
		Name:       vg.Name,
		IsRequired: vg.IsRequired,
		IsActive:   vg.IsActive,
		SortOrder:  vg.SortOrder,
	}
}

type createVariantRequest struct {
	Name            string `json:"name"`
	PriceAdjustment string `json:"price_adjustment"`
	SortOrder       int32  `json:"sort_order"`
}

type updateVariantRequest struct {
	Name            string `json:"name"`
	PriceAdjustment string `json:"price_adjustment"`
	SortOrder       int32  `json:"sort_order"`
}

type variantResponse struct {
	ID              uuid.UUID `json:"id"`
	VariantGroupID  uuid.UUID `json:"variant_group_id"`
	Name            string    `json:"name"`
	PriceAdjustment string    `json:"price_adjustment"`
	IsActive        bool      `json:"is_active"`
	SortOrder       int32     `json:"sort_order"`
}

func toVariantResponse(v database.Variant) variantResponse {
	resp := variantResponse{
		ID:             v.ID,
		VariantGroupID: v.VariantGroupID,
		Name:           v.Name,
		IsActive:       v.IsActive,
		SortOrder:      v.SortOrder,
	}

	if v.PriceAdjustment.Valid {
		val, err := v.PriceAdjustment.Value()
		if err == nil && val != nil {
			d, err := decimal.NewFromString(val.(string))
			if err == nil {
				resp.PriceAdjustment = d.StringFixed(2)
			}
		}
	}

	return resp
}

// --- Helpers ---

func parsePriceAdjustment(s string) (pgtype.Numeric, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return pgtype.Numeric{}, err
	}
	var n pgtype.Numeric
	if err := n.Scan(d.String()); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// verifyProductOwnership checks that the product belongs to the given outlet.
// Returns the parsed outlet ID and product ID, or writes an error response.
func (h *VariantHandler) verifyProductOwnership(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return uuid.Nil, uuid.Nil, false
	}

	productID, err := uuid.Parse(chi.URLParam(r, "pid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product ID"})
		return uuid.Nil, uuid.Nil, false
	}

	_, err = h.store.GetProduct(r.Context(), database.GetProductParams{
		ID:       productID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
			return uuid.Nil, uuid.Nil, false
		}
		log.Printf("ERROR: verify product ownership: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return uuid.Nil, uuid.Nil, false
	}

	return outletID, productID, true
}

// verifyVariantGroupOwnership verifies product ownership and then checks the variant group
// belongs to the product. Returns outlet ID, product ID, and variant group ID.
func (h *VariantHandler) verifyVariantGroupOwnership(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	outletID, productID, ok := h.verifyProductOwnership(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	vgID, err := uuid.Parse(chi.URLParam(r, "vgid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid variant group ID"})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	_, err = h.store.GetVariantGroup(r.Context(), database.GetVariantGroupParams{
		ID:        vgID,
		ProductID: productID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "variant group not found"})
			return uuid.Nil, uuid.Nil, uuid.Nil, false
		}
		log.Printf("ERROR: verify variant group ownership: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	return outletID, productID, vgID, true
}

// --- Variant Group Handlers ---

// ListGroups returns all active variant groups for the given product.
func (h *VariantHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyProductOwnership(w, r)
	if !ok {
		return
	}

	groups, err := h.store.ListVariantGroupsByProduct(r.Context(), productID)
	if err != nil {
		log.Printf("ERROR: list variant groups: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]variantGroupResponse, len(groups))
	for i, vg := range groups {
		resp[i] = toVariantGroupResponse(vg)
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateGroup adds a new variant group to the given product.
func (h *VariantHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyProductOwnership(w, r)
	if !ok {
		return
	}

	var req createVariantGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Default is_required to true when not specified
	isRequired := true
	if req.IsRequired != nil {
		isRequired = *req.IsRequired
	}

	vg, err := h.store.CreateVariantGroup(r.Context(), database.CreateVariantGroupParams{
		ProductID:  productID,
		Name:       req.Name,
		IsRequired: isRequired,
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
			return
		}
		log.Printf("ERROR: create variant group: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toVariantGroupResponse(vg))
}

// UpdateGroup modifies an existing variant group.
func (h *VariantHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyProductOwnership(w, r)
	if !ok {
		return
	}

	vgID, err := uuid.Parse(chi.URLParam(r, "vgid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid variant group ID"})
		return
	}

	var req updateVariantGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Default is_required to true when not specified
	isRequired := true
	if req.IsRequired != nil {
		isRequired = *req.IsRequired
	}

	vg, err := h.store.UpdateVariantGroup(r.Context(), database.UpdateVariantGroupParams{
		Name:       req.Name,
		IsRequired: isRequired,
		SortOrder:  req.SortOrder,
		ID:         vgID,
		ProductID:  productID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "variant group not found"})
			return
		}
		log.Printf("ERROR: update variant group: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toVariantGroupResponse(vg))
}

// DeleteGroup soft-deletes a variant group by setting is_active=false.
func (h *VariantHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyProductOwnership(w, r)
	if !ok {
		return
	}

	vgID, err := uuid.Parse(chi.URLParam(r, "vgid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid variant group ID"})
		return
	}

	_, err = h.store.SoftDeleteVariantGroup(r.Context(), database.SoftDeleteVariantGroupParams{
		ID:        vgID,
		ProductID: productID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "variant group not found"})
			return
		}
		log.Printf("ERROR: delete variant group: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Variant Handlers ---

// ListVariants returns all active variants for the given variant group.
func (h *VariantHandler) ListVariants(w http.ResponseWriter, r *http.Request) {
	_, _, vgID, ok := h.verifyVariantGroupOwnership(w, r)
	if !ok {
		return
	}

	variants, err := h.store.ListVariantsByGroup(r.Context(), vgID)
	if err != nil {
		log.Printf("ERROR: list variants: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]variantResponse, len(variants))
	for i, v := range variants {
		resp[i] = toVariantResponse(v)
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateVariant adds a new variant to the given variant group.
func (h *VariantHandler) CreateVariant(w http.ResponseWriter, r *http.Request) {
	_, _, vgID, ok := h.verifyVariantGroupOwnership(w, r)
	if !ok {
		return
	}

	var req createVariantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Default price_adjustment to "0" when not specified
	priceStr := req.PriceAdjustment
	if priceStr == "" {
		priceStr = "0"
	}

	priceAdj, err := parsePriceAdjustment(priceStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid price_adjustment"})
		return
	}

	v, err := h.store.CreateVariant(r.Context(), database.CreateVariantParams{
		VariantGroupID:  vgID,
		Name:            req.Name,
		PriceAdjustment: priceAdj,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid variant_group_id"})
			return
		}
		log.Printf("ERROR: create variant: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toVariantResponse(v))
}

// UpdateVariant modifies an existing variant.
func (h *VariantHandler) UpdateVariant(w http.ResponseWriter, r *http.Request) {
	_, _, vgID, ok := h.verifyVariantGroupOwnership(w, r)
	if !ok {
		return
	}

	variantID, err := uuid.Parse(chi.URLParam(r, "vid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid variant ID"})
		return
	}

	var req updateVariantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Default price_adjustment to "0" when not specified
	priceStr := req.PriceAdjustment
	if priceStr == "" {
		priceStr = "0"
	}

	priceAdj, err := parsePriceAdjustment(priceStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid price_adjustment"})
		return
	}

	v, err := h.store.UpdateVariant(r.Context(), database.UpdateVariantParams{
		Name:            req.Name,
		PriceAdjustment: priceAdj,
		SortOrder:       req.SortOrder,
		ID:              variantID,
		VariantGroupID:  vgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "variant not found"})
			return
		}
		log.Printf("ERROR: update variant: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toVariantResponse(v))
}

// DeleteVariant soft-deletes a variant by setting is_active=false.
func (h *VariantHandler) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	_, _, vgID, ok := h.verifyVariantGroupOwnership(w, r)
	if !ok {
		return
	}

	variantID, err := uuid.Parse(chi.URLParam(r, "vid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid variant ID"})
		return
	}

	_, err = h.store.SoftDeleteVariant(r.Context(), database.SoftDeleteVariantParams{
		ID:             variantID,
		VariantGroupID: vgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "variant not found"})
			return
		}
		log.Printf("ERROR: delete variant: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
