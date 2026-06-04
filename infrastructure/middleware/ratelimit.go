package middleware

import (
	"context"
	"fmt"
	"sync"

	"github.com/felixgeelhaar/fortify/ratelimit"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/logging"
)

// RateLimitScope defines the scope for rate limiting.
type RateLimitScope string

const (
	// ScopeGlobal applies rate limiting across all runs and tools.
	ScopeGlobal RateLimitScope = "global"
	// ScopePerRun applies rate limiting per individual run.
	ScopePerRun RateLimitScope = "per_run"
	// ScopePerTool applies rate limiting per tool type.
	ScopePerTool RateLimitScope = "per_tool"
	// ScopePerRunTool applies rate limiting per run and tool combination.
	ScopePerRunTool RateLimitScope = "per_run_tool"
	// ScopePerUser applies rate limiting per user (extracted from Vars["user_id"]).
	ScopePerUser RateLimitScope = "per_user"
	// ScopePerTenant applies rate limiting per tenant (extracted from Vars["tenant_id"]).
	ScopePerTenant RateLimitScope = "per_tenant"
	// ScopePerUserTool applies rate limiting per user and tool combination.
	ScopePerUserTool RateLimitScope = "per_user_tool"
)

// RateLimitConfig configures the rate limiting middleware.
type RateLimitConfig struct {
	// Limiter is the rate limiter to use.
	// If nil, a default limiter will be created with the specified options.
	Limiter ratelimit.RateLimiter

	// Scope determines how rate limiting keys are generated.
	// Default is ScopeGlobal.
	Scope RateLimitScope

	// Rate is the number of tokens added per interval.
	// Only used if Limiter is nil.
	Rate int

	// Burst is the maximum number of tokens (bucket capacity).
	// Only used if Limiter is nil.
	Burst int

	// FailOpen determines behavior when the rate limiter fails.
	// If true, allows requests when the rate limiter is unavailable.
	// If false (default), denies requests when the rate limiter fails.
	FailOpen bool

	// OnLimitExceeded is called when a request is rate limited.
	// Receives the execution context that was limited.
	OnLimitExceeded func(ctx context.Context, execCtx *middleware.ExecutionContext)

	// UserKey is the Vars key used to extract the user ID for ScopePerUser/ScopePerUserTool.
	// Defaults to "user_id" if empty.
	UserKey string

	// TenantKey is the Vars key used to extract the tenant ID for ScopePerTenant.
	// Defaults to "tenant_id" if empty.
	TenantKey string
}

// DefaultRateLimitConfig returns a sensible default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Scope: ScopeGlobal,
		Rate:  100,
		Burst: 100,
	}
}

// RateLimit returns middleware that enforces rate limits on tool executions.
// It uses fortify's token bucket rate limiter to control request rates.
func RateLimit(cfg RateLimitConfig) middleware.Middleware {
	// Create limiter if not provided
	limiter := cfg.Limiter
	if limiter == nil {
		rate := cfg.Rate
		if rate <= 0 {
			rate = 100
		}
		burst := cfg.Burst
		if burst <= 0 {
			burst = rate
		}
		limiter = ratelimit.New(ratelimit.Config{
			Rate:     rate,
			Burst:    burst,
			FailOpen: cfg.FailOpen,
		})
	}

	scope := cfg.Scope
	if scope == "" {
		scope = ScopeGlobal
	}

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Generate key based on scope
			key := generateRateLimitKeyWithConfig(scope, execCtx, cfg.UserKey, cfg.TenantKey)

			// Check rate limit
			if !limiter.Allow(ctx, key) {
				// Log rate limit event
				logging.Warn().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.ToolName(execCtx.Tool.Name())).
					Add(logging.Str("scope", string(scope))).
					Add(logging.Str("key", key)).
					Msg("rate limit exceeded")

				// Call callback if provided
				if cfg.OnLimitExceeded != nil {
					cfg.OnLimitExceeded(ctx, execCtx)
				}

				return tool.Result{}, policy.ErrRateLimitExceeded
			}

			return next(ctx, execCtx)
		}
	}
}

