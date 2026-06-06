package pattern

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// ===============================
// Composite Detector Tests
// ===============================

func TestNewCompositeDetector_CreatesEmptyComposite(t *testing.T) {
	detector := NewCompositeDetector()

	if detector == nil {
		t.Fatal("expected non-nil detector")
	}
	if len(detector.detectors) != 0 {
		t.Errorf("expected 0 detectors, got %d", len(detector.detectors))
	}
}

func TestNewCompositeDetector_WithDetectors(t *testing.T) {
	mock1 := &mockDetector{types: []pattern.PatternType{pattern.PatternTypeToolSequence}}
	mock2 := &mockDetector{types: []pattern.PatternType{pattern.PatternTypeToolFailure}}

	detector := NewCompositeDetector(mock1, mock2)

	if len(detector.detectors) != 2 {
		t.Errorf("expected 2 detectors, got %d", len(detector.detectors))
	}
}

func TestCompositeDetector_AddDetector(t *testing.T) {
	detector := NewCompositeDetector()
	mock := &mockDetector{types: []pattern.PatternType{pattern.PatternTypeToolSequence}}

	detector.AddDetector(mock)

	if len(detector.detectors) != 1 {
		t.Errorf("expected 1 detector, got %d", len(detector.detectors))
	}
}

func TestCompositeDetector_Types_CombinesAllTypes(t *testing.T) {
	mock1 := &mockDetector{types: []pattern.PatternType{pattern.PatternTypeToolSequence}}
	mock2 := &mockDetector{types: []pattern.PatternType{pattern.PatternTypeToolFailure, pattern.PatternTypeSlowTool}}
	detector := NewCompositeDetector(mock1, mock2)

	types := detector.Types()

	if len(types) != 3 {
		t.Errorf("expected 3 types, got %d", len(types))
	}

	typeSet := make(map[pattern.PatternType]bool)
	for _, tp := range types {
		typeSet[tp] = true
	}
	if !typeSet[pattern.PatternTypeToolSequence] {
		t.Error("expected PatternTypeToolSequence")
	}
	if !typeSet[pattern.PatternTypeToolFailure] {
		t.Error("expected PatternTypeToolFailure")
	}
	if !typeSet[pattern.PatternTypeSlowTool] {
		t.Error("expected PatternTypeSlowTool")
	}
}

func TestCompositeDetector_Types_DeduplicatesTypes(t *testing.T) {
	mock1 := &mockDetector{types: []pattern.PatternType{pattern.PatternTypeToolSequence}}
	mock2 := &mockDetector{types: []pattern.PatternType{pattern.PatternTypeToolSequence}} // Same type
	detector := NewCompositeDetector(mock1, mock2)

	types := detector.Types()

	if len(types) != 1 {
		t.Errorf("expected 1 unique type, got %d", len(types))
	}
}

func TestCompositeDetector_Detect_CombinesResults(t *testing.T) {
	pattern1 := *pattern.NewPattern(pattern.PatternTypeToolSequence, "seq1", "desc1")
	pattern1.Confidence = 0.9
	pattern2 := *pattern.NewPattern(pattern.PatternTypeToolFailure, "fail1", "desc2")
	pattern2.Confidence = 0.8

	mock1 := &mockDetector{
		types:    []pattern.PatternType{pattern.PatternTypeToolSequence},
		patterns: []pattern.Pattern{pattern1},
	}
	mock2 := &mockDetector{
		types:    []pattern.PatternType{pattern.PatternTypeToolFailure},
		patterns: []pattern.Pattern{pattern2},
	}

	detector := NewCompositeDetector(mock1, mock2)
	ctx := context.Background()

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 2 {
		t.Errorf("expected 2 patterns, got %d", len(patterns))
	}
}

