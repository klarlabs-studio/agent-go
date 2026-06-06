package inspector

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"go.klarlabs.de/agent/domain/inspector"
)

// PrometheusFormatter formats metrics data in Prometheus exposition format.
type PrometheusFormatter struct {
	namespace string
	subsystem string
}

// PrometheusFormatterOption configures the Prometheus formatter.
type PrometheusFormatterOption func(*PrometheusFormatter)

// WithNamespace sets the metric namespace.
func WithNamespace(namespace string) PrometheusFormatterOption {
	return func(f *PrometheusFormatter) {
		f.namespace = namespace
	}
}

// WithSubsystem sets the metric subsystem.
func WithSubsystem(subsystem string) PrometheusFormatterOption {
	return func(f *PrometheusFormatter) {
		f.subsystem = subsystem
	}
}

// NewPrometheusFormatter creates a new Prometheus formatter.
func NewPrometheusFormatter(opts ...PrometheusFormatterOption) *PrometheusFormatter {
	f := &PrometheusFormatter{
		namespace: "agent",
		subsystem: "",
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Format formats the data in Prometheus exposition format.
func (f *PrometheusFormatter) Format(data any) ([]byte, error) {
	var buf bytes.Buffer

	switch d := data.(type) {
	case *inspector.RunExport:
		f.formatRunExport(&buf, d)
	case *inspector.StateMachineExport:
		f.formatStateMachineExport(&buf, d)
	case *inspector.MetricsExport:
		f.formatMetricsExport(&buf, d)
	default:
		return nil, fmt.Errorf("unsupported data type for Prometheus: %T", data)
	}

	return buf.Bytes(), nil
}

func (f *PrometheusFormatter) metricName(name string) string {
	parts := []string{}
	if f.namespace != "" {
		parts = append(parts, f.namespace)
	}
	if f.subsystem != "" {
		parts = append(parts, f.subsystem)
	}
	parts = append(parts, name)
	return strings.Join(parts, "_")
}

func (f *PrometheusFormatter) formatRunExport(buf *bytes.Buffer, data *inspector.RunExport) {
	runLabels := fmt.Sprintf(`run_id="%s",status="%s",state="%s"`,
		data.Run.ID, data.Run.Status, data.Run.State)

	// Run metrics
	f.writeHelp(buf, "run_duration_seconds", "Total run duration in seconds")
	f.writeType(buf, "run_duration_seconds", "gauge")
	f.writeMetric(buf, "run_duration_seconds", runLabels, data.Metrics.TotalDuration.Seconds())

	f.writeHelp(buf, "run_tool_calls_total", "Total number of tool calls")
	f.writeType(buf, "run_tool_calls_total", "counter")
	f.writeMetric(buf, "run_tool_calls_total", runLabels, float64(data.Metrics.ToolCallCount))

	f.writeHelp(buf, "run_tool_calls_success_total", "Number of successful tool calls")
	f.writeType(buf, "run_tool_calls_success_total", "counter")
	f.writeMetric(buf, "run_tool_calls_success_total", runLabels, float64(data.Metrics.SuccessfulToolCalls))

	f.writeHelp(buf, "run_tool_calls_failed_total", "Number of failed tool calls")
	f.writeType(buf, "run_tool_calls_failed_total", "counter")
	f.writeMetric(buf, "run_tool_calls_failed_total", runLabels, float64(data.Metrics.FailedToolCalls))

	f.writeHelp(buf, "run_transitions_total", "Number of state transitions")
	f.writeType(buf, "run_transitions_total", "counter")
	f.writeMetric(buf, "run_transitions_total", runLabels, float64(data.Metrics.TransitionCount))

	f.writeHelp(buf, "run_tool_avg_duration_seconds", "Average tool execution duration")
	f.writeType(buf, "run_tool_avg_duration_seconds", "gauge")
	f.writeMetric(buf, "run_tool_avg_duration_seconds", runLabels, data.Metrics.AverageToolDuration.Seconds())

	// Time in state
	f.writeHelp(buf, "run_state_duration_seconds", "Time spent in each state")
	f.writeType(buf, "run_state_duration_seconds", "gauge")
	for state, duration := range data.Metrics.TimeInState {
		stateLabels := fmt.Sprintf(`run_id="%s",state="%s"`, data.Run.ID, state)
		f.writeMetric(buf, "run_state_duration_seconds", stateLabels, duration.Seconds())
	}

	// Tool usage
	f.writeHelp(buf, "run_tool_usage_total", "Number of times each tool was called")
	f.writeType(buf, "run_tool_usage_total", "counter")
	for tool, count := range data.Metrics.ToolUsage {
		toolLabels := fmt.Sprintf(`run_id="%s",tool="%s"`, data.Run.ID, tool)
		f.writeMetric(buf, "run_tool_usage_total", toolLabels, float64(count))
	}
}

func (f *PrometheusFormatter) formatStateMachineExport(buf *bytes.Buffer, data *inspector.StateMachineExport) {
	// State info
	f.writeHelp(buf, "state_info", "State machine state information")
	f.writeType(buf, "state_info", "gauge")
	for _, s := range data.States {
		var terminal, sideEffects float64
		if s.IsTerminal {
			terminal = 1
		}
		if s.AllowsSideEffects {
			sideEffects = 1
		}
		labels := fmt.Sprintf(`state="%s",is_terminal="%t",allows_side_effects="%t"`,
			s.Name, s.IsTerminal, s.AllowsSideEffects)
		f.writeMetric(buf, "state_info", labels, terminal+sideEffects)
	}

	// Transition counts
	f.writeHelp(buf, "state_transition_count", "Number of times a transition was taken")
	f.writeType(buf, "state_transition_count", "counter")
	for _, t := range data.Transitions {
		labels := fmt.Sprintf(`from="%s",to="%s"`, t.From, t.To)
		f.writeMetric(buf, "state_transition_count", labels, float64(t.Count))
	}

	// State counts
	f.writeHelp(buf, "state_machine_states_total", "Total number of states")
	f.writeType(buf, "state_machine_states_total", "gauge")
	f.writeMetric(buf, "state_machine_states_total", "", float64(len(data.States)))

	f.writeHelp(buf, "state_machine_transitions_total", "Total number of defined transitions")
	f.writeType(buf, "state_machine_transitions_total", "gauge")
	f.writeMetric(buf, "state_machine_transitions_total", "", float64(len(data.Transitions)))

	f.writeHelp(buf, "state_machine_terminal_states_total", "Number of terminal states")
	f.writeType(buf, "state_machine_terminal_states_total", "gauge")
	f.writeMetric(buf, "state_machine_terminal_states_total", "", float64(len(data.Terminal)))
}

func (f *PrometheusFormatter) formatMetricsExport(buf *bytes.Buffer, data *inspector.MetricsExport) {
	// Summary metrics
	f.writeHelp(buf, "runs_total", "Total number of runs")
	f.writeType(buf, "runs_total", "counter")
	f.writeMetric(buf, "runs_total", "", float64(data.Summary.TotalRuns))

	f.writeHelp(buf, "runs_completed_total", "Number of completed runs")
	f.writeType(buf, "runs_completed_total", "counter")
	f.writeMetric(buf, "runs_completed_total", "", float64(data.Summary.CompletedRuns))

	f.writeHelp(buf, "runs_failed_total", "Number of failed runs")
	f.writeType(buf, "runs_failed_total", "counter")
	f.writeMetric(buf, "runs_failed_total", "", float64(data.Summary.FailedRuns))

	f.writeHelp(buf, "runs_avg_duration_seconds", "Average run duration")
	f.writeType(buf, "runs_avg_duration_seconds", "gauge")
	f.writeMetric(buf, "runs_avg_duration_seconds", "", data.Summary.AverageDuration.Seconds())

	// Tool metrics
	f.writeHelp(buf, "tool_calls_total", "Total calls per tool")
	f.writeType(buf, "tool_calls_total", "counter")
	for _, tm := range data.ToolMetrics {
		labels := fmt.Sprintf(`tool="%s"`, tm.Name)
		f.writeMetric(buf, "tool_calls_total", labels, float64(tm.CallCount))
	}

	f.writeHelp(buf, "tool_success_total", "Successful calls per tool")
	f.writeType(buf, "tool_success_total", "counter")
	for _, tm := range data.ToolMetrics {
		labels := fmt.Sprintf(`tool="%s"`, tm.Name)
		f.writeMetric(buf, "tool_success_total", labels, float64(tm.SuccessCount))
	}

	f.writeHelp(buf, "tool_failure_total", "Failed calls per tool")
	f.writeType(buf, "tool_failure_total", "counter")
	for _, tm := range data.ToolMetrics {
		labels := fmt.Sprintf(`tool="%s"`, tm.Name)
		f.writeMetric(buf, "tool_failure_total", labels, float64(tm.FailureCount))
	}

	f.writeHelp(buf, "tool_success_rate", "Success rate per tool")
	f.writeType(buf, "tool_success_rate", "gauge")
	for _, tm := range data.ToolMetrics {
		labels := fmt.Sprintf(`tool="%s"`, tm.Name)
		f.writeMetric(buf, "tool_success_rate", labels, tm.SuccessRate)
	}

	f.writeHelp(buf, "tool_avg_duration_seconds", "Average duration per tool")
	f.writeType(buf, "tool_avg_duration_seconds", "gauge")
	for _, tm := range data.ToolMetrics {
		labels := fmt.Sprintf(`tool="%s"`, tm.Name)
		f.writeMetric(buf, "tool_avg_duration_seconds", labels, tm.AverageDuration.Seconds())
	}

	f.writeHelp(buf, "tool_p90_duration_seconds", "P90 duration per tool")
	f.writeType(buf, "tool_p90_duration_seconds", "gauge")
	for _, tm := range data.ToolMetrics {
		labels := fmt.Sprintf(`tool="%s"`, tm.Name)
		f.writeMetric(buf, "tool_p90_duration_seconds", labels, tm.P90Duration.Seconds())
	}

	// State metrics
	f.writeHelp(buf, "state_entries_total", "Number of times each state was entered")
	f.writeType(buf, "state_entries_total", "counter")
	for _, sm := range data.StateMetrics {
		labels := fmt.Sprintf(`state="%s"`, sm.State)
		f.writeMetric(buf, "state_entries_total", labels, float64(sm.EntryCount))
	}

	f.writeHelp(buf, "state_avg_time_seconds", "Average time in each state")
	f.writeType(buf, "state_avg_time_seconds", "gauge")
	for _, sm := range data.StateMetrics {
		labels := fmt.Sprintf(`state="%s"`, sm.State)
		f.writeMetric(buf, "state_avg_time_seconds", labels, sm.AverageTime.Seconds())
	}

	f.writeHelp(buf, "state_total_time_seconds", "Total time in each state")
	f.writeType(buf, "state_total_time_seconds", "counter")
	for _, sm := range data.StateMetrics {
		labels := fmt.Sprintf(`state="%s"`, sm.State)
		f.writeMetric(buf, "state_total_time_seconds", labels, sm.TotalTime.Seconds())
	}
}

func (f *PrometheusFormatter) writeHelp(buf *bytes.Buffer, name, help string) {
	fmt.Fprintf(buf, "# HELP %s %s\n", f.metricName(name), help)
}

func (f *PrometheusFormatter) writeType(buf *bytes.Buffer, name, metricType string) {
	fmt.Fprintf(buf, "# TYPE %s %s\n", f.metricName(name), metricType)
}

func (f *PrometheusFormatter) writeMetric(buf *bytes.Buffer, name, labels string, value float64) {
	metricName := f.metricName(name)
	if labels != "" {
		fmt.Fprintf(buf, "%s{%s} %g\n", metricName, labels, value)
	} else {
		fmt.Fprintf(buf, "%s %g\n", metricName, value)
	}
}

// FormatType returns the format type.
func (f *PrometheusFormatter) FormatType() inspector.ExportFormat {
	return inspector.FormatPrometheus
}

// Ensure PrometheusFormatter implements inspector.Formatter
var _ inspector.Formatter = (*PrometheusFormatter)(nil)

// FormatLabels formats labels for Prometheus output, sorting keys for consistency.
func FormatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, labels[k]))
	}
	return strings.Join(parts, ",")
}
