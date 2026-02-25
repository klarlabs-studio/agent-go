// Package etcd provides etcd-backed implementations of agent-go storage interfaces.
//
// etcd is a distributed, reliable key-value store for the most critical data of a
// distributed system. It provides strong consistency guarantees and is commonly used
// for configuration management, service discovery, and coordination.
//
// # Usage
//
//	client, err := clientv3.New(clientv3.Config{
//		Endpoints:   []string{"localhost:2379"},
//		DialTimeout: 5 * time.Second,
//	})
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
//	cache := etcd.NewCache(client)
package etcd

import (
	"context"
	"errors"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/cache"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Cache is an etcd-backed implementation of cache.Cache.
// It stores cached values as key-value pairs in etcd with optional lease-based TTL.
type Cache struct {
	client    *clientv3.Client
	keyPrefix string
}

// CacheConfig holds configuration for the etcd cache.
type CacheConfig struct {
	// KeyPrefix is an optional prefix for all cache keys.
	KeyPrefix string
}

// NewCache creates a new etcd cache with the given client.
func NewCache(client *clientv3.Client) *Cache {
	return &Cache{
		client:    client,
		keyPrefix: "agent/cache/",
	}
}

// NewCacheWithConfig creates a new etcd cache with full configuration.
func NewCacheWithConfig(client *clientv3.Client, cfg CacheConfig) *Cache {
	prefix := cfg.KeyPrefix
	if prefix == "" {
		prefix = "agent/cache/"
	}
	return &Cache{
		client:    client,
		keyPrefix: prefix,
	}
}

// Get retrieves a cached value by key.
// Returns the value, whether it was found, and any error.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, wrapError(err)
	}

	resp, err := c.client.Get(ctx, c.prefixKey(key))
	if err != nil {
		return nil, false, wrapError(err)
	}

	if resp.Count == 0 {
		return nil, false, nil
	}

	return resp.Kvs[0].Value, true, nil
}

// Set stores a value with the given key and options.
// TTL is implemented using etcd leases.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	if err := ctx.Err(); err != nil {
		return wrapError(err)
	}

	if key == "" {
		return cache.ErrInvalidKey
	}

	fullKey := c.prefixKey(key)

	if opts.TTL > 0 {
		// Create lease for TTL
		lease, err := c.client.Grant(ctx, int64(opts.TTL.Seconds()))
		if err != nil {
			return wrapError(err)
		}

		// Put with lease
		_, err = c.client.Put(ctx, fullKey, string(value), clientv3.WithLease(lease.ID))
		return wrapError(err)
	}

	// Put without lease
	_, err := c.client.Put(ctx, fullKey, string(value))
	return wrapError(err)
}

// Delete removes a cached entry by key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return wrapError(err)
	}

	_, err := c.client.Delete(ctx, c.prefixKey(key))
	return wrapError(err)
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, wrapError(err)
	}

	resp, err := c.client.Get(ctx, c.prefixKey(key), clientv3.WithCountOnly())
	if err != nil {
		return false, wrapError(err)
	}

	return resp.Count > 0, nil
}

// Clear removes all entries from the cache with the configured prefix.
// Uses etcd's prefix delete for atomic removal.
func (c *Cache) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return wrapError(err)
	}

	_, err := c.client.Delete(ctx, c.keyPrefix, clientv3.WithPrefix())
	return wrapError(err)
}

// Close closes the underlying etcd client connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

// prefixKey returns the full key with prefix.
func (c *Cache) prefixKey(key string) string {
	return c.keyPrefix + key
}

// wrapError maps etcd errors to cache domain errors.
func wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", cache.ErrOperationTimeout, err)
	}

	return fmt.Errorf("%w: %w", cache.ErrConnectionFailed, err)
}

// Ensure interface is implemented.
var _ cache.Cache = (*Cache)(nil)
