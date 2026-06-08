package pattern

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// ===============================
// Constructor and Options Tests
// ===============================

func TestNewCostAnomalyDetector_Defaults(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewCostAnomalyDetector(eventStore, runStore)

	if detector.deviationThreshold != 2.0 {
		t.Errorf("expected default deviationThreshold 2.0, got %f", detector.deviationThreshold)
	}
	if detector.minSampleSize != 5 {
		t.Errorf("expected default minSampleSize 5, got %d", detector.minSampleSize)
	}
	if len(detector.costTypes) != 3 {
		t.Errorf("expected 3 default cost types, got %d", len(detector.costTypes))
	}

	expectedTypes := map[string]bool{
		"tool_calls": true,
		"tokens":     true,
		"api_calls":  true,
	}
	for _, ct := range detector.costTypes {
		if !expectedTypes[ct] {
			t.Errorf("unexpected cost type: %s", ct)
		}
	}
}

func TestNewCostAnomalyDetector_WithDeviationThreshold(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithDeviationThreshold(3.5),
	)

	if detector.deviationThreshold != 3.5 {
		t.Errorf("expected deviationThreshold 3.5, got %f", detector.deviationThreshold)
	}
}

func TestNewCostAnomalyDetector_WithMinSampleSize(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(10),
	)

	if detector.minSampleSize != 10 {
		t.Errorf("expected minSampleSize 10, got %d", detector.minSampleSize)
	}
}

func TestNewCostAnomalyDetector_WithCostTypes(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	customTypes := []string{"custom_cost", "api_calls"}
	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithCostTypes(customTypes),
	)

	if len(detector.costTypes) != 2 {
		t.Errorf("expected 2 cost types, got %d", len(detector.costTypes))
	}
	if detector.costTypes[0] != "custom_cost" || detector.costTypes[1] != "api_calls" {
		t.Errorf("unexpected cost types: %v", detector.costTypes)
	}
}

func TestNewCostAnomalyDetector_WithMultipleOptions(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithDeviationThreshold(2.5),
		WithMinSampleSize(8),
		WithCostTypes([]string{"tool_calls"}),
	)

	if detector.deviationThreshold != 2.5 {
		t.Errorf("expected deviationThreshold 2.5, got %f", detector.deviationThreshold)
	}
	if detector.minSampleSize != 8 {
		t.Errorf("expected minSampleSize 8, got %d", detector.minSampleSize)
	}
	if len(detector.costTypes) != 1 || detector.costTypes[0] != "tool_calls" {
		t.Errorf("expected cost types [tool_calls], got %v", detector.costTypes)
	}
}

// ===============================
// Types Method Tests
// ===============================

func TestCostAnomalyDetector_Types(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewCostAnomalyDetector(eventStore, runStore)

	types := detector.Types()

	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
	if types[0] != pattern.PatternTypeCostAnomaly {
		t.Errorf("expected PatternTypeCostAnomaly, got %s", types[0])
	}
}

// ===============================
// Detection Tests
// ===============================

func TestCostAnomalyDetector_Detect_FindsCostSpike(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create 6 runs: 5 with ~10 tool calls, 1 with 50 tool calls (spike)
	for i := 0; i < 5; i++ {
		createTestRunWithToolCallCount(ctx, t, eventStore, runStore, i, 10)
	}
	// Spike: 5x the baseline
	createTestRunWithToolCallCount(ctx, t, eventStore, runStore, 100, 50)

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(5),
		WithDeviationThreshold(2.0),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one cost anomaly pattern")
	}

	foundToolCallsAnomaly := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeCostAnomaly {
			var data pattern.CostAnomalyData
			if err := p.GetData(&data); err != nil {
				t.Fatalf("failed to get pattern data: %v", err)
			}
			if data.CostType == "tool_calls" {
				foundToolCallsAnomaly = true
				if data.AnomalyCount < 1 {
					t.Errorf("expected at least 1 anomaly, got %d", data.AnomalyCount)
				}
				if data.AverageCost >= data.AnomalyCost {
					t.Errorf("expected anomaly cost (%.2f) > average cost (%.2f)", data.AnomalyCost, data.AverageCost)
				}
				if p.Frequency < 1 {
					t.Errorf("expected frequency >= 1, got %d", p.Frequency)
				}
			}
		}
	}

	if !foundToolCallsAnomaly {
		t.Error("expected to find tool_calls cost anomaly")
	}
}

