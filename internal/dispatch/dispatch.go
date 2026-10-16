// Package dispatch handles job dispatch to Kubernetes.
package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/domain"
	"github.com/rverhe/aiwx/internal/outbox"
)

// JobStateTransition represents a job state machine transition.
type JobStateTransition struct {
	From   domain.JobStatus
	To     domain.JobStatus
	Owner  string // "api", "scheduler", "dispatcher", "reconciler"
	Reason string
}

// StateMachine defines legal job state transitions.
var StateMachine = map[domain.JobStatus][]domain.JobStatus{
	domain.JobStatusNew: {
		domain.JobStatusPending,
		domain.JobStatusCanceled,
	},
	domain.JobStatusPending: {
		domain.JobStatusWaiting,
		domain.JobStatusCanceled,
	},
	domain.JobStatusWaiting: {
		domain.JobStatusRunning,
		domain.JobStatusCanceled,
		domain.JobStatusError,
	},
	domain.JobStatusRunning: {
		domain.JobStatusSuccessful,
		domain.JobStatusFailed,
		domain.JobStatusError,
		domain.JobStatusCanceled,
	},
	// Terminal states have no transitions
	domain.JobStatusSuccessful: {},
	domain.JobStatusFailed:     {},
	domain.JobStatusError:      {},
	domain.JobStatusCanceled:   {},
}

// IsValidTransition checks if a state transition is legal.
func IsValidTransition(from, to domain.JobStatus) bool {
	allowed, ok := StateMachine[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// TransitionOwner returns the authoritative owner for a transition.
func TransitionOwner(from, to domain.JobStatus) string {
	switch {
	case to == domain.JobStatusCanceled:
		return "scheduler"
	case from == domain.JobStatusNew && to == domain.JobStatusPending:
		return "api"
	case from == domain.JobStatusPending && to == domain.JobStatusWaiting:
		return "scheduler"
	case from == domain.JobStatusWaiting && to == domain.JobStatusRunning:
		return "reconciler"
	case from == domain.JobStatusRunning && to.Terminal():
		return "reconciler"
	default:
		return "unknown"
	}
}

// ─── Job Repository Interface ─────────────────────────────────────────────────

// JobRepository is the interface for job state operations.
type JobRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.JobStatus, reason string) error
	ListNonTerminal(ctx context.Context) ([]domain.Job, error)
}

// ─── Kubernetes Client Interface ──────────────────────────────────────────────

// KubeClient is the interface for Kubernetes operations.
type KubeClient interface {
	// CreateJob creates a Kubernetes Job for a job run.
	CreateJob(ctx context.Context, job *domain.Job, spec JobSpec) (string, error)
	// GetJobStatus returns the status of a Kubernetes Job.
	GetJobStatus(ctx context.Context, k8sJobName string) (*KubeJobStatus, error)
	// DeleteJob deletes a Kubernetes Job.
	DeleteJob(ctx context.Context, k8sJobName string) error
}

// JobSpec contains the specification for launching a job.
type JobSpec struct {
	Image              string
	Playbook           string
	ProjectArtifactRef string
	CredentialSecrets  map[string]string
	InventoryContent   string
	ExtraVars          map[string]any
}

// KubeJobStatus represents the status of a Kubernetes Job.
type KubeJobStatus struct {
	Active    int
	Succeeded int
	Failed    int
	Ready     bool
}

// ─── Dispatcher ───────────────────────────────────────────────────────────────

// Dispatcher consumes job events and creates Kubernetes Jobs.
type Dispatcher struct {
	jobs      JobRepository
	kube      KubeClient
	outbox    *outbox.Store
	logger    *slog.Logger
	batchSize int
}

// NewDispatcher creates a new dispatcher.
func NewDispatcher(jobs JobRepository, kube KubeClient, obs *outbox.Store, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		jobs:      jobs,
		kube:      kube,
		outbox:    obs,
		logger:    logger,
		batchSize: 10,
	}
}

