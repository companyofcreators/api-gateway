package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gookit/slog"
)

// ReverseProxy handles routing incoming requests to the appropriate
// internal microservice based on path prefixes.
type ReverseProxy struct {
	routes map[string]*httputil.ReverseProxy
}

// NewReverseProxy creates a ReverseProxy from a map of path prefixes to
// target service URLs. Each key is a path prefix (e.g., "/api/v1/auth/")
// and each value is the base URL of the target service.
//
// Route mappings:
//
//	/api/v1/auth/          -> auth-service
//	/api/v1/users/         -> user-service
//	/api/v1/orders/        -> order-service
//	/api/v1/offers/        -> offer-service
//	/api/v1/chat/          -> chat-service
//	/api/v1/files/         -> file-service
//	/api/v1/notifications/ -> notification-service
func NewReverseProxy(routes map[string]string) *ReverseProxy {
	proxyRoutes := make(map[string]*httputil.ReverseProxy, len(routes))

	for prefix, target := range routes {
		targetURL, err := url.Parse(target)
		if err != nil {
			slog.Errorf("invalid target URL for prefix %s: %v", prefix, err)
			continue
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// Preserve the original request's Host header, as well as headers
		// set by upstream middleware (X-User-Id, X-User-Email, X-User-Role, X-Request-ID).
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = targetURL.Host
		}

		// Handle proxy errors.
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("proxy error",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
				"target", target,
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"bad gateway","message":"upstream service unavailable"}`))
		}

		proxyRoutes[prefix] = proxy
	}

	return &ReverseProxy{
		routes: proxyRoutes,
	}
}

// Handler returns an http.Handler that routes incoming requests by matching
// the longest path prefix against the configured routes. If no route matches,
// it returns a 404 JSON response.
func (rp *ReverseProxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Find the matching route by longest prefix match.
		var matchedProxy *httputil.ReverseProxy
		var matchedPrefix string

		for prefix, proxy := range rp.routes {
			if strings.HasPrefix(path, prefix) {
				if len(prefix) > len(matchedPrefix) {
					matchedPrefix = prefix
					matchedProxy = proxy
				}
			}
		}

		if matchedProxy == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found","message":"no route matches the requested path"}`))
			return
		}

		matchedProxy.ServeHTTP(w, r)
	})
}
