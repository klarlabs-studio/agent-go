// Package api provides the public API for the agent runtime.
package api

import (
	"github.com/felixgeelhaar/agent-go/domain/inspector"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	infraInspector "github.com/felixgeelhaar/agent-go/infrastructure/inspector"
)

// Re-export inspector types for convenience.
type (
	// Inspector provides inspection and export capabilities.
	Inspector = inspector.Inspector

	// ExportFormat specifies the output format for exports.
	ExportFormat = inspector.ExportFormat

	// RunExport contains exported run data.
	RunExport = inspector.RunExport

	// RunMetadata contains run metadata.
	RunMetadata = inspector.RunMetadata

	// RunMetrics contains run performance metrics.
	RunMetrics = inspector.RunMetrics

	// TimelineEntry represents an entry in the run timeline.
	TimelineEntry = inspector.TimelineEntry

	// ToolCallExport contains exported tool call data.
	ToolCallExport = inspector.ToolCallExport

	// TransitionExport contains exported state transition data.
	TransitionExport = inspector.TransitionExport

	// StateMachineExport contains exported state machine data.
	StateMachineExport = inspector.StateMachineExport

	// StateExport contains exported state data.
	StateExport = inspector.StateExport

	// StateMachineTransition contains state transition data.
	StateMachineTransition = inspector.StateMachineTransition

	// MetricsExport contains exported metrics data.
	MetricsExport = inspector.MetricsExport

	// MetricsFilter filters metrics queries.
	MetricsFilter = inspector.MetricsFilter
)

// Re-export export format constants.
const (
	FormatJSON     = inspector.FormatJSON
	FormatDOT      = inspector.FormatDOT
	FormatMermaid  = inspector.FormatMermaid
	FormatTimeline = inspector.FormatTimeline
)

// NewInspector creates a new inspector with the provided exporters.
func NewInspector(
	runExporter inspector.RunExporter,
	stateMachineExporter inspector.StateMachineExporter,
	metricsExporter inspector.MetricsExporter,
) inspector.Inspector {
	return infraInspector.NewDefaultInspector(runExporter, stateMachineExporter, metricsExporter)
}

// NewRunExporter creates a new run exporter.
func NewRunExporter(runStore RunStore, eventStore EventStore) inspector.RunExporter {
	return infraInspector.NewRunExporter(runStore, eventStore)
}

// NewStateMachineExporter creates a new state machine exporter.
func NewStateMachineExporter(
	eligibility *policy.ToolEligibility,
	transitions *policy.StateTransitions,
) inspector.StateMachineExporter {
	return infraInspector.NewStateMachineExporter(eligibility, transitions)
}

// NewMetricsExporter creates a new metrics exporter.
func NewMetricsExporter(runStore RunStore, eventStore EventStore) inspector.MetricsExporter {
	return infraInspector.NewMetricsExporter(runStore, eventStore)
}
