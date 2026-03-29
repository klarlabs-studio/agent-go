// Package notification provides domain models for webhook notifications.
package notification

import (
	"encoding/json"
	"time"
)

// EventType represents the type of notification event.
type EventType string

// Event types for webhook notifications.
const (
	EventRunStarted      EventType = "run.started"
	EventRunCompleted    EventType = "run.completed"
	EventRunFailed       EventType = "run.failed"
	EventRunPaused       EventType = "run.paused"
	EventStateChanged    EventType = "state.changed"
	EventToolStarted     EventType = "tool.started"
	EventToolCompleted   EventType = "tool.completed"
	EventToolFailed      EventType = "tool.failed"
	EventApprovalNeeded  EventType = "approval.needed"
	EventBudgetWarning   EventType = "budget.warning"
	EventBudgetExhausted EventType = "budget.exhausted"
)

// Event represents a notification event to be sent to webhooks.
type Event struct {
	// ID is a unique identifier for this event.
	ID string `json:"id"`
	// Type is the event type.
	Type EventType `json:"type"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// RunID is the associated run ID.
	RunID string `json:"run_id"`
	// Payload contains the event-specific data.
	Payload json.RawMessage `json:"payload"`
}

// RunStartedPayload contains data for run.started events.
type RunStartedPayload struct {
	Goal string         `json:"goal"`
	Vars map[string]any `json:"vars,omitempty"`
}

// RunCompletedPayload contains data for run.completed events.
type RunCompletedPayload struct {
	Result   json.RawMessage `json:"result"`
	Duration time.Duration   `json:"duration_ms"`
	Steps    int             `json:"steps"`
}

// RunFailedPayload contains data for run.failed events.
type RunFailedPayload struct {
	Error    string        `json:"error"`
	State    string        `json:"state"`
	Duration time.Duration `json:"duration_ms"`
}

// StateChangedPayload contains data for state.changed events.
type StateChangedPayload struct {
	FromState string `json:"from_state"`
	ToState   string `json:"to_state"`
	Reason    string `json:"reason,omitempty"`
}

// ToolStartedPayload contains data for tool.started events.
type ToolStartedPayload struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input,omitempty"`
	State    string          `json:"state"`
}

// ToolCompletedPayload contains data for tool.completed events.
type ToolCompletedPayload struct {
	ToolName string          `json:"tool_name"`
	Output   json.RawMessage `json:"output,omitempty"`
	Duration time.Duration   `json:"duration_ms"`
	Cached   bool            `json:"cached,omitempty"`
}

// ToolFailedPayload contains data for tool.failed events.
type ToolFailedPayload struct {
	ToolName string `json:"tool_name"`
	Error    string `json:"error"`
}

// ApprovalNeededPayload contains data for approval.needed events.
type ApprovalNeededPayload struct {
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	RiskLevel string          `json:"risk_level"`
}

// BudgetWarningPayload contains data for budget.warning events.
type BudgetWarningPayload struct {
	BudgetName string `json:"budget_name"`
	Remaining  int    `json:"remaining"`
	Limit      int    `json:"limit"`
	Percentage int    `json:"percentage"` // Percentage remaining
}

// BudgetExhaustedPayload contains data for budget.exhausted events.
type BudgetExhaustedPayload struct {
	BudgetName string `json:"budget_name"`
}

// NewEvent creates a new notification event.
func NewEvent(id string, eventType EventType, runID string, payload any) (*Event, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:        id,
		Type:      eventType,
		Timestamp: time.Now(),
		RunID:     runID,
		Payload:   payloadBytes,
	}, nil
}

// DecodePayload unmarshals the event payload into the given struct.
func (e *Event) DecodePayload(v any) error {
	if e.Payload == nil {
		return nil
	}
	return json.Unmarshal(e.Payload, v)
}
