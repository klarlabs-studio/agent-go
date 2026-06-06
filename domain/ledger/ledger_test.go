package ledger_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/ledger"
)

func TestNew(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-123")
	if l == nil {
		t.Fatal("New() returned nil")
	}
	if l.RunID() != "run-123" {
		t.Errorf("RunID() = %s, want run-123", l.RunID())
	}
	if l.Count() != 0 {
		t.Errorf("Count() = %d, want 0 for new ledger", l.Count())
	}
}

func TestLedger_Append(t *testing.T) {
	t.Parallel()

	t.Run("appends entry", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")

		entry := ledger.NewEntry(ledger.EntryRunStarted, "run-1", agent.StateIntake, map[string]string{"goal": "test"})
		l.Append(entry)

		if l.Count() != 1 {
			t.Errorf("Count() = %d, want 1", l.Count())
		}
	})

	t.Run("sets run ID on entry", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")

		entry := ledger.NewEntry(ledger.EntryRunStarted, "", agent.StateIntake, nil)
		l.Append(entry)

		entries := l.Entries()
		if entries[0].RunID != "run-1" {
			t.Errorf("Entry RunID = %s, want run-1", entries[0].RunID)
		}
	})

	t.Run("assigns ID if empty", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")

		entry := ledger.Entry{Type: ledger.EntryRunStarted}
		l.Append(entry)

		entries := l.Entries()
		if entries[0].ID == "" {
			t.Error("Entry should have ID assigned")
		}
	})

	t.Run("assigns timestamp if zero", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")

		entry := ledger.Entry{Type: ledger.EntryRunStarted}
		l.Append(entry)

		entries := l.Entries()
		if entries[0].Timestamp.IsZero() {
			t.Error("Entry should have timestamp assigned")
		}
	})
}

func TestLedger_Entries(t *testing.T) {
	t.Parallel()

	t.Run("returns copy of entries", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")
		l.Append(ledger.NewEntry(ledger.EntryRunStarted, "run-1", agent.StateIntake, nil))
		l.Append(ledger.NewEntry(ledger.EntryStateTransition, "run-1", agent.StateExplore, nil))

		entries := l.Entries()
		if len(entries) != 2 {
			t.Errorf("Entries() count = %d, want 2", len(entries))
		}
	})

	t.Run("returns empty slice for new ledger", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")
		entries := l.Entries()
		if len(entries) != 0 {
			t.Errorf("Entries() count = %d, want 0", len(entries))
		}
	})
}

func TestLedger_EntriesByType(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	l.Append(ledger.NewEntry(ledger.EntryRunStarted, "run-1", agent.StateIntake, nil))
	l.Append(ledger.NewEntry(ledger.EntryToolCall, "run-1", agent.StateExplore, nil))
	l.Append(ledger.NewEntry(ledger.EntryToolResult, "run-1", agent.StateExplore, nil))
	l.Append(ledger.NewEntry(ledger.EntryToolCall, "run-1", agent.StateExplore, nil))

	entries := l.EntriesByType(ledger.EntryToolCall)
	if len(entries) != 2 {
		t.Errorf("EntriesByType(ToolCall) count = %d, want 2", len(entries))
	}

	entries = l.EntriesByType(ledger.EntryRunStarted)
	if len(entries) != 1 {
		t.Errorf("EntriesByType(RunStarted) count = %d, want 1", len(entries))
	}

	entries = l.EntriesByType(ledger.EntryRunCompleted)
	if len(entries) != 0 {
		t.Errorf("EntriesByType(RunCompleted) count = %d, want 0", len(entries))
	}
}

func TestLedger_LastEntry(t *testing.T) {
	t.Parallel()

	t.Run("returns last entry", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")
		l.Append(ledger.NewEntry(ledger.EntryRunStarted, "run-1", agent.StateIntake, nil))
		l.Append(ledger.NewEntry(ledger.EntryStateTransition, "run-1", agent.StateExplore, nil))

		last := l.LastEntry()
		if last == nil {
			t.Fatal("LastEntry() returned nil")
		}
		if last.Type != ledger.EntryStateTransition {
			t.Errorf("LastEntry().Type = %s, want state_transition", last.Type)
		}
	})

	t.Run("returns nil for empty ledger", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")
		last := l.LastEntry()
		if last != nil {
			t.Error("LastEntry() should return nil for empty ledger")
		}
	})
}

