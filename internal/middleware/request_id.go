package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// RequestID generates a UUID for each incoming request, sets the
// X-Request-ID header on the response, and stores the ID in the
// request context so downstream handlers and middlewares can retrieve it.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()

		w.Header().Set("X-Request-ID", id)

		ctx := context.WithValue(r.Context(), RequestIDKey, id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context.
// Returns an empty string if no request ID was set.
func GetRequestID(ctx context.Context) string {
	id, ok := ctx.Value(RequestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}
