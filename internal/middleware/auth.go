package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/companyofcreators/api-gateway/internal/auth"
	"github.com/companyofcreators/api-gateway/pkg/header_auth"
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
		"/docs":                      true,
		"/docs/":                     true,
		"/api/v1/auth/register":      true,
		"/api/v1/auth/login":         true,
		"/api/v1/auth/refresh":       true,
		"/api/v1/auth/verify-email":        true,
		"/api/v1/auth/resend-verification": true,
		"/api/v1/categories":               true,
	}
}

// Auth returns a middleware that:
//   - Extracts the JWT from the "access_token" cookie
//   - Validates the JWT using the provided JWTValidator
//   - On success: sets X-User-Id, X-User-Email, X-User-Role headers on the request
//   - Strips the Authorization header if present (security measure)
//   - On failure: returns 401 JSON error
//   - Skips authentication for public routes
func Auth(validator *auth.JWTValidator, signer *header_auth.HeaderSigner) func(http.Handler) http.Handler {
	publicPaths := PublicPaths()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Strip Authorization header unconditionally for security —
			// internal services must never see a client-supplied Authorization header.
			r.Header.Del("Authorization")

			// Skip authentication for public routes.
			// GET /api/v1/categories is public, but POST/PATCH/DELETE require auth (admin).
			if publicPaths[r.URL.Path] && !(r.URL.Path == "/api/v1/categories" && r.Method != http.MethodGet) {
				next.ServeHTTP(w, r)
				return
			}

			// Check path prefix for public API routes (handles paths with dynamic segments
			// like /api/v1/auth/login, /api/v1/auth/register, etc.)
			for publicPath := range publicPaths {
				if strings.HasPrefix(r.URL.Path, publicPath+"/") || r.URL.Path == publicPath {
					// GET /api/v1/categories (and sub-paths) is public; POST/PATCH/DELETE require auth.
					if (publicPath == "/api/v1/categories") && r.Method != http.MethodGet {
						break
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract JWT from cookie (HTTP requests) or query parameter
			// (WebSocket connections pass the token via ?token=).
			var tokenString string

			cookie, cookieErr := r.Cookie("access_token")
			if cookieErr == nil {
				tokenString = cookie.Value
			} else {
				tokenString = r.URL.Query().Get("token")
			}

			if tokenString == "" {
				writeAuthError(w, r, "отсутствует токен авторизации (cookie access_token или параметр token)")
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
				writeAuthError(w, r, "недействительный или истёкший токен")
				return
			}

			// Set internal headers that downstream services can trust.
			userID := claims.Subject
			r.Header.Set("X-User-Id", userID)
			r.Header.Set("X-User-Email", claims.Email)
			r.Header.Set("X-User-Role", claims.RoleString())

			// Sign internal headers so backend services can verify
			// that the headers came from the API gateway.
			signer.SignHeaders(r)

			// Also set context for handlers that use string keys (e.g., logout handler).
			ctx := context.WithValue(r.Context(), "user_id", userID)
			ctx = context.WithValue(ctx, "user_email", claims.Email)
			ctx = context.WithValue(ctx, "user_role", claims.RoleString())
			r = r.WithContext(ctx)

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

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode auth error response", "error", err)
	}
}
