package middleware

import (
	"context"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
)

// BudgetConfig configures the budget middleware.
type BudgetConfig struct {
	// Budget is the budget tracker to use.
	Budget *policy.Budget
	// BudgetName is the name of the budget to check (e.g., "tool_calls").
	BudgetName string
	// Amount is the amount to check/consume per call.
	Amount int
}

// Budget returns middleware that enforces budget limits.
// It checks budget availability before execution and consumes on success.
func Budget(cfg BudgetConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if no budget configured
			if cfg.Budget == nil {
				return next(ctx, execCtx)
			}

			// Check budget before execution
			if !cfg.Budget.CanConsume(cfg.BudgetName, cfg.Amount) {
				return tool.Result{}, policy.ErrBudgetExceeded
			}

			// Execute the handler
			result, err := next(ctx, execCtx)

			// Consume budget only on success
			if err == nil {
				_ = cfg.Budget.Consume(cfg.BudgetName, cfg.Amount)
			}

			return result, err
		}
	}
}

// BudgetFromContext returns middleware that uses the budget from ExecutionContext.
// This is useful when budget needs to be determined at runtime.
func BudgetFromContext(budgetName string, amount int) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if no budget in context
			if execCtx.Budget == nil {
				return next(ctx, execCtx)
			}

			// Check budget before execution
			if !execCtx.Budget.CanConsume(budgetName, amount) {
				return tool.Result{}, policy.ErrBudgetExceeded
			}

			// Execute the handler
			result, err := next(ctx, execCtx)

			// Note: Budget consumption must be done by caller since BudgetView is read-only
			// This middleware only does the pre-check

			return result, err
		}
	}
}
