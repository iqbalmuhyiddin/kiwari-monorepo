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
	"github.com/kiwari-pos/api/internal/database"
)

// ComboStore defines the database methods needed by combo item handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type ComboStore interface {
	// Product lookups
	GetProduct(ctx context.Context, arg database.GetProductParams) (database.Product, error)

	// Combo item operations
	ListComboItemsByCombo(ctx context.Context, comboID uuid.UUID) ([]database.ComboItem, error)
	CreateComboItem(ctx context.Context, arg database.CreateComboItemParams) (database.ComboItem, error)
	DeleteComboItem(ctx context.Context, arg database.DeleteComboItemParams) (int64, error)
}

// ComboHandler handles combo item CRUD endpoints.
type ComboHandler struct {
	store ComboStore
}

// NewComboHandler creates a new ComboHandler.
func NewComboHandler(store ComboStore) *ComboHandler {
	return &ComboHandler{store: store}
}

// RegisterRoutes registers combo item endpoints on the given Chi router.
// Expected to be mounted at /outlets/{oid}/products/{pid}
func (h *ComboHandler) RegisterRoutes(r chi.Router) {
	r.Get("/combo-items", h.List)
	r.Post("/combo-items", h.Create)
	r.Delete("/combo-items/{cid}", h.Delete)
}

// --- Request / Response types ---

type createComboItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
	SortOrder int32  `json:"sort_order"`
}

type comboItemResponse struct {
	ID        uuid.UUID `json:"id"`
	ComboID   uuid.UUID `json:"combo_id"`
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int32     `json:"quantity"`
	SortOrder int32     `json:"sort_order"`
}

func toComboItemResponse(ci database.ComboItem) comboItemResponse {
	return comboItemResponse{
		ID:        ci.ID,
		ComboID:   ci.ComboID,
		ProductID: ci.ProductID,
		Quantity:  ci.Quantity,
		SortOrder: ci.SortOrder,
	}
}

// --- Helpers ---

// verifyComboProductOwnership checks that the product belongs to the given outlet
// and that it is a combo product (is_combo=true).
// Returns the parsed outlet ID and product ID, or writes an error response.
func (h *ComboHandler) verifyComboProductOwnership(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
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

	product, err := h.store.GetProduct(r.Context(), database.GetProductParams{
		ID:       productID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
			return uuid.Nil, uuid.Nil, false
		}
		log.Printf("ERROR: verify combo product ownership: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return uuid.Nil, uuid.Nil, false
	}

	if !product.IsCombo {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product is not a combo"})
		return uuid.Nil, uuid.Nil, false
	}

	return outletID, productID, true
}

// --- Handlers ---

// List returns all combo items for the given combo product.
func (h *ComboHandler) List(w http.ResponseWriter, r *http.Request) {
	_, productID, ok := h.verifyComboProductOwnership(w, r)
	if !ok {
		return
	}

	items, err := h.store.ListComboItemsByCombo(r.Context(), productID)
	if err != nil {
		log.Printf("ERROR: list combo items: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]comboItemResponse, len(items))
	for i, ci := range items {
		resp[i] = toComboItemResponse(ci)
	}

	writeJSON(w, http.StatusOK, resp)
}

// Create adds a child product to the given combo product.
func (h *ComboHandler) Create(w http.ResponseWriter, r *http.Request) {
	outletID, comboID, ok := h.verifyComboProductOwnership(w, r)
	if !ok {
		return
	}

	var req createComboItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.ProductID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "product_id is required"})
		return
	}

	childProductID, err := uuid.Parse(req.ProductID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
		return
	}

	// A combo cannot contain itself
	if childProductID == comboID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "combo cannot contain itself"})
		return
	}

	// Verify child product exists and is active in the same outlet
	_, err = h.store.GetProduct(r.Context(), database.GetProductParams{
		ID:       childProductID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "child product not found in this outlet"})
			return
		}
		log.Printf("ERROR: verify child product: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Default quantity to 1 if not specified or zero
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}

	ci, err := h.store.CreateComboItem(r.Context(), database.CreateComboItemParams{
		ComboID:   comboID,
		ProductID: childProductID,
		Quantity:  quantity,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid product_id"})
			return
		}
		log.Printf("ERROR: create combo item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toComboItemResponse(ci))
}

// Delete hard-deletes a combo item.
func (h *ComboHandler) Delete(w http.ResponseWriter, r *http.Request) {
	_, comboID, ok := h.verifyComboProductOwnership(w, r)
	if !ok {
		return
	}

	comboItemID, err := uuid.Parse(chi.URLParam(r, "cid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid combo item ID"})
		return
	}

	rowsAffected, err := h.store.DeleteComboItem(r.Context(), database.DeleteComboItemParams{
		ID:      comboItemID,
		ComboID: comboID,
	})
	if err != nil {
		log.Printf("ERROR: delete combo item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if rowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "combo item not found"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
