// Package distributed provides distributed worker pool support for agent-go.
//
// This package enables horizontal scaling of agent workloads across multiple
// worker processes or machines. It includes:
//
//   - Task queue abstraction for job distribution
//   - Worker registration and health monitoring
//   - Run partitioning and load balancing
//   - Failure recovery and task redelivery
//
// # Usage
//
//	// Create a coordinator
//	coord := distributed.NewCoordinator(distributed.CoordinatorConfig{
//		Queue:       myQueue,
//		RunStore:    myRunStore,
//		EventStore:  myEventStore,
//	})
//
//	// Create and start a worker
//	worker := distributed.NewWorker(distributed.WorkerConfig{
//		ID:          "worker-1",
//		Coordinator: coord,
//		Engine:      myEngine,
//	})
//
//	worker.Start(ctx)
//
// # Architecture
//
// The distributed system follows a coordinator-worker pattern:
//   - Coordinator: Manages task distribution and worker health
//   - Workers: Execute agent runs and report progress
//   - Queue: Provides persistent task storage and delivery
package distributed

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.klarlabs.de/agent/domain/agent"
)

// Common errors for distributed operations.
var (
	ErrWorkerNotFound    = errors.New("worker not found")
	ErrTaskNotFound      = errors.New("task not found")
	ErrQueueFull         = errors.New("queue full")
	ErrWorkerBusy        = errors.New("worker busy")
	ErrCoordinatorClosed = errors.New("coordinator closed")
)

// TaskQueue is the interface for distributed task queues.
// Implementations may use Redis, NATS, RabbitMQ, or other message brokers.
type TaskQueue interface {
	// Enqueue adds a task to the queue.
	Enqueue(ctx context.Context, task Task) error

	// Dequeue retrieves the next available task.
	// Blocks until a task is available or context is canceled.
	Dequeue(ctx context.Context) (Task, error)

	// Acknowledge marks a task as successfully completed.
	Acknowledge(ctx context.Context, taskID string) error

	// Nack marks a task as failed and returns it to the queue.
	Nack(ctx context.Context, taskID string, reason string) error

	// Peek returns tasks without removing them.
	Peek(ctx context.Context, limit int) ([]Task, error)

	// Len returns the number of pending tasks.
	Len(ctx context.Context) (int64, error)
}

// Task represents a unit of work in the distributed system.
type Task struct {
	ID        string         `json:"id"`
	RunID     string         `json:"run_id"`
	Goal      string         `json:"goal"`
	Priority  int            `json:"priority"`
	CreatedAt time.Time      `json:"created_at"`
	Attempts  int            `json:"attempts"`
	MaxRetry  int            `json:"max_retry"`
	Timeout   time.Duration  `json:"timeout"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// TaskResult is the outcome of task execution.
type TaskResult struct {
	TaskID    string          `json:"task_id"`
	RunID     string          `json:"run_id"`
	Status    agent.RunStatus `json:"status"`
	Result    any             `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Duration  time.Duration   `json:"duration"`
	WorkerID  string          `json:"worker_id"`
	Timestamp time.Time       `json:"timestamp"`
}

// CoordinatorConfig configures the coordinator.
type CoordinatorConfig struct {
	// Queue is the task queue implementation.
	Queue TaskQueue

	// HeartbeatInterval is how often workers should report health.
	HeartbeatInterval time.Duration

	// WorkerTimeout is how long before a silent worker is considered dead.
	WorkerTimeout time.Duration

	// MaxRetries is the default maximum task retry count.
	MaxRetries int

	// TaskTimeout is the default task execution timeout.
	TaskTimeout time.Duration
}

// Coordinator manages task distribution to workers.
type Coordinator struct {
	config  CoordinatorConfig
	workers map[string]*WorkerInfo
	mu      sync.RWMutex
	closed  bool
}

