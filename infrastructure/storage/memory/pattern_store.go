// Package memory provides in-memory storage implementations.
package memory

import (
	"context"
	"sort"
	"sync"

	"go.klarlabs.de/agent/domain/pattern"
)

// PatternStore is an in-memory implementation of pattern.Store.
type PatternStore struct {
	mu       sync.RWMutex
	patterns map[string]*pattern.Pattern
}

// NewPatternStore creates a new in-memory pattern store.
func NewPatternStore() *PatternStore {
	return &PatternStore{
		patterns: make(map[string]*pattern.Pattern),
	}
}

// Save persists a pattern.
func (s *PatternStore) Save(ctx context.Context, p *pattern.Pattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		return pattern.ErrInvalidPattern
	}

	if _, exists := s.patterns[p.ID]; exists {
		return pattern.ErrPatternExists
	}

	// Store a copy
	stored := *p
	s.patterns[p.ID] = &stored

	return nil
}

// Get retrieves a pattern by ID.
func (s *PatternStore) Get(ctx context.Context, id string) (*pattern.Pattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.patterns[id]
	if !exists {
		return nil, pattern.ErrPatternNotFound
	}

	// Return a copy
	result := *p
	return &result, nil
}

// List returns patterns matching the filter.
func (s *PatternStore) List(ctx context.Context, filter pattern.ListFilter) ([]*pattern.Pattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*pattern.Pattern

	for _, p := range s.patterns {
		if !matchesFilter(p, filter) {
			continue
		}

		// Store a copy
		stored := *p
		results = append(results, &stored)
	}

	// Sort results
	sortPatterns(results, filter.OrderBy, filter.Descending)

	// Apply offset and limit
	if filter.Offset > 0 {
		if filter.Offset >= len(results) {
			return []*pattern.Pattern{}, nil
		}
		results = results[filter.Offset:]
	}

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// Delete removes a pattern.
func (s *PatternStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.patterns[id]; !exists {
		return pattern.ErrPatternNotFound
	}

	delete(s.patterns, id)
	return nil
}

// Update updates an existing pattern.
func (s *PatternStore) Update(ctx context.Context, p *pattern.Pattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		return pattern.ErrInvalidPattern
	}

	if _, exists := s.patterns[p.ID]; !exists {
		return pattern.ErrPatternNotFound
	}

	// Store a copy
	stored := *p
	s.patterns[p.ID] = &stored

	return nil
}

// Count returns the number of patterns matching the filter.
func (s *PatternStore) Count(ctx context.Context, filter pattern.ListFilter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, p := range s.patterns {
		if matchesFilter(p, filter) {
			count++
		}
	}

	return count, nil
}

// Summarize returns summary statistics.
func (s *PatternStore) Summarize(ctx context.Context, filter pattern.ListFilter) (*pattern.Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &pattern.Summary{
		ByType: make(map[pattern.PatternType]int64),
	}

	var totalConfidence float64
	var totalFrequency int

	for _, p := range s.patterns {
		if !matchesFilter(p, filter) {
			continue
		}

		summary.TotalPatterns++
		summary.ByType[p.Type]++
		totalConfidence += p.Confidence
		totalFrequency += p.Frequency
	}

	if summary.TotalPatterns > 0 {
		summary.AverageConfidence = totalConfidence / float64(summary.TotalPatterns)
		summary.AverageFrequency = float64(totalFrequency) / float64(summary.TotalPatterns)
	}

	return summary, nil
}

func matchesFilter(p *pattern.Pattern, filter pattern.ListFilter) bool {
	// Filter by types
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if p.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by minimum confidence
	if filter.MinConfidence > 0 && p.Confidence < filter.MinConfidence {
		return false
	}

	// Filter by minimum frequency
	if filter.MinFrequency > 0 && p.Frequency < filter.MinFrequency {
		return false
	}

	// Filter by time range
	if !filter.FromTime.IsZero() && p.FirstSeen.Before(filter.FromTime) {
		return false
	}
	if !filter.ToTime.IsZero() && p.LastSeen.After(filter.ToTime) {
		return false
	}

	// Filter by run ID
	if filter.RunID != "" {
		found := false
		for _, runID := range p.RunIDs {
			if runID == filter.RunID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func sortPatterns(patterns []*pattern.Pattern, orderBy pattern.OrderBy, descending bool) {
	sort.Slice(patterns, func(i, j int) bool {
		var less bool
		switch orderBy {
		case pattern.OrderByFirstSeen:
			less = patterns[i].FirstSeen.Before(patterns[j].FirstSeen)
		case pattern.OrderByLastSeen:
			less = patterns[i].LastSeen.Before(patterns[j].LastSeen)
		case pattern.OrderByFrequency:
			less = patterns[i].Frequency < patterns[j].Frequency
		case pattern.OrderByConfidence:
			less = patterns[i].Confidence < patterns[j].Confidence
		default:
			less = patterns[i].FirstSeen.Before(patterns[j].FirstSeen)
		}

		if descending {
			return !less
		}
		return less
	})
}

// Ensure PatternStore implements Store, Counter, and Summarizer
var _ pattern.Store = (*PatternStore)(nil)
var _ pattern.Counter = (*PatternStore)(nil)
var _ pattern.Summarizer = (*PatternStore)(nil)
