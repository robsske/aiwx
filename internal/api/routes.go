// Package api provides the HTTP API layer for AIWX.
package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/auth"
	"github.com/rverhe/aiwx/internal/metrics"
	"github.com/rverhe/aiwx/internal/outbox"
	"github.com/rverhe/aiwx/internal/rbac"
	"github.com/rverhe/aiwx/internal/repository"
)

// Deps holds dependencies for the API layer.
type Deps struct {
	Logger      *slog.Logger
	DB          *repository.Pool
	Repos       *repository.Repositories
	Extractor   *auth.Extractor
	Issuer      *auth.TokenIssuer
	Outbox      *outbox.Store
	Checker     *rbac.Checker
	SyncWorker  SyncRunner
	Config      any
}

// SyncRunner abstracts project sync execution.
type SyncRunner interface {
	Sync(ctx context.Context, projectID uuid.UUID) error
}

// NewRouter creates the HTTP router with all routes and middleware.
func NewRouter(deps Deps) http.Handler {
	// Public routes (no auth required)
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("GET /healthz", handleHealthz)
	publicMux.HandleFunc("GET /readyz", handleReadyz(deps))
	publicMux.Handle("GET /metrics", metrics.Handler())
	publicMux.HandleFunc("GET /api/v1/", handleAPIRoot)
	publicMux.HandleFunc("POST /api/v1/auth/token", deps.handleAuthToken)

	// Protected routes (auth required)
	protectedMux := http.NewServeMux()

	// Auth
	protectedMux.HandleFunc("GET /api/v1/auth/me", deps.handleAuthMe)

	// Organizations — create is platform-admin only
	protectedMux.HandleFunc("GET /api/v1/organizations", deps.handleListOrganizations)
	protectedMux.Handle("POST /api/v1/organizations",
		wrapHandler(deps.handleCreateOrganization, Require(deps.Checker, rbac.ActionManage, rbac.ResourceOrganization)))
	protectedMux.HandleFunc("GET /api/v1/organizations/{id}", deps.handleGetOrganization)

	// Teams
	protectedMux.HandleFunc("GET /api/v1/organizations/{orgId}/teams", deps.handleListTeams)
	protectedMux.HandleFunc("POST /api/v1/organizations/{orgId}/teams", deps.handleCreateTeam)
	protectedMux.HandleFunc("GET /api/v1/organizations/{orgId}/teams/{teamId}", deps.handleGetTeam)
	protectedMux.HandleFunc("DELETE /api/v1/organizations/{orgId}/teams/{teamId}", deps.handleDeleteTeam)
	protectedMux.HandleFunc("GET /api/v1/organizations/{orgId}/teams/{teamId}/members", deps.handleListTeamMembers)
	protectedMux.HandleFunc("POST /api/v1/organizations/{orgId}/teams/{teamId}/members", deps.handleAddTeamMember)
	protectedMux.HandleFunc("DELETE /api/v1/organizations/{orgId}/teams/{teamId}/members/{userId}", deps.handleRemoveTeamMember)

	// Users — create is platform-admin only
	protectedMux.HandleFunc("GET /api/v1/users", deps.handleListUsers)
	protectedMux.Handle("POST /api/v1/users",
		wrapHandler(deps.handleCreateUser, Require(deps.Checker, rbac.ActionManage, rbac.ResourceUser)))
	protectedMux.HandleFunc("GET /api/v1/users/{id}", deps.handleGetUser)

	// Role Assignments — manage is org-admin or platform-admin
	protectedMux.HandleFunc("GET /api/v1/roles", deps.handleListAvailableRoles)
	protectedMux.HandleFunc("GET /api/v1/organizations/{orgId}/role-assignments", deps.handleListRoleAssignments)
	protectedMux.Handle("POST /api/v1/organizations/{orgId}/role-assignments",
		wrapHandler(deps.handleCreateRoleAssignment, Require(deps.Checker, rbac.ActionManage, rbac.ResourceTeam)))
	protectedMux.Handle("DELETE /api/v1/organizations/{orgId}/role-assignments/{id}",
		wrapHandler(deps.handleDeleteRoleAssignment, Require(deps.Checker, rbac.ActionManage, rbac.ResourceTeam)))

	// Projects
	protectedMux.HandleFunc("GET /api/v1/projects", deps.handleListProjects)
	protectedMux.HandleFunc("POST /api/v1/projects", deps.handleCreateProject)
	protectedMux.HandleFunc("GET /api/v1/projects/{id}", deps.handleGetProject)
	protectedMux.HandleFunc("PATCH /api/v1/projects/{id}", deps.handleUpdateProject)
	protectedMux.HandleFunc("DELETE /api/v1/projects/{id}", deps.handleDeleteProject)
	protectedMux.HandleFunc("POST /api/v1/projects/{id}/sync", deps.handleSyncProject)

	// Credentials
	protectedMux.HandleFunc("GET /api/v1/credential-types", deps.handleListCredentialTypes)
	protectedMux.HandleFunc("GET /api/v1/credentials", deps.handleListCredentials)
	protectedMux.HandleFunc("POST /api/v1/credentials", deps.handleCreateCredential)
	protectedMux.HandleFunc("GET /api/v1/credentials/{id}", deps.handleGetCredential)
	protectedMux.HandleFunc("DELETE /api/v1/credentials/{id}", deps.handleDeleteCredential)

	// Inventories
	protectedMux.HandleFunc("GET /api/v1/inventories", deps.handleListInventories)
	protectedMux.HandleFunc("POST /api/v1/inventories", deps.handleCreateInventory)
	protectedMux.HandleFunc("GET /api/v1/inventories/{id}", deps.handleGetInventory)
	protectedMux.HandleFunc("DELETE /api/v1/inventories/{id}", deps.handleDeleteInventory)
	protectedMux.HandleFunc("GET /api/v1/inventories/{id}/hosts", deps.handleListInventoryHosts)
	protectedMux.HandleFunc("POST /api/v1/inventories/{id}/hosts", deps.handleCreateInventoryHost)
	protectedMux.HandleFunc("DELETE /api/v1/inventories/{id}/hosts/{hostId}", deps.handleDeleteInventoryHost)
	protectedMux.HandleFunc("GET /api/v1/inventories/{id}/groups", deps.handleListInventoryGroups)
	protectedMux.HandleFunc("POST /api/v1/inventories/{id}/groups", deps.handleCreateInventoryGroup)
	protectedMux.HandleFunc("DELETE /api/v1/inventories/{id}/groups/{groupId}", deps.handleDeleteInventoryGroup)
	protectedMux.HandleFunc("POST /api/v1/inventories/{id}/groups/{groupId}/hosts", deps.handleAddHostToGroup)
	protectedMux.HandleFunc("DELETE /api/v1/inventories/{id}/groups/{groupId}/hosts/{hostId}", deps.handleRemoveHostFromGroup)

	// Execution Environments
	protectedMux.HandleFunc("GET /api/v1/execution-environments", deps.handleListExecutionEnvironments)
	protectedMux.HandleFunc("POST /api/v1/execution-environments", deps.handleCreateExecutionEnvironment)
	protectedMux.HandleFunc("GET /api/v1/execution-environments/{id}", deps.handleGetExecutionEnvironment)
	protectedMux.HandleFunc("DELETE /api/v1/execution-environments/{id}", deps.handleDeleteExecutionEnvironment)

	// Job Templates
	protectedMux.HandleFunc("GET /api/v1/job-templates", deps.handleListJobTemplates)
	protectedMux.HandleFunc("POST /api/v1/job-templates", deps.handleCreateJobTemplate)
	protectedMux.HandleFunc("GET /api/v1/job-templates/{id}", deps.handleGetJobTemplate)
	protectedMux.HandleFunc("DELETE /api/v1/job-templates/{id}", deps.handleDeleteJobTemplate)
	protectedMux.HandleFunc("POST /api/v1/job-templates/{id}/launch", deps.handleLaunchJob)

	// Workflows (AWX-style workflow job templates)
	protectedMux.HandleFunc("GET /api/v1/workflow-templates", deps.handleListWorkflowTemplates)
	protectedMux.HandleFunc("POST /api/v1/workflow-templates", deps.handleCreateWorkflowTemplate)
	protectedMux.HandleFunc("GET /api/v1/workflow-templates/{id}", deps.handleGetWorkflowTemplate)
	protectedMux.HandleFunc("PUT /api/v1/workflow-templates/{id}", deps.handleUpdateWorkflowTemplate)
	protectedMux.HandleFunc("DELETE /api/v1/workflow-templates/{id}", deps.handleDeleteWorkflowTemplate)
	protectedMux.HandleFunc("POST /api/v1/workflow-templates/{id}/launch", deps.handleLaunchWorkflow)
	protectedMux.HandleFunc("GET /api/v1/workflow-jobs", deps.handleListWorkflowJobs)
	protectedMux.HandleFunc("GET /api/v1/workflow-jobs/{id}", deps.handleGetWorkflowJob)

	// Jobs
	protectedMux.HandleFunc("GET /api/v1/jobs", deps.handleListJobs)
	protectedMux.HandleFunc("GET /api/v1/jobs/{id}", deps.handleGetJob)
	protectedMux.HandleFunc("GET /api/v1/jobs/{id}/events", deps.handleGetJobEvents)
	protectedMux.HandleFunc("GET /api/v1/jobs/{id}/logs", deps.handleGetJobLogs)
	protectedMux.HandleFunc("POST /api/v1/jobs/{id}/cancel", deps.handleCancelJob)

	// Streaming (SSE)
	protectedMux.HandleFunc("GET /api/v1/jobs/{id}/stream", deps.handleJobStream)
	protectedMux.HandleFunc("GET /api/v1/jobs/{id}/logs/stream", deps.handleJobLogStream)

	// Create auth-protected handler
	protectedHandler := chain(protectedMux, authMiddleware(deps.Extractor))

	// Router that routes to public or protected based on path
	router := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/healthz" || path == "/readyz" || path == "/metrics" || path == "/api/v1/" || path == "/api/v1/auth/token" {
			publicMux.ServeHTTP(w, r)
		} else {
			protectedHandler.ServeHTTP(w, r)
		}
	})

	// Apply common middleware
	return chain(
		router,
		requestIDMiddleware,
		loggingMiddleware(deps.Logger),
		recoveryMiddleware(deps.Logger),
	)
}

// ─── Health Handlers ──────────────────────────────────────────────────────────

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleReadyz(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := deps.DB.Health(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  "database",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func handleAPIRoot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"name":    "AIWX API",
		"version": "v1",
		"status":  "ok",
	})
}
