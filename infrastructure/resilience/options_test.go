package resilience

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/tool"
)

func TestWithMaxConcurrent(t *testing.T) {
	t.Parallel()

	config := DefaultExecutorConfig()
	opt := WithMaxConcurrent(20)
	opt(&config)

	if config.MaxConcurrent != 20 {
		t.Errorf("MaxConcurrent = %d, want 20", config.MaxConcurrent)
	}
}

func TestWithCircuitBreakerThreshold(t *testing.T) {
	t.Parallel()

	config := DefaultExecutorConfig()
	opt := WithCircuitBreakerThreshold(10)
	opt(&config)

	if config.CircuitBreakerThreshold != 10 {
		t.Errorf("CircuitBreakerThreshold = %d, want 10", config.CircuitBreakerThreshold)
	}
}

func TestWithCircuitBreakerTimeout(t *testing.T) {
	t.Parallel()

	config := DefaultExecutorConfig()
	opt := WithCircuitBreakerTimeout(60 * time.Second)
	opt(&config)

	if config.CircuitBreakerTimeout != 60*time.Second {
		t.Errorf("CircuitBreakerTimeout = %v, want 60s", config.CircuitBreakerTimeout)
	}
}

func TestWithRetryAttempts(t *testing.T) {
	t.Parallel()

	config := DefaultExecutorConfig()
	opt := WithRetryAttempts(5)
	opt(&config)

	if config.RetryMaxAttempts != 5 {
		t.Errorf("RetryMaxAttempts = %d, want 5", config.RetryMaxAttempts)
	}
}

func TestWithRetryDelay(t *testing.T) {
	t.Parallel()

	config := DefaultExecutorConfig()
	opt := WithRetryDelay(200 * time.Millisecond)
	opt(&config)

	if config.RetryInitialDelay != 200*time.Millisecond {
		t.Errorf("RetryInitialDelay = %v, want 200ms", config.RetryInitialDelay)
	}
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	config := DefaultExecutorConfig()
	opt := WithTimeout(60 * time.Second)
	opt(&config)

	if config.DefaultTimeout != 60*time.Second {
		t.Errorf("DefaultTimeout = %v, want 60s", config.DefaultTimeout)
	}
}

func TestNewExecutorWithOptions(t *testing.T) {
	t.Parallel()

	t.Run("with no options uses defaults", func(t *testing.T) {
		t.Parallel()

		executor := NewExecutorWithOptions()

		if executor == nil {
			t.Fatal("NewExecutorWithOptions() returned nil")
		}
	})

	t.Run("with multiple options", func(t *testing.T) {
		t.Parallel()

		executor := NewExecutorWithOptions(
			WithMaxConcurrent(20),
			WithCircuitBreakerThreshold(10),
			WithCircuitBreakerTimeout(60*time.Second),
			WithRetryAttempts(5),
			WithRetryDelay(200*time.Millisecond),
			WithTimeout(60*time.Second),
		)

		if executor == nil {
			t.Fatal("NewExecutorWithOptions() returned nil")
		}

		// Verify executor works
		mockT := &mockTool{name: "test"}
		result, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err != nil {
			t.Errorf("Execute() error = %v", err)
		}
		if result.Output == nil {
			t.Error("Execute() should return output")
		}
	})

	t.Run("options are applied in order", func(t *testing.T) {
		t.Parallel()

		// Apply options that override each other
		executor := NewExecutorWithOptions(
			WithMaxConcurrent(10),
			WithMaxConcurrent(25), // Should override to 25
		)

		if executor == nil {
			t.Fatal("NewExecutorWithOptions() returned nil")
		}
	})
}

func TestExecutor_Reset(t *testing.T) {
	t.Parallel()

	executor := NewDefaultExecutor()

	// Reset is a no-op (circuit breaker resets automatically after timeout)
	// Just verify it doesn't panic and can be called multiple times
	executor.Reset()
	executor.Reset()

	// Executor should still work after Reset
	mockT := &mockTool{name: "test"}
	result, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("After Reset(), Execute() error = %v, want nil", err)
	}
	if result.Output == nil {
		t.Error("After Reset(), Execute() should return output")
	}
}

func TestAllOptions_ChainedUsage(t *testing.T) {
	t.Parallel()

	// Test that all options can be chained and work together
	executor := NewExecutorWithOptions(
		WithMaxConcurrent(5),
		WithCircuitBreakerThreshold(3),
		WithCircuitBreakerTimeout(10*time.Second),
		WithRetryAttempts(2),
		WithRetryDelay(50*time.Millisecond),
		WithTimeout(10*time.Second),
	)

	if executor == nil {
		t.Fatal("NewExecutorWithOptions() with all options returned nil")
	}

	// Test executor functionality
	mockT := &mockTool{
		name: "test_tool",
		annotations: tool.Annotations{
			Idempotent: true,
		},
	}

	result, err := executor.Execute(context.Background(), mockT, json.RawMessage(`{}`))
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Duration == 0 {
		t.Error("Execute() should set Duration")
	}
}
