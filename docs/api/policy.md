# Package `policy`

**Import path:** `go.klarlabs.de/agent/domain/policy`

## Overview

package policy // import "go.klarlabs.de/agent/domain/policy"

Package policy provides domain models for policy enforcement.

Package policy provides policy constraint types.

Package policy provides policy constraint types.

## Full API Reference

```
package policy // import "go.klarlabs.de/agent/domain/policy"

Package policy provides domain models for policy enforcement.

Package policy provides policy constraint types.

Package policy provides policy constraint types.

VARIABLES

var (
	// ErrBudgetExceeded indicates the budget limit has been exceeded.
	ErrBudgetExceeded = errors.New("budget exceeded")

	// ErrApprovalTimeout indicates the approval request timed out.
	ErrApprovalTimeout = errors.New("approval request timed out")

	// ErrConstraintViolation indicates a policy constraint was violated.
	ErrConstraintViolation = errors.New("constraint violation")

	// ErrTransitionNotAllowed indicates the state transition is not permitted.
	ErrTransitionNotAllowed = errors.New("state transition not allowed")

	// ErrToolNotEligible indicates the tool is not eligible in the current state.
	ErrToolNotEligible = errors.New("tool not eligible in current state")

	// ErrRateLimitExceeded indicates the rate limit has been exceeded.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)
    Domain errors for policy enforcement.

    Note: For approval-related errors (required, denied), use the
    canonical errors from the tool package (tool.ErrApprovalRequired,
    tool.ErrApprovalDenied). This avoids duplication and maintains clear
    ownership since approvals are fundamentally about tool execution
    authorization.


TYPES

type ApprovalDiff struct {
	ToolName string `json:"tool_name"`
	Before   bool   `json:"before"`
	After    bool   `json:"after"`
}
    ApprovalDiff represents an approval requirement change between versions.

type ApprovalPolicy struct {
	// RequireForDestructive requires approval for destructive tools.
	RequireForDestructive bool

	// RequireForHighRisk requires approval for high-risk tools.
	RequireForHighRisk bool

	// RequireForTools lists specific tools that always require approval.
	RequireForTools []string

	// ExemptTools lists tools that never require approval.
	ExemptTools []string
}
    ApprovalPolicy determines which actions require approval.

func DefaultApprovalPolicy() ApprovalPolicy
    DefaultApprovalPolicy returns a policy requiring approval for destructive
    actions.

func (p ApprovalPolicy) RequiresApproval(toolName string, isDestructive, isHighRisk bool) bool
    RequiresApproval checks if the given tool requires approval under this
    policy.

type ApprovalRequest struct {
	RunID     string          `json:"run_id"`
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	Reason    string          `json:"reason"`
	RiskLevel string          `json:"risk_level"`
	Timestamp time.Time       `json:"timestamp"`
}
    ApprovalRequest contains information for an approval decision.

type ApprovalResponse struct {
	Approved  bool      `json:"approved"`
	Approver  string    `json:"approver,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
    ApprovalResponse contains the result of an approval request.

type ApprovalSnapshot struct {
	// RequiredTools lists tools that require approval.
	RequiredTools []string `json:"required_tools"`
}
    ApprovalSnapshot captures approval requirements at a point in time.

func NewApprovalSnapshot() ApprovalSnapshot
    NewApprovalSnapshot creates a new approval snapshot.

func (s *ApprovalSnapshot) IsRequired(toolName string) bool
    IsRequired checks if a tool requires approval.

func (s *ApprovalSnapshot) RemoveApproval(toolName string)
    RemoveApproval removes a tool from the approval requirement list.

func (s *ApprovalSnapshot) RequireApproval(toolName string)
    RequireApproval adds a tool to the approval requirement list.

type Approver interface {
	// Approve processes an approval request and returns the decision.
	Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)
}
    Approver is the interface for approval handlers.

type AutoApprover struct {
	// Has unexported fields.
}
    AutoApprover automatically approves all requests.

func NewAutoApprover(name string) *AutoApprover
    NewAutoApprover creates an approver that automatically approves all
    requests.

func (a *AutoApprover) Approve(_ context.Context, _ ApprovalRequest) (ApprovalResponse, error)
    Approve automatically approves the request.

