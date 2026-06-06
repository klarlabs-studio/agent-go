package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.klarlabs.de/agent/domain/cache"
)

// ClusterConfig holds Redis Cluster connection configuration.
type ClusterConfig struct {
	// Addrs is a list of cluster node addresses (host:port).
	Addrs []string

	// Password for authentication (optional).
	Password string

	// MaxRetries is the maximum number of retries before giving up.
	MaxRetries int

	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration

	// ReadTimeout is the timeout for socket reads.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for socket writes.
	WriteTimeout time.Duration

	// PoolSize is the maximum number of socket connections per node.
	PoolSize int

	// MinIdleConns is the minimum number of idle connections per node.
	MinIdleConns int

	// KeyPrefix is prepended to all keys (for namespacing).
	KeyPrefix string

	// RouteByLatency enables routing read commands to the closest node.
	RouteByLatency bool

	// RouteRandomly enables routing read commands to a random node.
	RouteRandomly bool

	// MaxRedirects is the maximum number of redirects to follow.
	// Defaults to 3 if zero.
	MaxRedirects int
}

// DefaultClusterConfig returns a ClusterConfig with sensible defaults.
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		Addrs:        []string{"localhost:7000", "localhost:7001", "localhost:7002"},
		MaxRetries:   3,
		MaxRedirects: 3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
		KeyPrefix:    "agent:",
	}
}

// ClusterConfigOption configures the Redis Cluster connection.
type ClusterConfigOption func(*ClusterConfig)

// WithClusterAddrs sets the cluster node addresses.
func WithClusterAddrs(addrs ...string) ClusterConfigOption {
	return func(c *ClusterConfig) {
		c.Addrs = addrs
	}
}

// WithClusterPassword sets the authentication password.
func WithClusterPassword(password string) ClusterConfigOption {
	return func(c *ClusterConfig) {
		c.Password = password
	}
}

// WithClusterKeyPrefix sets the key prefix for namespacing.
func WithClusterKeyPrefix(prefix string) ClusterConfigOption {
	return func(c *ClusterConfig) {
		c.KeyPrefix = prefix
	}
}

// WithRouteByLatency enables routing reads to the closest node.
func WithRouteByLatency(enabled bool) ClusterConfigOption {
	return func(c *ClusterConfig) {
		c.RouteByLatency = enabled
	}
}

// WithRouteRandomly enables routing reads to a random node.
func WithRouteRandomly(enabled bool) ClusterConfigOption {
	return func(c *ClusterConfig) {
		c.RouteRandomly = enabled
	}
}

// WithMaxRedirects sets the maximum number of cluster redirects.
func WithMaxRedirects(n int) ClusterConfigOption {
	return func(c *ClusterConfig) {
		c.MaxRedirects = n
	}
}

// ClusterCache is a Redis Cluster-backed implementation of cache.Cache.
type ClusterCache struct {
	cacheBase
	client *redis.ClusterClient
}

// NewClusterCache creates a new Redis Cluster cache with the given configuration.
func NewClusterCache(cfg ClusterConfig, opts ...ClusterConfigOption) (*ClusterCache, error) {
	for _, opt := range opts {
		opt(&cfg)
	}

	if len(cfg.Addrs) == 0 {
		return nil, errors.New("redis cluster: at least one address is required")
	}

	maxRedirects := cfg.MaxRedirects
	if maxRedirects == 0 {
		maxRedirects = 3
	}

	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:          cfg.Addrs,
		Password:       cfg.Password,
		MaxRetries:     cfg.MaxRetries,
		MaxRedirects:   maxRedirects,
		DialTimeout:    cfg.DialTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		PoolSize:       cfg.PoolSize,
		MinIdleConns:   cfg.MinIdleConns,
		RouteByLatency: cfg.RouteByLatency,
		RouteRandomly:  cfg.RouteRandomly,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Join(cache.ErrConnectionFailed, err)
	}

	cc := &ClusterCache{client: client}
	cc.cacheBase.cmd = client
	cc.cacheBase.keyPrefix = cfg.KeyPrefix
	cc.cacheBase.closeFn = client.Close
	cc.cacheBase.pingFn = func(ctx context.Context) error { return client.Ping(ctx).Err() }
	cc.cacheBase.clearFn = func(ctx context.Context, pattern string) error {
		return clusterScanAndDelete(ctx, client, pattern)
	}
	return cc, nil
}

// NewClusterCacheFromClient creates a cluster cache from an existing ClusterClient.
func NewClusterCacheFromClient(client *redis.ClusterClient, keyPrefix string) *ClusterCache {
	cc := &ClusterCache{client: client}
	cc.cacheBase.cmd = client
	cc.cacheBase.keyPrefix = keyPrefix
	cc.cacheBase.closeFn = client.Close
	cc.cacheBase.pingFn = func(ctx context.Context) error { return client.Ping(ctx).Err() }
	cc.cacheBase.clearFn = func(ctx context.Context, pattern string) error {
		return clusterScanAndDelete(ctx, client, pattern)
	}
	return cc
}

// ClusterClient returns the underlying Redis ClusterClient for advanced operations.
func (cc *ClusterCache) ClusterClient() *redis.ClusterClient {
	return cc.client
}

// Cmdable returns the redis.Cmdable interface for shared operations.
func (cc *ClusterCache) Cmdable() redis.Cmdable {
	return cc.client
}

// clusterScanAndDelete scans each cluster node and deletes matching keys.
func clusterScanAndDelete(ctx context.Context, client *redis.ClusterClient, pattern string) error {
	return client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
		return scanAndDelete(ctx, node, pattern)
	})
}

// Ensure ClusterCache implements cache.Cache and cache.StatsProvider.
var (
	_ cache.Cache         = (*ClusterCache)(nil)
	_ cache.StatsProvider = (*ClusterCache)(nil)
)
