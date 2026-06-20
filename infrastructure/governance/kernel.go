package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.klarlabs.de/axi"
	axidomain "go.klarlabs.de/axi/domain"

	"go.klarlabs.de/agent/domain/policy"
)

// Full budget+evidence delegation to axi (spec § Changes Required #1, Track F).
//
// The approval-only AxiFactory delegates just the destructive-tool gate to
// axi; run-level budget and the evidence trail still live in agent-go. The
// KernelFactory closes that gap: each agent RUN executes as ONE axi session,
// so budget (run-spanning tool count), approval (per-effect gate), and the
// evidence chain are all axi-native.
//
// Design — one axi session per run, no axi changes:
//
// axi's ExecutionBudget (MaxCapabilityInvocations) is enforced per session,
// across every CapabilityInvoker.Invoke call made within one in-flight
// action execution. A held session is therefore realised in agent-go (the
// consumer) rather than added to axi: KernelFactory.Governor starts ONE
// kernel.Execute(runSessionAction) on a goroutine whose
// OrchestratorActionExecutor blocks on a channel. Each act-state tool call
// drives one caps.Invoke through that in-flight session, so axi's budget
// enforcer counts tool calls across the whole run and fails the (N+1)th
// natively with the budget error. The run loop stays statekit-driven; only
// the governed tool gate routes through the kernel session. axi is untouched
// — no held-session API, preserving its independence.
//
// Evidence: each committed tool call appends one EvidenceRecord to the run's
// axidomain.ExecutionSession, forming a single tamper-evident chain per run,
// verifiable via VerifyEvidenceChain.
//
// Approval: destructive tools drive the write-external gate action through
// the kernel (the same primitive AxiFactory uses), gated per call.

const (
	// runStepCapability is the no-op capability invoked once per act-state
	// tool call so axi's per-session budget enforcer counts run-level tool
	// calls. Its budget slot maps tool_calls → MaxCapabilityInvocations.
	runStepCapability = "agent.run.step"
	runStepExecRef    = "exec.cap.agent.run.step"
	runSessionAction  = "agent.run.session"
	runSessionExecRef = "exec.agent.run.session"
	runPluginID       = "agent.run"
)

// EvidenceVerifier exposes the run's axi evidence chain for verification.
// kernelGovernor implements it so callers can audit the single per-run chain.
type EvidenceVerifier interface {
	// VerifyEvidenceChain validates the run's evidence chain integrity,
	// returning *axidomain.ErrChainBroken at the first tampered record.
	VerifyEvidenceChain() error
	// EvidenceCount reports the number of records on the run's chain.
	EvidenceCount() int
}

// KernelFactory builds per-run kernel Governors that fully delegate budget,
// approval, and evidence to axi. Each run gets its own axi.Kernel so the
// run-level budget maps onto that kernel's per-session ExecutionBudget.
type KernelFactory struct {
	approver policy.Approver
}

// NewKernelFactory builds a Factory of full-delegation kernel Governors.
// approver may be nil; an approval-requiring tool then fails closed with
// ErrNoApprover at authorization time.
func NewKernelFactory(approver policy.Approver) (*KernelFactory, error) {
	return &KernelFactory{approver: approver}, nil
}

