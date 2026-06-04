package resilience

import (
	"context"
	"sync"

	"github.com/felixgeelhaar/fortify/ratelimit"
)

// RateLimiterConfig configures rate limiting for the executor pipeline.
type RateLimiterConfig struct {
	// GlobalRate is the token replenishment rate (tokens per second) applied
	// to all tool executions.  Zero disables the global limiter.
	GlobalRate int

	// GlobalBurst is the maximum burst size for the global limiter.
	// Defaults to GlobalRate when zero.
	GlobalBurst int

	// PerToolRates maps tool names to tool-specific rate/burst pairs.
	// Tools not listed here fall back to the global limiter.
	PerToolRates map[string]RateConfig

	// FailOpen determines behaviour when the limiter itself errors.
	// When true requests are allowed; when false they are denied.
	FailOpen bool
}

// RateConfig is a simple rate + burst pair.
type RateConfig struct {
	Rate  int
	Burst int
}

// rateLimiterSet holds the runtime limiters derived from RateLimiterConfig.
type rateLimiterSet struct {
	global   ratelimit.RateLimiter
	perTool  map[string]ratelimit.RateLimiter
	mu       sync.RWMutex
	failOpen bool
}

// newRateLimiterSet builds limiters from the config.  Returns nil when rate
// limiting is disabled (GlobalRate == 0 and no per-tool rates).
func newRateLimiterSet(cfg *RateLimiterConfig) *rateLimiterSet {
	if cfg == nil {
		return nil
	}
	if cfg.GlobalRate <= 0 && len(cfg.PerToolRates) == 0 {
		return nil
	}

	s := &rateLimiterSet{
		perTool:  make(map[string]ratelimit.RateLimiter, len(cfg.PerToolRates)),
		failOpen: cfg.FailOpen,
	}

	if cfg.GlobalRate > 0 {
		burst := cfg.GlobalBurst
		if burst <= 0 {
			burst = cfg.GlobalRate
		}
		s.global = ratelimit.New(ratelimit.Config{
			Rate:     cfg.GlobalRate,
			Burst:    burst,
			FailOpen: cfg.FailOpen,
		})
	}

	for name, rc := range cfg.PerToolRates {
		burst := rc.Burst
		if burst <= 0 {
			burst = rc.Rate
		}
		s.perTool[name] = ratelimit.New(ratelimit.Config{
			Rate:     rc.Rate,
			Burst:    burst,
			FailOpen: cfg.FailOpen,
		})
	}

	return s
}

// allow checks both the per-tool limiter (if configured) and the global limiter.
// Returns true when the request is permitted.
func (s *rateLimiterSet) allow(ctx context.Context, toolName string) bool {
	if s == nil {
		return true
	}

	// Check per-tool limiter first.
	s.mu.RLock()
	tl, hasPerTool := s.perTool[toolName]
	s.mu.RUnlock()

	if hasPerTool {
		if !tl.Allow(ctx, toolName) {
			return false
		}
	}

	// Check global limiter.
	if s.global != nil {
		if !s.global.Allow(ctx, "global") {
			return false
		}
	}

	return true
}

// --- Functional options ---

// WithRateLimiter configures rate limiting in the executor pipeline.
func WithRateLimiter(cfg RateLimiterConfig) Option {
	return func(c *ExecutorConfig) {
		c.RateLimiter = &cfg
	}
}

// WithGlobalRateLimit is a convenience option that sets only a global rate limit.
func WithGlobalRateLimit(rate, burst int) Option {
	return func(c *ExecutorConfig) {
		if c.RateLimiter == nil {
			c.RateLimiter = &RateLimiterConfig{}
		}
		c.RateLimiter.GlobalRate = rate
		c.RateLimiter.GlobalBurst = burst
	}
}
