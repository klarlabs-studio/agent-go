//go:build integration

// Package integration provides end-to-end tests for the agent runtime.
package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	api "github.com/felixgeelhaar/agent-go/interfaces/api"

	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// makeEchoTool creates a simple tool that echoes input as output.
func makeEchoTool(name string, readOnly bool) tool.Tool {
	return api.NewToolBuilder(name).
		WithDescription("Echo tool for testing").
		WithAnnotations(api.Annotations{ReadOnly: readOnly}).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()
}

// makeFileTool creates a tool that writes content to a temp directory.
func makeFileTool(dir string) tool.Tool {
	return api.NewToolBuilder("write_file").
		WithDescription("Writes a file").
		WithAnnotations(api.Annotations{Destructive: true}).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var req struct {
				Name    string `json:"name"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}
			path := filepath.Join(dir, req.Name)
			if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
				return tool.Result{}, err
			}
			out, _ := json.Marshal(map[string]string{"path": path, "status": "written"})
			return tool.Result{Output: out}, nil
		}).
		MustBuild()
}

// makeReadTool creates a tool that reads a file from a temp directory.
func makeReadTool(dir string) tool.Tool {
	return api.NewToolBuilder("read_file").
		WithDescription("Reads a file").
		WithAnnotations(api.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var req struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return tool.Result{}, err
			}
			data, err := os.ReadFile(filepath.Join(dir, req.Name))
			if err != nil {
				return tool.Result{}, err
			}
			out, _ := json.Marshal(map[string]string{"content": string(data)})
			return tool.Result{Output: out}, nil
		}).
		MustBuild()
}

// TestFullFlow_IntakeToCompletion tests a complete agent run through all states.
func TestFullFlow_IntakeToCompletion(t *testing.T) {
	dir := t.TempDir()
	readTool := makeReadTool(dir)
	writeTool := makeFileTool(dir)
	echoTool := makeEchoTool("analyze", true)

	// Write a seed file for the agent to read
	os.WriteFile(filepath.Join(dir, "input.txt"), []byte("hello world"), 0644)

	writeInput, _ := json.Marshal(map[string]string{"name": "output.txt", "content": "processed"})
	readInput, _ := json.Marshal(map[string]string{"name": "input.txt"})
	analyzeInput, _ := json.Marshal(map[string]string{"data": "hello"})
	validateInput, _ := json.Marshal(map[string]string{"name": "output.txt"})
	result, _ := json.Marshal(map[string]string{"status": "success"})

	planner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "start exploration")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("read_file", readInput, "read input")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "analyze findings")},
		api.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewCallToolDecision("analyze", analyzeInput, "think about it")},
		api.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewTransitionDecision(agent.StateAct, "take action")},
		api.ScriptStep{ExpectState: agent.StateAct, Decision: agent.NewCallToolDecision("write_file", writeInput, "write output")},
		api.ScriptStep{ExpectState: agent.StateAct, Decision: agent.NewTransitionDecision(agent.StateValidate, "verify result")},
		api.ScriptStep{ExpectState: agent.StateValidate, Decision: agent.NewCallToolDecision("read_file", validateInput, "validate output")},
		api.ScriptStep{ExpectState: agent.StateValidate, Decision: agent.NewFinishDecision("all done", result)},
	)

	eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
		agent.StateExplore:  {"read_file", "analyze"},
		agent.StateDecide:   {"analyze"},
		agent.StateAct:      {"write_file"},
		agent.StateValidate: {"read_file"},
	})

	engine, err := api.New(
		api.WithTool(readTool),
		api.WithTool(writeTool),
		api.WithTool(echoTool),
		api.WithPlanner(planner),
		api.WithToolEligibility(eligibility),
		api.WithMaxSteps(50),
		api.WithApprover(api.NewAutoApprover("test")),
	)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, err := engine.Run(context.Background(), "Process input.txt and create output.txt")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if run.Status != agent.RunStatusCompleted {
		t.Fatalf("expected completed, got %s (error: %s)", run.Status, run.Error)
	}

	// Verify evidence was accumulated
	if len(run.Evidence) < 4 {
		t.Errorf("expected at least 4 evidence items (4 tool calls), got %d", len(run.Evidence))
	}

	// Verify the file was actually written
	content, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if string(content) != "processed" {
		t.Errorf("expected 'processed', got %q", content)
	}
}

// TestFullFlow_WithRunPersistence verifies runs are saved to the run store.
func TestFullFlow_WithRunPersistence(t *testing.T) {
	runStore := memory.NewRunStore()
	result, _ := json.Marshal(map[string]string{"ok": "true"})

	planner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "go")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "decide")},
		api.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewFinishDecision("done", result)},
	)

	engine, err := api.New(
		api.WithPlanner(planner),
		api.WithRunStore(runStore),
		api.WithMaxSteps(20),
		api.WithToolEligibility(api.NewDefaultToolEligibility()),
	)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, err := engine.Run(context.Background(), "test persistence")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify run was persisted
	persisted, err := runStore.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("failed to get persisted run: %v", err)
	}
	if persisted.Status != agent.RunStatusCompleted {
		t.Errorf("persisted run status: got %s, want completed", persisted.Status)
	}
	if persisted.Goal != "test persistence" {
		t.Errorf("persisted run goal: got %q, want %q", persisted.Goal, "test persistence")
	}
}

// TestFullFlow_WithEventStreaming verifies events are published during execution.
func TestFullFlow_WithEventStreaming(t *testing.T) {
	eventStore := memory.NewEventStore()
	echoTool := makeEchoTool("echo", true)
	input, _ := json.Marshal(map[string]string{"msg": "hello"})
	result, _ := json.Marshal(map[string]string{"ok": "true"})

	planner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "start")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("echo", input, "echo test")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "decide")},
		api.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewFinishDecision("done", result)},
	)

	engine, err := api.New(
		api.WithTool(echoTool),
		api.WithPlanner(planner),
		api.WithEventStore(eventStore),
		api.WithMaxSteps(20),
		api.WithToolEligibility(api.NewDefaultToolEligibility()),
	)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, err := engine.Run(context.Background(), "test events")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Load events from store
	events, err := eventStore.LoadEvents(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("failed to load events: %v", err)
	}

	// Verify event types
	typeSet := make(map[event.Type]bool)
	for _, e := range events {
		typeSet[e.Type] = true
	}

	expectedTypes := []event.Type{
		event.TypeRunStarted,
		event.TypeDecisionMade,
		event.TypeToolCalled,
		event.TypeToolSucceeded,
		event.TypeStateTransitioned,
		event.TypeRunCompleted,
	}

	for _, et := range expectedTypes {
		if !typeSet[et] {
			t.Errorf("missing event type: %s", et)
		}
	}

	// Verify events are in chronological order
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.Before(events[i-1].Timestamp) {
			t.Errorf("events not in order: %s at %v before %s at %v",
				events[i-1].Type, events[i-1].Timestamp,
				events[i].Type, events[i].Timestamp)
		}
	}
}

// TestFullFlow_BudgetExhaustion verifies budget enforcement mid-run.
func TestFullFlow_BudgetExhaustion(t *testing.T) {
	echoTool := makeEchoTool("echo", true)
	input, _ := json.Marshal(map[string]string{"msg": "hello"})

	planner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "start")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("echo", input, "call 1")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("echo", input, "call 2")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("echo", input, "call 3 - should fail")},
	)

	engine, err := api.New(
		api.WithTool(echoTool),
		api.WithPlanner(planner),
		api.WithBudget("tool_calls", 2),
		api.WithMaxSteps(20),
		api.WithToolEligibility(api.NewDefaultToolEligibility()),
	)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_, err = engine.Run(context.Background(), "test budget")
	if err == nil {
		t.Fatal("expected error for budget exhaustion")
	}
}

// TestFullFlow_ContextCancellation verifies clean shutdown on context cancel.
func TestFullFlow_ContextCancellation(t *testing.T) {
	slowTool := api.NewToolBuilder("slow").
		WithDescription("Slow tool").
		WithAnnotations(api.Annotations{ReadOnly: true}).
		WithHandler(func(ctx context.Context, _ json.RawMessage) (tool.Result, error) {
			select {
			case <-ctx.Done():
				return tool.Result{}, ctx.Err()
			case <-time.After(5 * time.Second):
				return tool.Result{Output: json.RawMessage(`{"done":true}`)}, nil
			}
		}).
		MustBuild()

	input, _ := json.Marshal(map[string]string{})

	planner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "start")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("slow", input, "slow call")},
	)

	engine, err := api.New(
		api.WithTool(slowTool),
		api.WithPlanner(planner),
		api.WithMaxSteps(20),
		api.WithToolEligibility(api.NewDefaultToolEligibility()),
	)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = engine.Run(ctx, "test cancellation")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for context cancellation")
	}
	if elapsed > 2*time.Second {
		t.Errorf("cancellation took too long: %v", elapsed)
	}
}

// TestFullFlow_StreamExecution verifies the Stream() API returns events.
func TestFullFlow_StreamExecution(t *testing.T) {
	eventStore := memory.NewEventStore()
	echoTool := makeEchoTool("echo", true)
	input, _ := json.Marshal(map[string]string{"msg": "stream"})
	result, _ := json.Marshal(map[string]string{"ok": "true"})

	planner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "start")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewCallToolDecision("echo", input, "echo")},
		api.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "decide")},
		api.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewFinishDecision("done", result)},
	)

	engine, err := api.New(
		api.WithTool(echoTool),
		api.WithPlanner(planner),
		api.WithEventStore(eventStore),
		api.WithMaxSteps(20),
		api.WithToolEligibility(api.NewDefaultToolEligibility()),
	)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runID, ch, err := engine.Stream(ctx, "test streaming")
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}
	if runID == "" {
		t.Fatal("expected non-empty run ID")
	}

	var received []event.Event
	for evt := range ch {
		received = append(received, evt)
		if evt.Type == event.TypeRunCompleted || evt.Type == event.TypeRunFailed {
			break
		}
	}

	if len(received) == 0 {
		t.Fatal("expected to receive events via stream")
	}

	// First event should be run.started
	if received[0].Type != event.TypeRunStarted {
		t.Errorf("first event type: got %s, want run.started", received[0].Type)
	}
}
