// Package agent provides the core domain model for the agent runtime.
package agent

// State represents a structural constraint in the agent's execution.
// States are identified by stable strings, not behavioral definitions.
type State string

// Canonical states as defined in the TDD.
const (
	StateIntake   State = "intake"   // Normalize goal
	StateExplore  State = "explore"  // Gather evidence
	StateDecide   State = "decide"   // Choose next step
	StateAct      State = "act"      // Perform side-effects
	StateValidate State = "validate" // Confirm outcome
	StateDone     State = "done"     // Terminal success
	StateFailed   State = "failed"   // Terminal failure
)

// IsTerminal returns true if this is a terminal state (done or failed).
func (s State) IsTerminal() bool {
	return s == StateDone || s == StateFailed
}

// AllowsSideEffects returns true if the state permits side-effect operations.
func (s State) AllowsSideEffects() bool {
	return s == StateAct
}

// IsValid returns true if the state is a recognized canonical state.
// Custom states registered via a StateRegistry are not validated here;
// use StateRegistry.IsValid for combined validation.
func (s State) IsValid() bool {
	switch s {
	case StateIntake, StateExplore, StateDecide, StateAct, StateValidate, StateDone, StateFailed:
		return true
	default:
		return false
	}
}

// String returns the string representation of the state.
func (s State) String() string {
	return string(s)
}

// AllStates returns all canonical states.
func AllStates() []State {
	return []State{
		StateIntake,
		StateExplore,
		StateDecide,
		StateAct,
		StateValidate,
		StateDone,
		StateFailed,
	}
}

// TerminalStates returns all terminal states.
func TerminalStates() []State {
	return []State{StateDone, StateFailed}
}

// NonTerminalStates returns all non-terminal states.
func NonTerminalStates() []State {
	return []State{
		StateIntake,
		StateExplore,
		StateDecide,
		StateAct,
		StateValidate,
	}
}

// CustomState defines a user-registered state beyond the canonical seven.
// It carries semantics that govern how the state machine treats it.
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

// StateRegistry tracks both canonical and custom states.
// It provides unified validation across all registered states.
//
// Thread Safety: StateRegistry is NOT safe for concurrent modification.
// It should be fully configured before use and treated as immutable thereafter.
// Read methods (IsValid, IsTerminal, AllowsSideEffects, All) are safe for
// concurrent use after configuration is complete.
type StateRegistry struct {
	custom map[State]CustomState
}

// NewStateRegistry creates a new registry that recognizes the canonical states.
func NewStateRegistry() *StateRegistry {
	return &StateRegistry{
		custom: make(map[State]CustomState),
	}
}

// Register adds a custom state to the registry.
// Returns an error if the state name conflicts with a canonical state or
// a previously registered custom state.
func (r *StateRegistry) Register(cs CustomState) error {
	if cs.Name == "" {
		return ErrInvalidState
	}
	if cs.Name.IsValid() {
		return ErrCustomStateConflict
	}
	if _, exists := r.custom[cs.Name]; exists {
		return ErrCustomStateDuplicate
	}
	r.custom[cs.Name] = cs
	return nil
}

// IsValid returns true if the state is a canonical state or a registered custom state.
func (r *StateRegistry) IsValid(s State) bool {
	if s.IsValid() {
		return true
	}
	_, exists := r.custom[s]
	return exists
}

// IsTerminal returns true if the state is terminal, checking both canonical
// and custom states.
func (r *StateRegistry) IsTerminal(s State) bool {
	if s.IsTerminal() {
		return true
	}
	if cs, exists := r.custom[s]; exists {
		return cs.Terminal
	}
	return false
}

// AllowsSideEffects returns true if the state permits side-effect operations,
// checking both canonical and custom states.
func (r *StateRegistry) AllowsSideEffects(s State) bool {
	if s.AllowsSideEffects() {
		return true
	}
	if cs, exists := r.custom[s]; exists {
		return cs.AllowsSideEffects
	}
	return false
}

// Get returns the custom state definition, or an empty CustomState and false
// if not found. Canonical states are not returned by this method.
func (r *StateRegistry) Get(s State) (CustomState, bool) {
	cs, ok := r.custom[s]
	return cs, ok
}

// All returns all registered custom states.
func (r *StateRegistry) All() []CustomState {
	result := make([]CustomState, 0, len(r.custom))
	for _, cs := range r.custom {
		result = append(result, cs)
	}
	return result
}

// AllStatesIncludingCustom returns canonical states plus all custom states.
func (r *StateRegistry) AllStatesIncludingCustom() []State {
	states := AllStates()
	for name := range r.custom {
		states = append(states, name)
	}
	return states
}
