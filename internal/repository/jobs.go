package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rverhe/aiwx/internal/domain"
)

// OutboxInserter abstracts the outbox write for transactional job creation.
type OutboxInserter interface {
	Insert(ctx context.Context, tx pgx.Tx, eventType, aggregateType string, aggregateID uuid.UUID, payload map[string]any, idempotencyKey string) error
}

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
		Description:            input.Description,
		OrganizationID:         orgID,
		ProjectID:              input.ProjectID,
		InventoryID:            input.InventoryID,
		ExecutionEnvironmentID: input.ExecutionEnvironmentID,
		Playbook:               input.Playbook,
		CredentialIDs:          input.CredentialIDs,
		Limit:                  input.Limit,
		Verbosity:              input.Verbosity,
		Forks:                  input.Forks,
		JobTags:                input.JobTags,
		SkipTags:               input.SkipTags,
		ExtraVars:              input.ExtraVars,
		AskInventoryOnLaunch:   input.AskInventoryOnLaunch,
		AskLimitOnLaunch:       input.AskLimitOnLaunch,
		AskVerbosityOnLaunch:   input.AskVerbosityOnLaunch,
		AskVariablesOnLaunch:   input.AskVariablesOnLaunch,
		AskCredentialOnLaunch:  input.AskCredentialOnLaunch,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}
	if jt.ExtraVars == nil {
		jt.ExtraVars = map[string]any{}
	}
	if jt.Verbosity < 0 {
		jt.Verbosity = 0
	}
	if jt.Verbosity > 4 {
		jt.Verbosity = 4
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO job_templates (id, organization_id, name, description, project_id, inventory_id,
		                           execution_environment_id, playbook, limit_hosts, verbosity, forks,
		                           job_tags, skip_tags, extra_vars,
		                           ask_inventory_on_launch, ask_limit_on_launch, ask_verbosity_on_launch,
		                           ask_variables_on_launch, ask_credential_on_launch,
		                           created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`,
		jt.ID, jt.OrganizationID, jt.Name, jt.Description, jt.ProjectID, jt.InventoryID,
		jt.ExecutionEnvironmentID, jt.Playbook, jt.Limit, jt.Verbosity, jt.Forks,
		jt.JobTags, jt.SkipTags, jt.ExtraVars,
		jt.AskInventoryOnLaunch, jt.AskLimitOnLaunch, jt.AskVerbosityOnLaunch,
		jt.AskVariablesOnLaunch, jt.AskCredentialOnLaunch,
		jt.CreatedAt, jt.UpdatedAt,
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
	var extraVars map[string]any
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, description, project_id, inventory_id, execution_environment_id,
		        playbook, limit_hosts, verbosity, forks, job_tags, skip_tags, extra_vars,
		        ask_inventory_on_launch, ask_limit_on_launch, ask_verbosity_on_launch,
		        ask_variables_on_launch, ask_credential_on_launch,
		        created_at, updated_at
		 FROM job_templates WHERE id = $1`, id,
	).Scan(&jt.ID, &jt.OrganizationID, &jt.Name, &jt.Description, &jt.ProjectID, &jt.InventoryID,
		&jt.ExecutionEnvironmentID, &jt.Playbook, &jt.Limit, &jt.Verbosity, &jt.Forks,
		&jt.JobTags, &jt.SkipTags, &extraVars,
		&jt.AskInventoryOnLaunch, &jt.AskLimitOnLaunch, &jt.AskVerbosityOnLaunch,
		&jt.AskVariablesOnLaunch, &jt.AskCredentialOnLaunch,
		&jt.CreatedAt, &jt.UpdatedAt)
	if err != nil {
		return nil, err
	}
	jt.ExtraVars = extraVars

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

func (r *JobTemplateRepo) GetByIDScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) (*domain.JobTemplate, error) {
	if scope.IsPlatformAdmin {
		return r.GetByID(ctx, id)
	}
	jt := &domain.JobTemplate{}
	var extraVars map[string]any
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, description, project_id, inventory_id, execution_environment_id,
		        playbook, limit_hosts, verbosity, forks, job_tags, skip_tags, extra_vars,
		        ask_inventory_on_launch, ask_limit_on_launch, ask_verbosity_on_launch,
		        ask_variables_on_launch, ask_credential_on_launch,
		        created_at, updated_at
		 FROM job_templates WHERE id = $1 AND organization_id = ANY($2)`, id, scope.OrgIDs,
	).Scan(&jt.ID, &jt.OrganizationID, &jt.Name, &jt.Description, &jt.ProjectID, &jt.InventoryID,
		&jt.ExecutionEnvironmentID, &jt.Playbook, &jt.Limit, &jt.Verbosity, &jt.Forks,
		&jt.JobTags, &jt.SkipTags, &extraVars,
		&jt.AskInventoryOnLaunch, &jt.AskLimitOnLaunch, &jt.AskVerbosityOnLaunch,
		&jt.AskVariablesOnLaunch, &jt.AskCredentialOnLaunch,
		&jt.CreatedAt, &jt.UpdatedAt)
	if err != nil {
		return nil, err
	}
	jt.ExtraVars = extraVars

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
		`SELECT id, organization_id, name, description, project_id, inventory_id, execution_environment_id,
		        playbook, limit_hosts, verbosity, forks, job_tags, skip_tags, extra_vars,
		        ask_inventory_on_launch, ask_limit_on_launch, ask_verbosity_on_launch,
		        ask_variables_on_launch, ask_credential_on_launch,
		        created_at, updated_at
		 FROM job_templates WHERE organization_id = $1 ORDER BY name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []domain.JobTemplate
	for rows.Next() {
		var jt domain.JobTemplate
		var extraVars map[string]any
		if err := rows.Scan(&jt.ID, &jt.OrganizationID, &jt.Name, &jt.Description, &jt.ProjectID, &jt.InventoryID,
			&jt.ExecutionEnvironmentID, &jt.Playbook, &jt.Limit, &jt.Verbosity, &jt.Forks,
			&jt.JobTags, &jt.SkipTags, &extraVars,
			&jt.AskInventoryOnLaunch, &jt.AskLimitOnLaunch, &jt.AskVerbosityOnLaunch,
			&jt.AskVariablesOnLaunch, &jt.AskCredentialOnLaunch,
			&jt.CreatedAt, &jt.UpdatedAt); err != nil {
			return nil, err
		}
		jt.ExtraVars = extraVars
		templates = append(templates, jt)
	}
	return templates, nil
}

