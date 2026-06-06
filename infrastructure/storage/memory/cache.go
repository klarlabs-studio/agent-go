package memory

import (
	"context"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/cache"
)

// cacheEntry holds a cached value with expiration.
type cacheEntry struct {
	value     []byte
	expiresAt time.Time
	accessAt  time.Time
}

// isExpired checks if the entry has expired.
func (e *cacheEntry) isExpired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.expiresAt)
}

// Cache is an in-memory implementation of cache.Cache.
// It supports TTL-based expiration and LRU eviction when at capacity.
type Cache struct {
	entries  map[string]*cacheEntry
	maxSize  int
	mu       sync.RWMutex
	hits     int64
	misses   int64
	cleanups int64
}

// CacheOption configures the cache.
type CacheOption func(*Cache)

// WithMaxSize sets the maximum number of entries.
func WithMaxSize(size int) CacheOption {
	return func(c *Cache) {
		c.maxSize = size
	}
}

// NewCache creates a new in-memory cache.
func NewCache(opts ...CacheOption) *Cache {
	c := &Cache{
		entries: make(map[string]*cacheEntry),
		maxSize: 1000, // Default max entries
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Get retrieves a value from the cache.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil, false, nil
	}

	if entry.isExpired() {
		delete(c.entries, key)
		c.misses++
		return nil, false, nil
	}

	entry.accessAt = time.Now()
	c.hits++

	// Return a copy to prevent mutation
	value := make([]byte, len(entry.value))
	copy(value, entry.value)
	return value, true, nil
}

// Set stores a value in the cache.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if key == "" {
		return cache.ErrInvalidKey
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity and key doesn't exist
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	// Still at capacity after eviction
	if len(c.entries) >= c.maxSize {
		return cache.ErrCacheFull
	}

	// Store a copy to prevent external mutation
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	entry := &cacheEntry{
		value:    valueCopy,
		accessAt: time.Now(),
	}

	if opts.TTL > 0 {
		entry.expiresAt = time.Now().Add(opts.TTL)
	}

	c.entries[key] = entry
	return nil
}

// Delete removes a value from the cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return false, nil
	}

	return !entry.isExpired(), nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	return nil
}

// Stats returns cache statistics.
func (c *Cache) Stats() cache.Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return cache.Stats{
		Hits:    c.hits,
		Misses:  c.misses,
		Size:    int64(len(c.entries)),
		MaxSize: int64(c.maxSize),
	}
}

// evictLRU removes the least recently accessed entry.
// Must be called with lock held.
func (c *Cache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		// Skip first iteration or if this entry is older
		if oldestKey == "" || entry.accessAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.accessAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// Cleanup removes expired entries.
func (c *Cache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var removed int
	for key, entry := range c.entries {
		if entry.isExpired() {
			delete(c.entries, key)
			removed++
		}
	}
	c.cleanups++
	return removed
}

// Size returns the current number of entries.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Ensure Cache implements cache.Cache and cache.StatsProvider
var (
	_ cache.Cache         = (*Cache)(nil)
	_ cache.StatsProvider = (*Cache)(nil)
)
