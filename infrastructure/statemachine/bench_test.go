package statemachine

import (
	"testing"

	"github.com/felixgeelhaar/statekit"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/ledger"
	"github.com/felixgeelhaar/agent-go/domain/policy"
)

// BenchmarkStateMachine_Transition benchmarks a single state transition through
// the interpreter.
func BenchmarkStateMachine_Transition(b *testing.B) {
	machine, err := NewAgentMachine()
	if err != nil {
		b.Fatalf("failed to create machine: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		run := agent.NewRun("bench-run", "bench goal")
		budget := policy.NewBudget(map[string]int{"tool_calls": 100})
		lg := ledger.New("bench-run")

		ctx := NewContext(run, budget, lg)
		ctx.Transitions = policy.DefaultTransitions()

		interp := NewInterpreter(machine, ctx)
		interp.Start()

		_ = interp.Transition(agent.StateExplore, "explore")
		_ = interp.Transition(agent.StateDecide, "decide")
		_ = interp.Transition(agent.StateDone, "done")
	}
}

// BenchmarkStateMachine_GuardEvaluation benchmarks guard evaluation during
// transitions. Guards include canTransition and budgetAvailable checks.
func BenchmarkStateMachine_GuardEvaluation(b *testing.B) {
	machine, err := NewAgentMachine()
	if err != nil {
		b.Fatalf("failed to create machine: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		run := agent.NewRun("bench-run", "bench goal")
		budget := policy.NewBudget(map[string]int{"tool_calls": 100})
		lg := ledger.New("bench-run")

		ctx := NewContext(run, budget, lg)
		ctx.Transitions = policy.DefaultTransitions()

		interp := NewInterpreter(machine, ctx)
		interp.Start()

		// This path exercises guards: canTransition is checked on every transition,
		// budgetAvailable is checked on the decide->act transition.
		_ = interp.Transition(agent.StateExplore, "explore")
		_ = interp.Transition(agent.StateDecide, "decide")
		_ = interp.Transition(agent.StateAct, "act")
		_ = interp.Transition(agent.StateValidate, "validate")
		_ = interp.Transition(agent.StateExplore, "loop")
		_ = interp.Transition(agent.StateDecide, "decide again")
		_ = interp.Transition(agent.StateDone, "done")
	}
}

// BenchmarkStateMachine_CanTransition benchmarks the CanTransition check
// without actually performing the transition.
func BenchmarkStateMachine_CanTransition(b *testing.B) {
	machine, err := NewAgentMachine()
	if err != nil {
		b.Fatalf("failed to create machine: %v", err)
	}

	run := agent.NewRun("bench-run", "bench goal")
	budget := policy.NewBudget(nil)
	lg := ledger.New("bench-run")

	ctx := NewContext(run, budget, lg)
	ctx.Transitions = policy.DefaultTransitions()

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = interp.CanTransition(agent.StateExplore)
		_ = interp.CanTransition(agent.StateAct)      // invalid from intake
		_ = interp.CanTransition(agent.StateValidate) // invalid from intake
		_ = interp.CanTransition(agent.StateFailed)   // always valid
	}
}

// BenchmarkGuardCanTransition benchmarks the raw guard function directly.
func BenchmarkGuardCanTransition(b *testing.B) {
	run := agent.NewRun("bench-run", "bench goal")
	run.CurrentState = agent.StateExplore

	budget := policy.NewBudget(map[string]int{"tool_calls": 100})
	lg := ledger.New("bench-run")
	ctx := NewContext(run, budget, lg)
	ctx.Transitions = policy.DefaultTransitions()

	event := statekit.Event{
		Type: "DECIDE",
		Payload: TransitionPayload{
			ToState: agent.StateDecide,
			Reason:  "bench",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = guardCanTransition(ctx, event)
	}
}

// BenchmarkGuardBudgetAvailable benchmarks the budget guard function.
func BenchmarkGuardBudgetAvailable(b *testing.B) {
	run := agent.NewRun("bench-run", "bench goal")
	budget := policy.NewBudget(map[string]int{"tool_calls": 100})
	lg := ledger.New("bench-run")
	ctx := NewContext(run, budget, lg)

	event := statekit.Event{Type: "ACT"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = guardBudgetAvailable(ctx, event)
	}
}
