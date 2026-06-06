package telemetry_test

import (
	"testing"

	"go.klarlabs.de/agent/domain/telemetry"
)

func TestWithAttributes(t *testing.T) {
	t.Parallel()

	t.Run("adds attributes to config", func(t *testing.T) {
		t.Parallel()

		attr1 := telemetry.String("key1", "value1")
		attr2 := telemetry.Int("key2", 42)

		opt := telemetry.WithAttributes(attr1, attr2)

		config := &telemetry.SpanConfig{}
		opt.ApplySpan(config)

		if len(config.Attributes) != 2 {
			t.Fatalf("Attributes len = %d, want 2", len(config.Attributes))
		}
		if config.Attributes[0].Key != "key1" {
			t.Errorf("Attributes[0].Key = %s, want key1", config.Attributes[0].Key)
		}
	})

	t.Run("appends to existing attributes", func(t *testing.T) {
		t.Parallel()

		config := &telemetry.SpanConfig{
			Attributes: []telemetry.Attribute{telemetry.String("existing", "value")},
		}

		opt := telemetry.WithAttributes(telemetry.Int("new", 1))
		opt.ApplySpan(config)

		if len(config.Attributes) != 2 {
			t.Fatalf("Attributes len = %d, want 2", len(config.Attributes))
		}
	})
}

func TestWithSpanKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind telemetry.SpanKind
	}{
		{"Internal", telemetry.SpanKindInternal},
		{"Server", telemetry.SpanKindServer},
		{"Client", telemetry.SpanKindClient},
		{"Producer", telemetry.SpanKindProducer},
		{"Consumer", telemetry.SpanKindConsumer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opt := telemetry.WithSpanKind(tt.kind)

			config := &telemetry.SpanConfig{}
			opt.ApplySpan(config)

			if config.Kind != tt.kind {
				t.Errorf("Kind = %d, want %d", config.Kind, tt.kind)
			}
		})
	}
}

func TestSpanKindConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		kind     telemetry.SpanKind
		expected int
	}{
		{"Unspecified", telemetry.SpanKindUnspecified, 0},
		{"Internal", telemetry.SpanKindInternal, 1},
		{"Server", telemetry.SpanKindServer, 2},
		{"Client", telemetry.SpanKindClient, 3},
		{"Producer", telemetry.SpanKindProducer, 4},
		{"Consumer", telemetry.SpanKindConsumer, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if int(tt.kind) != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.kind, tt.expected)
			}
		})
	}
}

func TestStatusCodeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     telemetry.StatusCode
		expected int
	}{
		{"Unset", telemetry.StatusCodeUnset, 0},
		{"OK", telemetry.StatusCodeOK, 1},
		{"Error", telemetry.StatusCodeError, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if int(tt.code) != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.expected)
			}
		})
	}
}

func TestAttribute(t *testing.T) {
	t.Parallel()

	t.Run("holds key-value pair", func(t *testing.T) {
		t.Parallel()

		attr := telemetry.Attribute{
			Key:   "test_key",
			Value: "test_value",
		}

		if attr.Key != "test_key" {
			t.Errorf("Key = %s", attr.Key)
		}
		if attr.Value != "test_value" {
			t.Errorf("Value = %v", attr.Value)
		}
	})
}

func TestString(t *testing.T) {
	t.Parallel()

	attr := telemetry.String("name", "alice")

	if attr.Key != "name" {
		t.Errorf("Key = %s, want name", attr.Key)
	}
	if attr.Value != "alice" {
		t.Errorf("Value = %v, want alice", attr.Value)
	}
}

func TestInt(t *testing.T) {
	t.Parallel()

	attr := telemetry.Int("count", 42)

	if attr.Key != "count" {
		t.Errorf("Key = %s, want count", attr.Key)
	}
	if attr.Value != 42 {
		t.Errorf("Value = %v, want 42", attr.Value)
	}
}

func TestInt64(t *testing.T) {
	t.Parallel()

	attr := telemetry.Int64("big_count", int64(9223372036854775807))

	if attr.Key != "big_count" {
		t.Errorf("Key = %s, want big_count", attr.Key)
	}
	if attr.Value != int64(9223372036854775807) {
		t.Errorf("Value = %v", attr.Value)
	}
}

