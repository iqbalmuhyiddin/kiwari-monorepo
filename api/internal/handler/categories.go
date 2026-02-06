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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
)

// CategoryStore defines the database methods needed by category handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type CategoryStore interface {
	ListCategoriesByOutlet(ctx context.Context, outletID uuid.UUID) ([]database.Category, error)
	CreateCategory(ctx context.Context, arg database.CreateCategoryParams) (database.Category, error)
	UpdateCategory(ctx context.Context, arg database.UpdateCategoryParams) (database.Category, error)
	SoftDeleteCategory(ctx context.Context, arg database.SoftDeleteCategoryParams) (uuid.UUID, error)
}

// CategoryHandler handles category CRUD endpoints.
type CategoryHandler struct {
	store CategoryStore
}

// NewCategoryHandler creates a new CategoryHandler.
func NewCategoryHandler(store CategoryStore) *CategoryHandler {
	return &CategoryHandler{store: store}
}

// RegisterRoutes registers category CRUD endpoints on the given Chi router.
// Expected to be mounted inside an outlet-scoped subrouter: /outlets/{oid}/categories
func (h *CategoryHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
}

// --- Request / Response types ---

type createCategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int32  `json:"sort_order"`
}

type updateCategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int32  `json:"sort_order"`
}

type categoryResponse struct {
	ID          uuid.UUID `json:"id"`
	OutletID    uuid.UUID `json:"outlet_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	SortOrder   int32     `json:"sort_order"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

func toCategoryResponse(c database.Category) categoryResponse {
	resp := categoryResponse{
		ID:        c.ID,
		OutletID:  c.OutletID,
		Name:      c.Name,
		SortOrder: c.SortOrder,
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt,
	}
	if c.Description.Valid {
		resp.Description = &c.Description.String
	}
	return resp
}

// --- Handlers ---

// List returns all active categories for the given outlet.
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	categories, err := h.store.ListCategoriesByOutlet(r.Context(), outletID)
	if err != nil {
		log.Printf("ERROR: list categories: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]categoryResponse, len(categories))
	for i, c := range categories {
		resp[i] = toCategoryResponse(c)
	}

	writeJSON(w, http.StatusOK, resp)
}

// Create adds a new category to the given outlet.
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	var req createCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	desc := pgtype.Text{}
	if req.Description != "" {
		desc = pgtype.Text{String: req.Description, Valid: true}
	}

	category, err := h.store.CreateCategory(r.Context(), database.CreateCategoryParams{
		OutletID:    outletID,
		Name:        req.Name,
		Description: desc,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		log.Printf("ERROR: create category: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toCategoryResponse(category))
}

// Update modifies an existing category in the given outlet.
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	catID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category ID"})
		return
	}

	var req updateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	desc := pgtype.Text{}
	if req.Description != "" {
		desc = pgtype.Text{String: req.Description, Valid: true}
	}

	category, err := h.store.UpdateCategory(r.Context(), database.UpdateCategoryParams{
		Name:        req.Name,
		Description: desc,
		SortOrder:   req.SortOrder,
		ID:          catID,
		OutletID:    outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
			return
		}
		log.Printf("ERROR: update category: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toCategoryResponse(category))
}

// Delete soft-deletes a category by setting is_active=false.
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	catID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category ID"})
		return
	}

	_, err = h.store.SoftDeleteCategory(r.Context(), database.SoftDeleteCategoryParams{
		ID:       catID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
			return
		}
		log.Printf("ERROR: delete category: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
