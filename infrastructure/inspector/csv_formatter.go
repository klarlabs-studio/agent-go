package inspector

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"go.klarlabs.de/agent/domain/inspector"
)

// CSVFormatter formats data as CSV.
type CSVFormatter struct {
	includeHeaders bool
	delimiter      rune
}

// CSVFormatterOption configures the CSV formatter.
type CSVFormatterOption func(*CSVFormatter)

// WithCSVHeaders includes headers in the output.
func WithCSVHeaders() CSVFormatterOption {
	return func(f *CSVFormatter) {
		f.includeHeaders = true
	}
}

// WithDelimiter sets a custom delimiter (default is comma).
func WithDelimiter(d rune) CSVFormatterOption {
	return func(f *CSVFormatter) {
		f.delimiter = d
	}
}

// NewCSVFormatter creates a new CSV formatter.
func NewCSVFormatter(opts ...CSVFormatterOption) *CSVFormatter {
	f := &CSVFormatter{
		includeHeaders: true,
		delimiter:      ',',
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Format formats the data as CSV.
func (f *CSVFormatter) Format(data any) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = f.delimiter

	switch d := data.(type) {
	case *inspector.RunExport:
		if err := f.formatRunExport(w, d); err != nil {
			return nil, err
		}
	case *inspector.StateMachineExport:
		if err := f.formatStateMachineExport(w, d); err != nil {
			return nil, err
		}
	case *inspector.MetricsExport:
		if err := f.formatMetricsExport(w, d); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported data type for CSV: %T", data)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv write error: %w", err)
	}

	return buf.Bytes(), nil
}

func (f *CSVFormatter) formatRunExport(w *csv.Writer, data *inspector.RunExport) error {
	// Write run metadata as comments
	if err := w.Write([]string{"# Run Export: " + data.Run.ID}); err != nil {
		return err
	}
	if err := w.Write([]string{"# Goal: " + data.Run.Goal}); err != nil {
		return err
	}
	if err := w.Write([]string{"# Status: " + string(data.Run.Status)}); err != nil {
		return err
	}
	if err := w.Write([]string{""}); err != nil {
		return err
	}

	// Tool calls section
	if err := w.Write([]string{"# TOOL CALLS"}); err != nil {
		return err
	}
	if f.includeHeaders {
		if err := w.Write([]string{"name", "timestamp", "state", "duration_ms", "success", "error"}); err != nil {
			return err
		}
	}
	for _, tc := range data.ToolCalls {
		record := []string{
			tc.Name,
			tc.Timestamp.Format(time.RFC3339),
			string(tc.State),
			strconv.FormatInt(tc.Duration.Milliseconds(), 10),
			strconv.FormatBool(tc.Success),
			tc.Error,
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	if err := w.Write([]string{""}); err != nil {
		return err
	}

	// Transitions section
	if err := w.Write([]string{"# STATE TRANSITIONS"}); err != nil {
		return err
	}
	if f.includeHeaders {
		if err := w.Write([]string{"timestamp", "from", "to", "reason", "duration_ms"}); err != nil {
			return err
		}
	}
	for _, t := range data.Transitions {
		record := []string{
			t.Timestamp.Format(time.RFC3339),
			string(t.From),
			string(t.To),
			t.Reason,
			strconv.FormatInt(t.Duration.Milliseconds(), 10),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	if err := w.Write([]string{""}); err != nil {
		return err
	}

	// Timeline section
	if err := w.Write([]string{"# TIMELINE"}); err != nil {
		return err
	}
	if f.includeHeaders {
		if err := w.Write([]string{"timestamp", "type", "label", "state", "duration_ms"}); err != nil {
			return err
		}
	}
	for _, entry := range data.Timeline {
		record := []string{
			entry.Timestamp.Format(time.RFC3339),
			entry.Type,
			entry.Label,
			string(entry.State),
			strconv.FormatInt(entry.Duration.Milliseconds(), 10),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func (f *CSVFormatter) formatStateMachineExport(w *csv.Writer, data *inspector.StateMachineExport) error {
	// States section
	if err := w.Write([]string{"# STATES"}); err != nil {
		return err
	}
	if f.includeHeaders {
		if err := w.Write([]string{"name", "is_terminal", "allows_side_effects", "description"}); err != nil {
			return err
		}
	}
	for _, s := range data.States {
		record := []string{
			string(s.Name),
			strconv.FormatBool(s.IsTerminal),
			strconv.FormatBool(s.AllowsSideEffects),
			s.Description,
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	if err := w.Write([]string{""}); err != nil {
		return err
	}

	// Transitions section
	if err := w.Write([]string{"# TRANSITIONS"}); err != nil {
		return err
	}
	if f.includeHeaders {
		if err := w.Write([]string{"from", "to", "label", "count"}); err != nil {
			return err
		}
	}
	for _, t := range data.Transitions {
		record := []string{
			string(t.From),
			string(t.To),
			t.Label,
			strconv.Itoa(t.Count),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func (f *CSVFormatter) formatMetricsExport(w *csv.Writer, data *inspector.MetricsExport) error {
	// Summary section
	if err := w.Write([]string{"# SUMMARY"}); err != nil {
		return err
	}
	if f.includeHeaders {
		if err := w.Write([]string{"metric", "value"}); err != nil {
			return err
		}
	}
	summaryRows := [][]string{
		{"total_runs", strconv.FormatInt(data.Summary.TotalRuns, 10)},
		{"completed_runs", strconv.FormatInt(data.Summary.CompletedRuns, 10)},
		{"failed_runs", strconv.FormatInt(data.Summary.FailedRuns, 10)},
		{"average_duration_ms", strconv.FormatInt(data.Summary.AverageDuration.Milliseconds(), 10)},
	}
	for _, row := range summaryRows {
		if err := w.Write(row); err != nil {
			return err
		}
	}

	if err := w.Write([]string{""}); err != nil {
		return err
	}

	// Tool metrics section
	if err := w.Write([]string{"# TOOL METRICS"}); err != nil {
		return err
	}
	if f.includeHeaders {
		if err := w.Write([]string{"name", "call_count", "success_count", "failure_count", "success_rate", "avg_duration_ms", "p90_duration_ms"}); err != nil {
			return err
		}
	}
	for _, tm := range data.ToolMetrics {
		record := []string{
			tm.Name,
			strconv.FormatInt(tm.CallCount, 10),
			strconv.FormatInt(tm.SuccessCount, 10),
			strconv.FormatInt(tm.FailureCount, 10),
			strconv.FormatFloat(tm.SuccessRate, 'f', 4, 64),
			strconv.FormatInt(tm.AverageDuration.Milliseconds(), 10),
			strconv.FormatInt(tm.P90Duration.Milliseconds(), 10),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	if err := w.Write([]string{""}); err != nil {
		return err
	}

	// State metrics section
	if err := w.Write([]string{"# STATE METRICS"}); err != nil {
		return err
	}
	if f.includeHeaders {
		if err := w.Write([]string{"state", "entry_count", "avg_time_ms", "total_time_ms"}); err != nil {
			return err
		}
	}
	for _, sm := range data.StateMetrics {
		record := []string{
			string(sm.State),
			strconv.FormatInt(sm.EntryCount, 10),
			strconv.FormatInt(sm.AverageTime.Milliseconds(), 10),
			strconv.FormatInt(sm.TotalTime.Milliseconds(), 10),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// FormatType returns the format type.
func (f *CSVFormatter) FormatType() inspector.ExportFormat {
	return inspector.FormatCSV
}

// Ensure CSVFormatter implements inspector.Formatter
var _ inspector.Formatter = (*CSVFormatter)(nil)
