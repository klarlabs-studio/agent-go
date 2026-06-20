package test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/governance"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
	api "go.klarlabs.de/agent/interfaces/api"
)

// =============================================================================
// Full axi delegation (Track F): each agent run executes as ONE axi session,
// so budget, approval, and evidence all flow through the kernel. These tests
// drive the engine end-to-end with governance.NewKernelFactory and assert the
// externally-visible behaviour is preserved.
// =============================================================================

func kernelFactory(t *testing.T, approver policy.Approver) governance.Factory {
	t.Helper()
	f, err := governance.NewKernelFactory(approver)
	if err != nil {
		t.Fatalf("NewKernelFactory: %v", err)
	}
	return f
}

// The run's tool calls consume one axi session budget: the (limit+1)th tool
// call fails the run with the budget error.
func TestKernelDelegation_RunBudgetExhausts(t *testing.T) {
	readTool := api.NewToolBuilder("read_file").
		WithDescription("Reads a file").
		WithAnnotations(api.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}).
		MustBuild()

	registry := api.NewToolRegistry()
	if err := registry.Register(readTool); err != nil {
		t.Fatalf("register: %v", err)
	}
	eligibility := api.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "read_file")

	scripted := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: api.NewTransitionDecision(agent.StateExplore, "to explore")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second")},
	)

	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(scripted),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithBudgets(map[string]int{"tool_calls": 1}),
		api.WithGovernance(kernelFactory(t, nil)),
		api.WithMaxSteps(10),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = engine.Run(context.Background(), "budget exhausts")
	if err == nil {
		t.Fatal("expected budget-exceeded error, got nil")
	}
	if !errors.Is(err, policy.ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded, got: %v", err)
	}
}

// The run completes when tool calls stay within the run budget.
func TestKernelDelegation_RunBudgetWithinLimit(t *testing.T) {
	readTool := api.NewToolBuilder("read_file").
		WithDescription("Reads a file").
		WithAnnotations(api.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}).
		MustBuild()

	registry := api.NewToolRegistry()
	if err := registry.Register(readTool); err != nil {
		t.Fatalf("register: %v", err)
	}
	eligibility := api.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "read_file")

	scripted := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: api.NewTransitionDecision(agent.StateExplore, "to explore")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewTransitionDecision(agent.StateDecide, "to decide")},
		api.ScriptStep{ExpectState: agent.StateDecide, Decision: api.NewFinishDecision("done", json.RawMessage(`{}`))},
	)

	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(scripted),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithBudgets(map[string]int{"tool_calls": 5}),
		api.WithGovernance(kernelFactory(t, nil)),
		api.WithMaxSteps(10),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	run, err := engine.Run(context.Background(), "within budget")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if run.Status != agent.RunStatusCompleted {
		t.Fatalf("expected completed, got %s", run.Status)
	}
}

// Full delegation preserves the existing event types: a budget-exhausted run
// under the kernel governor still emits TypeBudgetExhausted, mapping axi's
// BUDGET_EXCEEDED back into agent-go's event surface.
func TestKernelDelegation_BudgetExhaustedEmitsEvent(t *testing.T) {
	readTool := api.NewToolBuilder("read_file").
		WithDescription("Reads a file").
		WithAnnotations(api.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}).
		MustBuild()

	registry := api.NewToolRegistry()
	if err := registry.Register(readTool); err != nil {
		t.Fatalf("register: %v", err)
	}
	eligibility := api.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "read_file")

	scripted := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: api.NewTransitionDecision(agent.StateExplore, "to explore")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second")},
	)

	store := memory.NewEventStore()
	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(scripted),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithBudgets(map[string]int{"tool_calls": 1}),
		api.WithGovernance(kernelFactory(t, nil)),
		api.WithEventStore(store),
		api.WithMaxSteps(10),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	run, runErr := engine.Run(ctx, "budget exhausts")
	if !errors.Is(runErr, policy.ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded, got: %v", runErr)
	}

	events, err := store.LoadEvents(ctx, run.ID)
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	var sawBudgetExhausted bool
	for _, e := range events {
		if e.Type == event.TypeBudgetExhausted {
			sawBudgetExhausted = true
		}
	}
	if !sawBudgetExhausted {
		t.Fatal("expected a budget.exhausted event under full axi delegation")
	}
}

// The approval gate still fires for destructive tools and lets an approved
// run complete.
func TestKernelDelegation_DestructiveToolApproved(t *testing.T) {
	deleteTool := api.NewToolBuilder("delete_file").
		WithDescription("Deletes a file").
		WithAnnotations(api.Annotations{Destructive: true, RiskLevel: api.RiskHigh}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"deleted": true}`)}, nil
		}).
		MustBuild()

	registry := api.NewToolRegistry()
	if err := registry.Register(deleteTool); err != nil {
		t.Fatalf("register: %v", err)
	}
	eligibility := api.NewToolEligibility()
	eligibility.Allow(agent.StateAct, "delete_file")

	scripted := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: api.NewTransitionDecision(agent.StateExplore, "to explore")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewTransitionDecision(agent.StateDecide, "to decide")},
		api.ScriptStep{ExpectState: agent.StateDecide, Decision: api.NewTransitionDecision(agent.StateAct, "to act")},
		api.ScriptStep{ExpectState: agent.StateAct, Decision: api.NewCallToolDecision("delete_file", json.RawMessage(`{}`), "delete")},
		api.ScriptStep{ExpectState: agent.StateAct, Decision: api.NewTransitionDecision(agent.StateValidate, "to validate")},
		api.ScriptStep{ExpectState: agent.StateValidate, Decision: api.NewFinishDecision("done", json.RawMessage(`{}`))},
	)

	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(scripted),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithApprover(api.NewAutoApprover("kernel-approver")),
		api.WithGovernance(kernelFactory(t, api.NewAutoApprover("kernel-approver"))),
		api.WithMaxSteps(20),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	run, err := engine.Run(context.Background(), "approved destructive")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if run.Status != agent.RunStatusCompleted {
		t.Fatalf("expected completed, got %s", run.Status)
	}
}

// Approval denial blocks a destructive tool and fails the run.
func TestKernelDelegation_DestructiveToolDenied(t *testing.T) {
	deleteTool := api.NewToolBuilder("delete_file").
		WithDescription("Deletes a file").
		WithAnnotations(api.Annotations{Destructive: true, RiskLevel: api.RiskHigh}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"deleted": true}`)}, nil
		}).
		MustBuild()

	registry := api.NewToolRegistry()
	if err := registry.Register(deleteTool); err != nil {
		t.Fatalf("register: %v", err)
	}
	eligibility := api.NewToolEligibility()
	eligibility.Allow(agent.StateAct, "delete_file")

	scripted := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: api.NewTransitionDecision(agent.StateExplore, "to explore")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: api.NewTransitionDecision(agent.StateDecide, "to decide")},
		api.ScriptStep{ExpectState: agent.StateDecide, Decision: api.NewTransitionDecision(agent.StateAct, "to act")},
		api.ScriptStep{ExpectState: agent.StateAct, Decision: api.NewCallToolDecision("delete_file", json.RawMessage(`{}`), "delete")},
	)

	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(scripted),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithApprover(api.NewDenyApprover("not allowed")),
		api.WithGovernance(kernelFactory(t, api.NewDenyApprover("not allowed"))),
		api.WithMaxSteps(10),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = engine.Run(context.Background(), "denied destructive")
	if err == nil {
		t.Fatal("expected approval-denied error, got nil")
	}
	if !errors.Is(err, tool.ErrApprovalDenied) {
		t.Fatalf("expected ErrApprovalDenied, got: %v", err)
	}
}
