package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/felixgeelhaar/agent-go/contrib/storage-sqlite"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/cache"
	_ "modernc.org/sqlite"
)

// Example demonstrates basic usage of SQLite storage implementations.
func Example() {
	// Open database connection (use :memory: for testing)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create and initialize cache
	cacheStore := sqlite.NewCache(db)
	if err := cacheStore.EnsureSchema(ctx); err != nil {
		log.Fatal(err)
	}

	// Store and retrieve cached data
	key := "example-key"
	value := []byte("example-value")
	err = cacheStore.Set(ctx, key, value, cache.SetOptions{
		TTL: 5 * time.Minute,
	})
	if err != nil {
		log.Fatal(err)
	}

	retrieved, found, err := cacheStore.Get(ctx, key)
	if err != nil {
		log.Fatal(err)
	}
	if found {
		fmt.Printf("Cache hit: %s\n", retrieved)
	}

	// Create and initialize event store
	eventStore := sqlite.NewEventStore(db)
	if err := eventStore.EnsureSchema(ctx); err != nil {
		log.Fatal(err)
	}

	// Create and initialize run store
	runStore := sqlite.NewRunStore(db)
	if err := runStore.EnsureSchema(ctx); err != nil {
		log.Fatal(err)
	}

	// Save a run
	run := &agent.Run{
		ID:           "run-001",
		Goal:         "Process user request",
		CurrentState: agent.StateIntake,
		Vars:         map[string]any{"user": "alice"},
		Evidence:     []agent.Evidence{},
		Status:       agent.RunStatusRunning,
		StartTime:    time.Now(),
	}

	if err := runStore.Save(ctx, run); err != nil {
		log.Fatal(err)
	}

	// Retrieve the run
	saved, err := runStore.Get(ctx, run.ID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Run ID: %s, Goal: %s, Status: %s\n", saved.ID, saved.Goal, saved.Status)

	// Output:
	// Cache hit: example-value
	// Run ID: run-001, Goal: Process user request, Status: running
}
