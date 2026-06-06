package resilience

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/tool"
)

func TestRateLimiter_GlobalAllows(t *testing.T) {
	t.Parallel()

	executor := NewExecutorWithOptions(
		WithGlobalRateLimit(100, 100),
	)

	mockT := &mockTool{name: "test_tool"}

	// Should allow requests within limit.
	for i := 0; i < 10; i++ {
		_, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}
}

func TestRateLimiter_GlobalDenies(t *testing.T) {
	t.Parallel()

	executor := NewExecutorWithOptions(
		WithGlobalRateLimit(1, 1),
	)

	mockT := &mockTool{name: "test_tool"}

	// First should succeed.
	_, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("first request should succeed: %v", err)
	}

	// Second immediate request should be rate limited.
	_, err = executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got: %v", err)
	}
}

func TestRateLimiter_PerToolRates(t *testing.T) {
	t.Parallel()

	executor := NewExecutorWithOptions(
		WithRateLimiter(RateLimiterConfig{
			GlobalRate:  1000,
			GlobalBurst: 1000,
			PerToolRates: map[string]RateConfig{
				"slow_tool": {Rate: 1, Burst: 1},
				"fast_tool": {Rate: 1000, Burst: 1000},
			},
		}),
	)

	slowTool := &mockTool{name: "slow_tool"}
	fastTool := &mockTool{name: "fast_tool"}

	// Fast tool should allow many.
	for i := 0; i < 10; i++ {
		_, err := executor.Execute(context.Background(), fastTool, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("fast_tool request %d should succeed: %v", i, err)
		}
	}

	// Slow tool should allow one, then deny.
	_, err := executor.Execute(context.Background(), slowTool, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("slow_tool first request should succeed: %v", err)
	}

	_, err = executor.Execute(context.Background(), slowTool, json.RawMessage(`{}`))
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("slow_tool second request: expected ErrRateLimited, got: %v", err)
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	t.Parallel()

	// No rate limiter configured -- everything should pass.
	executor := NewDefaultExecutor()

	mockT := &mockTool{name: "test_tool"}
	for i := 0; i < 20; i++ {
		_, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}
}

func TestRateLimiter_NilConfig(t *testing.T) {
	t.Parallel()

	// rateLimiterSet constructed from nil should allow everything.
	s := newRateLimiterSet(nil)
	if s != nil {
		t.Fatal("newRateLimiterSet(nil) should return nil")
	}

	// allow on nil set should return true.
	var nilSet *rateLimiterSet
	if !nilSet.allow(context.Background(), "any") {
		t.Error("nil rateLimiterSet should allow all")
	}
}

func TestRateLimiter_ZeroRates(t *testing.T) {
	t.Parallel()

	// GlobalRate=0 and no per-tool -> disabled.
	s := newRateLimiterSet(&RateLimiterConfig{GlobalRate: 0})
	if s != nil {
		t.Error("zero global rate with no per-tool should return nil")
	}
}

func TestPerToolBulkhead_Isolation(t *testing.T) {
	t.Parallel()

	executor := NewExecutorWithOptions(
		WithMaxConcurrent(10),
		WithPerToolBulkhead("isolated_tool", 2),
	)

	mockT := &mockTool{name: "isolated_tool"}
	sharedT := &mockTool{name: "shared_tool"}

	// Both should work with normal load.
	_, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("isolated_tool should succeed: %v", err)
	}

	_, err = executor.Execute(context.Background(), sharedT, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("shared_tool should succeed: %v", err)
	}
}

func TestPerToolBulkheads_Option(t *testing.T) {
	t.Parallel()

	executor := NewExecutorWithOptions(
		WithPerToolBulkheads(map[string]int{
			"tool_a": 5,
			"tool_b": 3,
		}),
	)

	if executor.perToolBulkheads == nil {
		t.Fatal("perToolBulkheads should be initialized")
	}
	if len(executor.perToolBulkheads) != 2 {
		t.Errorf("expected 2 per-tool bulkheads, got %d", len(executor.perToolBulkheads))
	}
}

