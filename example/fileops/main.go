// Package main demonstrates the agent-go runtime with file operation tools.
//
// This example shows:
// - Tool registration with annotations
// - State-driven execution
// - Policy enforcement (tool eligibility, transitions)
// - Scripted planner for deterministic execution
// - Budget management
//
// Run with: go run ./example/fileops
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/infrastructure/logging"
	api "go.klarlabs.de/agent/interfaces/api"
)

func main() {
	// Initialize logging
	logging.Init(logging.Config{
		Level:   "info",
		Format:  "console",
		NoColor: false,
	})

	// Create a temporary workspace directory
	workDir, err := os.MkdirTemp("", "fileops-example-*")
	if err != nil {
		fmt.Printf("Failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	// Use explicit cleanup to ensure it runs even on error
	cleanup := func() { _ = os.RemoveAll(workDir) } // Ignore cleanup errors
	defer cleanup()

	fmt.Printf("=== FileOps Agent Example ===\n")
	fmt.Printf("Workspace: %s\n\n", workDir)

	// Run the example
	if err := runExample(workDir); err != nil {
		fmt.Printf("Example failed: %v\n", err)
		cleanup() // Explicit cleanup before exit
		os.Exit(1)
	}

	fmt.Printf("\n=== Example completed successfully! ===\n")
}

func runExample(workDir string) error {
	// Create tool registry with file operation tools
	registry := api.NewToolRegistry()
	if err := registry.Register(NewReadFileTool(workDir)); err != nil {
		return fmt.Errorf("failed to register read_file: %w", err)
	}
	if err := registry.Register(NewWriteFileTool(workDir)); err != nil {
		return fmt.Errorf("failed to register write_file: %w", err)
	}
	if err := registry.Register(NewDeleteFileTool(workDir)); err != nil {
		return fmt.Errorf("failed to register delete_file: %w", err)
	}
	if err := registry.Register(NewListDirTool(workDir)); err != nil {
		return fmt.Errorf("failed to register list_dir: %w", err)
	}

	// Configure tool eligibility per state using imperative builder pattern.
	// This approach is useful when building eligibility dynamically or
	// when you prefer method chaining. For static configuration, consider
	// using NewToolEligibilityWith() with a declarative map instead.
	eligibility := api.NewToolEligibility()
	// Read-only tools allowed in explore state
	eligibility.Allow(agent.StateExplore, "read_file")
	eligibility.Allow(agent.StateExplore, "list_dir")
	// All tools allowed in act state
	eligibility.Allow(agent.StateAct, "read_file")
	eligibility.Allow(agent.StateAct, "write_file")
	eligibility.Allow(agent.StateAct, "delete_file")
	eligibility.Allow(agent.StateAct, "list_dir")
	// Read-only in validate state
	eligibility.Allow(agent.StateValidate, "read_file")
	eligibility.Allow(agent.StateValidate, "list_dir")

	// Create a scripted planner that simulates an agent workflow:
	// 1. List directory (explore)
	// 2. Create a file (act)
	// 3. Read the file back (validate)
	// 4. Finish
	planner := api.NewScriptedPlanner(
		// Start: transition from intake to explore
		api.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    api.NewTransitionDecision(agent.StateExplore, "Begin exploration"),
		},
		// Explore: list directory
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: api.NewCallToolDecision("list_dir",
				json.RawMessage(`{"path": "."}`),
				"Check initial directory contents"),
		},
		// Explore: transition to decide
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    api.NewTransitionDecision(agent.StateDecide, "Ready to decide on action"),
		},
		// Decide: transition to act
		api.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    api.NewTransitionDecision(agent.StateAct, "Proceeding with file creation"),
		},
		// Act: write a file
		api.ScriptStep{
			ExpectState: agent.StateAct,
			Decision: api.NewCallToolDecision("write_file",
				json.RawMessage(`{"path": "hello.txt", "content": "Hello, Agent World!"}`),
				"Create greeting file"),
		},
		// Act: transition to validate
		api.ScriptStep{
			ExpectState: agent.StateAct,
			Decision:    api.NewTransitionDecision(agent.StateValidate, "Validate file creation"),
		},
		// Validate: read back the file
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewCallToolDecision("read_file",
				json.RawMessage(`{"path": "hello.txt"}`),
				"Verify file contents"),
		},
		// Validate: list directory to confirm
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewCallToolDecision("list_dir",
				json.RawMessage(`{"path": "."}`),
				"Confirm file exists"),
		},
		// Finish
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewFinishDecision("Successfully created and verified file",
				json.RawMessage(`{"file": "hello.txt", "status": "created"}`)),
		},
	)

	// Create the engine
	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(planner),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithBudgets(map[string]int{
			"tool_calls": 10,
		}),
		api.WithMaxSteps(20),
	)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	// Run the agent
	ctx := context.Background()
	run, err := engine.Run(ctx, "Create and verify a greeting file")
	if err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	// Print results
	fmt.Printf("\n--- Run Results ---\n")
	fmt.Printf("Run ID: %s\n", run.ID)
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Final State: %s\n", run.CurrentState)
	fmt.Printf("Duration: %v\n", run.Duration())

	if run.Result != nil {
		var result map[string]interface{}
		if err := json.Unmarshal(run.Result, &result); err == nil {
			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			fmt.Printf("Result: %s\n", resultJSON)
		}
	}

	fmt.Printf("\n--- Evidence Trail ---\n")
	for i, ev := range run.Evidence {
		fmt.Printf("%d. [%s] %s\n", i+1, ev.Type, ev.Source)
		if ev.Content != nil {
			var output map[string]interface{}
			if err := json.Unmarshal(ev.Content, &output); err == nil {
				outputJSON, _ := json.MarshalIndent(output, "", "   ")
				fmt.Printf("   Output: %s\n", outputJSON)
			}
		}
	}

	// Verify the file was actually created
	filePath := filepath.Join(workDir, "hello.txt")
	// #nosec G304 -- example code reading known file in controlled temp directory
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("file verification failed: %w", err)
	}
	fmt.Printf("\n--- File Verification ---\n")
	fmt.Printf("File path: %s\n", filePath)
	fmt.Printf("Content: %s\n", string(content))

	return nil
}
