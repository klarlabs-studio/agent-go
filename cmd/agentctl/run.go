package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	domainconfig "github.com/felixgeelhaar/agent-go/domain/config"
	"github.com/felixgeelhaar/agent-go/domain/event"
	infraconfig "github.com/felixgeelhaar/agent-go/infrastructure/config"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

func newRunCmd() *cobra.Command {
	var (
		configPath string
		goal       string
		stream     bool
		maxSteps   int
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an agent with the given configuration and goal",
		Long: `Run executes an agent using the specified YAML/JSON configuration file.
The agent processes the goal through its state machine, executing tools
and transitioning states until reaching a terminal state.

In stream mode, events are printed as they arrive in real time.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if configPath == "" {
				return fmt.Errorf("--config is required")
			}
			if goal == "" {
				return fmt.Errorf("--goal is required")
			}

			return executeRun(cmd.Context(), configPath, goal, stream, maxSteps)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to agent configuration file (YAML/JSON)")
	cmd.Flags().StringVarP(&goal, "goal", "g", "", "Goal for the agent to accomplish")
	cmd.Flags().BoolVarP(&stream, "stream", "s", false, "Stream events as they arrive")
	cmd.Flags().IntVarP(&maxSteps, "max-steps", "m", 0, "Override maximum steps (0 = use config default)")

	return cmd
}

func executeRun(ctx context.Context, configPath, goal string, stream bool, maxSteps int) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	loader := infraconfig.NewLoader()
	cfg, err := loader.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	opts := buildEngineOptions(cfg, maxSteps)

	if stream {
		return executeStream(ctx, opts, goal)
	}
	return executeSynchronous(ctx, opts, goal)
}

func executeSynchronous(ctx context.Context, opts []api.Option, goal string) error {
	engine, err := api.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Running agent with goal: %s\n\n", goal)

	run, err := engine.Run(ctx, goal)
	if err != nil {
		if errors.Is(err, agent.ErrAwaitingHumanInput) && run != nil {
			_, _ = fmt.Fprintf(os.Stdout, "Agent paused for human input.\n")
			_, _ = fmt.Fprintf(os.Stdout, "  Question: %s\n", run.PendingQuestion.Question)
			if len(run.PendingQuestion.Options) > 0 {
				_, _ = fmt.Fprintf(os.Stdout, "  Options: %v\n", run.PendingQuestion.Options)
			}
			return nil
		}
		return fmt.Errorf("agent execution failed: %w", err)
	}

	printRunResult(run)
	return nil
}

func executeStream(ctx context.Context, opts []api.Option, goal string) error {
	eventStore := memory.NewEventStore()
	opts = append(opts, api.WithEventStore(eventStore))

	engine, err := api.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Streaming agent execution for goal: %s\n\n", goal)

	runID, events, err := engine.Stream(ctx, goal)
	if err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Run ID: %s\n\n", runID)

	for evt := range events {
		printEvent(evt)
	}

	return nil
}

func printRunResult(run *agent.Run) {
	_, _ = fmt.Fprintf(os.Stdout, "--- Run Complete ---\n")
	_, _ = fmt.Fprintf(os.Stdout, "  ID:       %s\n", run.ID)
	_, _ = fmt.Fprintf(os.Stdout, "  Status:   %s\n", run.Status)
	_, _ = fmt.Fprintf(os.Stdout, "  State:    %s\n", run.CurrentState)
	_, _ = fmt.Fprintf(os.Stdout, "  Duration: %s\n", run.EndTime.Sub(run.StartTime))

	if len(run.Result) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "  Result:   %s\n", string(run.Result))
	}
	if run.Error != "" {
		_, _ = fmt.Fprintf(os.Stdout, "  Error:    %s\n", run.Error)
	}

	if len(run.Evidence) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\n--- Evidence (%d entries) ---\n", len(run.Evidence))
		for i, ev := range run.Evidence {
			_, _ = fmt.Fprintf(os.Stdout, "  [%d] %s from %s: %s\n",
				i+1, ev.Type, ev.Source, truncate(string(ev.Content), 120))
		}
	}
}

func printEvent(evt event.Event) {
	_, _ = fmt.Fprintf(os.Stdout, "[%s] %s", evt.Type, evt.Timestamp.Format("15:04:05.000"))
	if len(evt.Payload) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, " %s", truncate(string(evt.Payload), 200))
	}
	_, _ = fmt.Fprintln(os.Stdout)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func buildEngineOptions(cfg *domainconfig.AgentConfig, maxSteps int) []api.Option {
	var opts []api.Option

	// Apply eligibility rules from config.
	if len(cfg.Tools.Eligibility) > 0 {
		rules := make(api.EligibilityRules)
		for state, tools := range cfg.Tools.Eligibility {
			rules[agent.State(state)] = tools
		}
		opts = append(opts, api.WithToolEligibility(api.NewToolEligibilityWith(rules)))
	} else {
		opts = append(opts, api.WithToolEligibility(api.NewDefaultToolEligibility()))
	}

	// Apply default transitions.
	opts = append(opts, api.WithTransitions(api.DefaultTransitions()))

	// Apply budgets from config.
	if len(cfg.Policy.Budgets) > 0 {
		opts = append(opts, api.WithBudgets(cfg.Policy.Budgets))
	}

	// CLI flag overrides config max steps.
	steps := cfg.Agent.MaxSteps
	if maxSteps > 0 {
		steps = maxSteps
	}
	if steps > 0 {
		opts = append(opts, api.WithMaxSteps(steps))
	}

	// Apply approval mode.
	if cfg.Policy.Approval.Mode == "auto" || cfg.Policy.Approval.RequireForDestructive {
		opts = append(opts, api.WithApprover(api.AutoApprover()))
	}

	return opts
}