func TestCostAnomalyDetector_Detect_UniformCostsNoAnomaly(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create 10 runs with identical tool call counts
	for i := 0; i < 10; i++ {
		createTestRunWithToolCallCount(ctx, t, eventStore, runStore, i, 10)
	}

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With uniform costs, stddev is 0, so no anomalies should be detected
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for uniform costs, got %d", len(patterns))
	}
}

func TestCostAnomalyDetector_Detect_NotEnoughSamples(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create only 3 runs, less than minSampleSize of 5
	for i := 0; i < 3; i++ {
		createTestRunWithToolCallCount(ctx, t, eventStore, runStore, i, 10)
	}

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected nil or empty patterns when sample size too small, got %d", len(patterns))
	}
}

func TestCostAnomalyDetector_Detect_FiltersByRunIDs(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create baseline runs and one anomaly
	var runIDs []string
	for i := 0; i < 5; i++ {
		runID := createTestRunWithToolCallCount(ctx, t, eventStore, runStore, i, 10)
		runIDs = append(runIDs, runID)
	}
	anomalyRunID := createTestRunWithToolCallCount(ctx, t, eventStore, runStore, 100, 50)

	// Include anomaly in filter
	filterIDs := append(append([]string{}, runIDs...), anomalyRunID)

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		RunIDs: filterIDs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify evidence only comes from filtered runs
	for _, p := range patterns {
		for _, e := range p.Evidence {
			found := false
			for _, id := range filterIDs {
				if e.RunID == id {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("found evidence from excluded run: %s", e.RunID)
			}
		}
	}
}

func TestCostAnomalyDetector_Detect_FiltersbyConfidence(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create moderate anomaly scenario
	for i := 0; i < 5; i++ {
		createTestRunWithToolCallCount(ctx, t, eventStore, runStore, i, 10)
	}
	createTestRunWithToolCallCount(ctx, t, eventStore, runStore, 100, 30)

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(5),
	)

	// Very high confidence requirement
	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		MinConfidence: 0.95,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range patterns {
		if p.Confidence < 0.95 {
			t.Errorf("expected confidence >= 0.95, got %f", p.Confidence)
		}
	}
}

func TestCostAnomalyDetector_Detect_AppliesLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create anomalies in multiple cost types
	for i := 0; i < 6; i++ {
		createTestRunWithToolCallCount(ctx, t, eventStore, runStore, i, 10)
	}
	// Create spikes
	createTestRunWithToolCallCount(ctx, t, eventStore, runStore, 100, 50)
	createTestRunWithToolCallCount(ctx, t, eventStore, runStore, 101, 60)

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 1 {
		t.Errorf("expected at most 1 pattern with limit, got %d", len(patterns))
	}
}

func TestCostAnomalyDetector_Detect_EmptyStore(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewCostAnomalyDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected nil or empty patterns for empty store, got %d", len(patterns))
	}
}

// ===============================
// calculateRunCosts Tests
// ===============================