type Budget struct {
	// Has unexported fields.
}
    Budget tracks consumption against configured limits.

func NewBudget(limits map[string]int) *Budget
    NewBudget creates a budget with the given limits.

func UnlimitedBudget() *Budget
    UnlimitedBudget creates a budget with no limits.

func (b *Budget) CanConsume(name string, amount int) bool
    CanConsume checks if the budget allows consuming the given amount.

func (b *Budget) Consume(name string, amount int) error
    Consume deducts from the budget if allowed.

func (b *Budget) ExhaustedBudgets() []string
    ExhaustedBudgets returns the names of all exhausted budgets.

func (b *Budget) IsExhausted() bool
    IsExhausted returns true if any budget is fully consumed.

func (b *Budget) Remaining(name string) int
    Remaining returns the remaining budget for a given name.

func (b *Budget) Reset()
    Reset resets all consumed values to zero.

func (b *Budget) SetLimit(name string, limit int)
    SetLimit sets or updates a budget limit.

func (b *Budget) Snapshot() BudgetSnapshot
    Snapshot returns an immutable view of the current budget state.

type BudgetDiff struct {
	BudgetName string `json:"budget_name"`
	Before     int    `json:"before"`
	After      int    `json:"after"`
}
    BudgetDiff represents a budget change between versions.

type BudgetLimitsSnapshot struct {
	// Limits maps budget names to their limits.
	Limits map[string]int `json:"limits"`
}
    BudgetLimitsSnapshot captures budget limits at a point in time.

func NewBudgetLimitsSnapshot() BudgetLimitsSnapshot
    NewBudgetLimitsSnapshot creates a new budget limits snapshot.

func (s *BudgetLimitsSnapshot) GetLimit(name string) (int, bool)
    GetLimit gets a budget limit.

func (s *BudgetLimitsSnapshot) SetLimit(name string, limit int)
    SetLimit sets a budget limit.

type BudgetSnapshot struct {
	Limits    map[string]int `json:"limits"`
	Consumed  map[string]int `json:"consumed"`
	Remaining map[string]int `json:"remaining"`
}
    BudgetSnapshot is an immutable view of budget state.

type Constraint interface {
	// Evaluate checks if the constraint is satisfied.
	Evaluate(ctx ConstraintContext) (bool, string)
}
    Constraint is a generic policy constraint that can be evaluated.

type ConstraintContext struct {
	RunID        string
	CurrentState agent.State
	ToolName     string
	Budget       *Budget
}
    ConstraintContext provides context for constraint evaluation.

type DenyApprover struct {
	// Has unexported fields.
}
    DenyApprover automatically denies all requests.

func NewDenyApprover(reason string) *DenyApprover
    NewDenyApprover creates an approver that automatically denies all requests.

func (d *DenyApprover) Approve(_ context.Context, _ ApprovalRequest) (ApprovalResponse, error)
    Approve automatically denies the request.

type EligibilityDiff struct {
	State    string `json:"state"`
	ToolName string `json:"tool_name"`
	Before   bool   `json:"before"`
	After    bool   `json:"after"`
}
    EligibilityDiff represents an eligibility change between versions.

type EligibilityRules map[agent.State][]string
    EligibilityRules maps states to the tools allowed in each state. This is the
    preferred way to configure tool eligibility declaratively.

    Example:

        rules := policy.EligibilityRules{
            agent.StateExplore: {"read_file", "list_dir"},
            agent.StateAct:     {"write_file", "delete_file"},
            agent.StateValidate: {"read_file"},
        }
        eligibility := policy.NewToolEligibilityWith(rules)

type EligibilitySnapshot struct {
	// StateTools maps states to allowed tool names.
	StateTools map[agent.State][]string `json:"state_tools"`
}
    EligibilitySnapshot captures tool eligibility at a point in time.

func NewEligibilitySnapshot() EligibilitySnapshot
    NewEligibilitySnapshot creates a new eligibility snapshot.

func (s *EligibilitySnapshot) AddTool(state agent.State, toolName string)
    AddTool adds a tool to a state's eligibility.

func (s *EligibilitySnapshot) IsAllowed(state agent.State, toolName string) bool
    IsAllowed checks if a tool is allowed in a state.

