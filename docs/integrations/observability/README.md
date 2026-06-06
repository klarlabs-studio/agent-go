# Observability

agent-go integrates with OpenTelemetry for distributed tracing and metrics collection.

## Quick Start

```go
import (
    "go.klarlabs.de/agent/infrastructure/observability"
    api "go.klarlabs.de/agent/interfaces/api"
)

// Create tracer and meter
tracer := observability.NewOTelTracer("my-agent")
meter := observability.NewOTelMeter("my-agent")

// Use with engine
engine, _ := api.New(
    api.WithPlanner(planner),
    api.WithMiddleware(
        observability.TracingMiddleware(tracer),
        observability.MetricsMiddleware(meter),
        api.LoggingMiddleware(nil),
    ),
)
```

## Tracing

The tracing middleware creates spans for agent operations:

```
agent.run (root span)
├── planner.decide
├── tool.execute: read_file
│   └── (tool-specific spans)
├── planner.decide
├── tool.execute: process_data
└── planner.decide (finish)
```

### Configuration

```go
// Create tracer with service name
tracer := observability.NewOTelTracer("my-service")

// Apply to engine
engine, _ := api.New(
    api.WithMiddleware(
        observability.TracingMiddleware(tracer),
    ),
)
```

### Span Attributes

Spans include these attributes:

| Attribute | Description |
|-----------|-------------|
| `agent.run_id` | Unique run identifier |
| `agent.state` | Current agent state |
| `tool.name` | Tool being executed |
| `tool.duration_ms` | Execution time in milliseconds |
| `tool.success` | Whether execution succeeded |
| `tool.cached` | Whether result was from cache |

### Custom Spans

Add spans within tool handlers:

```go
tool := api.NewToolBuilder("my_tool").
    WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
        ctx, span := tracer.StartSpan(ctx, "my_tool.process")
        defer span.End()

        span.SetAttributes(
            attribute.String("input.size", strconv.Itoa(len(input))),
        )

        // Do work...

        return result, nil
    }).
    MustBuild()
```

## Metrics

The metrics middleware collects operational metrics:

### Predefined Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `agent_tool_executions_total` | Counter | tool, state, status | Total tool executions |
| `agent_tool_duration_seconds` | Histogram | tool | Tool execution duration |
| `agent_runs_total` | Counter | status | Total agent runs |
| `agent_run_duration_seconds` | Histogram | - | Run duration |
| `agent_steps_total` | Counter | run_id | Steps per run |
| `agent_budget_remaining` | Gauge | budget | Remaining budget |

### Configuration

```go
// Create meter with service name
meter := observability.NewOTelMeter("my-service")

// Apply to engine
engine, _ := api.New(
    api.WithMiddleware(
        observability.MetricsMiddleware(meter),
    ),
)
```

### Custom Metrics

Record custom metrics in tool handlers:

```go
import (
    "go.opentelemetry.io/otel/metric"
)

var (
    requestCounter metric.Int64Counter
    requestLatency metric.Float64Histogram
)

func init() {
    meter := observability.NewOTelMeter("my-service")
    requestCounter, _ = meter.Int64Counter("my_tool.requests")
    requestLatency, _ = meter.Float64Histogram("my_tool.latency")
}

func myToolHandler(ctx context.Context, input json.RawMessage) (tool.Result, error) {
    start := time.Now()
    defer func() {
        requestLatency.Record(ctx, time.Since(start).Seconds())
    }()

    requestCounter.Add(ctx, 1)
    // Do work...
}
```

## Logging

Structured logging with run context:

```go
import "go.klarlabs.de/agent/infrastructure/logging"

engine, _ := api.New(
    api.WithMiddleware(
        api.LoggingMiddleware(nil), // Uses default logger
    ),
)
```

### Log Output

```json
{"level":"info","timestamp":"2024-01-15T10:30:00Z","msg":"run started","run_id":"run-123","goal":"Process files"}
{"level":"info","timestamp":"2024-01-15T10:30:01Z","msg":"executing tool","run_id":"run-123","state":"explore","tool":"read_file"}
{"level":"info","timestamp":"2024-01-15T10:30:01Z","msg":"tool executed","run_id":"run-123","tool":"read_file","duration_ms":45,"cached":false}
{"level":"info","timestamp":"2024-01-15T10:30:02Z","msg":"run completed","run_id":"run-123","state":"done","duration_ms":1234}
```

### Log Levels

| Level | Use |
|-------|-----|
| `debug` | Detailed execution flow |
| `info` | Normal operations (runs, tool executions) |
| `warn` | Recoverable issues (retries, fallbacks) |
| `error` | Failures (tool errors, run failures) |

## OpenTelemetry Export

Configure exporters before creating tracers/meters:

### OTLP (Jaeger, Honeycomb, etc.)

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
)

func initTracing() func() {
    exporter, _ := otlptracegrpc.New(context.Background(),
        otlptracegrpc.WithEndpoint("localhost:4317"),
        otlptracegrpc.WithInsecure(),
    )

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("my-agent"),
        )),
    )

    otel.SetTracerProvider(tp)

    return func() { tp.Shutdown(context.Background()) }
}

func main() {
    cleanup := initTracing()
    defer cleanup()

    tracer := observability.NewOTelTracer("my-agent")
    // ...
}
```

### Prometheus Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/sdk/metric"
)

func initMetrics() {
    exporter, _ := prometheus.New()
    provider := metric.NewMeterProvider(metric.WithReader(exporter))
    otel.SetMeterProvider(provider)

    // Expose /metrics endpoint
    http.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(":9090", nil)
}
```

### Stdout (Development)

```go
import (
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    "go.opentelemetry.io/otel/sdk/trace"
)

func initDevTracing() {
    exporter, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
    tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
    otel.SetTracerProvider(tp)
}
```

## Integration Example

Complete observability setup:

```go
package main

import (
    "context"
    "os"

    api "go.klarlabs.de/agent/interfaces/api"
    "go.klarlabs.de/agent/infrastructure/observability"
    "go.klarlabs.de/agent/infrastructure/security/audit"
)

func main() {
    // Initialize OpenTelemetry
    cleanup := initTracing()
    defer cleanup()

    // Create observability components
    tracer := observability.NewOTelTracer("file-processor")
    meter := observability.NewOTelMeter("file-processor")
    auditLogger := audit.NewJSONLogger(os.Stdout)

    // Build engine with full observability
    engine, _ := api.New(
        api.WithPlanner(myPlanner),
        api.WithRegistry(myRegistry),
        api.WithMiddleware(
            observability.TracingMiddleware(tracer),
            observability.MetricsMiddleware(meter),
            audit.AuditMiddleware(auditLogger),
            api.LoggingMiddleware(nil),
        ),
    )

    // Run with context for trace propagation
    ctx := context.Background()
    run, err := engine.Run(ctx, "Process all data files")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Run completed: %s (duration: %s)\n", run.Status, run.Duration())
}
```

## Best Practices

1. **Use consistent service names** - Same name for tracer, meter, and resource
2. **Add context to spans** - Include relevant business attributes
3. **Sample appropriately** - Use sampling in production to reduce costs
4. **Correlate logs and traces** - Include trace IDs in log output
5. **Monitor metrics** - Set up alerts for error rates and latencies
6. **Export to observability platform** - Jaeger, Honeycomb, Datadog, etc.

## See Also

- [Example: Observability](../../example/05-observability/) - Complete working example
- [Example: Production](../../example/07-production/) - Production setup
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
