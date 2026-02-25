// Package api provides the public API for the agent-go library.
// This file provides metrics-related exports.
package api

import (
	"github.com/felixgeelhaar/agent-go/domain/metrics"
	"github.com/felixgeelhaar/agent-go/domain/middleware"
	inframw "github.com/felixgeelhaar/agent-go/infrastructure/middleware"
)

// Re-export domain metrics types.
type (
	// Metrics is the interface for recording metrics.
	Metrics = metrics.Metrics

	// NoopMetricsProvider is a no-op implementation for testing.
	NoopMetricsProvider = metrics.NoopProvider
)

// Re-export middleware metrics types.
type (
	// MetricsMiddlewareConfig configures the metrics middleware.
	MetricsMiddlewareConfig = inframw.MetricsConfig

	// CacheMetricsRecorder records cache-related metrics.
	CacheMetricsRecorder = inframw.CacheMetricsRecorder

	// RateLimitMetricsRecorder records rate limit metrics.
	RateLimitMetricsRecorder = inframw.RateLimitMetricsRecorder

	// CircuitBreakerMetricsRecorder records circuit breaker metrics.
	CircuitBreakerMetricsRecorder = inframw.CircuitBreakerMetricsRecorder
)

// WithMetrics adds metrics middleware to the engine.
//
// This middleware records:
//   - Tool execution count (with tool name, state, and success attributes)
//   - Tool execution duration histogram
//   - Errors (when tool execution fails)
//
// Example:
//
//	engine, _ := api.New(
//	    api.WithPlanner(planner),
//	    api.WithMetrics(provider),
//	)
func WithMetrics(provider Metrics) Option {
	return func(c *engineConfig) {
		mw := inframw.Metrics(inframw.MetricsConfig{Provider: provider})
		if c.middleware == nil {
			c.middleware = middleware.NewRegistry()
		}
		c.middleware.Use(mw)
	}
}

// NewCacheMetricsRecorder creates a recorder for cache metrics.
// Use this with caching middleware to track cache hits and misses.
func NewCacheMetricsRecorder(provider Metrics) CacheMetricsRecorder {
	return inframw.MetricsWithCaching(inframw.MetricsConfig{Provider: provider})
}

// NewRateLimitMetricsRecorder creates a recorder for rate limit metrics.
func NewRateLimitMetricsRecorder(provider Metrics) RateLimitMetricsRecorder {
	return inframw.MetricsRateLimitRecorder(inframw.MetricsConfig{Provider: provider})
}

// NewCircuitBreakerMetricsRecorder creates a recorder for circuit breaker metrics.
func NewCircuitBreakerMetricsRecorder(provider Metrics) CircuitBreakerMetricsRecorder {
	return inframw.MetricsCircuitBreakerRecorder(inframw.MetricsConfig{Provider: provider})
}