func TestLedger_RecordRunStarted(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	l.RecordRunStarted("test goal")

	entries := l.EntriesByType(ledger.EntryRunStarted)
	if len(entries) != 1 {
		t.Fatalf("RecordRunStarted() should create 1 entry, got %d", len(entries))
	}

	var details map[string]string
	entries[0].DecodeDetails(&details)
	if details["goal"] != "test goal" {
		t.Errorf("RecordRunStarted() goal = %s, want 'test goal'", details["goal"])
	}
}

func TestLedger_RecordRunCompleted(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	result := json.RawMessage(`{"success": true}`)
	l.RecordRunCompleted(result)

	entries := l.EntriesByType(ledger.EntryRunCompleted)
	if len(entries) != 1 {
		t.Fatalf("RecordRunCompleted() should create 1 entry, got %d", len(entries))
	}
	if entries[0].State != agent.StateDone {
		t.Errorf("RecordRunCompleted() state = %s, want done", entries[0].State)
	}
}

func TestLedger_RecordRunFailed(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	l.RecordRunFailed(agent.StateFailed, "something went wrong")

	entries := l.EntriesByType(ledger.EntryRunFailed)
	if len(entries) != 1 {
		t.Fatalf("RecordRunFailed() should create 1 entry, got %d", len(entries))
	}
	if entries[0].State != agent.StateFailed {
		t.Errorf("RecordRunFailed() state = %s, want failed", entries[0].State)
	}
}

func TestLedger_RecordTransition(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	l.RecordTransition(agent.StateIntake, agent.StateExplore, "begin exploration")

	entries := l.EntriesByType(ledger.EntryStateTransition)
	if len(entries) != 1 {
		t.Fatalf("RecordTransition() should create 1 entry, got %d", len(entries))
	}

	var details ledger.TransitionDetails
	entries[0].DecodeDetails(&details)
	if details.FromState != agent.StateIntake {
		t.Errorf("RecordTransition() FromState = %s, want intake", details.FromState)
	}
	if details.ToState != agent.StateExplore {
		t.Errorf("RecordTransition() ToState = %s, want explore", details.ToState)
	}
	if details.Reason != "begin exploration" {
		t.Errorf("RecordTransition() Reason = %s, want 'begin exploration'", details.Reason)
	}
}

func TestLedger_RecordDecision(t *testing.T) {
	t.Parallel()

	t.Run("records CallTool decision", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")
		decision := agent.Decision{
			Type: agent.DecisionCallTool,
			CallTool: &agent.CallToolDecision{
				ToolName: "read_file",
				Input:    json.RawMessage(`{"path": "/test"}`),
				Reason:   "gather info",
			},
		}
		l.RecordDecision(agent.StateExplore, decision)

		entries := l.EntriesByType(ledger.EntryDecision)
		if len(entries) != 1 {
			t.Fatalf("RecordDecision() should create 1 entry, got %d", len(entries))
		}

		var details ledger.DecisionDetails
		entries[0].DecodeDetails(&details)
		if details.DecisionType != string(agent.DecisionCallTool) {
			t.Errorf("RecordDecision() DecisionType = %s, want call_tool", details.DecisionType)
		}
		if details.ToolName != "read_file" {
			t.Errorf("RecordDecision() ToolName = %s, want read_file", details.ToolName)
		}
	})

	t.Run("records Transition decision", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")
		decision := agent.Decision{
			Type: agent.DecisionTransition,
			Transition: &agent.TransitionDecision{
				ToState: agent.StateDecide,
				Reason:  "need to decide",
			},
		}
		l.RecordDecision(agent.StateExplore, decision)

		entries := l.EntriesByType(ledger.EntryDecision)
		var details ledger.DecisionDetails
		entries[0].DecodeDetails(&details)
		if details.ToState != agent.StateDecide {
			t.Errorf("RecordDecision() ToState = %s, want decide", details.ToState)
		}
	})

	t.Run("records Finish decision", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")
		decision := agent.Decision{
			Type: agent.DecisionFinish,
			Finish: &agent.FinishDecision{
				Summary: "completed successfully",
			},
		}
		l.RecordDecision(agent.StateValidate, decision)

		entries := l.EntriesByType(ledger.EntryDecision)
		var details ledger.DecisionDetails
		entries[0].DecodeDetails(&details)
		if details.Reason != "completed successfully" {
			t.Errorf("RecordDecision() Reason = %s, want 'completed successfully'", details.Reason)
		}
	})

	t.Run("records Fail decision", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("run-1")
		decision := agent.Decision{
			Type: agent.DecisionFail,
			Fail: &agent.FailDecision{
				Reason: "something went wrong",
			},
		}
		l.RecordDecision(agent.StateExplore, decision)

		entries := l.EntriesByType(ledger.EntryDecision)
		var details ledger.DecisionDetails
		entries[0].DecodeDetails(&details)
		if details.Reason != "something went wrong" {
			t.Errorf("RecordDecision() Reason = %s, want 'something went wrong'", details.Reason)
		}
	})
}

