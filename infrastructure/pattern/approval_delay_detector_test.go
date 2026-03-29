package pattern

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/pattern"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// ===============================
// Approval Delay Detector Tests
// ===============================

func TestNewApprovalDelayDetector_Defaults(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewApprovalDelayDetector(eventStore, runStore)

	if detector.delayThreshold != 5*time.Minute {
		t.Errorf("expected default delayThreshold 5m, got %v", detector.delayThreshold)
	}
	if detector.minOccurrences != 2 {
		t.Errorf("expected default minOccurrences 2, got %d", detector.minOccurrences)
	}
}

func TestNewApprovalDelayDetector_WithOptions(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(10*time.Minute),
		WithApprovalMinOccurrences(5),
	)

	if detector.delayThreshold != 10*time.Minute {
		t.Errorf("expected delayThreshold 10m, got %v", detector.delayThreshold)
	}
	if detector.minOccurrences != 5 {
		t.Errorf("expected minOccurrences 5, got %d", detector.minOccurrences)
	}
}

func TestApprovalDelayDetector_Types(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewApprovalDelayDetector(eventStore, runStore)

	types := detector.Types()

	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
	if types[0] != pattern.PatternTypeApprovalDelay {
		t.Errorf("expected PatternTypeApprovalDelay, got %s", types[0])
	}
}

func TestApprovalDelayDetector_Detect_FindsLongDelays(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with long approval delays
	for i := 0; i < 3; i++ {
		createTestRunWithApprovalDelay(
			ctx, t, eventStore, runStore,
			i, "destructive_tool", 10*time.Minute, true,
		)
	}

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one approval delay pattern")
	}

	// Verify we found an approval delay pattern
	foundApprovalDelay := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeApprovalDelay {
			foundApprovalDelay = true
			if p.Frequency < 2 {
				t.Errorf("expected frequency >= 2, got %d", p.Frequency)
			}

			// Check pattern data
			var data pattern.ApprovalDelayData
			if err := json.Unmarshal(p.Data, &data); err != nil {
				t.Fatalf("failed to unmarshal pattern data: %v", err)
			}
			if data.ToolName != "destructive_tool" {
				t.Errorf("expected tool name 'destructive_tool', got %s", data.ToolName)
			}
			if data.AverageWaitTime < 5*time.Minute {
				t.Errorf("expected average wait >= 5m, got %v", data.AverageWaitTime)
			}
		}
	}
	if !foundApprovalDelay {
		t.Error("expected to find an approval delay pattern")
	}
}

func TestApprovalDelayDetector_Detect_NoDetectionForQuickApprovals(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with quick approvals (below threshold)
	for i := 0; i < 3; i++ {
		createTestRunWithApprovalDelay(
			ctx, t, eventStore, runStore,
			i, "tool", 30*time.Second, true,
		)
	}

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not detect patterns for quick approvals
	if len(patterns) > 0 {
		t.Errorf("expected 0 patterns for quick approvals, got %d", len(patterns))
	}
}

func TestApprovalDelayDetector_Detect_TracksDeniedApprovals(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with denied approvals after long delay
	for i := 0; i < 3; i++ {
		createTestRunWithApprovalDelay(
			ctx, t, eventStore, runStore,
			i, "risky_tool", 8*time.Minute, false, // denied
		)
	}

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one approval delay pattern")
	}

	// Verify denied approvals are tracked
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeApprovalDelay {
			var data pattern.ApprovalDelayData
			if err := json.Unmarshal(p.Data, &data); err != nil {
				t.Fatalf("failed to unmarshal pattern data: %v", err)
			}

			// Should have tracked denials
			if data.TotalApprovals != 3 {
				t.Errorf("expected 3 total approvals, got %d", data.TotalApprovals)
			}
			if data.ApprovalRate != 0.0 {
				t.Errorf("expected approval rate 0.0 (all denied), got %f", data.ApprovalRate)
			}
		}
	}
}

