package api_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/tool"
	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewToolBuilder(t *testing.T) {
	t.Parallel()

	builder := api.NewToolBuilder("test_tool")
	if builder == nil {
		t.Fatal("NewToolBuilder() returned nil")
	}

	built := builder.
		WithDescription("Test tool").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	if built.Name() != "test_tool" {
		t.Errorf("Name() = %v, want test_tool", built.Name())
	}
	if built.Description() != "Test tool" {
		t.Errorf("Description() = %v, want 'Test tool'", built.Description())
	}
}

func TestNewToolRegistry(t *testing.T) {
	t.Parallel()

	registry := api.NewToolRegistry()
	if registry == nil {
		t.Fatal("NewToolRegistry() returned nil")
	}

	// Register a tool
	testTool := api.NewToolBuilder("test").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, nil
		}).
		MustBuild()

	registry.Register(testTool)

	// Retrieve it
	retrieved, ok := registry.Get("test")
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if retrieved.Name() != "test" {
		t.Errorf("Retrieved tool name = %v, want test", retrieved.Name())
	}
}

func TestNewMockPlanner(t *testing.T) {
	t.Parallel()

	decisions := []api.Decision{
		api.NewTransitionDecision(api.StateExplore, "start"),
		api.NewFinishDecision("done", nil),
	}

	planner := api.NewMockPlanner(decisions...)
	if planner == nil {
		t.Fatal("NewMockPlanner() returned nil")
	}
	if planner.Remaining() != 2 {
		t.Errorf("Remaining() = %d, want 2", planner.Remaining())
	}
}

func TestNewScriptedPlanner(t *testing.T) {
	t.Parallel()

	planner := api.NewScriptedPlanner(
		api.ScriptStep{
			ExpectState: api.StateIntake,
			Decision:    api.NewTransitionDecision(api.StateExplore, "start"),
		},
	)
	if planner == nil {
		t.Fatal("NewScriptedPlanner() returned nil")
	}
	if planner.IsComplete() {
		t.Error("Fresh planner should not be complete")
	}
}

func TestNewToolEligibility(t *testing.T) {
	t.Parallel()

	eligibility := api.NewToolEligibility()
	if eligibility == nil {
		t.Fatal("NewToolEligibility() returned nil")
	}

	// Initially no tools are allowed
	if eligibility.IsAllowed(api.StateExplore, "some_tool") {
		t.Error("No tools should be allowed initially")
	}

	// Allow a tool
	eligibility.Allow(api.StateExplore, "read_file")
	if !eligibility.IsAllowed(api.StateExplore, "read_file") {
		t.Error("read_file should be allowed in explore state")
	}
	if eligibility.IsAllowed(api.StateAct, "read_file") {
		t.Error("read_file should not be allowed in act state")
	}
}

func TestNewStateTransitions(t *testing.T) {
	t.Parallel()

	transitions := api.NewStateTransitions()
	if transitions == nil {
		t.Fatal("NewStateTransitions() returned nil")
	}

	// Initially no transitions allowed
	if transitions.CanTransition(api.StateIntake, api.StateExplore) {
		t.Error("No transitions should be allowed initially")
	}

	// Allow a transition
	transitions.Allow(api.StateIntake, api.StateExplore)
	if !transitions.CanTransition(api.StateIntake, api.StateExplore) {
		t.Error("intake -> explore should be allowed")
	}
}

func TestDefaultTransitions(t *testing.T) {
	t.Parallel()

	transitions := api.DefaultTransitions()
	if transitions == nil {
		t.Fatal("DefaultTransitions() returned nil")
	}

	// Test canonical transitions
	expectedTransitions := []struct {
		from, to api.State
		allowed  bool
	}{
		{api.StateIntake, api.StateExplore, true},
		{api.StateIntake, api.StateFailed, true},
		{api.StateExplore, api.StateDecide, true},
		{api.StateExplore, api.StateFailed, true},
		{api.StateDecide, api.StateAct, true},
		{api.StateDecide, api.StateDone, true},
		{api.StateDecide, api.StateFailed, true},
		{api.StateAct, api.StateValidate, true},
		{api.StateAct, api.StateFailed, true},
		{api.StateValidate, api.StateDone, true},
		{api.StateValidate, api.StateExplore, true},
		{api.StateValidate, api.StateFailed, true},
		// Not allowed
		{api.StateExplore, api.StateDone, false},
		{api.StateIntake, api.StateDone, false},
		{api.StateDone, api.StateIntake, false},
	}

	for _, tt := range expectedTransitions {
		got := transitions.CanTransition(tt.from, tt.to)
		if got != tt.allowed {
			t.Errorf("CanTransition(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.allowed)
		}
	}
}

func TestNewAutoApprover(t *testing.T) {
	t.Parallel()

	approver := api.NewAutoApprover("test-approver")
	if approver == nil {
		t.Fatal("NewAutoApprover() returned nil")
	}
}

func TestNewDenyApprover(t *testing.T) {
	t.Parallel()

	approver := api.NewDenyApprover("not allowed")
	if approver == nil {
		t.Fatal("NewDenyApprover() returned nil")
	}
}

func TestAutoApprover(t *testing.T) {
	t.Parallel()

	approver := api.AutoApprover()
	if approver == nil {
		t.Fatal("AutoApprover() returned nil")
	}
}

func TestDenyApprover(t *testing.T) {
	t.Parallel()

	approver := api.DenyApprover("testing")
	if approver == nil {
		t.Fatal("DenyApprover() returned nil")
	}
}

func TestDecisionConstructors(t *testing.T) {
	t.Parallel()

	t.Run("NewCallToolDecision", func(t *testing.T) {
		t.Parallel()

		d := api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "reading")
		if d.Type != agent.DecisionCallTool {
			t.Errorf("Type = %v, want call_tool", d.Type)
		}
		if d.CallTool == nil {
			t.Fatal("CallTool is nil")
		}
		if d.CallTool.ToolName != "read_file" {
			t.Errorf("ToolName = %v, want read_file", d.CallTool.ToolName)
		}
	})

	t.Run("NewTransitionDecision", func(t *testing.T) {
		t.Parallel()

		d := api.NewTransitionDecision(api.StateExplore, "exploring")
		if d.Type != agent.DecisionTransition {
			t.Errorf("Type = %v, want transition", d.Type)
		}
		if d.Transition == nil {
			t.Fatal("Transition is nil")
		}
		if d.Transition.ToState != api.StateExplore {
			t.Errorf("ToState = %v, want explore", d.Transition.ToState)
		}
	})

	t.Run("NewFinishDecision", func(t *testing.T) {
		t.Parallel()

		d := api.NewFinishDecision("completed", json.RawMessage(`{"status": "ok"}`))
		if d.Type != agent.DecisionFinish {
			t.Errorf("Type = %v, want finish", d.Type)
		}
		if d.Finish == nil {
			t.Fatal("Finish is nil")
		}
		if d.Finish.Summary != "completed" {
			t.Errorf("Summary = %v, want completed", d.Finish.Summary)
		}
	})

	t.Run("NewFailDecision", func(t *testing.T) {
		t.Parallel()

		d := api.NewFailDecision("failed", nil)
		if d.Type != agent.DecisionFail {
			t.Errorf("Type = %v, want fail", d.Type)
		}
		if d.Fail == nil {
			t.Fatal("Fail is nil")
		}
		if d.Fail.Reason != "failed" {
			t.Errorf("Reason = %v, want failed", d.Fail.Reason)
		}
	})
}

