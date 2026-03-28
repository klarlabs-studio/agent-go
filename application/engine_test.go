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

// Concurrency Tests

func TestRun_ConcurrentRuns_SameEngine(t *testing.T) {
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	// Use a MockPlanner which is safe for concurrent use (mutex-protected)
	// Each concurrent run gets its own planner since MockPlanner is stateful
	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Eligibility: eligibility,
		// We set planner per-run below; use a simple planner that finishes immediately
		Planner: &concurrentSafePlanner{},
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	const numRuns = 10
	errs := make(chan error, numRuns)
	runs := make(chan *agent.Run, numRuns)

	for i := 0; i < numRuns; i++ {
		go func() {
			ctx := context.Background()
			run, runErr := engine.Run(ctx, "concurrent test")
			if runErr != nil {
				errs <- runErr
				return
			}
			runs <- run
			errs <- nil
		}()
	}

	completedCount := 0
	for i := 0; i < numRuns; i++ {
		if e := <-errs; e != nil {
			t.Errorf("concurrent run failed: %v", e)
		} else {
			completedCount++
		}
	}

	if completedCount != numRuns {
		t.Errorf("expected %d completed runs, got %d", numRuns, completedCount)
	}

	// Drain and verify runs
	close(runs)
	runIDs := make(map[string]bool)
	for run := range runs {
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed status, got %s", run.Status)
		}
		if runIDs[run.ID] {
			t.Errorf("duplicate run ID: %s", run.ID)
		}
		runIDs[run.ID] = true
	}
}

// concurrentSafePlanner always follows a minimal path: intake -> explore -> decide -> done
type concurrentSafePlanner struct{}

func (p *concurrentSafePlanner) Plan(_ context.Context, req planner.PlanRequest) (agent.Decision, error) {
	switch req.CurrentState {
	case agent.StateIntake:
		return agent.NewTransitionDecision(agent.StateExplore, "explore"), nil
	case agent.StateExplore:
		return agent.NewTransitionDecision(agent.StateDecide, "decide"), nil
	case agent.StateDecide:
		return agent.NewFinishDecision("done", json.RawMessage(`{}`)), nil
	default:
		return agent.NewFinishDecision("done", json.RawMessage(`{}`)), nil
	}
}

// Context Cancellation Tests (Additional)

func TestRun_ContextCancelledBeforeFirstStep(t *testing.T) {
	registry := newTestRegistry()

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately before running

	run, runErr := engine.Run(ctx, "test pre-cancelled context")
	if !errors.Is(runErr, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
}

func TestRun_ContextCancelledDuringToolExecution(t *testing.T) {
	slowTool, err := tool.NewBuilder("slow_tool").
		WithDescription("Slow tool").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			select {
			case <-ctx.Done():
				return tool.Result{}, ctx.Err()
			case <-time.After(5 * time.Second):
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			}
		}).
		Build()
	if err != nil {
		t.Fatalf("failed to build tool: %v", err)
	}

	registry := newTestRegistry(slowTool)
	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"slow_tool"},
	})

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("slow_tool", json.RawMessage(`{}`), "call slow tool"),
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

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	run, runErr := engine.Run(ctx, "test cancel during tool")
	if runErr == nil {
		t.Error("expected error from context cancellation")
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected run to fail, got status %s", run.Status)
	}
}

// Max Steps Tests (Additional)

func TestRun_MaxSteps_ExactBoundary(t *testing.T) {
	registry := newTestRegistry()

	// Planner that finishes on exactly step 3
	// Step 1: intake -> explore
	// Step 2: explore -> decide
	// Step 3: decide -> finish (done)
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
		MaxSteps: 3, // Exactly enough steps
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, runErr := engine.Run(context.Background(), "test exact boundary")
	if runErr != nil {
		t.Fatalf("expected success, got error: %v", runErr)
	}
	if run.Status != agent.RunStatusCompleted {
		t.Errorf("expected completed, got %s", run.Status)
	}
}