func (s *EligibilitySnapshot) RemoveTool(state agent.State, toolName string)
    RemoveTool removes a tool from a state's eligibility.

type PolicyVersion struct {
	// Version is the version number (monotonically increasing).
	Version int `json:"version"`

	// CreatedAt is when this version was created.
	CreatedAt time.Time `json:"created_at"`

	// ProposalID links to the proposal that created this version (if any).
	ProposalID string `json:"proposal_id,omitempty"`

	// Description explains what changed in this version.
	Description string `json:"description,omitempty"`

	// Eligibility contains the tool eligibility snapshot.
	Eligibility EligibilitySnapshot `json:"eligibility"`

	// Transitions contains the state transitions snapshot.
	Transitions TransitionSnapshot `json:"transitions"`

	// Budgets contains the budget limits snapshot.
	Budgets BudgetLimitsSnapshot `json:"budgets"`

	// Approvals contains the approval requirements snapshot.
	Approvals ApprovalSnapshot `json:"approvals"`
}
    PolicyVersion represents an immutable snapshot of policy configuration.

type StateTransitions struct {
	// Has unexported fields.
}
    StateTransitions defines allowed state transitions.

    Thread Safety: StateTransitions is NOT safe for concurrent modification. It
    should be fully configured before being passed to the engine and treated as
    immutable thereafter. The read methods (CanTransition, AllowedTransitions)
    are safe for concurrent use after configuration is complete.

func DefaultTransitions() *StateTransitions
    DefaultTransitions returns the canonical state transition configuration.

    The default state machine flow is:

        intake → explore → decide → act → validate → done
                                            ↓
                                          explore (loop back)

    All non-terminal states can transition to failed.

func NewStateTransitions() *StateTransitions
    NewStateTransitions creates a new empty state transition configuration.
    Use the Allow method to add rules, or use DefaultTransitions() for the
    canonical configuration.

func NewStateTransitionsWith(rules TransitionRules) *StateTransitions
    NewStateTransitionsWith creates a state transition configuration from a
    rules map. This is the preferred constructor for declarative, readable
    configuration.

    Example:

        transitions := policy.NewStateTransitionsWith(policy.TransitionRules{
            agent.StateIntake:   {agent.StateExplore, agent.StateFailed},
            agent.StateExplore:  {agent.StateDecide, agent.StateFailed},
            agent.StateDecide:   {agent.StateAct, agent.StateDone, agent.StateFailed},
            agent.StateAct:      {agent.StateValidate, agent.StateFailed},
            agent.StateValidate: {agent.StateDone, agent.StateExplore, agent.StateFailed},
        })

func (t *StateTransitions) Allow(from, to agent.State) *StateTransitions
    Allow permits a transition from one state to another.

func (t *StateTransitions) AllowedTransitions(from agent.State) []agent.State
    AllowedTransitions returns all states reachable from the given state.

func (t *StateTransitions) CanTransition(from, to agent.State) bool
    CanTransition checks if a transition is allowed.

type ToolEligibility struct {
	// Has unexported fields.
}
    ToolEligibility defines which tools are allowed in which states.

    Thread Safety: ToolEligibility is NOT safe for concurrent modification.
    It should be fully configured before being passed to the engine and treated
    as immutable thereafter. The read methods (IsAllowed, AllowedTools) are safe
    for concurrent use after configuration is complete.

func NewDefaultToolEligibility() *ToolEligibility
    NewDefaultToolEligibility creates a tool eligibility configuration with
    sensible defaults. All registered tools are allowed (via wildcard "*") in
    explore, decide, act, and validate states. The intake state has no tools
    allowed (it normalizes the goal without tool use). Terminal states (done,
    failed) have no tools allowed.

    This is a convenient starting point for most agents. For fine-grained
    control, use NewToolEligibility() or NewToolEligibilityWith() instead.

func NewToolEligibility() *ToolEligibility
    NewToolEligibility creates a new empty tool eligibility configuration.
    Use the Allow or AllowMultiple methods to add rules.

