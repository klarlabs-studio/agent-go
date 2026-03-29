// Package inspector provides types for inspecting and exporting agent data.
package inspector

import (
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// ExportFormat identifies the export format.
type ExportFormat string

const (
	// FormatJSON exports as JSON.
	FormatJSON ExportFormat = "json"

	// FormatDOT exports as Graphviz DOT.
	FormatDOT ExportFormat = "dot"

	// FormatTimeline exports as timeline data.
	FormatTimeline ExportFormat = "timeline"

	// FormatMermaid exports as Mermaid diagram.
	FormatMermaid ExportFormat = "mermaid"

	// FormatHTML exports as an interactive HTML timeline.
	FormatHTML ExportFormat = "html"

	// FormatCSV exports as CSV (tool calls and state transitions).
	FormatCSV ExportFormat = "csv"

	// FormatPrometheus exports metrics in Prometheus format.
	FormatPrometheus ExportFormat = "prometheus"

	// FormatXState exports state machine as XState JSON.
	FormatXState ExportFormat = "xstate"
)

// RunExport contains exported data for a single run.
type RunExport struct {
	// Run contains the run metadata.
	Run RunMetadata `json:"run"`

	// Timeline contains ordered events.
	Timeline []TimelineEntry `json:"timeline"`

	// ToolCalls contains tool call details.
	ToolCalls []ToolCallExport `json:"tool_calls"`

	// Transitions contains state transition details.
	Transitions []TransitionExport `json:"transitions"`

	// Metrics contains computed metrics for the run.
	Metrics RunMetrics `json:"metrics"`
}

// RunMetadata contains run metadata for export.
type RunMetadata struct {
	ID        string          `json:"id"`
	Goal      string          `json:"goal"`
	Status    agent.RunStatus `json:"status"`
	State     agent.State     `json:"state"`
	StartTime time.Time       `json:"start_time"`
	EndTime   *time.Time      `json:"end_time,omitempty"`
	Result    string          `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// TimelineEntry represents a single event in the timeline.
type TimelineEntry struct {
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Type identifies the event type.
	Type string `json:"type"`

	// Label is a human-readable description.
	Label string `json:"label"`

	// State is the agent state at this point.
	State agent.State `json:"state"`

	// Duration is the time since the previous event.
	Duration time.Duration `json:"duration,omitempty"`

	// Details contains event-specific data.
	Details map[string]any `json:"details,omitempty"`
}

// ToolCallExport contains tool call details for export.
type ToolCallExport struct {
	// Name is the tool name.
	Name string `json:"name"`

	// Timestamp is when the tool was called.
	Timestamp time.Time `json:"timestamp"`

	// State is the agent state when called.
	State agent.State `json:"state"`

	// Duration is how long the tool took.
	Duration time.Duration `json:"duration"`

	// Success indicates if the tool succeeded.
	Success bool `json:"success"`

	// Input is the tool input (optional).
	Input string `json:"input,omitempty"`

	// Output is the tool output (optional).
	Output string `json:"output,omitempty"`

	// Error is the error message if failed.
	Error string `json:"error,omitempty"`
}

// TransitionExport contains state transition details for export.
type TransitionExport struct {
	// Timestamp is when the transition occurred.
	Timestamp time.Time `json:"timestamp"`

	// From is the source state.
	From agent.State `json:"from"`

	// To is the target state.
	To agent.State `json:"to"`

	// Reason explains why the transition happened.
	Reason string `json:"reason,omitempty"`

	// Duration is time spent in the source state.
	Duration time.Duration `json:"duration,omitempty"`
}

// RunMetrics contains computed metrics for a run.
type RunMetrics struct {
	// TotalDuration is the total run duration.
	TotalDuration time.Duration `json:"total_duration"`

	// ToolCallCount is the number of tool calls.
	ToolCallCount int `json:"tool_call_count"`

	// SuccessfulToolCalls is the count of successful tool calls.
	SuccessfulToolCalls int `json:"successful_tool_calls"`

	// FailedToolCalls is the count of failed tool calls.
	FailedToolCalls int `json:"failed_tool_calls"`

	// TransitionCount is the number of state transitions.
	TransitionCount int `json:"transition_count"`

	// TimeInState maps states to time spent in each.
	TimeInState map[agent.State]time.Duration `json:"time_in_state"`

	// AverageToolDuration is the average tool execution time.
	AverageToolDuration time.Duration `json:"average_tool_duration"`

	// ToolUsage counts calls per tool.
	ToolUsage map[string]int `json:"tool_usage"`
}

// StateMachineExport contains the state machine graph for export.
type StateMachineExport struct {
	// States contains all states.
	States []StateExport `json:"states"`

	// Transitions contains all transitions.
	Transitions []StateMachineTransition `json:"transitions"`

	// Initial is the initial state.
	Initial agent.State `json:"initial"`

	// Terminal contains terminal states.
	Terminal []agent.State `json:"terminal"`
}

// StateExport contains state details for export.
type StateExport struct {
	// Name is the state name.
	Name agent.State `json:"name"`

	// Description explains the state.
	Description string `json:"description,omitempty"`

	// IsTerminal indicates if this is a terminal state.
	IsTerminal bool `json:"is_terminal"`

	// AllowsSideEffects indicates if side effects are allowed.
	AllowsSideEffects bool `json:"allows_side_effects"`

	// EligibleTools lists tools allowed in this state.
	EligibleTools []string `json:"eligible_tools,omitempty"`
}

// StateMachineTransition represents a transition in the state machine.
type StateMachineTransition struct {
	// From is the source state.
	From agent.State `json:"from"`

	// To is the target state.
	To agent.State `json:"to"`

	// Label describes the transition.
	Label string `json:"label,omitempty"`

	// Count is how often this transition was taken (from analytics).
	Count int `json:"count,omitempty"`
}

// MetricsExport contains aggregated metrics for export.
type MetricsExport struct {
	// Period defines the time range.
	Period struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"period"`

	// Summary contains aggregate statistics.
	Summary struct {
		TotalRuns       int64         `json:"total_runs"`
		CompletedRuns   int64         `json:"completed_runs"`
		FailedRuns      int64         `json:"failed_runs"`
		AverageDuration time.Duration `json:"average_duration"`
	} `json:"summary"`

	// ToolMetrics contains per-tool statistics.
	ToolMetrics []ToolMetricsExport `json:"tool_metrics"`

	// StateMetrics contains per-state statistics.
	StateMetrics []StateMetricsExport `json:"state_metrics"`
}

// ToolMetricsExport contains metrics for a single tool.
type ToolMetricsExport struct {
	Name            string        `json:"name"`
	CallCount       int64         `json:"call_count"`
	SuccessCount    int64         `json:"success_count"`
	FailureCount    int64         `json:"failure_count"`
	SuccessRate     float64       `json:"success_rate"`
	AverageDuration time.Duration `json:"average_duration"`
	P90Duration     time.Duration `json:"p90_duration"`
}

// StateMetricsExport contains metrics for a single state.
type StateMetricsExport struct {
	State       agent.State   `json:"state"`
	EntryCount  int64         `json:"entry_count"`
	AverageTime time.Duration `json:"average_time"`
	TotalTime   time.Duration `json:"total_time"`
}
