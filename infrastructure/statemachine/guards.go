package statemachine

import (
	"strings"

	"github.com/felixgeelhaar/statekit"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// guardCanTransition checks if the transition is valid according to policy.
// Note: In statekit, guards receive the context by value. Since our context is *Context,
// the guard receives *Context directly.
func guardCanTransition(ctx *Context, event statekit.Event) bool {
	if ctx == nil || ctx.Run == nil || ctx.Transitions == nil {
		return false
	}

	fromState := ctx.Run.CurrentState

	// Get target state from the event payload if available
	var toState agent.State
	if payload, ok := event.Payload.(TransitionPayload); ok {
		toState = payload.ToState
	} else {
		// Fall back to deriving from event type
		toState = stateFromEventType(event.Type)
	}

	return ctx.Transitions.CanTransition(fromState, toState)
}

// guardBudgetAvailable checks if there is budget available.
func guardBudgetAvailable(ctx *Context, _ statekit.Event) bool {
	if ctx == nil || ctx.Budget == nil {
		return true // No budget means unlimited
	}

	return !ctx.Budget.IsExhausted()
}

// guardToolAllowed checks if a specific tool is allowed in the current state.
func guardToolAllowed(ctx *Context, toolName string) bool {
	if ctx == nil || ctx.Run == nil || ctx.Eligibility == nil {
		return false
	}

	return ctx.Eligibility.IsAllowed(ctx.Run.CurrentState, toolName)
}

// GuardToolAllowedFunc returns a guard function for a specific tool.
func GuardToolAllowedFunc(toolName string) statekit.Guard[*Context] {
	return func(ctx *Context, _ statekit.Event) bool {
		return guardToolAllowed(ctx, toolName)
	}
}

// stateFromEventType derives the target state from an event type.
// For canonical events, the well-known state is returned.
// For custom events (which are uppercase by convention), the lowercase
// form is returned to match custom state names.
func stateFromEventType(eventType statekit.EventType) agent.State {
	switch eventType {
	case "EXPLORE":
		return agent.StateExplore
	case "DECIDE":
		return agent.StateDecide
	case "ACT":
		return agent.StateAct
	case "VALIDATE":
		return agent.StateValidate
	case "DONE":
		return agent.StateDone
	case "FAIL":
		return agent.StateFailed
	default:
		return agent.State(strings.ToLower(string(eventType)))
	}
}