func TestFloat64(t *testing.T) {
	t.Parallel()

	attr := telemetry.Float64("rate", 3.14159)

	if attr.Key != "rate" {
		t.Errorf("Key = %s, want rate", attr.Key)
	}
	if attr.Value != 3.14159 {
		t.Errorf("Value = %v, want 3.14159", attr.Value)
	}
}

func TestBool(t *testing.T) {
	t.Parallel()

	t.Run("true value", func(t *testing.T) {
		t.Parallel()

		attr := telemetry.Bool("enabled", true)

		if attr.Key != "enabled" {
			t.Errorf("Key = %s, want enabled", attr.Key)
		}
		if attr.Value != true {
			t.Errorf("Value = %v, want true", attr.Value)
		}
	})

	t.Run("false value", func(t *testing.T) {
		t.Parallel()

		attr := telemetry.Bool("disabled", false)

		if attr.Value != false {
			t.Errorf("Value = %v, want false", attr.Value)
		}
	})
}

func TestSpanConfig(t *testing.T) {
	t.Parallel()

	config := telemetry.SpanConfig{
		Attributes: []telemetry.Attribute{
			telemetry.String("service", "agent"),
			telemetry.Int("version", 1),
		},
		Kind: telemetry.SpanKindServer,
	}

	if len(config.Attributes) != 2 {
		t.Errorf("Attributes len = %d, want 2", len(config.Attributes))
	}
	if config.Kind != telemetry.SpanKindServer {
		t.Errorf("Kind = %d, want SpanKindServer", config.Kind)
	}
}

func TestWithDescription(t *testing.T) {
	t.Parallel()

	opt := telemetry.WithDescription("Total number of requests")

	config := &telemetry.MetricConfig{}
	opt.ApplyMetric(config)

	if config.Description != "Total number of requests" {
		t.Errorf("Description = %s", config.Description)
	}
}

func TestWithUnit(t *testing.T) {
	t.Parallel()

	opt := telemetry.WithUnit("ms")

	config := &telemetry.MetricConfig{}
	opt.ApplyMetric(config)

	if config.Unit != "ms" {
		t.Errorf("Unit = %s, want ms", config.Unit)
	}
}

func TestMetricConfig(t *testing.T) {
	t.Parallel()

	config := telemetry.MetricConfig{
		Description: "Request latency",
		Unit:        "s",
	}

	if config.Description != "Request latency" {
		t.Errorf("Description = %s", config.Description)
	}
	if config.Unit != "s" {
		t.Errorf("Unit = %s", config.Unit)
	}
}

func TestMetricOptionFunc(t *testing.T) {
	t.Parallel()

	// Create a custom metric option
	customOpt := telemetry.MetricOptionFunc(func(c *telemetry.MetricConfig) {
		c.Description = "custom"
		c.Unit = "custom_unit"
	})

	config := &telemetry.MetricConfig{}
	customOpt.ApplyMetric(config)

	if config.Description != "custom" {
		t.Errorf("Description = %s, want custom", config.Description)
	}
	if config.Unit != "custom_unit" {
		t.Errorf("Unit = %s, want custom_unit", config.Unit)
	}
}

func TestSpanOptionFunc(t *testing.T) {
	t.Parallel()

	// Create a custom span option
	customOpt := telemetry.SpanOptionFunc(func(c *telemetry.SpanConfig) {
		c.Kind = telemetry.SpanKindClient
		c.Attributes = append(c.Attributes, telemetry.String("custom", "value"))
	})

	config := &telemetry.SpanConfig{}
	customOpt.ApplySpan(config)

	if config.Kind != telemetry.SpanKindClient {
		t.Errorf("Kind = %d, want SpanKindClient", config.Kind)
	}
	if len(config.Attributes) != 1 {
		t.Errorf("Attributes len = %d, want 1", len(config.Attributes))
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
			name: "ErrTracerNotConfigured",
			err:  telemetry.ErrTracerNotConfigured,
			msg:  "tracer not configured",
		},
		{
			name: "ErrMeterNotConfigured",
			err:  telemetry.ErrMeterNotConfigured,
			msg:  "meter not configured",
		},
		{
			name: "ErrExporterFailed",
			err:  telemetry.ErrExporterFailed,
			msg:  "exporter failed",
		},
		{
			name: "ErrShutdownFailed",
			err:  telemetry.ErrShutdownFailed,
			msg:  "shutdown failed",
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
