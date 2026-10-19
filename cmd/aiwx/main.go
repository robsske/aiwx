package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rverhe/aiwx/internal/api"
	"github.com/rverhe/aiwx/internal/auth"
	"github.com/rverhe/aiwx/internal/dispatch"
	"github.com/rverhe/aiwx/internal/events"
	"github.com/rverhe/aiwx/internal/metrics"
	"github.com/rverhe/aiwx/internal/outbox"
	"github.com/rverhe/aiwx/internal/rbac"
	"github.com/rverhe/aiwx/internal/reconcile"
	"github.com/rverhe/aiwx/internal/repository"
	"github.com/rverhe/aiwx/internal/security"
	"github.com/rverhe/aiwx/internal/service"
	"github.com/rverhe/aiwx/internal/workflow"
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

	syncAdapter := api.NewGitSyncAdapter(repos.Projects, repos.Credentials, logger)

	encSecret := os.Getenv("CRED_ENC_KEY")
	if encSecret == "" {
		encSecret = cfg.JWTSecret
	}
	encryptor, err := security.NewEncryptor(encSecret)
	if err != nil {
		logger.Error("credential encryptor init failed", "error", err)
		os.Exit(1)
	}
	repos.SetCredentialCrypto(encryptor)

	authLimiter := security.NewRateLimiter(10, time.Minute)

	// ─── NATS JetStream + outbox relay ─────────────────────────────────────────
	var nat *events.Client
	natsReady := func() bool { return false }
	if os.Getenv("AIWX_ALLOW_NO_NATS") != "1" {
		nat, err = events.NewClient(events.Config{URL: cfg.NATSURL})
		if err != nil {
			logger.Error("failed to connect to NATS", "url", cfg.NATSURL, "error", err)
			os.Exit(1)
		}
		defer nat.Close()
		natsReady = nat.IsConnected

		natPub := events.NewNATSPublisher(nat)
		relayPublisher := outbox.PublishFunc(func(ctx context.Context, e outbox.Event) error {
			metrics.NATSPublishedTotal.Inc()
			if err := natPub.Publish(ctx, events.OutboxEvent{
				ID:             e.ID,
				EventType:      e.EventType,
				AggregateType:  e.AggregateType,
				AggregateID:    e.AggregateID,
				Payload:        e.Payload,
				IdempotencyKey: e.IdempotencyKey,
				CreatedAt:      e.CreatedAt,
			}); err != nil {
				metrics.RecordOutboxFailed()
				return err
			}
			metrics.RecordOutboxPublished()
			return nil
		})

		outboxInterval := durationEnv("AIWX_OUTBOX_INTERVAL", time.Second)
		outboxBatch := intEnv("AIWX_OUTBOX_BATCH", 50)
		relay := outbox.NewRelay(outboxStore, relayPublisher, outboxBatch, outboxInterval, logger)
		go func() {
			logger.Info("starting outbox relay",
				"interval", outboxInterval, "batch", outboxBatch)
			if err := relay.Run(context.Background()); err != nil {
				logger.Error("outbox relay stopped", "error", err)
			}
		}()
	} else {
		logger.Warn("NATS disabled (AIWX_ALLOW_NO_NATS=1); jobs rely on fallback poll only")
	}

	// ─── Job execution ─────────────────────────────────────────────────────────
	ansiblePath := os.Getenv("ANSIBLE_PLAYBOOK")
	if ansiblePath == "" {
		ansiblePath = "/home/rverhe/aiwx/.venv/bin/ansible-playbook"
	}
	artifactsDir := os.Getenv("AIWX_ARTIFACTS_DIR")
	if artifactsDir == "" {
		artifactsDir = "/tmp/aiwx-artifacts"
	}
	runner := service.NewLocalRunner(ansiblePath, artifactsDir, repos.JobLogs)
	jobStore := service.NewJobStore(repos.Jobs, outboxStore)
	jobTimeout := durationEnv("AIWX_JOB_TIMEOUT", 30*time.Minute)
	exec := service.NewExecutor(db.Pool, jobStore, runner, logger, jobTimeout)

	// ─── Workflow runner ───────────────────────────────────────────────────────
	wfRunner := workflow.NewRunner(repos.Workflows, repos.Jobs, outboxStore, logger)

	router := api.NewRouter(api.Deps{
		Logger:         logger,
		DB:             db,
		Repos:          repos,
		Extractor:      extractor,
		Issuer:         tokenIssuer,
		Outbox:         outboxStore,
		Checker:        checker,
		SyncWorker:     syncAdapter,
		Config:         cfg,
		Encryptor:      encryptor,
		AuthLimiter:    authLimiter.Middleware,
		NATSReady:      natsReady,
		WorkflowRunner: wfRunner,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// NATS dispatcher (primary job execution path)
	if nat != nil {
		dispatcher := dispatch.NewConsumer(nat, exec, logger)
		go func() {
			logger.Info("starting NATS job dispatcher", "durable", "aiwx-dispatcher")
			if err := dispatcher.Run(ctx); err != nil && err != context.Canceled {
				logger.Error("NATS job dispatcher failed", "error", err)
			}
		}()

		wfConsumer := workflow.NewEventConsumer(nat, wfRunner, logger)
		go func() {
			logger.Info("starting workflow event consumer", "durable", "aiwx-workflow")
			if err := wfConsumer.Run(ctx); err != nil && err != context.Canceled {
				logger.Error("workflow event consumer failed", "error", err)
			}
		}()
	}

	// Slow fallback pollers (NATS outage / lag safety net)
	fallbackInterval := durationEnv("AIWX_WORKER_POLL_FALLBACK", 30*time.Second)
	poller := service.NewFallbackPoller(exec, fallbackInterval, logger)
	go func() {
		if err := poller.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("fallback job poller failed", "error", err)
		}
	}()
	go func() {
		if err := wfRunner.RunFallback(ctx); err != nil && err != context.Canceled {
			logger.Error("workflow fallback failed", "error", err)
		}
	}()

	// Reconciler marks stale running jobs as failed after 30 min
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
	NATSURL     string
}

func loadConfig() Config {
	cfg := Config{
		Port:        8080,
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		NATSURL:     os.Getenv("NATS_URL"),
	}
	if cfg.NATSURL == "" {
		cfg.NATSURL = "nats://localhost:4222"
	}
	if port := os.Getenv("PORT"); port != "" {
		_, _ = fmt.Sscanf(port, "%d", &cfg.Port)
	}

	if cfg.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "FATAL: DATABASE_URL is required")
		os.Exit(1)
	}
	if cfg.JWTSecret == "" {
		fmt.Fprintln(os.Stderr, "FATAL: JWT_SECRET is required")
		os.Exit(1)
	}
	if len(cfg.JWTSecret) < 32 && os.Getenv("AIWX_ALLOW_WEAK_JWT") != "1" {
		fmt.Fprintln(os.Stderr, "FATAL: JWT_SECRET must be at least 32 bytes (set AIWX_ALLOW_WEAK_JWT=1 to override in dev)")
		os.Exit(1)
	}

	return cfg
}

func durationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
