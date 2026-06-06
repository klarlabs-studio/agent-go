// Package suggestion provides suggestion types for policy evolution.
package suggestion

import (
	"context"

	"go.klarlabs.de/agent/domain/pattern"
)

// Generator generates suggestions from detected patterns.
type Generator interface {
	// Generate creates suggestions based on detected patterns.
	Generate(ctx context.Context, patterns []pattern.Pattern) ([]Suggestion, error)

	// Types returns the suggestion types this generator can create.
	Types() []SuggestionType
}

// GenerationOptions configures suggestion generation.
type GenerationOptions struct {
	// MinConfidence is the minimum pattern confidence to consider.
	MinConfidence float64

	// MinFrequency is the minimum pattern frequency to consider.
	MinFrequency int

	// PatternTypes filters to specific pattern types.
	PatternTypes []pattern.PatternType

	// MaxSuggestions limits the number of suggestions generated.
	MaxSuggestions int

	// IncludeHighImpact includes high-impact suggestions.
	IncludeHighImpact bool
}

// DefaultGenerationOptions returns sensible defaults.
func DefaultGenerationOptions() GenerationOptions {
	return GenerationOptions{
		MinConfidence:     0.6,
		MinFrequency:      3,
		MaxSuggestions:    50,
		IncludeHighImpact: false, // Require explicit opt-in for high-impact
	}
}

// GeneratorFunc is a function that implements Generator.
type GeneratorFunc func(ctx context.Context, patterns []pattern.Pattern) ([]Suggestion, error)

// Generate implements Generator.
func (f GeneratorFunc) Generate(ctx context.Context, patterns []pattern.Pattern) ([]Suggestion, error) {
	return f(ctx, patterns)
}

// Types implements Generator (returns empty slice).
func (f GeneratorFunc) Types() []SuggestionType {
	return nil
}
