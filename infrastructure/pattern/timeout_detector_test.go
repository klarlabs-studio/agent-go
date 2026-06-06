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
// Timeout Detector Tests
// ===============================

func TestNewTimeoutDetector_Defaults(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewTimeoutDetector(eventStore, runStore)

	if detector.minTimeouts != 3 {
		t.Errorf("expected default minTimeouts 3, got %d", detector.minTimeouts)
	}
	if detector.minTimeoutRate != 0.1 {
		t.Errorf("expected default minTimeoutRate 0.1, got %f", detector.minTimeoutRate)
	}
	if len(detector.timeoutKeywords) != 5 {
		t.Errorf("expected 5 default timeout keywords, got %d", len(detector.timeoutKeywords))
	}
}

func TestNewTimeoutDetector_WithOptions(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	customKeywords := []string{"custom_timeout", "custom_deadline"}
	detector := NewTimeoutDetector(
		eventStore,
		runStore,
		WithMinTimeouts(5),
		WithMinTimeoutRate(0.25),
		WithTimeoutKeywords(customKeywords),
	)

	if detector.minTimeouts != 5 {
		t.Errorf("expected minTimeouts 5, got %d", detector.minTimeouts)
	}
	if detector.minTimeoutRate != 0.25 {
		t.Errorf("expected minTimeoutRate 0.25, got %f", detector.minTimeoutRate)
	}
	if len(detector.timeoutKeywords) != 2 {
		t.Errorf("expected 2 custom timeout keywords, got %d", len(detector.timeoutKeywords))
	}
}

func TestTimeoutDetector_Types(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewTimeoutDetector(eventStore, runStore)

	types := detector.Types()

	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
	if types[0] != pattern.PatternTypeTimeout {
		t.Errorf("expected PatternTypeTimeout, got %s", types[0])
	}
}

func TestTimeoutDetector_Detect_FindsToolsWithTimeouts(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with timeout errors
	timeoutErrors := []string{
		"connection timeout exceeded",
		"operation timed out",
		"context deadline exceeded",
	}

	for i, errMsg := range timeoutErrors {
		createTestRunWithTimeout(ctx, t, eventStore, runStore, i, "timeout_tool", errMsg)
	}

	detector := NewTimeoutDetector(eventStore, runStore, WithMinTimeouts(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one timeout pattern")
	}

	// Verify we found a timeout pattern
	foundTimeout := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeTimeout {
			foundTimeout = true

			// Verify pattern data
			var data pattern.TimeoutData
			if err := p.GetData(&data); err != nil {
				t.Fatalf("failed to get timeout data: %v", err)
			}

			if data.ToolName != "timeout_tool" {
				t.Errorf("expected tool name 'timeout_tool', got %s", data.ToolName)
			}
			if data.TimeoutCount < 2 {
				t.Errorf("expected at least 2 timeouts, got %d", data.TimeoutCount)
			}
			if data.TimeoutRate < 0.1 {
				t.Errorf("expected timeout rate >= 0.1, got %f", data.TimeoutRate)
			}
		}
	}
	if !foundTimeout {
		t.Error("expected to find a timeout pattern")
	}
}

func TestTimeoutDetector_Detect_FastExecutionsNoTimeouts(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with successful fast tool execution
	for i := 0; i < 5; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"fast_tool"})
	}

	detector := NewTimeoutDetector(eventStore, runStore, WithMinTimeouts(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not detect any timeout patterns
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeTimeout {
			t.Error("unexpected timeout pattern for fast executions with no timeouts")
		}
	}
}

func TestTimeoutDetector_Detect_MinTimeoutsThreshold(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with only 2 timeouts (below threshold of 3)
	for i := 0; i < 2; i++ {
		createTestRunWithTimeout(ctx, t, eventStore, runStore, i, "rare_timeout_tool", "timeout")
	}

	detector := NewTimeoutDetector(eventStore, runStore) // Default minTimeouts = 3

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not detect pattern because timeouts < minTimeouts
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeTimeout {
			t.Error("unexpected timeout pattern below minTimeouts threshold")
		}
	}
}

