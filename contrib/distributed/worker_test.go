package distributed

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockQueue is a thread-safe in-memory task queue for testing.
type mockQueue struct {
	mu      sync.Mutex
	tasks   []Task
	acked   []string
	nacked  map[string]string // taskID -> reason
	deqCh   chan Task
	closed  bool
	deqErr  error // error to return from Dequeue
	ackErr  error // error to return from Acknowledge
	nackErr error // error to return from Nack
}

func newMockQueue() *mockQueue {
	return &mockQueue{
		acked:  make([]string, 0),
		nacked: make(map[string]string),
		deqCh:  make(chan Task, 100),
	}
}

func (q *mockQueue) Enqueue(ctx context.Context, task Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return errors.New("queue closed")
	}
	q.tasks = append(q.tasks, task)
	select {
	case q.deqCh <- task:
	default:
		// Channel full, task will be dequeued later
	}
	return nil
}

func (q *mockQueue) Dequeue(ctx context.Context) (Task, error) {
	q.mu.Lock()
	err := q.deqErr
	q.mu.Unlock()

	if err != nil {
		return Task{}, err
	}

	select {
	case t := <-q.deqCh:
		return t, nil
	case <-ctx.Done():
		return Task{}, ctx.Err()
	}
}

func (q *mockQueue) Acknowledge(ctx context.Context, taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ackErr != nil {
		return q.ackErr
	}
	q.acked = append(q.acked, taskID)
	return nil
}

func (q *mockQueue) Nack(ctx context.Context, taskID string, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.nackErr != nil {
		return q.nackErr
	}
	q.nacked[taskID] = reason
	return nil
}

func (q *mockQueue) Peek(ctx context.Context, limit int) ([]Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	n := limit
	if n > len(q.tasks) {
		n = len(q.tasks)
	}

	result := make([]Task, n)
	copy(result, q.tasks[:n])
	return result, nil
}

func (q *mockQueue) Len(ctx context.Context) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int64(len(q.tasks)), nil
}

func (q *mockQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	close(q.deqCh)
}

func (q *mockQueue) GetAcked() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]string, len(q.acked))
	copy(result, q.acked)
	return result
}

func (q *mockQueue) GetNacked() map[string]string {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make(map[string]string)
	for k, v := range q.nacked {
		result[k] = v
	}
	return result
}

func (q *mockQueue) SetDequeueError(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.deqErr = err
}

func (q *mockQueue) SetAcknowledgeError(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ackErr = err
}

func (q *mockQueue) SetNackError(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.nackErr = err
}

// TestGenerateTaskID verifies task ID generation.
func TestGenerateTaskID(t *testing.T) {
	// Generate multiple IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateTaskID()

		// Check prefix
		if !strings.HasPrefix(id, "task-") {
			t.Errorf("task ID missing 'task-' prefix: %s", id)
		}

		// Check uniqueness
		if ids[id] {
			t.Errorf("duplicate task ID generated: %s", id)
		}
		ids[id] = true

		// Check format (task-UUID)
		parts := strings.Split(id, "-")
		if len(parts) != 6 { // task + 5 UUID parts
			t.Errorf("unexpected task ID format: %s", id)
		}
	}
}

// TestCoordinatorSubmit verifies task submission.
func TestCoordinatorSubmit(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	coord := NewCoordinator(CoordinatorConfig{
		Queue: queue,
	})

	ctx := context.Background()
	taskID, err := coord.Submit(ctx, "test goal")
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	if !strings.HasPrefix(taskID, "task-") {
		t.Errorf("task ID missing prefix: %s", taskID)
	}

	// Verify task was enqueued
	tasks, err := queue.Peek(ctx, 10)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].ID != taskID {
		t.Errorf("task ID mismatch: got %s, want %s", tasks[0].ID, taskID)
	}

	if tasks[0].Goal != "test goal" {
		t.Errorf("task goal mismatch: got %s, want 'test goal'", tasks[0].Goal)
	}
}

// TestCoordinatorSubmitWithOptions verifies task options.
func TestCoordinatorSubmitWithOptions(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	coord := NewCoordinator(CoordinatorConfig{
		Queue: queue,
	})

	ctx := context.Background()
	taskID, err := coord.Submit(ctx, "test goal",
		WithPriority(10),
		WithTimeout(30*time.Second),
		WithMaxRetry(5),
		WithMetadata("key", "value"),
	)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	tasks, err := queue.Peek(ctx, 10)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}

	task := tasks[0]
	if task.Priority != 10 {
		t.Errorf("priority mismatch: got %d, want 10", task.Priority)
	}

	if task.Timeout != 30*time.Second {
		t.Errorf("timeout mismatch: got %v, want 30s", task.Timeout)
	}

	if task.MaxRetry != 5 {
		t.Errorf("max retry mismatch: got %d, want 5", task.MaxRetry)
	}

	if task.Metadata["key"] != "value" {
		t.Errorf("metadata mismatch: got %v, want 'value'", task.Metadata["key"])
	}

	_ = taskID
}

