package postgres

import (
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

func TestBulkInsertRuns_EmptySlice(t *testing.T) {
	store := NewRunStore(nil, "public")

	err := store.BulkInsertRuns(t.Context(), nil)
	if err != nil {
		t.Errorf("BulkInsertRuns() error = %v, want nil", err)
	}
}

func TestBulkInsertRuns_InvalidRunID(t *testing.T) {
	store := NewRunStore(nil, "public")

	runs := []*agent.Run{
		{ID: "valid-run", Goal: "test"},
		{ID: "", Goal: "test"},
	}

	err := store.BulkInsertRuns(t.Context(), runs)
	if err != run.ErrInvalidRunID {
		t.Errorf("BulkInsertRuns() error = %v, want %v", err, run.ErrInvalidRunID)
	}
}

func TestBulkInsertEvents_EmptySlice(t *testing.T) {
	store := NewEventStore(nil, "public")

	err := store.BulkInsertEvents(t.Context(), nil)
	if err != nil {
		t.Errorf("BulkInsertEvents() error = %v, want nil", err)
	}
}

func TestBulkInsertEvents_InvalidEvent(t *testing.T) {
	store := NewEventStore(nil, "public")

	events := []event.Event{
		{RunID: "run-1", Type: ""}, // Empty type should fail validation.
	}

	err := store.BulkInsertEvents(t.Context(), events)
	if err != event.ErrInvalidEvent {
		t.Errorf("BulkInsertEvents() error = %v, want %v", err, event.ErrInvalidEvent)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{"nil error", "", false},
		{"duplicate key", "ERROR: duplicate key value violates unique constraint", true},
		{"code 23505", "ERROR: 23505 unique_violation", true},
		{"other error", "connection refused", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errMsg == "" {
				if isUniqueViolation(nil) {
					t.Error("isUniqueViolation(nil) = true, want false")
				}
				return
			}
			err := testError(tt.errMsg)
			got := isUniqueViolation(err)
			if got != tt.expected {
				t.Errorf("isUniqueViolation(%q) = %v, want %v", tt.errMsg, got, tt.expected)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		s, substr string
		expected  bool
	}{
		{"hello world", "world", true},
		{"hello world", "xyz", false},
		{"", "", true},
		{"abc", "", true},
		{"", "abc", false},
	}

	for _, tt := range tests {
		got := containsString(tt.s, tt.substr)
		if got != tt.expected {
			t.Errorf("containsString(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.expected)
		}
	}
}

// testError is a simple error type for testing.
type testError string

func (e testError) Error() string { return string(e) }
