package otel

import (
	"context"
	"errors"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/telemetry"
)

func TestTracerProvider(t *testing.T) {
	tests := []struct {
		name        string
		config      TracerConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "none exporter",
			config: TracerConfig{
				ServiceName:  "test-service",
				ExporterType: "none",
			},
			wantErr: false,
		},
		{
			name: "stdout exporter",
			config: TracerConfig{
				ServiceName:  "test-service",
				ExporterType: "stdout",
			},
			wantErr: false,
		},
		{
			name: "otlp exporter without endpoint",
			config: TracerConfig{
				ServiceName:  "test-service",
				ExporterType: "otlp",
			},
			wantErr:     true,
			errContains: "endpoint required",
		},
		{
			name: "otlp exporter with endpoint",
			config: TracerConfig{
				ServiceName:  "test-service",
				ExporterType: "otlp",
				Endpoint:     "localhost:4317",
				Insecure:     true,
			},
			wantErr: false,
		},
		{
			name: "default config",
			config: TracerConfig{
				ServiceName: "test-service",
			},
			wantErr:     true,
			errContains: "endpoint required", // defaults to otlp
		},
		{
			name: "with resource attributes",
			config: TracerConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				ExporterType:   "none",
				ResourceAttributes: map[string]string{
					"environment": "test",
					"region":      "us-west-2",
				},
			},
			wantErr: false,
		},
		{
			name: "custom sample rate",
			config: TracerConfig{
				ServiceName:  "test-service",
				ExporterType: "none",
				SampleRate:   0.5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp, err := NewTracerProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewTracerProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}
			if err != nil {
				return
			}

			defer func() {
				if err := tp.Shutdown(context.Background()); err != nil {
					t.Errorf("Shutdown() error = %v", err)
				}
			}()

			// Test double shutdown
			if err := tp.Shutdown(context.Background()); err != nil {
				t.Errorf("Second Shutdown() error = %v", err)
			}
		})
	}
}

func TestTracer(t *testing.T) {
	tp, err := NewTracerProvider(TracerConfig{
		ServiceName:  "test-service",
		ExporterType: "none",
	})
	if err != nil {
		t.Fatalf("NewTracerProvider() error = %v", err)
	}
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test-tracer")
	if tracer == nil {
		t.Fatal("Tracer() returned nil")
	}
	if tracer.name != "test-tracer" {
		t.Errorf("tracer.name = %s, want test-tracer", tracer.name)
	}
}

func TestSpan(t *testing.T) {
	tp, err := NewTracerProvider(TracerConfig{
		ServiceName:  "test-service",
		ExporterType: "none",
	})
	if err != nil {
		t.Fatalf("NewTracerProvider() error = %v", err)
	}
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("test")
	ctx := context.Background()

	t.Run("basic span", func(t *testing.T) {
		_, span := tracer.StartSpan(ctx, "test-span")
		if span == nil {
			t.Fatal("StartSpan() returned nil span")
		}
		span.End()
	})

	t.Run("span with attributes", func(t *testing.T) {
		_, span := tracer.StartSpan(ctx, "test-span",
			telemetry.WithAttributes(
				telemetry.String("key", "value"),
				telemetry.Int("count", 42),
				telemetry.Bool("enabled", true),
			),
		)
		span.SetAttributes(
			telemetry.String("added", "later"),
			telemetry.Float64("duration", 123.45),
			telemetry.Int64("timestamp", 1234567890),
		)
		span.End()
	})

	t.Run("span with kind", func(t *testing.T) {
		_, span := tracer.StartSpan(ctx, "test-span",
			telemetry.WithSpanKind(telemetry.SpanKindClient),
		)
		span.End()
	})

	t.Run("span with error", func(t *testing.T) {
		_, span := tracer.StartSpan(ctx, "test-span")
		span.RecordError(errors.New("test error"))
		span.SetStatus(telemetry.StatusCodeError, "operation failed")
		span.End()
	})

	t.Run("span with events", func(t *testing.T) {
		_, span := tracer.StartSpan(ctx, "test-span")
		span.AddEvent("event1")
		span.AddEvent("event2",
			telemetry.String("detail", "important"),
			telemetry.Int("code", 200),
		)
		span.End()
	})

	t.Run("span status transitions", func(t *testing.T) {
		_, span := tracer.StartSpan(ctx, "test-span")
		span.SetStatus(telemetry.StatusCodeUnset, "")
		span.SetStatus(telemetry.StatusCodeOK, "success")
		span.SetStatus(telemetry.StatusCodeError, "failed")
		span.End()
	})
}

