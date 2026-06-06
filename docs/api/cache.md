# Package `cache`

**Import path:** `go.klarlabs.de/agent/domain/cache`

## Overview

package cache // import "go.klarlabs.de/agent/domain/cache"

Package cache provides the domain interface for tool result caching.

## Full API Reference

```
package cache // import "go.klarlabs.de/agent/domain/cache"

Package cache provides the domain interface for tool result caching.

VARIABLES

var (
	// ErrKeyNotFound is returned when a key does not exist in the cache.
	ErrKeyNotFound = errors.New("cache key not found")

	// ErrCacheFull is returned when the cache is at capacity and cannot accept new entries.
	ErrCacheFull = errors.New("cache is full")

	// ErrInvalidKey is returned when a key is invalid (e.g., empty).
	ErrInvalidKey = errors.New("invalid cache key")

	// ErrConnectionFailed is returned when connection to the cache backend fails.
	ErrConnectionFailed = errors.New("cache connection failed")

	// ErrOperationTimeout is returned when a cache operation times out.
	ErrOperationTimeout = errors.New("cache operation timeout")
)
    Domain errors for cache operations.


TYPES

type Cache interface {
	// Get retrieves a cached value by key.
	// Returns the value, whether it was found, and any error.
	Get(ctx context.Context, key string) ([]byte, bool, error)

	// Set stores a value with the given key and options.
	Set(ctx context.Context, key string, value []byte, opts SetOptions) error

	// Delete removes a cached entry by key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all entries from the cache.
	Clear(ctx context.Context) error
}
    Cache defines the interface for tool result caching. Implementations may be
    in-memory, Redis, or any other backend.

type SetOptions struct {
	// TTL is the time-to-live for the cached entry.
	// Zero means no expiration.
	TTL time.Duration
}
    SetOptions configures how a value is stored in the cache.

type Stats struct {
	// Hits is the number of cache hits.
	Hits int64
	// Misses is the number of cache misses.
	Misses int64
	// Size is the current number of entries.
	Size int64
	// MaxSize is the maximum number of entries (0 = unlimited).
	MaxSize int64
}
    Stats provides cache statistics.

type StatsProvider interface {
	// Stats returns current cache statistics.
	Stats() Stats
}
    StatsProvider is an optional interface for caches that support statistics.
```
