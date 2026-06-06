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
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// ===============================
// Loop Detector Tests
// ===============================

func TestNewLoopDetector_Defaults(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewLoopDetector(eventStore, runStore)

	if detector.minLoopLength != 2 {
		t.Errorf("expected default minLoopLength 2, got %d", detector.minLoopLength)
	}
	if detector.minOccurrences != 2 {
		t.Errorf("expected default minOccurrences 2, got %d", detector.minOccurrences)
	}
	if detector.maxLoopLength != 6 {
		t.Errorf("expected default maxLoopLength 6, got %d", detector.maxLoopLength)
	}
}

func TestNewLoopDetector_WithOptions(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewLoopDetector(
		eventStore,
		runStore,
		WithLoopMinLength(3),
		WithLoopMinOccurrences(5),
		WithMaxLoopLength(8),
	)

	if detector.minLoopLength != 3 {
		t.Errorf("expected minLoopLength 3, got %d", detector.minLoopLength)
	}
	if detector.minOccurrences != 5 {
		t.Errorf("expected minOccurrences 5, got %d", detector.minOccurrences)
	}
	if detector.maxLoopLength != 8 {
		t.Errorf("expected maxLoopLength 8, got %d", detector.maxLoopLength)
	}
}

func TestLoopDetector_Types(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewLoopDetector(eventStore, runStore)

	types := detector.Types()

	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
	if types[0] != pattern.PatternTypeStateLoop {
		t.Errorf("expected PatternTypeStateLoop, got %s", types[0])
	}
}

func TestLoopDetector_Detect_FindsRepeatedStateSequence(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with the same state loop: A→B→A→B
	for i := 0; i < 3; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
		})
	}

	detector := NewLoopDetector(eventStore, runStore, WithLoopMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one loop pattern to be detected")
	}

	foundLoop := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeStateLoop {
			foundLoop = true
			if p.Frequency < 2 {
				t.Errorf("expected frequency >= 2, got %d", p.Frequency)
			}
		}
	}
	if !foundLoop {
		t.Error("expected to find a state loop pattern")
	}
}

func TestLoopDetector_Detect_LinearSequenceNoLoop(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with linear progression (no loops): A→B→C→D
	for i := 0; i < 3; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateIntake,
			agent.StateExplore,
			agent.StateDecide,
			agent.StateAct,
		})
	}

	detector := NewLoopDetector(eventStore, runStore, WithLoopMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Linear sequences should not produce loop patterns
	if len(patterns) > 0 {
		t.Errorf("expected no loop patterns for linear sequence, got %d", len(patterns))
	}
}

func TestLoopDetector_Detect_DetectsLongerLoops(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with 3-state loop: A→B→C→A→B→C
	for i := 0; i < 3; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateAct,
			agent.StateExplore,
			agent.StateDecide,
			agent.StateAct,
		})
	}

	detector := NewLoopDetector(
		eventStore,
		runStore,
		WithLoopMinLength(3),
		WithLoopMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundLoop := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeStateLoop {
			foundLoop = true
			// Verify the loop data contains 3 states
			var loopData pattern.StateLoopData
			if err := p.GetData(&loopData); err == nil {
				if len(loopData.Loop) != 3 {
					t.Errorf("expected loop length 3, got %d", len(loopData.Loop))
				}
			}
		}
	}
	if !foundLoop {
		t.Error("expected to find a 3-state loop pattern")
	}
}

func TestLoopDetector_Detect_DetectsMultipleIterations(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with 3 iterations of the same loop: A→B→A→B→A→B
	for i := 0; i < 2; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
		})
	}

	detector := NewLoopDetector(eventStore, runStore, WithLoopMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundLoop := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeStateLoop {
			foundLoop = true
			var loopData pattern.StateLoopData
			if err := p.GetData(&loopData); err == nil {
				if loopData.Iterations < 3 {
					t.Errorf("expected at least 3 iterations, got %d", loopData.Iterations)
				}
			}
		}
	}
	if !foundLoop {
		t.Error("expected to find loop with multiple iterations")
	}
}

func TestLoopDetector_Detect_MinLoopLength(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create 2-state loops
	for i := 0; i < 3; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
		})
	}

	// With minLoopLength=3, should not detect 2-state loops
	detector := NewLoopDetector(
		eventStore,
		runStore,
		WithLoopMinLength(3),
		WithLoopMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 0 {
		t.Errorf("expected no patterns with minLoopLength=3, got %d", len(patterns))
	}
}

