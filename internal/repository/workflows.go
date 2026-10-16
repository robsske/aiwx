package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rverhe/aiwx/internal/domain"
)

var ErrWorkflowNotFound = errors.New("workflow not found")

type WorkflowRepo struct {
	pool *Pool
}

func NewWorkflowRepo(pool *Pool) *WorkflowRepo {
	return &WorkflowRepo{pool: pool}
}

func (r *WorkflowRepo) Create(ctx context.Context, in domain.WorkflowTemplateCreate) (*domain.WorkflowTemplate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	extra, err := json.Marshal(in.ExtraVars)
	if err != nil {
		return nil, err
	}
	if in.ExtraVars == nil {
		extra = []byte("{}")
	}

	var wf domain.WorkflowTemplate
	err = tx.QueryRow(ctx, `
		INSERT INTO workflow_templates (organization_id, name, description, extra_vars)
		VALUES ($1, $2, $3, $4)
		RETURNING id, organization_id, name, description, extra_vars, created_at, updated_at
	`, in.OrganizationID, in.Name, in.Description, extra).
		Scan(&wf.ID, &wf.OrganizationID, &wf.Name, &wf.Description, &extra, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(extra, &wf.ExtraVars)

	if err := r.replaceGraph(ctx, tx, wf.ID, in.Nodes, in.Links); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, wf.ID)
}

