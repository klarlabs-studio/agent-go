package statemachine

import (
	"fmt"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
)

// ExportDOT generates a Graphviz DOT representation of the state machine.
// The output includes state nodes with shape annotations for terminal states,
// and labeled edges for transitions including guard and action names.
func (d *MachineDefinition) ExportDOT() string {
	var b strings.Builder

	b.WriteString("digraph agent {\n")
	b.WriteString("    rankdir=LR;\n")
	b.WriteString("    node [shape=box, style=rounded];\n")
	b.WriteString("\n")

	// Emit canonical states
	for _, s := range agent.AllStates() {
		d.writeDOTState(&b, s)
	}

	// Emit custom states
	for _, cs := range d.StateRegistry.All() {
		d.writeDOTState(&b, cs.Name)
	}

	b.WriteString("\n")

	// Emit canonical transitions
	for _, ct := range d.canonicalTransitions {
		d.writeDOTTransition(&b, ct)
	}

	// Emit custom transitions
	for _, ct := range d.CustomTransitions {
		d.writeDOTTransition(&b, ct)
	}

	b.WriteString("}\n")

	return b.String()
}

// writeDOTState writes a single state node definition to the builder.
func (d *MachineDefinition) writeDOTState(b *strings.Builder, s agent.State) {
	attrs := []string{}

	if d.isTerminal(s) {
		attrs = append(attrs, "shape=doublecircle")
	}

	if d.allowsSideEffects(s) {
		attrs = append(attrs, `style="rounded,filled"`, `fillcolor="#ffcccc"`)
	}

	if len(attrs) > 0 {
		fmt.Fprintf(b, "    %s [%s];\n", s, strings.Join(attrs, ", "))
	} else {
		fmt.Fprintf(b, "    %s;\n", s)
	}
}

// writeDOTTransition writes a single transition edge to the builder.
func (d *MachineDefinition) writeDOTTransition(b *strings.Builder, ct CustomTransition) {
	label := string(ct.Event)

	// Add guard info for known guarded transitions
	guards := d.guardsForTransition(ct)
	if len(guards) > 0 {
		label += fmt.Sprintf("\\n[%s]", strings.Join(guards, ", "))
	}

	fmt.Fprintf(b, "    %s -> %s [label=%q];\n", ct.From, ct.To, label)
}

// guardsForTransition returns guard names applicable to a transition.
func (d *MachineDefinition) guardsForTransition(ct CustomTransition) []string {
	var guards []string

	// All non-FAIL transitions to non-terminal states have canTransition guard
	if ct.Event != "FAIL" && !d.isTerminal(ct.To) {
		guards = append(guards, "canTransition")
	}

	// ACT transitions additionally check budget
	if ct.Event == "ACT" {
		guards = append(guards, "budgetAvailable")
	}

	return guards
}

// ExportMermaid generates a Mermaid state diagram representation.
// The output can be rendered in Markdown or any Mermaid-compatible tool.
func (d *MachineDefinition) ExportMermaid() string {
	var b strings.Builder

	b.WriteString("stateDiagram-v2\n")

	// Initial transition
	b.WriteString("    [*] --> intake\n")

	// Emit canonical transitions
	for _, ct := range d.canonicalTransitions {
		d.writeMermaidTransition(&b, ct)
	}

	// Emit custom transitions
	for _, ct := range d.CustomTransitions {
		d.writeMermaidTransition(&b, ct)
	}

	// Terminal states transition to end
	b.WriteString("    done --> [*]\n")
	b.WriteString("    failed --> [*]\n")

	// Custom terminal states
	for _, cs := range d.StateRegistry.All() {
		if cs.Terminal {
			fmt.Fprintf(&b, "    %s --> [*]\n", cs.Name)
		}
	}

	// Add notes for states with side effects
	for _, s := range agent.AllStates() {
		if s.AllowsSideEffects() {
			fmt.Fprintf(&b, "    note right of %s : Side effects allowed\n", s)
		}
	}

	for _, cs := range d.StateRegistry.All() {
		if cs.AllowsSideEffects {
			fmt.Fprintf(&b, "    note right of %s : Side effects allowed\n", cs.Name)
		}
	}

	return b.String()
}

// writeMermaidTransition writes a single transition in Mermaid format.
func (d *MachineDefinition) writeMermaidTransition(b *strings.Builder, ct CustomTransition) {
	fmt.Fprintf(b, "    %s --> %s : %s\n", ct.From, ct.To, ct.Event)
}

// isTerminal checks whether a state is terminal, considering both canonical
// and custom states.
func (d *MachineDefinition) isTerminal(s agent.State) bool {
	if s.IsTerminal() {
		return true
	}
	return d.StateRegistry.IsTerminal(s)
}

// allowsSideEffects checks whether a state allows side effects, considering
// both canonical and custom states.
func (d *MachineDefinition) allowsSideEffects(s agent.State) bool {
	if s.AllowsSideEffects() {
		return true
	}
	return d.StateRegistry.AllowsSideEffects(s)
}
