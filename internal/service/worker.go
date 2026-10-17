package service

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rverhe/aiwx/internal/domain"
)

// OutboxInserter writes outbox rows inside a transaction (repository.OutboxInserter).
type OutboxInserter interface {
	Insert(ctx context.Context, tx interface{ /* pgx.Tx */ }, eventType, aggregateType string, aggregateID uuid.UUID, payload map[string]any, idempotencyKey string) error
}

// JobRepository is the job persistence surface used by the executor.
// Concrete type is *repository.JobRepo; wired in cmd/aiwx via a thin adapter
// because repository.OutboxInserter takes a concrete pgx.Tx.
type JobRepository interface {
	ClaimPendingByID(ctx context.Context, id uuid.UUID) (*domain.Job, error)
	ClaimNextPending(ctx context.Context) (*domain.Job, error)
	FinishJob(ctx context.Context, id uuid.UUID, status domain.JobStatus, reason string) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error)
}

// Executor claims jobs and runs them via a JobRunner.
// Used by both the NATS dispatcher and the slow fallback poller.
type Executor struct {
	pool    *pgxpool.Pool
	jobs    JobRepository
	runner  JobRunner
	logger  *slog.Logger
	timeout time.Duration
}

// NewExecutor creates a job executor. pool is used for spec loading.
func NewExecutor(pool *pgxpool.Pool, jobs JobRepository, runner JobRunner, logger *slog.Logger, timeout time.Duration) *Executor {
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Executor{
		pool:    pool,
		jobs:    jobs,
		runner:  runner,
		logger:  logger,
		timeout: timeout,
	}
}

// ExecuteByID claims and runs a specific job. Claim is idempotent.
// Returns true when this call actually executed the job.
func (e *Executor) ExecuteByID(ctx context.Context, jobID uuid.UUID) (bool, error) {
	job, err := e.jobs.ClaimPendingByID(ctx, jobID)
	if err != nil {
		return false, err
	}
	if job == nil {
		return false, nil
	}
	e.run(ctx, job)
	return true, nil
}

// ExecuteNext claims and runs the oldest pending job (fallback poll path).
func (e *Executor) ExecuteNext(ctx context.Context) (bool, error) {
	job, err := e.jobs.ClaimNextPending(ctx)
	if err != nil {
		return false, err
	}
	if job == nil {
		return false, nil
	}
	e.run(ctx, job)
	return true, nil
}

func (e *Executor) run(ctx context.Context, job *domain.Job) {
	e.logger.Info("processing job", "job_id", job.ID, "template_id", job.JobTemplateID)

	spec := e.loadSpec(ctx, job)

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	e.logger.Info("running job",
		"job_id", job.ID,
		"playbook", spec.Playbook,
		"project_dir", spec.ProjectDir,
		"inventory", spec.Inventory,
	)

	result, err := e.runner.Run(runCtx, job, spec)
	if err != nil {
		e.logger.Error("job run failed", "job_id", job.ID, "error", err)
		if uerr := e.jobs.FinishJob(ctx, job.ID, domain.JobStatusFailed, "runner error: "+err.Error()); uerr != nil {
			e.logger.Error("finish job failed", "job_id", job.ID, "error", uerr)
		}
		return
	}

	if result.ExitCode == 0 {
		if uerr := e.jobs.FinishJob(ctx, job.ID, domain.JobStatusSuccessful, "exit 0"); uerr != nil {
			e.logger.Error("finish job failed", "job_id", job.ID, "error", uerr)
		}
		return
	}
	if uerr := e.jobs.FinishJob(ctx, job.ID, domain.JobStatusFailed, "non-zero exit"); uerr != nil {
		e.logger.Error("finish job failed", "job_id", job.ID, "error", uerr)
	}
}

func (e *Executor) loadSpec(ctx context.Context, job *domain.Job) *JobSpec {
	spec := &JobSpec{}

	if job.JobTemplateID != nil {
		var playbook *string
		var projectID *uuid.UUID
		var inventoryID *uuid.UUID
		var limit string
		var verbosity int
		var forks int
		var jobTags string
		var skipTags string

		err := e.pool.QueryRow(ctx,
			`SELECT playbook, project_id, inventory_id, limit_hosts, verbosity, forks, job_tags, skip_tags
			 FROM job_templates WHERE id = $1`,
			job.JobTemplateID,
		).Scan(&playbook, &projectID, &inventoryID, &limit, &verbosity, &forks, &jobTags, &skipTags)
		if err == nil {
			if playbook != nil {
				spec.Playbook = *playbook
			}
			spec.Limit = limit
			spec.Verbosity = verbosity
			spec.Forks = forks
			spec.JobTags = jobTags
			spec.SkipTags = skipTags

			if projectID != nil {
				var orgID uuid.UUID
				if err := e.pool.QueryRow(ctx,
					`SELECT organization_id FROM projects WHERE id = $1`, projectID,
				).Scan(&orgID); err == nil {
					projectDir := filepath.Join("/tmp", "aiwx-artifacts", orgID.String(), projectID.String())
					if _, err := os.Stat(projectDir); err == nil {
						spec.ProjectDir = projectDir
					}
				}
			}

			if inventoryID != nil {
				var invName *string
				if err := e.pool.QueryRow(ctx,
					`SELECT name FROM inventories WHERE id = $1`, inventoryID,
				).Scan(&invName); err == nil && invName != nil {
					spec.Inventory = *invName
				}
			}
		}
	}

	var jobExtraVars map[string]any
	err := e.pool.QueryRow(ctx,
		`SELECT extra_vars FROM jobs WHERE id = $1`, job.ID,
	).Scan(&jobExtraVars)
	if err == nil && jobExtraVars != nil {
		if spec.ExtraVars == nil {
			spec.ExtraVars = jobExtraVars
		} else {
			for k, v := range jobExtraVars {
				spec.ExtraVars[k] = v
			}
		}
	}

	return spec
}

// FallbackPoller periodically claims leftover pending jobs (NATS outage / lag).
type FallbackPoller struct {
	exec     *Executor
	interval time.Duration
	logger   *slog.Logger
}

// NewFallbackPoller creates a slow poller. Interval defaults to 30s.
func NewFallbackPoller(exec *Executor, interval time.Duration, logger *slog.Logger) *FallbackPoller {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &FallbackPoller{exec: exec, interval: interval, logger: logger}
}

// Run blocks until ctx is canceled.
func (p *FallbackPoller) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	p.logger.Info("starting fallback job poller", "interval", p.interval)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := p.exec.ExecuteNext(ctx); err != nil {
				p.logger.Error("fallback poller error", "error", err)
			}
		}
	}
}
