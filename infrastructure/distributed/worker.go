// Package distributed provides distributed execution patterns.
package distributed

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/distributed/lock"
	"go.klarlabs.de/agent/infrastructure/distributed/queue"
)

// TaskHandler processes tasks of a specific type.
type TaskHandler func(ctx context.Context, task queue.Task) (json.RawMessage, error)

// Worker processes tasks from a queue.
type Worker struct {
	id           string
	queue        queue.Queue
	lock         lock.Lock
	handlers     map[queue.TaskType]TaskHandler
	registry     tool.Registry
	concurrency  int
	pollInterval time.Duration
	taskTimeout  time.Duration
	lockTTL      time.Duration

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	metrics *WorkerMetrics
	onError func(error)
	onTask  func(queue.Task)
}

// WorkerConfig configures a worker.
type WorkerConfig struct {
	ID           string
	Queue        queue.Queue
	Lock         lock.Lock
	Registry     tool.Registry
	Concurrency  int
	PollInterval time.Duration
	TaskTimeout  time.Duration
	LockTTL      time.Duration
}

// WorkerOption configures the worker.
type WorkerOption func(*Worker)

// WithConcurrency sets the number of concurrent task processors.
func WithConcurrency(n int) WorkerOption {
	return func(w *Worker) {
		if n > 0 {
			w.concurrency = n
		}
	}
}

// WithPollInterval sets the interval for polling the queue.
func WithPollInterval(d time.Duration) WorkerOption {
	return func(w *Worker) {
		w.pollInterval = d
	}
}

// WithTaskTimeout sets the default timeout for task execution.
func WithTaskTimeout(d time.Duration) WorkerOption {
	return func(w *Worker) {
		w.taskTimeout = d
	}
}

// WithLockTTL sets the TTL for task locks.
func WithLockTTL(d time.Duration) WorkerOption {
	return func(w *Worker) {
		w.lockTTL = d
	}
}

// WithErrorHandler sets the error callback.
func WithErrorHandler(fn func(error)) WorkerOption {
	return func(w *Worker) {
		w.onError = fn
	}
}

// WithTaskCallback sets the task start callback.
func WithTaskCallback(fn func(queue.Task)) WorkerOption {
	return func(w *Worker) {
		w.onTask = fn
	}
}

// NewWorker creates a new worker.
func NewWorker(config WorkerConfig, opts ...WorkerOption) *Worker {
	w := &Worker{
		id:           config.ID,
		queue:        config.Queue,
		lock:         config.Lock,
		registry:     config.Registry,
		handlers:     make(map[queue.TaskType]TaskHandler),
		concurrency:  1,
		pollInterval: 100 * time.Millisecond,
		taskTimeout:  30 * time.Second,
		lockTTL:      60 * time.Second,
		metrics:      &WorkerMetrics{},
	}

	if w.id == "" {
		w.id = generateWorkerID()
	}

	for _, opt := range opts {
		opt(w)
	}

	// Register default handlers
	w.RegisterHandler(queue.TaskTypeToolCall, w.handleToolCall)

	return w
}

// ID returns the worker's unique identifier.
func (w *Worker) ID() string {
	return w.id
}

// RegisterHandler registers a handler for a task type.
func (w *Worker) RegisterHandler(taskType queue.TaskType, handler TaskHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[taskType] = handler
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

	// Start worker goroutines
	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.processLoop(ctx)
	}

	return nil
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() error {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = false
	if w.cancel != nil {
		w.cancel()
	}
	w.mu.Unlock()

	w.wg.Wait()
	return nil
}

// IsRunning returns whether the worker is currently running.
func (w *Worker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// Metrics returns the worker's metrics.
func (w *Worker) Metrics() WorkerMetrics {
	w.mu.Lock()
	defer w.mu.Unlock()
	return *w.metrics
}

func (w *Worker) processLoop(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, err := w.queue.Dequeue(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, queue.ErrQueueClosed) {
				return
			}
			if w.onError != nil {
				w.onError(err)
			}
			time.Sleep(w.pollInterval)
			continue
		}

		if task != nil {
			w.processTask(ctx, task)
		}
	}
}

func (w *Worker) processTask(ctx context.Context, task *queue.Task) {
	w.mu.Lock()
	w.metrics.TasksStarted++
	w.mu.Unlock()

	if w.onTask != nil {
		w.onTask(*task)
	}

	start := time.Now()

	// Acquire lock for the task
	lockKey := "task:" + task.ID
	if w.lock != nil {
		acquired, err := w.lock.Acquire(ctx, lockKey, w.lockTTL)
		if err != nil || !acquired {
			_ = w.queue.Reject(ctx, task.ID, "failed to acquire lock", true)
			return
		}
		defer func() { _ = w.lock.Release(ctx, lockKey) }()
	}

	// Get handler for task type
	w.mu.Lock()
	handler, exists := w.handlers[task.Type]
	w.mu.Unlock()

	if !exists {
		_ = w.queue.Reject(ctx, task.ID, "no handler for task type: "+string(task.Type), false)
		w.mu.Lock()
		w.metrics.TasksFailed++
		w.mu.Unlock()
		return
	}

	// Execute with timeout
	taskCtx, cancel := context.WithTimeout(ctx, w.taskTimeout)
	defer cancel()

	result, err := handler(taskCtx, *task)
	duration := time.Since(start)

	if err != nil {
		_ = w.queue.Reject(ctx, task.ID, err.Error(), false)
		w.mu.Lock()
		w.metrics.TasksFailed++
		w.mu.Unlock()
		if w.onError != nil {
			w.onError(err)
		}
		return
	}

	// Acknowledge successful completion
	taskResult := queue.TaskResult{
		TaskID:      task.ID,
		Status:      queue.TaskStatusCompleted,
		Result:      result,
		CompletedAt: time.Now(),
		Duration:    duration,
		WorkerID:    w.id,
	}
	_ = w.queue.Acknowledge(ctx, task.ID, taskResult)

	w.mu.Lock()
	w.metrics.TasksCompleted++
	w.metrics.TotalDuration += duration
	w.mu.Unlock()
}

func (w *Worker) handleToolCall(ctx context.Context, task queue.Task) (json.RawMessage, error) {
	if w.registry == nil {
		return nil, errors.New("no tool registry configured")
	}

	var payload queue.ToolCallPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return nil, err
	}

	t, found := w.registry.Get(payload.ToolName)
	if !found {
		return nil, errors.New("tool not found: " + payload.ToolName)
	}

	result, err := t.Execute(ctx, payload.Input)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// WorkerMetrics tracks worker performance.
type WorkerMetrics struct {
	TasksStarted   int64
	TasksCompleted int64
	TasksFailed    int64
	TotalDuration  time.Duration
}

// AverageTaskDuration returns the average task duration.
func (m *WorkerMetrics) AverageTaskDuration() time.Duration {
	if m.TasksCompleted == 0 {
		return 0
	}
	return m.TotalDuration / time.Duration(m.TasksCompleted)
}

// SuccessRate returns the task success rate.
func (m *WorkerMetrics) SuccessRate() float64 {
	total := m.TasksCompleted + m.TasksFailed
	if total == 0 {
		return 0
	}
	return float64(m.TasksCompleted) / float64(total)
}

func generateWorkerID() string {
	return "worker-" + time.Now().Format("20060102150405")
}
