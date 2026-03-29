package pattern

import (
	"context"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/pattern"
	"github.com/felixgeelhaar/agent-go/domain/run"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// ToolPreferenceDetector detects over/under-used tools.
type ToolPreferenceDetector struct {
	eventStore        event.Store
	runStore          run.Store
	toolRegistry      tool.Registry
	overuseThreshold  float64 // Usage ratio above this is "overused"
	underuseThreshold float64 // Usage ratio below this is "underused"
	minCalls          int     // Minimum calls to consider
}

// ToolPreferenceOption configures the tool preference detector.
type ToolPreferenceOption func(*ToolPreferenceDetector)

// WithOveruseThreshold sets the overuse threshold.
func WithOveruseThreshold(t float64) ToolPreferenceOption {
	return func(d *ToolPreferenceDetector) {
		d.overuseThreshold = t
	}
}

// WithUnderuseThreshold sets the underuse threshold.
func WithUnderuseThreshold(t float64) ToolPreferenceOption {
	return func(d *ToolPreferenceDetector) {
		d.underuseThreshold = t
	}
}

// WithPreferenceMinCalls sets the minimum calls threshold.
func WithPreferenceMinCalls(n int) ToolPreferenceOption {
	return func(d *ToolPreferenceDetector) {
		d.minCalls = n
	}
}

// NewToolPreferenceDetector creates a new tool preference detector.
func NewToolPreferenceDetector(eventStore event.Store, runStore run.Store, toolRegistry tool.Registry, opts ...ToolPreferenceOption) *ToolPreferenceDetector {
	d := &ToolPreferenceDetector{
		eventStore:        eventStore,
		runStore:          runStore,
		toolRegistry:      toolRegistry,
		overuseThreshold:  2.0,  // 2x expected usage
		underuseThreshold: 0.25, // 25% of expected usage
		minCalls:          5,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect finds over/under-used tools.
func (d *ToolPreferenceDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
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

	if len(runs) == 0 {
		return nil, nil
	}

	// Track tool usage stats
	type toolUsageStats struct {
		calls     int
		successes int
		failures  int
		runIDs    map[string]bool
		states    map[agent.State]int // state -> call count
	}
	usageByTool := make(map[string]*toolUsageStats)

	// Get all available tools
	var availableTools []tool.Tool
	if d.toolRegistry != nil {
		availableTools = d.toolRegistry.List()
	}

	// Initialize stats for all known tools
	for _, t := range availableTools {
		usageByTool[t.Name()] = &toolUsageStats{
			runIDs: make(map[string]bool),
			states: make(map[agent.State]int),
		}
	}

	// Track total tool calls for expected usage calculation
	totalCalls := 0

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		for _, e := range events {
			switch e.Type {
			case event.TypeToolCalled:
				var payload event.ToolCalledPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if _, ok := usageByTool[payload.ToolName]; !ok {
						usageByTool[payload.ToolName] = &toolUsageStats{
							runIDs: make(map[string]bool),
							states: make(map[agent.State]int),
						}
					}
					stats := usageByTool[payload.ToolName]
					stats.calls++
					stats.runIDs[r.ID] = true
					stats.states[payload.State]++
					totalCalls++
				}

			case event.TypeToolSucceeded:
				var payload event.ToolSucceededPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if stats, ok := usageByTool[payload.ToolName]; ok {
						stats.successes++
					}
				}

			case event.TypeToolFailed:
				var payload event.ToolFailedPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					if stats, ok := usageByTool[payload.ToolName]; ok {
						stats.failures++
					}
				}
			}
		}
	}

	if totalCalls == 0 {
		return nil, nil
	}

	// Calculate expected usage per tool (uniform distribution baseline)
	numTools := len(usageByTool)
	if numTools == 0 {
		return nil, nil
	}
	expectedUsage := float64(totalCalls) / float64(numTools)

	// Create patterns for over/under-used tools
	var patterns []pattern.Pattern
	for toolName, stats := range usageByTool {
		if stats.calls < d.minCalls && stats.calls > 0 {
			// Too few calls but some usage - might still be underused
		} else if stats.calls == 0 && totalCalls > d.minCalls*numTools {
			// Tool never used but others are being used - underused
		} else if stats.calls < d.minCalls {
			continue
		}

		usageRatio := float64(stats.calls) / expectedUsage
		successRate := 0.0
		if stats.calls > 0 {
			successRate = float64(stats.successes) / float64(stats.calls)
		}

		var preferenceType string
		var isPreference bool

		if usageRatio >= d.overuseThreshold {
			preferenceType = "overused"
			isPreference = true
		} else if usageRatio <= d.underuseThreshold || stats.calls == 0 {
			preferenceType = "underused"
			isPreference = true
		}

		if !isPreference {
			continue
		}

		// Calculate confidence
		confidence := calculatePreferenceConfidence(stats.calls, usageRatio, d.overuseThreshold, d.underuseThreshold)
		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		// Get available states for this tool
		availableStates := make([]agent.State, 0)
		for state := range stats.states {
			availableStates = append(availableStates, state)
		}

		p := pattern.NewPattern(
			pattern.PatternTypeToolPreference,
			fmt.Sprintf("Tool Preference: %s (%s)", toolName, preferenceType),
			fmt.Sprintf("Tool %s is %s with usage ratio %.2f (%.1f%% success rate)",
				toolName, preferenceType, usageRatio, successRate*100),
		)
		p.Confidence = confidence
		p.Frequency = stats.calls

		data := pattern.ToolPreferenceData{
			ToolName:        toolName,
			UsageCount:      stats.calls,
			ExpectedUsage:   expectedUsage,
			UsageRatio:      usageRatio,
			PreferenceType:  preferenceType,
			SuccessRate:     successRate,
			AvailableStates: availableStates,
		}
		if err := p.SetData(data); err != nil {
			continue
		}

		// Add evidence
		for runID := range stats.runIDs {
			if err := p.AddEvidence(runID, map[string]any{
				"tool_name":   toolName,
				"usage_count": stats.calls,
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
func (d *ToolPreferenceDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{pattern.PatternTypeToolPreference}
}

func calculatePreferenceConfidence(calls int, ratio, overThreshold, underThreshold float64) float64 {
	// Base confidence on how extreme the ratio is
	var extremity float64
	if ratio >= overThreshold {
		extremity = ratio / overThreshold
	} else if ratio <= underThreshold && ratio > 0 {
		extremity = underThreshold / ratio
	} else if ratio == 0 {
		extremity = 3.0 // Never used is significant
	}

	confidence := 0.4 + extremity*0.15

	// Bonus for more calls
	callBonus := float64(calls) * 0.01
	if callBonus > 0.2 {
		callBonus = 0.2
	}
	confidence += callBonus

	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return confidence
}

// Ensure ToolPreferenceDetector implements Detector
var _ pattern.Detector = (*ToolPreferenceDetector)(nil)
