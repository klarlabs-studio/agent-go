package suggestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/suggestion"
)

// ===============================
// Composite Generator Tests
// ===============================

func TestNewCompositeGenerator_CreatesEmptyComposite(t *testing.T) {
	generator := NewCompositeGenerator()

	if generator == nil {
		t.Fatal("expected non-nil generator")
	}
	if len(generator.generators) != 0 {
		t.Errorf("expected 0 generators, got %d", len(generator.generators))
	}
}

func TestNewCompositeGenerator_WithGenerators(t *testing.T) {
	mock1 := &mockGenerator{types: []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility}}
	mock2 := &mockGenerator{types: []suggestion.SuggestionType{suggestion.SuggestionTypeIncreaseBudget}}

	generator := NewCompositeGenerator(mock1, mock2)

	if len(generator.generators) != 2 {
		t.Errorf("expected 2 generators, got %d", len(generator.generators))
	}
}

func TestCompositeGenerator_AddGenerator(t *testing.T) {
	generator := NewCompositeGenerator()
	mock := &mockGenerator{types: []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility}}

	generator.AddGenerator(mock)

	if len(generator.generators) != 1 {
		t.Errorf("expected 1 generator, got %d", len(generator.generators))
	}
}

func TestCompositeGenerator_Types_CombinesAllTypes(t *testing.T) {
	mock1 := &mockGenerator{types: []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility}}
	mock2 := &mockGenerator{types: []suggestion.SuggestionType{suggestion.SuggestionTypeIncreaseBudget, suggestion.SuggestionTypeDecreaseBudget}}
	generator := NewCompositeGenerator(mock1, mock2)

	types := generator.Types()

	if len(types) != 3 {
		t.Errorf("expected 3 types, got %d", len(types))
	}

	typeSet := make(map[suggestion.SuggestionType]bool)
	for _, tp := range types {
		typeSet[tp] = true
	}
	if !typeSet[suggestion.SuggestionTypeAddEligibility] {
		t.Error("expected SuggestionTypeAddEligibility")
	}
	if !typeSet[suggestion.SuggestionTypeIncreaseBudget] {
		t.Error("expected SuggestionTypeIncreaseBudget")
	}
	if !typeSet[suggestion.SuggestionTypeDecreaseBudget] {
		t.Error("expected SuggestionTypeDecreaseBudget")
	}
}

func TestCompositeGenerator_Types_DeduplicatesTypes(t *testing.T) {
	mock1 := &mockGenerator{types: []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility}}
	mock2 := &mockGenerator{types: []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility}} // Same type
	generator := NewCompositeGenerator(mock1, mock2)

	types := generator.Types()

	if len(types) != 1 {
		t.Errorf("expected 1 unique type, got %d", len(types))
	}
}

func TestCompositeGenerator_Generate_CombinesResults(t *testing.T) {
	sug1 := createTestSuggestion(suggestion.SuggestionTypeAddEligibility, "tool1", 0.9)
	sug2 := createTestSuggestion(suggestion.SuggestionTypeIncreaseBudget, "budget1", 0.8)

	mock1 := &mockGenerator{
		types:       []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility},
		suggestions: []suggestion.Suggestion{sug1},
	}
	mock2 := &mockGenerator{
		types:       []suggestion.SuggestionType{suggestion.SuggestionTypeIncreaseBudget},
		suggestions: []suggestion.Suggestion{sug2},
	}

	generator := NewCompositeGenerator(mock1, mock2)
	ctx := context.Background()

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(suggestions))
	}
}

