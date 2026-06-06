package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/metrics"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// mockMetricsProvider is a mock implementation of telemetry.Metrics for testing.
type mockMetricsProvider struct {
	mu sync.Mutex

	toolExecutions       []toolExecutionRecord
	stateTransitions     []stateTransitionRecord
	budgetConsumptions   []budgetConsumptionRecord
	rateLimitHits        []string
	cacheHits            []string
	cacheMisses          []string
	errors               []errorRecord
	planningDurations    []durationRecord
	runDurations         []runDurationRecord
	activeRunsIncrement  int
	activeRunsDecrement  int
	circuitBreakerStates []circuitBreakerRecord
}

type toolExecutionRecord struct {
	toolName string
	state    string
	success  bool
	duration time.Duration
}

type stateTransitionRecord struct {
	fromState string
	toState   string
	runID     string
}

type budgetConsumptionRecord struct {
	budgetName string
	amount     int64
	remaining  int64
}

type errorRecord struct {
	errorType string
	details   map[string]string
}

type durationRecord struct {
	duration time.Duration
	state    string
}

type runDurationRecord struct {
	duration   time.Duration
	finalState string
	success    bool
}

type circuitBreakerRecord struct {
	toolName string
	isOpen   bool
}

func (m *mockMetricsProvider) RecordToolExecution(ctx context.Context, toolName string, state string, success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolExecutions = append(m.toolExecutions, toolExecutionRecord{toolName, state, success, duration})
}

func (m *mockMetricsProvider) RecordStateTransition(ctx context.Context, fromState, toState string, runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateTransitions = append(m.stateTransitions, stateTransitionRecord{fromState, toState, runID})
}

func (m *mockMetricsProvider) RecordBudgetConsumption(ctx context.Context, budgetName string, amount int64, remaining int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budgetConsumptions = append(m.budgetConsumptions, budgetConsumptionRecord{budgetName, amount, remaining})
}

func (m *mockMetricsProvider) RecordRateLimitHit(ctx context.Context, toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimitHits = append(m.rateLimitHits, toolName)
}

func (m *mockMetricsProvider) RecordCacheHit(ctx context.Context, toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheHits = append(m.cacheHits, toolName)
}

func (m *mockMetricsProvider) RecordCacheMiss(ctx context.Context, toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheMisses = append(m.cacheMisses, toolName)
}

func (m *mockMetricsProvider) RecordError(ctx context.Context, errorType string, details map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, errorRecord{errorType, details})
}

func (m *mockMetricsProvider) RecordPlanningDuration(ctx context.Context, duration time.Duration, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.planningDurations = append(m.planningDurations, durationRecord{duration, state})
}

func (m *mockMetricsProvider) RecordRunDuration(ctx context.Context, duration time.Duration, finalState string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runDurations = append(m.runDurations, runDurationRecord{duration, finalState, success})
}

func (m *mockMetricsProvider) IncrementActiveRuns(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeRunsIncrement++
}

func (m *mockMetricsProvider) DecrementActiveRuns(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeRunsDecrement++
}

func (m *mockMetricsProvider) RecordCircuitBreakerStateChange(ctx context.Context, toolName string, isOpen bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.circuitBreakerStates = append(m.circuitBreakerStates, circuitBreakerRecord{toolName, isOpen})
}

// Ensure mockMetricsProvider implements metrics.Metrics
var _ metrics.Metrics = (*mockMetricsProvider)(nil)

