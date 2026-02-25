package middleware

import (
	"context"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/metrics"
	mw "github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// MetricsConfig configures the metrics middleware.
type MetricsConfig struct {
	// Provider is the metrics provider to use.
	Provider metrics.Metrics
}

// Metrics creates a middleware that records metrics for tool executions.
//
// This middleware records:
// - Tool execution count (with tool name, state, and success attributes)
// - Tool execution duration histogram
// - Errors (when tool execution fails)
//
// Example:
//
//	mw := middleware.Metrics(middleware.MetricsConfig{
//	    Provider: provider,
//	})
//
//	engine, _ := api.New(
//	    api.WithPlanner(planner),
//	    api.WithMiddleware(mw),
//	)
func Metrics(config MetricsConfig) mw.Middleware {
	if config.Provider == nil {
		config.Provider = &metrics.NoopProvider{}
	}

	return func(next mw.Handler) mw.Handler {
		return func(ctx context.Context, execCtx *mw.ExecutionContext) (tool.Result, error) {
			start := time.Now()

			// Execute the tool
			result, err := next(ctx, execCtx)

			// Record metrics
			duration := time.Since(start)
			success := err == nil && result.Error == nil
			state := execCtx.CurrentState.String()
			toolName := execCtx.Tool.Name()

			config.Provider.RecordToolExecution(ctx, toolName, state, success, duration)

			return result, err
		}
	}
}

// MetricsWithCaching creates a cache metrics recorder.
// This should be used in conjunction with a caching middleware.
func MetricsWithCaching(config MetricsConfig) CacheMetricsRecorder {
	if config.Provider == nil {
		config.Provider = &metrics.NoopProvider{}
	}

	return &cacheMetricsRecorder{
		provider: config.Provider,
	}
}

// CacheMetricsRecorder records cache-related metrics.
type CacheMetricsRecorder interface {
	RecordHit(ctx context.Context, toolName string)
	RecordMiss(ctx context.Context, toolName string)
}

type cacheMetricsRecorder struct {
	provider metrics.Metrics
}

func (r *cacheMetricsRecorder) RecordHit(ctx context.Context, toolName string) {
	r.provider.RecordCacheHit(ctx, toolName)
}

func (r *cacheMetricsRecorder) RecordMiss(ctx context.Context, toolName string) {
	r.provider.RecordCacheMiss(ctx, toolName)
}

// RateLimitMetricsRecorder records rate limit metrics.
type RateLimitMetricsRecorder interface {
	RecordLimitHit(ctx context.Context, toolName string)
}

// MetricsRateLimitRecorder returns a rate limit metrics recorder.
func MetricsRateLimitRecorder(config MetricsConfig) RateLimitMetricsRecorder {
	if config.Provider == nil {
		config.Provider = &metrics.NoopProvider{}
	}

	return &rateLimitMetricsRecorder{
		provider: config.Provider,
	}
}

type rateLimitMetricsRecorder struct {
	provider metrics.Metrics
}

func (r *rateLimitMetricsRecorder) RecordLimitHit(ctx context.Context, toolName string) {
	r.provider.RecordRateLimitHit(ctx, toolName)
}

// CircuitBreakerMetricsRecorder records circuit breaker metrics.
type CircuitBreakerMetricsRecorder interface {
	RecordStateChange(ctx context.Context, toolName string, isOpen bool)
}

// MetricsCircuitBreakerRecorder returns a circuit breaker metrics recorder.
func MetricsCircuitBreakerRecorder(config MetricsConfig) CircuitBreakerMetricsRecorder {
	if config.Provider == nil {
		config.Provider = &metrics.NoopProvider{}
	}

	return &circuitBreakerMetricsRecorder{
		provider: config.Provider,
	}
}

type circuitBreakerMetricsRecorder struct {
	provider metrics.Metrics
}

func (r *circuitBreakerMetricsRecorder) RecordStateChange(ctx context.Context, toolName string, isOpen bool) {
	r.provider.RecordCircuitBreakerStateChange(ctx, toolName, isOpen)
}
