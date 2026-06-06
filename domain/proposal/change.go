// Package proposal provides proposal types for policy evolution.
package proposal

import (
	"encoding/json"

	"go.klarlabs.de/agent/domain/agent"
)

// ChangeType classifies policy changes.
type ChangeType string

const (
	ChangeTypeEligibility ChangeType = "eligibility"
	ChangeTypeTransition  ChangeType = "transition"
	ChangeTypeBudget      ChangeType = "budget"
	ChangeTypeApproval    ChangeType = "approval"
)

// PolicyChange represents a single change to policy.
type PolicyChange struct {
	// Type identifies the kind of change.
	Type ChangeType `json:"type"`

	// Target is what the change affects (tool name, state, budget name).
	Target string `json:"target"`

	// Description explains the change.
	Description string `json:"description"`

	// Before contains the state before the change (for rollback).
	Before json.RawMessage `json:"before,omitempty"`

	// After contains the state after the change.
	After json.RawMessage `json:"after"`
}

// NewPolicyChange creates a new policy change.
func NewPolicyChange(changeType ChangeType, target, description string, before, after any) (*PolicyChange, error) {
	change := &PolicyChange{
		Type:        changeType,
		Target:      target,
		Description: description,
	}

	if before != nil {
		beforeJSON, err := json.Marshal(before)
		if err != nil {
			return nil, err
		}
		change.Before = beforeJSON
	}

	afterJSON, err := json.Marshal(after)
	if err != nil {
		return nil, err
	}
	change.After = afterJSON

	return change, nil
}

// GetBefore unmarshals the before state.
func (c *PolicyChange) GetBefore(v any) error {
	if c.Before == nil {
		return nil
	}
	return json.Unmarshal(c.Before, v)
}

// GetAfter unmarshals the after state.
func (c *PolicyChange) GetAfter(v any) error {
	if c.After == nil {
		return nil
	}
	return json.Unmarshal(c.After, v)
}

// EligibilityChange represents a tool eligibility change.
type EligibilityChange struct {
	State    agent.State `json:"state"`
	ToolName string      `json:"tool_name"`
	Allowed  bool        `json:"allowed"`
}

// TransitionChange represents a state transition change.
type TransitionChange struct {
	FromState agent.State `json:"from_state"`
	ToState   agent.State `json:"to_state"`
	Allowed   bool        `json:"allowed"`
}

// BudgetChange represents a budget limit change.
type BudgetChange struct {
	BudgetName string `json:"budget_name"`
	OldValue   int    `json:"old_value"`
	NewValue   int    `json:"new_value"`
}

// ApprovalChange represents an approval requirement change.
type ApprovalChange struct {
	ToolName string `json:"tool_name"`
	Required bool   `json:"required"`
}
