// Package pattern provides pattern detection implementations.
package pattern

import (
	"context"
	"fmt"
	"sort"

	"go.klarlabs.de/agent/domain/pattern"
)

// CompositeDetector combines multiple detectors.
type CompositeDetector struct {
	detectors []pattern.Detector
}

// NewCompositeDetector creates a detector that combines multiple detectors.
func NewCompositeDetector(detectors ...pattern.Detector) *CompositeDetector {
	return &CompositeDetector{
		detectors: detectors,
	}
}

// AddDetector adds a detector to the composite.
func (c *CompositeDetector) AddDetector(detector pattern.Detector) {
	c.detectors = append(c.detectors, detector)
}

// Detect runs all detectors and combines their results.
func (c *CompositeDetector) Detect(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
	var allPatterns []pattern.Pattern
	var errors []error

	// Filter detectors by requested pattern types
	detectorsToRun := c.detectors
	if len(opts.PatternTypes) > 0 {
		detectorsToRun = c.filterDetectorsByTypes(opts.PatternTypes)
	}

	// Run each detector
	for _, detector := range detectorsToRun {
		patterns, err := detector.Detect(ctx, opts)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		allPatterns = append(allPatterns, patterns...)
	}

	// If all detectors failed, return an error
	if len(errors) == len(detectorsToRun) && len(errors) > 0 {
		return nil, fmt.Errorf("all detectors failed: %v", errors)
	}

	// Sort patterns by confidence (descending) then frequency (descending)
	sort.Slice(allPatterns, func(i, j int) bool {
		if allPatterns[i].Confidence != allPatterns[j].Confidence {
			return allPatterns[i].Confidence > allPatterns[j].Confidence
		}
		return allPatterns[i].Frequency > allPatterns[j].Frequency
	})

	// Apply limit if specified
	if opts.Limit > 0 && len(allPatterns) > opts.Limit {
		allPatterns = allPatterns[:opts.Limit]
	}

	return allPatterns, nil
}

// Types returns all pattern types from all detectors.
func (c *CompositeDetector) Types() []pattern.PatternType {
	typeSet := make(map[pattern.PatternType]bool)
	for _, detector := range c.detectors {
		for _, t := range detector.Types() {
			typeSet[t] = true
		}
	}

	types := make([]pattern.PatternType, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

// filterDetectorsByTypes returns detectors that can detect any of the given types.
func (c *CompositeDetector) filterDetectorsByTypes(types []pattern.PatternType) []pattern.Detector {
	typeSet := make(map[pattern.PatternType]bool)
	for _, t := range types {
		typeSet[t] = true
	}

	var filtered []pattern.Detector
	for _, detector := range c.detectors {
		for _, t := range detector.Types() {
			if typeSet[t] {
				filtered = append(filtered, detector)
				break
			}
		}
	}
	return filtered
}

// Ensure CompositeDetector implements Detector
var _ pattern.Detector = (*CompositeDetector)(nil)
