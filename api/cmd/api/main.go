package main

import (
	"context"
	"errors"
	"log"
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
	cfg := config.FromEnv()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	var stateStore state.Store
	var closeStateStore func() error
	if cfg.DatabaseURL != "" {
		pgStore, err := state.NewPostgresStore(cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("postgres connect failed: %v", err)
		}
		if cfg.DatabaseAutoMigrate {
			if err := pgStore.EnsureSchema(); err != nil {
				_ = pgStore.Close()
				log.Fatalf("postgres migrate failed: %v", err)
			}
		}
		stateStore = pgStore
		closeStateStore = pgStore.Close
		log.Println("postgres state store enabled")
	}
	if closeStateStore != nil {
		defer func() {
			if err := closeStateStore(); err != nil {
				log.Printf("postgres close failed: %v", err)
			}
		}()
	}

	router, err := httpx.NewRouter(cfg, stateStore)
	if err != nil {
		log.Fatalf("router init failed: %v", err)
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
		log.Printf("mistypass api listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server stopped with error: %v", err)
		}
	}()

	<-stopSignals

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("server shut down gracefully")
}