func TestApprovalDelayDetector_Detect_TracksPendingApprovals(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs: 2 with resolved approvals, 1 with pending
	createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		1, "tool", 10*time.Minute, true,
	)
	createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		2, "tool", 10*time.Minute, true,
	)

	// Create run with pending (unresolved) approval
	r := agent.NewRun("run-pending", "test goal")
	r.CurrentState = agent.StateAct
	r.Status = agent.RunStatusPaused
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Only approval request, no resolution
	requestPayload := event.ApprovalRequestedPayload{
		ToolName:  "tool",
		Input:     json.RawMessage(`{}`),
		RiskLevel: "high",
	}
	requestBytes, _ := json.Marshal(requestPayload)

	if err := eventStore.Append(ctx, event.Event{
		RunID:     r.ID,
		Type:      event.TypeApprovalRequested,
		Timestamp: time.Now().Add(-10 * time.Minute),
		Payload:   requestBytes,
	}); err != nil {
		t.Fatalf("failed to append event: %v", err)
	}

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect pattern with pending count
	foundPattern := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeApprovalDelay {
			var data pattern.ApprovalDelayData
			if err := json.Unmarshal(p.Data, &data); err != nil {
				t.Fatalf("failed to unmarshal pattern data: %v", err)
			}

			if data.PendingCount != 1 {
				t.Errorf("expected 1 pending approval, got %d", data.PendingCount)
			}
			foundPattern = true
		}
	}
	if !foundPattern {
		t.Error("expected to find approval delay pattern with pending approvals")
	}
}

func TestApprovalDelayDetector_Detect_MixedApprovals(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Mix of granted and denied approvals
	for i := 0; i < 2; i++ {
		createTestRunWithApprovalDelay(
			ctx, t, eventStore, runStore,
			i, "tool", 10*time.Minute, true, // granted
		)
	}
	for i := 2; i < 5; i++ {
		createTestRunWithApprovalDelay(
			ctx, t, eventStore, runStore,
			i, "tool", 10*time.Minute, false, // denied
		)
	}

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one approval delay pattern")
	}

	// Verify mixed approval tracking
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeApprovalDelay {
			var data pattern.ApprovalDelayData
			if err := json.Unmarshal(p.Data, &data); err != nil {
				t.Fatalf("failed to unmarshal pattern data: %v", err)
			}

			if data.TotalApprovals != 5 {
				t.Errorf("expected 5 total approvals, got %d", data.TotalApprovals)
			}

			// Approval rate should be 2/5 = 0.4
			expectedRate := 2.0 / 5.0
			if data.ApprovalRate < expectedRate-0.01 || data.ApprovalRate > expectedRate+0.01 {
				t.Errorf("expected approval rate ~%f, got %f", expectedRate, data.ApprovalRate)
			}
		}
	}
}

func TestApprovalDelayDetector_Detect_FiltersByConfidence(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create delays
	for i := 0; i < 3; i++ {
		createTestRunWithApprovalDelay(
			ctx, t, eventStore, runStore,
			i, "tool", 10*time.Minute, true,
		)
	}

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	// Very high confidence requirement
	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		MinConfidence: 0.99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range patterns {
		if p.Confidence < 0.99 {
			t.Errorf("expected confidence >= 0.99, got %f", p.Confidence)
		}
	}
}

func TestApprovalDelayDetector_Detect_AppliesLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create multiple tools with delays
	for i := 0; i < 5; i++ {
		createTestRunWithApprovalDelay(
			ctx, t, eventStore, runStore,
			i, "tool_a", 10*time.Minute, true,
		)
		createTestRunWithApprovalDelay(
			ctx, t, eventStore, runStore,
			i+100, "tool_b", 10*time.Minute, true,
		)
	}

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 1 {
		t.Errorf("expected at most 1 pattern with limit, got %d", len(patterns))
	}
}

func TestApprovalDelayDetector_Detect_FiltersByRunIDs(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs
	runID1 := createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		1, "tool", 10*time.Minute, true,
	)
	runID2 := createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		2, "tool", 10*time.Minute, true,
	)
	createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		3, "tool", 10*time.Minute, true,
	)

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

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

func TestApprovalDelayDetector_Detect_EmptyStore(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewApprovalDelayDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for empty store, got %d", len(patterns))
	}
}

func TestApprovalDelayDetector_Detect_NoApprovalEvents(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs without approval events
	for i := 0; i < 3; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"read", "write"})
	}

	detector := NewApprovalDelayDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns without approval events, got %d", len(patterns))
	}
}

func TestApprovalDelayDetector_Detect_InsufficientOccurrences(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create only 1 run with delay (below minOccurrences)
	createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		1, "tool", 10*time.Minute, true,
	)

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns with insufficient occurrences, got %d", len(patterns))
	}
}

