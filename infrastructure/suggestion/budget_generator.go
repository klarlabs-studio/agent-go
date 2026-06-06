// Package suggestion provides suggestion generation implementations.
package suggestion

import (
	"context"
	"fmt"

	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/suggestion"
)

// BudgetGenerator generates budget-related suggestions.
type BudgetGenerator struct {
	minConfidence      float64
	budgetIncreaseRate float64 // Suggested increase as a percentage
	budgetDecreaseRate float64 // Suggested decrease as a percentage
}

// BudgetGeneratorOption configures the budget generator.
type BudgetGeneratorOption func(*BudgetGenerator)

// WithBudgetMinConfidence sets the minimum confidence threshold.
func WithBudgetMinConfidence(c float64) BudgetGeneratorOption {
	return func(g *BudgetGenerator) {
		g.minConfidence = c
	}
}

// WithBudgetIncreaseRate sets the suggested increase rate.
func WithBudgetIncreaseRate(rate float64) BudgetGeneratorOption {
	return func(g *BudgetGenerator) {
		g.budgetIncreaseRate = rate
	}
}

// WithBudgetDecreaseRate sets the suggested decrease rate.
func WithBudgetDecreaseRate(rate float64) BudgetGeneratorOption {
	return func(g *BudgetGenerator) {
		g.budgetDecreaseRate = rate
	}
}

// NewBudgetGenerator creates a new budget suggestion generator.
func NewBudgetGenerator(opts ...BudgetGeneratorOption) *BudgetGenerator {
	g := &BudgetGenerator{
		minConfidence:      0.6,
		budgetIncreaseRate: 0.25, // 25% increase
		budgetDecreaseRate: 0.10, // 10% decrease
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Generate creates budget suggestions from patterns.
func (g *BudgetGenerator) Generate(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error) {
	var suggestions []suggestion.Suggestion

	for _, p := range patterns {
		if p.Confidence < g.minConfidence {
			continue
		}

		switch p.Type {
		case pattern.PatternTypeBudgetExhaustion:
			sugs := g.suggestFromBudgetExhaustion(p)
			suggestions = append(suggestions, sugs...)

		case pattern.PatternTypeLongRuns:
			sugs := g.suggestFromLongRuns(p)
			suggestions = append(suggestions, sugs...)
		}
	}

	return suggestions, nil
}

// Types returns the suggestion types this generator can create.
func (g *BudgetGenerator) Types() []suggestion.SuggestionType {
	return []suggestion.SuggestionType{
		suggestion.SuggestionTypeIncreaseBudget,
		suggestion.SuggestionTypeDecreaseBudget,
	}
}

func (g *BudgetGenerator) suggestFromBudgetExhaustion(p pattern.Pattern) []suggestion.Suggestion {
	var sugs []suggestion.Suggestion

	var data pattern.BudgetExhaustionData
	if err := p.GetData(&data); err != nil {
		return sugs
	}

	// Suggest increasing budget if exhaustions are frequent
	if data.ExhaustionCount >= 3 {
		s := suggestion.NewSuggestion(
			suggestion.SuggestionTypeIncreaseBudget,
			"Increase budget limit",
			fmt.Sprintf("Budget exhaustion detected %d times. Consider increasing the budget by %.0f%% to allow runs to complete.",
				data.ExhaustionCount, g.budgetIncreaseRate*100),
		)
		s.Rationale = fmt.Sprintf("Frequent budget exhaustions (%d occurrences) indicate the current limit may be too restrictive", data.ExhaustionCount)
		s.Confidence = p.Confidence
		s.Impact = suggestion.ImpactLevelMedium
		s.AddPatternID(p.ID)
		s.Change = suggestion.PolicyChange{
			Type:   suggestion.PolicyChangeTypeBudget,
			Target: "tool_calls", // Default budget name
			To:     fmt.Sprintf("increase by %.0f%%", g.budgetIncreaseRate*100),
		}
		sugs = append(sugs, *s)
	}

	return sugs
}

func (g *BudgetGenerator) suggestFromLongRuns(p pattern.Pattern) []suggestion.Suggestion {
	var sugs []suggestion.Suggestion

	var data pattern.LongRunsData
	if err := p.GetData(&data); err != nil {
		return sugs
	}

	// If runs consistently exceed threshold but complete successfully,
	// the threshold might be too conservative
	if data.LongRunCount >= 5 {
		// Suggest adjusting timeout budget
		s := suggestion.NewSuggestion(
			suggestion.SuggestionTypeIncreaseBudget,
			"Adjust timeout budget",
			fmt.Sprintf("Runs frequently exceed the %v threshold (%d times). Consider increasing the timeout budget to accommodate longer operations.",
				data.Threshold, data.LongRunCount),
		)
		s.Rationale = fmt.Sprintf("Average run duration (%v) consistently exceeds threshold", data.AverageDuration)
		s.Confidence = p.Confidence * 0.9
		s.Impact = suggestion.ImpactLevelLow
		s.AddPatternID(p.ID)
		s.Change = suggestion.PolicyChange{
			Type:   suggestion.PolicyChangeTypeBudget,
			Target: "timeout",
			From:   data.Threshold.String(),
			To:     fmt.Sprintf("increase to ~%v", data.AverageDuration),
		}
		sugs = append(sugs, *s)
	}

	return sugs
}

// Ensure BudgetGenerator implements Generator
var _ suggestion.Generator = (*BudgetGenerator)(nil)
