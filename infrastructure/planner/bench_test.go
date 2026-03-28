package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// BenchmarkRuleBasedPlanner_SmallRuleSet benchmarks a planner with 5 rules.
func BenchmarkRuleBasedPlanner_SmallRuleSet(b *testing.B) {
	rules := buildRules(5)
	fallback := agent.NewFailDecision("no match", nil)
	p := NewRuleBasedPlanner(fallback, rules...)

	req := PlanRequest{
		CurrentState: agent.StateExplore,
		Goal:         "benchmark",
		Evidence: []agent.Evidence{
			agent.NewToolEvidence("read_file", json.RawMessage(`{"content":"data"}`)),
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = p.Plan(ctx, req)
	}
}

// BenchmarkRuleBasedPlanner_LargeRuleSet benchmarks a planner with 100 rules.
func BenchmarkRuleBasedPlanner_LargeRuleSet(b *testing.B) {
	rules := buildRules(100)
	fallback := agent.NewFailDecision("no match", nil)
	p := NewRuleBasedPlanner(fallback, rules...)

	// Request that matches the last rule to exercise full scan
	req := PlanRequest{
		CurrentState: agent.State("state_99"),
		Goal:         "benchmark",
		Evidence: []agent.Evidence{
			agent.NewToolEvidence("read_file", json.RawMessage(`{"content":"data"}`)),
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = p.Plan(ctx, req)
	}
}

// BenchmarkRuleBasedPlanner_FirstRuleMatch benchmarks the best-case scenario
// where the first rule matches.
func BenchmarkRuleBasedPlanner_FirstRuleMatch(b *testing.B) {
	rules := buildRules(100)
	fallback := agent.NewFailDecision("no match", nil)
	p := NewRuleBasedPlanner(fallback, rules...)

	// Request that matches the first rule (priority 0, state "state_0")
	req := PlanRequest{
		CurrentState: agent.State("state_0"),
		Goal:         "benchmark",
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = p.Plan(ctx, req)
	}
}

// BenchmarkRuleBasedPlanner_NoMatch benchmarks the worst case where no rule
// matches and the fallback is returned.
func BenchmarkRuleBasedPlanner_NoMatch(b *testing.B) {
	rules := buildRules(100)
	fallback := agent.NewFailDecision("no match", nil)
	p := NewRuleBasedPlanner(fallback, rules...)

	// Request with a state that does not match any rule
	req := PlanRequest{
		CurrentState: agent.State("nonexistent_state"),
		Goal:         "benchmark",
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = p.Plan(ctx, req)
	}
}

// BenchmarkScriptedPlanner benchmarks the ScriptedPlanner stepping through
// a sequence of decisions.
func BenchmarkScriptedPlanner(b *testing.B) {
	steps := []ScriptStep{
		{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "explore")},
		{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "decide")},
		{ExpectState: agent.StateDecide, Decision: agent.NewTransitionDecision(agent.StateAct, "act")},
		{ExpectState: agent.StateAct, Decision: agent.NewTransitionDecision(agent.StateValidate, "validate")},
		{ExpectState: agent.StateValidate, Decision: agent.NewFinishDecision("done", json.RawMessage(`{}`))},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := NewScriptedPlanner(steps...)

		for _, step := range steps {
			req := PlanRequest{
				CurrentState: step.ExpectState,
				Goal:         "benchmark",
			}
			_, _ = p.Plan(ctx, req)
		}
	}
}

// BenchmarkScriptedPlanner_WithLoop benchmarks the ScriptedPlanner with
// loop support enabled.
func BenchmarkScriptedPlanner_WithLoop(b *testing.B) {
	steps := []ScriptStep{
		{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "decide")},
		{ExpectState: agent.StateDecide, Decision: agent.NewTransitionDecision(agent.StateAct, "act")},
		{ExpectState: agent.StateAct, Decision: agent.NewTransitionDecision(agent.StateValidate, "validate")},
		{ExpectState: agent.StateValidate, Decision: agent.NewTransitionDecision(agent.StateExplore, "loop")},
	}

	ctx := context.Background()
	states := []agent.State{agent.StateExplore, agent.StateDecide, agent.StateAct, agent.StateValidate}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := NewScriptedPlanner(steps...).WithLoop(5)

		for loop := 0; loop < 5; loop++ {
			for _, st := range states {
				req := PlanRequest{
					CurrentState: st,
					Goal:         "benchmark",
				}
				_, _ = p.Plan(ctx, req)
			}
		}
	}
}

// BenchmarkMockPlanner benchmarks the MockPlanner for baseline comparison.
func BenchmarkMockPlanner(b *testing.B) {
	decisions := []agent.Decision{
		agent.NewTransitionDecision(agent.StateExplore, "explore"),
		agent.NewTransitionDecision(agent.StateDecide, "decide"),
		agent.NewFinishDecision("done", json.RawMessage(`{}`)),
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := NewMockPlanner(decisions...)
		for range decisions {
			_, _ = p.Plan(ctx, PlanRequest{})
		}
	}
}

// buildRules creates n rules, each targeting a unique state "state_0" through "state_{n-1}".
func buildRules(n int) []Rule {
	rules := make([]Rule, n)
	for i := 0; i < n; i++ {
		rules[i] = NewRule(fmt.Sprintf("rule_%d", i)).
			WithPriority(i).
			InState(agent.State(fmt.Sprintf("state_%d", i))).
			Then(agent.NewTransitionDecision(agent.StateExplore, fmt.Sprintf("matched rule %d", i))).
			Build()
	}
	return rules
}
