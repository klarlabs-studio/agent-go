package redis

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/redis/go-redis/v9"
)

// cacheBase provides shared cache logic for both standalone and cluster caches.
type cacheBase struct {
	cmd       redis.Cmdable
	keyPrefix string
	hits      atomic.Int64
	misses    atomic.Int64
	closeFn   func() error
	pingFn    func(ctx context.Context) error
	clearFn   func(ctx context.Context, pattern string) error
}

// prefixKey adds the cache key prefix.
func (b *cacheBase) prefixKey(key string) string {
	return b.keyPrefix + "cache:" + key
}

// tagKey returns the Redis key for a tag set.
func (b *cacheBase) tagKey(tag string) string {
	return b.keyPrefix + "tag:" + tag
}

// Get retrieves a value from the cache.
func (b *cacheBase) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	result, err := b.cmd.Get(ctx, b.prefixKey(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			b.misses.Add(1)
			return nil, false, nil
		}
		return nil, false, wrapError(err)
	}

	b.hits.Add(1)
	return result, true, nil
}

// Set stores a value in the cache.
func (b *cacheBase) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if key == "" {
		return cache.ErrInvalidKey
	}

	var expiration time.Duration
	if opts.TTL > 0 {
		expiration = opts.TTL
	}

	err := b.cmd.Set(ctx, b.prefixKey(key), value, expiration).Err()
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// Delete removes a value from the cache.
func (b *cacheBase) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := b.cmd.Del(ctx, b.prefixKey(key)).Err()
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// Exists checks if a key exists in the cache.
func (b *cacheBase) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	result, err := b.cmd.Exists(ctx, b.prefixKey(key)).Result()
	if err != nil {
		return false, wrapError(err)
	}

	return result > 0, nil
}

// Clear removes all entries with the cache prefix.
func (b *cacheBase) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	pattern := b.keyPrefix + "cache:*"
	if b.clearFn != nil {
		return b.clearFn(ctx, pattern)
	}
	return nil
}

// Stats returns cache statistics.
func (b *cacheBase) Stats() cache.Stats {
	return cache.Stats{
		Hits:   b.hits.Load(),
		Misses: b.misses.Load(),
	}
}

// Close closes the underlying connection.
func (b *cacheBase) Close() error {
	if b.closeFn != nil {
		return b.closeFn()
	}
	return nil
}

// Ping checks the connection health.
func (b *cacheBase) Ping(ctx context.Context) error {
	if b.pingFn != nil {
		return b.pingFn(ctx)
	}
	return b.cmd.Ping(ctx).Err()
}

// Cache is a Redis-backed implementation of cache.Cache.
type Cache struct {
	cacheBase
	client *redis.Client
}

// NewCache creates a new Redis cache with the given configuration.
func NewCache(cfg Config, opts ...ConfigOption) (*Cache, error) {
	for _, opt := range opts {
		opt(&cfg)
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Join(cache.ErrConnectionFailed, err)
	}

	c := &Cache{client: client}
	c.cacheBase.cmd = client
	c.cacheBase.keyPrefix = cfg.KeyPrefix
	c.cacheBase.closeFn = client.Close
	c.cacheBase.pingFn = func(ctx context.Context) error { return client.Ping(ctx).Err() }
	c.cacheBase.clearFn = func(ctx context.Context, pattern string) error {
		return scanAndDelete(ctx, client, pattern)
	}
	return c, nil
}

// NewCacheFromClient creates a cache from an existing Redis client.
func NewCacheFromClient(client *redis.Client, keyPrefix string) *Cache {
	c := &Cache{client: client}
	c.cacheBase.cmd = client
	c.cacheBase.keyPrefix = keyPrefix
	c.cacheBase.closeFn = client.Close
	c.cacheBase.pingFn = func(ctx context.Context) error { return client.Ping(ctx).Err() }
	c.cacheBase.clearFn = func(ctx context.Context, pattern string) error {
		return scanAndDelete(ctx, client, pattern)
	}
	return c
}

// Client returns the underlying Redis client for advanced operations.
func (c *Cache) Client() *redis.Client {
	return c.client
}

// scanAndDelete scans for keys matching pattern and deletes them in batches.
func scanAndDelete(ctx context.Context, client *redis.Client, pattern string) error {
	iter := client.Scan(ctx, 0, pattern, 100).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				return wrapError(err)
			}
			keys = keys[:0]
		}
	}

	if err := iter.Err(); err != nil {
		return wrapError(err)
	}

	if len(keys) > 0 {
		if err := client.Del(ctx, keys...).Err(); err != nil {
			return wrapError(err)
		}
	}

	return nil
}

// wrapError wraps Redis errors with domain errors.
func wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(cache.ErrOperationTimeout, err)
	}

	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.Join(cache.ErrOperationTimeout, err)
	}

	return err
}

// Ensure Cache implements cache.Cache and cache.StatsProvider.
var (
	_ cache.Cache         = (*Cache)(nil)
	_ cache.StatsProvider = (*Cache)(nil)
)
