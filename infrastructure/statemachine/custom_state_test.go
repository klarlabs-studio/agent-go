package statemachine

import (
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/ledger"
	"github.com/felixgeelhaar/agent-go/domain/policy"
)

func TestStateRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("registers valid custom state", func(t *testing.T) {
		t.Parallel()

		reg := agent.NewStateRegistry()
		err := reg.Register(agent.CustomState{
			Name:              agent.State("review"),
			AllowsSideEffects: false,
			Terminal:          false,
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		if !reg.IsValid(agent.State("review")) {
			t.Error("registered custom state should be valid")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		t.Parallel()

		reg := agent.NewStateRegistry()
		err := reg.Register(agent.CustomState{Name: ""})
		if err == nil {
			t.Error("Register() should reject empty name")
		}
	})

	t.Run("rejects canonical state name", func(t *testing.T) {
		t.Parallel()

		reg := agent.NewStateRegistry()
		err := reg.Register(agent.CustomState{Name: agent.StateIntake})
		if err == nil {
			t.Error("Register() should reject canonical state name")
		}

		for _, s := range agent.AllStates() {
			err := reg.Register(agent.CustomState{Name: s})
			if err == nil {
				t.Errorf("Register() should reject canonical state %q", s)
			}
		}
	})

	t.Run("rejects duplicate custom state", func(t *testing.T) {
		t.Parallel()

		reg := agent.NewStateRegistry()
		cs := agent.CustomState{Name: agent.State("review")}
		_ = reg.Register(cs)

		err := reg.Register(cs)
		if err == nil {
			t.Error("Register() should reject duplicate custom state")
		}
	})
}

func TestStateRegistry_IsTerminal(t *testing.T) {
	t.Parallel()

	reg := agent.NewStateRegistry()
	_ = reg.Register(agent.CustomState{Name: agent.State("aborted"), Terminal: true})
	_ = reg.Register(agent.CustomState{Name: agent.State("review"), Terminal: false})

	tests := []struct {
		state    agent.State
		terminal bool
	}{
		{agent.StateDone, true},
		{agent.StateFailed, true},
		{agent.StateIntake, false},
		{agent.State("aborted"), true},
		{agent.State("review"), false},
		{agent.State("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			t.Parallel()
			if got := reg.IsTerminal(tt.state); got != tt.terminal {
				t.Errorf("IsTerminal(%s) = %v, want %v", tt.state, got, tt.terminal)
			}
		})
	}
}

func TestStateRegistry_AllowsSideEffects(t *testing.T) {
	t.Parallel()

	reg := agent.NewStateRegistry()
	_ = reg.Register(agent.CustomState{Name: agent.State("execute"), AllowsSideEffects: true})
	_ = reg.Register(agent.CustomState{Name: agent.State("plan"), AllowsSideEffects: false})

	tests := []struct {
		state      agent.State
		sideEffect bool
	}{
		{agent.StateAct, true},
		{agent.StateExplore, false},
		{agent.State("execute"), true},
		{agent.State("plan"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			t.Parallel()
			if got := reg.AllowsSideEffects(tt.state); got != tt.sideEffect {
				t.Errorf("AllowsSideEffects(%s) = %v, want %v", tt.state, got, tt.sideEffect)
			}
		})
	}
}

func TestStateRegistry_Get(t *testing.T) {
	t.Parallel()

	reg := agent.NewStateRegistry()
	expected := agent.CustomState{Name: agent.State("review"), Terminal: false}
	_ = reg.Register(expected)

	t.Run("returns registered state", func(t *testing.T) {
		t.Parallel()
		cs, ok := reg.Get(agent.State("review"))
		if !ok {
			t.Fatal("Get() should find registered state")
		}
		if cs.Name != expected.Name {
			t.Errorf("Get() name = %s, want %s", cs.Name, expected.Name)
		}
	})

	t.Run("returns false for canonical state", func(t *testing.T) {
		t.Parallel()
		_, ok := reg.Get(agent.StateIntake)
		if ok {
			t.Error("Get() should not return canonical states")
		}
	})

	t.Run("returns false for unknown state", func(t *testing.T) {
		t.Parallel()
		_, ok := reg.Get(agent.State("unknown"))
		if ok {
			t.Error("Get() should return false for unknown state")
		}
	})
}

func TestStateRegistry_All(t *testing.T) {
	t.Parallel()

	reg := agent.NewStateRegistry()
	_ = reg.Register(agent.CustomState{Name: agent.State("review")})
	_ = reg.Register(agent.CustomState{Name: agent.State("approve")})

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d states, want 2", len(all))
	}

	names := make(map[agent.State]bool)
	for _, cs := range all {
		names[cs.Name] = true
	}
	if !names[agent.State("review")] || !names[agent.State("approve")] {
		t.Error("All() should return both registered states")
	}
}

func TestStateRegistry_AllStatesIncludingCustom(t *testing.T) {
	t.Parallel()

	reg := agent.NewStateRegistry()
	_ = reg.Register(agent.CustomState{Name: agent.State("review")})

	all := reg.AllStatesIncludingCustom()
	// 7 canonical + 1 custom
	if len(all) != 8 {
		t.Fatalf("AllStatesIncludingCustom() returned %d states, want 8", len(all))
	}
}

func TestMachineBuilder_CustomStates(t *testing.T) {
	t.Parallel()

	t.Run("builds machine with custom state", func(t *testing.T) {
		t.Parallel()

		def, err := NewAgentMachineBuilder().
			WithCustomState(agent.CustomState{
				Name:              agent.State("review"),
				AllowsSideEffects: false,
				Terminal:          false,
			}).
			WithCustomTransition(agent.StateValidate, agent.State("review")).
			WithCustomTransition(agent.State("review"), agent.StateExplore).
			Build()

		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if def == nil {
			t.Fatal("Build() returned nil definition")
		}
		if def.Config == nil {
			t.Fatal("Build() returned nil config")
		}
		if !def.StateRegistry.IsValid(agent.State("review")) {
			t.Error("custom state should be valid in registry")
		}
	})

	t.Run("builds machine with terminal custom state", func(t *testing.T) {
		t.Parallel()

		def, err := NewAgentMachineBuilder().
			WithCustomState(agent.CustomState{
				Name:     agent.State("cancelled"),
				Terminal: true,
			}).
			WithCustomTransition(agent.StateDecide, agent.State("cancelled")).
			Build()

		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if !def.StateRegistry.IsTerminal(agent.State("cancelled")) {
			t.Error("cancelled should be terminal")
		}
	})

	t.Run("rejects duplicate custom state name", func(t *testing.T) {
		t.Parallel()

		_, err := NewAgentMachineBuilder().
			WithCustomState(agent.CustomState{Name: agent.State("review")}).
			WithCustomState(agent.CustomState{Name: agent.State("review")}).
			Build()

		if err == nil {
			t.Error("Build() should reject duplicate custom state names")
		}
	})

	t.Run("rejects canonical state name as custom", func(t *testing.T) {
		t.Parallel()

		_, err := NewAgentMachineBuilder().
			WithCustomState(agent.CustomState{Name: agent.StateAct}).
			Build()

		if err == nil {
			t.Error("Build() should reject canonical state name as custom")
		}
	})
}

func TestMachineBuilder_BackwardCompatibility(t *testing.T) {
	t.Parallel()

	// A builder with no custom states should produce the same machine as
	// the original NewAgentMachine.
	def, err := NewAgentMachineBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreterFromDefinition(def, ctx)
	interp.Start()

	if interp.State() != agent.StateIntake {
		t.Errorf("Initial state = %s, want intake", interp.State())
	}

	// Full canonical workflow should still work
	steps := []agent.State{
		agent.StateExplore,
		agent.StateDecide,
		agent.StateAct,
		agent.StateValidate,
		agent.StateDone,
	}

	for _, s := range steps {
		if err := interp.Transition(s, "test"); err != nil {
			t.Fatalf("Transition to %s failed: %v", s, err)
		}
	}

	if !interp.IsTerminal() {
		t.Error("Should be terminal after done")
	}
}

func TestInterpreter_CustomStateTransitions(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineBuilder().
		WithCustomState(agent.CustomState{
			Name:              agent.State("review"),
			AllowsSideEffects: false,
			Terminal:          false,
		}).
		WithCustomTransition(agent.StateValidate, agent.State("review")).
		WithCustomTransition(agent.State("review"), agent.StateExplore).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 100})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	// Add custom transitions to policy layer
	ctx.Transitions = policy.NewStateTransitionsWith(policy.TransitionRules{
		agent.StateIntake:       {agent.StateExplore, agent.StateFailed},
		agent.StateExplore:      {agent.StateDecide, agent.StateFailed},
		agent.StateDecide:       {agent.StateAct, agent.StateDone, agent.StateFailed},
		agent.StateAct:          {agent.StateValidate, agent.StateFailed},
		agent.StateValidate:     {agent.StateDone, agent.StateExplore, agent.StateFailed, agent.State("review")},
		agent.State("review"):   {agent.StateExplore, agent.StateFailed},
	})

	interp := NewInterpreterFromDefinition(def, ctx)
	interp.Start()

	// Navigate to validate
	interp.Transition(agent.StateExplore, "explore")
	interp.Transition(agent.StateDecide, "decide")
	interp.Transition(agent.StateAct, "act")
	interp.Transition(agent.StateValidate, "validate")

	// Transition to custom state
	err = interp.Transition(agent.State("review"), "needs review")
	if err != nil {
		t.Fatalf("Transition to review failed: %v", err)
	}
	if interp.State() != agent.State("review") {
		t.Errorf("State = %s, want review", interp.State())
	}

	// Transition from custom state back to explore
	err = interp.Transition(agent.StateExplore, "back to explore")
	if err != nil {
		t.Fatalf("Transition from review to explore failed: %v", err)
	}
	if interp.State() != agent.StateExplore {
		t.Errorf("State = %s, want explore", interp.State())
	}
}

