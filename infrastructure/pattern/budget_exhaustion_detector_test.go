package pattern

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/run"
)

func TestNewBudgetExhaustionDetector(t *testing.T) {
	eventStore := newMockEventStore()
	runStore := newMockRunStore()

	d := NewBudgetExhaustionDetector(eventStore, runStore)
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
	if d.minOccurrences != 2 {
		t.Errorf("expected default minOccurrences 2, got %d", d.minOccurrences)
	}
	if d.warningRatio != 0.8 {
		t.Errorf("expected default warningRatio 0.8, got %f", d.warningRatio)
	}
}

func TestBudgetExhaustionDetector_WithOptions(t *testing.T) {
	eventStore := newMockEventStore()
	runStore := newMockRunStore()

	d := NewBudgetExhaustionDetector(eventStore, runStore,
		WithBudgetMinOccurrences(5),
		WithWarningRatio(0.9),
	)

	if d.minOccurrences != 5 {
		t.Errorf("expected minOccurrences 5, got %d", d.minOccurrences)
	}
	if d.warningRatio != 0.9 {
		t.Errorf("expected warningRatio 0.9, got %f", d.warningRatio)
	}
}

func TestBudgetExhaustionDetector_Types(t *testing.T) {
	eventStore := newMockEventStore()
	runStore := newMockRunStore()
	d := NewBudgetExhaustionDetector(eventStore, runStore)

	types := d.Types()
	if len(types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(types))
	}
	if types[0] != pattern.PatternTypeBudgetExhaustion {
		t.Errorf("expected %s, got %s", pattern.PatternTypeBudgetExhaustion, types[0])
	}
}

func TestBudgetExhaustionDetector_Detect_NoRuns(t *testing.T) {
	eventStore := newMockEventStore()
	runStore := newMockRunStore()
	d := NewBudgetExhaustionDetector(eventStore, runStore)

	patterns, err := d.Detect(context.Background(), pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(patterns))
	}
}

func TestBudgetExhaustionDetector_Detect_WithExhaustions(t *testing.T) {
	ctx := context.Background()
	eventStore := newMockEventStore()
	runStore := newMockRunStore()

	// Add runs
	now := time.Now()
	runs := []*agent.Run{
		{ID: "run-1", StartTime: now.Add(-3 * time.Hour)},
		{ID: "run-2", StartTime: now.Add(-2 * time.Hour)},
		{ID: "run-3", StartTime: now.Add(-1 * time.Hour)},
	}
	for _, r := range runs {
		runStore.runs[r.ID] = r
	}

	// Add budget exhaustion events
	for _, runID := range []string{"run-1", "run-2", "run-3"} {
		eventStore.events[runID] = []event.Event{
			makeBudgetExhaustedEvent(runID, "tool_calls", now),
		}
	}

	d := NewBudgetExhaustionDetector(eventStore, runStore, WithBudgetMinOccurrences(2))

	patterns, err := d.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}

	p := patterns[0]
	if p.Type != pattern.PatternTypeBudgetExhaustion {
		t.Errorf("expected type %s, got %s", pattern.PatternTypeBudgetExhaustion, p.Type)
	}
	if p.Frequency != 3 {
		t.Errorf("expected frequency 3, got %d", p.Frequency)
	}
}

func TestBudgetExhaustionDetector_Detect_MultipleBudgets(t *testing.T) {
	ctx := context.Background()
	eventStore := newMockEventStore()
	runStore := newMockRunStore()

	now := time.Now()
	runs := []*agent.Run{
		{ID: "run-1", StartTime: now.Add(-3 * time.Hour)},
		{ID: "run-2", StartTime: now.Add(-2 * time.Hour)},
		{ID: "run-3", StartTime: now.Add(-1 * time.Hour)},
	}
	for _, r := range runs {
		runStore.runs[r.ID] = r
	}

	// Add different budget exhaustion events
	eventStore.events["run-1"] = []event.Event{
		makeBudgetExhaustedEvent("run-1", "tool_calls", now),
		makeBudgetExhaustedEvent("run-1", "tokens", now),
	}
	eventStore.events["run-2"] = []event.Event{
		makeBudgetExhaustedEvent("run-2", "tool_calls", now),
	}
	eventStore.events["run-3"] = []event.Event{
		makeBudgetExhaustedEvent("run-3", "tokens", now),
	}

	d := NewBudgetExhaustionDetector(eventStore, runStore, WithBudgetMinOccurrences(2))

	patterns, err := d.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
}