func TestCalculateRunCosts_CountsToolCalls(t *testing.T) {
	payload1 := event.ToolCalledPayload{ToolName: "tool1", Input: json.RawMessage(`{}`), State: agent.StateAct}
	payload2 := event.ToolCalledPayload{ToolName: "tool2", Input: json.RawMessage(`{}`), State: agent.StateAct}
	payload1Bytes, _ := json.Marshal(payload1)
	payload2Bytes, _ := json.Marshal(payload2)

	events := []event.Event{
		{Type: event.TypeToolCalled, Payload: payload1Bytes},
		{Type: event.TypeToolCalled, Payload: payload2Bytes},
	}

	costs := calculateRunCosts(events)

	if costs["tool_calls"] != 2 {
		t.Errorf("expected 2 tool_calls, got %f", costs["tool_calls"])
	}
}

func TestCalculateRunCosts_CountsAPICalls(t *testing.T) {
	succPayload := event.ToolSucceededPayload{ToolName: "tool", Output: json.RawMessage(`{}`), Duration: 100 * time.Millisecond}
	failPayload := event.ToolFailedPayload{ToolName: "tool", Error: "error", Duration: 100 * time.Millisecond}
	succBytes, _ := json.Marshal(succPayload)
	failBytes, _ := json.Marshal(failPayload)

	events := []event.Event{
		{Type: event.TypeToolSucceeded, Payload: succBytes},
		{Type: event.TypeToolFailed, Payload: failBytes},
	}

	costs := calculateRunCosts(events)

	if costs["api_calls"] != 2 {
		t.Errorf("expected 2 api_calls (1 success + 1 failure), got %f", costs["api_calls"])
	}
}

func TestCalculateRunCosts_TokensAlwaysZero(t *testing.T) {
	payload := event.ToolCalledPayload{ToolName: "tool", Input: json.RawMessage(`{}`), State: agent.StateAct}
	payloadBytes, _ := json.Marshal(payload)

	events := []event.Event{
		{Type: event.TypeToolCalled, Payload: payloadBytes},
	}

	costs := calculateRunCosts(events)

	if costs["tokens"] != 0 {
		t.Errorf("expected tokens to be 0, got %f", costs["tokens"])
	}
}

func TestCalculateRunCosts_EmptyEvents(t *testing.T) {
	costs := calculateRunCosts([]event.Event{})

	if costs["tool_calls"] != 0 {
		t.Errorf("expected 0 tool_calls, got %f", costs["tool_calls"])
	}
	if costs["api_calls"] != 0 {
		t.Errorf("expected 0 api_calls, got %f", costs["api_calls"])
	}
	if costs["tokens"] != 0 {
		t.Errorf("expected 0 tokens, got %f", costs["tokens"])
	}
}

// ===============================
// detectAnomalies Tests
// ===============================

func TestDetectAnomalies_FindsHighDeviation(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithDeviationThreshold(2.0),
	)

	// Create baseline with tight distribution
	// Mean = (10+10+10+10+10+10+10+10+10+80)/10 = 17
	// The outlier at 80 will have high deviation
	costs := []runCost{
		{runID: "run-1", cost: 10},
		{runID: "run-2", cost: 10},
		{runID: "run-3", cost: 10},
		{runID: "run-4", cost: 10},
		{runID: "run-5", cost: 10},
		{runID: "run-6", cost: 10},
		{runID: "run-7", cost: 10},
		{runID: "run-8", cost: 10},
		{runID: "run-9", cost: 10},
		{runID: "run-10", cost: 80}, // Strong outlier
	}

	anomalies := detector.detectAnomalies(costs)

	if len(anomalies) == 0 {
		t.Fatal("expected at least one anomaly")
	}

	foundOutlier := false
	for _, a := range anomalies {
		if a.runID == "run-10" {
			foundOutlier = true
			if a.cost != 80 {
				t.Errorf("expected cost 80, got %f", a.cost)
			}
			if math.Abs(a.deviation) < 2.0 {
				t.Errorf("expected absolute deviation >= 2.0, got %f", math.Abs(a.deviation))
			}
		}
	}

	if !foundOutlier {
		t.Error("expected to find outlier run-10")
	}
}

