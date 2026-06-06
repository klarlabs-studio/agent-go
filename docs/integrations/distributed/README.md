# Distributed Execution

agent-go supports distributed execution with multiple workers, queues, and distributed locks for horizontal scaling.

## Quick Start

```go
import (
    "go.klarlabs.de/agent/infrastructure/distributed"
    "go.klarlabs.de/agent/infrastructure/distributed/queue"
    "go.klarlabs.de/agent/infrastructure/distributed/lock"
)

// Create shared infrastructure
taskQueue := queue.NewMemoryQueue()
sharedStore := lock.NewMemoryLockStore()

// Create worker
worker := distributed.NewWorker(distributed.WorkerConfig{
    ID:       "worker-1",
    Queue:    taskQueue,
    Lock:     lock.NewMemoryLock(lock.WithStore(sharedStore)),
    Registry: myRegistry,
},
    distributed.WithConcurrency(4),
    distributed.WithPollInterval(100*time.Millisecond),
    distributed.WithTaskTimeout(30*time.Second),
)

// Start worker
go worker.Start(ctx)
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Task Queue                            │
│    (Redis, NATS, or Memory)                                 │
└─────────────────────────────────────────────────────────────┘
           │              │              │
           ▼              ▼              ▼
    ┌──────────┐   ┌──────────┐   ┌──────────┐
    │ Worker 1 │   │ Worker 2 │   │ Worker 3 │
    │ (4 slots)│   │ (4 slots)│   │ (4 slots)│
    └──────────┘   └──────────┘   └──────────┘
           │              │              │
           └──────────────┼──────────────┘
                          ▼
           ┌──────────────────────────────┐
           │     Distributed Lock Store    │
           │    (Redis or Memory)          │
           └──────────────────────────────┘
```

## Queue Implementations

### Memory Queue (Development/Testing)

```go
import "go.klarlabs.de/agent/infrastructure/distributed/queue"

taskQueue := queue.NewMemoryQueue()
```

**Characteristics:**
- Single-process only
- No persistence
- Good for testing and development

### Redis Queue (Production)

```go
import "go.klarlabs.de/agent/infrastructure/distributed/queue"

taskQueue := queue.NewRedisQueue(queue.RedisConfig{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
    Prefix:   "agent:",
})
```

**Characteristics:**
- Multi-process/multi-host
- Persistence available
- Priority queuing
- Dead letter queue support

### NATS Queue (High Throughput)

```go
taskQueue := queue.NewNATSQueue(queue.NATSConfig{
    URL:     "nats://localhost:4222",
    Subject: "agent.tasks",
})
```

## Task Types

Tasks represent units of work:

```go
type Task struct {
    ID        string          // Unique identifier
    RunID     string          // Parent run ID
    Type      TaskType        // Task type
    Payload   json.RawMessage // Task-specific data
    Priority  int             // Higher = processed first
    Metadata  map[string]string
    CreatedAt time.Time
    Status    TaskStatus
}
```

### Task Types

| Type | Description |
|------|-------------|
| `TaskTypeToolCall` | Execute a tool |
| `TaskTypePlanning` | Make planning decision |
| `TaskTypeValidation` | Validate results |

### Creating Tasks

```go
// Tool call task
task, err := queue.NewToolCallTask(
    "run-123",        // Run ID
    "read_file",      // Tool name
    input,            // JSON input
    "gathering info", // Reason
)

// Set priority (0 = lowest)
task.Priority = 5

// Add metadata
task.Metadata["user_id"] = "user-123"

// Enqueue
taskQueue.Enqueue(ctx, task)
```

## Distributed Locks

Prevent concurrent execution of critical operations.

### Memory Lock (Development)

```go
import "go.klarlabs.de/agent/infrastructure/distributed/lock"

// Shared store for all workers in process
sharedStore := lock.NewMemoryLockStore()

// Create lock for each worker
lock1 := lock.NewMemoryLock(
    lock.WithHolderID("worker-1"),
    lock.WithStore(sharedStore),
)
```

### Redis Lock (Production)

```go
distLock := lock.NewRedisLock(lock.RedisLockConfig{
    Addr:     "localhost:6379",
    HolderID: "worker-1",
    TTL:      30 * time.Second,
})
```

### Using Locks

```go
// Acquire lock
acquired, err := distLock.Acquire(ctx, "resource-key", 30*time.Second)
if !acquired {
    return errors.New("could not acquire lock")
}
defer distLock.Release(ctx, "resource-key")

// Do exclusive work...
```

## Worker Configuration