// generateRateLimitKey generates a rate limiting key based on scope.
func generateRateLimitKey(scope RateLimitScope, execCtx *middleware.ExecutionContext) string {
	return generateRateLimitKeyWithConfig(scope, execCtx, "", "")
}

// generateRateLimitKeyWithConfig generates a rate limiting key with configurable Vars keys.
func generateRateLimitKeyWithConfig(scope RateLimitScope, execCtx *middleware.ExecutionContext, userKey, tenantKey string) string {
	switch scope {
	case ScopePerRun:
		return execCtx.RunID
	case ScopePerTool:
		return execCtx.Tool.Name()
	case ScopePerRunTool:
		return fmt.Sprintf("%s:%s", execCtx.RunID, execCtx.Tool.Name())
	case ScopePerUser:
		return fmt.Sprintf("user:%s", extractVarKey(execCtx, userKey, "user_id"))
	case ScopePerTenant:
		return fmt.Sprintf("tenant:%s", extractVarKey(execCtx, tenantKey, "tenant_id"))
	case ScopePerUserTool:
		return fmt.Sprintf("user:%s:tool:%s", extractVarKey(execCtx, userKey, "user_id"), execCtx.Tool.Name())
	default:
		return "global"
	}
}

// extractVarKey extracts a value from Vars, using the configured key or a default.
func extractVarKey(execCtx *middleware.ExecutionContext, key, defaultKey string) string {
	if key == "" {
		key = defaultKey
	}
	if v, ok := execCtx.Vars[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return "unknown"
}

// PerToolRateLimitConfig configures per-tool rate limiting.
type PerToolRateLimitConfig struct {
	// DefaultRate is the default rate for tools without specific configuration.
	DefaultRate int
	// DefaultBurst is the default burst for tools without specific configuration.
	DefaultBurst int
	// ToolRates maps tool names to their specific rate configurations.
	ToolRates map[string]RateLimitConfig
	// FailOpen determines behavior when rate limiting fails.
	FailOpen bool
	// OnLimitExceeded is called when a request is rate limited.
	OnLimitExceeded func(ctx context.Context, execCtx *middleware.ExecutionContext)
}

// PerToolRateLimit returns middleware that enforces different rate limits per tool.
// This allows fine-grained control over tool invocation rates.
func PerToolRateLimit(cfg PerToolRateLimitConfig) middleware.Middleware {
	// Create limiters for each tool
	limiters := make(map[string]ratelimit.RateLimiter)
	var limitersMu sync.RWMutex

	defaultRate := cfg.DefaultRate
	if defaultRate <= 0 {
		defaultRate = 100
	}
	defaultBurst := cfg.DefaultBurst
	if defaultBurst <= 0 {
		defaultBurst = defaultRate
	}

	// Pre-create limiters for configured tools
	for toolName, toolCfg := range cfg.ToolRates {
		rate := toolCfg.Rate
		if rate <= 0 {
			rate = defaultRate
		}
		burst := toolCfg.Burst
		if burst <= 0 {
			burst = rate
		}
		limiters[toolName] = ratelimit.New(ratelimit.Config{
			Rate:     rate,
			Burst:    burst,
			FailOpen: cfg.FailOpen,
		})
	}

	// Create default limiter
	defaultLimiter := ratelimit.New(ratelimit.Config{
		Rate:     defaultRate,
		Burst:    defaultBurst,
		FailOpen: cfg.FailOpen,
	})

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			toolName := execCtx.Tool.Name()

			// Get or create limiter for tool
			limitersMu.RLock()
			limiter, exists := limiters[toolName]
			limitersMu.RUnlock()

			if !exists {
				limiter = defaultLimiter
			}

			// Check rate limit using tool name as key
			if !limiter.Allow(ctx, toolName) {
				logging.Warn().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.ToolName(toolName)).
					Msg("per-tool rate limit exceeded")

				if cfg.OnLimitExceeded != nil {
					cfg.OnLimitExceeded(ctx, execCtx)
				}

				return tool.Result{}, policy.ErrRateLimitExceeded
			}

			return next(ctx, execCtx)
		}
	}
}