func TestRun_MaxSteps_OneStepShort(t *testing.T) {
	registry := newTestRegistry()

	// Needs 3 steps, but only allowed 2
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
		MaxSteps: 2, // One step short
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, runErr := engine.Run(context.Background(), "test one step short")
	if runErr == nil || runErr.Error() != "max steps exceeded" {
		t.Errorf("expected max steps exceeded, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected failed, got %s", run.Status)
	}
}

// Budget Exhaustion Tests (Additional)

func TestRun_BudgetExhaustion_MidRun(t *testing.T) {
	readTool := newTestTool("read_file", true)
	writeTool := newTestTool("write_file", false)
	registry := newTestRegistry(readTool, writeTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
		agent.StateAct:     {"write_file"},
	})

	// Budget allows 2 tool calls - first two succeed, third fails
	budgets := map[string]int{"tool_calls": 2}

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "third - should fail"),
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

	run, runErr := engine.Run(context.Background(), "test budget mid-run")
	if !errors.Is(runErr, policy.ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected failed, got %s", run.Status)
	}
	// Should have 2 evidence entries from the successful calls
	if len(run.Evidence) != 2 {
		t.Errorf("expected 2 evidence entries from successful calls, got %d", len(run.Evidence))
	}
}

// Empty Goal Test

func TestRun_EmptyGoal(t *testing.T) {
	registry := newTestRegistry()

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

	// Empty goal should still work - the engine does not validate goal content
	run, runErr := engine.Run(context.Background(), "")
	if runErr != nil {
		t.Fatalf("empty goal should not cause error, got: %v", runErr)
	}
	if run.Goal != "" {
		t.Errorf("expected empty goal, got %q", run.Goal)
	}
	if run.Status != agent.RunStatusCompleted {
		t.Errorf("expected completed, got %s", run.Status)
	}
}

// Nil Planner Test

func TestNewEngine_NilPlanner_ReturnsError(t *testing.T) {
	_, err := NewEngine(EngineConfig{
		Registry: newTestRegistry(),
		Planner:  nil,
	})
	if err == nil {
		t.Error("expected error when planner is nil")
	}
	if err.Error() != "planner is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// Planner Error Tests

func TestRun_PlannerError_FailsRun(t *testing.T) {
	registry := newTestRegistry()

	plannerErr := errors.New("LLM rate limit exceeded")
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Error:       plannerErr,
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, runErr := engine.Run(context.Background(), "test planner error")
	if runErr == nil {
		t.Error("expected error from planner")
	}
	if !errors.Is(runErr, plannerErr) {
		t.Errorf("expected planner error to be wrapped, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected failed, got %s", run.Status)
	}
}

func TestRun_PlannerError_MidRun(t *testing.T) {
	registry := newTestRegistry()

	plannerErr := errors.New("network timeout")
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Error:       plannerErr, // Fail on second call
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, runErr := engine.Run(context.Background(), "test planner error mid-run")
	if !errors.Is(runErr, plannerErr) {
		t.Errorf("expected wrapped planner error, got %v", runErr)
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected failed, got %s", run.Status)
	}
}

// Tool Execution Failure Tests (Additional)

func TestRun_ToolExecutionError_PreservesEvidence(t *testing.T) {
	readTool := newTestTool("read_file", true)
	toolErr := errors.New("disk full")
	failTool := newFailingTool("write_file", toolErr)
	registry := newTestRegistry(readTool, failTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
		agent.StateAct:     {"write_file"},
	})

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "read"),
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
			Decision:    agent.NewCallToolDecision("write_file", json.RawMessage(`{}`), "write - fails"),
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

	run, _ := engine.Run(context.Background(), "test evidence preservation on error")
	// Evidence from the successful read_file call should be preserved
	if len(run.Evidence) != 1 {
		t.Errorf("expected 1 evidence entry from successful call, got %d", len(run.Evidence))
	}
	if run.Evidence[0].Source != "read_file" {
		t.Errorf("expected evidence from read_file, got %s", run.Evidence[0].Source)
	}
}

