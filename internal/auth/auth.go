// Package auth provides authentication and identity for AIWX.
package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Context keys for request identity.
type contextKey string

const (
	UserIDKey         contextKey = "user_id"
	OrganizationIDKey contextKey = "organization_id"
	RolesKey          contextKey = "roles"
	AuthTypeKey       contextKey = "auth_type"
)

// AuthType represents how the request was authenticated.
type AuthType string

const (
	AuthTypeOIDC          AuthType = "oidc"
	AuthTypeServiceAccount AuthType = "service_account"
)

// Identity represents an authenticated principal.
type Identity struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Roles          []string
	AuthType       AuthType
}

// FromContext extracts the identity from context.
func FromContext(ctx context.Context) (*Identity, bool) {
	id := &Identity{}
	if v, ok := ctx.Value(UserIDKey).(uuid.UUID); ok {
		id.UserID = v
	} else {
		return nil, false
	}
	if v, ok := ctx.Value(OrganizationIDKey).(uuid.UUID); ok {
		id.OrganizationID = v
	}
	if v, ok := ctx.Value(RolesKey).([]string); ok {
		id.Roles = v
	}
	if v, ok := ctx.Value(AuthTypeKey).(AuthType); ok {
		id.AuthType = v
	}
	return id, true
}

// ContextWithIdentity stores identity in context.
func ContextWithIdentity(ctx context.Context, id *Identity) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, id.UserID)
	ctx = context.WithValue(ctx, OrganizationIDKey, id.OrganizationID)
	ctx = context.WithValue(ctx, RolesKey, id.Roles)
	ctx = context.WithValue(ctx, AuthTypeKey, id.AuthType)
	return ctx
}

// ─── JWT Validation ───────────────────────────────────────────────────────────

// JWTValidator validates OIDC tokens.
type JWTValidator struct {
	secret []byte
	issuer string
}

// NewJWTValidator creates a validator with the given secret.
func NewJWTValidator(secret, issuer string) *JWTValidator {
	return &JWTValidator{
		secret: []byte(secret),
		issuer: issuer,
	}
}

// Claims represents the JWT claims we care about.
type Claims struct {
	jwt.RegisteredClaims
	OrganizationID string   `json:"org_id"`
	Roles          []string `json:"roles"`
}

// Validate parses and validates a JWT token string.
func (v *JWTValidator) Validate(tokenString string) (*Identity, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject: %w", err)
	}

	identity := &Identity{
		UserID:   userID,
		Roles:    claims.Roles,
		AuthType: AuthTypeOIDC,
	}

	if claims.OrganizationID != "" {
		orgID, err := uuid.Parse(claims.OrganizationID)
		if err != nil {
			return nil, fmt.Errorf("invalid org_id: %w", err)
		}
		identity.OrganizationID = orgID
	}

	return identity, nil
}

// ─── Service Accounts ─────────────────────────────────────────────────────────

// ServiceAccountStore handles service account token validation.
type ServiceAccountStore interface {
	// LookupByTokenHash finds a service account by its token hash.
	LookupByTokenHash(ctx context.Context, hash string) (*ServiceAccount, error)
	// Touch updates the last_used_at timestamp.
	Touch(ctx context.Context, id uuid.UUID) error
}

// ServiceAccount represents an API token.
type ServiceAccount struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	TokenHash      string
	TokenPrefix    string
	ExpiresAt      *time.Time
	IsActive       bool
}

// TokenHasher hashes service account tokens.
type TokenHasher struct {
	salt []byte
}

// NewTokenHasher creates a hasher with the given salt.
func NewTokenHasher(salt string) *TokenHasher {
	return &TokenHasher{salt: []byte(salt)}
}

// Hash returns the SHA-256 hash of a token.
func (h *TokenHasher) Hash(token string) string {
	salted := append(h.salt, token...)
	sum := sha256.Sum256(salted)
	return hex.EncodeToString(sum[:])
}

// Verify checks a token against a hash using constant-time comparison.
func (h *TokenHasher) Verify(token, hash string) bool {
	computed := h.Hash(token)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1
}

// GenerateToken generates a new service account token with prefix.
func GenerateToken() (token, hash, prefix string, err error) {
	raw := uuid.New().String() + uuid.New().String()
	prefix = "aiwx_" + raw[:8]
	token = prefix + "_" + raw
	sum := sha256.Sum256([]byte(token))
	hash = hex.EncodeToString(sum[:])
	return token, hash, prefix, nil
}

// ─── Extractor ────────────────────────────────────────────────────────────────

// Extractor extracts identity from HTTP requests.
type Extractor struct {
	jwt      *JWTValidator
	store    ServiceAccountStore
	hasher   *TokenHasher
}

// NewExtractor creates an identity extractor.
func NewExtractor(jwt *JWTValidator, store ServiceAccountStore, hasher *TokenHasher) *Extractor {
	return &Extractor{jwt: jwt, store: store, hasher: hasher}
}

// ExtractFromHeader extracts identity from an Authorization header.
func (e *Extractor) ExtractFromHeader(ctx context.Context, authHeader string) (*Identity, error) {
	if authHeader == "" {
		return nil, errors.New("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, errors.New("invalid authorization header format")
	}

	token := parts[1]

	// Try JWT first (OIDC tokens are typically longer)
	if strings.Count(token, ".") == 2 {
		return e.jwt.Validate(token)
	}

	// Fall back to service account token
	return e.validateServiceAccount(ctx, token)
}

func (e *Extractor) validateServiceAccount(ctx context.Context, token string) (*Identity, error) {
	hash := e.hasher.Hash(token)
	sa, err := e.store.LookupByTokenHash(ctx, hash)
	if err != nil {
		return nil, errors.New("invalid service account token")
	}

	if !sa.IsActive {
		return nil, errors.New("service account is inactive")
	}
	if sa.ExpiresAt != nil && time.Now().After(*sa.ExpiresAt) {
		return nil, errors.New("service account token expired")
	}

	// Update last used (fire and forget)
	go e.store.Touch(context.Background(), sa.ID)

	return &Identity{
		OrganizationID: sa.OrganizationID,
		AuthType:       AuthTypeServiceAccount,
		Roles:          []string{"service-account"},
	}, nil
}
