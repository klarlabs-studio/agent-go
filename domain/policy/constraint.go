package policy

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// ToolEligibility defines which tools are allowed in which states.
//
// Thread Safety: ToolEligibility is NOT safe for concurrent modification.
// It should be fully configured before being passed to the engine and
// treated as immutable thereafter. The read methods (IsAllowed, AllowedTools)
// are safe for concurrent use after configuration is complete.
type ToolEligibility struct {
	allowed map[agent.State]map[string]bool
}

// EligibilityRules maps states to the tools allowed in each state.
// This is the preferred way to configure tool eligibility declaratively.
//
// Example:
//
//	rules := policy.EligibilityRules{
//	    agent.StateExplore: {"read_file", "list_dir"},
//	    agent.StateAct:     {"write_file", "delete_file"},
//	    agent.StateValidate: {"read_file"},
//	}
//	eligibility := policy.NewToolEligibilityWith(rules)
type EligibilityRules map[agent.State][]string

// NewToolEligibility creates a new empty tool eligibility configuration.
// Use the Allow or AllowMultiple methods to add rules.
func NewToolEligibility() *ToolEligibility {
	return &ToolEligibility{
		allowed: make(map[agent.State]map[string]bool),
	}
}

// NewToolEligibilityWith creates a tool eligibility configuration from a rules map.
// This is the preferred constructor for declarative, readable configuration.
//
// Example:
//
//	eligibility := policy.NewToolEligibilityWith(policy.EligibilityRules{
//	    agent.StateExplore: {"lookup_customer", "get_order_status", "search_kb"},
//	    agent.StateAct:     {"create_ticket", "escalate"},
//	    agent.StateValidate: {"search_kb"},
//	})
func NewToolEligibilityWith(rules EligibilityRules) *ToolEligibility {
	e := NewToolEligibility()
	for state, tools := range rules {
		e.AllowMultiple(state, tools...)
	}
	return e
}

// Allow permits a tool in the given state.
func (e *ToolEligibility) Allow(state agent.State, toolName string) *ToolEligibility {
	if e.allowed[state] == nil {
		e.allowed[state] = make(map[string]bool)
	}
	e.allowed[state][toolName] = true
	return e
}

// AllowMultiple permits multiple tools in the given state.
func (e *ToolEligibility) AllowMultiple(state agent.State, toolNames ...string) *ToolEligibility {
	for _, name := range toolNames {
		e.Allow(state, name)
	}
	return e
}

// IsAllowed checks if a tool is allowed in the given state.
// A wildcard entry "*" in a state's allowed tools permits all tools in that state.
func (e *ToolEligibility) IsAllowed(state agent.State, toolName string) bool {
	stateTools, exists := e.allowed[state]
	if !exists {
		return false
	}
	if stateTools["*"] {
		return true
	}
	return stateTools[toolName]
}

// AllowedTools returns all tools allowed in the given state.
// If a wildcard "*" entry exists, it is included in the returned list.
// Callers should check for "*" to determine if all tools are permitted.
func (e *ToolEligibility) AllowedTools(state agent.State) []string {
	stateTools, exists := e.allowed[state]
	if !exists {
		return nil
	}

	tools := make([]string, 0, len(stateTools))
	for name := range stateTools {
		tools = append(tools, name)
	}
	return tools
}

// HasWildcard returns true if the given state allows all tools via the "*" wildcard.
func (e *ToolEligibility) HasWildcard(state agent.State) bool {
	stateTools, exists := e.allowed[state]
	if !exists {
		return false
	}
	return stateTools["*"]
}

// NewDefaultToolEligibility creates a tool eligibility configuration with sensible defaults.
// All registered tools are allowed (via wildcard "*") in explore, decide, act, and validate states.
// The intake state has no tools allowed (it normalizes the goal without tool use).
// Terminal states (done, failed) have no tools allowed.
//
// This is a convenient starting point for most agents. For fine-grained control,
// use NewToolEligibility() or NewToolEligibilityWith() instead.
func NewDefaultToolEligibility() *ToolEligibility {
	return NewToolEligibilityWith(EligibilityRules{
		agent.StateExplore:  {"*"},
		agent.StateDecide:   {"*"},
		agent.StateAct:      {"*"},
		agent.StateValidate: {"*"},
	})
}

