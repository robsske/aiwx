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
	Encryptor   CredentialCryptor
	AuthLimiter HTTPMiddleware
}

// CredentialCryptor encrypts/decrypts credential secrets at rest.
type CredentialCryptor interface {
	Encrypt(plaintext []byte) (string, error)
	Decrypt(encoded string) ([]byte, error)
}

// HTTPMiddleware is a standard middleware function.
type HTTPMiddleware func(http.Handler) http.Handler

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
	// Rate-limit login to mitigate brute force
	authHandler := http.HandlerFunc(deps.handleAuthToken)
	if deps.AuthLimiter != nil {
		publicMux.Handle("POST /api/v1/auth/token", deps.AuthLimiter(authHandler))
	} else {
		publicMux.Handle("POST /api/v1/auth/token", authHandler)
	}

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
	protectedMux.Handle("POST /api/v1/organizations/{orgId}/teams",
		wrapHandler(deps.handleCreateTeam, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceTeam)))
	protectedMux.HandleFunc("GET /api/v1/organizations/{orgId}/teams/{teamId}", deps.handleGetTeam)
	protectedMux.Handle("DELETE /api/v1/organizations/{orgId}/teams/{teamId}",
		wrapHandler(deps.handleDeleteTeam, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceTeam)))
	protectedMux.HandleFunc("GET /api/v1/organizations/{orgId}/teams/{teamId}/members", deps.handleListTeamMembers)
	protectedMux.Handle("POST /api/v1/organizations/{orgId}/teams/{teamId}/members",
		wrapHandler(deps.handleAddTeamMember, Require(deps.Checker, rbac.ActionManage, rbac.ResourceTeam)))
	protectedMux.Handle("DELETE /api/v1/organizations/{orgId}/teams/{teamId}/members/{userId}",
		wrapHandler(deps.handleRemoveTeamMember, Require(deps.Checker, rbac.ActionManage, rbac.ResourceTeam)))

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
	protectedMux.Handle("POST /api/v1/projects",
		wrapHandler(deps.handleCreateProject, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceProject)))
	protectedMux.HandleFunc("GET /api/v1/projects/{id}", deps.handleGetProject)
	protectedMux.Handle("PATCH /api/v1/projects/{id}",
		wrapHandler(deps.handleUpdateProject, Require(deps.Checker, rbac.ActionUpdate, rbac.ResourceProject)))
	protectedMux.Handle("DELETE /api/v1/projects/{id}",
		wrapHandler(deps.handleDeleteProject, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceProject)))
	protectedMux.Handle("POST /api/v1/projects/{id}/sync",
		wrapHandler(deps.handleSyncProject, Require(deps.Checker, rbac.ActionExecute, rbac.ResourceProject)))

	// Credentials
	protectedMux.HandleFunc("GET /api/v1/credential-types", deps.handleListCredentialTypes)
	protectedMux.Handle("POST /api/v1/credential-types",
		wrapHandler(deps.handleCreateCredentialType, Require(deps.Checker, rbac.ActionManage, rbac.ResourceCredential)))
	protectedMux.Handle("PUT /api/v1/credential-types/{id}",
		wrapHandler(deps.handleUpdateCredentialType, Require(deps.Checker, rbac.ActionManage, rbac.ResourceCredential)))
	protectedMux.HandleFunc("GET /api/v1/credentials", deps.handleListCredentials)
	protectedMux.Handle("POST /api/v1/credentials",
		wrapHandler(deps.handleCreateCredential, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceCredential)))
	protectedMux.HandleFunc("GET /api/v1/credentials/{id}", deps.handleGetCredential)
	protectedMux.Handle("DELETE /api/v1/credentials/{id}",
		wrapHandler(deps.handleDeleteCredential, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceCredential)))

	// Inventories
	protectedMux.HandleFunc("GET /api/v1/inventories", deps.handleListInventories)
	protectedMux.Handle("POST /api/v1/inventories",
		wrapHandler(deps.handleCreateInventory, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceInventory)))
	protectedMux.HandleFunc("GET /api/v1/inventories/{id}", deps.handleGetInventory)
	protectedMux.Handle("DELETE /api/v1/inventories/{id}",
		wrapHandler(deps.handleDeleteInventory, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceInventory)))
	protectedMux.HandleFunc("GET /api/v1/inventories/{id}/hosts", deps.handleListInventoryHosts)
	protectedMux.Handle("POST /api/v1/inventories/{id}/hosts",
		wrapHandler(deps.handleCreateInventoryHost, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceInventory)))
	protectedMux.Handle("DELETE /api/v1/inventories/{id}/hosts/{hostId}",
		wrapHandler(deps.handleDeleteInventoryHost, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceInventory)))
	protectedMux.HandleFunc("GET /api/v1/inventories/{id}/groups", deps.handleListInventoryGroups)
	protectedMux.Handle("POST /api/v1/inventories/{id}/groups",
		wrapHandler(deps.handleCreateInventoryGroup, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceInventory)))
	protectedMux.Handle("DELETE /api/v1/inventories/{id}/groups/{groupId}",
		wrapHandler(deps.handleDeleteInventoryGroup, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceInventory)))
	protectedMux.Handle("POST /api/v1/inventories/{id}/groups/{groupId}/hosts",
		wrapHandler(deps.handleAddHostToGroup, Require(deps.Checker, rbac.ActionUpdate, rbac.ResourceInventory)))
	protectedMux.Handle("DELETE /api/v1/inventories/{id}/groups/{groupId}/hosts/{hostId}",
		wrapHandler(deps.handleRemoveHostFromGroup, Require(deps.Checker, rbac.ActionUpdate, rbac.ResourceInventory)))

	// Execution Environments
	protectedMux.HandleFunc("GET /api/v1/execution-environments", deps.handleListExecutionEnvironments)
	protectedMux.Handle("POST /api/v1/execution-environments",
		wrapHandler(deps.handleCreateExecutionEnvironment, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceExecutionEnvironment)))
	protectedMux.HandleFunc("GET /api/v1/execution-environments/{id}", deps.handleGetExecutionEnvironment)
	protectedMux.Handle("DELETE /api/v1/execution-environments/{id}",
		wrapHandler(deps.handleDeleteExecutionEnvironment, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceExecutionEnvironment)))

	// Job Templates
	protectedMux.HandleFunc("GET /api/v1/job-templates", deps.handleListJobTemplates)
	protectedMux.Handle("POST /api/v1/job-templates",
		wrapHandler(deps.handleCreateJobTemplate, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceJobTemplate)))
	protectedMux.HandleFunc("GET /api/v1/job-templates/{id}", deps.handleGetJobTemplate)
	protectedMux.Handle("DELETE /api/v1/job-templates/{id}",
		wrapHandler(deps.handleDeleteJobTemplate, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceJobTemplate)))
	protectedMux.Handle("POST /api/v1/job-templates/{id}/launch",
		wrapHandler(deps.handleLaunchJob, Require(deps.Checker, rbac.ActionExecute, rbac.ResourceJob)))

	// Workflows (AWX-style workflow job templates)
	protectedMux.HandleFunc("GET /api/v1/workflow-templates", deps.handleListWorkflowTemplates)
	protectedMux.Handle("POST /api/v1/workflow-templates",
		wrapHandler(deps.handleCreateWorkflowTemplate, Require(deps.Checker, rbac.ActionCreate, rbac.ResourceJobTemplate)))
	protectedMux.HandleFunc("GET /api/v1/workflow-templates/{id}", deps.handleGetWorkflowTemplate)
	protectedMux.Handle("PUT /api/v1/workflow-templates/{id}",
		wrapHandler(deps.handleUpdateWorkflowTemplate, Require(deps.Checker, rbac.ActionUpdate, rbac.ResourceJobTemplate)))
	protectedMux.Handle("DELETE /api/v1/workflow-templates/{id}",
		wrapHandler(deps.handleDeleteWorkflowTemplate, Require(deps.Checker, rbac.ActionDelete, rbac.ResourceJobTemplate)))
	protectedMux.Handle("POST /api/v1/workflow-templates/{id}/launch",
		wrapHandler(deps.handleLaunchWorkflow, Require(deps.Checker, rbac.ActionExecute, rbac.ResourceJob)))
	protectedMux.HandleFunc("GET /api/v1/workflow-jobs", deps.handleListWorkflowJobs)
	protectedMux.HandleFunc("GET /api/v1/workflow-jobs/{id}", deps.handleGetWorkflowJob)

	// Jobs
	protectedMux.HandleFunc("GET /api/v1/jobs", deps.handleListJobs)
	protectedMux.HandleFunc("GET /api/v1/jobs/{id}", deps.handleGetJob)
	protectedMux.HandleFunc("GET /api/v1/jobs/{id}/events", deps.handleGetJobEvents)
	protectedMux.HandleFunc("GET /api/v1/jobs/{id}/logs", deps.handleGetJobLogs)
	protectedMux.Handle("POST /api/v1/jobs/{id}/cancel",
		wrapHandler(deps.handleCancelJob, Require(deps.Checker, rbac.ActionExecute, rbac.ResourceJob)))

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
