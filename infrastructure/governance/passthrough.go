package governance

import (
	"context"
	"time"

	"go.klarlabs.de/agent/domain/policy"
)

// Passthrough is the in-process Governor backed by domain/policy. It wraps
// the run's *policy.Budget so budget snapshots stay consistent with the
// engine's ledger and run results.
//
// Passthrough deliberately leaves approval to the engine's approval
// middleware (OwnsApproval reports false): its behaviour is identical to
// the engine's original inline budget enforcement. Authorize still honours
// RequireApproval so the type is a complete, independently testable
// Governor — the engine simply does not set RequireApproval while approval
// lives in the middleware.
type Passthrough struct {
	budget   *policy.Budget
	approver policy.Approver
}

// NewPassthrough builds a Passthrough Governor over an existing budget.
// The same *policy.Budget must back the run's machine context so that
// snapshots agree. approver may be nil when no tool requires approval.
func NewPassthrough(budget *policy.Budget, approver policy.Approver) *Passthrough {
	return &Passthrough{budget: budget, approver: approver}
}

// Authorize checks the budget and, when the request demands it, approval.
func (p *Passthrough) Authorize(ctx context.Context, req ToolRequest) (Authorization, error) {
	if !p.budget.CanConsume(budgetKey, 1) {
		return Authorization{Decision: DecisionBudgetExhausted}, nil
	}

	if req.RequireApproval {
		if p.approver == nil {
			return Authorization{}, ErrNoApprover
		}
		resp, err := p.approver.Approve(ctx, policy.ApprovalRequest{
			RunID:     req.RunID,
			ToolName:  req.ToolName,
			Input:     req.Input,
			Reason:    req.Reason,
			RiskLevel: req.RiskLevel,
			Timestamp: time.Now(),
		})
		if err != nil {
			return Authorization{}, err
		}
		if !resp.Approved {
			reason := resp.Reason
			if reason == "" {
				reason = "approval denied"
			}
			return Authorization{Decision: DecisionDenied, Approver: resp.Approver, Reason: reason}, nil
		}
		return Authorization{Decision: DecisionAllow, Approver: resp.Approver}, nil
	}

	return Authorization{Decision: DecisionAllow}, nil
}

// Commit consumes one tool-call budget slot on success and returns the
// remaining budget. Evidence recording is the engine's responsibility in
// the passthrough path (run.AddEvidence + ledger); the axi Governor owns it
// once active.
func (p *Passthrough) Commit(_ context.Context, _ ToolRequest, out Outcome) (Commit, error) {
	if out.Success {
		_ = p.budget.Consume(budgetKey, 1)
	}
	return Commit{Remaining: p.budget.Remaining(budgetKey)}, nil
}

// BudgetSnapshot returns the wrapped budget's snapshot.
func (p *Passthrough) BudgetSnapshot() policy.BudgetSnapshot { return p.budget.Snapshot() }

// OwnsApproval reports false: approval stays with the engine middleware.
func (p *Passthrough) OwnsApproval() bool { return false }

// PassthroughFactory builds Passthrough Governors. Use it to keep approval
// in the engine's middleware (e.g. in tests, or LLM-free scripted runs that
// do not need the axi kernel).
type PassthroughFactory struct {
	approver policy.Approver
}

// NewPassthroughFactory builds a Factory of Passthrough Governors.
func NewPassthroughFactory(approver policy.Approver) *PassthroughFactory {
	return &PassthroughFactory{approver: approver}
}

// Governor returns a Passthrough Governor over the given budget. The
// Passthrough holds no per-run resource, so ctx is unused.
func (f *PassthroughFactory) Governor(_ context.Context, budget *policy.Budget) Governor {
	return NewPassthrough(budget, f.approver)
}

// OwnsApproval reports false: approval stays with the engine middleware.
func (f *PassthroughFactory) OwnsApproval() bool { return false }