func TestMeterProvider(t *testing.T) {
	tests := []struct {
		name        string
		config      MeterConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "none exporter",
			config: MeterConfig{
				ServiceName:  "test-service",
				ExporterType: "none",
			},
			wantErr: false,
		},
		{
			name: "stdout exporter",
			config: MeterConfig{
				ServiceName:  "test-service",
				ExporterType: "stdout",
			},
			wantErr: false,
		},
		{
			name: "custom export interval",
			config: MeterConfig{
				ServiceName:    "test-service",
				ExporterType:   "none",
				ExportInterval: 30,
			},
			wantErr: false,
		},
		{
			name: "default config",
			config: MeterConfig{
				ServiceName: "test-service",
			},
			wantErr: false, // OTLP falls back to stdout for now
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp, err := NewMeterProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewMeterProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}
			if err != nil {
				return
			}

			defer func() {
				if err := mp.Shutdown(context.Background()); err != nil {
					t.Errorf("Shutdown() error = %v", err)
				}
			}()
		})
	}
}

func TestMeter(t *testing.T) {
	mp, err := NewMeterProvider(MeterConfig{
		ServiceName:  "test-service",
		ExporterType: "none",
	})
	if err != nil {
		t.Fatalf("NewMeterProvider() error = %v", err)
	}
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test-meter")
	if meter == nil {
		t.Fatal("Meter() returned nil")
	}
	if meter.name != "test-meter" {
		t.Errorf("meter.name = %s, want test-meter", meter.name)
	}
}

func TestCounter(t *testing.T) {
	mp, err := NewMeterProvider(MeterConfig{
		ServiceName:  "test-service",
		ExporterType: "none",
	})
	if err != nil {
		t.Fatalf("NewMeterProvider() error = %v", err)
	}
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	ctx := context.Background()

	t.Run("basic counter", func(t *testing.T) {
		counter := meter.Counter("test_counter")
		if counter == nil {
			t.Fatal("Counter() returned nil")
		}
		counter.Add(ctx, 1)
		counter.Add(ctx, 5)
	})

	t.Run("counter with description and unit", func(t *testing.T) {
		counter := meter.Counter("test_counter",
			telemetry.WithDescription("Test counter"),
			telemetry.WithUnit("requests"),
		)
		counter.Add(ctx, 10)
	})

	t.Run("counter with attributes", func(t *testing.T) {
		counter := meter.Counter("test_counter")
		counter.Add(ctx, 1,
			telemetry.String("method", "GET"),
			telemetry.String("status", "200"),
		)
		counter.Add(ctx, 1,
			telemetry.String("method", "POST"),
			telemetry.String("status", "201"),
		)
	})
}

func TestHistogram(t *testing.T) {
	mp, err := NewMeterProvider(MeterConfig{
		ServiceName:  "test-service",
		ExporterType: "none",
	})
	if err != nil {
		t.Fatalf("NewMeterProvider() error = %v", err)
	}
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	ctx := context.Background()

	t.Run("basic histogram", func(t *testing.T) {
		histogram := meter.Histogram("test_histogram")
		if histogram == nil {
			t.Fatal("Histogram() returned nil")
		}
		histogram.Record(ctx, 10.5)
		histogram.Record(ctx, 20.3)
		histogram.Record(ctx, 15.7)
	})

	t.Run("histogram with description and unit", func(t *testing.T) {
		histogram := meter.Histogram("test_histogram",
			telemetry.WithDescription("Test histogram"),
			telemetry.WithUnit("ms"),
		)
		histogram.Record(ctx, 100.0)
	})

	t.Run("histogram with attributes", func(t *testing.T) {
		histogram := meter.Histogram("test_histogram")
		histogram.Record(ctx, 50.0,
			telemetry.String("endpoint", "/api/users"),
			telemetry.String("method", "GET"),
		)
		histogram.Record(ctx, 150.0,
			telemetry.String("endpoint", "/api/posts"),
			telemetry.String("method", "POST"),
		)
	})
}

