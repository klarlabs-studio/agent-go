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
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// ===============================
// Tool Preference Detector Tests
// ===============================

func TestNewToolPreferenceDetector_Defaults(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := memory.NewToolRegistry()

	detector := NewToolPreferenceDetector(eventStore, runStore, registry)

	if detector.overuseThreshold != 2.0 {
		t.Errorf("expected default overuseThreshold 2.0, got %f", detector.overuseThreshold)
	}
	if detector.underuseThreshold != 0.25 {
		t.Errorf("expected default underuseThreshold 0.25, got %f", detector.underuseThreshold)
	}
	if detector.minCalls != 5 {
		t.Errorf("expected default minCalls 5, got %d", detector.minCalls)
	}
}

func TestNewToolPreferenceDetector_WithOptions(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := memory.NewToolRegistry()

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithOveruseThreshold(3.0),
		WithUnderuseThreshold(0.1),
		WithPreferenceMinCalls(10),
	)

	if detector.overuseThreshold != 3.0 {
		t.Errorf("expected overuseThreshold 3.0, got %f", detector.overuseThreshold)
	}
	if detector.underuseThreshold != 0.1 {
		t.Errorf("expected underuseThreshold 0.1, got %f", detector.underuseThreshold)
	}
	if detector.minCalls != 10 {
		t.Errorf("expected minCalls 10, got %d", detector.minCalls)
	}
}

func TestToolPreferenceDetector_Types(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := memory.NewToolRegistry()

	detector := NewToolPreferenceDetector(eventStore, runStore, registry)

	types := detector.Types()

	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
	if types[0] != pattern.PatternTypeToolPreference {
		t.Errorf("expected PatternTypeToolPreference, got %s", types[0])
	}
}

func TestToolPreferenceDetector_Detect_OverusedTool(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b", "tool_c")

	// Create runs where tool_a is heavily used compared to others
	// Expected usage would be totalCalls/3 = 30/3 = 10
	// tool_a: 24 calls (ratio = 2.4, above threshold of 2.0)
	// tool_b: 3 calls
	// tool_c: 3 calls
	for i := 0; i < 8; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_a", "tool_a", "tool_a"})
	}
	for i := 8; i < 11; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_b", "tool_c"})
	}

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithOveruseThreshold(2.0),
		WithPreferenceMinCalls(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect tool_a as overused
	foundOverused := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeToolPreference {
			var data pattern.ToolPreferenceData
			if err := p.GetData(&data); err != nil {
				t.Fatalf("failed to get pattern data: %v", err)
			}
			if data.ToolName == "tool_a" && data.PreferenceType == "overused" {
				foundOverused = true
				if data.UsageRatio < 2.0 {
					t.Errorf("expected usage ratio >= 2.0, got %f", data.UsageRatio)
				}
			}
		}
	}

	if !foundOverused {
		t.Error("expected to find tool_a as overused")
	}
}

func TestToolPreferenceDetector_Detect_UnderusedTool(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b", "tool_c", "tool_d")

	// Create runs where tool_d is rarely used
	// Expected usage would be totalCalls/4 = 40/4 = 10
	// tool_a: 15 calls
	// tool_b: 13 calls
	// tool_c: 10 calls
	// tool_d: 2 calls (ratio = 0.2, below threshold of 0.25)
	for i := 0; i < 5; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_a", "tool_a", "tool_a"})
	}
	for i := 5; i < 10; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_b", "tool_b", "tool_c"})
	}
	for i := 10; i < 11; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_c", "tool_d", "tool_d"})
	}

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithUnderuseThreshold(0.25),
		WithPreferenceMinCalls(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect tool_d as underused
	foundUnderused := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeToolPreference {
			var data pattern.ToolPreferenceData
			if err := p.GetData(&data); err != nil {
				t.Fatalf("failed to get pattern data: %v", err)
			}
			if data.ToolName == "tool_d" && data.PreferenceType == "underused" {
				foundUnderused = true
				if data.UsageRatio > 0.25 {
					t.Errorf("expected usage ratio <= 0.25, got %f", data.UsageRatio)
				}
			}
		}
	}

	if !foundUnderused {
		t.Error("expected to find tool_d as underused")
	}
}

