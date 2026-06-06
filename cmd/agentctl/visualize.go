package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"go.klarlabs.de/agent/infrastructure/statemachine"
)

func newVisualizeCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "visualize",
		Short: "Visualize the agent state machine",
		Long: `Visualize generates a diagram of the agent state machine in DOT (Graphviz)
or Mermaid format. The output is written to stdout and can be piped to
rendering tools.

Examples:
  agentctl visualize --format dot | dot -Tpng -o states.png
  agentctl visualize --format mermaid >> README.md`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return executeVisualize(format)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "dot", "Output format: dot or mermaid")

	return cmd
}

func executeVisualize(format string) error {
	def, err := statemachine.NewAgentMachineDefinition()
	if err != nil {
		return fmt.Errorf("failed to create state machine: %w", err)
	}

	switch format {
	case "dot":
		fmt.Print(def.ExportDOT())
	case "mermaid":
		fmt.Print(def.ExportMermaid())
	default:
		return fmt.Errorf("unsupported format: %s (use 'dot' or 'mermaid')", format)
	}

	return nil
}
