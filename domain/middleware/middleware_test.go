package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// mockTool implements tool.Tool for testing
type mockTool struct {
	name string
}

func (m mockTool) Name() string                  { return m.name }
func (m mockTool) Description() string           { return "mock tool" }
func (m mockTool) Annotations() tool.Annotations { return tool.Annotations{} }
func (m mockTool) InputSchema() tool.Schema      { return tool.Schema{} }
func (m mockTool) OutputSchema() tool.Schema     { return tool.Schema{} }
func (m mockTool) Execute(context.Context, json.RawMessage) (tool.Result, error) {
	return tool.Result{}, nil
}

func TestChain(t *testing.T) {
	t.Parallel()

	t.Run("chains middleware in order", func(t *testing.T) {
		t.Parallel()

		var order []string

		mw1 := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
				order = append(order, "before-1")
				result, err := next(ctx, ec)
				order = append(order, "after-1")
				return result, err
			}
		}

		mw2 := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
				order = append(order, "before-2")
				result, err := next(ctx, ec)
				order = append(order, "after-2")
				return result, err
			}
		}

		mw3 := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
				order = append(order, "before-3")
				result, err := next(ctx, ec)
				order = append(order, "after-3")
				return result, err
			}
		}

		finalHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			order = append(order, "handler")
			return tool.Result{Output: json.RawMessage(`"done"`)}, nil
		}

		chain := middleware.Chain(mw1, mw2, mw3)
		handler := chain(finalHandler)

		ec := &middleware.ExecutionContext{
			RunID:        "run-1",
			CurrentState: agent.StateExplore,
			Tool:         mockTool{name: "test"},
			Input:        json.RawMessage(`{}`),
		}

		_, err := handler(context.Background(), ec)
		if err != nil {
			t.Fatalf("handler error = %v", err)
		}

		expected := []string{"before-1", "before-2", "before-3", "handler", "after-3", "after-2", "after-1"}
		if len(order) != len(expected) {
			t.Errorf("execution order length = %d, want %d", len(order), len(expected))
		}
		for i, v := range expected {
			if i < len(order) && order[i] != v {
				t.Errorf("execution order[%d] = %s, want %s", i, order[i], v)
			}
		}
	})

	t.Run("empty chain returns final handler directly", func(t *testing.T) {
		t.Parallel()

		finalHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`"direct"`)}, nil
		}

		chain := middleware.Chain()
		handler := chain(finalHandler)

		result, err := handler(context.Background(), &middleware.ExecutionContext{})
		if err != nil {
			t.Fatalf("handler error = %v", err)
		}
		if string(result.Output) != `"direct"` {
			t.Errorf("result = %s, want \"direct\"", string(result.Output))
		}
	})

	t.Run("middleware can short-circuit", func(t *testing.T) {
		t.Parallel()

		shortCircuit := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`"blocked"`)}, nil
			}
		}

		called := false
		finalHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			called = true
			return tool.Result{}, nil
		}

		chain := middleware.Chain(shortCircuit)
		handler := chain(finalHandler)

		result, err := handler(context.Background(), &middleware.ExecutionContext{})
		if err != nil {
			t.Fatalf("handler error = %v", err)
		}
		if called {
			t.Error("final handler should not have been called")
		}
		if string(result.Output) != `"blocked"` {
			t.Errorf("result = %s, want \"blocked\"", string(result.Output))
		}
	})

	t.Run("middleware can modify execution context", func(t *testing.T) {
		t.Parallel()

		modifier := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
				ec.Reason = "modified by middleware"
				return next(ctx, ec)
			}
		}

		var capturedReason string
		finalHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			capturedReason = ec.Reason
			return tool.Result{}, nil
		}

		chain := middleware.Chain(modifier)
		handler := chain(finalHandler)

		ec := &middleware.ExecutionContext{Reason: "original"}
		_, _ = handler(context.Background(), ec)

		if capturedReason != "modified by middleware" {
			t.Errorf("captured reason = %s, want 'modified by middleware'", capturedReason)
		}
	})

	t.Run("middleware can transform errors", func(t *testing.T) {
		t.Parallel()

		transformer := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
				_, err := next(ctx, ec)
				if err != nil {
					return tool.Result{}, errors.New("transformed: " + err.Error())
				}
				return tool.Result{}, nil
			}
		}

		finalHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{}, errors.New("original error")
		}

		chain := middleware.Chain(transformer)
		handler := chain(finalHandler)

		_, err := handler(context.Background(), &middleware.ExecutionContext{})
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "transformed: original error" {
			t.Errorf("error = %s, want 'transformed: original error'", err.Error())
		}
	})
}

func TestNoop(t *testing.T) {
	t.Parallel()

	t.Run("passes through unchanged", func(t *testing.T) {
		t.Parallel()

		finalHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`"passed"`)}, nil
		}

		noop := middleware.Noop()
		handler := noop(finalHandler)

		result, err := handler(context.Background(), &middleware.ExecutionContext{})
		if err != nil {
			t.Fatalf("handler error = %v", err)
		}
		if string(result.Output) != `"passed"` {
			t.Errorf("result = %s, want \"passed\"", string(result.Output))
		}
	})
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	registry := middleware.NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if registry.Len() != 0 {
		t.Errorf("NewRegistry() Len() = %d, want 0", registry.Len())
	}
}

