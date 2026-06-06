package ledger

import (
	"time"

	"go.klarlabs.de/agent/domain/agent"
)

// Event represents a domain event that can be published.
type Event interface {
	EventType() string
	Timestamp() time.Time
	RunID() string
}

// BaseEvent provides common event fields.
type BaseEvent struct {
	Type  string      `json:"type"`
	Time  time.Time   `json:"timestamp"`
	Run   string      `json:"run_id"`
	State agent.State `json:"state,omitempty"`
}

// EventType returns the event type.
func (e BaseEvent) EventType() string {
	return e.Type
}

// Timestamp returns the event timestamp.
func (e BaseEvent) Timestamp() time.Time {
	return e.Time
}

// RunID returns the run ID.
func (e BaseEvent) RunID() string {
	return e.Run
}

// RunStartedEvent is published when a run starts.
type RunStartedEvent struct {
	BaseEvent
	Goal string `json:"goal"`
}

// NewRunStartedEvent creates a run started event.
func NewRunStartedEvent(runID, goal string) RunStartedEvent {
	return RunStartedEvent{
		BaseEvent: BaseEvent{
			Type:  "run.started",
			Time:  time.Now(),
			Run:   runID,
			State: agent.StateIntake,
		},
		Goal: goal,
	}
}

// RunCompletedEvent is published when a run completes successfully.
type RunCompletedEvent struct {
	BaseEvent
	Summary  string        `json:"summary"`
	Duration time.Duration `json:"duration"`
}

// NewRunCompletedEvent creates a run completed event.
func NewRunCompletedEvent(runID, summary string, duration time.Duration) RunCompletedEvent {
	return RunCompletedEvent{
		BaseEvent: BaseEvent{
			Type:  "run.completed",
			Time:  time.Now(),
			Run:   runID,
			State: agent.StateDone,
		},
		Summary:  summary,
		Duration: duration,
	}
}

// RunFailedEvent is published when a run fails.
type RunFailedEvent struct {
	BaseEvent
	Reason   string        `json:"reason"`
	Duration time.Duration `json:"duration"`
}

// NewRunFailedEvent creates a run failed event.
func NewRunFailedEvent(runID, reason string, state agent.State, duration time.Duration) RunFailedEvent {
	return RunFailedEvent{
		BaseEvent: BaseEvent{
			Type:  "run.failed",
			Time:  time.Now(),
			Run:   runID,
			State: state,
		},
		Reason:   reason,
		Duration: duration,
	}
}

// StateChangedEvent is published when the state changes.
type StateChangedEvent struct {
	BaseEvent
	FromState agent.State `json:"from_state"`
	ToState   agent.State `json:"to_state"`
	Reason    string      `json:"reason,omitempty"`
}

// NewStateChangedEvent creates a state changed event.
func NewStateChangedEvent(runID string, from, to agent.State, reason string) StateChangedEvent {
	return StateChangedEvent{
		BaseEvent: BaseEvent{
			Type:  "state.changed",
			Time:  time.Now(),
			Run:   runID,
			State: to,
		},
		FromState: from,
		ToState:   to,
		Reason:    reason,
	}
}

// ToolExecutedEvent is published when a tool is executed.
type ToolExecutedEvent struct {
	BaseEvent
	ToolName string        `json:"tool_name"`
	Duration time.Duration `json:"duration"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
}

// NewToolExecutedEvent creates a tool executed event.
func NewToolExecutedEvent(runID string, state agent.State, toolName string, duration time.Duration, success bool, err string) ToolExecutedEvent {
	return ToolExecutedEvent{
		BaseEvent: BaseEvent{
			Type:  "tool.executed",
			Time:  time.Now(),
			Run:   runID,
			State: state,
		},
		ToolName: toolName,
		Duration: duration,
		Success:  success,
		Error:    err,
	}
}

// EventPublisher publishes domain events.
type EventPublisher interface {
	Publish(event Event) error
}

// NoOpPublisher discards all events.
type NoOpPublisher struct{}

// Publish discards the event.
func (NoOpPublisher) Publish(_ Event) error {
	return nil
}
