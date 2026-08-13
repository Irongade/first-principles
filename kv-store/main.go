package main

import (
	"context"
	"errors"
	"kvstore/api"
	"kvstore/config"
	"kvstore/storage"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	err := godotenv.Load()

	if err != nil {
		logger.Warn("Error loading .env file, proceeding with defaults")
	}

	cfg := config.Load(logger)
	cfg.LogSummary(logger)
	cfg.Validate(logger)

	store, err := storage.NewFileStore(cfg)

	if err != nil {
		logger.Error("Create store failed to work", "error", err)
		os.Exit(1)
	}

	httpEngine := api.NewEngine(store, logger)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           httpEngine.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("Server started", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case signal := <-signals:
		logger.Info("shutdown signal received", "signal", signal)

	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP shutdown failed", "error", err)
	}

	if err := store.Close(); err != nil {
		logger.Error("store close failed", "error", err)
	}
}
