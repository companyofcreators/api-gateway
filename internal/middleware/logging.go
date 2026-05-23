package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/gookit/slog"
)

// responseWriter wraps http.ResponseWriter to capture the HTTP status code
// written by downstream handlers.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Hijack lets the caller take over the connection (required for WebSocket upgrades).
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("response writer does not support hijacking")
}

// Flush sends any buffered data to the client.
func (rw *responseWriter) Flush() {
	if fl, ok := rw.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

// Unwrap returns the underlying http.ResponseWriter.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Logging logs every incoming request with its method, path, status code,
// duration, and request_id. It uses structured logging via slog.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		requestID := GetRequestID(r.Context())

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", duration.Milliseconds(),
			"request_id", requestID,
			"remote_addr", r.RemoteAddr,
		)
	})
}