func TestToolPreferenceDetector_Detect_NeverUsedTool(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b", "tool_c")

	// Create runs where tool_c is never used
	// Total calls: 30
	// tool_a: 15 calls
	// tool_b: 15 calls
	// tool_c: 0 calls (never used, should be detected as underused)
	for i := 0; i < 10; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_a", "tool_b", "tool_a"})
	}

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithUnderuseThreshold(0.25),
		WithPreferenceMinCalls(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect tool_c as underused (never used)
	foundUnderused := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeToolPreference {
			var data pattern.ToolPreferenceData
			if err := p.GetData(&data); err != nil {
				t.Fatalf("failed to get pattern data: %v", err)
			}
			if data.ToolName == "tool_c" && data.PreferenceType == "underused" {
				foundUnderused = true
				if data.UsageCount != 0 {
					t.Errorf("expected usage count 0, got %d", data.UsageCount)
				}
			}
		}
	}

	if !foundUnderused {
		t.Error("expected to find tool_c as underused (never used)")
	}
}

func TestToolPreferenceDetector_Detect_BalancedUsage(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b", "tool_c")

	// Create runs with balanced tool usage
	// Each tool used approximately the same number of times
	for i := 0; i < 10; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_a", "tool_b", "tool_c"})
	}

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithOveruseThreshold(2.0),
		WithUnderuseThreshold(0.25),
		WithPreferenceMinCalls(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not detect any preference patterns with balanced usage
	if len(patterns) > 0 {
		t.Errorf("expected no patterns for balanced usage, got %d", len(patterns))
	}
}

func TestToolPreferenceDetector_Detect_MinCallsFilter(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b")

	// Create runs with very few calls
	// tool_a: 3 calls
	// tool_b: 1 call
	createTestRunWithTools(ctx, t, eventStore, runStore, 0, []string{"tool_a", "tool_a", "tool_a"})
	createTestRunWithTools(ctx, t, eventStore, runStore, 1, []string{"tool_b"})

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithPreferenceMinCalls(10), // High threshold
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not detect patterns because calls are below minCalls threshold
	if len(patterns) > 0 {
		t.Errorf("expected no patterns with low call count, got %d", len(patterns))
	}
}

func TestToolPreferenceDetector_Detect_SuccessRateCalculation(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b")

	// Create runs with mixed success/failure
	// tool_a: 10 successful calls
	for i := 0; i < 10; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_a"})
	}
	// tool_b: 2 failures
	for i := 10; i < 12; i++ {
		createTestRunWithFailure(ctx, t, eventStore, runStore, i, "tool_b", "error")
	}

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithOveruseThreshold(2.0),
		WithPreferenceMinCalls(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify success rates are calculated correctly
	for _, p := range patterns {
		var data pattern.ToolPreferenceData
		if err := p.GetData(&data); err != nil {
			t.Fatalf("failed to get pattern data: %v", err)
		}
		switch data.ToolName {
		case "tool_a":
			if data.SuccessRate != 1.0 {
				t.Errorf("expected tool_a success rate 1.0, got %f", data.SuccessRate)
			}
		case "tool_b":
			if data.SuccessRate != 0.0 {
				t.Errorf("expected tool_b success rate 0.0, got %f", data.SuccessRate)
			}
		}
	}
}

func TestToolPreferenceDetector_Detect_MinConfidenceFilter(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b")

	// Create runs with some overuse
	for i := 0; i < 5; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_a", "tool_a", "tool_b"})
	}

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithOveruseThreshold(1.8),
		WithPreferenceMinCalls(5),
	)

	// First, detect without confidence filter
	allPatterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Then detect with high confidence filter
	filteredPatterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		MinConfidence: 0.95,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have fewer patterns with high confidence filter
	if len(filteredPatterns) > len(allPatterns) {
		t.Error("filtered patterns should be less than or equal to all patterns")
	}

	// All filtered patterns should meet confidence threshold
	for _, p := range filteredPatterns {
		if p.Confidence < 0.95 {
			t.Errorf("expected confidence >= 0.95, got %f", p.Confidence)
		}
	}
}

func TestToolPreferenceDetector_Detect_EmptyStore(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b")

	detector := NewToolPreferenceDetector(eventStore, runStore, registry)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for empty store, got %d", len(patterns))
	}
}

func TestToolPreferenceDetector_Detect_NoToolCalls(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b")

	// Create runs without tool calls (only state transitions)
	for i := 0; i < 3; i++ {
		r := agent.NewRun(fmt.Sprintf("run-%d", i), "test goal")
		r.Status = agent.RunStatusCompleted
		if err := runStore.Save(ctx, r); err != nil {
			t.Fatalf("failed to save run: %v", err)
		}

		payload := event.StateTransitionedPayload{
			FromState: agent.StateIntake,
			ToState:   agent.StateDone,
			Reason:    "completed",
		}
		payloadBytes, _ := json.Marshal(payload)

		events := []event.Event{
			{
				RunID:     r.ID,
				Type:      event.TypeStateTransitioned,
				Timestamp: time.Now(),
				Payload:   payloadBytes,
			},
		}
		if err := eventStore.Append(ctx, events...); err != nil {
			t.Fatalf("failed to append events: %v", err)
		}
	}

	detector := NewToolPreferenceDetector(eventStore, runStore, registry)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns with no tool calls, got %d", len(patterns))
	}
}

