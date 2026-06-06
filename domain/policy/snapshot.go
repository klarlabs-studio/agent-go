// Package policy provides policy constraint types.
package policy

import (
	"go.klarlabs.de/agent/domain/agent"
)

// EligibilitySnapshot captures tool eligibility at a point in time.
type EligibilitySnapshot struct {
	// StateTools maps states to allowed tool names.
	StateTools map[agent.State][]string `json:"state_tools"`
}

// NewEligibilitySnapshot creates a new eligibility snapshot.
func NewEligibilitySnapshot() EligibilitySnapshot {
	return EligibilitySnapshot{
		StateTools: make(map[agent.State][]string),
	}
}

// AddTool adds a tool to a state's eligibility.
func (s *EligibilitySnapshot) AddTool(state agent.State, toolName string) {
	if s.StateTools == nil {
		s.StateTools = make(map[agent.State][]string)
	}
	tools := s.StateTools[state]
	for _, t := range tools {
		if t == toolName {
			return // Already exists
		}
	}
	s.StateTools[state] = append(tools, toolName)
}

// RemoveTool removes a tool from a state's eligibility.
func (s *EligibilitySnapshot) RemoveTool(state agent.State, toolName string) {
	if s.StateTools == nil {
		return
	}
	tools := s.StateTools[state]
	for i, t := range tools {
		if t == toolName {
			s.StateTools[state] = append(tools[:i], tools[i+1:]...)
			return
		}
	}
}

// IsAllowed checks if a tool is allowed in a state.
func (s *EligibilitySnapshot) IsAllowed(state agent.State, toolName string) bool {
	tools, ok := s.StateTools[state]
	if !ok {
		return false
	}
	for _, t := range tools {
		if t == toolName {
			return true
		}
	}
	return false
}

// TransitionSnapshot captures state transitions at a point in time.
type TransitionSnapshot struct {
	// Transitions maps from-states to allowed to-states.
	Transitions map[agent.State][]agent.State `json:"transitions"`
}

// NewTransitionSnapshot creates a new transition snapshot.
func NewTransitionSnapshot() TransitionSnapshot {
	return TransitionSnapshot{
		Transitions: make(map[agent.State][]agent.State),
	}
}

// AddTransition adds a state transition.
func (s *TransitionSnapshot) AddTransition(from, to agent.State) {
	if s.Transitions == nil {
		s.Transitions = make(map[agent.State][]agent.State)
	}
	targets := s.Transitions[from]
	for _, t := range targets {
		if t == to {
			return // Already exists
		}
	}
	s.Transitions[from] = append(targets, to)
}

// RemoveTransition removes a state transition.
func (s *TransitionSnapshot) RemoveTransition(from, to agent.State) {
	if s.Transitions == nil {
		return
	}
	targets := s.Transitions[from]
	for i, t := range targets {
		if t == to {
			s.Transitions[from] = append(targets[:i], targets[i+1:]...)
			return
		}
	}
}

// IsAllowed checks if a transition is allowed.
func (s *TransitionSnapshot) IsAllowed(from, to agent.State) bool {
	targets, ok := s.Transitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// BudgetLimitsSnapshot captures budget limits at a point in time.
type BudgetLimitsSnapshot struct {
	// Limits maps budget names to their limits.
	Limits map[string]int `json:"limits"`
}

// NewBudgetLimitsSnapshot creates a new budget limits snapshot.
func NewBudgetLimitsSnapshot() BudgetLimitsSnapshot {
	return BudgetLimitsSnapshot{
		Limits: make(map[string]int),
	}
}

// SetLimit sets a budget limit.
func (s *BudgetLimitsSnapshot) SetLimit(name string, limit int) {
	if s.Limits == nil {
		s.Limits = make(map[string]int)
	}
	s.Limits[name] = limit
}

// GetLimit gets a budget limit.
func (s *BudgetLimitsSnapshot) GetLimit(name string) (int, bool) {
	limit, ok := s.Limits[name]
	return limit, ok
}

// ApprovalSnapshot captures approval requirements at a point in time.
type ApprovalSnapshot struct {
	// RequiredTools lists tools that require approval.
	RequiredTools []string `json:"required_tools"`
}

// NewApprovalSnapshot creates a new approval snapshot.
func NewApprovalSnapshot() ApprovalSnapshot {
	return ApprovalSnapshot{
		RequiredTools: make([]string, 0),
	}
}

// RequireApproval adds a tool to the approval requirement list.
func (s *ApprovalSnapshot) RequireApproval(toolName string) {
	for _, t := range s.RequiredTools {
		if t == toolName {
			return // Already required
		}
	}
	s.RequiredTools = append(s.RequiredTools, toolName)
}

// RemoveApproval removes a tool from the approval requirement list.
func (s *ApprovalSnapshot) RemoveApproval(toolName string) {
	for i, t := range s.RequiredTools {
		if t == toolName {
			s.RequiredTools = append(s.RequiredTools[:i], s.RequiredTools[i+1:]...)
			return
		}
	}
}

// IsRequired checks if a tool requires approval.
func (s *ApprovalSnapshot) IsRequired(toolName string) bool {
	for _, t := range s.RequiredTools {
		if t == toolName {
			return true
		}
	}
	return false
}
