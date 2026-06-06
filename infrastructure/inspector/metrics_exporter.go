// Package inspector provides inspector infrastructure implementations.
package inspector

import (
	"context"
	"sort"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/inspector"
	"go.klarlabs.de/agent/domain/run"
)

// MetricsExporter exports aggregated metrics.
type MetricsExporter struct {
	runStore   run.Store
	eventStore event.Store
}

// NewMetricsExporter creates a new metrics exporter.
func NewMetricsExporter(runStore run.Store, eventStore event.Store) *MetricsExporter {
	return &MetricsExporter{
		runStore:   runStore,
		eventStore: eventStore,
	}
}

// Export exports metrics for the given filter.
func (e *MetricsExporter) Export(ctx context.Context, filter inspector.MetricsFilter) (*inspector.MetricsExport, error) {
	// Build run filter
	runFilter := run.ListFilter{
		FromTime: filter.FromTime,
		ToTime:   filter.ToTime,
	}

	// Get runs
	runs, err := e.runStore.List(ctx, runFilter)
	if err != nil {
		return nil, err
	}

	export := &inspector.MetricsExport{}
	export.Period.From = filter.FromTime
	export.Period.To = filter.ToTime
	if export.Period.To.IsZero() {
		export.Period.To = time.Now()
	}

	// Calculate summary
	var totalDuration time.Duration
	for _, r := range runs {
		export.Summary.TotalRuns++
		switch r.Status {
		case agent.RunStatusCompleted:
			export.Summary.CompletedRuns++
		case agent.RunStatusFailed:
			export.Summary.FailedRuns++
		}
		if !r.EndTime.IsZero() {
			totalDuration += r.EndTime.Sub(r.StartTime)
		}
	}
	if export.Summary.TotalRuns > 0 {
		export.Summary.AverageDuration = totalDuration / time.Duration(export.Summary.TotalRuns)
	}

	// Calculate tool metrics
	if filter.IncludeToolMetrics {
		export.ToolMetrics = e.calculateToolMetrics(ctx, runs)
	}

	// Calculate state metrics
	if filter.IncludeStateMetrics {
		export.StateMetrics = e.calculateStateMetrics(ctx, runs)
	}

	return export, nil
}

func (e *MetricsExporter) calculateToolMetrics(ctx context.Context, runs []*agent.Run) []inspector.ToolMetricsExport {
	toolStats := make(map[string]*toolStats)

	for _, r := range runs {
		events, err := e.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		for _, evt := range events {
			switch evt.Type {
			case event.TypeToolSucceeded:
				var payload event.ToolSucceededPayload
				if err := evt.UnmarshalPayload(&payload); err == nil {
					stats := getOrCreateToolStats(toolStats, payload.ToolName)
					stats.callCount++
					stats.successCount++
					stats.durations = append(stats.durations, payload.Duration)
				}
			case event.TypeToolFailed:
				var payload event.ToolFailedPayload
				if err := evt.UnmarshalPayload(&payload); err == nil {
					stats := getOrCreateToolStats(toolStats, payload.ToolName)
					stats.callCount++
					stats.failureCount++
					stats.durations = append(stats.durations, payload.Duration)
				}
			}
		}
	}

	var metrics []inspector.ToolMetricsExport
	for name, stats := range toolStats {
		metric := inspector.ToolMetricsExport{
			Name:         name,
			CallCount:    stats.callCount,
			SuccessCount: stats.successCount,
			FailureCount: stats.failureCount,
		}
		if stats.callCount > 0 {
			metric.SuccessRate = float64(stats.successCount) / float64(stats.callCount)
			metric.AverageDuration = calculateAverageDuration(stats.durations)
			metric.P90Duration = calculateP90Duration(stats.durations)
		}
		metrics = append(metrics, metric)
	}

	// Sort by call count descending
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].CallCount > metrics[j].CallCount
	})

	return metrics
}

func (e *MetricsExporter) calculateStateMetrics(ctx context.Context, runs []*agent.Run) []inspector.StateMetricsExport {
	stateStats := make(map[agent.State]*stateStats)

	for _, r := range runs {
		events, err := e.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		var lastState agent.State
		var lastStateTime time.Time

		for _, evt := range events {
			if evt.Type == event.TypeStateTransitioned {
				var payload event.StateTransitionedPayload
				if err := evt.UnmarshalPayload(&payload); err == nil {
					// Record time in previous state
					if lastState != "" && !lastStateTime.IsZero() {
						stats := getOrCreateStateStats(stateStats, lastState)
						stats.totalTime += evt.Timestamp.Sub(lastStateTime)
					}

					// Record entry to new state
					stats := getOrCreateStateStats(stateStats, payload.ToState)
					stats.entryCount++

					lastState = payload.ToState
					lastStateTime = evt.Timestamp
				}
			}
		}
	}

	var metrics []inspector.StateMetricsExport
	for state, stats := range stateStats {
		metric := inspector.StateMetricsExport{
			State:      state,
			EntryCount: stats.entryCount,
			TotalTime:  stats.totalTime,
		}
		if stats.entryCount > 0 {
			metric.AverageTime = stats.totalTime / time.Duration(stats.entryCount)
		}
		metrics = append(metrics, metric)
	}

	// Sort by entry count descending
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].EntryCount > metrics[j].EntryCount
	})

	return metrics
}

type toolStats struct {
	callCount    int64
	successCount int64
	failureCount int64
	durations    []time.Duration
}

type stateStats struct {
	entryCount int64
	totalTime  time.Duration
}

func getOrCreateToolStats(m map[string]*toolStats, name string) *toolStats {
	if _, ok := m[name]; !ok {
		m[name] = &toolStats{}
	}
	return m[name]
}

func getOrCreateStateStats(m map[agent.State]*stateStats, state agent.State) *stateStats {
	if _, ok := m[state]; !ok {
		m[state] = &stateStats{}
	}
	return m[state]
}

func calculateAverageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func calculateP90Duration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	idx := int(float64(len(sorted)) * 0.9)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// Ensure MetricsExporter implements inspector.MetricsExporter
var _ inspector.MetricsExporter = (*MetricsExporter)(nil)
