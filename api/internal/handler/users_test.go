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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/enum"
	"github.com/kiwari-pos/api/internal/handler"
	"golang.org/x/crypto/bcrypt"
)

// --- Mock store ---

type mockUserStore struct {
	users map[uuid.UUID]database.User // keyed by user ID
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{users: make(map[uuid.UUID]database.User)}
}

func (m *mockUserStore) ListUsersByOutlet(_ context.Context, outletID uuid.UUID) ([]database.User, error) {
	var result []database.User
	for _, u := range m.users {
		if u.OutletID == outletID && u.IsActive {
			result = append(result, u)
		}
	}
	return result, nil
}

func (m *mockUserStore) CreateUser(_ context.Context, arg database.CreateUserParams) (database.User, error) {
	// Check for duplicate email (simulates PostgreSQL unique constraint)
	for _, existing := range m.users {
		if existing.Email == arg.Email && existing.IsActive {
			return database.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
		}
	}
	u := database.User{
		ID:             uuid.New(),
		OutletID:       arg.OutletID,
		Email:          arg.Email,
		HashedPassword: arg.HashedPassword,
		FullName:       arg.FullName,
		Role:           arg.Role,
		Pin:            arg.Pin,
		IsActive:       true,
	}
	m.users[u.ID] = u
	return u, nil
}

func (m *mockUserStore) UpdateUser(_ context.Context, arg database.UpdateUserParams) (database.User, error) {
	u, ok := m.users[arg.ID]
	if !ok || u.OutletID != arg.OutletID || !u.IsActive {
		return database.User{}, pgx.ErrNoRows
	}
	// Check for duplicate email (simulates PostgreSQL unique constraint)
	for _, existing := range m.users {
		if existing.Email == arg.Email && existing.ID != arg.ID && existing.IsActive {
			return database.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
		}
	}
	u.Email = arg.Email
	u.FullName = arg.FullName
	u.Role = arg.Role
	u.Pin = arg.Pin
	m.users[u.ID] = u
	return u, nil
}

func (m *mockUserStore) SoftDeleteUser(_ context.Context, arg database.SoftDeleteUserParams) (uuid.UUID, error) {
	u, ok := m.users[arg.ID]
	if !ok || u.OutletID != arg.OutletID || !u.IsActive {
		return uuid.Nil, pgx.ErrNoRows
	}
	u.IsActive = false
	m.users[u.ID] = u
	return u.ID, nil
}

// --- Helpers ---

func setupUserRouter(store *mockUserStore) *chi.Mux {
	h := handler.NewUserHandler(store)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/users", h.RegisterRoutes)
	return r
}

