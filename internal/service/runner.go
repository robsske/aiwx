// Package service provides job execution for AIWX.
package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/domain"
)

// JobSpec carries the data needed to run a job.
type JobSpec struct {
	Playbook    string
	ExtraVars   map[string]any
	Inventory   string
	Credentials []uuid.UUID
	ProjectDir  string
}

// JobResult is the outcome of a job run.
type JobResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// JobRunner executes a job.
type JobRunner interface {
	Run(ctx context.Context, job *domain.Job, spec *JobSpec) (*JobResult, error)
}
