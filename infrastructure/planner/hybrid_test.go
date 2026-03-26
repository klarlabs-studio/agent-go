package planner

import (
	"context"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

func TestHybridPlanner_RulesFirst(t *testing.T) {
	ruleFallback := agent.NewFailDecision("rule fallback", nil)
	ruleDecision := agent.NewTransitionDecision(agent.StateExplore, "from rules")

	rules := NewRuleBasedPlanner(ruleFallback,
		NewRule("intake-rule").
			InState(agent.StateIntake).
			Then(ruleDecision).
			Build(),
	)

	fallbackPlanner := NewMockPlanner(
		agent.NewFinishDecision("from fallback", nil),
	)

	hybrid := NewHybridPlanner(rules, fallbackPlanner)

	// Request matches a rule - should use rules
	decision, err := hybrid.Plan(context.Background(), PlanRequest{
		CurrentState: agent.StateIntake,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Type != agent.DecisionTransition {
		t.Errorf("decision type = %s, want %s", decision.Type, agent.DecisionTransition)
	}
	if hybrid.LastSource() != "rules" {
		t.Errorf("last source = %q, want %q", hybrid.LastSource(), "rules")
	}
}

func TestHybridPlanner_FallbackWhenNoRuleMatches(t *testing.T) {
	ruleFallback := agent.NewFailDecision("rule fallback", nil)
	rules := NewRuleBasedPlanner(ruleFallback,
		NewRule("only-intake").
			InState(agent.StateIntake).
			Then(agent.NewTransitionDecision(agent.StateExplore, "intake")).
			Build(),
	)

	fallbackDecision := agent.NewFinishDecision("from fallback", nil)
	fallbackPlanner := NewMockPlanner(fallbackDecision)

	hybrid := NewHybridPlanner(rules, fallbackPlanner)

	// Request in explore state - no rule matches, should fallback
	decision, err := hybrid.Plan(context.Background(), PlanRequest{
		CurrentState: agent.StateExplore,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Type != agent.DecisionFinish {
		t.Errorf("decision type = %s, want %s", decision.Type, agent.DecisionFinish)
	}
	if hybrid.LastSource() != "fallback" {
		t.Errorf("last source = %q, want %q", hybrid.LastSource(), "fallback")
	}
}

func TestHybridPlanner_ForceRulesOnly(t *testing.T) {
	ruleFallback := agent.NewFailDecision("rule fallback", nil)
	rules := NewRuleBasedPlanner(ruleFallback,
		NewRule("only-intake").
			InState(agent.StateIntake).
			Then(agent.NewTransitionDecision(agent.StateExplore, "intake")).
			Build(),
	)

	fallbackPlanner := NewMockPlanner(
		agent.NewFinishDecision("from fallback", nil),
	)

	hybrid := NewHybridPlanner(rules, fallbackPlanner).
		ForceRulesOnly(agent.StateExplore)

	// In explore state with rules-only forced: should get rule fallback, not planner fallback
	decision, err := hybrid.Plan(context.Background(), PlanRequest{
		CurrentState: agent.StateExplore,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Type != agent.DecisionFail {
		t.Errorf("decision type = %s, want %s (rule fallback)", decision.Type, agent.DecisionFail)
	}
	if hybrid.LastSource() != "rules" {
		t.Errorf("last source = %q, want %q", hybrid.LastSource(), "rules")
	}

	// In decide state (not rules-only): should fall through to fallback planner
	decision, err = hybrid.Plan(context.Background(), PlanRequest{
		CurrentState: agent.StateDecide,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Type != agent.DecisionFinish {
		t.Errorf("decision type = %s, want %s (fallback)", decision.Type, agent.DecisionFinish)
	}
	if hybrid.LastSource() != "fallback" {
		t.Errorf("last source = %q, want %q", hybrid.LastSource(), "fallback")
	}
}

func TestHybridPlanner_MultipleStates(t *testing.T) {
	ruleFallback := agent.NewFailDecision("rule fallback", nil)
	rules := NewRuleBasedPlanner(ruleFallback,
		NewRule("intake-rule").
			InState(agent.StateIntake).
			Then(agent.NewTransitionDecision(agent.StateExplore, "begin")).
			Build(),
		NewRule("explore-rule").
			InState(agent.StateExplore).
			Then(agent.NewTransitionDecision(agent.StateDecide, "gathered")).
			Build(),
	)

	fallbackPlanner := NewMockPlanner(
		agent.NewFinishDecision("fallback-1", nil),
		agent.NewFinishDecision("fallback-2", nil),
	)

	hybrid := NewHybridPlanner(rules, fallbackPlanner)

	tests := []struct {
		state      agent.State
		wantType   agent.DecisionType
		wantSource string
	}{
		{agent.StateIntake, agent.DecisionTransition, "rules"},
		{agent.StateExplore, agent.DecisionTransition, "rules"},
		{agent.StateDecide, agent.DecisionFinish, "fallback"},
		{agent.StateAct, agent.DecisionFinish, "fallback"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			decision, err := hybrid.Plan(context.Background(), PlanRequest{
				CurrentState: tt.state,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision.Type != tt.wantType {
				t.Errorf("decision type = %s, want %s", decision.Type, tt.wantType)
			}
			if hybrid.LastSource() != tt.wantSource {
				t.Errorf("source = %q, want %q", hybrid.LastSource(), tt.wantSource)
			}
		})
	}
}

func TestHybridPlanner_ForceRulesOnlyMultipleStates(t *testing.T) {
	ruleFallback := agent.NewFailDecision("rule fallback", nil)
	rules := NewRuleBasedPlanner(ruleFallback)

	fallbackPlanner := NewMockPlanner(
		agent.NewFinishDecision("fallback", nil),
	)

	hybrid := NewHybridPlanner(rules, fallbackPlanner).
		ForceRulesOnly(agent.StateIntake, agent.StateExplore, agent.StateDecide)

	// All three states should use rules (and get rule fallback since no rules defined)
	for _, state := range []agent.State{agent.StateIntake, agent.StateExplore, agent.StateDecide} {
		decision, err := hybrid.Plan(context.Background(), PlanRequest{
			CurrentState: state,
		})
		if err != nil {
			t.Fatalf("unexpected error for state %s: %v", state, err)
		}
		if decision.Type != agent.DecisionFail {
			t.Errorf("state %s: decision type = %s, want %s", state, decision.Type, agent.DecisionFail)
		}
		if hybrid.LastSource() != "rules" {
			t.Errorf("state %s: source = %q, want %q", state, hybrid.LastSource(), "rules")
		}
	}
}
