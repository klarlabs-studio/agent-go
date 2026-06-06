package api_test

import (
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewProposalStore(t *testing.T) {
	t.Parallel()

	t.Run("creates in-memory proposal store", func(t *testing.T) {
		t.Parallel()

		store := api.NewProposalStore()

		if store == nil {
			t.Fatal("NewProposalStore() returned nil")
		}
	})
}

func TestNewPolicyVersionStore(t *testing.T) {
	t.Parallel()

	t.Run("creates in-memory policy version store", func(t *testing.T) {
		t.Parallel()

		store := api.NewPolicyVersionStore()

		if store == nil {
			t.Fatal("NewPolicyVersionStore() returned nil")
		}
	})
}

func TestNewWorkflowService(t *testing.T) {
	t.Parallel()

	t.Run("creates workflow service", func(t *testing.T) {
		t.Parallel()

		proposalStore := api.NewProposalStore()
		versionStore := api.NewPolicyVersionStore()
		applier := api.NewPolicyApplier()

		service := api.NewWorkflowService(proposalStore, versionStore, applier)

		if service == nil {
			t.Fatal("NewWorkflowService() returned nil")
		}
	})
}

func TestNewPolicyApplier(t *testing.T) {
	t.Parallel()

	t.Run("creates policy applier", func(t *testing.T) {
		t.Parallel()

		applier := api.NewPolicyApplier()

		if applier == nil {
			t.Fatal("NewPolicyApplier() returned nil")
		}
	})
}

func TestCreateEligibilityChange(t *testing.T) {
	t.Parallel()

	t.Run("creates eligibility change to allow", func(t *testing.T) {
		t.Parallel()

		change, err := api.CreateEligibilityChange(
			agent.StateExplore,
			"read_file",
			true,
			"Allow read_file in explore state",
		)

		if err != nil {
			t.Fatalf("CreateEligibilityChange() error = %v", err)
		}
		if change == nil {
			t.Fatal("CreateEligibilityChange() returned nil")
		}
		if change.Type != api.ChangeTypeEligibility {
			t.Errorf("change.Type = %s, want eligibility", change.Type)
		}
	})

	t.Run("creates eligibility change to deny", func(t *testing.T) {
		t.Parallel()

		change, err := api.CreateEligibilityChange(
			agent.StateAct,
			"delete_file",
			false,
			"Disallow delete_file in act state",
		)

		if err != nil {
			t.Fatalf("CreateEligibilityChange() error = %v", err)
		}
		if change == nil {
			t.Fatal("CreateEligibilityChange() returned nil")
		}
	})
}

func TestCreateTransitionChange(t *testing.T) {
	t.Parallel()

	t.Run("creates transition change to allow", func(t *testing.T) {
		t.Parallel()

		change, err := api.CreateTransitionChange(
			agent.StateExplore,
			agent.StateAct,
			true,
			"Allow transition from explore to act",
		)

		if err != nil {
			t.Fatalf("CreateTransitionChange() error = %v", err)
		}
		if change == nil {
			t.Fatal("CreateTransitionChange() returned nil")
		}
		if change.Type != api.ChangeTypeTransition {
			t.Errorf("change.Type = %s, want transition", change.Type)
		}
	})

	t.Run("creates transition change to deny", func(t *testing.T) {
		t.Parallel()

		change, err := api.CreateTransitionChange(
			agent.StateIntake,
			agent.StateDone,
			false,
			"Disallow direct transition from intake to done",
		)

		if err != nil {
			t.Fatalf("CreateTransitionChange() error = %v", err)
		}
		if change == nil {
			t.Fatal("CreateTransitionChange() returned nil")
		}
	})
}

func TestCreateBudgetChange(t *testing.T) {
	t.Parallel()

	t.Run("creates budget increase change", func(t *testing.T) {
		t.Parallel()

		change, err := api.CreateBudgetChange(
			"tool_calls",
			100,
			200,
			"Increase tool_calls budget",
		)

		if err != nil {
			t.Fatalf("CreateBudgetChange() error = %v", err)
		}
		if change == nil {
			t.Fatal("CreateBudgetChange() returned nil")
		}
		if change.Type != api.ChangeTypeBudget {
			t.Errorf("change.Type = %s, want budget", change.Type)
		}
	})

	t.Run("creates budget decrease change", func(t *testing.T) {
		t.Parallel()

		change, err := api.CreateBudgetChange(
			"api_calls",
			50,
			25,
			"Decrease api_calls budget",
		)

		if err != nil {
			t.Fatalf("CreateBudgetChange() error = %v", err)
		}
		if change == nil {
			t.Fatal("CreateBudgetChange() returned nil")
		}
	})
}

func TestCreateApprovalChange(t *testing.T) {
	t.Parallel()

	t.Run("creates approval required change", func(t *testing.T) {
		t.Parallel()

		change, err := api.CreateApprovalChange(
			"delete_file",
			true,
			"Require approval for delete_file",
		)

		if err != nil {
			t.Fatalf("CreateApprovalChange() error = %v", err)
		}
		if change == nil {
			t.Fatal("CreateApprovalChange() returned nil")
		}
		if change.Type != api.ChangeTypeApproval {
			t.Errorf("change.Type = %s, want approval", change.Type)
		}
	})

	t.Run("creates approval not required change", func(t *testing.T) {
		t.Parallel()

		change, err := api.CreateApprovalChange(
			"read_file",
			false,
			"Remove approval requirement for read_file",
		)

		if err != nil {
			t.Fatalf("CreateApprovalChange() error = %v", err)
		}
		if change == nil {
			t.Fatal("CreateApprovalChange() returned nil")
		}
	})
}

func TestProposalStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   api.ProposalStatus
		expected string
	}{
		{"ProposalStatusDraft", api.ProposalStatusDraft, "draft"},
		{"ProposalStatusPendingReview", api.ProposalStatusPendingReview, "pending_review"},
		{"ProposalStatusApproved", api.ProposalStatusApproved, "approved"},
		{"ProposalStatusRejected", api.ProposalStatusRejected, "rejected"},
		{"ProposalStatusApplied", api.ProposalStatusApplied, "applied"},
		{"ProposalStatusRolledBack", api.ProposalStatusRolledBack, "rolled_back"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.status) != tt.expected {
				t.Errorf("ProposalStatus = %s, want %s", tt.status, tt.expected)
			}
		})
	}
}

func TestChangeTypeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ct       api.ChangeType
		expected string
	}{
		{"ChangeTypeEligibility", api.ChangeTypeEligibility, "eligibility"},
		{"ChangeTypeTransition", api.ChangeTypeTransition, "transition"},
		{"ChangeTypeBudget", api.ChangeTypeBudget, "budget"},
		{"ChangeTypeApproval", api.ChangeTypeApproval, "approval"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.ct) != tt.expected {
				t.Errorf("ChangeType = %s, want %s", tt.ct, tt.expected)
			}
		})
	}
}
