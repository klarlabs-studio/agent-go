package proposal

import (
	"context"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/proposal"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// WorkflowService Tests

func TestNewWorkflowService(t *testing.T) {
	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()
	applier := NewPolicyApplier()

	workflow := NewWorkflowService(proposalStore, versionStore, applier)

	if workflow == nil {
		t.Fatal("expected non-nil workflow service")
	}
	if workflow.proposalStore == nil {
		t.Error("expected proposalStore to be set")
	}
	if workflow.versionStore == nil {
		t.Error("expected versionStore to be set")
	}
	if workflow.applier == nil {
		t.Error("expected applier to be set")
	}
}

func TestWorkflowService_CreateProposal(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop, err := workflow.CreateProposal(ctx, "Test Title", "Test Description", "test-creator")
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	if prop.ID == "" {
		t.Error("expected non-empty ID")
	}
	if prop.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %s", prop.Title)
	}
	if prop.Description != "Test Description" {
		t.Errorf("expected description 'Test Description', got %s", prop.Description)
	}
	if prop.CreatedBy != "test-creator" {
		t.Errorf("expected creator 'test-creator', got %s", prop.CreatedBy)
	}
	if prop.Status != proposal.ProposalStatusDraft {
		t.Errorf("expected draft status, got %s", prop.Status)
	}
}

func TestWorkflowService_AddChange(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop, _ := workflow.CreateProposal(ctx, "Test", "Test", "creator")

	change, _ := CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	err := workflow.AddChange(ctx, prop.ID, *change)
	if err != nil {
		t.Fatalf("failed to add change: %v", err)
	}

	// Verify change was added
	updated, _ := workflow.proposalStore.Get(ctx, prop.ID)
	if len(updated.Changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(updated.Changes))
	}
}

func TestWorkflowService_AddChange_ProposalNotFound(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	change, _ := CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	err := workflow.AddChange(ctx, "non-existent", *change)
	if err == nil {
		t.Error("expected error for non-existent proposal")
	}
}

func TestWorkflowService_Submit(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop, _ := workflow.CreateProposal(ctx, "Test", "Test", "creator")
	change, _ := CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	workflow.AddChange(ctx, prop.ID, *change)

	err := workflow.Submit(ctx, prop.ID, "submitter")
	if err != nil {
		t.Fatalf("failed to submit: %v", err)
	}

	updated, _ := workflow.proposalStore.Get(ctx, prop.ID)
	if updated.Status != proposal.ProposalStatusPendingReview {
		t.Errorf("expected pending_review status, got %s", updated.Status)
	}
	if updated.SubmittedBy != "submitter" {
		t.Errorf("expected submitter 'submitter', got %s", updated.SubmittedBy)
	}
}

func TestWorkflowService_Submit_NoChanges(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop, _ := workflow.CreateProposal(ctx, "Test", "Test", "creator")

	err := workflow.Submit(ctx, prop.ID, "submitter")
	if err != proposal.ErrNoChanges {
		t.Errorf("expected ErrNoChanges, got: %v", err)
	}
}

func TestWorkflowService_Approve(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop := createAndSubmitProposal(ctx, t, workflow)

	err := workflow.Approve(ctx, prop.ID, "approver", "LGTM")
	if err != nil {
		t.Fatalf("failed to approve: %v", err)
	}

	updated, _ := workflow.proposalStore.Get(ctx, prop.ID)
	if updated.Status != proposal.ProposalStatusApproved {
		t.Errorf("expected approved status, got %s", updated.Status)
	}
	if updated.ApprovedBy != "approver" {
		t.Errorf("expected approver 'approver', got %s", updated.ApprovedBy)
	}
	if updated.ApprovalReason != "LGTM" {
		t.Errorf("expected reason 'LGTM', got %s", updated.ApprovalReason)
	}
}

func TestWorkflowService_Approve_EmptyActor(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop := createAndSubmitProposal(ctx, t, workflow)

	err := workflow.Approve(ctx, prop.ID, "", "reason")
	if err != proposal.ErrHumanActorRequired {
		t.Errorf("expected ErrHumanActorRequired, got: %v", err)
	}
}

func TestWorkflowService_Reject(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop := createAndSubmitProposal(ctx, t, workflow)

	err := workflow.Reject(ctx, prop.ID, "rejector", "Not ready")
	if err != nil {
		t.Fatalf("failed to reject: %v", err)
	}

	updated, _ := workflow.proposalStore.Get(ctx, prop.ID)
	if updated.Status != proposal.ProposalStatusRejected {
		t.Errorf("expected rejected status, got %s", updated.Status)
	}
}

