package memory

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"go.klarlabs.de/agent/domain/event"
)

// EventStore is an in-memory implementation of event.Store.
type EventStore struct {
	events      map[string][]event.Event // runID -> events
	subscribers map[string][]chan event.Event
	sequences   map[string]uint64 // runID -> next sequence
	mu          sync.RWMutex
}

// NewEventStore creates a new in-memory event store.
func NewEventStore() *EventStore {
	return &EventStore{
		events:      make(map[string][]event.Event),
		subscribers: make(map[string][]chan event.Event),
		sequences:   make(map[string]uint64),
	}
}

// Append persists one or more events atomically.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Group events by run ID
	byRun := make(map[string][]event.Event)
	for _, e := range events {
		byRun[e.RunID] = append(byRun[e.RunID], e)
	}

	// Process each run's events
	for runID, runEvents := range byRun {
		seq := s.sequences[runID]

		for i := range runEvents {
			// Assign ID if not set
			if runEvents[i].ID == "" {
				runEvents[i].ID = uuid.New().String()
			}

			// Assign sequence number
			seq++
			runEvents[i].Sequence = seq

			// Validate event
			if runEvents[i].Type == "" {
				return event.ErrInvalidEvent
			}
		}

		// Store events
		s.events[runID] = append(s.events[runID], runEvents...)
		s.sequences[runID] = seq

		// Notify subscribers
		if subs, ok := s.subscribers[runID]; ok {
			for _, sub := range subs {
				for _, e := range runEvents {
					select {
					case sub <- e:
					default:
						// Channel full, skip (non-blocking)
					}
				}
			}
		}
	}

	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	events, ok := s.events[runID]
	if !ok {
		return []event.Event{}, nil
	}

	// Return a copy to prevent mutation
	result := make([]event.Event, len(events))
	copy(result, events)
	return result, nil
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	events, ok := s.events[runID]
	if !ok {
		return []event.Event{}, nil
	}

	var result []event.Event
	for _, e := range events {
		if e.Sequence >= fromSeq {
			result = append(result, e)
		}
	}

	return result, nil
}

// Subscribe returns a channel that receives new events for a run.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan event.Event, 100)
	s.subscribers[runID] = append(s.subscribers[runID], ch)

	// Start cleanup goroutine
	go func() {
		<-ctx.Done()
		s.unsubscribe(runID, ch)
	}()

	return ch, nil
}

// unsubscribe removes a subscriber channel.
func (s *EventStore) unsubscribe(runID string, ch chan event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs := s.subscribers[runID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[runID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}

	// Clean up empty subscriber lists
	if len(s.subscribers[runID]) == 0 {
		delete(s.subscribers, runID)
	}
}

// Query retrieves events matching the given options.
func (s *EventStore) Query(ctx context.Context, runID string, opts event.QueryOptions) ([]event.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	events, ok := s.events[runID]
	if !ok {
		return []event.Event{}, nil
	}

	var result []event.Event
	for _, e := range events {
		if !s.matchesQuery(e, opts) {
			continue
		}
		result = append(result, e)
	}

	// Apply offset
	if opts.Offset > 0 {
		if opts.Offset >= len(result) {
			return []event.Event{}, nil
		}
		result = result[opts.Offset:]
	}

	// Apply limit
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result, nil
}

// matchesQuery checks if an event matches the query options.
func (s *EventStore) matchesQuery(e event.Event, opts event.QueryOptions) bool {
	// Filter by type
	if len(opts.Types) > 0 {
		found := false
		for _, t := range opts.Types {
			if e.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by time range
	ts := e.Timestamp.Unix()
	if opts.FromTime > 0 && ts < opts.FromTime {
		return false
	}
	if opts.ToTime > 0 && ts > opts.ToTime {
		return false
	}

	return true
}

// CountEvents returns the number of events for a run.
func (s *EventStore) CountEvents(ctx context.Context, runID string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return int64(len(s.events[runID])), nil
}

// ListRuns returns all run IDs with events in the store.
func (s *EventStore) ListRuns(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	runs := make([]string, 0, len(s.events))
	for runID := range s.events {
		runs = append(runs, runID)
	}

	return runs, nil
}

// Clear removes all events from the store.
func (s *EventStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close all subscriber channels
	for _, subs := range s.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}

	s.events = make(map[string][]event.Event)
	s.subscribers = make(map[string][]chan event.Event)
	s.sequences = make(map[string]uint64)
}

// DeleteRun removes all events for a specific run.
func (s *EventStore) DeleteRun(ctx context.Context, runID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Close subscriber channels for this run
	if subs, ok := s.subscribers[runID]; ok {
		for _, ch := range subs {
			close(ch)
		}
		delete(s.subscribers, runID)
	}

	delete(s.events, runID)
	delete(s.sequences, runID)

	return nil
}

// Len returns the total number of events across all runs.
func (s *EventStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	for _, events := range s.events {
		count += len(events)
	}
	return count
}

// Ensure EventStore implements event.Store and event.Querier
var (
	_ event.Store   = (*EventStore)(nil)
	_ event.Querier = (*EventStore)(nil)
)
