package app

import (
	"context"
	"net/http"
	"time"

	"github.com/companyofcreators/api-gateway/internal/config"
	"github.com/companyofcreators/api-gateway/internal/domain/ratelimit"
	ratelimitinfra "github.com/companyofcreators/api-gateway/internal/infrastructure/ratelimit"
	appmiddleware "github.com/companyofcreators/api-gateway/internal/middleware"
	"github.com/companyofcreators/api-gateway/internal/pkg/logger"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/gookit/slog"
	"github.com/redis/go-redis/v9"
)

type Container struct {
	Config *config.Config

	Router *chi.Mux

	HTTPServer *http.Server

	RedisClient *redis.Client

	RateLimiter ratelimit.Limiter
}

func NewContainer(cfg *config.Config) *Container {
	logger.Init(cfg)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Fatalf("failed to connect to redis: %s", err)
	}

	slog.Info("Redis connected")

	rateLimiter := ratelimitinfra.NewRedisSlidingWindowLimiter(
		redisClient,
	)

	router := chi.NewRouter()

	registerMiddlewares(router, rateLimiter)

	registerRoutes(router)

	httpServer := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Container{
		Config:      cfg,
		Router:      router,
		HTTPServer:  httpServer,
		RedisClient: redisClient,
		RateLimiter: rateLimiter,
	}
}

func registerMiddlewares(router *chi.Mux, limiter ratelimit.Limiter) {
	router.Use(chimiddleware.RequestID)

	router.Use(chimiddleware.RealIP)

	router.Use(chimiddleware.Recoverer)

	router.Use(chimiddleware.Timeout(30 * time.Second))

	router.Use(
		appmiddleware.NewRateLimitMiddleware(
			limiter, appmiddleware.RateLimitConfig{
				Limit:  100,
				Window: time.Minute,
			},
		),
	)
}

func registerRoutes(router *chi.Mux) {
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{\"status\": \"ok\"}"))
	})
}
