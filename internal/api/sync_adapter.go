// Package api provides adapters for external services.
package api

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/repository"
	"github.com/rverhe/aiwx/internal/sync"
)

// syncProjectRepo adapts repository.ProjectRepo to sync.ProjectRepo.
type syncProjectRepo struct {
	repo *repository.ProjectRepo
}

func (a *syncProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (*sync.Project, error) {
	p, err := a.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &sync.Project{
		ID:              p.ID,
		OrganizationID:  p.OrganizationID,
		Name:            p.Name,
		SCMURL:          p.SCMURL,
		SCMBranch:       p.SCMBranch,
		SCMCredentialID: p.SCMCredentialID,
		LatestRevision:  p.LatestRevision,
		SyncStatus:      sync.Status(p.SyncStatus),
	}, nil
}

func (a *syncProjectRepo) UpdateSyncStatus(ctx context.Context, id uuid.UUID, status sync.Status, revision, artifactPath, errMsg string) error {
	return a.repo.UpdateSyncStatus(ctx, id, string(status), revision, artifactPath, errMsg)
}

// GitSyncAdapter wraps sync.Worker to implement SyncRunner.
type GitSyncAdapter struct {
	worker *sync.Worker
}

// NewGitSyncAdapter creates a sync adapter from a repository.
func NewGitSyncAdapter(repo *repository.ProjectRepo, logger *slog.Logger) *GitSyncAdapter {
	adapter := &syncProjectRepo{repo: repo}
	worker := sync.NewWorker(adapter, logger, "/tmp/aiwx-sync", "/tmp/aiwx-artifacts")
	return &GitSyncAdapter{worker: worker}
}

// Sync runs a project sync.
func (a *GitSyncAdapter) Sync(ctx context.Context, projectID uuid.UUID) error {
	_, err := a.worker.Sync(ctx, projectID)
	return err
}
