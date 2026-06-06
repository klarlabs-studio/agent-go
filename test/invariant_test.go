// Package test contains the invariant test suite for the agent runtime.
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"go.klarlabs.de/agent/application"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/artifact"
	"go.klarlabs.de/agent/domain/cache"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/ledger"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/storage/filesystem"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
	api "go.klarlabs.de/agent/interfaces/api"
)

// =============================================================================
// Invariant 1: Tool Eligibility
// A tool can only execute in states where it is explicitly allowed.
// =============================================================================

func TestInvariant_ToolEligibility(t *testing.T) {
	t.Run("tool_executes_only_in_allowed_state", func(t *testing.T) {
		// Create a tool
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"content": "hello"}`)}, nil
			}).
			MustBuild()

		// Create registry with the tool
		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		// Create eligibility that only allows the tool in "explore" state
		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		// Create scripted planner that tries to call tool from intake state
		// This should fail because tool is not allowed in intake
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "attempt from intake"),
			},
		)

		// Create engine
		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		// Run should fail because tool is not allowed in intake state
		ctx := context.Background()
		_, err = engine.Run(ctx, "test tool eligibility")

		// Expect error about tool not allowed
		if err == nil {
			t.Fatal("expected error for tool not allowed in state, got nil")
		}
		if !errors.Is(err, tool.ErrToolNotAllowed) {
			t.Errorf("expected ErrToolNotAllowed, got: %v", err)
		}
	})

	t.Run("tool_allowed_in_correct_state", func(t *testing.T) {
		// Create a tool
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"content": "hello"}`)}, nil
			}).
			MustBuild()

		// Create registry
		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		// Create eligibility that allows tool in explore state
		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		// Create scripted planner that transitions to explore then calls tool
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "go to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "read in explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "done exploring"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewFinishDecision("completed", json.RawMessage(`{"status": "ok"}`)),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test tool in correct state")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed status, got: %s", run.Status)
		}
	})
}

// =============================================================================
// Invariant 2: Transition Validity
// State transitions must follow the allowed transition graph.
// =============================================================================

func TestInvariant_TransitionValidity(t *testing.T) {
	t.Run("invalid_transition_rejected", func(t *testing.T) {
		registry := api.NewToolRegistry()

		// Create transitions that don't allow intake -> act
		transitions := api.NewStateTransitions()
		transitions.Allow(agent.StateIntake, agent.StateExplore)

		// Create planner that tries invalid transition
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateAct, "skip to act"),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithTransitions(transitions),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		_, err = engine.Run(ctx, "test invalid transition")

		if err == nil {
			t.Fatal("expected error for invalid transition, got nil")
		}
	})

	t.Run("valid_transition_allowed", func(t *testing.T) {
		registry := api.NewToolRegistry()

		// Create scripted planner with valid transitions
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewFinishDecision("done", json.RawMessage(`{}`)),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test valid transition")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed status, got: %s", run.Status)
		}
	})
}

// =============================================================================
// Invariant 3: Approval Enforcement
// Destructive tools require approval before execution.
// =============================================================================

func TestInvariant_ApprovalEnforcement(t *testing.T) {
	t.Run("destructive_tool_requires_approval", func(t *testing.T) {
		// Create destructive tool
		deleteTool := api.NewToolBuilder("delete_file").
			WithDescription("Deletes a file").
			WithAnnotations(api.Annotations{
				Destructive: true,
				RiskLevel:   api.RiskHigh,
			}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"deleted": true}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(deleteTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateAct, "delete_file")

		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewTransitionDecision(agent.StateAct, "to act"),
			},
			api.ScriptStep{
				ExpectState: agent.StateAct,
				Decision:    api.NewCallToolDecision("delete_file", json.RawMessage(`{"path": "/tmp/test"}`), "delete file"),
			},
		)

		// Create engine WITHOUT an approver - should fail
		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		_, err = engine.Run(ctx, "test approval required")

		if err == nil {
			t.Fatal("expected error for approval required, got nil")
		}
		if !errors.Is(err, tool.ErrApprovalRequired) {
			t.Errorf("expected ErrApprovalRequired, got: %v", err)
		}
	})

	t.Run("destructive_tool_executes_with_approval", func(t *testing.T) {
		deleteTool := api.NewToolBuilder("delete_file").
			WithDescription("Deletes a file").
			WithAnnotations(api.Annotations{
				Destructive: true,
				RiskLevel:   api.RiskHigh,
			}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"deleted": true}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(deleteTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateAct, "delete_file")

		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewTransitionDecision(agent.StateAct, "to act"),
			},
			api.ScriptStep{
				ExpectState: agent.StateAct,
				Decision:    api.NewCallToolDecision("delete_file", json.RawMessage(`{}`), "delete"),
			},
			api.ScriptStep{
				ExpectState: agent.StateAct,
				Decision:    api.NewTransitionDecision(agent.StateValidate, "to validate"),
			},
			api.ScriptStep{
				ExpectState: agent.StateValidate,
				Decision:    api.NewFinishDecision("completed", json.RawMessage(`{}`)),
			},
		)

		// Create engine WITH an auto-approver
		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithApprover(api.NewAutoApprover("test-approver")),
			api.WithMaxSteps(20),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test with approval")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed, got: %s", run.Status)
		}
	})

	t.Run("approval_denied_blocks_execution", func(t *testing.T) {
		deleteTool := api.NewToolBuilder("delete_file").
			WithDescription("Deletes a file").
			WithAnnotations(api.Annotations{
				Destructive: true,
				RiskLevel:   api.RiskHigh,
			}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"deleted": true}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(deleteTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateAct, "delete_file")

		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewTransitionDecision(agent.StateAct, "to act"),
			},
			api.ScriptStep{
				ExpectState: agent.StateAct,
				Decision:    api.NewCallToolDecision("delete_file", json.RawMessage(`{}`), "delete"),
			},
		)

		// Create engine with deny approver
		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithApprover(api.NewDenyApprover("not allowed")),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		_, err = engine.Run(ctx, "test denied approval")

		if err == nil {
			t.Fatal("expected error for denied approval, got nil")
		}
		if !errors.Is(err, tool.ErrApprovalDenied) {
			t.Errorf("expected ErrApprovalDenied, got: %v", err)
		}
	})
}

