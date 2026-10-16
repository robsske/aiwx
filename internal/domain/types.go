// Package domain contains the core data types and validation for AIWX.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ─── Organization ─────────────────────────────────────────────────────────────

type Organization struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type OrganizationCreate struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
}

type OrganizationUpdate struct {
	DisplayName *string `json:"display_name,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// ─── Team ─────────────────────────────────────────────────────────────────────

type Team struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"display_name"`
	OrganizationID uuid.UUID `json:"organization_id"`
	MemberCount    int       `json:"member_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TeamCreate struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
}

// ─── User ─────────────────────────────────────────────────────────────────────

type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserCreate struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	Password    string `json:"password,omitempty"`
}

// ─── Role Assignment ──────────────────────────────────────────────────────────

// RoleName represents a built-in RBAC role.
type RoleName string

const (
	RolePlatformAdmin    RoleName = "platform-admin"
	RoleOrganizationAdmin RoleName = "organization-admin"
	RoleTeamAdmin        RoleName = "team-admin"
	RoleOperator         RoleName = "operator"
	RoleViewer           RoleName = "viewer"
	RoleAuditor          RoleName = "auditor"
)

type RoleAssignment struct {
	ID             uuid.UUID  `json:"id"`
	RoleName       RoleName   `json:"role_name"`
	UserID         *uuid.UUID `json:"user_id"`
	TeamID         *uuid.UUID `json:"team_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	CreatedAt      time.Time  `json:"created_at"`
}

type RoleAssignmentCreate struct {
	RoleName RoleName  `json:"role_name"`
	UserID   *uuid.UUID `json:"user_id"`
	TeamID   *uuid.UUID `json:"team_id"`
}

// ─── Project ──────────────────────────────────────────────────────────────────

type Project struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	SCMURL           string     `json:"scm_url"`
	SCMBranch        string     `json:"scm_branch"`
	SCMCredentialID  *uuid.UUID `json:"scm_credential_id"`
	LatestRevision   *string    `json:"latest_revision"`
	SyncStatus       string     `json:"sync_status"`
	LastSyncAt       *time.Time `json:"last_sync_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ProjectCreate struct {
	OrganizationID  *uuid.UUID `json:"organization_id,omitempty"`
	Name            string     `json:"name"`
	SCMURL          string     `json:"scm_url"`
	SCMBranch       string     `json:"scm_branch,omitempty"`
	SCMCredentialID *uuid.UUID `json:"scm_credential_id,omitempty"`
}

// ─── Credential ───────────────────────────────────────────────────────────────

type CredentialType struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Namespace    string    `json:"namespace"`
	InputsSchema map[string]any `json:"inputs_schema"`
}

type Credential struct {
	ID               uuid.UUID      `json:"id"`
	Name             string         `json:"name"`
	OrganizationID   uuid.UUID      `json:"organization_id"`
	CredentialTypeID uuid.UUID      `json:"credential_type_id"`
	Inputs           map[string]any `json:"inputs"` // secret values redacted
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type CredentialCreate struct {
	Name             string         `json:"name"`
	CredentialTypeID uuid.UUID      `json:"credential_type_id"`
	Inputs           map[string]any `json:"inputs"`
}

// ─── Inventory ────────────────────────────────────────────────────────────────

type Inventory struct {
	ID                          uuid.UUID `json:"id"`
	Name                        string    `json:"name"`
	Description                 string    `json:"description"`
	OrganizationID              uuid.UUID `json:"organization_id"`
	HostCount                   int       `json:"host_count"`
	GroupCount                  int       `json:"group_count"`
	HasActiveInventorySources   bool      `json:"has_active_inventory_sources"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

type InventoryCreate struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type InventoryHost struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InventoryID uuid.UUID      `json:"inventory_id"`
	Variables   map[string]any `json:"variables"`
	Enabled     bool           `json:"enabled"`
	Groups      []uuid.UUID    `json:"groups,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type InventoryHostCreate struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
}

type InventoryGroup struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	InventoryID   uuid.UUID      `json:"inventory_id"`
	Variables     map[string]any `json:"variables"`
	ParentGroupID *uuid.UUID     `json:"parent_group_id"`
	HostCount     int            `json:"host_count"`
	ChildCount    int            `json:"child_count"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type InventoryGroupCreate struct {
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
	ParentGroupID *uuid.UUID     `json:"parent_group_id,omitempty"`
}

// ─── Execution Environment ────────────────────────────────────────────────────

type ExecutionEnvironment struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Image          string     `json:"image"`
	Digest         *string    `json:"digest"`
	PullPolicy     string     `json:"pull_policy"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type ExecutionEnvironmentCreate struct {
	Name       string  `json:"name"`
	Image      string  `json:"image"`
	Digest     *string `json:"digest,omitempty"`
	PullPolicy string  `json:"pull_policy,omitempty"`
}

// ─── Job Template ─────────────────────────────────────────────────────────────

type JobTemplate struct {
	ID                     uuid.UUID   `json:"id"`
	Name                   string      `json:"name"`
	Description            string      `json:"description"`
	OrganizationID         uuid.UUID   `json:"organization_id"`
	ProjectID              uuid.UUID   `json:"project_id"`
	InventoryID            *uuid.UUID  `json:"inventory_id"`
	ExecutionEnvironmentID *uuid.UUID  `json:"execution_environment_id"`
	Playbook               string      `json:"playbook"`
	CredentialIDs          []uuid.UUID `json:"credential_ids"`
	// AWX-style targeting
	Limit               string `json:"limit"`
	Verbosity           int    `json:"verbosity"`
	Forks               int    `json:"forks"`
	JobTags             string `json:"job_tags"`
	SkipTags            string `json:"skip_tags"`
	ExtraVars           map[string]any `json:"extra_vars"`
	// Launch behavior
	AskInventoryOnLaunch  bool `json:"ask_inventory_on_launch"`
	AskLimitOnLaunch      bool `json:"ask_limit_on_launch"`
	AskVerbosityOnLaunch  bool `json:"ask_verbosity_on_launch"`
	AskVariablesOnLaunch  bool `json:"ask_variables_on_launch"`
	AskCredentialOnLaunch bool `json:"ask_credential_on_launch"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
}