// StateTransitions defines allowed state transitions.
//
// Thread Safety: StateTransitions is NOT safe for concurrent modification.
// It should be fully configured before being passed to the engine and
// treated as immutable thereafter. The read methods (CanTransition,
// AllowedTransitions) are safe for concurrent use after configuration is complete.
type StateTransitions struct {
	transitions map[agent.State][]agent.State
}

// TransitionRules maps states to the states they can transition to.
// This is the preferred way to configure state transitions declaratively.
//
// Example:
//
//	rules := policy.TransitionRules{
//	    agent.StateIntake:  {agent.StateExplore, agent.StateFailed},
//	    agent.StateExplore: {agent.StateDecide, agent.StateFailed},
//	    agent.StateDecide:  {agent.StateAct, agent.StateDone, agent.StateFailed},
//	}
//	transitions := policy.NewStateTransitionsWith(rules)
type TransitionRules map[agent.State][]agent.State

// NewStateTransitions creates a new empty state transition configuration.
// Use the Allow method to add rules, or use DefaultTransitions() for the canonical configuration.
func NewStateTransitions() *StateTransitions {
	return &StateTransitions{
		transitions: make(map[agent.State][]agent.State),
	}
}

// NewStateTransitionsWith creates a state transition configuration from a rules map.
// This is the preferred constructor for declarative, readable configuration.
//
// Example:
//
//	transitions := policy.NewStateTransitionsWith(policy.TransitionRules{
//	    agent.StateIntake:   {agent.StateExplore, agent.StateFailed},
//	    agent.StateExplore:  {agent.StateDecide, agent.StateFailed},
//	    agent.StateDecide:   {agent.StateAct, agent.StateDone, agent.StateFailed},
//	    agent.StateAct:      {agent.StateValidate, agent.StateFailed},
//	    agent.StateValidate: {agent.StateDone, agent.StateExplore, agent.StateFailed},
//	})
func NewStateTransitionsWith(rules TransitionRules) *StateTransitions {
	t := NewStateTransitions()
	for from, toStates := range rules {
		for _, to := range toStates {
			t.Allow(from, to)
		}
	}
	return t
}

// Allow permits a transition from one state to another.
func (t *StateTransitions) Allow(from, to agent.State) *StateTransitions {
	t.transitions[from] = append(t.transitions[from], to)
	return t
}

// CanTransition checks if a transition is allowed.
func (t *StateTransitions) CanTransition(from, to agent.State) bool {
	allowed, exists := t.transitions[from]
	if !exists {
		return false
	}

	for _, state := range allowed {
		if state == to {
			return true
		}
	}
	return false
}

// AllowedTransitions returns all states reachable from the given state.
func (t *StateTransitions) AllowedTransitions(from agent.State) []agent.State {
	return t.transitions[from]
}

// DefaultTransitions returns the canonical state transition configuration.
//
// The default state machine flow is:
//
//	intake → explore → decide → act → validate → done
//	                                    ↓
//	                                  explore (loop back)
//
// All non-terminal states can transition to failed.
func DefaultTransitions() *StateTransitions {
	return NewStateTransitionsWith(TransitionRules{
		agent.StateIntake:   {agent.StateExplore, agent.StateFailed},
		agent.StateExplore:  {agent.StateDecide, agent.StateFailed},
		agent.StateDecide:   {agent.StateAct, agent.StateDone, agent.StateFailed},
		agent.StateAct:      {agent.StateValidate, agent.StateFailed},
		agent.StateValidate: {agent.StateDone, agent.StateExplore, agent.StateFailed},
	})
}

// Constraint is a generic policy constraint that can be evaluated.
type Constraint interface {
	// Evaluate checks if the constraint is satisfied.
	Evaluate(ctx ConstraintContext) (bool, string)
}

// ConstraintContext provides context for constraint evaluation.
type ConstraintContext struct {
	RunID        string
	CurrentState agent.State
	ToolName     string
	Budget       *Budget
}
