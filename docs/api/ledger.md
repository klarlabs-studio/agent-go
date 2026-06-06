# Package `ledger`

**Import path:** `go.klarlabs.de/agent/domain/ledger`

## Overview

package ledger // import "go.klarlabs.de/agent/domain/ledger"

Package ledger provides domain models for audit trail recording.

## Full API Reference

```
package ledger // import "go.klarlabs.de/agent/domain/ledger"

Package ledger provides domain models for audit trail recording.

TYPES

type ApprovalRequestDetails struct {
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	RiskLevel string          `json:"risk_level"`
}
    ApprovalRequestDetails contains details for approval request entries.

type ApprovalResultDetails struct {
	ToolName string `json:"tool_name"`
	Approved bool   `json:"approved"`
	Approver string `json:"approver,omitempty"`
	Reason   string `json:"reason,omitempty"`
}
    ApprovalResultDetails contains details for approval result entries.

type BaseEvent struct {
	Type  string      `json:"type"`
	Time  time.Time   `json:"timestamp"`
	Run   string      `json:"run_id"`
	State agent.State `json:"state,omitempty"`
}
    BaseEvent provides common event fields.

func (e BaseEvent) EventType() string
    EventType returns the event type.

func (e BaseEvent) RunID() string
    RunID returns the run ID.

func (e BaseEvent) Timestamp() time.Time
    Timestamp returns the event timestamp.

type BudgetDetails struct {
	BudgetName string `json:"budget_name"`
	Amount     int    `json:"amount"`
	Remaining  int    `json:"remaining"`
}
    BudgetDetails contains details for budget entries.

type DecisionDetails struct {
	DecisionType string          `json:"decision_type"`
	ToolName     string          `json:"tool_name,omitempty"`
	ToState      agent.State     `json:"to_state,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
}
    DecisionDetails contains details for decision entries.

