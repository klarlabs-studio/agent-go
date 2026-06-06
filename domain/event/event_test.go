package event_test

import (
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
)

func TestNewEvent(t *testing.T) {
	t.Parallel()

	t.Run("creates event with valid payload", func(t *testing.T) {
		t.Parallel()

		payload := event.RunStartedPayload{
			Goal: "process files",
			Vars: map[string]any{"key": "value"},
		}

		e, err := event.NewEvent("run-123", event.TypeRunStarted, payload)
		if err != nil {
			t.Fatalf("NewEvent() error = %v", err)
		}

		if e.RunID != "run-123" {
			t.Errorf("NewEvent() RunID = %s, want run-123", e.RunID)
		}
		if e.Type != event.TypeRunStarted {
			t.Errorf("NewEvent() Type = %s, want run.started", e.Type)
		}
		if e.Timestamp.IsZero() {
			t.Error("NewEvent() Timestamp should not be zero")
		}
		if e.Version != 1 {
			t.Errorf("NewEvent() Version = %d, want 1", e.Version)
		}
		if len(e.Payload) == 0 {
			t.Error("NewEvent() Payload should not be empty")
		}
	})

	t.Run("returns error for unmarshalable payload", func(t *testing.T) {
		t.Parallel()

		// channels cannot be marshaled to JSON
		payload := make(chan int)

		_, err := event.NewEvent("run-123", event.TypeRunStarted, payload)
		if err == nil {
			t.Error("NewEvent() should return error for unmarshalable payload")
		}
	})

	t.Run("handles nil payload", func(t *testing.T) {
		t.Parallel()

		e, err := event.NewEvent("run-123", event.TypeRunStarted, nil)
		if err != nil {
			t.Fatalf("NewEvent() error = %v", err)
		}
		if string(e.Payload) != "null" {
			t.Errorf("NewEvent() Payload = %s, want null", string(e.Payload))
		}
	})
}

func TestEvent_UnmarshalPayload(t *testing.T) {
	t.Parallel()

	t.Run("unmarshals payload to struct", func(t *testing.T) {
		t.Parallel()

		original := event.RunStartedPayload{
			Goal: "analyze data",
			Vars: map[string]any{"file": "test.txt"},
		}

		e, _ := event.NewEvent("run-123", event.TypeRunStarted, original)

		var decoded event.RunStartedPayload
		err := e.UnmarshalPayload(&decoded)
		if err != nil {
			t.Fatalf("UnmarshalPayload() error = %v", err)
		}

		if decoded.Goal != original.Goal {
			t.Errorf("UnmarshalPayload() Goal = %s, want %s", decoded.Goal, original.Goal)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		e := event.Event{
			Payload: json.RawMessage(`invalid json`),
		}

		var decoded event.RunStartedPayload
		err := e.UnmarshalPayload(&decoded)
		if err == nil {
			t.Error("UnmarshalPayload() should return error for invalid JSON")
		}
	})
}

func TestEventTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		eventType event.Type
		expected  string
	}{
		{event.TypeRunStarted, "run.started"},
		{event.TypeRunCompleted, "run.completed"},
		{event.TypeRunFailed, "run.failed"},
		{event.TypeRunPaused, "run.paused"},
		{event.TypeRunResumed, "run.resumed"},
		{event.TypeStateTransitioned, "state.transitioned"},
		{event.TypeToolCalled, "tool.called"},
		{event.TypeToolSucceeded, "tool.succeeded"},
		{event.TypeToolFailed, "tool.failed"},
		{event.TypeDecisionMade, "decision.made"},
		{event.TypeApprovalRequested, "approval.requested"},
		{event.TypeApprovalGranted, "approval.granted"},
		{event.TypeApprovalDenied, "approval.denied"},
		{event.TypeBudgetConsumed, "budget.consumed"},
		{event.TypeBudgetExhausted, "budget.exhausted"},
		{event.TypeEvidenceAdded, "evidence.added"},
		{event.TypeVariableSet, "variable.set"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()

			if string(tt.eventType) != tt.expected {
				t.Errorf("Event type = %s, want %s", tt.eventType, tt.expected)
			}
		})
	}
}

