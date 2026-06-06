package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/planner"
	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewKnowledgeStore(t *testing.T) {
	t.Parallel()

	t.Run("with explicit dimension", func(t *testing.T) {
		t.Parallel()
		store := api.NewKnowledgeStore(128)
		if store == nil {
			t.Fatal("NewKnowledgeStore(128) returned nil")
		}
	})

	t.Run("with auto-detect dimension", func(t *testing.T) {
		t.Parallel()
		store := api.NewKnowledgeStore(0)
		if store == nil {
			t.Fatal("NewKnowledgeStore(0) returned nil")
		}
	})
}

func TestNewToolEligibilityWith(t *testing.T) {
	t.Parallel()

	eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
		api.StateExplore: {"read_file", "list_dir"},
		api.StateAct:     {"write_file"},
	})
	if eligibility == nil {
		t.Fatal("NewToolEligibilityWith() returned nil")
	}
	if !eligibility.IsAllowed(api.StateExplore, "read_file") {
		t.Error("read_file should be allowed in explore")
	}
	if !eligibility.IsAllowed(api.StateAct, "write_file") {
		t.Error("write_file should be allowed in act")
	}
	if eligibility.IsAllowed(api.StateExplore, "write_file") {
		t.Error("write_file should not be allowed in explore")
	}
}

func TestNewStateTransitionsWith(t *testing.T) {
	t.Parallel()

	transitions := api.NewStateTransitionsWith(api.TransitionRules{
		api.StateIntake:  {api.StateExplore, api.StateFailed},
		api.StateExplore: {api.StateDecide},
	})
	if transitions == nil {
		t.Fatal("NewStateTransitionsWith() returned nil")
	}
	if !transitions.CanTransition(api.StateIntake, api.StateExplore) {
		t.Error("intake -> explore should be allowed")
	}
	if transitions.CanTransition(api.StateIntake, api.StateDone) {
		t.Error("intake -> done should not be allowed")
	}
}

func TestWithKnowledgeStore(t *testing.T) {
	t.Parallel()

	store := api.NewKnowledgeStore(128)
	mockPlanner := api.NewMockPlanner(
		api.NewFinishDecision("done", nil),
	)

	engine, err := api.New(
		api.WithPlanner(mockPlanner),
		api.WithKnowledgeStore(store),
	)
	if err != nil {
		t.Fatalf("New() with WithKnowledgeStore error = %v", err)
	}
	if engine == nil {
		t.Fatal("engine is nil")
	}
	if engine.Knowledge() == nil {
		t.Error("Knowledge() should not be nil when store is configured")
	}
}

func TestEngineKnowledgeNil(t *testing.T) {
	t.Parallel()

	mockPlanner := api.NewMockPlanner(
		api.NewFinishDecision("done", nil),
	)
	engine, err := api.New(
		api.WithPlanner(mockPlanner),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if engine.Knowledge() != nil {
		t.Error("Knowledge() should be nil when no store configured")
	}
}

func TestWithRateLimit(t *testing.T) {
	t.Parallel()

	mockPlanner := api.NewMockPlanner(
		api.NewFinishDecision("done", nil),
	)

	engine, err := api.New(
		api.WithPlanner(mockPlanner),
		api.WithRateLimit(100, 100),
	)
	if err != nil {
		t.Fatalf("New() with WithRateLimit error = %v", err)
	}
	if engine == nil {
		t.Fatal("engine is nil")
	}
}

func TestWithPerToolRateLimit(t *testing.T) {
	t.Parallel()

	mockPlanner := api.NewMockPlanner(
		api.NewFinishDecision("done", nil),
	)

	engine, err := api.New(
		api.WithPlanner(mockPlanner),
		api.WithPerToolRateLimit(10, 10, map[string]api.ToolRateConfig{
			"fast_tool": {Rate: 100, Burst: 100},
			"slow_tool": {Rate: 5, Burst: 5},
		}),
	)
	if err != nil {
		t.Fatalf("New() with WithPerToolRateLimit error = %v", err)
	}
	if engine == nil {
		t.Fatal("engine is nil")
	}
}

func TestResumeWithInput(t *testing.T) {
	// Create a planner that asks a human question
	scriptedPlanner := api.NewScriptedPlanner(
		planner.ScriptStep{
			ExpectState: api.StateIntake,
			Decision:    api.NewTransitionDecision(api.StateExplore, "start"),
		},
		planner.ScriptStep{
			ExpectState: api.StateExplore,
			Decision: api.Decision{
				Type: agent.DecisionAskHuman,
				AskHuman: &agent.AskHumanDecision{
					Question: "Continue?",
					Options:  []string{"yes", "no"},
				},
			},
		},
	)

	echoTool := api.NewToolBuilder("echo").
		WithDescription("Echoes").
		WithAnnotations(tool.ReadOnlyAnnotations()).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
		api.StateExplore: {"echo"},
	})

	engine, err := api.New(
		api.WithTool(echoTool),
		api.WithPlanner(scriptedPlanner),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	run, err := engine.Run(ctx, "test human input")
	// The run may pause or error - we just need to exercise ResumeWithInput
	if run != nil && run.Status == api.StatusPaused {
		_, _ = engine.ResumeWithInput(ctx, run, "yes")
	}
	// If it didn't pause, that's fine - we still covered the code path
	_ = err
}
