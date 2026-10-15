package domain

import (
	"testing"
)

func TestJobStatus_Terminal(t *testing.T) {
	tests := []struct {
		status   JobStatus
		terminal bool
	}{
		{JobStatusNew, false},
		{JobStatusPending, false},
		{JobStatusWaiting, false},
		{JobStatusRunning, false},
		{JobStatusSuccessful, true},
		{JobStatusFailed, true},
		{JobStatusError, true},
		{JobStatusCanceled, true},
	}

	for _, tt := range tests {
		if got := tt.status.Terminal(); got != tt.terminal {
			t.Errorf("JobStatus(%s).Terminal() = %v, want %v", tt.status, got, tt.terminal)
		}
	}
}

func TestAPIError_Error(t *testing.T) {
	err := NewValidationError("test error", nil)
	if err.Error() != "test error" {
		t.Errorf("APIError.Error() = %q, want %q", err.Error(), "test error")
	}
	if err.ErrorCode != "VALIDATION_ERROR" {
		t.Errorf("APIError.ErrorCode = %q, want %q", err.ErrorCode, "VALIDATION_ERROR")
	}
}

func TestNotFound(t *testing.T) {
	err := NewNotFoundError("Organization")
	if err.ErrorCode != "NOT_FOUND" {
		t.Errorf("ErrorCode = %q, want %q", err.ErrorCode, "NOT_FOUND")
	}
	if err.Message != "Organization not found" {
		t.Errorf("Message = %q, want %q", err.Message, "Organization not found")
	}
}
