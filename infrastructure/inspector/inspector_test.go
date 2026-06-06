package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/inspector"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// ===============================
// JSON Formatter Tests
// ===============================

func TestNewJSONFormatter_Default(t *testing.T) {
	formatter := NewJSONFormatter()

	if formatter == nil {
		t.Fatal("expected non-nil formatter")
	}
	if formatter.pretty {
		t.Error("expected pretty=false by default")
	}
}

func TestNewJSONFormatter_WithPrettyPrint(t *testing.T) {
	formatter := NewJSONFormatter(WithPrettyPrint())

	if !formatter.pretty {
		t.Error("expected pretty=true with option")
	}
}

func TestJSONFormatter_FormatType(t *testing.T) {
	formatter := NewJSONFormatter()

	if formatter.FormatType() != inspector.FormatJSON {
		t.Errorf("expected FormatJSON, got %s", formatter.FormatType())
	}
}

func TestJSONFormatter_Format_Simple(t *testing.T) {
	formatter := NewJSONFormatter()
	data := map[string]string{"key": "value"}

	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result) != `{"key":"value"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestJSONFormatter_Format_PrettyPrint(t *testing.T) {
	formatter := NewJSONFormatter(WithPrettyPrint())
	data := map[string]string{"key": "value"}

	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "{\n  \"key\": \"value\"\n}"
	if string(result) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

// ===============================
// DOT Formatter Tests
// ===============================

func TestNewDOTFormatter(t *testing.T) {
	formatter := NewDOTFormatter()

	if formatter == nil {
		t.Fatal("expected non-nil formatter")
	}
}

func TestDOTFormatter_FormatType(t *testing.T) {
	formatter := NewDOTFormatter()

	if formatter.FormatType() != inspector.FormatDOT {
		t.Errorf("expected FormatDOT, got %s", formatter.FormatType())
	}
}

func TestDOTFormatter_Format_ValidStateMachine(t *testing.T) {
	formatter := NewDOTFormatter()
	sm := &inspector.StateMachineExport{
		States: []inspector.StateExport{
			{Name: agent.StateIntake, Description: "Start"},
			{Name: agent.StateDone, Description: "End", IsTerminal: true},
		},
		Transitions: []inspector.StateMachineTransition{
			{From: agent.StateIntake, To: agent.StateDone, Label: "finish"},
		},
		Initial:  agent.StateIntake,
		Terminal: []agent.State{agent.StateDone},
	}

	result, err := formatter.Format(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "digraph AgentStateMachine") {
		t.Error("expected digraph declaration")
	}
	if !strings.Contains(resultStr, "intake") {
		t.Error("expected intake state")
	}
	if !strings.Contains(resultStr, "done") {
		t.Error("expected done state")
	}
	if !strings.Contains(resultStr, "->") {
		t.Error("expected transition arrow")
	}
}

func TestDOTFormatter_Format_InvalidInput(t *testing.T) {
	formatter := NewDOTFormatter()

	_, err := formatter.Format("not a state machine")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestDOTFormatter_Format_TerminalStateColors(t *testing.T) {
	formatter := NewDOTFormatter()
	sm := &inspector.StateMachineExport{
		States: []inspector.StateExport{
			{Name: "done", IsTerminal: true},
			{Name: "failed", IsTerminal: true},
		},
	}

	result, err := formatter.Format(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "lightgreen") {
		t.Error("expected lightgreen for done state")
	}
	if !strings.Contains(resultStr, "lightcoral") {
		t.Error("expected lightcoral for failed state")
	}
}

func TestDOTFormatter_Format_SideEffectState(t *testing.T) {
	formatter := NewDOTFormatter()
	sm := &inspector.StateMachineExport{
		States: []inspector.StateExport{
			{Name: agent.StateAct, AllowsSideEffects: true},
		},
	}

	result, err := formatter.Format(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "lightyellow") {
		t.Error("expected lightyellow for side-effect state")
	}
}

func TestSanitizeDOTID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with_dash"},
		{"with.dot", "with_dot"},
		{"with space", "with_space"},
		{"mixed-case.test id", "mixed_case_test_id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeDOTID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeDOTID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ===============================
// Mermaid Formatter Tests
// ===============================

func TestNewMermaidFormatter(t *testing.T) {
	formatter := NewMermaidFormatter()

	if formatter == nil {
		t.Fatal("expected non-nil formatter")
	}
}

func TestMermaidFormatter_FormatType(t *testing.T) {
	formatter := NewMermaidFormatter()

	if formatter.FormatType() != inspector.FormatMermaid {
		t.Errorf("expected FormatMermaid, got %s", formatter.FormatType())
	}
}

func TestMermaidFormatter_Format_ValidStateMachine(t *testing.T) {
	formatter := NewMermaidFormatter()
	sm := &inspector.StateMachineExport{
		States: []inspector.StateExport{
			{Name: agent.StateIntake, Description: "Start"},
			{Name: agent.StateAct, AllowsSideEffects: true},
			{Name: agent.StateDone, Description: "End", IsTerminal: true},
		},
		Transitions: []inspector.StateMachineTransition{
			{From: agent.StateIntake, To: agent.StateAct, Label: "proceed"},
			{From: agent.StateAct, To: agent.StateDone},
		},
		Initial:  agent.StateIntake,
		Terminal: []agent.State{agent.StateDone},
	}

	result, err := formatter.Format(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "stateDiagram-v2") {
		t.Error("expected stateDiagram-v2 declaration")
	}
	if !strings.Contains(resultStr, "[*] --> intake") {
		t.Error("expected initial state marker")
	}
	if !strings.Contains(resultStr, "done --> [*]") {
		t.Error("expected terminal state marker")
	}
	if !strings.Contains(resultStr, "Side effects allowed") {
		t.Error("expected side effects note")
	}
}

func TestMermaidFormatter_Format_InvalidInput(t *testing.T) {
	formatter := NewMermaidFormatter()

	_, err := formatter.Format("not a state machine")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

// ===============================
// Run Exporter Tests
// ===============================

func TestNewRunExporter(t *testing.T) {
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	exporter := NewRunExporter(runStore, eventStore)

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}
}

func TestRunExporter_Export_ValidRun(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	// Create and save a run
	r := agent.NewRun("run-1", "test goal")
	r.Status = agent.RunStatusCompleted
	r.EndTime = time.Now()
	r.Result = json.RawMessage(`{"result":"success"}`)
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Add some events
	addTestEvents(ctx, t, eventStore, r.ID)

	exporter := NewRunExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if export.Run.ID != r.ID {
		t.Errorf("expected run ID %s, got %s", r.ID, export.Run.ID)
	}
	if export.Run.Goal != "test goal" {
		t.Errorf("expected goal 'test goal', got %s", export.Run.Goal)
	}
	if export.Run.Status != agent.RunStatusCompleted {
		t.Errorf("expected status Completed, got %s", export.Run.Status)
	}
}

func TestRunExporter_Export_RunNotFound(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	exporter := NewRunExporter(runStore, eventStore)
	_, err := exporter.Export(ctx, "nonexistent")

	if err == nil {
		t.Error("expected error for nonexistent run")
	}
}

func TestRunExporter_Export_BuildsTimeline(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	r := agent.NewRun("run-1", "test goal")
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	addTestEvents(ctx, t, eventStore, r.ID)

	exporter := NewRunExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(export.Timeline) == 0 {
		t.Error("expected non-empty timeline")
	}
}

func TestRunExporter_Export_BuildsToolCalls(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	r := agent.NewRun("run-1", "test goal")
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	addToolCallEvents(ctx, t, eventStore, r.ID)

	exporter := NewRunExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(export.ToolCalls) == 0 {
		t.Error("expected non-empty tool calls")
	}

	// Verify tool call has success/failure status
	for _, tc := range export.ToolCalls {
		if tc.Name == "" {
			t.Error("expected tool call to have name")
		}
	}
}

func TestRunExporter_Export_BuildsTransitions(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	r := agent.NewRun("run-1", "test goal")
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	addTransitionEvents(ctx, t, eventStore, r.ID)

	exporter := NewRunExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(export.Transitions) == 0 {
		t.Error("expected non-empty transitions")
	}
}

func TestRunExporter_Export_BuildsMetrics(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	r := agent.NewRun("run-1", "test goal")
	r.Status = agent.RunStatusCompleted
	r.EndTime = time.Now().Add(time.Minute)
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	addToolCallEvents(ctx, t, eventStore, r.ID)
	addTransitionEvents(ctx, t, eventStore, r.ID)

	exporter := NewRunExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, r.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if export.Metrics.TotalDuration == 0 {
		t.Error("expected non-zero total duration")
	}
	if export.Metrics.TimeInState == nil {
		t.Error("expected time in state map")
	}
	if export.Metrics.ToolUsage == nil {
		t.Error("expected tool usage map")
	}
}

// ===============================
// State Machine Exporter Tests
// ===============================

func TestNewStateMachineExporter(t *testing.T) {
	exporter := NewStateMachineExporter(nil, nil)

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}
}

func TestStateMachineExporter_Export_DefaultTransitions(t *testing.T) {
	ctx := context.Background()
	exporter := NewStateMachineExporter(nil, nil)

	export, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if export.Initial != agent.StateIntake {
		t.Errorf("expected initial state intake, got %s", export.Initial)
	}
	if len(export.Terminal) != 2 {
		t.Errorf("expected 2 terminal states, got %d", len(export.Terminal))
	}
	if len(export.States) != 7 {
		t.Errorf("expected 7 states, got %d", len(export.States))
	}
	if len(export.Transitions) == 0 {
		t.Error("expected non-empty transitions")
	}
}

func TestStateMachineExporter_Export_WithEligibility(t *testing.T) {
	ctx := context.Background()
	eligibility := policy.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "read_tool")
	eligibility.Allow(agent.StateAct, "write_tool")

	exporter := NewStateMachineExporter(eligibility, nil)

	export, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find explore state and verify eligible tools
	for _, state := range export.States {
		if state.Name == agent.StateExplore {
			if len(state.EligibleTools) == 0 {
				t.Error("expected eligible tools in explore state")
			}
		}
	}
}

func TestStateMachineExporter_Export_WithTransitions(t *testing.T) {
	ctx := context.Background()
	transitions := policy.DefaultTransitions()

	exporter := NewStateMachineExporter(nil, transitions)

	export, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(export.Transitions) == 0 {
		t.Error("expected non-empty transitions")
	}
}

func TestStateMachineExporter_Export_StateDescriptions(t *testing.T) {
	ctx := context.Background()
	exporter := NewStateMachineExporter(nil, nil)

	export, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, state := range export.States {
		if state.Description == "" {
			t.Errorf("expected description for state %s", state.Name)
		}
	}
}

func TestStateMachineExporter_Export_SideEffectsOnlyInAct(t *testing.T) {
	ctx := context.Background()
	exporter := NewStateMachineExporter(nil, nil)

	export, err := exporter.Export(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, state := range export.States {
		if state.AllowsSideEffects && state.Name != agent.StateAct {
			t.Errorf("expected side effects only in act state, found in %s", state.Name)
		}
		if state.Name == agent.StateAct && !state.AllowsSideEffects {
			t.Error("expected act state to allow side effects")
		}
	}
}

// ===============================
// Metrics Exporter Tests
// ===============================

func TestNewMetricsExporter(t *testing.T) {
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	exporter := NewMetricsExporter(runStore, eventStore)

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}
}

func TestMetricsExporter_Export_Summary(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	// Create some runs
	for i := 0; i < 3; i++ {
		r := agent.NewRun(fmt.Sprintf("run-%d", i), "test goal")
		r.Status = agent.RunStatusCompleted
		r.EndTime = r.StartTime.Add(time.Minute)
		if err := runStore.Save(ctx, r); err != nil {
			t.Fatalf("failed to save run: %v", err)
		}
	}

	// Add one failed run
	failedRun := agent.NewRun("failed-run", "test goal")
	failedRun.Status = agent.RunStatusFailed
	failedRun.EndTime = failedRun.StartTime.Add(30 * time.Second)
	if err := runStore.Save(ctx, failedRun); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	exporter := NewMetricsExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, inspector.MetricsFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if export.Summary.TotalRuns != 4 {
		t.Errorf("expected 4 total runs, got %d", export.Summary.TotalRuns)
	}
	if export.Summary.CompletedRuns != 3 {
		t.Errorf("expected 3 completed runs, got %d", export.Summary.CompletedRuns)
	}
	if export.Summary.FailedRuns != 1 {
		t.Errorf("expected 1 failed run, got %d", export.Summary.FailedRuns)
	}
}

func TestMetricsExporter_Export_ToolMetrics(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	// Create run with tool events
	r := agent.NewRun("run-1", "test goal")
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	addToolCallEvents(ctx, t, eventStore, r.ID)

	exporter := NewMetricsExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, inspector.MetricsFilter{
		IncludeToolMetrics: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(export.ToolMetrics) == 0 {
		t.Error("expected non-empty tool metrics")
	}
}

func TestMetricsExporter_Export_StateMetrics(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	// Create run with state transitions
	r := agent.NewRun("run-1", "test goal")
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	addTransitionEvents(ctx, t, eventStore, r.ID)

	exporter := NewMetricsExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, inspector.MetricsFilter{
		IncludeStateMetrics: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(export.StateMetrics) == 0 {
		t.Error("expected non-empty state metrics")
	}
}

func TestMetricsExporter_Export_TimeFilter(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	// Create old and new runs
	oldRun := agent.NewRun("old-run", "test goal")
	oldRun.StartTime = time.Now().Add(-24 * time.Hour)
	if err := runStore.Save(ctx, oldRun); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	newRun := agent.NewRun("new-run", "test goal")
	if err := runStore.Save(ctx, newRun); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	exporter := NewMetricsExporter(runStore, eventStore)

	// Filter to only recent runs
	export, err := exporter.Export(ctx, inspector.MetricsFilter{
		FromTime: time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if export.Summary.TotalRuns != 1 {
		t.Errorf("expected 1 recent run, got %d", export.Summary.TotalRuns)
	}
}

func TestMetricsExporter_Export_EmptyStore(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	exporter := NewMetricsExporter(runStore, eventStore)
	export, err := exporter.Export(ctx, inspector.MetricsFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if export.Summary.TotalRuns != 0 {
		t.Errorf("expected 0 runs, got %d", export.Summary.TotalRuns)
	}
}

// ===============================
// Default Inspector Tests
// ===============================

func TestNewDefaultInspector(t *testing.T) {
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	runExporter := NewRunExporter(runStore, eventStore)
	smExporter := NewStateMachineExporter(nil, nil)
	metricsExporter := NewMetricsExporter(runStore, eventStore)

	inspector := NewDefaultInspector(runExporter, smExporter, metricsExporter)

	if inspector == nil {
		t.Fatal("expected non-nil inspector")
	}
	if len(inspector.formatters) != 3 {
		t.Errorf("expected 3 formatters, got %d", len(inspector.formatters))
	}
}

func TestDefaultInspector_RegisterFormatter(t *testing.T) {
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	runExporter := NewRunExporter(runStore, eventStore)
	smExporter := NewStateMachineExporter(nil, nil)
	metricsExporter := NewMetricsExporter(runStore, eventStore)

	i := NewDefaultInspector(runExporter, smExporter, metricsExporter)

	// Register a custom formatter
	mockFormatter := &mockFormatter{formatType: "custom"}
	i.RegisterFormatter(mockFormatter)

	if len(i.formatters) != 4 {
		t.Errorf("expected 4 formatters after registration, got %d", len(i.formatters))
	}
}

func TestDefaultInspector_ExportRun_NoExporter(t *testing.T) {
	ctx := context.Background()
	i := NewDefaultInspector(nil, nil, nil)

	_, err := i.ExportRun(ctx, "run-1", inspector.FormatJSON)
	if err == nil {
		t.Error("expected error when run exporter is nil")
	}
}

func TestDefaultInspector_ExportStateMachine_NoExporter(t *testing.T) {
	ctx := context.Background()
	i := NewDefaultInspector(nil, nil, nil)

	_, err := i.ExportStateMachine(ctx, inspector.FormatJSON)
	if err == nil {
		t.Error("expected error when state machine exporter is nil")
	}
}

func TestDefaultInspector_ExportMetrics_NoExporter(t *testing.T) {
	ctx := context.Background()
	i := NewDefaultInspector(nil, nil, nil)

	_, err := i.ExportMetrics(ctx, inspector.MetricsFilter{}, inspector.FormatJSON)
	if err == nil {
		t.Error("expected error when metrics exporter is nil")
	}
}

func TestDefaultInspector_ExportRun_JSON(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	r := agent.NewRun("run-1", "test goal")
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	runExporter := NewRunExporter(runStore, eventStore)
	i := NewDefaultInspector(runExporter, nil, nil)

	result, err := i.ExportRun(ctx, r.ID, inspector.FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Error("expected non-empty result")
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Errorf("expected valid JSON, got error: %v", err)
	}
}

func TestDefaultInspector_ExportStateMachine_DOT(t *testing.T) {
	ctx := context.Background()
	smExporter := NewStateMachineExporter(nil, nil)
	i := NewDefaultInspector(nil, smExporter, nil)

	result, err := i.ExportStateMachine(ctx, inspector.FormatDOT)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(result), "digraph") {
		t.Error("expected DOT format output")
	}
}

func TestDefaultInspector_ExportStateMachine_Mermaid(t *testing.T) {
	ctx := context.Background()
	smExporter := NewStateMachineExporter(nil, nil)
	i := NewDefaultInspector(nil, smExporter, nil)

	result, err := i.ExportStateMachine(ctx, inspector.FormatMermaid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(result), "stateDiagram") {
		t.Error("expected Mermaid format output")
	}
}

func TestDefaultInspector_ExportMetrics_JSON(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	metricsExporter := NewMetricsExporter(runStore, eventStore)
	i := NewDefaultInspector(nil, nil, metricsExporter)

	result, err := i.ExportMetrics(ctx, inspector.MetricsFilter{}, inspector.FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Errorf("expected valid JSON, got error: %v", err)
	}
}

func TestDefaultInspector_FallbackToJSON(t *testing.T) {
	ctx := context.Background()
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	r := agent.NewRun("run-1", "test goal")
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	runExporter := NewRunExporter(runStore, eventStore)
	i := NewDefaultInspector(runExporter, nil, nil)

	// Use unknown format - should fallback to JSON
	result, err := i.ExportRun(ctx, r.ID, "unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Errorf("expected valid JSON fallback, got error: %v", err)
	}
}

// ===============================
// Helper Function Tests
// ===============================

func TestCalculateAverageDuration(t *testing.T) {
	tests := []struct {
		durations []time.Duration
		expected  time.Duration
	}{
		{[]time.Duration{}, 0},
		{[]time.Duration{time.Second}, time.Second},
		{[]time.Duration{time.Second, time.Second * 3}, time.Second * 2},
	}

	for _, tt := range tests {
		result := calculateAverageDuration(tt.durations)
		if result != tt.expected {
			t.Errorf("calculateAverageDuration(%v) = %v, want %v", tt.durations, result, tt.expected)
		}
	}
}

func TestCalculateP90Duration(t *testing.T) {
	tests := []struct {
		durations []time.Duration
		expected  time.Duration
	}{
		{[]time.Duration{}, 0},
		{[]time.Duration{time.Second}, time.Second},
		{[]time.Duration{
			time.Second,
			time.Second * 2,
			time.Second * 3,
			time.Second * 4,
			time.Second * 5,
			time.Second * 6,
			time.Second * 7,
			time.Second * 8,
			time.Second * 9,
			time.Second * 10,
		}, time.Second * 10},
	}

	for _, tt := range tests {
		result := calculateP90Duration(tt.durations)
		if result != tt.expected {
			t.Errorf("calculateP90Duration(%v) = %v, want %v", tt.durations, result, tt.expected)
		}
	}
}

func TestGetEventLabel(t *testing.T) {
	tests := []struct {
		eventType event.Type
		expected  string
	}{
		{event.TypeRunStarted, "Run Started"},
		{event.TypeRunCompleted, "Run Completed"},
		{event.TypeRunFailed, "Run Failed"},
		{event.TypeToolSucceeded, "Tool Succeeded"},
		{event.TypeToolFailed, "Tool Failed"},
	}

	for _, tt := range tests {
		e := event.Event{Type: tt.eventType}
		result := getEventLabel(e)
		if result != tt.expected {
			t.Errorf("getEventLabel(%s) = %q, want %q", tt.eventType, result, tt.expected)
		}
	}
}

func TestGetStateDescription(t *testing.T) {
	tests := []struct {
		state    agent.State
		expected string
	}{
		{agent.StateIntake, "Normalize and understand the goal"},
		{agent.StateExplore, "Gather evidence through read-only operations"},
		{agent.StateDecide, "Choose the next action"},
		{agent.StateAct, "Perform side-effects"},
		{agent.StateValidate, "Confirm outcomes"},
		{agent.StateDone, "Terminal success state"},
		{agent.StateFailed, "Terminal failure state"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := getStateDescription(tt.state)
		if result != tt.expected {
			t.Errorf("getStateDescription(%s) = %q, want %q", tt.state, result, tt.expected)
		}
	}
}

// ===============================
// Test Helpers
// ===============================

type mockFormatter struct {
	formatType inspector.ExportFormat
}

func (m *mockFormatter) Format(data any) ([]byte, error) {
	return json.Marshal(data)
}

func (m *mockFormatter) FormatType() inspector.ExportFormat {
	return m.formatType
}

func addTestEvents(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runID string) {
	t.Helper()

	events := []event.Event{
		{
			RunID:     runID,
			Type:      event.TypeRunStarted,
			Timestamp: time.Now().Add(-time.Minute),
		},
		{
			RunID:     runID,
			Type:      event.TypeRunCompleted,
			Timestamp: time.Now(),
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}
}

func addToolCallEvents(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runID string) {
	t.Helper()

	baseTime := time.Now().Add(-time.Minute)

	calledPayload := event.ToolCalledPayload{
		ToolName: "test_tool",
		Input:    json.RawMessage(`{}`),
		State:    agent.StateExplore,
	}
	calledBytes, _ := json.Marshal(calledPayload)

	succeededPayload := event.ToolSucceededPayload{
		ToolName: "test_tool",
		Output:   json.RawMessage(`{}`),
		Duration: 100 * time.Millisecond,
	}
	succeededBytes, _ := json.Marshal(succeededPayload)

	failedPayload := event.ToolFailedPayload{
		ToolName: "failing_tool",
		Error:    "test error",
		Duration: 50 * time.Millisecond,
	}
	failedBytes, _ := json.Marshal(failedPayload)

	events := []event.Event{
		{
			RunID:     runID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime,
			Payload:   calledBytes,
		},
		{
			RunID:     runID,
			Type:      event.TypeToolSucceeded,
			Timestamp: baseTime.Add(100 * time.Millisecond),
			Payload:   succeededBytes,
		},
		{
			RunID:     runID,
			Type:      event.TypeToolCalled,
			Timestamp: baseTime.Add(200 * time.Millisecond),
			Payload:   json.RawMessage(`{"tool_name":"failing_tool","input":{},"state":"explore"}`),
		},
		{
			RunID:     runID,
			Type:      event.TypeToolFailed,
			Timestamp: baseTime.Add(250 * time.Millisecond),
			Payload:   failedBytes,
		},
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}
}

func addTransitionEvents(ctx context.Context, t *testing.T, eventStore *memory.EventStore, runID string) {
	t.Helper()

	baseTime := time.Now().Add(-time.Minute)

	transitions := []struct {
		from agent.State
		to   agent.State
	}{
		{agent.StateIntake, agent.StateExplore},
		{agent.StateExplore, agent.StateDecide},
		{agent.StateDecide, agent.StateAct},
		{agent.StateAct, agent.StateValidate},
		{agent.StateValidate, agent.StateDone},
	}

	var events []event.Event
	for i, tr := range transitions {
		payload := event.StateTransitionedPayload{
			FromState: tr.from,
			ToState:   tr.to,
			Reason:    fmt.Sprintf("transition %d", i),
		}
		payloadBytes, _ := json.Marshal(payload)

		events = append(events, event.Event{
			RunID:     runID,
			Type:      event.TypeStateTransitioned,
			Timestamp: baseTime.Add(time.Duration(i) * 10 * time.Second),
			Payload:   payloadBytes,
		})
	}

	if err := eventStore.Append(ctx, events...); err != nil {
		t.Fatalf("failed to append events: %v", err)
	}
}
