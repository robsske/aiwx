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

// JobStore provides job lifecycle operations for the worker.
type JobStore interface {
	ClaimPending(ctx context.Context) (*domain.Job, error)
	MarkRunning(ctx context.Context, jobID string) error
	MarkFinished(ctx context.Context, jobID string, status domain.JobStatus, exitCode *int) error
	AddEvent(ctx context.Context, jobID string, eventType string, data map[string]any) error
}

// Worker polls for pending jobs and executes them.
type Worker struct {
	pool     *pgxpool.Pool
	runner   JobRunner
	logger   *slog.Logger
	interval time.Duration
	timeout  time.Duration
}

// NewWorker creates a job worker.
func NewWorker(pool *pgxpool.Pool, runner JobRunner, logger *slog.Logger) *Worker {
	return &Worker{
		pool:     pool,
		runner:   runner,
		logger:   logger,
		interval: 2 * time.Second,
		timeout:  30 * time.Minute,
	}
}

// Run starts the worker loop. Blocks until ctx is canceled.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.processOne(ctx); err != nil {
				w.logger.Error("worker error", "error", err)
			}
		}
	}
}

func (w *Worker) processOne(ctx context.Context) error {
	// Claim one pending job with FOR UPDATE SKIP LOCKED
	job, err := w.claimPending(ctx)
	if err != nil || job == nil {
		return err
	}

	w.logger.Info("processing job", "job_id", job.ID, "template_id", job.JobTemplateID)

	// Mark running
	if err := w.updateStatus(ctx, job.ID, domain.JobStatusRunning); err != nil {
		return err
	}

	// Load spec from template
	spec := w.loadSpec(ctx, job)

	// Run with timeout
	runCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	w.logger.Info("running job",
		"job_id", job.ID,
		"playbook", spec.Playbook,
		"project_dir", spec.ProjectDir,
		"inventory", spec.Inventory,
	)

	result, err := w.runner.Run(runCtx, job, spec)
	if err != nil {
		w.logger.Error("job run failed", "job_id", job.ID, "error", err)
		if uerr := w.updateStatus(ctx, job.ID, domain.JobStatusFailed); uerr != nil {
			w.logger.Error("update status failed", "job_id", job.ID, "error", uerr)
		}
		if aerr := w.addEvent(ctx, job.ID, "job.failed", map[string]any{"error": err.Error()}); aerr != nil {
			w.logger.Error("add event failed", "job_id", job.ID, "error", aerr)
		}
		return nil
	}

	// Write result
	if result.ExitCode == 0 {
		if uerr := w.updateStatus(ctx, job.ID, domain.JobStatusSuccessful); uerr != nil {
			w.logger.Error("update status failed", "job_id", job.ID, "error", uerr)
		}
		if aerr := w.addEvent(ctx, job.ID, "job.successful", map[string]any{"exit_code": result.ExitCode}); aerr != nil {
			w.logger.Error("add event failed", "job_id", job.ID, "error", aerr)
		}
	} else {
		if uerr := w.updateStatus(ctx, job.ID, domain.JobStatusFailed); uerr != nil {
			w.logger.Error("update status failed", "job_id", job.ID, "error", uerr)
		}
		if aerr := w.addEvent(ctx, job.ID, "job.failed", map[string]any{"exit_code": result.ExitCode}); aerr != nil {
			w.logger.Error("add event failed", "job_id", job.ID, "error", aerr)
		}
	}

	return nil
}

func (w *Worker) claimPending(ctx context.Context) (*domain.Job, error) {
	job := &domain.Job{}
	err := w.pool.QueryRow(ctx,
		`UPDATE jobs SET status = 'running', started_at = now(), updated_at = now()
		 WHERE id = (
		   SELECT id FROM jobs
		   WHERE status = 'pending'
		   ORDER BY created_at
		   FOR UPDATE SKIP LOCKED
		   LIMIT 1
		 )
		 RETURNING id, organization_id, job_template_id, status, created_at`,
	).Scan(&job.ID, &job.OrganizationID, &job.JobTemplateID, &job.Status, &job.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (w *Worker) updateStatus(ctx context.Context, jobID interface{ String() string }, status domain.JobStatus) error {
	_, err := w.pool.Exec(ctx,
		`UPDATE jobs SET status = $2, updated_at = now(),
		        finished_at = CASE WHEN $2 IN ('successful', 'failed', 'error', 'canceled') THEN now() ELSE finished_at END
		 WHERE id = $1`, jobID, status,
	)
	return err
}

func (w *Worker) addEvent(ctx context.Context, jobID interface{ String() string }, eventType string, data map[string]any) error {
	_, err := w.pool.Exec(ctx,
		`INSERT INTO job_events (job_id, event_type, event_data, created_at)
		 VALUES ($1, $2, $3, now())`, jobID, eventType, data,
	)
	return err
}

func (w *Worker) loadSpec(ctx context.Context, job *domain.Job) *JobSpec {
	spec := &JobSpec{}

	// Load from job template if present
	if job.JobTemplateID != nil {
		var playbook *string
		var projectID *uuid.UUID
		var inventoryID *uuid.UUID
		var limit string
		var verbosity int
		var forks int
		var jobTags string
		var skipTags string

		err := w.pool.QueryRow(ctx,
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

			// Resolve project directory from synced artifacts
			if projectID != nil {
				var orgID uuid.UUID
				if err := w.pool.QueryRow(ctx,
					`SELECT organization_id FROM projects WHERE id = $1`, projectID,
				).Scan(&orgID); err == nil {
					// Expected artifact path: /tmp/aiwx-artifacts/<orgID>/<projectID>/
					projectDir := filepath.Join("/tmp", "aiwx-artifacts", orgID.String(), projectID.String())
					if _, err := os.Stat(projectDir); err == nil {
						spec.ProjectDir = projectDir
					}
				}
			}

			// Load inventory path if set
			if inventoryID != nil {
				var invName *string
				if err := w.pool.QueryRow(ctx,
					`SELECT name FROM inventories WHERE id = $1`, inventoryID,
				).Scan(&invName); err == nil && invName != nil {
					spec.Inventory = *invName
				}
			}
		}
	}

	// Load extra vars from job
	var jobExtraVars map[string]any
	err := w.pool.QueryRow(ctx,
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