func TestCompositeDetector_Detect_SortsByConfidenceThenFrequency(t *testing.T) {
	p1 := *pattern.NewPattern(pattern.PatternTypeToolSequence, "low", "desc")
	p1.Confidence = 0.5
	p1.Frequency = 10

	p2 := *pattern.NewPattern(pattern.PatternTypeToolSequence, "high", "desc")
	p2.Confidence = 0.9
	p2.Frequency = 1

	p3 := *pattern.NewPattern(pattern.PatternTypeToolSequence, "mid-high-freq", "desc")
	p3.Confidence = 0.7
	p3.Frequency = 20

	p4 := *pattern.NewPattern(pattern.PatternTypeToolSequence, "mid-low-freq", "desc")
	p4.Confidence = 0.7
	p4.Frequency = 5

	mock := &mockDetector{
		types:    []pattern.PatternType{pattern.PatternTypeToolSequence},
		patterns: []pattern.Pattern{p1, p2, p3, p4},
	}

	detector := NewCompositeDetector(mock)
	ctx := context.Background()

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be sorted: p2 (0.9), p3 (0.7, 20), p4 (0.7, 5), p1 (0.5)
	if patterns[0].Name != "high" {
		t.Errorf("expected first pattern 'high', got '%s'", patterns[0].Name)
	}
	if patterns[1].Name != "mid-high-freq" {
		t.Errorf("expected second pattern 'mid-high-freq', got '%s'", patterns[1].Name)
	}
	if patterns[2].Name != "mid-low-freq" {
		t.Errorf("expected third pattern 'mid-low-freq', got '%s'", patterns[2].Name)
	}
	if patterns[3].Name != "low" {
		t.Errorf("expected fourth pattern 'low', got '%s'", patterns[3].Name)
	}
}

func TestCompositeDetector_Detect_AppliesLimit(t *testing.T) {
	patterns := make([]pattern.Pattern, 10)
	for i := 0; i < 10; i++ {
		p := *pattern.NewPattern(pattern.PatternTypeToolSequence, "pattern", "desc")
		p.Confidence = float64(10-i) / 10
		patterns[i] = p
	}

	mock := &mockDetector{
		types:    []pattern.PatternType{pattern.PatternTypeToolSequence},
		patterns: patterns,
	}

	detector := NewCompositeDetector(mock)
	ctx := context.Background()

	result, err := detector.Detect(ctx, pattern.DetectionOptions{Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 patterns with limit, got %d", len(result))
	}
}

func TestCompositeDetector_Detect_FiltersByPatternTypes(t *testing.T) {
	seqPattern := *pattern.NewPattern(pattern.PatternTypeToolSequence, "seq", "desc")
	failPattern := *pattern.NewPattern(pattern.PatternTypeToolFailure, "fail", "desc")

	mock1 := &mockDetector{
		types:    []pattern.PatternType{pattern.PatternTypeToolSequence},
		patterns: []pattern.Pattern{seqPattern},
	}
	mock2 := &mockDetector{
		types:    []pattern.PatternType{pattern.PatternTypeToolFailure},
		patterns: []pattern.Pattern{failPattern},
	}

	detector := NewCompositeDetector(mock1, mock2)
	ctx := context.Background()

	result, err := detector.Detect(ctx, pattern.DetectionOptions{
		PatternTypes: []pattern.PatternType{pattern.PatternTypeToolFailure},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only include failure patterns since we filtered by type
	if len(result) != 1 {
		t.Errorf("expected 1 pattern (filtered), got %d", len(result))
	}
	if len(result) > 0 && result[0].Type != pattern.PatternTypeToolFailure {
		t.Errorf("expected PatternTypeToolFailure, got %s", result[0].Type)
	}
}

func TestCompositeDetector_Detect_HandlesPartialFailure(t *testing.T) {
	successPattern := *pattern.NewPattern(pattern.PatternTypeToolSequence, "success", "desc")

	mock1 := &mockDetector{
		types:    []pattern.PatternType{pattern.PatternTypeToolSequence},
		patterns: []pattern.Pattern{successPattern},
	}
	mock2 := &mockDetector{
		types: []pattern.PatternType{pattern.PatternTypeToolFailure},
		err:   errors.New("detection failed"),
	}

	detector := NewCompositeDetector(mock1, mock2)
	ctx := context.Background()

	result, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("expected no error on partial failure, got: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 pattern from successful detector, got %d", len(result))
	}
}

func TestCompositeDetector_Detect_ReturnsErrorWhenAllFail(t *testing.T) {
	mock1 := &mockDetector{
		types: []pattern.PatternType{pattern.PatternTypeToolSequence},
		err:   errors.New("detection failed 1"),
	}
	mock2 := &mockDetector{
		types: []pattern.PatternType{pattern.PatternTypeToolFailure},
		err:   errors.New("detection failed 2"),
	}

	detector := NewCompositeDetector(mock1, mock2)
	ctx := context.Background()

	_, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err == nil {
		t.Error("expected error when all detectors fail")
	}
}

func TestCompositeDetector_Detect_EmptyDetectors(t *testing.T) {
	detector := NewCompositeDetector()
	ctx := context.Background()

	result, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(result))
	}
}