// Governor starts one axi session for the run and returns a Governor bound
// to it. The session's ExecutionBudget.MaxCapabilityInvocations is taken
// from the run's tool_calls budget, so axi enforces the run-level count.
func (f *KernelFactory) Governor(budget *policy.Budget) Governor {
	limit := budget.Remaining(budgetKey) // -1 when unlimited

	kernel := axi.New()
	if limit >= 0 {
		kernel.WithBudget(axi.Budget{MaxCapabilityInvocations: limit})
	}
	kernel.RegisterCapabilityExecutor(runStepExecRef, runStepExecutor{})
	kernel.RegisterActionExecutor(gateExecutorRef, gateExecutor{})

	g := &kernelGovernor{
		budget:   budget,
		approver: f.approver,
		kernel:   kernel,
		reqCh:    make(chan stepRequest),
		doneCh:   make(chan struct{}),
	}

	// Build the run-session executor that holds the in-flight session and
	// drives one caps.Invoke per tool call. Register it together with the
	// approval gate plugin.
	exec := &runSessionExecutor{reqCh: g.reqCh, doneCh: g.doneCh}
	kernel.RegisterActionExecutor(runSessionExecRef, exec)
	if err := kernel.RegisterPlugin(runPlugin{}); err != nil {
		g.startErr = fmt.Errorf("governance: register run session plugin: %w", err)
		return g
	}
	if err := kernel.RegisterPlugin(gatePlugin{}); err != nil {
		g.startErr = fmt.Errorf("governance: register approval gate: %w", err)
		return g
	}

	// Build the run's evidence session: one tamper-evident chain per run.
	session, err := axidomain.NewExecutionSession(
		axidomain.ExecutionSessionID("run-evidence"),
		runSessionAction,
		nil,
	)
	if err != nil {
		g.startErr = fmt.Errorf("governance: new evidence session: %w", err)
		return g
	}
	g.evidence = session

	// Launch the single in-flight run session. kernel.Execute blocks in the
	// executor on reqCh until Close signals completion.
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		_, runErr := kernel.Execute(context.Background(), axi.Invocation{Action: runSessionAction})
		g.mu.Lock()
		g.runErr = runErr
		g.mu.Unlock()
	}()

	return g
}

// OwnsApproval reports true: the kernel enforces budget, approval, and
// evidence, so the engine omits its approval middleware.
func (f *KernelFactory) OwnsApproval() bool { return true }

// stepRequest carries one act-state tool call into the in-flight run session.
type stepRequest struct {
	input  any
	respCh chan stepResponse
}

// stepResponse returns the budget verdict for one tool call.
type stepResponse struct {
	err error
}

// kernelGovernor enforces budget, approval, and evidence for one run, all
// through one axi session. Budget is enforced by the in-flight runSession
// (caps.Invoke per tool call); approval via the gate action; evidence on a
// single per-run chain.
type kernelGovernor struct {
	budget   *policy.Budget
	approver policy.Approver
	kernel   *axi.Kernel

	reqCh  chan stepRequest
	doneCh chan struct{}
	wg     sync.WaitGroup

	evidence *axidomain.ExecutionSession

	mu       sync.Mutex
	consumed int
	startErr error
	runErr   error
	closed   bool
}

// Authorize gates a tool. Precedence matches the in-process Governors:
// budget exhaustion is reported before approval. The run-level budget is
// enforced by the in-flight axi session — each authorized call drives one
// caps.Invoke, and the (limit+1)th invocation fails natively. The local
// policy.Budget mirror provides the non-consuming precedence check and keeps
// BudgetSnapshot in lockstep with axi's session count.
func (g *kernelGovernor) Authorize(ctx context.Context, req ToolRequest) (Authorization, error) {
	if g.startErr != nil {
		return Authorization{}, g.startErr
	}

	// Budget precedence: a non-consuming check so an exhausted budget is
	// reported before (and instead of) prompting for approval.
	if !g.budget.CanConsume(budgetKey, 1) {
		return Authorization{Decision: DecisionBudgetExhausted}, nil
	}

	// Approval gate next, so a denied tool consumes no budget — mirroring the
	// in-process Governors.
	approver := ""
	if req.RequireApproval {
		if g.approver == nil {
			return Authorization{}, ErrNoApprover
		}
		auth, err := g.gateApproval(ctx, req)
		if err != nil || auth.Decision != DecisionAllow {
			return auth, err
		}
		approver = auth.Approver
	}

	// Authoritative consume: one capability invocation against the run
	// session. axi's enforcer counts it and fails the (limit+1)th natively.
	if err := g.invokeStep(ctx, req); err != nil {
		if isBudgetExceeded(err) {
			return Authorization{Decision: DecisionBudgetExhausted}, nil
		}
		return Authorization{}, fmt.Errorf("governance: run session step: %w", err)
	}
	g.consumeStep()

	return Authorization{Decision: DecisionAllow, Approver: approver}, nil
}

