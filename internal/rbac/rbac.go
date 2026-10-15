// Package rbac provides permission evaluation for AIWX.
package rbac

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/domain"
)

// Action represents a permission action.
type Action string

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionExecute Action = "execute"
	ActionManage Action = "manage"
	ActionAudit  Action = "audit"
)

// ResourceType represents a resource type for permission checks.
type ResourceType string

const (
	ResourceOrganization         ResourceType = "organization"
	ResourceTeam                 ResourceType = "team"
	ResourceUser                 ResourceType = "user"
	ResourceProject              ResourceType = "project"
	ResourceCredential           ResourceType = "credential"
	ResourceInventory            ResourceType = "inventory"
	ResourceExecutionEnvironment ResourceType = "execution_environment"
	ResourceJobTemplate          ResourceType = "job_template"
	ResourceJob                  ResourceType = "job"
	ResourceJobEvent             ResourceType = "job_event"
	ResourceAuditLog             ResourceType = "audit_log"
)

// Resource represents a resource for permission evaluation.
type Resource struct {
	Type           ResourceType
	ID             uuid.UUID
	OrganizationID uuid.UUID
	TeamID         *uuid.UUID
}

// Checker evaluates permissions.
type Checker struct {
	// repos will be injected when database is available
}

// New creates a new permission checker.
func New() *Checker {
	return &Checker{}
}

// Check evaluates whether a user can perform an action on a resource.
func (c *Checker) Check(ctx context.Context, userID uuid.UUID, action Action, resource Resource) (bool, error) {
	// TODO: Implement full RBAC evaluation:
	// 1. Check if user has platform-admin role (bypass all checks)
	// 2. Get user's organization IDs through team membership
	// 3. Check if resource belongs to one of user's organizations
	// 4. Get user's effective roles for this resource type
	// 5. Check if any role grants the requested action
	// 6. Check scope (team-scoped roles only apply to team-owned resources)

	// Placeholder: deny all until implemented
	return false, fmt.Errorf("rbac evaluation not yet implemented")
}

// PermissionMatrix defines which roles can perform which actions on which resources.
// This is the authoritative source from permissions-matrix.md.
var PermissionMatrix = map[domain.RoleName]map[ResourceType]map[Action]bool{
	domain.RolePlatformAdmin: {
		ResourceOrganization: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionManage: true,
		},
		ResourceTeam: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionManage: true,
		},
		ResourceUser: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionManage: true,
		},
		ResourceProject: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionExecute: true, ActionManage: true,
		},
		ResourceCredential: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionExecute: true, ActionManage: true,
		},
		ResourceInventory: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionExecute: true, ActionManage: true,
		},
		ResourceExecutionEnvironment: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionManage: true,
		},
		ResourceJobTemplate: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionExecute: true, ActionManage: true,
		},
		ResourceJob: {
			ActionCreate: true, ActionRead: true,
			ActionDelete: true, ActionExecute: true, ActionAudit: true,
		},
		ResourceJobEvent: {
			ActionRead: true, ActionAudit: true,
		},
		ResourceAuditLog: {
			ActionRead: true, ActionAudit: true,
		},
	},
	domain.RoleOrganizationAdmin: {
		ResourceOrganization: {
			ActionRead: true, ActionUpdate: true, ActionManage: true,
		},
		ResourceTeam: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionManage: true,
		},
		ResourceUser: {
			ActionRead: true, ActionManage: true,
		},
		ResourceProject: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionExecute: true, ActionManage: true,
		},
		ResourceCredential: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionExecute: true, ActionManage: true,
		},
		ResourceInventory: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionExecute: true, ActionManage: true,
		},
		ResourceExecutionEnvironment: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionManage: true,
		},
		ResourceJobTemplate: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionDelete: true, ActionExecute: true, ActionManage: true,
		},
		ResourceJob: {
			ActionCreate: true, ActionRead: true,
			ActionDelete: true, ActionExecute: true, ActionAudit: true,
		},
		ResourceJobEvent: {
			ActionRead: true, ActionAudit: true,
		},
		ResourceAuditLog: {
			ActionRead: true, ActionAudit: true,
		},
	},
	domain.RoleOperator: {
		ResourceOrganization: {
			ActionRead: true,
		},
		ResourceTeam: {
			ActionRead: true,
		},
		ResourceUser: {
			ActionRead: true,
		},
		ResourceProject: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionExecute: true,
		},
		ResourceCredential: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionExecute: true,
		},
		ResourceInventory: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionExecute: true,
		},
		ResourceExecutionEnvironment: {
			ActionRead: true,
		},
		ResourceJobTemplate: {
			ActionCreate: true, ActionRead: true, ActionUpdate: true,
			ActionExecute: true,
		},
		ResourceJob: {
			ActionCreate: true, ActionRead: true,
			ActionExecute: true,
		},
		ResourceJobEvent: {
			ActionRead: true,
		},
	},
	domain.RoleViewer: {
		ResourceOrganization: {
			ActionRead: true,
		},
		ResourceTeam: {
			ActionRead: true,
		},
		ResourceUser: {
			ActionRead: true,
		},
		ResourceProject: {
			ActionRead: true,
		},
		ResourceCredential: {
			ActionRead: true,
		},
		ResourceInventory: {
			ActionRead: true,
		},
		ResourceExecutionEnvironment: {
			ActionRead: true,
		},
		ResourceJobTemplate: {
			ActionRead: true,
		},
		ResourceJob: {
			ActionRead: true,
		},
		ResourceJobEvent: {
			ActionRead: true,
		},
	},
	domain.RoleAuditor: {
		ResourceOrganization: {
			ActionRead: true,
		},
		ResourceTeam: {
			ActionRead: true,
		},
		ResourceUser: {
			ActionRead: true,
		},
		ResourceProject: {
			ActionRead: true,
		},
		ResourceCredential: {
			ActionRead: true,
		},
		ResourceInventory: {
			ActionRead: true,
		},
		ResourceExecutionEnvironment: {
			ActionRead: true,
		},
		ResourceJobTemplate: {
			ActionRead: true,
		},
		ResourceJob: {
			ActionRead: true, ActionAudit: true,
		},
		ResourceJobEvent: {
			ActionRead: true, ActionAudit: true,
		},
		ResourceAuditLog: {
			ActionRead: true, ActionAudit: true,
		},
	},
}

// RoleAllows checks if a role allows an action on a resource type.
func RoleAllows(role domain.RoleName, resourceType ResourceType, action Action) bool {
	perms, ok := PermissionMatrix[role]
	if !ok {
		return false
	}
	resourcePerms, ok := perms[resourceType]
	if !ok {
		return false
	}
	return resourcePerms[action]
}