func TestLoopDetector_Detect_MinOccurrences(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create only 2 runs with loop
	for i := 0; i < 2; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
		})
	}

	// With minOccurrences=3, should not detect
	detector := NewLoopDetector(
		eventStore,
		runStore,
		WithLoopMinOccurrences(3),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 0 {
		t.Errorf("expected no patterns with minOccurrences=3, got %d", len(patterns))
	}
}

func TestLoopDetector_Detect_MaxLoopLength(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create 4-state loops
	for i := 0; i < 3; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateIntake,
			agent.StateExplore,
			agent.StateDecide,
			agent.StateAct,
			agent.StateIntake,
			agent.StateExplore,
			agent.StateDecide,
			agent.StateAct,
		})
	}

	// With maxLoopLength=3, should not detect 4-state loops
	detector := NewLoopDetector(
		eventStore,
		runStore,
		WithMaxLoopLength(3),
		WithLoopMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that no 4-state loop was detected
	for _, p := range patterns {
		var loopData pattern.StateLoopData
		if err := p.GetData(&loopData); err == nil {
			if len(loopData.Loop) > 3 {
				t.Errorf("expected no loops longer than 3 states, got %d", len(loopData.Loop))
			}
		}
	}
}

func TestLoopDetector_Detect_EmptyRuns(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewLoopDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for empty store, got %d", len(patterns))
	}
}

func TestLoopDetector_Detect_NoEvents(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create run without events
	r := agent.NewRun("run-1", "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	detector := NewLoopDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for run with no events, got %d", len(patterns))
	}
}

func TestLoopDetector_Detect_FiltersByRunIDs(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with loops
	runID1 := createTestRunWithStateSequence(ctx, t, eventStore, runStore, 1, []agent.State{
		agent.StateExplore,
		agent.StateDecide,
		agent.StateExplore,
		agent.StateDecide,
	})
	runID2 := createTestRunWithStateSequence(ctx, t, eventStore, runStore, 2, []agent.State{
		agent.StateExplore,
		agent.StateDecide,
		agent.StateExplore,
		agent.StateDecide,
	})
	createTestRunWithStateSequence(ctx, t, eventStore, runStore, 3, []agent.State{
		agent.StateExplore,
		agent.StateDecide,
		agent.StateExplore,
		agent.StateDecide,
	})

	detector := NewLoopDetector(eventStore, runStore, WithLoopMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		RunIDs: []string{runID1, runID2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that evidence only comes from filtered runs
	for _, p := range patterns {
		for _, e := range p.Evidence {
			if e.RunID != runID1 && e.RunID != runID2 {
				t.Errorf("found evidence from excluded run: %s", e.RunID)
			}
		}
	}
}

func TestLoopDetector_Detect_FiltersByConfidence(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with loops
	for i := 0; i < 3; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
		})
	}

	detector := NewLoopDetector(eventStore, runStore, WithLoopMinOccurrences(2))

	// High confidence filter
	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		MinConfidence: 0.99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range patterns {
		if p.Confidence < 0.99 {
			t.Errorf("expected pattern confidence >= 0.99, got %f", p.Confidence)
		}
	}
}

func TestLoopDetector_Detect_AppliesLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create multiple different loop patterns
	for i := 0; i < 5; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
		})
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i+100, []agent.State{
			agent.StateAct,
			agent.StateValidate,
			agent.StateAct,
			agent.StateValidate,
		})
	}

	detector := NewLoopDetector(eventStore, runStore, WithLoopMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 1 {
		t.Errorf("expected at most 1 pattern with limit, got %d", len(patterns))
	}
}

func TestLoopDetector_Detect_ExitStateTracking(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with loop that exits to Done
	for i := 0; i < 3; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
			agent.StateDone, // Exit state
		})
	}

	detector := NewLoopDetector(eventStore, runStore, WithLoopMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundLoop := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeStateLoop {
			foundLoop = true
			var loopData pattern.StateLoopData
			if err := p.GetData(&loopData); err == nil {
				if loopData.ExitState != agent.StateDone {
					t.Errorf("expected exit state Done, got %s", loopData.ExitState)
				}
			}
		}
	}
	if !foundLoop {
		t.Error("expected to find loop with exit state")
	}
}

