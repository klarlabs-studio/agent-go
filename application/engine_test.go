package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// Test helpers

func newTestTool(name string, readOnly bool) tool.Tool {
	t, err := tool.NewBuilder(name).
		WithDescription("Test tool: " + name).
		WithAnnotations(tool.Annotations{ReadOnly: readOnly}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"status":"ok"}`)}, nil
		}).
		Build()
	if err != nil {
		panic(err)
	}
	return t
}

func newFailingTool(name string, toolErr error) tool.Tool {
	t, err := tool.NewBuilder(name).
		WithDescription("Failing tool: " + name).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, toolErr
		}).
		Build()
	if err != nil {
		panic(err)
	}
	return t
}

func newTestRegistry(tools ...tool.Tool) tool.Registry {
	registry := memory.NewToolRegistry()
	for _, t := range tools {
		registry.Register(t)
	}
	return registry
}

func newTestEligibility(stateTools map[agent.State][]string) *policy.ToolEligibility {
	eligibility := policy.NewToolEligibility()
	for state, tools := range stateTools {
		for _, t := range tools {
			eligibility.Allow(state, t)
		}
	}
	return eligibility
}

// Engine Creation Tests

func TestNewEngine_RequiresRegistry(t *testing.T) {
	_, err := NewEngine(EngineConfig{
		Planner: planner.NewMockPlanner(),
	})
	if err == nil {
		t.Error("expected error when registry is nil")
	}
}

func TestNewEngine_RequiresPlanner(t *testing.T) {
	_, err := NewEngine(EngineConfig{
		Registry: newTestRegistry(),
	})
	if err == nil {
		t.Error("expected error when planner is nil")
	}
}

func TestNewEngine_SetsDefaults(t *testing.T) {
	engine, err := NewEngine(EngineConfig{
		Registry: newTestRegistry(),
		Planner:  planner.NewMockPlanner(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have defaults set
	if engine.executor == nil {
		t.Error("expected default executor to be set")
	}
	if engine.eligibility == nil {
		t.Error("expected default eligibility to be set")
	}
	if engine.transitions == nil {
		t.Error("expected default transitions to be set")
	}
	if engine.maxSteps == 0 {
		t.Error("expected default maxSteps to be set")
	}
	if engine.middleware == nil {
		t.Error("expected default middleware to be set")
	}
}

// Run Lifecycle Tests

func TestRun_FullLifecycle_Success(t *testing.T) {
	// Setup tools
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)

	// Setup eligibility
	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	// Setup scripted planner for deterministic test
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "begin exploration"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "gather info"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "have enough info"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("completed successfully", json.RawMessage(`{"result":"done"}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, err := engine.Run(ctx, "test full lifecycle")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if run.Status != agent.RunStatusCompleted {
		t.Errorf("expected status Completed, got %s", run.Status)
	}
	if run.CurrentState != agent.StateDone {
		t.Errorf("expected state Done, got %s", run.CurrentState)
	}
	if len(run.Evidence) == 0 {
		t.Error("expected evidence to be collected")
	}
}

func TestRun_FullLifecycle_Failure(t *testing.T) {
	registry := newTestRegistry()

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewFailDecision("intentional failure", errors.New("test error")),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, _ := engine.Run(ctx, "test failure lifecycle")

	// Run should complete (return the run object) but with failed status
	if run == nil {
		t.Fatal("expected run object to be returned")
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected status Failed, got %s", run.Status)
	}
	if run.CurrentState != agent.StateFailed {
		t.Errorf("expected state Failed, got %s", run.CurrentState)
	}
}

