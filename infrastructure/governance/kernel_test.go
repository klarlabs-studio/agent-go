package governance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	axidomain "go.klarlabs.de/axi/domain"

	"go.klarlabs.de/agent/domain/policy"
)

// kernelGov builds a full-delegation Governor for one run with the given
// tool-call budget and approver, failing the test on construction error.
func kernelGov(t *testing.T, limit int, approver policy.Approver) (*KernelFactory, Governor) {
	t.Helper()
	f, err := NewKernelFactory(approver)
	if err != nil {
		t.Fatalf("NewKernelFactory: %v", err)
	}
	g := f.Governor(context.Background(), policy.NewBudget(map[string]int{budgetKey: limit}))
	return f, g
}

// allowAndCommit authorizes a successful tool call and commits it, advancing
// both the local mirror and axi's authoritative successful-call count.
func allowAndCommit(t *testing.T, g Governor, ctx context.Context, tool string) Authorization {
	t.Helper()
	auth, err := g.Authorize(ctx, ToolRequest{ToolName: tool, RunID: "run-1"})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Allowed() {
		if _, err := g.Commit(ctx, ToolRequest{ToolName: tool}, Outcome{Success: true, Output: json.RawMessage(`{}`)}); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}
	return auth
}

func TestKernel_OwnsApproval(t *testing.T) {
	f, err := NewKernelFactory(nil)
	if err != nil {
		t.Fatalf("NewKernelFactory: %v", err)
	}
	if !f.OwnsApproval() {
		t.Fatal("kernel factory must own approval")
	}
}

// A run's tool calls consume ONE axi session budget: the Nth tool call
// (N = limit+1) exceeds the budget and is denied. Budget is enforced
// natively by axi across CapabilityInvoker calls within one session.
func TestKernel_RunBudgetIsOneAxiSession(t *testing.T) {
	_, g := kernelGov(t, 2, nil)
	defer closeGov(t, g)
	ctx := context.Background()

	// First two tool calls authorized and committed (consume the session budget).
	for i := 0; i < 2; i++ {
		auth, err := g.Authorize(ctx, ToolRequest{ToolName: "read", RunID: "run-1"})
		if err != nil {
			t.Fatalf("Authorize call %d: %v", i+1, err)
		}
		if !auth.Allowed() {
			t.Fatalf("call %d: want allow, got %+v", i+1, auth)
		}
		if _, err := g.Commit(ctx, ToolRequest{ToolName: "read"}, Outcome{Success: true, Output: json.RawMessage(`{}`)}); err != nil {
			t.Fatalf("Commit call %d: %v", i+1, err)
		}
	}

	// Third tool call exceeds the run budget — axi's session enforces it.
	auth, err := g.Authorize(ctx, ToolRequest{ToolName: "read", RunID: "run-1"})
	if err != nil {
		t.Fatalf("Authorize call 3: %v", err)
	}
	if auth.Decision != DecisionBudgetExhausted {
		t.Fatalf("call 3: want budget exhausted, got %+v", auth)
	}
}

// axi (not the local mirror) is the authoritative budget enforcer: driving
// the run session directly past its MaxCapabilityInvocations limit returns
// axi's native budget-exceeded error from CapabilityInvoker.Invoke.
func TestKernel_AxiSessionIsAuthoritativeBudgetEnforcer(t *testing.T) {
	_, g := kernelGov(t, 1, nil)
	defer closeGov(t, g)
	kg := g.(*kernelGovernor)
	ctx := context.Background()

	// First invocation within budget.
	if err := kg.invokeStep(ctx, ToolRequest{ToolName: "read"}); err != nil {
		t.Fatalf("step 1 must succeed: %v", err)
	}
	// Second invocation exceeds MaxCapabilityInvocations=1 — axi's session
	// enforcer fails it natively, independent of the local mirror.
	err := kg.invokeStep(ctx, ToolRequest{ToolName: "read"})
	if err == nil {
		t.Fatal("step 2 must be rejected by axi budget enforcer")
	}
	if !isBudgetExceeded(err) {
		t.Fatalf("want axi budget-exceeded, got: %v", err)
	}
}

