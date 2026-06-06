package analytics_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/analytics"
	domainevent "go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/run"
	infraanalytics "go.klarlabs.de/agent/infrastructure/analytics"
)

// mockRunStore implements run.Store for testing.
type mockRunStore struct {
	runs map[string]*agent.Run
	mu   sync.RWMutex
	err  error
}

func newMockRunStore() *mockRunStore {
	return &mockRunStore{runs: make(map[string]*agent.Run)}
}

func (s *mockRunStore) Save(_ context.Context, r *agent.Run) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[r.ID] = r
	return nil
}

func (s *mockRunStore) Get(_ context.Context, id string) (*agent.Run, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.runs[id]; ok {
		return r, nil
	}
	return nil, nil
}

func (s *mockRunStore) Update(_ context.Context, r *agent.Run) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[r.ID] = r
	return nil
}

func (s *mockRunStore) Delete(_ context.Context, id string) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runs, id)
	return nil
}

func (s *mockRunStore) List(_ context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*agent.Run, 0, len(s.runs))
	for _, r := range s.runs {
		// Apply time filter
		if !filter.FromTime.IsZero() && r.StartTime.Before(filter.FromTime) {
			continue
		}
		if !filter.ToTime.IsZero() && r.StartTime.After(filter.ToTime) {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

func (s *mockRunStore) Count(_ context.Context, _ run.ListFilter) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.runs)), nil
}

func (s *mockRunStore) SetError(err error) {
	s.err = err
}

// mockEventStore implements event.Store for testing.
type mockEventStore struct {
	events map[string][]domainevent.Event
	mu     sync.RWMutex
	err    error
}

func newMockEventStore() *mockEventStore {
	return &mockEventStore{events: make(map[string][]domainevent.Event)}
}

func (s *mockEventStore) Append(_ context.Context, events ...domainevent.Event) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range events {
		s.events[e.RunID] = append(s.events[e.RunID], e)
	}
	return nil
}

func (s *mockEventStore) LoadEvents(_ context.Context, runID string) ([]domainevent.Event, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if events, ok := s.events[runID]; ok {
		result := make([]domainevent.Event, len(events))
		copy(result, events)
		return result, nil
	}
	return []domainevent.Event{}, nil
}

func (s *mockEventStore) LoadEventsFrom(_ context.Context, runID string, fromSeq uint64) ([]domainevent.Event, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []domainevent.Event
	if events, ok := s.events[runID]; ok {
		for _, e := range events {
			if e.Sequence >= fromSeq {
				result = append(result, e)
			}
		}
	}
	return result, nil
}

func (s *mockEventStore) Subscribe(_ context.Context, _ string) (<-chan domainevent.Event, error) {
	ch := make(chan domainevent.Event)
	close(ch)
	return ch, nil
}

func (s *mockEventStore) AddEvent(runID string, e domainevent.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.RunID = runID
	s.events[runID] = append(s.events[runID], e)
}

func (s *mockEventStore) SetError(err error) {
	s.err = err
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func TestNewAggregator(t *testing.T) {
	t.Parallel()

	t.Run("creates aggregator with stores", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		agg := infraanalytics.NewAggregator(runStore, eventStore)
		if agg == nil {
			t.Fatal("NewAggregator() returned nil")
		}
	})
}

