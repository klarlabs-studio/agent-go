# storage-nats

NATS JetStream-backed event store for agent-go.

## Overview

This package provides a production-ready implementation of `event.Store` using NATS JetStream for durable, ordered event streams with real-time subscriptions.

**Key Features:**
- Durable event persistence with JetStream
- Ordered event delivery per run
- Real-time event subscriptions
- Horizontal scalability
- Optional file-based persistence for disaster recovery

## Installation

```bash
go get github.com/felixgeelhaar/agent-go/contrib/storage-nats
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/nats-io/nats.go"
    storagenats "github.com/felixgeelhaar/agent-go/contrib/storage-nats"
)

func main() {
    // Connect to NATS
    nc, err := nats.Connect("nats://localhost:4222")
    if err != nil {
        log.Fatal(err)
    }
    defer nc.Close()

    // Create JetStream context
    js, err := nc.JetStream()
    if err != nil {
        log.Fatal(err)
    }

    // Create or update stream
    _, err = js.AddStream(&nats.StreamConfig{
        Name:     "AGENT_EVENTS",
        Subjects: []string{"agent.events.>"},
        Storage:  nats.FileStorage, // Use FileStorage for persistence
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create event store
    store := storagenats.NewEventStore(js, "AGENT_EVENTS")

    // Use with agent-go engine
    // engine, err := api.New(
    //     api.WithEventStore(store),
    //     ...
    // )
}
```

## Configuration

### Basic Configuration

```go
store := storagenats.NewEventStore(js, "AGENT_EVENTS")
```

### Advanced Configuration

```go
store := storagenats.NewEventStoreWithConfig(js, storagenats.EventStoreConfig{
    StreamName:        "AGENT_EVENTS",
    SubjectPrefix:     "custom.prefix",        // Default: "agent.events"
    MaxMsgsPerSubject: 10000,                  // Retention limit per run
})
```

### Stream Configuration

```go
streamConfig := &nats.StreamConfig{
    Name:     "AGENT_EVENTS",
    Subjects: []string{"agent.events.>"},

    // Storage type
    Storage: nats.FileStorage, // or nats.MemoryStorage

    // Retention policy
    Retention: nats.LimitsPolicy,
    MaxMsgs:   1_000_000,
    MaxAge:    30 * 24 * time.Hour, // 30 days

    // Per-subject limits
    MaxMsgsPerSubject: 10_000,

    // Replication (for clustered NATS)
    Replicas: 3,
}

js.AddStream(streamConfig)
```

## Storage Types

### Memory Storage
- Fastest performance
- No disk I/O overhead
- Data lost on server restart
- Best for: Development, testing, ephemeral workloads

```go
Storage: nats.MemoryStorage
```

### File Storage
- Durable across restarts
- Slightly slower than memory
- Disk space required
- Best for: Production, audit trails, disaster recovery

```go
Storage: nats.FileStorage
```

## Usage Patterns

### Appending Events

```go
ctx := context.Background()

event1, _ := event.NewEvent("run-123", event.TypeRunStarted, payload)
event2, _ := event.NewEvent("run-123", event.TypeStateChanged, payload)

err := store.Append(ctx, event1, event2)
```

### Loading Events

```go
// Load all events for a run
events, err := store.LoadEvents(ctx, "run-123")

// Load from specific sequence
events, err := store.LoadEventsFrom(ctx, "run-123", 10)
```

### Real-time Subscriptions

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

// Subscribe to new events
ch, err := store.Subscribe(ctx, "run-123")
if err != nil {
    log.Fatal(err)
}

