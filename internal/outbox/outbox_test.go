package outbox

import (
	"context"
	"testing"
	"time"
)

func TestEventType_Constants(t *testing.T) {
	// Verify event type constants are properly defined
	eventTypes := map[string]string{
		"job.created":    EventTypeJobCreated,
		"job.pending":    EventTypeJobPending,
		"job.waiting":    EventTypeJobWaiting,
		"job.running":    EventTypeJobRunning,
		"job.successful": EventTypeJobSuccessful,
		"job.failed":     EventTypeJobFailed,
		"job.error":      EventTypeJobError,
		"job.canceled":   EventTypeJobCanceled,
		"project.sync":   EventTypeProjectSync,
	}

	for expected, actual := range eventTypes {
		if expected != actual {
			t.Errorf("Event type mismatch: expected %q, got %q", expected, actual)
		}
	}
}

func TestRelay_Creation(t *testing.T) {
	store := &Store{pool: nil}
	publisher := &mockPublisher{}
	relay := NewRelay(store, publisher, 10, time.Second)

	if relay == nil {
		t.Fatal("NewRelay should not return nil")
	}
	if relay.batchSize != 10 {
		t.Errorf("batchSize = %d, want 10", relay.batchSize)
	}
	if relay.interval != time.Second {
		t.Errorf("interval = %v, want 1s", relay.interval)
	}
}

// mockPublisher implements the Publisher interface for testing.
type mockPublisher struct {
	published []Event
	errors    []error
}

func (m *mockPublisher) Publish(ctx context.Context, event Event) error {
	m.published = append(m.published, event)
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return err
	}
	return nil
}