// =============================================================================
// Invariant 4: Budget Enforcement
// Operations must respect budget limits.
// =============================================================================

func TestInvariant_BudgetEnforcement(t *testing.T) {
	t.Run("budget_exceeded_blocks_execution", func(t *testing.T) {
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		// Planner tries to call tool twice, but budget allows only 1
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first call"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second call"),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithBudgets(map[string]int{"tool_calls": 1}),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		_, err = engine.Run(ctx, "test budget exceeded")

		if err == nil {
			t.Fatal("expected error for budget exceeded, got nil")
		}
		if !errors.Is(err, policy.ErrBudgetExceeded) {
			t.Errorf("expected ErrBudgetExceeded, got: %v", err)
		}
	})

	t.Run("budget_allows_within_limit", func(t *testing.T) {
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			}).
			MustBuild()

		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "first"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "second"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "to decide"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewFinishDecision("done", json.RawMessage(`{}`)),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithBudgets(map[string]int{"tool_calls": 5}),
			api.WithMaxSteps(10),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test within budget")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed, got: %s", run.Status)
		}
	})
}

// =============================================================================
// Invariant 5: State Semantics
// Only Act state allows side effects, terminal states are final.
// =============================================================================

func TestInvariant_StateSemantics(t *testing.T) {
	t.Run("terminal_states_end_execution", func(t *testing.T) {
		if !agent.StateDone.IsTerminal() {
			t.Error("done state should be terminal")
		}
		if !agent.StateFailed.IsTerminal() {
			t.Error("failed state should be terminal")
		}
		if agent.StateExplore.IsTerminal() {
			t.Error("explore state should not be terminal")
		}
	})

	t.Run("side_effects_semantics", func(t *testing.T) {
		if !agent.StateAct.AllowsSideEffects() {
			t.Error("act state should allow side effects")
		}
		if agent.StateExplore.AllowsSideEffects() {
			t.Error("explore state should not allow side effects")
		}
		if agent.StateDecide.AllowsSideEffects() {
			t.Error("decide state should not allow side effects")
		}
	})
}

// =============================================================================
// Invariant 6: Tool Registration Uniqueness
// Tool names must be unique within a registry.
// =============================================================================

func TestInvariant_ToolRegistration(t *testing.T) {
	t.Run("duplicate_tool_rejected", func(t *testing.T) {
		registry := api.NewToolRegistry()

		tool1 := api.NewToolBuilder("my_tool").
			WithDescription("First tool").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{}, nil
			}).
			MustBuild()

		tool2 := api.NewToolBuilder("my_tool").
			WithDescription("Second tool with same name").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{}, nil
			}).
			MustBuild()

		err := registry.Register(tool1)
		if err != nil {
			t.Fatalf("first registration should succeed: %v", err)
		}

		err = registry.Register(tool2)
		if err == nil {
			t.Fatal("second registration should fail with duplicate name")
		}
		if !errors.Is(err, tool.ErrToolExists) {
			t.Errorf("expected ErrToolAlreadyExists, got: %v", err)
		}
	})

	t.Run("tool_not_found_error", func(t *testing.T) {
		registry := api.NewToolRegistry()

		_, ok := registry.Get("nonexistent")
		if ok {
			t.Error("nonexistent tool should not be found")
		}
	})
}

// =============================================================================
// Invariant 7: Run Lifecycle
// A run progresses through states and reaches terminal state.
// =============================================================================

func TestInvariant_RunLifecycle(t *testing.T) {
	t.Run("run_starts_in_intake", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")
		if run.CurrentState != agent.StateIntake {
			t.Errorf("new run should start in intake, got: %s", run.CurrentState)
		}
		if run.Status != agent.RunStatusPending {
			t.Errorf("new run should be pending, got: %s", run.Status)
		}
	})

	t.Run("run_completion_sets_result", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")
		run.Start()
		result := json.RawMessage(`{"answer": 42}`)
		run.Complete(result)

		if run.Status != agent.RunStatusCompleted {
			t.Errorf("completed run should have completed status, got: %s", run.Status)
		}
		if string(run.Result) != `{"answer": 42}` {
			t.Errorf("result mismatch: got %s", run.Result)
		}
	})

	t.Run("run_failure_sets_error", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")
		run.Start()
		run.Fail("something went wrong")

		if run.Status != agent.RunStatusFailed {
			t.Errorf("failed run should have failed status, got: %s", run.Status)
		}
		if run.Error != "something went wrong" {
			t.Errorf("error mismatch: got %s", run.Error)
		}
	})
}