func TestCompositeGenerator_Generate_SortsByConfidenceThenImpact(t *testing.T) {
	lowConfHighImpact := createTestSuggestionWithImpact("low-conf-high-imp", 0.5, suggestion.ImpactLevelHigh)
	highConfLowImpact := createTestSuggestionWithImpact("high-conf-low-imp", 0.9, suggestion.ImpactLevelLow)
	midConfMidImpact := createTestSuggestionWithImpact("mid-conf-mid-imp", 0.7, suggestion.ImpactLevelMedium)
	midConfHighImpact := createTestSuggestionWithImpact("mid-conf-high-imp", 0.7, suggestion.ImpactLevelHigh)

	mock := &mockGenerator{
		types:       []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility},
		suggestions: []suggestion.Suggestion{lowConfHighImpact, highConfLowImpact, midConfMidImpact, midConfHighImpact},
	}

	generator := NewCompositeGenerator(mock)
	ctx := context.Background()

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be sorted: high-conf-low-imp (0.9), mid-conf-high-imp (0.7, high), mid-conf-mid-imp (0.7, medium), low-conf-high-imp (0.5)
	if suggestions[0].Title != "high-conf-low-imp" {
		t.Errorf("expected first suggestion 'high-conf-low-imp', got '%s'", suggestions[0].Title)
	}
	if suggestions[1].Title != "mid-conf-high-imp" {
		t.Errorf("expected second suggestion 'mid-conf-high-imp', got '%s'", suggestions[1].Title)
	}
	if suggestions[2].Title != "mid-conf-mid-imp" {
		t.Errorf("expected third suggestion 'mid-conf-mid-imp', got '%s'", suggestions[2].Title)
	}
	if suggestions[3].Title != "low-conf-high-imp" {
		t.Errorf("expected fourth suggestion 'low-conf-high-imp', got '%s'", suggestions[3].Title)
	}
}

func TestCompositeGenerator_Generate_DeduplicatesSuggestions(t *testing.T) {
	// Two suggestions with the same type, target, and To value
	sug1 := createTestSuggestion(suggestion.SuggestionTypeAddEligibility, "tool1", 0.9)
	sug2 := createTestSuggestion(suggestion.SuggestionTypeAddEligibility, "tool1", 0.8)

	mock := &mockGenerator{
		types:       []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility},
		suggestions: []suggestion.Suggestion{sug1, sug2},
	}

	generator := NewCompositeGenerator(mock)
	ctx := context.Background()

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 1 {
		t.Errorf("expected 1 deduplicated suggestion, got %d", len(suggestions))
	}
}

func TestCompositeGenerator_Generate_HandlesPartialFailure(t *testing.T) {
	successSug := createTestSuggestion(suggestion.SuggestionTypeAddEligibility, "tool1", 0.9)

	mock1 := &mockGenerator{
		types:       []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility},
		suggestions: []suggestion.Suggestion{successSug},
	}
	mock2 := &mockGenerator{
		types: []suggestion.SuggestionType{suggestion.SuggestionTypeIncreaseBudget},
		err:   errors.New("generation failed"),
	}

	generator := NewCompositeGenerator(mock1, mock2)
	ctx := context.Background()

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{})
	if err != nil {
		t.Fatalf("expected no error on partial failure, got: %v", err)
	}

	if len(suggestions) != 1 {
		t.Errorf("expected 1 suggestion from successful generator, got %d", len(suggestions))
	}
}

func TestCompositeGenerator_Generate_ReturnsErrorWhenAllFail(t *testing.T) {
	mock1 := &mockGenerator{
		types: []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility},
		err:   errors.New("generation failed 1"),
	}
	mock2 := &mockGenerator{
		types: []suggestion.SuggestionType{suggestion.SuggestionTypeIncreaseBudget},
		err:   errors.New("generation failed 2"),
	}

	generator := NewCompositeGenerator(mock1, mock2)
	ctx := context.Background()

	_, err := generator.Generate(ctx, []pattern.Pattern{})
	if err == nil {
		t.Error("expected error when all generators fail")
	}
}

func TestCompositeGenerator_Generate_EmptyGenerators(t *testing.T) {
	generator := NewCompositeGenerator()
	ctx := context.Background()

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions, got %d", len(suggestions))
	}
}

// ===============================
// Eligibility Generator Tests
// ===============================

func TestNewEligibilityGenerator_Defaults(t *testing.T) {
	generator := NewEligibilityGenerator()

	if generator.minConfidence != 0.6 {
		t.Errorf("expected default minConfidence 0.6, got %f", generator.minConfidence)
	}
}

func TestNewEligibilityGenerator_WithOptions(t *testing.T) {
	generator := NewEligibilityGenerator(WithEligibilityMinConfidence(0.8))

	if generator.minConfidence != 0.8 {
		t.Errorf("expected minConfidence 0.8, got %f", generator.minConfidence)
	}
}

func TestEligibilityGenerator_Types(t *testing.T) {
	generator := NewEligibilityGenerator()
	types := generator.Types()

	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d", len(types))
	}

	typeSet := make(map[suggestion.SuggestionType]bool)
	for _, tp := range types {
		typeSet[tp] = true
	}

	if !typeSet[suggestion.SuggestionTypeAddEligibility] {
		t.Error("expected SuggestionTypeAddEligibility")
	}
	if !typeSet[suggestion.SuggestionTypeRemoveEligibility] {
		t.Error("expected SuggestionTypeRemoveEligibility")
	}
}

