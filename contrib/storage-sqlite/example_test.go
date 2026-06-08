package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sqlite "go.klarlabs.de/agent/contrib/storage-sqlite"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/cache"
	_ "modernc.org/sqlite"
)

// Example demonstrates basic usage of SQLite storage implementations.
func Example() {
	// Open database connection (use :memory: for testing)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	ctx := context.Background()

	// Create and initialize cache
	cacheStore := sqlite.NewCache(db)
	if err := cacheStore.EnsureSchema(ctx); err != nil {
		fmt.Println(err)
		return
	}

	// Store and retrieve cached data
	key := "example-key"
	value := []byte("example-value")
	err = cacheStore.Set(ctx, key, value, cache.SetOptions{
		TTL: 5 * time.Minute,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	retrieved, found, err := cacheStore.Get(ctx, key)
	if err != nil {
		fmt.Println(err)
		return
	}
	if found {
		fmt.Printf("Cache hit: %s\n", retrieved)
	}

	// Create and initialize event store
	eventStore := sqlite.NewEventStore(db)
	if err := eventStore.EnsureSchema(ctx); err != nil {
		fmt.Println(err)
		return
	}

	// Create and initialize run store
	runStore := sqlite.NewRunStore(db)
	if err := runStore.EnsureSchema(ctx); err != nil {
		fmt.Println(err)
		return
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
		fmt.Println(err)
		return
	}

	// Retrieve the run
	saved, err := runStore.Get(ctx, run.ID)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Run ID: %s, Goal: %s, Status: %s\n", saved.ID, saved.Goal, saved.Status)

	// Output:
	// Cache hit: example-value
	// Run ID: run-001, Goal: Process user request, Status: running
}
