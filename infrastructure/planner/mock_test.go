package planner

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
)

func TestNewMockPlanner(t *testing.T) {
	t.Parallel()

	t.Run("empty decisions", func(t *testing.T) {
		t.Parallel()

		planner := NewMockPlanner()
		if planner == nil {
			t.Fatal("NewMockPlanner() returned nil")
		}
		if planner.Remaining() != 0 {
			t.Errorf("Remaining() = %d, want 0", planner.Remaining())
		}
	})

	t.Run("with decisions", func(t *testing.T) {
		t.Parallel()

		decisions := []agent.Decision{
			agent.NewTransitionDecision(agent.StateExplore, "explore"),
			agent.NewFinishDecision("done", nil),
		}
		planner := NewMockPlanner(decisions...)

		if planner.Remaining() != 2 {
			t.Errorf("Remaining() = %d, want 2", planner.Remaining())
		}
	})
}

func TestMockPlanner_Plan(t *testing.T) {
	t.Parallel()

	t.Run("returns decisions in order", func(t *testing.T) {
		t.Parallel()

		decisions := []agent.Decision{
			agent.NewTransitionDecision(agent.StateExplore, "explore"),
			agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "read"),
			agent.NewFinishDecision("done", nil),
		}
		planner := NewMockPlanner(decisions...)

		ctx := context.Background()
		req := PlanRequest{CurrentState: agent.StateIntake}

		// First decision
		d1, err := planner.Plan(ctx, req)
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if d1.Type != agent.DecisionTransition {
			t.Errorf("First decision type = %v, want transition", d1.Type)
		}

		// Second decision
		d2, err := planner.Plan(ctx, req)
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if d2.Type != agent.DecisionCallTool {
			t.Errorf("Second decision type = %v, want call_tool", d2.Type)
		}

		// Third decision
		d3, err := planner.Plan(ctx, req)
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if d3.Type != agent.DecisionFinish {
			t.Errorf("Third decision type = %v, want finish", d3.Type)
		}
	})

	t.Run("returns finish when exhausted", func(t *testing.T) {
		t.Parallel()

		planner := NewMockPlanner(
			agent.NewTransitionDecision(agent.StateExplore, "explore"),
		)

		ctx := context.Background()
		req := PlanRequest{}

		// Use up the single decision
		_, _ = planner.Plan(ctx, req)

		// Next call should return default finish
		d, err := planner.Plan(ctx, req)
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if d.Type != agent.DecisionFinish {
			t.Errorf("Decision type = %v, want finish", d.Type)
		}
		if d.Finish.Summary != "completed" {
			t.Errorf("Summary = %v, want 'completed'", d.Finish.Summary)
		}
	})
}

func TestMockPlanner_Reset(t *testing.T) {
	t.Parallel()

	decisions := []agent.Decision{
		agent.NewTransitionDecision(agent.StateExplore, "explore"),
		agent.NewFinishDecision("done", nil),
	}
	planner := NewMockPlanner(decisions...)

	ctx := context.Background()
	req := PlanRequest{}

	// Use first decision
	_, _ = planner.Plan(ctx, req)
	if planner.Remaining() != 1 {
		t.Errorf("Remaining() after one Plan = %d, want 1", planner.Remaining())
	}

	// Reset
	planner.Reset()
	if planner.Remaining() != 2 {
		t.Errorf("Remaining() after Reset = %d, want 2", planner.Remaining())
	}

	// Can use all decisions again
	d1, _ := planner.Plan(ctx, req)
	if d1.Type != agent.DecisionTransition {
		t.Errorf("First decision after reset = %v, want transition", d1.Type)
	}
}

func TestMockPlanner_AddDecision(t *testing.T) {
	t.Parallel()

	planner := NewMockPlanner()
	if planner.Remaining() != 0 {
		t.Errorf("Initial Remaining() = %d, want 0", planner.Remaining())
	}

	planner.AddDecision(agent.NewTransitionDecision(agent.StateExplore, "explore"))
	if planner.Remaining() != 1 {
		t.Errorf("Remaining() after AddDecision = %d, want 1", planner.Remaining())
	}

	planner.AddDecision(agent.NewFinishDecision("done", nil))
	if planner.Remaining() != 2 {
		t.Errorf("Remaining() after second AddDecision = %d, want 2", planner.Remaining())
	}
}

func TestMockPlanner_Concurrency(t *testing.T) {
	t.Parallel()

	// Create a planner with many decisions
	numDecisions := 100
	decisions := make([]agent.Decision, numDecisions)
	for i := 0; i < numDecisions; i++ {
		decisions[i] = agent.NewFinishDecision("done", nil)
	}
	planner := NewMockPlanner(decisions...)

	// Concurrently consume decisions
	var wg sync.WaitGroup
	for i := 0; i < numDecisions; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = planner.Plan(context.Background(), PlanRequest{})
		}()
	}

	wg.Wait()

	// All decisions should be consumed
	if planner.Remaining() != 0 {
		t.Errorf("Remaining() after concurrent calls = %d, want 0", planner.Remaining())
	}
}
