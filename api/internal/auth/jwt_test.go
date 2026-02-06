package auth_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/kiwari-pos/api/internal/auth"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()
	outletID := uuid.New()
	role := "CASHIER"

	token, err := auth.GenerateToken(secret, userID, outletID, role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := auth.ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("user ID: got %v, want %v", claims.UserID, userID)
	}
	if claims.OutletID != outletID {
		t.Errorf("outlet ID: got %v, want %v", claims.OutletID, outletID)
	}
	if claims.Role != role {
		t.Errorf("role: got %v, want %v", claims.Role, role)
	}
}

func TestValidateTokenWithWrongSecret(t *testing.T) {
	userID := uuid.New()
	outletID := uuid.New()

	token, err := auth.GenerateToken("secret-a", userID, outletID, "CASHIER")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	_, err = auth.ValidateToken("secret-b", token)
	if err == nil {
		t.Fatal("expected error validating with wrong secret")
	}
}

func TestValidateTokenWithInvalidString(t *testing.T) {
	_, err := auth.ValidateToken("secret", "not-a-jwt")
	if err == nil {
		t.Fatal("expected error validating invalid token string")
	}
}
