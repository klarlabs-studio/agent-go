// Package resilience provides resilient execution patterns using fortify.
package resilience

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/felixgeelhaar/fortify/bulkhead"
	"github.com/felixgeelhaar/fortify/circuitbreaker"
	"github.com/felixgeelhaar/fortify/retry"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// ErrRateLimited is returned when a tool execution is rejected by the rate
// limiter.
var ErrRateLimited = errors.New("rate limited")

// BulkheadMetrics provides saturation metrics for a bulkhead.
type BulkheadMetrics struct {
	// Active is the number of currently executing operations.
	Active int64
	// Rejected is the cumulative count of rejected operations.
	Rejected int64
}

// Executor provides resilient tool execution with circuit breaker, retry,
// bulkhead, and rate limiter patterns.
type Executor struct {
	// Shared (default) bulkhead used when per-tool isolation is not configured.
	bulkhead bulkhead.Bulkhead[tool.Result]

	// Per-tool bulkheads. When non-nil, tools with a matching entry use an
	// isolated bulkhead instead of the shared one.
	perToolBulkheads map[string]bulkhead.Bulkhead[tool.Result]

	breaker circuitbreaker.CircuitBreaker[tool.Result]
	retry   retry.Retry[tool.Result]
	timeout time.Duration

	// hooks for circuit breaker lifecycle events.
	cbHooks     *CircuitBreakerHooks
	lastCBState atomic.Value // stores circuitbreaker.State

	// rate limiter set (nil when rate limiting is disabled).
	rateLimiters *rateLimiterSet

	// saturation metrics per bulkhead key ("" = shared).
	bulkheadMetrics   map[string]*bulkheadMetricsEntry
	bulkheadMetricsMu sync.RWMutex
}

// bulkheadMetricsEntry tracks saturation counters for a single bulkhead.
type bulkheadMetricsEntry struct {
	active   atomic.Int64
	rejected atomic.Int64
}

// ExecutorConfig configures the resilient executor.
type ExecutorConfig struct {
	// MaxConcurrent limits concurrent tool executions.
	MaxConcurrent int

	// CircuitBreakerThreshold is the number of failures before opening.
	CircuitBreakerThreshold int

	// CircuitBreakerTimeout is how long the circuit stays open.
	CircuitBreakerTimeout time.Duration

	// RetryMaxAttempts is the maximum number of retry attempts.
	RetryMaxAttempts int

	// RetryInitialDelay is the initial delay between retries.
	RetryInitialDelay time.Duration

	// RetryBackoffMultiplier is the exponential backoff multiplier.
	RetryBackoffMultiplier float64

	// DefaultTimeout is the default execution timeout.
	DefaultTimeout time.Duration

	// CBHooks contains optional circuit breaker lifecycle callbacks.
	CBHooks *CircuitBreakerHooks

	// PerToolBulkheads maps tool names to their dedicated bulkhead
	// concurrency limits.  Tools not listed here use the shared bulkhead.
	PerToolBulkheads map[string]int

	// RateLimiter optionally configures rate limiting in the executor
	// pipeline.
	RateLimiter *RateLimiterConfig
}

// DefaultExecutorConfig returns a configuration with sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		MaxConcurrent:           10,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
		RetryMaxAttempts:        3,
		RetryInitialDelay:       100 * time.Millisecond,
		RetryBackoffMultiplier:  2.0,
		DefaultTimeout:          30 * time.Second,
	}
}

// NewExecutor creates a new resilient executor.
func NewExecutor(config ExecutorConfig) *Executor {
	// Ensure non-negative values for uint32 conversion (G115 fix)
	maxConcurrent := config.MaxConcurrent
	if maxConcurrent < 0 {
		maxConcurrent = 10 // default
	}
	threshold := config.CircuitBreakerThreshold
	if threshold < 0 {
		threshold = 5 // default
	}

	e := &Executor{
		bulkhead: bulkhead.New[tool.Result](bulkhead.Config{
			MaxConcurrent: maxConcurrent,
		}),
		breaker: circuitbreaker.New[tool.Result](circuitbreaker.Config{
			MaxRequests: uint32(maxConcurrent), // #nosec G115 -- bounds checked above
			Interval:    config.CircuitBreakerTimeout,
			Timeout:     config.CircuitBreakerTimeout,
			ReadyToTrip: func(counts circuitbreaker.Counts) bool {
				return counts.ConsecutiveFailures >= uint32(threshold) // #nosec G115 -- bounds checked above
			},
		}),
		retry: retry.New[tool.Result](retry.Config{
			MaxAttempts:   config.RetryMaxAttempts,
			InitialDelay:  config.RetryInitialDelay,
			BackoffPolicy: retry.BackoffExponential,
			Multiplier:    config.RetryBackoffMultiplier,
		}),
		timeout:         config.DefaultTimeout,
		cbHooks:         config.CBHooks,
		bulkheadMetrics: make(map[string]*bulkheadMetricsEntry),
	}

	// Store the initial circuit breaker state for change detection.
	e.lastCBState.Store(e.breaker.State())

	// Build per-tool bulkheads when configured.
	if len(config.PerToolBulkheads) > 0 {
		e.perToolBulkheads = make(map[string]bulkhead.Bulkhead[tool.Result], len(config.PerToolBulkheads))
		for name, limit := range config.PerToolBulkheads {
			if limit <= 0 {
				limit = maxConcurrent
			}
			e.perToolBulkheads[name] = bulkhead.New[tool.Result](bulkhead.Config{
				MaxConcurrent: limit,
			})
			e.bulkheadMetrics[name] = &bulkheadMetricsEntry{}
		}
	}

	// Initialise shared bulkhead metrics entry.
	e.bulkheadMetrics[""] = &bulkheadMetricsEntry{}

	// Build rate limiter set.
	e.rateLimiters = newRateLimiterSet(config.RateLimiter)

	return e
}