// =============================================================================
// Invariant 8: Evidence Accumulation
// Evidence is append-only and preserves order.
// =============================================================================

func TestInvariant_EvidenceAccumulation(t *testing.T) {
	t.Run("evidence_is_append_only", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")

		e1 := agent.NewToolEvidence("tool1", json.RawMessage(`{"result": 1}`))
		e2 := agent.NewToolEvidence("tool2", json.RawMessage(`{"result": 2}`))
		e3 := agent.NewToolEvidence("tool3", json.RawMessage(`{"result": 3}`))

		run.AddEvidence(e1)
		run.AddEvidence(e2)
		run.AddEvidence(e3)

		if len(run.Evidence) != 3 {
			t.Errorf("expected 3 evidence items, got: %d", len(run.Evidence))
		}

		// Verify order is preserved
		if run.Evidence[0].Source != "tool1" {
			t.Errorf("first evidence should be tool1, got: %s", run.Evidence[0].Source)
		}
		if run.Evidence[1].Source != "tool2" {
			t.Errorf("second evidence should be tool2, got: %s", run.Evidence[1].Source)
		}
		if run.Evidence[2].Source != "tool3" {
			t.Errorf("third evidence should be tool3, got: %s", run.Evidence[2].Source)
		}
	})

	t.Run("evidence_timestamps_are_sequential", func(t *testing.T) {
		run := agent.NewRun("test-run", "test goal")

		run.AddEvidence(agent.NewToolEvidence("tool1", json.RawMessage(`{}`)))
		run.AddEvidence(agent.NewToolEvidence("tool2", json.RawMessage(`{}`)))

		// Second evidence should have timestamp >= first
		if run.Evidence[1].Timestamp.Before(run.Evidence[0].Timestamp) {
			t.Error("evidence timestamps should be sequential")
		}
	})
}

// =============================================================================
// Invariant 9: Cache Correctness
// Cache produces deterministic results and respects TTL/eviction policies.
// =============================================================================

func TestInvariant_CacheCorrectness(t *testing.T) {
	t.Run("cache_produces_deterministic_results", func(t *testing.T) {
		c := memory.NewCache()
		ctx := context.Background()

		key := "test-key"
		value := []byte(`{"result": "cached value"}`)

		// First set
		err := c.Set(ctx, key, value, cache.SetOptions{})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		// Multiple gets should return identical values
		for i := 0; i < 10; i++ {
			got, found, err := c.Get(ctx, key)
			if err != nil {
				t.Fatalf("Get %d failed: %v", i, err)
			}
			if !found {
				t.Fatalf("Get %d: expected to find cached value", i)
			}
			if !bytes.Equal(got, value) {
				t.Errorf("Get %d: expected %s, got %s", i, value, got)
			}
		}
	})

	t.Run("cache_ttl_expiration", func(t *testing.T) {
		c := memory.NewCache()
		ctx := context.Background()

		key := "expiring-key"
		value := []byte(`{"data": "temporary"}`)

		// Set with short TTL
		err := c.Set(ctx, key, value, cache.SetOptions{TTL: 50 * time.Millisecond})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		// Should find immediately
		_, found, _ := c.Get(ctx, key)
		if !found {
			t.Error("expected to find value immediately after set")
		}

		// Wait for TTL to expire
		time.Sleep(100 * time.Millisecond)

		// Should not find after expiration
		_, found, _ = c.Get(ctx, key)
		if found {
			t.Error("expected value to be expired")
		}
	})

	t.Run("cache_lru_eviction_preserves_recent", func(t *testing.T) {
		// Create cache with small max size
		c := memory.NewCache(memory.WithMaxSize(3))
		ctx := context.Background()

		// Fill cache
		_ = c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
		_ = c.Set(ctx, "key2", []byte("value2"), cache.SetOptions{})
		_ = c.Set(ctx, "key3", []byte("value3"), cache.SetOptions{})

		// Access key1 to make it recently used
		_, _, _ = c.Get(ctx, "key1")

		// Add new key, triggering eviction
		_ = c.Set(ctx, "key4", []byte("value4"), cache.SetOptions{})

		// key1 should still exist (recently accessed)
		_, found1, _ := c.Get(ctx, "key1")
		if !found1 {
			t.Error("recently accessed key1 should survive eviction")
		}

		// key4 should exist (just added)
		_, found4, _ := c.Get(ctx, "key4")
		if !found4 {
			t.Error("newly added key4 should exist")
		}
	})

	t.Run("cache_returns_copies_not_references", func(t *testing.T) {
		c := memory.NewCache()
		ctx := context.Background()

		key := "immutable-key"
		originalValue := []byte(`{"original": true}`)

		_ = c.Set(ctx, key, originalValue, cache.SetOptions{})

		// Get the value
		retrieved, _, _ := c.Get(ctx, key)

		// Modify the retrieved value
		retrieved[0] = 'X'

		// Get again and verify original is unchanged
		checkValue, _, _ := c.Get(ctx, key)
		if checkValue[0] == 'X' {
			t.Error("cache should return copies, not references")
		}
	})
}

