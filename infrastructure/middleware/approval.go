package middleware

import (
	"context"
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
)

// ApprovalConfig configures the approval middleware.
type ApprovalConfig struct {
	// Approver handles approval requests.
	Approver policy.Approver
}

// Approval returns middleware that enforces approval for high-risk tools.
// Tools that require approval (destructive, high-risk, or explicitly marked)
// must be approved before execution.
func Approval(cfg ApprovalConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			t := execCtx.Tool
			annotations := t.Annotations()

			// Check if approval is required
			if !annotations.ShouldRequireApproval() {
				return next(ctx, execCtx)
			}

			// No approver configured - fail if approval required
			if cfg.Approver == nil {
				return tool.Result{}, fmt.Errorf("%w: no approver configured for tool %s",
					tool.ErrApprovalRequired, t.Name())
			}

			// Build approval request
			req := policy.ApprovalRequest{
				RunID:     execCtx.RunID,
				ToolName:  t.Name(),
				Input:     execCtx.Input,
				Reason:    execCtx.Reason,
				RiskLevel: annotations.RiskLevel.String(),
				Timestamp: time.Now(),
			}

			// Publish approval.requested event
			publishEvent(execCtx, string(event.TypeApprovalRequested), event.ApprovalRequestedPayload{
				ToolName:  t.Name(),
				Input:     execCtx.Input,
				RiskLevel: annotations.RiskLevel.String(),
			})

			// Request approval
			resp, err := cfg.Approver.Approve(ctx, req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("approval error: %w", err)
			}

			if !resp.Approved {
				reason := "approval denied"
				if resp.Reason != "" {
					reason = resp.Reason
				}
				// Publish approval.denied event
				publishEvent(execCtx, string(event.TypeApprovalDenied), event.ApprovalResultPayload{
					ToolName: t.Name(),
					Approver: resp.Approver,
					Reason:   reason,
				})
				return tool.Result{}, fmt.Errorf("%w: %s", tool.ErrApprovalDenied, reason)
			}

			// Publish approval.granted event
			publishEvent(execCtx, string(event.TypeApprovalGranted), event.ApprovalResultPayload{
				ToolName: t.Name(),
				Approver: resp.Approver,
			})

			return next(ctx, execCtx)
		}
	}
}

// publishEvent is a nil-safe helper for middleware event publishing.
func publishEvent(execCtx *middleware.ExecutionContext, eventType string, payload any) {
	if execCtx.EventPublisher != nil {
		execCtx.EventPublisher(eventType, payload)
	}
}
