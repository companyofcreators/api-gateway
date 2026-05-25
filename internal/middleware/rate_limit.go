package middleware

import (
	"github.com/gookit/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/companyofcreators/api-gateway/internal/domain/ratelimit"
)

type RateLimitConfig struct {
	Limit  int
	Window time.Duration
}

func NewRateLimitMiddleware(
	limiter ratelimit.Limiter,
	cfg RateLimitConfig,
) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isWebSocketUpgrade(r) {
				next.ServeHTTP(w, r)
				return
			}

			ip := realIP(r)

			key := "rate_limit:ip:" + ip

			result, err := limiter.Allow(
				r.Context(),
				key,
				cfg.Limit,
				cfg.Window,
			)

			// fail-open strategy
			if err != nil {

				slog.Error(
					"rate limit failed",
					"error", err,
				)

				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set(
				"RateLimit-Limit",
				strconv.Itoa(cfg.Limit),
			)

			w.Header().Set(
				"RateLimit-Remaining",
				strconv.Itoa(result.Remaining),
			)

			w.Header().Set(
				"RateLimit-Reset",
				strconv.FormatInt(result.ResetAt.Unix(), 10),
			)

			if !result.Allowed {

				w.Header().Set(
					"Content-Type",
					"application/json",
				)

				w.WriteHeader(http.StatusTooManyRequests)

				if _, err := w.Write([]byte(
					`{"error":"слишком много запросов","message":"превышен лимит запросов, попробуйте позже"}`,
				)); err != nil {
					slog.Debug("rate limit write error", "error", err)
				}

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func realIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return r.RemoteAddr
	}

	return host
}
