package policy

import (
	"sort"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
)

func TestNewToolEligibility(t *testing.T) {
	t.Parallel()

	eligibility := NewToolEligibility()
	if eligibility == nil {
		t.Fatal("NewToolEligibility() returned nil")
	}

	// Initially no tools are allowed
	if eligibility.IsAllowed(agent.StateExplore, "any_tool") {
		t.Error("No tools should be allowed initially")
	}
}

func TestToolEligibility_Allow(t *testing.T) {
	t.Parallel()

	eligibility := NewToolEligibility()

	// Allow a tool in explore state
	result := eligibility.Allow(agent.StateExplore, "read_file")
	if result != eligibility {
		t.Error("Allow() should return the eligibility for chaining")
	}

	// Tool should be allowed in explore state
	if !eligibility.IsAllowed(agent.StateExplore, "read_file") {
		t.Error("read_file should be allowed in explore state")
	}

	// Tool should NOT be allowed in other states
	if eligibility.IsAllowed(agent.StateAct, "read_file") {
		t.Error("read_file should not be allowed in act state")
	}

	// Other tools should NOT be allowed
	if eligibility.IsAllowed(agent.StateExplore, "write_file") {
		t.Error("write_file should not be allowed")
	}
}

func TestToolEligibility_AllowMultiple(t *testing.T) {
	t.Parallel()

	eligibility := NewToolEligibility()
	eligibility.AllowMultiple(agent.StateExplore, "read_file", "list_dir", "search")

	// All tools should be allowed
	tools := []string{"read_file", "list_dir", "search"}
	for _, tool := range tools {
		if !eligibility.IsAllowed(agent.StateExplore, tool) {
			t.Errorf("%s should be allowed in explore state", tool)
		}
	}

	// Non-listed tools should not be allowed
	if eligibility.IsAllowed(agent.StateExplore, "write_file") {
		t.Error("write_file should not be allowed")
	}
}

func TestToolEligibility_AllowMultipleStates(t *testing.T) {
	t.Parallel()

	eligibility := NewToolEligibility().
		Allow(agent.StateExplore, "read_file").
		Allow(agent.StateExplore, "list_dir").
		Allow(agent.StateAct, "write_file").
		Allow(agent.StateAct, "delete_file")

	// Explore state tools
	if !eligibility.IsAllowed(agent.StateExplore, "read_file") {
		t.Error("read_file should be allowed in explore")
	}
	if !eligibility.IsAllowed(agent.StateExplore, "list_dir") {
		t.Error("list_dir should be allowed in explore")
	}
	if eligibility.IsAllowed(agent.StateExplore, "write_file") {
		t.Error("write_file should NOT be allowed in explore")
	}

	// Act state tools
	if !eligibility.IsAllowed(agent.StateAct, "write_file") {
		t.Error("write_file should be allowed in act")
	}
	if !eligibility.IsAllowed(agent.StateAct, "delete_file") {
		t.Error("delete_file should be allowed in act")
	}
	if eligibility.IsAllowed(agent.StateAct, "read_file") {
		t.Error("read_file should NOT be allowed in act")
	}
}

func TestToolEligibility_AllowedTools(t *testing.T) {
	t.Parallel()

	t.Run("returns tools for state", func(t *testing.T) {
		t.Parallel()

		eligibility := NewToolEligibility()
		eligibility.AllowMultiple(agent.StateExplore, "read_file", "list_dir", "search")

		tools := eligibility.AllowedTools(agent.StateExplore)
		if len(tools) != 3 {
			t.Errorf("AllowedTools() returned %d tools, want 3", len(tools))
		}

		// Check all tools are present (order may vary due to map iteration)
		sort.Strings(tools)
		expected := []string{"list_dir", "read_file", "search"}
		for i, exp := range expected {
			if tools[i] != exp {
				t.Errorf("AllowedTools()[%d] = %s, want %s", i, tools[i], exp)
			}
		}
	})

	t.Run("returns nil for unknown state", func(t *testing.T) {
		t.Parallel()

		eligibility := NewToolEligibility()
		tools := eligibility.AllowedTools(agent.StateExplore)
		if tools != nil {
			t.Errorf("AllowedTools() for unknown state should return nil, got %v", tools)
		}
	})

	t.Run("returns empty for state with no tools", func(t *testing.T) {
		t.Parallel()

		eligibility := NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		tools := eligibility.AllowedTools(agent.StateAct)
		if tools != nil {
			t.Errorf("AllowedTools() for state with no tools should return nil, got %v", tools)
		}
	})
}

