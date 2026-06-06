// Package suggestion provides suggestion types for policy evolution.
package suggestion

import (
	"go.klarlabs.de/agent/domain/agent"
)

// SuggestionType classifies suggestions.
type SuggestionType string

const (
	// Eligibility suggestions
	SuggestionTypeAddEligibility    SuggestionType = "add_eligibility"    // Add tool to state
	SuggestionTypeRemoveEligibility SuggestionType = "remove_eligibility" // Remove tool from state

	// Transition suggestions
	SuggestionTypeAddTransition    SuggestionType = "add_transition"    // Add state transition
	SuggestionTypeRemoveTransition SuggestionType = "remove_transition" // Remove state transition

	// Budget suggestions
	SuggestionTypeIncreaseBudget SuggestionType = "increase_budget" // Increase a budget limit
	SuggestionTypeDecreaseBudget SuggestionType = "decrease_budget" // Decrease a budget limit

	// Approval suggestions
	SuggestionTypeRequireApproval SuggestionType = "require_approval" // Add approval requirement
	SuggestionTypeRemoveApproval  SuggestionType = "remove_approval"  // Remove approval requirement
)

// SuggestionStatus tracks the lifecycle of a suggestion.
type SuggestionStatus string

const (
	SuggestionStatusPending    SuggestionStatus = "pending"    // Awaiting review
	SuggestionStatusAccepted   SuggestionStatus = "accepted"   // Converted to proposal
	SuggestionStatusRejected   SuggestionStatus = "rejected"   // Dismissed
	SuggestionStatusSuperseded SuggestionStatus = "superseded" // Replaced by newer suggestion
)

// ImpactLevel indicates the potential impact of a suggestion.
type ImpactLevel string

const (
	ImpactLevelLow    ImpactLevel = "low"    // Minimal risk, easy to reverse
	ImpactLevelMedium ImpactLevel = "medium" // Moderate risk
	ImpactLevelHigh   ImpactLevel = "high"   // Significant risk, requires careful review
)

// PolicyChangeType classifies policy changes.
type PolicyChangeType string

const (
	PolicyChangeTypeEligibility PolicyChangeType = "eligibility"
	PolicyChangeTypeTransition  PolicyChangeType = "transition"
	PolicyChangeTypeBudget      PolicyChangeType = "budget"
	PolicyChangeTypeApproval    PolicyChangeType = "approval"
)

// PolicyChange represents a proposed change to policy.
type PolicyChange struct {
	Type   PolicyChangeType `json:"type"`
	Target string           `json:"target"` // Tool name, state, or budget name
	From   any              `json:"from,omitempty"`
	To     any              `json:"to"`
}

// EligibilityChangeData captures eligibility change details.
type EligibilityChangeData struct {
	State    agent.State `json:"state"`
	ToolName string      `json:"tool_name"`
	Add      bool        `json:"add"` // true = add, false = remove
}

// TransitionChangeData captures transition change details.
type TransitionChangeData struct {
	FromState agent.State `json:"from_state"`
	ToState   agent.State `json:"to_state"`
	Add       bool        `json:"add"` // true = add, false = remove
}

// BudgetChangeData captures budget change details.
type BudgetChangeData struct {
	BudgetName string `json:"budget_name"`
	OldValue   int    `json:"old_value"`
	NewValue   int    `json:"new_value"`
}

// ApprovalChangeData captures approval requirement change details.
type ApprovalChangeData struct {
	ToolName string `json:"tool_name"`
	Add      bool   `json:"add"` // true = require approval, false = remove requirement
}
