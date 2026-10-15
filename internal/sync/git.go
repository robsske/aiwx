// Package sync implements project synchronization from Git repositories.
package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// Status represents a project sync status.
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusCanceled Status = "canceled"
)

// Project represents a project to sync.
type Project struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	Name            string
	SCMURL          string
	SCMBranch       string
	SCMCredentialID *uuid.UUID
	LatestRevision  *string
	SyncStatus      Status
}

// SyncResult contains the outcome of a project sync.
type SyncResult struct {
	ProjectID     uuid.UUID
	CommitSHA     string
	ContentDigest string
	ArtifactPath  string
	Revision      string
	Status        Status
	Error         string
	StartedAt     time.Time
	FinishedAt    time.Time
}

// ─── Repository Interface ─────────────────────────────────────────────────────

// ProjectRepo is the interface for project operations.
type ProjectRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Project, error)
	UpdateSyncStatus(ctx context.Context, id uuid.UUID, status Status, revision, artifactPath, errMsg string) error
}

// ─── Git Sync Worker ──────────────────────────────────────────────────────────

// Worker synchronizes projects from Git repositories.
type Worker struct {
	repo      ProjectRepo
	logger    *slog.Logger
	workDir   string
	artifactsDir string
}

// NewWorker creates a new Git sync worker.
func NewWorker(repo ProjectRepo, logger *slog.Logger, workDir, artifactsDir string) *Worker {
	return &Worker{
		repo:         repo,
		logger:       logger,
		workDir:      workDir,
		artifactsDir: artifactsDir,
	}
}

// Sync performs a full project sync.
func (w *Worker) Sync(ctx context.Context, projectID uuid.UUID) (*SyncResult, error) {
	result := &SyncResult{
		ProjectID: projectID,
		StartedAt: time.Now(),
	}

	// Get project
	project, err := w.repo.GetByID(ctx, projectID)
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("get project: %v", err)
		return result, err
	}

	// Update status to running
	if err := w.repo.UpdateSyncStatus(ctx, projectID, StatusRunning, "", "", ""); err != nil {
		w.logger.Error("failed to update sync status", "error", err)
	}

	// Perform sync
	defer func() {
		result.FinishedAt = time.Now()
		if result.Error != "" {
			result.Status = StatusFailed
		} else {
			result.Status = StatusSuccess
		}
		w.repo.UpdateSyncStatus(ctx, projectID, result.Status, result.Revision, result.ArtifactPath, result.Error)
	}()

	// Create working directory
	workPath := filepath.Join(w.workDir, projectID.String())
	if err := os.MkdirAll(workPath, 0755); err != nil {
		result.Error = fmt.Sprintf("create work dir: %v", err)
		return result, err
	}
	defer os.RemoveAll(workPath)

	// Clone or fetch
	if err := w.cloneOrFetch(ctx, project, workPath); err != nil {
		result.Error = fmt.Sprintf("git operation: %v", err)
		return result, err
	}

	// Get commit SHA
	commitSHA, err := w.getCommitSHA(ctx, workPath)
	if err != nil {
		result.Error = fmt.Sprintf("get commit SHA: %v", err)
		return result, err
	}
	result.CommitSHA = commitSHA
	result.Revision = commitSHA

	// Create immutable artifact
	artifactPath, contentDigest, err := w.createArtifact(ctx, project, workPath, commitSHA)
	if err != nil {
		result.Error = fmt.Sprintf("create artifact: %v", err)
		return result, err
	}
	result.ArtifactPath = artifactPath
	result.ContentDigest = contentDigest

	w.logger.Info("project synced",
		"project_id", projectID,
		"commit", commitSHA,
		"digest", contentDigest,
	)

	return result, nil
}

// cloneOrFetch clones or updates the repository.
func (w *Worker) cloneOrFetch(ctx context.Context, project *Project, workPath string) error {
	gitDir := filepath.Join(workPath, ".git")

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Clone
		args := []string{"clone", "--branch", project.SCMBranch, "--depth", "1", project.SCMURL, workPath}
		if err := w.runGit(ctx, "", args...); err != nil {
			return fmt.Errorf("git clone: %w", err)
		}
	} else {
		// Fetch and reset
		if err := w.runGit(ctx, workPath, "fetch", "origin"); err != nil {
			return fmt.Errorf("git fetch: %w", err)
		}
		if err := w.runGit(ctx, workPath, "reset", "--hard", "origin/"+project.SCMBranch); err != nil {
			return fmt.Errorf("git reset: %w", err)
		}
	}

	return nil
}

// getCommitSHA returns the current HEAD commit SHA.
func (w *Worker) getCommitSHA(ctx context.Context, workPath string) (string, error) {
	out, err := w.runGitOutput(ctx, workPath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// createArtifact creates an immutable tar.gz artifact of the project.
func (w *Worker) createArtifact(ctx context.Context, project *Project, workPath, commitSHA string) (string, string, error) {
	// Create artifact directory
	artifactDir := filepath.Join(w.artifactsDir, project.OrganizationID.String(), project.ID.String())
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return "", "", err
	}

	// Create tar.gz
	artifactPath := filepath.Join(artifactDir, commitSHA+".tar.gz")
	if err := w.createTarGz(ctx, workPath, artifactPath); err != nil {
		return "", "", err
	}

	// Calculate content digest
	digest, err := calculateDigest(artifactPath)
	if err != nil {
		return "", "", err
	}

	return artifactPath, digest, nil
}

// createTarGz creates a compressed archive of the project directory.
func (w *Worker) createTarGz(ctx context.Context, sourceDir, targetPath string) error {
	cmd := exec.CommandContext(ctx, "tar", "-czf", targetPath, "-C", sourceDir, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar: %w: %s", err, out)
	}
	return nil
}

// calculateDigest calculates the SHA-256 digest of a file.
func calculateDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// runGit executes a git command.
func (w *Worker) runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return nil
}

// runGitOutput executes a git command and returns its output.
func (w *Worker) runGitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// ─── Scheduler ────────────────────────────────────────────────────────────────

// Scheduler schedules periodic project syncs.
type Scheduler struct {
	worker   *Worker
	interval time.Duration
	logger   *slog.Logger
}

// NewScheduler creates a new sync scheduler.
func NewScheduler(worker *Worker, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		worker:   worker,
		interval: interval,
		logger:   logger,
	}
}

// Run starts the scheduler loop. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// TODO: Query projects with scheduled sync enabled
			s.logger.Debug("sync scheduler tick")
		}
	}
}

// TriggerSync triggers a sync for a specific project.
func (s *Scheduler) TriggerSync(ctx context.Context, projectID uuid.UUID) (*SyncResult, error) {
	return s.worker.Sync(ctx, projectID)
}
