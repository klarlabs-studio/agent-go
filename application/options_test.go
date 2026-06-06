package application_test

import (
	"testing"

	"go.klarlabs.de/agent/application"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/infrastructure/planner"
	"go.klarlabs.de/agent/infrastructure/resilience"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

func TestWithRegistry(t *testing.T) {
	t.Parallel()

	registry := memory.NewToolRegistry()
	config := &application.EngineConfig{}

	opt := application.WithRegistry(registry)
	opt(config)

	if config.Registry != registry {
		t.Error("WithRegistry should set the registry")
	}
}

func TestWithPlanner(t *testing.T) {
	t.Parallel()

	p := planner.NewMockPlanner(agent.NewFinishDecision("done", nil))
	config := &application.EngineConfig{}

	opt := application.WithPlanner(p)
	opt(config)

	if config.Planner != p {
		t.Error("WithPlanner should set the planner")
	}
}

func TestWithExecutor(t *testing.T) {
	t.Parallel()

	executor := resilience.NewDefaultExecutor()
	config := &application.EngineConfig{}

	opt := application.WithExecutor(executor)
	opt(config)

	if config.Executor != executor {
		t.Error("WithExecutor should set the executor")
	}
}

func TestWithArtifactStore(t *testing.T) {
	t.Parallel()

	// Test that the option sets the artifact store to nil (no store configured)
	config := &application.EngineConfig{}

	opt := application.WithArtifactStore(nil)
	opt(config)

	if config.Artifacts != nil {
		t.Error("WithArtifactStore(nil) should set artifacts to nil")
	}
}

func TestWithEligibility(t *testing.T) {
	t.Parallel()

	eligibility := policy.NewToolEligibility()
	config := &application.EngineConfig{}

	opt := application.WithEligibility(eligibility)
	opt(config)

	if config.Eligibility != eligibility {
		t.Error("WithEligibility should set the eligibility")
	}
}

func TestWithTransitions(t *testing.T) {
	t.Parallel()

	transitions := policy.DefaultTransitions()
	config := &application.EngineConfig{}

	opt := application.WithTransitions(transitions)
	opt(config)

	if config.Transitions != transitions {
		t.Error("WithTransitions should set the transitions")
	}
}

func TestWithApprover(t *testing.T) {
	t.Parallel()

	approver := policy.NewAutoApprover("test")
	config := &application.EngineConfig{}

	opt := application.WithApprover(approver)
	opt(config)

	if config.Approver != approver {
		t.Error("WithApprover should set the approver")
	}
}

func TestWithBudgets(t *testing.T) {
	t.Parallel()

	limits := map[string]int{"calls": 100, "tokens": 10000}
	config := &application.EngineConfig{}

	opt := application.WithBudgets(limits)
	opt(config)

	if len(config.BudgetLimits) != 2 {
		t.Errorf("BudgetLimits length = %d, want 2", len(config.BudgetLimits))
	}
	if config.BudgetLimits["calls"] != 100 {
		t.Errorf("BudgetLimits[calls] = %d, want 100", config.BudgetLimits["calls"])
	}
}

func TestWithMaxSteps(t *testing.T) {
	t.Parallel()

	config := &application.EngineConfig{}

	opt := application.WithMaxSteps(50)
	opt(config)

	if config.MaxSteps != 50 {
		t.Errorf("MaxSteps = %d, want 50", config.MaxSteps)
	}
}

func TestWithMiddleware(t *testing.T) {
	t.Parallel()

	// We can't easily test the middleware setting without accessing internal state
	// but we can verify the option doesn't panic
	config := &application.EngineConfig{}

	opt := application.WithMiddleware(nil)
	opt(config)

	// Verify it was set to nil (which is a valid option)
	if config.Middleware != nil {
		t.Error("WithMiddleware(nil) should set middleware to nil")
	}
}

func TestNewEngineWithOptions(t *testing.T) {
	t.Parallel()

	t.Run("with minimal config", func(t *testing.T) {
		t.Parallel()

		registry := memory.NewToolRegistry()
		mockPlanner := planner.NewMockPlanner(agent.NewFinishDecision("done", nil))
		eligibility := policy.NewToolEligibility()
		transitions := policy.DefaultTransitions()

		engine, err := application.NewEngineWithOptions(
			application.WithRegistry(registry),
			application.WithPlanner(mockPlanner),
			application.WithEligibility(eligibility),
			application.WithTransitions(transitions),
			application.WithMaxSteps(10),
		)

		if err != nil {
			t.Fatalf("NewEngineWithOptions() error = %v", err)
		}
		if engine == nil {
			t.Error("Engine should not be nil")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()

		registry := memory.NewToolRegistry()
		mockPlanner := planner.NewMockPlanner(agent.NewFinishDecision("done", nil))
		eligibility := policy.NewToolEligibility()
		transitions := policy.DefaultTransitions()
		executor := resilience.NewDefaultExecutor()
		approver := policy.NewAutoApprover("test")

		engine, err := application.NewEngineWithOptions(
			application.WithRegistry(registry),
			application.WithPlanner(mockPlanner),
			application.WithEligibility(eligibility),
			application.WithTransitions(transitions),
			application.WithExecutor(executor),
			application.WithApprover(approver),
			application.WithBudgets(map[string]int{"calls": 100}),
			application.WithMaxSteps(50),
		)

		if err != nil {
			t.Fatalf("NewEngineWithOptions() error = %v", err)
		}
		if engine == nil {
			t.Error("Engine should not be nil")
		}
	})

	t.Run("without required config", func(t *testing.T) {
		t.Parallel()

		_, err := application.NewEngineWithOptions(
			application.WithMaxSteps(10),
		)

		if err == nil {
			t.Error("NewEngineWithOptions should fail without required config")
		}
	})
}
