-- AWX-style workflow templates, nodes, links, and runs

CREATE TABLE workflow_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    extra_vars JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

CREATE INDEX idx_workflow_templates_org ON workflow_templates (organization_id);

CREATE TABLE workflow_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_template_id UUID NOT NULL REFERENCES workflow_templates(id) ON DELETE CASCADE,
    job_template_id UUID REFERENCES job_templates(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    position_x INTEGER NOT NULL DEFAULT 0,
    position_y INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_nodes_template ON workflow_nodes (workflow_template_id);

-- condition: success | failure | always (AWX success_nodes / failure_nodes / always_nodes)
CREATE TABLE workflow_node_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_template_id UUID NOT NULL REFERENCES workflow_templates(id) ON DELETE CASCADE,
    source_node_id UUID NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    target_node_id UUID NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    condition TEXT NOT NULL CHECK (condition IN ('success', 'failure', 'always')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_node_id, target_node_id, condition)
);

CREATE INDEX idx_workflow_links_template ON workflow_node_links (workflow_template_id);

CREATE TABLE workflow_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_template_id UUID NOT NULL REFERENCES workflow_templates(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'pending', 'waiting', 'running', 'successful', 'failed', 'error', 'canceled')),
    extra_vars JSONB NOT NULL DEFAULT '{}',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_jobs_template ON workflow_jobs (workflow_template_id);
CREATE INDEX idx_workflow_jobs_created ON workflow_jobs (created_at DESC);

CREATE TABLE workflow_job_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_job_id UUID NOT NULL REFERENCES workflow_jobs(id) ON DELETE CASCADE,
    workflow_node_id UUID NOT NULL REFERENCES workflow_nodes(id) ON DELETE CASCADE,
    job_template_id UUID REFERENCES job_templates(id) ON DELETE SET NULL,
    job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
    name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'success', 'failed', 'skipped', 'waiting', 'successful', 'error', 'canceled')),
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_job_nodes_job ON workflow_job_nodes (workflow_job_id);

CREATE TABLE workflow_job_node_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_job_id UUID NOT NULL REFERENCES workflow_jobs(id) ON DELETE CASCADE,
    source_node_id UUID NOT NULL REFERENCES workflow_job_nodes(id) ON DELETE CASCADE,
    target_node_id UUID NOT NULL REFERENCES workflow_job_nodes(id) ON DELETE CASCADE,
    condition TEXT NOT NULL CHECK (condition IN ('success', 'failure', 'always'))
);

CREATE INDEX idx_workflow_job_links_job ON workflow_job_node_links (workflow_job_id);
