package planner

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
)

func TestNewScriptedPlanner(t *testing.T) {
	t.Parallel()

	t.Run("empty steps", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner()
		if planner == nil {
			t.Fatal("NewScriptedPlanner() returned nil")
		}
		if !planner.IsComplete() {
			t.Error("Empty planner should be complete")
		}
	})

	t.Run("with steps", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
			ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewFinishDecision("done", nil)},
		)

		if planner.IsComplete() {
			t.Error("Planner with steps should not be complete initially")
		}
		if planner.CurrentStep() != 0 {
			t.Errorf("CurrentStep() = %d, want 0", planner.CurrentStep())
		}
	})
}

func TestScriptedPlanner_Plan(t *testing.T) {
	t.Parallel()

	t.Run("returns decisions when state matches", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
			ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "decide")},
			ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewFinishDecision("done", nil)},
		)

		ctx := context.Background()

		// Step 1: intake -> explore
		d1, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})
		if err != nil {
			t.Fatalf("Plan() step 1 error = %v", err)
		}
		if d1.Type != agent.DecisionTransition {
			t.Errorf("Step 1 decision type = %v, want transition", d1.Type)
		}
		if d1.Transition.ToState != agent.StateExplore {
			t.Errorf("Step 1 ToState = %v, want explore", d1.Transition.ToState)
		}

		// Step 2: explore -> decide
		d2, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateExplore})
		if err != nil {
			t.Fatalf("Plan() step 2 error = %v", err)
		}
		if d2.Transition.ToState != agent.StateDecide {
			t.Errorf("Step 2 ToState = %v, want decide", d2.Transition.ToState)
		}

		// Step 3: decide -> finish
		d3, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateDecide})
		if err != nil {
			t.Fatalf("Plan() step 3 error = %v", err)
		}
		if d3.Type != agent.DecisionFinish {
			t.Errorf("Step 3 decision type = %v, want finish", d3.Type)
		}

		// Planner should be complete
		if !planner.IsComplete() {
			t.Error("Planner should be complete after all steps")
		}
	})

	t.Run("returns error on unexpected state", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
		)

		ctx := context.Background()

		// Wrong state
		_, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateExplore})
		if err == nil {
			t.Fatal("Plan() should return error for unexpected state")
		}

		unexpectedErr, ok := err.(*UnexpectedStateError)
		if !ok {
			t.Fatalf("Error should be *UnexpectedStateError, got %T", err)
		}
		if unexpectedErr.Expected != agent.StateIntake {
			t.Errorf("Expected state = %v, want intake", unexpectedErr.Expected)
		}
		if unexpectedErr.Actual != agent.StateExplore {
			t.Errorf("Actual state = %v, want explore", unexpectedErr.Actual)
		}
	})

	t.Run("empty expect state matches any state", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{ExpectState: "", Decision: agent.NewFinishDecision("done", nil)},
		)

		ctx := context.Background()

		// Any state should match
		_, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateAct})
		if err != nil {
			t.Fatalf("Plan() error = %v, expected nil for empty ExpectState", err)
		}
	})
}

func TestScriptedPlanner_Plan_WithCondition(t *testing.T) {
	t.Parallel()

	t.Run("condition passes", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    agent.NewFinishDecision("done", nil),
				Condition: func(req PlanRequest) bool {
					return len(req.AllowedTools) > 0
				},
			},
		)

		ctx := context.Background()

		_, err := planner.Plan(ctx, PlanRequest{
			CurrentState: agent.StateExplore,
			AllowedTools: []string{"read_file"},
		})
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
	})

	t.Run("condition fails", func(t *testing.T) {
		t.Parallel()

		planner := NewScriptedPlanner(
			ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    agent.NewFinishDecision("done", nil),
				Condition: func(req PlanRequest) bool {
					return len(req.AllowedTools) > 0
				},
			},
		)

		ctx := context.Background()

		_, err := planner.Plan(ctx, PlanRequest{
			CurrentState: agent.StateExplore,
			AllowedTools: []string{}, // Empty - condition fails
		})
		if err == nil {
			t.Fatal("Plan() should return error when condition fails")
		}

		condErr, ok := err.(*ConditionFailedError)
		if !ok {
			t.Fatalf("Error should be *ConditionFailedError, got %T", err)
		}
		if condErr.State != agent.StateExplore {
			t.Errorf("State = %v, want explore", condErr.State)
		}
	})
}

