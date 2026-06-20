package aiplugin

import (
	"sync/atomic"

	"go.klarlabs.de/statekit/plugin"
)

// TransitionBudget caps the number of transitions an interpreter is
// allowed to perform. When the budget is exhausted, subsequent events
// are rewritten to a configurable halt-event type, signalling
// runaway prevention without forcing the machine into a bespoke
// error state.
//
// Why this exists: a recurring pain in incumbent Go FSM libraries
// (e.g. https://github.com/qmuntal/stateless/issues/77) is the
// inability to halt execution after N transitions. Wire a
// TransitionBudget into the plugin slot and your machine gets a
// bounded-blast-radius guarantee out of the box.
//
// Usage:
//
//	budget := aiplugin.NewTransitionBudget[Ctx](100, "BUDGET_EXHAUSTED")
//	interp.Use(budget)
//	// Add a transition for "BUDGET_EXHAUSTED" that routes to a
//	// halted/failed state from anywhere it can fire.
//
// TransitionBudget tracks transitions via the AfterTransition hook,
// not via OnEvent — this counts only events that actually changed
// state, matching the qmuntal-issue's "transition count" semantics.
// OnEvent rewrites the event type once the budget is exhausted so
// the machine can route to its halt state on the same Send call.
type TransitionBudget[C any] struct {
	limit       int64
	haltEvent   plugin.EventType
	transitions atomic.Int64
}

// NewTransitionBudget constructs a TransitionBudget[C] capped at
// `limit` transitions. Once the budget is reached, OnEvent rewrites
// every subsequent event to `haltEvent`.
func NewTransitionBudget[C any](limit int64, haltEvent plugin.EventType) *TransitionBudget[C] {
	return &TransitionBudget[C]{
		limit:     limit,
		haltEvent: haltEvent,
	}
}

// Name implements plugin.Plugin.
func (*TransitionBudget[C]) Name() string { return "ai-transition-budget" }

// AfterTransition implements plugin.OnTransitionHook. Every observed
// transition increments the counter.
func (b *TransitionBudget[C]) AfterTransition(_ plugin.Context[C], from, to plugin.StateID, _ plugin.Event) {
	if from == to {
		// Internal/no-op transitions don't count against the budget;
		// XState semantics: only state-changing transitions tick.
		return
	}
	b.transitions.Add(1)
}

// BeforeTransition is required to satisfy plugin.OnTransitionHook
// (paired with AfterTransition). No-op.
func (*TransitionBudget[C]) BeforeTransition(_ plugin.Context[C], _, _ plugin.StateID, _ plugin.Event) {
}

// OnEvent implements plugin.OnEventHook. Once the budget is
// exhausted, the event type is rewritten to the halt event.
func (b *TransitionBudget[C]) OnEvent(_ plugin.Context[C], event plugin.Event) plugin.Event {
	if b.Exhausted() {
		// Preserve payload — caller can read why the halt fired.
		return plugin.Event{Type: b.haltEvent, Payload: event.Payload}
	}
	return event
}

// Used returns the running transition count.
func (b *TransitionBudget[C]) Used() int64 { return b.transitions.Load() }

// Remaining returns how many transitions are still allowed before
// the halt event is forced. A negative value means the budget has
// already been overshot (race conditions on counter increments are
// possible but bounded by the number of in-flight events).
func (b *TransitionBudget[C]) Remaining() int64 {
	return b.limit - b.transitions.Load()
}

// Exhausted reports whether the budget has been reached.
func (b *TransitionBudget[C]) Exhausted() bool {
	return b.transitions.Load() >= b.limit
}

// Reset zeroes the transition counter. Useful between independent
// runs of the same interpreter, or after a manual recovery flow.
func (b *TransitionBudget[C]) Reset() {
	b.transitions.Store(0)
}
