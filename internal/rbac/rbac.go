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

// RoleLookup fetches effective roles for a user.
type RoleLookup interface {
	EffectiveRoles(ctx context.Context, userID uuid.UUID) ([]domain.EffectiveRole, error)
}

// Checker evaluates permissions.
type Checker struct {
	roles RoleLookup
}

// New creates a new permission checker.
func New(roles RoleLookup) *Checker {
	return &Checker{roles: roles}
}

// Check evaluates whether a user can perform an action on a resource.
func (c *Checker) Check(ctx context.Context, userID uuid.UUID, action Action, resource Resource) (bool, error) {
	roles, err := c.roles.EffectiveRoles(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("load effective roles: %w", err)
	}

	// Platform-admin short-circuit: allowed everywhere.
	for _, er := range roles {
		if er.Role == domain.RolePlatformAdmin {
			return true, nil
		}
	}

	// Build org set for this user.
	orgSet := map[uuid.UUID]bool{}
	for _, er := range roles {
		orgSet[er.OrgID] = true
	}

	// Resource must belong to one of the user's orgs.
	if !orgSet[resource.OrganizationID] {
		return false, nil
	}

	// Check each role that applies to this resource.
	for _, er := range roles {
		// Team-scoped roles only apply when the resource team matches
		// or the resource is org-level (TeamID is nil) and the role is org-level.
		if er.TeamID != nil {
			if resource.TeamID == nil {
				// Team role does not apply to org-level resources.
				continue
			}
			if *er.TeamID != *resource.TeamID {
				continue
			}
		}

		if RoleAllows(er.Role, resource.Type, action) {
			return true, nil
		}
	}

	return false, nil
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
