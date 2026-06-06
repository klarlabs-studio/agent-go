// Package application provides application services.
package application

import (
	"context"

	"go.klarlabs.de/agent/domain/inspector"
)

// InspectionService provides inspection and export capabilities.
type InspectionService struct {
	inspector inspector.Inspector
}

// NewInspectionService creates a new inspection service.
func NewInspectionService(insp inspector.Inspector) *InspectionService {
	return &InspectionService{
		inspector: insp,
	}
}

// ExportRun exports run data in the specified format.
func (s *InspectionService) ExportRun(ctx context.Context, runID string, format inspector.ExportFormat) ([]byte, error) {
	if s.inspector == nil {
		return nil, inspector.ErrExportFailed
	}
	return s.inspector.ExportRun(ctx, runID, format)
}

// ExportStateMachine exports the state machine graph.
func (s *InspectionService) ExportStateMachine(ctx context.Context, format inspector.ExportFormat) ([]byte, error) {
	if s.inspector == nil {
		return nil, inspector.ErrExportFailed
	}
	return s.inspector.ExportStateMachine(ctx, format)
}

// ExportMetrics exports aggregated metrics.
func (s *InspectionService) ExportMetrics(ctx context.Context, filter inspector.MetricsFilter, format inspector.ExportFormat) ([]byte, error) {
	if s.inspector == nil {
		return nil, inspector.ErrExportFailed
	}
	return s.inspector.ExportMetrics(ctx, filter, format)
}

// GetRunAsJSON exports run data as JSON (convenience method).
func (s *InspectionService) GetRunAsJSON(ctx context.Context, runID string) ([]byte, error) {
	return s.ExportRun(ctx, runID, inspector.FormatJSON)
}

// GetStateMachineAsDOT exports the state machine as DOT graph (convenience method).
func (s *InspectionService) GetStateMachineAsDOT(ctx context.Context) ([]byte, error) {
	return s.ExportStateMachine(ctx, inspector.FormatDOT)
}

// GetStateMachineAsMermaid exports the state machine as Mermaid diagram (convenience method).
func (s *InspectionService) GetStateMachineAsMermaid(ctx context.Context) ([]byte, error) {
	return s.ExportStateMachine(ctx, inspector.FormatMermaid)
}

// GetMetricsAsJSON exports metrics as JSON (convenience method).
func (s *InspectionService) GetMetricsAsJSON(ctx context.Context, filter inspector.MetricsFilter) ([]byte, error) {
	return s.ExportMetrics(ctx, filter, inspector.FormatJSON)
}
