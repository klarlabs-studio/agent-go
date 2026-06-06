// Package otel provides OpenTelemetry integration for agent-go.
//
// This package implements the telemetry interfaces from the domain layer using
// OpenTelemetry SDK, enabling distributed tracing and metrics collection for
// agent runs.
//
// # Usage
//
//	// Initialize tracer
//	tp, err := otel.NewTracerProvider(otel.TracerConfig{
//		ServiceName: "my-agent",
//		Endpoint:    "localhost:4317",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer tp.Shutdown(context.Background())
//
//	// Create tracer
//	tracer := otel.NewTracer(tp)
//
//	// Use with agent engine
//	engine, err := api.New(
//		api.WithTracer(tracer),
//	)
//
// # Exported Spans
//
// The tracer creates spans for:
//   - Run lifecycle (start, complete, fail)
//   - State transitions
//   - Tool executions
//   - Planner decisions
//
// All spans include relevant attributes like run ID, state, tool name, etc.
package otel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Common errors for OpenTelemetry operations.
var (
	ErrShutdown        = errors.New("tracer provider shutdown")
	ErrInvalidEndpoint = errors.New("invalid endpoint")
	ErrExporterFailed  = errors.New("exporter initialization failed")
)

// TracerConfig configures the OpenTelemetry tracer provider.
type TracerConfig struct {
	// ServiceName is the name of the service for tracing.
	ServiceName string

	// ServiceVersion is the version of the service.
	ServiceVersion string

	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317").
	Endpoint string

	// Insecure disables TLS for the exporter connection.
	Insecure bool

	// SampleRate controls the trace sampling rate (0.0 to 1.0).
	SampleRate float64

	// ExporterType specifies the exporter ("otlp", "stdout", "none").
	ExporterType string

	// Headers are additional headers for the OTLP exporter.
	Headers map[string]string

	// BatchSize is the maximum number of spans per batch.
	BatchSize int

	// BatchTimeout is the maximum time to wait before sending a batch.
	BatchTimeout int // milliseconds

	// ResourceAttributes are additional resource attributes.
	ResourceAttributes map[string]string
}

// TracerProvider wraps the OpenTelemetry tracer provider.
type TracerProvider struct {
	config   TracerConfig
	provider *sdktrace.TracerProvider
	shutdown bool
}

// NewTracerProvider creates a new OpenTelemetry tracer provider.
func NewTracerProvider(cfg TracerConfig) (*TracerProvider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "agent-go"
	}
	if cfg.ExporterType == "" {
		cfg.ExporterType = "otlp"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0 // Sample all traces by default
	}

	ctx := context.Background()

	// 1. Create resource with service name and version
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
	}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.ServiceVersion))
	}
	// Add additional resource attributes
	for k, v := range cfg.ResourceAttributes {
		attrs = append(attrs, attribute.String(k, v))
	}
	res := resource.NewWithAttributes(semconv.SchemaURL, attrs...)

	// 2. Create sampler
	sampler := sdktrace.TraceIDRatioBased(cfg.SampleRate)

	// 3. Create exporter based on ExporterType
	var spanProcessor sdktrace.SpanProcessor
	switch cfg.ExporterType {
	case "otlp":
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("%w: endpoint required for OTLP exporter", ErrInvalidEndpoint)
		}
		var exporterOpts []otlptracehttp.Option
		exporterOpts = append(exporterOpts, otlptracehttp.WithEndpoint(cfg.Endpoint))
		if cfg.Insecure {
			exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			exporterOpts = append(exporterOpts, otlptracehttp.WithHeaders(cfg.Headers))
		}
		exporter, err := otlptracehttp.New(ctx, exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrExporterFailed, err)
		}
		spanProcessor = sdktrace.NewBatchSpanProcessor(exporter)

	case "stdout":
		exporter, err := stdouttrace.New()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrExporterFailed, err)
		}
		spanProcessor = sdktrace.NewBatchSpanProcessor(exporter)

	case "none":
		// No exporter, no processor - just the provider with resource and sampler
		spanProcessor = nil

	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", cfg.ExporterType)
	}

	// 4. Create and register tracer provider
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}
	if spanProcessor != nil {
		opts = append(opts, sdktrace.WithSpanProcessor(spanProcessor))
	}
	provider := sdktrace.NewTracerProvider(opts...)

	return &TracerProvider{
		config:   cfg,
		provider: provider,
	}, nil
}

