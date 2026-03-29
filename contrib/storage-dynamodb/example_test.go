package dynamodb_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	storagedynamodb "github.com/felixgeelhaar/agent-go/contrib/storage-dynamodb"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

func ExampleCache() {
	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Initialize cache
	cacheStore := storagedynamodb.NewCache(client, "agent-cache")

	// Store a value with 1 hour TTL
	err = cacheStore.Set(ctx, "tool-result-123", []byte("cached result"), cache.SetOptions{
		TTL: 1 * time.Hour,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve the value
	value, found, err := cacheStore.Get(ctx, "tool-result-123")
	if err != nil {
		log.Fatal(err)
	}
	if found {
		fmt.Printf("Found: %s\n", string(value))
	}

	// Check if key exists
	exists, err := cacheStore.Exists(ctx, "tool-result-123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Exists: %v\n", exists)

	// Delete the value
	err = cacheStore.Delete(ctx, "tool-result-123")
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleRunStore() {
	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Initialize run store
	runStore := storagedynamodb.NewRunStore(client, "agent-runs")

	// Create a new run
	r := agent.NewRun("run-abc123", "Process customer data")
	r.Start()
	r.SetVar("customer_id", "12345")
	r.AddEvidence(agent.NewSystemEvidence("Started processing"))

	// Save the run
	err = runStore.Save(ctx, r)
	if err != nil {
		log.Fatal(err)
	}

	// Update the run
	r.CurrentState = agent.StateExplore
	r.AddEvidence(agent.NewToolEvidence("read_file", []byte(`{"path": "data.json"}`)))
	err = runStore.Update(ctx, r)
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve the run
	retrieved, err := runStore.Get(ctx, "run-abc123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Run: %s, State: %s\n", retrieved.ID, retrieved.CurrentState)

	// List completed runs
	runs, err := runStore.List(ctx, run.ListFilter{
		Status:     []agent.RunStatus{agent.RunStatusCompleted},
		OrderBy:    run.OrderByStartTime,
		Descending: true,
		Limit:      10,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d completed runs\n", len(runs))

	// Count all runs
	count, err := runStore.Count(ctx, run.ListFilter{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total runs: %d\n", count)

	// Delete the run
	err = runStore.Delete(ctx, "run-abc123")
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleNewCacheWithConfig() {
	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Create cache with custom configuration
	cacheStore := storagedynamodb.NewCacheWithConfig(client, storagedynamodb.CacheConfig{
		TableName:        "my-custom-cache-table",
		TTLAttributeName: "ttl",   // Default
		KeyPrefix:        "prod:", // Optional prefix for all keys
	})

	// Use the cache
	err = cacheStore.Set(ctx, "key1", []byte("value1"), cache.SetOptions{})
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleNewRunStoreWithConfig() {
	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Create run store with custom configuration
	runStore := storagedynamodb.NewRunStoreWithConfig(client, storagedynamodb.RunStoreConfig{
		TableName: "my-custom-runs-table",
		GSIName:   "status-index", // Optional GSI for status queries
	})

	// Use the run store
	r := agent.NewRun("run-123", "test goal")
	err = runStore.Save(ctx, r)
	if err != nil {
		log.Fatal(err)
	}
}