func TestScriptedPlanner_OnUnexpected(t *testing.T) {
	t.Parallel()

	customHandler := func(req PlanRequest) agent.Decision {
		return agent.NewFinishDecision("custom finish", json.RawMessage(`{"custom": true}`))
	}

	planner := NewScriptedPlanner(
		ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
	).OnUnexpected(customHandler)

	ctx := context.Background()

	// Use up the single step
	_, _ = planner.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})

	// Next call should use custom handler
	d, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateExplore})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if d.Type != agent.DecisionFinish {
		t.Errorf("Decision type = %v, want finish", d.Type)
	}
	if d.Finish.Summary != "custom finish" {
		t.Errorf("Summary = %v, want 'custom finish'", d.Finish.Summary)
	}
}

func TestScriptedPlanner_Reset(t *testing.T) {
	t.Parallel()

	planner := NewScriptedPlanner(
		ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
		ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewFinishDecision("done", nil)},
	)

	ctx := context.Background()

	// Use first step
	_, _ = planner.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})
	if planner.CurrentStep() != 1 {
		t.Errorf("CurrentStep() after one Plan = %d, want 1", planner.CurrentStep())
	}

	// Reset
	planner.Reset()
	if planner.CurrentStep() != 0 {
		t.Errorf("CurrentStep() after Reset = %d, want 0", planner.CurrentStep())
	}

	// Can use first step again
	d, err := planner.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})
	if err != nil {
		t.Fatalf("Plan() after reset error = %v", err)
	}
	if d.Transition.ToState != agent.StateExplore {
		t.Errorf("ToState after reset = %v, want explore", d.Transition.ToState)
	}
}

func TestScriptedPlanner_Concurrency(t *testing.T) {
	t.Parallel()

	numSteps := 100
	steps := make([]ScriptStep, numSteps)
	for i := 0; i < numSteps; i++ {
		steps[i] = ScriptStep{
			ExpectState: "", // Match any state
			Decision:    agent.NewFinishDecision("done", nil),
		}
	}
	planner := NewScriptedPlanner(steps...)

	var wg sync.WaitGroup
	for i := 0; i < numSteps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = planner.Plan(context.Background(), PlanRequest{})
		}()
	}

	wg.Wait()

	if !planner.IsComplete() {
		t.Errorf("Planner should be complete after all concurrent calls, CurrentStep = %d", planner.CurrentStep())
	}
}

func TestUnexpectedStateError_Error(t *testing.T) {
	t.Parallel()

	err := &UnexpectedStateError{
		Expected:  agent.StateIntake,
		Actual:    agent.StateExplore,
		StepIndex: 0,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Error() should return non-empty string")
	}
}

func TestConditionFailedError_Error(t *testing.T) {
	t.Parallel()

	err := &ConditionFailedError{
		StepIndex: 1,
		State:     agent.StateExplore,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Error() should return non-empty string")
	}
}

// --- Enhanced ScriptedPlanner tests ---