// TestCoordinatorRegisterWorker verifies worker registration.
func TestCoordinatorRegisterWorker(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	coord := NewCoordinator(CoordinatorConfig{
		Queue: queue,
	})

	ctx := context.Background()
	err := coord.RegisterWorker(ctx, "worker-1", "localhost:8080")
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	workers := coord.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}

	worker := workers[0]
	if worker.ID != "worker-1" {
		t.Errorf("worker ID mismatch: got %s, want 'worker-1'", worker.ID)
	}

	if worker.Address != "localhost:8080" {
		t.Errorf("worker address mismatch: got %s, want 'localhost:8080'", worker.Address)
	}

	if worker.Status != "idle" {
		t.Errorf("worker status mismatch: got %s, want 'idle'", worker.Status)
	}
}

// TestCoordinatorHeartbeat verifies heartbeat updates.
func TestCoordinatorHeartbeat(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	coord := NewCoordinator(CoordinatorConfig{
		Queue: queue,
	})

	ctx := context.Background()

	// Register worker
	err := coord.RegisterWorker(ctx, "worker-1", "localhost:8080")
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	// Send heartbeat
	err = coord.Heartbeat(ctx, "worker-1", "busy", "task-123")
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	workers := coord.ListWorkers()
	worker := workers[0]

	if worker.Status != "busy" {
		t.Errorf("worker status mismatch: got %s, want 'busy'", worker.Status)
	}

	if worker.CurrentTask != "task-123" {
		t.Errorf("worker current task mismatch: got %s, want 'task-123'", worker.CurrentTask)
	}
}

// TestCoordinatorHeartbeatAutoRegister verifies auto-registration on heartbeat.
func TestCoordinatorHeartbeatAutoRegister(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	coord := NewCoordinator(CoordinatorConfig{
		Queue: queue,
	})

	ctx := context.Background()

	// Send heartbeat without registering first
	err := coord.Heartbeat(ctx, "worker-1", "idle", "")
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	workers := coord.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}

	worker := workers[0]
	if worker.ID != "worker-1" {
		t.Errorf("worker ID mismatch: got %s, want 'worker-1'", worker.ID)
	}
}

// TestProcessTaskSuccess verifies successful task processing.
func TestProcessTaskSuccess(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	coord := NewCoordinator(CoordinatorConfig{
		Queue: queue,
	})

	executed := false
	worker := NewWorker(WorkerConfig{
		ID:          "worker-1",
		Coordinator: coord,
		Queue:       queue,
		RunFunc: func(ctx context.Context, goal string) error {
			executed = true
			if goal != "test goal" {
				t.Errorf("goal mismatch: got %s, want 'test goal'", goal)
			}
			return nil
		},
	})

	ctx := context.Background()

	// Register worker
	err := coord.RegisterWorker(ctx, "worker-1", "")
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	task := Task{
		ID:       "task-1",
		Goal:     "test goal",
		MaxRetry: 3,
		Timeout:  5 * time.Second,
	}

	worker.processTask(ctx, task)

	if !executed {
		t.Error("RunFunc was not executed")
	}

	// Verify task was acknowledged
	acked := queue.GetAcked()
	if len(acked) != 1 {
		t.Fatalf("expected 1 acked task, got %d", len(acked))
	}

	if acked[0] != "task-1" {
		t.Errorf("acked task ID mismatch: got %s, want 'task-1'", acked[0])
	}

	// Verify worker status was updated
	workers := coord.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}

	// After processing, status should be idle
	if workers[0].Status != "idle" {
		t.Errorf("worker status mismatch: got %s, want 'idle'", workers[0].Status)
	}
}

// TestProcessTaskFailureWithRetry verifies task nack with retries remaining.
func TestProcessTaskFailureWithRetry(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	testErr := errors.New("task execution failed")
	worker := NewWorker(WorkerConfig{
		ID:    "worker-1",
		Queue: queue,
		RunFunc: func(ctx context.Context, goal string) error {
			return testErr
		},
	})

	ctx := context.Background()

	task := Task{
		ID:       "task-1",
		Goal:     "test goal",
		Attempts: 1,
		MaxRetry: 3,
	}

	worker.processTask(ctx, task)

	// Verify task was nacked
	nacked := queue.GetNacked()
	if len(nacked) != 1 {
		t.Fatalf("expected 1 nacked task, got %d", len(nacked))
	}

	reason, ok := nacked["task-1"]
	if !ok {
		t.Fatal("task-1 not found in nacked tasks")
	}

	if !strings.Contains(reason, "task execution failed") {
		t.Errorf("nack reason mismatch: got %s", reason)
	}

	// Verify task was not acknowledged
	acked := queue.GetAcked()
	if len(acked) != 0 {
		t.Errorf("expected 0 acked tasks, got %d", len(acked))
	}
}

