// Package main demonstrates how to use LLM planners for intelligent planning.
// Shows how to integrate with contrib/planner-llm for real LLM backends.
//
// Note: LLM provider implementations have been moved to contrib/planner-llm.
// To use real LLM planners, import:
//
//	import "go.klarlabs.de/agent/contrib/planner-llm"
//
// This example uses the ScriptedPlanner to demonstrate the planning interface
// without requiring external API keys.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"go.klarlabs.de/agent/domain/tool"
	agent "go.klarlabs.de/agent/interfaces/api"
)

func main() {
	// ============================================
	// Create tools for the agent
	// ============================================

	calculateTool := agent.NewToolBuilder("calculate").
		WithDescription("Performs basic arithmetic. Supports add, subtract, multiply, divide operations.").
		WithAnnotations(agent.Annotations{
			ReadOnly:   true,
			Idempotent: true,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"operation": {"type": "string", "enum": ["add", "subtract", "multiply", "divide"]},
				"a": {"type": "number"},
				"b": {"type": "number"}
			},
			"required": ["operation", "a", "b"]
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Operation string  `json:"operation"`
				A         float64 `json:"a"`
				B         float64 `json:"b"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			var result float64
			switch in.Operation {
			case "add":
				result = in.A + in.B
			case "subtract":
				result = in.A - in.B
			case "multiply":
				result = in.A * in.B
			case "divide":
				if in.B == 0 {
					return tool.Result{}, fmt.Errorf("division by zero")
				}
				result = in.A / in.B
			default:
				return tool.Result{}, fmt.Errorf("unknown operation: %s", in.Operation)
			}

			fmt.Printf("  [calculate] %s(%g, %g) = %g\n", in.Operation, in.A, in.B, result)

			output, _ := json.Marshal(map[string]float64{"result": result})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// ============================================
	// Create a planner
	// ============================================
	//
	// For real LLM planning, use contrib/planner-llm:
	//
	//   import llmplanner "go.klarlabs.de/agent/contrib/planner-llm"
	//
	//   provider := llmplanner.NewAnthropicProvider(llmplanner.AnthropicConfig{
	//       APIKey: os.Getenv("ANTHROPIC_API_KEY"),
	//       Model:  "claude-sonnet-4-20250514",
	//   })
	//
	//   planner := llmplanner.NewLLMPlanner(llmplanner.LLMPlannerConfig{
	//       Provider:    provider,
	//       Temperature: 0.3,
	//       SystemPrompt: "You are a helpful calculator agent...",
	//   })
	//
	// This example uses ScriptedPlanner to demonstrate the flow.

	fmt.Println("=== Planner Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates the planning interface using ScriptedPlanner.")
	fmt.Println("For real LLM planning, use contrib/planner-llm with your API keys.")
	fmt.Println()

	planner := agent.NewScriptedPlanner(
		agent.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting calculation"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: agent.NewCallToolDecision("calculate",
				json.RawMessage(`{"operation":"multiply","a":15,"b":7}`),
				"multiplying 15 by 7"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "calculation complete"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision: agent.NewFinishDecision("The result of 15 × 7 = 105",
				json.RawMessage(`{"answer":105}`)),
		},
	)

	// ============================================
	// Build and run the engine
	// ============================================

	eligibility := agent.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "calculate")
	eligibility.Allow(agent.StateAct, "calculate")

	engine, err := agent.New(
		agent.WithTool(calculateTool),
		agent.WithPlanner(planner),
		agent.WithToolEligibility(eligibility),
		agent.WithBudget("tool_calls", 10),
		agent.WithMaxSteps(20),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Run with a math problem
	goal := "What is 15 multiplied by 7?"
	fmt.Printf("Goal: %s\n", goal)
	fmt.Println()

	run, err := engine.Run(context.Background(), goal)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println("=== Result ===")
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Steps: %d\n", len(run.Evidence))
	if run.Result != nil {
		fmt.Printf("Result: %s\n", string(run.Result))
	}
	if run.Status == agent.StatusFailed {
		fmt.Printf("Failure: %s\n", run.Error)
	}
}
