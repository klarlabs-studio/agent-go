package proposal

import (
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
)

// Proposal Creation Tests

func TestNewProposal_CreatesValidProposal(t *testing.T) {
	p := NewProposal("test title", "test description", "creator")

	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.Title != "test title" {
		t.Errorf("expected title 'test title', got %s", p.Title)
	}
	if p.Description != "test description" {
		t.Errorf("expected description 'test description', got %s", p.Description)
	}
	if p.CreatedBy != "creator" {
		t.Errorf("expected CreatedBy 'creator', got %s", p.CreatedBy)
	}
	if p.Status != ProposalStatusDraft {
		t.Errorf("expected status Draft, got %s", p.Status)
	}
	if p.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if len(p.Changes) != 0 {
		t.Errorf("expected empty Changes, got %d", len(p.Changes))
	}
	if len(p.Evidence) != 0 {
		t.Errorf("expected empty Evidence, got %d", len(p.Evidence))
	}
	if len(p.Notes) != 0 {
		t.Errorf("expected empty Notes, got %d", len(p.Notes))
	}
	if p.Metadata == nil {
		t.Error("expected Metadata to be initialized")
	}
}

func TestNewProposal_GeneratesUniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		p := NewProposal("test", "test", "creator")
		if ids[p.ID] {
			t.Errorf("duplicate ID generated: %s", p.ID)
		}
		ids[p.ID] = true
	}
}

// Change Management Tests

func TestAddChange_SucceedsInDraft(t *testing.T) {
	p := NewProposal("test", "test", "creator")

	change := PolicyChange{
		Type:        ChangeTypeEligibility,
		Target:      "read_file",
		Description: "Allow read_file in explore state",
	}
	err := p.AddChange(change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(p.Changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(p.Changes))
	}
}

func TestAddChange_FailsIfNotDraft(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	_ = p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool1"})
	_ = p.Submit("submitter")

	err := p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool2"})
	if err != ErrCannotModifyNonDraft {
		t.Errorf("expected ErrCannotModifyNonDraft, got %v", err)
	}
}

func TestAddEvidence_AddsEvidence(t *testing.T) {
	p := NewProposal("test", "test", "creator")

	err := p.AddEvidence("pattern", "Tool frequently used together", map[string]string{"pattern_id": "p123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(p.Evidence) != 1 {
		t.Errorf("expected 1 evidence entry, got %d", len(p.Evidence))
	}
	if p.Evidence[0].Type != "pattern" {
		t.Errorf("expected evidence type 'pattern', got %s", p.Evidence[0].Type)
	}
	if p.Evidence[0].AddedAt.IsZero() {
		t.Error("expected AddedAt to be set")
	}
}

func TestAddEvidence_HandlesUnserializableData(t *testing.T) {
	p := NewProposal("test", "test", "creator")

	err := p.AddEvidence("bad", "bad data", make(chan int))
	if err == nil {
		t.Error("expected error for unserializable data")
	}
}

func TestAddNote_AddsNote(t *testing.T) {
	p := NewProposal("test", "test", "creator")

	p.AddNote("alice", "This looks good to me")

	if len(p.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(p.Notes))
	}
	if p.Notes[0].Author != "alice" {
		t.Errorf("expected author 'alice', got %s", p.Notes[0].Author)
	}
	if p.Notes[0].Content != "This looks good to me" {
		t.Errorf("expected content, got %s", p.Notes[0].Content)
	}
	if p.Notes[0].CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

// Status Transition Tests

func TestSubmit_TransitionsFromDraft(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	_ = p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})

	err := p.Submit("submitter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Status != ProposalStatusPendingReview {
		t.Errorf("expected status PendingReview, got %s", p.Status)
	}
	if p.SubmittedBy != "submitter" {
		t.Errorf("expected SubmittedBy 'submitter', got %s", p.SubmittedBy)
	}
	if p.SubmittedAt == nil || p.SubmittedAt.IsZero() {
		t.Error("expected SubmittedAt to be set")
	}
}

func TestSubmit_FailsWithNoChanges(t *testing.T) {
	p := NewProposal("test", "test", "creator")

	err := p.Submit("submitter")
	if err != ErrNoChanges {
		t.Errorf("expected ErrNoChanges, got %v", err)
	}
}

