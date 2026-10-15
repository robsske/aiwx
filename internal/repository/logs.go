package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rverhe/aiwx/internal/domain"
)

// ─── Job Logs ─────────────────────────────────────────────────────────────────

type JobLogRepo struct {
	pool *Pool
}

func NewJobLogRepo(pool *Pool) *JobLogRepo {
	return &JobLogRepo{pool: pool}
}

// JobLogChunk represents a chunk of job log output.
type JobLogChunk struct {
	Sequence  int64     `json:"sequence"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	JobID     uuid.UUID `json:"job_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *JobLogRepo) Create(ctx context.Context, jobID uuid.UUID, content, source string) (*JobLogChunk, error) {
	chunk := &JobLogChunk{
		JobID:     jobID,
		Content:   content,
		Source:    source,
		CreatedAt: time.Now(),
	}

	// Get next sequence number
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(sequence), 0) + 1 FROM job_logs WHERE job_id = $1`, jobID,
	).Scan(&chunk.Sequence)
	if err != nil {
		return nil, fmt.Errorf("get next sequence: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO job_logs (job_id, sequence, content, source, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		chunk.JobID, chunk.Sequence, chunk.Content, chunk.Source, chunk.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert job log: %w", err)
	}

	return chunk, nil
}

func (r *JobLogRepo) ListSince(ctx context.Context, jobID uuid.UUID, sinceSequence int64, limit int) ([]JobLogChunk, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT job_id, sequence, content, source, created_at
		 FROM job_logs
		 WHERE job_id = $1 AND sequence > $2
		 ORDER BY sequence
		 LIMIT $3`, jobID, sinceSequence, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []JobLogChunk
	for rows.Next() {
		var chunk JobLogChunk
		if err := rows.Scan(&chunk.JobID, &chunk.Sequence, &chunk.Content,
			&chunk.Source, &chunk.CreatedAt); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

func (r *JobLogRepo) ListByJob(ctx context.Context, jobID uuid.UUID, limit int) ([]JobLogChunk, error) {
	return r.ListSince(ctx, jobID, 0, limit)
}

// ─── Job Events (extended) ────────────────────────────────────────────────────

func (r *JobEventRepo) ListByJobSince(ctx context.Context, jobID uuid.UUID, sinceID int64, limit int) ([]domain.JobEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, job_id, event_type, event_data, created_at
		 FROM job_events
		 WHERE job_id = $1 AND id > $2
		 ORDER BY id
		 LIMIT $3`, jobID, sinceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.JobEvent
	for rows.Next() {
		var event domain.JobEvent
		if err := rows.Scan(&event.ID, &event.JobID, &event.EventType,
			&event.EventData, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