func TestToolEligibility_Wildcard(t *testing.T) {
	t.Parallel()

	eligibility := NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "*")

	// Any tool should be allowed via wildcard
	if !eligibility.IsAllowed(agent.StateExplore, "read_file") {
		t.Error("wildcard should allow read_file in explore")
	}
	if !eligibility.IsAllowed(agent.StateExplore, "any_random_tool") {
		t.Error("wildcard should allow any tool in explore")
	}

	// Other states without wildcard should still deny
	if eligibility.IsAllowed(agent.StateAct, "read_file") {
		t.Error("act state has no wildcard, should deny")
	}
}

func TestToolEligibility_HasWildcard(t *testing.T) {
	t.Parallel()

	eligibility := NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "*")
	eligibility.Allow(agent.StateAct, "write_file")

	if !eligibility.HasWildcard(agent.StateExplore) {
		t.Error("explore should have wildcard")
	}
	if eligibility.HasWildcard(agent.StateAct) {
		t.Error("act should not have wildcard")
	}
	if eligibility.HasWildcard(agent.StateIntake) {
		t.Error("intake (unconfigured) should not have wildcard")
	}
}

func TestNewDefaultToolEligibility(t *testing.T) {
	t.Parallel()

	eligibility := NewDefaultToolEligibility()
	if eligibility == nil {
		t.Fatal("NewDefaultToolEligibility() returned nil")
	}

	// Explore, decide, act, validate should allow any tool
	wildcardStates := []agent.State{
		agent.StateExplore, agent.StateDecide,
		agent.StateAct, agent.StateValidate,
	}
	for _, state := range wildcardStates {
		if !eligibility.IsAllowed(state, "any_tool") {
			t.Errorf("state %s should allow any tool via wildcard", state)
		}
		if !eligibility.HasWildcard(state) {
			t.Errorf("state %s should have wildcard", state)
		}
	}

	// Intake should NOT allow tools
	if eligibility.IsAllowed(agent.StateIntake, "any_tool") {
		t.Error("intake should not allow tools")
	}

	// Terminal states should NOT allow tools
	if eligibility.IsAllowed(agent.StateDone, "any_tool") {
		t.Error("done should not allow tools")
	}
	if eligibility.IsAllowed(agent.StateFailed, "any_tool") {
		t.Error("failed should not allow tools")
	}
}

func TestToolEligibility_WildcardWithExplicitTools(t *testing.T) {
	t.Parallel()

	// Wildcard and explicit tools can coexist
	eligibility := NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "*")
	eligibility.Allow(agent.StateExplore, "read_file")

	if !eligibility.IsAllowed(agent.StateExplore, "read_file") {
		t.Error("explicit tool should still be allowed")
	}
	if !eligibility.IsAllowed(agent.StateExplore, "other_tool") {
		t.Error("wildcard should allow non-explicit tools too")
	}
}

func TestNewStateTransitions(t *testing.T) {
	t.Parallel()

	transitions := NewStateTransitions()
	if transitions == nil {
		t.Fatal("NewStateTransitions() returned nil")
	}

	// Initially no transitions are allowed
	if transitions.CanTransition(agent.StateIntake, agent.StateExplore) {
		t.Error("No transitions should be allowed initially")
	}
}

func TestStateTransitions_Allow(t *testing.T) {
	t.Parallel()

	transitions := NewStateTransitions()

	// Allow a transition
	result := transitions.Allow(agent.StateIntake, agent.StateExplore)
	if result != transitions {
		t.Error("Allow() should return the transitions for chaining")
	}

	// Transition should be allowed
	if !transitions.CanTransition(agent.StateIntake, agent.StateExplore) {
		t.Error("intake -> explore should be allowed")
	}

	// Reverse should NOT be allowed
	if transitions.CanTransition(agent.StateExplore, agent.StateIntake) {
		t.Error("explore -> intake should NOT be allowed")
	}
}

func TestStateTransitions_AllowMultiple(t *testing.T) {
	t.Parallel()

	transitions := NewStateTransitions().
		Allow(agent.StateIntake, agent.StateExplore).
		Allow(agent.StateIntake, agent.StateFailed).
		Allow(agent.StateExplore, agent.StateDecide)

	// All configured transitions should be allowed
	if !transitions.CanTransition(agent.StateIntake, agent.StateExplore) {
		t.Error("intake -> explore should be allowed")
	}
	if !transitions.CanTransition(agent.StateIntake, agent.StateFailed) {
		t.Error("intake -> failed should be allowed")
	}
	if !transitions.CanTransition(agent.StateExplore, agent.StateDecide) {
		t.Error("explore -> decide should be allowed")
	}

	// Non-configured transitions should not be allowed
	if transitions.CanTransition(agent.StateIntake, agent.StateDecide) {
		t.Error("intake -> decide should NOT be allowed")
	}
}

