package inspector_test

import (
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/inspector"
)

func TestDefaultMetricsFilter(t *testing.T) {
	t.Parallel()

	filter := inspector.DefaultMetricsFilter()

	if !filter.IncludeToolMetrics {
		t.Error("DefaultMetricsFilter() IncludeToolMetrics should be true")
	}
	if !filter.IncludeStateMetrics {
		t.Error("DefaultMetricsFilter() IncludeStateMetrics should be true")
	}
	if !filter.FromTime.IsZero() {
		t.Error("DefaultMetricsFilter() FromTime should be zero")
	}
	if !filter.ToTime.IsZero() {
		t.Error("DefaultMetricsFilter() ToTime should be zero")
	}
}

func TestMetricsFilter(t *testing.T) {
	t.Parallel()

	now := time.Now()
	filter := inspector.MetricsFilter{
		FromTime:            now.Add(-24 * time.Hour),
		ToTime:              now,
		IncludeToolMetrics:  true,
		IncludeStateMetrics: false,
	}

	if filter.FromTime.IsZero() {
		t.Error("MetricsFilter FromTime should be set")
	}
	if filter.ToTime.IsZero() {
		t.Error("MetricsFilter ToTime should be set")
	}
	if !filter.IncludeToolMetrics {
		t.Error("MetricsFilter IncludeToolMetrics should be true")
	}
	if filter.IncludeStateMetrics {
		t.Error("MetricsFilter IncludeStateMetrics should be false")
	}
}

func TestExportFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format   inspector.ExportFormat
		expected string
	}{
		{inspector.FormatJSON, "json"},
		{inspector.FormatDOT, "dot"},
		{inspector.FormatTimeline, "timeline"},
		{inspector.FormatMermaid, "mermaid"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()

			if string(tt.format) != tt.expected {
				t.Errorf("ExportFormat = %s, want %s", tt.format, tt.expected)
			}
		})
	}
}

func TestRunExport(t *testing.T) {
	t.Parallel()

	t.Run("holds all export data", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		endTime := now.Add(5 * time.Second)

		export := inspector.RunExport{
			Run: inspector.RunMetadata{
				ID:        "run-123",
				Goal:      "process files",
				Status:    agent.RunStatusCompleted,
				State:     agent.StateDone,
				StartTime: now,
				EndTime:   &endTime,
				Result:    "success",
			},
			Timeline: []inspector.TimelineEntry{
				{Timestamp: now, Type: "started", Label: "Run started"},
			},
			ToolCalls: []inspector.ToolCallExport{
				{Name: "read_file", Success: true, Duration: time.Second},
			},
			Transitions: []inspector.TransitionExport{
				{From: agent.StateIntake, To: agent.StateExplore, Reason: "begin"},
			},
			Metrics: inspector.RunMetrics{
				TotalDuration: 5 * time.Second,
				ToolCallCount: 1,
			},
		}

		if export.Run.ID != "run-123" {
			t.Errorf("Run.ID = %s, want run-123", export.Run.ID)
		}
		if len(export.Timeline) != 1 {
			t.Errorf("Timeline len = %d, want 1", len(export.Timeline))
		}
		if len(export.ToolCalls) != 1 {
			t.Errorf("ToolCalls len = %d, want 1", len(export.ToolCalls))
		}
	})

	t.Run("marshals to JSON", func(t *testing.T) {
		t.Parallel()

		export := inspector.RunExport{
			Run: inspector.RunMetadata{
				ID:   "run-123",
				Goal: "test",
			},
		}

		data, err := json.Marshal(export)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var decoded inspector.RunExport
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if decoded.Run.ID != export.Run.ID {
			t.Errorf("decoded Run.ID = %s, want %s", decoded.Run.ID, export.Run.ID)
		}
	})
}

func TestRunMetadata(t *testing.T) {
	t.Parallel()

	now := time.Now()
	endTime := now.Add(time.Minute)

	metadata := inspector.RunMetadata{
		ID:        "run-456",
		Goal:      "analyze data",
		Status:    agent.RunStatusFailed,
		State:     agent.StateFailed,
		StartTime: now,
		EndTime:   &endTime,
		Error:     "something went wrong",
	}

	if metadata.ID != "run-456" {
		t.Errorf("ID = %s", metadata.ID)
	}
	if metadata.Status != agent.RunStatusFailed {
		t.Errorf("Status = %s", metadata.Status)
	}
	if metadata.Error != "something went wrong" {
		t.Errorf("Error = %s", metadata.Error)
	}
}

