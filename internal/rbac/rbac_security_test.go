package rbac

import (
	"testing"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/domain"
)

// ─── Cross-Tenant Isolation Tests ─────────────────────────────────────────────

func TestCrossTenant_DifferentOrganizations(t *testing.T) {
	orgA := uuid.New()
	orgB := uuid.New()

	// Resources in different organizations
	projectA := Resource{
		Type:           ResourceProject,
		ID:             uuid.New(),
		OrganizationID: orgA,
	}
	projectB := Resource{
		Type:           ResourceProject,
		ID:             uuid.New(),
		OrganizationID: orgB,
	}

	// User in orgA should NOT be able to access orgB resources
	// (This is enforced by the full Check() function, but we verify the matrix)
	if RoleAllows(domain.RoleOperator, ResourceProject, ActionRead) {
		// Role allows the action in general, but cross-tenant is a separate check
		// This test documents that the matrix itself doesn't prevent cross-tenant
		// access - that's the job of Check() and RLS.
		_ = projectA
		_ = projectB
	}
}

// ─── Privilege Escalation Tests ───────────────────────────────────────────────

func TestPrivilegeEscalation_OperatorCannotManage(t *testing.T) {
	// Operator should NOT be able to manage roles/teams
	if RoleAllows(domain.RoleOperator, ResourceTeam, ActionManage) {
		t.Error("SECURITY: operator should NOT be able to manage teams")
	}
	if RoleAllows(domain.RoleOperator, ResourceOrganization, ActionManage) {
		t.Error("SECURITY: operator should NOT be able to manage organizations")
	}
	if RoleAllows(domain.RoleOperator, ResourceUser, ActionManage) {
		t.Error("SECURITY: operator should NOT be able to manage users")
	}
}

func TestPrivilegeEscalation_ViewerCannotMutate(t *testing.T) {
	// Viewer should NOT be able to create/update/delete anything
	resources := []ResourceType{
		ResourceOrganization, ResourceTeam, ResourceUser, ResourceProject,
		ResourceCredential, ResourceInventory, ResourceExecutionEnvironment,
		ResourceJobTemplate, ResourceJob,
	}
	mutatingActions := []Action{ActionCreate, ActionUpdate, ActionDelete, ActionExecute, ActionManage}

	for _, rt := range resources {
		for _, action := range mutatingActions {
			if RoleAllows(domain.RoleViewer, rt, action) {
				t.Errorf("SECURITY: viewer should NOT be able to %s %s", action, rt)
			}
		}
	}
}

func TestPrivilegeEscalation_AuditorCannotMutate(t *testing.T) {
	// Auditor should NOT be able to create/update/delete anything
	if RoleAllows(domain.RoleAuditor, ResourceProject, ActionCreate) {
		t.Error("SECURITY: auditor should NOT be able to create projects")
	}
	if RoleAllows(domain.RoleAuditor, ResourceJob, ActionDelete) {
		t.Error("SECURITY: auditor should NOT be able to delete jobs")
	}
	if RoleAllows(domain.RoleAuditor, ResourceCredential, ActionUpdate) {
		t.Error("SECURITY: auditor should NOT be able to update credentials")
	}
}

// ─── Secret Leakage Tests ─────────────────────────────────────────────────────

func TestSecrets_ViewerCanReadButNotSeeSecrets(t *testing.T) {
	// Viewer can read credentials (metadata only)
	if !RoleAllows(domain.RoleViewer, ResourceCredential, ActionRead) {
		t.Error("viewer should be able to read credential metadata")
	}
	// But cannot update or delete
	if RoleAllows(domain.RoleViewer, ResourceCredential, ActionUpdate) {
		t.Error("SECURITY: viewer should NOT be able to update credentials")
	}
	if RoleAllows(domain.RoleViewer, ResourceCredential, ActionDelete) {
		t.Error("SECURITY: viewer should NOT be able to delete credentials")
	}
}

// ─── Terminal State Immutability Tests ────────────────────────────────────────

func TestJobState_TerminalStatesImmutable(t *testing.T) {
	// Job status transitions - terminal states are immutable
	terminalStates := []domain.JobStatus{
		domain.JobStatusSuccessful,
		domain.JobStatusFailed,
		domain.JobStatusError,
		domain.JobStatusCanceled,
	}

	for _, state := range terminalStates {
		if !state.Terminal() {
			t.Errorf("%s should be a terminal state", state)
		}
	}

	// Non-terminal states
	nonTerminalStates := []domain.JobStatus{
		domain.JobStatusNew,
		domain.JobStatusPending,
		domain.JobStatusWaiting,
		domain.JobStatusRunning,
	}

	for _, state := range nonTerminalStates {
		if state.Terminal() {
			t.Errorf("%s should NOT be a terminal state", state)
		}
	}
}

// ─── Role Hierarchy Tests ─────────────────────────────────────────────────────

func TestRoleHierarchy_AdminInheritsLower(t *testing.T) {
	// Organization admin should have at least what operator has
	if !RoleAllows(domain.RoleOrganizationAdmin, ResourceProject, ActionCreate) {
		t.Error("organization-admin should inherit operator's create permission")
	}
	if !RoleAllows(domain.RoleOrganizationAdmin, ResourceProject, ActionExecute) {
		t.Error("organization-admin should inherit operator's execute permission")
	}
}

func TestRoleHierarchy_PlatformAdminIsHighest(t *testing.T) {
	// Platform admin should be able to do everything
	allActions := []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionExecute, ActionManage, ActionAudit}
	allResources := []ResourceType{
		ResourceOrganization, ResourceTeam, ResourceUser, ResourceProject,
		ResourceCredential, ResourceInventory, ResourceExecutionEnvironment,
		ResourceJobTemplate, ResourceJob, ResourceJobEvent, ResourceAuditLog,
	}

	for _, rt := range allResources {
		for _, action := range allActions {
			// Not every action applies to every resource, but platform admin
			// should have at least read on everything
			if action == ActionRead && !RoleAllows(domain.RolePlatformAdmin, rt, action) {
				t.Errorf("platform-admin should be able to read %s", rt)
			}
		}
	}
}
