// Package main demonstrates the full agent-go platform in a single example.
//
// This flagship example shows:
//   - Multi-agent coordination (coordinator delegates to researcher + executor)
//   - Shared task context for cross-agent state
//   - Policy enforcement (budgets, tool eligibility)
//   - Event streaming with real-time output
//   - Run persistence and audit trail
//   - Approval flow for destructive actions
//   - State machine traversal through all canonical states
//
// Run with: go run ./example/flagship
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/felixgeelhaar/agent-go/application"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/run"
	"github.com/felixgeelhaar/agent-go/domain/task"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	infraagent "github.com/felixgeelhaar/agent-go/infrastructure/agent"
	"github.com/felixgeelhaar/agent-go/infrastructure/logging"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

func main() {
	logging.Init(logging.Config{Level: "info", Format: "console", NoColor: false})

	fmt.Println("========================================")
	fmt.Println("  agent-go Flagship Example")
	fmt.Println("  Multi-Agent Coordination Platform")
	fmt.Println("========================================")
	fmt.Println()

	if err := runFlagship(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func runFlagship() error {
	ctx := context.Background()

	// ==========================================
	// 1. Shared Infrastructure
	// ==========================================
	fmt.Println("[1/6] Setting up shared infrastructure...")

	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	taskCtx := task.NewContext("flagship-task-1", "")

	// Set shared variables for all agents
	taskCtx.SetVar("project", "agent-go")
	taskCtx.SetVar("max_files", 10)

	fmt.Println("  - EventStore: in-memory (real-time streaming)")
	fmt.Println("  - RunStore: in-memory (persistence)")
	fmt.Println("  - TaskContext: shared state across agents")
	fmt.Println()

	// ==========================================
	// 2. Create the Researcher Agent
	// ==========================================
	fmt.Println("[2/6] Creating researcher agent...")

	researchTool := api.NewToolBuilder("analyze_data").
		WithDescription("Analyzes data and produces insights").
		WithAnnotations(api.Annotations{ReadOnly: true, Cacheable: true}).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var req struct {
				Topic string `json:"topic"`
			}
			_ = json.Unmarshal(input, &req)

			// Simulate research
			findings := map[string]any{
				"topic":      req.Topic,
				"findings":   []string{"insight-1: strong growth", "insight-2: risk identified", "insight-3: opportunity found"},
				"confidence": 0.87,
				"sources":    3,
			}
			out, _ := json.Marshal(findings)
			return tool.Result{Output: out}, nil
		}).
		MustBuild()

	researchInput, _ := json.Marshal(map[string]string{"topic": "quarterly performance"})
	researchResult, _ := json.Marshal(map[string]string{"summary": "research complete"})

	researchPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "begin research")},
		planner.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("analyze_data", researchInput, "gather insights")},
		planner.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "evaluate findings")},
		planner.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewFinishDecision("research complete", researchResult)},
	)

	researchEngine, err := application.NewEngine(application.EngineConfig{
		Registry:    createRegistry(researchTool),
		Planner:     researchPlanner,
		Eligibility: policy.NewDefaultToolEligibility(),
		Transitions: policy.DefaultTransitions(),
		EventStore:  eventStore,
		RunStore:    runStore,
		MaxSteps:    20,
	})
	if err != nil {
		return fmt.Errorf("researcher engine: %w", err)
	}

	fmt.Println("  - Tools: analyze_data (read-only, cacheable)")
	fmt.Println("  - Planner: scripted (intake -> explore -> decide -> done)")
	fmt.Println()

	// ==========================================
	// 3. Create the Executor Agent
	// ==========================================
	fmt.Println("[3/6] Creating executor agent...")

	writeTool := api.NewToolBuilder("write_report").
		WithDescription("Writes a formatted report").
		WithAnnotations(api.Annotations{Destructive: true}).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var req struct {
				Title   string `json:"title"`
				Content string `json:"content"`
			}
			_ = json.Unmarshal(input, &req)

			report := map[string]any{
				"title":      req.Title,
				"content":    req.Content,
				"generated":  time.Now().Format(time.RFC3339),
				"word_count": len(req.Content) / 5,
			}
			out, _ := json.Marshal(report)
			return tool.Result{Output: out}, nil
		}).
		MustBuild()

	writeInput, _ := json.Marshal(map[string]string{
		"title":   "Q4 Report",
		"content": "Based on analysis, strong growth observed with identified risks.",
	})
	execResult, _ := json.Marshal(map[string]string{"status": "report generated"})

	execPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "review task")},
		planner.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "plan execution")},
		planner.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewTransitionDecision(agent.StateAct, "execute")},
		planner.ScriptStep{ExpectState: agent.StateAct, Decision: agent.NewCallToolDecision("write_report", writeInput, "generate report")},
		planner.ScriptStep{ExpectState: agent.StateAct, Decision: agent.NewTransitionDecision(agent.StateValidate, "verify output")},
		planner.ScriptStep{ExpectState: agent.StateValidate, Decision: agent.NewFinishDecision("execution complete", execResult)},
	)

	execEngine, err := application.NewEngine(application.EngineConfig{
		Registry:    createRegistry(writeTool),
		Planner:     execPlanner,
		Eligibility: policy.NewDefaultToolEligibility(),
		Transitions: policy.DefaultTransitions(),
		Approver:    policy.NewAutoApprover("system"),
		EventStore:  eventStore,
		RunStore:    runStore,
		MaxSteps:    20,
	})
	if err != nil {
		return fmt.Errorf("executor engine: %w", err)
	}

	fmt.Println("  - Tools: write_report (destructive, requires approval)")
	fmt.Println("  - Approver: auto-approve (use approval-slack for production)")
	fmt.Println()

	// ==========================================
	// 4. Create the Coordinator Agent
	// ==========================================
	fmt.Println("[4/6] Creating coordinator agent with delegation...")

	// Wrap child engines as tools via DelegateTool
	researchDelegate := infraagent.NewDelegateTool(
		"research_agent", "Delegates research tasks to specialist",
		researchEngine, infraagent.WithDelegateTaskContext(taskCtx),
	)
	execDelegate := infraagent.NewDelegateTool(
		"executor_agent", "Delegates execution tasks to specialist",
		execEngine, infraagent.WithDelegateTaskContext(taskCtx),
	)

	delegateResearchInput, _ := json.Marshal(map[string]string{"goal": "Research quarterly performance data"})
	delegateExecInput, _ := json.Marshal(map[string]string{"goal": "Generate the quarterly report"})
	coordResult, _ := json.Marshal(map[string]string{"status": "task completed", "agents_used": "2"})

	coordPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "plan task")},
		// Delegate to researcher
		planner.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("research_agent", delegateResearchInput, "delegate research")},
		planner.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "evaluate research")},
		// Delegate to executor
		planner.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewTransitionDecision(agent.StateAct, "begin execution")},
		planner.ScriptStep{ExpectState: agent.StateAct, Decision: agent.NewCallToolDecision("executor_agent", delegateExecInput, "delegate execution")},
		planner.ScriptStep{ExpectState: agent.StateAct, Decision: agent.NewTransitionDecision(agent.StateValidate, "verify all")},
		planner.ScriptStep{ExpectState: agent.StateValidate, Decision: agent.NewFinishDecision("all tasks complete", coordResult)},
	)

	coordRegistry := api.NewToolRegistry()
	_ = coordRegistry.Register(researchDelegate)
	_ = coordRegistry.Register(execDelegate)

	coordEngine, err := api.New(
		api.WithRegistry(coordRegistry),
		api.WithPlanner(coordPlanner),
		api.WithToolEligibility(api.NewDefaultToolEligibility()),
		api.WithEventStore(eventStore),
		api.WithRunStore(runStore),
		api.WithTaskContext(taskCtx),
		api.WithBudget("tool_calls", 50),
		api.WithMaxSteps(30),
	)
	if err != nil {
		return fmt.Errorf("coordinator engine: %w", err)
	}

	fmt.Println("  - Delegates: research_agent, executor_agent")
	fmt.Println("  - Budget: 50 tool calls")
	fmt.Println("  - TaskContext: shared state propagated to children")
	fmt.Println()

	// ==========================================
	// 5. Stream Execution
	// ==========================================
	fmt.Println("[5/6] Streaming coordinator execution...")
	fmt.Println()

	runID, eventCh, err := coordEngine.Stream(ctx, "Create a comprehensive quarterly report")
	if err != nil {
		return fmt.Errorf("stream: %w", err)
	}

	fmt.Printf("  Run ID: %s\n", runID)
	fmt.Println("  Events:")

	// Consume events in real-time
	eventCount := 0
	for evt := range eventCh {
		eventCount++
		printEvent(evt)
		if evt.Type == event.TypeRunCompleted || evt.Type == event.TypeRunFailed {
			break
		}
	}

	fmt.Printf("\n  Total events: %d\n", eventCount)
	fmt.Println()

	// ==========================================
	// 6. Inspect Results
	// ==========================================
	fmt.Println("[6/6] Inspecting results...")
	fmt.Println()

	// Check shared task context
	sharedEvidence := taskCtx.Evidence()
	fmt.Printf("  Shared evidence items: %d\n", len(sharedEvidence))
	for i, e := range sharedEvidence {
		fmt.Printf("    [%d] %s: %s (%.40s...)\n", i, e.Type, e.Source, string(e.Content))
	}

	// Check persisted runs
	allRuns, _ := runStore.List(ctx, run.ListFilter{Limit: 10})
	fmt.Printf("\n  Persisted runs: %d\n", len(allRuns))
	for _, r := range allRuns {
		parentInfo := ""
		if r.ParentRunID != "" {
			parentInfo = fmt.Sprintf(" (parent: %.20s...)", r.ParentRunID)
		}
		fmt.Printf("    %s: %s | %s%s\n", r.Status, r.Goal[:min(40, len(r.Goal))], r.ID[:20], parentInfo)
	}

	// Check event store
	allEvents, _ := eventStore.LoadEvents(ctx, runID)
	fmt.Printf("\n  Coordinator events: %d\n", len(allEvents))

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  Flagship example completed!")
	fmt.Println("========================================")

	return nil
}