func TestDetectAnomalies_UniformCostsNoAnomaly(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithDeviationThreshold(2.0),
	)

	// All costs identical
	costs := []runCost{
		{runID: "run-1", cost: 10},
		{runID: "run-2", cost: 10},
		{runID: "run-3", cost: 10},
		{runID: "run-4", cost: 10},
	}

	anomalies := detector.detectAnomalies(costs)

	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies for uniform costs, got %d", len(anomalies))
	}
}

func TestDetectAnomalies_LessThanThreeSamples(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewCostAnomalyDetector(eventStore, runStore)

	costs := []runCost{
		{runID: "run-1", cost: 10},
		{runID: "run-2", cost: 50}, // Would be anomaly with more data
	}

	anomalies := detector.detectAnomalies(costs)

	if anomalies != nil {
		t.Errorf("expected nil for < 3 samples, got %d anomalies", len(anomalies))
	}
}

func TestDetectAnomalies_RespectsCustomThreshold(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithDeviationThreshold(3.0), // Higher threshold
	)

	// Create data with moderate deviation
	costs := []runCost{
		{runID: "run-1", cost: 10},
		{runID: "run-2", cost: 10},
		{runID: "run-3", cost: 10},
		{runID: "run-4", cost: 20}, // Moderate outlier
	}

	anomalies := detector.detectAnomalies(costs)

	// With higher threshold (3.0), moderate outlier might not trigger
	// This test verifies threshold configuration is respected
	for _, a := range anomalies {
		if math.Abs(a.deviation) < 3.0 {
			t.Errorf("expected all anomalies to have |deviation| >= 3.0, got %f", math.Abs(a.deviation))
		}
	}
}

// ===============================
// calculateTrend Tests
// ===============================

func TestCalculateTrend_Increasing(t *testing.T) {
	costs := []runCost{
		{runID: "run-1", cost: 10},
		{runID: "run-2", cost: 15},
		{runID: "run-3", cost: 20},
		{runID: "run-4", cost: 25},
		{runID: "run-5", cost: 30},
	}

	trend := calculateTrend(costs)

	if trend != "increasing" {
		t.Errorf("expected 'increasing', got '%s'", trend)
	}
}

func TestCalculateTrend_Decreasing(t *testing.T) {
	costs := []runCost{
		{runID: "run-1", cost: 30},
		{runID: "run-2", cost: 25},
		{runID: "run-3", cost: 20},
		{runID: "run-4", cost: 15},
		{runID: "run-5", cost: 10},
	}

	trend := calculateTrend(costs)

	if trend != "decreasing" {
		t.Errorf("expected 'decreasing', got '%s'", trend)
	}
}

func TestCalculateTrend_Stable(t *testing.T) {
	costs := []runCost{
		{runID: "run-1", cost: 10},
		{runID: "run-2", cost: 11},
		{runID: "run-3", cost: 10},
		{runID: "run-4", cost: 9},
		{runID: "run-5", cost: 10},
	}

	trend := calculateTrend(costs)

	if trend != "stable" {
		t.Errorf("expected 'stable', got '%s'", trend)
	}
}

func TestCalculateTrend_LessThanThreeSamples(t *testing.T) {
	costs := []runCost{
		{runID: "run-1", cost: 10},
		{runID: "run-2", cost: 20},
	}

	trend := calculateTrend(costs)

	if trend != "stable" {
		t.Errorf("expected 'stable' for < 3 samples, got '%s'", trend)
	}
}

// ===============================
// Helper Function Tests (mean, stddev)
// ===============================

func TestMean_CalculatesAverage(t *testing.T) {
	values := []float64{10, 20, 30, 40, 50}
	expected := 30.0

	result := mean(values)

	if result != expected {
		t.Errorf("expected mean %.2f, got %.2f", expected, result)
	}
}

func TestMean_EmptySlice(t *testing.T) {
	values := []float64{}

	result := mean(values)

	if result != 0 {
		t.Errorf("expected mean 0 for empty slice, got %.2f", result)
	}
}

