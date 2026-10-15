package rbac

import (
	"testing"

	"github.com/rverhe/aiwx/internal/domain"
)

func TestRoleAllows_PlatformAdmin(t *testing.T) {
	// Platform admin can do everything
	if !RoleAllows(domain.RolePlatformAdmin, ResourceOrganization, ActionCreate) {
		t.Error("platform-admin should be able to create organizations")
	}
	if !RoleAllows(domain.RolePlatformAdmin, ResourceJob, ActionDelete) {
		t.Error("platform-admin should be able to delete jobs")
	}
	if !RoleAllows(domain.RolePlatformAdmin, ResourceAuditLog, ActionAudit) {
		t.Error("platform-admin should be able to audit")
	}
}

func TestRoleAllows_OrganizationAdmin(t *testing.T) {
	// Org admin can manage org resources
	if !RoleAllows(domain.RoleOrganizationAdmin, ResourceProject, ActionCreate) {
		t.Error("organization-admin should be able to create projects")
	}
	if !RoleAllows(domain.RoleOrganizationAdmin, ResourceProject, ActionDelete) {
		t.Error("organization-admin should be able to delete projects")
	}
	// Org admin cannot create organizations
	if RoleAllows(domain.RoleOrganizationAdmin, ResourceOrganization, ActionCreate) {
		t.Error("organization-admin should NOT be able to create organizations")
	}
}

func TestRoleAllows_Operator(t *testing.T) {
	// Operator can run jobs
	if !RoleAllows(domain.RoleOperator, ResourceJobTemplate, ActionExecute) {
		t.Error("operator should be able to execute job templates")
	}
	if !RoleAllows(domain.RoleOperator, ResourceProject, ActionCreate) {
		t.Error("operator should be able to create projects")
	}
	// Operator cannot delete projects
	if RoleAllows(domain.RoleOperator, ResourceProject, ActionDelete) {
		t.Error("operator should NOT be able to delete projects")
	}
	// Operator cannot manage roles
	if RoleAllows(domain.RoleOperator, ResourceTeam, ActionManage) {
		t.Error("operator should NOT be able to manage teams")
	}
}

func TestRoleAllows_Viewer(t *testing.T) {
	// Viewer is read-only
	if !RoleAllows(domain.RoleViewer, ResourceProject, ActionRead) {
		t.Error("viewer should be able to read projects")
	}
	if RoleAllows(domain.RoleViewer, ResourceProject, ActionCreate) {
		t.Error("viewer should NOT be able to create projects")
	}
	if RoleAllows(domain.RoleViewer, ResourceProject, ActionDelete) {
		t.Error("viewer should NOT be able to delete projects")
	}
}

func TestRoleAllows_Auditor(t *testing.T) {
	// Auditor can read and audit
	if !RoleAllows(domain.RoleAuditor, ResourceJob, ActionRead) {
		t.Error("auditor should be able to read jobs")
	}
	if !RoleAllows(domain.RoleAuditor, ResourceJob, ActionAudit) {
		t.Error("auditor should be able to audit jobs")
	}
	// Auditor cannot mutate
	if RoleAllows(domain.RoleAuditor, ResourceJob, ActionDelete) {
		t.Error("auditor should NOT be able to delete jobs")
	}
}

func TestRoleAllows_UnknownRole(t *testing.T) {
	if RoleAllows("unknown-role", ResourceProject, ActionRead) {
		t.Error("unknown role should not have any permissions")
	}
}