func TestWorkflowService_Apply(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	// Setup initial version
	initialVersion := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	initialVersion.Budgets.SetLimit("tool_calls", 100)
	workflow.versionStore.Save(ctx, initialVersion)

	prop := createAndApproveProposal(ctx, t, workflow)

	err := workflow.Apply(ctx, prop.ID)
	if err != nil {
		t.Fatalf("failed to apply: %v", err)
	}

	updated, _ := workflow.proposalStore.Get(ctx, prop.ID)
	if updated.Status != proposal.ProposalStatusApplied {
		t.Errorf("expected applied status, got %s", updated.Status)
	}
	if updated.PolicyVersionBefore != 0 {
		t.Errorf("expected PolicyVersionBefore=0, got %d", updated.PolicyVersionBefore)
	}
	if updated.PolicyVersionAfter != 1 {
		t.Errorf("expected PolicyVersionAfter=1, got %d", updated.PolicyVersionAfter)
	}

	// Verify new version was created
	newVersion, err := workflow.versionStore.GetCurrent(ctx)
	if err != nil {
		t.Fatalf("failed to get new version: %v", err)
	}
	if newVersion.Version != 1 {
		t.Errorf("expected version 1, got %d", newVersion.Version)
	}
}

func TestWorkflowService_Apply_NotApproved(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop := createAndSubmitProposal(ctx, t, workflow)

	err := workflow.Apply(ctx, prop.ID)
	if err != proposal.ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got: %v", err)
	}
}

func TestWorkflowService_Rollback(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	// Setup initial version
	initialVersion := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	initialVersion.Budgets.SetLimit("tool_calls", 100)
	workflow.versionStore.Save(ctx, initialVersion)

	// Create, approve, and apply
	prop := createAndApproveProposal(ctx, t, workflow)
	workflow.Apply(ctx, prop.ID)

	// Rollback
	err := workflow.Rollback(ctx, prop.ID, "Problem discovered")
	if err != nil {
		t.Fatalf("failed to rollback: %v", err)
	}

	updated, _ := workflow.proposalStore.Get(ctx, prop.ID)
	if updated.Status != proposal.ProposalStatusRolledBack {
		t.Errorf("expected rolled_back status, got %s", updated.Status)
	}
	if updated.RollbackReason != "Problem discovered" {
		t.Errorf("expected reason 'Problem discovered', got %s", updated.RollbackReason)
	}

	// Verify rollback version was created
	current, _ := workflow.versionStore.GetCurrent(ctx)
	if current.Version != 2 {
		t.Errorf("expected version 2 after rollback, got %d", current.Version)
	}
}

func TestWorkflowService_AddNote(t *testing.T) {
	ctx := context.Background()
	workflow := setupWorkflow()

	prop, _ := workflow.CreateProposal(ctx, "Test", "Test", "creator")

	err := workflow.AddNote(ctx, prop.ID, "author", "Note content")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	updated, _ := workflow.proposalStore.Get(ctx, prop.ID)
	if len(updated.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(updated.Notes))
	}
	if updated.Notes[0].Author != "author" {
		t.Errorf("expected author 'author', got %s", updated.Notes[0].Author)
	}
	if updated.Notes[0].Content != "Note content" {
		t.Errorf("expected content 'Note content', got %s", updated.Notes[0].Content)
	}
}

// PolicyApplier Tests

func TestNewPolicyApplier(t *testing.T) {
	applier := NewPolicyApplier()
	if applier == nil {
		t.Fatal("expected non-nil applier")
	}
}

func TestPolicyApplier_Apply_BudgetChange(t *testing.T) {
	ctx := context.Background()
	applier := NewPolicyApplier()

	current := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	current.Budgets.SetLimit("tool_calls", 100)

	change, _ := CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	changes := []proposal.PolicyChange{*change}

	newVersion, err := applier.Apply(ctx, current, changes)
	if err != nil {
		t.Fatalf("failed to apply: %v", err)
	}

	if newVersion.Version != 1 {
		t.Errorf("expected version 1, got %d", newVersion.Version)
	}
	if newVersion.Budgets.Limits["tool_calls"] != 200 {
		t.Errorf("expected tool_calls=200, got %d", newVersion.Budgets.Limits["tool_calls"])
	}
}