// consumeStep advances the local budget mirror to track the run session's
// axi-enforced invocation count.
func (g *kernelGovernor) consumeStep() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.consumed++
	_ = g.budget.Consume(budgetKey, 1)
}

// invokeStep sends one tool call into the in-flight run session and waits for
// axi's budget verdict.
func (g *kernelGovernor) invokeStep(ctx context.Context, req ToolRequest) error {
	respCh := make(chan stepResponse, 1)
	select {
	case g.reqCh <- stepRequest{input: req.ToolName, respCh: respCh}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case resp := <-respCh:
		return resp.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// gateApproval runs the kernel's write-external approval gate for a
// destructive tool, settling it via the configured approver.
func (g *kernelGovernor) gateApproval(ctx context.Context, req ToolRequest) (Authorization, error) {
	out, err := g.kernel.Execute(ctx, axi.Invocation{Action: gateAction})
	if err != nil {
		return Authorization{}, fmt.Errorf("governance: axi approval gate: %w", err)
	}
	if !out.RequiresApproval {
		return Authorization{Decision: DecisionAllow}, nil
	}

	resp, err := g.approver.Approve(ctx, policy.ApprovalRequest{
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

// Commit records a completed tool call's evidence on the run's single
// axi-native chain. The budget slot was already consumed (authoritatively by
// axi) at Authorize time — axi gates the call attempt, so Commit does not
// double-count. It returns the remaining tool-call budget for the ledger.
func (g *kernelGovernor) Commit(_ context.Context, req ToolRequest, out Outcome) (Commit, error) {
	if out.Success {
		g.evidence.AppendEvidence(axidomain.EvidenceRecord{
			Kind:      "tool_result",
			Source:    req.ToolName,
			Value:     string(out.Output),
			Timestamp: time.Now().UnixMilli(),
		})
	}
	return Commit{Remaining: g.budget.Remaining(budgetKey)}, nil
}

// BudgetSnapshot returns the run budget's snapshot. The local mirror is kept
// in lockstep with axi's session budget via invokeStep + Consume.
func (g *kernelGovernor) BudgetSnapshot() policy.BudgetSnapshot { return g.budget.Snapshot() }

// OwnsApproval reports true: the kernel owns budget, approval, and evidence.
func (g *kernelGovernor) OwnsApproval() bool { return true }

// VerifyEvidenceChain validates the run's single evidence chain.
func (g *kernelGovernor) VerifyEvidenceChain() error {
	return g.evidence.VerifyEvidenceChain()
}

// EvidenceCount reports the number of records on the run's evidence chain.
func (g *kernelGovernor) EvidenceCount() int {
	return len(g.evidence.Evidence())
}

// Close ends the in-flight run session and waits for the goroutine to finish.
func (g *kernelGovernor) Close() error {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil
	}
	g.closed = true
	g.mu.Unlock()

	if g.startErr == nil {
		close(g.doneCh)
		g.wg.Wait()
	}

	g.mu.Lock()
	runErr := g.runErr
	g.mu.Unlock()
	if runErr != nil && !errors.Is(runErr, errRunSessionClosed) {
		return runErr
	}
	return nil
}

// corruptEvidence is a test-only hook (see kernel_test.go) that tampers with
// a record on the run chain so verification failure can be exercised. It
// round-trips the session through axi's public snapshot API, mutates the
// record's Value while leaving its Hash untouched, and reloads — so
// VerifyEvidenceChain recomputes a mismatching hash. No axi internals are
// touched; the chain is otherwise append-only.
func (g *kernelGovernor) corruptEvidence(index int) {
	snap := g.evidence.ToSnapshot()
	if index < 0 || index >= len(snap.Evidence) {
		return
	}
	snap.Evidence[index].Value = "tampered"
	reloaded, err := axidomain.SessionFromSnapshot(snap)
	if err != nil {
		return
	}
	g.evidence = reloaded
}

// errRunSessionClosed unwinds the in-flight run session executor cleanly when
// the governor is closed. It is an expected terminal signal, not a fault.
var errRunSessionClosed = errors.New("governance: run session closed")

// runSessionExecutor is the OrchestratorActionExecutor that holds the run's
// in-flight axi session. It blocks on reqCh, driving exactly one caps.Invoke
// per act-state tool call so axi's per-session budget counts run-level tool
// calls, and unwinds when doneCh closes.
type runSessionExecutor struct {
	reqCh  <-chan stepRequest
	doneCh <-chan struct{}
}

// ExecuteOrchestrated drives the run session. caps is the per-session
// CapabilityInvoker whose budget enforcer counts every Invoke across the run.
func (e *runSessionExecutor) ExecuteOrchestrated(
	ctx context.Context,
	_ any,
	caps axidomain.CapabilityInvoker,
	_ axidomain.ActionInvoker,
) (axidomain.ExecutionResult, []axidomain.EvidenceRecord, error) {
	for {
		select {
		case <-e.doneCh:
			return axidomain.ExecutionResult{Summary: "run session complete"}, nil, nil
		case <-ctx.Done():
			return axidomain.ExecutionResult{}, nil, ctx.Err()
		case req := <-e.reqCh:
			_, err := caps.Invoke(runStepCapability, req.input)
			req.respCh <- stepResponse{err: err}
		}
	}
}

// Execute is the synchronous fallback required of OrchestratorActionExecutor
// implementations. It is unused in the run-session path (an ActionInvoker is
// always wired by the kernel) but keeps the contract complete.
func (e *runSessionExecutor) Execute(
	ctx context.Context,
	input any,
	caps axidomain.CapabilityInvoker,
) (axidomain.ExecutionResult, []axidomain.EvidenceRecord, error) {
	return e.ExecuteOrchestrated(ctx, input, caps, nil)
}

// runStepExecutor is the no-op capability behind runStepCapability. Invoking
// it consumes one MaxCapabilityInvocations slot — that is the only effect
// that matters; the real tool runs in agent-go after authorization.
type runStepExecutor struct{}

func (runStepExecutor) Execute(_ context.Context, _ any) (any, error) {
	return struct{}{}, nil
}

// runPlugin contributes the run-session action and its step capability. The
// action has a none effect profile so the run session itself never pauses for
// approval — per-tool approval is gated separately via the write-external
// gate action.
type runPlugin struct{}

func (runPlugin) Contribute() (*axidomain.PluginContribution, error) {
	stepCap, err := axidomain.NewCapabilityDefinition(
		runStepCapability,
		"Counts one act-state tool call against the run budget",
		axidomain.EmptyContract(),
		axidomain.EmptyContract(),
	)
	if err != nil {
		return nil, err
	}
	if err := stepCap.BindExecutor(runStepExecRef); err != nil {
		return nil, err
	}

	req, err := axidomain.NewRequirementSet(axidomain.Requirement{Capability: runStepCapability})
	if err != nil {
		return nil, err
	}
	action, err := axidomain.NewActionDefinition(
		runSessionAction,
		"One agent run as a single axi session enforcing the tool-call budget",
		axidomain.EmptyContract(),
		axidomain.EmptyContract(),
		req,
		axidomain.EffectProfile{Level: axidomain.EffectNone},
		axidomain.IdempotencyProfile{IsIdempotent: false},
	)
	if err != nil {
		return nil, err
	}
	if err := action.BindExecutor(runSessionExecRef); err != nil {
		return nil, err
	}
	return axidomain.NewPluginContribution(
		runPluginID,
		[]*axidomain.ActionDefinition{action},
		[]*axidomain.CapabilityDefinition{stepCap},
	)
}

// isBudgetExceeded reports whether err is axi's per-session budget-exceeded
// error raised by CapabilityInvoker.Invoke. axi encodes the limit kind in the
// message ("execution budget exceeded: max N capability invocations"); it does
// not yet return a typed sentinel, so a substring match is the available seam.
func isBudgetExceeded(err error) bool {
	return err != nil && strings.Contains(err.Error(), "budget exceeded")
}
