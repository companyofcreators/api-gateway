package http

import (
	"net/http"

	"github.com/companyofcreators/api-gateway/internal/aggregator"
	"github.com/companyofcreators/api-gateway/internal/client"
	"github.com/companyofcreators/api-gateway/internal/proxy"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes sets up all API routes on the given chi router.
//
// Route groups:
//   - Health:     GET  /health
//   - Auth:       POST /api/v1/auth/register, login, refresh, logout
//   - Aggregator: GET  /api/v1/profile (authenticated)
//   - Proxy:      all other /api/v1/* routes forwarded to internal services
//
// NOTE: Admin/moderator RBAC-protected routes are registered directly
// in container.go because they require the RequireRoles middleware.
func RegisterRoutes(
	r *chi.Mux,
	rp *proxy.ReverseProxy,
	userClient *client.UserClient,
	orderClient *client.OrderClient,
) {
	// Health check — always public.
	r.Get("/health", healthHandler)

	// Aggregated profile endpoint — authenticated users only.
	r.Get("/api/v1/profile", aggregator.UserProfileHandler(userClient, orderClient))

	// Catch-all: proxy all remaining /api/v1/* requests to internal services.
	// Admin and moderator routes with RBAC middleware are registered in container.go
	// and will match before this catch-all.
	r.Handle("/api/v1/*", rp.Handler())
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
