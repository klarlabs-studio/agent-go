package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	infraconfig "github.com/felixgeelhaar/agent-go/infrastructure/config"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

func newREPLCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "repl",
		Short: "Interactive REPL for step-by-step agent execution",
		Long: `The REPL (Read-Eval-Print Loop) provides an interactive mode where you
control the agent's decisions manually. At each step, the current state,
allowed tools, and evidence are displayed, and you enter the next decision.

Commands:
  call <tool> <json>       Execute a tool with the given JSON input
  transition <state>       Transition to the given state
  finish <result>          Complete successfully with the given result
  fail <reason>            Terminate with the given failure reason
  help                     Show available commands
  quit                     Exit the REPL`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeREPL(cmd.Context(), configPath)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to agent configuration file (optional)")

	return cmd
}

func executeREPL(ctx context.Context, configPath string) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var opts []api.Option

	// Load config if provided.
	if configPath != "" {
		loader := infraconfig.NewLoader()
		cfg, err := loader.LoadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		opts = buildEngineOptions(cfg, 0)
	} else {
		// Use sensible defaults when no config is provided.
		opts = append(opts,
			api.WithToolEligibility(api.NewDefaultToolEligibility()),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(100),
		)
	}

	// Create interactive planner.
	ip := newInteractivePlanner(os.Stdin, os.Stdout)
	opts = append(opts, api.WithPlanner(ip))

	engine, err := api.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, "agent-go Interactive REPL")
	_, _ = fmt.Fprintln(os.Stdout, "Type 'help' for available commands, 'quit' to exit.")
	_, _ = fmt.Fprintln(os.Stdout)

	// Prompt for goal.
	_, _ = fmt.Fprint(os.Stdout, "Goal: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}
	goal := strings.TrimSpace(scanner.Text())
	if goal == "" {
		goal = "interactive session"
	}

	// Run the engine. The interactive planner blocks in Plan() waiting for
	// user input via a channel, so the engine goroutine blocks naturally.
	run, err := engine.Run(ctx, goal)
	if err != nil {
		if errors.Is(err, errREPLQuit) {
			_, _ = fmt.Fprintln(os.Stdout, "\nSession ended.")
			return nil
		}
		return fmt.Errorf("agent execution failed: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout)
	printRunResult(run)
	return nil
}

// errREPLQuit signals the user wants to exit the REPL.
var errREPLQuit = errors.New("repl: quit requested")

// interactivePlanner implements planner.Planner by reading decisions from
// user input. It blocks in Plan() until the user provides a command.
type interactivePlanner struct {
	scanner *bufio.Scanner
	out     *os.File
}

func newInteractivePlanner(in *os.File, out *os.File) *interactivePlanner {
	return &interactivePlanner{
		scanner: bufio.NewScanner(in),
		out:     out,
	}
}

// Plan implements planner.Planner. It prints the current state and waits
// for user input to construct a decision.
func (p *interactivePlanner) Plan(ctx context.Context, req planner.PlanRequest) (agent.Decision, error) {
	// Print current state context.
	_, _ = fmt.Fprintf(p.out, "\n--- Step ---\n")
	_, _ = fmt.Fprintf(p.out, "  State:    %s\n", req.CurrentState)
	_, _ = fmt.Fprintf(p.out, "  Goal:     %s\n", req.Goal)

	if len(req.AllowedTools) > 0 {
		_, _ = fmt.Fprintf(p.out, "  Tools:    %s\n", strings.Join(req.AllowedTools, ", "))
	}

	if len(req.Evidence) > 0 {
		_, _ = fmt.Fprintf(p.out, "  Evidence: %d entries\n", len(req.Evidence))
		last := req.Evidence[len(req.Evidence)-1]
		_, _ = fmt.Fprintf(p.out, "    last: [%s] %s: %s\n",
			last.Type, last.Source, truncate(string(last.Content), 80))
	}

	if len(req.Vars) > 0 {
		_, _ = fmt.Fprintf(p.out, "  Vars:     %v\n", req.Vars)
	}

	// Read user input.
	for {
		select {
		case <-ctx.Done():
			return agent.Decision{}, ctx.Err()
		default:
		}

		_, _ = fmt.Fprintf(p.out, "\n> ")
		if !p.scanner.Scan() {
			// EOF or error.
			return agent.Decision{}, errREPLQuit
		}

		line := strings.TrimSpace(p.scanner.Text())
		if line == "" {
			continue
		}

		decision, err := parseREPLCommand(line)
		if err != nil {
			_, _ = fmt.Fprintf(p.out, "  Error: %v\n", err)
			continue
		}

		return decision, nil
	}
}

func parseREPLCommand(line string) (agent.Decision, error) {
	parts := strings.SplitN(line, " ", 3)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "call":
		return parseCallCommand(parts)
	case "transition", "trans", "t":
		return parseTransitionCommand(parts)
	case "finish", "done":
		return parseFinishCommand(parts)
	case "fail":
		return parseFailCommand(parts)
	case "quit", "exit", "q":
		return agent.Decision{}, errREPLQuit
	case "help", "h", "?":
		printREPLHelp(os.Stdout)
		return agent.Decision{}, fmt.Errorf("enter a command")
	default:
		return agent.Decision{}, fmt.Errorf("unknown command: %s (type 'help' for usage)", cmd)
	}
}

func parseCallCommand(parts []string) (agent.Decision, error) {
	if len(parts) < 2 {
		return agent.Decision{}, fmt.Errorf("usage: call <tool> [<json-input>]")
	}

	toolName := parts[1]
	var input json.RawMessage
	if len(parts) >= 3 {
		raw := parts[2]
		if !json.Valid([]byte(raw)) {
			return agent.Decision{}, fmt.Errorf("invalid JSON input: %s", raw)
		}
		input = json.RawMessage(raw)
	} else {
		input = json.RawMessage(`{}`)
	}

	return agent.NewCallToolDecision(toolName, input, "repl"), nil
}

func parseTransitionCommand(parts []string) (agent.Decision, error) {
	if len(parts) < 2 {
		return agent.Decision{}, fmt.Errorf("usage: transition <state>")
	}

	state := agent.State(parts[1])
	reason := "repl transition"
	if len(parts) >= 3 {
		reason = parts[2]
	}

	return agent.NewTransitionDecision(state, reason), nil
}

func parseFinishCommand(parts []string) (agent.Decision, error) {
	summary := "completed via repl"
	var result json.RawMessage

	if len(parts) >= 2 {
		// Check if second part is JSON.
		rest := strings.Join(parts[1:], " ")
		if json.Valid([]byte(rest)) {
			result = json.RawMessage(rest)
		} else {
			summary = rest
		}
	}

	return agent.NewFinishDecision(summary, result), nil
}

func parseFailCommand(parts []string) (agent.Decision, error) {
	reason := "failed via repl"
	if len(parts) >= 2 {
		reason = strings.Join(parts[1:], " ")
	}

	return agent.NewFailDecision(reason, nil), nil
}

func printREPLHelp(out *os.File) {
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Available commands:")
	_, _ = fmt.Fprintln(out, "  call <tool> [<json>]      Execute a tool (default input: {})")
	_, _ = fmt.Fprintln(out, "  transition <state> [reason] Transition to a state")
	_, _ = fmt.Fprintln(out, "  finish [summary|json]     Complete successfully")
	_, _ = fmt.Fprintln(out, "  fail [reason]             Terminate with failure")
	_, _ = fmt.Fprintln(out, "  help                      Show this help")
	_, _ = fmt.Fprintln(out, "  quit                      Exit the REPL")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "States: intake, explore, decide, act, validate, done, failed")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Examples:")
	_, _ = fmt.Fprintln(out, `  call read_file {"path": "/tmp/data.txt"}`)
	_, _ = fmt.Fprintln(out, "  transition explore")
	_, _ = fmt.Fprintln(out, `  finish {"status": "all files processed"}`)
	_, _ = fmt.Fprintln(out, "  fail budget exceeded")
}
