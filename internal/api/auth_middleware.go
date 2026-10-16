package api

import (
	"net/http"

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