func TestLoopDetector_Detect_FiltersByTimeRange(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	now := time.Now()

	// Create old run
	r1 := agent.NewRun("run-old", "test goal")
	r1.Status = agent.RunStatusCompleted
	r1.StartTime = now.Add(-2 * time.Hour)
	if err := runStore.Save(ctx, r1); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Create recent run
	r2 := agent.NewRun("run-recent", "test goal")
	r2.Status = agent.RunStatusCompleted
	r2.StartTime = now
	if err := runStore.Save(ctx, r2); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Add events to both
	for _, r := range []*agent.Run{r1, r2} {
		createEventsForRun(ctx, t, eventStore, r.ID, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
			agent.StateExplore,
			agent.StateDecide,
		}, now)
	}

	detector := NewLoopDetector(eventStore, runStore)

	// Filter to only recent runs
	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		FromTime: now.Add(-30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not detect patterns (only 1 run in time range)
	if len(patterns) > 0 {
		// Check that evidence doesn't include old run
		for _, p := range patterns {
			for _, e := range p.Evidence {
				if e.RunID == "run-old" {
					t.Error("expected to filter out old run")
				}
			}
		}
	}
}

func TestLoopDetector_Detect_ShortSequenceTooShort(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with sequences too short to contain loops
	for i := 0; i < 3; i++ {
		createTestRunWithStateSequence(ctx, t, eventStore, runStore, i, []agent.State{
			agent.StateExplore,
			agent.StateDecide,
		})
	}

	detector := NewLoopDetector(eventStore, runStore, WithLoopMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single occurrence of pattern is not a loop
	if len(patterns) > 0 {
		t.Errorf("expected no patterns for sequences with single occurrence, got %d", len(patterns))
	}
}

// ===============================
// Loop Detection Helper Functions
// ===============================

func TestFindLoops(t *testing.T) {
	tests := []struct {
		name          string
		states        []agent.State
		loopLen       int
		expectedLoops int
		expectedIters int
	}{
		{
			name: "simple 2-state loop",
			states: []agent.State{
				agent.StateExplore,
				agent.StateDecide,
				agent.StateExplore,
				agent.StateDecide,
			},
			loopLen:       2,
			expectedLoops: 1,
			expectedIters: 2,
		},
		{
			name: "3 iterations",
			states: []agent.State{
				agent.StateExplore,
				agent.StateDecide,
				agent.StateExplore,
				agent.StateDecide,
				agent.StateExplore,
				agent.StateDecide,
			},
			loopLen:       2,
			expectedLoops: 1,
			expectedIters: 3,
		},
		{
			name: "no loop",
			states: []agent.State{
				agent.StateIntake,
				agent.StateExplore,
				agent.StateDecide,
				agent.StateAct,
			},
			loopLen:       2,
			expectedLoops: 0,
			expectedIters: 0,
		},
		{
			name: "3-state loop",
			states: []agent.State{
				agent.StateExplore,
				agent.StateDecide,
				agent.StateAct,
				agent.StateExplore,
				agent.StateDecide,
				agent.StateAct,
			},
			loopLen:       3,
			expectedLoops: 1,
			expectedIters: 2,
		},
		{
			name: "partial match not loop",
			states: []agent.State{
				agent.StateExplore,
				agent.StateDecide,
				agent.StateExplore,
				agent.StateAct, // Different, breaks loop
			},
			loopLen:       2,
			expectedLoops: 0,
			expectedIters: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loops := findLoops(tt.states, tt.loopLen)

			if len(loops) != tt.expectedLoops {
				t.Errorf("expected %d loops, got %d", tt.expectedLoops, len(loops))
			}

			if len(loops) > 0 && tt.expectedIters > 0 {
				if loops[0].iterations != tt.expectedIters {
					t.Errorf("expected %d iterations, got %d", tt.expectedIters, loops[0].iterations)
				}
			}
		})
	}
}