// ===============================
// Sequence Detector Tests
// ===============================

func TestNewSequenceDetector_Defaults(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewSequenceDetector(eventStore, runStore)

	if detector.minSequenceLen != 2 {
		t.Errorf("expected default minSequenceLen 2, got %d", detector.minSequenceLen)
	}
	if detector.minOccurrences != 3 {
		t.Errorf("expected default minOccurrences 3, got %d", detector.minOccurrences)
	}
}

func TestNewSequenceDetector_WithOptions(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewSequenceDetector(
		eventStore,
		runStore,
		WithMinSequenceLength(4),
		WithMinOccurrences(5),
	)

	if detector.minSequenceLen != 4 {
		t.Errorf("expected minSequenceLen 4, got %d", detector.minSequenceLen)
	}
	if detector.minOccurrences != 5 {
		t.Errorf("expected minOccurrences 5, got %d", detector.minOccurrences)
	}
}

func TestSequenceDetector_Types(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewSequenceDetector(eventStore, runStore)

	types := detector.Types()

	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
	if types[0] != pattern.PatternTypeToolSequence {
		t.Errorf("expected PatternTypeToolSequence, got %s", types[0])
	}
}

func TestSequenceDetector_Detect_FindsRepeatedSequences(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create 3 runs with the same tool sequence
	for i := 0; i < 3; i++ {
		runID := createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"read", "process", "write"})
		_ = runID
	}

	detector := NewSequenceDetector(eventStore, runStore, WithMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Error("expected at least one pattern to be detected")
	}

	// Verify we found a tool sequence pattern
	foundSequence := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeToolSequence {
			foundSequence = true
			break
		}
	}
	if !foundSequence {
		t.Error("expected to find a tool sequence pattern")
	}
}

func TestSequenceDetector_Detect_FiltersByRunIDs(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs
	runID1 := createTestRunWithTools(ctx, t, eventStore, runStore, 1, []string{"a", "b"})
	runID2 := createTestRunWithTools(ctx, t, eventStore, runStore, 2, []string{"a", "b"})
	createTestRunWithTools(ctx, t, eventStore, runStore, 3, []string{"a", "b"})

	detector := NewSequenceDetector(eventStore, runStore, WithMinOccurrences(2))

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

func TestSequenceDetector_Detect_FiltersbyConfidence(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with sequences
	for i := 0; i < 3; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"x", "y"})
	}

	detector := NewSequenceDetector(eventStore, runStore, WithMinOccurrences(2))

	// High confidence filter should reduce results
	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		MinConfidence: 0.99, // Very high threshold
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With such high confidence requirement, we may get no patterns
	// This test verifies the filtering works
	for _, p := range patterns {
		if p.Confidence < 0.99 {
			t.Errorf("expected pattern confidence >= 0.99, got %f", p.Confidence)
		}
	}
}

func TestSequenceDetector_Detect_AppliesLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create multiple runs with different sequences
	for i := 0; i < 5; i++ {
		createTestRunWithTools(ctx, t, eventStore, runStore, i, []string{"a", "b", "c", "d", "e"})
	}

	detector := NewSequenceDetector(eventStore, runStore, WithMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 1 {
		t.Errorf("expected at most 1 pattern with limit, got %d", len(patterns))
	}
}

