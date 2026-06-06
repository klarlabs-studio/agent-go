# SQLite Storage for agent-go

This module provides SQLite-backed implementations of agent-go storage interfaces, offering lightweight, file-based storage suitable for development, testing, and single-node deployments.

## Features

- **Cache**: Tool result caching with TTL support
- **EventStore**: Event sourcing with atomic append operations
- **RunStore**: Persistent run state and history

## Installation

```bash
go get go.klarlabs.de/agent/contrib/storage-sqlite
```

## Usage

### Basic Setup

```go
package main

import (
    "context"
    "database/sql"
    "log"

    "go.klarlabs.de/agent/contrib/storage-sqlite"
    _ "modernc.org/sqlite"
)

func main() {
    // Open database connection
    db, err := sql.Open("sqlite", "agent.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    ctx := context.Background()

    // Create stores
    cache := sqlite.NewCache(db)
    eventStore := sqlite.NewEventStore(db)
    runStore := sqlite.NewRunStore(db)

    // Initialize schemas
    if err := cache.EnsureSchema(ctx); err != nil {
        log.Fatal(err)
    }
    if err := eventStore.EnsureSchema(ctx); err != nil {
        log.Fatal(err)
    }
    if err := runStore.EnsureSchema(ctx); err != nil {
        log.Fatal(err)
    }

    // Use stores...
}
```

### Cache Operations

```go
// Set with TTL
err := cache.Set(ctx, "key", []byte("value"), cache.SetOptions{
    TTL: 5 * time.Minute,
})

// Get
value, found, err := cache.Get(ctx, "key")
if found {
    // Use value
}

// Check existence
exists, err := cache.Exists(ctx, "key")

// Delete
err = cache.Delete(ctx, "key")

// Clear all
err = cache.Clear(ctx)
```

### Event Store Operations

```go
// Append events
events := []event.Event{
    {
        RunID:     "run-001",
        Type:      "tool.executed",
        Timestamp: time.Now(),
        Payload:   json.RawMessage(`{"tool":"read_file"}`),
    },
}
err := eventStore.Append(ctx, events...)

// Load all events for a run
events, err := eventStore.LoadEvents(ctx, "run-001")

// Load from specific sequence
events, err := eventStore.LoadEventsFrom(ctx, "run-001", 5)

// Subscribe to new events
ch, err := eventStore.Subscribe(ctx, "run-001")
for event := range ch {
    // Handle event
}
```

### Run Store Operations

```go
// Save a run
run := &agent.Run{
    ID:        "run-001",
    Goal:      "Process data",
    Status:    agent.RunStatusRunning,
    StartTime: time.Now(),
    Vars:      map[string]any{},
    Evidence:  []agent.Evidence{},
}
err := runStore.Save(ctx, run)

// Get a run
run, err := runStore.Get(ctx, "run-001")

// Update a run
run.Status = agent.RunStatusCompleted
err = runStore.Update(ctx, run)

// Delete a run
err = runStore.Delete(ctx, "run-001")

// List runs with filters
runs, err := runStore.List(ctx, run.ListFilter{
    Status:      []agent.RunStatus{agent.RunStatusCompleted},
    GoalPattern: "Process",
    Limit:       10,
    OrderBy:     run.OrderByStartTime,
    Descending:  true,
})

// Count runs
count, err := runStore.Count(ctx, run.ListFilter{
    Status: []agent.RunStatus{agent.RunStatusRunning},
})
```

## Database Schema

### Cache Table

```sql
CREATE TABLE cache_entries (
    key TEXT PRIMARY KEY,
    value BLOB NOT NULL,
    expires_at INTEGER  -- Unix timestamp in milliseconds, NULL = no expiry
);
```

### Events Table

```sql
CREATE TABLE events (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    type TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    payload BLOB,
    sequence INTEGER NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    UNIQUE(run_id, sequence)
);
CREATE INDEX idx_events_run_id ON events(run_id);
CREATE INDEX idx_events_run_seq ON events(run_id, sequence);
```

### Runs Table

```sql
CREATE TABLE runs (
    id TEXT PRIMARY KEY,
    goal TEXT NOT NULL,
    current_state TEXT NOT NULL,
    vars BLOB,
    evidence BLOB,
    status TEXT NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    result BLOB,
    error TEXT
);
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_runs_start_time ON runs(start_time);
```

## In-Memory Testing

For testing, use an in-memory database:

```go
db, err := sql.Open("sqlite", ":memory:")
```

## File-Based Storage

For persistent storage, specify a file path:

```go
db, err := sql.Open("sqlite", "agent.db")
```

## Features

### Cache
- TTL-based expiration with millisecond precision
- Automatic cleanup of expired entries on access
- BLOB storage for arbitrary data

### EventStore
- Atomic multi-event append with transactions
- Automatic sequence number assignment
- Event subscription with in-memory notification
- Thread-safe subscriber management

### RunStore
- Full CRUD operations for runs
- Rich filtering by status, state, time range, and goal pattern
- Pagination support with LIMIT/OFFSET
- Proper error handling with domain-specific errors

## Error Handling

The stores return domain-specific errors:

- `run.ErrRunNotFound` - Run does not exist
- `run.ErrRunExists` - Run already exists (duplicate ID)
- `run.ErrInvalidRunID` - Empty or invalid run ID
- `event.ErrInvalidEvent` - Malformed event

## Thread Safety

All stores are safe for concurrent use:
- Cache operations use database transactions
- EventStore uses mutex-protected subscriber map
- RunStore operations are atomic at the database level

## Performance Considerations

SQLite is suitable for:
- Single-node deployments
- Development and testing
- Low to moderate throughput (< 1000 ops/sec)
- File-based persistence without external dependencies

For high-throughput or distributed systems, consider PostgreSQL storage instead.

## License

This module is part of agent-go and shares the same license.