func TestExtractStateTransitions(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()

	payload1 := event.StateTransitionedPayload{
		FromState: agent.StateIntake,
		ToState:   agent.StateExplore,
	}
	payload1Bytes, _ := json.Marshal(payload1)

	payload2 := event.StateTransitionedPayload{
		FromState: agent.StateExplore,
		ToState:   agent.StateDecide,
	}
	payload2Bytes, _ := json.Marshal(payload2)

	events := []event.Event{
		{
			RunID:     "run-1",
			Type:      event.TypeStateTransitioned,
			Timestamp: time.Now(),
			Payload:   payload1Bytes,
		},
		{
			RunID:     "run-1",
			Type:      event.TypeStateTransitioned,
			Timestamp: time.Now(),
			Payload:   payload2Bytes,
		},
		{
			RunID:     "run-1",
			Type:      event.TypeToolCalled,
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{}`),
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	loadedEvents, err := eventStore.LoadEvents(ctx, "run-1")
	if err != nil {
		t.Fatalf("failed to load events: %v", err)
	}

	states := extractStateTransitions(loadedEvents)

	if len(states) != 2 {
		t.Errorf("expected 2 state transitions, got %d", len(states))
	}
	if len(states) >= 2 {
		if states[0] != agent.StateExplore {
			t.Errorf("expected first state Explore, got %s", states[0])
		}
		if states[1] != agent.StateDecide {
			t.Errorf("expected second state Decide, got %s", states[1])
		}
	}
}

func TestCalculateLoopConfidence(t *testing.T) {
	tests := []struct {
		name        string
		occurrences []loopOccurrence
		minExpected float64
		maxExpected float64
	}{
		{
			name: "single occurrence",
			occurrences: []loopOccurrence{
				{runID: "run1", iterations: 2},
			},
			minExpected: 0.3,
			maxExpected: 0.6,
		},
		{
			name: "consistent iterations",
			occurrences: []loopOccurrence{
				{runID: "run1", iterations: 3},
				{runID: "run2", iterations: 3},
				{runID: "run3", iterations: 3},
			},
			minExpected: 0.7,
			maxExpected: 0.95,
		},
		{
			name: "inconsistent iterations",
			occurrences: []loopOccurrence{
				{runID: "run1", iterations: 2},
				{runID: "run2", iterations: 10},
				{runID: "run3", iterations: 3},
			},
			minExpected: 0.3,
			maxExpected: 0.8,
		},
		{
			name: "many occurrences",
			occurrences: []loopOccurrence{
				{runID: "run1", iterations: 2},
				{runID: "run2", iterations: 2},
				{runID: "run3", iterations: 2},
				{runID: "run4", iterations: 2},
				{runID: "run5", iterations: 2},
			},
			minExpected: 0.8,
			maxExpected: 0.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateLoopConfidence(tt.occurrences)

			if confidence < tt.minExpected || confidence > tt.maxExpected {
				t.Errorf("expected confidence in [%f, %f], got %f", tt.minExpected, tt.maxExpected, confidence)
			}
		})
	}
}

func TestCalculateAverageIterations(t *testing.T) {
	tests := []struct {
		name        string
		occurrences []loopOccurrence
		expected    int
	}{
		{
			name:        "empty",
			occurrences: []loopOccurrence{},
			expected:    0,
		},
		{
			name: "single",
			occurrences: []loopOccurrence{
				{iterations: 5},
			},
			expected: 5,
		},
		{
			name: "multiple",
			occurrences: []loopOccurrence{
				{iterations: 2},
				{iterations: 4},
				{iterations: 6},
			},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAverageIterations(tt.occurrences)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestFindMostCommonExitState(t *testing.T) {
	tests := []struct {
		name        string
		occurrences []loopOccurrence
		expected    agent.State
	}{
		{
			name:        "empty",
			occurrences: []loopOccurrence{},
			expected:    "",
		},
		{
			name: "single exit state",
			occurrences: []loopOccurrence{
				{exitState: agent.StateDone},
				{exitState: agent.StateDone},
				{exitState: agent.StateDone},
			},
			expected: agent.StateDone,
		},
		{
			name: "multiple exit states",
			occurrences: []loopOccurrence{
				{exitState: agent.StateDone},
				{exitState: agent.StateFailed},
				{exitState: agent.StateDone},
			},
			expected: agent.StateDone,
		},
		{
			name: "empty exit states ignored",
			occurrences: []loopOccurrence{
				{exitState: ""},
				{exitState: agent.StateAct},
				{exitState: ""},
			},
			expected: agent.StateAct,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMostCommonExitState(tt.occurrences)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestStateSequenceKey(t *testing.T) {
	states := []agent.State{
		agent.StateExplore,
		agent.StateDecide,
		agent.StateAct,
	}

	key := stateSequenceKey(states)

	expected := "explore->decide->act"
	if key != expected {
		t.Errorf("expected key %q, got %q", expected, key)
	}
}

// ===============================
// Helper Functions for Tests
// ===============================

func createTestRunWithStateSequence(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int, states []agent.State) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now().Add(-time.Duration(index) * time.Hour)
	createEventsForRun(ctx, t, eventStore, r.ID, states, baseTime)

	return r.ID
}

func createEventsForRun(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runID string, states []agent.State, baseTime time.Time) {
	t.Helper()

	events := make([]event.Event, 0)
	for i, toState := range states {
		var fromState agent.State
		if i > 0 {
			fromState = states[i-1]
		}

		payload := event.StateTransitionedPayload{
			FromState: fromState,
			ToState:   toState,
		}
		payloadBytes, _ := json.Marshal(payload)

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeStateTransitioned,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Payload:   payloadBytes,
		})
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}
}
