// Package inspector provides inspector infrastructure implementations.
package inspector

import (
	"fmt"
	"strings"

	"go.klarlabs.de/agent/domain/inspector"
)

// DOTFormatter formats state machine data as Graphviz DOT.
type DOTFormatter struct{}

// NewDOTFormatter creates a new DOT formatter.
func NewDOTFormatter() *DOTFormatter {
	return &DOTFormatter{}
}

// Format formats the data as DOT.
func (f *DOTFormatter) Format(data any) ([]byte, error) {
	sm, ok := data.(*inspector.StateMachineExport)
	if !ok {
		return nil, inspector.ErrInvalidFormat
	}

	return f.formatStateMachine(sm), nil
}

// FormatType returns the format type.
func (f *DOTFormatter) FormatType() inspector.ExportFormat {
	return inspector.FormatDOT
}

func (f *DOTFormatter) formatStateMachine(sm *inspector.StateMachineExport) []byte {
	var b strings.Builder

	b.WriteString("digraph AgentStateMachine {\n")
	b.WriteString("  rankdir=TB;\n")
	b.WriteString("  node [shape=box, style=rounded];\n")
	b.WriteString("\n")

	// Define nodes
	for _, state := range sm.States {
		attrs := []string{
			fmt.Sprintf(`label="%s"`, state.Name),
		}

		if state.IsTerminal {
			attrs = append(attrs, "style=\"rounded,filled\"")
			if state.Name == "done" {
				attrs = append(attrs, "fillcolor=lightgreen")
			} else {
				attrs = append(attrs, "fillcolor=lightcoral")
			}
		} else if state.AllowsSideEffects {
			attrs = append(attrs, "style=\"rounded,filled\"", "fillcolor=lightyellow")
		}

		fmt.Fprintf(&b, "  %s [%s];\n", sanitizeDOTID(string(state.Name)), strings.Join(attrs, ", "))
	}

	b.WriteString("\n")

	// Define edges
	for _, trans := range sm.Transitions {
		attrs := []string{}
		if trans.Label != "" {
			attrs = append(attrs, fmt.Sprintf(`label="%s"`, trans.Label))
		}
		if trans.Count > 0 {
			attrs = append(attrs, fmt.Sprintf(`penwidth=%d`, min(trans.Count/10+1, 5)))
		}

		attrStr := ""
		if len(attrs) > 0 {
			attrStr = fmt.Sprintf(" [%s]", strings.Join(attrs, ", "))
		}

		fmt.Fprintf(&b, "  %s -> %s%s;\n",
			sanitizeDOTID(string(trans.From)),
			sanitizeDOTID(string(trans.To)),
			attrStr,
		)
	}

	b.WriteString("}\n")

	return []byte(b.String())
}

func sanitizeDOTID(s string) string {
	// Replace any characters that are invalid in DOT identifiers
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// Ensure DOTFormatter implements inspector.Formatter
var _ inspector.Formatter = (*DOTFormatter)(nil)
