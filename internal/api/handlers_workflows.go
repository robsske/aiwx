package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/domain"
	"github.com/rverhe/aiwx/internal/repository"
)

func rContext() context.Context {
	// Background context for async workflow advancement
	return context.Background()
}

// ─── Workflow Templates ──────────────────────────────────────────────────────

func (d *Deps) handleListWorkflowTemplates(w http.ResponseWriter, r *http.Request) {
	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}
	list, err := d.Repos.Workflows.List(r.Context(), scope)
	if err != nil {
		d.Logger.Error("list workflow templates failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list workflow templates")
		return
	}
	if list == nil {
		list = []domain.WorkflowTemplate{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": list, "has_more": false})
}

func (d *Deps) handleCreateWorkflowTemplate(w http.ResponseWriter, r *http.Request) {
	var input domain.WorkflowTemplateCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Field 'name' is required")
		return
	}
	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	// Enforce tenant boundary on client-supplied org.
	if input.OrganizationID != uuid.Nil && input.OrganizationID != orgID {
		scope, ok := d.requireScope(w, r)
		if !ok {
			return
		}
		if !scope.AllowsOrg(input.OrganizationID) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found")
			return
		}
	}
	if input.OrganizationID == uuid.Nil {
		input.OrganizationID = orgID
	}
	if input.Nodes == nil {
		input.Nodes = []domain.WorkflowNodeIn{}
	}
	if input.Links == nil {
		input.Links = []domain.WorkflowLinkIn{}
	}

	wf, err := d.Repos.Workflows.Create(r.Context(), input)
	if err != nil {
		d.Logger.Error("create workflow template failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create workflow template")
		return
	}
	writeJSON(w, http.StatusCreated, wf)
}

func (d *Deps) handleGetWorkflowTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid workflow template ID")
		return
	}
	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}
	wf, err := d.Repos.Workflows.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow template not found")
		return
	}
	if !scope.AllowsOrg(wf.OrganizationID) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow template not found")
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

func (d *Deps) handleUpdateWorkflowTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid workflow template ID")
		return
	}
	var input domain.WorkflowTemplateUpdate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	wf, err := d.Repos.Workflows.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow template not found")
		return
	}
	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}
	if !scope.AllowsOrg(wf.OrganizationID) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow template not found")
		return
	}

	updated, err := d.Repos.Workflows.Update(r.Context(), id, input)
	if err != nil {
		d.Logger.Error("update workflow template failed", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update workflow template")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (d *Deps) handleDeleteWorkflowTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid workflow template ID")
		return
	}
	wf, err := d.Repos.Workflows.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow template not found")
		return
	}
	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}
	if !scope.AllowsOrg(wf.OrganizationID) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow template not found")
		return
	}
	if err := d.Repos.Workflows.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete workflow template")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (d *Deps) handleLaunchWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid workflow template ID")
		return
	}
	wf, err := d.Repos.Workflows.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow template not found")
		return
	}
	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}
	if !scope.AllowsOrg(wf.OrganizationID) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow template not found")
		return
	}

	job, err := d.Repos.Workflows.Launch(r.Context(), id, wf.OrganizationID)
	if err != nil {
		d.Logger.Error("launch workflow failed", "error", err, "id", id)
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Advance via the event-driven workflow runner (async first wave).
	if d.WorkflowRunner != nil {
		go func(id uuid.UUID) {
			ctx := rContext()
			if err := d.WorkflowRunner.StartWorkflow(ctx, id); err != nil {
				d.Logger.Error("workflow start failed", "workflow_job_id", id, "error", err)
			}
		}(job.ID)
	} else {
		go d.runWorkflow(job.ID)
	}

	writeJSON(w, http.StatusCreated, job)
}

// ─── Workflow Jobs ───────────────────────────────────────────────────────────

func (d *Deps) handleListWorkflowJobs(w http.ResponseWriter, r *http.Request) {
	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}
	list, err := d.Repos.Workflows.ListJobs(r.Context(), scope)
	if err != nil {
		d.Logger.Error("list workflow jobs failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list workflow jobs")
		return
	}
	if list == nil {
		list = []domain.WorkflowJob{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": list, "has_more": false})
}

func (d *Deps) handleGetWorkflowJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid workflow job ID")
		return
	}
	job, err := d.Repos.Workflows.GetJobByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrWorkflowNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load workflow job")
		return
	}
	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}
	if !scope.AllowsOrg(job.OrganizationID) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workflow job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// runWorkflow launches ready nodes as jobs, then advances the DAG based on job outcomes.