// A failed tool call does NOT consume a run-budget slot — parity with the
// passthrough and approval-only axi Governors, which consume only on success.
// Under full delegation, the authoritative axi caps.Invoke happens at Commit
// gated on Outcome.Success, so a failure burns nothing.
func TestKernel_FailedToolDoesNotConsumeBudget(t *testing.T) {
	_, g := kernelGov(t, 2, nil)
	defer closeGov(t, g)
	ctx := context.Background()
	req := ToolRequest{ToolName: "write", RunID: "run-1"}

	// Authorize, then the tool fails: Commit with Success:false consumes nothing.
	auth, err := g.Authorize(ctx, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !auth.Allowed() {
		t.Fatalf("want allow, got %+v", auth)
	}
	commit, err := g.Commit(ctx, req, Outcome{Success: false})
	if err != nil {
		t.Fatalf("Commit(failure): %v", err)
	}
	if commit.Remaining != 2 {
		t.Fatalf("failed tool must not consume budget; remaining=%d want 2", commit.Remaining)
	}

	// The whole budget remains: two successful calls must still be allowed.
	for i := 0; i < 2; i++ {
		if a := allowAndCommit(t, g, ctx, "read"); !a.Allowed() {
			t.Fatalf("success call %d should be allowed, got %+v", i+1, a)
		}
	}
	// Now exhausted.
	a, err := g.Authorize(ctx, ToolRequest{ToolName: "read", RunID: "run-1"})
	if err != nil {
		t.Fatalf("Authorize after exhaustion: %v", err)
	}
	if a.Decision != DecisionBudgetExhausted {
		t.Fatalf("want budget exhausted after 2 successes, got %+v", a)
	}
}

// A successful tool call consumes exactly one slot, matching the other
// Governors' success-only accounting.
func TestKernel_SuccessConsumesOneSlot(t *testing.T) {
	_, g := kernelGov(t, 3, nil)
	defer closeGov(t, g)
	ctx := context.Background()
	allowAndCommit(t, g, ctx, "read")
	snap := g.BudgetSnapshot()
	if snap.Remaining[budgetKey] != 2 {
		t.Fatalf("one success must leave 2; remaining=%d", snap.Remaining[budgetKey])
	}
}

// A zero tool_calls budget must allow ZERO tool calls. axi treats
// MaxCapabilityInvocations:0 as no-limit, so the governor must block the very
// first call on its own authoritative precedence gate, never delegating an
// unlimited session.
func TestKernel_ZeroBudgetAllowsNoToolCalls(t *testing.T) {
	_, g := kernelGov(t, 0, nil)
	defer closeGov(t, g)
	auth, err := g.Authorize(context.Background(), ToolRequest{ToolName: "read", RunID: "run-1"})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Decision != DecisionBudgetExhausted {
		t.Fatalf("zero budget must deny first call, got %+v", auth)
	}
}

// Approval gating still fires for destructive tools, granted via the kernel.
func TestKernel_ApprovalGatedGranted(t *testing.T) {
	_, g := kernelGov(t, 5, policy.NewAutoApprover("ci"))
	defer closeGov(t, g)
	auth, err := g.Authorize(context.Background(), ToolRequest{
		ToolName: "rm", RiskLevel: "high", RequireApproval: true, RunID: "run-1",
	})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !auth.Allowed() || auth.Approver != "ci" {
		t.Fatalf("want allow by ci, got %+v", auth)
	}
}

// Approval gating still fires for destructive tools, denied via the kernel.
func TestKernel_ApprovalGatedDenied(t *testing.T) {
	_, g := kernelGov(t, 5, policy.NewDenyApprover("blocked"))
	defer closeGov(t, g)
	auth, err := g.Authorize(context.Background(), ToolRequest{
		ToolName: "rm", RequireApproval: true, RunID: "run-1",
	})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Decision != DecisionDenied || auth.Reason != "blocked" {
		t.Fatalf("want denied/blocked, got %+v", auth)
	}
}

func TestKernel_ApprovalRequiredNoApprover(t *testing.T) {
	_, g := kernelGov(t, 5, nil)
	defer closeGov(t, g)
	_, err := g.Authorize(context.Background(), ToolRequest{ToolName: "rm", RequireApproval: true})
	if !errors.Is(err, ErrNoApprover) {
		t.Fatalf("want ErrNoApprover, got %v", err)
	}
}

// The run's evidence chain verifies after a sequence of tool calls, and
// it is a single axi-native chain (one EvidenceRecord per committed call).
func TestKernel_EvidenceChainVerifies(t *testing.T) {
	_, g := kernelGov(t, 5, nil)
	defer closeGov(t, g)
	ctx := context.Background()

	v, ok := g.(EvidenceVerifier)
	if !ok {
		t.Fatalf("kernel governor must expose EvidenceVerifier")
	}

	for i := 0; i < 3; i++ {
		if _, err := g.Authorize(ctx, ToolRequest{ToolName: "read", RunID: "run-1"}); err != nil {
			t.Fatalf("Authorize %d: %v", i, err)
		}
		if _, err := g.Commit(ctx, ToolRequest{ToolName: "read"},
			Outcome{Success: true, Output: json.RawMessage(`{"i":` + string(rune('0'+i)) + `}`)}); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}

	if err := v.VerifyEvidenceChain(); err != nil {
		t.Fatalf("evidence chain must verify, got: %v", err)
	}
	if n := v.EvidenceCount(); n != 3 {
		t.Fatalf("want 3 evidence records, got %d", n)
	}
}

// Evidence corruption fails verification: a tampered record breaks the chain.
func TestKernel_EvidenceCorruptionFailsVerification(t *testing.T) {
	_, g := kernelGov(t, 5, nil)
	defer closeGov(t, g)
	ctx := context.Background()

	if _, err := g.Authorize(ctx, ToolRequest{ToolName: "read"}); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if _, err := g.Commit(ctx, ToolRequest{ToolName: "read"}, Outcome{Success: true, Output: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	corrupter, ok := g.(evidenceCorrupter)
	if !ok {
		t.Fatalf("test requires evidenceCorrupter test hook")
	}
	corrupter.corruptEvidence(0)

	v := g.(EvidenceVerifier)
	err := v.VerifyEvidenceChain()
	if err == nil {
		t.Fatal("expected corrupted evidence chain to fail verification")
	}
	var broken *axidomain.ErrChainBroken
	if !errors.As(err, &broken) {
		t.Fatalf("want ErrChainBroken, got %T: %v", err, err)
	}
}

// Authorize after Close must not deadlock — it returns a governance fault,
// never blocking on the vanished run-session receiver.
func TestKernel_AuthorizeAfterCloseDoesNotDeadlock(t *testing.T) {
	_, g := kernelGov(t, 5, nil)
	closeGov(t, g)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = g.Authorize(context.Background(), ToolRequest{ToolName: "read"})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Authorize after Close deadlocked")
	}
}

// If the held run session's goroutine exits before Close (e.g. the run ctx is
// cancelled, or kernel.Execute returns early before the orchestrator loop),
// invokeStep must not block forever waiting on a vanished receiver. The
// governor closes runExited on EVERY goroutine exit path, so invokeStep
// unwinds promptly. Driven here by cancelling the run ctx, which makes the
// held session return and close runExited without Close being called.
func TestKernel_EarlyGoroutineExitDoesNotDeadlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f, err := NewKernelFactory(nil)
	if err != nil {
		t.Fatalf("NewKernelFactory: %v", err)
	}
	g := f.Governor(ctx, policy.NewBudget(map[string]int{budgetKey: 5}))
	kg := g.(*kernelGovernor)
	defer closeGov(t, g)

	// Cancel the run ctx; the held session's goroutine returns and closes
	// runExited. Wait for it so the early-exit state is in effect.
	cancel()
	select {
	case <-kg.runExited:
	case <-time.After(2 * time.Second):
		t.Fatal("run session goroutine did not exit on ctx cancel")
	}

	// A tool call after the early exit must not deadlock — it unwinds on
	// runExited (here ctx is also done, but the point is no hang on reqCh).
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = kg.invokeStep(context.Background(), ToolRequest{ToolName: "read"})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("invokeStep deadlocked after early goroutine exit")
	}
}