func TestToolPreferenceDetector_Detect_NilRegistry(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with extremely unbalanced tool calls
	// tool_a: 45 calls (heavily overused)
	// tool_b: 1 call (heavily underused)
	for i := 0; i < 15; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_a", "tool_a", "tool_a"})
	}
	createTestRunWithTools(ctx, t, eventStore, runStore, 15, []string{"tool_b"})

	// Create detector with nil registry
	// Even without a registry, it should detect patterns based on observed usage
	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		nil,
		WithOveruseThreshold(1.8),
		WithUnderuseThreshold(0.25),
		WithPreferenceMinCalls(1),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With nil registry, detector can still find patterns from observed tool usage
	// Expected usage = totalCalls/numObservedTools = 46/2 = 23
	// tool_a: 45 calls (ratio = 1.96, above threshold of 1.8)
	// tool_b: 1 call (ratio = 0.043, well below underuse threshold of 0.25)
	// Should detect both as overused and underused
	if len(patterns) == 0 {
		t.Error("expected patterns even without registry")
	}
}

func TestToolPreferenceDetector_Detect_FiltersByRunIDs(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b")

	// Create runs with tool usage
	runID1 := createTestRunWithTools(ctx, t, eventStore, runStore, 1, []string{"tool_a", "tool_a", "tool_a"})
	runID2 := createTestRunWithTools(ctx, t, eventStore, runStore, 2, []string{"tool_a", "tool_a", "tool_a"})
	createTestRunWithTools(ctx, t, eventStore, runStore, 3, []string{"tool_b"})

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithOveruseThreshold(2.0),
		WithPreferenceMinCalls(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		RunIDs: []string{runID1, runID2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify evidence only comes from filtered runs
	for _, p := range patterns {
		for _, e := range p.Evidence {
			if e.RunID != runID1 && e.RunID != runID2 {
				t.Errorf("found evidence from excluded run: %s", e.RunID)
			}
		}
	}
}

func TestToolPreferenceDetector_Detect_AppliesLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b", "tool_c", "tool_d")

	// Create runs with multiple preference patterns
	for i := 0; i < 10; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_a", "tool_a", "tool_a"})
	}
	for i := 10; i < 20; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"tool_b", "tool_b", "tool_b"})
	}
	createTestRunWithTools(ctx, t, eventStore, runStore, 20, []string{"tool_c"})

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithOveruseThreshold(1.5),
		WithPreferenceMinCalls(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 2 {
		t.Errorf("expected at most 2 patterns with limit, got %d", len(patterns))
	}
}

func TestToolPreferenceDetector_Detect_TimeFilter(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	registry := createTestRegistryWithTools(t, "tool_a", "tool_b")

	// Create old runs
	for i := 0; i < 5; i++ {
		createTestRunWithToolsAtTime(ctx, t, eventStore, runStore, i, []string{"tool_a", "tool_a"}, time.Now().Add(-48*time.Hour))
	}

	// Create recent runs
	for i := 5; i < 10; i++ {
		createTestRunWithToolsAtTime(ctx, t, eventStore, runStore, i, []string{"tool_b", "tool_b", "tool_b"}, time.Now())
	}

	detector := NewToolPreferenceDetector(
		eventStore,
		runStore,
		registry,
		WithOveruseThreshold(1.5),
		WithPreferenceMinCalls(5),
	)

	// Detect patterns from last 24 hours
	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		FromTime: time.Now().Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only detect patterns from recent runs
	for _, p := range patterns {
		var data pattern.ToolPreferenceData
		if err := p.GetData(&data); err != nil {
			t.Fatalf("failed to get pattern data: %v", err)
		}
		// Should see tool_b (recent) but not tool_a (old)
		if data.ToolName == "tool_a" && data.UsageCount > 0 {
			t.Error("expected not to find tool_a in recent time window")
		}
	}
}

// ===============================
// Confidence Calculation Tests
// ===============================

