package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/auth"
	"github.com/rverhe/aiwx/internal/domain"
)

// ─── Auth Handlers ────────────────────────────────────────────────────────────

// handleAuthToken implements the password grant: POST /api/v1/auth/token
func (d *Deps) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GrantType string `json:"grant_type"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if req.GrantType != "" && req.GrantType != "password" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unsupported grant_type")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "username and password are required")
		return
	}

	cred, err := d.Repos.Users.GetCredentialByEmail(r.Context(), req.Username)
	if err != nil || cred.PasswordHash == "" || !auth.CheckPassword(cred.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials")
		return
	}
	if !cred.User.IsActive {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Account is disabled")
		return
	}

	roles, orgID, err := d.Repos.RoleAssignments.ListRolesForUser(r.Context(), cred.User.ID)
	if err != nil || len(roles) == 0 {
		roles = []string{string(domain.RolePlatformAdmin)}
	}

	accessToken, err := d.Issuer.Issue(cred.User.ID, orgID, roles)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(d.Issuer.TTL().Seconds()),
	})
}

func (d *Deps) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}

	user, err := d.Repos.Users.GetByID(r.Context(), identity.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "User not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"display_name":  user.DisplayName,
		"auth_type":     identity.AuthType,
		"organization":  identity.OrganizationID,
		"roles":         identity.Roles,
	})
}

// ─── Team Handlers ────────────────────────────────────────────────────────────

func (d *Deps) handleListTeams(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r.PathValue("orgId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid organization ID")
		return
	}

	teams, err := d.Repos.Teams.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list teams")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  teams,
		"has_more": false,
	})
}

func (d *Deps) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r.PathValue("orgId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid organization ID")
		return
	}

	var input domain.TeamCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Field 'name' is required")
		return
	}

	team, err := d.Repos.Teams.Create(r.Context(), input, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create team")
		return
	}

	writeJSON(w, http.StatusCreated, team)
}

func (d *Deps) handleGetTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := parseUUID(r.PathValue("teamId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid team ID")
		return
	}

	team, err := d.Repos.Teams.GetByID(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Team not found")
		return
	}

	writeJSON(w, http.StatusOK, team)
}

func (d *Deps) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := parseUUID(r.PathValue("teamId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid team ID")
		return
	}

	if err := d.Repos.Teams.Delete(r.Context(), teamID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete team")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (d *Deps) handleAddTeamMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := parseUUID(r.PathValue("teamId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid team ID")
		return
	}

	var input struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if err := d.Repos.Teams.AddMember(r.Context(), teamID, input.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to add member")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

func (d *Deps) handleRemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	teamID, err := parseUUID(r.PathValue("teamId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid team ID")
		return
	}
	userID, err := parseUUID(r.PathValue("userId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid user ID")
		return
	}

	if err := d.Repos.Teams.RemoveMember(r.Context(), teamID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to remove member")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Role Assignment Handlers ─────────────────────────────────────────────────

func (d *Deps) handleCreateRoleAssignment(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r.PathValue("orgId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid organization ID")
		return
	}

	var input domain.RoleAssignmentCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	// Validate that exactly one of user_id or team_id is set
	if (input.UserID == nil) == (input.TeamID == nil) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Exactly one of 'user_id' or 'team_id' is required")
		return
	}

	assignment, err := d.Repos.RoleAssignments.Create(r.Context(), input, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create role assignment")
		return
	}

	writeJSON(w, http.StatusCreated, assignment)
}

func (d *Deps) handleDeleteRoleAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid assignment ID")
		return
	}

	if err := d.Repos.RoleAssignments.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete role assignment")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Credential Handlers ──────────────────────────────────────────────────────

func (d *Deps) handleListCredentialTypes(w http.ResponseWriter, r *http.Request) {
	types, err := d.Repos.Credentials.ListTypes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list credential types")
		return
	}
	if types == nil {
		types = []domain.CredentialType{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"results":  types,
		"has_more": false,
	})
}

func (d *Deps) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	creds, err := d.Repos.Credentials.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list credentials")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  creds,
		"has_more": false,
	})
}

func (d *Deps) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	var input domain.CredentialCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Field 'name' is required")
		return
	}
	if input.CredentialTypeID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Field 'credential_type_id' is required")
		return
	}

	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	cred, err := d.Repos.Credentials.Create(r.Context(), input, orgID)
	if err != nil {
		d.Logger.Error("create credential failed", "error", err, "org_id", orgID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create credential")
		return
	}

	writeJSON(w, http.StatusCreated, cred)
}

func (d *Deps) handleGetCredential(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid credential ID")
		return
	}

	cred, err := d.Repos.Credentials.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Credential not found")
		return
	}

	writeJSON(w, http.StatusOK, cred)
}

func (d *Deps) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid credential ID")
		return
	}

	if err := d.Repos.Credentials.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete credential")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Inventory Handlers ───────────────────────────────────────────────────────

func (d *Deps) handleListInventories(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	invs, err := d.Repos.Inventories.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list inventories")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  invs,
		"has_more": false,
	})
}

func (d *Deps) handleCreateInventory(w http.ResponseWriter, r *http.Request) {
	var input domain.InventoryCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Field 'name' is required")
		return
	}

	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	inv, err := d.Repos.Inventories.Create(r.Context(), input, orgID)
	if err != nil {
		d.Logger.Error("create inventory failed", "error", err, "org_id", orgID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create inventory")
		return
	}

	writeJSON(w, http.StatusCreated, inv)
}

func (d *Deps) handleGetInventory(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid inventory ID")
		return
	}

	inv, err := d.Repos.Inventories.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inventory not found")
		return
	}

	writeJSON(w, http.StatusOK, inv)
}

func (d *Deps) handleDeleteInventory(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid inventory ID")
		return
	}

	if err := d.Repos.Inventories.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete inventory")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (d *Deps) handleListInventoryHosts(w http.ResponseWriter, r *http.Request) {
	invID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid inventory ID")
		return
	}

	hosts, err := d.Repos.Inventories.ListHosts(r.Context(), invID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list hosts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  hosts,
		"has_more": false,
	})
}

func (d *Deps) handleCreateInventoryHost(w http.ResponseWriter, r *http.Request) {
	invID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid inventory ID")
		return
	}

	var input struct {
		Name      string         `json:"name"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Field 'name' is required")
		return
	}

	host, err := d.Repos.Inventories.AddHost(r.Context(), invID, input.Name, input.Variables)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to add host")
		return
	}

	writeJSON(w, http.StatusCreated, host)
}

