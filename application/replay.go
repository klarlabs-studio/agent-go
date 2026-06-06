package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
)

// Replay provides event replay capabilities.
type Replay struct {
	eventStore event.Store
}

// NewReplay creates a new replay engine.
func NewReplay(eventStore event.Store) *Replay {
	return &Replay{
		eventStore: eventStore,
	}
}

// ReconstructRun rebuilds a run's state from its event history.
func (r *Replay) ReconstructRun(ctx context.Context, runID string) (*agent.Run, error) {
	events, err := r.eventStore.LoadEvents(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}

	if len(events) == 0 {
		return nil, event.ErrRunNotFound
	}

	return r.applyEvents(events)
}

// ReconstructRunFrom rebuilds a run's state from a starting sequence.
func (r *Replay) ReconstructRunFrom(ctx context.Context, runID string, fromSeq uint64) (*agent.Run, error) {
	events, err := r.eventStore.LoadEventsFrom(ctx, runID, fromSeq)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}

	if len(events) == 0 {
		return nil, event.ErrRunNotFound
	}

	return r.applyEvents(events)
}

// applyEvents applies a sequence of events to build run state.
func (r *Replay) applyEvents(events []event.Event) (*agent.Run, error) {
	if len(events) == 0 {
		return nil, event.ErrRunNotFound
	}

	var run *agent.Run

	for _, e := range events {
		switch e.Type {
		case event.TypeRunStarted:
			var payload event.RunStartedPayload
			if err := e.UnmarshalPayload(&payload); err != nil {
				return nil, fmt.Errorf("unmarshal run.started: %w", err)
			}
			run = agent.NewRun(e.RunID, payload.Goal)
			run.Vars = payload.Vars
			if run.Vars == nil {
				run.Vars = make(map[string]any)
			}
			run.StartTime = e.Timestamp

		case event.TypeRunCompleted:
			if run == nil {
				continue
			}
			var payload event.RunCompletedPayload
			if err := e.UnmarshalPayload(&payload); err != nil {
				return nil, fmt.Errorf("unmarshal run.completed: %w", err)
			}
			run.Complete(payload.Result)
			run.EndTime = e.Timestamp

		case event.TypeRunFailed:
			if run == nil {
				continue
			}
			var payload event.RunFailedPayload
			if err := e.UnmarshalPayload(&payload); err != nil {
				return nil, fmt.Errorf("unmarshal run.failed: %w", err)
			}
			run.Fail(payload.Error)
			run.EndTime = e.Timestamp

		case event.TypeRunPaused:
			if run == nil {
				continue
			}
			run.Pause()

		case event.TypeRunResumed:
			if run == nil {
				continue
			}
			run.Resume()

		case event.TypeStateTransitioned:
			if run == nil {
				continue
			}
			var payload event.StateTransitionedPayload
			if err := e.UnmarshalPayload(&payload); err != nil {
				return nil, fmt.Errorf("unmarshal state.transitioned: %w", err)
			}
			run.TransitionTo(payload.ToState)

		case event.TypeEvidenceAdded:
			if run == nil {
				continue
			}
			var payload event.EvidenceAddedPayload
			if err := e.UnmarshalPayload(&payload); err != nil {
				return nil, fmt.Errorf("unmarshal evidence.added: %w", err)
			}
			run.AddEvidence(agent.Evidence{
				Type:      agent.EvidenceType(payload.Type),
				Source:    payload.Source,
				Content:   payload.Content,
				Timestamp: e.Timestamp,
			})

		case event.TypeVariableSet:
			if run == nil {
				continue
			}
			var payload event.VariableSetPayload
			if err := e.UnmarshalPayload(&payload); err != nil {
				return nil, fmt.Errorf("unmarshal variable.set: %w", err)
			}
			run.SetVar(payload.Key, payload.Value)

		// Tool events don't directly modify run state but can be tracked
		case event.TypeToolCalled, event.TypeToolSucceeded, event.TypeToolFailed:
			// These events are for audit/analytics, not state reconstruction

		// Decision and approval events are also for audit
		case event.TypeDecisionMade, event.TypeApprovalRequested,
			event.TypeApprovalGranted, event.TypeApprovalDenied:
			// Audit events

		// Budget events
		case event.TypeBudgetConsumed, event.TypeBudgetExhausted:
			// Budget tracking events
		}
	}

	if run == nil {
		return nil, event.ErrRunNotFound
	}

	return run, nil
}

// EventIterator allows iterating over events one at a time.
type EventIterator struct {
	events []event.Event
	index  int
}

