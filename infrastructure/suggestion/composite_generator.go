// Package suggestion provides suggestion generation implementations.
package suggestion

import (
	"context"
	"fmt"
	"sort"

	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/suggestion"
)

// CompositeGenerator combines multiple suggestion generators.
type CompositeGenerator struct {
	generators []suggestion.Generator
}

// NewCompositeGenerator creates a generator that combines multiple generators.
func NewCompositeGenerator(generators ...suggestion.Generator) *CompositeGenerator {
	return &CompositeGenerator{
		generators: generators,
	}
}

// AddGenerator adds a generator to the composite.
func (c *CompositeGenerator) AddGenerator(generator suggestion.Generator) {
	c.generators = append(c.generators, generator)
}

// Generate runs all generators and combines their results.
func (c *CompositeGenerator) Generate(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error) {
	var allSuggestions []suggestion.Suggestion
	var errors []error

	// Run each generator
	for _, generator := range c.generators {
		suggestions, err := generator.Generate(ctx, patterns)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		allSuggestions = append(allSuggestions, suggestions...)
	}

	// If all generators failed, return an error
	if len(errors) == len(c.generators) && len(errors) > 0 {
		return nil, fmt.Errorf("all generators failed: %v", errors)
	}

	// Deduplicate similar suggestions
	allSuggestions = deduplicateSuggestions(allSuggestions)

	// Sort by confidence (descending) then impact level
	sort.Slice(allSuggestions, func(i, j int) bool {
		if allSuggestions[i].Confidence != allSuggestions[j].Confidence {
			return allSuggestions[i].Confidence > allSuggestions[j].Confidence
		}
		return impactWeight(allSuggestions[i].Impact) > impactWeight(allSuggestions[j].Impact)
	})

	return allSuggestions, nil
}

// Types returns all suggestion types from all generators.
func (c *CompositeGenerator) Types() []suggestion.SuggestionType {
	typeSet := make(map[suggestion.SuggestionType]bool)
	for _, generator := range c.generators {
		for _, t := range generator.Types() {
			typeSet[t] = true
		}
	}

	types := make([]suggestion.SuggestionType, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

func deduplicateSuggestions(suggestions []suggestion.Suggestion) []suggestion.Suggestion {
	seen := make(map[string]bool)
	var unique []suggestion.Suggestion

	for _, s := range suggestions {
		// Create a key based on type and target
		key := fmt.Sprintf("%s:%s:%v", s.Type, s.Change.Target, s.Change.To)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, s)
	}

	return unique
}

func impactWeight(impact suggestion.ImpactLevel) int {
	switch impact {
	case suggestion.ImpactLevelHigh:
		return 3
	case suggestion.ImpactLevelMedium:
		return 2
	case suggestion.ImpactLevelLow:
		return 1
	default:
		return 0
	}
}

// Ensure CompositeGenerator implements Generator
var _ suggestion.Generator = (*CompositeGenerator)(nil)
