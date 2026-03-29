// Package main demonstrates creating multiple tools with different annotations.
// Shows how tool annotations affect behavior and eligibility.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	agent "github.com/felixgeelhaar/agent-go/interfaces/api"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Create a temporary directory for the example
	tmpDir, err := os.MkdirTemp("", "agent-tools-example")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("Hello from the test file!"), 0600) // #nosec G306

	// ============================================
	// Tool 1: read_file (ReadOnly, Idempotent, Cacheable)
	// ============================================
	readFile := agent.NewToolBuilder("read_file").
		WithDescription("Reads contents of a file").
		WithAnnotations(agent.Annotations{
			ReadOnly:   true, // Does not modify state
			Idempotent: true, // Same input = same output
			Cacheable:  true, // Results can be cached
			RiskLevel:  agent.RiskLow,
		}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			content, err := os.ReadFile(in.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"content": string(content),
				"size":    len(content),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// ============================================
	// Tool 2: write_file (Destructive, NOT ReadOnly)
	// ============================================
	writeFile := agent.NewToolBuilder("write_file").
		WithDescription("Writes content to a file").
		WithAnnotations(agent.Annotations{
			ReadOnly:    false, // Modifies state
			Destructive: false, // Overwrites are recoverable
			Idempotent:  true,  // Same input = same result
			RiskLevel:   agent.RiskMedium,
		}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if err := os.WriteFile(in.Path, []byte(in.Content), 0600); err != nil { // #nosec G306
				return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"written": true,
				"bytes":   len(in.Content),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// ============================================
	// Tool 3: delete_file (Destructive, High Risk)
	// ============================================
	deleteFile := agent.NewToolBuilder("delete_file").
		WithDescription("Deletes a file permanently").
		WithAnnotations(agent.Annotations{
			ReadOnly:    false, // Modifies state
			Destructive: true,  // Irreversible action
			Idempotent:  false, // Can't delete twice
			RiskLevel:   agent.RiskHigh,
		}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if err := os.Remove(in.Path); err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete file: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"deleted": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	// ============================================
	// Set up tool eligibility (which tools in which states)
	// ============================================
	eligibility := agent.NewToolEligibility()
	// Read-only tools can run in explore
	eligibility.Allow(agent.StateExplore, "read_file")
	// Destructive tools can only run in act
	eligibility.AllowMultiple(agent.StateAct, "read_file", "write_file", "delete_file")
	// Allow read_file in validate to verify results
	eligibility.Allow(agent.StateValidate, "read_file")

	// Create scripted planner
	// State flow: intake -> explore -> decide -> act -> validate -> done
	outputFile := filepath.Join(tmpDir, "output.txt")
	planner := agent.NewScriptedPlanner(
		// Start: intake -> explore
		agent.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "beginning exploration"),
		},
		// Explore: read the test file
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: agent.NewCallToolDecision("read_file",
				json.RawMessage(fmt.Sprintf(`{"path":"%s"}`, testFile)),
				"reading test file"),
		},
		// Explore -> Decide: ready to act
		agent.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "gathered info, ready to decide"),
		},
		// Decide -> Act: proceed with write
		agent.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewTransitionDecision(agent.StateAct, "proceeding to write output"),
		},
		// Act: write the output file
		agent.ScriptStep{
			ExpectState: agent.StateAct,
			Decision: agent.NewCallToolDecision("write_file",
				json.RawMessage(fmt.Sprintf(`{"path":"%s","content":"Processed output!"}`, outputFile)),
				"writing output file"),
		},
		// Act -> Validate: verify the result
		agent.ScriptStep{
			ExpectState: agent.StateAct,
			Decision:    agent.NewTransitionDecision(agent.StateValidate, "verifying write"),
		},
		// Validate: finish successfully
		agent.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision:    agent.NewFinishDecision("completed", json.RawMessage(`{"status":"success"}`)),
		},
	)

	// Build engine
	engine, err := agent.New(
		agent.WithTool(readFile),
		agent.WithTool(writeFile),
		agent.WithTool(deleteFile),
		agent.WithPlanner(planner),
		agent.WithToolEligibility(eligibility),
		agent.WithMaxSteps(10),
	)
	if err != nil {
		return err
	}

	// Run the agent
	agentRun, err := engine.Run(context.Background(), "Read test file and write output")
	if err != nil {
		return err
	}

	// Display results
	fmt.Println("=== Tools Example ===")
	fmt.Printf("Status: %s\n", agentRun.Status)
	fmt.Printf("Steps: %d\n", len(agentRun.Evidence))
	fmt.Println()

	// Verify output was written
	// #nosec G304 -- example code reading known output file in controlled temp directory
	if content, err := os.ReadFile(outputFile); err == nil {
		fmt.Printf("Output file contents: %s\n", string(content))
	}

	// Show tool annotations summary
	fmt.Println("\n=== Tool Annotations Summary ===")
	fmt.Println("read_file:   ReadOnly=true,  Destructive=false, Risk=Low")
	fmt.Println("write_file:  ReadOnly=false, Destructive=false, Risk=Medium")
	fmt.Println("delete_file: ReadOnly=false, Destructive=true,  Risk=High")

	return nil
}
