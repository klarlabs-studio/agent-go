// Package pattern provides pattern detection implementations.
package pattern

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/run"
)

// SequenceDetector detects repeated tool call sequences.
type SequenceDetector struct {
	eventStore     event.Store
	runStore       run.Store
	minSequenceLen int
	minOccurrences int
}

// SequenceOption configures the sequence detector.
type SequenceOption func(*SequenceDetector)

// WithMinSequenceLength sets the minimum sequence length.
func WithMinSequenceLength(n int) SequenceOption {
	return func(d *SequenceDetector) {
		d.minSequenceLen = n
	}
}

// WithMinOccurrences sets the minimum occurrences.
func WithMinOccurrences(n int) SequenceOption {
	return func(d *SequenceDetector) {
		d.minOccurrences = n
	}
}

// NewSequenceDetector creates a new sequence detector.
func NewSequenceDetector(eventStore event.Store, runStore run.Store, opts ...SequenceOption) *SequenceDetector {
	d := &SequenceDetector{
		eventStore:     eventStore,
		runStore:       runStore,
		minSequenceLen: 2,
		minOccurrences: 3,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect finds repeated tool call sequences across runs.
func (d *SequenceDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
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

	// Extract tool sequences from each run
	type sequenceKey string
	sequenceOccurrences := make(map[sequenceKey][]sequenceOccurrence)

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		// Extract tool calls
		toolCalls := extractToolCalls(events)
		if len(toolCalls) < d.minSequenceLen {
			continue
		}

		// Find sequences of various lengths
		for seqLen := d.minSequenceLen; seqLen <= min(len(toolCalls), 5); seqLen++ {
			for i := 0; i <= len(toolCalls)-seqLen; i++ {
				seq := toolCalls[i : i+seqLen]
				key := sequenceKey(strings.Join(toolNames(seq), "->"))
				sequenceOccurrences[key] = append(sequenceOccurrences[key], sequenceOccurrence{
					runID:     r.ID,
					startTime: seq[0].timestamp,
					endTime:   seq[len(seq)-1].timestamp,
					tools:     seq,
				})
			}
		}
	}

	// Create patterns from sequences that meet the threshold
	var patterns []pattern.Pattern
	for key, occurrences := range sequenceOccurrences {
		if len(occurrences) < d.minOccurrences {
			continue
		}

		// Calculate confidence based on consistency
		confidence := calculateSequenceConfidence(occurrences)
		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		toolSeq := strings.Split(string(key), "->")
		p := pattern.NewPattern(
			pattern.PatternTypeToolSequence,
			fmt.Sprintf("Tool Sequence: %s", key),
			fmt.Sprintf("Repeated sequence of %d tools occurring %d times", len(toolSeq), len(occurrences)),
		)
		p.Confidence = confidence
		p.Frequency = len(occurrences)

		// Calculate average gap
		var totalGap time.Duration
		for _, occ := range occurrences {
			totalGap += occ.endTime.Sub(occ.startTime)
		}
		avgGap := totalGap / time.Duration(len(occurrences))

		data := pattern.ToolSequenceData{
			Sequence:   toolSeq,
			AverageGap: avgGap,
		}
		if err := p.SetData(data); err != nil {
			continue
		}

		// Add evidence
		for _, occ := range occurrences {
			if err := p.AddEvidence(occ.runID, map[string]any{
				"start_time": occ.startTime,
				"end_time":   occ.endTime,
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
func (d *SequenceDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{pattern.PatternTypeToolSequence}
}

type toolCall struct {
	name      string
	timestamp time.Time
}

type sequenceOccurrence struct {
	runID     string
	startTime time.Time
	endTime   time.Time
	tools     []toolCall
}

func extractToolCalls(events []event.Event) []toolCall {
	var calls []toolCall
	for _, e := range events {
		if e.Type == event.TypeToolCalled {
			var payload event.ToolCalledPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				calls = append(calls, toolCall{
					name:      payload.ToolName,
					timestamp: e.Timestamp,
				})
			}
		}
	}
	return calls
}

func toolNames(calls []toolCall) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.name
	}
	return names
}

func calculateSequenceConfidence(occurrences []sequenceOccurrence) float64 {
	if len(occurrences) < 2 {
		return 0.5
	}

	// Higher confidence with more occurrences
	confidence := 0.5 + float64(len(occurrences))*0.1
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// Ensure SequenceDetector implements Detector
var _ pattern.Detector = (*SequenceDetector)(nil)
