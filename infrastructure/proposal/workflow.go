// Package proposal provides proposal infrastructure implementations.
package proposal

import (
	"context"
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/proposal"
)

// WorkflowService manages the proposal approval workflow.
type WorkflowService struct {
	proposalStore proposal.Store
	versionStore  policy.VersionStore
	applier       *PolicyApplier
}

// NewWorkflowService creates a new workflow service.
func NewWorkflowService(
	proposalStore proposal.Store,
	versionStore policy.VersionStore,
	applier *PolicyApplier,
) *WorkflowService {
	return &WorkflowService{
		proposalStore: proposalStore,
		versionStore:  versionStore,
		applier:       applier,
	}
}

// CreateProposal creates a new proposal.
func (w *WorkflowService) CreateProposal(ctx context.Context, title, description, creator string) (*proposal.Proposal, error) {
	p := proposal.NewProposal(title, description, creator)
	if err := w.proposalStore.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to save proposal: %w", err)
	}
	return p, nil
}

// AddChange adds a change to a draft proposal.
func (w *WorkflowService) AddChange(ctx context.Context, proposalID string, change proposal.PolicyChange) error {
	p, err := w.proposalStore.Get(ctx, proposalID)
	if err != nil {
		return err
	}

	if err := p.AddChange(change); err != nil {
		return err
	}

	return w.proposalStore.Update(ctx, p)
}

// Submit submits a proposal for review.
func (w *WorkflowService) Submit(ctx context.Context, proposalID, submitter string) error {
	p, err := w.proposalStore.Get(ctx, proposalID)
	if err != nil {
		return err
	}

	if err := p.Submit(submitter); err != nil {
		return err
	}

	return w.proposalStore.Update(ctx, p)
}

// Approve approves a proposal.
func (w *WorkflowService) Approve(ctx context.Context, proposalID, approver, reason string) error {
	p, err := w.proposalStore.Get(ctx, proposalID)
	if err != nil {
		return err
	}

	if err := p.Approve(approver, reason); err != nil {
		return err
	}

	return w.proposalStore.Update(ctx, p)
}

// Reject rejects a proposal.
func (w *WorkflowService) Reject(ctx context.Context, proposalID, rejector, reason string) error {
	p, err := w.proposalStore.Get(ctx, proposalID)
	if err != nil {
		return err
	}

	if err := p.Reject(rejector, reason); err != nil {
		return err
	}

	return w.proposalStore.Update(ctx, p)
}

// Apply applies an approved proposal's changes.
func (w *WorkflowService) Apply(ctx context.Context, proposalID string) error {
	p, err := w.proposalStore.Get(ctx, proposalID)
	if err != nil {
		return err
	}

	if p.Status != proposal.ProposalStatusApproved {
		return proposal.ErrInvalidStatusTransition
	}

	// Get current policy version
	currentVersion, err := w.versionStore.GetCurrent(ctx)
	if err != nil {
		// If no version exists, start with version 0
		currentVersion = &policy.PolicyVersion{
			Version:     0,
			Eligibility: policy.NewEligibilitySnapshot(),
			Transitions: policy.NewTransitionSnapshot(),
			Budgets:     policy.NewBudgetLimitsSnapshot(),
			Approvals:   policy.NewApprovalSnapshot(),
		}
	}

	// Apply changes and create new version
	newVersion, err := w.applier.Apply(ctx, currentVersion, p.Changes)
	if err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	newVersion.ProposalID = proposalID
	newVersion.Description = p.Title
	newVersion.CreatedAt = time.Now()

	// Save new version
	if err := w.versionStore.Save(ctx, newVersion); err != nil {
		return fmt.Errorf("failed to save policy version: %w", err)
	}

	// Update proposal status
	if err := p.Apply(currentVersion.Version, newVersion.Version); err != nil {
		return err
	}

	return w.proposalStore.Update(ctx, p)
}

// Rollback rolls back an applied proposal's changes.
func (w *WorkflowService) Rollback(ctx context.Context, proposalID, reason string) error {
	p, err := w.proposalStore.Get(ctx, proposalID)
	if err != nil {
		return err
	}

	if p.Status != proposal.ProposalStatusApplied {
		return proposal.ErrInvalidStatusTransition
	}

	// Get the version before the proposal was applied
	previousVersion, err := w.versionStore.Get(ctx, p.PolicyVersionBefore)
	if err != nil {
		return fmt.Errorf("failed to get previous version: %w", err)
	}

	// Create a new version that's a copy of the previous one
	rollbackVersion := &policy.PolicyVersion{
		Version:     p.PolicyVersionAfter + 1,
		CreatedAt:   time.Now(),
		ProposalID:  proposalID,
		Description: fmt.Sprintf("Rollback of proposal: %s", p.Title),
		Eligibility: previousVersion.Eligibility,
		Transitions: previousVersion.Transitions,
		Budgets:     previousVersion.Budgets,
		Approvals:   previousVersion.Approvals,
	}

	// Save rollback version
	if err := w.versionStore.Save(ctx, rollbackVersion); err != nil {
		return fmt.Errorf("failed to save rollback version: %w", err)
	}

	// Update proposal status
	if err := p.Rollback(reason); err != nil {
		return err
	}

	return w.proposalStore.Update(ctx, p)
}

// AddNote adds a note to a proposal.
func (w *WorkflowService) AddNote(ctx context.Context, proposalID, author, content string) error {
	p, err := w.proposalStore.Get(ctx, proposalID)
	if err != nil {
		return err
	}

	p.AddNote(author, content)

	return w.proposalStore.Update(ctx, p)
}
