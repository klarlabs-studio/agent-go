package resilience

import (
	"time"

	"go.klarlabs.de/agent/domain/clock"
)

// Option configures the executor.
type Option func(*ExecutorConfig)

// WithClock sets the clock used for tool-duration accounting. Inject a fixed
// clock to make tool.succeeded/tool.failed payload durations deterministic.
func WithClock(c clock.Clock) Option {
	return func(cfg *ExecutorConfig) {
		cfg.Clock = c
	}
}

// WithMaxConcurrent sets the maximum concurrent executions.
func WithMaxConcurrent(n int) Option {
	return func(c *ExecutorConfig) {
		c.MaxConcurrent = n
	}
}

// WithCircuitBreakerThreshold sets the failure threshold for circuit breaker.
func WithCircuitBreakerThreshold(n int) Option {
	return func(c *ExecutorConfig) {
		c.CircuitBreakerThreshold = n
	}
}

// WithCircuitBreakerTimeout sets the circuit breaker open duration.
func WithCircuitBreakerTimeout(d time.Duration) Option {
	return func(c *ExecutorConfig) {
		c.CircuitBreakerTimeout = d
	}
}

// WithRetryAttempts sets the maximum retry attempts.
func WithRetryAttempts(n int) Option {
	return func(c *ExecutorConfig) {
		c.RetryMaxAttempts = n
	}
}

// WithRetryDelay sets the initial retry delay.
func WithRetryDelay(d time.Duration) Option {
	return func(c *ExecutorConfig) {
		c.RetryInitialDelay = d
	}
}

// WithTimeout sets the default execution timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *ExecutorConfig) {
		c.DefaultTimeout = d
	}
}

// WithPerToolBulkhead adds a dedicated bulkhead for the named tool with the
// given concurrency limit.  Tools without a dedicated bulkhead share the
// default one configured via WithMaxConcurrent.
func WithPerToolBulkhead(toolName string, maxConcurrent int) Option {
	return func(c *ExecutorConfig) {
		if c.PerToolBulkheads == nil {
			c.PerToolBulkheads = make(map[string]int)
		}
		c.PerToolBulkheads[toolName] = maxConcurrent
	}
}

// WithPerToolBulkheads sets dedicated bulkheads for multiple tools at once.
func WithPerToolBulkheads(bulkheads map[string]int) Option {
	return func(c *ExecutorConfig) {
		if c.PerToolBulkheads == nil {
			c.PerToolBulkheads = make(map[string]int, len(bulkheads))
		}
		for name, limit := range bulkheads {
			c.PerToolBulkheads[name] = limit
		}
	}
}

// NewExecutorWithOptions creates an executor with the given options.
func NewExecutorWithOptions(opts ...Option) *Executor {
	config := DefaultExecutorConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return NewExecutor(config)
}
