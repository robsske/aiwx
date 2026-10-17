package workflow

import (
	"context"
	"log/slog"
	"time"

	"github.com/rverhe/aiwx/internal/events"
	"github.com/rverhe/aiwx/internal/metrics"
)

// EventConsumer advances workflows from NATS job lifecycle events.
type EventConsumer struct {
	client *events.Client
	runner *Runner
	logger *slog.Logger
}

// NewEventConsumer creates a durable consumer for job terminal events.
func NewEventConsumer(client *events.Client, runner *Runner, logger *slog.Logger) *EventConsumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventConsumer{client: client, runner: runner, logger: logger}
}

// Run blocks until ctx is canceled.
func (c *EventConsumer) Run(ctx context.Context) error {
	return c.client.Consume(ctx,
		"AIWX_JOB_EVENTS",
		"aiwx.job.>",
		"aiwx-workflow",
		c.handle,
		events.WithBatch(10),
		events.WithAckWait(30*time.Second),
	)
}

func (c *EventConsumer) handle(ctx context.Context, ev *events.Event) error {
	metrics.NATSReceivedTotal.Inc()
	if err := c.runner.OnJobEvent(ctx, ev); err != nil {
		c.logger.Error("workflow event handling failed",
			"event_type", ev.Type,
			"aggregate_id", ev.AggregateID,
			"error", err,
		)
		return err
	}
	return nil
}