func TestInterpreter_CustomTerminalState(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineBuilder().
		WithCustomState(agent.CustomState{
			Name:     agent.State("cancelled"),
			Terminal: true,
		}).
		WithCustomTransition(agent.StateDecide, agent.State("cancelled")).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 100})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	ctx.Transitions = policy.NewStateTransitionsWith(policy.TransitionRules{
		agent.StateIntake:   {agent.StateExplore, agent.StateFailed},
		agent.StateExplore:  {agent.StateDecide, agent.StateFailed},
		agent.StateDecide:   {agent.StateAct, agent.StateDone, agent.StateFailed, agent.State("cancelled")},
		agent.StateAct:      {agent.StateValidate, agent.StateFailed},
		agent.StateValidate: {agent.StateDone, agent.StateExplore, agent.StateFailed},
	})

	interp := NewInterpreterFromDefinition(def, ctx)
	interp.Start()

	interp.Transition(agent.StateExplore, "explore")
	interp.Transition(agent.StateDecide, "decide")

	err = interp.Transition(agent.State("cancelled"), "user cancelled")
	if err != nil {
		t.Fatalf("Transition to cancelled failed: %v", err)
	}

	if interp.State() != agent.State("cancelled") {
		t.Errorf("State = %s, want cancelled", interp.State())
	}

	if !interp.IsTerminal() {
		t.Error("cancelled should be a terminal state")
	}
}

func TestMachineBuilder_WithCustomTransitionEvent(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineBuilder().
		WithCustomState(agent.CustomState{
			Name: agent.State("review"),
		}).
		WithCustomTransitionEvent(agent.StateValidate, "REQUEST_REVIEW", agent.State("review")).
		WithCustomTransitionEvent(agent.State("review"), "APPROVE", agent.StateExplore).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(def.CustomTransitions) != 2 {
		t.Fatalf("CustomTransitions length = %d, want 2", len(def.CustomTransitions))
	}
	if def.CustomTransitions[0].Event != "REQUEST_REVIEW" {
		t.Errorf("first transition event = %s, want REQUEST_REVIEW", def.CustomTransitions[0].Event)
	}
}