func createRegistry(tools ...tool.Tool) tool.Registry {
	r := memory.NewToolRegistry()
	for _, t := range tools {
		_ = r.Register(t)
	}
	return r
}

func printEvent(evt event.Event) {
	icon := "  "
	switch evt.Type {
	case event.TypeRunStarted:
		icon = "  "
	case event.TypeRunCompleted:
		icon = "  "
	case event.TypeRunFailed:
		icon = "  "
	case event.TypeToolCalled:
		icon = "  "
	case event.TypeToolSucceeded:
		icon = "  "
	case event.TypeStateTransitioned:
		icon = "  "
	case event.TypeDecisionMade:
		icon = "  "
	case event.TypeBudgetConsumed:
		icon = "  "
	case event.TypeEvidenceAdded:
		icon = "  "
	}

	payload := ""
	switch evt.Type {
	case event.TypeToolCalled:
		var p event.ToolCalledPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			payload = fmt.Sprintf("tool=%s state=%s", p.ToolName, p.State)
		}
	case event.TypeToolSucceeded:
		var p event.ToolSucceededPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			payload = fmt.Sprintf("tool=%s duration=%v", p.ToolName, p.Duration)
		}
	case event.TypeStateTransitioned:
		var p event.StateTransitionedPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			payload = fmt.Sprintf("%s -> %s (%s)", p.FromState, p.ToState, p.Reason)
		}
	case event.TypeDecisionMade:
		var p event.DecisionMadePayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			payload = fmt.Sprintf("type=%s", p.DecisionType)
			if p.ToolName != "" {
				payload += fmt.Sprintf(" tool=%s", p.ToolName)
			}
		}
	case event.TypeRunStarted:
		var p event.RunStartedPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			payload = fmt.Sprintf("goal=%.50s", p.Goal)
		}
	case event.TypeRunCompleted:
		payload = "success"
	case event.TypeRunFailed:
		var p event.RunFailedPayload
		if err := evt.UnmarshalPayload(&p); err == nil {
			payload = fmt.Sprintf("error=%s", p.Error)
		}
	default:
		payload = string(evt.Type)
	}

	fmt.Printf("  %s %-22s %s\n", icon, evt.Type, payload)
}
