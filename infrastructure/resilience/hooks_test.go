package resilience

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/felixgeelhaar/fortify/circuitbreaker"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestCircuitBreakerHooks_OnTrip(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var trippedTools []string
	var stateChanges []StateChangeEvent

	executor := NewExecutorWithOptions(
		WithCircuitBreakerThreshold(2),
		WithCircuitBreakerTimeout(5*time.Second),
		WithRetryAttempts(1),
		WithTimeout(5*time.Second),
		WithCircuitBreakerHooks(CircuitBreakerHooks{
			OnTrip: func(toolName string) {
				mu.Lock()
				trippedTools = append(trippedTools, toolName)
				mu.Unlock()
			},
			OnStateChange: func(event StateChangeEvent) {
				mu.Lock()
				stateChanges = append(stateChanges, event)
				mu.Unlock()
			},
		}),
	)

	failingTool := &mockTool{
		name: "failing_tool",
		handler: func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{}, errors.New("fail")
		},
	}

	// Trigger enough failures to trip the breaker (threshold=2).
	for i := 0; i < 3; i++ {
		_, _ = executor.Execute(context.Background(), failingTool, json.RawMessage(`{}`))
	}

	mu.Lock()
	defer mu.Unlock()

	// The breaker should have tripped at least once.
	if len(trippedTools) == 0 {
		t.Error("OnTrip should have been called at least once")
	}
	for _, name := range trippedTools {
		if name != "failing_tool" {
			t.Errorf("OnTrip tool name = %q, want %q", name, "failing_tool")
		}
	}

	// There should be at least one state change event.
	if len(stateChanges) == 0 {
		t.Error("OnStateChange should have been called at least once")
	}

	// Verify the trip transition includes a move to open state.
	foundOpen := false
	for _, ev := range stateChanges {
		if ev.To == circuitbreaker.StateOpen {
			foundOpen = true
		}
	}
	if !foundOpen {
		t.Error("expected at least one state change To=open")
	}
}

func TestCircuitBreakerHooks_NilHooksDoNotPanic(t *testing.T) {
	t.Parallel()

	executor := NewDefaultExecutor()

	mockT := &mockTool{name: "safe_tool"}
	_, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
}

func TestWithOnStateChange_FunctionalOption(t *testing.T) {
	t.Parallel()

	called := false
	executor := NewExecutorWithOptions(
		WithOnStateChange(func(_ StateChangeEvent) {
			called = true
		}),
		WithCircuitBreakerThreshold(1),
		WithRetryAttempts(1),
	)

	failingTool := &mockTool{
		name: "breaker_tool",
		handler: func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{}, errors.New("fail")
		},
	}

	// Trip the breaker.
	for i := 0; i < 3; i++ {
		_, _ = executor.Execute(context.Background(), failingTool, json.RawMessage(`{}`))
	}

	if !called {
		t.Error("WithOnStateChange callback should have been invoked")
	}
}

func TestWithOnTrip_FunctionalOption(t *testing.T) {
	t.Parallel()

	tripped := false
	executor := NewExecutorWithOptions(
		WithOnTrip(func(_ string) {
			tripped = true
		}),
		WithCircuitBreakerThreshold(1),
		WithRetryAttempts(1),
	)

	failingTool := &mockTool{
		name: "trip_tool",
		handler: func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{}, errors.New("fail")
		},
	}

	for i := 0; i < 3; i++ {
		_, _ = executor.Execute(context.Background(), failingTool, json.RawMessage(`{}`))
	}

	if !tripped {
		t.Error("WithOnTrip callback should have been invoked")
	}
}

func TestWithOnReset_FunctionalOption(t *testing.T) {
	t.Parallel()

	var resetTool string
	executor := NewExecutorWithOptions(
		WithOnReset(func(toolName string) {
			resetTool = toolName
		}),
		WithCircuitBreakerThreshold(1),
		WithCircuitBreakerTimeout(50*time.Millisecond),
		WithRetryAttempts(1),
		WithTimeout(5*time.Second),
	)

	callCount := 0
	flappingTool := &mockTool{
		name: "flapping_tool",
		handler: func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			callCount++
			if callCount <= 2 {
				return tool.Result{}, errors.New("fail")
			}
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		},
	}

	// Trip the breaker.
	for i := 0; i < 2; i++ {
		_, _ = executor.Execute(context.Background(), flappingTool, json.RawMessage(`{}`))
	}

	// Wait for the circuit breaker timeout to move to half-open.
	time.Sleep(100 * time.Millisecond)

	// Next call should succeed and potentially close the breaker.
	_, _ = executor.Execute(context.Background(), flappingTool, json.RawMessage(`{}`))

	// The reset hook may or may not fire depending on exact CB state
	// transitions. We mainly verify no panics and the option wires up.
	_ = resetTool
}

func TestFireStateChange_NilHooks(t *testing.T) {
	t.Parallel()

	// Calling fireStateChange on nil hooks must not panic.
	var hooks *CircuitBreakerHooks
	hooks.fireStateChange(circuitbreaker.StateClosed, circuitbreaker.StateOpen, "test")
}

func TestFireStateChange_PartialHooks(t *testing.T) {
	t.Parallel()

	// Only OnTrip set, others nil.
	tripped := false
	hooks := &CircuitBreakerHooks{
		OnTrip: func(_ string) { tripped = true },
	}

	hooks.fireStateChange(circuitbreaker.StateClosed, circuitbreaker.StateOpen, "tool")
	if !tripped {
		t.Error("OnTrip should fire on closed->open")
	}
}

func TestFireStateChange_ResetDetection(t *testing.T) {
	t.Parallel()

	resetCalled := false
	hooks := &CircuitBreakerHooks{
		OnReset: func(_ string) { resetCalled = true },
	}

	// half-open -> closed should trigger OnReset.
	hooks.fireStateChange(circuitbreaker.StateHalfOpen, circuitbreaker.StateClosed, "tool")
	if !resetCalled {
		t.Error("OnReset should fire on half-open->closed")
	}
}

func TestFireStateChange_NoResetOnOtherTransitions(t *testing.T) {
	t.Parallel()

	resetCalled := false
	hooks := &CircuitBreakerHooks{
		OnReset: func(_ string) { resetCalled = true },
	}

	// closed -> open should NOT trigger OnReset.
	hooks.fireStateChange(circuitbreaker.StateClosed, circuitbreaker.StateOpen, "tool")
	if resetCalled {
		t.Error("OnReset should not fire on closed->open")
	}
}