func TestGauge(t *testing.T) {
	mp, err := NewMeterProvider(MeterConfig{
		ServiceName:  "test-service",
		ExporterType: "none",
	})
	if err != nil {
		t.Fatalf("NewMeterProvider() error = %v", err)
	}
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	ctx := context.Background()

	t.Run("basic gauge", func(t *testing.T) {
		gauge := meter.Gauge("test_gauge")
		if gauge == nil {
			t.Fatal("Gauge() returned nil")
		}
		gauge.Record(ctx, 42.0)
		gauge.Record(ctx, 37.5)
	})

	t.Run("gauge with description and unit", func(t *testing.T) {
		gauge := meter.Gauge("test_gauge",
			telemetry.WithDescription("Test gauge"),
			telemetry.WithUnit("connections"),
		)
		gauge.Record(ctx, 100.0)
	})

	t.Run("gauge with attributes", func(t *testing.T) {
		gauge := meter.Gauge("test_gauge")
		gauge.Record(ctx, 75.0,
			telemetry.String("pool", "main"),
			telemetry.String("region", "us-west-2"),
		)
		gauge.Record(ctx, 50.0,
			telemetry.String("pool", "secondary"),
			telemetry.String("region", "us-east-1"),
		)
	})
}

func TestAttributeConversion(t *testing.T) {
	tests := []struct {
		name  string
		attrs []telemetry.Attribute
	}{
		{
			name:  "empty attributes",
			attrs: []telemetry.Attribute{},
		},
		{
			name: "string attributes",
			attrs: []telemetry.Attribute{
				telemetry.String("key1", "value1"),
				telemetry.String("key2", "value2"),
			},
		},
		{
			name: "int attributes",
			attrs: []telemetry.Attribute{
				telemetry.Int("count", 42),
				telemetry.Int("size", 1024),
			},
		},
		{
			name: "int64 attributes",
			attrs: []telemetry.Attribute{
				telemetry.Int64("timestamp", 1234567890),
				telemetry.Int64("bytes", 9876543210),
			},
		},
		{
			name: "float64 attributes",
			attrs: []telemetry.Attribute{
				telemetry.Float64("duration", 123.45),
				telemetry.Float64("rate", 0.95),
			},
		},
		{
			name: "bool attributes",
			attrs: []telemetry.Attribute{
				telemetry.Bool("enabled", true),
				telemetry.Bool("cached", false),
			},
		},
		{
			name: "mixed attributes",
			attrs: []telemetry.Attribute{
				telemetry.String("name", "test"),
				telemetry.Int("count", 10),
				telemetry.Float64("score", 98.5),
				telemetry.Bool("success", true),
				telemetry.Int64("id", 123456),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			otelAttrs := convertAttributes(tt.attrs)
			if len(otelAttrs) != len(tt.attrs) {
				t.Errorf("convertAttributes() returned %d attributes, want %d",
					len(otelAttrs), len(tt.attrs))
			}
		})
	}
}

func TestNoOpBehavior(t *testing.T) {
	t.Run("tracer without provider", func(t *testing.T) {
		tracer := NewTracer("test")
		_, span := tracer.StartSpan(context.Background(), "test-span")
		span.SetAttributes(telemetry.String("key", "value"))
		span.RecordError(errors.New("test"))
		span.SetStatus(telemetry.StatusCodeError, "failed")
		span.AddEvent("event")
		span.End()
	})

	t.Run("meter without provider", func(t *testing.T) {
		meter := NewMeter("test")
		ctx := context.Background()

		counter := meter.Counter("test_counter")
		counter.Add(ctx, 1)

		histogram := meter.Histogram("test_histogram")
		histogram.Record(ctx, 10.0)

		gauge := meter.Gauge("test_gauge")
		gauge.Record(ctx, 42.0)
	})
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