func NewToolEligibilityWith(rules EligibilityRules) *ToolEligibility
    NewToolEligibilityWith creates a tool eligibility configuration from a
    rules map. This is the preferred constructor for declarative, readable
    configuration.

    Example:

        eligibility := policy.NewToolEligibilityWith(policy.EligibilityRules{
            agent.StateExplore: {"lookup_customer", "get_order_status", "search_kb"},
            agent.StateAct:     {"create_ticket", "escalate"},
            agent.StateValidate: {"search_kb"},
        })

func (e *ToolEligibility) Allow(state agent.State, toolName string) *ToolEligibility
    Allow permits a tool in the given state.

func (e *ToolEligibility) AllowMultiple(state agent.State, toolNames ...string) *ToolEligibility
    AllowMultiple permits multiple tools in the given state.

func (e *ToolEligibility) AllowedTools(state agent.State) []string
    AllowedTools returns all tools allowed in the given state. If a wildcard "*"
    entry exists, it is included in the returned list. Callers should check for
    "*" to determine if all tools are permitted.

func (e *ToolEligibility) HasWildcard(state agent.State) bool
    HasWildcard returns true if the given state allows all tools via the "*"
    wildcard.

func (e *ToolEligibility) IsAllowed(state agent.State, toolName string) bool
    IsAllowed checks if a tool is allowed in the given state. A wildcard entry
    "*" in a state's allowed tools permits all tools in that state.

type TransitionDiff struct {
	FromState string `json:"from_state"`
	ToState   string `json:"to_state"`
	Before    bool   `json:"before"`
	After     bool   `json:"after"`
}
    TransitionDiff represents a transition change between versions.

type TransitionRules map[agent.State][]agent.State
    TransitionRules maps states to the states they can transition to. This is
    the preferred way to configure state transitions declaratively.

    Example:

        rules := policy.TransitionRules{
            agent.StateIntake:  {agent.StateExplore, agent.StateFailed},
            agent.StateExplore: {agent.StateDecide, agent.StateFailed},
            agent.StateDecide:  {agent.StateAct, agent.StateDone, agent.StateFailed},
        }
        transitions := policy.NewStateTransitionsWith(rules)

type TransitionSnapshot struct {
	// Transitions maps from-states to allowed to-states.
	Transitions map[agent.State][]agent.State `json:"transitions"`
}
    TransitionSnapshot captures state transitions at a point in time.

func NewTransitionSnapshot() TransitionSnapshot
    NewTransitionSnapshot creates a new transition snapshot.

func (s *TransitionSnapshot) AddTransition(from, to agent.State)
    AddTransition adds a state transition.

func (s *TransitionSnapshot) IsAllowed(from, to agent.State) bool
    IsAllowed checks if a transition is allowed.

func (s *TransitionSnapshot) RemoveTransition(from, to agent.State)
    RemoveTransition removes a state transition.

type VersionDiff struct {
	// FromVersion is the starting version.
	FromVersion int `json:"from_version"`

	// ToVersion is the ending version.
	ToVersion int `json:"to_version"`

	// EligibilityChanges lists eligibility differences.
	EligibilityChanges []EligibilityDiff `json:"eligibility_changes,omitempty"`

	// TransitionChanges lists transition differences.
	TransitionChanges []TransitionDiff `json:"transition_changes,omitempty"`

	// BudgetChanges lists budget differences.
	BudgetChanges []BudgetDiff `json:"budget_changes,omitempty"`

	// ApprovalChanges lists approval requirement differences.
	ApprovalChanges []ApprovalDiff `json:"approval_changes,omitempty"`
}
    VersionDiff represents the differences between two policy versions.

type VersionStore interface {
	// Save persists a new policy version.
	Save(ctx context.Context, version *PolicyVersion) error

	// GetCurrent retrieves the current (latest) policy version.
	GetCurrent(ctx context.Context) (*PolicyVersion, error)

	// Get retrieves a specific policy version.
	Get(ctx context.Context, version int) (*PolicyVersion, error)

	// List returns all policy versions.
	List(ctx context.Context) ([]*PolicyVersion, error)

	// GetByProposal retrieves the policy version created by a proposal.
	GetByProposal(ctx context.Context, proposalID string) (*PolicyVersion, error)
}
    VersionStore persists policy versions.
```