func TestSequenceDetector_Detect_NoPatterns(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create run with short sequence
	createTestRunWithTools(ctx, t, eventStore, runStore, 1, []string{"single"})

	detector := NewSequenceDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for short sequence, got %d", len(patterns))
	}
}

func TestSequenceDetector_Detect_EmptyStore(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewSequenceDetector(eventStore, runStore)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for empty store, got %d", len(patterns))
	}
}

// ===============================
// Failure Detector Tests
// ===============================

func TestNewFailureDetector_Defaults(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewFailureDetector(eventStore, runStore)

	if detector.minOccurrences != 3 {
		t.Errorf("expected default minOccurrences 3, got %d", detector.minOccurrences)
	}
}

func TestNewFailureDetector_WithOptions(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewFailureDetector(eventStore, runStore, WithFailureMinOccurrences(5))

	if detector.minOccurrences != 5 {
		t.Errorf("expected minOccurrences 5, got %d", detector.minOccurrences)
	}
}

func TestFailureDetector_Types(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewFailureDetector(eventStore, runStore)

	types := detector.Types()

	if len(types) != 3 {
		t.Errorf("expected 3 types, got %d", len(types))
	}

	typeSet := make(map[pattern.PatternType]bool)
	for _, tp := range types {
		typeSet[tp] = true
	}

	if !typeSet[pattern.PatternTypeToolFailure] {
		t.Error("expected PatternTypeToolFailure")
	}
	if !typeSet[pattern.PatternTypeRecurringFailure] {
		t.Error("expected PatternTypeRecurringFailure")
	}
	if !typeSet[pattern.PatternTypeBudgetExhaustion] {
		t.Error("expected PatternTypeBudgetExhaustion")
	}
}

func TestFailureDetector_Detect_FindsToolFailures(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with the same tool failure
	for i := 0; i < 3; i++ {
		createTestRunWithFailure(ctx, t, eventStore, runStore, i, "failing_tool", "connection refused")
	}

	detector := NewFailureDetector(eventStore, runStore, WithFailureMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one failure pattern")
	}

	// Verify we found a tool failure pattern
	foundFailure := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeToolFailure {
			foundFailure = true
			if p.Frequency < 2 {
				t.Errorf("expected frequency >= 2, got %d", p.Frequency)
			}
		}
	}
	if !foundFailure {
		t.Error("expected to find a tool failure pattern")
	}
}

func TestFailureDetector_Detect_FindsBudgetExhaustion(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with budget exhaustion
	for i := 0; i < 3; i++ {
		createTestRunWithBudgetExhaustion(ctx, t, eventStore, runStore, i)
	}

	detector := NewFailureDetector(eventStore, runStore, WithFailureMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we found a budget exhaustion pattern
	foundBudgetExhaustion := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeBudgetExhaustion {
			foundBudgetExhaustion = true
		}
	}
	if !foundBudgetExhaustion {
		t.Error("expected to find a budget exhaustion pattern")
	}
}

func TestFailureDetector_Detect_ClassifiesErrorTypes(t *testing.T) {
	testCases := []struct {
		errorMsg     string
		expectedType string
	}{
		{"connection timeout exceeded", "timeout"},
		{"deadline exceeded", "timeout"},
		{"connection refused", "network"},
		{"network unreachable", "network"},
		{"permission denied", "permission"},
		{"forbidden access", "permission"},
		{"file not found", "not_found"},
		{"invalid input format", "validation"},
		{"rate limit exceeded", "rate_limit"},
		{"too many requests", "rate_limit"},
		{"some random error", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.errorMsg, func(t *testing.T) {
			result := classifyError(tc.errorMsg)
			if result != tc.expectedType {
				t.Errorf("classifyError(%q) = %q, want %q", tc.errorMsg, result, tc.expectedType)
			}
		})
	}
}