func TestApprovalDelayDetector_Detect_MaxWaitTimeTracked(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with varying delays
	createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		1, "tool", 6*time.Minute, true,
	)
	createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		2, "tool", 15*time.Minute, true, // max
	)
	createTestRunWithApprovalDelay(
		ctx, t, eventStore, runStore,
		3, "tool", 8*time.Minute, true,
	)

	detector := NewApprovalDelayDetector(
		eventStore,
		runStore,
		WithDelayThreshold(5*time.Minute),
		WithApprovalMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one approval delay pattern")
	}

	// Verify max wait time is tracked
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeApprovalDelay {
			var data pattern.ApprovalDelayData
			if err := json.Unmarshal(p.Data, &data); err != nil {
				t.Fatalf("failed to unmarshal pattern data: %v", err)
			}

			if data.MaxWaitTime != 15*time.Minute {
				t.Errorf("expected max wait time 15m, got %v", data.MaxWaitTime)
			}
		}
	}
}

// ===============================
// Confidence Calculation Tests
// ===============================

func TestCalculateApprovalDelayConfidence_HighDelayRate(t *testing.T) {
	// All delays exceed threshold
	confidence := calculateApprovalDelayConfidence(10, 10, 10*time.Minute, 5*time.Minute)

	// Should have high confidence: base (0.8) + wait bonus
	if confidence < 0.85 {
		t.Errorf("expected confidence >= 0.85 for all delays, got %f", confidence)
	}
}

func TestCalculateApprovalDelayConfidence_LowDelayRate(t *testing.T) {
	// Only 1 out of 10 delays exceeds threshold
	confidence := calculateApprovalDelayConfidence(1, 10, 6*time.Minute, 5*time.Minute)

	// Should have lower confidence
	if confidence > 0.6 {
		t.Errorf("expected confidence <= 0.6 for low delay rate, got %f", confidence)
	}
}

func TestCalculateApprovalDelayConfidence_WaitTimeBonus(t *testing.T) {
	// Test wait time significantly exceeding threshold (2x)
	conf1 := calculateApprovalDelayConfidence(5, 10, 10*time.Minute, 5*time.Minute)

	// Test wait time just above threshold
	conf2 := calculateApprovalDelayConfidence(5, 10, 6*time.Minute, 5*time.Minute)

	// First should have higher confidence due to wait bonus
	if conf1 <= conf2 {
		t.Errorf("expected higher confidence for 2x wait time: got %f vs %f", conf1, conf2)
	}
}

func TestCalculateApprovalDelayConfidence_BoundsCheck(t *testing.T) {
	tests := []struct {
		name        string
		delayCount  int
		totalCount  int
		avgWait     time.Duration
		threshold   time.Duration
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "minimum confidence",
			delayCount:  1,
			totalCount:  100,
			avgWait:     5 * time.Minute,
			threshold:   5 * time.Minute,
			minExpected: 0.3,
			maxExpected: 0.5,
		},
		{
			name:        "maximum confidence capped",
			delayCount:  100,
			totalCount:  100,
			avgWait:     50 * time.Minute,
			threshold:   5 * time.Minute,
			minExpected: 0.9,
			maxExpected: 0.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateApprovalDelayConfidence(
				tt.delayCount,
				tt.totalCount,
				tt.avgWait,
				tt.threshold,
			)

			if confidence < tt.minExpected || confidence > tt.maxExpected {
				t.Errorf("expected confidence in [%f, %f], got %f",
					tt.minExpected, tt.maxExpected, confidence)
			}
		})
	}
}

// ===============================
// Helper Functions
// ===============================

// createTestRunWithApprovalDelay creates a run with approval request/response events
func createTestRunWithApprovalDelay(
	ctx context.Context,
	t *testing.T,
	eventStore *memory.EventStore,
	runStore *memory.RunStore,
	index int,
	toolName string,
	delay time.Duration,
	granted bool,
) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
	r.CurrentState = agent.StateAct
	if granted {
		r.Status = agent.RunStatusCompleted
	} else {
		r.Status = agent.RunStatusFailed
	}
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now().Add(-time.Duration(index) * time.Hour)

	// Approval request
	requestPayload := event.ApprovalRequestedPayload{
		ToolName:  toolName,
		Input:     json.RawMessage(`{}`),
		RiskLevel: "high",
	}
	requestBytes, _ := json.Marshal(requestPayload)

	// Approval resolution (granted or denied)
	resultPayload := event.ApprovalResultPayload{
		ToolName: toolName,
		Approver: "human",
		Reason:   "test approval",
	}
	resultBytes, _ := json.Marshal(resultPayload)

	resultType := event.TypeApprovalGranted
	if !granted {
		resultType = event.TypeApprovalDenied
	}

	events := []event.Event{
		{
			RunID:     r.ID,
			Type:      event.TypeApprovalRequested,
			Timestamp: baseTime,
			Payload:   requestBytes,
		},
		{
			RunID:     r.ID,
			Type:      resultType,
			Timestamp: baseTime.Add(delay),
			Payload:   resultBytes,
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	return r.ID
}
