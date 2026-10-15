package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ─── JWT Tests ────────────────────────────────────────────────────────────────

func TestJWT_ValidateValidToken(t *testing.T) {
	secret := "test-secret"
	validator := NewJWTValidator(secret, "test-issuer")

	userID := uuid.New()
	orgID := uuid.New()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    "test-issuer",
		},
		OrganizationID: orgID.String(),
		Roles:          []string{"operator"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	identity, err := validator.Validate(tokenString)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if identity.UserID != userID {
		t.Errorf("UserID = %v, want %v", identity.UserID, userID)
	}
	if identity.OrganizationID != orgID {
		t.Errorf("OrganizationID = %v, want %v", identity.OrganizationID, orgID)
	}
	if len(identity.Roles) != 1 || identity.Roles[0] != "operator" {
		t.Errorf("Roles = %v, want [operator]", identity.Roles)
	}
	if identity.AuthType != AuthTypeOIDC {
		t.Errorf("AuthType = %v, want %v", identity.AuthType, AuthTypeOIDC)
	}
}

func TestJWT_ValidateExpiredToken(t *testing.T) {
	secret := "test-secret"
	validator := NewJWTValidator(secret, "test-issuer")

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = validator.Validate(tokenString)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestJWT_ValidateWrongSecret(t *testing.T) {
	validator := NewJWTValidator("correct-secret", "test-issuer")

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("wrong-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = validator.Validate(tokenString)
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

// ─── Service Account Token Tests ──────────────────────────────────────────────

func TestTokenHasher_Hash(t *testing.T) {
	hasher := NewTokenHasher("test-salt")

	token := "aiwx_test_token"
	hash := hasher.Hash(token)

	if hash == "" {
		t.Error("hash should not be empty")
	}
	if hash == token {
		t.Error("hash should not equal token")
	}
}

func TestTokenHasher_Verify(t *testing.T) {
	hasher := NewTokenHasher("test-salt")

	token := "aiwx_test_token"
	hash := hasher.Hash(token)

	if !hasher.Verify(token, hash) {
		t.Error("Verify should return true for matching token/hash")
	}

	if hasher.Verify("wrong-token", hash) {
		t.Error("Verify should return false for wrong token")
	}
}

func TestGenerateToken(t *testing.T) {
	token, hash, prefix, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Error("token should not be empty")
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if prefix == "" {
		t.Error("prefix should not be empty")
	}
	if len(prefix) < 5 {
		t.Errorf("prefix too short: %s", prefix)
	}

	// Verify hash matches token
	hasher := NewTokenHasher("")
	if !hasher.Verify(token, hash) {
		t.Error("generated hash should verify against generated token")
	}
}

// ─── Identity Context Tests ───────────────────────────────────────────────────

func TestContextWithIdentity(t *testing.T) {
	id := &Identity{
		UserID:         uuid.New(),
		OrganizationID: uuid.New(),
		Roles:          []string{"operator"},
		AuthType:       AuthTypeOIDC,
	}

	ctx := ContextWithIdentity(context.Background(), id)
	extracted, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext should return true")
	}

	if extracted.UserID != id.UserID {
		t.Errorf("UserID = %v, want %v", extracted.UserID, id.UserID)
	}
	if extracted.OrganizationID != id.OrganizationID {
		t.Errorf("OrganizationID = %v, want %v", extracted.OrganizationID, id.OrganizationID)
	}
	if extracted.AuthType != id.AuthType {
		t.Errorf("AuthType = %v, want %v", extracted.AuthType, id.AuthType)
	}
}

func TestFromContext_Empty(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("FromContext should return false for empty context")
	}
}
