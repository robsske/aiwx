-- AIWX initial schema
-- Migration: 0001_init.up.sql

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";

-- ─── Organizations (tenant root) ─────────────────────────────────────────────

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    display_name TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_organizations_name ON organizations (name);

-- ─── Users ───────────────────────────────────────────────────────────────────

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    password_hash TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── Teams ───────────────────────────────────────────────────────────────────

CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

-- ─── Team Members ────────────────────────────────────────────────────────────

CREATE TABLE team_members (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX idx_team_members_user ON team_members (user_id);

-- ─── Roles ───────────────────────────────────────────────────────────────────

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, organization_id)
);

-- Seed platform roles (organization_id = NULL)
INSERT INTO roles (name, organization_id) VALUES
    ('platform-admin', NULL);

-- ─── Role Assignments ────────────────────────────────────────────────────────

CREATE TABLE role_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((user_id IS NOT NULL) != (team_id IS NOT NULL))
);

CREATE INDEX idx_role_assignments_user ON role_assignments (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_role_assignments_team ON role_assignments (team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_role_assignments_org ON role_assignments (organization_id);

-- ─── Credential Types ────────────────────────────────────────────────────────

CREATE TABLE credential_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT 'aiwx',
    inputs_schema JSONB NOT NULL DEFAULT '{}',
    injectors JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (namespace, name)
);

-- Seed default credential types
INSERT INTO credential_types (name, namespace, inputs_schema) VALUES
    ('Git SSH Key', 'aiwx', '{"ssh_key": {"type": "string", "secret": true}, "ssh_public_key": {"type": "string"}}'),
    ('Username/Password', 'aiwx', '{"username": {"type": "string"}, "password": {"type": "string", "secret": true}}'),
    ('Vault Token', 'aiwx', '{"vault_url": {"type": "string"}, "vault_token": {"type": "string", "secret": true}}'),
    ('Kubernetes Secret Reference', 'aiwx', '{"secret_name": {"type": "string"}, "namespace": {"type": "string"}}'),
    ('Cloud Auth', 'aiwx', '{"access_key": {"type": "string"}, "secret_key": {"type": "string", "secret": true}, "region": {"type": "string"}}');

-- ─── Credentials ─────────────────────────────────────────────────────────────

CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    credential_type_id UUID NOT NULL REFERENCES credential_types(id),
    inputs JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

-- ─── Projects ────────────────────────────────────────────────────────────────

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    scm_url TEXT NOT NULL,
    scm_branch TEXT NOT NULL DEFAULT 'main',
    scm_credential_id UUID REFERENCES credentials(id) ON DELETE SET NULL,
    latest_revision TEXT,
    latest_artifact_ref TEXT,
    sync_status TEXT NOT NULL DEFAULT 'pending' CHECK (sync_status IN ('pending', 'running', 'success', 'failed', 'canceled')),
    last_sync_at TIMESTAMPTZ,
    last_sync_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

-- ─── Inventories ─────────────────────────────────────────────────────────────

CREATE TABLE inventories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    has_active_inventory_sources BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

CREATE TABLE inventory_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id UUID NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    variables JSONB NOT NULL DEFAULT '{}',
    parent_group_id UUID REFERENCES inventory_groups(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (inventory_id, name)
);

CREATE TABLE inventory_hosts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id UUID NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    variables JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (inventory_id, name)
);

CREATE TABLE inventory_group_hosts (
    group_id UUID NOT NULL REFERENCES inventory_groups(id) ON DELETE CASCADE,
    host_id UUID NOT NULL REFERENCES inventory_hosts(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, host_id)
);

CREATE TABLE inventory_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id UUID NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('static', 'ec2')),
    credential_id UUID REFERENCES credentials(id) ON DELETE SET NULL,
    source_vars JSONB NOT NULL DEFAULT '{}',
    schedule_interval_seconds INTEGER,
    last_sync_status TEXT NOT NULL DEFAULT 'pending',
    last_sync_at TIMESTAMPTZ,
    last_sync_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (inventory_id, name)
);

-- ─── Execution Environments ──────────────────────────────────────────────────

CREATE TABLE execution_environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    image TEXT NOT NULL,
    digest TEXT,
    pull_policy TEXT NOT NULL DEFAULT 'IfNotPresent' CHECK (pull_policy IN ('Always', 'IfNotPresent', 'Never')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_ee_org_name ON execution_environments (organization_id, name) WHERE organization_id IS NOT NULL;
CREATE UNIQUE INDEX idx_ee_global_name ON execution_environments (name) WHERE organization_id IS NULL;

-- ─── Job Templates ───────────────────────────────────────────────────────────

CREATE TABLE job_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    inventory_id UUID REFERENCES inventories(id) ON DELETE SET NULL,
    execution_environment_id UUID REFERENCES execution_environments(id) ON DELETE SET NULL,
    playbook TEXT NOT NULL,
    survey_enabled BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

CREATE TABLE job_template_credentials (
    job_template_id UUID NOT NULL REFERENCES job_templates(id) ON DELETE CASCADE,
    credential_id UUID NOT NULL REFERENCES credentials(id) ON DELETE CASCADE,
    PRIMARY KEY (job_template_id, credential_id)
);

-- ─── Jobs ────────────────────────────────────────────────────────────────────

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    job_template_id UUID REFERENCES job_templates(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'pending', 'waiting', 'running', 'successful', 'failed', 'error', 'canceled')),
    project_revision TEXT,
    artifact_ref TEXT,
    kubernetes_job_name TEXT,
    extra_vars JSONB NOT NULL DEFAULT '{}',
    exit_code INTEGER,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_jobs_org ON jobs (organization_id);