// =============================================================================
// Invariant 10: Replay Determinism
// Event replay produces identical run state from the same events.
// =============================================================================

func TestInvariant_ReplayDeterminism(t *testing.T) {
	t.Run("reconstruct_run_from_events", func(t *testing.T) {
		eventStore := memory.NewEventStore()
		ctx := context.Background()
		runID := "replay-test-run"

		// Create a sequence of events
		events := []event.Event{
			mustCreateEvent(t, runID, event.TypeRunStarted, event.RunStartedPayload{
				Goal: "test goal",
				Vars: map[string]any{"key": "value"},
			}),
			mustCreateEvent(t, runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
				FromState: agent.StateIntake,
				ToState:   agent.StateExplore,
				Reason:    "begin exploration",
			}),
			mustCreateEvent(t, runID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
				Type:    string(agent.EvidenceToolResult),
				Source:  "read_file",
				Content: json.RawMessage(`{"data": "file content"}`),
			}),
			mustCreateEvent(t, runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
				FromState: agent.StateExplore,
				ToState:   agent.StateDecide,
				Reason:    "done exploring",
			}),
			mustCreateEvent(t, runID, event.TypeRunCompleted, event.RunCompletedPayload{
				Result:   json.RawMessage(`{"status": "success"}`),
				Duration: 5 * time.Second,
			}),
		}

		// Append events
		if err := eventStore.Append(ctx, events...); err != nil {
			t.Fatalf("failed to append events: %v", err)
		}

		// Replay and reconstruct
		replay := application.NewReplay(eventStore)
		run, err := replay.ReconstructRun(ctx, runID)
		if err != nil {
			t.Fatalf("ReconstructRun failed: %v", err)
		}

		// Verify reconstructed state
		if run.ID != runID {
			t.Errorf("expected run ID %s, got %s", runID, run.ID)
		}
		if run.Goal != "test goal" {
			t.Errorf("expected goal 'test goal', got %s", run.Goal)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Errorf("expected completed status, got %s", run.Status)
		}
		if len(run.Evidence) != 1 {
			t.Errorf("expected 1 evidence item, got %d", len(run.Evidence))
		}
	})

	t.Run("multiple_replays_produce_identical_results", func(t *testing.T) {
		eventStore := memory.NewEventStore()
		ctx := context.Background()
		runID := "determinism-test"

		// Create events
		events := []event.Event{
			mustCreateEvent(t, runID, event.TypeRunStarted, event.RunStartedPayload{
				Goal: "determinism test",
			}),
			mustCreateEvent(t, runID, event.TypeVariableSet, event.VariableSetPayload{
				Key:   "counter",
				Value: 42,
			}),
			mustCreateEvent(t, runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
				FromState: agent.StateIntake,
				ToState:   agent.StateExplore,
				Reason:    "start",
			}),
		}

		_ = eventStore.Append(ctx, events...)

		replay := application.NewReplay(eventStore)

		// Replay multiple times
		var runs []*agent.Run
		for i := 0; i < 5; i++ {
			run, err := replay.ReconstructRun(ctx, runID)
			if err != nil {
				t.Fatalf("Replay %d failed: %v", i, err)
			}
			runs = append(runs, run)
		}

		// Verify all replays are identical
		first := runs[0]
		for i := 1; i < len(runs); i++ {
			if runs[i].ID != first.ID {
				t.Errorf("Replay %d: ID mismatch", i)
			}
			if runs[i].Goal != first.Goal {
				t.Errorf("Replay %d: Goal mismatch", i)
			}
			if runs[i].CurrentState != first.CurrentState {
				t.Errorf("Replay %d: CurrentState mismatch", i)
			}
		}
	})

	t.Run("timeline_state_transitions_ordered", func(t *testing.T) {
		eventStore := memory.NewEventStore()
		ctx := context.Background()
		runID := "timeline-test"

		// Create events with state transitions
		events := []event.Event{
			mustCreateEvent(t, runID, event.TypeRunStarted, event.RunStartedPayload{Goal: "timeline"}),
			mustCreateEvent(t, runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
				FromState: agent.StateIntake, ToState: agent.StateExplore, Reason: "1",
			}),
			mustCreateEvent(t, runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
				FromState: agent.StateExplore, ToState: agent.StateDecide, Reason: "2",
			}),
			mustCreateEvent(t, runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
				FromState: agent.StateDecide, ToState: agent.StateAct, Reason: "3",
			}),
		}

		_ = eventStore.Append(ctx, events...)

		replay := application.NewReplay(eventStore)
		timeline, err := replay.NewTimeline(ctx, runID)
		if err != nil {
			t.Fatalf("NewTimeline failed: %v", err)
		}

		transitions := timeline.StateTransitions()
		if len(transitions) != 3 {
			t.Fatalf("expected 3 transitions, got %d", len(transitions))
		}

		// Verify order
		expectedStates := []agent.State{agent.StateExplore, agent.StateDecide, agent.StateAct}
		for i, tr := range transitions {
			if tr.To != expectedStates[i] {
				t.Errorf("transition %d: expected %s, got %s", i, expectedStates[i], tr.To)
			}
		}
	})
}

// =============================================================================
// Invariant 11: Artifact Integrity
// Artifacts maintain stable references and preserve content integrity.
// =============================================================================

