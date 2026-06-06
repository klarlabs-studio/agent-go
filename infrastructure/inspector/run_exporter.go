// Package inspector provides inspector infrastructure implementations.
package inspector

import (
	"context"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/inspector"
	"go.klarlabs.de/agent/domain/run"
)

// RunExporter exports run data.
type RunExporter struct {
	runStore   run.Store
	eventStore event.Store
}

// NewRunExporter creates a new run exporter.
func NewRunExporter(runStore run.Store, eventStore event.Store) *RunExporter {
	return &RunExporter{
		runStore:   runStore,
		eventStore: eventStore,
	}
}

// Export exports run data.
func (e *RunExporter) Export(ctx context.Context, runID string) (*inspector.RunExport, error) {
	// Get run
	r, err := e.runStore.Get(ctx, runID)
	if err != nil {
		return nil, inspector.ErrRunNotFound
	}

	// Get events
	events, err := e.eventStore.LoadEvents(ctx, runID)
	if err != nil {
		events = []event.Event{} // Continue with empty events
	}

	// Build export
	metadata := inspector.RunMetadata{
		ID:        r.ID,
		Goal:      r.Goal,
		Status:    r.Status,
		State:     r.CurrentState,
		StartTime: r.StartTime,
		Error:     r.Error,
	}

	if !r.EndTime.IsZero() {
		metadata.EndTime = &r.EndTime
	}

	if len(r.Result) > 0 {
		metadata.Result = string(r.Result)
	}

	export := &inspector.RunExport{
		Run:         metadata,
		Timeline:    buildTimeline(events),
		ToolCalls:   buildToolCalls(events),
		Transitions: buildTransitions(events),
		Metrics:     buildRunMetrics(r, events),
	}

	return export, nil
}

func buildTimeline(events []event.Event) []inspector.TimelineEntry {
	var timeline []inspector.TimelineEntry
	var prevTime time.Time

	for _, e := range events {
		entry := inspector.TimelineEntry{
			Timestamp: e.Timestamp,
			Type:      string(e.Type),
			Label:     getEventLabel(e),
			State:     getEventState(e),
		}

		if !prevTime.IsZero() {
			entry.Duration = e.Timestamp.Sub(prevTime)
		}

		entry.Details = getEventDetails(e)
		timeline = append(timeline, entry)
		prevTime = e.Timestamp
	}

	return timeline
}

func buildToolCalls(events []event.Event) []inspector.ToolCallExport {
	var toolCalls []inspector.ToolCallExport

	// Track pending tool calls
	pendingCalls := make(map[string]struct {
		timestamp time.Time
		state     agent.State
		input     string
	})

	for _, e := range events {
		switch e.Type {
		case event.TypeToolCalled:
			var payload event.ToolCalledPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				pendingCalls[payload.ToolName] = struct {
					timestamp time.Time
					state     agent.State
					input     string
				}{
					timestamp: e.Timestamp,
					state:     payload.State,
				}
			}

		case event.TypeToolSucceeded:
			var payload event.ToolSucceededPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				if pending, ok := pendingCalls[payload.ToolName]; ok {
					toolCalls = append(toolCalls, inspector.ToolCallExport{
						Name:      payload.ToolName,
						Timestamp: pending.timestamp,
						State:     pending.state,
						Duration:  payload.Duration,
						Success:   true,
						Input:     pending.input,
					})
					delete(pendingCalls, payload.ToolName)
				}
			}

		case event.TypeToolFailed:
			var payload event.ToolFailedPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				if pending, ok := pendingCalls[payload.ToolName]; ok {
					toolCalls = append(toolCalls, inspector.ToolCallExport{
						Name:      payload.ToolName,
						Timestamp: pending.timestamp,
						State:     pending.state,
						Duration:  payload.Duration,
						Success:   false,
						Input:     pending.input,
						Error:     payload.Error,
					})
					delete(pendingCalls, payload.ToolName)
				}
			}
		}
	}

	return toolCalls
}

func buildTransitions(events []event.Event) []inspector.TransitionExport {
	var transitions []inspector.TransitionExport
	var lastState agent.State
	var lastTransitionTime time.Time

	for _, e := range events {
		if e.Type == event.TypeStateTransitioned {
			var payload event.StateTransitionedPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				trans := inspector.TransitionExport{
					Timestamp: e.Timestamp,
					From:      payload.FromState,
					To:        payload.ToState,
					Reason:    payload.Reason,
				}

				if lastState != "" && !lastTransitionTime.IsZero() {
					trans.Duration = e.Timestamp.Sub(lastTransitionTime)
				}

				transitions = append(transitions, trans)
				lastState = payload.ToState
				lastTransitionTime = e.Timestamp
			}
		}
	}

	return transitions
}

