package http

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/companyofcreators/api-gateway/internal/aggregator"
	"github.com/companyofcreators/api-gateway/internal/client"
	"github.com/companyofcreators/api-gateway/internal/proxy"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(
	r *chi.Mux,
	rp *proxy.ReverseProxy,
	userClient *client.UserClient,
	orderClient *client.OrderClient,
	docsFS embed.FS,
) {
	r.Get("/health", healthHandler)

	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/scalar.html", http.StatusMovedPermanently)
	})

	docsStatic, err := fs.Sub(docsFS, "docs")
	if err != nil {
		panic("failed to mount docs static files: " + err.Error())
	}
	docsFileServer := http.FileServer(http.FS(docsStatic))
	r.Handle("/docs/*", http.StripPrefix("/docs/", docsFileServer))

	r.Get("/api/v1/profile", aggregator.UserProfileHandler(userClient, orderClient))

	// All unmatched paths → reverse proxy (chi NotFound is reliable here).
	r.NotFound(rp.Handler().ServeHTTP)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// APIMiddleware intercepts /api/v1/* (except profile and admin) before chi routing.
// All chi middlewares (auth, rate-limit) have already run at this point.
func APIMiddleware(rp *proxy.ReverseProxy) func(http.Handler) http.Handler {
	proxyHandler := rp.Handler()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/v1/") &&
				r.URL.Path != "/api/v1/profile" &&
				!strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
				w.Header().Set("X-Proxied", "true")
				proxyHandler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
