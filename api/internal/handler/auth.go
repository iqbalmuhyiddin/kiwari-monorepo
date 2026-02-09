package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/auth"
	"github.com/kiwari-pos/api/internal/database"
	"golang.org/x/crypto/bcrypt"
)

// AuthStore defines the database methods needed by auth handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type AuthStore interface {
	GetUserByEmail(ctx context.Context, email string) (database.User, error)
	GetUserByOutletAndPin(ctx context.Context, arg database.GetUserByOutletAndPinParams) (database.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (database.User, error)
}

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	store     AuthStore
	jwtSecret string
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(store AuthStore, jwtSecret string) *AuthHandler {
	return &AuthHandler{store: store, jwtSecret: jwtSecret}
}

// RegisterRoutes registers auth endpoints on the given Chi router.
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/auth/login", h.Login)
	r.Post("/auth/pin-login", h.PinLogin)
	r.Post("/auth/refresh", h.Refresh)
}

// --- Request / Response types ---

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type pinLoginRequest struct {
	OutletID string `json:"outlet_id"`
	Pin      string `json:"pin"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         userResponse `json:"user"`
}

type userResponse struct {
	ID       uuid.UUID `json:"id"`
	OutletID uuid.UUID `json:"outlet_id"`
	FullName string    `json:"full_name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
}

// --- Handlers ---

// Login handles email + password authentication.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	h.respondWithTokens(w, user)
}

// PinLogin handles outlet_id + PIN authentication (for cashiers/kitchen staff).
// PINs are stored as plaintext by design. They are low-entropy (4-6 digits)
// and only grant cashier/kitchen access within a known outlet. The threat
// model accepts this tradeoff for faster cashier login flow.
func (h *AuthHandler) PinLogin(w http.ResponseWriter, r *http.Request) {
	var req pinLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.OutletID == "" || req.Pin == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "outlet_id and pin are required"})
		return
	}

	outletID, err := uuid.Parse(req.OutletID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
		return
	}

	user, err := h.store.GetUserByOutletAndPin(r.Context(), database.GetUserByOutletAndPinParams{
		OutletID: outletID,
		Pin:      pgtype.Text{String: req.Pin, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	h.respondWithTokens(w, user)
}

// Refresh exchanges a valid refresh token for a new access + refresh token pair.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "refresh_token is required"})
		return
	}

	// Parse refresh token -- it uses RegisteredClaims with Subject = user ID.
	token, err := jwt.ParseWithClaims(req.RefreshToken, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}

	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	h.respondWithTokens(w, user)
}

// --- Helpers ---

func (h *AuthHandler) respondWithTokens(w http.ResponseWriter, user database.User) {
	accessToken, err := auth.GenerateToken(h.jwtSecret, user.ID, user.OutletID, user.Role)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(h.jwtSecret, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: userResponse{
			ID:       user.ID,
			OutletID: user.OutletID,
			FullName: user.FullName,
			Email:    user.Email,
			Role:     user.Role,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ERROR: failed to encode JSON response: %v", err)
	}
}
