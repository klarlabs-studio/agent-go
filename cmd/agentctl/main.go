// Package main provides the agentctl CLI for the agent-go runtime.
//
// agentctl is a command-line tool for running, validating, and visualizing
// agent configurations. It supports interactive REPL mode for step-by-step
// agent execution with human-driven planning decisions.
package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
