package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/auth"
	"github.com/rverhe/aiwx/internal/rbac"
)

// ─── Context Keys ─────────────────────────────────────────────────────────────

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	OrgIDKey     contextKey = "org_id"
)

// RequestID returns the request ID from context.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// ─── Middleware ───────────────────────────────────────────────────────────────

// chain applies middleware to a handler in reverse order.
func chain(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// requestIDMiddleware adds a unique request ID to each request.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingMiddleware logs each request.
func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestID(r.Context()),
			)
		})
	}
}

// recoveryMiddleware recovers from panics.
func recoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						"error", err,
						"stack", string(debug.Stack()),
						"request_id", RequestID(r.Context()),
					)
					writeJSON(w, http.StatusInternalServerError, map[string]string{
						"error_code": "INTERNAL_ERROR",
						"message":    "An internal error occurred",
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// statusWriter captures the response status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error_code": code,
		"message":    message,
	})
}

// ─── RBAC Middleware ──────────────────────────────────────────────────────────

// PermissionChecker is the interface used by Require middleware.
type PermissionChecker interface {
	Check(ctx context.Context, userID uuid.UUID, action rbac.Action, resource rbac.Resource) (bool, error)
}

// Require returns middleware that checks the caller has the given action on the resource type.
// OrganizationID is taken from the auth context (primary org); TeamID is nil (org-level check).
func Require(checker PermissionChecker, action rbac.Action, resourceType rbac.ResourceType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.FromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
				return
			}

			resource := rbac.Resource{
				Type:           resourceType,
				OrganizationID: identity.OrganizationID,
			}

			allowed, err := checker.Check(r.Context(), identity.UserID, action, resource)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Permission check failed")
				return
			}
			if !allowed {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// wrapHandler wraps a handler with middleware (convenience for route registration).
func wrapHandler(h http.HandlerFunc, middleware ...func(http.Handler) http.Handler) http.Handler {
	return chain(h, middleware...)
}