func TestRegistry_Use(t *testing.T) {
	t.Parallel()

	t.Run("adds middleware", func(t *testing.T) {
		t.Parallel()

		registry := middleware.NewRegistry()
		mw := middleware.Noop()

		result := registry.Use(mw)

		if result != registry {
			t.Error("Use() should return the registry for chaining")
		}
		if registry.Len() != 1 {
			t.Errorf("Use() Len() = %d, want 1", registry.Len())
		}
	})

	t.Run("supports method chaining", func(t *testing.T) {
		t.Parallel()

		registry := middleware.NewRegistry()

		registry.
			Use(middleware.Noop()).
			Use(middleware.Noop()).
			Use(middleware.Noop())

		if registry.Len() != 3 {
			t.Errorf("chained Use() Len() = %d, want 3", registry.Len())
		}
	})
}

func TestRegistry_UseMany(t *testing.T) {
	t.Parallel()

	t.Run("adds multiple middleware", func(t *testing.T) {
		t.Parallel()

		registry := middleware.NewRegistry()
		mw1 := middleware.Noop()
		mw2 := middleware.Noop()
		mw3 := middleware.Noop()

		result := registry.UseMany(mw1, mw2, mw3)

		if result != registry {
			t.Error("UseMany() should return the registry for chaining")
		}
		if registry.Len() != 3 {
			t.Errorf("UseMany() Len() = %d, want 3", registry.Len())
		}
	})

	t.Run("handles empty slice", func(t *testing.T) {
		t.Parallel()

		registry := middleware.NewRegistry()
		registry.UseMany()

		if registry.Len() != 0 {
			t.Errorf("UseMany() with no args Len() = %d, want 0", registry.Len())
		}
	})
}

func TestRegistry_Chain(t *testing.T) {
	t.Parallel()

	t.Run("returns noop for empty registry", func(t *testing.T) {
		t.Parallel()

		registry := middleware.NewRegistry()
		chain := registry.Chain()

		finalHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`"handled"`)}, nil
		}

		handler := chain(finalHandler)
		result, err := handler(context.Background(), &middleware.ExecutionContext{})
		if err != nil {
			t.Fatalf("handler error = %v", err)
		}
		if string(result.Output) != `"handled"` {
			t.Errorf("result = %s, want \"handled\"", string(result.Output))
		}
	})

	t.Run("returns chained middleware", func(t *testing.T) {
		t.Parallel()

		var order []string

		mw1 := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
				order = append(order, "mw1")
				return next(ctx, ec)
			}
		}

		mw2 := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
				order = append(order, "mw2")
				return next(ctx, ec)
			}
		}

		registry := middleware.NewRegistry()
		registry.UseMany(mw1, mw2)

		chain := registry.Chain()

		finalHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			order = append(order, "handler")
			return tool.Result{}, nil
		}

		handler := chain(finalHandler)
		_, _ = handler(context.Background(), &middleware.ExecutionContext{})

		if len(order) != 3 {
			t.Errorf("execution order length = %d, want 3", len(order))
		}
		if order[0] != "mw1" || order[1] != "mw2" || order[2] != "handler" {
			t.Errorf("execution order = %v, want [mw1 mw2 handler]", order)
		}
	})
}

func TestRegistry_Len(t *testing.T) {
	t.Parallel()

	registry := middleware.NewRegistry()

	if registry.Len() != 0 {
		t.Errorf("empty registry Len() = %d, want 0", registry.Len())
	}

	registry.Use(middleware.Noop())
	if registry.Len() != 1 {
		t.Errorf("registry with 1 middleware Len() = %d, want 1", registry.Len())
	}

	registry.UseMany(middleware.Noop(), middleware.Noop())
	if registry.Len() != 3 {
		t.Errorf("registry with 3 middleware Len() = %d, want 3", registry.Len())
	}
}

func TestRegistry_Clone(t *testing.T) {
	t.Parallel()

	t.Run("creates independent copy", func(t *testing.T) {
		t.Parallel()

		original := middleware.NewRegistry()
		original.Use(middleware.Noop())
		original.Use(middleware.Noop())

		clone := original.Clone()

		if clone.Len() != original.Len() {
			t.Errorf("clone Len() = %d, want %d", clone.Len(), original.Len())
		}

		// Modify original
		original.Use(middleware.Noop())

		if clone.Len() == original.Len() {
			t.Error("clone should be independent of original")
		}
		if clone.Len() != 2 {
			t.Errorf("clone Len() after original modification = %d, want 2", clone.Len())
		}
	})

	t.Run("clones empty registry", func(t *testing.T) {
		t.Parallel()

		original := middleware.NewRegistry()
		clone := original.Clone()

		if clone.Len() != 0 {
			t.Errorf("clone of empty registry Len() = %d, want 0", clone.Len())
		}
	})
}

func TestExecutionContext(t *testing.T) {
	t.Parallel()

	t.Run("holds all execution data", func(t *testing.T) {
		t.Parallel()

		ec := &middleware.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockTool{name: "read_file"},
			Input:        json.RawMessage(`{"path":"/test"}`),
			Reason:       "gather information",
			Vars:         map[string]any{"key": "value"},
		}

		if ec.RunID != "run-123" {
			t.Errorf("RunID = %s, want run-123", ec.RunID)
		}
		if ec.CurrentState != agent.StateExplore {
			t.Errorf("CurrentState = %s, want explore", ec.CurrentState)
		}
		if ec.Tool.Name() != "read_file" {
			t.Errorf("Tool.Name() = %s, want read_file", ec.Tool.Name())
		}
		if ec.Reason != "gather information" {
			t.Errorf("Reason = %s, want 'gather information'", ec.Reason)
		}
		if ec.Vars["key"] != "value" {
			t.Errorf("Vars[key] = %v, want value", ec.Vars["key"])
		}
	})
}