func TestSubmit_FailsFromNonDraftStatus(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	_ = p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	_ = p.Submit("submitter")
	_ = p.Approve("approver", "approved")

	err := p.Submit("another")
	if err != ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestApprove_TransitionsFromPendingReview(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	_ = p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	_ = p.Submit("submitter")

	err := p.Approve("approver", "Looks good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Status != ProposalStatusApproved {
		t.Errorf("expected status Approved, got %s", p.Status)
	}
	if p.ApprovedBy != "approver" {
		t.Errorf("expected ApprovedBy 'approver', got %s", p.ApprovedBy)
	}
	if p.ApprovalReason != "Looks good" {
		t.Errorf("expected ApprovalReason 'Looks good', got %s", p.ApprovalReason)
	}
	if p.ApprovedAt == nil || p.ApprovedAt.IsZero() {
		t.Error("expected ApprovedAt to be set")
	}
}

func TestApprove_RequiresHumanActor(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")

	err := p.Approve("", "reason")
	if err != ErrHumanActorRequired {
		t.Errorf("expected ErrHumanActorRequired, got %v", err)
	}
}

func TestApprove_FailsFromNonPendingStatus(t *testing.T) {
	p := NewProposal("test", "test", "creator")

	err := p.Approve("approver", "reason")
	if err != ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestReject_TransitionsFromPendingReview(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")

	err := p.Reject("rejector", "Not needed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Status != ProposalStatusRejected {
		t.Errorf("expected status Rejected, got %s", p.Status)
	}
	if p.RejectedBy != "rejector" {
		t.Errorf("expected RejectedBy 'rejector', got %s", p.RejectedBy)
	}
	if p.RejectionReason != "Not needed" {
		t.Errorf("expected RejectionReason 'Not needed', got %s", p.RejectionReason)
	}
}

func TestReject_RequiresHumanActor(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")

	err := p.Reject("", "reason")
	if err != ErrHumanActorRequired {
		t.Errorf("expected ErrHumanActorRequired, got %v", err)
	}
}

func TestApply_TransitionsFromApproved(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	_ = p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	_ = p.Submit("submitter")
	_ = p.Approve("approver", "approved")

	err := p.Apply(0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Status != ProposalStatusApplied {
		t.Errorf("expected status Applied, got %s", p.Status)
	}
	if p.PolicyVersionBefore != 0 {
		t.Errorf("expected PolicyVersionBefore 0, got %d", p.PolicyVersionBefore)
	}
	if p.PolicyVersionAfter != 1 {
		t.Errorf("expected PolicyVersionAfter 1, got %d", p.PolicyVersionAfter)
	}
	if p.AppliedAt == nil || p.AppliedAt.IsZero() {
		t.Error("expected AppliedAt to be set")
	}
}

func TestApply_FailsFromNonApprovedStatus(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")

	err := p.Apply(0, 1)
	if err != ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestRollback_TransitionsFromApplied(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	_ = p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	_ = p.Submit("submitter")
	_ = p.Approve("approver", "approved")
	_ = p.Apply(0, 1)

	err := p.Rollback("Caused issues")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Status != ProposalStatusRolledBack {
		t.Errorf("expected status RolledBack, got %s", p.Status)
	}
	if p.RollbackReason != "Caused issues" {
		t.Errorf("expected RollbackReason 'Caused issues', got %s", p.RollbackReason)
	}
	if p.RolledBackAt == nil || p.RolledBackAt.IsZero() {
		t.Error("expected RolledBackAt to be set")
	}
}

func TestRollback_FailsFromNonAppliedStatus(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")
	p.Approve("approver", "approved")

	err := p.Rollback("reason")
	if err != ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestReturnToDraft_FromRejected(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")
	p.Reject("rejector", "reason")

	err := p.ReturnToDraft()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Status != ProposalStatusDraft {
		t.Errorf("expected status Draft, got %s", p.Status)
	}
}

func TestReturnToDraft_FromRolledBack(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")
	p.Approve("approver", "approved")
	p.Apply(0, 1)
	p.Rollback("reason")

	err := p.ReturnToDraft()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Status != ProposalStatusDraft {
		t.Errorf("expected status Draft, got %s", p.Status)
	}
}

func TestReturnToDraft_FailsFromApplied(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")
	p.Approve("approver", "approved")
	p.Apply(0, 1)

	err := p.ReturnToDraft()
	if err != ErrInvalidStatusTransition {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

// Helper Method Tests

func TestCanBeModified_TrueForDraft(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	if !p.CanBeModified() {
		t.Error("expected CanBeModified to be true for draft")
	}
}

func TestCanBeModified_FalseForNonDraft(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")

	if p.CanBeModified() {
		t.Error("expected CanBeModified to be false for pending_review")
	}
}

func TestIsApplied(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")
	p.Approve("approver", "approved")

	if p.IsApplied() {
		t.Error("expected IsApplied to be false before apply")
	}

	p.Apply(0, 1)

	if !p.IsApplied() {
		t.Error("expected IsApplied to be true after apply")
	}
}

func TestIsRolledBack(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})
	p.Submit("submitter")
	p.Approve("approver", "approved")
	p.Apply(0, 1)

	if p.IsRolledBack() {
		t.Error("expected IsRolledBack to be false before rollback")
	}

	p.Rollback("reason")

	if !p.IsRolledBack() {
		t.Error("expected IsRolledBack to be true after rollback")
	}
}

// Status Tests

func TestProposalStatus_CanTransitionTo_ValidTransitions(t *testing.T) {
	testCases := []struct {
		from  ProposalStatus
		to    ProposalStatus
		valid bool
	}{
		{ProposalStatusDraft, ProposalStatusPendingReview, true},
		{ProposalStatusDraft, ProposalStatusRejected, true},
		{ProposalStatusDraft, ProposalStatusApproved, false},
		{ProposalStatusPendingReview, ProposalStatusApproved, true},
		{ProposalStatusPendingReview, ProposalStatusRejected, true},
		{ProposalStatusPendingReview, ProposalStatusDraft, true},
		{ProposalStatusApproved, ProposalStatusApplied, true},
		{ProposalStatusApproved, ProposalStatusRejected, true},
		{ProposalStatusApplied, ProposalStatusRolledBack, true},
		{ProposalStatusApplied, ProposalStatusDraft, false},
		{ProposalStatusRejected, ProposalStatusDraft, true},
		{ProposalStatusRolledBack, ProposalStatusDraft, true},
	}

	for _, tc := range testCases {
		result := tc.from.CanTransitionTo(tc.to)
		if result != tc.valid {
			t.Errorf("%s -> %s: expected %v, got %v", tc.from, tc.to, tc.valid, result)
		}
	}
}

func TestProposalStatus_IsTerminal(t *testing.T) {
	testCases := []struct {
		status   ProposalStatus
		terminal bool
	}{
		{ProposalStatusDraft, false},
		{ProposalStatusPendingReview, false},
		{ProposalStatusApproved, false},
		{ProposalStatusApplied, false},
		{ProposalStatusRejected, true},
		{ProposalStatusRolledBack, true},
	}

	for _, tc := range testCases {
		result := tc.status.IsTerminal()
		if result != tc.terminal {
			t.Errorf("%s.IsTerminal(): expected %v, got %v", tc.status, tc.terminal, result)
		}
	}
}

func TestProposalStatus_IsActive(t *testing.T) {
	testCases := []struct {
		status ProposalStatus
		active bool
	}{
		{ProposalStatusDraft, true},
		{ProposalStatusPendingReview, true},
		{ProposalStatusApproved, true},
		{ProposalStatusApplied, false},
		{ProposalStatusRejected, false},
		{ProposalStatusRolledBack, false},
	}

	for _, tc := range testCases {
		result := tc.status.IsActive()
		if result != tc.active {
			t.Errorf("%s.IsActive(): expected %v, got %v", tc.status, tc.active, result)
		}
	}
}

func TestProposalStatus_RequiresHumanAction(t *testing.T) {
	testCases := []struct {
		status   ProposalStatus
		requires bool
	}{
		{ProposalStatusDraft, false},
		{ProposalStatusPendingReview, true},
		{ProposalStatusApproved, false},
		{ProposalStatusApplied, false},
		{ProposalStatusRejected, false},
		{ProposalStatusRolledBack, false},
	}

	for _, tc := range testCases {
		result := tc.status.RequiresHumanAction()
		if result != tc.requires {
			t.Errorf("%s.RequiresHumanAction(): expected %v, got %v", tc.status, tc.requires, result)
		}
	}
}

func TestProposalStatus_CanTransitionTo_UnknownStatus(t *testing.T) {
	unknown := ProposalStatus("unknown")
	if unknown.CanTransitionTo(ProposalStatusDraft) {
		t.Error("expected unknown status to not be able to transition")
	}
}

// PolicyChange Tests

func TestNewPolicyChange_CreatesValidChange(t *testing.T) {
	change, err := NewPolicyChange(
		ChangeTypeEligibility,
		"read_file",
		"Allow read_file in explore",
		EligibilityChange{State: agent.StateExplore, ToolName: "read_file", Allowed: false},
		EligibilityChange{State: agent.StateExplore, ToolName: "read_file", Allowed: true},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if change.Type != ChangeTypeEligibility {
		t.Errorf("expected type Eligibility, got %s", change.Type)
	}
	if change.Target != "read_file" {
		t.Errorf("expected target 'read_file', got %s", change.Target)
	}
	if change.Before == nil {
		t.Error("expected Before to be set")
	}
	if change.After == nil {
		t.Error("expected After to be set")
	}
}

func TestNewPolicyChange_HandlesNilBefore(t *testing.T) {
	change, err := NewPolicyChange(
		ChangeTypeEligibility,
		"tool",
		"Add new eligibility",
		nil,
		EligibilityChange{State: agent.StateExplore, ToolName: "tool", Allowed: true},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if change.Before != nil {
		t.Error("expected Before to be nil")
	}
}

func TestNewPolicyChange_HandlesUnserializableBefore(t *testing.T) {
	_, err := NewPolicyChange(
		ChangeTypeEligibility,
		"tool",
		"desc",
		make(chan int),
		"after",
	)
	if err == nil {
		t.Error("expected error for unserializable before")
	}
}

func TestNewPolicyChange_HandlesUnserializableAfter(t *testing.T) {
	_, err := NewPolicyChange(
		ChangeTypeEligibility,
		"tool",
		"desc",
		nil,
		make(chan int),
	)
	if err == nil {
		t.Error("expected error for unserializable after")
	}
}

func TestPolicyChange_GetBefore(t *testing.T) {
	original := EligibilityChange{State: agent.StateExplore, ToolName: "read_file", Allowed: false}
	change, _ := NewPolicyChange(ChangeTypeEligibility, "read_file", "desc", original, true)

	var retrieved EligibilityChange
	err := change.GetBefore(&retrieved)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ToolName != "read_file" {
		t.Errorf("expected ToolName 'read_file', got %s", retrieved.ToolName)
	}
}

func TestPolicyChange_GetBefore_HandlesNil(t *testing.T) {
	change, _ := NewPolicyChange(ChangeTypeEligibility, "tool", "desc", nil, true)

	var retrieved EligibilityChange
	err := change.GetBefore(&retrieved)
	if err != nil {
		t.Errorf("expected no error for nil before, got %v", err)
	}
}

func TestPolicyChange_GetAfter(t *testing.T) {
	after := BudgetChange{BudgetName: "tool_calls", OldValue: 100, NewValue: 200}
	change, _ := NewPolicyChange(ChangeTypeBudget, "tool_calls", "desc", nil, after)

	var retrieved BudgetChange
	err := change.GetAfter(&retrieved)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.NewValue != 200 {
		t.Errorf("expected NewValue 200, got %d", retrieved.NewValue)
	}
}

func TestPolicyChange_GetAfter_HandlesNil(t *testing.T) {
	change := PolicyChange{Type: ChangeTypeEligibility, Target: "tool"}
	// After is nil (unusual but should be handled)

	var retrieved EligibilityChange
	err := change.GetAfter(&retrieved)
	if err != nil {
		t.Errorf("expected no error for nil after, got %v", err)
	}
}

// Change Type Data Serialization Tests

func TestEligibilityChange_Serialization(t *testing.T) {
	data := EligibilityChange{
		State:    agent.StateExplore,
		ToolName: "read_file",
		Allowed:  true,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled EligibilityChange
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.State != agent.StateExplore {
		t.Errorf("expected state Explore, got %s", unmarshaled.State)
	}
}

func TestTransitionChange_Serialization(t *testing.T) {
	data := TransitionChange{
		FromState: agent.StateExplore,
		ToState:   agent.StateDecide,
		Allowed:   true,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled TransitionChange
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.FromState != agent.StateExplore {
		t.Errorf("expected FromState Explore, got %s", unmarshaled.FromState)
	}
}

func TestBudgetChange_Serialization(t *testing.T) {
	data := BudgetChange{
		BudgetName: "tool_calls",
		OldValue:   100,
		NewValue:   200,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled BudgetChange
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.BudgetName != "tool_calls" {
		t.Errorf("expected BudgetName 'tool_calls', got %s", unmarshaled.BudgetName)
	}
}

func TestApprovalChange_Serialization(t *testing.T) {
	data := ApprovalChange{
		ToolName: "delete_file",
		Required: true,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ApprovalChange
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !unmarshaled.Required {
		t.Error("expected Required to be true")
	}
}

// Error Sentinel Tests

func TestErrorSentinels_Defined(t *testing.T) {
	errors := []error{
		ErrProposalNotFound,
		ErrProposalExists,
		ErrInvalidProposal,
		ErrInvalidStatusTransition,
		ErrCannotModifyNonDraft,
		ErrNoChanges,
		ErrHumanActorRequired,
		ErrApplyFailed,
		ErrRollbackFailed,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("error should not be nil")
		}
		if err.Error() == "" {
			t.Errorf("error %v should have a message", err)
		}
	}
}

// Time-related Tests

func TestTimestamps_Updated(t *testing.T) {
	p := NewProposal("test", "test", "creator")
	p.AddChange(PolicyChange{Type: ChangeTypeEligibility, Target: "tool"})

	createdAt := p.CreatedAt
	time.Sleep(1 * time.Millisecond)

	p.Submit("submitter")
	if !p.SubmittedAt.After(createdAt) {
		t.Error("expected SubmittedAt to be after CreatedAt")
	}

	time.Sleep(1 * time.Millisecond)
	p.Approve("approver", "approved")
	if !p.ApprovedAt.After(*p.SubmittedAt) {
		t.Error("expected ApprovedAt to be after SubmittedAt")
	}
}