func TestStateTransitions_AllowedTransitions(t *testing.T) {
	t.Parallel()

	t.Run("returns transitions from state", func(t *testing.T) {
		t.Parallel()

		transitions := NewStateTransitions().
			Allow(agent.StateIntake, agent.StateExplore).
			Allow(agent.StateIntake, agent.StateFailed)

		allowed := transitions.AllowedTransitions(agent.StateIntake)
		if len(allowed) != 2 {
			t.Errorf("AllowedTransitions() returned %d states, want 2", len(allowed))
		}
	})

	t.Run("returns nil for unknown state", func(t *testing.T) {
		t.Parallel()

		transitions := NewStateTransitions()
		allowed := transitions.AllowedTransitions(agent.StateIntake)
		if allowed != nil {
			t.Errorf("AllowedTransitions() for unknown state should return nil, got %v", allowed)
		}
	})
}

func TestDefaultTransitions(t *testing.T) {
	t.Parallel()

	transitions := DefaultTransitions()
	if transitions == nil {
		t.Fatal("DefaultTransitions() returned nil")
	}

	// Test canonical transitions that should be allowed
	allowedTransitions := []struct {
		from, to agent.State
	}{
		{agent.StateIntake, agent.StateExplore},
		{agent.StateIntake, agent.StateFailed},
		{agent.StateExplore, agent.StateDecide},
		{agent.StateExplore, agent.StateFailed},
		{agent.StateDecide, agent.StateAct},
		{agent.StateDecide, agent.StateDone},
		{agent.StateDecide, agent.StateFailed},
		{agent.StateAct, agent.StateValidate},
		{agent.StateAct, agent.StateFailed},
		{agent.StateValidate, agent.StateDone},
		{agent.StateValidate, agent.StateExplore}, // Loop back
		{agent.StateValidate, agent.StateFailed},
	}

	for _, tt := range allowedTransitions {
		if !transitions.CanTransition(tt.from, tt.to) {
			t.Errorf("DefaultTransitions: %s -> %s should be allowed", tt.from, tt.to)
		}
	}

	// Test transitions that should NOT be allowed
	disallowedTransitions := []struct {
		from, to agent.State
	}{
		{agent.StateIntake, agent.StateDecide},
		{agent.StateIntake, agent.StateAct},
		{agent.StateIntake, agent.StateDone},
		{agent.StateExplore, agent.StateAct},
		{agent.StateExplore, agent.StateDone},
		{agent.StateDecide, agent.StateExplore},
		{agent.StateAct, agent.StateExplore},
		{agent.StateDone, agent.StateIntake},
		{agent.StateFailed, agent.StateIntake},
	}

	for _, tt := range disallowedTransitions {
		if transitions.CanTransition(tt.from, tt.to) {
			t.Errorf("DefaultTransitions: %s -> %s should NOT be allowed", tt.from, tt.to)
		}
	}
}

func TestDefaultTransitions_TerminalStates(t *testing.T) {
	t.Parallel()

	transitions := DefaultTransitions()

	// Terminal states should not have any outgoing transitions
	terminalStates := []agent.State{agent.StateDone, agent.StateFailed}
	allStates := []agent.State{
		agent.StateIntake, agent.StateExplore, agent.StateDecide,
		agent.StateAct, agent.StateValidate, agent.StateDone, agent.StateFailed,
	}

	for _, terminal := range terminalStates {
		allowed := transitions.AllowedTransitions(terminal)
		if len(allowed) > 0 {
			t.Errorf("Terminal state %s should have no outgoing transitions, got %v", terminal, allowed)
		}

		for _, state := range allStates {
			if transitions.CanTransition(terminal, state) {
				t.Errorf("Should not be able to transition from terminal state %s to %s", terminal, state)
			}
		}
	}
}

func TestDefaultTransitions_EveryStateReachesFailed(t *testing.T) {
	t.Parallel()

	transitions := DefaultTransitions()

	// Every non-terminal state should be able to transition to failed
	nonTerminalStates := []agent.State{
		agent.StateIntake, agent.StateExplore, agent.StateDecide,
		agent.StateAct, agent.StateValidate,
	}

	for _, state := range nonTerminalStates {
		if !transitions.CanTransition(state, agent.StateFailed) {
			t.Errorf("State %s should be able to transition to failed", state)
		}
	}
}
