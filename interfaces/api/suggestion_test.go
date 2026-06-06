package api_test

import (
	"testing"

	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewSuggestionStore(t *testing.T) {
	t.Parallel()

	t.Run("creates in-memory suggestion store", func(t *testing.T) {
		t.Parallel()

		store := api.NewSuggestionStore()

		if store == nil {
			t.Fatal("NewSuggestionStore() returned nil")
		}
	})
}

func TestNewEligibilityGenerator(t *testing.T) {
	t.Parallel()

	t.Run("creates eligibility generator", func(t *testing.T) {
		t.Parallel()

		generator := api.NewEligibilityGenerator()

		if generator == nil {
			t.Fatal("NewEligibilityGenerator() returned nil")
		}
	})
}

func TestNewBudgetGenerator(t *testing.T) {
	t.Parallel()

	t.Run("creates budget generator", func(t *testing.T) {
		t.Parallel()

		generator := api.NewBudgetGenerator()

		if generator == nil {
			t.Fatal("NewBudgetGenerator() returned nil")
		}
	})
}

func TestNewCompositeSuggestionGenerator(t *testing.T) {
	t.Parallel()

	t.Run("creates composite generator with multiple generators", func(t *testing.T) {
		t.Parallel()

		eligibility := api.NewEligibilityGenerator()
		budget := api.NewBudgetGenerator()

		composite := api.NewCompositeSuggestionGenerator(eligibility, budget)

		if composite == nil {
			t.Fatal("NewCompositeSuggestionGenerator() returned nil")
		}
	})

	t.Run("creates composite generator with single generator", func(t *testing.T) {
		t.Parallel()

		eligibility := api.NewEligibilityGenerator()

		composite := api.NewCompositeSuggestionGenerator(eligibility)

		if composite == nil {
			t.Fatal("NewCompositeSuggestionGenerator() with single generator returned nil")
		}
	})

	t.Run("creates empty composite generator", func(t *testing.T) {
		t.Parallel()

		composite := api.NewCompositeSuggestionGenerator()

		if composite == nil {
			t.Fatal("NewCompositeSuggestionGenerator() with no generators returned nil")
		}
	})
}

func TestSuggestionTypeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		st       api.SuggestionType
		expected string
	}{
		{"SuggestionTypeAddEligibility", api.SuggestionTypeAddEligibility, "add_eligibility"},
		{"SuggestionTypeRemoveEligibility", api.SuggestionTypeRemoveEligibility, "remove_eligibility"},
		{"SuggestionTypeAddTransition", api.SuggestionTypeAddTransition, "add_transition"},
		{"SuggestionTypeRemoveTransition", api.SuggestionTypeRemoveTransition, "remove_transition"},
		{"SuggestionTypeIncreaseBudget", api.SuggestionTypeIncreaseBudget, "increase_budget"},
		{"SuggestionTypeDecreaseBudget", api.SuggestionTypeDecreaseBudget, "decrease_budget"},
		{"SuggestionTypeRequireApproval", api.SuggestionTypeRequireApproval, "require_approval"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.st) != tt.expected {
				t.Errorf("SuggestionType = %s, want %s", tt.st, tt.expected)
			}
		})
	}
}

func TestSuggestionStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   api.SuggestionStatus
		expected string
	}{
		{"SuggestionStatusPending", api.SuggestionStatusPending, "pending"},
		{"SuggestionStatusAccepted", api.SuggestionStatusAccepted, "accepted"},
		{"SuggestionStatusRejected", api.SuggestionStatusRejected, "rejected"},
		{"SuggestionStatusSuperseded", api.SuggestionStatusSuperseded, "superseded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.status) != tt.expected {
				t.Errorf("SuggestionStatus = %s, want %s", tt.status, tt.expected)
			}
		})
	}
}

func TestImpactLevelConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		level    api.ImpactLevel
		expected string
	}{
		{"ImpactLevelLow", api.ImpactLevelLow, "low"},
		{"ImpactLevelMedium", api.ImpactLevelMedium, "medium"},
		{"ImpactLevelHigh", api.ImpactLevelHigh, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.level) != tt.expected {
				t.Errorf("ImpactLevel = %s, want %s", tt.level, tt.expected)
			}
		})
	}
}
