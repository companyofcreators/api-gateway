package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/companyofcreators/api-gateway/internal/auth"
	"github.com/gookit/slog"
)

// context keys for storing user info extracted from JWT claims.
// contextKey is defined in request_id.go.
const (
	UserIDKey    contextKey = "user_id"
	UserEmailKey contextKey = "user_email"
	UserRoleKey  contextKey = "user_role"
)

// PublicPaths returns the set of paths that do NOT require authentication.
// These paths are accessible without a valid JWT.
func PublicPaths() map[string]bool {
	return map[string]bool{
		"/health":                    true,
		"/api/v1/auth/register":      true,
		"/api/v1/auth/login":         true,
		"/api/v1/auth/refresh":       true,
		"/api/v1/auth/logout":        true,
	}
}

// Auth returns a middleware that:
//   - Extracts the JWT from the "access_token" cookie
//   - Validates the JWT using the provided JWTValidator
//   - On success: sets X-User-Id, X-User-Email, X-User-Role headers on the request
//   - Strips the Authorization header if present (security measure)
//   - On failure: returns 401 JSON error
//   - Skips authentication for public routes
func Auth(validator *auth.JWTValidator) func(http.Handler) http.Handler {
	publicPaths := PublicPaths()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Strip Authorization header unconditionally for security —
			// internal services must never see a client-supplied Authorization header.
			r.Header.Del("Authorization")

			// Skip authentication for public routes.
			if publicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Check path prefix for public API routes (handles paths with dynamic segments
			// like /api/v1/auth/login, /api/v1/auth/register, etc.)
			for publicPath := range publicPaths {
				if strings.HasPrefix(r.URL.Path, publicPath+"/") || r.URL.Path == publicPath {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract JWT from the "access_token" cookie.
			cookie, err := r.Cookie("access_token")
			if err != nil {
				writeAuthError(w, r, "missing access_token cookie")
				return
			}

			tokenString := cookie.Value
			if tokenString == "" {
				writeAuthError(w, r, "empty access_token cookie")
				return
			}

			// Validate the JWT.
			claims, err := validator.Validate(tokenString)
			if err != nil {
				slog.Warn("jwt validation failed",
					"error", err,
					"request_id", GetRequestID(r.Context()),
					"path", r.URL.Path,
				)
				writeAuthError(w, r, "invalid or expired token")
				return
			}

			// Set internal headers that downstream services can trust.
			userID := claims.Subject
			r.Header.Set("X-User-Id", userID)
			r.Header.Set("X-User-Email", claims.Email)
			r.Header.Set("X-User-Role", claims.Role)

			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	resp := map[string]string{
		"error":      message,
		"request_id": GetRequestID(r.Context()),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
