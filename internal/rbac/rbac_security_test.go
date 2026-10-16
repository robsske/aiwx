package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/domain"
)

// mockRoleLookup returns a fixed set of effective roles.
type mockRoleLookup struct {
	roles []domain.EffectiveRole
}

func (m *mockRoleLookup) EffectiveRoles(_ context.Context, _ uuid.UUID) ([]domain.EffectiveRole, error) {
	return m.roles, nil
}

func newChecker(roles ...domain.EffectiveRole) *Checker {
	return New(&mockRoleLookup{roles: roles})
}

// ─── Platform Admin ───────────────────────────────────────────────────────────

func TestCheck_PlatformAdmin_Allows(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	c := newChecker(domain.EffectiveRole{Role: domain.RolePlatformAdmin, OrgID: orgID})

	allowed, err := c.Check(context.Background(), userID, ActionDelete, Resource{
		Type: ResourceOrganization, ID: uuid.New(), OrganizationID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("platform-admin should be allowed on any resource")
	}
}

// ─── Viewer Denies Mutations ──────────────────────────────────────────────────

func TestCheck_Viewer_DeniesCreate(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	c := newChecker(domain.EffectiveRole{Role: domain.RoleViewer, OrgID: orgID})

	allowed, err := c.Check(context.Background(), userID, ActionCreate, Resource{
		Type: ResourceProject, ID: uuid.New(), OrganizationID: orgID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("viewer should NOT be allowed to create projects")
	}

	allowed, err = c.Check(context.Background(), userID, ActionRead, Resource{
		Type: ResourceProject, ID: uuid.New(), OrganizationID: orgID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("viewer should be allowed to read projects")
	}
}

// ─── Cross-Tenant Isolation ───────────────────────────────────────────────────

func TestCheck_OrgA_DeniesOrgB(t *testing.T) {
	userID := uuid.New()
	orgA := uuid.New()
	orgB := uuid.New()
	c := newChecker(domain.EffectiveRole{Role: domain.RoleOrganizationAdmin, OrgID: orgA})

	allowed, err := c.Check(context.Background(), userID, ActionRead, Resource{
		Type: ResourceCredential, ID: uuid.New(), OrganizationID: orgB,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("org A role must NOT allow access to org B resource")
	}
}

// ─── Team Scope ───────────────────────────────────────────────────────────────

func TestCheck_TeamRole_AppliesToTeamResource(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	teamID := uuid.New()
	otherTeam := uuid.New()
	c := newChecker(domain.EffectiveRole{Role: domain.RoleOperator, OrgID: orgID, TeamID: &teamID})

	// Matching team resource → allowed
	allowed, err := c.Check(context.Background(), userID, ActionRead, Resource{
		Type: ResourceJob, ID: uuid.New(), OrganizationID: orgID, TeamID: &teamID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("team role should allow action on matching team resource")
	}

	// Different team resource → denied
	allowed, err = c.Check(context.Background(), userID, ActionRead, Resource{
		Type: ResourceJob, ID: uuid.New(), OrganizationID: orgID, TeamID: &otherTeam,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("team role must NOT allow action on different team resource")
	}

	// Org-level resource (no team) → team role does not apply
	allowed, err = c.Check(context.Background(), userID, ActionCreate, Resource{
		Type: ResourceProject, ID: uuid.New(), OrganizationID: orgID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("team role must NOT allow action on org-level resource")
	}
}

// ─── Org-Scoped Role on Org-Level Resource ────────────────────────────────────

func TestCheck_OrgRole_AppliesToOrgResource(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	c := newChecker(domain.EffectiveRole{Role: domain.RoleOperator, OrgID: orgID})

	allowed, err := c.Check(context.Background(), userID, ActionCreate, Resource{
		Type: ResourceJobTemplate, ID: uuid.New(), OrganizationID: orgID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("org-level role should allow action on org-level resource")
	}
}

// ─── No Roles ─────────────────────────────────────────────────────────────────

func TestCheck_NoRoles_Denies(t *testing.T) {
	userID := uuid.New()
	c := newChecker()

	allowed, err := c.Check(context.Background(), userID, ActionRead, Resource{
		Type: ResourceProject, ID: uuid.New(), OrganizationID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("user with no roles should be denied")
	}
}
