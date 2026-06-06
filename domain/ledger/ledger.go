package ledger

import (
	"encoding/json"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
)

// Ledger provides an append-only record of all actions during a run.
type Ledger struct {
	runID   string
	entries []Entry
	mu      sync.RWMutex
}

// New creates a new ledger for the given run.
func New(runID string) *Ledger {
	return &Ledger{
		runID:   runID,
		entries: make([]Entry, 0),
	}
}

// Append adds an entry to the ledger.
func (l *Ledger) Append(entry Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry.RunID = l.runID
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.ID == "" {
		entry.ID = generateEntryID()
	}

	l.entries = append(l.entries, entry)
}

// Entries returns a copy of all entries.
func (l *Ledger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entries := make([]Entry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// EntriesByType returns entries filtered by type.
func (l *Ledger) EntriesByType(entryType EntryType) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []Entry
	for _, e := range l.entries {
		if e.Type == entryType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// LastEntry returns the most recent entry, or nil if empty.
func (l *Ledger) LastEntry() *Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.entries) == 0 {
		return nil
	}
	entry := l.entries[len(l.entries)-1]
	return &entry
}

// Count returns the number of entries.
func (l *Ledger) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// RunID returns the associated run ID.
func (l *Ledger) RunID() string {
	return l.runID
}

// RecordRunStarted records the start of a run.
func (l *Ledger) RecordRunStarted(goal string) {
	l.Append(NewEntry(EntryRunStarted, l.runID, agent.StateIntake, map[string]string{
		"goal": goal,
	}))
}

// RecordRunCompleted records the successful completion of a run.
func (l *Ledger) RecordRunCompleted(result json.RawMessage) {
	l.Append(NewEntry(EntryRunCompleted, l.runID, agent.StateDone, map[string]json.RawMessage{
		"result": result,
	}))
}

// RecordRunFailed records the failure of a run.
func (l *Ledger) RecordRunFailed(state agent.State, reason string) {
	l.Append(NewEntry(EntryRunFailed, l.runID, state, map[string]string{
		"reason": reason,
	}))
}

// RecordTransition records a state transition.
func (l *Ledger) RecordTransition(from, to agent.State, reason string) {
	l.Append(NewEntry(EntryStateTransition, l.runID, to, TransitionDetails{
		FromState: from,
		ToState:   to,
		Reason:    reason,
	}))
}

// RecordDecision records a planner decision.
func (l *Ledger) RecordDecision(state agent.State, decision agent.Decision) {
	details := DecisionDetails{
		DecisionType: string(decision.Type),
	}

	switch decision.Type {
	case agent.DecisionCallTool:
		details.ToolName = decision.CallTool.ToolName
		details.Input = decision.CallTool.Input
		details.Reason = decision.CallTool.Reason
	case agent.DecisionTransition:
		details.ToState = decision.Transition.ToState
		details.Reason = decision.Transition.Reason
	case agent.DecisionFinish:
		details.Reason = decision.Finish.Summary
	case agent.DecisionFail:
		details.Reason = decision.Fail.Reason
	}

	l.Append(NewEntry(EntryDecision, l.runID, state, details))
}

// RecordToolCall records a tool invocation.
func (l *Ledger) RecordToolCall(state agent.State, toolName string, input json.RawMessage) {
	l.Append(NewEntry(EntryToolCall, l.runID, state, ToolCallDetails{
		ToolName: toolName,
		Input:    input,
	}))
}

// RecordToolResult records a tool result.
func (l *Ledger) RecordToolResult(state agent.State, toolName string, output json.RawMessage, duration time.Duration, cached bool) {
	l.Append(NewEntry(EntryToolResult, l.runID, state, ToolResultDetails{
		ToolName: toolName,
		Output:   output,
		Duration: duration,
		Cached:   cached,
	}))
}

// RecordToolError records a tool error.
func (l *Ledger) RecordToolError(state agent.State, toolName string, err error) {
	l.Append(NewEntry(EntryToolError, l.runID, state, ToolErrorDetails{
		ToolName: toolName,
		Error:    err.Error(),
	}))
}

// RecordApprovalRequest records an approval request.
func (l *Ledger) RecordApprovalRequest(state agent.State, toolName string, input json.RawMessage, riskLevel string) {
	l.Append(NewEntry(EntryApprovalRequest, l.runID, state, ApprovalRequestDetails{
		ToolName:  toolName,
		Input:     input,
		RiskLevel: riskLevel,
	}))
}

// RecordApprovalResult records an approval result.
func (l *Ledger) RecordApprovalResult(state agent.State, toolName string, approved bool, approver, reason string) {
	l.Append(NewEntry(EntryApprovalResult, l.runID, state, ApprovalResultDetails{
		ToolName: toolName,
		Approved: approved,
		Approver: approver,
		Reason:   reason,
	}))
}

// RecordBudgetConsumed records budget consumption.
func (l *Ledger) RecordBudgetConsumed(state agent.State, budgetName string, amount, remaining int) {
	l.Append(NewEntry(EntryBudgetConsumed, l.runID, state, BudgetDetails{
		BudgetName: budgetName,
		Amount:     amount,
		Remaining:  remaining,
	}))
}

// RecordBudgetExhausted records budget exhaustion.
func (l *Ledger) RecordBudgetExhausted(state agent.State, budgetName string) {
	l.Append(NewEntry(EntryBudgetExhausted, l.runID, state, BudgetDetails{
		BudgetName: budgetName,
		Remaining:  0,
	}))
}

// RecordHumanInputRequest records a request for human input.
func (l *Ledger) RecordHumanInputRequest(state agent.State, question string, options []string) {
	l.Append(NewEntry(EntryHumanInputRequest, l.runID, state, HumanInputRequestDetails{
		Question: question,
		Options:  options,
	}))
}

// RecordHumanInputResponse records a human input response.
func (l *Ledger) RecordHumanInputResponse(state agent.State, question, response string) {
	l.Append(NewEntry(EntryHumanInputResponse, l.runID, state, HumanInputResponseDetails{
		Question: question,
		Response: response,
	}))
}
