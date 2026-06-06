// Package pattern provides pattern detection implementations.
package pattern

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/run"
)

// PerformanceDetector detects performance anomaly patterns.
type PerformanceDetector struct {
	eventStore        event.Store
	runStore          run.Store
	slowToolThreshold time.Duration
	longRunThreshold  time.Duration
	minOccurrences    int
	stdDevMultiplier  float64
}

// PerformanceOption configures the performance detector.
type PerformanceOption func(*PerformanceDetector)

// WithSlowToolThreshold sets the threshold for slow tool detection.
func WithSlowToolThreshold(d time.Duration) PerformanceOption {
	return func(det *PerformanceDetector) {
		det.slowToolThreshold = d
	}
}

// WithLongRunThreshold sets the threshold for long run detection.
func WithLongRunThreshold(d time.Duration) PerformanceOption {
	return func(det *PerformanceDetector) {
		det.longRunThreshold = d
	}
}

// WithPerformanceMinOccurrences sets the minimum occurrences for detection.
func WithPerformanceMinOccurrences(n int) PerformanceOption {
	return func(det *PerformanceDetector) {
		det.minOccurrences = n
	}
}

// WithStdDevMultiplier sets the standard deviation multiplier for anomaly detection.
func WithStdDevMultiplier(m float64) PerformanceOption {
	return func(det *PerformanceDetector) {
		det.stdDevMultiplier = m
	}
}

