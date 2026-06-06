// Package inspector provides inspector infrastructure implementations.
package inspector

import (
	"context"
	"fmt"

	"go.klarlabs.de/agent/domain/inspector"
)

// DefaultInspector provides a default implementation of Inspector.
type DefaultInspector struct {
	runExporter          inspector.RunExporter
	stateMachineExporter inspector.StateMachineExporter
	metricsExporter      inspector.MetricsExporter
	formatters           map[inspector.ExportFormat]inspector.Formatter
}

// NewDefaultInspector creates a new default inspector.
func NewDefaultInspector(
	runExporter inspector.RunExporter,
	stateMachineExporter inspector.StateMachineExporter,
	metricsExporter inspector.MetricsExporter,
) *DefaultInspector {
	i := &DefaultInspector{
		runExporter:          runExporter,
		stateMachineExporter: stateMachineExporter,
		metricsExporter:      metricsExporter,
		formatters:           make(map[inspector.ExportFormat]inspector.Formatter),
	}

	// Register default formatters
	i.RegisterFormatter(NewJSONFormatter(WithPrettyPrint()))
	i.RegisterFormatter(NewDOTFormatter())
	i.RegisterFormatter(NewMermaidFormatter())

	return i
}

// RegisterFormatter registers a formatter for a specific format.
func (i *DefaultInspector) RegisterFormatter(formatter inspector.Formatter) {
	i.formatters[formatter.FormatType()] = formatter
}

// ExportRun exports data for a single run.
func (i *DefaultInspector) ExportRun(ctx context.Context, runID string, format inspector.ExportFormat) ([]byte, error) {
	if i.runExporter == nil {
		return nil, inspector.ErrExportFailed
	}

	data, err := i.runExporter.Export(ctx, runID)
	if err != nil {
		return nil, err
	}

	return i.format(data, format)
}

// ExportStateMachine exports the state machine graph.
func (i *DefaultInspector) ExportStateMachine(ctx context.Context, format inspector.ExportFormat) ([]byte, error) {
	if i.stateMachineExporter == nil {
		return nil, inspector.ErrExportFailed
	}

	data, err := i.stateMachineExporter.Export(ctx)
	if err != nil {
		return nil, err
	}

	return i.format(data, format)
}

// ExportMetrics exports aggregated metrics.
func (i *DefaultInspector) ExportMetrics(ctx context.Context, filter inspector.MetricsFilter, format inspector.ExportFormat) ([]byte, error) {
	if i.metricsExporter == nil {
		return nil, inspector.ErrExportFailed
	}

	data, err := i.metricsExporter.Export(ctx, filter)
	if err != nil {
		return nil, err
	}

	return i.format(data, format)
}

func (i *DefaultInspector) format(data any, format inspector.ExportFormat) ([]byte, error) {
	formatter, ok := i.formatters[format]
	if !ok {
		// Default to JSON if format not found
		formatter = NewJSONFormatter(WithPrettyPrint())
	}

	result, err := formatter.Format(data)
	if err != nil {
		return nil, fmt.Errorf("formatting failed: %w", err)
	}

	return result, nil
}

// Ensure DefaultInspector implements inspector.Inspector
var _ inspector.Inspector = (*DefaultInspector)(nil)