func buildRunMetrics(r *agent.Run, events []event.Event) inspector.RunMetrics {
	metrics := inspector.RunMetrics{
		TimeInState: make(map[agent.State]time.Duration),
		ToolUsage:   make(map[string]int),
	}

	// Calculate total duration
	if !r.EndTime.IsZero() {
		metrics.TotalDuration = r.EndTime.Sub(r.StartTime)
	} else {
		metrics.TotalDuration = time.Since(r.StartTime)
	}

	// Count tool calls and calculate stats
	var totalToolDuration time.Duration
	var lastStateTime time.Time
	var lastState agent.State

	for _, e := range events {
		switch e.Type {
		case event.TypeToolCalled:
			metrics.ToolCallCount++
			var payload event.ToolCalledPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				metrics.ToolUsage[payload.ToolName]++
			}

		case event.TypeToolSucceeded:
			metrics.SuccessfulToolCalls++
			var payload event.ToolSucceededPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				totalToolDuration += payload.Duration
			}

		case event.TypeToolFailed:
			metrics.FailedToolCalls++
			var payload event.ToolFailedPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				totalToolDuration += payload.Duration
			}

		case event.TypeStateTransitioned:
			metrics.TransitionCount++
			var payload event.StateTransitionedPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				if lastState != "" && !lastStateTime.IsZero() {
					metrics.TimeInState[lastState] += e.Timestamp.Sub(lastStateTime)
				}
				lastState = payload.ToState
				lastStateTime = e.Timestamp
			}
		}
	}

	// Calculate average tool duration
	if metrics.ToolCallCount > 0 {
		metrics.AverageToolDuration = totalToolDuration / time.Duration(metrics.ToolCallCount)
	}

	return metrics
}

func getEventLabel(e event.Event) string {
	switch e.Type {
	case event.TypeRunStarted:
		return "Run Started"
	case event.TypeRunCompleted:
		return "Run Completed"
	case event.TypeRunFailed:
		return "Run Failed"
	case event.TypeStateTransitioned:
		var payload event.StateTransitionedPayload
		if err := e.UnmarshalPayload(&payload); err == nil {
			return "State: " + string(payload.ToState)
		}
		return "State Changed"
	case event.TypeToolCalled:
		var payload event.ToolCalledPayload
		if err := e.UnmarshalPayload(&payload); err == nil {
			return "Tool: " + payload.ToolName
		}
		return "Tool Called"
	case event.TypeToolSucceeded:
		return "Tool Succeeded"
	case event.TypeToolFailed:
		return "Tool Failed"
	default:
		return string(e.Type)
	}
}

func getEventState(e event.Event) agent.State {
	switch e.Type {
	case event.TypeStateTransitioned:
		var payload event.StateTransitionedPayload
		if err := e.UnmarshalPayload(&payload); err == nil {
			return payload.ToState
		}
	case event.TypeToolCalled:
		var payload event.ToolCalledPayload
		if err := e.UnmarshalPayload(&payload); err == nil {
			return payload.State
		}
	}
	return ""
}

func getEventDetails(e event.Event) map[string]any {
	details := make(map[string]any)

	switch e.Type {
	case event.TypeToolSucceeded:
		var payload event.ToolSucceededPayload
		if err := e.UnmarshalPayload(&payload); err == nil {
			details["tool"] = payload.ToolName
			details["duration"] = payload.Duration.String()
		}
	case event.TypeToolFailed:
		var payload event.ToolFailedPayload
		if err := e.UnmarshalPayload(&payload); err == nil {
			details["tool"] = payload.ToolName
			details["error"] = payload.Error
			details["duration"] = payload.Duration.String()
		}
	case event.TypeStateTransitioned:
		var payload event.StateTransitionedPayload
		if err := e.UnmarshalPayload(&payload); err == nil {
			details["from"] = payload.FromState
			details["to"] = payload.ToState
			details["reason"] = payload.Reason
		}
	}

	if len(details) == 0 {
		return nil
	}
	return details
}

// Ensure RunExporter implements inspector.RunExporter
var _ inspector.RunExporter = (*RunExporter)(nil)