func TestTimelineEntry(t *testing.T) {
	t.Parallel()

	entry := inspector.TimelineEntry{
		Timestamp: time.Now(),
		Type:      "tool_call",
		Label:     "Called read_file",
		State:     agent.StateExplore,
		Duration:  500 * time.Millisecond,
		Details: map[string]any{
			"tool": "read_file",
			"path": "/test",
		},
	}

	if entry.Type != "tool_call" {
		t.Errorf("Type = %s", entry.Type)
	}
	if entry.State != agent.StateExplore {
		t.Errorf("State = %s", entry.State)
	}
	if entry.Details["tool"] != "read_file" {
		t.Errorf("Details[tool] = %v", entry.Details["tool"])
	}
}

func TestToolCallExport(t *testing.T) {
	t.Parallel()

	t.Run("successful tool call", func(t *testing.T) {
		t.Parallel()

		call := inspector.ToolCallExport{
			Name:      "read_file",
			Timestamp: time.Now(),
			State:     agent.StateExplore,
			Duration:  100 * time.Millisecond,
			Success:   true,
			Input:     `{"path":"/test"}`,
			Output:    `{"content":"hello"}`,
		}

		if !call.Success {
			t.Error("Success should be true")
		}
		if call.Error != "" {
			t.Error("Error should be empty for success")
		}
	})

	t.Run("failed tool call", func(t *testing.T) {
		t.Parallel()

		call := inspector.ToolCallExport{
			Name:     "write_file",
			Success:  false,
			Error:    "permission denied",
			Duration: 50 * time.Millisecond,
		}

		if call.Success {
			t.Error("Success should be false")
		}
		if call.Error != "permission denied" {
			t.Errorf("Error = %s", call.Error)
		}
	})
}

func TestTransitionExport(t *testing.T) {
	t.Parallel()

	transition := inspector.TransitionExport{
		Timestamp: time.Now(),
		From:      agent.StateIntake,
		To:        agent.StateExplore,
		Reason:    "begin exploration",
		Duration:  200 * time.Millisecond,
	}

	if transition.From != agent.StateIntake {
		t.Errorf("From = %s", transition.From)
	}
	if transition.To != agent.StateExplore {
		t.Errorf("To = %s", transition.To)
	}
	if transition.Reason != "begin exploration" {
		t.Errorf("Reason = %s", transition.Reason)
	}
}

func TestRunMetrics(t *testing.T) {
	t.Parallel()

	metrics := inspector.RunMetrics{
		TotalDuration:       30 * time.Second,
		ToolCallCount:       10,
		SuccessfulToolCalls: 8,
		FailedToolCalls:     2,
		TransitionCount:     5,
		TimeInState: map[agent.State]time.Duration{
			agent.StateExplore: 15 * time.Second,
			agent.StateAct:     10 * time.Second,
		},
		AverageToolDuration: 2 * time.Second,
		ToolUsage: map[string]int{
			"read_file":  6,
			"write_file": 4,
		},
	}

	if metrics.ToolCallCount != 10 {
		t.Errorf("ToolCallCount = %d", metrics.ToolCallCount)
	}
	if metrics.SuccessfulToolCalls+metrics.FailedToolCalls != metrics.ToolCallCount {
		t.Error("Successful + Failed should equal total")
	}
	if metrics.TimeInState[agent.StateExplore] != 15*time.Second {
		t.Errorf("TimeInState[explore] = %v", metrics.TimeInState[agent.StateExplore])
	}
	if metrics.ToolUsage["read_file"] != 6 {
		t.Errorf("ToolUsage[read_file] = %d", metrics.ToolUsage["read_file"])
	}
}

func TestStateMachineExport(t *testing.T) {
	t.Parallel()

	export := inspector.StateMachineExport{
		States: []inspector.StateExport{
			{Name: agent.StateIntake, Description: "Initial state", IsTerminal: false},
			{Name: agent.StateDone, Description: "Success", IsTerminal: true},
		},
		Transitions: []inspector.StateMachineTransition{
			{From: agent.StateIntake, To: agent.StateExplore, Label: "begin"},
		},
		Initial:  agent.StateIntake,
		Terminal: []agent.State{agent.StateDone, agent.StateFailed},
	}

	if len(export.States) != 2 {
		t.Errorf("States len = %d", len(export.States))
	}
	if export.Initial != agent.StateIntake {
		t.Errorf("Initial = %s", export.Initial)
	}
	if len(export.Terminal) != 2 {
		t.Errorf("Terminal len = %d", len(export.Terminal))
	}
}

