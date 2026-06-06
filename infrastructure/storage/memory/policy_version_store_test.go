package memory_test

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

func TestNewPolicyVersionStore(t *testing.T) {
	t.Parallel()

	store := memory.NewPolicyVersionStore()
	if store == nil {
		t.Fatal("NewPolicyVersionStore() returned nil")
	}
}

func TestPolicyVersionStore_Save(t *testing.T) {
	t.Parallel()

	t.Run("saves valid policy version", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		version := &policy.PolicyVersion{
			Version:     1,
			CreatedAt:   time.Now(),
			ProposalID:  "prop-1",
			Description: "Initial version",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		}

		err := store.Save(ctx, version)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	})

	t.Run("returns error for duplicate version", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		version := &policy.PolicyVersion{
			Version:     1,
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		}

		store.Save(ctx, version)
		err := store.Save(ctx, version)
		if err == nil {
			t.Error("Save() should return error for duplicate version")
		}
	})
}

func TestPolicyVersionStore_GetCurrent(t *testing.T) {
	t.Parallel()

	t.Run("returns highest version", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			Description: "v1",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})
		store.Save(ctx, &policy.PolicyVersion{
			Version:     3,
			Description: "v3",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})
		store.Save(ctx, &policy.PolicyVersion{
			Version:     2,
			Description: "v2",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})

		current, err := store.GetCurrent(ctx)
		if err != nil {
			t.Fatalf("GetCurrent() error = %v", err)
		}
		if current.Version != 3 {
			t.Errorf("GetCurrent() Version = %d, want 3", current.Version)
		}
		if current.Description != "v3" {
			t.Errorf("GetCurrent() Description = %s, want v3", current.Description)
		}
	})

	t.Run("returns error for empty store", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		_, err := store.GetCurrent(ctx)
		if err != memory.ErrVersionNotFound {
			t.Errorf("GetCurrent() error = %v, want ErrVersionNotFound", err)
		}
	})
}

func TestPolicyVersionStore_Get(t *testing.T) {
	t.Parallel()

	t.Run("gets specific version", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			Description: "v1",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})
		store.Save(ctx, &policy.PolicyVersion{
			Version:     2,
			Description: "v2",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})

		result, err := store.Get(ctx, 1)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if result.Version != 1 {
			t.Errorf("Get() Version = %d, want 1", result.Version)
		}
		if result.Description != "v1" {
			t.Errorf("Get() Description = %s, want v1", result.Description)
		}
	})

	t.Run("returns error for non-existent version", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		_, err := store.Get(ctx, 999)
		if err != memory.ErrVersionNotFound {
			t.Errorf("Get() error = %v, want ErrVersionNotFound", err)
		}
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		eligibility := policy.NewEligibilitySnapshot()
		eligibility.AddTool("explore", "read_file")

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			Description: "Original",
			Eligibility: eligibility,
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})

		result, _ := store.Get(ctx, 1)
		result.Description = "Modified"

		result2, _ := store.Get(ctx, 1)
		if result2.Description != "Original" {
			t.Error("Get() should return a copy, not reference")
		}
	})
}

func TestPolicyVersionStore_List(t *testing.T) {
	t.Parallel()

	t.Run("lists all versions sorted descending", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})
		store.Save(ctx, &policy.PolicyVersion{
			Version:     3,
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})
		store.Save(ctx, &policy.PolicyVersion{
			Version:     2,
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})

		results, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 3 {
			t.Errorf("List() count = %d, want 3", len(results))
		}

		// Should be sorted by version descending (3, 2, 1)
		if results[0].Version != 3 || results[1].Version != 2 || results[2].Version != 1 {
			t.Error("List() should be sorted by version descending")
		}
	})

	t.Run("returns empty for empty store", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		results, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("List() count = %d, want 0", len(results))
		}
	})
}

func TestPolicyVersionStore_GetByProposal(t *testing.T) {
	t.Parallel()

	t.Run("finds version by proposal ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			ProposalID:  "prop-1",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})
		store.Save(ctx, &policy.PolicyVersion{
			Version:     2,
			ProposalID:  "prop-2",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})

		result, err := store.GetByProposal(ctx, "prop-2")
		if err != nil {
			t.Fatalf("GetByProposal() error = %v", err)
		}
		if result.Version != 2 {
			t.Errorf("GetByProposal() Version = %d, want 2", result.Version)
		}
		if result.ProposalID != "prop-2" {
			t.Errorf("GetByProposal() ProposalID = %s, want prop-2", result.ProposalID)
		}
	})

	t.Run("returns error for non-existent proposal", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			ProposalID:  "prop-1",
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})

		_, err := store.GetByProposal(ctx, "nonexistent")
		if err != memory.ErrVersionNotFound {
			t.Errorf("GetByProposal() error = %v, want ErrVersionNotFound", err)
		}
	})
}

func TestPolicyVersionStore_DeepCopy(t *testing.T) {
	t.Parallel()

	t.Run("deep copies eligibility", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		eligibility := policy.NewEligibilitySnapshot()
		eligibility.AddTool("explore", "read_file")

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			Eligibility: eligibility,
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})

		// Modify original
		eligibility.AddTool("explore", "write_file")

		result, _ := store.Get(ctx, 1)
		if len(result.Eligibility.StateTools["explore"]) != 1 {
			t.Error("Save() should store a deep copy of eligibility")
		}
	})

	t.Run("deep copies transitions", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		transitions := policy.NewTransitionSnapshot()
		transitions.AddTransition("intake", "explore")

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: transitions,
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		})

		// Modify original
		transitions.AddTransition("explore", "decide")

		result, _ := store.Get(ctx, 1)
		if len(result.Transitions.Transitions["explore"]) != 0 {
			t.Error("Save() should store a deep copy of transitions")
		}
	})

	t.Run("deep copies budgets", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		budgets := policy.NewBudgetLimitsSnapshot()
		budgets.SetLimit("tool_calls", 100)

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     budgets,
			Approvals:   policy.NewApprovalSnapshot(),
		})

		// Modify original
		budgets.SetLimit("tool_calls", 200)

		result, _ := store.Get(ctx, 1)
		if result.Budgets.Limits["tool_calls"] != 100 {
			t.Error("Save() should store a deep copy of budgets")
		}
	})

	t.Run("deep copies approvals", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPolicyVersionStore()
		ctx := context.Background()

		approvals := policy.NewApprovalSnapshot()
		approvals.RequireApproval("delete_file")

		store.Save(ctx, &policy.PolicyVersion{
			Version:     1,
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   approvals,
		})

		// Modify original
		approvals.RequireApproval("exec_command")

		result, _ := store.Get(ctx, 1)
		if len(result.Approvals.RequiredTools) != 1 {
			t.Error("Save() should store a deep copy of approvals")
		}
	})
}
