// Package api provides the public API for the agent runtime.
package api

import (
	"go.klarlabs.de/agent/domain/suggestion"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
	infraSuggestion "go.klarlabs.de/agent/infrastructure/suggestion"
)

// Re-export suggestion types for convenience.
type (
	// Suggestion represents a policy improvement suggestion.
	Suggestion = suggestion.Suggestion

	// SuggestionType categorizes suggestions.
	SuggestionType = suggestion.SuggestionType

	// SuggestionStatus tracks suggestion lifecycle.
	SuggestionStatus = suggestion.SuggestionStatus

	// ImpactLevel indicates the potential impact of a suggestion.
	ImpactLevel = suggestion.ImpactLevel

	// SuggestionChange describes a proposed policy change.
	SuggestionChange = suggestion.PolicyChange

	// SuggestionChangeType categorizes the type of change.
	SuggestionChangeType = suggestion.PolicyChangeType

	// SuggestionGenerator generates suggestions from patterns.
	SuggestionGenerator = suggestion.Generator

	// SuggestionStore stores suggestions.
	SuggestionStore = suggestion.Store

	// SuggestionListFilter filters suggestion queries.
	SuggestionListFilter = suggestion.ListFilter
)

// Re-export suggestion type constants.
const (
	SuggestionTypeAddEligibility    = suggestion.SuggestionTypeAddEligibility
	SuggestionTypeRemoveEligibility = suggestion.SuggestionTypeRemoveEligibility
	SuggestionTypeAddTransition     = suggestion.SuggestionTypeAddTransition
	SuggestionTypeRemoveTransition  = suggestion.SuggestionTypeRemoveTransition
	SuggestionTypeIncreaseBudget    = suggestion.SuggestionTypeIncreaseBudget
	SuggestionTypeDecreaseBudget    = suggestion.SuggestionTypeDecreaseBudget
	SuggestionTypeRequireApproval   = suggestion.SuggestionTypeRequireApproval
)

// Re-export suggestion status constants.
const (
	SuggestionStatusPending    = suggestion.SuggestionStatusPending
	SuggestionStatusAccepted   = suggestion.SuggestionStatusAccepted
	SuggestionStatusRejected   = suggestion.SuggestionStatusRejected
	SuggestionStatusSuperseded = suggestion.SuggestionStatusSuperseded
)

// Re-export impact level constants.
const (
	ImpactLevelLow    = suggestion.ImpactLevelLow
	ImpactLevelMedium = suggestion.ImpactLevelMedium
	ImpactLevelHigh   = suggestion.ImpactLevelHigh
)

// NewSuggestionStore creates a new in-memory suggestion store.
func NewSuggestionStore() suggestion.Store {
	return memory.NewSuggestionStore()
}

// NewEligibilityGenerator creates a generator for eligibility suggestions.
func NewEligibilityGenerator() suggestion.Generator {
	return infraSuggestion.NewEligibilityGenerator()
}

// NewBudgetGenerator creates a generator for budget suggestions.
func NewBudgetGenerator() suggestion.Generator {
	return infraSuggestion.NewBudgetGenerator()
}

// NewCompositeSuggestionGenerator creates a generator that combines multiple generators.
func NewCompositeSuggestionGenerator(generators ...suggestion.Generator) suggestion.Generator {
	return infraSuggestion.NewCompositeGenerator(generators...)
}
