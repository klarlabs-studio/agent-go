// Package middleware provides pre-built middleware implementations.
package middleware

import (
	"context"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/logging"
)

// EligibilityConfig configures the eligibility middleware.
type EligibilityConfig struct {
	// Eligibility defines which tools are allowed in which states.
	Eligibility *policy.ToolEligibility
}

// Eligibility returns middleware that enforces tool eligibility per state.
// If a tool is not allowed in the current state, execution is blocked.
// When a destructive tool is allowed via wildcard in a non-Act state,
// a warning is logged to alert operators of potential state semantic violations.
func Eligibility(cfg EligibilityConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if no eligibility configured
			if cfg.Eligibility == nil {
				return next(ctx, execCtx)
			}

			// Check if tool is allowed in current state
			if !cfg.Eligibility.IsAllowed(execCtx.CurrentState, execCtx.Tool.Name()) {
				return tool.Result{}, fmt.Errorf("%w: %s in state %s",
					tool.ErrToolNotAllowed, execCtx.Tool.Name(), execCtx.CurrentState)
			}

			// Warn when a destructive tool passes via wildcard in a non-Act state.
			// This is informational only — approval middleware handles enforcement.
			if execCtx.CurrentState != agent.StateAct &&
				cfg.Eligibility.HasWildcard(execCtx.CurrentState) &&
				execCtx.Tool.Annotations().ShouldRequireApproval() {
				logging.Warn().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.State(execCtx.CurrentState)).
					Add(logging.ToolName(execCtx.Tool.Name())).
					Msg("destructive tool allowed via wildcard in non-act state")
			}

			return next(ctx, execCtx)
		}
	}
}
