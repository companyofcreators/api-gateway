package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/companyofcreators/api-gateway/internal/app"
	"github.com/companyofcreators/api-gateway/internal/config"
	"github.com/gookit/slog"
)

//go:embed docs/*
var docsFS embed.FS

func main() {
	cfg := config.Load()

	container := app.NewContainer(cfg, docsFS)

	serverErr := make(chan error, 1)
	go func() {
		slog.Infof("api gateway started on %s", cfg.HTTP.Address)

		if err := container.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	waitShutdown(container, serverErr)
}

func waitShutdown(container *app.Container, serverErr <-chan error) {

	quit := make(chan os.Signal, 1)

	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	select {
	case <-quit:
		slog.Info("shutting down api gateway")
	case err := <-serverErr:
		slog.Error("http server failed", "error", err)
	}

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