func TestLedger_RecordToolCall(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	input := json.RawMessage(`{"path": "/test"}`)
	l.RecordToolCall(agent.StateAct, "read_file", input)

	entries := l.EntriesByType(ledger.EntryToolCall)
	if len(entries) != 1 {
		t.Fatalf("RecordToolCall() should create 1 entry, got %d", len(entries))
	}

	var details ledger.ToolCallDetails
	entries[0].DecodeDetails(&details)
	if details.ToolName != "read_file" {
		t.Errorf("RecordToolCall() ToolName = %s, want read_file", details.ToolName)
	}
}

func TestLedger_RecordToolResult(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	output := json.RawMessage(`{"content": "data"}`)
	l.RecordToolResult(agent.StateAct, "read_file", output, 100*time.Millisecond, true)

	entries := l.EntriesByType(ledger.EntryToolResult)
	if len(entries) != 1 {
		t.Fatalf("RecordToolResult() should create 1 entry, got %d", len(entries))
	}

	var details ledger.ToolResultDetails
	entries[0].DecodeDetails(&details)
	if details.ToolName != "read_file" {
		t.Errorf("RecordToolResult() ToolName = %s, want read_file", details.ToolName)
	}
	if !details.Cached {
		t.Error("RecordToolResult() Cached = false, want true")
	}
}

func TestLedger_RecordToolError(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	l.RecordToolError(agent.StateAct, "read_file", errors.New("file not found"))

	entries := l.EntriesByType(ledger.EntryToolError)
	if len(entries) != 1 {
		t.Fatalf("RecordToolError() should create 1 entry, got %d", len(entries))
	}

	var details ledger.ToolErrorDetails
	entries[0].DecodeDetails(&details)
	if details.Error != "file not found" {
		t.Errorf("RecordToolError() Error = %s, want 'file not found'", details.Error)
	}
}

func TestLedger_RecordApprovalRequest(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	input := json.RawMessage(`{"path": "/etc/passwd"}`)
	l.RecordApprovalRequest(agent.StateAct, "delete_file", input, "high")

	entries := l.EntriesByType(ledger.EntryApprovalRequest)
	if len(entries) != 1 {
		t.Fatalf("RecordApprovalRequest() should create 1 entry, got %d", len(entries))
	}

	var details ledger.ApprovalRequestDetails
	entries[0].DecodeDetails(&details)
	if details.RiskLevel != "high" {
		t.Errorf("RecordApprovalRequest() RiskLevel = %s, want high", details.RiskLevel)
	}
}

func TestLedger_RecordApprovalResult(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	l.RecordApprovalResult(agent.StateAct, "delete_file", false, "admin", "too risky")

	entries := l.EntriesByType(ledger.EntryApprovalResult)
	if len(entries) != 1 {
		t.Fatalf("RecordApprovalResult() should create 1 entry, got %d", len(entries))
	}

	var details ledger.ApprovalResultDetails
	entries[0].DecodeDetails(&details)
	if details.Approved {
		t.Error("RecordApprovalResult() Approved = true, want false")
	}
	if details.Approver != "admin" {
		t.Errorf("RecordApprovalResult() Approver = %s, want admin", details.Approver)
	}
}

func TestLedger_RecordBudgetConsumed(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	l.RecordBudgetConsumed(agent.StateAct, "tool_calls", 1, 49)

	entries := l.EntriesByType(ledger.EntryBudgetConsumed)
	if len(entries) != 1 {
		t.Fatalf("RecordBudgetConsumed() should create 1 entry, got %d", len(entries))
	}

	var details ledger.BudgetDetails
	entries[0].DecodeDetails(&details)
	if details.BudgetName != "tool_calls" {
		t.Errorf("RecordBudgetConsumed() BudgetName = %s, want tool_calls", details.BudgetName)
	}
	if details.Remaining != 49 {
		t.Errorf("RecordBudgetConsumed() Remaining = %d, want 49", details.Remaining)
	}
}