// MVP: topological waves — nodes with no unsatisfied inbound links launch together.
func (d *Deps) runWorkflow(workflowJobID uuid.UUID) {
	ctx := rContext()
	_ = d.Repos.Workflows.UpdateJobStatus(ctx, workflowJobID, "running")

	for i := 0; i < 128; i++ {
		job, err := d.Repos.Workflows.GetJobByID(ctx, workflowJobID)
		if err != nil {
			d.Logger.Error("workflow run load failed", "error", err, "workflow_job_id", workflowJobID)
			return
		}

		// Always sync running nodes from their jobs before planning the next wave
		for _, n := range job.Nodes {
			if n.JobID == nil || (n.Status != "running" && n.Status != "pending") {
				continue
			}
			j, err := d.Repos.Jobs.GetByID(ctx, *n.JobID)
			if err != nil {
				continue
			}
			switch string(j.Status) {
			case "successful":
				_ = d.Repos.Workflows.UpdateJobNodeStatus(ctx, n.ID, "success", nil, "")
			case "failed", "error":
				_ = d.Repos.Workflows.UpdateJobNodeStatus(ctx, n.ID, "failed", nil, "")
			case "canceled":
				_ = d.Repos.Workflows.UpdateJobNodeStatus(ctx, n.ID, "skipped", nil, "canceled")
			}
		}
		// Reload after sync
		job, err = d.Repos.Workflows.GetJobByID(ctx, workflowJobID)
		if err != nil {
			return
		}

		// Build dependency map from links
		dependsOn := map[uuid.UUID][]domain.WorkflowJobNodeLink{}
		for _, l := range job.Links {
			dependsOn[l.TargetNodeID] = append(dependsOn[l.TargetNodeID], l)
		}
		statusOf := map[uuid.UUID]string{}
		for _, n := range job.Nodes {
			statusOf[n.ID] = n.Status
		}

		ready := []domain.WorkflowJobNode{}
		skipped := false
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
						allDone = allDone && true
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
				_ = d.Repos.Workflows.UpdateJobNodeStatus(ctx, n.ID, "skipped", nil, "")
				skipped = true
			} else if canRun {
				ready = append(ready, n)
			}
		}
		if skipped {
			continue
		}

		if len(ready) == 0 {
			// No ready nodes — check if complete
			job, _ = d.Repos.Workflows.GetJobByID(ctx, workflowJobID)
			if job == nil {
				return
			}
			allTerminal := true
			anyFailed := false
			for _, n := range job.Nodes {
				switch n.Status {
				case "success", "successful", "skipped":
				case "failed", "error":
					anyFailed = true
				default:
					allTerminal = false
				}
			}
			if allTerminal {
				if anyFailed {
					_ = d.Repos.Workflows.UpdateJobStatus(ctx, workflowJobID, "failed")
				} else {
					_ = d.Repos.Workflows.UpdateJobStatus(ctx, workflowJobID, "successful")
				}
				return
			}
			// Still waiting on running jobs — poll
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		// Launch each ready node as a job
		for _, n := range ready {
			if n.JobTemplateID == nil {
				_ = d.Repos.Workflows.UpdateJobNodeStatus(ctx, n.ID, "skipped", nil, "node has no job template")
				continue
			}
			tmplID := *n.JobTemplateID
			newJob, err := d.Repos.Jobs.CreateAndEnqueue(ctx, &tmplID, job.OrganizationID, domain.JobLaunch{}, d.Outbox)
			if err != nil {
				d.Logger.Error("workflow node job create failed", "error", err, "node", n.ID)
				_ = d.Repos.Workflows.UpdateJobNodeStatus(ctx, n.ID, "failed", nil, err.Error())
				continue
			}
			_ = d.Repos.Workflows.UpdateJobNodeStatus(ctx, n.ID, "running", &newJob.ID, "")
		}

		// Wait for launched jobs / in-flight nodes before next wave
		select {
		case <-ctx.Done():
			return
		case <-time.After(1500 * time.Millisecond):
		}
	}
	_ = d.Repos.Workflows.UpdateJobStatus(ctx, workflowJobID, "error")
}
