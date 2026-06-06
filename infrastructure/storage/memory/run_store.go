package memory

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/run"
)

// runEntry holds a deep copy of a run for storage.
type runEntry struct {
	data []byte
}

// RunStore is an in-memory implementation of run.Store.
type RunStore struct {
	runs map[string]*runEntry
	mu   sync.RWMutex
}

// NewRunStore creates a new in-memory run store.
func NewRunStore() *RunStore {
	return &RunStore{
		runs: make(map[string]*runEntry),
	}
}

// Save persists a new run.
func (s *RunStore) Save(ctx context.Context, r *agent.Run) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[r.ID]; exists {
		return run.ErrRunExists
	}

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	s.runs[r.ID] = &runEntry{data: data}
	return nil
}

// Get retrieves a run by ID.
func (s *RunStore) Get(ctx context.Context, id string) (*agent.Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, run.ErrInvalidRunID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.runs[id]
	if !ok {
		return nil, run.ErrRunNotFound
	}

	var r agent.Run
	if err := json.Unmarshal(entry.data, &r); err != nil {
		return nil, err
	}

	return &r, nil
}

// Update updates an existing run.
func (s *RunStore) Update(ctx context.Context, r *agent.Run) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[r.ID]; !exists {
		return run.ErrRunNotFound
	}

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	s.runs[r.ID] = &runEntry{data: data}
	return nil
}

// Delete removes a run by ID.
func (s *RunStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if id == "" {
		return run.ErrInvalidRunID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[id]; !exists {
		return run.ErrRunNotFound
	}

	delete(s.runs, id)
	return nil
}

// List returns runs matching the filter.
func (s *RunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*agent.Run

	for _, entry := range s.runs {
		var r agent.Run
		if err := json.Unmarshal(entry.data, &r); err != nil {
			continue
		}

		if !s.matchesFilter(&r, filter) {
			continue
		}

		result = append(result, &r)
	}

	// Sort results
	s.sortRuns(result, filter.OrderBy, filter.Descending)

	// Apply offset and limit
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []*agent.Run{}, nil
		}
		result = result[filter.Offset:]
	}

	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

// Count returns the number of runs matching the filter.
func (s *RunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64

	for _, entry := range s.runs {
		var r agent.Run
		if err := json.Unmarshal(entry.data, &r); err != nil {
			continue
		}

		if s.matchesFilter(&r, filter) {
			count++
		}
	}

	return count, nil
}

// Summary returns aggregate statistics.
func (s *RunStore) Summary(ctx context.Context, filter run.ListFilter) (run.Summary, error) {
	if err := ctx.Err(); err != nil {
		return run.Summary{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var summary run.Summary
	var totalDuration time.Duration

	for _, entry := range s.runs {
		var r agent.Run
		if err := json.Unmarshal(entry.data, &r); err != nil {
			continue
		}

		if !s.matchesFilter(&r, filter) {
			continue
		}

		summary.TotalRuns++

		switch r.Status {
		case agent.RunStatusCompleted:
			summary.CompletedRuns++
			totalDuration += r.Duration()
		case agent.RunStatusFailed:
			summary.FailedRuns++
			totalDuration += r.Duration()
		case agent.RunStatusRunning:
			summary.RunningRuns++
		}
	}

	if summary.CompletedRuns+summary.FailedRuns > 0 {
		summary.AverageDuration = totalDuration / time.Duration(summary.CompletedRuns+summary.FailedRuns)
	}

	return summary, nil
}

// matchesFilter checks if a run matches the filter criteria.
func (s *RunStore) matchesFilter(r *agent.Run, filter run.ListFilter) bool {
	// Filter by status
	if len(filter.Status) > 0 {
		found := false
		for _, status := range filter.Status {
			if r.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by state
	if len(filter.States) > 0 {
		found := false
		for _, state := range filter.States {
			if r.CurrentState == state {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by time range
	if !filter.FromTime.IsZero() && r.StartTime.Before(filter.FromTime) {
		return false
	}

	if !filter.ToTime.IsZero() && r.StartTime.After(filter.ToTime) {
		return false
	}

	// Filter by goal pattern
	if filter.GoalPattern != "" && !strings.Contains(r.Goal, filter.GoalPattern) {
		return false
	}

	return true
}

// sortRuns sorts runs by the specified field.
func (s *RunStore) sortRuns(runs []*agent.Run, orderBy run.OrderBy, descending bool) {
	sort.Slice(runs, func(i, j int) bool {
		var less bool

		switch orderBy {
		case run.OrderByStartTime:
			less = runs[i].StartTime.Before(runs[j].StartTime)
		case run.OrderByEndTime:
			less = runs[i].EndTime.Before(runs[j].EndTime)
		case run.OrderByID:
			less = runs[i].ID < runs[j].ID
		case run.OrderByStatus:
			less = string(runs[i].Status) < string(runs[j].Status)
		default:
			less = runs[i].StartTime.Before(runs[j].StartTime)
		}

		if descending {
			return !less
		}
		return less
	})
}

// Clear removes all runs from the store.
func (s *RunStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = make(map[string]*runEntry)
}

// Len returns the number of stored runs.
func (s *RunStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runs)
}

// Ensure RunStore implements run.Store and run.SummaryProvider
var (
	_ run.Store           = (*RunStore)(nil)
	_ run.SummaryProvider = (*RunStore)(nil)
)
