package event

import (
	"encoding/json"
	"time"

	"go.klarlabs.de/agent/domain/agent"
)

// Type classifies domain events.
type Type string

// Event types for the agent runtime.
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

	// Planner events
	TypePlannerProposed Type = "planner.proposed"

	// Agent protocol events
	TypeAgentMessageSent     Type = "agent.message.sent"
	TypeAgentMessageReceived Type = "agent.message.received"
	TypeAgentDelegated       Type = "agent.delegated"
)

// Event payload structures

// RunStartedPayload contains data for run.started events.
type RunStartedPayload struct {
	Goal string         `json:"goal"`
	Vars map[string]any `json:"vars,omitempty"`
}

// RunCompletedPayload contains data for run.completed events.
type RunCompletedPayload struct {
	Result   json.RawMessage `json:"result,omitempty"`
	Duration time.Duration   `json:"duration"`
}

// RunFailedPayload contains data for run.failed events.
type RunFailedPayload struct {
	Error    string        `json:"error"`
	State    agent.State   `json:"state"`
	Duration time.Duration `json:"duration"`
}

// StateTransitionedPayload contains data for state.transitioned events.
type StateTransitionedPayload struct {
	FromState agent.State `json:"from_state"`
	ToState   agent.State `json:"to_state"`
	Reason    string      `json:"reason"`
}

// ToolCalledPayload contains data for tool.called events.
type ToolCalledPayload struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
	State    agent.State     `json:"state"`
	Reason   string          `json:"reason,omitempty"`
}

// ToolSucceededPayload contains data for tool.succeeded events.
type ToolSucceededPayload struct {
	ToolName string          `json:"tool_name"`
	Output   json.RawMessage `json:"output"`
	Duration time.Duration   `json:"duration"`
	Cached   bool            `json:"cached"`
}

// ToolFailedPayload contains data for tool.failed events.
type ToolFailedPayload struct {
	ToolName string        `json:"tool_name"`
	Error    string        `json:"error"`
	Duration time.Duration `json:"duration"`
}

// DecisionMadePayload contains data for decision.made events.
type DecisionMadePayload struct {
	DecisionType string          `json:"decision_type"`
	ToolName     string          `json:"tool_name,omitempty"`
	ToState      agent.State     `json:"to_state,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
}

// ApprovalRequestedPayload contains data for approval.requested events.
type ApprovalRequestedPayload struct {
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	RiskLevel string          `json:"risk_level"`
}

// ApprovalResultPayload contains data for approval.granted/denied events.
type ApprovalResultPayload struct {
	ToolName string `json:"tool_name"`
	Approver string `json:"approver"`
	Reason   string `json:"reason,omitempty"`
}

// BudgetConsumedPayload contains data for budget.consumed events.
type BudgetConsumedPayload struct {
	BudgetName string `json:"budget_name"`
	Amount     int    `json:"amount"`
	Remaining  int    `json:"remaining"`
}

// BudgetExhaustedPayload contains data for budget.exhausted events.
type BudgetExhaustedPayload struct {
	BudgetName string `json:"budget_name"`
}

// EvidenceAddedPayload contains data for evidence.added events.
type EvidenceAddedPayload struct {
	Type    string          `json:"type"`
	Source  string          `json:"source"`
	Content json.RawMessage `json:"content"`
}

// VariableSetPayload contains data for variable.set events.
type VariableSetPayload struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// PlannerProposedPayload contains data for planner.proposed events.
// This captures what the planner intended BEFORE execution, allowing
// comparison with the actual outcome (decision.made).
type PlannerProposedPayload struct {
	DecisionType string          `json:"decision_type"`
	ToolName     string          `json:"tool_name,omitempty"`
	ToState      agent.State     `json:"to_state,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
}

// AgentMessagePayload contains data for agent message events.
type AgentMessagePayload struct {
	MessageID     string `json:"message_id"`
	CorrelationID string `json:"correlation_id"`
	Sender        string `json:"sender"`
	Receiver      string `json:"receiver,omitempty"`
	Action        string `json:"action"`
	MessageType   string `json:"message_type"`
}

// AgentDelegatedPayload contains data for agent.delegated events.
type AgentDelegatedPayload struct {
	ParentRunID string `json:"parent_run_id"`
	ChildRunID  string `json:"child_run_id"`
	AgentName   string `json:"agent_name"`
	Goal        string `json:"goal"`
}
