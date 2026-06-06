// Package suggestion provides suggestion generation implementations.
package suggestion

import (
	"context"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/suggestion"
)

// EligibilityGenerator generates eligibility-related suggestions.
type EligibilityGenerator struct {
	minConfidence float64
}

// EligibilityGeneratorOption configures the eligibility generator.
type EligibilityGeneratorOption func(*EligibilityGenerator)

// WithEligibilityMinConfidence sets the minimum confidence threshold.
func WithEligibilityMinConfidence(c float64) EligibilityGeneratorOption {
	return func(g *EligibilityGenerator) {
		g.minConfidence = c
	}
}

// NewEligibilityGenerator creates a new eligibility suggestion generator.
func NewEligibilityGenerator(opts ...EligibilityGeneratorOption) *EligibilityGenerator {
	g := &EligibilityGenerator{
		minConfidence: 0.6,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Generate creates eligibility suggestions from patterns.
func (g *EligibilityGenerator) Generate(ctx context.Context, patterns []pattern.Pattern) ([]suggestion.Suggestion, error) {
	var suggestions []suggestion.Suggestion

	for _, p := range patterns {
		if p.Confidence < g.minConfidence {
			continue
		}

		switch p.Type {
		case pattern.PatternTypeToolSequence:
			sugs := g.suggestFromToolSequence(p)
			suggestions = append(suggestions, sugs...)

		case pattern.PatternTypeToolAffinity:
			sugs := g.suggestFromToolAffinity(p)
			suggestions = append(suggestions, sugs...)

		case pattern.PatternTypeToolFailure:
			sugs := g.suggestFromToolFailure(p)
			suggestions = append(suggestions, sugs...)
		}
	}

	return suggestions, nil
}

// Types returns the suggestion types this generator can create.
func (g *EligibilityGenerator) Types() []suggestion.SuggestionType {
	return []suggestion.SuggestionType{
		suggestion.SuggestionTypeAddEligibility,
		suggestion.SuggestionTypeRemoveEligibility,
	}
}

func (g *EligibilityGenerator) suggestFromToolSequence(p pattern.Pattern) []suggestion.Suggestion {
	var sugs []suggestion.Suggestion

	var data pattern.ToolSequenceData
	if err := p.GetData(&data); err != nil {
		return sugs
	}

	// If tools are frequently used in sequence, suggest adding them to common states
	if len(data.Sequence) >= 2 && p.Frequency >= 5 {
		for _, state := range data.States {
			for _, toolName := range data.Sequence {
				s := suggestion.NewSuggestion(
					suggestion.SuggestionTypeAddEligibility,
					fmt.Sprintf("Add %s to %s state", toolName, state),
					fmt.Sprintf("Tool '%s' is frequently used in sequences in the %s state. Adding explicit eligibility may improve agent behavior.",
						toolName, state),
				)
				s.Rationale = fmt.Sprintf("Pattern detected %d times with %.0f%% confidence", p.Frequency, p.Confidence*100)
				s.Confidence = p.Confidence * 0.8 // Slightly lower confidence for derived suggestions
				s.Impact = suggestion.ImpactLevelLow
				s.AddPatternID(p.ID)
				s.Change = suggestion.PolicyChange{
					Type:   suggestion.PolicyChangeTypeEligibility,
					Target: toolName,
					To:     state,
				}
				changeData := suggestion.EligibilityChangeData{
					State:    state,
					ToolName: toolName,
					Add:      true,
				}
				_ = s.SetChangeData(changeData)
				sugs = append(sugs, *s)
			}
		}
	}

	return sugs
}

func (g *EligibilityGenerator) suggestFromToolAffinity(p pattern.Pattern) []suggestion.Suggestion {
	var sugs []suggestion.Suggestion

	var data pattern.ToolAffinityData
	if err := p.GetData(&data); err != nil {
		return sugs
	}

	// High affinity tools should be available in the same states
	if data.Correlation >= 0.7 && len(data.Tools) >= 2 {
		s := suggestion.NewSuggestion(
			suggestion.SuggestionTypeAddEligibility,
			fmt.Sprintf("Co-locate tools: %v", data.Tools),
			fmt.Sprintf("Tools %v are frequently used together (%.0f%% correlation). Consider making them available in the same states.",
				data.Tools, data.Correlation*100),
		)
		s.Rationale = fmt.Sprintf("High correlation (%.0f%%) suggests these tools work together", data.Correlation*100)
		s.Confidence = p.Confidence * data.Correlation
		s.Impact = suggestion.ImpactLevelMedium
		s.AddPatternID(p.ID)
		sugs = append(sugs, *s)
	}

	return sugs
}

func (g *EligibilityGenerator) suggestFromToolFailure(p pattern.Pattern) []suggestion.Suggestion {
	var sugs []suggestion.Suggestion

	var data pattern.ToolFailureData
	if err := p.GetData(&data); err != nil {
		return sugs
	}

	// If a tool consistently fails, suggest removing its eligibility
	if data.ErrorCount >= 5 && p.Confidence >= 0.7 {
		s := suggestion.NewSuggestion(
			suggestion.SuggestionTypeRemoveEligibility,
			fmt.Sprintf("Remove failing tool: %s", data.ToolName),
			fmt.Sprintf("Tool '%s' has failed %d times with '%s' errors. Consider removing its eligibility until the issue is resolved.",
				data.ToolName, data.ErrorCount, data.ErrorType),
		)
		s.Rationale = fmt.Sprintf("Consistent failures (%d occurrences) suggest the tool is unreliable in its current configuration", data.ErrorCount)
		s.Confidence = p.Confidence
		s.Impact = suggestion.ImpactLevelHigh
		s.AddPatternID(p.ID)
		s.Change = suggestion.PolicyChange{
			Type:   suggestion.PolicyChangeTypeEligibility,
			Target: data.ToolName,
			From:   "eligible",
			To:     "ineligible",
		}
		changeData := suggestion.EligibilityChangeData{
			State:    agent.StateAct, // Default to act state
			ToolName: data.ToolName,
			Add:      false,
		}
		_ = s.SetChangeData(changeData)
		sugs = append(sugs, *s)
	}

	return sugs
}

// Ensure EligibilityGenerator implements Generator
var _ suggestion.Generator = (*EligibilityGenerator)(nil)
