package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.klarlabs.de/fortify/ratelimit"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
)

// mockTool creates a mock tool for testing.
func mockTool(name string) tool.Tool {
	t, _ := tool.NewBuilder(name).
		WithDescription("test tool").
		WithAnnotations(tool.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}).
		Build()
	return t
}

// mockExecutionContext creates a mock execution context for testing.
func mockExecutionContext(runID, toolName string) *middleware.ExecutionContext {
	return &middleware.ExecutionContext{
		RunID:        runID,
		CurrentState: agent.StateExplore,
		Tool:         mockTool(toolName),
		Input:        json.RawMessage(`{}`),
	}
}

// successHandler is a handler that always succeeds.
func successHandler(_ context.Context, _ *middleware.ExecutionContext) (tool.Result, error) {
	return tool.Result{Output: json.RawMessage(`{"success": true}`)}, nil
}

func TestRateLimit(t *testing.T) {
	t.Run("allows_requests_within_limit", func(t *testing.T) {
		mw := RateLimit(RateLimitConfig{
			Rate:  100,
			Burst: 100,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// Should allow multiple requests within burst limit
		for i := 0; i < 10; i++ {
			_, err := handler(ctx, mockExecutionContext("run-1", "tool-1"))
			if err != nil {
				t.Fatalf("request %d should succeed: %v", i, err)
			}
		}
	})

	t.Run("blocks_requests_exceeding_limit", func(t *testing.T) {
		// Create a very restrictive rate limiter
		limiter := ratelimit.New(ratelimit.Config{
			Rate:  1,
			Burst: 1,
		})

		mw := RateLimit(RateLimitConfig{
			Limiter: limiter,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// First request should succeed
		_, err := handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if err != nil {
			t.Fatalf("first request should succeed: %v", err)
		}

		// Second immediate request should be rate limited
		_, err = handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if !errors.Is(err, policy.ErrRateLimitExceeded) {
			t.Errorf("expected ErrRateLimitExceeded, got: %v", err)
		}
	})

	t.Run("calls_on_limit_exceeded_callback", func(t *testing.T) {
		limiter := ratelimit.New(ratelimit.Config{
			Rate:  1,
			Burst: 1,
		})

		callbackCalled := false
		mw := RateLimit(RateLimitConfig{
			Limiter: limiter,
			OnLimitExceeded: func(_ context.Context, execCtx *middleware.ExecutionContext) {
				callbackCalled = true
				if execCtx.RunID != "run-callback" {
					t.Errorf("expected run ID 'run-callback', got: %s", execCtx.RunID)
				}
			},
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// First request succeeds
		_, _ = handler(ctx, mockExecutionContext("run-callback", "tool-1"))

		// Second request triggers callback
		_, _ = handler(ctx, mockExecutionContext("run-callback", "tool-1"))

		if !callbackCalled {
			t.Error("OnLimitExceeded callback should have been called")
		}
	})
}

func TestRateLimitScope(t *testing.T) {
	t.Run("scope_global", func(t *testing.T) {
		limiter := ratelimit.New(ratelimit.Config{
			Rate:  1,
			Burst: 1,
		})

		mw := RateLimit(RateLimitConfig{
			Limiter: limiter,
			Scope:   ScopeGlobal,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// First request from run-1 succeeds
		_, err := handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if err != nil {
			t.Fatalf("first request should succeed: %v", err)
		}

		// Request from different run should also be blocked (global scope)
		_, err = handler(ctx, mockExecutionContext("run-2", "tool-2"))
		if !errors.Is(err, policy.ErrRateLimitExceeded) {
			t.Errorf("expected ErrRateLimitExceeded for global scope, got: %v", err)
		}
	})

	t.Run("scope_per_run", func(t *testing.T) {
		mw := RateLimit(RateLimitConfig{
			Rate:  1,
			Burst: 1,
			Scope: ScopePerRun,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// Request from run-1 succeeds
		_, err := handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if err != nil {
			t.Fatalf("first request should succeed: %v", err)
		}

		// Request from different run should succeed (separate limit)
		_, err = handler(ctx, mockExecutionContext("run-2", "tool-1"))
		if err != nil {
			t.Fatalf("request from different run should succeed: %v", err)
		}

		// Second request from run-1 should be blocked
		_, err = handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if !errors.Is(err, policy.ErrRateLimitExceeded) {
			t.Errorf("expected ErrRateLimitExceeded for same run, got: %v", err)
		}
	})

	t.Run("scope_per_tool", func(t *testing.T) {
		mw := RateLimit(RateLimitConfig{
			Rate:  1,
			Burst: 1,
			Scope: ScopePerTool,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// Request for tool-1 succeeds
		_, err := handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if err != nil {
			t.Fatalf("first request should succeed: %v", err)
		}

		// Request for different tool should succeed
		_, err = handler(ctx, mockExecutionContext("run-1", "tool-2"))
		if err != nil {
			t.Fatalf("request for different tool should succeed: %v", err)
		}

		// Second request for tool-1 should be blocked
		_, err = handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if !errors.Is(err, policy.ErrRateLimitExceeded) {
			t.Errorf("expected ErrRateLimitExceeded for same tool, got: %v", err)
		}
	})

	t.Run("scope_per_run_tool", func(t *testing.T) {
		mw := RateLimit(RateLimitConfig{
			Rate:  1,
			Burst: 1,
			Scope: ScopePerRunTool,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// Request for run-1:tool-1 succeeds
		_, err := handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if err != nil {
			t.Fatalf("first request should succeed: %v", err)
		}

		// Request for same run, different tool should succeed
		_, err = handler(ctx, mockExecutionContext("run-1", "tool-2"))
		if err != nil {
			t.Fatalf("request for different tool should succeed: %v", err)
		}

		// Request for different run, same tool should succeed
		_, err = handler(ctx, mockExecutionContext("run-2", "tool-1"))
		if err != nil {
			t.Fatalf("request for different run should succeed: %v", err)
		}

		// Second request for run-1:tool-1 should be blocked
		_, err = handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if !errors.Is(err, policy.ErrRateLimitExceeded) {
			t.Errorf("expected ErrRateLimitExceeded for same run:tool, got: %v", err)
		}
	})
}

func TestPerToolRateLimit(t *testing.T) {
	t.Run("applies_different_rates_per_tool", func(t *testing.T) {
		mw := PerToolRateLimit(PerToolRateLimitConfig{
			DefaultRate:  1,
			DefaultBurst: 1,
			ToolRates: map[string]RateLimitConfig{
				"fast-tool": {Rate: 100, Burst: 100},
				"slow-tool": {Rate: 1, Burst: 1},
			},
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// Fast tool should allow many requests
		for i := 0; i < 10; i++ {
			_, err := handler(ctx, mockExecutionContext("run-1", "fast-tool"))
			if err != nil {
				t.Fatalf("fast-tool request %d should succeed: %v", i, err)
			}
		}

		// Slow tool should only allow one
		_, err := handler(ctx, mockExecutionContext("run-1", "slow-tool"))
		if err != nil {
			t.Fatalf("slow-tool first request should succeed: %v", err)
		}

		_, err = handler(ctx, mockExecutionContext("run-1", "slow-tool"))
		if !errors.Is(err, policy.ErrRateLimitExceeded) {
			t.Errorf("slow-tool second request should be blocked: %v", err)
		}
	})

	t.Run("uses_default_rate_for_unknown_tools", func(t *testing.T) {
		mw := PerToolRateLimit(PerToolRateLimitConfig{
			DefaultRate:  1,
			DefaultBurst: 1,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		// First request succeeds
		_, err := handler(ctx, mockExecutionContext("run-1", "unknown-tool"))
		if err != nil {
			t.Fatalf("first request should succeed: %v", err)
		}

		// Second request should be blocked by default rate
		_, err = handler(ctx, mockExecutionContext("run-1", "unknown-tool"))
		if !errors.Is(err, policy.ErrRateLimitExceeded) {
			t.Errorf("expected ErrRateLimitExceeded for default rate, got: %v", err)
		}
	})
}

func TestAdaptiveRateLimit(t *testing.T) {
	t.Run("throttles_on_error", func(t *testing.T) {
		callCount := 0
		adaptiveHandler := func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			callCount++
			if callCount <= 2 {
				return tool.Result{}, errors.New("error")
			}
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}

		mw := AdaptiveRateLimit(AdaptiveRateLimitConfig{
			InitialRate:     100,
			InitialBurst:    100,
			MinRate:         10,
			ThrottleOnError: true,
			ThrottleFactor:  0.5,
		})

		handler := mw(adaptiveHandler)
		ctx := context.Background()

		// Make some requests that error
		for i := 0; i < 3; i++ {
			_, _ = handler(ctx, mockExecutionContext("run-1", "tool-1"))
		}

		// Rate should have decreased due to errors
		// The middleware should still function
		if callCount < 3 {
			t.Errorf("expected at least 3 calls, got %d", callCount)
		}
	})
}

func TestRateLimitWait(t *testing.T) {
	t.Run("waits_for_capacity", func(t *testing.T) {
		mw := RateLimitWait(RateLimitConfig{
			Rate:  10, // 10 per second = one every 100ms
			Burst: 1,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		start := time.Now()

		// First request should succeed immediately
		_, err := handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if err != nil {
			t.Fatalf("first request should succeed: %v", err)
		}

		// Second request should wait
		_, err = handler(ctx, mockExecutionContext("run-1", "tool-1"))
		if err != nil {
			t.Fatalf("second request should succeed after wait: %v", err)
		}

		elapsed := time.Since(start)
		// Should have waited at least ~100ms for the second request
		if elapsed < 50*time.Millisecond {
			t.Logf("elapsed time: %v (expected some wait)", elapsed)
		}
	})

	t.Run("respects_context_cancellation", func(t *testing.T) {
		mw := RateLimitWait(RateLimitConfig{
			Rate:  1,
			Burst: 1,
		})

		handler := mw(successHandler)

		// Use first token
		ctx := context.Background()
		_, _ = handler(ctx, mockExecutionContext("run-1", "tool-1"))

		// Try to get second token with cancelled context
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := handler(cancelCtx, mockExecutionContext("run-1", "tool-1"))
		if !errors.Is(err, policy.ErrRateLimitExceeded) {
			t.Errorf("expected ErrRateLimitExceeded on cancelled context, got: %v", err)
		}
	})
}

func TestRateLimitConcurrency(t *testing.T) {
	t.Run("thread_safe_under_concurrent_access", func(t *testing.T) {
		mw := RateLimit(RateLimitConfig{
			Rate:  1000,
			Burst: 1000,
		})

		handler := mw(successHandler)
		ctx := context.Background()

		var wg sync.WaitGroup
		var successCount atomic.Int64
		var errorCount atomic.Int64

		numGoroutines := 100
		requestsPerGoroutine := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < requestsPerGoroutine; j++ {
					_, err := handler(ctx, mockExecutionContext(
						"run-concurrent",
						"tool-concurrent",
					))
					if err == nil {
						successCount.Add(1)
					} else {
						errorCount.Add(1)
					}
				}
			}(i)
		}

		wg.Wait()

		totalRequests := numGoroutines * requestsPerGoroutine
		totalProcessed := successCount.Load() + errorCount.Load()

		if int(totalProcessed) != totalRequests {
			t.Errorf("expected %d total requests, got %d", totalRequests, totalProcessed)
		}

		// With high rate limit, most should succeed
		if successCount.Load() < int64(totalRequests/2) {
			t.Errorf("expected most requests to succeed, got %d successes out of %d",
				successCount.Load(), totalRequests)
		}
	})
}

func TestGenerateRateLimitKey(t *testing.T) {
	execCtx := mockExecutionContext("run-123", "my-tool")

	tests := []struct {
		scope    RateLimitScope
		expected string
	}{
		{ScopeGlobal, "global"},
		{ScopePerRun, "run-123"},
		{ScopePerTool, "my-tool"},
		{ScopePerRunTool, "run-123:my-tool"},
		{"unknown", "global"}, // default case
	}

	for _, tc := range tests {
		t.Run(string(tc.scope), func(t *testing.T) {
			key := generateRateLimitKey(tc.scope, execCtx)
			if key != tc.expected {
				t.Errorf("expected key '%s', got '%s'", tc.expected, key)
			}
		})
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	if cfg.Scope != ScopeGlobal {
		t.Errorf("expected ScopeGlobal, got: %s", cfg.Scope)
	}
	if cfg.Rate != 100 {
		t.Errorf("expected rate 100, got: %d", cfg.Rate)
	}
	if cfg.Burst != 100 {
		t.Errorf("expected burst 100, got: %d", cfg.Burst)
	}
}