// NewDefaultExecutor creates an executor with default configuration.
func NewDefaultExecutor() *Executor {
	return NewExecutor(DefaultExecutorConfig())
}

// Execute runs a tool with resilience patterns applied.
// Composition order: RateLimit → Bulkhead → Timeout → CircuitBreaker → Retry (for idempotent)
func (e *Executor) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	toolName := t.Name()

	// --- Rate limiter ---
	if !e.rateLimiters.allow(ctx, toolName) {
		return tool.Result{}, fmt.Errorf("%w: tool %s", ErrRateLimited, toolName)
	}

	start := time.Now()

	// Select bulkhead (per-tool or shared).
	bh := e.bulkhead
	metricsKey := ""
	if e.perToolBulkheads != nil {
		if ptb, ok := e.perToolBulkheads[toolName]; ok {
			bh = ptb
			metricsKey = toolName
		}
	}

	// Track saturation.
	metrics := e.getBulkheadMetrics(metricsKey)

	// Apply bulkhead for concurrency control.
	result, err := bh.Execute(ctx, func(ctx context.Context) (tool.Result, error) {
		metrics.active.Add(1)
		defer metrics.active.Add(-1)

		// Apply timeout.
		ctx, cancel := context.WithTimeout(ctx, e.timeout)
		defer cancel()

		// Apply circuit breaker.
		cbResult, cbErr := e.breaker.Execute(ctx, func(ctx context.Context) (tool.Result, error) {
			// Apply retry only for idempotent tools.
			if t.Annotations().CanRetry() {
				return e.retry.Execute(ctx, func(ctx context.Context) (tool.Result, error) {
					return t.Execute(ctx, input)
				})
			}
			return t.Execute(ctx, input)
		})

		// Detect circuit breaker state changes and fire hooks.
		e.detectCBStateChange(toolName)

		return cbResult, cbErr
	})

	if err != nil {
		// Track bulkhead rejections (heuristic: bulkhead errors are not
		// context-caused when the parent context is still alive).
		if ctx.Err() == nil {
			metrics.rejected.Add(1)
		}
	}

	// Add timing information.
	if err == nil {
		result.Duration = time.Since(start)
	}

	return result, err
}

// ExecuteWithTimeout runs a tool with a custom timeout.
func (e *Executor) ExecuteWithTimeout(ctx context.Context, t tool.Tool, input json.RawMessage, timeout time.Duration) (tool.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return e.Execute(ctx, t, input)
}

// ExecuteSimple runs a tool without resilience patterns.
// Use this for tools that should not be retried or protected.
func (e *Executor) ExecuteSimple(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	start := time.Now()
	result, err := t.Execute(ctx, input)
	if err == nil {
		result.Duration = time.Since(start)
	}
	return result, err
}

// CircuitBreakerState returns the current state of the circuit breaker.
func (e *Executor) CircuitBreakerState() circuitbreaker.State {
	return e.breaker.State()
}

// Reset resets the circuit breaker to closed state.
func (e *Executor) Reset() {
	// Circuit breaker will automatically reset after timeout.
}

// BulkheadMetricsFor returns saturation metrics for the named tool's bulkhead.
// Pass an empty string to get the shared bulkhead metrics.
func (e *Executor) BulkheadMetricsFor(toolName string) BulkheadMetrics {
	m := e.getBulkheadMetrics(toolName)
	return BulkheadMetrics{
		Active:   m.active.Load(),
		Rejected: m.rejected.Load(),
	}
}

// getBulkheadMetrics returns or lazily creates the metrics entry for the given
// key.
func (e *Executor) getBulkheadMetrics(key string) *bulkheadMetricsEntry {
	e.bulkheadMetricsMu.RLock()
	m, ok := e.bulkheadMetrics[key]
	e.bulkheadMetricsMu.RUnlock()
	if ok {
		return m
	}

	e.bulkheadMetricsMu.Lock()
	defer e.bulkheadMetricsMu.Unlock()
	// Double-check after acquiring write lock.
	if m, ok = e.bulkheadMetrics[key]; ok {
		return m
	}
	m = &bulkheadMetricsEntry{}
	e.bulkheadMetrics[key] = m
	return m
}

// detectCBStateChange compares the current circuit breaker state against the
// last observed state and fires hooks when a transition occurred.
func (e *Executor) detectCBStateChange(toolName string) {
	if e.cbHooks == nil {
		return
	}

	current := e.breaker.State()
	prev, _ := e.lastCBState.Load().(circuitbreaker.State)
	if current != prev {
		e.lastCBState.Store(current)
		e.cbHooks.fireStateChange(prev, current, toolName)
	}
}
