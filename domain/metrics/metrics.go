// Package metrics defines the metrics recording interface for the agent-go runtime.
//
// This package belongs to the domain layer and has no external dependencies.
// Concrete implementations (e.g., OpenTelemetry) live in contrib/otel.
package metrics

import (
	"context"
	"time"
)

// Metrics defines the interface for recording runtime metrics.
type Metrics interface {
	RecordToolExecution(ctx context.Context, toolName string, state string, success bool, duration time.Duration)
	RecordStateTransition(ctx context.Context, fromState, toState string, runID string)
	RecordBudgetConsumption(ctx context.Context, budgetName string, amount int64, remaining int64)
	RecordRateLimitHit(ctx context.Context, toolName string)
	RecordCacheHit(ctx context.Context, toolName string)
	RecordCacheMiss(ctx context.Context, toolName string)
	RecordError(ctx context.Context, errorType string, details map[string]string)
	RecordPlanningDuration(ctx context.Context, duration time.Duration, state string)
	RecordRunDuration(ctx context.Context, duration time.Duration, finalState string, success bool)
	IncrementActiveRuns(ctx context.Context)
	DecrementActiveRuns(ctx context.Context)
	RecordCircuitBreakerStateChange(ctx context.Context, toolName string, isOpen bool)
}

// NoopProvider is a no-op metrics provider for testing or when metrics are disabled.
type NoopProvider struct{}

// RecordToolExecution is a no-op.
func (n *NoopProvider) RecordToolExecution(ctx context.Context, toolName string, state string, success bool, duration time.Duration) {
}

// RecordStateTransition is a no-op.
func (n *NoopProvider) RecordStateTransition(ctx context.Context, fromState, toState string, runID string) {
}

// RecordBudgetConsumption is a no-op.
func (n *NoopProvider) RecordBudgetConsumption(ctx context.Context, budgetName string, amount int64, remaining int64) {
}

// RecordRateLimitHit is a no-op.
func (n *NoopProvider) RecordRateLimitHit(ctx context.Context, toolName string) {}

// RecordCacheHit is a no-op.
func (n *NoopProvider) RecordCacheHit(ctx context.Context, toolName string) {}

// RecordCacheMiss is a no-op.
func (n *NoopProvider) RecordCacheMiss(ctx context.Context, toolName string) {}

// RecordError is a no-op.
func (n *NoopProvider) RecordError(ctx context.Context, errorType string, details map[string]string) {
}

// RecordPlanningDuration is a no-op.
func (n *NoopProvider) RecordPlanningDuration(ctx context.Context, duration time.Duration, state string) {
}

// RecordRunDuration is a no-op.
func (n *NoopProvider) RecordRunDuration(ctx context.Context, duration time.Duration, finalState string, success bool) {
}

// IncrementActiveRuns is a no-op.
func (n *NoopProvider) IncrementActiveRuns(ctx context.Context) {}

// DecrementActiveRuns is a no-op.
func (n *NoopProvider) DecrementActiveRuns(ctx context.Context) {}

// RecordCircuitBreakerStateChange is a no-op.
func (n *NoopProvider) RecordCircuitBreakerStateChange(ctx context.Context, toolName string, isOpen bool) {
}

// Ensure NoopProvider satisfies the Metrics interface.
var _ Metrics = (*NoopProvider)(nil)