```go
worker := distributed.NewWorker(distributed.WorkerConfig{
    ID:       "worker-1",      // Unique worker ID
    Queue:    taskQueue,       // Task queue
    Lock:     distLock,        // Distributed lock
    Registry: toolRegistry,    // Tool registry
},
    // Options
    distributed.WithConcurrency(4),              // Parallel tasks
    distributed.WithPollInterval(100*time.Millisecond), // Queue polling
    distributed.WithTaskTimeout(30*time.Second), // Max task duration
    distributed.WithRetryLimit(3),               // Retry failed tasks
    distributed.WithRetryDelay(time.Second),     // Delay between retries
)
```

### Custom Task Handlers

Register handlers for different task types:

```go
worker.RegisterHandler(queue.TaskTypeToolCall, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
    var payload queue.ToolCallPayload
    json.Unmarshal(task.Payload, &payload)

    tool, _ := registry.Get(payload.ToolName)
    result, err := tool.Execute(ctx, payload.Input)
    if err != nil {
        return nil, err
    }
    return result.Output, nil
})
```

## Worker Lifecycle

```go
// Create worker
worker := distributed.NewWorker(config, options...)

// Start processing (blocks)
go func() {
    if err := worker.Start(ctx); err != nil {
        log.Printf("Worker error: %v", err)
    }
}()

// Graceful shutdown
worker.Stop()

// Get metrics
metrics := worker.Metrics()
fmt.Printf("Completed: %d, Failed: %d\n",
    metrics.TasksCompleted, metrics.TasksFailed)
```

## Worker Metrics

```go
type WorkerMetrics struct {
    TasksStarted   int64
    TasksCompleted int64
    TasksFailed    int64
    TasksRetried   int64
    TotalDuration  time.Duration
}

// Get average task duration
avg := metrics.AverageTaskDuration()
```

## Multi-Worker Setup

```go
const numWorkers = 5
var workers []*distributed.Worker

// Create shared infrastructure
taskQueue := queue.NewRedisQueue(redisConfig)
sharedStore := lock.NewRedisLockStore(redisConfig)

// Create workers
for i := 0; i < numWorkers; i++ {
    workerID := fmt.Sprintf("worker-%d", i+1)

    worker := distributed.NewWorker(distributed.WorkerConfig{
        ID:       workerID,
        Queue:    taskQueue,
        Lock:     lock.NewRedisLock(lock.RedisLockConfig{
            Store:    sharedStore,
            HolderID: workerID,
        }),
        Registry: registry,
    },
        distributed.WithConcurrency(4),
    )

    workers = append(workers, worker)
}

// Start all workers
ctx, cancel := context.WithCancel(context.Background())
var wg sync.WaitGroup

for _, w := range workers {
    wg.Add(1)
    go func(worker *distributed.Worker) {
        defer wg.Done()
        worker.Start(ctx)
    }(w)
}

// Graceful shutdown on signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
<-sigChan

cancel() // Stop all workers
wg.Wait()
```

## Task Submission

Submit tasks from any process:

```go
// Connect to queue
taskQueue := queue.NewRedisQueue(redisConfig)

// Submit task
task, _ := queue.NewToolCallTask(runID, "process_data", input, "batch processing")
task.Priority = 10 // High priority

err := taskQueue.Enqueue(ctx, task)
```

## Error Handling

### Task Failures

```go
worker.RegisterHandler(queue.TaskTypeToolCall, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
    result, err := executeTask(ctx, task)
    if err != nil {
        // Return error - task will be retried (up to retry limit)
        return nil, fmt.Errorf("task failed: %w", err)
    }
    return result, nil
})
```

### Dead Letter Queue

Failed tasks after all retries:

```go
// Get failed tasks
failedTasks, _ := taskQueue.DeadLetterQueue(ctx)

for _, task := range failedTasks {
    log.Printf("Task %s failed: %s", task.ID, task.Error)

    // Optionally retry
    task.Retries = 0
    taskQueue.Enqueue(ctx, task)
}
```

## Best Practices

1. **Use Redis in production** - Memory queue is single-process only
2. **Set appropriate concurrency** - Based on CPU/memory resources
3. **Configure task timeouts** - Prevent stuck tasks
4. **Monitor metrics** - Track completion rates and durations
5. **Handle graceful shutdown** - Allow in-flight tasks to complete
6. **Use unique worker IDs** - For debugging and lock ownership
7. **Implement dead letter handling** - Don't lose failed tasks

## See Also

- [Example: Distributed](../../example/06-distributed/) - Complete working example
- [Example: Production](../../example/07-production/) - Production setup
