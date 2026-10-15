package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/domain"
)

// ─── Job Templates ────────────────────────────────────────────────────────────

type JobTemplateRepo struct {
	pool *Pool
}

func NewJobTemplateRepo(pool *Pool) *JobTemplateRepo {
	return &JobTemplateRepo{pool: pool}
}

func (r *JobTemplateRepo) Create(ctx context.Context, input domain.JobTemplateCreate, orgID uuid.UUID) (*domain.JobTemplate, error) {
	jt := &domain.JobTemplate{
		ID:                     uuid.New(),
		Name:                   input.Name,
		OrganizationID:         orgID,
		ProjectID:              input.ProjectID,
		InventoryID:            input.InventoryID,
		ExecutionEnvironmentID: input.ExecutionEnvironmentID,
		Playbook:               input.Playbook,
		CredentialIDs:          input.CredentialIDs,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO job_templates (id, organization_id, name, project_id, inventory_id,
		                           execution_environment_id, playbook, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		jt.ID, jt.OrganizationID, jt.Name, jt.ProjectID, jt.InventoryID,
		jt.ExecutionEnvironmentID, jt.Playbook, jt.CreatedAt, jt.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job template: %w", err)
	}

	// Insert credentials
	for _, credID := range jt.CredentialIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO job_template_credentials (job_template_id, credential_id) VALUES ($1, $2)`,
			jt.ID, credID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert job template credential: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return jt, nil
}

func (r *JobTemplateRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.JobTemplate, error) {
	jt := &domain.JobTemplate{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, project_id, inventory_id, execution_environment_id,
		        playbook, created_at, updated_at
		 FROM job_templates WHERE id = $1`, id,
	).Scan(&jt.ID, &jt.OrganizationID, &jt.Name, &jt.ProjectID, &jt.InventoryID,
		&jt.ExecutionEnvironmentID, &jt.Playbook, &jt.CreatedAt, &jt.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Load credentials
	rows, err := r.pool.Query(ctx,
		`SELECT credential_id FROM job_template_credentials WHERE job_template_id = $1`, id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var credID uuid.UUID
		if err := rows.Scan(&credID); err != nil {
			return nil, err
		}
		jt.CredentialIDs = append(jt.CredentialIDs, credID)
	}

	return jt, nil
}

func (r *JobTemplateRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.JobTemplate, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, name, project_id, inventory_id, execution_environment_id,
		        playbook, created_at, updated_at
		 FROM job_templates WHERE organization_id = $1 ORDER BY name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []domain.JobTemplate
	for rows.Next() {
		var jt domain.JobTemplate
		if err := rows.Scan(&jt.ID, &jt.OrganizationID, &jt.Name, &jt.ProjectID, &jt.InventoryID,
			&jt.ExecutionEnvironmentID, &jt.Playbook, &jt.CreatedAt, &jt.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, jt)
	}
	return templates, nil
}

func (r *JobTemplateRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM job_templates WHERE id = $1`, id)
	return err
}

// ─── Jobs ────────────────────────────────────────────────────────────────────

type JobRepo struct {
	pool *Pool
}

func NewJobRepo(pool *Pool) *JobRepo {
	return &JobRepo{pool: pool}
}

func (r *JobRepo) Create(ctx context.Context, jobTemplateID *uuid.UUID, orgID uuid.UUID, launch domain.JobLaunch) (*domain.Job, error) {
	job := &domain.Job{
		ID:             uuid.New(),
		JobTemplateID:  jobTemplateID,
		OrganizationID: orgID,
		Status:         domain.JobStatusNew,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO jobs (id, organization_id, job_template_id, status, extra_vars, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		job.ID, job.OrganizationID, job.JobTemplateID, job.Status,
		launch.ExtraVars, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job: %w", err)
	}

	return job, nil
}

func (r *JobRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	job := &domain.Job{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, job_template_id, status, project_revision, artifact_ref,
		        kubernetes_job_name, exit_code, started_at, finished_at, created_at, updated_at
		 FROM jobs WHERE id = $1`, id,
	).Scan(&job.ID, &job.OrganizationID, &job.JobTemplateID, &job.Status,
		&job.ProjectRevision, &job.ArtifactRef, &job.KubernetesJobName,
		&job.ExitCode, &job.StartedAt, &job.FinishedAt,
		&job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (r *JobRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.JobStatus, reason string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = $2, updated_at = $3,
		        started_at = CASE WHEN $2 = 'running' AND started_at IS NULL THEN $3 ELSE started_at END,
		        finished_at = CASE WHEN $2 IN ('successful', 'failed', 'error', 'canceled') THEN $3 ELSE finished_at END
		 WHERE id = $1`,
		id, status, now,
	)
	return err
}

func (r *JobRepo) UpdateK8sJobName(ctx context.Context, id uuid.UUID, k8sJobName string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET kubernetes_job_name = $2, updated_at = now() WHERE id = $1`,
		id, k8sJobName,
	)
	return err
}

func (r *JobRepo) ListByOrg(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.Job, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, job_template_id, status, project_revision, artifact_ref,
		        kubernetes_job_name, exit_code, started_at, finished_at, created_at, updated_at
		 FROM jobs WHERE organization_id = $1
		 ORDER BY created_at DESC LIMIT $2`, orgID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		var job domain.Job
		if err := rows.Scan(&job.ID, &job.OrganizationID, &job.JobTemplateID, &job.Status,
			&job.ProjectRevision, &job.ArtifactRef, &job.KubernetesJobName,
			&job.ExitCode, &job.StartedAt, &job.FinishedAt,
			&job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *JobRepo) ListNonTerminal(ctx context.Context) ([]domain.Job, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, job_template_id, status, project_revision, artifact_ref,
		        kubernetes_job_name, exit_code, started_at, finished_at, created_at, updated_at
		 FROM jobs
		 WHERE status NOT IN ('successful', 'failed', 'error', 'canceled')
		 ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		var job domain.Job
		if err := rows.Scan(&job.ID, &job.OrganizationID, &job.JobTemplateID, &job.Status,
			&job.ProjectRevision, &job.ArtifactRef, &job.KubernetesJobName,
			&job.ExitCode, &job.StartedAt, &job.FinishedAt,
			&job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// ─── Job Events ───────────────────────────────────────────────────────────────

type JobEventRepo struct {
	pool *Pool
}

func NewJobEventRepo(pool *Pool) *JobEventRepo {
	return &JobEventRepo{pool: pool}
}

func (r *JobEventRepo) Create(ctx context.Context, jobID uuid.UUID, eventType string, eventData map[string]any) (*domain.JobEvent, error) {
	event := &domain.JobEvent{
		JobID:     jobID,
		EventType: eventType,
		EventData: eventData,
		CreatedAt: time.Now(),
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO job_events (job_id, event_type, event_data, created_at)
		 VALUES ($1, $2, $3, $4)`,
		event.JobID, event.EventType, event.EventData, event.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job event: %w", err)
	}

	return event, nil
}

func (r *JobEventRepo) ListByJob(ctx context.Context, jobID uuid.UUID, limit int) ([]domain.JobEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, job_id, event_type, event_data, created_at
		 FROM job_events WHERE job_id = $1
		 ORDER BY id LIMIT $2`, jobID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.JobEvent
	for rows.Next() {
		var event domain.JobEvent
		if err := rows.Scan(&event.ID, &event.JobID, &event.EventType,
			&event.EventData, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
