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
	"github.com/rverhe/aiwx/internal/outbox"
	"github.com/rverhe/aiwx/internal/rbac"
	"github.com/rverhe/aiwx/internal/reconcile"
	"github.com/rverhe/aiwx/internal/repository"
	"github.com/rverhe/aiwx/internal/service"
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

	// Seed admin only when explicitly requested (dev convenience).
	if os.Getenv("AIWX_SEED_ADMIN") == "1" {
		password := os.Getenv("SEED_ADMIN_PASSWORD")
		if password == "" {
			logger.Error("SEED_ADMIN_PASSWORD is required when AIWX_SEED_ADMIN=1")
			os.Exit(1)
		}
		if err := repository.SeedDevAdmin(context.Background(), repos, logger, password); err != nil {
			logger.Error("failed to seed dev admin", "error", err)
			os.Exit(1)
		}
	}

	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret, "aiwx")
	tokenIssuer := auth.NewTokenIssuer(cfg.JWTSecret, "aiwx", 24*time.Hour)
	hasher := auth.NewTokenHasher(cfg.JWTSecret)
	extractor := auth.NewExtractor(jwtValidator, nil, hasher)

	outboxStore := outbox.NewStore(db.Pool)
	checker := rbac.New(repos.RoleAssignments)

	// Set up project sync worker
	syncAdapter := api.NewGitSyncAdapter(repos.Projects, logger)

	router := api.NewRouter(api.Deps{
		Logger:     logger,
		DB:         db,
		Repos:      repos,
		Extractor:  extractor,
		Issuer:     tokenIssuer,
		Outbox:     outboxStore,
		Checker:    checker,
		SyncWorker: syncAdapter,
		Config:     cfg,
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

	// Start job worker with real ansible-playbook runner
	runner := service.NewLocalRunner(
		"/home/rverhe/aiwx/.venv/bin/ansible-playbook",
		"/tmp/aiwx-artifacts",
		repos.JobLogs,
	)
	worker := service.NewWorker(db.Pool, runner, logger)
	go func() {
		logger.Info("starting job worker", "ansible", "/home/rverhe/aiwx/.venv/bin/ansible-playbook")
		if err := worker.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("worker failed", "error", err)
		}
	}()

	// Start reconciler (marks stale running jobs as failed after 30 min)
	rec := reconcile.New(repos.Jobs, logger, 30*time.Minute)
	go func() {
		logger.Info("starting reconciler")
		if err := rec.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("reconciler failed", "error", err)
		}
	}()

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

	if cfg.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "FATAL: DATABASE_URL is required")
		os.Exit(1)
	}
	if cfg.JWTSecret == "" {
		fmt.Fprintln(os.Stderr, "FATAL: JWT_SECRET is required")
		os.Exit(1)
	}

	return cfg
}
