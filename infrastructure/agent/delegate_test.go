package agent_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/application"
	domainagent "go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
	infraagent "go.klarlabs.de/agent/infrastructure/agent"
	"go.klarlabs.de/agent/infrastructure/planner"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// buildChildEngine creates a child engine with a scripted planner for testing.
func buildChildEngine(t *testing.T, steps ...planner.ScriptStep) *application.Engine {
	t.Helper()

	registry := memory.NewToolRegistry()
	echoTool := tool.NewBuilder("echo").
		WithDescription("Echoes input").
		ReadOnly().
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.NewResult(input), nil
		}).
		MustBuild()
	if err := registry.Register(echoTool); err != nil {
		t.Fatalf("failed to register echo tool: %v", err)
	}

	eligibility := policy.NewDefaultToolEligibility()
	transitions := policy.DefaultTransitions()

	p := planner.NewScriptedPlanner(steps...)

	engine, err := application.NewEngine(application.EngineConfig{
		Registry:    registry,
		Planner:     p,
		Eligibility: eligibility,
		Transitions: transitions,
		MaxSteps:    50,
	})
	if err != nil {
		t.Fatalf("failed to create child engine: %v", err)
	}
	return engine
}

func TestDelegateTool_Interface(t *testing.T) {
	childEngine := buildChildEngine(t,
		planner.ScriptStep{
			ExpectState: domainagent.StateIntake,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateExplore, "start"),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateExplore,
			Decision:    domainagent.NewFinishDecision("done", json.RawMessage(`{"answer":"42"}`)),
		},
	)

	delegate := infraagent.NewDelegateTool("child_agent", "A child agent", childEngine)

	// Verify it implements tool.Tool.
	var _ tool.Tool = delegate

	if delegate.Name() != "child_agent" {
		t.Errorf("expected name 'child_agent', got %q", delegate.Name())
	}
	if delegate.Description() != "A child agent" {
		t.Errorf("expected description 'A child agent', got %q", delegate.Description())
	}

	annotations := delegate.Annotations()
	if !annotations.ReadOnly {
		t.Error("expected DelegateTool to be read-only")
	}
	if annotations.RiskLevel != tool.RiskLow {
		t.Errorf("expected default risk level RiskLow, got %v", annotations.RiskLevel)
	}

	inputSchema := delegate.InputSchema()
	if inputSchema.IsEmpty() {
		t.Error("expected non-empty input schema")
	}

	outputSchema := delegate.OutputSchema()
	if outputSchema.IsEmpty() {
		t.Error("expected non-empty output schema")
	}
}

func TestDelegateTool_Execute_Success(t *testing.T) {
	childResult := json.RawMessage(`{"answer":"42"}`)

	childEngine := buildChildEngine(t,
		planner.ScriptStep{
			ExpectState: domainagent.StateIntake,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateExplore, "start"),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateExplore,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateDecide, "gathered"),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateDecide,
			Decision:    domainagent.NewFinishDecision("done", childResult),
		},
	)

	delegate := infraagent.NewDelegateTool("child_agent", "A child agent", childEngine)

	input := json.RawMessage(`{"goal":"find the answer"}`)
	result, err := delegate.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out struct {
		RunID  string          `json:"run_id"`
		Status string          `json:"status"`
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.RunID == "" {
		t.Error("expected non-empty run_id")
	}
	if out.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", out.Status)
	}
	if string(out.Result) != string(childResult) {
		t.Errorf("expected result %s, got %s", childResult, out.Result)
	}
	if out.Error != "" {
		t.Errorf("expected no error, got %q", out.Error)
	}
}

func TestDelegateTool_Execute_ChildFailure(t *testing.T) {
	childEngine := buildChildEngine(t,
		planner.ScriptStep{
			ExpectState: domainagent.StateIntake,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateExplore, "start"),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateExplore,
			Decision:    domainagent.NewFailDecision("something went wrong", nil),
		},
	)

	delegate := infraagent.NewDelegateTool("child_agent", "A child agent", childEngine)

	input := json.RawMessage(`{"goal":"do something risky"}`)
	result, err := delegate.Execute(context.Background(), input)

	// The delegate tool returns the child's failure as structured output, not as an error.
	// The error in result indicates the child failed.
	if err != nil {
		t.Fatalf("unexpected Go-level error: %v", err)
	}

	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", out.Status)
	}
}

func TestDelegateTool_Execute_InvalidInput(t *testing.T) {
	childEngine := buildChildEngine(t,
		planner.ScriptStep{
			ExpectState: domainagent.StateIntake,
			Decision:    domainagent.NewFinishDecision("done", nil),
		},
	)

	delegate := infraagent.NewDelegateTool("child_agent", "A child agent", childEngine)

	// Test invalid JSON.
	_, err := delegate.Execute(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}

	// Test empty goal.
	_, err = delegate.Execute(context.Background(), json.RawMessage(`{"goal":""}`))
	if err == nil {
		t.Error("expected error for empty goal")
	}

	// Test missing goal field.
	_, err = delegate.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for missing goal")
	}
}

