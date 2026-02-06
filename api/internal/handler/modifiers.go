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

// ModifierStore defines the database methods needed by modifier handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type ModifierStore interface {
	// Product ownership verification
	GetProduct(ctx context.Context, arg database.GetProductParams) (database.Product, error)

	// Modifier groups
	ListModifierGroupsByProduct(ctx context.Context, productID uuid.UUID) ([]database.ModifierGroup, error)
	GetModifierGroup(ctx context.Context, arg database.GetModifierGroupParams) (database.ModifierGroup, error)
	CreateModifierGroup(ctx context.Context, arg database.CreateModifierGroupParams) (database.ModifierGroup, error)
	UpdateModifierGroup(ctx context.Context, arg database.UpdateModifierGroupParams) (database.ModifierGroup, error)
	SoftDeleteModifierGroup(ctx context.Context, arg database.SoftDeleteModifierGroupParams) (uuid.UUID, error)

	// Modifiers
	ListModifiersByGroup(ctx context.Context, modifierGroupID uuid.UUID) ([]database.Modifier, error)
	CreateModifier(ctx context.Context, arg database.CreateModifierParams) (database.Modifier, error)
	UpdateModifier(ctx context.Context, arg database.UpdateModifierParams) (database.Modifier, error)
	SoftDeleteModifier(ctx context.Context, arg database.SoftDeleteModifierParams) (uuid.UUID, error)
}

// ModifierHandler handles modifier group and modifier CRUD endpoints.
type ModifierHandler struct {
	store ModifierStore
}

// NewModifierHandler creates a new ModifierHandler.
func NewModifierHandler(store ModifierStore) *ModifierHandler {
	return &ModifierHandler{store: store}
}

// RegisterRoutes registers modifier group and modifier endpoints on the given Chi router.
// Expected to be mounted at /outlets/{oid}/products/{pid}
func (h *ModifierHandler) RegisterRoutes(r chi.Router) {
	r.Get("/modifier-groups", h.ListGroups)
	r.Post("/modifier-groups", h.CreateGroup)
	r.Put("/modifier-groups/{mgid}", h.UpdateGroup)
	r.Delete("/modifier-groups/{mgid}", h.DeleteGroup)

	r.Get("/modifier-groups/{mgid}/modifiers", h.ListModifiers)
	r.Post("/modifier-groups/{mgid}/modifiers", h.CreateModifier)
	r.Put("/modifier-groups/{mgid}/modifiers/{mid}", h.UpdateModifier)
	r.Delete("/modifier-groups/{mgid}/modifiers/{mid}", h.DeleteModifier)
}

// --- Request / Response types ---

type createModifierGroupRequest struct {
	Name      string `json:"name"`
	MinSelect *int32 `json:"min_select"`
	MaxSelect *int32 `json:"max_select"`
	SortOrder int32  `json:"sort_order"`
}

type updateModifierGroupRequest struct {
	Name      string `json:"name"`
	MinSelect *int32 `json:"min_select"`
	MaxSelect *int32 `json:"max_select"`
	SortOrder int32  `json:"sort_order"`
}

