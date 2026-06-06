// Package analytics provides analytics aggregation implementation.
package analytics

import (
	"context"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/analytics"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/run"
)

// Aggregator computes analytics from run and event stores.
type Aggregator struct {
	runStore   run.Store
	eventStore event.Store
}

// NewAggregator creates a new analytics aggregator.
func NewAggregator(runStore run.Store, eventStore event.Store) *Aggregator {
	return &Aggregator{
		runStore:   runStore,
		eventStore: eventStore,
	}
}

// ToolUsage returns tool usage statistics.
func (a *Aggregator) ToolUsage(ctx context.Context, filter analytics.Filter) ([]analytics.ToolUsageStat, error) {
	// Get runs matching filter
	runs, err := a.getRuns(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Aggregate tool stats
	stats := make(map[string]*analytics.ToolUsageStat)

	for _, r := range runs {
		events, err := a.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		// Track tool calls
		toolCalls := make(map[string]time.Time) // toolName -> start time

		for _, e := range events {
			switch e.Type {
			case event.TypeToolCalled:
				var payload event.ToolCalledPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					// Check tool name filter
					if len(filter.ToolNames) > 0 && !contains(filter.ToolNames, payload.ToolName) {
						continue
					}

					if _, ok := stats[payload.ToolName]; !ok {
						stats[payload.ToolName] = &analytics.ToolUsageStat{
							ToolName:    payload.ToolName,
							MinDuration: time.Duration(1<<63 - 1), // Max duration
						}
					}
					stats[payload.ToolName].CallCount++
					toolCalls[payload.ToolName] = e.Timestamp
				}

			case event.TypeToolSucceeded:
				var payload event.ToolSucceededPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if stat, ok := stats[payload.ToolName]; ok {
						stat.SuccessCount++
						stat.TotalDuration += payload.Duration
						if payload.Duration < stat.MinDuration {
							stat.MinDuration = payload.Duration
						}
						if payload.Duration > stat.MaxDuration {
							stat.MaxDuration = payload.Duration
						}
						if payload.Cached {
							stat.CacheHitCount++
						}
					}
					delete(toolCalls, payload.ToolName)
				}

			case event.TypeToolFailed:
				var payload event.ToolFailedPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if stat, ok := stats[payload.ToolName]; ok {
						stat.FailureCount++
						stat.TotalDuration += payload.Duration
						if payload.Duration < stat.MinDuration {
							stat.MinDuration = payload.Duration
						}
						if payload.Duration > stat.MaxDuration {
							stat.MaxDuration = payload.Duration
						}
					}
					delete(toolCalls, payload.ToolName)
				}
			}
		}
	}

	// Calculate averages and convert to slice
	result := make([]analytics.ToolUsageStat, 0, len(stats))
	for _, stat := range stats {
		if stat.CallCount > 0 {
			stat.AverageDuration = stat.TotalDuration / time.Duration(stat.CallCount)
		}
		if stat.MinDuration == time.Duration(1<<63-1) {
			stat.MinDuration = 0
		}
		result = append(result, *stat)
	}

	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

// StateDistribution returns time spent in each state.
func (a *Aggregator) StateDistribution(ctx context.Context, filter analytics.Filter) ([]analytics.StateTimeStat, error) {
	runs, err := a.getRuns(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Aggregate state stats
	stats := make(map[agent.State]*analytics.StateTimeStat)
	var totalTime time.Duration

	for _, r := range runs {
		events, err := a.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		var lastState agent.State
		var lastTime time.Time

		for _, e := range events {
			if e.Type == event.TypeStateTransitioned {
				var payload event.StateTransitionedPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					// Check state filter
					if len(filter.States) > 0 && !containsState(filter.States, payload.FromState) {
						continue
					}

					// Calculate time in previous state
					if !lastTime.IsZero() {
						duration := e.Timestamp.Sub(lastTime)
						if stat, ok := stats[lastState]; ok {
							stat.TotalTime += duration
						} else {
							stats[lastState] = &analytics.StateTimeStat{
								State:      lastState,
								TotalTime:  duration,
								EntryCount: 1,
							}
						}
						totalTime += duration
					}

					// Track new state
					if stat, ok := stats[payload.ToState]; ok {
						stat.EntryCount++
					} else {
						stats[payload.ToState] = &analytics.StateTimeStat{
							State:      payload.ToState,
							EntryCount: 1,
						}
					}

					lastState = payload.ToState
					lastTime = e.Timestamp
				}
			}
		}
	}

	// Calculate averages and percentages
	result := make([]analytics.StateTimeStat, 0, len(stats))
	for _, stat := range stats {
		if stat.EntryCount > 0 {
			stat.AverageTime = stat.TotalTime / time.Duration(stat.EntryCount)
		}
		if totalTime > 0 {
			stat.Percentage = float64(stat.TotalTime) / float64(totalTime) * 100
		}
		result = append(result, *stat)
	}

	return result, nil
}