func TestTimeoutDetector_Detect_MinTimeoutRateThreshold(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create run with 3 timeouts but 50 total calls (6% timeout rate < 10% threshold)
	runID := "run-low-rate"
	r := agent.NewRun(runID, "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now()
	events := make([]event.Event, 0)

	// Add 3 timeout failures
	for i := 0; i < 3; i++ {
		callPayload := event.ToolCalledPayload{
			ToolName: "low_rate_tool",
			Input:    json.RawMessage(`{}`),
			State:    agent.StateAct,
		}
		callPayloadBytes, _ := json.Marshal(callPayload)

		failPayload := event.ToolFailedPayload{
			ToolName: "low_rate_tool",
			Error:    "timeout occurred",
			Duration: 100 * time.Millisecond,
		}
		failPayloadBytes, _ := json.Marshal(failPayload)

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime.Add(time.Duration(i*2) * time.Second),
			Payload:   callPayloadBytes,
		})

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolFailed,
			Timestamp: baseTime.Add(time.Duration(i*2+1) * time.Second),
			Payload:   failPayloadBytes,
		})
	}

	// Add 47 successful calls (total 50 calls, 3/50 = 6% < 10% threshold)
	for i := 0; i < 47; i++ {
		callPayload := event.ToolCalledPayload{
			ToolName: "low_rate_tool",
			Input:    json.RawMessage(`{}`),
			State:    agent.StateAct,
		}
		callPayloadBytes, _ := json.Marshal(callPayload)

		succPayload := event.ToolSucceededPayload{
			ToolName: "low_rate_tool",
			Output:   json.RawMessage(`{}`),
			Duration: 100 * time.Millisecond,
		}
		succPayloadBytes, _ := json.Marshal(succPayload)

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime.Add(time.Duration((i+3)*2) * time.Second),
			Payload:   callPayloadBytes,
		})

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolSucceeded,
			Timestamp: baseTime.Add(time.Duration((i+3)*2+1) * time.Second),
			Payload:   succPayloadBytes,
		})
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	detector := NewTimeoutDetector(eventStore, runStore) // Default minTimeoutRate = 0.1 (10%)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not detect pattern because timeout rate < minTimeoutRate
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeTimeout {
			t.Error("unexpected timeout pattern below minTimeoutRate threshold")
		}
	}
}

func TestTimeoutDetector_Detect_CustomTimeoutKeywords(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with custom timeout error messages
	customErrors := []string{
		"custom_timeout occurred",
		"operation hit CUSTOM_TIMEOUT",
		"CUSTOM_TIMEOUT error",
	}

	for i, errMsg := range customErrors {
		createTestRunWithTimeout(ctx, t, eventStore, runStore, i, "custom_timeout_tool", errMsg)
	}

	customKeywords := []string{"custom_timeout"}
	detector := NewTimeoutDetector(
		eventStore,
		runStore,
		WithMinTimeouts(2),
		WithTimeoutKeywords(customKeywords),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected to find timeout pattern with custom keywords")
	}

	foundTimeout := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeTimeout {
			foundTimeout = true
		}
	}
	if !foundTimeout {
		t.Error("expected to find timeout pattern with custom keywords")
	}
}

func TestTimeoutDetector_isTimeoutError_VariousErrorStrings(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewTimeoutDetector(eventStore, runStore)

	testCases := []struct {
		errMsg   string
		expected bool
	}{
		{"connection timeout exceeded", true},
		{"operation timed out", true},
		{"context deadline exceeded", true},
		{"request timeout", true},
		{"Context Deadline Exceeded", true}, // Case insensitive
		{"TIMEOUT ERROR", true},
		{"context canceled", true},
		{"normal error message", false},
		{"connection refused", false},
		{"invalid argument", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.errMsg, func(t *testing.T) {
			result := detector.isTimeoutError(tc.errMsg)
			if result != tc.expected {
				t.Errorf("isTimeoutError(%q) = %v, want %v", tc.errMsg, result, tc.expected)
			}
		})
	}
}

func TestTimeoutDetector_Detect_MinConfidenceFiltering(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with timeouts
	for i := 0; i < 3; i++ {
		createTestRunWithTimeout(ctx, t, eventStore, runStore, i, "timeout_tool", "timeout")
	}

	detector := NewTimeoutDetector(eventStore, runStore, WithMinTimeouts(2))

	// Very high confidence requirement
	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		MinConfidence: 0.99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all returned patterns meet confidence threshold
	for _, p := range patterns {
		if p.Confidence < 0.99 {
			t.Errorf("expected confidence >= 0.99, got %f", p.Confidence)
		}
	}
}