func (r *WorkflowRepo) Update(ctx context.Context, id uuid.UUID, in domain.WorkflowTemplateUpdate) (*domain.WorkflowTemplate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if in.Name != nil {
		if _, err := tx.Exec(ctx, `UPDATE workflow_templates SET name=$2, updated_at=now() WHERE id=$1`, id, *in.Name); err != nil {
			return nil, err
		}
	}
	if in.Description != nil {
		if _, err := tx.Exec(ctx, `UPDATE workflow_templates SET description=$2, updated_at=now() WHERE id=$1`, id, *in.Description); err != nil {
			return nil, err
		}
	}
	if in.ExtraVars != nil {
		extra, err := json.Marshal(in.ExtraVars)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE workflow_templates SET extra_vars=$2, updated_at=now() WHERE id=$1`, id, extra); err != nil {
			return nil, err
		}
	}

	// Always replace graph when nodes/links provided (even empty = clear)
	if in.Nodes != nil || in.Links != nil {
		nodes := in.Nodes
		if nodes == nil {
			nodes = []domain.WorkflowNodeIn{}
		}
		links := in.Links
		if links == nil {
			links = []domain.WorkflowLinkIn{}
		}
		if err := r.replaceGraph(ctx, tx, id, nodes, links); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE workflow_templates SET updated_at=now() WHERE id=$1`, id); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *WorkflowRepo) replaceGraph(ctx context.Context, tx pgx.Tx, wfID uuid.UUID, nodes []domain.WorkflowNodeIn, links []domain.WorkflowLinkIn) error {
	if _, err := tx.Exec(ctx, `DELETE FROM workflow_node_links WHERE workflow_template_id=$1`, wfID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM workflow_nodes WHERE workflow_template_id=$1`, wfID); err != nil {
		return err
	}

	idMap := make(map[string]uuid.UUID, len(nodes)) // client id → db id
	for _, n := range nodes {
		nodeID := uuid.New()
		if n.ID != nil {
			nodeID = *n.ID
		}
		name := n.Name
		if name == "" {
			name = "Node"
		}
		var jobTmpl any
		if n.JobTemplateID != nil {
			jobTmpl = *n.JobTemplateID
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO workflow_nodes (id, workflow_template_id, job_template_id, name, position_x, position_y)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, nodeID, wfID, jobTmpl, name, n.PositionX, n.PositionY)
		if err != nil {
			return err
		}
		idMap[nodeID.String()] = nodeID
	}

	for _, l := range links {
		cond := l.Condition
		if cond == "" {
			cond = "success"
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO workflow_node_links (workflow_template_id, source_node_id, target_node_id, condition)
			VALUES ($1, $2, $3, $4)
		`, wfID, l.SourceNodeID, l.TargetNodeID, cond)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkflowRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.WorkflowTemplate, error) {
	var wf domain.WorkflowTemplate
	var extra []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, organization_id, name, description, extra_vars, created_at, updated_at
		FROM workflow_templates WHERE id=$1
	`, id).Scan(&wf.ID, &wf.OrganizationID, &wf.Name, &wf.Description, &extra, &wf.CreatedAt, &wf.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(extra, &wf.ExtraVars)

	nodes, err := r.listNodes(ctx, id)
	if err != nil {
		return nil, err
	}
	links, err := r.listLinks(ctx, id)
	if err != nil {
		return nil, err
	}
	wf.Nodes = nodes
	wf.Links = links
	return &wf, nil
}

func (r *WorkflowRepo) listNodes(ctx context.Context, wfID uuid.UUID) ([]domain.WorkflowNode, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT n.id, n.workflow_template_id, n.job_template_id, n.name, n.position_x, n.position_y,
		       COALESCE(jt.name, '')
		FROM workflow_nodes n
		LEFT JOIN job_templates jt ON jt.id = n.job_template_id
		WHERE n.workflow_template_id=$1
		ORDER BY n.position_x, n.position_y, n.name
	`, wfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WorkflowNode
	for rows.Next() {
		var n domain.WorkflowNode
		if err := rows.Scan(&n.ID, &n.WorkflowTemplateID, &n.JobTemplateID, &n.Name, &n.PositionX, &n.PositionY, &n.JobTemplateName); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) listLinks(ctx context.Context, wfID uuid.UUID) ([]domain.WorkflowLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workflow_template_id, source_node_id, target_node_id, condition
		FROM workflow_node_links WHERE workflow_template_id=$1
	`, wfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WorkflowLink
	for rows.Next() {
		var l domain.WorkflowLink
		if err := rows.Scan(&l.ID, &l.WorkflowTemplateID, &l.SourceNodeID, &l.TargetNodeID, &l.Condition); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) List(ctx context.Context, scope domain.Scope) ([]domain.WorkflowTemplate, error) {
	query := `
		SELECT wt.id, wt.organization_id, wt.name, wt.description, wt.extra_vars, wt.created_at, wt.updated_at,
		       COALESCE(nc.cnt, 0)
		FROM workflow_templates wt
		LEFT JOIN (
			SELECT workflow_template_id, COUNT(*) AS cnt FROM workflow_nodes GROUP BY workflow_template_id
		) nc ON nc.workflow_template_id = wt.id
	`
	args := []any{}
	if !scope.IsPlatformAdmin {
		query += ` WHERE wt.organization_id = ANY($1)`
		args = append(args, scope.OrgIDs)
	}
	query += ` ORDER BY wt.name`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	for rows.Next() {
		var wf domain.WorkflowTemplate
		var extra []byte
		var nodeCount int
		if err := rows.Scan(&wf.ID, &wf.OrganizationID, &wf.Name, &wf.Description, &extra, &wf.CreatedAt, &wf.UpdatedAt, &nodeCount); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(extra, &wf.ExtraVars)
		// light list: node count via placeholder nodes
		wf.Nodes = make([]domain.WorkflowNode, nodeCount)
		wf.Links = []domain.WorkflowLink{}
		out = append(out, wf)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM workflow_templates WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWorkflowNotFound
	}
	return nil
}

// ─── Workflow Jobs ───────────────────────────────────────────────────────────

func (r *WorkflowRepo) Launch(ctx context.Context, templateID uuid.UUID, orgID uuid.UUID) (*domain.WorkflowJob, error) {
	tmpl, err := r.GetByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if len(tmpl.Nodes) == 0 {
		return nil, fmt.Errorf("workflow has no nodes")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var job domain.WorkflowJob
	err = tx.QueryRow(ctx, `
		INSERT INTO workflow_jobs (workflow_template_id, organization_id, status)
		VALUES ($1, $2, 'pending')
		RETURNING id, workflow_template_id, organization_id, status, extra_vars, started_at, finished_at, created_at, updated_at
	`, templateID, orgID).Scan(
		&job.ID, &job.WorkflowTemplateID, &job.OrganizationID, &job.Status, &job.ExtraVars,
		&job.StartedAt, &job.FinishedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	job.WorkflowName = tmpl.Name

	nodeIDMap := make(map[uuid.UUID]uuid.UUID, len(tmpl.Nodes))
	for _, n := range tmpl.Nodes {
		var jnID uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO workflow_job_nodes (workflow_job_id, workflow_node_id, job_template_id, name, status)
			VALUES ($1, $2, $3, $4, 'pending')
			RETURNING id
		`, job.ID, n.ID, n.JobTemplateID, n.Name).Scan(&jnID)
		if err != nil {
			return nil, err
		}
		nodeIDMap[n.ID] = jnID
	}

	for _, l := range tmpl.Links {
		src, ok1 := nodeIDMap[l.SourceNodeID]
		dst, ok2 := nodeIDMap[l.TargetNodeID]
		if !ok1 || !ok2 {
			continue
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO workflow_job_node_links (workflow_job_id, source_node_id, target_node_id, condition)
			VALUES ($1, $2, $3, $4)
		`, job.ID, src, dst, l.Condition)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetJobByID(ctx, job.ID)
}

func (r *WorkflowRepo) GetJobByID(ctx context.Context, id uuid.UUID) (*domain.WorkflowJob, error) {
	var job domain.WorkflowJob
	var extra []byte
	err := r.pool.QueryRow(ctx, `
		SELECT wj.id, wj.workflow_template_id, wt.name, wj.organization_id, wj.status, wj.extra_vars,
		       wj.started_at, wj.finished_at, wj.created_at, wj.updated_at
		FROM workflow_jobs wj
		JOIN workflow_templates wt ON wt.id = wj.workflow_template_id
		WHERE wj.id=$1
	`, id).Scan(
		&job.ID, &job.WorkflowTemplateID, &job.WorkflowName, &job.OrganizationID, &job.Status,
		&extra, &job.StartedAt, &job.FinishedAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(extra, &job.ExtraVars)

	nodes, err := r.listJobNodes(ctx, id)
	if err != nil {
		return nil, err
	}
	links, err := r.listJobLinks(ctx, id)
	if err != nil {
		return nil, err
	}
	job.Nodes = nodes
	job.Links = links
	return &job, nil
}

func (r *WorkflowRepo) listJobNodes(ctx context.Context, jobID uuid.UUID) ([]domain.WorkflowJobNode, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workflow_job_id, workflow_node_id, job_template_id, job_id, name, status,
		       error_message, started_at, finished_at
		FROM workflow_job_nodes WHERE workflow_job_id=$1
		ORDER BY created_at
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WorkflowJobNode
	for rows.Next() {
		var n domain.WorkflowJobNode
		if err := rows.Scan(
			&n.ID, &n.WorkflowJobID, &n.WorkflowNodeID, &n.JobTemplateID, &n.JobID, &n.Name, &n.Status,
			&n.ErrorMessage, &n.StartedAt, &n.FinishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) listJobLinks(ctx context.Context, jobID uuid.UUID) ([]domain.WorkflowJobNodeLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, workflow_job_id, source_node_id, target_node_id, condition
		FROM workflow_job_node_links WHERE workflow_job_id=$1
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WorkflowJobNodeLink
	for rows.Next() {
		var l domain.WorkflowJobNodeLink
		if err := rows.Scan(&l.ID, &l.WorkflowJobID, &l.SourceNodeID, &l.TargetNodeID, &l.Condition); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) ListJobs(ctx context.Context, scope domain.Scope) ([]domain.WorkflowJob, error) {
	query := `
		SELECT wj.id, wj.workflow_template_id, wt.name, wj.organization_id, wj.status, wj.extra_vars,
		       wj.started_at, wj.finished_at, wj.created_at, wj.updated_at
		FROM workflow_jobs wj
		JOIN workflow_templates wt ON wt.id = wj.workflow_template_id
	`
	args := []any{}
	if !scope.IsPlatformAdmin {
		query += ` WHERE wj.organization_id = ANY($1)`
		args = append(args, scope.OrgIDs)
	}
	query += ` ORDER BY wj.created_at DESC LIMIT 200`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WorkflowJob
	for rows.Next() {
		var job domain.WorkflowJob
		var extra []byte
		if err := rows.Scan(
			&job.ID, &job.WorkflowTemplateID, &job.WorkflowName, &job.OrganizationID, &job.Status,
			&extra, &job.StartedAt, &job.FinishedAt, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(extra, &job.ExtraVars)
		job.Nodes = []domain.WorkflowJobNode{}
		job.Links = []domain.WorkflowJobNodeLink{}
		out = append(out, job)
	}
	return out, rows.Err()
}

func (r *WorkflowRepo) UpdateJobNodeStatus(ctx context.Context, nodeID uuid.UUID, status string, jobID *uuid.UUID, errMsg string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE workflow_job_nodes
		SET status=$2, job_id=COALESCE($3, job_id), error_message=$4,
		    started_at = CASE WHEN $2='running' THEN now() ELSE started_at END,
		    finished_at = CASE WHEN $2 IN ('success','failed','skipped','successful','error','canceled') THEN now() ELSE finished_at END,
		    updated_at=now()
		WHERE id=$1
	`, nodeID, status, jobID, errMsg)
	return err
}

func (r *WorkflowRepo) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE workflow_jobs
		SET status=$2,
		    started_at = CASE WHEN $2='running' AND started_at IS NULL THEN now() ELSE started_at END,
		    finished_at = CASE WHEN $2 IN ('successful','failed','error','canceled') THEN now() ELSE finished_at END,
		    updated_at=now()
		WHERE id=$1
	`, jobID, status)
	return err
}
