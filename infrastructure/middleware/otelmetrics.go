package middleware

import (
	"context"
	"time"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/logging"
)

// MetricsCollector defines the interface for collecting metrics.
// This allows for different metrics backends (OTel, Prometheus, etc.).
type MetricsCollector interface {
	// IncrementCounter increments a counter metric.
	IncrementCounter(ctx context.Context, name string, value int64, labels map[string]string)

	// RecordDuration records a duration metric.
	RecordDuration(ctx context.Context, name string, duration time.Duration, labels map[string]string)

	// RecordGauge records a gauge metric.
	RecordGauge(ctx context.Context, name string, value float64, labels map[string]string)

	// RecordHistogram records a histogram metric.
	RecordHistogram(ctx context.Context, name string, value float64, labels map[string]string)
}

// OTelMetricsConfig configures the OpenTelemetry metrics middleware.
type OTelMetricsConfig struct {
	// Collector is the metrics collector to use.
	Collector MetricsCollector

	// MetricPrefix is the prefix for all metrics.
	MetricPrefix string

	// IncludeToolName includes tool name as a label.
	IncludeToolName bool

	// IncludeRunID includes run ID as a label.
	IncludeRunID bool

	// IncludeState includes agent state as a label.
	IncludeState bool

	// RecordDuration records tool execution duration.
	RecordDuration bool

	// RecordSuccess records success/failure counts.
	RecordSuccess bool

	// RecordInputSize records tool input size.
	RecordInputSize bool

	// RecordOutputSize records tool output size.
	RecordOutputSize bool

	// Logger is the injected structured logger. When nil, a no-op logger is
	// used — never the package-level logging singleton.
	Logger *logging.Logger
}

// DefaultOTelMetricsConfig returns a sensible default configuration.
func DefaultOTelMetricsConfig() OTelMetricsConfig {
	return OTelMetricsConfig{
		MetricPrefix:     "agent",
		IncludeToolName:  true,
		IncludeRunID:     false, // Can cause high cardinality
		IncludeState:     true,
		RecordDuration:   true,
		RecordSuccess:    true,
		RecordInputSize:  true,
		RecordOutputSize: true,
	}
}

// OTelMetrics returns middleware that records OpenTelemetry metrics for tool executions.
func OTelMetrics(cfg OTelMetricsConfig) middleware.Middleware {
	if cfg.Collector == nil {
		cfg.Collector = &noopCollector{}
	}

	if cfg.MetricPrefix == "" {
		cfg.MetricPrefix = "agent"
	}

	log := resolveLogger(cfg.Logger)

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Build labels
			labels := make(map[string]string)
			if cfg.IncludeToolName {
				labels["tool"] = execCtx.Tool.Name()
			}
			if cfg.IncludeRunID {
				labels["run_id"] = execCtx.RunID
			}
			if cfg.IncludeState {
				labels["state"] = string(execCtx.CurrentState)
			}

			// Record input size
			if cfg.RecordInputSize && len(execCtx.Input) > 0 {
				cfg.Collector.RecordHistogram(
					ctx,
					cfg.MetricPrefix+"_tool_input_bytes",
					float64(len(execCtx.Input)),
					labels,
				)
			}

			// Start timing
			startTime := time.Now()

			// Execute the handler
			result, err := next(ctx, execCtx)

			// Record duration
			duration := time.Since(startTime)
			if cfg.RecordDuration {
				cfg.Collector.RecordDuration(
					ctx,
					cfg.MetricPrefix+"_tool_duration_seconds",
					duration,
					labels,
				)
			}

			// Record success/failure
			if cfg.RecordSuccess {
				successLabels := copyLabels(labels)
				if err != nil {
					successLabels["status"] = "error"
				} else {
					successLabels["status"] = "success"
				}
				cfg.Collector.IncrementCounter(
					ctx,
					cfg.MetricPrefix+"_tool_calls_total",
					1,
					successLabels,
				)
			}

			// Record output size
			if cfg.RecordOutputSize && len(result.Output) > 0 {
				cfg.Collector.RecordHistogram(
					ctx,
					cfg.MetricPrefix+"_tool_output_bytes",
					float64(len(result.Output)),
					labels,
				)
			}

			// Log metrics
			log.Debug().
				Add(logging.RunID(execCtx.RunID)).
				Add(logging.ToolName(execCtx.Tool.Name())).
				Add(logging.Duration(duration)).
				Add(logging.Bool("success", err == nil)).
				Msg("tool metrics recorded")

			return result, err
		}
	}
}

// copyLabels creates a copy of the labels map.
func copyLabels(labels map[string]string) map[string]string {
	copy := make(map[string]string, len(labels)+1)
	for k, v := range labels {
		copy[k] = v
	}
	return copy
}

// noopCollector is a no-op metrics collector.
type noopCollector struct{}

func (c *noopCollector) IncrementCounter(_ context.Context, _ string, _ int64, _ map[string]string) {
}

func (c *noopCollector) RecordDuration(_ context.Context, _ string, _ time.Duration, _ map[string]string) {
}

func (c *noopCollector) RecordGauge(_ context.Context, _ string, _ float64, _ map[string]string) {
}

func (c *noopCollector) RecordHistogram(_ context.Context, _ string, _ float64, _ map[string]string) {
}

// Ensure noopCollector implements MetricsCollector
var _ MetricsCollector = (*noopCollector)(nil)

// InMemoryMetricsCollector is a simple in-memory metrics collector for testing.
type InMemoryMetricsCollector struct {
	Counters   map[string]int64
	Durations  map[string][]time.Duration
	Gauges     map[string]float64
	Histograms map[string][]float64
}

// NewInMemoryMetricsCollector creates a new in-memory metrics collector.
func NewInMemoryMetricsCollector() *InMemoryMetricsCollector {
	return &InMemoryMetricsCollector{
		Counters:   make(map[string]int64),
		Durations:  make(map[string][]time.Duration),
		Gauges:     make(map[string]float64),
		Histograms: make(map[string][]float64),
	}
}

// IncrementCounter implements MetricsCollector.IncrementCounter.
func (c *InMemoryMetricsCollector) IncrementCounter(_ context.Context, name string, value int64, labels map[string]string) {
	key := formatMetricKey(name, labels)
	c.Counters[key] += value
}

// RecordDuration implements MetricsCollector.RecordDuration.
func (c *InMemoryMetricsCollector) RecordDuration(_ context.Context, name string, duration time.Duration, labels map[string]string) {
	key := formatMetricKey(name, labels)
	c.Durations[key] = append(c.Durations[key], duration)
}

// RecordGauge implements MetricsCollector.RecordGauge.
func (c *InMemoryMetricsCollector) RecordGauge(_ context.Context, name string, value float64, labels map[string]string) {
	key := formatMetricKey(name, labels)
	c.Gauges[key] = value
}

// RecordHistogram implements MetricsCollector.RecordHistogram.
func (c *InMemoryMetricsCollector) RecordHistogram(_ context.Context, name string, value float64, labels map[string]string) {
	key := formatMetricKey(name, labels)
	c.Histograms[key] = append(c.Histograms[key], value)
}

// formatMetricKey creates a unique key from metric name and labels.
func formatMetricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += "|" + k + "=" + v
	}
	return key
}

// Ensure InMemoryMetricsCollector implements MetricsCollector
var _ MetricsCollector = (*InMemoryMetricsCollector)(nil)