// NewPerformanceDetector creates a new performance detector.
func NewPerformanceDetector(eventStore event.Store, runStore run.Store, opts ...PerformanceOption) *PerformanceDetector {
	d := &PerformanceDetector{
		eventStore:        eventStore,
		runStore:          runStore,
		slowToolThreshold: 5 * time.Second,
		longRunThreshold:  5 * time.Minute,
		minOccurrences:    3,
		stdDevMultiplier:  2.0,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect finds performance anomaly patterns across runs.
func (d *PerformanceDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
	// Get runs matching filter
	runs, err := d.runStore.List(ctx, run.ListFilter{
		FromTime: opts.FromTime,
		ToTime:   opts.ToTime,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}

	// Filter by run IDs if specified
	if len(opts.RunIDs) > 0 {
		filtered := make([]*agent.Run, 0)
		runIDSet := make(map[string]bool)
		for _, id := range opts.RunIDs {
			runIDSet[id] = true
		}
		for _, r := range runs {
			if runIDSet[r.ID] {
				filtered = append(filtered, r)
			}
		}
		runs = filtered
	}

	// Track tool execution times
	type toolExecution struct {
		runID     string
		timestamp time.Time
		duration  time.Duration
	}
	toolExecutions := make(map[string][]toolExecution)

	// Track run durations
	type runDuration struct {
		runID    string
		duration time.Duration
	}
	runDurations := make([]runDuration, 0)

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		// Calculate run duration
		if len(events) >= 2 {
			runDur := events[len(events)-1].Timestamp.Sub(events[0].Timestamp)
			runDurations = append(runDurations, runDuration{
				runID:    r.ID,
				duration: runDur,
			})
		}

		// Extract tool execution times
		var currentToolCall *event.ToolCalledPayload
		var currentToolTime time.Time

		for _, e := range events {
			switch e.Type {
			case event.TypeToolCalled:
				var payload event.ToolCalledPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					currentToolCall = &payload
					currentToolTime = e.Timestamp
				}
			case event.TypeToolSucceeded, event.TypeToolFailed:
				if currentToolCall != nil {
					duration := e.Timestamp.Sub(currentToolTime)
					toolExecutions[currentToolCall.ToolName] = append(
						toolExecutions[currentToolCall.ToolName],
						toolExecution{
							runID:     r.ID,
							timestamp: currentToolTime,
							duration:  duration,
						},
					)
					currentToolCall = nil
				}
			}
		}
	}

	var patterns []pattern.Pattern

	// Detect slow tools
	for toolName, executions := range toolExecutions {
		slowExecutions := make([]toolExecution, 0)
		var totalDuration time.Duration

		for _, exec := range executions {
			totalDuration += exec.duration
			if exec.duration > d.slowToolThreshold {
				slowExecutions = append(slowExecutions, exec)
			}
		}

		if len(slowExecutions) < d.minOccurrences {
			continue
		}

		avgDuration := totalDuration / time.Duration(len(executions))
		confidence := calculatePerformanceConfidence(len(slowExecutions), len(executions))

		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		p := pattern.NewPattern(
			pattern.PatternTypeSlowTool,
			fmt.Sprintf("Slow Tool: %s", toolName),
			fmt.Sprintf("Tool '%s' exceeds %v threshold %d times (avg: %v)",
				toolName, d.slowToolThreshold, len(slowExecutions), avgDuration),
		)
		p.Confidence = confidence
		p.Frequency = len(slowExecutions)

		// Calculate p90 duration
		durations := make([]time.Duration, len(executions))
		for i, exec := range executions {
			durations[i] = exec.duration
		}
		sort.Slice(durations, func(i, j int) bool {
			return durations[i] < durations[j]
		})
		p90Idx := int(float64(len(durations)) * 0.9)
		if p90Idx >= len(durations) {
			p90Idx = len(durations) - 1
		}

		data := pattern.SlowToolData{
			ToolName:        toolName,
			AverageDuration: avgDuration,
			P90Duration:     durations[p90Idx],
			SlowCount:       len(slowExecutions),
		}
		if err := p.SetData(data); err != nil {
			continue
		}

		for _, exec := range slowExecutions {
			_ = p.AddEvidence(exec.runID, map[string]any{
				"timestamp": exec.timestamp,
				"duration":  exec.duration.String(),
			})
		}

		patterns = append(patterns, *p)

		if opts.Limit > 0 && len(patterns) >= opts.Limit {
			return patterns, nil
		}
	}

	// Detect long runs
	longRuns := make([]runDuration, 0)
	var totalRunDuration time.Duration
	for _, rd := range runDurations {
		totalRunDuration += rd.duration
		if rd.duration > d.longRunThreshold {
			longRuns = append(longRuns, rd)
		}
	}

	if len(longRuns) >= d.minOccurrences {
		avgRunDuration := totalRunDuration / time.Duration(len(runDurations))
		confidence := calculatePerformanceConfidence(len(longRuns), len(runDurations))

		if opts.MinConfidence == 0 || confidence >= opts.MinConfidence {
			p := pattern.NewPattern(
				pattern.PatternTypeLongRuns,
				"Long Runs",
				fmt.Sprintf("Runs exceeding %v threshold %d times (avg: %v)",
					d.longRunThreshold, len(longRuns), avgRunDuration),
			)
			p.Confidence = confidence
			p.Frequency = len(longRuns)

			data := pattern.LongRunsData{
				AverageDuration: avgRunDuration,
				Threshold:       d.longRunThreshold,
				LongRunCount:    len(longRuns),
			}
			if err := p.SetData(data); err == nil {
				for _, lr := range longRuns {
					_ = p.AddEvidence(lr.runID, map[string]any{
						"duration": lr.duration.String(),
					})
				}
				patterns = append(patterns, *p)
			}
		}
	}

	return patterns, nil
}

// Types returns the pattern types this detector can find.
func (d *PerformanceDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{
		pattern.PatternTypeSlowTool,
		pattern.PatternTypeLongRuns,
	}
}

func calculatePerformanceConfidence(anomalyCount, totalCount int) float64 {
	if totalCount == 0 {
		return 0.5
	}

	// Confidence based on ratio of anomalies and total count
	ratio := float64(anomalyCount) / float64(totalCount)
	baseConfidence := 0.5 + ratio*0.3

	// Boost confidence with more data points
	dataBonus := float64(totalCount) * 0.01
	if dataBonus > 0.15 {
		dataBonus = 0.15
	}

	confidence := baseConfidence + dataBonus
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// Ensure PerformanceDetector implements Detector
var _ pattern.Detector = (*PerformanceDetector)(nil)