// State Machine Violation Tests (Additional)

func TestRun_InvalidTransition_IntakeToValidate(t *testing.T) {
	registry := newTestRegistry()

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateValidate, "jump to validate"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, runErr := engine.Run(context.Background(), "test invalid intake->validate")
	if runErr == nil {
		t.Error("expected error for invalid transition")
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected failed, got %s", run.Status)
	}
}

func TestRun_InvalidTransition_ExploreToAct(t *testing.T) {
	registry := newTestRegistry()

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateAct, "skip decide - invalid"),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, runErr := engine.Run(context.Background(), "test invalid explore->act")
	if runErr == nil {
		t.Error("expected error for invalid transition")
	}
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected failed, got %s", run.Status)
	}
}

// ResumeWithInput Tests (Additional)

func TestResumeWithInput_NilRun(t *testing.T) {
	registry := newTestRegistry()
	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  planner.NewMockPlanner(),
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_, err = engine.ResumeWithInput(context.Background(), nil, "input")
	if err == nil {
		t.Error("expected error for nil run")
	}
}

func TestResumeWithInput_MultipleResumes(t *testing.T) {
	registry := newTestRegistry()

	// Planner asks two questions in sequence
	scriptedPlanner := planner.NewScriptedPlanner(
		// First run: ask first question
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewAskHumanDecision("First question?", "A", "B"),
		},
		// After first resume: transition then ask second question
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewAskHumanDecision("Second question?"),
		},
		// After second resume: complete
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

	// First run pauses
	run, err := engine.Run(ctx, "test multiple resumes")
	if !errors.Is(err, agent.ErrAwaitingHumanInput) {
		t.Fatalf("expected ErrAwaitingHumanInput, got %v", err)
	}
	if run.PendingQuestion.Question != "First question?" {
		t.Errorf("expected first question, got %q", run.PendingQuestion.Question)
	}

	// First resume - pauses again
	run, err = engine.ResumeWithInput(ctx, run, "A")
	if !errors.Is(err, agent.ErrAwaitingHumanInput) {
		t.Fatalf("expected second ErrAwaitingHumanInput, got %v", err)
	}
	if run.PendingQuestion.Question != "Second question?" {
		t.Errorf("expected second question, got %q", run.PendingQuestion.Question)
	}

	// Second resume - completes
	run, err = engine.ResumeWithInput(ctx, run, "freeform answer")
	if err != nil {
		t.Fatalf("second resume failed: %v", err)
	}
	if run.Status != agent.RunStatusCompleted {
		t.Errorf("expected completed, got %s", run.Status)
	}

	// Should have 2 human input evidence entries
	humanCount := 0
	for _, e := range run.Evidence {
		if e.Type == agent.EvidenceHumanInput {
			humanCount++
		}
	}
	if humanCount != 2 {
		t.Errorf("expected 2 human input evidence entries, got %d", humanCount)
	}
}

// Evidence Accumulation Tests (Additional)

