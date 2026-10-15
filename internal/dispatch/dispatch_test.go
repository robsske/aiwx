package dispatch

import (
	"testing"

	"github.com/rverhe/aiwx/internal/domain"
)

// ─── State Machine Tests ──────────────────────────────────────────────────────

func TestIsValidTransition_Valid(t *testing.T) {
	validTransitions := []struct {
		from domain.JobStatus
		to   domain.JobStatus
	}{
		{domain.JobStatusNew, domain.JobStatusPending},
		{domain.JobStatusNew, domain.JobStatusCanceled},
		{domain.JobStatusPending, domain.JobStatusWaiting},
		{domain.JobStatusPending, domain.JobStatusCanceled},
		{domain.JobStatusWaiting, domain.JobStatusRunning},
		{domain.JobStatusWaiting, domain.JobStatusCanceled},
		{domain.JobStatusWaiting, domain.JobStatusError},
		{domain.JobStatusRunning, domain.JobStatusSuccessful},
		{domain.JobStatusRunning, domain.JobStatusFailed},
		{domain.JobStatusRunning, domain.JobStatusError},
		{domain.JobStatusRunning, domain.JobStatusCanceled},
	}

	for _, tt := range validTransitions {
		if !IsValidTransition(tt.from, tt.to) {
			t.Errorf("IsValidTransition(%s, %s) = false, want true", tt.from, tt.to)
		}
	}
}

func TestIsValidTransition_Invalid(t *testing.T) {
	invalidTransitions := []struct {
		from domain.JobStatus
		to   domain.JobStatus
	}{
		// Can't skip states
		{domain.JobStatusNew, domain.JobStatusRunning},
		{domain.JobStatusNew, domain.JobStatusSuccessful},
		// Terminal states are immutable
		{domain.JobStatusSuccessful, domain.JobStatusRunning},
		{domain.JobStatusFailed, domain.JobStatusPending},
		{domain.JobStatusError, domain.JobStatusRunning},
		{domain.JobStatusCanceled, domain.JobStatusRunning},
		// Can't go backwards
		{domain.JobStatusRunning, domain.JobStatusPending},
		{domain.JobStatusWaiting, domain.JobStatusPending},
	}

	for _, tt := range invalidTransitions {
		if IsValidTransition(tt.from, tt.to) {
			t.Errorf("IsValidTransition(%s, %s) = true, want false", tt.from, tt.to)
		}
	}
}

func TestTransitionOwner(t *testing.T) {
	tests := []struct {
		from  domain.JobStatus
		to    domain.JobStatus
		owner string
	}{
		{domain.JobStatusNew, domain.JobStatusPending, "api"},
		{domain.JobStatusPending, domain.JobStatusWaiting, "scheduler"},
		{domain.JobStatusWaiting, domain.JobStatusRunning, "reconciler"},
		{domain.JobStatusRunning, domain.JobStatusSuccessful, "reconciler"},
		{domain.JobStatusRunning, domain.JobStatusFailed, "reconciler"},
		{domain.JobStatusRunning, domain.JobStatusError, "reconciler"},
		{domain.JobStatusRunning, domain.JobStatusCanceled, "scheduler"},
	}

	for _, tt := range tests {
		owner := TransitionOwner(tt.from, tt.to)
		if owner != tt.owner {
			t.Errorf("TransitionOwner(%s, %s) = %q, want %q", tt.from, tt.to, owner, tt.owner)
		}
	}
}

// ─── Terminal State Immutability ──────────────────────────────────────────────

func TestTerminalStates_NoTransitions(t *testing.T) {
	terminalStates := []domain.JobStatus{
		domain.JobStatusSuccessful,
		domain.JobStatusFailed,
		domain.JobStatusError,
		domain.JobStatusCanceled,
	}

	allStates := []domain.JobStatus{
		domain.JobStatusNew,
		domain.JobStatusPending,
		domain.JobStatusWaiting,
		domain.JobStatusRunning,
		domain.JobStatusSuccessful,
		domain.JobStatusFailed,
		domain.JobStatusError,
		domain.JobStatusCanceled,
	}

	for _, terminal := range terminalStates {
		if !terminal.Terminal() {
			t.Errorf("%s should be terminal", terminal)
		}
		for _, to := range allStates {
			if IsValidTransition(terminal, to) {
				t.Errorf("SECURITY: terminal state %s should not transition to %s", terminal, to)
			}
		}
	}
}