// Shutdown gracefully shuts down the tracer provider.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.shutdown {
		return nil
	}
	tp.shutdown = true

	if tp.provider != nil {
		return tp.provider.Shutdown(ctx)
	}
	return nil
}

// Tracer returns a new tracer from this provider.
func (tp *TracerProvider) Tracer(name string) *Tracer {
	var tracer oteltrace.Tracer
	if tp.provider != nil {
		tracer = tp.provider.Tracer(name)
	}
	return &Tracer{
		provider: tp,
		name:     name,
		tracer:   tracer,
	}
}

// Tracer implements the telemetry.Tracer interface using OpenTelemetry.
type Tracer struct {
	provider *TracerProvider
	name     string
	tracer   oteltrace.Tracer
}

// NewTracer creates a tracer from the global provider.
// Use TracerProvider.Tracer() for explicit provider control.
func NewTracer(name string) *Tracer {
	return &Tracer{
		name: name,
	}
}

// StartSpan starts a new span and returns a new context containing the span.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...telemetry.SpanOption) (context.Context, telemetry.Span) {
	// Apply options
	cfg := &telemetry.SpanConfig{}
	for _, opt := range opts {
		opt.ApplySpan(cfg)
	}

	// If no tracer is available, return no-op span
	if t.tracer == nil {
		span := &Span{
			name:   name,
			tracer: t,
		}
		return ctx, span
	}

	// Build OpenTelemetry span options
	otelOpts := []oteltrace.SpanStartOption{}

	// Convert span kind
	switch cfg.Kind {
	case telemetry.SpanKindInternal:
		otelOpts = append(otelOpts, oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	case telemetry.SpanKindServer:
		otelOpts = append(otelOpts, oteltrace.WithSpanKind(oteltrace.SpanKindServer))
	case telemetry.SpanKindClient:
		otelOpts = append(otelOpts, oteltrace.WithSpanKind(oteltrace.SpanKindClient))
	case telemetry.SpanKindProducer:
		otelOpts = append(otelOpts, oteltrace.WithSpanKind(oteltrace.SpanKindProducer))
	case telemetry.SpanKindConsumer:
		otelOpts = append(otelOpts, oteltrace.WithSpanKind(oteltrace.SpanKindConsumer))
	}

	// Convert initial attributes
	if len(cfg.Attributes) > 0 {
		otelAttrs := convertAttributes(cfg.Attributes)
		otelOpts = append(otelOpts, oteltrace.WithAttributes(otelAttrs...))
	}

	// Start the span
	ctx, otelSpan := t.tracer.Start(ctx, name, otelOpts...)

	span := &Span{
		name:   name,
		tracer: t,
		span:   otelSpan,
	}

	return ctx, span
}

// Span implements the telemetry.Span interface using OpenTelemetry.
type Span struct {
	name   string
	tracer *Tracer
	span   oteltrace.Span
}

// End completes the span.
func (s *Span) End() {
	if s.span != nil {
		s.span.End()
	}
}

// SetAttributes sets attributes on the span.
func (s *Span) SetAttributes(attrs ...telemetry.Attribute) {
	if s.span != nil && len(attrs) > 0 {
		otelAttrs := convertAttributes(attrs)
		s.span.SetAttributes(otelAttrs...)
	}
}

// RecordError records an error on the span.
func (s *Span) RecordError(err error) {
	if s.span != nil && err != nil {
		s.span.RecordError(err)
	}
}

// SetStatus sets the span status.
func (s *Span) SetStatus(code telemetry.StatusCode, description string) {
	if s.span == nil {
		return
	}

	var otelCode codes.Code
	switch code {
	case telemetry.StatusCodeOK:
		otelCode = codes.Ok
	case telemetry.StatusCodeError:
		otelCode = codes.Error
	default:
		otelCode = codes.Unset
	}

	s.span.SetStatus(otelCode, description)
}

// AddEvent adds an event to the span.
func (s *Span) AddEvent(name string, attrs ...telemetry.Attribute) {
	if s.span == nil {
		return
	}

	if len(attrs) > 0 {
		otelAttrs := convertAttributes(attrs)
		s.span.AddEvent(name, oteltrace.WithAttributes(otelAttrs...))
	} else {
		s.span.AddEvent(name)
	}
}

