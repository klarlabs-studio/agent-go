// Package main demonstrates the absolute minimum working agent.
// This is the simplest possible agent-go example.
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
	// 1. Create a simple tool that echoes input
	echoTool := agent.NewToolBuilder("echo").
		WithDescription("Echoes the input message").
		WithAnnotations(agent.Annotations{ReadOnly: true}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			output, err := json.Marshal(map[string]string{
				"echoed": in.Message,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to marshal output: %w", err)
			}
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// 2. Create a scripted planner with predetermined steps
	// Note: State flow must follow canonical transitions:
	// intake -> explore -> decide -> done
	planner := agent.NewScriptedPlanner(
		agent.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("echo", json.RawMessage(`{"message":"Hello, Agent!"}`), "echoing"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "gathered info"),
		},
		agent.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("done", json.RawMessage(`{"status":"complete"}`)),
		},
	)

	// 3. Set up tool eligibility
	eligibility := agent.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "echo")

	// 4. Build the engine
	engine, err := agent.New(
		agent.WithTool(echoTool),
		agent.WithPlanner(planner),
		agent.WithToolEligibility(eligibility),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 5. Run the agent
	run, err := engine.Run(context.Background(), "Echo a message")
	if err != nil {
		log.Fatal(err)
	}

	// 6. Check results
	fmt.Println("=== Minimal Agent Example ===")
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Result: %s\n", run.Result)
}
