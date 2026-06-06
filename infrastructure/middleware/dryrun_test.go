package middleware_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	domainmw "go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
	mw "go.klarlabs.de/agent/infrastructure/middleware"
)

func TestDryRun(t *testing.T) {
	t.Parallel()

	t.Run("passes through when disabled", func(t *testing.T) {
		t.Parallel()

		middleware := mw.DryRun(mw.WithDryRunEnabled(false))

		mockT := &mockTool{
			name:        "destructive_tool",
			annotations: tool.Annotations{Destructive: true},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"executed":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("allows read-only tools when enabled", func(t *testing.T) {
		t.Parallel()

		middleware := mw.DryRun(
			mw.WithDryRunEnabled(true),
			mw.WithAllowReadOnly(true),
		)

		mockT := &mockTool{
			name:        "read_tool",
			annotations: tool.Annotations{ReadOnly: true},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"read":"data"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("allows cacheable tools when enabled", func(t *testing.T) {
		t.Parallel()

		middleware := mw.DryRun(
			mw.WithDryRunEnabled(true),
			mw.WithAllowCacheable(true),
		)

		mockT := &mockTool{
			name:        "cacheable_tool",
			annotations: tool.Annotations{Cacheable: true},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"cached":"data"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("blocks destructive tools and returns dry-run response", func(t *testing.T) {
		t.Parallel()

		middleware := mw.DryRun(
			mw.WithDryRunEnabled(true),
			mw.WithAllowReadOnly(true),
		)

		mockT := &mockTool{
			name:        "delete_tool",
			annotations: tool.Annotations{Destructive: true, RiskLevel: tool.RiskHigh},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
			Input:        json.RawMessage(`{"file":"/important.txt"}`),
			Reason:       "cleanup",
		}

		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{"should":"not execute"}`)}, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should return dry-run response
		var response struct {
			DryRun  bool   `json:"dry_run"`
			Tool    string `json:"tool"`
			Skipped bool   `json:"skipped"`
		}
		if err := json.Unmarshal(result.Output, &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if !response.DryRun {
			t.Error("expected dry_run to be true")
		}
		if response.Tool != "delete_tool" {
			t.Errorf("expected tool 'delete_tool', got '%s'", response.Tool)
		}
		if !response.Skipped {
			t.Error("expected skipped to be true")
		}
	})

	t.Run("returns mock response when configured", func(t *testing.T) {
		t.Parallel()

		mockResponse := json.RawMessage(`{"mocked":"response"}`)
		middleware := mw.DryRun(
			mw.WithDryRunEnabled(true),
			mw.WithMockResponse("test_tool", mockResponse),
		)

		mockT := &mockTool{
			name:        "test_tool",
			annotations: tool.Annotations{},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(mockResponse) {
			t.Errorf("got output %s, want %s", result.Output, mockResponse)
		}
	})

	t.Run("calls callback when tool skipped", func(t *testing.T) {
		t.Parallel()

		callbackCalled := false
		middleware := mw.DryRun(
			mw.WithDryRunEnabled(true),
			mw.WithDryRunCallback(func(_ context.Context, _ *domainmw.ExecutionContext) {
				callbackCalled = true
			}),
		)

		mockT := &mockTool{
			name:        "tool",
			annotations: tool.Annotations{},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))
		handler(context.Background(), execCtx)

		if !callbackCalled {
			t.Error("expected callback to be called")
		}
	})

	t.Run("records operations when recorder configured", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewDryRunRecorder()
		middleware := mw.DryRun(
			mw.WithDryRunEnabled(true),
			mw.WithDryRunRecorder(recorder),
		)

		mockT := &mockTool{
			name:        "tracked_tool",
			annotations: tool.Annotations{},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
			Input:        json.RawMessage(`{"action":"test"}`),
			Reason:       "testing",
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))
		handler(context.Background(), execCtx)

		ops := recorder.Operations()
		if len(ops) != 1 {
			t.Fatalf("expected 1 operation, got %d", len(ops))
		}
		if ops[0].ToolName != "tracked_tool" {
			t.Errorf("expected tool name 'tracked_tool', got '%s'", ops[0].ToolName)
		}
		if ops[0].RunID != "run-123" {
			t.Errorf("expected run ID 'run-123', got '%s'", ops[0].RunID)
		}
	})
}

func TestDryRunRecorder(t *testing.T) {
	t.Parallel()

	t.Run("records operations", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewDryRunRecorder()

		recorder.Record(mw.DryRunOperation{
			RunID:    "run-1",
			State:    "act",
			ToolName: "tool1",
		})
		recorder.Record(mw.DryRunOperation{
			RunID:    "run-1",
			State:    "act",
			ToolName: "tool2",
		})

		ops := recorder.Operations()
		if len(ops) != 2 {
			t.Errorf("expected 2 operations, got %d", len(ops))
		}
	})

	t.Run("returns count", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewDryRunRecorder()

		recorder.Record(mw.DryRunOperation{ToolName: "tool1"})
		recorder.Record(mw.DryRunOperation{ToolName: "tool2"})
		recorder.Record(mw.DryRunOperation{ToolName: "tool3"})

		if recorder.Count() != 3 {
			t.Errorf("expected count 3, got %d", recorder.Count())
		}
	})

	t.Run("clears operations", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewDryRunRecorder()

		recorder.Record(mw.DryRunOperation{ToolName: "tool1"})
		recorder.Record(mw.DryRunOperation{ToolName: "tool2"})
		recorder.Clear()

		if recorder.Count() != 0 {
			t.Errorf("expected count 0 after clear, got %d", recorder.Count())
		}
	})

	t.Run("filters by tool", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewDryRunRecorder()

		recorder.Record(mw.DryRunOperation{ToolName: "tool1"})
		recorder.Record(mw.DryRunOperation{ToolName: "tool2"})
		recorder.Record(mw.DryRunOperation{ToolName: "tool1"})

		ops := recorder.OperationsByTool("tool1")
		if len(ops) != 2 {
			t.Errorf("expected 2 operations for tool1, got %d", len(ops))
		}
	})

	t.Run("filters by run", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewDryRunRecorder()

		recorder.Record(mw.DryRunOperation{RunID: "run-1", ToolName: "tool1"})
		recorder.Record(mw.DryRunOperation{RunID: "run-2", ToolName: "tool2"})
		recorder.Record(mw.DryRunOperation{RunID: "run-1", ToolName: "tool3"})

		ops := recorder.OperationsByRun("run-1")
		if len(ops) != 2 {
			t.Errorf("expected 2 operations for run-1, got %d", len(ops))
		}
	})

	t.Run("generates summary", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewDryRunRecorder()

		recorder.Record(mw.DryRunOperation{State: "act", ToolName: "tool1"})
		recorder.Record(mw.DryRunOperation{State: "act", ToolName: "tool2"})
		recorder.Record(mw.DryRunOperation{State: "validate", ToolName: "tool1"})

		summary := recorder.Summary()

		if summary.TotalOperations != 3 {
			t.Errorf("expected 3 total operations, got %d", summary.TotalOperations)
		}
		if summary.ByTool["tool1"] != 2 {
			t.Errorf("expected 2 operations for tool1, got %d", summary.ByTool["tool1"])
		}
		if summary.ByState["act"] != 2 {
			t.Errorf("expected 2 operations in act state, got %d", summary.ByState["act"])
		}
	})

	t.Run("generates report", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewDryRunRecorder()

		recorder.Record(mw.DryRunOperation{ToolName: "tool1"})

		report := recorder.GenerateReport()

		if report.Summary.TotalOperations != 1 {
			t.Errorf("expected 1 operation in report, got %d", report.Summary.TotalOperations)
		}
		if len(report.Operations) != 1 {
			t.Errorf("expected 1 operation in report details, got %d", len(report.Operations))
		}
	})
}

func TestDryRunSummary(t *testing.T) {
	t.Parallel()

	t.Run("serializes to JSON", func(t *testing.T) {
		t.Parallel()

		summary := mw.DryRunSummary{
			TotalOperations: 5,
			ByTool:          map[string]int{"tool1": 3, "tool2": 2},
			ByState:         map[string]int{"act": 4, "validate": 1},
		}

		data, err := summary.ToJSON()
		if err != nil {
			t.Fatalf("ToJSON failed: %v", err)
		}

		var parsed mw.DryRunSummary
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if parsed.TotalOperations != 5 {
			t.Errorf("expected 5 total operations, got %d", parsed.TotalOperations)
		}
	})
}

func TestConditionalDryRun(t *testing.T) {
	t.Parallel()

	t.Run("applies dry-run when condition is true", func(t *testing.T) {
		t.Parallel()

		middleware := mw.ConditionalDryRun(
			func(_ context.Context, execCtx *domainmw.ExecutionContext) bool {
				return execCtx.Tool.Annotations().Destructive
			},
		)

		mockT := &mockTool{
			name:        "destructive_tool",
			annotations: tool.Annotations{Destructive: true},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{"executed":true}`)}, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should be dry-run result
		var response struct {
			DryRun bool `json:"dry_run"`
		}
		if err := json.Unmarshal(result.Output, &response); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if !response.DryRun {
			t.Error("expected dry-run response")
		}
	})

	t.Run("passes through when condition is false", func(t *testing.T) {
		t.Parallel()

		middleware := mw.ConditionalDryRun(
			func(_ context.Context, execCtx *domainmw.ExecutionContext) bool {
				return execCtx.Tool.Annotations().Destructive
			},
		)

		mockT := &mockTool{
			name:        "safe_tool",
			annotations: tool.Annotations{ReadOnly: true},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"executed":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})
}

func TestContextAwareDryRun(t *testing.T) {
	t.Parallel()

	t.Run("applies dry-run when context has flag", func(t *testing.T) {
		t.Parallel()

		middleware := mw.ContextAwareDryRun()

		mockT := &mockTool{
			name:        "tool",
			annotations: tool.Annotations{},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		ctx := mw.ContextWithDryRun(context.Background(), true)
		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{"executed":true}`)}, nil))

		result, err := handler(ctx, execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should be dry-run result
		var response struct {
			DryRun bool `json:"dry_run"`
		}
		if err := json.Unmarshal(result.Output, &response); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if !response.DryRun {
			t.Error("expected dry-run response")
		}
	})

	t.Run("passes through when context flag is false", func(t *testing.T) {
		t.Parallel()

		middleware := mw.ContextAwareDryRun()

		mockT := &mockTool{
			name:        "tool",
			annotations: tool.Annotations{},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		ctx := mw.ContextWithDryRun(context.Background(), false)
		expected := tool.Result{Output: json.RawMessage(`{"executed":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(ctx, execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("passes through when no context flag", func(t *testing.T) {
		t.Parallel()

		middleware := mw.ContextAwareDryRun()

		mockT := &mockTool{
			name:        "tool",
			annotations: tool.Annotations{},
		}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"executed":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})
}

func TestDryRunFromContext(t *testing.T) {
	t.Parallel()

	t.Run("returns true when flag is set", func(t *testing.T) {
		t.Parallel()

		ctx := mw.ContextWithDryRun(context.Background(), true)
		if !mw.DryRunFromContext(ctx) {
			t.Error("expected DryRunFromContext to return true")
		}
	})

	t.Run("returns false when flag is not set", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		if mw.DryRunFromContext(ctx) {
			t.Error("expected DryRunFromContext to return false")
		}
	})

	t.Run("returns false when flag is explicitly false", func(t *testing.T) {
		t.Parallel()

		ctx := mw.ContextWithDryRun(context.Background(), false)
		if mw.DryRunFromContext(ctx) {
			t.Error("expected DryRunFromContext to return false")
		}
	})
}

func TestWithMockResponses(t *testing.T) {
	t.Parallel()

	responses := map[string]json.RawMessage{
		"tool1": json.RawMessage(`{"mock":"response1"}`),
		"tool2": json.RawMessage(`{"mock":"response2"}`),
	}

	middleware := mw.DryRun(
		mw.WithDryRunEnabled(true),
		mw.WithMockResponses(responses),
	)

	// Test tool1
	mockT := &mockTool{name: "tool1"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	handler := middleware(createTestHandler(tool.Result{}, nil))
	result, _ := handler(context.Background(), execCtx)

	if string(result.Output) != `{"mock":"response1"}` {
		t.Errorf("got output %s, want mock response", result.Output)
	}
}