func (d *Deps) handleDeleteInventoryHost(w http.ResponseWriter, r *http.Request) {
	invID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid inventory ID")
		return
	}
	hostID, err := parseUUID(r.PathValue("hostId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid host ID")
		return
	}

	if err := d.Repos.Inventories.RemoveHost(r.Context(), invID, hostID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to remove host")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Execution Environment Handlers ───────────────────────────────────────────

func (d *Deps) handleListExecutionEnvironments(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	ees, err := d.Repos.ExecutionEnvironments.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list execution environments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  ees,
		"has_more": false,
	})
}

func (d *Deps) handleCreateExecutionEnvironment(w http.ResponseWriter, r *http.Request) {
	var input domain.ExecutionEnvironmentCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if input.Name == "" || input.Image == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Fields 'name' and 'image' are required")
		return
	}

	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	ee, err := d.Repos.ExecutionEnvironments.Create(r.Context(), input, orgID)
	if err != nil {
		d.Logger.Error("create execution environment failed", "error", err, "org_id", orgID)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create execution environment")
		return
	}

	writeJSON(w, http.StatusCreated, ee)
}

func (d *Deps) handleGetExecutionEnvironment(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid EE ID")
		return
	}

	ee, err := d.Repos.ExecutionEnvironments.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Execution environment not found")
		return
	}

	writeJSON(w, http.StatusOK, ee)
}

func (d *Deps) handleDeleteExecutionEnvironment(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid EE ID")
		return
	}

	if err := d.Repos.ExecutionEnvironments.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete execution environment")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Project Handlers (extended) ──────────────────────────────────────────────

func (d *Deps) handleListProjects(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgID(w, r)
	if !ok {
		return
	}
	projects, err := d.Repos.Projects.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list projects")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":  projects,
		"has_more": false,
	})
}

func (d *Deps) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid project ID")
		return
	}

	var input domain.ProjectCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	project, err := d.Repos.Projects.Update(r.Context(), id, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update project")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (d *Deps) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid project ID")
		return
	}

	if err := d.Repos.Projects.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete project")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (d *Deps) handleSyncProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid project ID")
		return
	}

	project, err := d.Repos.Projects.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
		return
	}

	if project.SyncStatus == "running" {
		writeError(w, http.StatusConflict, "CONFLICT", "Sync already in progress")
		return
	}

	// TODO: Trigger async project sync
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "sync_started",
		"project_id": id.String(),
	})
}