for event := range ch {
    log.Printf("Received event: %s (seq=%d)", event.Type, event.Sequence)
}
```

## Architecture

### Subject Structure

Events are published to subjects with the pattern:
```
{prefix}.{runID}
```

Default: `agent.events.{runID}`

This enables:
- Per-run event isolation
- Efficient subject-based filtering
- Horizontal scalability

### Sequence Numbers

The event store maintains per-run sequence numbers:
- Sequences start at 1 for each run
- Incremented atomically within the store
- Used for ordered replay and checkpointing

### Concurrency

The event store is fully thread-safe:
- Concurrent appends are serialized per run
- Concurrent loads are safe
- Multiple subscribers can coexist
- No external locking required

## Performance Characteristics

### Throughput
- **Append**: ~10,000-50,000 events/sec (memory storage)
- **Load**: ~20,000-100,000 events/sec (depends on batch size)
- **Subscribe**: Real-time delivery with <10ms latency

### Resource Usage
- **Memory**: ~1-2 KB per event (depends on payload size)
- **Disk**: Compressed storage with JetStream (file storage)
- **Network**: Minimal overhead with binary protocol

## Production Considerations

### Stream Configuration
```go
// Production-grade stream setup
streamConfig := &nats.StreamConfig{
    Name:     "AGENT_EVENTS",
    Subjects: []string{"agent.events.>"},
    Storage:  nats.FileStorage,

    // Retention: Keep 30 days or 10M events
    Retention:         nats.LimitsPolicy,
    MaxAge:            30 * 24 * time.Hour,
    MaxMsgs:           10_000_000,
    MaxMsgsPerSubject: 100_000,

    // Replication for high availability
    Replicas: 3,

    // Discard old messages when limits reached
    Discard: nats.DiscardOld,
}
```

### Connection Options
```go
nc, err := nats.Connect(
    "nats://localhost:4222",
    nats.Name("agent-go-event-store"),
    nats.MaxReconnects(-1),              // Unlimited reconnects
    nats.ReconnectWait(1*time.Second),
    nats.PingInterval(20*time.Second),
    nats.MaxPingsOutstanding(5),
    nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
        log.Printf("NATS error: %v", err)
    }),
    nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
        log.Printf("NATS disconnected: %v", err)
    }),
    nats.ReconnectHandler(func(_ *nats.Conn) {
        log.Println("NATS reconnected")
    }),
)
```

### Monitoring

```go
// Check stream status
info, err := js.StreamInfo("AGENT_EVENTS")
if err != nil {
    log.Fatal(err)
}

log.Printf("Messages: %d", info.State.Msgs)
log.Printf("Bytes: %d", info.State.Bytes)
log.Printf("Subjects: %d", info.State.NumSubjects)
log.Printf("Consumers: %d", info.State.Consumers)
```

## High Availability

### Clustered NATS
For production deployments, use NATS clustering:

```bash
# Start 3-node NATS cluster with JetStream
nats-server --cluster nats://node1:6222 --routes nats://node2:6222,nats://node3:6222 --jetstream
nats-server --cluster nats://node2:6222 --routes nats://node1:6222,nats://node3:6222 --jetstream
nats-server --cluster nats://node3:6222 --routes nats://node1:6222,nats://node2:6222 --jetstream
```

Connect to cluster:
```go
nc, err := nats.Connect(
    "nats://node1:4222,nats://node2:4222,nats://node3:4222",
    nats.MaxReconnects(-1),
)
```

### Stream Replication
Set `Replicas` to distribute data across nodes:
```go
streamConfig := &nats.StreamConfig{
    Name:     "AGENT_EVENTS",
    Subjects: []string{"agent.events.>"},
    Storage:  nats.FileStorage,
    Replicas: 3, // Replicate to 3 nodes
}
```

## Testing

Run tests with embedded NATS server:
```bash
cd contrib/storage-nats
go test -race -v ./...
```

Benchmark performance:
```bash
go test -bench=. -benchmem
```

## Comparison with Other Stores

| Feature | NATS | PostgreSQL | In-Memory |
|---------|------|------------|-----------|
| Durability | File/Memory | Transactional | None |
| Subscriptions | Real-time | Polling | In-process |
| Horizontal Scale | Excellent | Limited | N/A |
| Setup Complexity | Low | Medium | None |
| Operational Overhead | Low | Medium | None |

## Limitations

1. **Sequence tracking**: Per-run sequences are maintained in-memory. After restart, sequences resume from the last persisted event.
2. **No transactions**: Events are published individually (no multi-run atomicity).
3. **No pruning**: Old events must be managed via JetStream retention policies.

## Contributing

Contributions are welcome! Please ensure:
- All tests pass (`go test -race ./...`)
- Code follows Go conventions
- Changes are documented

## License

Same as agent-go parent project.
