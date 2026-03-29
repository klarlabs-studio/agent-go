package pattern

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/pattern"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

// TimeoutDetector detects tools that consistently timeout.
type TimeoutDetector struct {
	eventStore      event.Store
	runStore        run.Store
	minTimeouts     int      // Minimum timeouts to report
	minTimeoutRate  float64  // Minimum timeout rate to report
	timeoutKeywords []string // Keywords in error messages indicating timeout
}

// TimeoutOption configures the timeout detector.
type TimeoutOption func(*TimeoutDetector)

// WithMinTimeouts sets the minimum number of timeouts.
func WithMinTimeouts(n int) TimeoutOption {
	return func(d *TimeoutDetector) {
		d.minTimeouts = n
	}
}

// WithMinTimeoutRate sets the minimum timeout rate.
func WithMinTimeoutRate(rate float64) TimeoutOption {
	return func(d *TimeoutDetector) {
		d.minTimeoutRate = rate
	}
}

// WithTimeoutKeywords sets keywords to identify timeouts.
func WithTimeoutKeywords(keywords []string) TimeoutOption {
	return func(d *TimeoutDetector) {
		d.timeoutKeywords = keywords
	}
}

// NewTimeoutDetector creates a new timeout detector.
func NewTimeoutDetector(eventStore event.Store, runStore run.Store, opts ...TimeoutOption) *TimeoutDetector {
	d := &TimeoutDetector{
		eventStore:     eventStore,
		runStore:       runStore,
		minTimeouts:    3,
		minTimeoutRate: 0.1, // 10% timeout rate
		timeoutKeywords: []string{
			"timeout",
			"timed out",
			"deadline exceeded",
			"context deadline",
			"context canceled",
		},
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect finds tools that consistently timeout.
func (d *TimeoutDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
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

	// Track tool call stats
	type toolStats struct {
		totalCalls    int
		timeouts      int
		totalDuration time.Duration
		runIDs        map[string]bool
	}
	toolStatsByName := make(map[string]*toolStats)

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		// Track in-flight tool calls
		inFlight := make(map[string]time.Time) // toolName -> start time

		for _, e := range events {
			switch e.Type {
			case event.TypeToolCalled:
				var payload event.ToolCalledPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					inFlight[payload.ToolName] = e.Timestamp

					if _, ok := toolStatsByName[payload.ToolName]; !ok {
						toolStatsByName[payload.ToolName] = &toolStats{
							runIDs: make(map[string]bool),
						}
					}
					toolStatsByName[payload.ToolName].totalCalls++
					toolStatsByName[payload.ToolName].runIDs[r.ID] = true
				}

			case event.TypeToolSucceeded:
				var payload event.ToolSucceededPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if stats, ok := toolStatsByName[payload.ToolName]; ok {
						stats.totalDuration += payload.Duration
					}
					delete(inFlight, payload.ToolName)
				}

			case event.TypeToolFailed:
				var payload event.ToolFailedPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if stats, ok := toolStatsByName[payload.ToolName]; ok {
						stats.totalDuration += payload.Duration

						// Check if this was a timeout
						if d.isTimeoutError(payload.Error) {
							stats.timeouts++
						}
					}
					delete(inFlight, payload.ToolName)
				}
			}
		}
	}

	// Create patterns for tools with significant timeout rates
	var patterns []pattern.Pattern
	for toolName, stats := range toolStatsByName {
		if stats.totalCalls == 0 || stats.timeouts < d.minTimeouts {
			continue
		}

		timeoutRate := float64(stats.timeouts) / float64(stats.totalCalls)
		if timeoutRate < d.minTimeoutRate {
			continue
		}

		avgDuration := stats.totalDuration / time.Duration(stats.totalCalls)

		// Calculate confidence based on timeout rate and sample size
		confidence := calculateTimeoutConfidence(stats.timeouts, stats.totalCalls, timeoutRate)
		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		p := pattern.NewPattern(
			pattern.PatternTypeTimeout,
			fmt.Sprintf("Timeout Pattern: %s", toolName),
			fmt.Sprintf("Tool %s has %d timeouts out of %d calls (%.1f%% timeout rate)",
				toolName, stats.timeouts, stats.totalCalls, timeoutRate*100),
		)
		p.Confidence = confidence
		p.Frequency = stats.timeouts

		data := pattern.TimeoutData{
			ToolName:     toolName,
			TimeoutCount: stats.timeouts,
			TotalCalls:   stats.totalCalls,
			TimeoutRate:  timeoutRate,
			AvgDuration:  avgDuration,
		}
		if err := p.SetData(data); err != nil {
			continue
		}

		// Add evidence from each affected run
		for runID := range stats.runIDs {
			if err := p.AddEvidence(runID, map[string]any{
				"tool_name": toolName,
			}); err != nil {
				continue
			}
		}

		patterns = append(patterns, *p)

		if opts.Limit > 0 && len(patterns) >= opts.Limit {
			break
		}
	}

	return patterns, nil
}

// Types returns the pattern types this detector can find.
func (d *TimeoutDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{pattern.PatternTypeTimeout}
}

func (d *TimeoutDetector) isTimeoutError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	for _, keyword := range d.timeoutKeywords {
		if strings.Contains(errLower, keyword) {
			return true
		}
	}
	return false
}

func calculateTimeoutConfidence(timeouts, totalCalls int, rate float64) float64 {
	// Base confidence on sample size
	sampleConfidence := 0.5 + float64(totalCalls)*0.02
	if sampleConfidence > 0.8 {
		sampleConfidence = 0.8
	}

	// Higher rate increases confidence
	rateBonus := rate * 0.2

	confidence := sampleConfidence + rateBonus
	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return confidence
}

// Ensure TimeoutDetector implements Detector
var _ pattern.Detector = (*TimeoutDetector)(nil)
