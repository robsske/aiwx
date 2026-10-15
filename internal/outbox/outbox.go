// Package outbox implements the transactional outbox pattern for reliable
// event publishing. Events are written to PostgreSQL in the same transaction
// as the state change, then relayed to NATS by a background publisher.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event represents an event to be published.
type Event struct {
	ID             int64
	EventType      string
	AggregateType  string
	AggregateID    uuid.UUID
	Payload        map[string]any
	IdempotencyKey string
	CreatedAt      time.Time
	PublishedAt    *time.Time
	RetryCount     int
	ErrorMessage   *string
}

// Store handles outbox operations.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new outbox store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Enqueue writes an event to the outbox outside a transaction.
func (s *Store) Enqueue(ctx context.Context, eventType, aggregateType string, aggregateID uuid.UUID, payload map[string]any, idempotencyKey string) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO outbox (event_type, aggregate_type, aggregate_id, payload, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5)`,
		eventType, aggregateType, aggregateID, payloadJSON, idempotencyKey,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

// EnqueueTx writes an event to the outbox within the given transaction.
// This MUST be called in the same transaction as the state change.
func (s *Store) EnqueueTx(tx pgx.Tx, ctx context.Context, eventType, aggregateType string, aggregateID uuid.UUID, payload map[string]any, idempotencyKey string) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO outbox (event_type, aggregate_type, aggregate_id, payload, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5)`,
		eventType, aggregateType, aggregateID, payloadJSON, idempotencyKey,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

// Insert satisfies the repository.OutboxInserter interface for transactional writes.
func (s *Store) Insert(ctx context.Context, tx pgx.Tx, eventType, aggregateType string, aggregateID uuid.UUID, payload map[string]any, idempotencyKey string) error {
	return s.EnqueueTx(tx, ctx, eventType, aggregateType, aggregateID, payload, idempotencyKey)
}

// FetchUnpublished retrieves unpublished events up to the limit.
func (s *Store) FetchUnpublished(ctx context.Context, limit int) ([]Event, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, event_type, aggregate_type, aggregate_id, payload, idempotency_key,
		        created_at, published_at, retry_count, error_message
		 FROM outbox
		 WHERE published_at IS NULL
		 ORDER BY id
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch unpublished: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var payloadJSON []byte
		if err := rows.Scan(&e.ID, &e.EventType, &e.AggregateType, &e.AggregateID,
			&payloadJSON, &e.IdempotencyKey, &e.CreatedAt, &e.PublishedAt,
			&e.RetryCount, &e.ErrorMessage); err != nil {
			return nil, fmt.Errorf("scan outbox row: %w", err)
		}
		if err := json.Unmarshal(payloadJSON, &e.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

// MarkPublished marks an event as published.
func (s *Store) MarkPublished(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE outbox SET published_at = now(), error_message = NULL WHERE id = $1`, id,
	)
	return err
}

// MarkFailed records a publish failure and increments the retry count.
func (s *Store) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE outbox SET retry_count = retry_count + 1, error_message = $2 WHERE id = $1`,
		id, errMsg,
	)
	return err
}

// Cleanup removes published events older than the retention period.
func (s *Store) Cleanup(ctx context.Context, retention time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM outbox WHERE published_at IS NOT NULL AND published_at < now() - $1::interval`,
		fmt.Sprintf("%d hours", int(retention.Hours())),
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup outbox: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CountUnpublished returns the number of unpublished events.
func (s *Store) CountUnpublished(ctx context.Context) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox WHERE published_at IS NULL`,
	).Scan(&count)
	return count, err
}

// ─── Event Types ──────────────────────────────────────────────────────────────

// Standard event types for the outbox.
const (
	EventTypeJobCreated       = "job.created"
	EventTypeJobPending       = "job.pending"
	EventTypeJobWaiting       = "job.waiting"
	EventTypeJobRunning       = "job.running"
	EventTypeJobSuccessful    = "job.successful"
	EventTypeJobFailed        = "job.failed"
	EventTypeJobError         = "job.error"
	EventTypeJobCanceled      = "job.canceled"
	EventTypeProjectSync      = "project.sync"
	EventTypeProjectSyncDone  = "project.sync.completed"
	EventTypeProjectSyncFail  = "project.sync.failed"
)

// ─── Publisher (relay to NATS) ────────────────────────────────────────────────

// Publisher publishes events to a message bus.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

// Relay relays outbox events to the message bus.
type Relay struct {
	store     *Store
	publisher Publisher
	batchSize int
	interval  time.Duration
}

// NewRelay creates a new outbox relay.
func NewRelay(store *Store, publisher Publisher, batchSize int, interval time.Duration) *Relay {
	return &Relay{
		store:     store,
		publisher: publisher,
		batchSize: batchSize,
		interval:  interval,
	}
}

// Run starts the relay loop. Blocks until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.processBatch(ctx); err != nil {
				// Log but continue - we'll retry on next tick
				fmt.Printf("outbox relay error: %v\n", err)
			}
		}
	}
}

// ProcessOnce processes a single batch (useful for testing).
func (r *Relay) ProcessOnce(ctx context.Context) error {
	return r.processBatch(ctx)
}

func (r *Relay) processBatch(ctx context.Context) error {
	events, err := r.store.FetchUnpublished(ctx, r.batchSize)
	if err != nil {
		return fmt.Errorf("fetch unpublished: %w", err)
	}

	for _, event := range events {
		if err := r.publisher.Publish(ctx, event); err != nil {
			// Mark as failed and continue with next event
			if markErr := r.store.MarkFailed(ctx, event.ID, err.Error()); markErr != nil {
				return fmt.Errorf("mark failed (publish: %v, mark: %w)", err, markErr)
			}
			continue
		}

		if err := r.store.MarkPublished(ctx, event.ID); err != nil {
			return fmt.Errorf("mark published: %w", err)
		}
	}

	return nil
}