func TestFailureDetector_Detect_FiltersByConfidence(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create failures
	for i := 0; i < 3; i++ {
		createTestRunWithFailure(ctx, t, eventStore, runStore, i, "tool", "error")
	}

	detector := NewFailureDetector(eventStore, runStore, WithFailureMinOccurrences(2))

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

func TestFailureDetector_Detect_AppliesLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create different failure types
	for i := 0; i < 5; i++ {
		createTestRunWithFailure(ctx, t, eventStore, runStore, i, "tool_a", "error a")
		createTestRunWithFailure(ctx, t, eventStore, runStore, i+100, "tool_b", "error b")
	}

	detector := NewFailureDetector(eventStore, runStore, WithFailureMinOccurrences(2))

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 1 {
		t.Errorf("expected at most 1 pattern with limit, got %d", len(patterns))
	}
}

// ===============================
// Performance Detector Tests
// ===============================

func TestNewPerformanceDetector_Defaults(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewPerformanceDetector(eventStore, runStore)

	if detector.slowToolThreshold != 5*time.Second {
		t.Errorf("expected default slowToolThreshold 5s, got %v", detector.slowToolThreshold)
	}
	if detector.longRunThreshold != 5*time.Minute {
		t.Errorf("expected default longRunThreshold 5m, got %v", detector.longRunThreshold)
	}
	if detector.minOccurrences != 3 {
		t.Errorf("expected default minOccurrences 3, got %d", detector.minOccurrences)
	}
	if detector.stdDevMultiplier != 2.0 {
		t.Errorf("expected default stdDevMultiplier 2.0, got %f", detector.stdDevMultiplier)
	}
}

func TestNewPerformanceDetector_WithOptions(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	detector := NewPerformanceDetector(
		eventStore,
		runStore,
		WithSlowToolThreshold(10*time.Second),
		WithLongRunThreshold(10*time.Minute),
		WithPerformanceMinOccurrences(5),
		WithStdDevMultiplier(3.0),
	)

	if detector.slowToolThreshold != 10*time.Second {
		t.Errorf("expected slowToolThreshold 10s, got %v", detector.slowToolThreshold)
	}
	if detector.longRunThreshold != 10*time.Minute {
		t.Errorf("expected longRunThreshold 10m, got %v", detector.longRunThreshold)
	}
	if detector.minOccurrences != 5 {
		t.Errorf("expected minOccurrences 5, got %d", detector.minOccurrences)
	}
	if detector.stdDevMultiplier != 3.0 {
		t.Errorf("expected stdDevMultiplier 3.0, got %f", detector.stdDevMultiplier)
	}
}

func TestPerformanceDetector_Types(t *testing.T) {
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()
	detector := NewPerformanceDetector(eventStore, runStore)

	types := detector.Types()

	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d", len(types))
	}

	typeSet := make(map[pattern.PatternType]bool)
	for _, tp := range types {
		typeSet[tp] = true
	}

	if !typeSet[pattern.PatternTypeSlowTool] {
		t.Error("expected PatternTypeSlowTool")
	}
	if !typeSet[pattern.PatternTypeLongRuns] {
		t.Error("expected PatternTypeLongRuns")
	}
}

func TestPerformanceDetector_Detect_FindsSlowTools(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create runs with slow tool execution
	for i := 0; i < 3; i++ {
		createTestRunWithSlowTool(ctx, t, eventStore, runStore, i, "slow_tool", 10*time.Second)
	}

	detector := NewPerformanceDetector(
		eventStore,
		runStore,
		WithSlowToolThreshold(5*time.Second),
		WithPerformanceMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundSlowTool := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeSlowTool {
			foundSlowTool = true
		}
	}
	if !foundSlowTool {
		t.Error("expected to find a slow tool pattern")
	}
}