func TestPolicyApplier_Apply_EligibilityChange(t *testing.T) {
	ctx := context.Background()
	applier := NewPolicyApplier()

	current := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}

	change, _ := CreateEligibilityChange(agent.StateExplore, "read_file", true, "Allow read_file in explore")
	changes := []proposal.PolicyChange{*change}

	newVersion, err := applier.Apply(ctx, current, changes)
	if err != nil {
		t.Fatalf("failed to apply: %v", err)
	}

	tools := newVersion.Eligibility.StateTools[agent.StateExplore]
	found := false
	for _, tool := range tools {
		if tool == "read_file" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected read_file to be eligible in explore state")
	}
}

func TestPolicyApplier_Apply_TransitionChange(t *testing.T) {
	ctx := context.Background()
	applier := NewPolicyApplier()

	current := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}

	change, _ := CreateTransitionChange(agent.StateExplore, agent.StateAct, true, "Allow explore->act")
	changes := []proposal.PolicyChange{*change}

	newVersion, err := applier.Apply(ctx, current, changes)
	if err != nil {
		t.Fatalf("failed to apply: %v", err)
	}

	targets := newVersion.Transitions.Transitions[agent.StateExplore]
	found := false
	for _, target := range targets {
		if target == agent.StateAct {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected explore->act transition to be allowed")
	}
}

func TestPolicyApplier_Apply_ApprovalChange(t *testing.T) {
	ctx := context.Background()
	applier := NewPolicyApplier()

	current := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}

	change, _ := CreateApprovalChange("dangerous_tool", true, "Require approval")
	changes := []proposal.PolicyChange{*change}

	newVersion, err := applier.Apply(ctx, current, changes)
	if err != nil {
		t.Fatalf("failed to apply: %v", err)
	}

	found := false
	for _, tool := range newVersion.Approvals.RequiredTools {
		if tool == "dangerous_tool" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected dangerous_tool to require approval")
	}
}

func TestPolicyApplier_Apply_MultipleChanges(t *testing.T) {
	ctx := context.Background()
	applier := NewPolicyApplier()

	current := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	current.Budgets.SetLimit("tool_calls", 100)

	budgetChange, _ := CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	eligibilityChange, _ := CreateEligibilityChange(agent.StateExplore, "read_file", true, "Allow read_file")
	changes := []proposal.PolicyChange{*budgetChange, *eligibilityChange}

	newVersion, err := applier.Apply(ctx, current, changes)
	if err != nil {
		t.Fatalf("failed to apply: %v", err)
	}

	// Verify both changes applied
	if newVersion.Budgets.Limits["tool_calls"] != 200 {
		t.Errorf("expected tool_calls=200, got %d", newVersion.Budgets.Limits["tool_calls"])
	}

	tools := newVersion.Eligibility.StateTools[agent.StateExplore]
	found := false
	for _, tool := range tools {
		if tool == "read_file" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected read_file to be eligible in explore state")
	}
}

// Change Creation Helper Tests

func TestCreateBudgetChange(t *testing.T) {
	change, err := CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	if err != nil {
		t.Fatalf("failed to create budget change: %v", err)
	}

	if change.Type != proposal.ChangeTypeBudget {
		t.Errorf("expected type budget, got %s", change.Type)
	}
	if change.Target != "tool_calls" {
		t.Errorf("expected target 'tool_calls', got %s", change.Target)
	}
	if change.Description != "Increase budget" {
		t.Errorf("expected description 'Increase budget', got %s", change.Description)
	}
}

func TestCreateEligibilityChange(t *testing.T) {
	change, err := CreateEligibilityChange(agent.StateExplore, "read_file", true, "Allow read_file")
	if err != nil {
		t.Fatalf("failed to create eligibility change: %v", err)
	}

	if change.Type != proposal.ChangeTypeEligibility {
		t.Errorf("expected type eligibility, got %s", change.Type)
	}
	if change.Target != "read_file" {
		t.Errorf("expected target 'read_file', got %s", change.Target)
	}
}

func TestCreateTransitionChange(t *testing.T) {
	change, err := CreateTransitionChange(agent.StateExplore, agent.StateAct, true, "Allow transition")
	if err != nil {
		t.Fatalf("failed to create transition change: %v", err)
	}

	if change.Type != proposal.ChangeTypeTransition {
		t.Errorf("expected type transition, got %s", change.Type)
	}
	if change.Target != "explore->act" {
		t.Errorf("expected target 'explore->act', got %s", change.Target)
	}
}