func TestCalculatePreferenceConfidence_Overused(t *testing.T) {
	tests := []struct {
		name      string
		calls     int
		ratio     float64
		threshold float64
		minConf   float64
		maxConf   float64
	}{
		{"barely overused", 5, 2.1, 2.0, 0.45, 0.65},
		{"moderately overused", 10, 3.0, 2.0, 0.6, 0.8},
		{"heavily overused", 20, 5.0, 2.0, 0.7, 0.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculatePreferenceConfidence(tt.calls, tt.ratio, tt.threshold, 0.25)

			if confidence < tt.minConf || confidence > tt.maxConf {
				t.Errorf("expected confidence in [%f, %f], got %f", tt.minConf, tt.maxConf, confidence)
			}
		})
	}
}

func TestCalculatePreferenceConfidence_Underused(t *testing.T) {
	tests := []struct {
		name      string
		calls     int
		ratio     float64
		threshold float64
		minConf   float64
		maxConf   float64
	}{
		{"barely underused", 5, 0.23, 0.25, 0.50, 0.65},
		{"moderately underused", 10, 0.1, 0.25, 0.85, 0.90}, // extremity=2.5, base+bonus=0.875
		{"heavily underused", 20, 0.05, 0.25, 0.90, 0.95},   // extremity=5.0, capped at 0.95
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculatePreferenceConfidence(tt.calls, tt.ratio, 2.0, tt.threshold)

			if confidence < tt.minConf || confidence > tt.maxConf {
				t.Errorf("expected confidence in [%f, %f], got %f", tt.minConf, tt.maxConf, confidence)
			}
		})
	}
}

func TestCalculatePreferenceConfidence_NeverUsed(t *testing.T) {
	// Never used tools should have high confidence
	// extremity = 3.0, confidence = 0.4 + 3.0*0.15 = 0.85
	confidence := calculatePreferenceConfidence(0, 0.0, 2.0, 0.25)

	if confidence < 0.84 || confidence > 0.86 {
		t.Errorf("expected confidence for never used tool in [0.84, 0.86], got %f", confidence)
	}
}

func TestCalculatePreferenceConfidence_CallBonus(t *testing.T) {
	// More calls should increase confidence
	lowCallConf := calculatePreferenceConfidence(5, 3.0, 2.0, 0.25)
	highCallConf := calculatePreferenceConfidence(50, 3.0, 2.0, 0.25)

	if highCallConf <= lowCallConf {
		t.Errorf("expected higher confidence for more calls: %f vs %f", highCallConf, lowCallConf)
	}
}

func TestCalculatePreferenceConfidence_Bounds(t *testing.T) {
	// Test that confidence is bounded [0.3, 0.95]
	tests := []struct {
		name  string
		calls int
		ratio float64
	}{
		{"very low extremity", 1, 2.01},
		{"very high extremity", 100, 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculatePreferenceConfidence(tt.calls, tt.ratio, 2.0, 0.25)

			if confidence < 0.3 {
				t.Errorf("confidence below minimum bound 0.3: %f", confidence)
			}
			if confidence > 0.95 {
				t.Errorf("confidence above maximum bound 0.95: %f", confidence)
			}
		})
	}
}

// ===============================
// Helper Functions
// ===============================

// createTestRegistryWithTools creates a tool registry with the given tool names
func createTestRegistryWithTools(t *testing.T, toolNames ...string) *memory.ToolRegistry {
	t.Helper()

	registry := memory.NewToolRegistry()
	for _, name := range toolNames {
		tool := tool.NewBuilder(name).
			WithDescription(fmt.Sprintf("Test tool %s", name)).
			ReadOnly().
			WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			}).
			MustBuild()
		if err := registry.Register(tool); err != nil {
			t.Fatalf("failed to register tool %s: %v", name, err)
		}
	}
	return registry
}

// createTestRunWithToolsAtTime creates a test run with specific tools at a specific time
func createTestRunWithToolsAtTime(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int, tools []string, baseTime time.Time) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	events := make([]event.Event, 0)
	for i, toolName := range tools {
		payload := event.ToolCalledPayload{
			ToolName: toolName,
			Input:    json.RawMessage(`{}`),
			State:    agent.StateExplore,
		}
		payloadBytes, _ := json.Marshal(payload)

		events = append(events, event.Event{
			RunID:     r.ID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Payload:   payloadBytes,
		})

		succPayload := event.ToolSucceededPayload{
			ToolName: toolName,
			Output:   json.RawMessage(`{}`),
			Duration: 100 * time.Millisecond,
		}
		succPayloadBytes, _ := json.Marshal(succPayload)

		events = append(events, event.Event{
			RunID:     r.ID,
			Type:      event.TypeToolSucceeded,
			Timestamp: baseTime.Add(time.Duration(i)*time.Minute + 100*time.Millisecond),
			Payload:   succPayloadBytes,
		})
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	return r.ID
}
