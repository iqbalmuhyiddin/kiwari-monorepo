package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/auth"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/handler"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "test-secret"

// --- Mock store ---

type mockAuthStore struct {
	userByEmail     map[string]database.User
	userByOutletPin map[string]database.User // key: "outletID:pin"
	userByID        map[uuid.UUID]database.User
}

func newMockStore() *mockAuthStore {
	return &mockAuthStore{
		userByEmail:     make(map[string]database.User),
		userByOutletPin: make(map[string]database.User),
		userByID:        make(map[uuid.UUID]database.User),
	}
}

func (m *mockAuthStore) addUser(u database.User) {
	m.userByEmail[u.Email] = u
	m.userByID[u.ID] = u
	if u.Pin.Valid {
		key := u.OutletID.String() + ":" + u.Pin.String
		m.userByOutletPin[key] = u
	}
}

func (m *mockAuthStore) GetUserByEmail(_ context.Context, email string) (database.User, error) {
	u, ok := m.userByEmail[email]
	if !ok {
		return database.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockAuthStore) GetUserByOutletAndPin(_ context.Context, arg database.GetUserByOutletAndPinParams) (database.User, error) {
	key := arg.OutletID.String() + ":" + arg.Pin.String
	u, ok := m.userByOutletPin[key]
	if !ok {
		return database.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockAuthStore) GetUserByID(_ context.Context, id uuid.UUID) (database.User, error) {
	u, ok := m.userByID[id]
	if !ok {
		return database.User{}, pgx.ErrNoRows
	}
	return u, nil
}

// --- Helpers ---

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(h)
}

func makeTestUser(t *testing.T) database.User {
	t.Helper()
	return database.User{
		ID:             uuid.New(),
		OutletID:       uuid.New(),
		Email:          "cashier@test.com",
		HashedPassword: hashPassword(t, "correct-password"),
		FullName:       "Test Cashier",
		Role:           database.UserRoleCASHIER,
		Pin:            pgtype.Text{String: "1234", Valid: true},
		IsActive:       true,
	}
}

func postJSON(t *testing.T, router http.Handler, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// --- Login tests ---

func TestLogin_ValidCredentials(t *testing.T) {
	store := newMockStore()
	user := makeTestUser(t)
	store.addUser(user)

	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/login", map[string]string{
		"email":    "cashier@test.com",
		"password": "correct-password",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeResponse(t, rr)
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected non-empty access_token")
	}
	if resp["refresh_token"] == nil || resp["refresh_token"] == "" {
		t.Error("expected non-empty refresh_token")
	}

	userResp, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user object in response")
	}
	if userResp["email"] != "cashier@test.com" {
		t.Errorf("user email: got %v, want cashier@test.com", userResp["email"])
	}
	if userResp["role"] != "CASHIER" {
		t.Errorf("user role: got %v, want CASHIER", userResp["role"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	store := newMockStore()
	store.addUser(makeTestUser(t))

	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/login", map[string]string{
		"email":    "cashier@test.com",
		"password": "wrong-password",
	})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	store := newMockStore()
	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/login", map[string]string{
		"email":    "nobody@test.com",
		"password": "password",
	})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	store := newMockStore()
	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/login", map[string]string{
		"email": "cashier@test.com",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- PIN Login tests ---

func TestPinLogin_ValidCredentials(t *testing.T) {
	store := newMockStore()
	user := makeTestUser(t)
	store.addUser(user)

	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/pin-login", map[string]string{
		"outlet_id": user.OutletID.String(),
		"pin":       "1234",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeResponse(t, rr)
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected non-empty access_token")
	}

	userResp, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user object in response")
	}
	if userResp["role"] != "CASHIER" {
		t.Errorf("user role: got %v, want CASHIER", userResp["role"])
	}
}

func TestPinLogin_WrongPin(t *testing.T) {
	store := newMockStore()
	user := makeTestUser(t)
	store.addUser(user)

	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/pin-login", map[string]string{
		"outlet_id": user.OutletID.String(),
		"pin":       "9999",
	})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestPinLogin_InvalidOutletID(t *testing.T) {
	store := newMockStore()
	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/pin-login", map[string]string{
		"outlet_id": "not-a-uuid",
		"pin":       "1234",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestPinLogin_MissingFields(t *testing.T) {
	store := newMockStore()
	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/pin-login", map[string]string{
		"outlet_id": uuid.New().String(),
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Refresh tests ---

func TestRefresh_ValidToken(t *testing.T) {
	store := newMockStore()
	user := makeTestUser(t)
	store.addUser(user)

	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Generate a valid refresh token for the user.
	refreshToken, err := auth.GenerateRefreshToken(testSecret, user.ID)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	rr := postJSON(t, r, "/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeResponse(t, rr)
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected non-empty access_token")
	}
	if resp["refresh_token"] == nil || resp["refresh_token"] == "" {
		t.Error("expected non-empty refresh_token")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	store := newMockStore()
	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/refresh", map[string]string{
		"refresh_token": "not-a-valid-token",
	})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestRefresh_UserDeleted(t *testing.T) {
	store := newMockStore()
	// Generate refresh token for a user that doesn't exist in the store.
	deletedUserID := uuid.New()
	refreshToken, err := auth.GenerateRefreshToken(testSecret, deletedUserID)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestRefresh_MissingField(t *testing.T) {
	store := newMockStore()
	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/refresh", map[string]string{})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Access token validation ---

func TestLogin_ReturnsValidAccessToken(t *testing.T) {
	store := newMockStore()
	user := makeTestUser(t)
	store.addUser(user)

	h := handler.NewAuthHandler(store, testSecret)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	rr := postJSON(t, r, "/auth/login", map[string]string{
		"email":    "cashier@test.com",
		"password": "correct-password",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeResponse(t, rr)
	accessToken, ok := resp["access_token"].(string)
	if !ok || accessToken == "" {
		t.Fatal("expected non-empty access_token string")
	}

	// Validate the returned access token contains correct claims.
	claims, err := auth.ValidateToken(testSecret, accessToken)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("claims user ID: got %v, want %v", claims.UserID, user.ID)
	}
	if claims.OutletID != user.OutletID {
		t.Errorf("claims outlet ID: got %v, want %v", claims.OutletID, user.OutletID)
	}
	if claims.Role != string(user.Role) {
		t.Errorf("claims role: got %v, want %v", claims.Role, user.Role)
	}
}
