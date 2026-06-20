package aiplugin_test

import (
	"sync"
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/aiplugin"
	"go.klarlabs.de/statekit/plugin"
)

type budgetCtx struct{}

func TestTransitionBudget_CountsAndExhausts(t *testing.T) {
	t.Parallel()
	b := aiplugin.NewTransitionBudget[budgetCtx](2, "HALT")

	// AfterTransition counts state-changing transitions.
	b.AfterTransition(plugin.Context[budgetCtx]{}, "a", "b", plugin.Event{Type: "X"})
	if b.Used() != 1 {
		t.Errorf("Used = %d, want 1", b.Used())
	}
	b.AfterTransition(plugin.Context[budgetCtx]{}, "b", "c", plugin.Event{Type: "Y"})
	if b.Used() != 2 {
		t.Errorf("Used = %d, want 2", b.Used())
	}
	if !b.Exhausted() {
		t.Error("expected Exhausted")
	}
	if b.Remaining() != 0 {
		t.Errorf("Remaining = %d, want 0", b.Remaining())
	}
}

func TestTransitionBudget_SkipsSelfTransition(t *testing.T) {
	t.Parallel()
	b := aiplugin.NewTransitionBudget[budgetCtx](100, "HALT")

	// Self-transitions don't count.
	b.AfterTransition(plugin.Context[budgetCtx]{}, "x", "x", plugin.Event{Type: "X"})
	if b.Used() != 0 {
		t.Errorf("self-transition counted: Used = %d", b.Used())
	}
}

func TestTransitionBudget_RewritesEventWhenExhausted(t *testing.T) {
	t.Parallel()
	b := aiplugin.NewTransitionBudget[budgetCtx](1, "HALT")

	// Below budget — pass-through.
	out := b.OnEvent(plugin.Context[budgetCtx]{}, plugin.Event{Type: "GO", Payload: 42})
	if out.Type != "GO" {
		t.Errorf("below budget, event rewritten to %q", out.Type)
	}

	// Tick the budget.
	b.AfterTransition(plugin.Context[budgetCtx]{}, "a", "b", plugin.Event{Type: "GO"})

	// Above budget — rewritten.
	out = b.OnEvent(plugin.Context[budgetCtx]{}, plugin.Event{Type: "STILL_GO", Payload: 99})
	if out.Type != "HALT" {
		t.Errorf("above budget, event = %q, want HALT", out.Type)
	}
	if out.Payload != 99 {
		t.Errorf("payload not preserved: %v", out.Payload)
	}
}

func TestTransitionBudget_Reset(t *testing.T) {
	t.Parallel()
	b := aiplugin.NewTransitionBudget[budgetCtx](2, "HALT")
	b.AfterTransition(plugin.Context[budgetCtx]{}, "a", "b", plugin.Event{})
	b.AfterTransition(plugin.Context[budgetCtx]{}, "b", "c", plugin.Event{})
	if !b.Exhausted() {
		t.Fatal("expected exhausted before Reset")
	}
	b.Reset()
	if b.Exhausted() {
		t.Error("expected not exhausted after Reset")
	}
	if b.Used() != 0 {
		t.Errorf("Used after Reset = %d, want 0", b.Used())
	}
}

func TestTransitionBudget_Concurrent(t *testing.T) {
	t.Parallel()
	b := aiplugin.NewTransitionBudget[budgetCtx](100_000, "HALT")
	var wg sync.WaitGroup
	const goroutines = 16
	const perG = 1000
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				b.AfterTransition(plugin.Context[budgetCtx]{}, "a", "b", plugin.Event{})
			}
		}()
	}
	wg.Wait()
	if got := b.Used(); got != goroutines*perG {
		t.Errorf("Used = %d, want %d", got, goroutines*perG)
	}
}

// TestTransitionBudget_PluginInterface verifies the type satisfies
// the relevant plugin hook interfaces at compile time.
func TestTransitionBudget_PluginInterface(t *testing.T) {
	t.Parallel()
	var _ plugin.OnTransitionHook[budgetCtx] = aiplugin.NewTransitionBudget[budgetCtx](1, "X")
	var _ plugin.OnEventHook[budgetCtx] = aiplugin.NewTransitionBudget[budgetCtx](1, "X")
	if aiplugin.NewTransitionBudget[budgetCtx](1, "X").Name() != "ai-transition-budget" {
		t.Error("name mismatch")
	}
}

// TestTransitionBudget_EndToEnd_HaltsRunawayMachine drops a budget
// into a real interpreter and confirms it halts a runaway loop —
// the pattern qmuntal/stateless #77 author wanted.
func TestTransitionBudget_EndToEnd_HaltsRunawayMachine(t *testing.T) {
	t.Parallel()

	machine, err := statekit.NewMachine[budgetCtx]("loop").
		WithInitial("a").
		State("a").On("TICK").Target("b").On("HALT").Target("done").Done().
		State("b").On("TICK").Target("a").On("HALT").Target("done").Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()

	budget := aiplugin.NewTransitionBudget[budgetCtx](3, "HALT")
	interp.Use(budget)
	interp.Start()

	// Send 10 TICKs — should halt by the 4th transition.
	for i := 0; i < 10; i++ {
		interp.Send(statekit.Event{Type: "TICK"})
	}

	if !interp.Done() {
		t.Errorf("expected interpreter Done (in halt final state); state = %q", interp.State().Value)
	}
	if got := budget.Used(); got > 4 {
		t.Errorf("budget overshot: Used = %d, expected <= 4", got)
	}
}