// convertAttributes converts telemetry attributes to OpenTelemetry attributes.
func convertAttributes(attrs []telemetry.Attribute) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}

	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		var kv attribute.KeyValue
		switch v := attr.Value.(type) {
		case string:
			kv = attribute.String(attr.Key, v)
		case int:
			kv = attribute.Int(attr.Key, v)
		case int64:
			kv = attribute.Int64(attr.Key, v)
		case float64:
			kv = attribute.Float64(attr.Key, v)
		case bool:
			kv = attribute.Bool(attr.Key, v)
		default:
			// For unsupported types, convert to string
			kv = attribute.String(attr.Key, fmt.Sprintf("%v", v))
		}
		otelAttrs = append(otelAttrs, kv)
	}
	return otelAttrs
}

// MeterConfig configures the OpenTelemetry meter provider.
type MeterConfig struct {
	// ServiceName is the name of the service for metrics.
	ServiceName string

	// Endpoint is the OTLP collector endpoint.
	Endpoint string

	// Insecure disables TLS for the exporter connection.
	Insecure bool

	// ExportInterval is how often to export metrics.
	ExportInterval int // seconds

	// ExporterType specifies the exporter ("otlp", "stdout", "none").
	ExporterType string
}

// MeterProvider wraps the OpenTelemetry meter provider.
type MeterProvider struct {
	config   MeterConfig
	provider *metric.MeterProvider
}

// NewMeterProvider creates a new OpenTelemetry meter provider.
func NewMeterProvider(cfg MeterConfig) (*MeterProvider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "agent-go"
	}
	if cfg.ExporterType == "" {
		cfg.ExporterType = "otlp"
	}
	if cfg.ExportInterval == 0 {
		cfg.ExportInterval = 60
	}

	ctx := context.Background()

	// 1. Create resource with service name
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
	)

	// 2. Create exporter based on ExporterType
	var reader metric.Reader
	switch cfg.ExporterType {
	case "otlp":
		// Note: OTLP metric exporter would require otlpmetrichttp package
		// For now, fall back to stdout for OTLP until we add that dependency
		fallthrough
	case "stdout":
		exporter, err := stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrExporterFailed, err)
		}
		reader = metric.NewPeriodicReader(
			exporter,
			metric.WithInterval(time.Duration(cfg.ExportInterval)*time.Second),
		)

	case "none":
		// No exporter, no reader - metrics recorded but not exported
		reader = nil

	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", cfg.ExporterType)
	}

	// 3. Build meter provider
	opts := []metric.Option{
		metric.WithResource(res),
	}
	if reader != nil {
		opts = append(opts, metric.WithReader(reader))
	}
	provider := metric.NewMeterProvider(opts...)

	_ = ctx // used for potential future exporter initialization

	return &MeterProvider{
		config:   cfg,
		provider: provider,
	}, nil
}

// Shutdown gracefully shuts down the meter provider.
func (mp *MeterProvider) Shutdown(ctx context.Context) error {
	if mp.provider != nil {
		return mp.provider.Shutdown(ctx)
	}
	return nil
}

// Meter returns a new meter from this provider.
func (mp *MeterProvider) Meter(name string) *Meter {
	var meter otelmetric.Meter
	if mp.provider != nil {
		meter = mp.provider.Meter(name)
	}
	return &Meter{
		provider: mp,
		name:     name,
		meter:    meter,
	}
}

// Meter implements the telemetry.Meter interface using OpenTelemetry.
type Meter struct {
	provider *MeterProvider
	name     string
	meter    otelmetric.Meter
}

// NewMeter creates a meter from the global provider.
func NewMeter(name string) *Meter {
	return &Meter{
		name: name,
	}
}

// Counter creates a new counter metric.
func (m *Meter) Counter(name string, opts ...telemetry.MetricOption) telemetry.Counter {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	c := &Counter{name: name, meter: m}

	if m.meter != nil {
		counter, err := m.meter.Int64Counter(
			name,
			otelmetric.WithDescription(cfg.Description),
			otelmetric.WithUnit(cfg.Unit),
		)
		if err == nil {
			c.counter = counter
		}
	}

	return c
}

