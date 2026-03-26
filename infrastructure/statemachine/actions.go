package statemachine

import (
	"fmt"
	"sort"

	"github.com/felixgeelhaar/statekit"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// logStateEntry logs when entering a state.
// In statekit, actions receive a pointer to the context. Since our context is *Context,
// actions receive **Context.
func logStateEntry(ctx **Context, event statekit.Event) {
	if ctx == nil || *ctx == nil || (*ctx).Run == nil {
		return
	}

	c := *ctx

	// Get target state from payload if available
	var newState agent.State
	if payload, ok := event.Payload.(TransitionPayload); ok {
		newState = payload.ToState
	} else {
		// Derive from event type
		newState = stateFromEventType(event.Type)
	}

	if newState != "" {
		c.Run.CurrentState = newState
	}
}

// recordTransition records the state transition in the ledger.
func recordTransition(ctx **Context, event statekit.Event) {
	if ctx == nil || *ctx == nil || (*ctx).Run == nil || (*ctx).Ledger == nil {
		return
	}

	c := *ctx
	fromState := c.Run.CurrentState

	// Get target state and reason from payload
	var toState agent.State
	var reason string
	if payload, ok := event.Payload.(TransitionPayload); ok {
		toState = payload.ToState
		reason = payload.Reason
	} else {
		// Derive from event type
		toState = stateFromEventType(event.Type)
	}

	c.Ledger.RecordTransition(fromState, toState, reason)

	// Update run state
	c.Run.TransitionTo(toState)
}

// ActionWithReason creates a payload that includes a reason in the event.
func ActionWithReason(reason string) TransitionPayload {
	return TransitionPayload{
		Reason: reason,
	}
}

// ActionFunc is a function that performs work during a state transition.
// It returns an error if the action fails.
type ActionFunc func(ctx *Context, event statekit.Event) error

// RollbackFunc is a function that reverses the effects of an action on failure.
type RollbackFunc func(ctx *Context, event statekit.Event) error

// RegisteredAction is a named action with priority, execution function,
// and optional rollback.
type RegisteredAction struct {
	// Name identifies the action for debugging and logging.
	Name string

	// Priority determines execution order. Lower values execute first.
	// Actions with equal priority execute in registration order.
	Priority int

	// Execute performs the action's work.
	Execute ActionFunc

	// Rollback reverses the action's effects. May be nil if no rollback is needed.
	Rollback RollbackFunc
}

// ActionRegistry manages named actions with priority ordering and rollback support.
//
// Thread Safety: ActionRegistry is NOT safe for concurrent modification.
// It should be fully configured before use. The Execute method is safe for
// concurrent use after configuration is complete, as long as the individual
// action functions are themselves safe.
type ActionRegistry struct {
	actions []RegisteredAction
	byName  map[string]*RegisteredAction
}

// NewActionRegistry creates a new empty action registry.
func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		byName: make(map[string]*RegisteredAction),
	}
}

// Register adds an action to the registry. Returns an error if an action
// with the same name is already registered.
func (r *ActionRegistry) Register(action RegisteredAction) error {
	if action.Name == "" {
		return fmt.Errorf("action name must not be empty")
	}
	if _, exists := r.byName[action.Name]; exists {
		return fmt.Errorf("action %q already registered", action.Name)
	}
	r.actions = append(r.actions, action)
	r.byName[action.Name] = &r.actions[len(r.actions)-1]
	return nil
}

// Get returns the named action, or nil if not found.
func (r *ActionRegistry) Get(name string) *RegisteredAction {
	return r.byName[name]
}

// All returns all registered actions in no particular order.
func (r *ActionRegistry) All() []RegisteredAction {
	result := make([]RegisteredAction, len(r.actions))
	copy(result, r.actions)
	return result
}

// Names returns the names of all registered actions.
func (r *ActionRegistry) Names() []string {
	names := make([]string, 0, len(r.actions))
	for _, a := range r.actions {
		names = append(names, a.Name)
	}
	return names
}

// Execute runs all registered actions in priority order (lower priority first).
// If any action fails, rollback functions for previously executed actions are
// called in reverse order. Returns the first action error encountered.
func (r *ActionRegistry) Execute(ctx *Context, event statekit.Event) error {
	sorted := r.sorted()

	var executed []RegisteredAction
	for _, action := range sorted {
		if err := action.Execute(ctx, event); err != nil {
			// Rollback in reverse order
			r.rollback(executed, ctx, event)
			return fmt.Errorf("action %q failed: %w", action.Name, err)
		}
		executed = append(executed, action)
	}

	return nil
}

// sorted returns actions sorted by priority (ascending), preserving
// registration order for equal priorities.
func (r *ActionRegistry) sorted() []RegisteredAction {
	sorted := make([]RegisteredAction, len(r.actions))
	copy(sorted, r.actions)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	return sorted
}

// rollback runs rollback functions in reverse order, ignoring errors from
// individual rollbacks to ensure all rollbacks are attempted.
func (r *ActionRegistry) rollback(executed []RegisteredAction, ctx *Context, event statekit.Event) {
	for i := len(executed) - 1; i >= 0; i-- {
		if executed[i].Rollback != nil {
			// Rollback errors are intentionally ignored to ensure all
			// rollbacks execute. Callers should log rollback failures
			// through the context's ledger.
			_ = executed[i].Rollback(ctx, event)
		}
	}
}