// The evidence chain is a single continuous physical chain across a
// human-input pause: a governor rehydrated from a prior segment's snapshot
// continues the same hash chain, and verification still holds end-to-end.
func TestKernel_EvidenceRehydratesAcrossPause(t *testing.T) {
	ctx := context.Background()
	f, err := NewKernelFactory(nil)
	if err != nil {
		t.Fatalf("NewKernelFactory: %v", err)
	}

	// Segment 1: two successful tool calls, then snapshot (as a pause would).
	b1 := policy.NewBudget(map[string]int{budgetKey: 5})
	g1 := f.Governor(ctx, b1)
	for i := 0; i < 2; i++ {
		allowAndCommit(t, g1, ctx, "read")
	}
	snap, err := g1.(EvidenceSnapshotter).EvidenceSnapshot()
	if err != nil {
		t.Fatalf("EvidenceSnapshot: %v", err)
	}
	closeGov(t, g1)

	// Segment 2: rehydrate, seed remaining budget, append one more call.
	b2 := policy.NewBudget(map[string]int{budgetKey: 5})
	_ = b2.Consume(budgetKey, 2) // seed consumed from segment 1
	g2 := f.Governor(ctx, b2)
	if err := g2.(EvidenceRehydrator).RehydrateEvidence(snap); err != nil {
		t.Fatalf("RehydrateEvidence: %v", err)
	}
	allowAndCommit(t, g2, ctx, "read")
	defer closeGov(t, g2)

	v := g2.(EvidenceVerifier)
	if err := v.VerifyEvidenceChain(); err != nil {
		t.Fatalf("continuous chain must verify across pause: %v", err)
	}
	if n := v.EvidenceCount(); n != 3 {
		t.Fatalf("want 3 records on the continuous chain, got %d", n)
	}
}