func TestPayloadTypes(t *testing.T) {
	t.Parallel()

	t.Run("RunStartedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.RunStartedPayload{
			Goal: "test goal",
			Vars: map[string]any{"key": "value"},
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.RunStartedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Goal != payload.Goal {
			t.Errorf("Goal = %s, want %s", decoded.Goal, payload.Goal)
		}
	})

	t.Run("RunCompletedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.RunCompletedPayload{
			Result:   json.RawMessage(`{"output":"success"}`),
			Duration: 5 * time.Second,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.RunCompletedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Duration != payload.Duration {
			t.Errorf("Duration = %v, want %v", decoded.Duration, payload.Duration)
		}
	})

	t.Run("RunFailedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.RunFailedPayload{
			Error:    "something went wrong",
			State:    agent.StateFailed,
			Duration: 3 * time.Second,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.RunFailedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Error != payload.Error {
			t.Errorf("Error = %s, want %s", decoded.Error, payload.Error)
		}
		if decoded.State != payload.State {
			t.Errorf("State = %s, want %s", decoded.State, payload.State)
		}
	})

	t.Run("StateTransitionedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.StateTransitionedPayload{
			FromState: agent.StateIntake,
			ToState:   agent.StateExplore,
			Reason:    "begin exploration",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.StateTransitionedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.FromState != payload.FromState {
			t.Errorf("FromState = %s, want %s", decoded.FromState, payload.FromState)
		}
		if decoded.ToState != payload.ToState {
			t.Errorf("ToState = %s, want %s", decoded.ToState, payload.ToState)
		}
	})

	t.Run("ToolCalledPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.ToolCalledPayload{
			ToolName: "read_file",
			Input:    json.RawMessage(`{"path":"/test"}`),
			State:    agent.StateExplore,
			Reason:   "gather info",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.ToolCalledPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.ToolName != payload.ToolName {
			t.Errorf("ToolName = %s, want %s", decoded.ToolName, payload.ToolName)
		}
	})

	t.Run("ToolSucceededPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.ToolSucceededPayload{
			ToolName: "read_file",
			Output:   json.RawMessage(`{"content":"hello"}`),
			Duration: 100 * time.Millisecond,
			Cached:   false,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.ToolSucceededPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Cached != payload.Cached {
			t.Errorf("Cached = %v, want %v", decoded.Cached, payload.Cached)
		}
	})

	t.Run("ToolFailedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.ToolFailedPayload{
			ToolName: "write_file",
			Error:    "permission denied",
			Duration: 50 * time.Millisecond,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.ToolFailedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Error != payload.Error {
			t.Errorf("Error = %s, want %s", decoded.Error, payload.Error)
		}
	})

	t.Run("DecisionMadePayload", func(t *testing.T) {
		t.Parallel()

		payload := event.DecisionMadePayload{
			DecisionType: "call_tool",
			ToolName:     "read_file",
			Reason:       "need info",
			Input:        json.RawMessage(`{}`),
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.DecisionMadePayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.DecisionType != payload.DecisionType {
			t.Errorf("DecisionType = %s, want %s", decoded.DecisionType, payload.DecisionType)
		}
	})

	t.Run("ApprovalRequestedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.ApprovalRequestedPayload{
			ToolName:  "delete_file",
			Input:     json.RawMessage(`{"path":"/tmp/test"}`),
			RiskLevel: "high",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.ApprovalRequestedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.RiskLevel != payload.RiskLevel {
			t.Errorf("RiskLevel = %s, want %s", decoded.RiskLevel, payload.RiskLevel)
		}
	})

	t.Run("ApprovalResultPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.ApprovalResultPayload{
			ToolName: "delete_file",
			Approver: "user@example.com",
			Reason:   "approved for cleanup",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.ApprovalResultPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Approver != payload.Approver {
			t.Errorf("Approver = %s, want %s", decoded.Approver, payload.Approver)
		}
	})

	t.Run("BudgetConsumedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.BudgetConsumedPayload{
			BudgetName: "tool_calls",
			Amount:     1,
			Remaining:  99,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.BudgetConsumedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Remaining != payload.Remaining {
			t.Errorf("Remaining = %d, want %d", decoded.Remaining, payload.Remaining)
		}
	})

	t.Run("BudgetExhaustedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.BudgetExhaustedPayload{
			BudgetName: "tool_calls",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.BudgetExhaustedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.BudgetName != payload.BudgetName {
			t.Errorf("BudgetName = %s, want %s", decoded.BudgetName, payload.BudgetName)
		}
	})

	t.Run("EvidenceAddedPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.EvidenceAddedPayload{
			Type:    "file_content",
			Source:  "read_file",
			Content: json.RawMessage(`{"data":"content"}`),
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.EvidenceAddedPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Source != payload.Source {
			t.Errorf("Source = %s, want %s", decoded.Source, payload.Source)
		}
	})

	t.Run("VariableSetPayload", func(t *testing.T) {
		t.Parallel()

		payload := event.VariableSetPayload{
			Key:   "counter",
			Value: 42,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var decoded event.VariableSetPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if decoded.Key != payload.Key {
			t.Errorf("Key = %s, want %s", decoded.Key, payload.Key)
		}
	})
}

