package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/enum"
	"golang.org/x/crypto/bcrypt"
)

// UserStore defines the database methods needed by user handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type UserStore interface {
	ListUsersByOutlet(ctx context.Context, outletID uuid.UUID) ([]database.User, error)
	CreateUser(ctx context.Context, arg database.CreateUserParams) (database.User, error)
	UpdateUser(ctx context.Context, arg database.UpdateUserParams) (database.User, error)
	SoftDeleteUser(ctx context.Context, arg database.SoftDeleteUserParams) (uuid.UUID, error)
}

// UserHandler handles user CRUD endpoints.
type UserHandler struct {
	store UserStore
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(store UserStore) *UserHandler {
	return &UserHandler{store: store}
}

// RegisterRoutes registers user CRUD endpoints on the given Chi router.
// Expected to be mounted inside an outlet-scoped subrouter: /outlets/{oid}/users
func (h *UserHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
}

// --- Request / Response types ---

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	Pin      string `json:"pin"`
}

type updateUserRequest struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	Pin      string `json:"pin"`
}

type userDetailResponse struct {
	ID        uuid.UUID `json:"id"`
	OutletID  uuid.UUID `json:"outlet_id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	Pin       string    `json:"pin,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserDetailResponse(u database.User) userDetailResponse {
	resp := userDetailResponse{
		ID:        u.ID,
		OutletID:  u.OutletID,
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      u.Role,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
	if u.Pin.Valid {
		resp.Pin = u.Pin.String
	}
	return resp
}

// --- Handlers ---

// List returns all active users for the given outlet.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	users, err := h.store.ListUsersByOutlet(r.Context(), outletID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]userDetailResponse, len(users))
	for i, u := range users {
		resp[i] = toUserDetailResponse(u)
	}

	writeJSON(w, http.StatusOK, resp)
}

// Create adds a new user to the given outlet.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Email == "" || req.Password == "" || req.FullName == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, password, full_name, and role are required"})
		return
	}

	if !strings.Contains(req.Email, "@") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email format"})
		return
	}

	if !isValidRole(req.Role) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role"})
		return
	}

	if req.Pin != "" {
		if len(req.Pin) < 4 || len(req.Pin) > 6 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PIN must be 4-6 digits"})
			return
		}
		for _, c := range req.Pin {
			if c < '0' || c > '9' {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PIN must be 4-6 digits"})
				return
			}
		}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("ERROR: create user: hash password: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	pin := pgtype.Text{}
	if req.Pin != "" {
		pin = pgtype.Text{String: req.Pin, Valid: true}
	}

	user, err := h.store.CreateUser(r.Context(), database.CreateUserParams{
		OutletID:       outletID,
		Email:          req.Email,
		HashedPassword: string(hashed),
		FullName:       req.FullName,
		Role:           req.Role,
		Pin:            pin,
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already exists"})
			return
		}
		log.Printf("ERROR: create user: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toUserDetailResponse(user))
}

// Update modifies an existing user in the given outlet.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Email == "" || req.FullName == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, full_name, and role are required"})
		return
	}

	if !strings.Contains(req.Email, "@") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email format"})
		return
	}

	if !isValidRole(req.Role) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role"})
		return
	}

	if req.Pin != "" {
		if len(req.Pin) < 4 || len(req.Pin) > 6 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PIN must be 4-6 digits"})
			return
		}
		for _, c := range req.Pin {
			if c < '0' || c > '9' {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PIN must be 4-6 digits"})
				return
			}
		}
	}

	pin := pgtype.Text{}
	if req.Pin != "" {
		pin = pgtype.Text{String: req.Pin, Valid: true}
	}

	user, err := h.store.UpdateUser(r.Context(), database.UpdateUserParams{
		Email:    req.Email,
		FullName: req.FullName,
		Role:     req.Role,
		Pin:      pin,
		ID:       userID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already exists"})
			return
		}
		log.Printf("ERROR: update user: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toUserDetailResponse(user))
}

// Delete soft-deletes a user by setting is_active=false.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	_, err = h.store.SoftDeleteUser(r.Context(), database.SoftDeleteUserParams{
		ID:       userID,
		OutletID: outletID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		log.Printf("ERROR: delete user: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Helpers ---

func isValidRole(role string) bool {
	switch role {
	case enum.UserRoleOwner, enum.UserRoleManager,
		enum.UserRoleCashier, enum.UserRoleKitchen:
		return true
	}
	return false
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
