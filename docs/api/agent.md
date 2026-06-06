# Package `agent`

**Import path:** `go.klarlabs.de/agent/domain/agent`

## Overview

package agent // import "go.klarlabs.de/agent/domain/agent"

Package agent provides the core domain model for the agent runtime.

## Full API Reference

```
package agent // import "go.klarlabs.de/agent/domain/agent"

Package agent provides the core domain model for the agent runtime.

VARIABLES

var (
	// ErrInvalidState indicates the state is not a recognized canonical state.
	ErrInvalidState = errors.New("invalid state")

	// ErrInvalidTransition indicates an attempted state transition is not allowed.
	ErrInvalidTransition = errors.New("invalid state transition")

	// ErrRunTerminated indicates an operation was attempted on a terminated run.
	ErrRunTerminated = errors.New("run already terminated")

	// ErrRunNotStarted indicates an operation requires a started run.
	ErrRunNotStarted = errors.New("run not started")

	// ErrRunPaused indicates an operation requires an active run.
	ErrRunPaused = errors.New("run is paused")

	// ErrAwaitingHumanInput indicates the run is paused awaiting human input.
	ErrAwaitingHumanInput = errors.New("run is awaiting human input")

	// ErrNoPendingQuestion indicates no pending question exists for human input.
	ErrNoPendingQuestion = errors.New("run does not have a pending question")

	// ErrInvalidHumanInput indicates the human input is not valid for the pending question.
	ErrInvalidHumanInput = errors.New("invalid human input for pending question")

	// ErrCustomStateConflict indicates a custom state name conflicts with a canonical state.
	ErrCustomStateConflict = errors.New("custom state name conflicts with canonical state")

	// ErrCustomStateDuplicate indicates a custom state with the same name is already registered.
	ErrCustomStateDuplicate = errors.New("custom state already registered")
)
    Domain errors for the agent runtime.


TYPES

type AskHumanDecision struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"` // Optional constrained choices
}
    AskHumanDecision requests human input before proceeding.

type CallToolDecision struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
	Reason   string          `json:"reason"`
}
    CallToolDecision instructs the engine to execute a tool.

type CustomState struct {
	// Name is the stable string identifier for the state.
	Name State

	// AllowsSideEffects indicates whether tools with side effects may execute
	// in this state. Only the canonical "act" state allows this by default.
	AllowsSideEffects bool

	// Terminal indicates whether this is a terminal (final) state.
	// Terminal states cannot transition to other states.
	Terminal bool
}
    CustomState defines a user-registered state beyond the canonical seven.
    It carries semantics that govern how the state machine treats it.

type Decision struct {
	Type       DecisionType
	CallTool   *CallToolDecision
	Transition *TransitionDecision
	AskHuman   *AskHumanDecision
	Finish     *FinishDecision
	Fail       *FailDecision
}
    Decision represents the planner's output - exactly one of the fields is set.

func NewAskHumanDecision(question string, options ...string) Decision
    NewAskHumanDecision creates a decision to request human input.

func NewCallToolDecision(toolName string, input json.RawMessage, reason string) Decision
    NewCallToolDecision creates a decision to execute a tool.

func NewFailDecision(reason string, err error) Decision
    NewFailDecision creates a decision to terminate with failure.

func NewFinishDecision(summary string, result json.RawMessage) Decision
    NewFinishDecision creates a decision to complete successfully.

func NewTransitionDecision(toState State, reason string) Decision
    NewTransitionDecision creates a decision to transition states.

func (d Decision) IsTerminal() bool
    IsTerminal returns true if the decision leads to a terminal state.

type DecisionType string
    DecisionType identifies the kind of decision made by the planner.

const (
	DecisionCallTool   DecisionType = "call_tool"  // Execute a tool
	DecisionTransition DecisionType = "transition" // Move to another state
	DecisionAskHuman   DecisionType = "ask_human"  // Request human input
	DecisionFinish     DecisionType = "finish"     // Complete successfully
	DecisionFail       DecisionType = "fail"       // Terminate with failure
)
type Evidence struct {
	Type      EvidenceType    `json:"type"`
	Source    string          `json:"source"` // Tool name or "system"
	Content   json.RawMessage `json:"content"`
	Timestamp time.Time       `json:"timestamp"`
}
    Evidence represents an observation or result accumulated during a run.

func NewHumanEvidence(content json.RawMessage) Evidence
    NewHumanEvidence creates evidence from human input.

func NewSystemEvidence(note string) Evidence
    NewSystemEvidence creates system-generated evidence.

func NewToolEvidence(toolName string, content json.RawMessage) Evidence
    NewToolEvidence creates evidence from a tool result.

type EvidenceType string
    EvidenceType classifies the source of evidence.

const (
	EvidenceToolResult EvidenceType = "tool_result" // Result from tool execution
	EvidenceHumanInput EvidenceType = "human_input" // Input from human
	EvidenceSystemNote EvidenceType = "system_note" // System-generated observation
)
type FailDecision struct {
	Reason string `json:"reason"`
	Err    error  `json:"-"` // Not serialized
}
    FailDecision indicates terminal failure.

type FinishDecision struct {
	Summary string          `json:"summary"`
	Result  json.RawMessage `json:"result,omitempty"`
}
    FinishDecision indicates successful completion.

type PendingQuestion struct {
	Question string    `json:"question"`
	Options  []string  `json:"options,omitempty"`
	AskedAt  time.Time `json:"asked_at"`
}
    PendingQuestion represents a question awaiting human input.