func TestBudgetExhaustionDetector_Detect_BelowThreshold(t *testing.T) {
	ctx := context.Background()
	eventStore := newMockEventStore()
	runStore := newMockRunStore()

	now := time.Now()
	runStore.runs["run-1"] = &agent.Run{ID: "run-1", StartTime: now}

	eventStore.events["run-1"] = []event.Event{
		makeBudgetExhaustedEvent("run-1", "tool_calls", now),
	}

	// minOccurrences is 2, only 1 exhaustion
	d := NewBudgetExhaustionDetector(eventStore, runStore, WithBudgetMinOccurrences(2))

	patterns, err := d.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns below threshold, got %d", len(patterns))
	}
}

func TestBudgetExhaustionDetector_Detect_WithLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := newMockEventStore()
	runStore := newMockRunStore()

	now := time.Now()
	for i := 0; i < 5; i++ {
		runID := "run-" + string(rune('a'+i))
		runStore.runs[runID] = &agent.Run{ID: runID, StartTime: now}
		eventStore.events[runID] = []event.Event{
			makeBudgetExhaustedEvent(runID, "budget-a", now),
			makeBudgetExhaustedEvent(runID, "budget-b", now),
			makeBudgetExhaustedEvent(runID, "budget-c", now),
		}
	}

	d := NewBudgetExhaustionDetector(eventStore, runStore, WithBudgetMinOccurrences(1))

	patterns, err := d.Detect(ctx, pattern.DetectionOptions{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 2 {
		t.Errorf("expected 2 patterns with limit, got %d", len(patterns))
	}
}

func TestBudgetExhaustionDetector_Detect_WithMinConfidence(t *testing.T) {
	ctx := context.Background()
	eventStore := newMockEventStore()
	runStore := newMockRunStore()

	now := time.Now()
	runStore.runs["run-1"] = &agent.Run{ID: "run-1", StartTime: now}
	runStore.runs["run-2"] = &agent.Run{ID: "run-2", StartTime: now}

	eventStore.events["run-1"] = []event.Event{
		makeBudgetExhaustedEvent("run-1", "tool_calls", now),
	}
	eventStore.events["run-2"] = []event.Event{
		makeBudgetExhaustedEvent("run-2", "tool_calls", now),
	}

	d := NewBudgetExhaustionDetector(eventStore, runStore, WithBudgetMinOccurrences(1))

	// High min confidence should filter out low-confidence patterns
	patterns, err := d.Detect(ctx, pattern.DetectionOptions{MinConfidence: 0.99})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns with high confidence threshold, got %d", len(patterns))
	}
}

func TestBudgetExhaustionDetector_TrendDetection(t *testing.T) {
	ctx := context.Background()
	eventStore := newMockEventStore()
	runStore := newMockRunStore()

	now := time.Now()

	// Create runs with accelerating exhaustion pattern
	for i := 0; i < 6; i++ {
		runID := "run-" + string(rune('a'+i))
		// Events get closer together over time (increasing frequency)
		eventTime := now.Add(-time.Duration(6-i) * time.Hour)
		if i >= 3 {
			eventTime = now.Add(-time.Duration(6-i) * 30 * time.Minute) // Closer together
		}
		runStore.runs[runID] = &agent.Run{ID: runID, StartTime: eventTime}
		eventStore.events[runID] = []event.Event{
			makeBudgetExhaustedEventAt("tool_calls", eventTime),
		}
	}

	d := NewBudgetExhaustionDetector(eventStore, runStore, WithBudgetMinOccurrences(1))

	patterns, err := d.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}

	// Check extended data for trend
	var data BudgetExhaustionExtendedData
	if err := patterns[0].GetData(&data); err != nil {
		t.Fatalf("failed to get data: %v", err)
	}

	// Trend should be detected (either increasing or stable depending on time intervals)
	if data.Trend == "" {
		t.Error("expected trend to be set")
	}
}

