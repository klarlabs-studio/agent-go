package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// HealthStatus represents the overall health state of a Redis connection.
type HealthStatus string

const (
	// HealthStatusUp indicates the Redis connection is healthy.
	HealthStatusUp HealthStatus = "up"
	// HealthStatusDown indicates the Redis connection is unavailable.
	HealthStatusDown HealthStatus = "down"
	// HealthStatusDegraded indicates the connection works but with elevated latency.
	HealthStatusDegraded HealthStatus = "degraded"
)

// HealthInfo holds detailed health information about a Redis connection.
type HealthInfo struct {
	// Status is the overall health status.
	Status HealthStatus `json:"status"`
	// Latency is the round-trip time of the last health check PING.
	Latency time.Duration `json:"latency"`
	// Connected indicates whether the PING succeeded.
	Connected bool `json:"connected"`
	// MemoryUsedBytes is the used_memory value from Redis INFO, if available.
	MemoryUsedBytes int64 `json:"memory_used_bytes,omitempty"`
	// MemoryMaxBytes is the maxmemory value from Redis INFO, if available.
	MemoryMaxBytes int64 `json:"memory_max_bytes,omitempty"`
	// ConnectedClients is the number of connected clients, if available.
	ConnectedClients int64 `json:"connected_clients,omitempty"`
	// UptimeSeconds is the Redis server uptime, if available.
	UptimeSeconds int64 `json:"uptime_seconds,omitempty"`
	// LastCheckedAt is the timestamp of the last health check.
	LastCheckedAt time.Time `json:"last_checked_at"`
	// Error holds the error message if the check failed.
	Error string `json:"error,omitempty"`
}

// HealthCheckConfig configures periodic health monitoring.
type HealthCheckConfig struct {
	// Interval is the time between health checks. Defaults to 30 seconds.
	Interval time.Duration
	// Timeout is the maximum time allowed for a single health check. Defaults to 5 seconds.
	Timeout time.Duration
	// DegradedLatencyThreshold is the latency above which the status is
	// reported as degraded instead of up. Defaults to 100ms.
	DegradedLatencyThreshold time.Duration
}

// DefaultHealthCheckConfig returns a HealthCheckConfig with sensible defaults.
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Interval:                 30 * time.Second,
		Timeout:                  5 * time.Second,
		DegradedLatencyThreshold: 100 * time.Millisecond,
	}
}

// HealthChecker performs health checks against a Redis connection.
type HealthChecker struct {
	cmd    goredis.Cmdable
	config HealthCheckConfig

	mu     sync.RWMutex
	last   HealthInfo
	stopCh chan struct{}
}

// NewHealthChecker creates a HealthChecker for the given Cache.
func NewHealthChecker(c *Cache, cfg HealthCheckConfig) *HealthChecker {
	return newHealthChecker(c.client, cfg)
}

// NewClusterHealthChecker creates a HealthChecker for the given ClusterCache.
func NewClusterHealthChecker(cc *ClusterCache, cfg HealthCheckConfig) *HealthChecker {
	return newHealthChecker(cc.client, cfg)
}

func newHealthChecker(cmd goredis.Cmdable, cfg HealthCheckConfig) *HealthChecker {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.DegradedLatencyThreshold <= 0 {
		cfg.DegradedLatencyThreshold = 100 * time.Millisecond
	}
	return &HealthChecker{
		cmd:    cmd,
		config: cfg,
	}
}

// Check performs a single health check and returns the result.
func (hc *HealthChecker) Check(ctx context.Context) HealthInfo {
	checkCtx, cancel := context.WithTimeout(ctx, hc.config.Timeout)
	defer cancel()

	info := HealthInfo{
		LastCheckedAt: time.Now(),
	}

	// Measure PING latency.
	start := time.Now()
	err := hc.cmd.Ping(checkCtx).Err()
	info.Latency = time.Since(start)

	if err != nil {
		info.Status = HealthStatusDown
		info.Connected = false
		info.Error = err.Error()
		hc.updateLast(info)
		return info
	}

	info.Connected = true

	// Gather server info if available.
	hc.populateServerInfo(checkCtx, &info)

	// Determine status based on latency.
	if info.Latency > hc.config.DegradedLatencyThreshold {
		info.Status = HealthStatusDegraded
	} else {
		info.Status = HealthStatusUp
	}

	hc.updateLast(info)
	return info
}

// LastStatus returns the most recent health check result.
func (hc *HealthChecker) LastStatus() HealthInfo {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.last
}

// StartMonitoring begins periodic health checks in the background.
// It stops when the context is cancelled or Stop is called.
func (hc *HealthChecker) StartMonitoring(ctx context.Context) {
	hc.mu.Lock()
	if hc.stopCh != nil {
		hc.mu.Unlock()
		return
	}
	hc.stopCh = make(chan struct{})
	stopCh := hc.stopCh
	hc.mu.Unlock()

	// Perform an initial check immediately.
	hc.Check(ctx)

	go func() {
		ticker := time.NewTicker(hc.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				hc.Check(ctx)
			}
		}
	}()
}

// Stop stops periodic health monitoring.
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if hc.stopCh != nil {
		close(hc.stopCh)
		hc.stopCh = nil
	}
}

func (hc *HealthChecker) updateLast(info HealthInfo) {
	hc.mu.Lock()
	hc.last = info
	hc.mu.Unlock()
}

// populateServerInfo extracts memory and connection info from Redis INFO.
func (hc *HealthChecker) populateServerInfo(ctx context.Context, info *HealthInfo) {
	// INFO returns a bulk string; parse key fields.
	result, err := hc.cmd.Info(ctx, "memory", "clients", "server").Result()
	if err != nil {
		return
	}

	info.MemoryUsedBytes = parseInfoInt(result, "used_memory")
	info.MemoryMaxBytes = parseInfoInt(result, "maxmemory")
	info.ConnectedClients = parseInfoInt(result, "connected_clients")
	info.UptimeSeconds = parseInfoInt(result, "uptime_in_seconds")
}

// parseInfoInt extracts an integer value from Redis INFO output by field name.
func parseInfoInt(info, field string) int64 {
	prefix := fmt.Sprintf("%s:", field)
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			valStr := strings.TrimPrefix(line, prefix)
			valStr = strings.TrimSpace(valStr)
			v, err := strconv.ParseInt(valStr, 10, 64)
			if err != nil {
				return 0
			}
			return v
		}
	}
	return 0
}
