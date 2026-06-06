package resilience

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/tool"
)

// mockTool implements tool.Tool for testing.
type mockTool struct {
	name        string
	annotations tool.Annotations
	handler     func(context.Context, json.RawMessage) (tool.Result, error)
}

func (m *mockTool) Name() string                  { return m.name }
func (m *mockTool) Description() string           { return "Mock tool" }
func (m *mockTool) InputSchema() tool.Schema      { return tool.Schema{} }
func (m *mockTool) OutputSchema() tool.Schema     { return tool.Schema{} }
func (m *mockTool) Annotations() tool.Annotations { return m.annotations }
func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if m.handler != nil {
		return m.handler(ctx, input)
	}
	return tool.Result{Output: json.RawMessage(`{"success": true}`)}, nil
}

func TestDefaultExecutorConfig(t *testing.T) {
	config := DefaultExecutorConfig()

	if config.MaxConcurrent != 10 {
		t.Errorf("MaxConcurrent = %d, want 10", config.MaxConcurrent)
	}
	if config.CircuitBreakerThreshold != 5 {
		t.Errorf("CircuitBreakerThreshold = %d, want 5", config.CircuitBreakerThreshold)
	}
	if config.RetryMaxAttempts != 3 {
		t.Errorf("RetryMaxAttempts = %d, want 3", config.RetryMaxAttempts)
	}
	if config.DefaultTimeout != 30*time.Second {
		t.Errorf("DefaultTimeout = %v, want 30s", config.DefaultTimeout)
	}
}

func TestNewExecutor(t *testing.T) {
	executor := NewExecutor(DefaultExecutorConfig())
	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}
}

func TestNewDefaultExecutor(t *testing.T) {
	executor := NewDefaultExecutor()
	if executor == nil {
		t.Fatal("NewDefaultExecutor() returned nil")
	}
}

func TestExecutor_Execute_Success(t *testing.T) {
	executor := NewDefaultExecutor()
	mockT := &mockTool{
		name: "test_tool",
		annotations: tool.Annotations{
			Idempotent: true,
		},
	}

	result, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if string(result.Output) != `{"success": true}` {
		t.Errorf("Execute() output = %s, want {\"success\": true}", result.Output)
	}
	if result.Duration == 0 {
		t.Error("Execute() should set Duration")
	}
}

func TestExecutor_Execute_Failure(t *testing.T) {
	executor := NewDefaultExecutor()
	expectedErr := errors.New("tool error")
	mockT := &mockTool{
		name: "failing_tool",
		handler: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, expectedErr
		},
	}

	_, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err == nil {
		t.Error("Execute() should return error")
	}
}

func TestExecutor_Execute_ContextCancellation(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		MaxConcurrent:           10,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
		RetryMaxAttempts:        1,
		RetryInitialDelay:       10 * time.Millisecond,
		DefaultTimeout:          5 * time.Second,
	})

	mockT := &mockTool{
		name: "slow_tool",
		handler: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			select {
			case <-ctx.Done():
				return tool.Result{}, ctx.Err()
			case <-time.After(10 * time.Second):
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := executor.Execute(ctx, mockT, json.RawMessage(`{}`))
	if err == nil {
		t.Error("Execute() should return error on context cancellation")
	}
}

func TestExecutor_ExecuteWithTimeout(t *testing.T) {
	executor := NewDefaultExecutor()
	mockT := &mockTool{
		name: "quick_tool",
	}

	result, err := executor.ExecuteWithTimeout(
		context.Background(),
		mockT,
		json.RawMessage(`{}`),
		5*time.Second,
	)
	if err != nil {
		t.Errorf("ExecuteWithTimeout() error = %v, want nil", err)
	}
	if result.Output == nil {
		t.Error("ExecuteWithTimeout() should return output")
	}
}

func TestExecutor_ExecuteSimple(t *testing.T) {
	executor := NewDefaultExecutor()
	mockT := &mockTool{name: "simple_tool"}

	result, err := executor.ExecuteSimple(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("ExecuteSimple() error = %v, want nil", err)
	}
	if result.Duration == 0 {
		t.Error("ExecuteSimple() should set Duration")
	}
}

func TestExecutor_CircuitBreakerState(t *testing.T) {
	executor := NewDefaultExecutor()
	state := executor.CircuitBreakerState()
	// Initial state should be closed
	if state.String() != "closed" {
		t.Errorf("Initial CircuitBreakerState() = %v, want closed", state)
	}
}

func TestExecutor_NegativeConfig(t *testing.T) {
	// Test that negative values are handled gracefully
	executor := NewExecutor(ExecutorConfig{
		MaxConcurrent:           -1,
		CircuitBreakerThreshold: -1,
		CircuitBreakerTimeout:   30 * time.Second,
		RetryMaxAttempts:        3,
		RetryInitialDelay:       100 * time.Millisecond,
		DefaultTimeout:          30 * time.Second,
	})

	if executor == nil {
		t.Fatal("NewExecutor() with negative values returned nil")
	}

	// Should still work
	mockT := &mockTool{name: "test"}
	_, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("Execute() with negative config error = %v", err)
	}
}
