package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenIssuer mints HMAC-signed JWTs for authenticated users.
type TokenIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewTokenIssuer creates an issuer with the given secret and TTL.
func NewTokenIssuer(secret, issuer string, ttl time.Duration) *TokenIssuer {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &TokenIssuer{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

// Issue creates a signed JWT for the given identity fields.
func (i *TokenIssuer) Issue(userID, orgID uuid.UUID, roles []string) (string, error) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
		OrganizationID: orgID.String(),
		Roles:          roles,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(i.secret)
}

// TTL returns the token lifetime.
func (i *TokenIssuer) TTL() time.Duration {
	return i.ttl
}
