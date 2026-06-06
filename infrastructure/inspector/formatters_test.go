package inspector

import (
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/inspector"
)

func TestHTMLFormatter(t *testing.T) {
	t.Run("formats run export as HTML", func(t *testing.T) {
		f := NewHTMLFormatter(WithTitle("Test Run"), WithTheme("dark"))

		data := &inspector.RunExport{
			Run: inspector.RunMetadata{
				ID:        "run-123",
				Goal:      "Test goal",
				Status:    agent.RunStatusRunning,
				State:     agent.StateExplore,
				StartTime: time.Now(),
			},
			Timeline: []inspector.TimelineEntry{
				{
					Timestamp: time.Now(),
					Type:      "RunStarted",
					Label:     "Run Started",
					State:     agent.StateIntake,
				},
			},
			ToolCalls: []inspector.ToolCallExport{
				{
					Name:      "test_tool",
					Timestamp: time.Now(),
					State:     agent.StateExplore,
					Duration:  100 * time.Millisecond,
					Success:   true,
				},
			},
			Transitions: []inspector.TransitionExport{
				{
					Timestamp: time.Now(),
					From:      agent.StateIntake,
					To:        agent.StateExplore,
					Reason:    "begin exploration",
				},
			},
			Metrics: inspector.RunMetrics{
				TotalDuration: 5 * time.Second,
				ToolCallCount: 1,
			},
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		html := string(result)
		if !strings.Contains(html, "<!DOCTYPE html>") {
			t.Error("expected HTML doctype")
		}
		if !strings.Contains(html, "Test Run") {
			t.Error("expected title in HTML")
		}
		if !strings.Contains(html, "#1a1a2e") {
			t.Error("expected dark theme colors")
		}
		if !strings.Contains(html, "run-123") {
			t.Error("expected run ID in data")
		}
	})

	t.Run("format type is HTML", func(t *testing.T) {
		f := NewHTMLFormatter()
		if f.FormatType() != inspector.FormatHTML {
			t.Errorf("expected FormatHTML, got %s", f.FormatType())
		}
	})
}

func TestCSVFormatter(t *testing.T) {
	t.Run("formats run export as CSV", func(t *testing.T) {
		f := NewCSVFormatter(WithCSVHeaders())

		data := &inspector.RunExport{
			Run: inspector.RunMetadata{
				ID:     "run-456",
				Goal:   "CSV test",
				Status: agent.RunStatusCompleted,
			},
			ToolCalls: []inspector.ToolCallExport{
				{
					Name:      "tool_a",
					Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					State:     agent.StateAct,
					Duration:  50 * time.Millisecond,
					Success:   true,
				},
				{
					Name:      "tool_b",
					Timestamp: time.Date(2024, 1, 1, 12, 0, 1, 0, time.UTC),
					State:     agent.StateAct,
					Duration:  100 * time.Millisecond,
					Success:   false,
					Error:     "test error",
				},
			},
			Transitions: []inspector.TransitionExport{
				{
					Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					From:      agent.StateIntake,
					To:        agent.StateExplore,
					Reason:    "start",
				},
			},
			Timeline: []inspector.TimelineEntry{},
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		csv := string(result)
		if !strings.Contains(csv, "# Run Export: run-456") {
			t.Error("expected run ID comment")
		}
		if !strings.Contains(csv, "# TOOL CALLS") {
			t.Error("expected tool calls section")
		}
		if !strings.Contains(csv, "tool_a") {
			t.Error("expected tool_a in output")
		}
		if !strings.Contains(csv, "# STATE TRANSITIONS") {
			t.Error("expected transitions section")
		}
	})

	t.Run("formats state machine export as CSV", func(t *testing.T) {
		f := NewCSVFormatter()

		data := &inspector.StateMachineExport{
			States: []inspector.StateExport{
				{Name: agent.StateIntake, IsTerminal: false, AllowsSideEffects: false},
				{Name: agent.StateDone, IsTerminal: true, AllowsSideEffects: false},
			},
			Transitions: []inspector.StateMachineTransition{
				{From: agent.StateIntake, To: agent.StateExplore, Label: "begin"},
			},
			Initial:  agent.StateIntake,
			Terminal: []agent.State{agent.StateDone, agent.StateFailed},
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		csv := string(result)
		if !strings.Contains(csv, "# STATES") {
			t.Error("expected states section")
		}
		if !strings.Contains(csv, "intake") {
			t.Error("expected intake state")
		}
	})

	t.Run("formats metrics export as CSV", func(t *testing.T) {
		f := NewCSVFormatter()

		data := &inspector.MetricsExport{
			Summary: struct {
				TotalRuns       int64         `json:"total_runs"`
				CompletedRuns   int64         `json:"completed_runs"`
				FailedRuns      int64         `json:"failed_runs"`
				AverageDuration time.Duration `json:"average_duration"`
			}{
				TotalRuns:       100,
				CompletedRuns:   90,
				FailedRuns:      10,
				AverageDuration: 5 * time.Second,
			},
			ToolMetrics: []inspector.ToolMetricsExport{
				{
					Name:            "test_tool",
					CallCount:       50,
					SuccessCount:    45,
					FailureCount:    5,
					SuccessRate:     0.9,
					AverageDuration: 100 * time.Millisecond,
				},
			},
			StateMetrics: []inspector.StateMetricsExport{
				{State: agent.StateExplore, EntryCount: 100, AverageTime: time.Second},
			},
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		csv := string(result)
		if !strings.Contains(csv, "# SUMMARY") {
			t.Error("expected summary section")
		}
		if !strings.Contains(csv, "100") {
			t.Error("expected total runs value")
		}
	})

	t.Run("custom delimiter", func(t *testing.T) {
		f := NewCSVFormatter(WithDelimiter('\t'))
		// The delimiter is set, which affects how fields are separated
		if f.delimiter != '\t' {
			t.Error("expected tab delimiter")
		}
	})

	t.Run("format type is CSV", func(t *testing.T) {
		f := NewCSVFormatter()
		if f.FormatType() != inspector.FormatCSV {
			t.Errorf("expected FormatCSV, got %s", f.FormatType())
		}
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		f := NewCSVFormatter()
		_, err := f.Format("invalid")
		if err == nil {
			t.Error("expected error for unsupported type")
		}
	})
}

func TestPrometheusFormatter(t *testing.T) {
	t.Run("formats run export as Prometheus", func(t *testing.T) {
		f := NewPrometheusFormatter(WithNamespace("test"), WithSubsystem("agent"))

		data := &inspector.RunExport{
			Run: inspector.RunMetadata{
				ID:     "run-789",
				Status: agent.RunStatusCompleted,
				State:  agent.StateDone,
			},
			Metrics: inspector.RunMetrics{
				TotalDuration:       10 * time.Second,
				ToolCallCount:       5,
				SuccessfulToolCalls: 4,
				FailedToolCalls:     1,
				TransitionCount:     8,
				TimeInState: map[agent.State]time.Duration{
					agent.StateExplore: 3 * time.Second,
					agent.StateAct:     5 * time.Second,
				},
				ToolUsage: map[string]int{
					"read_file":  3,
					"write_file": 2,
				},
			},
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		prom := string(result)
		if !strings.Contains(prom, "# HELP test_agent_run_duration_seconds") {
			t.Error("expected duration help")
		}
		if !strings.Contains(prom, "# TYPE test_agent_run_duration_seconds gauge") {
			t.Error("expected duration type")
		}
		if !strings.Contains(prom, "test_agent_run_tool_calls_total") {
			t.Error("expected tool calls metric")
		}
		if !strings.Contains(prom, `run_id="run-789"`) {
			t.Error("expected run_id label")
		}
	})

	t.Run("formats state machine export as Prometheus", func(t *testing.T) {
		f := NewPrometheusFormatter()

		data := &inspector.StateMachineExport{
			States: []inspector.StateExport{
				{Name: agent.StateIntake, IsTerminal: false, AllowsSideEffects: false},
				{Name: agent.StateDone, IsTerminal: true, AllowsSideEffects: false},
			},
			Transitions: []inspector.StateMachineTransition{
				{From: agent.StateIntake, To: agent.StateExplore, Count: 10},
			},
			Initial:  agent.StateIntake,
			Terminal: []agent.State{agent.StateDone},
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		prom := string(result)
		if !strings.Contains(prom, "agent_state_info") {
			t.Error("expected state_info metric")
		}
		if !strings.Contains(prom, "agent_state_transition_count") {
			t.Error("expected transition_count metric")
		}
	})

	t.Run("formats metrics export as Prometheus", func(t *testing.T) {
		f := NewPrometheusFormatter()

		data := &inspector.MetricsExport{
			Summary: struct {
				TotalRuns       int64         `json:"total_runs"`
				CompletedRuns   int64         `json:"completed_runs"`
				FailedRuns      int64         `json:"failed_runs"`
				AverageDuration time.Duration `json:"average_duration"`
			}{
				TotalRuns:     100,
				CompletedRuns: 90,
				FailedRuns:    10,
			},
			ToolMetrics: []inspector.ToolMetricsExport{
				{Name: "test_tool", CallCount: 50, SuccessRate: 0.9},
			},
			StateMetrics: []inspector.StateMetricsExport{
				{State: agent.StateExplore, EntryCount: 100},
			},
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		prom := string(result)
		if !strings.Contains(prom, "agent_runs_total 100") {
			t.Error("expected runs_total metric")
		}
		if !strings.Contains(prom, "agent_tool_calls_total") {
			t.Error("expected tool_calls_total metric")
		}
	})

	t.Run("format type is Prometheus", func(t *testing.T) {
		f := NewPrometheusFormatter()
		if f.FormatType() != inspector.FormatPrometheus {
			t.Errorf("expected FormatPrometheus, got %s", f.FormatType())
		}
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		f := NewPrometheusFormatter()
		_, err := f.Format("invalid")
		if err == nil {
			t.Error("expected error for unsupported type")
		}
	})
}

func TestXStateFormatter(t *testing.T) {
	t.Run("formats state machine as XState JSON", func(t *testing.T) {
		f := NewXStateFormatter(
			WithMachineID("testMachine"),
			WithVersion("1.0"),
			WithXStatePretty(),
		)

		data := &inspector.StateMachineExport{
			States: []inspector.StateExport{
				{
					Name:              agent.StateIntake,
					Description:       "Intake state",
					IsTerminal:        false,
					AllowsSideEffects: false,
					EligibleTools:     []string{},
				},
				{
					Name:              agent.StateExplore,
					Description:       "Explore state",
					IsTerminal:        false,
					AllowsSideEffects: false,
					EligibleTools:     []string{"read_file"},
				},
				{
					Name:              agent.StateAct,
					Description:       "Act state",
					IsTerminal:        false,
					AllowsSideEffects: true,
					EligibleTools:     []string{"write_file"},
				},
				{
					Name:              agent.StateDone,
					Description:       "Done state",
					IsTerminal:        true,
					AllowsSideEffects: false,
				},
			},
			Transitions: []inspector.StateMachineTransition{
				{From: agent.StateIntake, To: agent.StateExplore, Label: "EXPLORE"},
				{From: agent.StateExplore, To: agent.StateAct, Label: "ACT"},
				{From: agent.StateAct, To: agent.StateDone, Label: "DONE"},
			},
			Initial:  agent.StateIntake,
			Terminal: []agent.State{agent.StateDone},
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		xstate := string(result)
		if !strings.Contains(xstate, `"id": "testMachine"`) {
			t.Error("expected machine ID")
		}
		if !strings.Contains(xstate, `"version": "1.0"`) {
			t.Error("expected version")
		}
		if !strings.Contains(xstate, `"initial": "intake"`) {
			t.Error("expected initial state")
		}
		if !strings.Contains(xstate, `"type": "final"`) {
			t.Error("expected final state type for terminal states")
		}
		if !strings.Contains(xstate, `"EXPLORE"`) {
			t.Error("expected transition event name")
		}
		if !strings.Contains(xstate, `"allowsSideEffects": true`) {
			t.Error("expected allowsSideEffects meta")
		}
	})

	t.Run("generates default event names when label is empty", func(t *testing.T) {
		f := NewXStateFormatter()

		data := &inspector.StateMachineExport{
			States: []inspector.StateExport{
				{Name: agent.StateIntake},
				{Name: agent.StateExplore},
			},
			Transitions: []inspector.StateMachineTransition{
				{From: agent.StateIntake, To: agent.StateExplore, Label: ""}, // No label
			},
			Initial: agent.StateIntake,
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		xstate := string(result)
		if !strings.Contains(xstate, "TO_explore") {
			t.Error("expected generated event name TO_explore")
		}
	})

	t.Run("format type is XState", func(t *testing.T) {
		f := NewXStateFormatter()
		if f.FormatType() != inspector.FormatXState {
			t.Errorf("expected FormatXState, got %s", f.FormatType())
		}
	})

	t.Run("returns error for non-StateMachineExport", func(t *testing.T) {
		f := NewXStateFormatter()
		_, err := f.Format(&inspector.RunExport{})
		if err == nil {
			t.Error("expected error for RunExport")
		}
	})

	t.Run("compact output without pretty print", func(t *testing.T) {
		f := &XStateFormatter{
			machineID: "test",
			version:   "1.0",
			pretty:    false,
		}

		data := &inspector.StateMachineExport{
			States:  []inspector.StateExport{{Name: agent.StateIntake}},
			Initial: agent.StateIntake,
		}

		result, err := f.Format(data)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		// Compact JSON should not have newlines between elements
		if strings.Contains(string(result), "\n  ") {
			t.Error("expected compact JSON without indentation")
		}
	})
}

func TestFormatLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "empty labels",
			labels: map[string]string{},
			want:   "",
		},
		{
			name:   "single label",
			labels: map[string]string{"tool": "read_file"},
			want:   `tool="read_file"`,
		},
		{
			name:   "multiple labels sorted",
			labels: map[string]string{"tool": "read", "state": "act"},
			want:   `state="act",tool="read"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLabels(tt.labels)
			if got != tt.want {
				t.Errorf("FormatLabels() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestXStateVisualizerURL(t *testing.T) {
	url := XStateVisualizerURL()
	if url != "https://stately.ai/viz" {
		t.Errorf("expected stately.ai URL, got %s", url)
	}
}