func TestTimeoutDetector_Detect_EmptyStore(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewTimeoutDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for empty store, got %d", len(patterns))
	}
}

func TestTimeoutDetector_Detect_EmptyRunsList(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewTimeoutDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for empty runs list, got %d", len(patterns))
	}
}

func TestTimeoutDetector_Detect_FiltersByRunIDs(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with timeouts
	runID1 := createTestRunWithTimeout(ctx, t, eventStore, runStore, 1, "timeout_tool", "timeout")
	runID2 := createTestRunWithTimeout(ctx, t, eventStore, runStore, 2, "timeout_tool", "timeout")
	createTestRunWithTimeout(ctx, t, eventStore, runStore, 3, "timeout_tool", "timeout")

	detector := NewTimeoutDetector(eventStore, runStore, WithMinTimeouts(2))

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

func TestTimeoutDetector_Detect_AppliesLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create multiple tools with timeouts
	for i := 0; i < 5; i++ {
		createTestRunWithTimeout(ctx, t, eventStore, runStore, i, "timeout_tool_a", "timeout")
		createTestRunWithTimeout(ctx, t, eventStore, runStore, i+100, "timeout_tool_b", "timeout")
	}

	detector := NewTimeoutDetector(eventStore, runStore, WithMinTimeouts(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 1 {
		t.Errorf("expected at most 1 pattern with limit, got %d", len(patterns))
	}
}

func TestTimeoutDetector_Detect_CalculatesTimeoutRate(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create run with 5 calls: 3 timeouts, 2 successes (60% timeout rate)
	runID := "run-timeout-rate"
	r := agent.NewRun(runID, "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now()
	events := make([]event.Event, 0)

	// Add 3 timeout failures
	for i := 0; i < 3; i++ {
		callPayload := event.ToolCalledPayload{
			ToolName: "rate_test_tool",
			Input:    json.RawMessage(`{}`),
			State:    agent.StateAct,
		}
		callPayloadBytes, _ := json.Marshal(callPayload)

		failPayload := event.ToolFailedPayload{
			ToolName: "rate_test_tool",
			Error:    "timeout",
			Duration: 100 * time.Millisecond,
		}
		failPayloadBytes, _ := json.Marshal(failPayload)

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime.Add(time.Duration(i*2) * time.Second),
			Payload:   callPayloadBytes,
		})

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolFailed,
			Timestamp: baseTime.Add(time.Duration(i*2+1) * time.Second),
			Payload:   failPayloadBytes,
		})
	}

	// Add 2 successes
	for i := 0; i < 2; i++ {
		callPayload := event.ToolCalledPayload{
			ToolName: "rate_test_tool",
			Input:    json.RawMessage(`{}`),
			State:    agent.StateAct,
		}
		callPayloadBytes, _ := json.Marshal(callPayload)

		succPayload := event.ToolSucceededPayload{
			ToolName: "rate_test_tool",
			Output:   json.RawMessage(`{}`),
			Duration: 100 * time.Millisecond,
		}
		succPayloadBytes, _ := json.Marshal(succPayload)

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime.Add(time.Duration((i+3)*2) * time.Second),
			Payload:   callPayloadBytes,
		})

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolSucceeded,
			Timestamp: baseTime.Add(time.Duration((i+3)*2+1) * time.Second),
			Payload:   succPayloadBytes,
		})
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	detector := NewTimeoutDetector(eventStore, runStore, WithMinTimeouts(2), WithMinTimeoutRate(0.5))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected to find timeout pattern")
	}

	// Verify timeout rate is calculated correctly
	foundTimeout := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeTimeout {
			foundTimeout = true

			var data pattern.TimeoutData
			if err := p.GetData(&data); err != nil {
				t.Fatalf("failed to get timeout data: %v", err)
			}

			expectedRate := 3.0 / 5.0 // 3 timeouts out of 5 calls
			if data.TimeoutRate < expectedRate-0.01 || data.TimeoutRate > expectedRate+0.01 {
				t.Errorf("expected timeout rate ~%.2f, got %.2f", expectedRate, data.TimeoutRate)
			}
		}
	}
	if !foundTimeout {
		t.Error("expected to find timeout pattern")
	}
}