type Entry struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Type      EntryType       `json:"type"`
	RunID     string          `json:"run_id"`
	State     agent.State     `json:"state,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
}
    Entry represents a single record in the ledger.

func NewEntry(entryType EntryType, runID string, state agent.State, details any) Entry
    NewEntry creates a new ledger entry.

func (e Entry) DecodeDetails(v any) error
    DecodeDetails unmarshals the entry details into the given struct.

type EntryType string
    EntryType classifies the type of ledger entry.

const (
	EntryRunStarted         EntryType = "run_started"
	EntryRunCompleted       EntryType = "run_completed"
	EntryRunFailed          EntryType = "run_failed"
	EntryStateTransition    EntryType = "state_transition"
	EntryDecision           EntryType = "decision"
	EntryToolCall           EntryType = "tool_call"
	EntryToolResult         EntryType = "tool_result"
	EntryToolError          EntryType = "tool_error"
	EntryApprovalRequest    EntryType = "approval_request"
	EntryApprovalResult     EntryType = "approval_result"
	EntryHumanInputRequest  EntryType = "human_input_request"
	EntryHumanInputResponse EntryType = "human_input_response"
	EntryBudgetConsumed     EntryType = "budget_consumed"
	EntryBudgetExhausted    EntryType = "budget_exhausted"
)
type Event interface {
	EventType() string
	Timestamp() time.Time
	RunID() string
}
    Event represents a domain event that can be published.

type EventPublisher interface {
	Publish(event Event) error
}
    EventPublisher publishes domain events.

type HumanInputRequestDetails struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}
    HumanInputRequestDetails contains details for human input request entries.

type HumanInputResponseDetails struct {
	Question string `json:"question"`
	Response string `json:"response"`
}
    HumanInputResponseDetails contains details for human input response entries.

type Ledger struct {
	// Has unexported fields.
}
    Ledger provides an append-only record of all actions during a run.

func New(runID string) *Ledger
    New creates a new ledger for the given run.

func (l *Ledger) Append(entry Entry)
    Append adds an entry to the ledger.

func (l *Ledger) Count() int
    Count returns the number of entries.

func (l *Ledger) Entries() []Entry
    Entries returns a copy of all entries.

func (l *Ledger) EntriesByType(entryType EntryType) []Entry
    EntriesByType returns entries filtered by type.

func (l *Ledger) LastEntry() *Entry
    LastEntry returns the most recent entry, or nil if empty.

func (l *Ledger) RecordApprovalRequest(state agent.State, toolName string, input json.RawMessage, riskLevel string)
    RecordApprovalRequest records an approval request.

func (l *Ledger) RecordApprovalResult(state agent.State, toolName string, approved bool, approver, reason string)
    RecordApprovalResult records an approval result.

func (l *Ledger) RecordBudgetConsumed(state agent.State, budgetName string, amount, remaining int)
    RecordBudgetConsumed records budget consumption.

func (l *Ledger) RecordBudgetExhausted(state agent.State, budgetName string)
    RecordBudgetExhausted records budget exhaustion.

func (l *Ledger) RecordDecision(state agent.State, decision agent.Decision)
    RecordDecision records a planner decision.

func (l *Ledger) RecordHumanInputRequest(state agent.State, question string, options []string)
    RecordHumanInputRequest records a request for human input.

func (l *Ledger) RecordHumanInputResponse(state agent.State, question, response string)
    RecordHumanInputResponse records a human input response.

func (l *Ledger) RecordRunCompleted(result json.RawMessage)
    RecordRunCompleted records the successful completion of a run.

func (l *Ledger) RecordRunFailed(state agent.State, reason string)
    RecordRunFailed records the failure of a run.

func (l *Ledger) RecordRunStarted(goal string)
    RecordRunStarted records the start of a run.

func (l *Ledger) RecordToolCall(state agent.State, toolName string, input json.RawMessage)
    RecordToolCall records a tool invocation.

func (l *Ledger) RecordToolError(state agent.State, toolName string, err error)
    RecordToolError records a tool error.

func (l *Ledger) RecordToolResult(state agent.State, toolName string, output json.RawMessage, duration time.Duration, cached bool)
    RecordToolResult records a tool result.

func (l *Ledger) RecordTransition(from, to agent.State, reason string)
    RecordTransition records a state transition.

func (l *Ledger) RunID() string
    RunID returns the associated run ID.

type NoOpPublisher struct{}
    NoOpPublisher discards all events.

func (NoOpPublisher) Publish(_ Event) error
    Publish discards the event.

type RunCompletedEvent struct {
	BaseEvent
	Summary  string        `json:"summary"`
	Duration time.Duration `json:"duration"`
}
    RunCompletedEvent is published when a run completes successfully.

func NewRunCompletedEvent(runID, summary string, duration time.Duration) RunCompletedEvent
    NewRunCompletedEvent creates a run completed event.

type RunFailedEvent struct {
	BaseEvent
	Reason   string        `json:"reason"`
	Duration time.Duration `json:"duration"`
}
    RunFailedEvent is published when a run fails.

func NewRunFailedEvent(runID, reason string, state agent.State, duration time.Duration) RunFailedEvent
    NewRunFailedEvent creates a run failed event.

type RunStartedEvent struct {
	BaseEvent
	Goal string `json:"goal"`
}
    RunStartedEvent is published when a run starts.

func NewRunStartedEvent(runID, goal string) RunStartedEvent
    NewRunStartedEvent creates a run started event.

type StateChangedEvent struct {
	BaseEvent
	FromState agent.State `json:"from_state"`
	ToState   agent.State `json:"to_state"`
	Reason    string      `json:"reason,omitempty"`
}
    StateChangedEvent is published when the state changes.

func NewStateChangedEvent(runID string, from, to agent.State, reason string) StateChangedEvent
    NewStateChangedEvent creates a state changed event.

type ToolCallDetails struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
}
    ToolCallDetails contains details for tool call entries.

type ToolErrorDetails struct {
	ToolName string `json:"tool_name"`
	Error    string `json:"error"`
}
    ToolErrorDetails contains details for tool error entries.

type ToolExecutedEvent struct {
	BaseEvent
	ToolName string        `json:"tool_name"`
	Duration time.Duration `json:"duration"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
}
    ToolExecutedEvent is published when a tool is executed.

func NewToolExecutedEvent(runID string, state agent.State, toolName string, duration time.Duration, success bool, err string) ToolExecutedEvent
    NewToolExecutedEvent creates a tool executed event.

type ToolResultDetails struct {
	ToolName string          `json:"tool_name"`
	Output   json.RawMessage `json:"output"`
	Duration time.Duration   `json:"duration"`
	Cached   bool            `json:"cached,omitempty"`
}
    ToolResultDetails contains details for tool result entries.

type TransitionDetails struct {
	FromState agent.State `json:"from_state"`
	ToState   agent.State `json:"to_state"`
	Reason    string      `json:"reason,omitempty"`
}
    TransitionDetails contains details for state transition entries.
```
