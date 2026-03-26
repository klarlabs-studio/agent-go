package api

import (
	"context"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	domaintool "github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// NewToolBuilder creates a new tool builder.
func NewToolBuilder(name string) *domaintool.Builder {
	return domaintool.NewBuilder(name)
}

// NewToolRegistry creates a new in-memory tool registry.
func NewToolRegistry() *memory.ToolRegistry {
	return memory.NewToolRegistry()
}

// NewKnowledgeStore creates a new in-memory knowledge store for vector embeddings.
// If dimension is 0, it will be auto-detected from the first vector stored.
func NewKnowledgeStore(dimension int) *memory.KnowledgeStore {
	return memory.NewKnowledgeStore(dimension)
}

// NewMockPlanner creates a mock planner with predefined decisions.
func NewMockPlanner(decisions ...Decision) *planner.MockPlanner {
	return planner.NewMockPlanner(decisions...)
}

// NewScriptedPlanner creates a scripted planner for deterministic testing.
func NewScriptedPlanner(steps ...planner.ScriptStep) *planner.ScriptedPlanner {
	return planner.NewScriptedPlanner(steps...)
}

// ScriptStep is a step in a scripted planner.
type ScriptStep = planner.ScriptStep

// Rule is a condition-decision pair for the rule-based planner.
type Rule = planner.Rule

// RuleBuilder constructs rules using a fluent API.
type RuleBuilder = planner.RuleBuilder

// NewRule creates a new rule builder with the given name.
func NewRule(name string) *planner.RuleBuilder {
	return planner.NewRule(name)
}

// NewRuleBasedPlanner creates a rule-based planner that evaluates rules in priority order.
// The fallback decision is returned when no rule matches.
func NewRuleBasedPlanner(fallback Decision, rules ...planner.Rule) *planner.RuleBasedPlanner {
	return planner.NewRuleBasedPlanner(fallback, rules...)
}

// NewHybridPlanner creates a hybrid planner that tries rules first,
// then falls back to the given planner when no rule matches.
func NewHybridPlanner(rules *planner.RuleBasedPlanner, fallback planner.Planner) *planner.HybridPlanner {
	return planner.NewHybridPlanner(rules, fallback)
}

// EligibilityRules maps states to the tools allowed in each state.
// This is the preferred way to configure tool eligibility declaratively.
//
// Example:
//
//	eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
//	    api.StateExplore: {"read_file", "list_dir"},
//	    api.StateAct:     {"write_file", "delete_file"},
//	    api.StateValidate: {"read_file"},
//	})
type EligibilityRules = policy.EligibilityRules

// TransitionRules maps states to the states they can transition to.
// This is the preferred way to configure state transitions declaratively.
//
// Example:
//
//	transitions := api.NewStateTransitionsWith(api.TransitionRules{
//	    api.StateIntake:   {api.StateExplore, api.StateFailed},
//	    api.StateExplore:  {api.StateDecide, api.StateFailed},
//	    api.StateDecide:   {api.StateAct, api.StateDone, api.StateFailed},
//	})
type TransitionRules = policy.TransitionRules

// NewToolEligibility creates a new empty tool eligibility configuration.
// Use the Allow or AllowMultiple methods to add rules incrementally.
//
// Use this imperative style when:
//   - Building eligibility dynamically based on runtime conditions
//   - Adding tools conditionally or in a loop
//   - Preferring method chaining for readability
//
// For static configuration, prefer NewToolEligibilityWith instead.
//
// Example:
//
//	eligibility := api.NewToolEligibility()
//	eligibility.Allow(api.StateExplore, "read_file")
//	eligibility.Allow(api.StateExplore, "list_dir")
//	eligibility.Allow(api.StateAct, "write_file")
func NewToolEligibility() *policy.ToolEligibility {
	return policy.NewToolEligibility()
}

// NewToolEligibilityWith creates a tool eligibility configuration from a rules map.
// This is the preferred constructor for declarative, readable configuration.
//
// Example:
//
//	eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
//	    api.StateExplore: {"lookup_customer", "get_order_status", "search_kb"},
//	    api.StateAct:     {"create_ticket", "escalate"},
//	    api.StateValidate: {"search_kb"},
//	})
func NewToolEligibilityWith(rules EligibilityRules) *policy.ToolEligibility {
	return policy.NewToolEligibilityWith(rules)
}

// NewDefaultToolEligibility creates a tool eligibility with sensible defaults.
// All registered tools are allowed (via wildcard "*") in explore, decide, act, and validate states.
// The intake state has no tools allowed. Terminal states (done, failed) have no tools allowed.
//
// This is the easiest way to get started. For fine-grained per-state control,
// use NewToolEligibility() or NewToolEligibilityWith() instead.
func NewDefaultToolEligibility() *policy.ToolEligibility {
	return policy.NewDefaultToolEligibility()
}

// NewStateTransitions creates a new empty state transitions configuration.
// Use the Allow method to add rules incrementally.
//
// Use this imperative style when:
//   - Building transitions dynamically based on runtime conditions
//   - Adding transitions conditionally or in a loop
//   - Preferring method chaining for readability
//
// For static configuration, prefer NewStateTransitionsWith or DefaultTransitions instead.
//
// Example:
//
//	transitions := api.NewStateTransitions()
//	transitions.Allow(api.StateIntake, api.StateExplore)
//	transitions.Allow(api.StateExplore, api.StateDecide)
func NewStateTransitions() *policy.StateTransitions {
	return policy.NewStateTransitions()
}

// NewStateTransitionsWith creates a state transition configuration from a rules map.
// This is the preferred constructor for declarative, readable configuration.
//
// Example:
//
//	transitions := api.NewStateTransitionsWith(api.TransitionRules{
//	    api.StateIntake:   {api.StateExplore, api.StateFailed},
//	    api.StateExplore:  {api.StateDecide, api.StateFailed},
//	    api.StateDecide:   {api.StateAct, api.StateDone, api.StateFailed},
//	    api.StateAct:      {api.StateValidate, api.StateFailed},
//	    api.StateValidate: {api.StateDone, api.StateExplore, api.StateFailed},
//	})
func NewStateTransitionsWith(rules TransitionRules) *policy.StateTransitions {
	return policy.NewStateTransitionsWith(rules)
}

// DefaultTransitions returns the canonical state transition configuration.
func DefaultTransitions() *policy.StateTransitions {
	return policy.DefaultTransitions()
}

// NewAutoApprover creates an approver that automatically approves all requests.
func NewAutoApprover(name string) *policy.AutoApprover {
	return policy.NewAutoApprover(name)
}

// NewDenyApprover creates an approver that automatically denies all requests.
func NewDenyApprover(reason string) *policy.DenyApprover {
	return policy.NewDenyApprover(reason)
}

// Decision constructors

// NewCallToolDecision creates a decision to execute a tool.
func NewCallToolDecision(toolName string, input []byte, reason string) Decision {
	return agent.NewCallToolDecision(toolName, input, reason)
}

// NewTransitionDecision creates a decision to transition states.
func NewTransitionDecision(toState State, reason string) Decision {
	return agent.NewTransitionDecision(toState, reason)
}

// NewFinishDecision creates a decision to complete successfully.
func NewFinishDecision(summary string, result []byte) Decision {
	return agent.NewFinishDecision(summary, result)
}

// NewFailDecision creates a decision to terminate with failure.
func NewFailDecision(reason string, err error) Decision {
	return agent.NewFailDecision(reason, err)
}

// AutoApprover returns an approver that automatically approves all requests.
// This is a convenience function for development and testing.
func AutoApprover() policy.Approver {
	return policy.NewAutoApprover("auto")
}

// DenyApprover returns an approver that automatically denies all requests.
// This is a convenience function for testing rejection scenarios.
func DenyApprover(reason string) policy.Approver {
	return policy.NewDenyApprover(reason)
}

// ApprovalRequest is re-exported for callback approvers.
type ApprovalRequest = policy.ApprovalRequest

// ApprovalResponse is re-exported for callback approvers.
type ApprovalResponse = policy.ApprovalResponse

// CallbackApprover implements the Approver interface using a callback function.
type CallbackApprover struct {
	callback func(ctx context.Context, req ApprovalRequest) (bool, error)
}

// NewCallbackApprover creates an approver that uses a callback function for decisions.
func NewCallbackApprover(fn func(ctx context.Context, req ApprovalRequest) (bool, error)) *CallbackApprover {
	return &CallbackApprover{callback: fn}
}

// Approve processes the approval request using the callback function.
func (c *CallbackApprover) Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
	approved, err := c.callback(ctx, req)
	if err != nil {
		return ApprovalResponse{}, err
	}
	return ApprovalResponse{
		Approved:  approved,
		Approver:  "callback",
		Timestamp: time.Now(),
	}, nil
}
