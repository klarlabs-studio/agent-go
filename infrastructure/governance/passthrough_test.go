package governance

import (
	"context"
	"testing"

	"go.klarlabs.de/agent/domain/policy"
)

func newBudget(t *testing.T, limit int) *policy.Budget {
	t.Helper()
	return policy.NewBudget(map[string]int{budgetKey: limit})
}

func TestPassthrough_AllowsWithinBudget(t *testing.T) {
	g := NewPassthrough(newBudget(t, 2), nil)

	auth, err := g.Authorize(context.Background(), ToolRequest{ToolName: "read"})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !auth.Allowed() {
		t.Fatalf("want allow, got %+v", auth)
	}
}

func TestPassthrough_BudgetExhausted(t *testing.T) {
	g := NewPassthrough(newBudget(t, 1), nil)
	ctx := context.Background()
	req := ToolRequest{ToolName: "read"}

	if _, err := g.Commit(ctx, req, Outcome{Success: true}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	auth, err := g.Authorize(ctx, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Decision != DecisionBudgetExhausted {
		t.Fatalf("want DecisionBudgetExhausted, got %+v", auth)
	}
}

func TestPassthrough_CommitConsumesOnlyOnSuccess(t *testing.T) {
	g := NewPassthrough(newBudget(t, 3), nil)
	ctx := context.Background()
	req := ToolRequest{ToolName: "write"}

	c, _ := g.Commit(ctx, req, Outcome{Success: false})
	if c.Remaining != 3 {
		t.Fatalf("failed call must not consume budget; remaining=%d", c.Remaining)
	}
	c, _ = g.Commit(ctx, req, Outcome{Success: true})
	if c.Remaining != 2 {
		t.Fatalf("success must consume one slot; remaining=%d", c.Remaining)
	}
}

func TestPassthrough_ApprovalRequiredNoApprover(t *testing.T) {
	g := NewPassthrough(newBudget(t, 1), nil)

	_, err := g.Authorize(context.Background(), ToolRequest{ToolName: "rm", RequireApproval: true})
	if err != ErrNoApprover {
		t.Fatalf("want ErrNoApprover, got %v", err)
	}
}

func TestPassthrough_ApprovalDenied(t *testing.T) {
	g := NewPassthrough(newBudget(t, 1), policy.NewDenyApprover("nope"))

	auth, err := g.Authorize(context.Background(), ToolRequest{ToolName: "rm", RequireApproval: true})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Decision != DecisionDenied || auth.Reason != "nope" {
		t.Fatalf("want denied with reason, got %+v", auth)
	}
}

func TestPassthrough_ApprovalGranted(t *testing.T) {
	g := NewPassthrough(newBudget(t, 1), policy.NewAutoApprover("ci"))

	auth, err := g.Authorize(context.Background(), ToolRequest{ToolName: "rm", RequireApproval: true})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !auth.Allowed() || auth.Approver != "ci" {
		t.Fatalf("want allow by ci, got %+v", auth)
	}
}

func TestPassthrough_DoesNotOwnApproval(t *testing.T) {
	g := NewPassthrough(newBudget(t, 1), nil)
	if g.OwnsApproval() {
		t.Fatal("passthrough must leave approval to the middleware")
	}
}