func doRequest(t *testing.T, router http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeUserResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func decodeUserListResponse(t *testing.T, rr *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// --- List tests ---

func TestListUsers_Empty(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/users", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeUserListResponse(t, rr)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestListUsers_ReturnsOutletUsers(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	otherOutletID := uuid.New()

	store.users[uuid.New()] = database.User{
		ID: uuid.New(), OutletID: outletID, Email: "a@test.com",
		FullName: "Alice", Role: enum.UserRoleCashier, IsActive: true,
	}
	store.users[uuid.New()] = database.User{
		ID: uuid.New(), OutletID: otherOutletID, Email: "b@test.com",
		FullName: "Bob", Role: enum.UserRoleManager, IsActive: true,
	}

	router := setupUserRouter(store)
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/users", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeUserListResponse(t, rr)
	if len(resp) != 1 {
		t.Fatalf("expected 1 user, got %d", len(resp))
	}
	if resp[0]["email"] != "a@test.com" {
		t.Errorf("expected a@test.com, got %v", resp[0]["email"])
	}
}

func TestListUsers_ExcludesHashedPassword(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()

	store.users[uuid.New()] = database.User{
		ID: uuid.New(), OutletID: outletID, Email: "a@test.com",
		HashedPassword: "$2a$10$somehash", FullName: "Alice",
		Role: enum.UserRoleCashier, IsActive: true,
	}

	router := setupUserRouter(store)
	rr := doRequest(t, router, "GET", "/outlets/"+outletID.String()+"/users", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeUserListResponse(t, rr)
	if len(resp) != 1 {
		t.Fatalf("expected 1 user, got %d", len(resp))
	}
	if _, exists := resp[0]["hashed_password"]; exists {
		t.Error("response must not include hashed_password")
	}
}

func TestListUsers_InvalidOutletID(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)

	rr := doRequest(t, router, "GET", "/outlets/not-a-uuid/users", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Create tests ---

func TestCreateUser_Valid(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "new@test.com",
		"password":  "securepass",
		"full_name": "New User",
		"role":      "CASHIER",
		"pin":       "1234",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeUserResponse(t, rr)
	if resp["email"] != "new@test.com" {
		t.Errorf("email: got %v, want new@test.com", resp["email"])
	}
	if resp["full_name"] != "New User" {
		t.Errorf("full_name: got %v, want New User", resp["full_name"])
	}
	if resp["role"] != "CASHIER" {
		t.Errorf("role: got %v, want CASHIER", resp["role"])
	}
	if resp["pin"] != "1234" {
		t.Errorf("pin: got %v, want 1234", resp["pin"])
	}
}

func TestCreateUser_HashesPassword(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "hash@test.com",
		"password":  "plaintext-password",
		"full_name": "Hash Test",
		"role":      "CASHIER",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	// Find the created user in the mock store and verify the password was hashed.
	var found database.User
	for _, u := range store.users {
		if u.Email == "hash@test.com" {
			found = u
			break
		}
	}
	if found.ID == uuid.Nil {
		t.Fatal("user not found in store")
	}

	if found.HashedPassword == "plaintext-password" {
		t.Fatal("password was stored in plaintext; expected bcrypt hash")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(found.HashedPassword), []byte("plaintext-password")); err != nil {
		t.Errorf("stored hash does not match original password: %v", err)
	}
}

func TestCreateUser_ExcludesHashedPassword(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "nopass@test.com",
		"password":  "secret",
		"full_name": "No Pass In Response",
		"role":      "MANAGER",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusCreated)
	}

	resp := decodeUserResponse(t, rr)
	if _, exists := resp["hashed_password"]; exists {
		t.Error("response must not include hashed_password")
	}
	if _, exists := resp["password"]; exists {
		t.Error("response must not include password")
	}
}

func TestCreateUser_MissingFields(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email": "incomplete@test.com",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "bad@test.com",
		"password":  "secret",
		"full_name": "Bad Role",
		"role":      "ADMIN",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateUser_InvalidOutletID(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/not-a-uuid/users", map[string]string{
		"email":     "new@test.com",
		"password":  "secret",
		"full_name": "New User",
		"role":      "CASHIER",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Update tests ---

func TestUpdateUser_Valid(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	userID := uuid.New()

	store.users[userID] = database.User{
		ID:       userID,
		OutletID: outletID,
		Email:    "old@test.com",
		FullName: "Old Name",
		Role:     enum.UserRoleCashier,
		IsActive: true,
	}

	router := setupUserRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "updated@test.com",
		"full_name": "Updated Name",
		"role":      "MANAGER",
		"pin":       "5678",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeUserResponse(t, rr)
	if resp["email"] != "updated@test.com" {
		t.Errorf("email: got %v, want updated@test.com", resp["email"])
	}
	if resp["full_name"] != "Updated Name" {
		t.Errorf("full_name: got %v, want Updated Name", resp["full_name"])
	}
	if resp["role"] != "MANAGER" {
		t.Errorf("role: got %v, want MANAGER", resp["role"])
	}
}

func TestUpdateUser_NotFound(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()
	userID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "updated@test.com",
		"full_name": "Updated Name",
		"role":      "MANAGER",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestUpdateUser_WrongOutlet(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	wrongOutletID := uuid.New()
	userID := uuid.New()

	store.users[userID] = database.User{
		ID:       userID,
		OutletID: outletID,
		Email:    "old@test.com",
		FullName: "Old Name",
		Role:     enum.UserRoleCashier,
		IsActive: true,
	}

	router := setupUserRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+wrongOutletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "updated@test.com",
		"full_name": "Updated Name",
		"role":      "MANAGER",
	})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestUpdateUser_ExcludesHashedPassword(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	userID := uuid.New()

	store.users[userID] = database.User{
		ID:             userID,
		OutletID:       outletID,
		Email:          "old@test.com",
		HashedPassword: "$2a$10$somehash",
		FullName:       "Old Name",
		Role:           enum.UserRoleCashier,
		IsActive:       true,
	}

	router := setupUserRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "updated@test.com",
		"full_name": "Updated Name",
		"role":      "MANAGER",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	resp := decodeUserResponse(t, rr)
	if _, exists := resp["hashed_password"]; exists {
		t.Error("response must not include hashed_password")
	}
}

func TestUpdateUser_MissingFields(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()
	userID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email": "partial@test.com",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdateUser_InvalidRole(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()
	userID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "bad@test.com",
		"full_name": "Bad Role",
		"role":      "SUPERADMIN",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdateUser_InvalidUserID(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/not-a-uuid", map[string]string{
		"email":     "bad@test.com",
		"full_name": "Bad ID",
		"role":      "CASHIER",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Delete tests ---

func TestDeleteUser_Valid(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	userID := uuid.New()

	store.users[userID] = database.User{
		ID:       userID,
		OutletID: outletID,
		Email:    "delete@test.com",
		FullName: "Delete Me",
		Role:     enum.UserRoleCashier,
		IsActive: true,
	}

	router := setupUserRouter(store)

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/users/"+userID.String(), nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	// Verify the user is soft-deleted.
	u := store.users[userID]
	if u.IsActive {
		t.Error("expected user to be soft-deleted (is_active=false)")
	}
}

func TestDeleteUser_SoftDeleteDoesNotRemove(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	userID := uuid.New()

	store.users[userID] = database.User{
		ID:       userID,
		OutletID: outletID,
		Email:    "softdel@test.com",
		FullName: "Soft Delete",
		Role:     enum.UserRoleCashier,
		IsActive: true,
	}

	router := setupUserRouter(store)

	doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/users/"+userID.String(), nil)

	// User should still exist in store, just inactive.
	u, exists := store.users[userID]
	if !exists {
		t.Fatal("expected user to still exist in store after soft delete")
	}
	if u.IsActive {
		t.Error("expected is_active=false after soft delete")
	}
}

func TestDeleteUser_InvalidOutletID(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	userID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/not-a-uuid/users/"+userID.String(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDeleteUser_InvalidUserID(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/users/not-a-uuid", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- Create without PIN (optional field) ---

func TestCreateUser_WithoutPin(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "nopin@test.com",
		"password":  "secret",
		"full_name": "No Pin",
		"role":      "MANAGER",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	resp := decodeUserResponse(t, rr)
	if resp["pin"] != nil {
		// pin should be omitted (omitempty) when not set
		if pin, ok := resp["pin"].(string); ok && pin != "" {
			t.Errorf("expected empty/absent pin, got %v", resp["pin"])
		}
	}

	// Verify in store that pin is not valid.
	for _, u := range store.users {
		if u.Email == "nopin@test.com" {
			if u.Pin != (pgtype.Text{}) {
				t.Errorf("expected empty pgtype.Text for pin, got %+v", u.Pin)
			}
			return
		}
	}
	t.Fatal("user not found in store")
}

// --- Duplicate email tests (409 Conflict) ---

func TestCreateUser_DuplicateEmail(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()

	// Pre-populate a user with this email.
	store.users[uuid.New()] = database.User{
		ID: uuid.New(), OutletID: outletID, Email: "taken@test.com",
		FullName: "Existing", Role: enum.UserRoleCashier, IsActive: true,
	}

	router := setupUserRouter(store)

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "taken@test.com",
		"password":  "secret",
		"full_name": "Duplicate",
		"role":      "CASHIER",
	})

	if rr.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}

	resp := decodeUserResponse(t, rr)
	if resp["error"] != "email already exists" {
		t.Errorf("error: got %v, want 'email already exists'", resp["error"])
	}
}

func TestUpdateUser_DuplicateEmail(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()

	store.users[otherUserID] = database.User{
		ID: otherUserID, OutletID: outletID, Email: "taken@test.com",
		FullName: "Other", Role: enum.UserRoleCashier, IsActive: true,
	}
	store.users[userID] = database.User{
		ID: userID, OutletID: outletID, Email: "me@test.com",
		FullName: "Me", Role: enum.UserRoleCashier, IsActive: true,
	}

	router := setupUserRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "taken@test.com",
		"full_name": "Me Updated",
		"role":      "CASHIER",
	})

	if rr.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

// --- Delete non-existent user (404) ---

func TestDeleteUser_NotFound(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()
	userID := uuid.New()

	rr := doRequest(t, router, "DELETE", "/outlets/"+outletID.String()+"/users/"+userID.String(), nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// --- Email validation tests ---

func TestCreateUser_InvalidEmail(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "not-an-email",
		"password":  "secret",
		"full_name": "Bad Email",
		"role":      "CASHIER",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	resp := decodeUserResponse(t, rr)
	if resp["error"] != "invalid email format" {
		t.Errorf("error: got %v, want 'invalid email format'", resp["error"])
	}
}

func TestUpdateUser_InvalidEmail(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	userID := uuid.New()

	store.users[userID] = database.User{
		ID: userID, OutletID: outletID, Email: "old@test.com",
		FullName: "Old", Role: enum.UserRoleCashier, IsActive: true,
	}

	router := setupUserRouter(store)

	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "bad-email",
		"full_name": "Updated",
		"role":      "CASHIER",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	resp := decodeUserResponse(t, rr)
	if resp["error"] != "invalid email format" {
		t.Errorf("error: got %v, want 'invalid email format'", resp["error"])
	}
}

// --- PIN validation tests ---

func TestCreateUser_PinTooShort(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "pin@test.com",
		"password":  "secret",
		"full_name": "Pin Test",
		"role":      "CASHIER",
		"pin":       "12",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeUserResponse(t, rr)
	if resp["error"] != "PIN must be 4-6 digits" {
		t.Errorf("error: got %v, want 'PIN must be 4-6 digits'", resp["error"])
	}
}

func TestCreateUser_PinTooLong(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "pin@test.com",
		"password":  "secret",
		"full_name": "Pin Test",
		"role":      "CASHIER",
		"pin":       "1234567",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestCreateUser_PinNonDigits(t *testing.T) {
	store := newMockUserStore()
	router := setupUserRouter(store)
	outletID := uuid.New()

	rr := doRequest(t, router, "POST", "/outlets/"+outletID.String()+"/users", map[string]string{
		"email":     "pin@test.com",
		"password":  "secret",
		"full_name": "Pin Test",
		"role":      "CASHIER",
		"pin":       "12ab",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeUserResponse(t, rr)
	if resp["error"] != "PIN must be 4-6 digits" {
		t.Errorf("error: got %v, want 'PIN must be 4-6 digits'", resp["error"])
	}
}

func TestUpdateUser_PinValidation(t *testing.T) {
	store := newMockUserStore()
	outletID := uuid.New()
	userID := uuid.New()

	store.users[userID] = database.User{
		ID: userID, OutletID: outletID, Email: "pin@test.com",
		FullName: "Pin Test", Role: enum.UserRoleCashier, IsActive: true,
	}

	router := setupUserRouter(store)

	// Too short
	rr := doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "pin@test.com",
		"full_name": "Pin Test",
		"role":      "CASHIER",
		"pin":       "1",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("short pin: status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	// Non-digit
	rr = doRequest(t, router, "PUT", "/outlets/"+outletID.String()+"/users/"+userID.String(), map[string]string{
		"email":     "pin@test.com",
		"full_name": "Pin Test",
		"role":      "CASHIER",
		"pin":       "abcd",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("non-digit pin: status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
