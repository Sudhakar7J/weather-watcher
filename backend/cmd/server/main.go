// Command server runs the Weather Watcher API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/incubrix/weather-watcher/backend/internal/cache"
	"github.com/incubrix/weather-watcher/backend/internal/config"
	"github.com/incubrix/weather-watcher/backend/internal/httpapi"
	"github.com/incubrix/weather-watcher/backend/internal/weather"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger.Info("starting weather watcher api",
		"addr", cfg.Addr,
		"cache_ttl", cfg.CacheTTL.String(),
		"max_cities", cfg.MaxCities,
		"allowed_origins", cfg.AllowedOrigins,
	)

	// Wire the layers: client -> cache -> service -> HTTP.
	client := weather.NewClient()
	cacheLayer := cache.New[weather.Report](cfg.CacheTTL)
	service := weather.NewService(client, cacheLayer)

	handler := httpapi.New(service, cfg.AllowedOrigins,
		httpapi.WithLogger(logger),
		httpapi.WithMaxCities(cfg.MaxCities),
		httpapi.WithRequestTimeout(cfg.RequestTimeout),
	)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
		// Timeouts guard against slow clients. Without ReadHeaderTimeout a
		// single idle connection can hold a goroutine open indefinitely.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Listen for shutdown signals so in-flight requests can finish. App
	// Runner sends SIGTERM during a deployment.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received", "grace_period", cfg.ShutdownTimeout.String())
	}

	// Stop accepting connections, then wait for active requests. A second
	// signal during this window force-exits via the deferred stop().
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Shutdown timed out with requests still running; report it rather
		// than exiting silently.
		return errors.Join(errors.New("graceful shutdown failed"), err)
	}

	logger.Info("shutdown complete")
	return nil
}
