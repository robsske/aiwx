// Package dispatch consumes job launch events from NATS and executes jobs.
package dispatch

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/events"
	"github.com/rverhe/aiwx/internal/metrics"
	"github.com/rverhe/aiwx/internal/service"
)

// Consumer is a durable JetStream consumer that executes jobs on job.created.
type Consumer struct {
	client *events.Client
	exec   *service.Executor
	logger *slog.Logger
}

// NewConsumer creates a NATS-backed job dispatcher.
func NewConsumer(client *events.Client, exec *service.Executor, logger *slog.Logger) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{client: client, exec: exec, logger: logger}
}

// Run blocks until ctx is canceled, consuming aiwx.job.created.
// AckWait is long enough for a full playbook run.
func (c *Consumer) Run(ctx context.Context) error {
	return c.client.Consume(ctx,
		"AIWX_JOB_EVENTS",
		"aiwx.job.created",
		"aiwx-dispatcher",
		c.handle,
		events.WithBatch(1),
		events.WithAckWait(35*time.Minute),
	)
}

func (c *Consumer) handle(ctx context.Context, ev *events.Event) error {
	jobID := ev.AggregateID
	if jobID == uuid.Nil {
		if s, ok := ev.Payload["job_id"].(string); ok {
			parsed, err := uuid.Parse(s)
			if err != nil {
				return nil // unparseable job id — ack and drop
			}
			jobID = parsed
		}
	}
	if jobID == uuid.Nil {
		c.logger.Warn("job.created event without job id", "idempotency_key", ev.IdempotencyKey)
		return nil
	}

	metrics.NATSReceivedTotal.Inc()
	c.logger.Info("dispatch received job.created",
		"job_id", jobID,
		"idempotency_key", ev.IdempotencyKey,
	)

	started := time.Now()
	ok, err := c.exec.ExecuteByID(ctx, jobID)
	if err != nil {
		c.logger.Error("dispatch execute failed", "job_id", jobID, "error", err)
		return err // Nak → redeliver
	}
	if ok {
		c.logger.Info("dispatch executed job", "job_id", jobID, "duration", time.Since(started))
	}
	return nil
}