func TestInvariant_ArtifactIntegrity(t *testing.T) {
	t.Run("artifact_ref_id_is_stable", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "artifact-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		store, err := filesystem.NewArtifactStore(tempDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		ctx := context.Background()
		content := []byte("test artifact content")

		// Store artifact
		ref, err := store.Store(ctx, bytes.NewReader(content), artifact.StoreOptions{
			Name:            "test-artifact",
			ContentType:     "text/plain",
			ComputeChecksum: true,
		})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		// ID should be non-empty and stable
		if ref.ID == "" {
			t.Error("artifact ID should not be empty")
		}

		// Retrieve metadata and verify ID is unchanged
		metaRef, err := store.Metadata(ctx, ref)
		if err != nil {
			t.Fatalf("Metadata failed: %v", err)
		}
		if metaRef.ID != ref.ID {
			t.Errorf("artifact ID changed: expected %s, got %s", ref.ID, metaRef.ID)
		}
	})

	t.Run("artifact_content_preserved_exactly", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "artifact-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		store, err := filesystem.NewArtifactStore(tempDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		ctx := context.Background()

		// Test various content types
		testCases := []struct {
			name    string
			content []byte
		}{
			{"empty", []byte{}},
			{"text", []byte("Hello, World!")},
			{"binary", []byte{0x00, 0xFF, 0x42, 0x89, 0xAB}},
			{"json", []byte(`{"key": "value", "number": 123}`)},
			{"large", bytes.Repeat([]byte("X"), 10000)},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ref, err := store.Store(ctx, bytes.NewReader(tc.content), artifact.DefaultStoreOptions())
				if err != nil {
					t.Fatalf("Store failed: %v", err)
				}

				// Retrieve and compare
				reader, err := store.Retrieve(ctx, ref)
				if err != nil {
					t.Fatalf("Retrieve failed: %v", err)
				}
				defer reader.Close()

				retrieved, err := io.ReadAll(reader)
				if err != nil {
					t.Fatalf("ReadAll failed: %v", err)
				}

				if !bytes.Equal(retrieved, tc.content) {
					t.Errorf("content mismatch: expected %d bytes, got %d bytes",
						len(tc.content), len(retrieved))
				}
			})
		}
	})

	t.Run("artifact_checksum_validates_integrity", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "artifact-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		store, err := filesystem.NewArtifactStore(tempDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		ctx := context.Background()
		content := []byte("content for checksum verification")

		ref, err := store.Store(ctx, bytes.NewReader(content), artifact.StoreOptions{
			ComputeChecksum: true,
		})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		// Checksum should be computed
		if ref.Checksum == "" {
			t.Error("checksum should be computed when requested")
		}

		// Store same content again - should produce same checksum
		ref2, err := store.Store(ctx, bytes.NewReader(content), artifact.StoreOptions{
			ComputeChecksum: true,
		})
		if err != nil {
			t.Fatalf("Second Store failed: %v", err)
		}

		if ref.Checksum != ref2.Checksum {
			t.Errorf("same content should produce same checksum: %s vs %s",
				ref.Checksum, ref2.Checksum)
		}

		// Different content should produce different checksum
		ref3, err := store.Store(ctx, bytes.NewReader([]byte("different content")), artifact.StoreOptions{
			ComputeChecksum: true,
		})
		if err != nil {
			t.Fatalf("Third Store failed: %v", err)
		}

		if ref.Checksum == ref3.Checksum {
			t.Error("different content should produce different checksum")
		}
	})

	t.Run("artifact_exists_and_delete", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "artifact-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		store, err := filesystem.NewArtifactStore(tempDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		ctx := context.Background()
		content := []byte("deletable content")

		ref, err := store.Store(ctx, bytes.NewReader(content), artifact.DefaultStoreOptions())
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		// Should exist after store
		exists, err := store.Exists(ctx, ref)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("artifact should exist after store")
		}

		// Delete
		err = store.Delete(ctx, ref)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Should not exist after delete
		exists, _ = store.Exists(ctx, ref)
		if exists {
			t.Error("artifact should not exist after delete")
		}

		// Retrieve should fail
		_, err = store.Retrieve(ctx, ref)
		if !errors.Is(err, artifact.ErrArtifactNotFound) {
			t.Errorf("expected ErrArtifactNotFound, got: %v", err)
		}
	})

	t.Run("artifact_metadata_preserved", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "artifact-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		store, err := filesystem.NewArtifactStore(tempDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		ctx := context.Background()
		content := []byte("content with metadata")

		ref, err := store.Store(ctx, bytes.NewReader(content), artifact.StoreOptions{
			Name:            "my-artifact",
			ContentType:     "application/json",
			ComputeChecksum: true,
			Metadata: map[string]string{
				"source": "test",
				"runID":  "run-123",
			},
		})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		// Retrieve metadata
		meta, err := store.Metadata(ctx, ref)
		if err != nil {
			t.Fatalf("Metadata failed: %v", err)
		}

		// Verify all metadata preserved
		if meta.Name != "my-artifact" {
			t.Errorf("name not preserved: expected 'my-artifact', got %s", meta.Name)
		}
		if meta.ContentType != "application/json" {
			t.Errorf("content type not preserved: expected 'application/json', got %s", meta.ContentType)
		}
		if meta.Size != int64(len(content)) {
			t.Errorf("size not preserved: expected %d, got %d", len(content), meta.Size)
		}
		if meta.Metadata["source"] != "test" {
			t.Errorf("metadata 'source' not preserved")
		}
		if meta.Metadata["runID"] != "run-123" {
			t.Errorf("metadata 'runID' not preserved")
		}
	})
}