func TestRun_EvidenceAccumulation_OrderPreserved(t *testing.T) {
	tool1 := newTestTool("tool_a", true)
	tool2 := newTestTool("tool_b", true)
	registry := newTestRegistry(tool1, tool2)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"tool_a", "tool_b"},
	})

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("tool_a", json.RawMessage(`{}`), "first"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("tool_b", json.RawMessage(`{}`), "second"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("tool_a", json.RawMessage(`{}`), "third"),
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

	run, err := engine.Run(context.Background(), "test evidence order")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if len(run.Evidence) != 3 {
		t.Fatalf("expected 3 evidence entries, got %d", len(run.Evidence))
	}

	// Verify order: tool_a, tool_b, tool_a
	expectedSources := []string{"tool_a", "tool_b", "tool_a"}
	for i, src := range expectedSources {
		if run.Evidence[i].Source != src {
			t.Errorf("evidence[%d]: expected source %q, got %q", i, src, run.Evidence[i].Source)
		}
	}

	// Verify timestamps are non-decreasing
	for i := 1; i < len(run.Evidence); i++ {
		if run.Evidence[i].Timestamp.Before(run.Evidence[i-1].Timestamp) {
			t.Errorf("evidence[%d] timestamp is before evidence[%d]", i, i-1)
		}
	}
}

func TestRun_EvidencePassedToPlanner(t *testing.T) {
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	var capturedEvidence []agent.Evidence
	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewCallToolDecision("read_file", json.RawMessage(`{}`), "call tool"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewTransitionDecision(agent.StateDecide, "decide"),
			Condition: func(req planner.PlanRequest) bool {
				capturedEvidence = req.Evidence
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

	_, err = engine.Run(context.Background(), "test evidence in plan request")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// After the tool call, the planner should receive 1 evidence entry
	if len(capturedEvidence) != 1 {
		t.Fatalf("expected planner to see 1 evidence entry, got %d", len(capturedEvidence))
	}
	if capturedEvidence[0].Source != "read_file" {
		t.Errorf("expected evidence from read_file, got %s", capturedEvidence[0].Source)
	}
}

// Sequential Runs on Same Engine

func TestRun_SequentialRuns_SameEngine(t *testing.T) {
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)

	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	// Use a concurrent safe planner for multiple sequential runs
	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     &concurrentSafePlanner{},
		Eligibility: eligibility,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	var runs []*agent.Run

	for i := 0; i < 5; i++ {
		run, runErr := engine.Run(ctx, "sequential run")
		if runErr != nil {
			t.Fatalf("run %d failed: %v", i, runErr)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("run %d: expected completed, got %s", i, run.Status)
		}
		runs = append(runs, run)
	}

	// Each run should have a unique ID
	ids := make(map[string]bool)
	for _, run := range runs {
		if ids[run.ID] {
			t.Errorf("duplicate run ID across sequential runs: %s", run.ID)
		}
		ids[run.ID] = true
	}
}

// Fail Decision Tests

func TestRun_FailDecision_FromExploreState(t *testing.T) {
	registry := newTestRegistry()

	scriptedPlanner := planner.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    agent.NewTransitionDecision(agent.StateExplore, "explore"),
		},
		planner.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    agent.NewFailDecision("discovered unsolvable problem", errors.New("cannot proceed")),
		},
	)

	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  scriptedPlanner,
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	run, _ := engine.Run(context.Background(), "test fail from explore")
	if run.Status != agent.RunStatusFailed {
		t.Errorf("expected failed, got %s", run.Status)
	}
	if run.CurrentState != agent.StateFailed {
		t.Errorf("expected state failed, got %s", run.CurrentState)
	}
	if run.Error != "discovered unsolvable problem" {
		t.Errorf("expected error message, got %q", run.Error)
	}
}

// NewEngineWithOptions Tests

func TestNewEngineWithOptions_AllOptions(t *testing.T) {
	readTool := newTestTool("read_file", true)
	registry := newTestRegistry(readTool)
	eligibility := newTestEligibility(map[agent.State][]string{
		agent.StateExplore: {"read_file"},
	})

	engine, err := NewEngineWithOptions(
		WithRegistry(registry),
		WithPlanner(&concurrentSafePlanner{}),
		WithEligibility(eligibility),
		WithBudgets(map[string]int{"tool_calls": 50}),
		WithMaxSteps(25),
	)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	if engine.maxSteps != 25 {
		t.Errorf("expected maxSteps 25, got %d", engine.maxSteps)
	}
	if engine.budgetLimits["tool_calls"] != 50 {
		t.Errorf("expected budget limit 50, got %d", engine.budgetLimits["tool_calls"])
	}
}
