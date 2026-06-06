package statemachine

import (
	"go.klarlabs.de/statekit"
)

// Guard is a function that evaluates a condition and returns whether
// the guard passes along with an optional error for diagnostics.
type Guard func(ctx *Context, event statekit.Event) (bool, error)

// AndGuard returns a Guard that passes only when all provided guards pass.
// If any guard returns an error, that error is returned immediately.
// An empty guard list always passes.
func AndGuard(guards ...Guard) Guard {
	return func(ctx *Context, event statekit.Event) (bool, error) {
		for _, g := range guards {
			ok, err := g(ctx, event)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	}
}

// OrGuard returns a Guard that passes when any of the provided guards passes.
// Errors from individual guards are collected; if no guard passes and at least
// one returned an error, the last error is returned.
// An empty guard list always fails.
func OrGuard(guards ...Guard) Guard {
	return func(ctx *Context, event statekit.Event) (bool, error) {
		var lastErr error
		for _, g := range guards {
			ok, err := g(ctx, event)
			if err != nil {
				lastErr = err
				continue
			}
			if ok {
				return true, nil
			}
		}
		if lastErr != nil {
			return false, lastErr
		}
		return false, nil
	}
}

// NotGuard returns a Guard that inverts the result of the provided guard.
// Errors are passed through without inversion.
func NotGuard(guard Guard) Guard {
	return func(ctx *Context, event statekit.Event) (bool, error) {
		ok, err := guard(ctx, event)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}
}

// GuardBuilder provides a fluent interface for composing guards.
type GuardBuilder struct {
	guards []guardOp
}

type guardOpType int

const (
	guardOpAnd guardOpType = iota
	guardOpOr
	guardOpNot
)

type guardOp struct {
	typ   guardOpType
	guard Guard
}

// NewGuardBuilder creates a new guard builder.
func NewGuardBuilder() *GuardBuilder {
	return &GuardBuilder{}
}

// And adds a guard that must pass (AND semantics with previous guards).
func (b *GuardBuilder) And(g Guard) *GuardBuilder {
	b.guards = append(b.guards, guardOp{typ: guardOpAnd, guard: g})
	return b
}

// Or adds a guard using OR semantics with the previous guard.
func (b *GuardBuilder) Or(g Guard) *GuardBuilder {
	b.guards = append(b.guards, guardOp{typ: guardOpOr, guard: g})
	return b
}

// Not adds a negated guard (AND NOT semantics).
func (b *GuardBuilder) Not(g Guard) *GuardBuilder {
	b.guards = append(b.guards, guardOp{typ: guardOpNot, guard: g})
	return b
}

// Build composes all registered guards into a single Guard.
// Guards are evaluated left to right. And/Not operations are combined into
// an AND chain; Or operations create an OR branch with the preceding guard.
//
// Example: And(g1).Or(g2).Not(g3) produces: (g1 OR g2) AND (NOT g3)
func (b *GuardBuilder) Build() Guard {
	if len(b.guards) == 0 {
		return func(_ *Context, _ statekit.Event) (bool, error) {
			return true, nil
		}
	}

	// Process guards into groups separated by OR operations.
	// Each group is an AND chain; groups are combined with OR.
	var orGroups []Guard
	var currentAnd []Guard

	for _, op := range b.guards {
		switch op.typ {
		case guardOpAnd:
			currentAnd = append(currentAnd, op.guard)
		case guardOpOr:
			// Flush current AND group and start a new one
			if len(currentAnd) > 0 {
				orGroups = append(orGroups, AndGuard(currentAnd...))
				currentAnd = nil
			}
			currentAnd = append(currentAnd, op.guard)
		case guardOpNot:
			currentAnd = append(currentAnd, NotGuard(op.guard))
		}
	}

	// Flush remaining AND group
	if len(currentAnd) > 0 {
		orGroups = append(orGroups, AndGuard(currentAnd...))
	}

	if len(orGroups) == 1 {
		return orGroups[0]
	}
	return OrGuard(orGroups...)
}

// ToStatekitGuard converts a Guard (with error return) to a statekit.Guard
// that only returns bool. Errors cause the guard to return false.
func ToStatekitGuard(g Guard) statekit.Guard[*Context] {
	return func(ctx *Context, event statekit.Event) bool {
		ok, _ := g(ctx, event)
		return ok
	}
}

// FromStatekitGuard converts a statekit.Guard to a Guard with error return.
// The resulting Guard never returns an error.
func FromStatekitGuard(g statekit.Guard[*Context]) Guard {
	return func(ctx *Context, event statekit.Event) (bool, error) {
		return g(ctx, event), nil
	}
}
