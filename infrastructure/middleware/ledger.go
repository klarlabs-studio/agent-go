package middleware

import (
	"context"

	"go.klarlabs.de/agent/domain/ledger"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// LedgerConfig configures the ledger recording middleware.
type LedgerConfig struct {
	// Ledger is the ledger to record to.
	Ledger *ledger.Ledger
}

// LedgerRecording returns middleware that records tool calls to the ledger.
// This provides an audit trail of all tool executions.
func LedgerRecording(cfg LedgerConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if no ledger configured
			if cfg.Ledger == nil {
				return next(ctx, execCtx)
			}

			state := execCtx.CurrentState
			toolName := execCtx.Tool.Name()

			// Record tool call
			cfg.Ledger.RecordToolCall(state, toolName, execCtx.Input)

			// Execute
			result, err := next(ctx, execCtx)

			// Record result or error
			if err != nil {
				cfg.Ledger.RecordToolError(state, toolName, err)
			} else {
				cfg.Ledger.RecordToolResult(state, toolName, result.Output, result.Duration, result.Cached)
			}

			return result, err
		}
	}
}
