package main

import (
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agentctl",
		Short: "CLI for the agent-go state-driven agent runtime",
		Long: `agentctl is a command-line tool for the agent-go framework.

It provides commands for running agents, validating configurations,
visualizing state machines, and interacting with agents in a REPL.

Trust is the product. Intelligence is constrained by design, not hope.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newVisualizeCmd())
	cmd.AddCommand(newREPLCmd())

	return cmd
}
