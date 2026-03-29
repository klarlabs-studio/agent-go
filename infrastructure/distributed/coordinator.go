package distributed

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/infrastructure/distributed/lock"
	"github.com/felixgeelhaar/agent-go/infrastructure/distributed/queue"
)

// Coordinator distributes work across multiple workers.
type Coordinator struct {
	queue   queue.Queue
	lock    lock.Lock
	workers []*Worker

	mu            sync.Mutex
	running       bool
	cancel        context.CancelFunc
	runStates     map[string]*RunState
	onRunStart    func(runID string)
	onRunComplete func(runID string, result json.RawMessage)
	onRunFailed   func(runID string, err error)
}

// RunState tracks the state of a distributed run.
type RunState struct {
	RunID        string
	Status       RunStatus
	CurrentState string
	TasksPending int
	TasksRunning int
	TasksDone    int
	StartedAt    time.Time
	UpdatedAt    time.Time
	Result       json.RawMessage
	Error        error
}

// RunStatus represents the status of a run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

// CoordinatorConfig configures the coordinator.
type CoordinatorConfig struct {
	Queue   queue.Queue
	Lock    lock.Lock
	Workers int
}

// CoordinatorOption configures the coordinator.
type CoordinatorOption func(*Coordinator)

// WithRunStartCallback sets the callback for run start events.
func WithRunStartCallback(fn func(runID string)) CoordinatorOption {
	return func(c *Coordinator) {
		c.onRunStart = fn
	}
}

// WithRunCompleteCallback sets the callback for run completion events.
func WithRunCompleteCallback(fn func(runID string, result json.RawMessage)) CoordinatorOption {
	return func(c *Coordinator) {
		c.onRunComplete = fn
	}
}

// WithRunFailedCallback sets the callback for run failure events.
func WithRunFailedCallback(fn func(runID string, err error)) CoordinatorOption {
	return func(c *Coordinator) {
		c.onRunFailed = fn
	}
}

// NewCoordinator creates a new coordinator.
func NewCoordinator(config CoordinatorConfig, opts ...CoordinatorOption) *Coordinator {
	c := &Coordinator{
		queue:     config.Queue,
		lock:      config.Lock,
		workers:   make([]*Worker, 0),
		runStates: make(map[string]*RunState),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Create workers
	for i := 0; i < config.Workers; i++ {
		worker := NewWorker(WorkerConfig{
			Queue: config.Queue,
			Lock:  config.Lock,
		})
		c.workers = append(c.workers, worker)
	}

	return c
}

// AddWorker adds a worker to the coordinator.
func (c *Coordinator) AddWorker(worker *Worker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workers = append(c.workers, worker)
}

// Start begins the coordinator and all workers.
func (c *Coordinator) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return errors.New("coordinator already running")
	}
	c.running = true
	ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	// Start all workers
	for _, worker := range c.workers {
		if err := worker.Start(ctx); err != nil {
			_ = c.Stop()
			return err
		}
	}

	return nil
}

