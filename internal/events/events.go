// Package events provides event types and the NATS JetStream client for AIWX.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// ─── Event Types ──────────────────────────────────────────────────────────────

// EventType represents a system event.
type EventType string

const (
	// Job lifecycle events
	JobCreated    EventType = "job.created"
	JobPending    EventType = "job.pending"
	JobWaiting    EventType = "job.waiting"
	JobRunning    EventType = "job.running"
	JobProgress   EventType = "job.progress"
	JobSuccessful EventType = "job.successful"
	JobFailed     EventType = "job.failed"
	JobError      EventType = "job.error"
	JobCanceled   EventType = "job.canceled"

	// Project events
	ProjectSyncStarted   EventType = "project.sync.started"
	ProjectSyncCompleted EventType = "project.sync.completed"
	ProjectSyncFailed    EventType = "project.sync.failed"

	// Credential events
	CredentialAccessDenied EventType = "credential.access.denied"
)

// Event is the single wire envelope for everything published on the bus.
type Event struct {
	Type           EventType      `json:"type"`
	AggregateType  string         `json:"aggregate_type"`
	AggregateID    uuid.UUID      `json:"aggregate_id"`
	OrganizationID uuid.UUID      `json:"organization_id,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
	IdempotencyKey string         `json:"idempotency_key"`
	OutboxID       int64          `json:"id,omitempty"`
}

// ─── NATS Client ──────────────────────────────────────────────────────────────

// Client wraps NATS JetStream for event publishing and consumption.
type Client struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

// Config for NATS connection.
type Config struct {
	URL     string
	Cluster string
}

// NewClient creates a new NATS client and ensures required streams exist.
func NewClient(cfg Config) (*Client, error) {
	url := cfg.URL
	if url == "" {
		url = nats.DefaultURL
	}
	nc, err := nats.Connect(url,
		nats.Name("aiwx-control-plane"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	client := &Client{nc: nc, js: js}
	if err := client.ensureStreams(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure streams: %w", err)
	}
	return client, nil
}

// ensureStreams creates required JetStream streams.
func (c *Client) ensureStreams() error {
	streams := []struct {
		name      string
		subjects  []string
		retention nats.RetentionPolicy
		maxAge    time.Duration
	}{
		{
			name:      "AIWX_JOB_EVENTS",
			subjects:  []string{"aiwx.job.>"},
			retention: nats.LimitsPolicy,
			maxAge:    24 * time.Hour,
		},
		{
			name:      "AIWX_PROJECT_EVENTS",
			subjects:  []string{"aiwx.project.>"},
			retention: nats.LimitsPolicy,
			maxAge:    24 * time.Hour,
		},
		{
			name:      "AIWX_SYSTEM_EVENTS",
			subjects:  []string{"aiwx.system.>"},
			retention: nats.LimitsPolicy,
			maxAge:    24 * time.Hour,
		},
	}

	for _, s := range streams {
		_, err := c.js.StreamInfo(s.name)
		if err == nats.ErrStreamNotFound {
			_, err = c.js.AddStream(&nats.StreamConfig{
				Name:      s.name,
				Subjects:  s.subjects,
				Retention: s.retention,
				MaxAge:    s.maxAge,
				Storage:   nats.FileStorage,
			})
			if err != nil {
				return fmt.Errorf("add stream %s: %w", s.name, err)
			}
		} else if err != nil {
			return fmt.Errorf("stream info %s: %w", s.name, err)
		}
	}
	return nil
}

// Publish publishes an event to the appropriate stream.
func (c *Client) Publish(ctx context.Context, event Event) error {
	subject := SubjectForEvent(event.Type)
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	_, err = c.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	return nil
}

// PublishRaw publishes raw data to a subject (used by the outbox relay).
func (c *Client) PublishRaw(ctx context.Context, subject string, data []byte) error {
	_, err := c.js.Publish(subject, data)
	return err
}

// EventHandler processes a decoded event. Return nil to Ack; non-nil Nak/Term handled by Consume.
type EventHandler func(ctx context.Context, event *Event) error

// Consume runs a durable pull consumer over a subject until ctx is canceled.
// Ack happens only after the handler returns nil. Poison messages (bad JSON) are Terminated.
func (c *Client) Consume(ctx context.Context, stream, subject, durable string, handler EventHandler, opts ...ConsumeOpt) error {
	cfg := consumeConfig{
		batch:    1,
		maxWait:  2 * time.Second,
		ackWait:  35 * time.Minute,
		consumer: durable,
	}
	for _, o := range opts {
		o(&cfg)
	}

	sub, err := c.js.PullSubscribe(subject, durable,
		nats.BindStream(stream),
		nats.ManualAck(),
		nats.AckWait(cfg.ackWait),
		nats.DeliverAll(),
	)
	if err != nil {
		return fmt.Errorf("pull subscribe %s/%s: %w", stream, durable, err)
	}
	defer sub.Unsubscribe()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msgs, err := sub.Fetch(cfg.batch, nats.MaxWait(cfg.maxWait))
		if err != nil {
			if err == nats.ErrTimeout || err == context.DeadlineExceeded {
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Transient consumer error — back off briefly
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
			continue
		}
		for _, msg := range msgs {
			var event Event
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				// Poison message — do not redeliver forever.
				_ = msg.Term()
				continue
			}
			if err := handler(ctx, &event); err != nil {
				_ = msg.Nak()
				continue
			}
			_ = msg.Ack()
		}
	}
}

type consumeConfig struct {
	batch    int
	maxWait  time.Duration
	ackWait  time.Duration
	consumer string
}

// ConsumeOpt customizes a durable consumer.
type ConsumeOpt func(*consumeConfig)

// WithBatch sets the pull batch size.
func WithBatch(n int) ConsumeOpt {
	return func(c *consumeConfig) {
		if n > 0 {
			c.batch = n
		}
	}
}

// WithAckWait sets the JetStream AckWait.
func WithAckWait(d time.Duration) ConsumeOpt {
	return func(c *consumeConfig) {
		if d > 0 {
			c.ackWait = d
		}
	}
}

// Close closes the NATS connection.
func (c *Client) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}

// IsConnected returns true if NATS is connected.
func (c *Client) IsConnected() bool {
	return c.nc != nil && c.nc.IsConnected()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// SubjectForEvent maps event types to NATS subjects.
//
//	job.*           → aiwx.job.<rest>        (e.g. job.created → aiwx.job.created)
//	project.*       → aiwx.project.<rest>    (e.g. project.sync.completed → aiwx.project.sync.completed)
//	credential.*    → aiwx.system.<rest>
//	other           → aiwx.system.unknown
func SubjectForEvent(t EventType) string {
	s := string(t)
	switch {
	case strings.HasPrefix(s, "job."):
		return "aiwx.job." + strings.TrimPrefix(s, "job.")
	case strings.HasPrefix(s, "project."):
		return "aiwx.project." + strings.TrimPrefix(s, "project.")
	case strings.HasPrefix(s, "credential.") || strings.HasPrefix(s, "cred."):
		rest := strings.TrimPrefix(s, "credential.")
		rest = strings.TrimPrefix(rest, "cred.")
		return "aiwx.system." + rest
	default:
		return "aiwx.system.unknown"
	}
}

// ─── Outbox Publisher Adapter ─────────────────────────────────────────────────

// OutboxEvent is the outbox row shape published onto the bus.
type OutboxEvent struct {
	ID             int64
	EventType      string
	AggregateType  string
	AggregateID    uuid.UUID
	Payload        map[string]any
	IdempotencyKey string
	CreatedAt      time.Time
}

// NATSPublisher adapts the NATS client to the outbox.Publisher interface.
type NATSPublisher struct {
	client *Client
}

// NewNATSPublisher creates a new NATS publisher adapter.
func NewNATSPublisher(client *Client) *NATSPublisher {
	return &NATSPublisher{client: client}
}

// Publish publishes an outbox event to NATS using the unified Event envelope.
func (p *NATSPublisher) Publish(ctx context.Context, event OutboxEvent) error {
	orgID := uuid.Nil
	if event.Payload != nil {
		if s, ok := event.Payload["organization_id"].(string); ok {
			if parsed, err := uuid.Parse(s); err == nil {
				orgID = parsed
			}
		}
	}
	ts := event.CreatedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	envelope := Event{
		Type:           EventType(event.EventType),
		AggregateType:  event.AggregateType,
		AggregateID:    event.AggregateID,
		OrganizationID: orgID,
		Payload:        event.Payload,
		Timestamp:      ts,
		IdempotencyKey: event.IdempotencyKey,
		OutboxID:       event.ID,
	}
	return p.client.Publish(ctx, envelope)
}
