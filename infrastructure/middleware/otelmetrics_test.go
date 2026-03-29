package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// createMockToolForOTel creates a simple mock tool for OTel metrics testing.
func createMockToolForOTel(name string) tool.Tool {
	t, _ := tool.NewBuilder(name).
		WithDescription("Mock tool for OTel testing").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"status": "ok"}`)}, nil
		}).
		Build()
	return t
}

func TestOTelMetrics_RecordsSuccessMetrics(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	cfg := OTelMetricsConfig{
		Collector:        collector,
		MetricPrefix:     "test",
		IncludeToolName:  true,
		IncludeState:     true,
		RecordDuration:   true,
		RecordSuccess:    true,
		RecordOutputSize: true,
	}

	mw := OTelMetrics(cfg)
	testTool := createMockToolForOTel("test_tool")
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateExplore,
		Tool:         testTool,
		Input:        json.RawMessage(`{}`),
	}

	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		time.Sleep(1 * time.Millisecond) // Ensure non-zero duration measurement
		return tool.Result{Output: json.RawMessage(`{"result": "success"}`)}, nil
	})

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output == nil {
		t.Error("expected output")
	}

	// Verify counter recorded
	counterKey := findMetricKey(collector.Counters, "test_tool_calls_total")
	if counterKey == "" {
		t.Fatal("expected tool_calls_total counter")
	}
	if !strings.Contains(counterKey, "status=success") {
		t.Error("expected status=success label")
	}
	if collector.Counters[counterKey] != 1 {
		t.Errorf("counter = %d, want 1", collector.Counters[counterKey])
	}

	// Verify duration recorded
	durationKey := findMetricKey(collector.Durations, "test_tool_duration_seconds")
	if durationKey == "" {
		t.Fatal("expected tool_duration_seconds metric")
	}
	if len(collector.Durations[durationKey]) != 1 {
		t.Errorf("got %d durations, want 1", len(collector.Durations[durationKey]))
	}
	if collector.Durations[durationKey][0] <= 0 {
		t.Errorf("duration should be positive: %v", collector.Durations[durationKey][0])
	}

	// Verify output size recorded
	outputKey := findMetricKey(collector.Histograms, "test_tool_output_bytes")
	if outputKey == "" {
		t.Fatal("expected tool_output_bytes histogram")
	}
	if len(collector.Histograms[outputKey]) != 1 {
		t.Errorf("got %d histogram values, want 1", len(collector.Histograms[outputKey]))
	}
	expectedSize := float64(len(result.Output))
	if collector.Histograms[outputKey][0] != expectedSize {
		t.Errorf("output size = %f, want %f", collector.Histograms[outputKey][0], expectedSize)
	}
}

func TestOTelMetrics_RecordsFailureMetrics(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	cfg := OTelMetricsConfig{
		Collector:       collector,
		MetricPrefix:    "test",
		RecordSuccess:   true,
		IncludeToolName: true,
	}

	mw := OTelMetrics(cfg)
	testTool := createMockToolForOTel("failing_tool")
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-456",
		CurrentState: agent.StateAct,
		Tool:         testTool,
		Input:        json.RawMessage(`{}`),
	}

	expectedErr := errors.New("tool execution failed")
	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		return tool.Result{}, expectedErr
	})

	_, err := handler(context.Background(), execCtx)
	if err == nil || err.Error() != expectedErr.Error() {
		t.Fatalf("expected error: %v, got: %v", expectedErr, err)
	}

	// Verify counter recorded with error status
	counterKey := findMetricKey(collector.Counters, "test_tool_calls_total")
	if counterKey == "" {
		t.Fatal("expected tool_calls_total counter")
	}
	if !strings.Contains(counterKey, "status=error") {
		t.Error("expected status=error label")
	}
	if collector.Counters[counterKey] != 1 {
		t.Errorf("counter = %d, want 1", collector.Counters[counterKey])
	}
}

func TestOTelMetrics_RecordsInputSize(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	cfg := OTelMetricsConfig{
		Collector:       collector,
		MetricPrefix:    "test",
		RecordInputSize: true,
		IncludeToolName: true,
	}

	mw := OTelMetrics(cfg)
	testTool := createMockToolForOTel("input_tool")
	inputData := json.RawMessage(`{"large": "input data with some content"}`)
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-789",
		CurrentState: agent.StateExplore,
		Tool:         testTool,
		Input:        inputData,
	}

	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		return tool.Result{Output: json.RawMessage(`{}`)}, nil
	})

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify input size recorded
	inputKey := findMetricKey(collector.Histograms, "test_tool_input_bytes")
	if inputKey == "" {
		t.Fatal("expected tool_input_bytes histogram")
	}
	if len(collector.Histograms[inputKey]) != 1 {
		t.Errorf("got %d histogram values, want 1", len(collector.Histograms[inputKey]))
	}
	expectedSize := float64(len(inputData))
	if collector.Histograms[inputKey][0] != expectedSize {
		t.Errorf("input size = %f, want %f", collector.Histograms[inputKey][0], expectedSize)
	}
}

func TestOTelMetrics_LabelsBuiltCorrectly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		cfg              OTelMetricsConfig
		toolName         string
		runID            string
		state            agent.State
		expectedContains []string
		notContains      []string
	}{
		{
			name: "includes all labels",
			cfg: OTelMetricsConfig{
				IncludeToolName: true,
				IncludeRunID:    true,
				IncludeState:    true,
				RecordSuccess:   true,
			},
			toolName: "test_tool",
			runID:    "run-123",
			state:    agent.StateExplore,
			expectedContains: []string{
				"tool=test_tool",
				"run_id=run-123",
				"state=explore",
			},
		},
		{
			name: "excludes run_id",
			cfg: OTelMetricsConfig{
				IncludeToolName: true,
				IncludeRunID:    false,
				IncludeState:    true,
				RecordSuccess:   true,
			},
			toolName: "test_tool",
			runID:    "run-456",
			state:    agent.StateAct,
			expectedContains: []string{
				"tool=test_tool",
				"state=act",
			},
			notContains: []string{
				"run_id=",
			},
		},
		{
			name: "excludes tool name",
			cfg: OTelMetricsConfig{
				IncludeToolName: false,
				IncludeRunID:    false,
				IncludeState:    true,
				RecordSuccess:   true,
			},
			toolName: "test_tool",
			runID:    "run-789",
			state:    agent.StateDecide,
			expectedContains: []string{
				"state=decide",
			},
			notContains: []string{
				"tool=",
				"run_id=",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			collector := NewInMemoryMetricsCollector()
			tt.cfg.Collector = collector
			tt.cfg.MetricPrefix = "test"

			mw := OTelMetrics(tt.cfg)
			testTool := createMockToolForOTel(tt.toolName)
			execCtx := &middleware.ExecutionContext{
				RunID:        tt.runID,
				CurrentState: tt.state,
				Tool:         testTool,
				Input:        json.RawMessage(`{}`),
			}

			handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			})

			_, err := handler(context.Background(), execCtx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Find any counter key to verify labels
			var counterKey string
			for k := range collector.Counters {
				counterKey = k
				break
			}

			if counterKey == "" {
				t.Fatal("expected at least one counter")
			}

			for _, expected := range tt.expectedContains {
				if !strings.Contains(counterKey, expected) {
					t.Errorf("key %q should contain %q", counterKey, expected)
				}
			}

			for _, notExpected := range tt.notContains {
				if strings.Contains(counterKey, notExpected) {
					t.Errorf("key %q should not contain %q", counterKey, notExpected)
				}
			}
		})
	}
}

func TestOTelMetrics_NoopCollectorDoesNotPanic(t *testing.T) {
	t.Parallel()

	// Test with nil collector (should use noopCollector)
	cfg := OTelMetricsConfig{
		Collector:        nil,
		RecordDuration:   true,
		RecordSuccess:    true,
		RecordInputSize:  true,
		RecordOutputSize: true,
	}

	mw := OTelMetrics(cfg)
	testTool := createMockToolForOTel("test_tool")
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-noop",
		CurrentState: agent.StateExplore,
		Tool:         testTool,
		Input:        json.RawMessage(`{"data": "test"}`),
	}

	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		return tool.Result{Output: json.RawMessage(`{"result": "ok"}`)}, nil
	})

	// Should not panic
	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOTelMetrics_DefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultOTelMetricsConfig()

	if cfg.MetricPrefix != "agent" {
		t.Errorf("MetricPrefix = %s, want agent", cfg.MetricPrefix)
	}
	if !cfg.IncludeToolName {
		t.Error("IncludeToolName should be true")
	}
	if cfg.IncludeRunID {
		t.Error("IncludeRunID should be false (high cardinality)")
	}
	if !cfg.IncludeState {
		t.Error("IncludeState should be true")
	}
	if !cfg.RecordDuration {
		t.Error("RecordDuration should be true")
	}
	if !cfg.RecordSuccess {
		t.Error("RecordSuccess should be true")
	}
	if !cfg.RecordInputSize {
		t.Error("RecordInputSize should be true")
	}
	if !cfg.RecordOutputSize {
		t.Error("RecordOutputSize should be true")
	}
}

func TestOTelMetrics_EmptyMetricPrefix(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	cfg := OTelMetricsConfig{
		Collector:       collector,
		MetricPrefix:    "", // Empty prefix
		RecordSuccess:   true,
		IncludeToolName: true,
	}

	mw := OTelMetrics(cfg)
	testTool := createMockToolForOTel("test_tool")
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-prefix",
		CurrentState: agent.StateExplore,
		Tool:         testTool,
		Input:        json.RawMessage(`{}`),
	}

	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		return tool.Result{Output: json.RawMessage(`{}`)}, nil
	})

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default to "agent" prefix
	counterKey := findMetricKey(collector.Counters, "agent_tool_calls_total")
	if counterKey == "" {
		t.Error("expected default prefix 'agent' to be used")
	}
}

func TestInMemoryMetricsCollector_IncrementCounter(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	ctx := context.Background()

	// Test with single label to avoid map iteration order issues
	labels1 := map[string]string{"tool": "test"}
	collector.IncrementCounter(ctx, "test_counter", 1, labels1)
	collector.IncrementCounter(ctx, "test_counter", 2, labels1)
	collector.IncrementCounter(ctx, "test_counter", 3, labels1)

	// Verify accumulation
	key := formatMetricKey("test_counter", labels1)
	if collector.Counters[key] != 6 {
		t.Errorf("counter = %d, want 6", collector.Counters[key])
	}

	// Test with different labels creates different counter
	labels2 := map[string]string{"tool": "other"}
	collector.IncrementCounter(ctx, "test_counter", 5, labels2)

	key2 := formatMetricKey("test_counter", labels2)
	if collector.Counters[key2] != 5 {
		t.Errorf("counter with different labels = %d, want 5", collector.Counters[key2])
	}
}

func TestInMemoryMetricsCollector_RecordDuration(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	ctx := context.Background()
	labels := map[string]string{"tool": "test"}

	collector.RecordDuration(ctx, "test_duration", 100*time.Millisecond, labels)
	collector.RecordDuration(ctx, "test_duration", 200*time.Millisecond, labels)

	key := formatMetricKey("test_duration", labels)
	if len(collector.Durations[key]) != 2 {
		t.Errorf("got %d durations, want 2", len(collector.Durations[key]))
	}
	if collector.Durations[key][0] != 100*time.Millisecond {
		t.Errorf("first duration = %v, want 100ms", collector.Durations[key][0])
	}
	if collector.Durations[key][1] != 200*time.Millisecond {
		t.Errorf("second duration = %v, want 200ms", collector.Durations[key][1])
	}
}

func TestInMemoryMetricsCollector_RecordGauge(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	ctx := context.Background()
	labels := map[string]string{"resource": "memory"}

	collector.RecordGauge(ctx, "test_gauge", 42.5, labels)
	collector.RecordGauge(ctx, "test_gauge", 84.0, labels) // Should overwrite

	key := formatMetricKey("test_gauge", labels)
	if collector.Gauges[key] != 84.0 {
		t.Errorf("gauge = %f, want 84.0", collector.Gauges[key])
	}
}

func TestInMemoryMetricsCollector_RecordHistogram(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	ctx := context.Background()
	labels := map[string]string{"operation": "read"}

	collector.RecordHistogram(ctx, "test_histogram", 10.0, labels)
	collector.RecordHistogram(ctx, "test_histogram", 20.0, labels)
	collector.RecordHistogram(ctx, "test_histogram", 30.0, labels)

	key := formatMetricKey("test_histogram", labels)
	if len(collector.Histograms[key]) != 3 {
		t.Errorf("got %d histogram values, want 3", len(collector.Histograms[key]))
	}
	expectedValues := []float64{10.0, 20.0, 30.0}
	for i, expected := range expectedValues {
		if collector.Histograms[key][i] != expected {
			t.Errorf("histogram[%d] = %f, want %f", i, collector.Histograms[key][i], expected)
		}
	}
}

func TestNoopCollector_DoesNotPanic(t *testing.T) {
	t.Parallel()

	collector := &noopCollector{}
	ctx := context.Background()
	labels := map[string]string{"test": "label"}

	// Should not panic
	collector.IncrementCounter(ctx, "test", 1, labels)
	collector.RecordDuration(ctx, "test", time.Second, labels)
	collector.RecordGauge(ctx, "test", 1.0, labels)
	collector.RecordHistogram(ctx, "test", 1.0, labels)
}

func TestCopyLabels(t *testing.T) {
	t.Parallel()

	original := map[string]string{
		"tool":  "test_tool",
		"state": "explore",
	}

	copied := copyLabels(original)

	// Verify copy has same values
	if len(copied) != len(original) {
		t.Errorf("copied length = %d, want %d", len(copied), len(original))
	}
	for k, v := range original {
		if copied[k] != v {
			t.Errorf("copied[%s] = %s, want %s", k, copied[k], v)
		}
	}

	// Verify modifying copy doesn't affect original
	copied["new_key"] = "new_value"
	if _, exists := original["new_key"]; exists {
		t.Error("modifying copy should not affect original")
	}

	// Verify modifying original doesn't affect copy
	original["another_key"] = "another_value"
	if _, exists := copied["another_key"]; exists {
		t.Error("modifying original should not affect copy")
	}
}

func TestFormatMetricKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		labels   map[string]string
		expected []string // Parts that should be in the key
	}{
		{
			name:     "no labels",
			labels:   map[string]string{},
			expected: []string{"test_metric"},
		},
		{
			name: "single label",
			labels: map[string]string{
				"tool": "read_file",
			},
			expected: []string{"test_metric", "tool=read_file"},
		},
		{
			name: "multiple labels",
			labels: map[string]string{
				"tool":   "write_file",
				"state":  "act",
				"status": "success",
			},
			expected: []string{"test_metric", "tool=write_file", "state=act", "status=success"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			key := formatMetricKey("test_metric", tt.labels)

			for _, part := range tt.expected {
				if !strings.Contains(key, part) {
					t.Errorf("key %q should contain %q", key, part)
				}
			}
		})
	}
}

func TestFormatMetricKey_Uniqueness(t *testing.T) {
	t.Parallel()

	labels1 := map[string]string{"tool": "tool1", "state": "explore"}
	labels2 := map[string]string{"tool": "tool2", "state": "explore"}
	labels3 := map[string]string{"tool": "tool1", "state": "act"}

	key1 := formatMetricKey("metric", labels1)
	key2 := formatMetricKey("metric", labels2)
	key3 := formatMetricKey("metric", labels3)

	if key1 == key2 {
		t.Error("keys with different tool names should be unique")
	}
	if key1 == key3 {
		t.Error("keys with different states should be unique")
	}
	if key2 == key3 {
		t.Error("keys with different labels should be unique")
	}
}

func TestOTelMetrics_DisabledRecordings(t *testing.T) {
	t.Parallel()

	collector := NewInMemoryMetricsCollector()
	cfg := OTelMetricsConfig{
		Collector:        collector,
		MetricPrefix:     "test",
		RecordDuration:   false, // Disabled
		RecordSuccess:    false, // Disabled
		RecordInputSize:  false, // Disabled
		RecordOutputSize: false, // Disabled
	}

	mw := OTelMetrics(cfg)
	testTool := createMockToolForOTel("test_tool")
	execCtx := &middleware.ExecutionContext{
		RunID:        "run-disabled",
		CurrentState: agent.StateExplore,
		Tool:         testTool,
		Input:        json.RawMessage(`{"data": "test"}`),
	}

	handler := mw(func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
		return tool.Result{Output: json.RawMessage(`{"result": "ok"}`)}, nil
	})

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no metrics recorded
	if len(collector.Counters) > 0 {
		t.Error("no counters should be recorded when RecordSuccess is disabled")
	}
	if len(collector.Durations) > 0 {
		t.Error("no durations should be recorded when RecordDuration is disabled")
	}
	if len(collector.Histograms) > 0 {
		t.Error("no histograms should be recorded when input/output recording is disabled")
	}
}

// findMetricKey is a helper to find a metric key containing the given metric name.
func findMetricKey(metrics interface{}, metricName string) string {
	switch m := metrics.(type) {
	case map[string]int64:
		for k := range m {
			if strings.Contains(k, metricName) {
				return k
			}
		}
	case map[string][]time.Duration:
		for k := range m {
			if strings.Contains(k, metricName) {
				return k
			}
		}
	case map[string][]float64:
		for k := range m {
			if strings.Contains(k, metricName) {
				return k
			}
		}
	}
	return ""
}
