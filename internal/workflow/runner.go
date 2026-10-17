package workflow

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/domain"
	"github.com/rverhe/aiwx/internal/events"
	"github.com/rverhe/aiwx/internal/outbox"
	"github.com/rverhe/aiwx/internal/repository"
)

// Runner advances workflow DAGs. Primary path is NATS job events;
// a slow loop unstuck nodes whose events were missed.
type Runner struct {
	workflows *repository.WorkflowRepo
	jobs      *repository.JobRepo
	outbox    *outbox.Store
	logger    *slog.Logger
	fallback  time.Duration
	maxWaves  int
}

// NewRunner creates a workflow runner.
func NewRunner(workflows *repository.WorkflowRepo, jobs *repository.JobRepo, ob *outbox.Store, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		workflows: workflows,
		jobs:      jobs,
		outbox:    ob,
		logger:    logger,
		fallback:  30 * time.Second,
		maxWaves:  128,
	}
}

// StartWorkflow marks the workflow running and launches the first wave.
func (r *Runner) StartWorkflow(ctx context.Context, workflowJobID uuid.UUID) error {
	if _, err := r.workflows.TryTransitionJobStatus(ctx, workflowJobID, "running"); err != nil {
		return err
	}
	return r.Advance(ctx, workflowJobID)
}

// OnJobEvent handles a terminal job lifecycle event by advancing affected workflows.
func (r *Runner) OnJobEvent(ctx context.Context, ev *events.Event) error {
	switch ev.Type {
	case events.JobSuccessful, events.JobFailed, events.JobError, events.JobCanceled:
	default:
		return nil
	}

	jobID := ev.AggregateID
	if jobID == uuid.Nil {
		if s, ok := ev.Payload["job_id"].(string); ok {
			if parsed, err := uuid.Parse(s); err == nil {
				jobID = parsed
			}
		}
	}
	if jobID == uuid.Nil {
		return nil
	}

	nodes, err := r.workflows.FindJobNodesByJobID(ctx, jobID)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	seen := map[uuid.UUID]bool{}
	for _, n := range nodes {
		status := nodeStatusForJobEvent(ev.Type)
		ok, err := r.workflows.TryTransitionNodeStatus(ctx, n.ID, status, &jobID, "")
		if err != nil {
			r.logger.Error("update workflow node failed", "node_id", n.ID, "error", err)
			continue
		}
		if ok {
			r.logger.Info("workflow node advanced from job event",
				"node_id", n.ID,
				"workflow_job_id", n.WorkflowJobID,
				"job_id", jobID,
				"node_status", status,
			)
		}
		if !seen[n.WorkflowJobID] {
			seen[n.WorkflowJobID] = true
			if err := r.Advance(ctx, n.WorkflowJobID); err != nil {
				r.logger.Error("workflow advance failed",
					"workflow_job_id", n.WorkflowJobID, "error", err)
			}
		}
	}
	return nil
}

// Advance plans the next wave (or completes the workflow).
func (r *Runner) Advance(ctx context.Context, workflowJobID uuid.UUID) error {
	for i := 0; i < r.maxWaves; i++ {
		job, err := r.workflows.GetJobByID(ctx, workflowJobID)
		if err != nil {
			return err
		}

		r.syncNodesFromJobs(ctx, job)

		job, err = r.workflows.GetJobByID(ctx, workflowJobID)
		if err != nil {
			return err
		}

		ready, anySkipped := r.plan(ctx, job)
		if anySkipped {
			continue
		}

		if len(ready) == 0 {
			allTerminal, anyFailed, done := summarize(job)
			_ = allTerminal
			if done {
				status := "successful"
				if anyFailed {
					status = "failed"
				}
				if _, err := r.workflows.TryTransitionJobStatus(ctx, workflowJobID, status); err != nil {
					return err
				}
				r.logger.Info("workflow completed",
					"workflow_job_id", workflowJobID, "status", status)
				return nil
			}
			// Waiting on in-flight jobs — event path / fallback will Advance again.
			return nil
		}

		for _, n := range ready {
			if n.JobTemplateID == nil {
				_, _ = r.workflows.TryTransitionNodeStatus(ctx, n.ID, "skipped", nil, "node has no job template")
				continue
			}
			newJob, err := r.jobs.CreateAndEnqueue(ctx, n.JobTemplateID, job.OrganizationID, domain.JobLaunch{}, r.outbox)
			if err != nil {
				r.logger.Error("workflow node job create failed", "error", err, "node", n.ID)
				_, _ = r.workflows.TryTransitionNodeStatus(ctx, n.ID, "failed", nil, err.Error())
				continue
			}
			_, _ = r.workflows.TryTransitionNodeStatus(ctx, n.ID, "running", &newJob.ID, "")
		}
	}
	r.logger.Warn("workflow advance hit wave cap", "workflow_job_id", workflowJobID)
	_, _ = r.workflows.TryTransitionJobStatus(ctx, workflowJobID, "error")
	return nil
}

