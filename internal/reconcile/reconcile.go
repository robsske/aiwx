// Package reconcile provides background reconciliation for AIWX.
package reconcile

import (
	"context"
	"log/slog"
	"time"
)

// Store provides access to job state for reconciliation.
type Store interface {
	// FailStaleRunning marks running jobs older than maxAge as failed.
	FailStaleRunning(ctx context.Context, maxAge time.Duration) (int64, error)
}

// Reconciler runs periodic reconciliation tasks.
type Reconciler struct {
	store    Store
	logger   *slog.Logger
	interval time.Duration
	maxAge   time.Duration
}

// New creates a reconciler. Stale running jobs older than maxAge are marked failed.
func New(store Store, logger *slog.Logger, maxAge time.Duration) *Reconciler {
	return &Reconciler{
		store:    store,
		logger:   logger,
		interval: 30 * time.Second,
		maxAge:   maxAge,
	}
}

// Run starts the reconciler loop. Blocks until ctx is canceled.
func (r *Reconciler) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			r.reconcileOnce(ctx)
		}
	}
}

// ReconcileOnce runs a single reconciliation pass (useful for testing).
func (r *Reconciler) ReconcileOnce(ctx context.Context) {
	r.reconcileOnce(ctx)
}

func (r *Reconciler) reconcileOnce(ctx context.Context) {
	count, err := r.store.FailStaleRunning(ctx, r.maxAge)
	if err != nil {
		r.logger.Error("reconcile failed", "error", err)
		return
	}
	if count > 0 {
		r.logger.Info("reconciled stale jobs", "count", count)
	}
}
