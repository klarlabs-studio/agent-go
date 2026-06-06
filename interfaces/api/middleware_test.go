package api_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/ledger"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewMiddlewareRegistry(t *testing.T) {
	t.Parallel()

	t.Run("creates empty registry", func(t *testing.T) {
		t.Parallel()

		registry := api.NewMiddlewareRegistry()

		if registry == nil {
			t.Fatal("NewMiddlewareRegistry() returned nil")
		}
		if registry.Len() != 0 {
			t.Errorf("Len() = %d, want 0", registry.Len())
		}
	})

	t.Run("can add middleware to registry", func(t *testing.T) {
		t.Parallel()

		registry := api.NewMiddlewareRegistry()
		registry.Use(api.NoopMiddleware())

		if registry.Len() != 1 {
			t.Errorf("Len() = %d, want 1", registry.Len())
		}
	})
}

func TestNewMemoryCache(t *testing.T) {
	t.Parallel()

	t.Run("creates cache with max entries", func(t *testing.T) {
		t.Parallel()

		cache := api.NewMemoryCache(100)

		if cache == nil {
			t.Fatal("NewMemoryCache() returned nil")
		}
	})
}

func TestNewLegacyMiddlewareCache(t *testing.T) {
	t.Parallel()

	t.Run("creates legacy cache", func(t *testing.T) {
		t.Parallel()

		cache := api.NewLegacyMiddlewareCache(50)

		if cache == nil {
			t.Fatal("NewLegacyMiddlewareCache() returned nil")
		}
	})
}

func TestChainMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("chains multiple middleware", func(t *testing.T) {
		t.Parallel()

		chain := api.ChainMiddleware(
			api.NoopMiddleware(),
			api.NoopMiddleware(),
			api.NoopMiddleware(),
		)

		if chain == nil {
			t.Fatal("ChainMiddleware() returned nil")
		}
	})

	t.Run("chained middleware executes handler", func(t *testing.T) {
		t.Parallel()

		var callOrder []string

		first := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
				callOrder = append(callOrder, "first")
				return next(ctx, execCtx)
			}
		}

		second := func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
				callOrder = append(callOrder, "second")
				return next(ctx, execCtx)
			}
		}

		chain := api.ChainMiddleware(first, second)

		handler := func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			callOrder = append(callOrder, "handler")
			return tool.Result{}, nil
		}

		wrapped := chain(handler)
		_, _ = wrapped(context.Background(), &middleware.ExecutionContext{})

		if len(callOrder) != 3 {
			t.Errorf("call order len = %d, want 3", len(callOrder))
		}
		if callOrder[0] != "first" {
			t.Errorf("first call = %s, want first", callOrder[0])
		}
		if callOrder[1] != "second" {
			t.Errorf("second call = %s, want second", callOrder[1])
		}
		if callOrder[2] != "handler" {
			t.Errorf("third call = %s, want handler", callOrder[2])
		}
	})
}

func TestNoopMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("passes through to handler", func(t *testing.T) {
		t.Parallel()

		noop := api.NoopMiddleware()

		called := false
		handler := func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			called = true
			return tool.Result{Output: json.RawMessage(`{"test":"value"}`)}, nil
		}

		wrapped := noop(handler)
		result, err := wrapped(context.Background(), &middleware.ExecutionContext{})

		if err != nil {
			t.Errorf("Noop middleware returned error: %v", err)
		}
		if !called {
			t.Error("Handler was not called")
		}
		if string(result.Output) != `{"test":"value"}` {
			t.Errorf("Output = %s", result.Output)
		}
	})
}

func TestEligibilityMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates eligibility middleware", func(t *testing.T) {
		t.Parallel()

		eligibility := api.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		mw := api.EligibilityMiddleware(eligibility)

		if mw == nil {
			t.Fatal("EligibilityMiddleware() returned nil")
		}
	})
}

func TestApprovalMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates approval middleware", func(t *testing.T) {
		t.Parallel()

		approver := api.NewAutoApprover("test-approver")

		mw := api.ApprovalMiddleware(approver)

		if mw == nil {
			t.Fatal("ApprovalMiddleware() returned nil")
		}
	})
}

func TestBudgetMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates budget middleware", func(t *testing.T) {
		t.Parallel()

		budget := policy.NewBudget(map[string]int{"tool_calls": 100})

		mw := api.BudgetMiddleware(budget, "tool_calls", 1)

		if mw == nil {
			t.Fatal("BudgetMiddleware() returned nil")
		}
	})
}

func TestBudgetFromContextMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates budget from context middleware", func(t *testing.T) {
		t.Parallel()

		mw := api.BudgetFromContextMiddleware("tool_calls", 1)

		if mw == nil {
			t.Fatal("BudgetFromContextMiddleware() returned nil")
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates logging middleware with nil config", func(t *testing.T) {
		t.Parallel()

		mw := api.LoggingMiddleware(nil)

		if mw == nil {
			t.Fatal("LoggingMiddleware(nil) returned nil")
		}
	})

	t.Run("creates logging middleware with config", func(t *testing.T) {
		t.Parallel()

		cfg := &api.LoggingMiddlewareConfig{
			LogInput:  true,
			LogOutput: true,
		}

		mw := api.LoggingMiddleware(cfg)

		if mw == nil {
			t.Fatal("LoggingMiddleware(cfg) returned nil")
		}
	})
}

func TestCachingMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates caching middleware with memory cache", func(t *testing.T) {
		t.Parallel()

		cache := api.NewMemoryCache(100)

		mw := api.CachingMiddleware(cache)

		if mw == nil {
			t.Fatal("CachingMiddleware() returned nil")
		}
	})

	t.Run("creates caching middleware with TTL option", func(t *testing.T) {
		t.Parallel()

		cache := api.NewMemoryCache(100)

		mw := api.CachingMiddleware(cache, api.WithCacheTTL(5*time.Minute))

		if mw == nil {
			t.Fatal("CachingMiddleware() with TTL returned nil")
		}
	})
}

func TestWithCacheTTL(t *testing.T) {
	t.Parallel()

	t.Run("creates cache TTL option", func(t *testing.T) {
		t.Parallel()

		opt := api.WithCacheTTL(10 * time.Second)

		if opt == nil {
			t.Fatal("WithCacheTTL() returned nil")
		}
	})
}

func TestLegacyCachingMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates legacy caching middleware", func(t *testing.T) {
		t.Parallel()

		cache := api.NewLegacyMiddlewareCache(100)

		mw := api.LegacyCachingMiddleware(cache)

		if mw == nil {
			t.Fatal("LegacyCachingMiddleware() returned nil")
		}
	})
}

func TestLedgerRecordingMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("creates ledger recording middleware", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("test-run-id")

		mw := api.LedgerRecordingMiddleware(l)

		if mw == nil {
			t.Fatal("LedgerRecordingMiddleware() returned nil")
		}
	})
}
