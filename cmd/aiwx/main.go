package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rverhe/aiwx/internal/api"
	"github.com/rverhe/aiwx/internal/auth"
	"github.com/rverhe/aiwx/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	db, err := repository.NewPool(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	repos := repository.New(db)

	if err := repository.SeedDevAdmin(context.Background(), repos, logger); err != nil {
		logger.Error("failed to seed dev admin", "error", err)
		os.Exit(1)
	}

	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret, "aiwx")
	tokenIssuer := auth.NewTokenIssuer(cfg.JWTSecret, "aiwx", 24*time.Hour)
	hasher := auth.NewTokenHasher(cfg.JWTSecret)
	extractor := auth.NewExtractor(jwtValidator, nil, hasher)

	router := api.NewRouter(api.Deps{
		Logger:    logger,
		DB:        db,
		Repos:     repos,
		Extractor: extractor,
		Issuer:    tokenIssuer,
		Config:    cfg,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("starting AIWX control plane", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}

// Config holds runtime configuration.
type Config struct {
	Port        int
	DatabaseURL string
	JWTSecret   string
}

func loadConfig() Config {
	cfg := Config{
		Port:        8080,
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if port := os.Getenv("PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Port)
	}
	return cfg
}
