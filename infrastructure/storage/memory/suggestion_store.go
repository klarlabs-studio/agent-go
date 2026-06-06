// Package memory provides in-memory storage implementations.
package memory

import (
	"context"
	"sort"
	"sync"

	"go.klarlabs.de/agent/domain/suggestion"
)

// SuggestionStore is an in-memory implementation of suggestion.Store.
type SuggestionStore struct {
	mu          sync.RWMutex
	suggestions map[string]*suggestion.Suggestion
}

// NewSuggestionStore creates a new in-memory suggestion store.
func NewSuggestionStore() *SuggestionStore {
	return &SuggestionStore{
		suggestions: make(map[string]*suggestion.Suggestion),
	}
}

// Save persists a new suggestion.
func (s *SuggestionStore) Save(ctx context.Context, sug *suggestion.Suggestion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sug.ID == "" {
		return suggestion.ErrInvalidSuggestion
	}

	if _, exists := s.suggestions[sug.ID]; exists {
		return suggestion.ErrSuggestionExists
	}

	// Store a copy
	stored := *sug
	s.suggestions[sug.ID] = &stored

	return nil
}

// Get retrieves a suggestion by ID.
func (s *SuggestionStore) Get(ctx context.Context, id string) (*suggestion.Suggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sug, exists := s.suggestions[id]
	if !exists {
		return nil, suggestion.ErrSuggestionNotFound
	}

	// Return a copy
	result := *sug
	return &result, nil
}

// List returns suggestions matching the filter.
func (s *SuggestionStore) List(ctx context.Context, filter suggestion.ListFilter) ([]*suggestion.Suggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*suggestion.Suggestion

	for _, sug := range s.suggestions {
		if !matchesSuggestionFilter(sug, filter) {
			continue
		}

		// Store a copy
		stored := *sug
		results = append(results, &stored)
	}

	// Sort results
	sortSuggestions(results, filter.OrderBy, filter.Descending)

	// Apply offset and limit
	if filter.Offset > 0 {
		if filter.Offset >= len(results) {
			return []*suggestion.Suggestion{}, nil
		}
		results = results[filter.Offset:]
	}

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// Delete removes a suggestion.
func (s *SuggestionStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.suggestions[id]; !exists {
		return suggestion.ErrSuggestionNotFound
	}

	delete(s.suggestions, id)
	return nil
}

// Update updates an existing suggestion.
func (s *SuggestionStore) Update(ctx context.Context, sug *suggestion.Suggestion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sug.ID == "" {
		return suggestion.ErrInvalidSuggestion
	}

	if _, exists := s.suggestions[sug.ID]; !exists {
		return suggestion.ErrSuggestionNotFound
	}

	// Store a copy
	stored := *sug
	s.suggestions[sug.ID] = &stored

	return nil
}

// Count returns the number of suggestions matching the filter.
func (s *SuggestionStore) Count(ctx context.Context, filter suggestion.ListFilter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, sug := range s.suggestions {
		if matchesSuggestionFilter(sug, filter) {
			count++
		}
	}

	return count, nil
}

// Summarize returns summary statistics.
func (s *SuggestionStore) Summarize(ctx context.Context, filter suggestion.ListFilter) (*suggestion.Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &suggestion.Summary{
		ByType:   make(map[suggestion.SuggestionType]int64),
		ByStatus: make(map[suggestion.SuggestionStatus]int64),
		ByImpact: make(map[suggestion.ImpactLevel]int64),
	}

	var totalConfidence float64

	for _, sug := range s.suggestions {
		if !matchesSuggestionFilter(sug, filter) {
			continue
		}

		summary.TotalSuggestions++
		summary.ByType[sug.Type]++
		summary.ByStatus[sug.Status]++
		summary.ByImpact[sug.Impact]++
		totalConfidence += sug.Confidence
	}

	if summary.TotalSuggestions > 0 {
		summary.AverageConfidence = totalConfidence / float64(summary.TotalSuggestions)
	}

	return summary, nil
}

func matchesSuggestionFilter(sug *suggestion.Suggestion, filter suggestion.ListFilter) bool {
	// Filter by types
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if sug.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by status
	if len(filter.Status) > 0 {
		found := false
		for _, st := range filter.Status {
			if sug.Status == st {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by minimum confidence
	if filter.MinConfidence > 0 && sug.Confidence < filter.MinConfidence {
		return false
	}

	// Filter by impact
	if len(filter.Impact) > 0 {
		found := false
		for _, imp := range filter.Impact {
			if sug.Impact == imp {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by pattern ID
	if filter.PatternID != "" {
		found := false
		for _, pid := range sug.PatternIDs {
			if pid == filter.PatternID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by time range
	if !filter.FromTime.IsZero() && sug.CreatedAt.Before(filter.FromTime) {
		return false
	}
	if !filter.ToTime.IsZero() && sug.CreatedAt.After(filter.ToTime) {
		return false
	}

	return true
}

func sortSuggestions(suggestions []*suggestion.Suggestion, orderBy suggestion.OrderBy, descending bool) {
	sort.Slice(suggestions, func(i, j int) bool {
		var less bool
		switch orderBy {
		case suggestion.OrderByCreatedAt:
			less = suggestions[i].CreatedAt.Before(suggestions[j].CreatedAt)
		case suggestion.OrderByConfidence:
			less = suggestions[i].Confidence < suggestions[j].Confidence
		case suggestion.OrderByImpact:
			less = impactOrder(suggestions[i].Impact) < impactOrder(suggestions[j].Impact)
		case suggestion.OrderByStatus:
			less = statusOrder(suggestions[i].Status) < statusOrder(suggestions[j].Status)
		default:
			less = suggestions[i].CreatedAt.Before(suggestions[j].CreatedAt)
		}

		if descending {
			return !less
		}
		return less
	})
}

func impactOrder(impact suggestion.ImpactLevel) int {
	switch impact {
	case suggestion.ImpactLevelLow:
		return 1
	case suggestion.ImpactLevelMedium:
		return 2
	case suggestion.ImpactLevelHigh:
		return 3
	default:
		return 0
	}
}

func statusOrder(status suggestion.SuggestionStatus) int {
	switch status {
	case suggestion.SuggestionStatusPending:
		return 1
	case suggestion.SuggestionStatusAccepted:
		return 2
	case suggestion.SuggestionStatusRejected:
		return 3
	case suggestion.SuggestionStatusSuperseded:
		return 4
	default:
		return 0
	}
}

// Ensure SuggestionStore implements Store, Counter, and Summarizer
var _ suggestion.Store = (*SuggestionStore)(nil)
var _ suggestion.Counter = (*SuggestionStore)(nil)
var _ suggestion.Summarizer = (*SuggestionStore)(nil)
