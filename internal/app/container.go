package app

import (
	"context"
	"net/http"
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
	ReverseProxy *proxy.ReverseProxy
	UserClient   *client.UserClient
	OrderClient  *client.OrderClient
}

// NewContainer initializes all dependencies, wires up the middleware pipeline,
// registers routes, and returns a ready-to-use Container.
func NewContainer(cfg *config.Config) *Container {
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

	// ---- Reverse Proxy ----
	routeMap := map[string]string{
		"/api/v1/auth/":          cfg.AuthServiceURL,
		"/api/v1/users/":         cfg.UserServiceURL,
		"/api/v1/orders/":        cfg.OrderServiceURL,
		"/api/v1/offers/":        cfg.OfferServiceURL,
		"/api/v1/chat/":          cfg.ChatServiceURL,
		"/api/v1/files/":         cfg.FileServiceURL,
		"/api/v1/notifications/": cfg.NotificationServiceURL,
		"/api/v1/admin/":         cfg.AuthServiceURL, // admin routes go through auth-service for RBAC
	}
	reverseProxy := proxy.NewReverseProxy(routeMap)

	// ---- Service Clients ----
	userClient := client.NewUserClient(cfg.UserServiceURL)
	orderClient := client.NewOrderClient(cfg.OrderServiceURL)

	// ---- Router & Middleware Pipeline ----
	router := chi.NewRouter()

	// Middleware order matters:
	// RequestID -> RealIP -> Recovery -> Logging -> RateLimit -> Auth -> Router
	router.Use(appmiddleware.RequestID)      // custom: UUID-based request ID
	router.Use(chimiddleware.RealIP)          // trust X-Forwarded-For / X-Real-IP
	router.Use(appmiddleware.Recovery)       // custom: panic recovery with structured logging
	router.Use(appmiddleware.Logging)        // custom: structured request logging
	router.Use(chimiddleware.Timeout(30 * time.Second))
	router.Use(appmiddleware.NewRateLimitMiddleware(rateLimiter, appmiddleware.RateLimitConfig{
		Limit:  100,
		Window: time.Minute,
	}))
	router.Use(appmiddleware.Auth(jwtValidator)) // JWT validation via cookie

	// Register routes (with role-based access control for admin/moderator routes).
	registerRoutes(router, reverseProxy, userClient, orderClient, jwtValidator)

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
) {
	// Public routes are handled by the Auth middleware (it skips them).
	// Admin-only routes and moderator-only routes get additional RBAC middleware.
	router.Group(func(admin chi.Router) {
		admin.Use(appmiddleware.RequireRoles("admin"))
		admin.Delete("/api/v1/admin/users/{id}", rp.Handler().ServeHTTP)
	})

	router.Group(func(moderator chi.Router) {
		moderator.Use(appmiddleware.RequireRoles("moderator", "admin"))
		moderator.Put("/api/v1/admin/complaints/{id}", rp.Handler().ServeHTTP)
	})

	// Register all other routes via the transport layer.
	transporthttp.RegisterRoutes(router, rp, userClient, orderClient)
}
