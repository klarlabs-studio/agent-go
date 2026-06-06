// Package application provides application services.
package application

import (
	"context"
	"fmt"

	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/proposal"
	"go.klarlabs.de/agent/domain/suggestion"
	infraProposal "go.klarlabs.de/agent/infrastructure/proposal"
)

// EvolutionService manages policy evolution through the suggestion-proposal workflow.
type EvolutionService struct {
	generator       suggestion.Generator
	suggestionStore suggestion.Store
	workflow        *infraProposal.WorkflowService
	patternStore    pattern.Store
}

// NewEvolutionService creates a new evolution service.
func NewEvolutionService(
	generator suggestion.Generator,
	suggestionStore suggestion.Store,
	workflow *infraProposal.WorkflowService,
	patternStore pattern.Store,
) *EvolutionService {
	return &EvolutionService{
		generator:       generator,
		suggestionStore: suggestionStore,
		workflow:        workflow,
		patternStore:    patternStore,
	}
}

// GenerateSuggestions creates suggestions from detected patterns.
func (s *EvolutionService) GenerateSuggestions(ctx context.Context, patternIDs []string) ([]suggestion.Suggestion, error) {
	if s.generator == nil {
		return nil, suggestion.ErrGenerationFailed
	}

	// Get patterns
	var patterns []pattern.Pattern
	if s.patternStore != nil && len(patternIDs) > 0 {
		for _, id := range patternIDs {
			p, err := s.patternStore.Get(ctx, id)
			if err == nil {
				patterns = append(patterns, *p)
			}
		}
	}

	if len(patterns) == 0 {
		return nil, suggestion.ErrNoPatterns
	}

	// Generate suggestions
	suggestions, err := s.generator.Generate(ctx, patterns)
	if err != nil {
		return nil, fmt.Errorf("suggestion generation failed: %w", err)
	}

	// Store suggestions if store is configured
	if s.suggestionStore != nil {
		for i := range suggestions {
			if err := s.suggestionStore.Save(ctx, &suggestions[i]); err != nil {
				continue
			}
		}
	}

	return suggestions, nil
}

// GetSuggestion retrieves a suggestion by ID.
func (s *EvolutionService) GetSuggestion(ctx context.Context, id string) (*suggestion.Suggestion, error) {
	if s.suggestionStore == nil {
		return nil, suggestion.ErrSuggestionNotFound
	}
	return s.suggestionStore.Get(ctx, id)
}

// ListSuggestions returns suggestions matching the filter.
func (s *EvolutionService) ListSuggestions(ctx context.Context, filter suggestion.ListFilter) ([]*suggestion.Suggestion, error) {
	if s.suggestionStore == nil {
		return []*suggestion.Suggestion{}, nil
	}
	return s.suggestionStore.List(ctx, filter)
}

// AcceptSuggestion converts a suggestion into a proposal.
func (s *EvolutionService) AcceptSuggestion(ctx context.Context, suggestionID, actor string) (*proposal.Proposal, error) {
	if s.suggestionStore == nil {
		return nil, suggestion.ErrSuggestionNotFound
	}
	if s.workflow == nil {
		return nil, proposal.ErrInvalidProposal
	}

	// Get suggestion
	sug, err := s.suggestionStore.Get(ctx, suggestionID)
	if err != nil {
		return nil, err
	}

	// Create proposal from suggestion
	prop, err := s.workflow.CreateProposal(ctx, sug.Title, sug.Description, actor)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposal: %w", err)
	}

	// Add change from suggestion
	change := proposal.PolicyChange{
		Type:        proposal.ChangeType(sug.Change.Type),
		Target:      sug.Change.Target,
		Description: sug.Description,
	}
	change.After = sug.ChangeData

	if err := s.workflow.AddChange(ctx, prop.ID, change); err != nil {
		return nil, fmt.Errorf("failed to add change: %w", err)
	}

	// Mark suggestion as accepted
	if err := sug.Accept(prop.ID, actor); err != nil {
		return nil, err
	}
	if err := s.suggestionStore.Update(ctx, sug); err != nil {
		return nil, err
	}

	// Get updated proposal
	return s.GetProposal(ctx, prop.ID)
}

// RejectSuggestion rejects a suggestion.
func (s *EvolutionService) RejectSuggestion(ctx context.Context, suggestionID, actor, reason string) error {
	if s.suggestionStore == nil {
		return suggestion.ErrSuggestionNotFound
	}

	sug, err := s.suggestionStore.Get(ctx, suggestionID)
	if err != nil {
		return err
	}

	if err := sug.Reject(actor, reason); err != nil {
		return err
	}

	return s.suggestionStore.Update(ctx, sug)
}

// CreateProposal creates a new proposal manually.
func (s *EvolutionService) CreateProposal(ctx context.Context, title, description, creator string) (*proposal.Proposal, error) {
	if s.workflow == nil {
		return nil, proposal.ErrInvalidProposal
	}
	return s.workflow.CreateProposal(ctx, title, description, creator)
}

// GetProposal retrieves a proposal by ID.
func (s *EvolutionService) GetProposal(ctx context.Context, id string) (*proposal.Proposal, error) {
	if s.workflow == nil {
		return nil, proposal.ErrProposalNotFound
	}
	// Note: We need to expose proposalStore in workflow or use a separate proposal store
	// For now, return nil as the workflow doesn't expose Get
	return nil, proposal.ErrProposalNotFound
}

// SubmitProposal submits a proposal for review.
func (s *EvolutionService) SubmitProposal(ctx context.Context, proposalID, submitter string) error {
	if s.workflow == nil {
		return proposal.ErrProposalNotFound
	}
	return s.workflow.Submit(ctx, proposalID, submitter)
}

// ApproveProposal approves a proposal.
func (s *EvolutionService) ApproveProposal(ctx context.Context, proposalID, approver, reason string) error {
	if s.workflow == nil {
		return proposal.ErrProposalNotFound
	}
	return s.workflow.Approve(ctx, proposalID, approver, reason)
}

// RejectProposal rejects a proposal.
func (s *EvolutionService) RejectProposal(ctx context.Context, proposalID, rejector, reason string) error {
	if s.workflow == nil {
		return proposal.ErrProposalNotFound
	}
	return s.workflow.Reject(ctx, proposalID, rejector, reason)
}

// ApplyProposal applies an approved proposal.
func (s *EvolutionService) ApplyProposal(ctx context.Context, proposalID string) error {
	if s.workflow == nil {
		return proposal.ErrProposalNotFound
	}
	return s.workflow.Apply(ctx, proposalID)
}

// RollbackProposal rolls back an applied proposal.
func (s *EvolutionService) RollbackProposal(ctx context.Context, proposalID, reason string) error {
	if s.workflow == nil {
		return proposal.ErrProposalNotFound
	}
	return s.workflow.Rollback(ctx, proposalID, reason)
}