func TestScriptedPlanner_SkipConditionalStep(t *testing.T) {
	t.Parallel()

	p := NewScriptedPlanner(
		ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("optional_tool", nil, "optional"),
			Skip:        true,
			Condition: func(req PlanRequest) bool {
				_, ok := req.Vars["need_tool"]
				return ok
			},
		},
		ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewFinishDecision("done", nil),
		},
	)

	ctx := context.Background()

	// Without the var, the first step should be skipped
	d, err := p.Plan(ctx, PlanRequest{
		CurrentState: agent.StateExplore,
		Vars:         map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionFinish {
		t.Errorf("should skip to finish, got %s", d.Type)
	}

	// Reset and try with the var set
	p.Reset()
	d, err = p.Plan(ctx, PlanRequest{
		CurrentState: agent.StateExplore,
		Vars:         map[string]any{"need_tool": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionCallTool {
		t.Errorf("should execute tool call, got %s", d.Type)
	}
}

func TestScriptedPlanner_ErrorInjection(t *testing.T) {
	t.Parallel()

	injectedErr := errors.New("simulated planner failure")

	p := NewScriptedPlanner(
		ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "begin"),
		},
		ScriptStep{
			ExpectState: agent.StateExplore,
			Error:       injectedErr,
		},
		ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewFinishDecision("reachable after error", nil),
		},
	)

	ctx := context.Background()

	// Step 1 - normal
	_, err := p.Plan(ctx, PlanRequest{CurrentState: agent.StateIntake})
	if err != nil {
		t.Fatalf("step 1: %v", err)
	}

	// Step 2 - error injection
	_, err = p.Plan(ctx, PlanRequest{CurrentState: agent.StateExplore})
	if err == nil {
		t.Fatal("expected injected error")
	}
	if !errors.Is(err, injectedErr) {
		t.Errorf("error = %v, want %v", err, injectedErr)
	}

	// Step 3 should still be reachable (error step was consumed)
	if p.CurrentStep() != 2 {
		t.Errorf("current step = %d, want 2", p.CurrentStep())
	}

	d, err := p.Plan(ctx, PlanRequest{CurrentState: agent.StateExplore})
	if err != nil {
		t.Fatalf("step 3: %v", err)
	}
	if d.Type != agent.DecisionFinish {
		t.Errorf("step 3: type = %s, want finish", d.Type)
	}
}

func TestScriptedPlanner_LoopCount(t *testing.T) {
	t.Parallel()

	p := NewScriptedPlanner(
		ScriptStep{
			Decision: agent.NewTransitionDecision(agent.StateExplore, "loop step"),
		},
	).WithLoop(3)

	ctx := context.Background()

	// Should execute the step 3 times
	for i := 0; i < 3; i++ {
		d, err := p.Plan(ctx, PlanRequest{})
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if d.Type != agent.DecisionTransition {
			t.Errorf("iteration %d: type = %s, want transition", i, d.Type)
		}
	}

	if p.LoopIteration() != 2 {
		t.Errorf("loop iteration = %d, want 2", p.LoopIteration())
	}

	// 4th call should hit onUnexpected
	d, err := p.Plan(ctx, PlanRequest{})
	if err != nil {
		t.Fatalf("post-loop: %v", err)
	}
	if d.Type != agent.DecisionFail {
		t.Errorf("post-loop: type = %s, want fail (onUnexpected)", d.Type)
	}
}

func TestScriptedPlanner_LoopUntil(t *testing.T) {
	iteration := 0

	p := NewScriptedPlanner(
		ScriptStep{
			Decision: agent.NewTransitionDecision(agent.StateExplore, "loop"),
		},
	).WithLoopUntil(func(_ PlanRequest) bool {
		iteration++
		return iteration >= 3 // Stop after 3 iterations
	})

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		d, err := p.Plan(ctx, PlanRequest{})
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if d.Type != agent.DecisionTransition {
			t.Errorf("iteration %d: type = %s, want transition", i, d.Type)
		}
	}

	// Next call should hit onUnexpected since loopUntil returned true
	d, err := p.Plan(ctx, PlanRequest{})
	if err != nil {
		t.Fatalf("post-loop: %v", err)
	}
	if d.Type != agent.DecisionFail {
		t.Errorf("post-loop: type = %s, want fail", d.Type)
	}
}

func TestScriptedPlanner_LoopMultipleSteps(t *testing.T) {
	t.Parallel()

	p := NewScriptedPlanner(
		ScriptStep{
			Decision: agent.NewTransitionDecision(agent.StateExplore, "step-1"),
		},
		ScriptStep{
			Decision: agent.NewTransitionDecision(agent.StateDecide, "step-2"),
		},
	).WithLoop(2)

	ctx := context.Background()

	// 4 total calls: 2 steps * 2 iterations
	expectedStates := []agent.State{
		agent.StateExplore, agent.StateDecide, // iteration 1
		agent.StateExplore, agent.StateDecide, // iteration 2
	}

	for i, expected := range expectedStates {
		d, err := p.Plan(ctx, PlanRequest{})
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if d.Transition.ToState != expected {
			t.Errorf("call %d: to_state = %s, want %s", i, d.Transition.ToState, expected)
		}
	}

	// 5th call should exhaust
	d, err := p.Plan(ctx, PlanRequest{})
	if err != nil {
		t.Fatalf("post-loop: %v", err)
	}
	if d.Type != agent.DecisionFail {
		t.Errorf("post-loop: type = %s, want fail", d.Type)
	}
}

func TestScriptedPlanner_SkipAllStepsInLoop(t *testing.T) {
	t.Parallel()

	// All steps are skippable, should exhaust after loop count
	p := NewScriptedPlanner(
		ScriptStep{
			Decision:  agent.NewTransitionDecision(agent.StateExplore, "skipped"),
			Skip:      true,
			Condition: func(_ PlanRequest) bool { return false },
		},
	).WithLoop(2)

	// Both iterations skip all steps, so we hit onUnexpected
	d, err := p.Plan(context.Background(), PlanRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionFail {
		t.Errorf("type = %s, want fail", d.Type)
	}
}