func TestEligibilityGenerator_Generate_FiltersLowConfidence(t *testing.T) {
	generator := NewEligibilityGenerator(WithEligibilityMinConfidence(0.8))
	ctx := context.Background()

	// Create pattern with low confidence
	p := *pattern.NewPattern(pattern.PatternTypeToolSequence, "low confidence", "test")
	p.Confidence = 0.5

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for low confidence pattern, got %d", len(suggestions))
	}
}

func TestEligibilityGenerator_Generate_FromToolSequence(t *testing.T) {
	generator := NewEligibilityGenerator(WithEligibilityMinConfidence(0.5))
	ctx := context.Background()

	// Create tool sequence pattern with data
	p := *pattern.NewPattern(pattern.PatternTypeToolSequence, "sequence", "test")
	p.Confidence = 0.8
	p.Frequency = 10

	data := pattern.ToolSequenceData{
		Sequence: []string{"read", "write"},
		States:   []agent.State{agent.StateExplore},
	}
	_ = p.SetData(data)

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should generate suggestions for tools in states
	if len(suggestions) == 0 {
		t.Error("expected suggestions from tool sequence pattern")
	}

	// Verify suggestions are of correct type
	for _, s := range suggestions {
		if s.Type != suggestion.SuggestionTypeAddEligibility {
			t.Errorf("expected SuggestionTypeAddEligibility, got %s", s.Type)
		}
	}
}

func TestEligibilityGenerator_Generate_FromToolFailure(t *testing.T) {
	generator := NewEligibilityGenerator(WithEligibilityMinConfidence(0.5))
	ctx := context.Background()

	// Create tool failure pattern with data
	p := *pattern.NewPattern(pattern.PatternTypeToolFailure, "failure", "test")
	p.Confidence = 0.8
	p.Frequency = 10

	data := pattern.ToolFailureData{
		ToolName:   "failing_tool",
		ErrorType:  "timeout",
		ErrorCount: 10,
	}
	_ = p.SetData(data)

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("expected suggestions from tool failure pattern")
	}

	// Verify suggestion type
	for _, s := range suggestions {
		if s.Type != suggestion.SuggestionTypeRemoveEligibility {
			t.Errorf("expected SuggestionTypeRemoveEligibility, got %s", s.Type)
		}
	}
}

func TestEligibilityGenerator_Generate_FromToolAffinity(t *testing.T) {
	generator := NewEligibilityGenerator(WithEligibilityMinConfidence(0.5))
	ctx := context.Background()

	// Create tool affinity pattern with data
	p := *pattern.NewPattern(pattern.PatternTypeToolAffinity, "affinity", "test")
	p.Confidence = 0.8

	data := pattern.ToolAffinityData{
		Tools:       []string{"tool_a", "tool_b"},
		Correlation: 0.9,
	}
	_ = p.SetData(data)

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("expected suggestions from tool affinity pattern")
	}
}

func TestEligibilityGenerator_Generate_IgnoresUnknownPatternTypes(t *testing.T) {
	generator := NewEligibilityGenerator()
	ctx := context.Background()

	// Create a pattern type that eligibility generator doesn't handle
	p := *pattern.NewPattern(pattern.PatternTypeBudgetExhaustion, "budget", "test")
	p.Confidence = 0.9

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for budget exhaustion pattern, got %d", len(suggestions))
	}
}

func TestEligibilityGenerator_Generate_EmptyPatterns(t *testing.T) {
	generator := NewEligibilityGenerator()
	ctx := context.Background()

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for empty patterns, got %d", len(suggestions))
	}
}

// ===============================
// Budget Generator Tests
// ===============================

func TestNewBudgetGenerator_Defaults(t *testing.T) {
	generator := NewBudgetGenerator()

	if generator.minConfidence != 0.6 {
		t.Errorf("expected default minConfidence 0.6, got %f", generator.minConfidence)
	}
	if generator.budgetIncreaseRate != 0.25 {
		t.Errorf("expected default budgetIncreaseRate 0.25, got %f", generator.budgetIncreaseRate)
	}
	if generator.budgetDecreaseRate != 0.10 {
		t.Errorf("expected default budgetDecreaseRate 0.10, got %f", generator.budgetDecreaseRate)
	}
}

