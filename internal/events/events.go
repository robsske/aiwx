// Package events provides event types and NATS client for AIWX.
package events

import (
	"context"
	"encoding/json"
	"fmt"
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

// Event represents a system event.
type Event struct {
	Type           EventType      `json:"type"`
	AggregateType  string         `json:"aggregate_type"`
	AggregateID    uuid.UUID      `json:"aggregate_id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	Payload        map[string]any `json:"payload"`
	Timestamp      time.Time      `json:"timestamp"`
	IdempotencyKey string         `json:"idempotency_key"`
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

// NewClient creates a new NATS client.
func NewClient(cfg Config) (*Client, error) {
	nc, err := nats.Connect(cfg.URL,
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

	// Ensure streams exist
	if err := client.ensureStreams(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure streams: %w", err)
	}

	return client, nil
}

// ensureStreams creates required JetStream streams.
func (c *Client) ensureStreams() error {
	streams := []struct {
		name     string
		subjects []string
		retention nats.RetentionPolicy
		maxAge   time.Duration
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
	subject := subjectForEvent(event.Type)
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

// PublishRaw publishes raw data to a subject (used by outbox relay).
func (c *Client) PublishRaw(ctx context.Context, subject string, data []byte) error {
	_, err := c.js.Publish(subject, data)
	return err
}

// Subscribe creates a subscription to a subject with a handler.
func (c *Client) Subscribe(subject string, handler func(*Event)) (*nats.Subscription, error) {
	sub, err := c.js.Subscribe(subject, func(msg *nats.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			msg.Nak()
			return
		}
		handler(&event)
		msg.Ack()
	}, nats.Durable("aiwx-consumer"), nats.ManualAck())
	if err != nil {
		return nil, fmt.Errorf("subscribe: %w", err)
	}
	return sub, nil
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

// subjectForEvent maps event types to NATS subjects.
func subjectForEvent(t EventType) string {
	switch {
	case len(t) > 4 && t[:4] == "job.":
		return "aiwx.job." + string(t[4:])
	case len(t) > 8 && t[:8] == "project.":
		return "aiwx.project." + string(t[8:])
	case len(t) > 5 && t[:5] == "cred.":
		return "aiwx.system." + string(t[5:])
	default:
		return "aiwx.system.unknown"
	}
}

// ─── Outbox Publisher Adapter ─────────────────────────────────────────────────

// NATSPublisher adapts the NATS client to the outbox.Publisher interface.
type NATSPublisher struct {
	client *Client
}

// NewNATSPublisher creates a new NATS publisher adapter.
func NewNATSPublisher(client *Client) *NATSPublisher {
	return &NATSPublisher{client: client}
}

// OutboxEvent is the interface expected by the outbox relay.
type OutboxEvent struct {
	ID             int64
	EventType      string
	AggregateType  string
	AggregateID    uuid.UUID
	Payload        map[string]any
	IdempotencyKey string
}

// Publish publishes an outbox event to NATS.
func (p *NATSPublisher) Publish(ctx context.Context, event OutboxEvent) error {
	subject := subjectForEvent(EventType(event.EventType))
	data, err := json.Marshal(map[string]any{
		"id":             event.ID,
		"type":           event.EventType,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"payload":        event.Payload,
		"idempotency_key": event.IdempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}

	return p.client.PublishRaw(ctx, subject, data)
}
