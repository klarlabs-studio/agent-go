// Package ledger provides domain models for audit trail recording.
package ledger

import (
	"encoding/json"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// EntryType classifies the type of ledger entry.
type EntryType string

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

// Entry represents a single record in the ledger.
type Entry struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Type      EntryType       `json:"type"`
	RunID     string          `json:"run_id"`
	State     agent.State     `json:"state,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
}

// TransitionDetails contains details for state transition entries.
type TransitionDetails struct {
	FromState agent.State `json:"from_state"`
	ToState   agent.State `json:"to_state"`
	Reason    string      `json:"reason,omitempty"`
}

// DecisionDetails contains details for decision entries.
type DecisionDetails struct {
	DecisionType string          `json:"decision_type"`
	ToolName     string          `json:"tool_name,omitempty"`
	ToState      agent.State     `json:"to_state,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
}

// ToolCallDetails contains details for tool call entries.
type ToolCallDetails struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
}

// ToolResultDetails contains details for tool result entries.
type ToolResultDetails struct {
	ToolName string          `json:"tool_name"`
	Output   json.RawMessage `json:"output"`
	Duration time.Duration   `json:"duration"`
	Cached   bool            `json:"cached,omitempty"`
}

// ToolErrorDetails contains details for tool error entries.
type ToolErrorDetails struct {
	ToolName string `json:"tool_name"`
	Error    string `json:"error"`
}

// ApprovalRequestDetails contains details for approval request entries.
type ApprovalRequestDetails struct {
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	RiskLevel string          `json:"risk_level"`
}

// ApprovalResultDetails contains details for approval result entries.
type ApprovalResultDetails struct {
	ToolName string `json:"tool_name"`
	Approved bool   `json:"approved"`
	Approver string `json:"approver,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// BudgetDetails contains details for budget entries.
type BudgetDetails struct {
	BudgetName string `json:"budget_name"`
	Amount     int    `json:"amount"`
	Remaining  int    `json:"remaining"`
}

// HumanInputRequestDetails contains details for human input request entries.
type HumanInputRequestDetails struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

// HumanInputResponseDetails contains details for human input response entries.
type HumanInputResponseDetails struct {
	Question string `json:"question"`
	Response string `json:"response"`
}

// NewEntry creates a new ledger entry.
func NewEntry(entryType EntryType, runID string, state agent.State, details any) Entry {
	var detailsJSON json.RawMessage
	if details != nil {
		detailsJSON, _ = json.Marshal(details)
	}

	return Entry{
		ID:        generateEntryID(),
		Timestamp: time.Now(),
		Type:      entryType,
		RunID:     runID,
		State:     state,
		Details:   detailsJSON,
	}
}

// generateEntryID creates a unique entry ID.
func generateEntryID() string {
	return time.Now().Format("20060102150405.000000000")
}

// DecodeDetails unmarshals the entry details into the given struct.
func (e Entry) DecodeDetails(v any) error {
	if e.Details == nil {
		return nil
	}
	return json.Unmarshal(e.Details, v)
}