func (r *JobTemplateRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM job_templates WHERE id = $1`, id)
	return err
}

func (r *JobTemplateRepo) DeleteScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) error {
	if scope.IsPlatformAdmin {
		return r.Delete(ctx, id)
	}
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM job_templates WHERE id = $1 AND organization_id = ANY($2)`,
		id, scope.OrgIDs,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
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

	// Ensure extra_vars is never NULL (DB has NOT NULL constraint)
	extraVars := launch.ExtraVars
	if extraVars == nil {
		extraVars = map[string]any{}
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO jobs (id, organization_id, job_template_id, status, extra_vars, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		job.ID, job.OrganizationID, job.JobTemplateID, job.Status,
		extraVars, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job: %w", err)
	}

	return job, nil
}

// CreateAndEnqueue atomically creates a job, enqueues an outbox event,
// sets status to pending, and writes the first job_events row.
func (r *JobRepo) CreateAndEnqueue(ctx context.Context, jobTemplateID *uuid.UUID, orgID uuid.UUID, launch domain.JobLaunch, outbox OutboxInserter) (*domain.Job, error) {
	job := &domain.Job{
		ID:             uuid.New(),
		JobTemplateID:  jobTemplateID,
		OrganizationID: orgID,
		Status:         domain.JobStatusPending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Ensure extra_vars is never NULL (DB has NOT NULL constraint)
	extraVars := launch.ExtraVars
	if extraVars == nil {
		extraVars = map[string]any{}
	}

	// Insert job with pending status
	_, err = tx.Exec(ctx,
		`INSERT INTO jobs (id, organization_id, job_template_id, status, extra_vars, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		job.ID, job.OrganizationID, job.JobTemplateID, job.Status,
		extraVars, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job: %w", err)
	}

	// Insert outbox event in same tx
	if outbox != nil {
		if err := outbox.Insert(ctx, tx, "job.created", "job", job.ID,
			map[string]any{
				"job_id":          job.ID.String(),
				"job_template_id": jobTemplateID.String(),
				"organization_id": orgID.String(),
			},
			"job-launch-"+job.ID.String(),
		); err != nil {
			return nil, fmt.Errorf("insert outbox event: %w", err)
		}
	}

	// Insert first job event
	_, err = tx.Exec(ctx,
		`INSERT INTO job_events (job_id, event_type, event_data, created_at)
		 VALUES ($1, $2, $3, $4)`,
		job.ID, "job.created", map[string]any{"template_id": jobTemplateID.String()}, job.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
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

func (r *JobRepo) GetByIDScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) (*domain.Job, error) {
	if scope.IsPlatformAdmin {
		return r.GetByID(ctx, id)
	}
	job := &domain.Job{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, job_template_id, status, project_revision, artifact_ref,
		        kubernetes_job_name, exit_code, started_at, finished_at, created_at, updated_at
		 FROM jobs WHERE id = $1 AND organization_id = ANY($2)`, id, scope.OrgIDs,
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
	// Do not overwrite terminal states (e.g. canceled) with worker results.
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = $2, updated_at = $3,
		        started_at = CASE WHEN $2 = 'running' AND started_at IS NULL THEN $3 ELSE started_at END,
		        finished_at = CASE WHEN $2 IN ('successful', 'failed', 'error', 'canceled') THEN $3 ELSE finished_at END
		 WHERE id = $1
		   AND status NOT IN ('successful', 'failed', 'error', 'canceled')`,
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

// FailStaleRunning marks running jobs older than maxAge as failed.
func (r *JobRepo) FailStaleRunning(ctx context.Context, maxAge time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'failed', finished_at = now(), updated_at = now()
		 WHERE status = 'running' AND started_at < now() - $1::interval`,
		maxAge,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
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