func TestRun_WithVariables(t *testing.T) {
	registry := newTestRegistry()

	// Variables should be passed through proper transition path:
	// intake -> explore -> decide -> done
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
			// Use Condition to verify variables are passed
			Condition: func(req planner.PlanRequest) bool {
				return req.Vars["key1"] == "value1" && req.Vars["key2"] == 42
			},
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("done", json.RawMessage(`{}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	vars := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	run, err := engine.RunWithVars(ctx, "test with vars", vars)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if run.Vars["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", run.Vars["key1"])
	}
	if run.Vars["key2"] != 42 {
		t.Errorf("expected key2=42, got %v", run.Vars["key2"])
	}
}

// Tool Eligibility Tests

func TestRun_ToolEligibility_Enforced(t *testing.T) {
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)

	// Tool NOT allowed in intake state
	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	// Try to call tool in intake (not allowed)
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "attempt tool"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, _ := engine.Run(ctx, "test eligibility")

	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail due to eligibility, got status %s", run.Status)
	}
}

// Budget Enforcement Tests

func TestRun_BudgetEnforcement(t *testing.T) {
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	// Budget of only 1 tool call
	budgets := map[string]int{"tool_calls": 1}

	// Try to call tool twice
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first call"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second call - should fail"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:     registry,
		Planner:      scriptedPlanner,
		Eligibility:  eligibility,
		BudgetLimits: budgets,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, runErr := engine.Run(ctx, "test budget")

	if !errors.Is(runErr, policy.ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
}

// Max Steps Tests

func TestRun_MaxStepsEnforced(t *testing.T) {
	registry := newTestRegistry()

	// Planner that never finishes - just keeps transitioning
	infinitePlanner := &infiniteTransitionPlanner{}

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  infinitePlanner,
		MaxSteps: 5, // Limit to 5 steps
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, runErr := engine.Run(ctx, "test max steps")

	if runErr == nil || runErr.Error() != "max steps exceeded" {
		t.Errorf("expected max steps exceeded error, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
}

// infiniteTransitionPlanner keeps transitioning using valid paths:
// intake -> explore -> decide -> act -> validate -> explore (loop)
type infiniteTransitionPlanner struct {
	callCount int
}

func (p *infiniteTransitionPlanner) Plan(ctx context.Context, req planner.PlanRequest) (agent.Decision, error) {
	p.callCount++
	switch req.CurrentState {
	case agent.StateIntake:
		return agent.NewTransitionDecision(agent.StateExplore, "go explore"), nil
	case agent.StateExplore:
		return agent.NewTransitionDecision(agent.StateDecide, "go decide"), nil
	case agent.StateDecide:
		return agent.NewTransitionDecision(agent.StateAct, "go act"), nil
	case agent.StateAct:
		return agent.NewTransitionDecision(agent.StateValidate, "go validate"), nil
	case agent.StateValidate:
		return agent.NewTransitionDecision(agent.StateExplore, "loop back"), nil
	default:
		return agent.NewTransitionDecision(agent.StateExplore, "fallback"), nil
	}
}

// Context Cancellation Tests

func TestRun_ContextCancellation(t *testing.T) {
	registry := newTestRegistry()

	// Slow planner that respects context
	slowPlanner := &slowPlanner{delay: 100 * time.Millisecond}

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  slowPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	run, runErr := engine.Run(ctx, "test cancellation")

	if !errors.Is(runErr, context.DeadlineExceeded) {
		t.Errorf("expected context deadline exceeded, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
}

// slowPlanner introduces delay to test cancellation
type slowPlanner struct {
	delay time.Duration
}

func (p *slowPlanner) Plan(ctx context.Context, req planner.PlanRequest) (agent.Decision, error) {
	select {
	case <-ctx.Done():
		return agent.Decision{}, ctx.Err()
	case <-time.After(p.delay):
		return agent.NewTransitionDecision(agent.StateExplore, "delayed"), nil
	}
}

// Tool Execution Error Tests

func TestRun_ToolExecutionError(t *testing.T) {
	toolErr := errors.New("tool execution failed")
	failingTool := newFailingTool("bad_tool", toolErr)
	registry := newTestRegistry(failingTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"bad_tool"},
	})

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("bad_tool", json.RawMessage(`{}`), "call failing tool"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, runErr := engine.Run(ctx, "test tool error")

	if runErr == nil {
		t.Error("expected error from tool execution")
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
}

// Tool Not Found Tests

func TestRun_ToolNotFound(t *testing.T) {
	registry := newTestRegistry() // Empty registry

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"nonexistent_tool"},
	})

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("nonexistent_tool", json.RawMessage(`{}`), "call missing tool"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, runErr := engine.Run(ctx, "test tool not found")

	if !errors.Is(runErr, tool.ErrToolNotFound) {
		t.Errorf("expected ErrToolNotFound, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
}

// State Transition Tests

func TestRun_InvalidTransition_Rejected(t *testing.T) {
	registry := newTestRegistry()

	// Try invalid transition: intake -> act (not allowed)
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateAct, "invalid jump to act"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, runErr := engine.Run(ctx, "test invalid transition")

	if runErr == nil {
		t.Error("expected error for invalid transition")
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
}

// Human Input Tests

func TestRun_AskHuman_PausesExecution(t *testing.T) {
	registry := newTestRegistry()

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision: agent.Decision{
				Type: agent.DecisionAskHuman,
				AskHuman: &agent.AskHumanDecision{
					Question: "What should I do?",
					Options:  []string{"Option A", "Option B"},
				},
			},
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, runErr := engine.Run(ctx, "test human input")

	// Should return ErrAwaitingHumanInput
	if !errors.Is(runErr, agent.ErrAwaitingHumanInput) {
		t.Errorf("expected ErrAwaitingHumanInput error, got %v", runErr)
	}

	// Run should be paused, not failed
	if run.Status != agent.RunStatusPaused {
		t.Errorf("expected run status paused, got %s", run.Status)
	}

	// Should have pending question
	if !run.HasPendingQuestion() {
		t.Error("expected run to have pending question")
	}

	if run.PendingQuestion.Question != "What should I do?" {
		t.Errorf("expected question 'What should I do?', got %s", run.PendingQuestion.Question)
	}

	if len(run.PendingQuestion.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(run.PendingQuestion.Options))
	}
}

func TestResumeWithInput_AddsEvidenceAndContinues(t *testing.T) {
	registry := newTestRegistry()

	// First step: ask human, then follow valid transition path to done
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision: agent.Decision{
				Type: agent.DecisionAskHuman,
				AskHuman: &agent.AskHumanDecision{
					Question: "Which option?",
					Options:  []string{"A", "B"},
				},
			},
		},
		// After resume, continue with valid path: intake -> explore -> decide -> done
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "exploring after human input"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "ready to decide"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("completed with input", json.RawMessage(`{"result":"done"}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	// First run pauses for human input
	run, err := engine.Run(ctx, "test resume")
	if !errors.Is(err, agent.ErrAwaitingHumanInput) {
		t.Fatalf("expected ErrAwaitingHumanInput, got %v", err)
	}

	// Resume with valid input
	run, err = engine.ResumeWithInput(ctx, run, "A")
	if err != nil {
		t.Fatalf("ResumeWithInput failed: %v", err)
	}

	// Run should complete
	if run.Status != agent.RunStatusCompleted {
		t.Errorf("expected completed status, got %s", run.Status)
	}

	// Should have human input in evidence
	hasHumanEvidence := false
	for _, e := range run.Evidence {
		if e.Type == agent.EvidenceHumanInput {
			hasHumanEvidence = true
			// Verify content includes question and response
			var content map[string]string
			if err := json.Unmarshal(e.Content, &content); err != nil {
				t.Errorf("failed to unmarshal evidence content: %v", err)
			}
			if content["question"] != "Which option?" {
				t.Errorf("expected question in evidence, got %v", content)
			}
			if content["response"] != "A" {
				t.Errorf("expected response 'A' in evidence, got %v", content)
			}
		}
	}
	if !hasHumanEvidence {
		t.Error("expected human input evidence to be added")
	}

	// Pending question should be cleared
	if run.HasPendingQuestion() {
		t.Error("pending question should be cleared after resume")
	}
}

func TestResumeWithInput_ValidatesOptions(t *testing.T) {
	registry := newTestRegistry()

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision: agent.Decision{
				Type: agent.DecisionAskHuman,
				AskHuman: &agent.AskHumanDecision{
					Question: "Choose one",
					Options:  []string{"yes", "no"},
				},
			},
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	run, err := engine.Run(ctx, "test validation")
	if !errors.Is(err, agent.ErrAwaitingHumanInput) {
		t.Fatalf("expected ErrAwaitingHumanInput, got %v", err)
	}

	// Try invalid input
	_, err = engine.ResumeWithInput(ctx, run, "maybe")
	if !errors.Is(err, agent.ErrInvalidHumanInput) {
		t.Errorf("expected ErrInvalidHumanInput for invalid option, got %v", err)
	}
}

func TestResumeWithInput_RequiresPendingQuestion(t *testing.T) {
	registry := newTestRegistry()

	// Valid path: intake -> explore -> decide -> done
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("done", json.RawMessage(`{}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	run, err := engine.Run(ctx, "test no pending question")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Try to resume a run that has no pending question
	_, err = engine.ResumeWithInput(ctx, run, "some input")
	if !errors.Is(err, agent.ErrNoPendingQuestion) {
		t.Errorf("expected ErrNoPendingQuestion, got %v", err)
	}
}

func TestResumeWithInput_AllowsFreeformInput(t *testing.T) {
	registry := newTestRegistry()

	// No options = freeform input allowed
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision: agent.Decision{
				Type: agent.DecisionAskHuman,
				AskHuman: &agent.AskHumanDecision{
					Question: "What is your name?",
					Options:  nil, // No options = freeform
				},
			},
		},
		// After resume, follow valid path: intake -> explore -> decide -> done
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("got name", json.RawMessage(`{}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	run, err := engine.Run(ctx, "test freeform")
	if !errors.Is(err, agent.ErrAwaitingHumanInput) {
		t.Fatalf("expected ErrAwaitingHumanInput, got %v", err)
	}

	// Any input should be accepted when no options specified
	run, err = engine.ResumeWithInput(ctx, run, "Claude")
	if err != nil {
		t.Fatalf("ResumeWithInput failed: %v", err)
	}

	if run.Status != agent.RunStatusCompleted {
		t.Errorf("expected completed status, got %s", run.Status)
	}
}

// Evidence Collection Tests

func TestRun_EvidenceCollection(t *testing.T) {
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	// Valid path: intake -> explore (tool calls) -> decide -> done
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{"path":"file1"}`), "read first"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{"path":"file2"}`), "read second"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "have enough evidence"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("done reading", json.RawMessage(`{}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, err := engine.Run(ctx, "test evidence")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Should have 2 pieces of evidence from the tool calls
	if len(run.Evidence) != 2 {
		t.Errorf("expected 2 evidence entries, got %d", len(run.Evidence))
	}

	// Evidence should be ordered (append-only)
	for i, e := range run.Evidence {
		if e.Type != agent.EvidenceToolResult {
			t.Errorf("evidence[%d]: expected type EvidenceToolResult, got %s", i, e.Type)
		}
		if e.Source != "read_file" {
			t.Errorf("evidence[%d]: expected source read_file, got %s", i, e.Source)
		}
	}
}

// PlanRequest Population Tests

func TestRun_PlanRequest_GoalPopulated(t *testing.T) {
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	var capturedReq planner.PlanRequest
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
			Condition: func(req planner.PlanRequest) bool {
				capturedReq = req
				return true
			},
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("done", json.RawMessage(`{}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	_, err = engine.Run(ctx, "find security vulnerabilities")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if capturedReq.Goal != "find security vulnerabilities" {
		t.Errorf("expected goal 'find security vulnerabilities', got %q", capturedReq.Goal)
	}
}

func TestRun_PlanRequest_ToolDefsPopulated(t *testing.T) {
	readTool, err := tool.NewBuilder("read_file").
		WithDescription("Read a file from disk").
		WithInputSchema(tool.NewSchema(json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"content":"hello"}`)}, nil
		}).
		Build()
	if err != nil {
		t.Fatalf("failed to build tool: %v", err)
	}

	registry := newTestRegistry(readTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	var capturedReq planner.PlanRequest
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "decide"),
			Condition: func(req planner.PlanRequest) bool {
				capturedReq = req
				return true
			},
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewFinishDecision("done", json.RawMessage(`{}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	_, err = engine.Run(ctx, "test tool defs")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// In explore state, read_file should be allowed and have ToolDefs populated
	if len(capturedReq.ToolDefs) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(capturedReq.ToolDefs))
	}
	td := capturedReq.ToolDefs[0]
	if td.Name != "read_file" {
		t.Errorf("expected tool def name 'read_file', got %q", td.Name)
	}
	if td.Description != "Read a file from disk" {
		t.Errorf("expected description 'Read a file from disk', got %q", td.Description)
	}
	if len(td.InputSchema) == 0 {
		t.Error("expected non-empty input schema")
	}
}

// Wildcard Eligibility + Approval Integration Tests

func TestRun_WildcardEligibility_DestructiveToolRequiresApproval(t *testing.T) {
	// Destructive tool that would normally need approval
	destructiveTool, err := tool.NewBuilder("delete_file").
		WithDescription("Delete a file from disk").
		WithAnnotations(tool.DestructiveAnnotations()).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"deleted":true}`)}, nil
		}).
		Build()
	if err != nil {
		t.Fatalf("failed to build tool: %v", err)
	}

	registry := newTestRegistry(destructiveTool)

	// Wildcard eligibility allows all tools in act state
	eligibility := policy.NewDefaultToolEligibility()

	// DenyApprover blocks destructive actions
	approver := policy.NewDenyApprover("not authorized")

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewTransitionDecision(agent.StateAct, "act"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateAct,
			Decision:    agent.NewCallToolDecision("delete_file", json.RawMessage(`{"path":"/tmp/test"}`), "delete it"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
		Approver:    approver,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, runErr := engine.Run(ctx, "test wildcard + destructive + approval")

	// Should fail because approval was denied — wildcard eligibility does NOT bypass approval
	if runErr == nil {
		t.Fatal("expected error when approval is denied for destructive tool")
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
	if !errors.Is(runErr, tool.ErrApprovalDenied) {
		t.Errorf("expected ErrApprovalDenied, got %v", runErr)
	}
}

func TestRun_WildcardEligibility_DestructiveToolApproved(t *testing.T) {
	// Destructive tool with auto-approval
	destructiveTool, err := tool.NewBuilder("delete_file").
		WithDescription("Delete a file from disk").
		WithAnnotations(tool.DestructiveAnnotations()).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"deleted":true}`)}, nil
		}).
		Build()
	if err != nil {
		t.Fatalf("failed to build tool: %v", err)
	}

	registry := newTestRegistry(destructiveTool)

	// Wildcard eligibility + auto approver — tool should execute
	eligibility := policy.NewDefaultToolEligibility()
	approver := policy.NewAutoApprover("test")

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    agent.NewTransitionDecision(agent.StateAct, "act"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateAct,
			Decision:    agent.NewCallToolDecision("delete_file", json.RawMessage(`{"path":"/tmp/test"}`), "delete it"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateAct,
			Decision:    agent.NewTransitionDecision(agent.StateValidate, "validate"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision:    agent.NewFinishDecision("deleted file", json.RawMessage(`{"deleted":true}`)),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     scriptedPlanner,
		Eligibility: eligibility,
		Approver:    approver,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	run, err := engine.Run(ctx, "test wildcard + destructive + approved")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if run.Status != agent.RunStatusCompleted {
		t.Errorf("expected completed, got %s", run.Status)
	}
	if len(run.Evidence) != 1 {
		t.Errorf("expected 1 evidence entry from tool call, got %d", len(run.Evidence))
	}
}

// Run ID Generation Tests

func TestGenerateRunID_Format(t *testing.T) {
	id := generateRunID()
	if id == "" {
		t.Error("expected non-empty run ID")
	}
	if len(id) < 10 {
		t.Error("expected run ID to have reasonable length")
	}
}
