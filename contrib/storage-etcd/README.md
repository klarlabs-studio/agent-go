# etcd Storage Backend

This package provides an etcd-backed implementation of the `cache.Cache` interface from agent-go.

## Overview

etcd is a distributed, reliable key-value store for the most critical data of a distributed system. It provides strong consistency guarantees and is commonly used for configuration management, service discovery, and coordination.

## Features

- Full implementation of `cache.Cache` interface
- TTL support via etcd leases
- Key prefixing for namespace isolation
- Atomic prefix-based clear operations
- Context-aware error handling
- Production-ready error mapping

## Installation

```bash
go get github.com/felixgeelhaar/agent-go/contrib/storage-etcd
```

## Usage

### Basic Setup

```go
import (
    "context"
    "time"

    "github.com/felixgeelhaar/agent-go/contrib/storage-etcd"
    clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
    // Create etcd client
    client, err := clientv3.New(clientv3.Config{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Create cache
    cache := etcd.NewCache(client)
    ctx := context.Background()

    // Store a value
    err = cache.Set(ctx, "my-key", []byte("my-value"), cache.SetOptions{})

    // Retrieve a value
    value, found, err := cache.Get(ctx, "my-key")
    if found {
        fmt.Printf("Value: %s\n", value)
    }
}
```

### Custom Configuration

```go
// Create cache with custom key prefix
cache := etcd.NewCacheWithConfig(client, etcd.CacheConfig{
    KeyPrefix: "myapp/cache/",
})
```

### TTL Support

```go
import "github.com/felixgeelhaar/agent-go/domain/cache"

// Store with 5-minute TTL
err := cache.Set(ctx, "temporary-key", []byte("value"), cache.SetOptions{
    TTL: 5 * time.Minute,
})
```

### Operations

```go
// Check if key exists
exists, err := cache.Exists(ctx, "my-key")

// Delete a key
err = cache.Delete(ctx, "my-key")

// Clear all keys with prefix
err = cache.Clear(ctx)
```

## Error Handling

The implementation maps etcd errors to domain errors:

- `context.DeadlineExceeded` → `cache.ErrOperationTimeout`
- Other errors → `cache.ErrConnectionFailed`
- Empty key on Set → `cache.ErrInvalidKey`

Example:

```go
value, found, err := cache.Get(ctx, "key")
if err != nil {
    if errors.Is(err, cache.ErrOperationTimeout) {
        // Handle timeout
    } else if errors.Is(err, cache.ErrConnectionFailed) {
        // Handle connection failure
    }
}
```

## Implementation Details

### Key Prefixing

All cache keys are automatically prefixed with `agent/cache/` by default. This provides namespace isolation and enables efficient prefix-based operations like `Clear()`.

### TTL Implementation

TTL is implemented using etcd leases:
1. When `SetOptions.TTL > 0`, a lease is created with the specified duration
2. The key-value pair is stored with the lease ID
3. etcd automatically removes the key when the lease expires

### Atomic Operations

- `Clear()` uses etcd's prefix delete for atomic removal of all keys with the configured prefix
- `Exists()` uses `WithCountOnly()` for efficient existence checks without retrieving values

## Testing

The package includes comprehensive tests:

```bash
# Run unit tests (no etcd required)
go test -short ./...

# Run integration tests (requires etcd at localhost:2379)
go test ./...
```

Integration tests are automatically skipped if etcd is not available.

## Performance Considerations

- etcd is optimized for consistent reads and writes in distributed systems
- For high-throughput caching, consider Redis or in-memory caches
- Use etcd when you need strong consistency and distributed coordination
- The `Exists()` method uses count-only queries for efficiency

## Production Deployment

### Cluster Configuration

For production use, connect to an etcd cluster:

```go
client, err := clientv3.New(clientv3.Config{
    Endpoints: []string{
        "etcd-1:2379",
        "etcd-2:2379",
        "etcd-3:2379",
    },
    DialTimeout: 5 * time.Second,
})
```

### Context Timeouts

Always use contexts with timeouts to prevent indefinite blocking:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

err := cache.Set(ctx, key, value, opts)
```

### Monitoring

Monitor these metrics:
- etcd connection health
- Operation latencies
- Lease expiration rates
- Key space usage

## License

See the main agent-go repository for license information.