func TestCreateApprovalChange(t *testing.T) {
	change, err := CreateApprovalChange("dangerous_tool", true, "Require approval")
	if err != nil {
		t.Fatalf("failed to create approval change: %v", err)
	}

	if change.Type != proposal.ChangeTypeApproval {
		t.Errorf("expected type approval, got %s", change.Type)
	}
	if change.Target != "dangerous_tool" {
		t.Errorf("expected target 'dangerous_tool', got %s", change.Target)
	}
}

// Copy Functions Tests

func TestCopyEligibilitySnapshot(t *testing.T) {
	src := policy.NewEligibilitySnapshot()
	src.AddTool(agent.StateExplore, "read_file")
	src.AddTool(agent.StateExplore, "list_files")
	src.AddTool(agent.StateAct, "write_file")

	dst := copyEligibilitySnapshot(src)

	// Verify copy has same data
	if len(dst.StateTools[agent.StateExplore]) != 2 {
		t.Errorf("expected 2 tools in explore, got %d", len(dst.StateTools[agent.StateExplore]))
	}
	if len(dst.StateTools[agent.StateAct]) != 1 {
		t.Errorf("expected 1 tool in act, got %d", len(dst.StateTools[agent.StateAct]))
	}

	// Verify it's a true copy (modifying dst doesn't affect src)
	dst.AddTool(agent.StateExplore, "new_tool")
	if len(src.StateTools[agent.StateExplore]) != 2 {
		t.Error("modifying copy should not affect source")
	}
}

func TestCopyTransitionSnapshot(t *testing.T) {
	src := policy.NewTransitionSnapshot()
	src.AddTransition(agent.StateIntake, agent.StateExplore)
	src.AddTransition(agent.StateExplore, agent.StateDecide)

	dst := copyTransitionSnapshot(src)

	if len(dst.Transitions[agent.StateIntake]) != 1 {
		t.Errorf("expected 1 transition from intake, got %d", len(dst.Transitions[agent.StateIntake]))
	}
	if len(dst.Transitions[agent.StateExplore]) != 1 {
		t.Errorf("expected 1 transition from explore, got %d", len(dst.Transitions[agent.StateExplore]))
	}
}

func TestCopyBudgetSnapshot(t *testing.T) {
	src := policy.NewBudgetLimitsSnapshot()
	src.SetLimit("tool_calls", 100)
	src.SetLimit("api_calls", 50)

	dst := copyBudgetSnapshot(src)

	if dst.Limits["tool_calls"] != 100 {
		t.Errorf("expected tool_calls=100, got %d", dst.Limits["tool_calls"])
	}
	if dst.Limits["api_calls"] != 50 {
		t.Errorf("expected api_calls=50, got %d", dst.Limits["api_calls"])
	}

	// Verify it's a true copy
	dst.SetLimit("tool_calls", 200)
	if src.Limits["tool_calls"] != 100 {
		t.Error("modifying copy should not affect source")
	}
}

func TestCopyApprovalSnapshot(t *testing.T) {
	src := policy.NewApprovalSnapshot()
	src.RequireApproval("dangerous_tool")
	src.RequireApproval("expensive_tool")

	dst := copyApprovalSnapshot(src)

	if len(dst.RequiredTools) != 2 {
		t.Errorf("expected 2 required tools, got %d", len(dst.RequiredTools))
	}
}

// Helper functions

func setupWorkflow() *WorkflowService {
	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()
	applier := NewPolicyApplier()
	return NewWorkflowService(proposalStore, versionStore, applier)
}

func createAndSubmitProposal(ctx context.Context, t *testing.T, workflow *WorkflowService) *proposal.Proposal {
	t.Helper()
	prop, err := workflow.CreateProposal(ctx, "Test Proposal", "Description", "creator")
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	change, _ := CreateBudgetChange("tool_calls", 100, 200, "Increase budget")
	workflow.AddChange(ctx, prop.ID, *change)
	workflow.Submit(ctx, prop.ID, "submitter")

	return prop
}

func createAndApproveProposal(ctx context.Context, t *testing.T, workflow *WorkflowService) *proposal.Proposal {
	t.Helper()
	prop := createAndSubmitProposal(ctx, t, workflow)
	workflow.Approve(ctx, prop.ID, "approver", "LGTM")
	return prop
}