func TestLedger_RecordBudgetExhausted(t *testing.T) {
	t.Parallel()

	l := ledger.New("run-1")
	l.RecordBudgetExhausted(agent.StateAct, "tool_calls")

	entries := l.EntriesByType(ledger.EntryBudgetExhausted)
	if len(entries) != 1 {
		t.Fatalf("RecordBudgetExhausted() should create 1 entry, got %d", len(entries))
	}

	var details ledger.BudgetDetails
	entries[0].DecodeDetails(&details)
	if details.Remaining != 0 {
		t.Errorf("RecordBudgetExhausted() Remaining = %d, want 0", details.Remaining)
	}
}

func TestEntry_DecodeDetails(t *testing.T) {
	t.Parallel()

	t.Run("decodes details into struct", func(t *testing.T) {
		t.Parallel()

		entry := ledger.NewEntry(ledger.EntryStateTransition, "run-1", agent.StateExplore, ledger.TransitionDetails{
			FromState: agent.StateIntake,
			ToState:   agent.StateExplore,
			Reason:    "test",
		})

		var details ledger.TransitionDetails
		err := entry.DecodeDetails(&details)
		if err != nil {
			t.Fatalf("DecodeDetails() error = %v", err)
		}
		if details.FromState != agent.StateIntake {
			t.Errorf("DecodeDetails() FromState = %s, want intake", details.FromState)
		}
	})

	t.Run("returns nil for nil details", func(t *testing.T) {
		t.Parallel()

		entry := ledger.Entry{Details: nil}
		var details map[string]any
		err := entry.DecodeDetails(&details)
		if err != nil {
			t.Errorf("DecodeDetails() error = %v, want nil", err)
		}
	})
}

// Test events

func TestNewRunStartedEvent(t *testing.T) {
	t.Parallel()

	event := ledger.NewRunStartedEvent("run-1", "test goal")

	if event.EventType() != "run.started" {
		t.Errorf("EventType() = %s, want run.started", event.EventType())
	}
	if event.RunID() != "run-1" {
		t.Errorf("RunID() = %s, want run-1", event.RunID())
	}
	if event.Goal != "test goal" {
		t.Errorf("Goal = %s, want 'test goal'", event.Goal)
	}
	if event.Timestamp().IsZero() {
		t.Error("Timestamp() should not be zero")
	}
}

func TestNewRunCompletedEvent(t *testing.T) {
	t.Parallel()

	event := ledger.NewRunCompletedEvent("run-1", "done", 5*time.Second)

	if event.EventType() != "run.completed" {
		t.Errorf("EventType() = %s, want run.completed", event.EventType())
	}
	if event.Summary != "done" {
		t.Errorf("Summary = %s, want done", event.Summary)
	}
	if event.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", event.Duration)
	}
}

func TestNewRunFailedEvent(t *testing.T) {
	t.Parallel()

	event := ledger.NewRunFailedEvent("run-1", "error occurred", agent.StateFailed, 3*time.Second)

	if event.EventType() != "run.failed" {
		t.Errorf("EventType() = %s, want run.failed", event.EventType())
	}
	if event.Reason != "error occurred" {
		t.Errorf("Reason = %s, want 'error occurred'", event.Reason)
	}
}

func TestNewStateChangedEvent(t *testing.T) {
	t.Parallel()

	event := ledger.NewStateChangedEvent("run-1", agent.StateIntake, agent.StateExplore, "begin")

	if event.EventType() != "state.changed" {
		t.Errorf("EventType() = %s, want state.changed", event.EventType())
	}
	if event.FromState != agent.StateIntake {
		t.Errorf("FromState = %s, want intake", event.FromState)
	}
	if event.ToState != agent.StateExplore {
		t.Errorf("ToState = %s, want explore", event.ToState)
	}
}

func TestNewToolExecutedEvent(t *testing.T) {
	t.Parallel()

	event := ledger.NewToolExecutedEvent("run-1", agent.StateAct, "read_file", 100*time.Millisecond, true, "")

	if event.EventType() != "tool.executed" {
		t.Errorf("EventType() = %s, want tool.executed", event.EventType())
	}
	if event.ToolName != "read_file" {
		t.Errorf("ToolName = %s, want read_file", event.ToolName)
	}
	if !event.Success {
		t.Error("Success = false, want true")
	}
}

func TestNoOpPublisher(t *testing.T) {
	t.Parallel()

	publisher := ledger.NoOpPublisher{}
	event := ledger.NewRunStartedEvent("run-1", "test")

	err := publisher.Publish(event)
	if err != nil {
		t.Errorf("Publish() error = %v, want nil", err)
	}
}