type modifierGroupResponse struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"product_id"`
	Name      string    `json:"name"`
	MinSelect int32     `json:"min_select"`
	MaxSelect *int32    `json:"max_select"`
	IsActive  bool      `json:"is_active"`
	SortOrder int32     `json:"sort_order"`
}

func toModifierGroupResponse(mg database.ModifierGroup) modifierGroupResponse {
	resp := modifierGroupResponse{
		ID:        mg.ID,
		ProductID: mg.ProductID,
		Name:      mg.Name,
		MinSelect: mg.MinSelect,
		IsActive:  mg.IsActive,
		SortOrder: mg.SortOrder,
	}

	if mg.MaxSelect.Valid {
		val := mg.MaxSelect.Int32
		resp.MaxSelect = &val
	}

	return resp
}

type createModifierRequest struct {
	Name      string `json:"name"`
	Price     string `json:"price"`
	SortOrder int32  `json:"sort_order"`
}

type updateModifierRequest struct {
	Name      string `json:"name"`
	Price     string `json:"price"`
	SortOrder int32  `json:"sort_order"`
}

type modifierResponse struct {
	ID              uuid.UUID `json:"id"`
	ModifierGroupID uuid.UUID `json:"modifier_group_id"`
	Name            string    `json:"name"`
	Price           string    `json:"price"`
	IsActive        bool      `json:"is_active"`
	SortOrder       int32     `json:"sort_order"`
}

func toModifierResponse(m database.Modifier) modifierResponse {
	resp := modifierResponse{
		ID:              m.ID,
		ModifierGroupID: m.ModifierGroupID,
		Name:            m.Name,
		IsActive:        m.IsActive,
		SortOrder:       m.SortOrder,
	}

	if m.Price.Valid {
		val, err := m.Price.Value()
		if err == nil && val != nil {
			d, err := decimal.NewFromString(val.(string))
			if err == nil {
				resp.Price = d.StringFixed(2)
			}
		}
	}

	return resp
}

// --- Helpers ---

func parsePrice(s string) (pgtype.Numeric, error) {
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

// verifyModifierProductOwnership checks that the product belongs to the given outlet.
// Returns the parsed outlet ID and product ID, or writes an error response.
func (h *ModifierHandler) verifyModifierProductOwnership(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
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

// verifyModifierGroupOwnership verifies product ownership and then checks the modifier group
// belongs to the product. Returns outlet ID, product ID, and modifier group ID.
func (h *ModifierHandler) verifyModifierGroupOwnership(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	outletID, productID, ok := h.verifyModifierProductOwnership(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	mgID, err := uuid.Parse(chi.URLParam(r, "mgid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid modifier group ID"})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	_, err = h.store.GetModifierGroup(r.Context(), database.GetModifierGroupParams{
		ID:        mgID,
		ProductID: productID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "modifier group not found"})
			return uuid.Nil, uuid.Nil, uuid.Nil, false
		}
		log.Printf("ERROR: verify modifier group ownership: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}

	return outletID, productID, mgID, true
}

// --- Modifier Group Handlers ---

// ListGroups returns all active modifier groups for the given product.
func (h *ModifierHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyModifierProductOwnership(w, r)
	if !ok {
		return
	}

	groups, err := h.store.ListModifierGroupsByProduct(r.Context(), productID)
	if err != nil {
		log.Printf("ERROR: list modifier groups: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]modifierGroupResponse, len(groups))
	for i, mg := range groups {
		resp[i] = toModifierGroupResponse(mg)
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateGroup adds a new modifier group to the given product.
func (h *ModifierHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyModifierProductOwnership(w, r)
	if !ok {
		return
	}

	var req createModifierGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Default min_select to 0 when not specified
	minSelect := int32(0)
	if req.MinSelect != nil {
		minSelect = *req.MinSelect
	}

	if minSelect < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "min_select must be >= 0"})
		return
	}

	// max_select: nullable (unlimited when null)
	var maxSelect pgtype.Int4
	if req.MaxSelect != nil {
		if *req.MaxSelect < minSelect {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "max_select must be >= min_select"})
			return
		}
		maxSelect = pgtype.Int4{Int32: *req.MaxSelect, Valid: true}
	}

	mg, err := h.store.CreateModifierGroup(r.Context(), database.CreateModifierGroupParams{
		ProductID: productID,
		Name:      req.Name,
		MinSelect: minSelect,
		MaxSelect: maxSelect,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
			return
		}
		log.Printf("ERROR: create modifier group: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toModifierGroupResponse(mg))
}

// UpdateGroup modifies an existing modifier group.
func (h *ModifierHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyModifierProductOwnership(w, r)
	if !ok {
		return
	}

	mgID, err := uuid.Parse(chi.URLParam(r, "mgid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid modifier group ID"})
		return
	}

	var req updateModifierGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Default min_select to 0 when not specified
	minSelect := int32(0)
	if req.MinSelect != nil {
		minSelect = *req.MinSelect
	}

	if minSelect < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "min_select must be >= 0"})
		return
	}

	// max_select: nullable (unlimited when null)
	var maxSelect pgtype.Int4
	if req.MaxSelect != nil {
		if *req.MaxSelect < minSelect {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "max_select must be >= min_select"})
			return
		}
		maxSelect = pgtype.Int4{Int32: *req.MaxSelect, Valid: true}
	}

	mg, err := h.store.UpdateModifierGroup(r.Context(), database.UpdateModifierGroupParams{
		Name:      req.Name,
		MinSelect: minSelect,
		MaxSelect: maxSelect,
		SortOrder: req.SortOrder,
		ID:        mgID,
		ProductID: productID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "modifier group not found"})
			return
		}
		log.Printf("ERROR: update modifier group: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toModifierGroupResponse(mg))
}

// DeleteGroup soft-deletes a modifier group by setting is_active=false.
func (h *ModifierHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyModifierProductOwnership(w, r)
	if !ok {
		return
	}

	mgID, err := uuid.Parse(chi.URLParam(r, "mgid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid modifier group ID"})
		return
	}

	_, err = h.store.SoftDeleteModifierGroup(r.Context(), database.SoftDeleteModifierGroupParams{
		ID:        mgID,
		ProductID: productID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "modifier group not found"})
			return
		}
		log.Printf("ERROR: delete modifier group: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Modifier Handlers ---

// ListModifiers returns all active modifiers for the given modifier group.
func (h *ModifierHandler) ListModifiers(w http.ResponseWriter, r *http.Request) {
	_, _, mgID, ok := h.verifyModifierGroupOwnership(w, r)
	if !ok {
		return
	}

	modifiers, err := h.store.ListModifiersByGroup(r.Context(), mgID)
	if err != nil {
		log.Printf("ERROR: list modifiers: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]modifierResponse, len(modifiers))
	for i, m := range modifiers {
		resp[i] = toModifierResponse(m)
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateModifier adds a new modifier to the given modifier group.
func (h *ModifierHandler) CreateModifier(w http.ResponseWriter, r *http.Request) {
	_, _, mgID, ok := h.verifyModifierGroupOwnership(w, r)
	if !ok {
		return
	}

	var req createModifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Default price to "0" when not specified
	priceStr := req.Price
	if priceStr == "" {
		priceStr = "0"
	}

	price, err := parsePrice(priceStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid price"})
		return
	}

	// Validate price >= 0
	d, _ := decimal.NewFromString(priceStr)
	if d.IsNegative() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "price must be >= 0"})
		return
	}

	m, err := h.store.CreateModifier(r.Context(), database.CreateModifierParams{
		ModifierGroupID: mgID,
		Name:            req.Name,
		Price:           price,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid modifier_group_id"})
			return
		}
		log.Printf("ERROR: create modifier: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toModifierResponse(m))
}

// UpdateModifier modifies an existing modifier.
func (h *ModifierHandler) UpdateModifier(w http.ResponseWriter, r *http.Request) {
	_, _, mgID, ok := h.verifyModifierGroupOwnership(w, r)
	if !ok {
		return
	}

	modifierID, err := uuid.Parse(chi.URLParam(r, "mid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid modifier ID"})
		return
	}

	var req updateModifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Default price to "0" when not specified
	priceStr := req.Price
	if priceStr == "" {
		priceStr = "0"
	}

	price, err := parsePrice(priceStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid price"})
		return
	}

	// Validate price >= 0
	d, _ := decimal.NewFromString(priceStr)
	if d.IsNegative() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "price must be >= 0"})
		return
	}

	m, err := h.store.UpdateModifier(r.Context(), database.UpdateModifierParams{
		Name:            req.Name,
		Price:           price,
		SortOrder:       req.SortOrder,
		ID:              modifierID,
		ModifierGroupID: mgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "modifier not found"})
			return
		}
		log.Printf("ERROR: update modifier: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toModifierResponse(m))
}

// DeleteModifier soft-deletes a modifier by setting is_active=false.
func (h *ModifierHandler) DeleteModifier(w http.ResponseWriter, r *http.Request) {
	_, _, mgID, ok := h.verifyModifierGroupOwnership(w, r)
	if !ok {
		return
	}

	modifierID, err := uuid.Parse(chi.URLParam(r, "mid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid modifier ID"})
		return
	}

	_, err = h.store.SoftDeleteModifier(r.Context(), database.SoftDeleteModifierParams{
		ID:              modifierID,
		ModifierGroupID: mgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "modifier not found"})
			return
		}
		log.Printf("ERROR: delete modifier: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