// createMockToolForMetrics creates a simple mock tool for metrics testing.
func createMockToolForMetrics(name string) tool.Tool {
	t, _ := tool.NewBuilder(name).
		WithDescription("Mock tool for testing").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"status": "ok"}`)}, nil
		}).
		Build()
	return t
}

func TestMetrics_RecordsSuccessfulExecution(t *testing.T) {
	provider := &mockMetricsProvider{}
	mw := Metrics(MetricsConfig{Provider: provider})

	testTool := createMockToolForMetrics("test_tool")
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateExplore,
		Tool:         testTool,
		Input:        json.RawMessage(`{}`),
	}

	// Create a handler that returns success
	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		return tool.Result{Output: json.RawMessage(`{"ok": true}`)}, nil
	})

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.toolExecutions) != 1 {
		t.Fatalf("expected 1 tool execution, got %d", len(provider.toolExecutions))
	}

	rec := provider.toolExecutions[0]
	if rec.toolName != "test_tool" {
		t.Errorf("toolName = %s, want test_tool", rec.toolName)
	}
	if rec.state != "explore" {
		t.Errorf("state = %s, want explore", rec.state)
	}
	if !rec.success {
		t.Error("success should be true")
	}
	if rec.duration < 0 {
		t.Error("duration should not be negative")
	}
}

func TestMetrics_RecordsFailedExecution(t *testing.T) {
	provider := &mockMetricsProvider{}
	mw := Metrics(MetricsConfig{Provider: provider})

	testTool := createMockToolForMetrics("failing_tool")
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-456",
		CurrentState: agent.StateAct,
		Tool:         testTool,
		Input:        json.RawMessage(`{}`),
	}

	// Create a handler that returns an error result
	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		return tool.NewErrorResult(errors.New("test error")), nil
	})

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.toolExecutions) != 1 {
		t.Fatalf("expected 1 tool execution, got %d", len(provider.toolExecutions))
	}

	rec := provider.toolExecutions[0]
	if rec.toolName != "failing_tool" {
		t.Errorf("toolName = %s, want failing_tool", rec.toolName)
	}
	if rec.state != "act" {
		t.Errorf("state = %s, want act", rec.state)
	}
	if rec.success {
		t.Error("success should be false")
	}
}

func TestMetrics_NilProviderUsesNoop(t *testing.T) {
	// Should not panic with nil provider
	mw := Metrics(MetricsConfig{Provider: nil})

	testTool := createMockToolForMetrics("test_tool")
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-789",
		CurrentState: agent.StateExplore,
		Tool:         testTool,
		Input:        json.RawMessage(`{}`),
	}

	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		return tool.Result{Output: json.RawMessage(`{}`)}, nil
	})

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMetricsWithCaching(t *testing.T) {
	provider := &mockMetricsProvider{}
	recorder := MetricsWithCaching(MetricsConfig{Provider: provider})

	ctx := context.Background()
	recorder.RecordHit(ctx, "cached_tool")
	recorder.RecordMiss(ctx, "uncached_tool")

	if len(provider.cacheHits) != 1 || provider.cacheHits[0] != "cached_tool" {
		t.Errorf("expected cache hit for cached_tool, got %v", provider.cacheHits)
	}
	if len(provider.cacheMisses) != 1 || provider.cacheMisses[0] != "uncached_tool" {
		t.Errorf("expected cache miss for uncached_tool, got %v", provider.cacheMisses)
	}
}

func TestMetricsRateLimitRecorder(t *testing.T) {
	provider := &mockMetricsProvider{}
	recorder := MetricsRateLimitRecorder(MetricsConfig{Provider: provider})

	ctx := context.Background()
	recorder.RecordLimitHit(ctx, "expensive_tool")
	recorder.RecordLimitHit(ctx, "expensive_tool")

	if len(provider.rateLimitHits) != 2 {
		t.Errorf("expected 2 rate limit hits, got %d", len(provider.rateLimitHits))
	}
}

func TestMetricsCircuitBreakerRecorder(t *testing.T) {
	provider := &mockMetricsProvider{}
	recorder := MetricsCircuitBreakerRecorder(MetricsConfig{Provider: provider})

	ctx := context.Background()
	recorder.RecordStateChange(ctx, "flaky_tool", true)
	recorder.RecordStateChange(ctx, "flaky_tool", false)

	if len(provider.circuitBreakerStates) != 2 {
		t.Fatalf("expected 2 state changes, got %d", len(provider.circuitBreakerStates))
	}
	if !provider.circuitBreakerStates[0].isOpen {
		t.Error("first state should be open")
	}
	if provider.circuitBreakerStates[1].isOpen {
		t.Error("second state should be closed")
	}
}

func TestMetricsWithCaching_NilProvider(t *testing.T) {
	// Should not panic with nil provider
	recorder := MetricsWithCaching(MetricsConfig{Provider: nil})

	ctx := context.Background()
	recorder.RecordHit(ctx, "tool")
	recorder.RecordMiss(ctx, "tool")
}

func TestMetricsRateLimitRecorder_NilProvider(t *testing.T) {
	// Should not panic with nil provider
	recorder := MetricsRateLimitRecorder(MetricsConfig{Provider: nil})

	ctx := context.Background()
	recorder.RecordLimitHit(ctx, "tool")
}

func TestMetricsCircuitBreakerRecorder_NilProvider(t *testing.T) {
	// Should not panic with nil provider
	recorder := MetricsCircuitBreakerRecorder(MetricsConfig{Provider: nil})

	ctx := context.Background()
	recorder.RecordStateChange(ctx, "tool", true)
}