func TestBudgetExhaustionDetector_Recommendations(t *testing.T) {
	tests := []struct {
		name            string
		exhaustionCount int
		totalRuns       int
		trend           string
		nearMissCount   int
		expectContains  string
	}{
		{
			name:            "critical rate",
			exhaustionCount: 6,
			totalRuns:       10,
			trend:           "stable",
			expectContains:  "Critical",
		},
		{
			name:            "warning rate",
			exhaustionCount: 3,
			totalRuns:       10,
			trend:           "stable",
			expectContains:  "Warning",
		},
		{
			name:            "increasing trend",
			exhaustionCount: 1,
			totalRuns:       10,
			trend:           "increasing",
			expectContains:  "Monitor",
		},
		{
			name:            "many near misses",
			exhaustionCount: 1,
			totalRuns:       10,
			trend:           "stable",
			nearMissCount:   5,
			expectContains:  "Optimize",
		},
	}

	d := &BudgetExhaustionDetector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &budgetUsageStats{
				name:            "test_budget",
				exhaustionCount: tt.exhaustionCount,
				totalRuns:       tt.totalRuns,
				nearMissCount:   tt.nearMissCount,
			}

			recommendation := d.generateRecommendation(s, tt.trend)
			if recommendation == "" {
				t.Error("expected non-empty recommendation")
			}
			// Check that recommendation contains expected keyword
			// This is a simple check - in real tests we might be more specific
			if tt.expectContains != "" && !containsIgnoreCase(recommendation, tt.expectContains) {
				t.Errorf("expected recommendation to contain %q, got: %s", tt.expectContains, recommendation)
			}
		})
	}
}

func TestAverageInterval(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		events   []budgetExhaustionEvent
		expected time.Duration
	}{
		{
			name:     "empty events",
			events:   []budgetExhaustionEvent{},
			expected: 0,
		},
		{
			name: "single event",
			events: []budgetExhaustionEvent{
				{timestamp: now},
			},
			expected: 0,
		},
		{
			name: "two events",
			events: []budgetExhaustionEvent{
				{timestamp: now},
				{timestamp: now.Add(time.Hour)},
			},
			expected: time.Hour,
		},
		{
			name: "three events",
			events: []budgetExhaustionEvent{
				{timestamp: now},
				{timestamp: now.Add(time.Hour)},
				{timestamp: now.Add(3 * time.Hour)},
			},
			expected: 90 * time.Minute, // (1h + 2h) / 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := averageInterval(tt.events)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

// Helper functions for tests

func makeBudgetExhaustedEvent(runID, budgetName string, ts time.Time) event.Event {
	return makeBudgetExhaustedEventAt(budgetName, ts)
}

func makeBudgetExhaustedEventAt(budgetName string, ts time.Time) event.Event {
	payload := event.BudgetExhaustedPayload{
		BudgetName: budgetName,
	}
	payloadBytes, _ := json.Marshal(payload)
	return event.Event{
		Type:      event.TypeBudgetExhausted,
		Timestamp: ts,
		Payload:   payloadBytes,
	}
}

// Mock stores for testing

type mockEventStore struct {
	events map[string][]event.Event
}

func newMockEventStore() *mockEventStore {
	return &mockEventStore{
		events: make(map[string][]event.Event),
	}
}

func (m *mockEventStore) Append(ctx context.Context, events ...event.Event) error {
	for _, e := range events {
		m.events[e.RunID] = append(m.events[e.RunID], e)
	}
	return nil
}

func (m *mockEventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	return m.events[runID], nil
}

func (m *mockEventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	events := m.events[runID]
	if int(fromSeq) >= len(events) {
		return nil, nil
	}
	return events[fromSeq:], nil
}

func (m *mockEventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	ch := make(chan event.Event)
	close(ch)
	return ch, nil
}

type mockRunStore struct {
	runs map[string]*agent.Run
}

func newMockRunStore() *mockRunStore {
	return &mockRunStore{
		runs: make(map[string]*agent.Run),
	}
}

func (m *mockRunStore) Save(ctx context.Context, r *agent.Run) error {
	m.runs[r.ID] = r
	return nil
}

func (m *mockRunStore) Get(ctx context.Context, id string) (*agent.Run, error) {
	r, ok := m.runs[id]
	if !ok {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	return r, nil
}

func (m *mockRunStore) Update(ctx context.Context, r *agent.Run) error {
	m.runs[r.ID] = r
	return nil
}

func (m *mockRunStore) Delete(ctx context.Context, id string) error {
	delete(m.runs, id)
	return nil
}

func (m *mockRunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	var result []*agent.Run
	for _, r := range m.runs {
		// Apply time filters if set
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

func (m *mockRunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	runs, err := m.List(ctx, filter)
	if err != nil {
		return 0, err
	}
	return int64(len(runs)), nil
}