func TestQueryOptions(t *testing.T) {
	t.Parallel()

	t.Run("zero value is valid", func(t *testing.T) {
		t.Parallel()

		opts := event.QueryOptions{}

		if opts.Limit != 0 {
			t.Errorf("QueryOptions zero Limit = %d, want 0", opts.Limit)
		}
		if opts.Offset != 0 {
			t.Errorf("QueryOptions zero Offset = %d, want 0", opts.Offset)
		}
		if len(opts.Types) != 0 {
			t.Errorf("QueryOptions zero Types len = %d, want 0", len(opts.Types))
		}
	})

	t.Run("can set query filters", func(t *testing.T) {
		t.Parallel()

		opts := event.QueryOptions{
			Types:    []event.Type{event.TypeRunStarted, event.TypeRunCompleted},
			FromTime: 1000,
			ToTime:   2000,
			Limit:    50,
			Offset:   10,
		}

		if len(opts.Types) != 2 {
			t.Errorf("QueryOptions Types len = %d, want 2", len(opts.Types))
		}
		if opts.FromTime != 1000 {
			t.Errorf("QueryOptions FromTime = %d, want 1000", opts.FromTime)
		}
		if opts.ToTime != 2000 {
			t.Errorf("QueryOptions ToTime = %d, want 2000", opts.ToTime)
		}
		if opts.Limit != 50 {
			t.Errorf("QueryOptions Limit = %d, want 50", opts.Limit)
		}
		if opts.Offset != 10 {
			t.Errorf("QueryOptions Offset = %d, want 10", opts.Offset)
		}
	})
}

func TestDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrEventNotFound",
			err:  event.ErrEventNotFound,
			msg:  "event not found",
		},
		{
			name: "ErrRunNotFound",
			err:  event.ErrRunNotFound,
			msg:  "run not found in event store",
		},
		{
			name: "ErrSequenceConflict",
			err:  event.ErrSequenceConflict,
			msg:  "event sequence conflict",
		},
		{
			name: "ErrInvalidEvent",
			err:  event.ErrInvalidEvent,
			msg:  "invalid event",
		},
		{
			name: "ErrSnapshotNotFound",
			err:  event.ErrSnapshotNotFound,
			msg:  "snapshot not found",
		},
		{
			name: "ErrConnectionFailed",
			err:  event.ErrConnectionFailed,
			msg:  "event store connection failed",
		},
		{
			name: "ErrOperationTimeout",
			err:  event.ErrOperationTimeout,
			msg:  "event store operation timeout",
		},
		{
			name: "ErrSubscriptionClosed",
			err:  event.ErrSubscriptionClosed,
			msg:  "event subscription closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err.Error() != tt.msg {
				t.Errorf("%s.Error() = %s, want %s", tt.name, tt.err.Error(), tt.msg)
			}
		})
	}
}
