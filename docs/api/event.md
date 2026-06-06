# Package `event`

**Import path:** `go.klarlabs.de/agent/domain/event`

## Overview

package event // import "go.klarlabs.de/agent/domain/event"

Package event provides domain types and interfaces for event sourcing.

## Full API Reference

```
package event // import "go.klarlabs.de/agent/domain/event"

Package event provides domain types and interfaces for event sourcing.

VARIABLES

var (
	// ErrEventNotFound is returned when an event does not exist.
	ErrEventNotFound = errors.New("event not found")

	// ErrRunNotFound is returned when events for a run do not exist.
	ErrRunNotFound = errors.New("run not found in event store")

	// ErrSequenceConflict is returned when event sequence numbers conflict.
	ErrSequenceConflict = errors.New("event sequence conflict")

	// ErrInvalidEvent is returned when an event is malformed.
	ErrInvalidEvent = errors.New("invalid event")

	// ErrSnapshotNotFound is returned when no snapshot exists for a run.
	ErrSnapshotNotFound = errors.New("snapshot not found")

	// ErrConnectionFailed is returned when connection to the store backend fails.
	ErrConnectionFailed = errors.New("event store connection failed")

	// ErrOperationTimeout is returned when a store operation times out.
	ErrOperationTimeout = errors.New("event store operation timeout")

	// ErrSubscriptionClosed is returned when a subscription channel is closed.
	ErrSubscriptionClosed = errors.New("event subscription closed")
)
    Domain errors for event store operations.


TYPES

type ApprovalRequestedPayload struct {
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	RiskLevel string          `json:"risk_level"`
}
    ApprovalRequestedPayload contains data for approval.requested events.

type ApprovalResultPayload struct {
	ToolName string `json:"tool_name"`
	Approver string `json:"approver"`
	Reason   string `json:"reason,omitempty"`
}
    ApprovalResultPayload contains data for approval.granted/denied events.

type BudgetConsumedPayload struct {
	BudgetName string `json:"budget_name"`
	Amount     int    `json:"amount"`
	Remaining  int    `json:"remaining"`
}
    BudgetConsumedPayload contains data for budget.consumed events.

type BudgetExhaustedPayload struct {
	BudgetName string `json:"budget_name"`
}
    BudgetExhaustedPayload contains data for budget.exhausted events.

type DecisionMadePayload struct {
	DecisionType string          `json:"decision_type"`
	ToolName     string          `json:"tool_name,omitempty"`
	ToState      agent.State     `json:"to_state,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
}
    DecisionMadePayload contains data for decision.made events.

