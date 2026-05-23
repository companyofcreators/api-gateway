package http

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/companyofcreators/api-gateway/internal/aggregator"
	"github.com/companyofcreators/api-gateway/internal/client"
	"github.com/companyofcreators/api-gateway/internal/proxy"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func RegisterRoutes(
	r *chi.Mux,
	rp *proxy.ReverseProxy,
	userClient *client.UserClient,
	orderClient *client.OrderClient,
	docsFS embed.FS,
	redisClient *redis.Client,
) {
	r.Get("/health", healthHandler(redisClient))

	docsStatic, err := fs.Sub(docsFS, "docs")
	if err != nil {
		slog.Error("failed to mount docs static files", "error", err)
	} else {
		docsFileServer := http.FileServer(http.FS(docsStatic))
		r.Handle("/docs/*", http.StripPrefix("/docs/", docsFileServer))
		r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/docs/scalar.html", http.StatusMovedPermanently)
		})
	}

	r.Get("/api/v1/profile", aggregator.UserProfileHandler(userClient, orderClient))

	// All unmatched paths → reverse proxy (chi NotFound is reliable here).
	r.NotFound(rp.Handler().ServeHTTP)
}

func healthHandler(redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := "ok"
		httpStatus := http.StatusOK

		if err := redisClient.Ping(ctx).Err(); err != nil {
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		if _, err := w.Write([]byte(`{"status":"` + status + `"}`)); err != nil {
			slog.Debug("health check write error", "error", err)
		}
	}
}

// APIMiddleware intercepts /api/v1/* (except profile, admin, and websocket) before chi routing.
// All chi middlewares (auth, rate-limit) have already run at this point.
func APIMiddleware(rp *proxy.ReverseProxy) func(http.Handler) http.Handler {
	proxyHandler := rp.Handler()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/v1/") &&
				r.URL.Path != "/api/v1/profile" &&
				!strings.HasPrefix(r.URL.Path, "/api/v1/admin/") &&
				!isWebSocketPath(r.URL.Path) {
				w.Header().Set("X-Proxied", "true")
				proxyHandler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isWebSocketPath returns true for paths that need WebSocket upgrade handling.
func isWebSocketPath(path string) bool {
	return strings.HasSuffix(path, "/ws")
}