func TestNewBudgetGenerator_WithOptions(t *testing.T) {
	generator := NewBudgetGenerator(
		WithBudgetMinConfidence(0.8),
		WithBudgetIncreaseRate(0.5),
		WithBudgetDecreaseRate(0.2),
	)

	if generator.minConfidence != 0.8 {
		t.Errorf("expected minConfidence 0.8, got %f", generator.minConfidence)
	}
	if generator.budgetIncreaseRate != 0.5 {
		t.Errorf("expected budgetIncreaseRate 0.5, got %f", generator.budgetIncreaseRate)
	}
	if generator.budgetDecreaseRate != 0.2 {
		t.Errorf("expected budgetDecreaseRate 0.2, got %f", generator.budgetDecreaseRate)
	}
}

func TestBudgetGenerator_Types(t *testing.T) {
	generator := NewBudgetGenerator()
	types := generator.Types()

	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d", len(types))
	}

	typeSet := make(map[suggestion.SuggestionType]bool)
	for _, tp := range types {
		typeSet[tp] = true
	}

	if !typeSet[suggestion.SuggestionTypeIncreaseBudget] {
		t.Error("expected SuggestionTypeIncreaseBudget")
	}
	if !typeSet[suggestion.SuggestionTypeDecreaseBudget] {
		t.Error("expected SuggestionTypeDecreaseBudget")
	}
}

func TestBudgetGenerator_Generate_FiltersLowConfidence(t *testing.T) {
	generator := NewBudgetGenerator(WithBudgetMinConfidence(0.8))
	ctx := context.Background()

	// Create pattern with low confidence
	p := *pattern.NewPattern(pattern.PatternTypeBudgetExhaustion, "low confidence", "test")
	p.Confidence = 0.5

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for low confidence pattern, got %d", len(suggestions))
	}
}

func TestBudgetGenerator_Generate_FromBudgetExhaustion(t *testing.T) {
	generator := NewBudgetGenerator(WithBudgetMinConfidence(0.5))
	ctx := context.Background()

	// Create budget exhaustion pattern with data
	p := *pattern.NewPattern(pattern.PatternTypeBudgetExhaustion, "exhaustion", "test")
	p.Confidence = 0.8

	data := pattern.BudgetExhaustionData{
		ExhaustionCount: 5,
	}
	_ = p.SetData(data)

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("expected suggestions from budget exhaustion pattern")
	}

	// Verify suggestion type
	for _, s := range suggestions {
		if s.Type != suggestion.SuggestionTypeIncreaseBudget {
			t.Errorf("expected SuggestionTypeIncreaseBudget, got %s", s.Type)
		}
	}
}

func TestBudgetGenerator_Generate_FromLongRuns(t *testing.T) {
	generator := NewBudgetGenerator(WithBudgetMinConfidence(0.5))
	ctx := context.Background()

	// Create long runs pattern with data
	p := *pattern.NewPattern(pattern.PatternTypeLongRuns, "long runs", "test")
	p.Confidence = 0.8

	data := pattern.LongRunsData{
		AverageDuration: 10 * time.Minute,
		Threshold:       5 * time.Minute,
		LongRunCount:    10,
	}
	_ = p.SetData(data)

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("expected suggestions from long runs pattern")
	}

	// Verify suggestion type
	for _, s := range suggestions {
		if s.Type != suggestion.SuggestionTypeIncreaseBudget {
			t.Errorf("expected SuggestionTypeIncreaseBudget, got %s", s.Type)
		}
	}
}

func TestBudgetGenerator_Generate_BudgetExhaustion_LowCount(t *testing.T) {
	generator := NewBudgetGenerator(WithBudgetMinConfidence(0.5))
	ctx := context.Background()

	// Create budget exhaustion pattern with low count
	p := *pattern.NewPattern(pattern.PatternTypeBudgetExhaustion, "exhaustion", "test")
	p.Confidence = 0.8

	data := pattern.BudgetExhaustionData{
		ExhaustionCount: 2, // Below threshold of 3
	}
	_ = p.SetData(data)

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for low exhaustion count, got %d", len(suggestions))
	}
}

func TestBudgetGenerator_Generate_LongRuns_LowCount(t *testing.T) {
	generator := NewBudgetGenerator(WithBudgetMinConfidence(0.5))
	ctx := context.Background()

	// Create long runs pattern with low count
	p := *pattern.NewPattern(pattern.PatternTypeLongRuns, "long runs", "test")
	p.Confidence = 0.8

	data := pattern.LongRunsData{
		AverageDuration: 10 * time.Minute,
		Threshold:       5 * time.Minute,
		LongRunCount:    3, // Below threshold of 5
	}
	_ = p.SetData(data)

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for low long run count, got %d", len(suggestions))
	}
}

