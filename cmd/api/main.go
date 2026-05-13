package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/companyofcreators/api-gateway/internal/app"
	"github.com/companyofcreators/api-gateway/internal/config"
	"github.com/gookit/slog"
)

func main() {
	cfg := config.Load()

	container := app.NewContainer(cfg)

	go func() {
		slog.Infof("api gateway started on %s", cfg.HTTP.Address)

		if err := container.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Fatalf("http server failed: %v", err)
		}
	}()

	waitShutdown(container)
}

func waitShutdown(container *app.Container) {

	quit := make(chan os.Signal, 1)

	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	<-quit

	slog.Info("shutting down api gateway")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	defer cancel()

	// shutdown http server
	if err := container.HTTPServer.Shutdown(ctx); err != nil {

		slog.Errorf(
			"http server shutdown failed: %v",
			err,
		)
	}

	// close redis
	if err := container.RedisClient.Close(); err != nil {

		slog.Errorf(
			"redis close failed: %v",
			err,
		)
	}

	slog.Info("api gateway stopped")
}
