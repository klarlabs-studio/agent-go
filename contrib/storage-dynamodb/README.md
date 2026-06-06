# DynamoDB Storage Backend

This package provides AWS DynamoDB-backed implementations of agent-go storage interfaces:

- `cache.Cache` - Tool result caching with TTL support
- `run.Store` - Persistent agent run state and history

## Features

- **Fully managed**: Leverages AWS DynamoDB's scalability and reliability
- **TTL support**: Native DynamoDB TTL for cache expiration
- **Race-safe**: Thread-safe operations suitable for concurrent use
- **Production-ready**: Proper error handling, retries, and observability
- **Testable**: Mock-friendly interface for unit testing

## Installation

```bash
go get go.klarlabs.de/agent/contrib/storage-dynamodb
```

## Quick Start

### Prerequisites

1. AWS account with DynamoDB access
2. Two DynamoDB tables:
   - Cache table with partition key `pk` (String)
   - Run store table with partition key `id` (String)

### Table Creation

```bash
# Cache table
aws dynamodb create-table \
  --table-name agent-cache \
  --attribute-definitions AttributeName=pk,AttributeType=S \
  --key-schema AttributeName=pk,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# Enable TTL for cache expiration
aws dynamodb update-time-to-live \
  --table-name agent-cache \
  --time-to-live-specification "Enabled=true, AttributeName=ttl"

# Run store table
aws dynamodb create-table \
  --table-name agent-runs \
  --attribute-definitions AttributeName=id,AttributeType=S \
  --key-schema AttributeName=id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```

### Usage Example

```go
package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	storagedynamodb "go.klarlabs.de/agent/contrib/storage-dynamodb"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/cache"
)

func main() {
	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Initialize cache
	cacheStore := storagedynamodb.NewCache(client, "agent-cache")

	// Store a value with TTL
	err = cacheStore.Set(ctx, "my-key", []byte("my-value"), cache.SetOptions{
		TTL: 1 * time.Hour,
	})
	if err != nil {
		panic(err)
	}

	// Retrieve the value
	value, found, err := cacheStore.Get(ctx, "my-key")
	if err != nil {
		panic(err)
	}
	if found {
		println("Value:", string(value))
	}

	// Initialize run store
	runStore := storagedynamodb.NewRunStore(client, "agent-runs")

	// Create and save a run
	run := agent.NewRun("run-123", "Process data")
	run.Start()
	err = runStore.Save(ctx, run)
	if err != nil {
		panic(err)
	}

	// Retrieve the run
	retrieved, err := runStore.Get(ctx, "run-123")
	if err != nil {
		panic(err)
	}
	println("Retrieved run:", retrieved.ID)
}
```

## Cache Interface

### Methods

- `Get(ctx, key) ([]byte, bool, error)` - Retrieve cached value
- `Set(ctx, key, value, opts) error` - Store value with optional TTL
- `Delete(ctx, key) error` - Remove cached entry
- `Exists(ctx, key) (bool, error)` - Check if key exists
- `Clear(ctx) error` - Remove all entries

### TTL Behavior

Cache entries with TTL are automatically expired by DynamoDB:

```go
// Set with 1 hour TTL
err := cache.Set(ctx, "key", []byte("value"), cache.SetOptions{
    TTL: 1 * time.Hour,
})

// Set without TTL (never expires)
err := cache.Set(ctx, "key", []byte("value"), cache.SetOptions{})
```

DynamoDB typically deletes expired items within 48 hours. The Get and Exists methods check TTL client-side to ensure immediate expiration behavior.

## RunStore Interface

### Methods

- `Save(ctx, run) error` - Create new run (fails if exists)
- `Get(ctx, id) (*agent.Run, error)` - Retrieve run by ID
- `Update(ctx, run) error` - Update existing run (fails if not exists)
- `Delete(ctx, id) error` - Remove run by ID
- `List(ctx, filter) ([]*agent.Run, error)` - Query runs with filtering
- `Count(ctx, filter) (int64, error)` - Count runs matching filter

### Filtering

List and Count support rich filtering:

```go
import "go.klarlabs.de/agent/domain/run"

// List completed runs from the last 24 hours
runs, err := store.List(ctx, run.ListFilter{
    Status:     []agent.RunStatus{agent.RunStatusCompleted},
    FromTime:   time.Now().Add(-24 * time.Hour),
    OrderBy:    run.OrderByStartTime,
    Descending: true,
    Limit:      10,
})

// Count failed runs
count, err := store.Count(ctx, run.ListFilter{
    Status: []agent.RunStatus{agent.RunStatusFailed},
})

// Search by goal pattern
runs, err := store.List(ctx, run.ListFilter{
    GoalPattern: "process",
    Limit:       20,
})
```

Note: DynamoDB Scan operations are used for List/Count, which can be expensive for large tables. Consider using pagination and filters to reduce costs.

## Error Handling

The package returns domain-specific errors:

```go
import (
    "errors"
    "go.klarlabs.de/agent/domain/cache"
    "go.klarlabs.de/agent/domain/run"
)

// Cache errors
_, _, err := cache.Get(ctx, "")
if errors.Is(err, cache.ErrInvalidKey) {
    // Handle invalid key
}

// Run store errors
_, err = store.Get(ctx, "nonexistent")
if errors.Is(err, run.ErrRunNotFound) {
    // Handle not found
}

err = store.Save(ctx, existingRun)
if errors.Is(err, run.ErrRunExists) {
    // Handle duplicate
}
```

## Testing

The package includes comprehensive tests using mocked DynamoDB clients:

```bash
go test -race -v ./...
```

For integration testing with real DynamoDB:

1. Use [LocalStack](https://localstack.cloud/) for local DynamoDB
2. Use AWS credentials with test tables
3. Set `AWS_ENDPOINT_URL` environment variable for custom endpoints

```bash
# Start LocalStack
docker run -d -p 4566:4566 localstack/localstack

# Run tests against LocalStack
AWS_ENDPOINT_URL=http://localhost:4566 go test -v ./...
```

## Performance Considerations

### Cache

- **GetItem**: Single-item reads are fast (~1ms p50)
- **PutItem**: Single-item writes are fast (~5ms p50)
- **Clear**: Expensive operation using Scan + BatchWriteItem
  - Consider TTL for automatic cleanup instead

### RunStore

- **Get/Save/Update/Delete**: Fast single-item operations
- **List/Count**: Expensive Scan operations
  - Use filters to reduce scanned items
  - Implement pagination with Limit/Offset
  - Consider adding Global Secondary Indexes for frequent queries

### Cost Optimization

1. Use `PAY_PER_REQUEST` billing for unpredictable workloads
2. Use `PROVISIONED` billing for predictable, sustained traffic
3. Enable point-in-time recovery only if needed
4. Monitor CloudWatch metrics for capacity planning

## Table Schema

### Cache Table

```
Partition Key: pk (String)

Attributes:
- pk (String): Cache key
- value (Binary): Cached value
- ttl (Number, optional): Unix timestamp for expiration
```

### Run Store Table

```
Partition Key: id (String)

Attributes:
- id (String): Run ID
- goal (String): Run goal
- current_state (String): Current state
- status (String): Run status
- start_time (String): ISO 8601 timestamp
- end_time (String, optional): ISO 8601 timestamp
- vars (String, optional): JSON-encoded variables
- evidence (String): JSON-encoded evidence array
- result (String, optional): JSON-encoded result
- error (String, optional): Error message
- pending_question (String, optional): JSON-encoded question
```

## AWS IAM Permissions

Minimum required permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Scan",
        "dynamodb:BatchWriteItem"
      ],
      "Resource": [
        "arn:aws:dynamodb:*:*:table/agent-cache",
        "arn:aws:dynamodb:*:*:table/agent-runs"
      ]
    }
  ]
}
```

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass: `go test -race -v ./...`
2. Code is formatted: `go fmt ./...`
3. No linting errors: `golangci-lint run`
4. Documentation is updated

## License

Same as agent-go parent project.
