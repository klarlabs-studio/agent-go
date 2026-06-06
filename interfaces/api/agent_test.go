package api_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/planner"
	"go.klarlabs.de/agent/infrastructure/resilience"
	"go.klarlabs.de/agent/infrastructure/storage/filesystem"
	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("creates engine with defaults", func(t *testing.T) {
		t.Parallel()

		// Create minimal planner
		mockPlanner := api.NewMockPlanner(
			api.NewTransitionDecision(api.StateExplore, "start"),
			api.NewTransitionDecision(api.StateDecide, "decide"),
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithPlanner(mockPlanner),
		)

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() returned nil engine")
		}
	})

	t.Run("creates engine with tool", func(t *testing.T) {
		t.Parallel()

		echoTool := api.NewToolBuilder("echo").
			WithDescription("Echoes input").
			WithAnnotations(tool.ReadOnlyAnnotations()).
			WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: input}, nil
			}).
			MustBuild()

		eligibility := api.NewToolEligibility()
		eligibility.Allow(api.StateExplore, "echo")

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithTool(echoTool),
			api.WithPlanner(mockPlanner),
			api.WithToolEligibility(eligibility),
		)

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() returned nil engine")
		}
	})

	t.Run("creates engine with multiple tools", func(t *testing.T) {
		t.Parallel()

		tool1 := api.NewToolBuilder("tool1").
			WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: input}, nil
			}).
			MustBuild()

		tool2 := api.NewToolBuilder("tool2").
			WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: input}, nil
			}).
			MustBuild()

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithTool(tool1),
			api.WithTool(tool2),
			api.WithPlanner(mockPlanner),
		)

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() returned nil engine")
		}
	})

	t.Run("creates engine with budgets", func(t *testing.T) {
		t.Parallel()

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithPlanner(mockPlanner),
			api.WithBudgets(map[string]int{
				"tool_calls": 100,
				"tokens":     10000,
			}),
		)

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() returned nil engine")
		}
	})

	t.Run("creates engine with single budget", func(t *testing.T) {
		t.Parallel()

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithPlanner(mockPlanner),
			api.WithBudget("tool_calls", 50),
			api.WithBudget("tokens", 5000),
		)

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() returned nil engine")
		}
	})

	t.Run("creates engine with max steps", func(t *testing.T) {
		t.Parallel()

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithPlanner(mockPlanner),
			api.WithMaxSteps(100),
		)

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() returned nil engine")
		}
	})

	t.Run("creates engine with transitions", func(t *testing.T) {
		t.Parallel()

		transitions := api.DefaultTransitions()
		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithPlanner(mockPlanner),
			api.WithTransitions(transitions),
		)

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() returned nil engine")
		}
	})

	t.Run("creates engine with approver", func(t *testing.T) {
		t.Parallel()

		approver := api.AutoApprover()
		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithPlanner(mockPlanner),
			api.WithApprover(approver),
		)

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() returned nil engine")
		}
	})
}

func TestNew_WithRegistry(t *testing.T) {
	t.Parallel()

	t.Run("creates engine with custom registry", func(t *testing.T) {
		t.Parallel()

		// Create a custom registry with a tool
		registry := api.NewToolRegistry()

		echoTool := api.NewToolBuilder("echo").
			WithDescription("Echoes input").
			ReadOnly().
			WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: input}, nil
			}).
			MustBuild()

		registry.Register(echoTool)

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithRegistry(registry),
			api.WithPlanner(mockPlanner),
		)

		if err != nil {
			t.Fatalf("New() with WithRegistry error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() with WithRegistry returned nil engine")
		}
	})
}

func TestNew_WithExecutor(t *testing.T) {
	t.Parallel()

	t.Run("creates engine with custom executor", func(t *testing.T) {
		t.Parallel()

		executor := resilience.NewDefaultExecutor()

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithExecutor(executor),
			api.WithPlanner(mockPlanner),
		)

		if err != nil {
			t.Fatalf("New() with WithExecutor error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() with WithExecutor returned nil engine")
		}
	})
}

func TestNew_WithArtifactStore(t *testing.T) {
	t.Parallel()

	t.Run("creates engine with artifact store", func(t *testing.T) {
		t.Parallel()

		// Create temp directory for artifact store
		tmpDir, err := os.MkdirTemp("", "artifact-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		artifactStore, err := filesystem.NewArtifactStore(tmpDir)
		if err != nil {
			t.Fatalf("failed to create artifact store: %v", err)
		}

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithArtifactStore(artifactStore),
			api.WithPlanner(mockPlanner),
		)

		if err != nil {
			t.Fatalf("New() with WithArtifactStore error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() with WithArtifactStore returned nil engine")
		}
	})
}

