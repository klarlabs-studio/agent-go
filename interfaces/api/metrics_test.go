package api_test

import (
	"testing"

	api "github.com/felixgeelhaar/agent-go/interfaces/api"
)

func TestWithMetrics(t *testing.T) {
	t.Parallel()

	provider := &api.NoopMetricsProvider{}
	mockPlanner := api.NewMockPlanner(
		api.NewFinishDecision("done", nil),
	)

	engine, err := api.New(
		api.WithPlanner(mockPlanner),
		api.WithMetrics(provider),
	)
	if err != nil {
		t.Fatalf("New() with WithMetrics error = %v", err)
	}
	if engine == nil {
		t.Fatal("New() with WithMetrics returned nil engine")
	}
}

func TestNewCacheMetricsRecorder(t *testing.T) {
	t.Parallel()
	provider := &api.NoopMetricsProvider{}
	recorder := api.NewCacheMetricsRecorder(provider)
	_ = recorder
}

func TestNewRateLimitMetricsRecorder(t *testing.T) {
	t.Parallel()
	provider := &api.NoopMetricsProvider{}
	recorder := api.NewRateLimitMetricsRecorder(provider)
	_ = recorder
}

func TestNewCircuitBreakerMetricsRecorder(t *testing.T) {
	t.Parallel()
	provider := &api.NoopMetricsProvider{}
	recorder := api.NewCircuitBreakerMetricsRecorder(provider)
	_ = recorder
}