func TestBudgetGenerator_Generate_IgnoresUnknownPatternTypes(t *testing.T) {
	generator := NewBudgetGenerator()
	ctx := context.Background()

	// Create a pattern type that budget generator doesn't handle
	p := *pattern.NewPattern(pattern.PatternTypeToolSequence, "sequence", "test")
	p.Confidence = 0.9

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for tool sequence pattern, got %d", len(suggestions))
	}
}

func TestBudgetGenerator_Generate_EmptyPatterns(t *testing.T) {
	generator := NewBudgetGenerator()
	ctx := context.Background()

	suggestions, err := generator.Generate(ctx, []pattern.Pattern{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for empty patterns, got %d", len(suggestions))
	}
}

// ===============================
// Helper Functions
// ===============================

func TestImpactWeight(t *testing.T) {
	tests := []struct {
		impact   suggestion.ImpactLevel
		expected int
	}{
		{suggestion.ImpactLevelHigh, 3},
		{suggestion.ImpactLevelMedium, 2},
		{suggestion.ImpactLevelLow, 1},
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.impact), func(t *testing.T) {
			weight := impactWeight(tt.impact)
			if weight != tt.expected {
				t.Errorf("impactWeight(%q) = %d, want %d", tt.impact, weight, tt.expected)
			}
		})
	}
}

func TestDeduplicateSuggestions(t *testing.T) {
	// Create duplicates with same type, target, and To
	sug1 := createTestSuggestion(suggestion.SuggestionTypeAddEligibility, "tool1", 0.9)
	sug2 := createTestSuggestion(suggestion.SuggestionTypeAddEligibility, "tool1", 0.8)
	sug3 := createTestSuggestion(suggestion.SuggestionTypeAddEligibility, "tool2", 0.7)

	result := deduplicateSuggestions([]suggestion.Suggestion{sug1, sug2, sug3})

	if len(result) != 2 {
		t.Errorf("expected 2 unique suggestions, got %d", len(result))
	}
}

func TestDeduplicateSuggestions_EmptyList(t *testing.T) {
	result := deduplicateSuggestions([]suggestion.Suggestion{})

	if len(result) != 0 {
		t.Errorf("expected 0 suggestions, got %d", len(result))
	}
}

func TestDeduplicateSuggestions_NoDuplicates(t *testing.T) {
	sug1 := createTestSuggestion(suggestion.SuggestionTypeAddEligibility, "tool1", 0.9)
	sug2 := createTestSuggestion(suggestion.SuggestionTypeRemoveEligibility, "tool2", 0.8)
	sug3 := createTestSuggestion(suggestion.SuggestionTypeIncreaseBudget, "budget1", 0.7)

	result := deduplicateSuggestions([]suggestion.Suggestion{sug1, sug2, sug3})

	if len(result) != 3 {
		t.Errorf("expected 3 suggestions, got %d", len(result))
	}
}

// ===============================
// Test Helpers
// ===============================

// mockGenerator is a test implementation of suggestion.Generator
type mockGenerator struct {
	types       []suggestion.SuggestionType
	suggestions []suggestion.Suggestion
	err         error
}

func (m *mockGenerator) Generate(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.suggestions, nil
}

func (m *mockGenerator) Types() []suggestion.SuggestionType {
	return m.types
}

var _ suggestion.Generator = (*mockGenerator)(nil)

func createTestSuggestion(sugType suggestion.SuggestionType, target string, confidence float64) suggestion.Suggestion {
	s := suggestion.NewSuggestion(sugType, "Test "+target, "Test description")
	s.Confidence = confidence
	s.Impact = suggestion.ImpactLevelLow
	s.Change = suggestion.PolicyChange{
		Type:   suggestion.PolicyChangeTypeEligibility,
		Target: target,
		To:     "explore",
	}
	return *s
}

func createTestSuggestionWithImpact(title string, confidence float64, impact suggestion.ImpactLevel) suggestion.Suggestion {
	s := suggestion.NewSuggestion(suggestion.SuggestionTypeAddEligibility, title, "Test description")
	s.Confidence = confidence
	s.Impact = impact
	s.Change = suggestion.PolicyChange{
		Type:   suggestion.PolicyChangeTypeEligibility,
		Target: title,
		To:     "explore",
	}
	return *s
}