func TestMean_SingleValue(t *testing.T) {
	values := []float64{42}

	result := mean(values)

	if result != 42 {
		t.Errorf("expected mean 42, got %.2f", result)
	}
}

func TestStddev_CalculatesStandardDeviation(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	avg := mean(values) // Should be 5

	result := stddev(values, avg)

	// Expected stddev ≈ 2.138
	if result < 2.0 || result > 2.2 {
		t.Errorf("expected stddev around 2.138, got %.3f", result)
	}
}

func TestStddev_UniformValues(t *testing.T) {
	values := []float64{10, 10, 10, 10, 10}
	avg := mean(values)

	result := stddev(values, avg)

	if result != 0 {
		t.Errorf("expected stddev 0 for uniform values, got %.3f", result)
	}
}

func TestStddev_LessThanTwoSamples(t *testing.T) {
	values := []float64{42}
	avg := mean(values)

	result := stddev(values, avg)

	if result != 0 {
		t.Errorf("expected stddev 0 for < 2 samples, got %.3f", result)
	}
}

func TestStddev_EmptySlice(t *testing.T) {
	values := []float64{}
	avg := mean(values)

	result := stddev(values, avg)

	if result != 0 {
		t.Errorf("expected stddev 0 for empty slice, got %.3f", result)
	}
}

// ===============================
// calculateAnomalyConfidence Tests
// ===============================

func TestCalculateAnomalyConfidence_StrongDeviations(t *testing.T) {
	anomalies := []anomaly{
		{runID: "run-1", cost: 50, deviation: 5.0},
		{runID: "run-2", cost: 60, deviation: 6.0},
	}

	confidence := calculateAnomalyConfidence(anomalies, 2.0)

	// Strong deviations should produce high confidence
	if confidence < 0.7 {
		t.Errorf("expected high confidence for strong deviations, got %.2f", confidence)
	}
}

func TestCalculateAnomalyConfidence_WeakDeviations(t *testing.T) {
	anomalies := []anomaly{
		{runID: "run-1", cost: 30, deviation: 2.1},
	}

	confidence := calculateAnomalyConfidence(anomalies, 2.0)

	// Weak deviations should produce lower confidence
	if confidence >= 0.7 {
		t.Errorf("expected moderate or low confidence for weak deviations, got %.2f", confidence)
	}
}

func TestCalculateAnomalyConfidence_EmptyAnomalies(t *testing.T) {
	anomalies := []anomaly{}

	confidence := calculateAnomalyConfidence(anomalies, 2.0)

	if confidence != 0 {
		t.Errorf("expected confidence 0 for no anomalies, got %.2f", confidence)
	}
}

func TestCalculateAnomalyConfidence_CapsAtMaximum(t *testing.T) {
	anomalies := []anomaly{
		{runID: "run-1", cost: 100, deviation: 20.0},
		{runID: "run-2", cost: 110, deviation: 25.0},
	}

	confidence := calculateAnomalyConfidence(anomalies, 2.0)

	// Should be capped at 0.95
	if confidence > 0.95 {
		t.Errorf("expected confidence capped at 0.95, got %.2f", confidence)
	}
}

func TestCalculateAnomalyConfidence_FloorAtMinimum(t *testing.T) {
	anomalies := []anomaly{
		{runID: "run-1", cost: 11, deviation: 2.0},
	}

	confidence := calculateAnomalyConfidence(anomalies, 2.0)

	// Should have floor at 0.3
	if confidence < 0.3 {
		t.Errorf("expected confidence floor at 0.3, got %.2f", confidence)
	}
}

// ===============================
// Integration Tests
// ===============================