type Run struct {
	ID              string           `json:"id"`
	Goal            string           `json:"goal"`
	CurrentState    State            `json:"current_state"`
	Vars            map[string]any   `json:"vars"`
	Evidence        []Evidence       `json:"evidence"`
	Status          RunStatus        `json:"status"`
	StartTime       time.Time        `json:"start_time"`
	EndTime         time.Time        `json:"end_time,omitempty"`
	Result          json.RawMessage  `json:"result,omitempty"`
	Error           string           `json:"error,omitempty"`
	PendingQuestion *PendingQuestion `json:"pending_question,omitempty"`
}
    Run represents a single execution of the agent. It is the aggregate root for
    the agent domain.

func NewRun(id string, goal string) *Run
    NewRun creates a new run with the given ID and initial state.

func (r *Run) AddEvidence(e Evidence)
    AddEvidence appends evidence to the run.

func (r *Run) AskHuman(question string, options []string)
    AskHuman sets a pending question and pauses the run.

func (r *Run) ClearPendingQuestion()
    ClearPendingQuestion removes the pending question from the run.

func (r *Run) Complete(result json.RawMessage)
    Complete marks the run as successfully completed.

func (r *Run) Duration() time.Duration
    Duration returns the duration of the run.

func (r *Run) Fail(err string)
    Fail marks the run as failed with an error.

func (r *Run) GetVar(key string) (any, bool)
    GetVar retrieves a variable from the run context.

func (r *Run) HasPendingQuestion() bool
    HasPendingQuestion returns true if the run has a pending question awaiting
    human input.

func (r *Run) IsTerminal() bool
    IsTerminal returns true if the run has reached a terminal status.

func (r *Run) Pause()
    Pause suspends the run for later resumption.

func (r *Run) Resume()
    Resume continues a paused run.

func (r *Run) SetVar(key string, value any)
    SetVar sets a variable in the run context.

func (r *Run) Start()
    Start marks the run as running.

func (r *Run) TransitionTo(state State)
    TransitionTo changes the current state.

type RunStatus string
    RunStatus represents the current status of a run.

const (
	RunStatusPending   RunStatus = "pending"   // Not yet started
	RunStatusRunning   RunStatus = "running"   // Currently executing
	RunStatusPaused    RunStatus = "paused"    // Temporarily suspended
	RunStatusCompleted RunStatus = "completed" // Successfully finished
	RunStatusFailed    RunStatus = "failed"    // Terminated with error
)
type State string
    State represents a structural constraint in the agent's execution. States
    are identified by stable strings, not behavioral definitions.

const (
	StateIntake   State = "intake"   // Normalize goal
	StateExplore  State = "explore"  // Gather evidence
	StateDecide   State = "decide"   // Choose next step
	StateAct      State = "act"      // Perform side-effects
	StateValidate State = "validate" // Confirm outcome
	StateDone     State = "done"     // Terminal success
	StateFailed   State = "failed"   // Terminal failure
)
    Canonical states as defined in the TDD.

func AllStates() []State
    AllStates returns all canonical states.

func NonTerminalStates() []State
    NonTerminalStates returns all non-terminal states.

func TerminalStates() []State
    TerminalStates returns all terminal states.

func (s State) AllowsSideEffects() bool
    AllowsSideEffects returns true if the state permits side-effect operations.

func (s State) IsTerminal() bool
    IsTerminal returns true if this is a terminal state (done or failed).

func (s State) IsValid() bool
    IsValid returns true if the state is a recognized canonical state.
    Custom states registered via a StateRegistry are not validated here;
    use StateRegistry.IsValid for combined validation.

func (s State) String() string
    String returns the string representation of the state.

type StateRegistry struct {
	// Has unexported fields.
}
    StateRegistry tracks both canonical and custom states. It provides unified
    validation across all registered states.

    Thread Safety: StateRegistry is NOT safe for concurrent modification. It
    should be fully configured before use and treated as immutable thereafter.
    Read methods (IsValid, IsTerminal, AllowsSideEffects, All) are safe for
    concurrent use after configuration is complete.

func NewStateRegistry() *StateRegistry
    NewStateRegistry creates a new registry that recognizes the canonical
    states.

func (r *StateRegistry) All() []CustomState
    All returns all registered custom states.

func (r *StateRegistry) AllStatesIncludingCustom() []State
    AllStatesIncludingCustom returns canonical states plus all custom states.

func (r *StateRegistry) AllowsSideEffects(s State) bool
    AllowsSideEffects returns true if the state permits side-effect operations,
    checking both canonical and custom states.

func (r *StateRegistry) Get(s State) (CustomState, bool)
    Get returns the custom state definition, or an empty CustomState and false
    if not found. Canonical states are not returned by this method.

func (r *StateRegistry) IsTerminal(s State) bool
    IsTerminal returns true if the state is terminal, checking both canonical
    and custom states.

func (r *StateRegistry) IsValid(s State) bool
    IsValid returns true if the state is a canonical state or a registered
    custom state.

func (r *StateRegistry) Register(cs CustomState) error
    Register adds a custom state to the registry. Returns an error if the state
    name conflicts with a canonical state or a previously registered custom
    state.

type TransitionDecision struct {
	ToState State  `json:"to_state"`
	Reason  string `json:"reason"`
}
    TransitionDecision instructs the engine to transition to another state.
```