func TestBulkheadMetrics_SharedBulkhead(t *testing.T) {
	t.Parallel()

	executor := NewDefaultExecutor()

	// Initially, metrics should be zero.
	m := executor.BulkheadMetricsFor("")
	if m.Active != 0 {
		t.Errorf("initial active = %d, want 0", m.Active)
	}
	if m.Rejected != 0 {
		t.Errorf("initial rejected = %d, want 0", m.Rejected)
	}

	// After a successful execution, metrics should still be zero for active
	// (since it completes), rejected stays zero.
	mockT := &mockTool{name: "test"}
	_, _ = executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))

	m = executor.BulkheadMetricsFor("")
	if m.Active != 0 {
		t.Errorf("after execution active = %d, want 0", m.Active)
	}
}

func TestBulkheadMetrics_PerToolBulkhead(t *testing.T) {
	t.Parallel()

	executor := NewExecutorWithOptions(
		WithPerToolBulkhead("metered_tool", 5),
	)

	mockT := &mockTool{name: "metered_tool"}
	_, _ = executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))

	m := executor.BulkheadMetricsFor("metered_tool")
	if m.Active != 0 {
		t.Errorf("after execution active = %d, want 0", m.Active)
	}
}

func TestBulkheadMetrics_ActiveDuringExecution(t *testing.T) {
	t.Parallel()

	executor := NewExecutorWithOptions(
		WithPerToolBulkhead("blocking_tool", 5),
	)

	activeSeen := make(chan int64, 1)

	blockingTool := &mockTool{
		name: "blocking_tool",
		handler: func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			// Read metrics while inside the handler.
			m := executor.BulkheadMetricsFor("blocking_tool")
			activeSeen <- m.Active
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		},
	}

	_, _ = executor.Execute(context.Background(), blockingTool, json.RawMessage(`{}`))

	select {
	case active := <-activeSeen:
		if active != 1 {
			t.Errorf("active during execution = %d, want 1", active)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for active metric")
	}
}

func TestExecutor_BackwardCompatibility_Execute(t *testing.T) {
	t.Parallel()

	// Verify that the original NewExecutor with DefaultExecutorConfig still
	// works exactly as before.
	executor := NewExecutor(DefaultExecutorConfig())

	mockT := &mockTool{
		name:        "compat_tool",
		annotations: tool.Annotations{Idempotent: true},
	}

	result, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if string(result.Output) != `{"success": true}` {
		t.Errorf("Execute() output = %s", result.Output)
	}
	if result.Duration == 0 {
		t.Error("Execute() should set Duration")
	}
}

func TestExecutor_BackwardCompatibility_ExecuteSimple(t *testing.T) {
	t.Parallel()

	executor := NewDefaultExecutor()
	mockT := &mockTool{name: "simple"}

	result, err := executor.ExecuteSimple(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("ExecuteSimple() error = %v, want nil", err)
	}
	if result.Duration == 0 {
		t.Error("ExecuteSimple() should set Duration")
	}
}

func TestExecutor_BackwardCompatibility_ExecuteWithTimeout(t *testing.T) {
	t.Parallel()

	executor := NewDefaultExecutor()
	mockT := &mockTool{name: "timeout_test"}

	result, err := executor.ExecuteWithTimeout(
		context.Background(), mockT, json.RawMessage(`{}`), 5*time.Second,
	)
	if err != nil {
		t.Errorf("ExecuteWithTimeout() error = %v, want nil", err)
	}
	if result.Output == nil {
		t.Error("ExecuteWithTimeout() should return output")
	}
}

func TestExecutor_BackwardCompatibility_CircuitBreakerState(t *testing.T) {
	t.Parallel()

	executor := NewDefaultExecutor()
	state := executor.CircuitBreakerState()
	if state.String() != "closed" {
		t.Errorf("initial CircuitBreakerState() = %v, want closed", state)
	}
}
