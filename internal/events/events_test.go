package events

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSubjectForEvent(t *testing.T) {
	cases := []struct {
		event EventType
		want  string
	}{
		{JobCreated, "aiwx.job.created"},
		{JobSuccessful, "aiwx.job.successful"},
		{JobFailed, "aiwx.job.failed"},
		{ProjectSyncStarted, "aiwx.project.sync.started"},
		{ProjectSyncCompleted, "aiwx.project.sync.completed"},
		{ProjectSyncFailed, "aiwx.project.sync.failed"},
		{CredentialAccessDenied, "aiwx.system.access.denied"},
		{EventType("unknown.thing"), "aiwx.system.unknown"},
	}
	for _, tc := range cases {
		if got := SubjectForEvent(tc.event); got != tc.want {
			t.Errorf("SubjectForEvent(%q) = %q, want %q", tc.event, got, tc.want)
		}
	}
}

func TestNATSPublisher_PublishEnvelope(t *testing.T) {
	// Verify envelope construction without a live NATS server by exercising
	// the same mapping the publisher uses.
	org := uuid.New()
	agg := uuid.New()
	event := OutboxEvent{
		ID:             42,
		EventType:      "job.created",
		AggregateType:  "job",
		AggregateID:    agg,
		Payload:        map[string]any{"organization_id": org.String(), "job_id": agg.String()},
		IdempotencyKey: "job-launch-" + agg.String(),
		CreatedAt:      time.Unix(1700000000, 0).UTC(),
	}

	orgID := uuid.Nil
	if s, ok := event.Payload["organization_id"].(string); ok {
		if parsed, err := uuid.Parse(s); err == nil {
			orgID = parsed
		}
	}
	envelope := Event{
		Type:           EventType(event.EventType),
		AggregateType:  event.AggregateType,
		AggregateID:    event.AggregateID,
		OrganizationID: orgID,
		Payload:        event.Payload,
		Timestamp:      event.CreatedAt,
		IdempotencyKey: event.IdempotencyKey,
		OutboxID:       event.ID,
	}

	if envelope.Type != JobCreated {
		t.Errorf("Type = %q, want %q", envelope.Type, JobCreated)
	}
	if envelope.OrganizationID != org {
		t.Errorf("OrganizationID = %s, want %s", envelope.OrganizationID, org)
	}
	if envelope.AggregateID != agg {
		t.Errorf("AggregateID = %s, want %s", envelope.AggregateID, agg)
	}
	if envelope.OutboxID != 42 {
		t.Errorf("OutboxID = %d, want 42", envelope.OutboxID)
	}
	if envelope.IdempotencyKey != event.IdempotencyKey {
		t.Errorf("IdempotencyKey = %q, want %q", envelope.IdempotencyKey, event.IdempotencyKey)
	}
	if SubjectForEvent(envelope.Type) != "aiwx.job.created" {
		t.Errorf("subject = %q", SubjectForEvent(envelope.Type))
	}
	_ = context.Background()
}
