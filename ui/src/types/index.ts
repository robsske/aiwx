// ─── Organization ─────────────────────────────────────────────────────────────

export interface Organization {
  id: string
  name: string
  display_name: string
  status: 'active' | 'inactive'
  created_at: string
  updated_at: string
}

// ─── Team ─────────────────────────────────────────────────────────────────────

export interface Team {
  id: string
  name: string
  display_name: string
  organization_id: string
  member_count: number
  created_at: string
  updated_at: string
}

// ─── Project ──────────────────────────────────────────────────────────────────

export interface Project {
  id: string
  name: string
  organization_id: string
  scm_url: string
  scm_branch: string
  scm_credential_id: string | null
  latest_revision: string | null
  sync_status: 'pending' | 'running' | 'success' | 'failed' | 'canceled'
  last_sync_at: string | null
  created_at: string
  updated_at: string
}

// ─── Job ──────────────────────────────────────────────────────────────────────

export type JobStatus = 'new' | 'pending' | 'waiting' | 'running' | 'successful' | 'failed' | 'error' | 'canceled'

export interface Job {
  id: string
  job_template_id: string | null
  organization_id: string
  status: JobStatus
  project_revision: string | null
  artifact_ref: string | null
  kubernetes_job_name: string | null
  exit_code: number | null
  started_at: string | null
  finished_at: string | null
  created_at: string
  updated_at: string
}

/** Human-readable label for a job: template name, or Project Sync when unlinked. */
export function jobDisplayName(
  job: Pick<Job, 'job_template_id'>,
  templates?: Array<{ id: string; name: string }> | null,
): string {
  if (!job.job_template_id) return 'Project Sync'
  const template = templates?.find((t) => t.id === job.job_template_id)
  return template?.name ?? `Template ${job.job_template_id.slice(0, 8)}`
}

export interface JobEvent {
  id: number
  job_id: string
  event_type: string
  event_data: Record<string, any>
  created_at: string
}

// ─── Credential ───────────────────────────────────────────────────────────────

export interface CredentialType {
  id: string
  name: string
  namespace: string
  inputs_schema: Record<string, any>
}

export interface Credential {
  id: string
  name: string
  organization_id: string
  credential_type_id: string
  inputs: Record<string, any>
  created_at: string
  updated_at: string
}

// ─── Inventory ────────────────────────────────────────────────────────────────

export interface Inventory {
  id: string
  name: string
  description: string
  organization_id: string
  host_count: number
  group_count: number
  has_active_inventory_sources: boolean
  created_at: string
  updated_at: string
}

export interface InventoryHost {
  id: string
  name: string
  description: string
  inventory_id: string
  variables: Record<string, any>
  enabled: boolean
  groups: string[]
  created_at: string
  updated_at: string
}

export interface InventoryGroup {
  id: string
  name: string
  description: string
  inventory_id: string
  variables: Record<string, any>
  parent_group_id: string | null
  host_count: number
  child_count: number
  created_at: string
  updated_at: string
}

// ─── Job Template ─────────────────────────────────────────────────────────────

export interface JobTemplate {
  id: string
  name: string
  description: string
  organization_id: string
  project_id: string
  inventory_id: string | null
  execution_environment_id: string | null
  playbook: string
  credential_ids: string[]
  limit: string
  verbosity: number
  forks: number
  job_tags: string
  skip_tags: string
  extra_vars: Record<string, any>
  ask_inventory_on_launch: boolean
  ask_limit_on_launch: boolean
  ask_verbosity_on_launch: boolean
  ask_variables_on_launch: boolean
  ask_credential_on_launch: boolean
  created_at: string
  updated_at: string
}

// ─── Pagination ───────────────────────────────────────────────────────────────

export interface PaginatedResponse<T> {
  results: T[]
  next_cursor: string | null
  has_more: boolean
}

// ─── User ─────────────────────────────────────────────────────────────────────

export interface User {
  id: string
  email: string
  display_name: string
  is_active: boolean
  created_at: string
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

export interface UserInfo {
  id: string
  email: string
  display_name: string
  auth_type: string
  organization: string
  roles: string[]
}

// ─── API Error ────────────────────────────────────────────────────────────────

export interface APIError {
  error_code: string
  message: string
  details?: Record<string, any>
}

// ─── Workflows (AWX Workflow Job Templates) ──────────────────────────────────

export interface WorkflowNode {
  id: string
  workflow_template_id: string
  job_template_id: string | null
  name: string
  position_x: number
  position_y: number
  job_template_name?: string
}

export interface WorkflowLink {
  id: string
  workflow_template_id: string
  source_node_id: string
  target_node_id: string
  condition: 'success' | 'failure' | 'always'
}

export interface WorkflowTemplate {
  id: string
  organization_id: string
  name: string
  description: string
  extra_vars: Record<string, any>
  nodes: WorkflowNode[]
  links: WorkflowLink[]
  created_at: string
  updated_at: string
}

export interface WorkflowNodeIn {
  id?: string
  job_template_id: string | null
  name?: string
  position_x: number
  position_y: number
}

export interface WorkflowLinkIn {
  source_node_id: string
  target_node_id: string
  condition: 'success' | 'failure' | 'always'
}

export interface WorkflowTemplateCreate {
  organization_id?: string
  name: string
  description?: string
  nodes: WorkflowNodeIn[]
  links: WorkflowLinkIn[]
}

export interface WorkflowTemplateUpdate {
  name?: string
  description?: string
  nodes?: WorkflowNodeIn[]
  links?: WorkflowLinkIn[]
}

export interface WorkflowJobNode {
  id: string
  workflow_job_id: string
  workflow_node_id: string
  job_template_id: string | null
  job_id: string | null
  name: string
  status: string
  error_message: string
  started_at: string | null
  finished_at: string | null
}

export interface WorkflowJobNodeLink {
  id: string
  workflow_job_id: string
  source_node_id: string
  target_node_id: string
  condition: 'success' | 'failure' | 'always'
}

export interface WorkflowJob {
  id: string
  workflow_template_id: string
  workflow_name: string
  organization_id: string
  status: string
  nodes: WorkflowJobNode[]
  links: WorkflowJobNodeLink[]
  started_at: string | null
  finished_at: string | null
  created_at: string
  updated_at: string
}