// HandleJobWaiting processes a job in waiting state and creates a Kubernetes Job.
func (d *Dispatcher) HandleJobWaiting(ctx context.Context, jobID uuid.UUID) error {
	job, err := d.jobs.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	if job.Status != domain.JobStatusWaiting {
		return fmt.Errorf("job %s is not in waiting state (current: %s)", jobID, job.Status)
	}

	// Build job spec
	spec := JobSpec{
		Playbook:           "site.yml", // TODO: from job template
		ProjectArtifactRef: *job.ArtifactRef,
		ExtraVars:          map[string]any{},
	}

	// Create Kubernetes Job
	k8sJobName, err := d.kube.CreateJob(ctx, job, spec)
	if err != nil {
		// Transition to error state
		if tErr := d.jobs.UpdateStatus(ctx, jobID, domain.JobStatusError, fmt.Sprintf("failed to create k8s job: %v", err)); tErr != nil {
			d.logger.Error("failed to update job status", "job_id", jobID, "error", tErr)
		}
		return fmt.Errorf("create k8s job: %w", err)
	}

	// Update job with k8s job name and transition to running
	if err := d.jobs.UpdateStatus(ctx, jobID, domain.JobStatusRunning, "k8s job created: "+k8sJobName); err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	d.logger.Info("job dispatched",
		"job_id", jobID,
		"k8s_job", k8sJobName,
	)

	return nil
}

// ─── Reconciler ───────────────────────────────────────────────────────────────

// Reconciler compares job records with Kubernetes Job status and repairs stale state.
type Reconciler struct {
	jobs     JobRepository
	kube     KubeClient
	outbox   *outbox.Store
	logger   *slog.Logger
	interval time.Duration
}

// NewReconciler creates a new reconciler.
func NewReconciler(jobs JobRepository, kube KubeClient, obs *outbox.Store, logger *slog.Logger, interval time.Duration) *Reconciler {
	return &Reconciler{
		jobs:     jobs,
		kube:     kube,
		outbox:   obs,
		logger:   logger,
		interval: interval,
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
			if err := r.Reconcile(ctx); err != nil {
				r.logger.Error("reconcile error", "error", err)
			}
		}
	}
}

// Reconcile performs a single reconciliation pass.
func (r *Reconciler) Reconcile(ctx context.Context) error {
	jobs, err := r.jobs.ListNonTerminal(ctx)
	if err != nil {
		return fmt.Errorf("list non-terminal jobs: %w", err)
	}

	for _, job := range jobs {
		if err := r.reconcileJob(ctx, &job); err != nil {
			r.logger.Error("failed to reconcile job",
				"job_id", job.ID,
				"error", err,
			)
		}
	}

	return nil
}

func (r *Reconciler) reconcileJob(ctx context.Context, job *domain.Job) error {
	// Skip jobs without a k8s job name (not yet dispatched)
	if job.KubernetesJobName == nil {
		// Check if job is stuck in waiting for too long
		if job.Status == domain.JobStatusWaiting && time.Since(job.UpdatedAt) > 5*time.Minute {
			return r.transitionToError(ctx, job.ID, "job stuck in waiting state (no k8s job created)")
		}
		return nil
	}

	// Get k8s job status
	status, err := r.kube.GetJobStatus(ctx, *job.KubernetesJobName)
	if err != nil {
		// Job might be deleted - check if it's been too long
		if time.Since(job.UpdatedAt) > 10*time.Minute {
			return r.transitionToError(ctx, job.ID, fmt.Sprintf("k8s job not found: %v", err))
		}
		return nil
	}

	// Determine target status based on k8s job status
	var targetStatus domain.JobStatus
	var reason string

	switch {
	case status.Succeeded > 0:
		targetStatus = domain.JobStatusSuccessful
		reason = "job completed successfully"
	case status.Failed > 0:
		targetStatus = domain.JobStatusFailed
		reason = fmt.Sprintf("job failed (%d failures)", status.Failed)
	case status.Active > 0:
		targetStatus = domain.JobStatusRunning
		reason = "job is active"
	default:
		// No active, no success, no failure - might be pending
		return nil
	}

	// Only transition if the status actually changed
	if job.Status == targetStatus {
		return nil
	}

	// Validate transition
	if !IsValidTransition(job.Status, targetStatus) {
		r.logger.Warn("invalid state transition attempted",
			"job_id", job.ID,
			"from", job.Status,
			"to", targetStatus,
		)
		return nil
	}

	// Perform transition
	if err := r.jobs.UpdateStatus(ctx, job.ID, targetStatus, reason); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	r.logger.Info("job state reconciled",
		"job_id", job.ID,
		"from", job.Status,
		"to", targetStatus,
		"reason", reason,
	)

	return nil
}

func (r *Reconciler) transitionToError(ctx context.Context, jobID uuid.UUID, reason string) error {
	return r.jobs.UpdateStatus(ctx, jobID, domain.JobStatusError, reason)
}