CREATE INDEX idx_jobs_status ON jobs (status) WHERE status NOT IN ('successful', 'failed', 'error', 'canceled');
CREATE INDEX idx_jobs_template ON jobs (job_template_id);
CREATE INDEX idx_jobs_created ON jobs (created_at DESC);

-- ─── Job Events ──────────────────────────────────────────────────────────────

CREATE TABLE job_events (
    id BIGSERIAL,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    event_data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create initial partitions (current month + next 2 months)
CREATE TABLE job_events_2026_10 PARTITION OF job_events
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE job_events_2026_11 PARTITION OF job_events
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE job_events_2026_12 PARTITION OF job_events
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

CREATE INDEX idx_job_events_job ON job_events (job_id, id);

-- ─── Outbox (transactional outbox) ───────────────────────────────────────────

CREATE TABLE outbox (
    id BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    payload JSONB NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    retry_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT
);

CREATE INDEX idx_outbox_unpublished ON outbox (id) WHERE published_at IS NULL;

-- ─── Audit Log ───────────────────────────────────────────────────────────────

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id UUID,
    details JSONB NOT NULL DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_org ON audit_logs (organization_id, created_at DESC);
CREATE INDEX idx_audit_logs_user ON audit_logs (user_id, created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs (resource_type, resource_id);

-- ─── Service Accounts ────────────────────────────────────────────────────────

CREATE TABLE service_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

-- ─── RLS Policies ────────────────────────────────────────────────────────────

-- Enable RLS on tenant-owned tables
ALTER TABLE teams ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventories ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_hosts ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_group_hosts ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE job_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE job_template_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE job_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_accounts ENABLE ROW LEVEL SECURITY;

-- Create application role
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'aiwx_app') THEN
        CREATE ROLE aiwx_app NOLOGIN;
    END IF;
END
$$;

-- Tenant isolation policies
CREATE POLICY tenant_isolation_teams ON teams
    FOR ALL TO aiwx_app
    USING (organization_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation_team_members ON team_members
    FOR ALL TO aiwx_app
    USING (team_id IN (SELECT id FROM teams WHERE organization_id = current_setting('app.current_tenant', true)::uuid));

CREATE POLICY tenant_isolation_credentials ON credentials
    FOR ALL TO aiwx_app
    USING (organization_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation_projects ON projects
    FOR ALL TO aiwx_app
    USING (organization_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation_inventories ON inventories
    FOR ALL TO aiwx_app
    USING (organization_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation_inventory_groups ON inventory_groups
    FOR ALL TO aiwx_app
    USING (inventory_id IN (SELECT id FROM inventories WHERE organization_id = current_setting('app.current_tenant', true)::uuid));

CREATE POLICY tenant_isolation_inventory_hosts ON inventory_hosts
    FOR ALL TO aiwx_app
    USING (inventory_id IN (SELECT id FROM inventories WHERE organization_id = current_setting('app.current_tenant', true)::uuid));

CREATE POLICY tenant_isolation_inventory_group_hosts ON inventory_group_hosts
    FOR ALL TO aiwx_app
    USING (host_id IN (SELECT id FROM inventory_hosts WHERE inventory_id IN (SELECT id FROM inventories WHERE organization_id = current_setting('app.current_tenant', true)::uuid)));

CREATE POLICY tenant_isolation_inventory_sources ON inventory_sources
    FOR ALL TO aiwx_app
    USING (inventory_id IN (SELECT id FROM inventories WHERE organization_id = current_setting('app.current_tenant', true)::uuid));

CREATE POLICY tenant_isolation_job_templates ON job_templates
    FOR ALL TO aiwx_app
    USING (organization_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation_jobs ON jobs
    FOR ALL TO aiwx_app
    USING (organization_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation_job_events ON job_events
    FOR ALL TO aiwx_app
    USING (job_id IN (SELECT id FROM jobs WHERE organization_id = current_setting('app.current_tenant', true)::uuid));

CREATE POLICY tenant_isolation_audit_logs ON audit_logs
    FOR ALL TO aiwx_app
    USING (organization_id = current_setting('app.current_tenant', true)::uuid OR organization_id IS NULL);

CREATE POLICY tenant_isolation_service_accounts ON service_accounts
    FOR ALL TO aiwx_app
    USING (organization_id = current_setting('app.current_tenant', true)::uuid);

-- Grant permissions to application role
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO aiwx_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO aiwx_app;
