// Package memory provides in-memory storage implementations.
package memory

import (
	"context"
	"sort"
	"sync"

	"go.klarlabs.de/agent/domain/proposal"
)

// ProposalStore is an in-memory implementation of proposal.Store.
type ProposalStore struct {
	mu        sync.RWMutex
	proposals map[string]*proposal.Proposal
}

// NewProposalStore creates a new in-memory proposal store.
func NewProposalStore() *ProposalStore {
	return &ProposalStore{
		proposals: make(map[string]*proposal.Proposal),
	}
}

// Save persists a new proposal.
func (s *ProposalStore) Save(ctx context.Context, p *proposal.Proposal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		return proposal.ErrInvalidProposal
	}

	if _, exists := s.proposals[p.ID]; exists {
		return proposal.ErrProposalExists
	}

	// Store a copy
	stored := copyProposal(p)
	s.proposals[p.ID] = stored

	return nil
}

// Get retrieves a proposal by ID.
func (s *ProposalStore) Get(ctx context.Context, id string) (*proposal.Proposal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.proposals[id]
	if !exists {
		return nil, proposal.ErrProposalNotFound
	}

	// Return a copy
	return copyProposal(p), nil
}

// List returns proposals matching the filter.
func (s *ProposalStore) List(ctx context.Context, filter proposal.ListFilter) ([]*proposal.Proposal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*proposal.Proposal

	for _, p := range s.proposals {
		if !matchesProposalFilter(p, filter) {
			continue
		}

		results = append(results, copyProposal(p))
	}

	// Sort results
	sortProposals(results, filter.OrderBy, filter.Descending)

	// Apply offset and limit
	if filter.Offset > 0 {
		if filter.Offset >= len(results) {
			return []*proposal.Proposal{}, nil
		}
		results = results[filter.Offset:]
	}

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// Delete removes a proposal.
func (s *ProposalStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.proposals[id]; !exists {
		return proposal.ErrProposalNotFound
	}

	delete(s.proposals, id)
	return nil
}

// Update updates an existing proposal.
func (s *ProposalStore) Update(ctx context.Context, p *proposal.Proposal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		return proposal.ErrInvalidProposal
	}

	if _, exists := s.proposals[p.ID]; !exists {
		return proposal.ErrProposalNotFound
	}

	// Store a copy
	s.proposals[p.ID] = copyProposal(p)

	return nil
}

// Count returns the number of proposals matching the filter.
func (s *ProposalStore) Count(ctx context.Context, filter proposal.ListFilter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, p := range s.proposals {
		if matchesProposalFilter(p, filter) {
			count++
		}
	}

	return count, nil
}

// Summarize returns summary statistics.
func (s *ProposalStore) Summarize(ctx context.Context, filter proposal.ListFilter) (*proposal.Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &proposal.Summary{
		ByStatus: make(map[proposal.ProposalStatus]int64),
	}

	for _, p := range s.proposals {
		if !matchesProposalFilter(p, filter) {
			continue
		}

		summary.TotalProposals++
		summary.ByStatus[p.Status]++

		switch p.Status {
		case proposal.ProposalStatusPendingReview:
			summary.PendingReview++
		case proposal.ProposalStatusApplied:
			summary.AppliedCount++
		case proposal.ProposalStatusRolledBack:
			summary.RolledBackCount++
		}
	}

	return summary, nil
}

func matchesProposalFilter(p *proposal.Proposal, filter proposal.ListFilter) bool {
	// Filter by status
	if len(filter.Status) > 0 {
		found := false
		for _, st := range filter.Status {
			if p.Status == st {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by creator
	if filter.CreatedBy != "" && p.CreatedBy != filter.CreatedBy {
		return false
	}

	// Filter by suggestion ID
	if filter.SuggestionID != "" && p.SuggestionID != filter.SuggestionID {
		return false
	}

	// Filter by time range
	if !filter.FromTime.IsZero() && p.CreatedAt.Before(filter.FromTime) {
		return false
	}
	if !filter.ToTime.IsZero() && p.CreatedAt.After(filter.ToTime) {
		return false
	}

	return true
}

func sortProposals(proposals []*proposal.Proposal, orderBy proposal.OrderBy, descending bool) {
	sort.Slice(proposals, func(i, j int) bool {
		var less bool
		switch orderBy {
		case proposal.OrderByCreatedAt:
			less = proposals[i].CreatedAt.Before(proposals[j].CreatedAt)
		case proposal.OrderBySubmittedAt:
			switch {
			case proposals[i].SubmittedAt == nil && proposals[j].SubmittedAt == nil:
				less = false
			case proposals[i].SubmittedAt == nil:
				less = true
			case proposals[j].SubmittedAt == nil:
				less = false
			default:
				less = proposals[i].SubmittedAt.Before(*proposals[j].SubmittedAt)
			}
		case proposal.OrderByApprovedAt:
			switch {
			case proposals[i].ApprovedAt == nil && proposals[j].ApprovedAt == nil:
				less = false
			case proposals[i].ApprovedAt == nil:
				less = true
			case proposals[j].ApprovedAt == nil:
				less = false
			default:
				less = proposals[i].ApprovedAt.Before(*proposals[j].ApprovedAt)
			}
		case proposal.OrderByStatus:
			less = proposalStatusOrder(proposals[i].Status) < proposalStatusOrder(proposals[j].Status)
		default:
			less = proposals[i].CreatedAt.Before(proposals[j].CreatedAt)
		}

		if descending {
			return !less
		}
		return less
	})
}

func proposalStatusOrder(status proposal.ProposalStatus) int {
	switch status {
	case proposal.ProposalStatusDraft:
		return 1
	case proposal.ProposalStatusPendingReview:
		return 2
	case proposal.ProposalStatusApproved:
		return 3
	case proposal.ProposalStatusApplied:
		return 4
	case proposal.ProposalStatusRejected:
		return 5
	case proposal.ProposalStatusRolledBack:
		return 6
	default:
		return 0
	}
}

func copyProposal(p *proposal.Proposal) *proposal.Proposal {
	if p == nil {
		return nil
	}

	// Create a shallow copy
	copied := *p

	// Deep copy slices
	if p.Changes != nil {
		copied.Changes = make([]proposal.PolicyChange, len(p.Changes))
		copy(copied.Changes, p.Changes)
	}

	if p.Evidence != nil {
		copied.Evidence = make([]proposal.ProposalEvidence, len(p.Evidence))
		copy(copied.Evidence, p.Evidence)
	}

	if p.Notes != nil {
		copied.Notes = make([]proposal.ProposalNote, len(p.Notes))
		copy(copied.Notes, p.Notes)
	}

	// Deep copy map
	if p.Metadata != nil {
		copied.Metadata = make(map[string]any)
		for k, v := range p.Metadata {
			copied.Metadata[k] = v
		}
	}

	return &copied
}

// Ensure ProposalStore implements Store, Counter, and Summarizer
var _ proposal.Store = (*ProposalStore)(nil)
var _ proposal.Counter = (*ProposalStore)(nil)
var _ proposal.Summarizer = (*ProposalStore)(nil)
