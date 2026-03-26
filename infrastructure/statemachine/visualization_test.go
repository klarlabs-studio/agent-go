package statemachine

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

func TestExportDOT_Canonical(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineDefinition()
	if err != nil {
		t.Fatalf("NewAgentMachineDefinition() error = %v", err)
	}

	dot := def.ExportDOT()

	// Verify it is valid DOT
	if !strings.HasPrefix(dot, "digraph agent {") {
		t.Error("DOT output should start with 'digraph agent {'")
	}
	if !strings.HasSuffix(dot, "}\n") {
		t.Error("DOT output should end with '}'")
	}

	// Verify all canonical states are present
	for _, s := range agent.AllStates() {
		if !strings.Contains(dot, string(s)) {
			t.Errorf("DOT output should contain state %q", s)
		}
	}

	// Verify terminal states have double circle shape
	if !strings.Contains(dot, "done [shape=doublecircle]") {
		t.Error("DOT output should mark 'done' as doublecircle")
	}
	if !strings.Contains(dot, "failed [shape=doublecircle]") {
		t.Error("DOT output should mark 'failed' as doublecircle")
	}

	// Verify act state has special fill
	if !strings.Contains(dot, "act [") && !strings.Contains(dot, "fillcolor") {
		t.Error("DOT output should highlight 'act' state for side effects")
	}

	// Verify transitions are present
	expectedTransitions := []string{
		"intake -> explore",
		"intake -> failed",
		"explore -> decide",
		"decide -> act",
		"act -> validate",
		"validate -> done",
		"validate -> explore",
	}

	for _, tr := range expectedTransitions {
		if !strings.Contains(dot, tr) {
			t.Errorf("DOT output should contain transition %q", tr)
		}
	}

	// Verify guard annotations
	if !strings.Contains(dot, "canTransition") {
		t.Error("DOT output should include guard names")
	}
	if !strings.Contains(dot, "budgetAvailable") {
		t.Error("DOT output should include budgetAvailable guard for ACT transition")
	}
}

func TestExportDOT_CustomStates(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineBuilder().
		WithCustomState(agent.CustomState{
			Name:              agent.State("review"),
			AllowsSideEffects: false,
			Terminal:          false,
		}).
		WithCustomState(agent.CustomState{
			Name:     agent.State("cancelled"),
			Terminal: true,
		}).
		WithCustomTransition(agent.StateValidate, agent.State("review")).
		WithCustomTransition(agent.State("review"), agent.StateExplore).
		WithCustomTransition(agent.StateDecide, agent.State("cancelled")).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	dot := def.ExportDOT()

	// Verify custom states appear
	if !strings.Contains(dot, "review") {
		t.Error("DOT output should contain custom state 'review'")
	}
	if !strings.Contains(dot, "cancelled") {
		t.Error("DOT output should contain custom state 'cancelled'")
	}

	// Verify custom terminal state has double circle
	if !strings.Contains(dot, "cancelled [shape=doublecircle]") {
		t.Error("DOT output should mark 'cancelled' as doublecircle")
	}

	// Verify custom transitions
	if !strings.Contains(dot, "validate -> review") {
		t.Error("DOT output should contain transition 'validate -> review'")
	}
	if !strings.Contains(dot, "review -> explore") {
		t.Error("DOT output should contain transition 'review -> explore'")
	}
	if !strings.Contains(dot, "decide -> cancelled") {
		t.Error("DOT output should contain transition 'decide -> cancelled'")
	}
}

func TestExportMermaid_Canonical(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineDefinition()
	if err != nil {
		t.Fatalf("NewAgentMachineDefinition() error = %v", err)
	}

	mermaid := def.ExportMermaid()

	// Verify header
	if !strings.HasPrefix(mermaid, "stateDiagram-v2") {
		t.Error("Mermaid output should start with 'stateDiagram-v2'")
	}

	// Verify initial transition
	if !strings.Contains(mermaid, "[*] --> intake") {
		t.Error("Mermaid output should have initial transition to intake")
	}

	// Verify terminal transitions
	if !strings.Contains(mermaid, "done --> [*]") {
		t.Error("Mermaid output should have done -> [*]")
	}
	if !strings.Contains(mermaid, "failed --> [*]") {
		t.Error("Mermaid output should have failed -> [*]")
	}

	// Verify key transitions
	expectedTransitions := []string{
		"intake --> explore : EXPLORE",
		"explore --> decide : DECIDE",
		"decide --> act : ACT",
		"act --> validate : VALIDATE",
		"validate --> done : DONE",
		"validate --> explore : EXPLORE",
	}

	for _, tr := range expectedTransitions {
		if !strings.Contains(mermaid, tr) {
			t.Errorf("Mermaid output should contain transition %q", tr)
		}
	}

	// Verify side-effect note
	if !strings.Contains(mermaid, "note right of act : Side effects allowed") {
		t.Error("Mermaid output should note side effects for act state")
	}
}

func TestExportMermaid_CustomStates(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineBuilder().
		WithCustomState(agent.CustomState{
			Name:              agent.State("execute"),
			AllowsSideEffects: true,
			Terminal:          false,
		}).
		WithCustomState(agent.CustomState{
			Name:     agent.State("aborted"),
			Terminal: true,
		}).
		WithCustomTransition(agent.StateDecide, agent.State("execute")).
		WithCustomTransition(agent.State("execute"), agent.StateValidate).
		WithCustomTransition(agent.StateAct, agent.State("aborted")).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	mermaid := def.ExportMermaid()

	// Custom transitions present
	if !strings.Contains(mermaid, "decide --> execute") {
		t.Error("Mermaid should contain transition 'decide --> execute'")
	}
	if !strings.Contains(mermaid, "execute --> validate") {
		t.Error("Mermaid should contain transition 'execute --> validate'")
	}
	if !strings.Contains(mermaid, "act --> aborted") {
		t.Error("Mermaid should contain transition 'act --> aborted'")
	}

	// Custom terminal state end transition
	if !strings.Contains(mermaid, "aborted --> [*]") {
		t.Error("Mermaid should have aborted -> [*] for custom terminal state")
	}

	// Side effects note for custom state
	if !strings.Contains(mermaid, "note right of execute : Side effects allowed") {
		t.Error("Mermaid should note side effects for custom 'execute' state")
	}
}

func TestExportDOT_SideEffectHighlight(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineBuilder().
		WithCustomState(agent.CustomState{
			Name:              agent.State("deploy"),
			AllowsSideEffects: true,
		}).
		WithCustomTransition(agent.StateAct, agent.State("deploy")).
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	dot := def.ExportDOT()

	// Both act and deploy should be highlighted
	if !strings.Contains(dot, "act [") {
		t.Error("act state should be in DOT output with attributes")
	}
	if !strings.Contains(dot, "deploy [") {
		t.Error("deploy state should be in DOT output with side-effect styling")
	}
}

func TestExportDOT_Deterministic(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineDefinition()
	if err != nil {
		t.Fatalf("NewAgentMachineDefinition() error = %v", err)
	}

	// Multiple calls should produce identical output
	dot1 := def.ExportDOT()
	dot2 := def.ExportDOT()

	if dot1 != dot2 {
		t.Error("ExportDOT() should be deterministic")
	}
}

func TestExportMermaid_Deterministic(t *testing.T) {
	t.Parallel()

	def, err := NewAgentMachineDefinition()
	if err != nil {
		t.Fatalf("NewAgentMachineDefinition() error = %v", err)
	}

	m1 := def.ExportMermaid()
	m2 := def.ExportMermaid()

	if m1 != m2 {
		t.Error("ExportMermaid() should be deterministic")
	}
}