// Budget survives a pause: a governor seeded with the consumed count from a
// prior segment enforces the run-spanning remainder, not a reset full budget.
func TestKernel_BudgetSeededAcrossPause(t *testing.T) {
	ctx := context.Background()
	f, err := NewKernelFactory(nil)
	if err != nil {
		t.Fatalf("NewKernelFactory: %v", err)
	}
	// Run-level budget is 2; segment 1 already consumed 2 before the pause.
	b := policy.NewBudget(map[string]int{budgetKey: 2})
	_ = b.Consume(budgetKey, 2)
	g := f.Governor(ctx, b)
	defer closeGov(t, g)

	auth, err := g.Authorize(ctx, ToolRequest{ToolName: "read", RunID: "run-1"})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Decision != DecisionBudgetExhausted {
		t.Fatalf("seeded-exhausted budget must deny, got %+v (no reset to full)", auth)
	}
}

// Two concurrent kernel-governed runs do not interfere: each has its own
// session, budget, and evidence chain.
func TestKernel_ConcurrentRunsDoNotInterfere(t *testing.T) {
	ctx := context.Background()
	f, err := NewKernelFactory(nil)
	if err != nil {
		t.Fatalf("NewKernelFactory: %v", err)
	}
	run := func(limit int) error {
		g := f.Governor(ctx, policy.NewBudget(map[string]int{budgetKey: limit}))
		defer closeGov(t, g)
		for i := 0; i < limit; i++ {
			if a := allowAndCommit(t, g, ctx, "read"); !a.Allowed() {
				return errors.New("expected allow within budget")
			}
		}
		a, err := g.Authorize(ctx, ToolRequest{ToolName: "read", RunID: "run-1"})
		if err != nil {
			return err
		}
		if a.Decision != DecisionBudgetExhausted {
			return errors.New("expected exhaustion at limit")
		}
		if v, ok := g.(EvidenceVerifier); ok {
			if err := v.VerifyEvidenceChain(); err != nil {
				return err
			}
			if v.EvidenceCount() != limit {
				return errors.New("evidence count mismatch")
			}
		}
		return nil
	}
	errCh := make(chan error, 2)
	go func() { errCh <- run(2) }()
	go func() { errCh <- run(3) }()
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent run %d: %v", i, err)
		}
	}
}

func closeGov(t *testing.T, g Governor) {
	t.Helper()
	if c, ok := g.(interface{ Close() error }); ok {
		if err := c.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}
}

// evidenceCorrupter is a test-only hook to tamper with the evidence chain
// so verification failure can be exercised.
type evidenceCorrupter interface {
	corruptEvidence(index int)
}
