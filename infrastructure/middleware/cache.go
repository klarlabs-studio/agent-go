package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/cache"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// LegacyCache provides in-memory caching for tool results.
//
// Deprecated: Use cache.Cache implementations instead.
type LegacyCache struct {
	entries map[string]tool.Result
	mu      sync.RWMutex
	maxSize int
}

// NewLegacyCache creates a new legacy cache with the specified maximum entries.
//
// Deprecated: Use NewCache from infrastructure/storage/memory instead.
func NewLegacyCache(maxEntries int) *LegacyCache {
	return &LegacyCache{
		entries: make(map[string]tool.Result),
		maxSize: maxEntries,
	}
}

// Get retrieves a cached result by key.
func (c *LegacyCache) Get(key string) (tool.Result, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.entries[key]
	return result, ok
}

// Set stores a result in the cache.
func (c *LegacyCache) Set(key string, result tool.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) < c.maxSize {
		c.entries[key] = result
	}
}

// Clear removes all entries from the cache.
func (c *LegacyCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]tool.Result)
}

// Len returns the number of cached entries.
func (c *LegacyCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// CacheOptions configures caching behavior.
type CacheOptions struct {
	// TTL is the time-to-live for cached entries.
	TTL time.Duration
}

// CacheOption configures the caching middleware.
type CacheOption func(*CacheOptions)

// WithCacheTTL sets the cache TTL.
func WithCacheTTL(ttl time.Duration) CacheOption {
	return func(o *CacheOptions) {
		o.TTL = ttl
	}
}

// Caching returns middleware that caches cacheable tool results using the cache.Cache interface.
// This works with any cache implementation (memory, Redis, etc).
func Caching(c cache.Cache, opts ...CacheOption) middleware.Middleware {
	options := CacheOptions{
		TTL: 0, // No expiration by default
	}
	for _, opt := range opts {
		opt(&options)
	}

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if cache not provided
			if c == nil {
				return next(ctx, execCtx)
			}

			// Only cache if tool is cacheable
			if !execCtx.Tool.Annotations().CanCache() {
				return next(ctx, execCtx)
			}

			// Generate cache key
			key := cacheKey(execCtx.Tool.Name(), execCtx.Input)

			// Check cache
			data, ok, err := c.Get(ctx, key)
			if err == nil && ok {
				var result tool.Result
				if err := json.Unmarshal(data, &result); err == nil {
					result.Cached = true
					return result, nil
				}
			}

			// Execute
			result, err := next(ctx, execCtx)
			if err != nil {
				return result, err
			}

			// Store in cache
			data, err = json.Marshal(result)
			if err == nil {
				setOpts := cache.SetOptions{TTL: options.TTL}
				_ = c.Set(ctx, key, data, setOpts)
			}

			return result, nil
		}
	}
}

// LegacyCaching returns middleware using the deprecated LegacyCache.
//
// Deprecated: Use Caching with cache.Cache instead.
func LegacyCaching(legacyCache *LegacyCache) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Skip if cache not provided
			if legacyCache == nil {
				return next(ctx, execCtx)
			}

			// Only cache if tool is cacheable
			if !execCtx.Tool.Annotations().CanCache() {
				return next(ctx, execCtx)
			}

			// Generate cache key
			key := cacheKey(execCtx.Tool.Name(), execCtx.Input)

			// Check cache
			if result, ok := legacyCache.Get(key); ok {
				result.Cached = true
				return result, nil
			}

			// Execute
			result, err := next(ctx, execCtx)
			if err != nil {
				return result, err
			}

			// Store in cache
			legacyCache.Set(key, result)

			return result, nil
		}
	}
}

// cacheKey generates a unique key for a tool invocation.
func cacheKey(toolName string, input []byte) string {
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte(":"))
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))
}
