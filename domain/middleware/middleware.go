// Package middleware provides composable middleware for tool execution.
package middleware

import (
	"context"
	"encoding/json"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/tool"
)

// BudgetView provides read-only access to budget state.
type BudgetView interface {
	// CanConsume checks if the budget allows consumption.
	CanConsume(name string, amount int) bool
	// Remaining returns the remaining budget for the given name.
	Remaining(name string) int
}

// ExecutionContext contains all information needed for middleware decisions.
type ExecutionContext struct {
	// RunID is the unique identifier for the current run.
	RunID string
	// CurrentState is the current state of the agent.
	CurrentState agent.State
	// Tool is the tool being executed.
	Tool tool.Tool
	// Input is the JSON input for the tool.
	Input json.RawMessage
	// Reason is the planner's reason for calling this tool.
	Reason string
	// Budget provides access to budget state.
	Budget BudgetView
	// Vars contains shared variables for the run.
	Vars map[string]any
	// EventPublisher is an optional callback for middleware to publish events.
	// The eventType is a string like "approval.requested" and payload is the
	// event-specific data struct. Nil-safe: middleware must check before calling.
	EventPublisher func(eventType string, payload any)
}

// Handler executes a tool and returns its result.
type Handler func(ctx context.Context, execCtx *ExecutionContext) (tool.Result, error)

// Middleware wraps a Handler with additional behavior.
// Middleware can:
// - Execute code before the next handler
// - Execute code after the next handler
// - Short-circuit by not calling next
// - Modify the execution context
// - Transform results or errors
type Middleware func(next Handler) Handler

// Chain composes multiple middleware into a single middleware.
// Middleware are executed in the order provided, with each wrapping the next.
// For example, Chain(A, B, C) produces: A -> B -> C -> handler
func Chain(middlewares ...Middleware) Middleware {
	return func(final Handler) Handler {
		// Build chain from right to left so execution is left to right
		handler := final
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}
		return handler
	}
}

// Noop returns a middleware that does nothing, just passes through.
func Noop() Middleware {
	return func(next Handler) Handler {
		return next
	}
}
