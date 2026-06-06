// Package inspector provides inspector infrastructure implementations.
package inspector

import (
	"fmt"
	"strings"

	"go.klarlabs.de/agent/domain/inspector"
)

// MermaidFormatter formats state machine data as Mermaid diagram.
type MermaidFormatter struct{}

// NewMermaidFormatter creates a new Mermaid formatter.
func NewMermaidFormatter() *MermaidFormatter {
	return &MermaidFormatter{}
}

// Format formats the data as Mermaid.
func (f *MermaidFormatter) Format(data any) ([]byte, error) {
	sm, ok := data.(*inspector.StateMachineExport)
	if !ok {
		return nil, inspector.ErrInvalidFormat
	}

	return f.formatStateMachine(sm), nil
}

// FormatType returns the format type.
func (f *MermaidFormatter) FormatType() inspector.ExportFormat {
	return inspector.FormatMermaid
}

func (f *MermaidFormatter) formatStateMachine(sm *inspector.StateMachineExport) []byte {
	var b strings.Builder

	b.WriteString("stateDiagram-v2\n")

	// Mark initial state
	fmt.Fprintf(&b, "  [*] --> %s\n", sm.Initial)

	// Define transitions
	for _, trans := range sm.Transitions {
		if trans.Label != "" {
			fmt.Fprintf(&b, "  %s --> %s: %s\n", trans.From, trans.To, trans.Label)
		} else {
			fmt.Fprintf(&b, "  %s --> %s\n", trans.From, trans.To)
		}
	}

	// Mark terminal states
	for _, terminal := range sm.Terminal {
		fmt.Fprintf(&b, "  %s --> [*]\n", terminal)
	}

	// Add state notes for states with side effects
	b.WriteString("\n")
	for _, state := range sm.States {
		if state.AllowsSideEffects {
			fmt.Fprintf(&b, "  note right of %s: Side effects allowed\n", state.Name)
		}
	}

	return []byte(b.String())
}

// Ensure MermaidFormatter implements inspector.Formatter
var _ inspector.Formatter = (*MermaidFormatter)(nil)
