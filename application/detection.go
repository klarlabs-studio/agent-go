// Package application provides application services.
package application

import (
	"context"
	"fmt"

	"go.klarlabs.de/agent/domain/pattern"
)

// DetectionService manages pattern detection.
type DetectionService struct {
	detector     pattern.Detector
	patternStore pattern.Store
}

// NewDetectionService creates a new detection service.
func NewDetectionService(detector pattern.Detector, patternStore pattern.Store) *DetectionService {
	return &DetectionService{
		detector:     detector,
		patternStore: patternStore,
	}
}

// DetectPatterns runs pattern detection and stores the results.
func (s *DetectionService) DetectPatterns(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error) {
	if s.detector == nil {
		return nil, pattern.ErrDetectionFailed
	}

	// Run detection
	patterns, err := s.detector.Detect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("pattern detection failed: %w", err)
	}

	// Store patterns if store is configured
	if s.patternStore != nil {
		for i := range patterns {
			if err := s.patternStore.Save(ctx, &patterns[i]); err != nil {
				// Try to update if already exists
				if err == pattern.ErrPatternExists {
					if updateErr := s.patternStore.Update(ctx, &patterns[i]); updateErr != nil {
						continue
					}
				}
			}
		}
	}

	return patterns, nil
}

// GetPattern retrieves a pattern by ID.
func (s *DetectionService) GetPattern(ctx context.Context, id string) (*pattern.Pattern, error) {
	if s.patternStore == nil {
		return nil, pattern.ErrPatternNotFound
	}
	return s.patternStore.Get(ctx, id)
}

// ListPatterns returns patterns matching the filter.
func (s *DetectionService) ListPatterns(ctx context.Context, filter pattern.ListFilter) ([]*pattern.Pattern, error) {
	if s.patternStore == nil {
		return []*pattern.Pattern{}, nil
	}
	return s.patternStore.List(ctx, filter)
}

// DeletePattern removes a pattern.
func (s *DetectionService) DeletePattern(ctx context.Context, id string) error {
	if s.patternStore == nil {
		return pattern.ErrPatternNotFound
	}
	return s.patternStore.Delete(ctx, id)
}

// GetSupportedPatternTypes returns the pattern types the detector can find.
func (s *DetectionService) GetSupportedPatternTypes() []pattern.PatternType {
	if s.detector == nil {
		return []pattern.PatternType{}
	}
	return s.detector.Types()
}
