package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/kiwari-pos/api/internal/auth"
	"github.com/kiwari-pos/api/internal/middleware"
)

const testSecret = "test-secret"

func TestAuthMiddleware_ValidToken(t *testing.T) {
	userID := uuid.New()
	outletID := uuid.New()
	token, _ := auth.GenerateToken(testSecret, userID, outletID, "CASHIER")

	handler := middleware.Authenticate(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.ClaimsFromContext(r.Context())
		if claims == nil {
			t.Fatal("expected claims in context")
		}
		if claims.UserID != userID {
			t.Errorf("user ID: got %v, want %v", claims.UserID, userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	handler := middleware.Authenticate(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	handler := middleware.Authenticate(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestRequireOutlet_MatchingOutlet(t *testing.T) {
	outletID := uuid.New()
	token, _ := auth.GenerateToken(testSecret, uuid.New(), outletID, "CASHIER")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(testSecret)(middleware.RequireOutlet(inner))

	req := httptest.NewRequest("GET", "/outlets/"+outletID.String()+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("oid", outletID.String())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRequireOutlet_MismatchedOutlet(t *testing.T) {
	outletID := uuid.New()
	otherOutletID := uuid.New()
	token, _ := auth.GenerateToken(testSecret, uuid.New(), outletID, "CASHIER")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	handler := middleware.Authenticate(testSecret)(middleware.RequireOutlet(inner))

	req := httptest.NewRequest("GET", "/outlets/"+otherOutletID.String()+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("oid", otherOutletID.String())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestRequireOutlet_OwnerBypassesCheck(t *testing.T) {
	outletID := uuid.New()
	otherOutletID := uuid.New()
	token, _ := auth.GenerateToken(testSecret, uuid.New(), outletID, "OWNER")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(testSecret)(middleware.RequireOutlet(inner))

	req := httptest.NewRequest("GET", "/outlets/"+otherOutletID.String()+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("oid", otherOutletID.String())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d (OWNER should bypass outlet check)", rr.Code, http.StatusOK)
	}
}

func TestRequireRole(t *testing.T) {
	token, _ := auth.GenerateToken(testSecret, uuid.New(), uuid.New(), "CASHIER")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// CASHIER trying to access OWNER-only endpoint
	handler := middleware.Authenticate(testSecret)(middleware.RequireRole("OWNER", "MANAGER")(inner))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusForbidden)
	}
}
