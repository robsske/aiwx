package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rverhe/aiwx/internal/domain"
)

// ─── Job Template Handlers ────────────────────────────────────────────────────

func (d *Deps) handleListJobTemplates(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	templates, err := d.Repos.JobTemplates.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list job templates")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  templates,
		"has_more": false,
	})
}

func (d *Deps) handleCreateJobTemplate(w http.ResponseWriter, r *http.Request) {
	var input domain.JobTemplateCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" || input.ProjectID == uuid.Nil || input.Playbook == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Fields 'name', 'project_id', and 'playbook' are required")
		return
	}

	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	jt, err := d.Repos.JobTemplates.Create(r.Context(), input, orgID)
	if err != nil {
		d.Logger.Error("create job template failed", "error", err, "org_id", orgID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create job template")
		return
	}

	writeJSON(w, http.StatusCreated, jt)
}

func (d *Deps) handleGetJobTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job template ID")
		return
	}

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	jt, err := d.Repos.JobTemplates.GetByIDScoped(r.Context(), id, scope)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Job template not found")
		return
	}

	writeJSON(w, http.StatusOK, jt)
}

func (d *Deps) handleDeleteJobTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job template ID")
		return
	}

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	if err := d.Repos.JobTemplates.DeleteScoped(r.Context(), id, scope); err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Job template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete job template")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Job Launch ───────────────────────────────────────────────────────────────

func (d *Deps) handleLaunchJob(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job template ID")
		return
	}

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	// Validate template exists and is in scope
	template, err := d.Repos.JobTemplates.GetByIDScoped(r.Context(), templateID, scope)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Job template not found")
		return
	}

	// Parse launch parameters
	var launch domain.JobLaunch
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&launch); err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
			return
		}
	}

	// Atomically create job + outbox event + first job event
	job, err := d.Repos.Jobs.CreateAndEnqueue(r.Context(), &templateID, template.OrganizationID, launch, d.Outbox)
	if err != nil {
		d.Logger.Error("create and enqueue job failed", "error", err, "template_id", templateID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create job")
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

// ─── Job Handlers ─────────────────────────────────────────────────────────────

func (d *Deps) handleListJobs(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	jobs, err := d.Repos.Jobs.ListByOrg(r.Context(), orgID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list jobs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  jobs,
		"has_more": false,
	})
}

func (d *Deps) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job ID")
		return
	}

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	job, err := d.Repos.Jobs.GetByIDScoped(r.Context(), id, scope)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (d *Deps) handleGetJobEvents(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job ID")
		return
	}

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	// Verify job is in scope before listing events
	if _, err := d.Repos.Jobs.GetByIDScoped(r.Context(), jobID, scope); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
		return
	}

	events, err := d.Repos.JobEvents.ListByJob(r.Context(), jobID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list job events")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  events,
		"has_more": false,
	})
}

func (d *Deps) handleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job ID")
		return
	}

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	// Verify job is in scope
	if _, err := d.Repos.Jobs.GetByIDScoped(r.Context(), jobID, scope); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
		return
	}

	logs, err := d.Repos.JobLogs.ListByJob(r.Context(), jobID, 500)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list job logs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  logs,
		"has_more": false,
	})
}

func (d *Deps) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid job ID")
		return
	}

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	job, err := d.Repos.Jobs.GetByIDScoped(r.Context(), id, scope)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
		return
	}

	if job.Status.Terminal() {
		writeError(w, http.StatusConflict, "CONFLICT", "Job is already in a terminal state")
		return
	}

	// Validate transition
	if !isValidTransition(job.Status, domain.JobStatusCanceled) {
		writeError(w, http.StatusConflict, "CONFLICT", "Cannot cancel job in current state")
		return
	}

	if err := d.Repos.Jobs.UpdateStatus(r.Context(), id, domain.JobStatusCanceled, "canceled by user"); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to cancel job")
		return
	}

	// Create job event
	d.Repos.JobEvents.Create(r.Context(), id, "job.canceled", map[string]any{
		"reason": "user request",
	})

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "canceled"})
}

// isValidTransition checks job state machine transitions.
func isValidTransition(from, to domain.JobStatus) bool {
	transitions := map[domain.JobStatus][]domain.JobStatus{
		domain.JobStatusNew:     {domain.JobStatusPending, domain.JobStatusCanceled},
		domain.JobStatusPending: {domain.JobStatusWaiting, domain.JobStatusCanceled},
		domain.JobStatusWaiting: {domain.JobStatusRunning, domain.JobStatusCanceled, domain.JobStatusError},
		domain.JobStatusRunning: {domain.JobStatusSuccessful, domain.JobStatusFailed, domain.JobStatusError, domain.JobStatusCanceled},
	}
	allowed, ok := transitions[from]
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