// Stop gracefully stops the coordinator and all workers.
func (c *Coordinator) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()

	// Stop all workers
	var firstErr error
	for _, worker := range c.workers {
		if err := worker.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// SubmitRun creates a new distributed run and enqueues initial tasks.
func (c *Coordinator) SubmitRun(ctx context.Context, runID string, initialTasks []queue.Task) error {
	c.mu.Lock()
	if _, exists := c.runStates[runID]; exists {
		c.mu.Unlock()
		return errors.New("run already exists")
	}

	state := &RunState{
		RunID:        runID,
		Status:       RunStatusPending,
		TasksPending: len(initialTasks),
		StartedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	c.runStates[runID] = state
	c.mu.Unlock()

	// Enqueue all tasks
	for _, task := range initialTasks {
		task.RunID = runID
		if err := c.queue.Enqueue(ctx, task); err != nil {
			c.mu.Lock()
			state.Status = RunStatusFailed
			state.Error = err
			c.mu.Unlock()
			return err
		}
	}

	c.mu.Lock()
	state.Status = RunStatusRunning
	c.mu.Unlock()

	if c.onRunStart != nil {
		c.onRunStart(runID)
	}

	return nil
}

// GetRunState returns the current state of a run.
func (c *Coordinator) GetRunState(runID string) (*RunState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, exists := c.runStates[runID]
	if !exists {
		return nil, false
	}
	// Return a copy
	copy := *state
	return &copy, true
}

// CompleteRun marks a run as completed.
func (c *Coordinator) CompleteRun(runID string, result json.RawMessage) {
	c.mu.Lock()
	state, exists := c.runStates[runID]
	if exists {
		state.Status = RunStatusCompleted
		state.Result = result
		state.UpdatedAt = time.Now()
	}
	c.mu.Unlock()

	if c.onRunComplete != nil {
		c.onRunComplete(runID, result)
	}
}

// FailRun marks a run as failed.
func (c *Coordinator) FailRun(runID string, err error) {
	c.mu.Lock()
	state, exists := c.runStates[runID]
	if exists {
		state.Status = RunStatusFailed
		state.Error = err
		state.UpdatedAt = time.Now()
	}
	c.mu.Unlock()

	if c.onRunFailed != nil {
		c.onRunFailed(runID, err)
	}
}

// EnqueueTask adds a task to the queue for a run.
func (c *Coordinator) EnqueueTask(ctx context.Context, runID string, task queue.Task) error {
	task.RunID = runID

	c.mu.Lock()
	state, exists := c.runStates[runID]
	if exists {
		state.TasksPending++
		state.UpdatedAt = time.Now()
	}
	c.mu.Unlock()

	return c.queue.Enqueue(ctx, task)
}

// TaskCompleted marks a task as completed for tracking.
func (c *Coordinator) TaskCompleted(runID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, exists := c.runStates[runID]
	if exists {
		if state.TasksPending > 0 {
			state.TasksPending--
		}
		state.TasksDone++
		state.UpdatedAt = time.Now()
	}
}

// TaskFailed marks a task as failed for tracking.
func (c *Coordinator) TaskFailed(runID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, exists := c.runStates[runID]
	if exists {
		if state.TasksPending > 0 {
			state.TasksPending--
		}
		state.UpdatedAt = time.Now()
	}
}

// WorkerCount returns the number of workers.
func (c *Coordinator) WorkerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.workers)
}

// ActiveRuns returns the number of active runs.
func (c *Coordinator) ActiveRuns() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	for _, state := range c.runStates {
		if state.Status == RunStatusRunning {
			count++
		}
	}
	return count
}

// Metrics returns aggregated metrics from all workers.
func (c *Coordinator) Metrics() CoordinatorMetrics {
	var metrics CoordinatorMetrics

	c.mu.Lock()
	workers := make([]*Worker, len(c.workers))
	copy(workers, c.workers)
	c.mu.Unlock()

	for _, worker := range workers {
		wm := worker.Metrics()
		metrics.TotalTasksStarted += wm.TasksStarted
		metrics.TotalTasksCompleted += wm.TasksCompleted
		metrics.TotalTasksFailed += wm.TasksFailed
		metrics.TotalDuration += wm.TotalDuration
	}
	metrics.WorkerCount = len(workers)

	return metrics
}

// CoordinatorMetrics aggregates metrics across workers.
type CoordinatorMetrics struct {
	WorkerCount         int
	TotalTasksStarted   int64
	TotalTasksCompleted int64
	TotalTasksFailed    int64
	TotalDuration       time.Duration
}

// AverageTaskDuration returns the average task duration.
func (m *CoordinatorMetrics) AverageTaskDuration() time.Duration {
	if m.TotalTasksCompleted == 0 {
		return 0
	}
	return m.TotalDuration / time.Duration(m.TotalTasksCompleted)
}

// SuccessRate returns the overall success rate.
func (m *CoordinatorMetrics) SuccessRate() float64 {
	total := m.TotalTasksCompleted + m.TotalTasksFailed
	if total == 0 {
		return 0
	}
	return float64(m.TotalTasksCompleted) / float64(total)
}
