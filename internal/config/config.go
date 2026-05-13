package config

import (
	"github.com/gookit/slog"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type HTTP struct {
	Address string `env:"ADDRESS" env-required:"true"`
}

type RedisConfig struct {
	Address  string `env:"ADDRESS" env-required:"true"`
	DB       int    `env:"DB" env-required:"true"`
	Password string `env:"PASSWORD" env-required:"true"`
}

type LogConfig struct {
	Level  string `env:"LEVEL" env-default:"info"`
	Format string `env:"FORMAT" env-default:"text"`
}

type Config struct {
	Env string `env:"ENV" env-default:"local"`

	HTTP HTTP `env-prefix:"HTTP_"`

	Redis RedisConfig `env-prefix:"REDIS_"`
	Log   LogConfig   `env-prefix:"LOG_"`
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		slog.Fatal("Error loading .env file", err)
	}

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("Error reading env", err)
		return nil
	}

	return &cfg
}
