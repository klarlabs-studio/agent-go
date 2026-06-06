package resilience

import "go.klarlabs.de/fortify/circuitbreaker"

// CircuitBreakerState re-exports the fortify state type so callers do not need
// to import the fortify package directly.
type CircuitBreakerState = circuitbreaker.State

// StateChangeEvent carries context about a circuit breaker state transition.
type StateChangeEvent struct {
	// From is the state the circuit breaker was in before the transition.
	From CircuitBreakerState
	// To is the state the circuit breaker transitioned to.
	To CircuitBreakerState
	// ToolName is the tool whose circuit breaker changed state, or empty for
	// the shared breaker.
	ToolName string
}

// CircuitBreakerHooks groups callbacks that fire on circuit breaker lifecycle
// events.
type CircuitBreakerHooks struct {
	// OnStateChange is called whenever the circuit breaker transitions
	// between states (closed, open, half-open).
	OnStateChange func(event StateChangeEvent)

	// OnTrip is called when the circuit breaker opens (trips) due to
	// reaching the failure threshold.
	OnTrip func(toolName string)

	// OnReset is called when the circuit breaker returns to the closed
	// state after a successful probe in half-open.
	OnReset func(toolName string)
}

// fireStateChange dispatches the OnStateChange, OnTrip, and OnReset hooks
// as appropriate for the given transition.
func (h *CircuitBreakerHooks) fireStateChange(from, to CircuitBreakerState, toolName string) {
	if h == nil {
		return
	}

	if h.OnStateChange != nil {
		h.OnStateChange(StateChangeEvent{
			From:     from,
			To:       to,
			ToolName: toolName,
		})
	}

	// Detect trip: any state -> open.
	if to == circuitbreaker.StateOpen && h.OnTrip != nil {
		h.OnTrip(toolName)
	}

	// Detect reset: half-open -> closed.
	if from == circuitbreaker.StateHalfOpen && to == circuitbreaker.StateClosed && h.OnReset != nil {
		h.OnReset(toolName)
	}
}

// --- Functional options for hooks ---

// WithCircuitBreakerHooks registers lifecycle hooks for the circuit breaker.
func WithCircuitBreakerHooks(hooks CircuitBreakerHooks) Option {
	return func(c *ExecutorConfig) {
		c.CBHooks = &hooks
	}
}

// WithOnStateChange registers a callback invoked on every circuit breaker
// state transition.
func WithOnStateChange(fn func(event StateChangeEvent)) Option {
	return func(c *ExecutorConfig) {
		if c.CBHooks == nil {
			c.CBHooks = &CircuitBreakerHooks{}
		}
		c.CBHooks.OnStateChange = fn
	}
}

// WithOnTrip registers a callback invoked when the circuit breaker opens.
func WithOnTrip(fn func(toolName string)) Option {
	return func(c *ExecutorConfig) {
		if c.CBHooks == nil {
			c.CBHooks = &CircuitBreakerHooks{}
		}
		c.CBHooks.OnTrip = fn
	}
}

// WithOnReset registers a callback invoked when the circuit breaker closes
// after a successful half-open probe.
func WithOnReset(fn func(toolName string)) Option {
	return func(c *ExecutorConfig) {
		if c.CBHooks == nil {
			c.CBHooks = &CircuitBreakerHooks{}
		}
		c.CBHooks.OnReset = fn
	}
}
