package config

import (
	"os"

	"github.com/gookit/slog"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type HTTPConfig struct {
	Address string `env:"ADDRESS" env-required:"true"`
}

type RedisConfig struct {
	Address  string `env:"ADDRESS" env-required:"true"`
	DB       int    `env:"DB" env-default:"0"`
	Password string `env:"PASSWORD"`
}

type LogConfig struct {
	Level  string `env:"LEVEL" env-default:"info"`
	Format string `env:"FORMAT" env-default:"text"`
}

type RateLimitConfig struct {
	Limit         int `env:"LIMIT" env-default:"300"`
	WindowSeconds int `env:"WINDOW_SECONDS" env-default:"60"`
}

type Config struct {
	Env       string          `env:"ENV" env-default:"dev"`
	HTTP      HTTPConfig      `env-prefix:"HTTP_"`
	Redis     RedisConfig     `env-prefix:"REDIS_"`
	Log       LogConfig       `env-prefix:"LOG_"`
	RateLimit RateLimitConfig `env-prefix:"RATE_LIMIT_"`

	JWTPublicKeyPath string `env:"JWT_PUBLIC_KEY_PATH" env-default:"/app/keys/public.pem"`
	HeaderHMACKey    string `env:"HEADER_HMAC_KEY" env-default:"diploma-internal-hmac-secret-key-2026"`

	AuthServiceURL         string `env:"AUTH_SERVICE_URL" env-default:"http://auth-service:8081"`
	UserServiceURL         string `env:"USER_SERVICE_URL" env-default:"http://user-service:8082"`
	OrderServiceURL        string `env:"ORDER_SERVICE_URL" env-default:"http://order-service:8083"`
	OfferServiceURL        string `env:"OFFER_SERVICE_URL" env-default:"http://offer-service:8084"`
	ChatServiceURL         string `env:"CHAT_SERVICE_URL" env-default:"http://chat-service:8085"`
	FileServiceURL         string `env:"FILE_SERVICE_URL" env-default:"http://file-service:8086"`
	NotificationServiceURL string `env:"NOTIFICATION_SERVICE_URL" env-default:"http://notification-service:8087"`
}

func Load() *Config {
	_ = godotenv.Load(".env") // best-effort: .env may not exist in CWD

	// Set defaults for required vars if not provided (CWD may not have .env)
	setDefaultEnv("HTTP_ADDRESS", ":8080")
	setDefaultEnv("REDIS_ADDRESS", "localhost:6379")
	setDefaultEnv("REDIS_PASSWORD", "")
	setDefaultEnv("AUTH_SERVICE_URL", "http://localhost:8081")
	setDefaultEnv("USER_SERVICE_URL", "http://localhost:8082")
	setDefaultEnv("ORDER_SERVICE_URL", "http://localhost:8083")
	setDefaultEnv("OFFER_SERVICE_URL", "http://localhost:8084")
	setDefaultEnv("CHAT_SERVICE_URL", "http://localhost:8085")
	setDefaultEnv("FILE_SERVICE_URL", "http://localhost:8086")
	setDefaultEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8087")
	setDefaultEnv("JWT_PUBLIC_KEY_PATH", "/Users/ostapchichiginarov/Work/diploma/backend/keys/public.pem")

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	return &cfg
}

func setDefaultEnv(key, val string) {
	if os.Getenv(key) == "" {
		os.Setenv(key, val)
	}
}