type Event struct {
	// ID is the unique identifier for this event.
	ID string `json:"id"`

	// RunID is the ID of the run this event belongs to.
	RunID string `json:"run_id"`

	// Type classifies the event.
	Type Type `json:"type"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Payload contains the event-specific data.
	Payload json.RawMessage `json:"payload"`

	// Sequence is the ordering number within the run's event stream.
	Sequence uint64 `json:"sequence"`

	// Version is the event schema version for forward compatibility.
	Version int `json:"version,omitempty"`
}
    Event represents a domain event in the event store.

func NewEvent(runID string, eventType Type, payload any) (Event, error)
    NewEvent creates a new event with the given type and payload.

func (e *Event) UnmarshalPayload(v any) error
    UnmarshalPayload decodes the event payload into the given value.

type EvidenceAddedPayload struct {
	Type    string          `json:"type"`
	Source  string          `json:"source"`
	Content json.RawMessage `json:"content"`
}
    EvidenceAddedPayload contains data for evidence.added events.

type Pruner interface {
	// PruneEvents removes events before a sequence number.
	// Typically called after a snapshot is taken.
	PruneEvents(ctx context.Context, runID string, beforeSeq uint64) error
}
    Pruner is an optional interface for stores that support event pruning.
    This enables cleanup of old events after snapshotting.

type Publisher interface {
	// Publish sends events to the event store.
	Publish(ctx context.Context, events ...Event) error

	// Close releases any resources held by the publisher.
	Close() error
}
    Publisher publishes domain events to the event store.

type Querier interface {
	// Query retrieves events matching the given options.
	Query(ctx context.Context, runID string, opts QueryOptions) ([]Event, error)

	// CountEvents returns the number of events for a run.
	CountEvents(ctx context.Context, runID string) (int64, error)

	// ListRuns returns all run IDs with events in the store.
	ListRuns(ctx context.Context) ([]string, error)
}
    Querier is an optional interface for stores that support advanced queries.

type QueryOptions struct {
	// Types filters to specific event types (empty means all).
	Types []Type

	// FromTime filters events after this timestamp.
	FromTime int64

	// ToTime filters events before this timestamp.
	ToTime int64

	// Limit is the maximum number of events to return (0 = no limit).
	Limit int

	// Offset is the number of events to skip.
	Offset int
}
    QueryOptions configures event queries.

type RunCompletedPayload struct {
	Result   json.RawMessage `json:"result,omitempty"`
	Duration time.Duration   `json:"duration"`
}
    RunCompletedPayload contains data for run.completed events.

type RunFailedPayload struct {
	Error    string        `json:"error"`
	State    agent.State   `json:"state"`
	Duration time.Duration `json:"duration"`
}
    RunFailedPayload contains data for run.failed events.

type RunStartedPayload struct {
	Goal string         `json:"goal"`
	Vars map[string]any `json:"vars,omitempty"`
}
    RunStartedPayload contains data for run.started events.

type Snapshotter interface {
	// SaveSnapshot persists a snapshot of run state at a sequence number.
	SaveSnapshot(ctx context.Context, runID string, sequence uint64, data []byte) error

	// LoadSnapshot retrieves the latest snapshot for a run.
	// Returns the snapshot data and the sequence number it was taken at.
	LoadSnapshot(ctx context.Context, runID string) (data []byte, sequence uint64, err error)
}
    Snapshotter is an optional interface for stores that support snapshotting.
    Snapshots allow efficient replay by storing aggregate state at checkpoints.

type StateTransitionedPayload struct {
	FromState agent.State `json:"from_state"`
	ToState   agent.State `json:"to_state"`
	Reason    string      `json:"reason"`
}
    StateTransitionedPayload contains data for state.transitioned events.

type Store interface {
	// Append persists one or more events atomically.
	// Events are assigned sequence numbers in order of appearance.
	Append(ctx context.Context, events ...Event) error

	// LoadEvents retrieves all events for a run in sequence order.
	LoadEvents(ctx context.Context, runID string) ([]Event, error)

	// LoadEventsFrom retrieves events starting from a specific sequence number.
	// This enables incremental replay from a known checkpoint.
	LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]Event, error)

	// Subscribe returns a channel that receives new events for a run.
	// The channel is closed when the context is cancelled or the run completes.
	Subscribe(ctx context.Context, runID string) (<-chan Event, error)
}
    Store defines the interface for event persistence. Implementations may be
    in-memory, PostgreSQL, or any other backend.

type Subscriber interface {
	// Subscribe returns a channel that receives events for a run.
	Subscribe(ctx context.Context, runID string) (<-chan Event, error)
}
    Subscriber receives events from a publisher or store.

type ToolCalledPayload struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
	State    agent.State     `json:"state"`
	Reason   string          `json:"reason,omitempty"`
}
    ToolCalledPayload contains data for tool.called events.

type ToolFailedPayload struct {
	ToolName string        `json:"tool_name"`
	Error    string        `json:"error"`
	Duration time.Duration `json:"duration"`
}
    ToolFailedPayload contains data for tool.failed events.

type ToolSucceededPayload struct {
	ToolName string          `json:"tool_name"`
	Output   json.RawMessage `json:"output"`
	Duration time.Duration   `json:"duration"`
	Cached   bool            `json:"cached"`
}
    ToolSucceededPayload contains data for tool.succeeded events.

type Type string
    Type classifies domain events.

const (
	// Run lifecycle events
	TypeRunStarted   Type = "run.started"
	TypeRunCompleted Type = "run.completed"
	TypeRunFailed    Type = "run.failed"
	TypeRunPaused    Type = "run.paused"
	TypeRunResumed   Type = "run.resumed"

	// State machine events
	TypeStateTransitioned Type = "state.transitioned"

	// Tool execution events
	TypeToolCalled    Type = "tool.called"
	TypeToolSucceeded Type = "tool.succeeded"
	TypeToolFailed    Type = "tool.failed"

	// Decision events
	TypeDecisionMade Type = "decision.made"

	// Approval events
	TypeApprovalRequested Type = "approval.requested"
	TypeApprovalGranted   Type = "approval.granted"
	TypeApprovalDenied    Type = "approval.denied"

	// Budget events
	TypeBudgetConsumed  Type = "budget.consumed"
	TypeBudgetExhausted Type = "budget.exhausted"

	// Evidence events
	TypeEvidenceAdded Type = "evidence.added"

	// Variable events
	TypeVariableSet Type = "variable.set"
)
    Event types for the agent runtime.

type VariableSetPayload struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}
    VariableSetPayload contains data for variable.set events.
```
