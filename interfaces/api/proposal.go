// Package api provides the public API for the agent runtime.
package api

import (
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/proposal"
	infraProposal "go.klarlabs.de/agent/infrastructure/proposal"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// Re-export proposal types for convenience.
type (
	// Proposal represents a policy change proposal requiring human approval.
	Proposal = proposal.Proposal

	// ProposalStatus tracks proposal lifecycle.
	ProposalStatus = proposal.ProposalStatus

	// PolicyChange describes a change to be applied.
	PolicyChange = proposal.PolicyChange

	// ChangeType categorizes the type of policy change.
	ChangeType = proposal.ChangeType

	// ProposalEvidence records evidence supporting a proposal.
	ProposalEvidence = proposal.ProposalEvidence

	// ProposalNote is a comment on a proposal.
	ProposalNote = proposal.ProposalNote

	// ProposalStore stores proposals.
	ProposalStore = proposal.Store

	// ProposalListFilter filters proposal queries.
	ProposalListFilter = proposal.ListFilter

	// PolicyVersion represents a versioned policy snapshot.
	PolicyVersion = policy.PolicyVersion

	// PolicyVersionStore stores policy versions.
	PolicyVersionStore = policy.VersionStore
)

// Re-export proposal status constants.
const (
	ProposalStatusDraft         = proposal.ProposalStatusDraft
	ProposalStatusPendingReview = proposal.ProposalStatusPendingReview
	ProposalStatusApproved      = proposal.ProposalStatusApproved
	ProposalStatusRejected      = proposal.ProposalStatusRejected
	ProposalStatusApplied       = proposal.ProposalStatusApplied
	ProposalStatusRolledBack    = proposal.ProposalStatusRolledBack
)

// Re-export change type constants.
const (
	ChangeTypeEligibility = proposal.ChangeTypeEligibility
	ChangeTypeTransition  = proposal.ChangeTypeTransition
	ChangeTypeBudget      = proposal.ChangeTypeBudget
	ChangeTypeApproval    = proposal.ChangeTypeApproval
)

// NewProposalStore creates a new in-memory proposal store.
func NewProposalStore() proposal.Store {
	return memory.NewProposalStore()
}

// NewPolicyVersionStore creates a new in-memory policy version store.
func NewPolicyVersionStore() policy.VersionStore {
	return memory.NewPolicyVersionStore()
}

// WorkflowService manages the proposal approval workflow.
type WorkflowService = infraProposal.WorkflowService

// NewWorkflowService creates a new workflow service.
func NewWorkflowService(
	proposalStore proposal.Store,
	versionStore policy.VersionStore,
	applier *infraProposal.PolicyApplier,
) *infraProposal.WorkflowService {
	return infraProposal.NewWorkflowService(proposalStore, versionStore, applier)
}

// PolicyApplier applies approved proposals to policy configuration.
type PolicyApplier = infraProposal.PolicyApplier

// NewPolicyApplier creates a new policy applier.
func NewPolicyApplier() *infraProposal.PolicyApplier {
	return infraProposal.NewPolicyApplier()
}

// Change creation helpers

// CreateEligibilityChange creates an eligibility policy change.
func CreateEligibilityChange(state State, toolName string, allowed bool, description string) (*proposal.PolicyChange, error) {
	return infraProposal.CreateEligibilityChange(state, toolName, allowed, description)
}

// CreateTransitionChange creates a transition policy change.
func CreateTransitionChange(from, to State, allowed bool, description string) (*proposal.PolicyChange, error) {
	return infraProposal.CreateTransitionChange(from, to, allowed, description)
}

// CreateBudgetChange creates a budget policy change.
func CreateBudgetChange(budgetName string, oldValue, newValue int, description string) (*proposal.PolicyChange, error) {
	return infraProposal.CreateBudgetChange(budgetName, oldValue, newValue, description)
}

// CreateApprovalChange creates an approval requirement policy change.
func CreateApprovalChange(toolName string, required bool, description string) (*proposal.PolicyChange, error) {
	return infraProposal.CreateApprovalChange(toolName, required, description)
}