// Histogram creates a new histogram metric.
func (m *Meter) Histogram(name string, opts ...telemetry.MetricOption) telemetry.Histogram {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	h := &Histogram{name: name, meter: m}

	if m.meter != nil {
		histogram, err := m.meter.Float64Histogram(
			name,
			otelmetric.WithDescription(cfg.Description),
			otelmetric.WithUnit(cfg.Unit),
		)
		if err == nil {
			h.histogram = histogram
		}
	}

	return h
}

// Gauge creates a new gauge metric.
func (m *Meter) Gauge(name string, opts ...telemetry.MetricOption) telemetry.Gauge {
	cfg := &telemetry.MetricConfig{}
	for _, opt := range opts {
		opt.ApplyMetric(cfg)
	}

	g := &Gauge{name: name, meter: m}

	if m.meter != nil {
		gauge, err := m.meter.Float64Gauge(
			name,
			otelmetric.WithDescription(cfg.Description),
			otelmetric.WithUnit(cfg.Unit),
		)
		if err == nil {
			g.gauge = gauge
		}
	}

	return g
}

// Counter implements telemetry.Counter using OpenTelemetry.
type Counter struct {
	name    string
	meter   *Meter
	counter otelmetric.Int64Counter
}

// Add adds a value to the counter.
func (c *Counter) Add(ctx context.Context, value int64, attrs ...telemetry.Attribute) {
	if c.counter == nil {
		return
	}

	if len(attrs) > 0 {
		otelAttrs := convertAttributes(attrs)
		c.counter.Add(ctx, value, otelmetric.WithAttributes(otelAttrs...))
	} else {
		c.counter.Add(ctx, value)
	}
}

// Histogram implements telemetry.Histogram using OpenTelemetry.
type Histogram struct {
	name      string
	meter     *Meter
	histogram otelmetric.Float64Histogram
}

// Record records a value to the histogram.
func (h *Histogram) Record(ctx context.Context, value float64, attrs ...telemetry.Attribute) {
	if h.histogram == nil {
		return
	}

	if len(attrs) > 0 {
		otelAttrs := convertAttributes(attrs)
		h.histogram.Record(ctx, value, otelmetric.WithAttributes(otelAttrs...))
	} else {
		h.histogram.Record(ctx, value)
	}
}

// Gauge implements telemetry.Gauge using OpenTelemetry.
type Gauge struct {
	name  string
	meter *Meter
	gauge otelmetric.Float64Gauge
}

// Record records the current value.
func (g *Gauge) Record(ctx context.Context, value float64, attrs ...telemetry.Attribute) {
	if g.gauge == nil {
		return
	}

	if len(attrs) > 0 {
		otelAttrs := convertAttributes(attrs)
		g.gauge.Record(ctx, value, otelmetric.WithAttributes(otelAttrs...))
	} else {
		g.gauge.Record(ctx, value)
	}
}

// Predefined span and metric names for agent operations.
const (
	// Span names
	SpanRunStart        = "agent.run.start"
	SpanRunComplete     = "agent.run.complete"
	SpanStateTransition = "agent.state.transition"
	SpanToolExecute     = "agent.tool.execute"
	SpanPlannerDecide   = "agent.planner.decide"
	SpanApprovalWait    = "agent.approval.wait"

	// Metric names
	MetricRunDuration  = "agent.run.duration"
	MetricRunCount     = "agent.run.count"
	MetricToolCalls    = "agent.tool.calls"
	MetricToolDuration = "agent.tool.duration"
	MetricToolErrors   = "agent.tool.errors"
	MetricStateChanges = "agent.state.changes"
	MetricApprovalWait = "agent.approval.wait_time"
)

// Attribute key constants for consistent labeling.
const (
	AttrRunID    = "agent.run.id"
	AttrGoal     = "agent.run.goal"
	AttrState    = "agent.state"
	AttrToolName = "agent.tool.name"
	AttrDecision = "agent.decision.type"
	AttrError    = "agent.error"
	AttrApproved = "agent.approval.approved"
)

// Ensure interfaces are satisfied.
var (
	_ telemetry.Tracer    = (*Tracer)(nil)
	_ telemetry.Span      = (*Span)(nil)
	_ telemetry.Meter     = (*Meter)(nil)
	_ telemetry.Counter   = (*Counter)(nil)
	_ telemetry.Histogram = (*Histogram)(nil)
	_ telemetry.Gauge     = (*Gauge)(nil)
)