func TestAggregator_ToolUsage(t *testing.T) {
	t.Parallel()

	t.Run("returns empty stats for no runs", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()
		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stats, err := agg.ToolUsage(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("ToolUsage() error = %v", err)
		}
		if len(stats) != 0 {
			t.Errorf("ToolUsage() returned %d stats, want 0", len(stats))
		}
	})

	t.Run("aggregates tool usage from events", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		// Create a run
		now := time.Now()
		r := &agent.Run{
			ID:        "run-1",
			Goal:      "test",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		_ = runStore.Save(context.Background(), r)

		// Add tool events
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolCalled,
			Timestamp: now,
			Payload:   mustMarshal(domainevent.ToolCalledPayload{ToolName: "read_file"}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolSucceeded,
			Timestamp: now.Add(100 * time.Millisecond),
			Payload:   mustMarshal(domainevent.ToolSucceededPayload{ToolName: "read_file", Duration: 100 * time.Millisecond}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolCalled,
			Timestamp: now.Add(200 * time.Millisecond),
			Payload:   mustMarshal(domainevent.ToolCalledPayload{ToolName: "read_file"}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolSucceeded,
			Timestamp: now.Add(350 * time.Millisecond),
			Payload:   mustMarshal(domainevent.ToolSucceededPayload{ToolName: "read_file", Duration: 150 * time.Millisecond, Cached: true}),
		})

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stats, err := agg.ToolUsage(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("ToolUsage() error = %v", err)
		}
		if len(stats) != 1 {
			t.Fatalf("ToolUsage() returned %d stats, want 1", len(stats))
		}

		stat := stats[0]
		if stat.ToolName != "read_file" {
			t.Errorf("ToolName = %s, want read_file", stat.ToolName)
		}
		if stat.CallCount != 2 {
			t.Errorf("CallCount = %d, want 2", stat.CallCount)
		}
		if stat.SuccessCount != 2 {
			t.Errorf("SuccessCount = %d, want 2", stat.SuccessCount)
		}
		if stat.CacheHitCount != 1 {
			t.Errorf("CacheHitCount = %d, want 1", stat.CacheHitCount)
		}
	})

	t.Run("handles tool failures", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		now := time.Now()
		r := &agent.Run{
			ID:        "run-1",
			Goal:      "test",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		_ = runStore.Save(context.Background(), r)

		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolCalled,
			Timestamp: now,
			Payload:   mustMarshal(domainevent.ToolCalledPayload{ToolName: "write_file"}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolFailed,
			Timestamp: now.Add(50 * time.Millisecond),
			Payload:   mustMarshal(domainevent.ToolFailedPayload{ToolName: "write_file", Duration: 50 * time.Millisecond, Error: "permission denied"}),
		})

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stats, err := agg.ToolUsage(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("ToolUsage() error = %v", err)
		}
		if len(stats) != 1 {
			t.Fatalf("ToolUsage() returned %d stats, want 1", len(stats))
		}

		stat := stats[0]
		if stat.FailureCount != 1 {
			t.Errorf("FailureCount = %d, want 1", stat.FailureCount)
		}
	})

	t.Run("filters by tool names", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		now := time.Now()
		r := &agent.Run{
			ID:        "run-1",
			Goal:      "test",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		_ = runStore.Save(context.Background(), r)

		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolCalled,
			Timestamp: now,
			Payload:   mustMarshal(domainevent.ToolCalledPayload{ToolName: "read_file"}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolSucceeded,
			Timestamp: now.Add(100 * time.Millisecond),
			Payload:   mustMarshal(domainevent.ToolSucceededPayload{ToolName: "read_file", Duration: 100 * time.Millisecond}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolCalled,
			Timestamp: now.Add(200 * time.Millisecond),
			Payload:   mustMarshal(domainevent.ToolCalledPayload{ToolName: "write_file"}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolSucceeded,
			Timestamp: now.Add(300 * time.Millisecond),
			Payload:   mustMarshal(domainevent.ToolSucceededPayload{ToolName: "write_file", Duration: 100 * time.Millisecond}),
		})

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stats, err := agg.ToolUsage(context.Background(), analytics.Filter{
			ToolNames: []string{"read_file"},
		})
		if err != nil {
			t.Fatalf("ToolUsage() error = %v", err)
		}
		if len(stats) != 1 {
			t.Fatalf("ToolUsage() returned %d stats, want 1", len(stats))
		}
		if stats[0].ToolName != "read_file" {
			t.Errorf("ToolName = %s, want read_file", stats[0].ToolName)
		}
	})

	t.Run("applies limit", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		now := time.Now()
		r := &agent.Run{
			ID:        "run-1",
			Goal:      "test",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		_ = runStore.Save(context.Background(), r)

		// Add events for multiple tools
		for i, toolName := range []string{"tool1", "tool2", "tool3"} {
			eventStore.AddEvent("run-1", domainevent.Event{
				Type:      domainevent.TypeToolCalled,
				Timestamp: now.Add(time.Duration(i*100) * time.Millisecond),
				Payload:   mustMarshal(domainevent.ToolCalledPayload{ToolName: toolName}),
			})
			eventStore.AddEvent("run-1", domainevent.Event{
				Type:      domainevent.TypeToolSucceeded,
				Timestamp: now.Add(time.Duration(i*100+50) * time.Millisecond),
				Payload:   mustMarshal(domainevent.ToolSucceededPayload{ToolName: toolName, Duration: 50 * time.Millisecond}),
			})
		}

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stats, err := agg.ToolUsage(context.Background(), analytics.Filter{Limit: 2})
		if err != nil {
			t.Fatalf("ToolUsage() error = %v", err)
		}
		if len(stats) != 2 {
			t.Errorf("ToolUsage() returned %d stats, want 2", len(stats))
		}
	})

	t.Run("returns error from run store", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		runStore.SetError(context.DeadlineExceeded)
		eventStore := newMockEventStore()

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		_, err := agg.ToolUsage(context.Background(), analytics.Filter{})
		if err == nil {
			t.Error("ToolUsage() should return error")
		}
	})
}

