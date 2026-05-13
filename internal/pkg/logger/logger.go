package logger

import (
	config "github.com/companyofcreators/api-gateway/internal/config"
	"github.com/gookit/slog"
)

func Init(cfg *config.Config) {
	level := slog.LevelByName(cfg.Log.Level)

	if cfg.Log.Format == "json" {
		slog.SetFormatter(slog.NewJSONFormatter())
	}

	slog.SetLogLevel(level)
}
