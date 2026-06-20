// Package governance is agent-go's governance seam for act-state tool
// execution: budget enforcement, approval gating, and the audit/evidence
// trail.
//
// Per the agent-go spec, governance is delegated to axi-go so the whole
// stack shares one governance primitive — memory writes (mnemos) and tool
// execution (agent-go) produce compatible, single-source evidence trails.
// The domain/policy package keeps its interfaces (Budget, Approval,
// Eligibility); this package binds them to an implementation.
//
// Two implementations satisfy Governor:
//
//   - Passthrough — the in-process implementation backed by domain/policy
//     (Budget, Approver). Behaviour-identical to the pre-delegation engine.
//     Built by default. Approval is left to the engine's approval
//     middleware (OwnsApproval reports false).
//
//   - axiGovernor (build tag "axi") — routes every act-state tool call
//     through an axi.Kernel, so budget, approval, and the evidence trail
//     all come from the single axi governance kernel (OwnsApproval reports
//     true; the engine drops its approval middleware).
//
// Activation is deferred until the toolchain bump that axi-go requires:
//
//  1. bump the agent-go module go directive to 1.26.2 (axi-go's floor)
//  2. add `require go.klarlabs.de/axi v1.4.0` to go.mod
//  3. build and test with `-tags axi`
//  4. construct the engine's per-run Governor with NewAxi(...) instead of
//     NewPassthrough(...)
//
// See doc.go for the full migration checklist.
package governance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.klarlabs.de/agent/domain/policy"
)

// budgetKey is the budget dimension consumed by one act-state tool call.
// It mirrors the key the engine has always used so budgets configured via
// WithBudget("tool_calls", N) map straight through, and — under axi — onto
// ExecutionBudget.MaxCapabilityInvocations.
const budgetKey = "tool_calls"

// ErrNoApprover reports that a tool requires approval but no approver was
// configured on the Governor.
var ErrNoApprover = errors.New("governance: tool requires approval but no approver is configured")

// Decision is the verdict a Governor returns for a tool authorization.
type Decision int

const (
	// DecisionAllow permits the tool to execute.
	DecisionAllow Decision = iota
	// DecisionBudgetExhausted denies execution because the budget is spent.
	DecisionBudgetExhausted
	// DecisionDenied denies execution because approval was refused.
	DecisionDenied
)

// ToolRequest describes a single act-state tool invocation seeking
// authorization. The engine builds it from a CallTool decision.
type ToolRequest struct {
	RunID string
	State string
	// ToolName is the registered name of the tool.
	ToolName string
	// Input is the raw tool input payload.
	Input json.RawMessage
	// Reason is the planner's stated reason for the call.
	Reason string
	// RiskLevel is the tool's annotated risk level, for the approval record.
	RiskLevel string
	// RequireApproval is true when the tool's annotations demand human
	// approval. The engine only sets it when the Governor OwnsApproval;
	// otherwise approval is handled by the approval middleware.
	RequireApproval bool
}

// Authorization is a Governor's verdict for a ToolRequest.
type Authorization struct {
	Decision Decision
	// Approver identifies who approved, when Decision is DecisionAllow via
	// an approval gate. Empty when no approval was required.
	Approver string
	// Reason carries the denial reason when Decision is DecisionDenied.
	Reason string
}

// Allowed reports whether the tool may execute.
func (a Authorization) Allowed() bool { return a.Decision == DecisionAllow }

// Outcome reports a completed tool execution to the Governor for budget
// accounting and evidence recording.
type Outcome struct {
	Success  bool
	Output   json.RawMessage
	Duration time.Duration
}

// Commit is the result of accounting a completed tool call.
type Commit struct {
	// Remaining is the remaining tool-call budget (-1 means unlimited).
	Remaining int
}

// Governor enforces governance for act-state tool execution. One Governor
// is created per run, alongside the run's budget and ledger.
//
// Implementations enforce budget and (when OwnsApproval is true) approval,
// and emit the audit/evidence trail.
type Governor interface {
	// Authorize gates a tool before it executes. It returns
	// DecisionBudgetExhausted when the budget is spent, DecisionDenied when
	// approval is refused, or DecisionAllow otherwise. A non-nil error
	// signals a governance fault (e.g. ErrNoApprover), not a denial.
	Authorize(ctx context.Context, req ToolRequest) (Authorization, error)

	// Commit accounts a completed tool execution against the budget and
	// records its evidence. It returns the remaining tool-call budget.
	Commit(ctx context.Context, req ToolRequest, out Outcome) (Commit, error)

	// BudgetSnapshot exposes the current budget for run results and ledger.
	BudgetSnapshot() policy.BudgetSnapshot

	// OwnsApproval reports whether this Governor enforces approval itself.
	// When true, the engine omits its approval middleware to avoid a double
	// gate; when false, approval stays with the middleware.
	OwnsApproval() bool
}

// Factory builds a per-run Governor bound to that run's budget. The engine
// holds one Factory (so shared state like an axi.Kernel is built once) and
// asks it for a Governor at the start of each run.
type Factory interface {
	// Governor returns a Governor for one run over the given budget.
	Governor(budget *policy.Budget) Governor
	// OwnsApproval reports whether Governors from this Factory enforce
	// approval, so the engine can drop its approval middleware once, at
	// construction, rather than per run.
	OwnsApproval() bool
}