// Helper function to create events for tests
func mustCreateEvent(t *testing.T, runID string, eventType event.Type, payload any) event.Event {
	t.Helper()
	e, err := event.NewEvent(runID, eventType, payload)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}
	return e
}

// =============================================================================
// Invariant 12: Ledger Completeness
// All significant operations are recorded in the ledger for audit purposes.
// =============================================================================

func TestInvariant_LedgerCompleteness(t *testing.T) {
	t.Run("all_tool_executions_are_recorded", func(t *testing.T) {
		// Create a read-only tool
		readTool := api.NewToolBuilder("read_file").
			WithDescription("Reads a file").
			WithAnnotations(api.Annotations{ReadOnly: true}).
			WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{"content": "hello"}`)}, nil
			}).
			MustBuild()

		// Create registry and eligibility
		registry := api.NewToolRegistry()
		if err := registry.Register(readTool); err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		// Create scripted planner that calls the tool multiple times
		scriptedPlanner := api.NewScriptedPlanner(
			api.ScriptStep{
				ExpectState: agent.StateIntake,
				Decision:    api.NewTransitionDecision(agent.StateExplore, "to explore"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{"path": "/file1"}`), "read first"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewCallToolDecision("read_file", json.RawMessage(`{"path": "/file2"}`), "read second"),
			},
			api.ScriptStep{
				ExpectState: agent.StateExplore,
				Decision:    api.NewTransitionDecision(agent.StateDecide, "done exploring"),
			},
			api.ScriptStep{
				ExpectState: agent.StateDecide,
				Decision:    api.NewFinishDecision("completed", json.RawMessage(`{}`)),
			},
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(scriptedPlanner),
			api.WithToolEligibility(eligibility),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(20),
		)
		if err != nil {
			t.Fatalf("failed to create engine: %v", err)
		}

		ctx := context.Background()
		run, err := engine.Run(ctx, "test tool recording")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if run.Status != agent.RunStatusCompleted {
			t.Fatalf("expected completed, got: %s", run.Status)
		}

		// Verify evidence contains tool results (which come from ledger recording)
		toolEvidenceCount := 0
		for _, e := range run.Evidence {
			if e.Type == agent.EvidenceToolResult {
				toolEvidenceCount++
			}
		}

		if toolEvidenceCount != 2 {
			t.Errorf("expected 2 tool evidence items, got: %d", toolEvidenceCount)
		}
	})

	t.Run("all_state_transitions_are_recorded", func(t *testing.T) {
		// Test that ledger records state transitions by verifying
		// the run's state history matches expected transitions
		l := ledger.New("test-run-transitions")

		// Record a sequence of transitions
		transitions := []struct {
			from   agent.State
			to     agent.State
			reason string
		}{
			{agent.StateIntake, agent.StateExplore, "begin exploration"},
			{agent.StateExplore, agent.StateDecide, "gathered enough info"},
			{agent.StateDecide, agent.StateAct, "decided to act"},
			{agent.StateAct, agent.StateValidate, "action complete"},
			{agent.StateValidate, agent.StateDone, "validated successfully"},
		}

		for _, tr := range transitions {
			l.RecordTransition(tr.from, tr.to, tr.reason)
		}

		// Query by type
		transitionEntries := l.EntriesByType(ledger.EntryStateTransition)
		if len(transitionEntries) != len(transitions) {
			t.Errorf("expected %d transition entries, got: %d", len(transitions), len(transitionEntries))
		}

		// Verify each transition was recorded correctly
		for i, entry := range transitionEntries {
			var details ledger.TransitionDetails
			if err := entry.DecodeDetails(&details); err != nil {
				t.Fatalf("failed to decode details: %v", err)
			}
			if details.FromState != transitions[i].from {
				t.Errorf("transition %d: expected from %s, got %s", i, transitions[i].from, details.FromState)
			}
			if details.ToState != transitions[i].to {
				t.Errorf("transition %d: expected to %s, got %s", i, transitions[i].to, details.ToState)
			}
			if details.Reason != transitions[i].reason {
				t.Errorf("transition %d: expected reason %s, got %s", i, transitions[i].reason, details.Reason)
			}
		}
	})

	t.Run("entries_are_immutable_and_append_only", func(t *testing.T) {
		l := ledger.New("test-run-immutable")

		// Record initial entry
		l.RecordRunStarted("test goal")
		initialCount := l.Count()

		// Get entries - should be a copy
		entries := l.Entries()
		initialEntryID := entries[0].ID

		// Add more entries
		l.RecordTransition(agent.StateIntake, agent.StateExplore, "transition")
		l.RecordToolCall(agent.StateExplore, "test_tool", json.RawMessage(`{}`))

		// Verify count increased (append-only)
		if l.Count() != initialCount+2 {
			t.Errorf("expected %d entries, got %d", initialCount+2, l.Count())
		}

		// Verify original entry ID is unchanged (immutable)
		newEntries := l.Entries()
		if newEntries[0].ID != initialEntryID {
			t.Error("original entry ID changed - entries should be immutable")
		}

		// Verify entries returned are copies (modifying copy shouldn't affect ledger)
		copiedEntries := l.Entries()
		originalTimestamp := copiedEntries[0].Timestamp
		copiedEntries[0].Timestamp = time.Time{} // Try to modify

		freshEntries := l.Entries()
		if freshEntries[0].Timestamp.IsZero() {
			t.Error("modifying returned entries affected ledger - should return copies")
		}
		if freshEntries[0].Timestamp != originalTimestamp {
			t.Error("timestamp changed - entries should be immutable")
		}
	})

	t.Run("timestamps_and_ordering_are_correct", func(t *testing.T) {
		l := ledger.New("test-run-timestamps")

		// Record entries with small delays
		l.RecordRunStarted("test goal")
		time.Sleep(1 * time.Millisecond)
		l.RecordTransition(agent.StateIntake, agent.StateExplore, "t1")
		time.Sleep(1 * time.Millisecond)
		l.RecordToolCall(agent.StateExplore, "tool1", json.RawMessage(`{}`))
		time.Sleep(1 * time.Millisecond)
		l.RecordToolResult(agent.StateExplore, "tool1", json.RawMessage(`{}`), 100*time.Millisecond, false)

		entries := l.Entries()
		if len(entries) != 4 {
			t.Fatalf("expected 4 entries, got %d", len(entries))
		}

		// Verify chronological ordering
		for i := 1; i < len(entries); i++ {
			if entries[i].Timestamp.Before(entries[i-1].Timestamp) {
				t.Errorf("entry %d timestamp (%v) is before entry %d timestamp (%v)",
					i, entries[i].Timestamp, i-1, entries[i-1].Timestamp)
			}
		}

		// Verify all timestamps are non-zero
		for i, e := range entries {
			if e.Timestamp.IsZero() {
				t.Errorf("entry %d has zero timestamp", i)
			}
		}

		// Verify all entries have unique IDs
		ids := make(map[string]bool)
		for i, e := range entries {
			if e.ID == "" {
				t.Errorf("entry %d has empty ID", i)
			}
			if ids[e.ID] {
				t.Errorf("entry %d has duplicate ID: %s", i, e.ID)
			}
			ids[e.ID] = true
		}
	})

	t.Run("queryable_by_run_id", func(t *testing.T) {
		// Create two ledgers for different runs
		l1 := ledger.New("run-1")
		l2 := ledger.New("run-2")

		l1.RecordRunStarted("goal 1")
		l1.RecordToolCall(agent.StateExplore, "tool1", json.RawMessage(`{}`))

		l2.RecordRunStarted("goal 2")
		l2.RecordToolCall(agent.StateExplore, "tool2", json.RawMessage(`{}`))
		l2.RecordToolCall(agent.StateExplore, "tool3", json.RawMessage(`{}`))

		// Verify each ledger only contains its run's entries
		if l1.RunID() != "run-1" {
			t.Errorf("expected run ID 'run-1', got: %s", l1.RunID())
		}
		if l2.RunID() != "run-2" {
			t.Errorf("expected run ID 'run-2', got: %s", l2.RunID())
		}

		// Verify entry counts
		if l1.Count() != 2 {
			t.Errorf("run-1 should have 2 entries, got: %d", l1.Count())
		}
		if l2.Count() != 3 {
			t.Errorf("run-2 should have 3 entries, got: %d", l2.Count())
		}

		// Verify all entries in each ledger have correct run ID
		for _, e := range l1.Entries() {
			if e.RunID != "run-1" {
				t.Errorf("entry in l1 has wrong run ID: %s", e.RunID)
			}
		}
		for _, e := range l2.Entries() {
			if e.RunID != "run-2" {
				t.Errorf("entry in l2 has wrong run ID: %s", e.RunID)
			}
		}
	})

	t.Run("queryable_by_type", func(t *testing.T) {
		l := ledger.New("test-run-query-type")

		// Record various entry types
		l.RecordRunStarted("test goal")
		l.RecordTransition(agent.StateIntake, agent.StateExplore, "transition 1")
		l.RecordTransition(agent.StateExplore, agent.StateDecide, "transition 2")
		l.RecordDecision(agent.StateDecide, agent.Decision{
			Type:     agent.DecisionCallTool,
			CallTool: &agent.CallToolDecision{ToolName: "test_tool"},
		})
		l.RecordToolCall(agent.StateExplore, "tool1", json.RawMessage(`{}`))
		l.RecordToolCall(agent.StateExplore, "tool2", json.RawMessage(`{}`))
		l.RecordToolResult(agent.StateExplore, "tool1", json.RawMessage(`{}`), 100*time.Millisecond, false)
		l.RecordBudgetConsumed(agent.StateExplore, "tool_calls", 1, 9)
		l.RecordRunCompleted(json.RawMessage(`{"result": "success"}`))

		// Query by different types and verify counts
		typeTests := []struct {
			entryType ledger.EntryType
			expected  int
		}{
			{ledger.EntryRunStarted, 1},
			{ledger.EntryRunCompleted, 1},
			{ledger.EntryStateTransition, 2},
			{ledger.EntryDecision, 1},
			{ledger.EntryToolCall, 2},
			{ledger.EntryToolResult, 1},
			{ledger.EntryBudgetConsumed, 1},
		}

		for _, tc := range typeTests {
			entries := l.EntriesByType(tc.entryType)
			if len(entries) != tc.expected {
				t.Errorf("expected %d entries of type %s, got %d", tc.expected, tc.entryType, len(entries))
			}

			// Verify all returned entries have correct type
			for _, e := range entries {
				if e.Type != tc.entryType {
					t.Errorf("entry has wrong type: expected %s, got %s", tc.entryType, e.Type)
				}
			}
		}
	})

	t.Run("last_entry_returns_most_recent", func(t *testing.T) {
		l := ledger.New("test-run-last")

		// Empty ledger should return nil
		if l.LastEntry() != nil {
			t.Error("empty ledger should return nil for LastEntry")
		}

		// Add entries and verify last entry is correct
		l.RecordRunStarted("test")
		last := l.LastEntry()
		if last == nil || last.Type != ledger.EntryRunStarted {
			t.Error("last entry should be run_started")
		}

		l.RecordTransition(agent.StateIntake, agent.StateExplore, "transition")
		last = l.LastEntry()
		if last == nil || last.Type != ledger.EntryStateTransition {
			t.Error("last entry should be state_transition")
		}

		l.RecordToolCall(agent.StateExplore, "tool", json.RawMessage(`{}`))
		last = l.LastEntry()
		if last == nil || last.Type != ledger.EntryToolCall {
			t.Error("last entry should be tool_call")
		}
	})

	t.Run("all_entry_types_are_recorded_correctly", func(t *testing.T) {
		l := ledger.New("test-run-all-types")

		// Record all entry types
		l.RecordRunStarted("goal")
		l.RecordTransition(agent.StateIntake, agent.StateExplore, "reason")
		l.RecordDecision(agent.StateExplore, agent.Decision{
			Type:     agent.DecisionCallTool,
			CallTool: &agent.CallToolDecision{ToolName: "test"},
		})
		l.RecordToolCall(agent.StateExplore, "tool", json.RawMessage(`{"input": 1}`))
		l.RecordToolResult(agent.StateExplore, "tool", json.RawMessage(`{"output": 2}`), 50*time.Millisecond, false)
		l.RecordToolError(agent.StateExplore, "tool", errors.New("test error"))
		l.RecordApprovalRequest(agent.StateAct, "dangerous_tool", json.RawMessage(`{}`), "high")
		l.RecordApprovalResult(agent.StateAct, "dangerous_tool", true, "admin", "approved for testing")
		l.RecordBudgetConsumed(agent.StateExplore, "tool_calls", 1, 99)
		l.RecordBudgetExhausted(agent.StateExplore, "api_tokens")
		l.RecordHumanInputRequest(agent.StateDecide, "Continue?", []string{"yes", "no"})
		l.RecordHumanInputResponse(agent.StateDecide, "Continue?", "yes")
		l.RecordRunCompleted(json.RawMessage(`{"final": "result"}`))

		// Verify total count
		if l.Count() != 13 {
			t.Errorf("expected 13 entries, got %d", l.Count())
		}

		// Verify each type was recorded
		expectedTypes := map[ledger.EntryType]int{
			ledger.EntryRunStarted:         1,
			ledger.EntryStateTransition:    1,
			ledger.EntryDecision:           1,
			ledger.EntryToolCall:           1,
			ledger.EntryToolResult:         1,
			ledger.EntryToolError:          1,
			ledger.EntryApprovalRequest:    1,
			ledger.EntryApprovalResult:     1,
			ledger.EntryBudgetConsumed:     1,
			ledger.EntryBudgetExhausted:    1,
			ledger.EntryHumanInputRequest:  1,
			ledger.EntryHumanInputResponse: 1,
			ledger.EntryRunCompleted:       1,
		}

		for entryType, expectedCount := range expectedTypes {
			entries := l.EntriesByType(entryType)
			if len(entries) != expectedCount {
				t.Errorf("expected %d entries of type %s, got %d", expectedCount, entryType, len(entries))
			}
		}
	})

	t.Run("run_failed_is_recorded", func(t *testing.T) {
		l := ledger.New("test-run-failed")

		l.RecordRunStarted("will fail")
		l.RecordTransition(agent.StateIntake, agent.StateExplore, "start")
		l.RecordRunFailed(agent.StateExplore, "something went wrong")

		failedEntries := l.EntriesByType(ledger.EntryRunFailed)
		if len(failedEntries) != 1 {
			t.Fatalf("expected 1 run_failed entry, got %d", len(failedEntries))
		}

		// Verify failure details
		var details map[string]string
		if err := failedEntries[0].DecodeDetails(&details); err != nil {
			t.Fatalf("failed to decode details: %v", err)
		}
		if details["reason"] != "something went wrong" {
			t.Errorf("expected reason 'something went wrong', got: %s", details["reason"])
		}
	})

	t.Run("concurrent_access_is_thread_safe", func(t *testing.T) {
		l := ledger.New("test-run-concurrent")

		// Spawn multiple goroutines writing to the ledger
		const numGoroutines = 10
		const entriesPerGoroutine = 100

		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < entriesPerGoroutine; j++ {
					l.RecordToolCall(
						agent.StateExplore,
						"tool",
						json.RawMessage(`{}`),
					)
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify total count
		expectedTotal := numGoroutines * entriesPerGoroutine
		if l.Count() != expectedTotal {
			t.Errorf("expected %d entries, got %d", expectedTotal, l.Count())
		}
	})
}