// WorkerInfo tracks worker status.
type WorkerInfo struct {
	ID            string    `json:"id"`
	Address       string    `json:"address,omitempty"`
	Status        string    `json:"status"` // "idle", "busy", "dead"
	CurrentTask   string    `json:"current_task,omitempty"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	TasksComplete int64     `json:"tasks_complete"`
	TasksFailed   int64     `json:"tasks_failed"`
}

// NewCoordinator creates a new coordinator.
func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 10 * time.Second
	}
	if cfg.WorkerTimeout == 0 {
		cfg.WorkerTimeout = 30 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.TaskTimeout == 0 {
		cfg.TaskTimeout = 5 * time.Minute
	}

	return &Coordinator{
		config:  cfg,
		workers: make(map[string]*WorkerInfo),
	}
}

// Submit adds a new task to the queue.
func (c *Coordinator) Submit(ctx context.Context, goal string, opts ...TaskOption) (string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return "", ErrCoordinatorClosed
	}
	c.mu.RUnlock()

	task := Task{
		ID:        generateTaskID(),
		Goal:      goal,
		CreatedAt: time.Now(),
		MaxRetry:  c.config.MaxRetries,
		Timeout:   c.config.TaskTimeout,
	}

	for _, opt := range opts {
		opt(&task)
	}

	if err := c.config.Queue.Enqueue(ctx, task); err != nil {
		return "", err
	}

	return task.ID, nil
}

// RegisterWorker adds a worker to the pool.
func (c *Coordinator) RegisterWorker(ctx context.Context, workerID string, address string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrCoordinatorClosed
	}

	c.workers[workerID] = &WorkerInfo{
		ID:            workerID,
		Address:       address,
		Status:        "idle",
		LastHeartbeat: time.Now(),
	}

	return nil
}

// UnregisterWorker removes a worker from the pool.
func (c *Coordinator) UnregisterWorker(_ context.Context, workerID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.workers, workerID)
	return nil
}

// Heartbeat updates worker status.
func (c *Coordinator) Heartbeat(ctx context.Context, workerID string, status string, currentTask string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	worker, ok := c.workers[workerID]
	if !ok {
		// Re-register the worker
		c.workers[workerID] = &WorkerInfo{
			ID:            workerID,
			Status:        status,
			CurrentTask:   currentTask,
			LastHeartbeat: time.Now(),
		}
		return nil
	}

	worker.Status = status
	worker.CurrentTask = currentTask
	worker.LastHeartbeat = time.Now()
	return nil
}

// ListWorkers returns all registered workers.
func (c *Coordinator) ListWorkers() []WorkerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	workers := make([]WorkerInfo, 0, len(c.workers))
	for _, w := range c.workers {
		workers = append(workers, *w)
	}
	return workers
}

// Close shuts down the coordinator.
func (c *Coordinator) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

// TaskOption configures a task.
type TaskOption func(*Task)

// WithPriority sets the task priority (higher = more urgent).
func WithPriority(priority int) TaskOption {
	return func(t *Task) {
		t.Priority = priority
	}
}

// WithTimeout sets the task timeout.
func WithTimeout(timeout time.Duration) TaskOption {
	return func(t *Task) {
		t.Timeout = timeout
	}
}

// WithMaxRetry sets the maximum retry count.
func WithMaxRetry(maxRetry int) TaskOption {
	return func(t *Task) {
		t.MaxRetry = maxRetry
	}
}

// WithMetadata adds metadata to the task.
func WithMetadata(key string, value any) TaskOption {
	return func(t *Task) {
		if t.Metadata == nil {
			t.Metadata = make(map[string]any)
		}
		t.Metadata[key] = value
	}
}

// RunFunc is a function that executes an agent run.
// It takes a context and goal, returns an error.
type RunFunc func(ctx context.Context, goal string) error

// WorkerConfig configures a worker.
type WorkerConfig struct {
	// ID uniquely identifies this worker.
	ID string

	// Coordinator provides task coordination.
	Coordinator *Coordinator

	// Queue is the task queue to consume from.
	Queue TaskQueue

	// Concurrency is the number of concurrent tasks to process.
	Concurrency int

	// HeartbeatInterval is how often to send heartbeats.
	HeartbeatInterval time.Duration

	// RunFunc executes an agent run. If not set, tasks are acknowledged without execution.
	RunFunc RunFunc
}

// Worker processes tasks from the queue.
type Worker struct {
	config  WorkerConfig
	running bool
	mu      sync.Mutex
	cancel  context.CancelFunc
}

// NewWorker creates a new worker.
func NewWorker(cfg WorkerConfig) *Worker {
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 1
	}
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 10 * time.Second
	}

	return &Worker{
		config: cfg,
	}
}

// Start begins processing tasks.
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return errors.New("worker already running")
	}
	w.running = true

	ctx, w.cancel = context.WithCancel(ctx)
	w.mu.Unlock()

	// Register with coordinator
	if w.config.Coordinator != nil {
		if err := w.config.Coordinator.RegisterWorker(ctx, w.config.ID, ""); err != nil {
			return err
		}
	}

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < w.config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.processLoop(ctx)
		}()
	}

	// Start heartbeat
	go w.heartbeatLoop(ctx)

	wg.Wait()
	return nil
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	w.running = false
	if w.cancel != nil {
		w.cancel()
	}

	return nil
}

// processLoop continuously processes tasks.
func (w *Worker) processLoop(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, err := w.config.Queue.Dequeue(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			// Log error and apply exponential backoff
			log.Printf("distributed: dequeue error: %v, backing off %v", err, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			// Exponential backoff with cap
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Reset backoff on successful dequeue
		backoff = time.Second

		w.processTask(ctx, task)
	}
}

// processTask handles a single task.
func (w *Worker) processTask(ctx context.Context, task Task) {
	// Apply task timeout
	taskCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Update coordinator status
	if w.config.Coordinator != nil {
		_ = w.config.Coordinator.Heartbeat(ctx, w.config.ID, "busy", task.ID)
	}

	// Execute the task
	var taskErr error
	if w.config.RunFunc != nil {
		taskErr = w.config.RunFunc(taskCtx, task.Goal)
	}

	if taskErr != nil {
		// Task failed
		log.Printf("distributed: task %s failed: %v", task.ID, taskErr)
		if task.Attempts < task.MaxRetry {
			if err := w.config.Queue.Nack(ctx, task.ID, taskErr.Error()); err != nil {
				log.Printf("distributed: failed to nack task %s: %v", task.ID, err)
			}
		} else {
			// Max retries exceeded, acknowledge to remove from queue
			log.Printf("distributed: task %s exceeded max retries (%d), dropping", task.ID, task.MaxRetry)
			if err := w.config.Queue.Acknowledge(ctx, task.ID); err != nil {
				log.Printf("distributed: failed to ack failed task %s: %v", task.ID, err)
			}
		}
	} else {
		// Task succeeded
		if err := w.config.Queue.Acknowledge(ctx, task.ID); err != nil {
			log.Printf("distributed: failed to ack task %s: %v", task.ID, err)
		}
	}

	// Update coordinator status back to idle
	if w.config.Coordinator != nil {
		_ = w.config.Coordinator.Heartbeat(ctx, w.config.ID, "idle", "")
	}
}

// heartbeatLoop sends periodic heartbeats.
func (w *Worker) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(w.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.config.Coordinator != nil {
				_ = w.config.Coordinator.Heartbeat(ctx, w.config.ID, "idle", "")
			}
		}
	}
}

// generateTaskID creates a unique task ID.
func generateTaskID() string {
	return "task-" + uuid.New().String()
}
