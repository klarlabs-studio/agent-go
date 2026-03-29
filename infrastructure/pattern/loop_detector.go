package pattern

import (
	"context"
	"fmt"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/pattern"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

// LoopDetector detects state cycles and repeated transitions.
type LoopDetector struct {
	eventStore     event.Store
	runStore       run.Store
	minLoopLength  int
	minOccurrences int
	maxLoopLength  int
}

// LoopOption configures the loop detector.
type LoopOption func(*LoopDetector)

// WithLoopMinLength sets the minimum loop length.
func WithLoopMinLength(n int) LoopOption {
	return func(d *LoopDetector) {
		d.minLoopLength = n
	}
}

// WithLoopMinOccurrences sets the minimum occurrences.
func WithLoopMinOccurrences(n int) LoopOption {
	return func(d *LoopDetector) {
		d.minOccurrences = n
	}
}

// WithMaxLoopLength sets the maximum loop length to detect.
func WithMaxLoopLength(n int) LoopOption {
	return func(d *LoopDetector) {
		d.maxLoopLength = n
	}
}

// NewLoopDetector creates a new loop detector.
func NewLoopDetector(eventStore event.Store, runStore run.Store, opts ...LoopOption) *LoopDetector {
	d := &LoopDetector{
		eventStore:     eventStore,
		runStore:       runStore,
		minLoopLength:  2,
		minOccurrences: 2,
		maxLoopLength:  6,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect finds state loops and cycles in runs.
func (d *LoopDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
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

	// Track loop occurrences across runs
	type loopKey string
	loopOccurrences := make(map[loopKey][]loopOccurrence)

	for _, r := range runs {
		events, err := d.eventStore.LoadEvents(ctx, r.ID)
		if err != nil {
			continue
		}

		// Extract state transitions
		transitions := extractStateTransitions(events)
		if len(transitions) < d.minLoopLength*2 {
			continue
		}

		// Detect loops of various lengths
		for loopLen := d.minLoopLength; loopLen <= min(len(transitions)/2, d.maxLoopLength); loopLen++ {
			loops := findLoops(transitions, loopLen)
			for _, loop := range loops {
				key := loopKey(stateSequenceKey(loop.states))
				loopOccurrences[key] = append(loopOccurrences[key], loopOccurrence{
					runID:      r.ID,
					states:     loop.states,
					iterations: loop.iterations,
					exitState:  loop.exitState,
				})
			}
		}
	}

	// Create patterns from loops that meet the threshold
	var patterns []pattern.Pattern
	for key, occurrences := range loopOccurrences {
		if len(occurrences) < d.minOccurrences {
			continue
		}

		// Calculate confidence based on consistency
		confidence := calculateLoopConfidence(occurrences)
		if opts.MinConfidence > 0 && confidence < opts.MinConfidence {
			continue
		}

		stateSeq := strings.Split(string(key), "->")
		agentStates := make([]agent.State, len(stateSeq))
		for i, s := range stateSeq {
			agentStates[i] = agent.State(s)
		}

		avgIterations := calculateAverageIterations(occurrences)
		exitState := findMostCommonExitState(occurrences)

		p := pattern.NewPattern(
			pattern.PatternTypeStateLoop,
			fmt.Sprintf("State Loop: %s", key),
			fmt.Sprintf("Detected loop of %d states with avg %d iterations, typically exits to %s",
				len(stateSeq), avgIterations, exitState),
		)
		p.Confidence = confidence
		p.Frequency = len(occurrences)

		data := pattern.StateLoopData{
			Loop:       agentStates,
			Iterations: avgIterations,
			ExitState:  exitState,
		}
		if err := p.SetData(data); err != nil {
			continue
		}

		// Add evidence
		for _, occ := range occurrences {
			if err := p.AddEvidence(occ.runID, map[string]any{
				"iterations": occ.iterations,
				"exit_state": occ.exitState,
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
func (d *LoopDetector) Types() []pattern.PatternType {
	return []pattern.PatternType{pattern.PatternTypeStateLoop}
}

type loopOccurrence struct {
	runID      string
	states     []agent.State
	iterations int
	exitState  agent.State
}

type detectedLoop struct {
	states     []agent.State
	iterations int
	exitState  agent.State
}

func extractStateTransitions(events []event.Event) []agent.State {
	var states []agent.State
	for _, e := range events {
		if e.Type == event.TypeStateTransitioned {
			var payload event.StateTransitionedPayload
			if err := e.UnmarshalPayload(&payload); err == nil {
				states = append(states, payload.ToState)
			}
		}
	}
	return states
}

func findLoops(states []agent.State, loopLen int) []detectedLoop {
	var loops []detectedLoop

	for i := 0; i <= len(states)-loopLen*2; i++ {
		// Extract potential loop pattern
		loopPattern := states[i : i+loopLen]

		// Count consecutive repetitions
		iterations := 1
		j := i + loopLen
		for j+loopLen <= len(states) {
			matches := true
			for k := 0; k < loopLen; k++ {
				if states[j+k] != loopPattern[k] {
					matches = false
					break
				}
			}
			if !matches {
				break
			}
			iterations++
			j += loopLen
		}

		// If we found at least 2 iterations, it's a loop
		if iterations >= 2 {
			exitState := agent.State("")
			if j < len(states) {
				exitState = states[j]
			}

			loops = append(loops, detectedLoop{
				states:     loopPattern,
				iterations: iterations,
				exitState:  exitState,
			})

			// Skip past this loop to avoid counting it multiple times
			i = j - 1
		}
	}

	return loops
}

func stateSequenceKey(states []agent.State) string {
	strs := make([]string, len(states))
	for i, s := range states {
		strs[i] = string(s)
	}
	return strings.Join(strs, "->")
}

func calculateLoopConfidence(occurrences []loopOccurrence) float64 {
	if len(occurrences) < 2 {
		return 0.5
	}

	// Higher confidence with more occurrences and consistent iterations
	baseConfidence := 0.5 + float64(len(occurrences))*0.1

	// Check iteration consistency
	var totalIterations int
	for _, occ := range occurrences {
		totalIterations += occ.iterations
	}
	avgIterations := float64(totalIterations) / float64(len(occurrences))

	// Penalize high variance in iterations
	var variance float64
	for _, occ := range occurrences {
		diff := float64(occ.iterations) - avgIterations
		variance += diff * diff
	}
	variance /= float64(len(occurrences))

	// High variance reduces confidence
	variancePenalty := variance / 10.0
	if variancePenalty > 0.3 {
		variancePenalty = 0.3
	}

	confidence := baseConfidence - variancePenalty
	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return confidence
}

func calculateAverageIterations(occurrences []loopOccurrence) int {
	if len(occurrences) == 0 {
		return 0
	}
	total := 0
	for _, occ := range occurrences {
		total += occ.iterations
	}
	return total / len(occurrences)
}

func findMostCommonExitState(occurrences []loopOccurrence) agent.State {
	counts := make(map[agent.State]int)
	for _, occ := range occurrences {
		if occ.exitState != "" {
			counts[occ.exitState]++
		}
	}

	var maxState agent.State
	var maxCount int
	for state, count := range counts {
		if count > maxCount {
			maxCount = count
			maxState = state
		}
	}

	return maxState
}

// Ensure LoopDetector implements Detector
var _ pattern.Detector = (*LoopDetector)(nil)