func TestPerformanceDetector_Detect_FindsLongRuns(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create long-running runs
	for i := 0; i < 3; i++ {
		createTestRunWithDuration(ctx, t, eventStore, runStore, i, 10*time.Minute)
	}

	detector := NewPerformanceDetector(
		eventStore,
		runStore,
		WithLongRunThreshold(5*time.Minute),
		WithPerformanceMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundLongRuns := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeLongRuns {
			foundLongRuns = true
		}
	}
	if !foundLongRuns {
		t.Error("expected to find a long runs pattern")
	}
}

func TestPerformanceDetector_Detect_FiltersByConfidence(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	for i := 0; i < 3; i++ {
		createTestRunWithSlowTool(ctx, t, eventStore, runStore, i, "tool", 10*time.Second)
	}

	detector := NewPerformanceDetector(
		eventStore,
		runStore,
		WithSlowToolThreshold(5*time.Second),
		WithPerformanceMinOccurrences(2),
	)

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

func TestPerformanceDetector_Detect_AppliesLimit(t *testing.T) {
	ctx := context.Background()
	eventStore := memory.NewEventStore()
	runStore := memory.NewRunStore()

	// Create multiple slow tools
	for i := 0; i < 5; i++ {
		createTestRunWithSlowTool(ctx, t, eventStore, runStore, i, "tool_a", 10*time.Second)
		createTestRunWithSlowTool(ctx, t, eventStore, runStore, i+100, "tool_b", 10*time.Second)
	}

	detector := NewPerformanceDetector(
		eventStore,
		runStore,
		WithSlowToolThreshold(5*time.Second),
		WithPerformanceMinOccurrences(2),
	)

	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) > 1 {
		t.Errorf("expected at most 1 pattern with limit, got %d", len(patterns))
	}
}

// ===============================
// Helper Functions
// ===============================

// mockDetector is a test implementation of pattern.Detector
type mockDetector struct {
	types    []pattern.PatternType
	patterns []pattern.Pattern
	err      error
}

func (m *mockDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.patterns, nil
}

func (m *mockDetector) Types() []pattern.PatternType {
	return m.types
}

var _ pattern.Detector = (*mockDetector)(nil)

func createTestRunWithTools(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int, tools []string) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now().Add(-time.Duration(index) * time.Hour)

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