func TestNewCallbackApprover(t *testing.T) {
	t.Parallel()

	t.Run("approves when callback returns true", func(t *testing.T) {
		t.Parallel()

		approver := api.NewCallbackApprover(func(ctx context.Context, req api.ApprovalRequest) (bool, error) {
			return true, nil
		})

		resp, err := approver.Approve(context.Background(), api.ApprovalRequest{
			ToolName: "test_tool",
		})
		if err != nil {
			t.Fatalf("Approve() error = %v", err)
		}
		if !resp.Approved {
			t.Error("Expected approval")
		}
		if resp.Approver != "callback" {
			t.Errorf("Approver = %v, want callback", resp.Approver)
		}
		if resp.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	})

	t.Run("denies when callback returns false", func(t *testing.T) {
		t.Parallel()

		approver := api.NewCallbackApprover(func(ctx context.Context, req api.ApprovalRequest) (bool, error) {
			return false, nil
		})

		resp, err := approver.Approve(context.Background(), api.ApprovalRequest{
			ToolName: "test_tool",
		})
		if err != nil {
			t.Fatalf("Approve() error = %v", err)
		}
		if resp.Approved {
			t.Error("Expected denial")
		}
	})

	t.Run("passes through callback error", func(t *testing.T) {
		t.Parallel()

		expectedErr := context.DeadlineExceeded
		approver := api.NewCallbackApprover(func(ctx context.Context, req api.ApprovalRequest) (bool, error) {
			return false, expectedErr
		})

		_, err := approver.Approve(context.Background(), api.ApprovalRequest{})
		if err != expectedErr {
			t.Errorf("Error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("timestamp is reasonable", func(t *testing.T) {
		t.Parallel()

		before := time.Now()

		approver := api.NewCallbackApprover(func(ctx context.Context, req api.ApprovalRequest) (bool, error) {
			return true, nil
		})

		resp, err := approver.Approve(context.Background(), api.ApprovalRequest{})
		if err != nil {
			t.Fatalf("Approve() error = %v", err)
		}

		after := time.Now()

		if resp.Timestamp.Before(before) || resp.Timestamp.After(after) {
			t.Errorf("Timestamp %v is not between %v and %v", resp.Timestamp, before, after)
		}
	})
}

func TestStateConstants(t *testing.T) {
	t.Parallel()

	// Verify state constants are properly re-exported
	states := []api.State{
		api.StateIntake,
		api.StateExplore,
		api.StateDecide,
		api.StateAct,
		api.StateValidate,
		api.StateDone,
		api.StateFailed,
	}

	for _, s := range states {
		if s == "" {
			t.Error("State constant is empty")
		}
		if !s.IsValid() {
			t.Errorf("State %v should be valid", s)
		}
	}
}

func TestRiskLevelConstants(t *testing.T) {
	t.Parallel()

	// Verify risk levels are properly re-exported
	levels := []api.RiskLevel{
		api.RiskNone,
		api.RiskLow,
		api.RiskMedium,
		api.RiskHigh,
		api.RiskCritical,
	}

	for i, level := range levels {
		if int(level) != i {
			t.Errorf("RiskLevel %v has unexpected value %d, want %d", level, level, i)
		}
	}
}

func TestRunStatusConstants(t *testing.T) {
	t.Parallel()

	// Verify run status constants are properly re-exported
	statuses := []api.RunStatus{
		api.StatusPending,
		api.StatusRunning,
		api.StatusPaused,
		api.StatusCompleted,
		api.StatusFailed,
	}

	seen := make(map[api.RunStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("Duplicate status: %v", s)
		}
		seen[s] = true
	}
}
