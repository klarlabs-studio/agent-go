package governance

import (
	"context"
	"testing"

	"go.klarlabs.de/agent/domain/policy"
)

func axiGov(t *testing.T, limit int, approver policy.Approver) Governor {
	t.Helper()
	f, err := NewAxiFactory(approver)
	if err != nil {
		t.Fatalf("NewAxiFactory: %v", err)
	}
	return f.Governor(context.Background(), policy.NewBudget(map[string]int{budgetKey: limit}))
}

func TestAxi_NonApprovalToolAllowed(t *testing.T) {
	g := axiGov(t, 5, nil)
	auth, err := g.Authorize(context.Background(), ToolRequest{ToolName: "read", RequireApproval: false})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !auth.Allowed() {
		t.Fatalf("want allow, got %+v", auth)
	}
}

func TestAxi_ApprovalGatedGranted(t *testing.T) {
	g := axiGov(t, 5, policy.NewAutoApprover("ci"))
	auth, err := g.Authorize(context.Background(), ToolRequest{ToolName: "rm", RiskLevel: "high", RequireApproval: true})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !auth.Allowed() || auth.Approver != "ci" {
		t.Fatalf("want allow by ci, got %+v", auth)
	}
}

func TestAxi_ApprovalGatedDenied(t *testing.T) {
	g := axiGov(t, 5, policy.NewDenyApprover("blocked"))
	auth, err := g.Authorize(context.Background(), ToolRequest{ToolName: "rm", RequireApproval: true})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Decision != DecisionDenied || auth.Reason != "blocked" {
		t.Fatalf("want denied/blocked, got %+v", auth)
	}
}

func TestAxi_ApprovalRequiredNoApprover(t *testing.T) {
	g := axiGov(t, 5, nil)
	if _, err := g.Authorize(context.Background(), ToolRequest{ToolName: "rm", RequireApproval: true}); err != ErrNoApprover {
		t.Fatalf("want ErrNoApprover, got %v", err)
	}
}

func TestAxi_BudgetExhausted(t *testing.T) {
	g := axiGov(t, 1, nil)
	ctx := context.Background()
	if _, err := g.Commit(ctx, ToolRequest{ToolName: "read"}, Outcome{Success: true}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	auth, err := g.Authorize(ctx, ToolRequest{ToolName: "read"})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Decision != DecisionBudgetExhausted {
		t.Fatalf("want budget exhausted, got %+v", auth)
	}
}

func TestAxiFactory_OwnsApproval(t *testing.T) {
	f, err := NewAxiFactory(nil)
	if err != nil {
		t.Fatalf("NewAxiFactory: %v", err)
	}
	if !f.OwnsApproval() {
		t.Fatal("axi factory must own approval")
	}
}