func createTestRunWithFailure(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int, toolName, errorMsg string) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
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
		Duration: 100 * time.Millisecond,
	}
	failPayloadBytes, _ := json.Marshal(failPayload)

	events := []event.Event{
		{
			RunID:     r.ID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime,
			Payload:   callPayloadBytes,
		},
		{
			RunID:     r.ID,
			Type:      event.TypeToolFailed,
			Timestamp: baseTime.Add(100 * time.Millisecond),
			Payload:   failPayloadBytes,
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	return r.ID
}

func createTestRunWithBudgetExhaustion(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
	r.Status = agent.RunStatusFailed
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now().Add(-time.Duration(index) * time.Hour)

	payload := event.BudgetExhaustedPayload{
		BudgetName: "tool_calls",
	}
	payloadBytes, _ := json.Marshal(payload)

	events := []event.Event{
		{
			RunID:     r.ID,
			Type:      event.TypeBudgetExhausted,
			Timestamp: baseTime,
			Payload:   payloadBytes,
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	return r.ID
}

func createTestRunWithSlowTool(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int, toolName string, duration time.Duration) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
	r.Status = agent.RunStatusCompleted
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

	succPayload := event.ToolSucceededPayload{
		ToolName: toolName,
		Output:   json.RawMessage(`{}`),
		Duration: duration,
	}
	succPayloadBytes, _ := json.Marshal(succPayload)

	events := []event.Event{
		{
			RunID:     r.ID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime,
			Payload:   callPayloadBytes,
		},
		{
			RunID:     r.ID,
			Type:      event.TypeToolSucceeded,
			Timestamp: baseTime.Add(duration),
			Payload:   succPayloadBytes,
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	return r.ID
}

func createTestRunWithDuration(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runStore *memory.RunStore, index int, duration time.Duration) string {
	t.Helper()

	r := agent.NewRun(fmt.Sprintf("run-%d", index), "test goal")
	r.Status = agent.RunStatusCompleted
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	baseTime := time.Now().Add(-time.Duration(index) * time.Hour)

	startPayload := event.RunStartedPayload{
		Goal: "test goal",
	}
	startPayloadBytes, _ := json.Marshal(startPayload)

	endPayload := event.RunCompletedPayload{
		Duration: duration,
	}
	endPayloadBytes, _ := json.Marshal(endPayload)

	events := []event.Event{
		{
			RunID:     r.ID,
			Type:      event.TypeRunStarted,
			Timestamp: baseTime,
			Payload:   startPayloadBytes,
		},
		{
			RunID:     r.ID,
			Type:      event.TypeRunCompleted,
			Timestamp: baseTime.Add(duration),
			Payload:   endPayloadBytes,
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}

	return r.ID
}

// ===============================
// Confidence Calculation Tests
// ===============================

func TestCalculateSequenceConfidence(t *testing.T) {
	tests := []struct {
		name        string
		occurrences int
		minExpected float64
		maxExpected float64
	}{
		{"single occurrence", 1, 0.5, 0.6},
		{"few occurrences", 3, 0.7, 0.85},
		{"many occurrences", 10, 0.9, 0.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			occurrences := make([]sequenceOccurrence, tt.occurrences)
			for i := 0; i < tt.occurrences; i++ {
				occurrences[i] = sequenceOccurrence{runID: "run"}
			}

			confidence := calculateSequenceConfidence(occurrences)

			if confidence < tt.minExpected || confidence > tt.maxExpected {
				t.Errorf("expected confidence in [%f, %f], got %f", tt.minExpected, tt.maxExpected, confidence)
			}
		})
	}
}

func TestCalculateFailureConfidence(t *testing.T) {
	tests := []struct {
		name        string
		count       int
		minExpected float64
		maxExpected float64
	}{
		{"single failure", 1, 0.5, 0.6},
		{"few failures", 3, 0.7, 0.85},
		{"many failures", 10, 0.9, 0.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateFailureConfidence(tt.count)

			if confidence < tt.minExpected || confidence > tt.maxExpected {
				t.Errorf("expected confidence in [%f, %f], got %f", tt.minExpected, tt.maxExpected, confidence)
			}
		})
	}
}

func TestCalculatePerformanceConfidence(t *testing.T) {
	tests := []struct {
		name         string
		anomalyCount int
		totalCount   int
		minExpected  float64
		maxExpected  float64
	}{
		{"no data", 0, 0, 0.5, 0.5},
		{"low ratio", 1, 10, 0.5, 0.7},
		{"high ratio", 8, 10, 0.7, 0.95},
		{"all anomalies", 10, 10, 0.8, 0.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculatePerformanceConfidence(tt.anomalyCount, tt.totalCount)

			if confidence < tt.minExpected || confidence > tt.maxExpected {
				t.Errorf("expected confidence in [%f, %f], got %f", tt.minExpected, tt.maxExpected, confidence)
			}
		})
	}
}

// ===============================
// Helper Function Tests
// ===============================

func TestExtractToolCalls(t *testing.T) {
	payload := event.ToolCalledPayload{
		ToolName: "test_tool",
		Input:    json.RawMessage(`{}`),
		State:    agent.StateExplore,
	}
	payloadBytes, _ := json.Marshal(payload)

	events := []event.Event{
		{
			Type:      event.TypeToolCalled,
			Timestamp: time.Now(),
			Payload:   payloadBytes,
		},
		{
			Type:      event.TypeStateTransitioned,
			Timestamp: time.Now(),
			Payload:   nil,
		},
	}

	calls := extractToolCalls(events)

	if len(calls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got %s", calls[0].name)
	}
}

func TestToolNames(t *testing.T) {
	calls := []toolCall{
		{name: "a", timestamp: time.Now()},
		{name: "b", timestamp: time.Now()},
		{name: "c", timestamp: time.Now()},
	}

	names := toolNames(calls)

	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}
	if names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "foo", false},
		{"", "test", false},
		{"test", "", true},
		{"test", "TEST", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}
