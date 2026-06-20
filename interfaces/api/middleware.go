// Package api provides the public API for the agent runtime.
package api

import (
	"time"

	"go.klarlabs.de/agent/domain/cache"
	"go.klarlabs.de/agent/domain/ledger"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/logging"
	inframw "go.klarlabs.de/agent/infrastructure/middleware"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// Re-export middleware types for convenience.
type (
	// Middleware wraps a Handler with additional behavior.
	Middleware = middleware.Middleware

	// Handler executes a tool and returns its result.
	Handler = middleware.Handler

	// ExecutionContext contains all information needed for middleware decisions.
	ExecutionContext = middleware.ExecutionContext

	// BudgetView provides read-only access to budget state.
	BudgetView = middleware.BudgetView

	// MiddlewareRegistry manages an ordered list of middleware.
	MiddlewareRegistry = middleware.Registry

	// Cache is the interface for tool result caching.
	Cache = cache.Cache

	// LegacyMiddlewareCache provides in-memory caching for tool results.
	//
	// Deprecated: Use Cache interface with NewMemoryCache instead.
	LegacyMiddlewareCache = inframw.LegacyCache //nolint:staticcheck // intentional backward compatibility
)

// NewMiddlewareRegistry creates a new middleware registry.
func NewMiddlewareRegistry() *MiddlewareRegistry {
	return middleware.NewRegistry()
}

// NewMemoryCache creates a new in-memory cache with the specified maximum entries.
func NewMemoryCache(maxEntries int) *memory.Cache {
	return memory.NewCache(memory.WithMaxSize(maxEntries))
}

// NewLegacyMiddlewareCache creates a new cache with the specified maximum entries.
//
// Deprecated: Use NewMemoryCache instead.
func NewLegacyMiddlewareCache(maxEntries int) *LegacyMiddlewareCache {
	return inframw.NewLegacyCache(maxEntries)
}

// ChainMiddleware composes multiple middleware into a single middleware.
// Middleware are executed in the order provided, with each wrapping the next.
func ChainMiddleware(middlewares ...Middleware) Middleware {
	return middleware.Chain(middlewares...)
}

// NoopMiddleware returns a middleware that does nothing, just passes through.
func NoopMiddleware() Middleware {
	return middleware.Noop()
}

// EligibilityMiddleware returns middleware that enforces tool eligibility per state.
// It checks if the tool is allowed in the current state before execution.
func EligibilityMiddleware(eligibility *policy.ToolEligibility) Middleware {
	return inframw.Eligibility(inframw.EligibilityConfig{
		Eligibility: eligibility,
	})
}

// ApprovalMiddleware returns middleware that enforces human approval for risky tools.
// Tools marked with ShouldRequireApproval() must be approved before execution.
func ApprovalMiddleware(approver policy.Approver) Middleware {
	return inframw.Approval(inframw.ApprovalConfig{
		Approver: approver,
	})
}

// BudgetMiddleware returns middleware that enforces budget limits.
// It checks budget availability before execution and consumes on success.
func BudgetMiddleware(budget *policy.Budget, budgetName string, amount int) Middleware {
	return inframw.Budget(inframw.BudgetConfig{
		Budget:     budget,
		BudgetName: budgetName,
		Amount:     amount,
	})
}

// BudgetFromContextMiddleware returns middleware that uses the budget from ExecutionContext.
// This is useful when budget needs to be determined at runtime.
func BudgetFromContextMiddleware(budgetName string, amount int) Middleware {
	return inframw.BudgetFromContext(budgetName, amount)
}

// LoggingMiddlewareConfig configures the logging middleware.
type LoggingMiddlewareConfig struct {
	// LogInput logs the tool input (may contain sensitive data).
	LogInput bool
	// LogOutput logs the tool output (may be large).
	LogOutput bool
	// Logger is the injected structured logger. When nil, the middleware uses
	// a no-op logger and emits nothing — it never falls back to the
	// package-level logging singleton. Build one with NewLogger.
	Logger *logging.Logger
}

// LoggingMiddleware returns middleware that logs tool execution.
// Pass nil config for default settings (no input/output logging).
func LoggingMiddleware(cfg *LoggingMiddlewareConfig) Middleware {
	if cfg == nil {
		cfg = &LoggingMiddlewareConfig{}
	}
	return inframw.Logging(inframw.LoggingConfig{
		LogInput:  cfg.LogInput,
		LogOutput: cfg.LogOutput,
		Logger:    cfg.Logger,
	})
}

// CachingMiddleware returns middleware that caches cacheable tool results.
// Only tools marked as cacheable (via annotations) will be cached.
// Accepts any cache.Cache implementation (memory, Redis, etc).
func CachingMiddleware(c Cache, opts ...CacheOption) Middleware {
	// CacheOption is an alias of inframw.CacheOption, so opts pass through.
	return inframw.Caching(c, opts...)
}

// CacheOption configures the caching middleware.
type CacheOption = inframw.CacheOption

// WithCacheTTL sets the cache TTL for cached entries.
func WithCacheTTL(ttl time.Duration) CacheOption {
	return inframw.WithCacheTTL(ttl)
}

// LegacyCachingMiddleware returns middleware using the deprecated LegacyCache.
//
// Deprecated: Use CachingMiddleware with Cache interface instead.
func LegacyCachingMiddleware(legacyCache *LegacyMiddlewareCache) Middleware {
	return inframw.LegacyCaching(legacyCache)
}

// LedgerRecordingMiddleware returns middleware that records tool calls to the ledger.
// This provides an audit trail of all tool executions.
func LedgerRecordingMiddleware(l *ledger.Ledger) Middleware {
	return inframw.LedgerRecording(inframw.LedgerConfig{
		Ledger: l,
	})
}

// Re-export types for convenience.
type (
	// ToolResult is returned by tool execution.
	ToolResult = tool.Result
)
