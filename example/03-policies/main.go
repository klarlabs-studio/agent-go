// Package main demonstrates policy enforcement: budgets and approvals.
// Shows how hard limits prevent runaway agents.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"go.klarlabs.de/agent/domain/tool"
	agent "go.klarlabs.de/agent/interfaces/api"
)

func main() {
	// ============================================
	// Create tools
	// ============================================

	// A simple counting tool
	counter := 0
	countTool := agent.NewToolBuilder("count").
		WithDescription("Increments and returns a counter").
		WithAnnotations(agent.Annotations{
			ReadOnly:   true,
			Idempotent: false, // Each call produces different result
		}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			counter++
			output, _ := json.Marshal(map[string]int{"count": counter})
			fmt.Printf("  [count tool] Counter is now: %d\n", counter)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// A destructive tool that requires approval
	dangerTool := agent.NewToolBuilder("danger").
		WithDescription("A dangerous operation that requires approval").
		WithAnnotations(agent.Annotations{
			ReadOnly:    false,
			Destructive: true,
			RiskLevel:   agent.RiskHigh,
		}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			fmt.Println("  [danger tool] DANGER EXECUTED!")
			output, _ := json.Marshal(map[string]bool{"executed": true})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// ============================================
	// Example 1: Budget Enforcement
	// ============================================
	fmt.Println("=== Example 1: Budget Enforcement ===")
	fmt.Println("Setting budget to 3 tool calls...")
	fmt.Println()

	// Planner that tries to call count 5 times
	// When budget exhausts on call 4, the run will fail
	budgetPlanner := agent.NewScriptedPlanner(
		agent.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "start")},
		agent.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("count", nil, "call 1")},
		agent.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("count", nil, "call 2")},
		agent.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("count", nil, "call 3")},
		agent.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("count", nil, "call 4")}, // Budget exhausted here
		agent.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("count", nil, "call 5")},
		agent.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "ready")},
		agent.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewFinishDecision("done", nil)},
	)

	eligibility := agent.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "count")
	eligibility.Allow(agent.StateAct, "danger")

	engine1, _ := agent.New(
		agent.WithTool(countTool),
		agent.WithPlanner(budgetPlanner),
		agent.WithToolEligibility(eligibility),
		agent.WithBudget("tool_calls", 3), // Only 3 tool calls allowed!
		agent.WithMaxSteps(10),
	)

	run1, err := engine1.Run(context.Background(), "Count multiple times")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Printf("Run status: %s\n", run1.Status)
	if run1.Status == agent.StatusFailed {
		fmt.Printf("Failure reason: %s\n", run1.Error)
	}
	fmt.Printf("Steps taken: %d (stopped at budget limit)\n", len(run1.Evidence))

	// ============================================
	// Example 2: Approval Workflow
	// ============================================
	fmt.Println()
	fmt.Println("=== Example 2: Approval Workflow ===")
	fmt.Println("Dangerous tools require human approval...")
	fmt.Println()

	// Reset counter
	counter = 0

	// Interactive approver that prompts the user
	interactiveApprover := agent.NewCallbackApprover(func(ctx context.Context, req agent.ApprovalRequest) (bool, error) {
		fmt.Printf("\n  [APPROVAL REQUIRED]\n")
		fmt.Printf("  Tool: %s\n", req.ToolName)
		fmt.Printf("  Risk Level: %s\n", req.RiskLevel)
		fmt.Printf("  Input: %s\n", string(req.Input))
		fmt.Print("  Approve? (y/n): ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		approved := response == "y" || response == "yes"
		if approved {
			fmt.Println("  -> Approved!")
		} else {
			fmt.Println("  -> Denied!")
		}
		return approved, nil
	})

	// State flow: intake -> explore -> decide -> act -> validate -> done
	approvalPlanner := agent.NewScriptedPlanner(
		agent.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "start")},
		agent.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "ready to act")},
		agent.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewTransitionDecision(agent.StateAct, "proceeding")},
		agent.ScriptStep{ExpectState: agent.StateAct, Decision: agent.NewCallToolDecision("danger", json.RawMessage(`{"action":"delete_everything"}`), "dangerous op")},
		agent.ScriptStep{ExpectState: agent.StateAct, Decision: agent.NewTransitionDecision(agent.StateValidate, "verifying")},
		agent.ScriptStep{ExpectState: agent.StateValidate, Decision: agent.NewFinishDecision("done", nil)},
	)

	engine2, _ := agent.New(
		agent.WithTool(dangerTool),
		agent.WithPlanner(approvalPlanner),
		agent.WithToolEligibility(eligibility),
		agent.WithApprover(interactiveApprover),
		agent.WithMaxSteps(10),
	)

	run2, err := engine2.Run(context.Background(), "Do something dangerous")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Printf("Run status: %s\n", run2.Status)
	if run2.Status == agent.StatusFailed {
		fmt.Printf("Failure reason: %s\n", run2.Error)
	}

	// ============================================
	// Summary
	// ============================================
	fmt.Println()
	fmt.Println("=== Policy Summary ===")
	fmt.Println("Budgets: Hard limits that stop execution when exhausted")
	fmt.Println("Approvals: Human sign-off required for destructive operations")
	fmt.Println("Both enforce constraints that the LLM cannot override")
}
