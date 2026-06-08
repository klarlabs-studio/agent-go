// Package api provides the public API for the agent runtime.
package api

import (
	"time"

	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/run"
	infraPattern "go.klarlabs.de/agent/infrastructure/pattern"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// Re-export pattern types for convenience.
type (
	// Pattern represents a detected behavioral pattern.
	Pattern = pattern.Pattern

	// PatternType categorizes patterns.
	PatternType = pattern.PatternType

	// PatternEvidence records evidence supporting a pattern.
	PatternEvidence = pattern.PatternEvidence

	// PatternDetector detects patterns from run data.
	PatternDetector = pattern.Detector

	// DetectionOptions configures pattern detection.
	DetectionOptions = pattern.DetectionOptions

	// PatternStore stores detected patterns.
	PatternStore = pattern.Store

	// PatternListFilter filters pattern queries.
	PatternListFilter = pattern.ListFilter

	// EventStore stores events for pattern detection.
	EventStore = event.Store

	// RunStore stores runs for pattern detection.
	RunStore = run.Store
)

// Re-export pattern type constants.
const (
	PatternTypeToolSequence     = pattern.PatternTypeToolSequence
	PatternTypeStateLoop        = pattern.PatternTypeStateLoop
	PatternTypeToolAffinity     = pattern.PatternTypeToolAffinity
	PatternTypeRecurringFailure = pattern.PatternTypeRecurringFailure
	PatternTypeToolFailure      = pattern.PatternTypeToolFailure
	PatternTypeBudgetExhaustion = pattern.PatternTypeBudgetExhaustion
	PatternTypeSlowTool         = pattern.PatternTypeSlowTool
	PatternTypeLongRuns         = pattern.PatternTypeLongRuns
)

// NewPatternStore creates a new in-memory pattern store.
func NewPatternStore() pattern.Store {
	return memory.NewPatternStore()
}

// NewSequenceDetector creates a detector for tool sequence patterns.
func NewSequenceDetector(eventStore EventStore, runStore RunStore) pattern.Detector {
	return infraPattern.NewSequenceDetector(eventStore, runStore)
}

// NewFailureDetector creates a detector for failure patterns.
func NewFailureDetector(eventStore EventStore, runStore RunStore) pattern.Detector {
	return infraPattern.NewFailureDetector(eventStore, runStore)
}

// NewPerformanceDetector creates a detector for performance patterns.
func NewPerformanceDetector(eventStore EventStore, runStore RunStore, opts ...PerformanceDetectorOption) pattern.Detector {
	// PerformanceDetectorOption is an alias of infraPattern.PerformanceOption.
	return infraPattern.NewPerformanceDetector(eventStore, runStore, opts...)
}

// PerformanceDetectorOption configures the performance detector.
type PerformanceDetectorOption = infraPattern.PerformanceOption

// WithSlowToolThreshold sets the threshold for slow tool detection.
func WithSlowToolThreshold(d time.Duration) PerformanceDetectorOption {
	return infraPattern.WithSlowToolThreshold(d)
}

// WithLongRunThreshold sets the threshold for long run detection.
func WithLongRunThreshold(d time.Duration) PerformanceDetectorOption {
	return infraPattern.WithLongRunThreshold(d)
}

// NewCompositeDetector creates a detector that combines multiple detectors.
func NewCompositeDetector(detectors ...pattern.Detector) pattern.Detector {
	return infraPattern.NewCompositeDetector(detectors...)
}
