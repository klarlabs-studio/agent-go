package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	infraconfig "github.com/felixgeelhaar/agent-go/infrastructure/config"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate an agent configuration file",
		Long: `Validate loads and validates an agent configuration file (YAML or JSON).
Reports any errors found in the configuration, including missing required
fields, invalid values, and structural issues.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return executeValidate(args[0])
		},
	}

	return cmd
}

func executeValidate(path string) error {
	loader := infraconfig.NewLoaderWithOptions(
		infraconfig.WithValidation(true),
	)

	cfg, err := loader.LoadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Validation FAILED: %v\n", err)
		return err
	}

	_, _ = fmt.Fprintf(os.Stdout, "Configuration valid.\n")
	_, _ = fmt.Fprintf(os.Stdout, "  Name:    %s\n", cfg.Name)
	_, _ = fmt.Fprintf(os.Stdout, "  Version: %s\n", cfg.Version)
	if cfg.Description != "" {
		_, _ = fmt.Fprintf(os.Stdout, "  Description: %s\n", cfg.Description)
	}
	if cfg.Agent.MaxSteps > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "  Max Steps: %d\n", cfg.Agent.MaxSteps)
	}
	if len(cfg.Tools.Eligibility) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "  Eligibility rules: %d states\n", len(cfg.Tools.Eligibility))
	}
	if len(cfg.Policy.Budgets) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "  Budgets: %v\n", cfg.Policy.Budgets)
	}

	return nil
}
