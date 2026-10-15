package api

import (
	"net/http"
	"strings"

	"github.com/rverhe/aiwx/internal/auth"
)

// authMiddleware validates the Authorization header and injects identity into context.
func authMiddleware(extractor *auth.Extractor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if extractor == nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication not configured")
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing authorization header")
				return
			}

			identity, err := extractor.ExtractFromHeader(r.Context(), authHeader)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials")
				return
			}

			ctx := auth.ContextWithIdentity(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// optionalAuthMiddleware attempts to authenticate but allows anonymous access.
func optionalAuthMiddleware(extractor *auth.Extractor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				identity, err := extractor.ExtractFromHeader(r.Context(), authHeader)
				if err == nil {
					ctx := auth.ContextWithIdentity(r.Context(), identity)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
