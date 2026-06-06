// Package main demonstrates distributed execution with multiple workers.
// Shows how to scale agent execution across workers using queues and locks.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/distributed"
	"go.klarlabs.de/agent/infrastructure/distributed/lock"
	"go.klarlabs.de/agent/infrastructure/distributed/queue"
	agent "go.klarlabs.de/agent/interfaces/api"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const workerIDKey contextKey = "worker_id"

func main() {
	fmt.Println("=== Distributed Execution Example ===")
	fmt.Println()

	// ============================================
	// Create shared infrastructure
	// ============================================

	// In production, use Redis or NATS instead of memory
	taskQueue := queue.NewMemoryQueue()
	sharedStore := lock.NewMemoryLockStore()

	fmt.Println("Infrastructure created:")
	fmt.Println("  - Queue: MemoryQueue (use Redis/NATS in production)")
	fmt.Println("  - Lock: MemoryLock (use Redis in production)")
	fmt.Println()

	// ============================================
	// Create tools
	// ============================================

	registry := agent.NewToolRegistry()

	processTool := agent.NewToolBuilder("process").
		WithDescription("Processes a work item").
		WithAnnotations(agent.Annotations{ReadOnly: true}).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ItemID string `json:"item_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			// Simulate work
			time.Sleep(50 * time.Millisecond)

			workerID := ctx.Value("worker_id")
			fmt.Printf("  [Worker %v] Processed item: %s\n", workerID, in.ItemID)

			output, err := json.Marshal(map[string]string{
				"status":  "processed",
				"item_id": in.ItemID,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to marshal output: %w", err)
			}
			return tool.Result{Output: output}, nil
		}).
		MustBuild()

	_ = registry.Register(processTool) // Ignore error in example

	// ============================================
	// Create and start workers
	// ============================================

	const numWorkers = 3
	var wg sync.WaitGroup
	workers := make([]*distributed.Worker, numWorkers)

	fmt.Printf("Starting %d workers...\n", numWorkers)
	fmt.Println()

	for i := 0; i < numWorkers; i++ {
		workerID := fmt.Sprintf("worker-%d", i+1)

		// Each worker gets its own lock with shared store
		distLock := lock.NewMemoryLock(
			lock.WithHolderID(workerID),
			lock.WithStore(sharedStore),
		)

		workers[i] = distributed.NewWorker(distributed.WorkerConfig{
			ID:       workerID,
			Queue:    taskQueue,
			Lock:     distLock,
			Registry: registry,
		},
			distributed.WithConcurrency(2),
			distributed.WithPollInterval(10*time.Millisecond),
			distributed.WithTaskTimeout(5*time.Second),
		)

		// Register custom handler for our task type
		workers[i].RegisterHandler(queue.TaskTypeToolCall, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
			// Add worker ID to context
			ctx = context.WithValue(ctx, workerIDKey, task.Metadata["worker_id"])

			var payload queue.ToolCallPayload
			if err := json.Unmarshal(task.Payload, &payload); err != nil {
				return nil, fmt.Errorf("invalid task payload: %w", err)
			}

			t, ok := registry.Get(payload.ToolName)
			if !ok {
				return nil, fmt.Errorf("tool not found: %s", payload.ToolName)
			}
			result, err := t.Execute(ctx, payload.Input)
			if err != nil {
				return nil, err
			}
			return result.Output, nil
		})

		wg.Add(1)
		go func(w *distributed.Worker) {
			defer wg.Done()
			_ = w.Start(context.Background()) // Ignore error in example
		}(workers[i])
	}

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// ============================================
	// Submit tasks to the queue
	// ============================================

	const numTasks = 10
	fmt.Printf("Submitting %d tasks to the queue...\n", numTasks)
	fmt.Println()

	ctx := context.Background()
	for i := 0; i < numTasks; i++ {
		input, err := json.Marshal(map[string]string{
			"item_id": fmt.Sprintf("item-%d", i+1),
		})
		if err != nil {
			fmt.Printf("Failed to marshal input: %v\n", err)
			continue
		}

		task, err := queue.NewToolCallTask(
			fmt.Sprintf("run-%d", i+1),
			"process",
			input,
			"processing",
		)
		if err != nil {
			fmt.Printf("Failed to create task: %v\n", err)
			continue
		}
		task.Priority = i % 3 // Vary priorities

		_ = taskQueue.Enqueue(ctx, task) // Ignore enqueue error in example
	}

	// Wait for processing
	fmt.Println("Processing tasks...")
	time.Sleep(500 * time.Millisecond)

	// ============================================
	// Stop workers
	// ============================================

	fmt.Println()
	fmt.Println("Stopping workers...")
	for _, w := range workers {
		_ = w.Stop() // Ignore stop error in example
	}

	// ============================================
	// Show metrics
	// ============================================

	fmt.Println()
	fmt.Println("=== Worker Metrics ===")
	for i, w := range workers {
		metrics := w.Metrics()
		fmt.Printf("Worker %d:\n", i+1)
		fmt.Printf("  Tasks Started:   %d\n", metrics.TasksStarted)
		fmt.Printf("  Tasks Completed: %d\n", metrics.TasksCompleted)
		fmt.Printf("  Tasks Failed:    %d\n", metrics.TasksFailed)
		if metrics.TasksCompleted > 0 {
			fmt.Printf("  Avg Duration:    %s\n", metrics.AverageTaskDuration())
		}
	}

	// ============================================
	// Summary
	// ============================================

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Println("Distributed execution allows you to:")
	fmt.Println("  - Scale horizontally with multiple workers")
	fmt.Println("  - Process tasks in parallel")
	fmt.Println("  - Use distributed locking for coordination")
	fmt.Println("  - Swap queue backends (Memory -> Redis/NATS)")
	fmt.Println()
	fmt.Println("Production recommendations:")
	fmt.Println("  - Use Redis or NATS for the queue")
	fmt.Println("  - Use Redis for distributed locks")
	fmt.Println("  - Add observability middleware")
	fmt.Println("  - Configure appropriate timeouts and retries")
}