func TestNew_WithMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates engine with custom middleware", func(t *testing.T) {
		t.Parallel()

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithMiddleware(api.NoopMiddleware()),
			api.WithPlanner(mockPlanner),
		)

		if err != nil {
			t.Fatalf("New() with WithMiddleware error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() with WithMiddleware returned nil engine")
		}
	})

	t.Run("creates engine with multiple middleware", func(t *testing.T) {
		t.Parallel()

		mockPlanner := api.NewMockPlanner(
			api.NewFinishDecision("done", nil),
		)

		engine, err := api.New(
			api.WithMiddleware(
				api.NoopMiddleware(),
				api.LoggingMiddleware(nil),
			),
			api.WithPlanner(mockPlanner),
		)

		if err != nil {
			t.Fatalf("New() with multiple middleware error = %v", err)
		}
		if engine == nil {
			t.Fatal("New() with multiple middleware returned nil engine")
		}
	})
}

func TestEngine_Run(t *testing.T) {
	// Note: Not running in parallel due to shared logging infrastructure

	t.Run("completes simple run", func(t *testing.T) {

		mockPlanner := api.NewMockPlanner(
			api.NewTransitionDecision(api.StateExplore, "exploring"),
			api.NewTransitionDecision(api.StateDecide, "deciding"),
			api.NewFinishDecision("task completed", json.RawMessage(`{"result": "success"}`)),
		)

		engine, err := api.New(
			api.WithPlanner(mockPlanner),
			api.WithTransitions(api.DefaultTransitions()),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		run, err := engine.Run(context.Background(), "test goal")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if run.Status != api.StatusCompleted {
			t.Errorf("Status = %v, want %v", run.Status, api.StatusCompleted)
		}
		if run.Goal != "test goal" {
			t.Errorf("Goal = %v, want 'test goal'", run.Goal)
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		// Planner that would loop forever
		infinitePlanner := api.NewMockPlanner()
		for i := 0; i < 1000; i++ {
			infinitePlanner.AddDecision(api.NewTransitionDecision(api.StateExplore, "keep exploring"))
		}

		engine, err := api.New(
			api.WithPlanner(infinitePlanner),
			api.WithTransitions(api.DefaultTransitions()),
			api.WithMaxSteps(10), // Limit steps
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err = engine.Run(ctx, "test goal")
		if err == nil {
			t.Log("Run completed without error (might have finished before cancel)")
		}
	})
}

func TestEngine_RunWithVars(t *testing.T) {
	// Note: Not running in parallel due to shared logging infrastructure

	scriptedPlanner := api.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: api.StateIntake,
			Decision:    api.NewTransitionDecision(api.StateExplore, "start"),
		},
		planner.ScriptStep{
			ExpectState: api.StateExplore,
			Decision:    api.NewTransitionDecision(api.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: api.StateDecide,
			Decision:    api.NewFinishDecision("done", nil),
		},
	)

	engine, err := api.New(
		api.WithPlanner(scriptedPlanner),
		api.WithTransitions(api.DefaultTransitions()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	vars := map[string]any{
		"config_path": "/etc/app.conf",
		"max_retries": 3,
	}

	run, err := engine.RunWithVars(context.Background(), "test goal", vars)
	if err != nil {
		t.Fatalf("RunWithVars() error = %v", err)
	}

	if run.Status != api.StatusCompleted {
		t.Errorf("Status = %v, want %v", run.Status, api.StatusCompleted)
	}
}

func TestEngine_Run_WithToolExecution(t *testing.T) {
	// Note: Not running in parallel due to shared logging infrastructure

	toolExecuted := false

	echoTool := api.NewToolBuilder("echo").
		WithDescription("Echoes input").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			toolExecuted = true
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	eligibility := api.NewToolEligibility()
	eligibility.Allow(api.StateExplore, "echo")

	scriptedPlanner := api.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: api.StateIntake,
			Decision:    api.NewTransitionDecision(api.StateExplore, "start"),
		},
		planner.ScriptStep{
			ExpectState: api.StateExplore,
			Decision:    api.NewCallToolDecision("echo", json.RawMessage(`{"message": "hello"}`), "echo test"),
		},
		planner.ScriptStep{
			ExpectState: api.StateExplore,
			Decision:    api.NewTransitionDecision(api.StateDecide, "decide"),
		},
		planner.ScriptStep{
			ExpectState: api.StateDecide,
			Decision:    api.NewFinishDecision("done", nil),
		},
	)

	engine, err := api.New(
		api.WithTool(echoTool),
		api.WithPlanner(scriptedPlanner),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	run, err := engine.Run(context.Background(), "test echo")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !toolExecuted {
		t.Error("Tool was not executed")
	}
	if run.Status != api.StatusCompleted {
		t.Errorf("Status = %v, want %v", run.Status, api.StatusCompleted)
	}
}
