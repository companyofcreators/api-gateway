package app

import (
	"context"
	"embed"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/companyofcreators/api-gateway/internal/auth"
	"github.com/companyofcreators/api-gateway/internal/client"
	"github.com/companyofcreators/api-gateway/internal/config"
	"github.com/companyofcreators/api-gateway/internal/domain/ratelimit"
	ratelimitinfra "github.com/companyofcreators/api-gateway/internal/infrastructure/ratelimit"
	appmiddleware "github.com/companyofcreators/api-gateway/internal/middleware"
	"github.com/companyofcreators/api-gateway/internal/pkg/logger"
	"github.com/companyofcreators/api-gateway/internal/proxy"
	transporthttp "github.com/companyofcreators/api-gateway/internal/transport/http"
	"github.com/companyofcreators/api-gateway/pkg/header_auth"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/gookit/slog"
	"github.com/redis/go-redis/v9"
)

// Container holds all application dependencies and the HTTP server.
// It follows the service-locator / DI container pattern for the gateway.
type Container struct {
	Config       *config.Config
	Router       *chi.Mux
	HTTPServer   *http.Server
	RedisClient  *redis.Client
	RateLimiter  ratelimit.Limiter
	JWTValidator *auth.JWTValidator
	HeaderSigner *header_auth.HeaderSigner
	ReverseProxy *proxy.ReverseProxy
	UserClient   *client.UserClient
	OrderClient  *client.OrderClient
}

