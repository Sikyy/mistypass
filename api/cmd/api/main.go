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

	"github.com/mistypass/cloud/api/internal/config"
	httpx "github.com/mistypass/cloud/api/internal/http"
	"github.com/mistypass/cloud/api/internal/state"
)

func main() {
	logger := initLogger()

	cfg := config.FromEnv()
	if err := cfg.Validate(); err != nil {
		failFast(logger, "invalid config", err)
	}
	otelShutdown, err := setupOpenTelemetry(context.Background(), cfg)
	if err != nil {
		failFast(logger, "open telemetry init failed", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(ctx); err != nil {
			logger.Error("open telemetry shutdown failed", "err", err)
		}
	}()

	var stateStore state.Store
	var closeStateStore func() error
	if cfg.DatabaseURL != "" {
		pgStore, err := state.NewPostgresStore(cfg.DatabaseURL)
		if err != nil {
			failFast(logger, "postgres connect failed", err)
		}
		if cfg.DatabaseAutoMigrate {
			if err := pgStore.EnsureSchema(); err != nil {
				_ = pgStore.Close()
				failFast(logger, "postgres migrate failed", err)
			}
		}
		stateStore = pgStore
		closeStateStore = pgStore.Close
		logger.Info("postgres state store enabled")
	}
	if closeStateStore != nil {
		defer func() {
			if err := closeStateStore(); err != nil {
				logger.Error("postgres close failed", "err", err)
			}
		}()
	}

	router, stopWorkers, err := httpx.NewRouter(cfg, stateStore)
	if err != nil {
		failFast(logger, "router init failed", err)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stopSignals := make(chan os.Signal, 1)
	signal.Notify(stopSignals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("mistypass api listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failFast(logger, "server stopped with error", err)
		}
	}()

	<-stopSignals
	logger.Info("received shutdown signal, stopping workers...")
	stopWorkers()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		failFast(logger, "graceful shutdown failed", err)
	}
	logger.Info("server shut down gracefully")
}

func initLogger() *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	return logger
}

func failFast(logger *slog.Logger, message string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error(message, "err", err)
	os.Exit(1)
}