// NewEventIterator creates an iterator over events.
func (r *Replay) NewEventIterator(ctx context.Context, runID string) (*EventIterator, error) {
	events, err := r.eventStore.LoadEvents(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}

	return &EventIterator{
		events: events,
		index:  0,
	}, nil
}

// Next returns the next event, or nil if done.
func (it *EventIterator) Next() *event.Event {
	if it.index >= len(it.events) {
		return nil
	}
	e := &it.events[it.index]
	it.index++
	return e
}

// Peek returns the next event without advancing.
func (it *EventIterator) Peek() *event.Event {
	if it.index >= len(it.events) {
		return nil
	}
	return &it.events[it.index]
}

// Reset returns to the beginning.
func (it *EventIterator) Reset() {
	it.index = 0
}

// Len returns the total number of events.
func (it *EventIterator) Len() int {
	return len(it.events)
}

// Index returns the current position.
func (it *EventIterator) Index() int {
	return it.index
}

// Timeline provides a time-based view of events.
type Timeline struct {
	events []event.Event
}

// NewTimeline creates a timeline from events.
func (r *Replay) NewTimeline(ctx context.Context, runID string) (*Timeline, error) {
	events, err := r.eventStore.LoadEvents(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}

	return &Timeline{events: events}, nil
}

// Duration returns the total duration of the run.
func (tl *Timeline) Duration() time.Duration {
	if len(tl.events) < 2 {
		return 0
	}
	first := tl.events[0].Timestamp
	last := tl.events[len(tl.events)-1].Timestamp
	return last.Sub(first)
}

// EventsInRange returns events within a time range.
func (tl *Timeline) EventsInRange(from, to time.Time) []event.Event {
	var result []event.Event
	for _, e := range tl.events {
		if (from.IsZero() || !e.Timestamp.Before(from)) &&
			(to.IsZero() || !e.Timestamp.After(to)) {
			result = append(result, e)
		}
	}
	return result
}

// EventsByType returns events of a specific type.
func (tl *Timeline) EventsByType(eventType event.Type) []event.Event {
	var result []event.Event
	for _, e := range tl.events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// StateTransitions returns all state transition events.
func (tl *Timeline) StateTransitions() []StateTransition {
	var transitions []StateTransition
	for _, e := range tl.events {
		if e.Type == event.TypeStateTransitioned {
			var payload event.StateTransitionedPayload
			if err := json.Unmarshal(e.Payload, &payload); err == nil {
				transitions = append(transitions, StateTransition{
					From:      payload.FromState,
					To:        payload.ToState,
					Reason:    payload.Reason,
					Timestamp: e.Timestamp,
				})
			}
		}
	}
	return transitions
}

// StateTransition represents a state change.
type StateTransition struct {
	From      agent.State
	To        agent.State
	Reason    string
	Timestamp time.Time
}

// ToolCalls returns all tool call events with their results.
func (tl *Timeline) ToolCalls() []ToolCall {
	// Map of tool call sequence to call info
	calls := make(map[uint64]*ToolCall)

	for _, e := range tl.events {
		switch e.Type {
		case event.TypeToolCalled:
			var payload event.ToolCalledPayload
			if err := json.Unmarshal(e.Payload, &payload); err == nil {
				calls[e.Sequence] = &ToolCall{
					ToolName:  payload.ToolName,
					Input:     payload.Input,
					StartTime: e.Timestamp,
					State:     payload.State,
				}
			}

		case event.TypeToolSucceeded:
			var payload event.ToolSucceededPayload
			if err := json.Unmarshal(e.Payload, &payload); err == nil {
				// Find matching call by tool name
				for _, call := range calls {
					if call.ToolName == payload.ToolName && call.Output == nil {
						call.Output = payload.Output
						call.Duration = payload.Duration
						call.Cached = payload.Cached
						call.Success = true
						break
					}
				}
			}

		case event.TypeToolFailed:
			var payload event.ToolFailedPayload
			if err := json.Unmarshal(e.Payload, &payload); err == nil {
				// Find matching call by tool name
				for _, call := range calls {
					if call.ToolName == payload.ToolName && !call.Success && call.Error == "" {
						call.Error = payload.Error
						call.Duration = payload.Duration
						call.Success = false
						break
					}
				}
			}
		}
	}

	// Convert map to slice
	var result []ToolCall
	for _, call := range calls {
		result = append(result, *call)
	}
	return result
}

// ToolCall represents a tool invocation with its result.
type ToolCall struct {
	ToolName  string
	Input     json.RawMessage
	Output    json.RawMessage
	StartTime time.Time
	Duration  time.Duration
	State     agent.State
	Success   bool
	Cached    bool
	Error     string
}
