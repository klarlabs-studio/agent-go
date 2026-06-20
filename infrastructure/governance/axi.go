package governance

import (
	"context"
	"fmt"

	"go.klarlabs.de/axi"
	axidomain "go.klarlabs.de/axi/domain"

	"go.klarlabs.de/agent/domain/policy"
)

// gateAction is the axi action agent-go executes to delegate the
// destructive-tool approval gate. Its write-external effect profile makes
// the kernel pause the session at awaiting_approval until a human decision
// arrives via Approve/Reject.
const (
	gateAction      = "agent.tool.gated"
	gateExecutorRef = "exec.agent.tool.gated"
	gatePluginID    = "agent.governance"
)

// AxiFactory builds per-run axi-backed Governors that share one kernel.
//
// Governance split (axi v1.4.0): the destructive-tool approval gate is
// delegated to axi — its native, effect-gated pause is the spec's headline
// safety primitive. Run-level tool-call budget stays in agent-go: axi's
// ExecutionBudget is per-session (one action's capability fan-out) and
// cannot express a run-spanning tool count without a held-session API axi
// does not yet expose. The evidence trail likewise stays on the engine's
// run ledger, which is one chain per run rather than the fragmented
// per-call chains axi would produce. See doc.go for the path to full
// budget/evidence delegation (axi held-session API).
type AxiFactory struct {
	kernel   *axi.Kernel
	approver policy.Approver
}

// NewAxiFactory builds the shared kernel and registers the approval-gate
// action once. approver may be nil; an approval-requiring tool then fails
// closed with ErrNoApprover at authorization time.
func NewAxiFactory(approver policy.Approver) (*AxiFactory, error) {
	kernel := axi.New()
	kernel.RegisterActionExecutor(gateExecutorRef, gateExecutor{})
	if err := kernel.RegisterPlugin(gatePlugin{}); err != nil {
		return nil, fmt.Errorf("governance: register axi approval gate: %w", err)
	}
	return &AxiFactory{kernel: kernel, approver: approver}, nil
}

// Governor returns a Governor for one run, bound to that run's budget.
func (f *AxiFactory) Governor(budget *policy.Budget) Governor {
	return &axiGovernor{budget: budget, approver: f.approver, kernel: f.kernel}
}

// OwnsApproval reports true: the kernel enforces the approval gate, so the
// engine omits its approval middleware.
func (f *AxiFactory) OwnsApproval() bool { return true }

// axiGovernor enforces run-level budget in process and delegates the
// destructive-tool approval gate to the shared axi.Kernel.
type axiGovernor struct {
	budget   *policy.Budget
	approver policy.Approver
	kernel   *axi.Kernel
}

// Authorize checks the run budget, then — for approval-requiring tools —
// drives the kernel's effect-gated approval pause through the approver.
func (g *axiGovernor) Authorize(ctx context.Context, req ToolRequest) (Authorization, error) {
	if !g.budget.CanConsume(budgetKey, 1) {
		return Authorization{Decision: DecisionBudgetExhausted}, nil
	}
	if !req.RequireApproval {
		return Authorization{Decision: DecisionAllow}, nil
	}
	if g.approver == nil {
		return Authorization{}, ErrNoApprover
	}

	// Execute the gate action: its write-external effect pauses the session
	// at awaiting_approval before any executor runs.
	out, err := g.kernel.Execute(ctx, axi.Invocation{Action: gateAction})
	if err != nil {
		return Authorization{}, fmt.Errorf("governance: axi approval gate: %w", err)
	}
	if !out.RequiresApproval {
		// Effect profile did not pause (no gate) — nothing to approve.
		return Authorization{Decision: DecisionAllow}, nil
	}

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

	// axi requires a non-empty principal on every decision; fall back to a
	// generic identity when the approver does not name itself.
	principal := resp.Approver
	if principal == "" {
		principal = "agent-go"
	}
	decision := axidomain.ApprovalDecision{Principal: principal, Rationale: resp.Reason}
	sessionID := string(out.SessionID)
	if !resp.Approved {
		reason := resp.Reason
		if reason == "" {
			reason = "approval denied"
		}
		if _, rerr := g.kernel.Reject(ctx, sessionID, decision); rerr != nil {
			return Authorization{}, fmt.Errorf("governance: axi reject: %w", rerr)
		}
		return Authorization{Decision: DecisionDenied, Approver: resp.Approver, Reason: reason}, nil
	}
	if _, aerr := g.kernel.Approve(ctx, sessionID, decision); aerr != nil {
		return Authorization{}, fmt.Errorf("governance: axi approve: %w", aerr)
	}
	return Authorization{Decision: DecisionAllow, Approver: resp.Approver}, nil
}

// Commit consumes one tool-call budget slot on success. Evidence stays on
// the engine's run ledger.
func (g *axiGovernor) Commit(_ context.Context, _ ToolRequest, out Outcome) (Commit, error) {
	if out.Success {
		_ = g.budget.Consume(budgetKey, 1)
	}
	return Commit{Remaining: g.budget.Remaining(budgetKey)}, nil
}

// BudgetSnapshot returns the run budget's snapshot.
func (g *axiGovernor) BudgetSnapshot() policy.BudgetSnapshot { return g.budget.Snapshot() }

// OwnsApproval reports true: the kernel owns the approval gate.
func (g *axiGovernor) OwnsApproval() bool { return true }

// gateExecutor is the no-op executor behind the approval-gate action. The
// real tool runs in agent-go's executor after authorization; this executor
// only marks the gated action complete once approved.
type gateExecutor struct{}

func (gateExecutor) Execute(_ context.Context, _ any, _ axidomain.CapabilityInvoker) (axidomain.ExecutionResult, []axidomain.EvidenceRecord, error) {
	return axidomain.ExecutionResult{Summary: "tool call authorized"}, nil, nil
}

// gatePlugin contributes the write-external approval-gate action.
type gatePlugin struct{}

func (gatePlugin) Contribute() (*axidomain.PluginContribution, error) {
	action, err := axidomain.NewActionDefinition(
		gateAction,
		"Approval gate for a destructive agent tool call",
		axidomain.EmptyContract(),
		axidomain.EmptyContract(),
		nil,
		axidomain.EffectProfile{Level: axidomain.EffectWriteExternal},
		axidomain.IdempotencyProfile{IsIdempotent: false},
	)
	if err != nil {
		return nil, err
	}
	if err := action.BindExecutor(gateExecutorRef); err != nil {
		return nil, err
	}
	return axidomain.NewPluginContribution(gatePluginID, []*axidomain.ActionDefinition{action}, nil)
}