// AdaptiveRateLimitConfig configures adaptive rate limiting.
type AdaptiveRateLimitConfig struct {
	// InitialRate is the starting rate limit.
	InitialRate int
	// InitialBurst is the starting burst limit.
	InitialBurst int
	// MinRate is the minimum rate after throttling.
	MinRate int
	// MaxRate is the maximum rate after increase.
	MaxRate int
	// ThrottleOnError reduces rate when errors occur.
	ThrottleOnError bool
	// ThrottleFactor is the multiplier applied on error (e.g., 0.5 halves the rate).
	ThrottleFactor float64
	// RecoveryFactor is the multiplier applied on success (e.g., 1.1 increases by 10%).
	RecoveryFactor float64
	// FailOpen determines behavior when rate limiting fails.
	FailOpen bool
}

// AdaptiveRateLimit returns middleware that adjusts rate limits based on response patterns.
// Rate decreases on errors and gradually recovers on success.
func AdaptiveRateLimit(cfg AdaptiveRateLimitConfig) middleware.Middleware {
	// Set defaults
	if cfg.InitialRate <= 0 {
		cfg.InitialRate = 100
	}
	if cfg.InitialBurst <= 0 {
		cfg.InitialBurst = cfg.InitialRate
	}
	if cfg.MinRate <= 0 {
		cfg.MinRate = 10
	}
	if cfg.MaxRate <= 0 {
		cfg.MaxRate = 1000
	}
	if cfg.ThrottleFactor <= 0 || cfg.ThrottleFactor >= 1 {
		cfg.ThrottleFactor = 0.5
	}
	if cfg.RecoveryFactor <= 1 {
		cfg.RecoveryFactor = 1.1
	}

	var mu sync.Mutex
	currentRate := float64(cfg.InitialRate)

	limiter := ratelimit.New(ratelimit.Config{
		Rate:     cfg.InitialRate,
		Burst:    cfg.InitialBurst,
		FailOpen: cfg.FailOpen,
	})

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Check rate limit
			if !limiter.Allow(ctx, "adaptive") {
				logging.Warn().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.ToolName(execCtx.Tool.Name())).
					Add(logging.Float64("current_rate", currentRate)).
					Msg("adaptive rate limit exceeded")

				return tool.Result{}, policy.ErrRateLimitExceeded
			}

			// Execute the handler
			result, err := next(ctx, execCtx)

			// Adjust rate based on result
			if cfg.ThrottleOnError && err != nil {
				mu.Lock()
				currentRate = max(float64(cfg.MinRate), currentRate*cfg.ThrottleFactor)
				newRate := int(currentRate)
				mu.Unlock()

				// Recreate limiter with new rate
				limiter = ratelimit.New(ratelimit.Config{
					Rate:     newRate,
					Burst:    newRate,
					FailOpen: cfg.FailOpen,
				})

				logging.Debug().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.Int("new_rate", newRate)).
					Msg("rate decreased due to error")
			} else if err == nil {
				mu.Lock()
				currentRate = min(float64(cfg.MaxRate), currentRate*cfg.RecoveryFactor)
				mu.Unlock()
			}

			return result, err
		}
	}
}

// RateLimitWait returns middleware that waits for rate limit capacity instead of failing.
// This is useful when you want to slow down rather than reject requests.
func RateLimitWait(cfg RateLimitConfig) middleware.Middleware {
	// Create limiter if not provided
	limiter := cfg.Limiter
	if limiter == nil {
		rate := cfg.Rate
		if rate <= 0 {
			rate = 100
		}
		burst := cfg.Burst
		if burst <= 0 {
			burst = rate
		}
		limiter = ratelimit.New(ratelimit.Config{
			Rate:     rate,
			Burst:    burst,
			FailOpen: cfg.FailOpen,
		})
	}

	scope := cfg.Scope
	if scope == "" {
		scope = ScopeGlobal
	}

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			key := generateRateLimitKey(scope, execCtx)

			// Wait for rate limit capacity
			if err := limiter.Wait(ctx, key); err != nil {
				logging.Warn().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.ToolName(execCtx.Tool.Name())).
					Add(logging.ErrorField(err)).
					Msg("rate limit wait failed")

				return tool.Result{}, policy.ErrRateLimitExceeded
			}

			return next(ctx, execCtx)
		}
	}
}