type JobTemplateCreate struct {
	Name                   string      `json:"name"`
	Description            string      `json:"description,omitempty"`
	ProjectID              uuid.UUID   `json:"project_id"`
	InventoryID            *uuid.UUID  `json:"inventory_id,omitempty"`
	ExecutionEnvironmentID *uuid.UUID  `json:"execution_environment_id,omitempty"`
	Playbook               string      `json:"playbook"`
	CredentialIDs          []uuid.UUID `json:"credential_ids,omitempty"`
	Limit                  string      `json:"limit,omitempty"`
	Verbosity              int         `json:"verbosity,omitempty"`
	Forks                  int         `json:"forks,omitempty"`
	JobTags                string      `json:"job_tags,omitempty"`
	SkipTags               string      `json:"skip_tags,omitempty"`
	ExtraVars              map[string]any `json:"extra_vars,omitempty"`
	AskInventoryOnLaunch   bool        `json:"ask_inventory_on_launch,omitempty"`
	AskLimitOnLaunch       bool        `json:"ask_limit_on_launch,omitempty"`
	AskVerbosityOnLaunch   bool        `json:"ask_verbosity_on_launch,omitempty"`
	AskVariablesOnLaunch   bool        `json:"ask_variables_on_launch,omitempty"`
	AskCredentialOnLaunch  bool        `json:"ask_credential_on_launch,omitempty"`
}

// ─── Job ──────────────────────────────────────────────────────────────────────

// JobStatus represents the job lifecycle state.
type JobStatus string

const (
	JobStatusNew       JobStatus = "new"
	JobStatusPending   JobStatus = "pending"
	JobStatusWaiting   JobStatus = "waiting"
	JobStatusRunning   JobStatus = "running"
	JobStatusSuccessful JobStatus = "successful"
	JobStatusFailed    JobStatus = "failed"
	JobStatusError     JobStatus = "error"
	JobStatusCanceled  JobStatus = "canceled"
)

// Terminal returns true if the job is in a terminal state.
func (s JobStatus) Terminal() bool {
	return s == JobStatusSuccessful || s == JobStatusFailed || s == JobStatusError || s == JobStatusCanceled
}

type Job struct {
	ID                uuid.UUID  `json:"id"`
	JobTemplateID     *uuid.UUID `json:"job_template_id"`
	OrganizationID    uuid.UUID  `json:"organization_id"`
	Status            JobStatus  `json:"status"`
	ProjectRevision   *string    `json:"project_revision"`
	ArtifactRef       *string    `json:"artifact_ref"`
	KubernetesJobName *string    `json:"kubernetes_job_name"`
	ExitCode          *int       `json:"exit_code"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type JobLaunch struct {
	InventoryID            *uuid.UUID        `json:"inventory_id,omitempty"`
	ExecutionEnvironmentID *uuid.UUID        `json:"execution_environment_id,omitempty"`
	ExtraVars              map[string]any    `json:"extra_vars,omitempty"`
	Credentials            []uuid.UUID       `json:"credentials,omitempty"`
	Limit                  string            `json:"limit,omitempty"`
	Verbosity              *int              `json:"verbosity,omitempty"`
	JobTags                string            `json:"job_tags,omitempty"`
	SkipTags               string            `json:"skip_tags,omitempty"`
}

type JobEvent struct {
	ID        int64          `json:"id"`
	JobID     uuid.UUID      `json:"job_id"`
	EventType string         `json:"event_type"`
	EventData map[string]any `json:"event_data"`
	CreatedAt time.Time      `json:"created_at"`
}

// ─── Pagination ───────────────────────────────────────────────────────────────

type PageInfo struct {
	Results    any    `json:"results"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// ─── Authorization Scope ──────────────────────────────────────────────────────

// Scope represents a caller's authorization scope for tenant isolation.
type Scope struct {
	UserID           uuid.UUID
	OrgIDs           []uuid.UUID
	IsPlatformAdmin  bool
}

// EffectiveRole represents a role assignment with its scope.
type EffectiveRole struct {
	Role  RoleName
	OrgID uuid.UUID
	TeamID *uuid.UUID
}

// AllowsOrg returns true if the scope grants access to the given org.
func (s Scope) AllowsOrg(orgID uuid.UUID) bool {
	if s.IsPlatformAdmin {
		return true
	}
	for _, id := range s.OrgIDs {
		if id == orgID {
			return true
		}
	}
	return false
}

// ─── Errors ───────────────────────────────────────────────────────────────────

type APIError struct {
	ErrorCode string         `json:"error_code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return e.Message
}

// NewValidationError creates a validation error.
func NewValidationError(msg string, details map[string]any) *APIError {
	return &APIError{ErrorCode: "VALIDATION_ERROR", Message: msg, Details: details}
}

// NewNotFoundError creates a not-found error.
func NewNotFoundError(resource string) *APIError {
	return &APIError{ErrorCode: "NOT_FOUND", Message: resource + " not found"}
}

// NewForbiddenError creates a forbidden error.
func NewForbiddenError(msg string) *APIError {
	return &APIError{ErrorCode: "FORBIDDEN", Message: msg}
}

// NewConflictError creates a conflict error.
func NewConflictError(msg string) *APIError {
	return &APIError{ErrorCode: "CONFLICT", Message: msg}
}
