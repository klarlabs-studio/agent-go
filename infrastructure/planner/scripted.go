package planner

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// ScriptStep defines an expected state and the decision to return.
type ScriptStep struct {
	// ExpectState asserts we're in this state before returning the decision.
	ExpectState agent.State

	// Decision is the decision to return.
	Decision agent.Decision

	// Condition is an optional additional condition that must be true.
	Condition func(PlanRequest) bool

	// Skip causes the step to be skipped (not consumed) if the condition returns false.
	// Unlike the default behavior where a failed Condition returns an error,
	// when Skip is true, the planner moves to the next step silently.
	// Requires Condition to be set.
	Skip bool

	// Error injects a planner error for testing error handling paths.
	// When set, this error is returned instead of the decision.
	Error error
}

// ScriptedPlanner executes a predefined sequence for deterministic testing.
// It validates that the agent is in the expected state before returning decisions.
//
// Enhanced features:
//   - Conditional steps: Set Skip=true with a Condition to skip non-matching steps
//   - Error injection: Set Error on a step to simulate planner failures
//   - Looping: Use WithLoop to repeat the entire sequence multiple times
type ScriptedPlanner struct {
	steps        []ScriptStep
	index        int
	onUnexpected func(PlanRequest) agent.Decision
	mu           sync.Mutex

	// Loop support
	loopCount int // 0 = no looping, >0 = total iterations (including first)
	loopIndex int // current loop iteration (0-based)
	loopUntil func(PlanRequest) bool
}

// NewScriptedPlanner creates a scripted planner with the given steps.
func NewScriptedPlanner(steps ...ScriptStep) *ScriptedPlanner {
	return &ScriptedPlanner{
		steps: steps,
		index: 0,
		onUnexpected: func(_ PlanRequest) agent.Decision {
			return agent.NewFailDecision("unexpected state", errors.New("script exhausted"))
		},
	}
}

// OnUnexpected sets the handler for unexpected states.
func (p *ScriptedPlanner) OnUnexpected(handler func(PlanRequest) agent.Decision) *ScriptedPlanner {
	p.onUnexpected = handler
	return p
}

// WithLoop configures the planner to repeat its step sequence n times.
// After n iterations, the onUnexpected handler is called.
// Pass 0 to disable looping (default).
func (p *ScriptedPlanner) WithLoop(n int) *ScriptedPlanner {
	p.loopCount = n
	return p
}

// WithLoopUntil configures the planner to repeat its step sequence until
// the given condition returns true. The condition is checked at the end of
// each complete iteration. This takes precedence over WithLoop count.
func (p *ScriptedPlanner) WithLoopUntil(condition func(PlanRequest) bool) *ScriptedPlanner {
	p.loopUntil = condition
	return p
}

// Plan returns the next decision if the state matches expectations.
func (p *ScriptedPlanner) Plan(_ context.Context, req PlanRequest) (agent.Decision, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.planLocked(req)
}

// planLocked is the internal plan implementation (caller must hold mutex).
func (p *ScriptedPlanner) planLocked(req PlanRequest) (agent.Decision, error) {
	// Handle end of steps: check looping
	if p.index >= len(p.steps) {
		if p.shouldLoop(req) {
			p.index = 0
			p.loopIndex++
		} else {
			return p.onUnexpected(req), nil
		}
	}

	// Scan forward through skippable steps
	for p.index < len(p.steps) {
		step := p.steps[p.index]

		// Check skip condition
		if step.Skip && step.Condition != nil && !step.Condition(req) {
			p.index++
			// If we ran out of steps during skip scan, check looping
			if p.index >= len(p.steps) {
				if p.shouldLoop(req) {
					p.index = 0
					p.loopIndex++
				} else {
					return p.onUnexpected(req), nil
				}
			}
			continue
		}

		// Validate expected state
		if step.ExpectState != "" && step.ExpectState != req.CurrentState {
			return agent.Decision{}, &UnexpectedStateError{
				Expected:  step.ExpectState,
				Actual:    req.CurrentState,
				StepIndex: p.index,
			}
		}

		// Validate non-skip condition
		if !step.Skip && step.Condition != nil && !step.Condition(req) {
			return agent.Decision{}, &ConditionFailedError{
				StepIndex: p.index,
				State:     req.CurrentState,
			}
		}

		// Check error injection
		if step.Error != nil {
			p.index++
			return agent.Decision{}, step.Error
		}

		p.index++
		return step.Decision, nil
	}

	return p.onUnexpected(req), nil
}

// shouldLoop returns true if the planner should restart its step sequence.
func (p *ScriptedPlanner) shouldLoop(req PlanRequest) bool {
	// Condition-based looping takes precedence
	if p.loopUntil != nil {
		return !p.loopUntil(req)
	}

	// Count-based looping
	if p.loopCount > 0 {
		return p.loopIndex+1 < p.loopCount
	}

	return false
}

// LoopIteration returns the current loop iteration (0-based).
func (p *ScriptedPlanner) LoopIteration() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.loopIndex
}

// Reset resets the planner to the beginning.
func (p *ScriptedPlanner) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.index = 0
}

// CurrentStep returns the current step index.
func (p *ScriptedPlanner) CurrentStep() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.index
}

// IsComplete returns true if all steps have been executed.
func (p *ScriptedPlanner) IsComplete() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.index >= len(p.steps)
}

// UnexpectedStateError indicates the planner received an unexpected state.
type UnexpectedStateError struct {
	Expected  agent.State
	Actual    agent.State
	StepIndex int
}

func (e *UnexpectedStateError) Error() string {
	return fmt.Sprintf("unexpected state at step %d: expected %s, got %s", e.StepIndex, e.Expected, e.Actual)
}

// ConditionFailedError indicates a step condition was not met.
type ConditionFailedError struct {
	StepIndex int
	State     agent.State
}

func (e *ConditionFailedError) Error() string {
	return fmt.Sprintf("condition failed at step %d in state %s", e.StepIndex, e.State)
}