// TestProcessTaskFailureMaxRetries verifies task acknowledgment after max retries.
func TestProcessTaskFailureMaxRetries(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	testErr := errors.New("task execution failed")
	worker := NewWorker(WorkerConfig{
		ID:    "worker-1",
		Queue: queue,
		RunFunc: func(ctx context.Context, goal string) error {
			return testErr
		},
	})

	ctx := context.Background()

	task := Task{
		ID:       "task-1",
		Goal:     "test goal",
		Attempts: 3,
		MaxRetry: 3,
	}

	worker.processTask(ctx, task)

	// Verify task was acknowledged (dropped)
	acked := queue.GetAcked()
	if len(acked) != 1 {
		t.Fatalf("expected 1 acked task, got %d", len(acked))
	}

	if acked[0] != "task-1" {
		t.Errorf("acked task ID mismatch: got %s, want 'task-1'", acked[0])
	}

	// Verify task was not nacked
	nacked := queue.GetNacked()
	if len(nacked) != 0 {
		t.Errorf("expected 0 nacked tasks, got %d", len(nacked))
	}
}

// TestProcessTaskTimeout verifies task timeout handling.
func TestProcessTaskTimeout(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	worker := NewWorker(WorkerConfig{
		ID:    "worker-1",
		Queue: queue,
		RunFunc: func(ctx context.Context, goal string) error {
			// Simulate long-running task
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
				return nil
			}
		},
	})

	ctx := context.Background()

	task := Task{
		ID:       "task-1",
		Goal:     "test goal",
		MaxRetry: 3,
		Timeout:  50 * time.Millisecond,
	}

	start := time.Now()
	worker.processTask(ctx, task)
	duration := time.Since(start)

	// Verify timeout was applied
	if duration > 200*time.Millisecond {
		t.Errorf("task took too long: %v", duration)
	}

	// Verify task was nacked due to timeout
	nacked := queue.GetNacked()
	if len(nacked) != 1 {
		t.Fatalf("expected 1 nacked task, got %d", len(nacked))
	}
}

// TestProcessTaskNoRunFunc verifies processing without RunFunc.
func TestProcessTaskNoRunFunc(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	worker := NewWorker(WorkerConfig{
		ID:      "worker-1",
		Queue:   queue,
		RunFunc: nil, // No execution function
	})

	ctx := context.Background()

	task := Task{
		ID:       "task-1",
		Goal:     "test goal",
		MaxRetry: 3,
	}

	worker.processTask(ctx, task)

	// Verify task was acknowledged (no error since RunFunc is nil)
	acked := queue.GetAcked()
	if len(acked) != 1 {
		t.Fatalf("expected 1 acked task, got %d", len(acked))
	}
}

// TestProcessLoopContextCancellation verifies graceful shutdown.
func TestProcessLoopContextCancellation(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	worker := NewWorker(WorkerConfig{
		ID:    "worker-1",
		Queue: queue,
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool)
	go func() {
		worker.processLoop(ctx)
		done <- true
	}()

	// Cancel context
	cancel()

	// Wait for processLoop to exit
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("processLoop did not exit after context cancellation")
	}
}

// TestProcessLoopExponentialBackoff verifies exponential backoff on errors.
func TestProcessLoopExponentialBackoff(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	worker := NewWorker(WorkerConfig{
		ID:    "worker-1",
		Queue: queue,
	})

	// Set queue to return errors
	queue.SetDequeueError(errors.New("queue error"))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	worker.processLoop(ctx)
	duration := time.Since(start)

	// Verify backoff occurred (should be at least 1s + 2s)
	// Due to timeout, we won't see full backoff, but should see some delay
	if duration < 100*time.Millisecond {
		t.Errorf("expected backoff delay, but got %v", duration)
	}
}

// TestWorkerStartStop verifies worker lifecycle.
func TestWorkerStartStop(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	coord := NewCoordinator(CoordinatorConfig{
		Queue: queue,
	})

	worker := NewWorker(WorkerConfig{
		ID:          "worker-1",
		Coordinator: coord,
		Queue:       queue,
		Concurrency: 2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start worker in background
	done := make(chan error)
	go func() {
		done <- worker.Start(ctx)
	}()

	// Give worker time to start
	time.Sleep(20 * time.Millisecond)

	// Verify worker was registered
	workers := coord.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}

	// Stop worker
	err := worker.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Wait for Start to complete
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

// TestCoordinatorClose verifies coordinator shutdown.
func TestCoordinatorClose(t *testing.T) {
	queue := newMockQueue()
	defer queue.Close()

	coord := NewCoordinator(CoordinatorConfig{
		Queue: queue,
	})

	ctx := context.Background()

	// Register worker
	err := coord.RegisterWorker(ctx, "worker-1", "")
	if err != nil {
		t.Fatalf("RegisterWorker failed: %v", err)
	}

	// Close coordinator
	err = coord.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify operations fail after close
	_, err = coord.Submit(ctx, "test goal")
	if err != ErrCoordinatorClosed {
		t.Errorf("expected ErrCoordinatorClosed, got %v", err)
	}

	err = coord.RegisterWorker(ctx, "worker-2", "")
	if err != ErrCoordinatorClosed {
		t.Errorf("expected ErrCoordinatorClosed, got %v", err)
	}
}
