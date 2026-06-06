// Package proposal provides proposal infrastructure implementations.
package proposal

import (
	"context"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/proposal"
)

// PolicyApplier applies proposal changes to policy snapshots.
type PolicyApplier struct{}

// NewPolicyApplier creates a new policy applier.
func NewPolicyApplier() *PolicyApplier {
	return &PolicyApplier{}
}

// Apply applies changes to a policy version and returns a new version.
func (a *PolicyApplier) Apply(ctx context.Context, current *policy.PolicyVersion, changes []proposal.PolicyChange) (*policy.PolicyVersion, error) {
	// Create new version based on current
	newVersion := &policy.PolicyVersion{
		Version:     current.Version + 1,
		Eligibility: copyEligibilitySnapshot(current.Eligibility),
		Transitions: copyTransitionSnapshot(current.Transitions),
		Budgets:     copyBudgetSnapshot(current.Budgets),
		Approvals:   copyApprovalSnapshot(current.Approvals),
	}

	// Apply each change
	for _, change := range changes {
		if err := a.applyChange(newVersion, change); err != nil {
			return nil, fmt.Errorf("failed to apply change: %w", err)
		}
	}

	return newVersion, nil
}

func (a *PolicyApplier) applyChange(version *policy.PolicyVersion, change proposal.PolicyChange) error {
	switch change.Type {
	case proposal.ChangeTypeEligibility:
		return a.applyEligibilityChange(version, change)
	case proposal.ChangeTypeTransition:
		return a.applyTransitionChange(version, change)
	case proposal.ChangeTypeBudget:
		return a.applyBudgetChange(version, change)
	case proposal.ChangeTypeApproval:
		return a.applyApprovalChange(version, change)
	default:
		return fmt.Errorf("unknown change type: %s", change.Type)
	}
}

func (a *PolicyApplier) applyEligibilityChange(version *policy.PolicyVersion, change proposal.PolicyChange) error {
	var ec proposal.EligibilityChange
	if err := change.GetAfter(&ec); err != nil {
		return fmt.Errorf("failed to unmarshal eligibility change: %w", err)
	}

	if ec.Allowed {
		version.Eligibility.AddTool(ec.State, ec.ToolName)
	} else {
		version.Eligibility.RemoveTool(ec.State, ec.ToolName)
	}

	return nil
}

func (a *PolicyApplier) applyTransitionChange(version *policy.PolicyVersion, change proposal.PolicyChange) error {
	var tc proposal.TransitionChange
	if err := change.GetAfter(&tc); err != nil {
		return fmt.Errorf("failed to unmarshal transition change: %w", err)
	}

	if tc.Allowed {
		version.Transitions.AddTransition(tc.FromState, tc.ToState)
	} else {
		version.Transitions.RemoveTransition(tc.FromState, tc.ToState)
	}

	return nil
}

func (a *PolicyApplier) applyBudgetChange(version *policy.PolicyVersion, change proposal.PolicyChange) error {
	var bc proposal.BudgetChange
	if err := change.GetAfter(&bc); err != nil {
		return fmt.Errorf("failed to unmarshal budget change: %w", err)
	}

	version.Budgets.SetLimit(bc.BudgetName, bc.NewValue)
	return nil
}

func (a *PolicyApplier) applyApprovalChange(version *policy.PolicyVersion, change proposal.PolicyChange) error {
	var ac proposal.ApprovalChange
	if err := change.GetAfter(&ac); err != nil {
		return fmt.Errorf("failed to unmarshal approval change: %w", err)
	}

	if ac.Required {
		version.Approvals.RequireApproval(ac.ToolName)
	} else {
		version.Approvals.RemoveApproval(ac.ToolName)
	}

	return nil
}

// Helper functions to copy snapshots

func copyEligibilitySnapshot(src policy.EligibilitySnapshot) policy.EligibilitySnapshot {
	dst := policy.NewEligibilitySnapshot()
	for state, tools := range src.StateTools {
		for _, tool := range tools {
			dst.AddTool(state, tool)
		}
	}
	return dst
}

func copyTransitionSnapshot(src policy.TransitionSnapshot) policy.TransitionSnapshot {
	dst := policy.NewTransitionSnapshot()
	for from, tos := range src.Transitions {
		for _, to := range tos {
			dst.AddTransition(from, to)
		}
	}
	return dst
}

func copyBudgetSnapshot(src policy.BudgetLimitsSnapshot) policy.BudgetLimitsSnapshot {
	dst := policy.NewBudgetLimitsSnapshot()
	for name, limit := range src.Limits {
		dst.SetLimit(name, limit)
	}
	return dst
}

func copyApprovalSnapshot(src policy.ApprovalSnapshot) policy.ApprovalSnapshot {
	dst := policy.NewApprovalSnapshot()
	for _, tool := range src.RequiredTools {
		dst.RequireApproval(tool)
	}
	return dst
}

// CreateEligibilityChange creates an eligibility policy change.
func CreateEligibilityChange(state agent.State, toolName string, allowed bool, description string) (*proposal.PolicyChange, error) {
	before := proposal.EligibilityChange{
		State:    state,
		ToolName: toolName,
		Allowed:  !allowed,
	}
	after := proposal.EligibilityChange{
		State:    state,
		ToolName: toolName,
		Allowed:  allowed,
	}
	return proposal.NewPolicyChange(proposal.ChangeTypeEligibility, toolName, description, before, after)
}

// CreateTransitionChange creates a transition policy change.
func CreateTransitionChange(from, to agent.State, allowed bool, description string) (*proposal.PolicyChange, error) {
	before := proposal.TransitionChange{
		FromState: from,
		ToState:   to,
		Allowed:   !allowed,
	}
	after := proposal.TransitionChange{
		FromState: from,
		ToState:   to,
		Allowed:   allowed,
	}
	target := fmt.Sprintf("%s->%s", from, to)
	return proposal.NewPolicyChange(proposal.ChangeTypeTransition, target, description, before, after)
}

// CreateBudgetChange creates a budget policy change.
func CreateBudgetChange(budgetName string, oldValue, newValue int, description string) (*proposal.PolicyChange, error) {
	before := proposal.BudgetChange{
		BudgetName: budgetName,
		OldValue:   oldValue,
		NewValue:   oldValue,
	}
	after := proposal.BudgetChange{
		BudgetName: budgetName,
		OldValue:   oldValue,
		NewValue:   newValue,
	}
	return proposal.NewPolicyChange(proposal.ChangeTypeBudget, budgetName, description, before, after)
}

// CreateApprovalChange creates an approval requirement policy change.
func CreateApprovalChange(toolName string, required bool, description string) (*proposal.PolicyChange, error) {
	before := proposal.ApprovalChange{
		ToolName: toolName,
		Required: !required,
	}
	after := proposal.ApprovalChange{
		ToolName: toolName,
		Required: required,
	}
	return proposal.NewPolicyChange(proposal.ChangeTypeApproval, toolName, description, before, after)
}