func TestStateExport(t *testing.T) {
	t.Parallel()

	state := inspector.StateExport{
		Name:              agent.StateAct,
		Description:       "Perform side effects",
		IsTerminal:        false,
		AllowsSideEffects: true,
		EligibleTools:     []string{"write_file", "delete_file"},
	}

	if state.Name != agent.StateAct {
		t.Errorf("Name = %s", state.Name)
	}
	if !state.AllowsSideEffects {
		t.Error("AllowsSideEffects should be true for act state")
	}
	if len(state.EligibleTools) != 2 {
		t.Errorf("EligibleTools len = %d", len(state.EligibleTools))
	}
}

func TestStateMachineTransition(t *testing.T) {
	t.Parallel()

	transition := inspector.StateMachineTransition{
		From:  agent.StateExplore,
		To:    agent.StateDecide,
		Label: "gathered evidence",
		Count: 42,
	}

	if transition.From != agent.StateExplore {
		t.Errorf("From = %s", transition.From)
	}
	if transition.Count != 42 {
		t.Errorf("Count = %d", transition.Count)
	}
}

func TestMetricsExport(t *testing.T) {
	t.Parallel()

	now := time.Now()
	export := inspector.MetricsExport{}
	export.Period.From = now.Add(-24 * time.Hour)
	export.Period.To = now
	export.Summary.TotalRuns = 100
	export.Summary.CompletedRuns = 90
	export.Summary.FailedRuns = 10
	export.Summary.AverageDuration = 30 * time.Second
	export.ToolMetrics = []inspector.ToolMetricsExport{
		{Name: "read_file", CallCount: 500, SuccessRate: 0.98},
	}
	export.StateMetrics = []inspector.StateMetricsExport{
		{State: agent.StateExplore, EntryCount: 200},
	}

	if export.Summary.TotalRuns != 100 {
		t.Errorf("Summary.TotalRuns = %d", export.Summary.TotalRuns)
	}
	if len(export.ToolMetrics) != 1 {
		t.Errorf("ToolMetrics len = %d", len(export.ToolMetrics))
	}
	if len(export.StateMetrics) != 1 {
		t.Errorf("StateMetrics len = %d", len(export.StateMetrics))
	}
}

func TestToolMetricsExport(t *testing.T) {
	t.Parallel()

	metrics := inspector.ToolMetricsExport{
		Name:            "read_file",
		CallCount:       1000,
		SuccessCount:    980,
		FailureCount:    20,
		SuccessRate:     0.98,
		AverageDuration: 100 * time.Millisecond,
		P90Duration:     250 * time.Millisecond,
	}

	if metrics.CallCount != metrics.SuccessCount+metrics.FailureCount {
		t.Error("CallCount should equal SuccessCount + FailureCount")
	}
	if metrics.SuccessRate != 0.98 {
		t.Errorf("SuccessRate = %f", metrics.SuccessRate)
	}
}

func TestStateMetricsExport(t *testing.T) {
	t.Parallel()

	metrics := inspector.StateMetricsExport{
		State:       agent.StateExplore,
		EntryCount:  500,
		AverageTime: 5 * time.Second,
		TotalTime:   2500 * time.Second,
	}

	if metrics.State != agent.StateExplore {
		t.Errorf("State = %s", metrics.State)
	}
	if metrics.EntryCount != 500 {
		t.Errorf("EntryCount = %d", metrics.EntryCount)
	}
}

func TestDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrRunNotFound",
			err:  inspector.ErrRunNotFound,
			msg:  "run not found",
		},
		{
			name: "ErrInvalidFormat",
			err:  inspector.ErrInvalidFormat,
			msg:  "invalid export format",
		},
		{
			name: "ErrExportFailed",
			err:  inspector.ErrExportFailed,
			msg:  "export failed",
		},
		{
			name: "ErrNoData",
			err:  inspector.ErrNoData,
			msg:  "no data to export",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err.Error() != tt.msg {
				t.Errorf("%s.Error() = %s, want %s", tt.name, tt.err.Error(), tt.msg)
			}
		})
	}
}
