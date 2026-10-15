package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/auth"
	"github.com/rverhe/aiwx/internal/domain"
)

// ─── Organization Handlers ────────────────────────────────────────────────────

func (d *Deps) handleListOrganizations(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	cursor := r.URL.Query().Get("cursor")

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	// Platform admin sees all; others see only their orgs.
	var orgs []domain.Organization
	var nextCursor string
	var hasMore bool
	var err error

	if scope.IsPlatformAdmin {
		orgs, nextCursor, hasMore, err = d.Repos.Organizations.List(r.Context(), limit, cursor)
	} else {
		orgs, nextCursor, hasMore, err = d.Repos.Organizations.ListByIDs(r.Context(), scope.OrgIDs, limit, cursor)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list organizations")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":     orgs,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}

func (d *Deps) handleCreateOrganization(w http.ResponseWriter, r *http.Request) {
	var input domain.OrganizationCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Field 'name' is required")
		return
	}

	org, err := d.Repos.Organizations.Create(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create organization")
		return
	}

	writeJSON(w, http.StatusCreated, org)
}

func (d *Deps) handleGetOrganization(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid organization ID")
		return
	}

	org, err := d.Repos.Organizations.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found")
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// ─── User Handlers ────────────────────────────────────────────────────────────

func (d *Deps) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var input domain.UserCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Email == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Field 'email' is required")
		return
	}

	user, err := d.Repos.Users.Create(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (d *Deps) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid user ID")
		return
	}

	user, err := d.Repos.Users.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "User not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// ─── Project Handlers ─────────────────────────────────────────────────────────

func (d *Deps) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var input domain.ProjectCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" || input.SCMURL == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Fields 'name' and 'scm_url' are required")
		return
	}

	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}

	project, err := d.Repos.Projects.Create(r.Context(), input, orgID)
	if err != nil {
		d.Logger.Error("create project failed", "error", err, "org_id", orgID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create project")
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

func (d *Deps) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid project ID")
		return
	}

	scope, ok := d.requireScope(w, r)
	if !ok {
		return
	}

	project, err := d.Repos.Projects.GetByIDScoped(r.Context(), id, scope)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// requestScope builds the caller's authorization scope from the auth context.
func (d *Deps) requestScope(r *http.Request) (domain.Scope, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		return domain.Scope{}, false
	}

	// Load effective roles to determine org membership and platform-admin status.
	effectiveRoles, err := d.Repos.RoleAssignments.EffectiveRoles(r.Context(), id.UserID)
	if err != nil {
		return domain.Scope{}, false
	}

	scope := domain.Scope{UserID: id.UserID}
	orgSet := map[uuid.UUID]bool{}
	for _, er := range effectiveRoles {
		if er.Role == domain.RolePlatformAdmin {
			scope.IsPlatformAdmin = true
		}
		if !orgSet[er.OrgID] {
			orgSet[er.OrgID] = true
			scope.OrgIDs = append(scope.OrgIDs, er.OrgID)
		}
	}
	return scope, true
}

// requireScope writes a 401 and returns a zero scope when no auth is present.
func (d *Deps) requireScope(w http.ResponseWriter, r *http.Request) (domain.Scope, bool) {
	scope, ok := d.requestScope(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return domain.Scope{}, false
	}
	return scope, true
}

// requestOrgID returns the caller's primary organization from the auth context.
func requestOrgID(r *http.Request) (uuid.UUID, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok || id.OrganizationID == uuid.Nil {
		return uuid.Nil, false
	}
	return id.OrganizationID, true
}

// requireOrgID writes a 401 and returns false when no org is present.
func requireOrgID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	orgID, ok := requestOrgID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "No organization in auth context")
		return uuid.Nil, false
	}
	return orgID, true
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func parseLimit(r *http.Request) int {
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := parseInt(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	return limit
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func stringsContain(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
