// Package memory provides in-memory storage implementations.
package memory

import (
	"context"
	"errors"
	"sort"
	"sync"

	"go.klarlabs.de/agent/domain/policy"
)

// ErrVersionNotFound indicates the policy version was not found.
var ErrVersionNotFound = errors.New("policy version not found")

// PolicyVersionStore is an in-memory implementation of policy.VersionStore.
type PolicyVersionStore struct {
	mu       sync.RWMutex
	versions map[int]*policy.PolicyVersion
}

// NewPolicyVersionStore creates a new in-memory policy version store.
func NewPolicyVersionStore() *PolicyVersionStore {
	return &PolicyVersionStore{
		versions: make(map[int]*policy.PolicyVersion),
	}
}

// Save persists a new policy version.
func (s *PolicyVersionStore) Save(ctx context.Context, version *policy.PolicyVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.versions[version.Version]; exists {
		return errors.New("policy version already exists")
	}

	// Store a copy
	stored := copyPolicyVersion(version)
	s.versions[version.Version] = stored

	return nil
}

// GetCurrent retrieves the current (latest) policy version.
func (s *PolicyVersionStore) GetCurrent(ctx context.Context) (*policy.PolicyVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.versions) == 0 {
		return nil, ErrVersionNotFound
	}

	// Find the highest version number
	var maxVersion int
	for v := range s.versions {
		if v > maxVersion {
			maxVersion = v
		}
	}

	return copyPolicyVersion(s.versions[maxVersion]), nil
}

// Get retrieves a specific policy version.
func (s *PolicyVersionStore) Get(ctx context.Context, version int) (*policy.PolicyVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, exists := s.versions[version]
	if !exists {
		return nil, ErrVersionNotFound
	}

	return copyPolicyVersion(v), nil
}

// List returns all policy versions.
func (s *PolicyVersionStore) List(ctx context.Context) ([]*policy.PolicyVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]*policy.PolicyVersion, 0, len(s.versions))
	for _, v := range s.versions {
		results = append(results, copyPolicyVersion(v))
	}

	// Sort by version number descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Version > results[j].Version
	})

	return results, nil
}

// GetByProposal retrieves the policy version created by a proposal.
func (s *PolicyVersionStore) GetByProposal(ctx context.Context, proposalID string) (*policy.PolicyVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, v := range s.versions {
		if v.ProposalID == proposalID {
			return copyPolicyVersion(v), nil
		}
	}

	return nil, ErrVersionNotFound
}

func copyPolicyVersion(v *policy.PolicyVersion) *policy.PolicyVersion {
	if v == nil {
		return nil
	}

	copied := &policy.PolicyVersion{
		Version:     v.Version,
		CreatedAt:   v.CreatedAt,
		ProposalID:  v.ProposalID,
		Description: v.Description,
		Eligibility: copyEligibility(v.Eligibility),
		Transitions: copyTransitions(v.Transitions),
		Budgets:     copyBudgets(v.Budgets),
		Approvals:   copyApprovals(v.Approvals),
	}

	return copied
}

func copyEligibility(src policy.EligibilitySnapshot) policy.EligibilitySnapshot {
	dst := policy.NewEligibilitySnapshot()
	for state, tools := range src.StateTools {
		for _, tool := range tools {
			dst.AddTool(state, tool)
		}
	}
	return dst
}

func copyTransitions(src policy.TransitionSnapshot) policy.TransitionSnapshot {
	dst := policy.NewTransitionSnapshot()
	for from, tos := range src.Transitions {
		for _, to := range tos {
			dst.AddTransition(from, to)
		}
	}
	return dst
}

func copyBudgets(src policy.BudgetLimitsSnapshot) policy.BudgetLimitsSnapshot {
	dst := policy.NewBudgetLimitsSnapshot()
	for name, limit := range src.Limits {
		dst.SetLimit(name, limit)
	}
	return dst
}

func copyApprovals(src policy.ApprovalSnapshot) policy.ApprovalSnapshot {
	dst := policy.NewApprovalSnapshot()
	for _, tool := range src.RequiredTools {
		dst.RequireApproval(tool)
	}
	return dst
}

// Ensure PolicyVersionStore implements VersionStore
var _ policy.VersionStore = (*PolicyVersionStore)(nil)