func TestDelegateTool_ContextCancellation(t *testing.T) {
	// Create a child engine with a tool that blocks until context is done.
	registry := memory.NewToolRegistry()
	slowTool := tool.NewBuilder("slow_tool").
		WithDescription("A slow tool").
		ReadOnly().
		WithHandler(func(ctx context.Context, _ json.RawMessage) (tool.Result, error) {
			select {
			case <-ctx.Done():
				return tool.Result{}, ctx.Err()
			case <-time.After(10 * time.Second):
				return tool.NewResult(json.RawMessage(`"done"`)), nil
			}
		}).
		MustBuild()
	if err := registry.Register(slowTool); err != nil {
		t.Fatalf("failed to register slow tool: %v", err)
	}

	steps := []planner.ScriptStep{
		{
			ExpectState: domainagent.StateIntake,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateExplore, "start"),
		},
		{
			ExpectState: domainagent.StateExplore,
			Decision:    domainagent.NewCallToolDecision("slow_tool", json.RawMessage(`{}`), "wait"),
		},
		{
			ExpectState: domainagent.StateExplore,
			Decision:    domainagent.NewFinishDecision("done", nil),
		},
	}

	p := planner.NewScriptedPlanner(steps...)
	childEngine, err := application.NewEngine(application.EngineConfig{
		Registry:    registry,
		Planner:     p,
		Eligibility: policy.NewDefaultToolEligibility(),
		Transitions: policy.DefaultTransitions(),
		MaxSteps:    50,
	})
	if err != nil {
		t.Fatalf("failed to create child engine: %v", err)
	}

	delegate := infraagent.NewDelegateTool("slow_child", "A slow child agent", childEngine)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	input := json.RawMessage(`{"goal":"do something slow"}`)
	result, err := delegate.Execute(ctx, input)

	// The delegate should propagate context cancellation.
	// Either we get an error or the result indicates failure.
	if err == nil && result.Error == nil {
		var out struct {
			Status string `json:"status"`
		}
		if unmarshalErr := json.Unmarshal(result.Output, &out); unmarshalErr == nil {
			if out.Status == "completed" {
				t.Error("expected child to not complete due to context cancellation")
			}
		}
	}
}

func TestDelegateTool_WithRiskLevel(t *testing.T) {
	childEngine := buildChildEngine(t,
		planner.ScriptStep{
			ExpectState: domainagent.StateIntake,
			Decision:    domainagent.NewFinishDecision("done", nil),
		},
	)

	delegate := infraagent.NewDelegateTool(
		"risky_child",
		"A risky child agent",
		childEngine,
		infraagent.WithRiskLevel(tool.RiskHigh),
	)

	if delegate.Annotations().RiskLevel != tool.RiskHigh {
		t.Errorf("expected risk level RiskHigh, got %v", delegate.Annotations().RiskLevel)
	}
}

func TestDelegateTool_ParentReceivesChildResult(t *testing.T) {
	// Build child engine that produces a specific result.
	childResult := json.RawMessage(`{"data":"from child"}`)
	childEngine := buildChildEngine(t,
		planner.ScriptStep{
			ExpectState: domainagent.StateIntake,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateExplore, "start"),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateExplore,
			Decision: domainagent.NewCallToolDecision(
				"echo",
				json.RawMessage(`{"msg":"hello"}`),
				"echo test",
			),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateExplore,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateDecide,
			Decision:    domainagent.NewFinishDecision("done", childResult),
		},
	)

	delegate := infraagent.NewDelegateTool("child_agent", "A child agent", childEngine)

	// Build parent engine with delegate tool.
	parentRegistry := memory.NewToolRegistry()
	if err := parentRegistry.Register(delegate); err != nil {
		t.Fatalf("failed to register delegate tool: %v", err)
	}

	parentResult := json.RawMessage(`{"summary":"parent done"}`)
	parentPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: domainagent.StateIntake,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateExplore, "start"),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateExplore,
			Decision: domainagent.NewCallToolDecision(
				"child_agent",
				json.RawMessage(`{"goal":"get child data"}`),
				"delegate to child",
			),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateExplore,
			Decision:    domainagent.NewTransitionDecision(domainagent.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: domainagent.StateDecide,
			Decision:    domainagent.NewFinishDecision("done", parentResult),
		},
	)

	parentEngine, err := application.NewEngine(application.EngineConfig{
		Registry:    parentRegistry,
		Planner:     parentPlanner,
		Eligibility: policy.NewDefaultToolEligibility(),
		Transitions: policy.DefaultTransitions(),
		MaxSteps:    50,
	})
	if err != nil {
		t.Fatalf("failed to create parent engine: %v", err)
	}

	run, err := parentEngine.Run(context.Background(), "delegate to child agent")
	if err != nil {
		t.Fatalf("parent run failed: %v", err)
	}

	if run.Status != domainagent.RunStatusCompleted {
		t.Fatalf("expected parent run completed, got %s (error: %s)", run.Status, run.Error)
	}

	// Verify the parent received child result as evidence.
	foundChildEvidence := false
	for _, ev := range run.Evidence {
		if ev.Source == "child_agent" {
			foundChildEvidence = true
			// The evidence should contain the delegate output with child result.
			var out struct {
				Status string          `json:"status"`
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(ev.Content, &out); err != nil {
				t.Fatalf("failed to unmarshal child evidence: %v", err)
			}
			if out.Status != "completed" {
				t.Errorf("expected child status 'completed', got %q", out.Status)
			}
			if string(out.Result) != string(childResult) {
				t.Errorf("expected child result %s, got %s", childResult, out.Result)
			}
		}
	}

	if !foundChildEvidence {
		t.Error("parent did not receive child agent evidence")
	}
}