// NewContainer initializes all dependencies, wires up the middleware pipeline,
// registers routes, and returns a ready-to-use Container.
func NewContainer(cfg *config.Config, docsFS embed.FS) *Container {
	logger.Init(cfg)

	// ---- Redis ----
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Fatalf("failed to connect to redis: %s", err)
	}
	slog.Info("redis connected")

	// ---- Rate Limiter ----
	rateLimiter := ratelimitinfra.NewRedisSlidingWindowLimiter(redisClient)

	// ---- JWT Validator ----
	jwtValidator, err := auth.NewJWTValidator(cfg.JWTPublicKeyPath)
	if err != nil {
		slog.Fatalf("failed to initialize JWT validator: %s", err)
	}
	slog.Info("jwt validator initialized")

	// ---- Header Signer ----
	headerSigner := header_auth.NewHeaderSigner(cfg.HeaderHMACKey)
	slog.Info("header signer initialized")

	// ---- Reverse Proxy ----
	routeMap := map[string]string{
		"/api/v1/auth/":          cfg.AuthServiceURL,
		"/api/v1/users/":         cfg.UserServiceURL,
		"/api/v1/masters/":       cfg.UserServiceURL,
		"/api/v1/orders/":        cfg.OrderServiceURL,
		"/api/v1/categories/":    cfg.OrderServiceURL,
		"/api/v1/reviews/":       cfg.OrderServiceURL,
		"/api/v1/complaints/":    cfg.OrderServiceURL,
		"/api/v1/offers/":        cfg.OfferServiceURL,
		"/api/v1/chat/":          cfg.ChatServiceURL,
		"/api/v1/chats/":         cfg.ChatServiceURL,
		"/api/v1/files/":         cfg.FileServiceURL,
		"/api/v1/notifications/": cfg.NotificationServiceURL,
		"/api/v1/admin/":         cfg.AuthServiceURL,
	}
	proxyHTTPClient := &http.Client{Timeout: 30 * time.Second}
	reverseProxy := proxy.NewReverseProxy(routeMap, proxyHTTPClient)

	// ---- Service Clients ----
	userClient := client.NewUserClient(cfg.UserServiceURL)
	orderClient := client.NewOrderClient(cfg.OrderServiceURL)

	// ---- Router & Middleware Pipeline ----
	router := chi.NewRouter()

	// Middleware order matters:
	// RequestID -> RealIP -> Recovery -> Logging -> RateLimit -> Auth -> Router
	router.Use(appmiddleware.RequestID)  // custom: UUID-based request ID
	router.Use(chimiddleware.RealIP)     // trust X-Forwarded-For / X-Real-IP
	router.Use(corsMiddleware)           // allow local network access
	router.Use(appmiddleware.Recovery)   // custom: panic recovery with structured logging
	router.Use(appmiddleware.Logging)    // custom: structured request logging
	router.Use(bodySizeLimiter(50 << 20)) // 50MB body limit for file uploads
	router.Use(chimiddleware.Timeout(30 * time.Second))
	router.Use(appmiddleware.NewRateLimitMiddleware(rateLimiter, appmiddleware.RateLimitConfig{
		Limit:  cfg.RateLimit.Limit,
		Window: time.Duration(cfg.RateLimit.WindowSeconds) * time.Second,
	}))
	router.Use(appmiddleware.Auth(jwtValidator, headerSigner)) // JWT validation via cookie
	router.Use(transporthttp.APIMiddleware(reverseProxy))      // proxy /api/v1/*

	// Register routes. Admin routes use With() — safe here because NotFound
	// is not used for /api/v1/ paths (APIMiddleware handles them first).
	registerRoutes(router, reverseProxy, userClient, orderClient, jwtValidator, docsFS, redisClient)

	// ---- WebSocket proxy routes ----
	// These use httputil.ReverseProxy which supports WebSocket upgrade natively.
	// APIMiddleware skips these paths, so the router handles them directly.
	chatWSURL, _ := url.Parse(cfg.ChatServiceURL)
	notifyWSURL, _ := url.Parse(cfg.NotificationServiceURL)
	orderWSURL, _ := url.Parse(cfg.OrderServiceURL)
	offerWSURL, _ := url.Parse(cfg.OfferServiceURL)

	chatWSProxy := httputil.NewSingleHostReverseProxy(chatWSURL)
	notifyWSProxy := httputil.NewSingleHostReverseProxy(notifyWSURL)
	orderWSProxy := httputil.NewSingleHostReverseProxy(orderWSURL)
	offerWSProxy := httputil.NewSingleHostReverseProxy(offerWSURL)

	router.Get("/api/v1/chat/ws", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/ws"
		chatWSProxy.ServeHTTP(w, r)
	})

	router.Get("/api/v1/notifications/ws", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/ws"
		notifyWSProxy.ServeHTTP(w, r)
	})

	router.Get("/api/v1/orders/ws", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/ws"
		orderWSProxy.ServeHTTP(w, r)
	})

	router.Get("/api/v1/offers/ws", func(w http.ResponseWriter, r *http.Request) {
		// Pass query params through (order_id is in the URL query)
		r.URL.Path = "/ws"
		offerWSProxy.ServeHTTP(w, r)
	})

	// Admin routes: each goes to the correct backend service via direct proxy.
	router.With(appmiddleware.RequireRoles("admin")).
		Delete("/api/v1/admin/users/{id}",
			reverseProxy.ProxyTo(cfg.AuthServiceURL, "/api/v1/admin/", "/api/v1/admin/"))

	router.With(appmiddleware.RequireRoles("moderator", "admin")).
		Patch("/api/v1/admin/complaints/{id}",
			reverseProxy.ProxyTo(cfg.OrderServiceURL, "/api/v1/admin/", "/internal/"))

	router.With(appmiddleware.RequireRoles("admin")).
		Post("/api/v1/admin/users/{id}/ban",
			reverseProxy.ProxyTo(cfg.AuthServiceURL, "/api/v1/admin/", "/api/v1/admin/"))

	router.With(appmiddleware.RequireRoles("admin")).
		Post("/api/v1/admin/users/{id}/unban",
			reverseProxy.ProxyTo(cfg.AuthServiceURL, "/api/v1/admin/", "/api/v1/admin/"))

	// ---- HTTP Server ----
	httpServer := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Container{
		Config:       cfg,
		Router:       router,
		HTTPServer:   httpServer,
		RedisClient:  redisClient,
		RateLimiter:  rateLimiter,
		JWTValidator: jwtValidator,
		HeaderSigner: headerSigner,
		ReverseProxy: reverseProxy,
		UserClient:   userClient,
		OrderClient:  orderClient,
	}
}

// registerRoutes sets up all route groups with appropriate middleware.
func registerRoutes(
	router *chi.Mux,
	rp *proxy.ReverseProxy,
	userClient *client.UserClient,
	orderClient *client.OrderClient,
	jwtValidator *auth.JWTValidator,
	docsFS embed.FS,
	redisClient *redis.Client,
) {
	// Register all routes. Admin routes use inline middleware to avoid
	// chi's Group/With which resets NotFound handler.
	transporthttp.RegisterRoutes(router, rp, userClient, orderClient, docsFS, redisClient)
}

// bodySizeLimiter returns middleware that wraps http.MaxBytesReader to limit
// request body size and prevent memory exhaustion attacks.
func bodySizeLimiter(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware allows cross-origin requests from the local network.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			// Allow localhost (any port) and 192.168.0.103 (any port, http or https)
			isLocalhost := strings.Contains(origin, "//localhost:") || strings.Contains(origin, "//127.0.0.1:")
			originLower := strings.ToLower(origin)
			isLocalNetwork := strings.Contains(originLower, "//192.168.0.103") || strings.Contains(originLower, "//192.168.0.103:")
			if isLocalhost || isLocalNetwork {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