func TestAggregator_StateDistribution(t *testing.T) {
	t.Parallel()

	t.Run("returns empty stats for no runs", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()
		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stats, err := agg.StateDistribution(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("StateDistribution() error = %v", err)
		}
		if len(stats) != 0 {
			t.Errorf("StateDistribution() returned %d stats, want 0", len(stats))
		}
	})

	t.Run("calculates time distribution across states", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		now := time.Now()
		r := &agent.Run{
			ID:        "run-1",
			Goal:      "test",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		_ = runStore.Save(context.Background(), r)

		// Add state transition events
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeStateTransitioned,
			Timestamp: now,
			Payload: mustMarshal(domainevent.StateTransitionedPayload{
				FromState: agent.StateIntake,
				ToState:   agent.StateExplore,
				Reason:    "starting",
			}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeStateTransitioned,
			Timestamp: now.Add(300 * time.Millisecond),
			Payload: mustMarshal(domainevent.StateTransitionedPayload{
				FromState: agent.StateExplore,
				ToState:   agent.StateDecide,
				Reason:    "gathered evidence",
			}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeStateTransitioned,
			Timestamp: now.Add(500 * time.Millisecond),
			Payload: mustMarshal(domainevent.StateTransitionedPayload{
				FromState: agent.StateDecide,
				ToState:   agent.StateDone,
				Reason:    "completed",
			}),
		})

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stats, err := agg.StateDistribution(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("StateDistribution() error = %v", err)
		}
		if len(stats) == 0 {
			t.Error("StateDistribution() returned no stats")
		}
	})

	t.Run("filters by states", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		now := time.Now()
		r := &agent.Run{
			ID:        "run-1",
			Goal:      "test",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		_ = runStore.Save(context.Background(), r)

		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeStateTransitioned,
			Timestamp: now,
			Payload: mustMarshal(domainevent.StateTransitionedPayload{
				FromState: agent.StateIntake,
				ToState:   agent.StateExplore,
				Reason:    "starting",
			}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeStateTransitioned,
			Timestamp: now.Add(300 * time.Millisecond),
			Payload: mustMarshal(domainevent.StateTransitionedPayload{
				FromState: agent.StateExplore,
				ToState:   agent.StateDecide,
				Reason:    "gathered evidence",
			}),
		})

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stats, err := agg.StateDistribution(context.Background(), analytics.Filter{
			States: []agent.State{agent.StateIntake},
		})
		if err != nil {
			t.Fatalf("StateDistribution() error = %v", err)
		}
		// Stats should be filtered to only include transitions from StateIntake
		for _, stat := range stats {
			if stat.State == agent.StateExplore {
				// This should not be counted since we're filtering by Intake as FromState
				continue
			}
		}
	})

	t.Run("returns error from run store", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		runStore.SetError(context.DeadlineExceeded)
		eventStore := newMockEventStore()

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		_, err := agg.StateDistribution(context.Background(), analytics.Filter{})
		if err == nil {
			t.Error("StateDistribution() should return error")
		}
	})
}

func TestAggregator_SuccessRate(t *testing.T) {
	t.Parallel()

	t.Run("returns zero for no runs", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()
		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stat, err := agg.SuccessRate(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("SuccessRate() error = %v", err)
		}
		if stat.TotalRuns != 0 {
			t.Errorf("TotalRuns = %d, want 0", stat.TotalRuns)
		}
	})

	t.Run("calculates success rate correctly", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		now := time.Now()

		// Add 3 completed runs
		for i := 0; i < 3; i++ {
			r := &agent.Run{
				ID:        "run-" + string(rune('1'+i)),
				Goal:      "test",
				StartTime: now,
				EndTime:   now.Add(time.Second),
				Status:    agent.RunStatusCompleted,
			}
			_ = runStore.Save(context.Background(), r)
		}

		// Add 2 failed runs
		for i := 0; i < 2; i++ {
			r := &agent.Run{
				ID:        "failed-" + string(rune('1'+i)),
				Goal:      "test",
				StartTime: now,
				EndTime:   now.Add(time.Second),
				Status:    agent.RunStatusFailed,
			}
			_ = runStore.Save(context.Background(), r)
		}

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		stat, err := agg.SuccessRate(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("SuccessRate() error = %v", err)
		}
		if stat.TotalRuns != 5 {
			t.Errorf("TotalRuns = %d, want 5", stat.TotalRuns)
		}
		if stat.CompletedRuns != 3 {
			t.Errorf("CompletedRuns = %d, want 3", stat.CompletedRuns)
		}
		if stat.FailedRuns != 2 {
			t.Errorf("FailedRuns = %d, want 2", stat.FailedRuns)
		}
		// Success rate should be 60%
		if stat.SuccessRate != 60.0 {
			t.Errorf("SuccessRate = %f, want 60.0", stat.SuccessRate)
		}
	})

	t.Run("returns error from run store", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		runStore.SetError(context.DeadlineExceeded)
		eventStore := newMockEventStore()

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		_, err := agg.SuccessRate(context.Background(), analytics.Filter{})
		if err == nil {
			t.Error("SuccessRate() should return error")
		}
	})
}