// SuccessRate returns the success rate for runs.
func (a *Aggregator) SuccessRate(ctx context.Context, filter analytics.Filter) (analytics.SuccessRateStat, error) {
	runs, err := a.getRuns(ctx, filter)
	if err != nil {
		return analytics.SuccessRateStat{}, err
	}

	var stat analytics.SuccessRateStat
	var totalDuration time.Duration

	for _, r := range runs {
		stat.TotalRuns++
		switch r.Status {
		case agent.RunStatusCompleted:
			stat.CompletedRuns++
			totalDuration += r.Duration()
		case agent.RunStatusFailed:
			stat.FailedRuns++
			totalDuration += r.Duration()
		}
	}

	if stat.TotalRuns > 0 {
		stat.SuccessRate = float64(stat.CompletedRuns) / float64(stat.TotalRuns) * 100
		if stat.CompletedRuns+stat.FailedRuns > 0 {
			stat.AverageDuration = totalDuration / time.Duration(stat.CompletedRuns+stat.FailedRuns)
		}
	}

	return stat, nil
}

// RunSummary returns summary statistics for runs.
func (a *Aggregator) RunSummary(ctx context.Context, filter analytics.Filter) (analytics.RunSummary, error) {
	runs, err := a.getRuns(ctx, filter)
	if err != nil {
		return analytics.RunSummary{}, err
	}

	var summary analytics.RunSummary

	for _, r := range runs {
		summary.TotalRuns++
		summary.TotalDuration += r.Duration()

		switch r.Status {
		case agent.RunStatusCompleted:
			summary.CompletedRuns++
		case agent.RunStatusFailed:
			summary.FailedRuns++
		case agent.RunStatusRunning:
			summary.RunningRuns++
		}

		// Count tool calls from events
		events, err := a.eventStore.LoadEvents(ctx, r.ID)
		if err == nil {
			for _, e := range events {
				if e.Type == event.TypeToolCalled {
					summary.TotalToolCalls++
				}
			}
		}
	}

	if summary.TotalRuns > 0 {
		summary.AverageDuration = summary.TotalDuration / time.Duration(summary.TotalRuns)
		summary.AverageToolCallsPerRun = float64(summary.TotalToolCalls) / float64(summary.TotalRuns)
	}

	return summary, nil
}

// getRuns retrieves runs matching the filter.
func (a *Aggregator) getRuns(ctx context.Context, filter analytics.Filter) ([]*agent.Run, error) {
	listFilter := run.ListFilter{
		FromTime: filter.FromTime,
		ToTime:   filter.ToTime,
	}

	// Convert states filter
	if len(filter.States) > 0 {
		listFilter.States = filter.States
	}

	runs, err := a.runStore.List(ctx, listFilter)
	if err != nil {
		return nil, err
	}

	// Filter by run IDs if specified
	if len(filter.RunIDs) > 0 {
		filtered := make([]*agent.Run, 0)
		for _, r := range runs {
			if contains(filter.RunIDs, r.ID) {
				filtered = append(filtered, r)
			}
		}
		runs = filtered
	}

	return runs, nil
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// containsState checks if a state slice contains a value.
func containsState(slice []agent.State, val agent.State) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// Ensure Aggregator implements analytics.Analytics
var _ analytics.Analytics = (*Aggregator)(nil)
