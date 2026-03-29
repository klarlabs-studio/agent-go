package statemachine

import (
	"fmt"
	"time"

	"github.com/felixgeelhaar/statekit"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/policy"
)

// TransitionPayload carries additional data with a transition event.
type TransitionPayload struct {
	ToState agent.State
	Reason  string
}

// Interpreter wraps the statekit interpreter with agent-specific functionality.
type Interpreter struct {
	interp   *statekit.Interpreter[*Context]
	ctx      *Context
	registry *agent.StateRegistry
}

// NewInterpreter creates a new interpreter for the agent state machine.
func NewInterpreter(machine *statekit.MachineConfig[*Context], ctx *Context) *Interpreter {
	interp := statekit.NewInterpreter(machine)
	// Update the context reference in the machine
	interp.UpdateContext(func(c **Context) {
		*c = ctx
	})
	return &Interpreter{
		interp:   interp,
		ctx:      ctx,
		registry: agent.NewStateRegistry(),
	}
}

// NewInterpreterFromDefinition creates a new interpreter from a MachineDefinition,
// inheriting the custom state registry.
func NewInterpreterFromDefinition(def *MachineDefinition, ctx *Context) *Interpreter {
	interp := statekit.NewInterpreter(def.Config)
	interp.UpdateContext(func(c **Context) {
		*c = ctx
	})
	ctx.StateRegistry = def.StateRegistry
	return &Interpreter{
		interp:   interp,
		ctx:      ctx,
		registry: def.StateRegistry,
	}
}

// Start initializes the interpreter and enters the initial state.
func (i *Interpreter) Start() {
	i.interp.Start()
	// Sync context state with interpreter
	state := i.interp.State()
	i.ctx.Run.CurrentState = agent.State(state.Value)
	i.ctx.Run.Start()
}

// Stop stops the interpreter.
func (i *Interpreter) Stop() {
	i.interp.Stop()
}

// State returns the current state.
func (i *Interpreter) State() agent.State {
	state := i.interp.State()
	return agent.State(state.Value)
}

// Transition attempts to transition to the target state.
func (i *Interpreter) Transition(to agent.State, reason string) error {
	// Check if transition is allowed
	if !i.CanTransition(to) {
		return fmt.Errorf("transition from %s to %s not allowed", i.ctx.Run.CurrentState, to)
	}

	eventType := EventForTransition(to)
	payload := TransitionPayload{
		ToState: to,
		Reason:  reason,
	}

	event := statekit.Event{
		Type:    eventType,
		Payload: payload,
	}

	// Send the event (doesn't return error, uses panic for invalid events)
	i.interp.Send(event)

	// Update the run's current state
	newState := i.interp.State()
	i.ctx.Run.CurrentState = agent.State(newState.Value)

	return nil
}

// CanTransition checks if a transition to the target state is possible.
func (i *Interpreter) CanTransition(to agent.State) bool {
	return i.ctx.Transitions.CanTransition(i.ctx.Run.CurrentState, to)
}

// IsToolAllowed checks if a tool is allowed in the current state.
func (i *Interpreter) IsToolAllowed(toolName string) bool {
	return i.ctx.Eligibility.IsAllowed(i.ctx.Run.CurrentState, toolName)
}

// AllowedTools returns tools allowed in the current state.
func (i *Interpreter) AllowedTools() []string {
	return i.ctx.Eligibility.AllowedTools(i.ctx.Run.CurrentState)
}

// IsTerminal returns true if the interpreter is in a terminal state.
func (i *Interpreter) IsTerminal() bool {
	return i.interp.Done()
}

// Context returns the interpreter context.
func (i *Interpreter) Context() *Context {
	return i.ctx
}

// ConfigureEligibility sets tool eligibility for the interpreter.
func (i *Interpreter) ConfigureEligibility(eligibility *policy.ToolEligibility) {
	i.ctx.Eligibility = eligibility
}

// Matches checks if the current state matches the given state ID.
func (i *Interpreter) Matches(stateID string) bool {
	return i.interp.Matches(statekit.StateID(stateID))
}

// ResumeFrom restores the interpreter to a specific state.
// This is used when resuming a paused run.
func (i *Interpreter) ResumeFrom(state agent.State) error {
	// Create a snapshot with the desired state
	snapshot := statekit.Snapshot[*Context]{
		MachineID:    "agent",
		CurrentState: statekit.StateID(string(state)),
		Context:      i.ctx,
		CreatedAt:    time.Now(),
	}

	// Restore the interpreter to this state
	if err := i.interp.Restore(snapshot); err != nil {
		return fmt.Errorf("failed to restore state: %w", err)
	}

	// Sync run state
	i.ctx.Run.CurrentState = state

	return nil
}