func TestTimeoutDetector_Detect_TracksAvgDuration(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create run with known durations
	runID := "run-duration"
	r := agent.NewRun(runID, "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now()
	events := make([]event.Event, 0)

	durations := []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second}

	for i, duration := range durations {
		callPayload := event.ToolCalledPayload{
			ToolName: "duration_tool",
			Input:    json.RawMessage(`{}`),
			State:    agent.StateAct,
		}
		callPayloadBytes, _ := json.Marshal(callPayload)

		failPayload := event.ToolFailedPayload{
			ToolName: "duration_tool",
			Error:    "timeout",
			Duration: duration,
		}
		failPayloadBytes, _ := json.Marshal(failPayload)

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime.Add(time.Duration(i*4) * time.Second),
			Payload:   callPayloadBytes,
		})

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeToolFailed,
			Timestamp: baseTime.Add(time.Duration(i*4) * time.Second).Add(duration),
			Payload:   failPayloadBytes,
		})
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	detector := NewTimeoutDetector(eventStore, runStore, WithMinTimeouts(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected to find timeout pattern")
	}

	// Verify average duration is calculated correctly
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeTimeout {
			var data pattern.TimeoutData
			if err := p.GetData(&data); err != nil {
				t.Fatalf("failed to get timeout data: %v", err)
			}

			// Average of 1s, 2s, 3s = 2s
			expectedAvg := 2 * time.Second
			if data.AvgDuration < expectedAvg-100*time.Millisecond || data.AvgDuration > expectedAvg+100*time.Millisecond {
				t.Errorf("expected avg duration ~%v, got %v", expectedAvg, data.AvgDuration)
			}
		}
	}
}

// ===============================
// Confidence Calculation Tests
// ===============================

func TestCalculateTimeoutConfidence(t *testing.T) {
	tests := []struct {
		name        string
		timeouts    int
		totalCalls  int
		rate        float64
		minExpected float64
		maxExpected float64
	}{
		{"few timeouts low rate", 3, 30, 0.1, 0.80, 0.85},     // Base 0.8 + rate bonus 0.02
		{"many timeouts low rate", 10, 100, 0.1, 0.80, 0.85},  // Base 0.8 (capped) + rate bonus 0.02
		{"few timeouts high rate", 3, 5, 0.6, 0.70, 0.85},     // Base 0.6 + rate bonus 0.12
		{"many timeouts high rate", 50, 60, 0.83, 0.93, 0.95}, // Base 0.8 + rate bonus 0.166 = 0.966 (capped at 0.95)
		{"perfect timeout rate", 10, 10, 1.0, 0.88, 0.92},     // Base 0.7 + rate bonus 0.2 = 0.9
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateTimeoutConfidence(tt.timeouts, tt.totalCalls, tt.rate)

			if confidence < tt.minExpected || confidence > tt.maxExpected {
				t.Errorf("expected confidence in [%f, %f], got %f", tt.minExpected, tt.maxExpected, confidence)
			}

			// Confidence should never be below 0.3 or above 0.95
			if confidence < 0.3 {
				t.Errorf("confidence below minimum 0.3: %f", confidence)
			}
			if confidence > 0.95 {
				t.Errorf("confidence above maximum 0.95: %f", confidence)
			}
		})
	}
}

// ===============================
// Helper Functions
// ===============================

func createTestRunWithTimeout(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int, toolName, errorMsg string) string {
	t.Helper()

	runID := fmt.Sprintf("run-%d", index)
	r := agent.NewRun(runID, "test goal")
	r.Status = agent.RunStatusFailed
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now().Add(-time.Duration(index) * time.Hour)

	callPayload := event.ToolCalledPayload{
		ToolName: toolName,
		Input:    json.RawMessage(`{}`),
		State:    agent.StateAct,
	}
	callPayloadBytes, _ := json.Marshal(callPayload)

	failPayload := event.ToolFailedPayload{
		ToolName: toolName,
		Error:    errorMsg,
		Duration: 5 * time.Second,
	}
	failPayloadBytes, _ := json.Marshal(failPayload)

	events := []event.Event{
		{
			RunID:     runID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime,
			Payload:   callPayloadBytes,
		},
		{
			RunID:     runID,
			Type:      event.TypeToolFailed,
			Timestamp: baseTime.Add(5 * time.Second),
			Payload:   failPayloadBytes,
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	return runID
}