func (r *Runner) syncNodesFromJobs(ctx context.Context, job *domain.WorkflowJob) {
	for _, n := range job.Nodes {
		if n.JobID == nil {
			continue
		}
		if n.Status != "running" && n.Status != "pending" {
			continue
		}
		j, err := r.jobs.GetByID(ctx, *n.JobID)
		if err != nil {
			continue
		}
		var status string
		switch string(j.Status) {
		case "successful":
			status = "success"
		case "failed", "error":
			status = "failed"
		case "canceled":
			status = "skipped"
		default:
			continue
		}
		if ok, err := r.workflows.TryTransitionNodeStatus(ctx, n.ID, status, nil, ""); err == nil && ok {
			r.logger.Info("workflow node unstuck from job status",
				"node_id", n.ID, "job_id", *n.JobID, "node_status", status)
		}
	}
}

// plan returns ready pending nodes and whether any node was skipped this pass.
func (r *Runner) plan(ctx context.Context, job *domain.WorkflowJob) (ready []domain.WorkflowJobNode, anySkipped bool) {
	dependsOn := map[uuid.UUID][]domain.WorkflowJobNodeLink{}
	for _, l := range job.Links {
		dependsOn[l.TargetNodeID] = append(dependsOn[l.TargetNodeID], l)
	}
	statusOf := map[uuid.UUID]string{}
	for _, n := range job.Nodes {
		statusOf[n.ID] = n.Status
	}

	for _, n := range job.Nodes {
		if n.Status != "pending" {
			continue
		}
		deps := dependsOn[n.ID]
		if len(deps) == 0 {
			ready = append(ready, n)
			continue
		}
		canRun := true
		allDone := true
		for _, l := range deps {
			src := statusOf[l.SourceNodeID]
			switch l.Condition {
			case "success":
				if src == "failed" || src == "skipped" || src == "error" {
					canRun = false
				} else if src != "success" && src != "successful" {
					canRun = false
					allDone = false
				}
			case "failure":
				if src == "success" || src == "successful" || src == "skipped" {
					canRun = false
				} else if src != "failed" && src != "error" {
					canRun = false
					allDone = false
				}
			case "always":
				if src == "pending" || src == "running" {
					canRun = false
					allDone = false
				}
			}
		}
		if !canRun && allDone {
			_, _ = r.workflows.TryTransitionNodeStatus(ctx, n.ID, "skipped", nil, "")
			anySkipped = true
		} else if canRun {
			ready = append(ready, n)
		}
	}
	return ready, anySkipped
}

func summarize(job *domain.WorkflowJob) (allTerminal, anyFailed, done bool) {
	allTerminal = true
	for _, n := range job.Nodes {
		switch n.Status {
		case "success", "successful", "skipped":
		case "failed", "error":
			anyFailed = true
		default:
			allTerminal = false
		}
	}
	return allTerminal, anyFailed, allTerminal
}

func nodeStatusForJobEvent(t events.EventType) string {
	switch t {
	case events.JobSuccessful:
		return "success"
	case events.JobFailed, events.JobError:
		return "failed"
	case events.JobCanceled:
		return "skipped"
	default:
		return "failed"
	}
}

// RunFallback periodically unstuck/advances active workflows (missed events).
func (r *Runner) RunFallback(ctx context.Context) error {
	ticker := time.NewTicker(r.fallback)
	defer ticker.Stop()
	r.logger.Info("starting workflow fallback loop", "interval", r.fallback)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ids, err := r.workflows.ListActiveWorkflowJobIDs(ctx)
			if err != nil {
				r.logger.Error("list active workflows failed", "error", err)
				continue
			}
			for _, id := range ids {
				if err := r.Advance(ctx, id); err != nil {
					r.logger.Error("workflow fallback advance failed",
						"workflow_job_id", id, "error", err)
				}
			}
		}
	}
}
