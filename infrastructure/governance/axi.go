//go:build axi

// This file is the route-through governance target. It compiles only with
// `-tags axi`, which in turn requires the toolchain/dependency bump axi-go
// needs (go 1.26.2, require go.klarlabs.de/axi v1.4.0). Until then it is
// excluded from the default build so agent-go core stays on go 1.25 with no
// axi dependency. See doc.go for the activation checklist.

package governance

import (
	"context"
	"fmt"

	"go.klarlabs.de/axi"
	axidomain "go.klarlabs.de/axi/domain"

	"go.klarlabs.de/agent/domain/policy"
)

// axiGovernor delegates governance to an axi.Kernel: budget, approval, and
// the evidence trail are all owned by the single axi governance kernel, so
// agent-go tool execution shares one governance primitive with the rest of
// the stack (e.g. mnemos memory writes).
//
// One axiGovernor is created per run. The kernel is configured with the
// run's tool-call budget mapped onto ExecutionBudget.MaxCapabilityInvocations.
// agent-go still performs the actual tool work (its WASM sandbox, resilient
// executor, middleware); the kernel is the governance authority — it decides
// budget, gates approval, and records the run's evidence chain.
type axiGovernor struct {
	kernel    *axi.Kernel
	approver  policy.Approver
	sessionID string
}

// NewAxi builds an axi-backed Governor. limits["tool_calls"] becomes the
// kernel's MaxCapabilityInvocations. approver may be nil when no tool
// requires approval.
//
// TODO(activation): finalise the action/capability registration against the
// live axi v1.4.0 API and replace the placeholder action names below. The
// shape (budget via WithBudget, approval via Execute/Approve/Reject,
// evidence via the session trail) is fixed; only the registration calls are
// pending the real dependency.
func NewAxi(limits map[string]int, approver policy.Approver) (Governor, error) {
	kernel := axi.New().WithBudget(axi.Budget{
		MaxCapabilityInvocations: limits[budgetKey],
	})
	g := &axiGovernor{kernel: kernel, approver: approver}
	if err := g.registerToolAction(); err != nil {
		return nil, fmt.Errorf("governance: register axi tool action: %w", err)
	}
	return g, nil
}

// registerToolAction registers the agent-go "tool call" action with the
// kernel. Authorize invokes this action so the kernel ticks the budget and
// evaluates the approval gate before agent-go runs the tool.
//
// TODO(activation): register a CapabilityExecutor that records the requested
// tool as evidence and surfaces RequiresApproval from the tool's risk
// annotations carried in the invocation input.
func (g *axiGovernor) registerToolAction() error {
	return nil
}

// Authorize asks the kernel to authorize one tool call: it consumes a
// budget slot and, when the tool requires approval, drives the kernel's
// approval gate through the configured approver.
func (g *axiGovernor) Authorize(ctx context.Context, req ToolRequest) (Authorization, error) {
	out, err := g.kernel.Execute(ctx, axi.Invocation{
		Action: "agent.tool.authorize",
		Input: map[string]any{
			"run_id":           req.RunID,
			"tool":             req.ToolName,
			"input":            string(req.Input),
			"reason":           req.Reason,
			"risk":             req.RiskLevel,
			"require_approval": req.RequireApproval,
		},
	})
	if err != nil {
		// A budget-exceeded failure surfaces as a domain failure on the
		// outcome, not a Go error; a Go error is a governance fault.
		return Authorization{}, fmt.Errorf("governance: axi authorize: %w", err)
	}
	g.sessionID = out.SessionID

	if budgetExhausted(out) {
		return Authorization{Decision: DecisionBudgetExhausted}, nil
	}

	if out.RequiresApproval {
		if g.approver == nil {
			return Authorization{}, ErrNoApprover
		}
		return g.resolveApproval(ctx, req, out)
	}

	return Authorization{Decision: DecisionAllow}, nil
}

// resolveApproval drives the human approval gate: it asks the configured
// approver and then settles the kernel session via Approve or Reject so the
// approval decision is recorded on the evidence trail.
func (g *axiGovernor) resolveApproval(ctx context.Context, req ToolRequest, out *axi.Result) (Authorization, error) {
	resp, err := g.approver.Approve(ctx, policy.ApprovalRequest{
		RunID:     req.RunID,
		ToolName:  req.ToolName,
		Input:     req.Input,
		Reason:    req.Reason,
		RiskLevel: req.RiskLevel,
	})
	if err != nil {
		return Authorization{}, err
	}

	decision := axidomain.ApprovalDecision{Approver: resp.Approver, Reason: resp.Reason}
	if !resp.Approved {
		reason := resp.Reason
		if reason == "" {
			reason = "approval denied"
		}
		if _, rerr := g.kernel.Reject(ctx, out.SessionID, decision); rerr != nil {
			return Authorization{}, fmt.Errorf("governance: axi reject: %w", rerr)
		}
		return Authorization{Decision: DecisionDenied, Approver: resp.Approver, Reason: reason}, nil
	}

	if _, aerr := g.kernel.Approve(ctx, out.SessionID, decision); aerr != nil {
		return Authorization{}, fmt.Errorf("governance: axi approve: %w", aerr)
	}
	return Authorization{Decision: DecisionAllow, Approver: resp.Approver}, nil
}

// Commit records the tool's outcome on the kernel's evidence trail and
// reports the remaining budget.
//
// TODO(activation): record the outcome as an EvidenceRecord on the session
// (tool name, output, duration, TokensUsed) so the evidence chain is the
// single source of audit truth, and derive Remaining from the session's
// budget usage.
func (g *axiGovernor) Commit(_ context.Context, _ ToolRequest, _ Outcome) (Commit, error) {
	return Commit{Remaining: -1}, nil
}

// BudgetSnapshot projects the kernel's budget usage into the policy
// snapshot shape the engine reports.
//
// TODO(activation): map the session's ExecutionBudget usage into
// policy.BudgetSnapshot keyed by "tool_calls".
func (g *axiGovernor) BudgetSnapshot() policy.BudgetSnapshot {
	return policy.BudgetSnapshot{
		Limits:    map[string]int{},
		Consumed:  map[string]int{},
		Remaining: map[string]int{},
	}
}

// OwnsApproval reports true: the kernel enforces approval, so the engine
// drops its approval middleware.
func (g *axiGovernor) OwnsApproval() bool { return true }

// budgetExhausted reports whether the kernel failed the invocation because
// the execution budget was spent.
//
// TODO(activation): match against axidomain.BudgetExceeded on the outcome's
// failure rather than the status string once wired to the real API.
func budgetExhausted(out *axi.Result) bool {
	return out.Failure != nil && out.Status == axidomain.StatusFailed
}