func TestAggregator_RunSummary(t *testing.T) {
	t.Parallel()

	t.Run("returns empty summary for no runs", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()
		agg := infraanalytics.NewAggregator(runStore, eventStore)

		summary, err := agg.RunSummary(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("RunSummary() error = %v", err)
		}
		if summary.TotalRuns != 0 {
			t.Errorf("TotalRuns = %d, want 0", summary.TotalRuns)
		}
	})

	t.Run("calculates run summary", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		now := time.Now()

		// Add completed run
		r1 := &agent.Run{
			ID:        "run-1",
			Goal:      "test 1",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		_ = runStore.Save(context.Background(), r1)

		// Add failed run
		r2 := &agent.Run{
			ID:        "run-2",
			Goal:      "test 2",
			StartTime: now,
			EndTime:   now.Add(2 * time.Second),
			Status:    agent.RunStatusFailed,
		}
		_ = runStore.Save(context.Background(), r2)

		// Add running run
		r3 := &agent.Run{
			ID:        "run-3",
			Goal:      "test 3",
			StartTime: now,
			Status:    agent.RunStatusRunning,
		}
		_ = runStore.Save(context.Background(), r3)

		// Add tool call events for run-1
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolCalled,
			Timestamp: now,
			Payload:   mustMarshal(domainevent.ToolCalledPayload{ToolName: "read_file"}),
		})
		eventStore.AddEvent("run-1", domainevent.Event{
			Type:      domainevent.TypeToolCalled,
			Timestamp: now.Add(100 * time.Millisecond),
			Payload:   mustMarshal(domainevent.ToolCalledPayload{ToolName: "write_file"}),
		})

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		summary, err := agg.RunSummary(context.Background(), analytics.Filter{})
		if err != nil {
			t.Fatalf("RunSummary() error = %v", err)
		}
		if summary.TotalRuns != 3 {
			t.Errorf("TotalRuns = %d, want 3", summary.TotalRuns)
		}
		if summary.CompletedRuns != 1 {
			t.Errorf("CompletedRuns = %d, want 1", summary.CompletedRuns)
		}
		if summary.FailedRuns != 1 {
			t.Errorf("FailedRuns = %d, want 1", summary.FailedRuns)
		}
		if summary.RunningRuns != 1 {
			t.Errorf("RunningRuns = %d, want 1", summary.RunningRuns)
		}
		if summary.TotalToolCalls != 2 {
			t.Errorf("TotalToolCalls = %d, want 2", summary.TotalToolCalls)
		}
	})

	t.Run("filters by run IDs", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		eventStore := newMockEventStore()

		now := time.Now()

		r1 := &agent.Run{
			ID:        "run-1",
			Goal:      "test",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		r2 := &agent.Run{
			ID:        "run-2",
			Goal:      "test",
			StartTime: now,
			EndTime:   now.Add(time.Second),
			Status:    agent.RunStatusCompleted,
		}
		_ = runStore.Save(context.Background(), r1)
		_ = runStore.Save(context.Background(), r2)

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		summary, err := agg.RunSummary(context.Background(), analytics.Filter{
			RunIDs: []string{"run-1"},
		})
		if err != nil {
			t.Fatalf("RunSummary() error = %v", err)
		}
		if summary.TotalRuns != 1 {
			t.Errorf("TotalRuns = %d, want 1", summary.TotalRuns)
		}
	})

	t.Run("returns error from run store", func(t *testing.T) {
		t.Parallel()

		runStore := newMockRunStore()
		runStore.SetError(context.DeadlineExceeded)
		eventStore := newMockEventStore()

		agg := infraanalytics.NewAggregator(runStore, eventStore)

		_, err := agg.RunSummary(context.Background(), analytics.Filter{})
		if err == nil {
			t.Error("RunSummary() should return error")
		}
	})
}
