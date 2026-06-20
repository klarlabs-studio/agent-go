package governance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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
	g := f.Governor(policy.NewBudget(map[string]int{budgetKey: limit}))
	return f, g
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
