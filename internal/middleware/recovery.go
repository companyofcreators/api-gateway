package middleware

import (
	"encoding/json"
	"net/http"
	"runtime/debug"

	"github.com/gookit/slog"
)

// Recovery is a middleware that recovers from panics in downstream handlers.
// It logs the stack trace and returns a 500 Internal Server Error JSON response
// instead of crashing the server.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}

			stack := debug.Stack()

			requestID := GetRequestID(r.Context())

			slog.Error("panic recovered",
				"panic", rec,
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"stack", string(stack),
			)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)

			resp := map[string]string{
				"error":  "внутренняя ошибка сервера",
				"request_id": requestID,
			}

			_ = json.NewEncoder(w).Encode(resp)
		}()

		next.ServeHTTP(w, r)
	})
}
