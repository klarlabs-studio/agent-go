package application

import (
	"context"
	"errors"
	"testing"

	axidomain "go.klarlabs.de/axi/domain"

	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/infrastructure/governance"
)

// stubVerifierGovernor is a Governor that also exposes EvidenceVerifier with a
// controllable verification result, to test the engine's run-completion
// evidence-chain check without touching axi internals.
type stubVerifierGovernor struct {
	verifyErr error
}

func (s *stubVerifierGovernor) Authorize(context.Context, governance.ToolRequest) (governance.Authorization, error) {
	return governance.Authorization{Decision: governance.DecisionAllow}, nil
}

func (s *stubVerifierGovernor) Commit(context.Context, governance.ToolRequest, governance.Outcome) (governance.Commit, error) {
	return governance.Commit{Remaining: -1}, nil
}

func (s *stubVerifierGovernor) BudgetSnapshot() policy.BudgetSnapshot {
	return policy.NewBudget(nil).Snapshot()
}

func (s *stubVerifierGovernor) OwnsApproval() bool { return true }

func (s *stubVerifierGovernor) VerifyEvidenceChain() error { return s.verifyErr }

func (s *stubVerifierGovernor) EvidenceCount() int { return 0 }

func TestVerifyRunEvidence_NonVerifierGovernorPasses(t *testing.T) {
	// A Governor without an evidence chain verifies trivially.
	if err := verifyRunEvidence(governance.NewPassthrough(policy.NewBudget(nil), nil)); err != nil {
		t.Fatalf("non-verifier governor must pass, got: %v", err)
	}
}

func TestVerifyRunEvidence_ValidChainPasses(t *testing.T) {
	if err := verifyRunEvidence(&stubVerifierGovernor{verifyErr: nil}); err != nil {
		t.Fatalf("valid chain must pass, got: %v", err)
	}
}

func TestVerifyRunEvidence_BrokenChainFails(t *testing.T) {
	broken := &axidomain.ErrChainBroken{Index: 1, Reason: "Hash mismatch"}
	err := verifyRunEvidence(&stubVerifierGovernor{verifyErr: broken})
	if err == nil {
		t.Fatal("broken chain must fail run evidence verification")
	}
	var chainErr *axidomain.ErrChainBroken
	if !errors.As(err, &chainErr) {
		t.Fatalf("want wrapped ErrChainBroken, got %T: %v", err, err)
	}
}
