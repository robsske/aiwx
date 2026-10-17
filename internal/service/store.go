package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/domain"
	"github.com/rverhe/aiwx/internal/outbox"
	"github.com/rverhe/aiwx/internal/repository"
)

// jobStore adapts *repository.JobRepo to JobRepository, binding the outbox
// so FinishJob can update status and enqueue an event in one transaction.
type jobStore struct {
	jobs   *repository.JobRepo
	outbox *outbox.Store
}

// NewJobStore wraps a JobRepo + outbox.Store for the Executor.
func NewJobStore(jobs *repository.JobRepo, ob *outbox.Store) JobRepository {
	return &jobStore{jobs: jobs, outbox: ob}
}

func (s *jobStore) ClaimPendingByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	return s.jobs.ClaimPendingByID(ctx, id)
}

func (s *jobStore) ClaimNextPending(ctx context.Context) (*domain.Job, error) {
	return s.jobs.ClaimNextPending(ctx)
}

func (s *jobStore) FinishJob(ctx context.Context, id uuid.UUID, status domain.JobStatus, reason string) error {
	return s.jobs.UpdateStatusAndEnqueue(ctx, id, status, reason, s.outbox)
}

func (s *jobStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	return s.jobs.GetByID(ctx, id)
}
