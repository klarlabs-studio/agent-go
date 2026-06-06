// Package pattern provides pattern detection implementations.
package pattern

import (
	"context"
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/run"
)

// FailureDetector detects recurring failure patterns.
type FailureDetector struct {
	eventStore     event.Store
	runStore       run.Store
	minOccurrences int
}

// FailureOption configures the failure detector.
type FailureOption func(*FailureDetector)

// WithFailureMinOccurrences sets the minimum occurrences for failure detection.
func WithFailureMinOccurrences(n int) FailureOption {
	return func(d *FailureDetector) {
		d.minOccurrences = n
	}
}

// NewFailureDetector creates a new failure detector.
func NewFailureDetector(eventStore event.Store, runStore run.Store, opts ...FailureOption) *FailureDetector {
	d := &FailureDetector{
		eventStore:     eventStore,
		runStore:       runStore,
		minOccurrences: 3,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect finds recurring failure patterns across runs.
func (d *FailureDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
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

	// Track failure types
	type failureKey struct {
		toolName string
		errType  string
	}
	type failureOccurrence struct {
		runID     string
		timestamp time.Time
		errorMsg  string
	}
	failureOccurrences := make(map[failureKey][]failureOccurrence)
	budgetExhaustions := make([]failureOccurrence, 0)

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		for _, e := range events {
			switch e.Type {
			case event.TypeToolFailed:
				var payload event.ToolFailedPayload
				if err := e.UnmarshalPayload(&payload); err == nil {
					key := failureKey{
						toolName: payload.ToolName,
						errType:  classifyError(payload.Error),
					}
					failureOccurrences[key] = append(failureOccurrences[key], failureOccurrence{
						runID:     r.ID,
						timestamp: e.Timestamp,
						errorMsg:  payload.Error,
					})
				}
			case event.TypeBudgetExhausted:
				budgetExhaustions = append(budgetExhaustions, failureOccurrence{
					runID:     r.ID,
					timestamp: e.Timestamp,
				})
			}
		}
	}

	// Create patterns from failures that meet the threshold
	var patterns []pattern.Pattern

	// Tool failure patterns
	for key, occurrences := range failureOccurrences {
		if len(occurrences) < d.minOccurrences {
			continue
		}

		confidence := calculateFailureConfidence(len(occurrences))
		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		p := pattern.NewPattern(
			pattern.PatternTypeToolFailure,
			fmt.Sprintf("Tool Failure: %s", key.toolName),
			fmt.Sprintf("Tool '%s' fails with '%s' error type %d times", key.toolName, key.errType, len(occurrences)),
		)
		p.Confidence = confidence
		p.Frequency = len(occurrences)

		data := pattern.ToolFailureData{
			ToolName:   key.toolName,
			ErrorType:  key.errType,
			ErrorCount: len(occurrences),
		}
		if err := p.SetData(data); err != nil {
			continue
		}

		// Add evidence
		for _, occ := range occurrences {
			if err := p.AddEvidence(occ.runID, map[string]any{
				"timestamp": occ.timestamp,
				"error":     occ.errorMsg,
			}); err != nil {
				continue
			}
		}

		patterns = append(patterns, *p)

		if opts.Limit > 0 && len(patterns) >= opts.Limit {
			return patterns, nil
		}
	}

	// Budget exhaustion pattern
	if len(budgetExhaustions) >= d.minOccurrences {
		confidence := 0.5 + float64(len(budgetExhaustions))*0.1
		if confidence > 0.95 {
			confidence = 0.95
		}

		if opts.MinConfidence == 0 || confidence >= opts.MinConfidence {
			p := pattern.NewPattern(
				pattern.PatternTypeBudgetExhaustion,
				"Budget Exhaustion",
				fmt.Sprintf("Runs hitting budget limits %d times", len(budgetExhaustions)),
			)
			p.Confidence = confidence
			p.Frequency = len(budgetExhaustions)

			data := pattern.BudgetExhaustionData{
				ExhaustionCount: len(budgetExhaustions),
			}
			if err := p.SetData(data); err == nil {
				for _, occ := range budgetExhaustions {
					_ = p.AddEvidence(occ.runID, map[string]any{
						"timestamp": occ.timestamp,
					})
				}
				patterns = append(patterns, *p)
			}
		}
	}

	return patterns, nil
}

// Types returns the pattern types this detector can find.
func (d *FailureDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{
		pattern.PatternTypeToolFailure,
		pattern.PatternTypeRecurringFailure,
		pattern.PatternTypeBudgetExhaustion,
	}
}

func classifyError(errMsg string) string {
	// Simple error classification based on common patterns
	switch {
	case containsAny(errMsg, "timeout", "deadline exceeded"):
		return "timeout"
	case containsAny(errMsg, "connection refused", "network", "unreachable"):
		return "network"
	case containsAny(errMsg, "permission denied", "unauthorized", "forbidden"):
		return "permission"
	case containsAny(errMsg, "not found", "no such file", "does not exist"):
		return "not_found"
	case containsAny(errMsg, "invalid", "malformed", "parse"):
		return "validation"
	case containsAny(errMsg, "rate limit", "throttle", "too many"):
		return "rate_limit"
	default:
		return "unknown"
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if containsIgnoreCase(s, substr) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		findIgnoreCase(s, substr) >= 0)
}

func findIgnoreCase(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, i, substr) {
			return i
		}
	}
	return -1
}

func equalFoldAt(s string, start int, substr string) bool {
	for j := 0; j < len(substr); j++ {
		c1, c2 := s[start+j], substr[j]
		if c1 != c2 && toLower(c1) != toLower(c2) {
			return false
		}
	}
	return true
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func calculateFailureConfidence(occurrenceCount int) float64 {
	if occurrenceCount < 2 {
		return 0.5
	}

	// Higher confidence with more occurrences
	confidence := 0.5 + float64(occurrenceCount)*0.1
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// Ensure FailureDetector implements Detector
var _ pattern.Detector = (*FailureDetector)(nil)
