package test

import (
	"context"
	"testing"

	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/proposal"
	infraProposal "go.klarlabs.de/agent/infrastructure/proposal"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// Horizon 3 Design Invariants:
// 1. No unsupervised modification - All policy changes require explicit human approval
// 2. Audit trail - Every proposal action recorded with actor, timestamp, reason
// 3. Rollback capability - Any applied change can be rolled back
// 4. Suggestion-only patterns - Patterns generate suggestions, never direct changes
// 5. Version immutability - Policy versions are append-only
// 6. Human actor requirement - Approve requires non-empty actor

func TestHorizon3_HumanApprovalRequired(t *testing.T) {
	// Invariant 1 & 6: All policy changes require explicit human approval
	ctx := context.Background()

	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()
	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	// Create a proposal
	prop, err := workflow.CreateProposal(ctx, "Test Proposal", "Test description", "test-creator")
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	// Add a change
	change, err := infraProposal.CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	if err != nil {
		t.Fatalf("failed to create change: %v", err)
	}
	if err := workflow.AddChange(ctx, prop.ID, *change); err != nil {
		t.Fatalf("failed to add change: %v", err)
	}

	// Submit for review
	if err := workflow.Submit(ctx, prop.ID, "test-submitter"); err != nil {
		t.Fatalf("failed to submit proposal: %v", err)
	}

	// Attempt to approve with empty actor - should fail
	err = workflow.Approve(ctx, prop.ID, "", "auto-approved")
	if err != proposal.ErrHumanActorRequired {
		t.Errorf("expected ErrHumanActorRequired, got: %v", err)
	}

	// Approve with valid human actor - should succeed
	if err := workflow.Approve(ctx, prop.ID, "human-approver", "Looks good"); err != nil {
		t.Fatalf("failed to approve with human actor: %v", err)
	}
}

func TestHorizon3_AuditTrail(t *testing.T) {
	// Invariant 2: Every proposal action recorded with actor, timestamp, reason
	ctx := context.Background()

	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()
	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	// Create a proposal
	prop, err := workflow.CreateProposal(ctx, "Audit Test", "Test audit trail", "alice")
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	// Verify creator is recorded
	if prop.CreatedBy != "alice" {
		t.Errorf("expected CreatedBy=alice, got %s", prop.CreatedBy)
	}
	if prop.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Add change and submit
	change, err := infraProposal.CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	if err != nil {
		t.Fatalf("failed to create change: %v", err)
	}
	if err := workflow.AddChange(ctx, prop.ID, *change); err != nil {
		t.Fatalf("failed to add change: %v", err)
	}
	if err := workflow.Submit(ctx, prop.ID, "bob"); err != nil {
		t.Fatalf("failed to submit: %v", err)
	}

	// Reload and verify submission is recorded
	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.SubmittedBy != "bob" {
		t.Errorf("expected SubmittedBy=bob, got %s", prop.SubmittedBy)
	}
	if prop.SubmittedAt == nil || prop.SubmittedAt.IsZero() {
		t.Error("SubmittedAt should not be nil or zero")
	}

	// Approve
	if err := workflow.Approve(ctx, prop.ID, "carol", "LGTM"); err != nil {
		t.Fatalf("failed to approve: %v", err)
	}

	// Reload and verify approval is recorded
	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.ApprovedBy != "carol" {
		t.Errorf("expected ApprovedBy=carol, got %s", prop.ApprovedBy)
	}
	if prop.ApprovalReason != "LGTM" {
		t.Errorf("expected ApprovalReason=LGTM, got %s", prop.ApprovalReason)
	}
	if prop.ApprovedAt == nil || prop.ApprovedAt.IsZero() {
		t.Error("ApprovedAt should not be nil or zero")
	}
}

func TestHorizon3_RollbackCapability(t *testing.T) {
	// Invariant 3: Any applied change can be rolled back
	ctx := context.Background()

	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()
	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	// Create initial version
	initialVersion := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	initialVersion.Budgets.SetLimit("tool_calls", 100)
	if err := versionStore.Save(ctx, initialVersion); err != nil {
		t.Fatalf("failed to save initial version: %v", err)
	}

	// Create, submit, approve, and apply a proposal
	prop, _ := workflow.CreateProposal(ctx, "Rollback Test", "Test rollback", "creator")
	change, _ := infraProposal.CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	workflow.AddChange(ctx, prop.ID, *change)
	workflow.Submit(ctx, prop.ID, "submitter")
	workflow.Approve(ctx, prop.ID, "approver", "approved")
	if err := workflow.Apply(ctx, prop.ID); err != nil {
		t.Fatalf("failed to apply: %v", err)
	}

	// Verify proposal is applied
	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.Status != proposal.ProposalStatusApplied {
		t.Fatalf("expected status Applied, got %s", prop.Status)
	}

	// Verify new version exists
	currentVersion, err := versionStore.GetCurrent(ctx)
	if err != nil {
		t.Fatalf("failed to get current version: %v", err)
	}
	if currentVersion.Version != 1 {
		t.Errorf("expected version 1, got %d", currentVersion.Version)
	}

	// Rollback
	if err := workflow.Rollback(ctx, prop.ID, "Budget increase caused issues"); err != nil {
		t.Fatalf("failed to rollback: %v", err)
	}

	// Verify proposal is rolled back
	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.Status != proposal.ProposalStatusRolledBack {
		t.Errorf("expected status RolledBack, got %s", prop.Status)
	}
	if prop.RollbackReason != "Budget increase caused issues" {
		t.Errorf("expected rollback reason, got %s", prop.RollbackReason)
	}

	// Verify rollback version exists
	rollbackVersion, err := versionStore.GetCurrent(ctx)
	if err != nil {
		t.Fatalf("failed to get rollback version: %v", err)
	}
	if rollbackVersion.Version != 2 {
		t.Errorf("expected version 2 after rollback, got %d", rollbackVersion.Version)
	}
}

func TestHorizon3_VersionImmutability(t *testing.T) {
	// Invariant 5: Policy versions are append-only
	ctx := context.Background()

	versionStore := memory.NewPolicyVersionStore()

	// Create and save version 0
	v0 := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	v0.Budgets.SetLimit("tool_calls", 100)
	if err := versionStore.Save(ctx, v0); err != nil {
		t.Fatalf("failed to save v0: %v", err)
	}

	// Create and save version 1
	v1 := &policy.PolicyVersion{
		Version:     1,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	v1.Budgets.SetLimit("tool_calls", 200)
	if err := versionStore.Save(ctx, v1); err != nil {
		t.Fatalf("failed to save v1: %v", err)
	}

	// Verify both versions can be retrieved
	retrieved0, err := versionStore.Get(ctx, 0)
	if err != nil {
		t.Fatalf("failed to get v0: %v", err)
	}
	if retrieved0.Budgets.Limits["tool_calls"] != 100 {
		t.Errorf("v0 budget should be 100, got %d", retrieved0.Budgets.Limits["tool_calls"])
	}

	retrieved1, err := versionStore.Get(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get v1: %v", err)
	}
	if retrieved1.Budgets.Limits["tool_calls"] != 200 {
		t.Errorf("v1 budget should be 200, got %d", retrieved1.Budgets.Limits["tool_calls"])
	}

	// Verify current version is the latest
	current, err := versionStore.GetCurrent(ctx)
	if err != nil {
		t.Fatalf("failed to get current: %v", err)
	}
	if current.Version != 1 {
		t.Errorf("current version should be 1, got %d", current.Version)
	}

	// List all versions
	versions, err := versionStore.List(ctx)
	if err != nil {
		t.Fatalf("failed to list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestHorizon3_ProposalStatusTransitions(t *testing.T) {
	// Test valid and invalid status transitions
	ctx := context.Background()

	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()
	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	// Create a proposal
	prop, _ := workflow.CreateProposal(ctx, "Status Test", "Test status transitions", "creator")
	change, _ := infraProposal.CreateBudgetChange("tool_calls", 100, 200, "change")
	workflow.AddChange(ctx, prop.ID, *change)

	// Cannot approve a draft
	if err := workflow.Approve(ctx, prop.ID, "approver", "reason"); err == nil {
		t.Error("should not be able to approve a draft")
	}

	// Cannot apply a draft
	if err := workflow.Apply(ctx, prop.ID); err == nil {
		t.Error("should not be able to apply a draft")
	}

	// Submit
	workflow.Submit(ctx, prop.ID, "submitter")

	// Cannot apply a pending_review
	if err := workflow.Apply(ctx, prop.ID); err == nil {
		t.Error("should not be able to apply a pending_review proposal")
	}

	// Approve
	workflow.Approve(ctx, prop.ID, "approver", "approved")

	// Cannot submit an approved proposal
	if err := workflow.Submit(ctx, prop.ID, "submitter"); err == nil {
		t.Error("should not be able to submit an approved proposal")
	}
}

func TestHorizon3_NoChangesProposal(t *testing.T) {
	// Cannot submit a proposal with no changes
	ctx := context.Background()

	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()
	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	prop, _ := workflow.CreateProposal(ctx, "Empty Test", "No changes", "creator")

	// Try to submit without any changes
	err := workflow.Submit(ctx, prop.ID, "submitter")
	if err != proposal.ErrNoChanges {
		t.Errorf("expected ErrNoChanges, got: %v", err)
	}
}
