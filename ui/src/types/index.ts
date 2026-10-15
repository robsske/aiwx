// ─── Organization ─────────────────────────────────────────────────────────────

export interface Organization {
  id: string
  name: string
  display_name: string
  status: 'active' | 'inactive'
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
  variables: Record<string, any>
  enabled: boolean
}

// ─── Job Template ─────────────────────────────────────────────────────────────

export interface JobTemplate {
  id: string
  name: string
  organization_id: string
  project_id: string
  inventory_id: string | null
  execution_environment_id: string | null
  playbook: string
  credential_ids: string[]
  created_at: string
  updated_at: string
}

// ─── Pagination ───────────────────────────────────────────────────────────────

export interface PaginatedResponse<T> {
  results: T[]
  next_cursor: string | null
  has_more: boolean
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
