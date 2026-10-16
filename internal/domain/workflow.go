package domain

import (
	"time"

	"github.com/google/uuid"
)

// ─── Workflow Templates (AWX WorkflowJobTemplate) ────────────────────────────

type WorkflowTemplate struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	ExtraVars      map[string]any `json:"extra_vars"`
	Nodes          []WorkflowNode `json:"nodes"`
	Links          []WorkflowLink `json:"links"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type WorkflowNode struct {
	ID                 uuid.UUID  `json:"id"`
	WorkflowTemplateID uuid.UUID  `json:"workflow_template_id"`
	JobTemplateID      *uuid.UUID `json:"job_template_id"`
	Name               string     `json:"name"`
	PositionX          int        `json:"position_x"`
	PositionY          int        `json:"position_y"`
	// Denormalized for UI convenience
	JobTemplateName string `json:"job_template_name,omitempty"`
}

// WorkflowLink mirrors AWX success_nodes / failure_nodes / always_nodes.
type WorkflowLink struct {
	ID                 uuid.UUID `json:"id"`
	WorkflowTemplateID uuid.UUID `json:"workflow_template_id"`
	SourceNodeID       uuid.UUID `json:"source_node_id"`
	TargetNodeID       uuid.UUID `json:"target_node_id"`
	Condition          string    `json:"condition"` // success | failure | always
}

type WorkflowTemplateCreate struct {
	OrganizationID uuid.UUID         `json:"organization_id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	ExtraVars      map[string]any    `json:"extra_vars,omitempty"`
	Nodes          []WorkflowNodeIn  `json:"nodes"`
	Links          []WorkflowLinkIn  `json:"links"`
}

type WorkflowTemplateUpdate struct {
	Name        *string            `json:"name,omitempty"`
	Description *string            `json:"description,omitempty"`
	ExtraVars   map[string]any     `json:"extra_vars,omitempty"`
	Nodes       []WorkflowNodeIn   `json:"nodes"`
	Links       []WorkflowLinkIn   `json:"links"`
}

// WorkflowNodeIn is create/update payload for a graph node (client-generated id ok).
type WorkflowNodeIn struct {
	ID            *uuid.UUID `json:"id,omitempty"`
	JobTemplateID *uuid.UUID `json:"job_template_id"`
	Name          string     `json:"name,omitempty"`
	PositionX     int        `json:"position_x"`
	PositionY     int        `json:"position_y"`
}

type WorkflowLinkIn struct {
	SourceNodeID uuid.UUID `json:"source_node_id"`
	TargetNodeID uuid.UUID `json:"target_node_id"`
	Condition    string    `json:"condition"`
}

// ─── Workflow Jobs (AWX WorkflowJob) ─────────────────────────────────────────

type WorkflowJob struct {
	ID                 uuid.UUID             `json:"id"`
	WorkflowTemplateID uuid.UUID             `json:"workflow_template_id"`
	WorkflowName       string                `json:"workflow_name"`
	OrganizationID     uuid.UUID             `json:"organization_id"`
	Status             string                `json:"status"`
	ExtraVars          map[string]any        `json:"extra_vars"`
	Nodes              []WorkflowJobNode     `json:"nodes"`
	Links              []WorkflowJobNodeLink `json:"links"`
	StartedAt          *time.Time            `json:"started_at"`
	FinishedAt         *time.Time            `json:"finished_at"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
}

type WorkflowJobNode struct {
	ID             uuid.UUID  `json:"id"`
	WorkflowJobID  uuid.UUID  `json:"workflow_job_id"`
	WorkflowNodeID uuid.UUID  `json:"workflow_node_id"`
	JobTemplateID  *uuid.UUID `json:"job_template_id"`
	JobID          *uuid.UUID `json:"job_id"`
	Name           string     `json:"name"`
	Status         string     `json:"status"`
	ErrorMessage   string     `json:"error_message"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}

type WorkflowJobNodeLink struct {
	ID           uuid.UUID `json:"id"`
	WorkflowJobID uuid.UUID `json:"workflow_job_id"`
	SourceNodeID uuid.UUID `json:"source_node_id"`
	TargetNodeID uuid.UUID `json:"target_node_id"`
	Condition    string    `json:"condition"`
}