func TestCostAnomalyDetector_Detect_MultipleAnomalies(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create more baseline runs and stronger anomalies
	for i := 0; i < 8; i++ {
		createTestRunWithToolCallCount(ctx, t, eventStore, runStore, i, 10)
	}
	// Create multiple strong anomalies
	createTestRunWithToolCallCount(ctx, t, eventStore, runStore, 100, 60)
	createTestRunWithToolCallCount(ctx, t, eventStore, runStore, 101, 70)

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(5),
		WithDeviationThreshold(2.0),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one pattern")
	}

	for _, p := range patterns {
		var data pattern.CostAnomalyData
		if err := p.GetData(&data); err != nil {
			t.Fatalf("failed to get pattern data: %v", err)
		}
		if data.AnomalyCount < 1 {
			t.Errorf("expected at least 1 anomaly, got %d", data.AnomalyCount)
		}
	}
}

func TestCostAnomalyDetector_Detect_PatternDataIntegrity(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	for i := 0; i < 6; i++ {
		createTestRunWithToolCallCount(ctx, t, eventStore, runStore, i, 10)
	}
	createTestRunWithToolCallCount(ctx, t, eventStore, runStore, 100, 50)

	detector := NewCostAnomalyDetector(
		eventStore,
		runStore,
		WithMinSampleSize(5),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range patterns {
		// Verify pattern structure
		if p.Type != pattern.PatternTypeCostAnomaly {
			t.Errorf("expected PatternTypeCostAnomaly, got %s", p.Type)
		}
		if p.Name == "" {
			t.Error("expected non-empty pattern name")
		}
		if p.Description == "" {
			t.Error("expected non-empty pattern description")
		}
		if p.Confidence <= 0 || p.Confidence > 1 {
			t.Errorf("expected confidence in (0,1], got %.2f", p.Confidence)
		}

		// Verify data integrity
		var data pattern.CostAnomalyData
		if err := p.GetData(&data); err != nil {
			t.Fatalf("failed to get pattern data: %v", err)
		}
		if data.CostType == "" {
			t.Error("expected non-empty cost type")
		}
		if data.AverageCost < 0 {
			t.Errorf("expected non-negative average cost, got %.2f", data.AverageCost)
		}
		if data.AnomalyCost < 0 {
			t.Errorf("expected non-negative anomaly cost, got %.2f", data.AnomalyCost)
		}
		if data.AnomalyCount < 1 {
			t.Errorf("expected at least 1 anomaly, got %d", data.AnomalyCount)
		}

		// Verify evidence
		if len(p.Evidence) != data.AnomalyCount {
			t.Errorf("expected evidence count (%d) to match anomaly count (%d)", len(p.Evidence), data.AnomalyCount)
		}
		for _, e := range p.Evidence {
			if e.RunID == "" {
				t.Error("expected non-empty run ID in evidence")
			}
			if e.Details == nil {
				t.Error("expected non-nil evidence details")
			}
		}
	}
}

// ===============================
// Test Helpers
// ===============================

func createTestRunWithToolCallCount(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int, toolCallCount int) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now().Add(-time.Duration(index) * time.Hour)

	events := make([]event.Event, 0)
	for i := 0; i < toolCallCount; i++ {
		callPayload := event.ToolCalledPayload{
			ToolName: fmt.Sprintf("tool-%d", i),
			Input:    json.RawMessage(`{}`),
			State:    agent.StateAct,
		}
		callPayloadBytes, _ := json.Marshal(callPayload)

		events = append(events, event.Event{
			RunID:     r.ID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Payload:   callPayloadBytes,
		})

		succPayload := event.ToolSucceededPayload{
			ToolName: fmt.Sprintf("tool-%d", i),
			Output:   json.RawMessage(`{}`),
			Duration: 100 * time.Millisecond,
		}
		succPayloadBytes, _ := json.Marshal(succPayload)

		events = append(events, event.Event{
			RunID:     r.ID,
			Type:      event.TypeToolSucceeded,
			Timestamp: baseTime.Add(time.Duration(i)*time.Second + 100*time.Millisecond),
			Payload:   succPayloadBytes,
		})
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	return r.ID
}
