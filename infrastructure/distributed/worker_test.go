package distributed

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.klarlabs.de/agent/infrastructure/distributed/lock"
	"go.klarlabs.de/agent/infrastructure/distributed/queue"
)

func TestWorkerStartStop(t *testing.T) {
	q := queue.NewMemoryQueue()
	w := NewWorker(WorkerConfig{
		Queue: q,
	})

	ctx := context.Background()
	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	if !w.IsRunning() {
		t.Error("expected worker to be running")
	}

	err = w.Stop()
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if w.IsRunning() {
		t.Error("expected worker to be stopped")
	}
}

func TestWorkerDoubleStart(t *testing.T) {
	q := queue.NewMemoryQueue()
	w := NewWorker(WorkerConfig{Queue: q})

	ctx := context.Background()
	_ = w.Start(ctx)
	defer w.Stop()

	err := w.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}
}

func TestWorkerID(t *testing.T) {
	w1 := NewWorker(WorkerConfig{})
	w2 := NewWorker(WorkerConfig{ID: "custom-id"})

	if w1.ID() == "" {
		t.Error("expected auto-generated ID")
	}
	if w2.ID() != "custom-id" {
		t.Errorf("expected custom-id, got %s", w2.ID())
	}
}

func TestWorkerCustomHandler(t *testing.T) {
	q := queue.NewMemoryQueue()
	w := NewWorker(WorkerConfig{Queue: q})

	handlerCalled := make(chan struct{}, 1)
	w.RegisterHandler(queue.TaskTypeCustom, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
		select {
		case handlerCalled <- struct{}{}:
		default:
		}
		return json.RawMessage(`{"result": "ok"}`), nil
	})

	ctx := context.Background()
	w.Start(ctx)
	defer w.Stop()

	task := queue.NewTask(queue.TaskTypeCustom, json.RawMessage(`{}`))
	q.Enqueue(ctx, task)

	select {
	case <-handlerCalled:
		// Handler was called
	case <-time.After(100 * time.Millisecond):
		t.Error("expected custom handler to be called")
	}
}

func TestWorkerHandlerError(t *testing.T) {
	q := queue.NewMemoryQueue()
	w := NewWorker(WorkerConfig{Queue: q})

	w.RegisterHandler(queue.TaskTypeCustom, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
		return nil, errors.New("handler error")
	})

	errorCh := make(chan error, 1)
	w.onError = func(err error) {
		select {
		case errorCh <- err:
		default:
		}
	}

	ctx := context.Background()
	w.Start(ctx)
	defer w.Stop()

	task := queue.NewTask(queue.TaskTypeCustom, json.RawMessage(`{}`))
	q.Enqueue(ctx, task)

	select {
	case <-errorCh:
		// Error was captured
	case <-time.After(100 * time.Millisecond):
		t.Error("expected error to be captured")
	}

	// Give time for metrics to update
	time.Sleep(10 * time.Millisecond)

	metrics := w.Metrics()
	if metrics.TasksFailed != 1 {
		t.Errorf("expected 1 failed task, got %d", metrics.TasksFailed)
	}
}

func TestWorkerMetrics(t *testing.T) {
	q := queue.NewMemoryQueue()
	w := NewWorker(WorkerConfig{Queue: q})

	var completed int32
	w.RegisterHandler(queue.TaskTypeCustom, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&completed, 1)
		return json.RawMessage(`{}`), nil
	})

	ctx := context.Background()
	w.Start(ctx)
	defer w.Stop()

	for i := 0; i < 3; i++ {
		task := queue.NewTask(queue.TaskTypeCustom, json.RawMessage(`{}`))
		q.Enqueue(ctx, task)
	}

	// Wait for all tasks to complete
	for atomic.LoadInt32(&completed) < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // Allow metrics to update

	metrics := w.Metrics()
	if metrics.TasksCompleted != 3 {
		t.Errorf("expected 3 completed tasks, got %d", metrics.TasksCompleted)
	}
	if metrics.TotalDuration < 30*time.Millisecond {
		t.Error("expected total duration to be at least 30ms")
	}
	if metrics.AverageTaskDuration() < 10*time.Millisecond {
		t.Error("expected average duration to be at least 10ms")
	}
	if metrics.SuccessRate() != 1.0 {
		t.Errorf("expected 100%% success rate, got %.2f", metrics.SuccessRate())
	}
}

func TestWorkerWithLock(t *testing.T) {
	q := queue.NewMemoryQueue()
	l := lock.NewMemoryLock()
	w := NewWorker(WorkerConfig{
		Queue: q,
		Lock:  l,
	})

	processed := make(chan struct{})
	w.RegisterHandler(queue.TaskTypeCustom, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
		close(processed)
		return json.RawMessage(`{}`), nil
	})

	ctx := context.Background()
	w.Start(ctx)
	defer w.Stop()

	task := queue.NewTask(queue.TaskTypeCustom, json.RawMessage(`{}`))
	q.Enqueue(ctx, task)

	select {
	case <-processed:
	case <-time.After(100 * time.Millisecond):
		t.Error("task was not processed")
	}
}

func TestWorkerConcurrency(t *testing.T) {
	q := queue.NewMemoryQueue()
	w := NewWorker(WorkerConfig{Queue: q}, WithConcurrency(3))

	var mu sync.Mutex
	activeCount := 0
	maxActive := 0

	var completedCount int32

	w.RegisterHandler(queue.TaskTypeCustom, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
		mu.Lock()
		activeCount++
		if activeCount > maxActive {
			maxActive = activeCount
		}
		mu.Unlock()

		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		activeCount--
		mu.Unlock()

		atomic.AddInt32(&completedCount, 1)
		return json.RawMessage(`{}`), nil
	})

	ctx := context.Background()
	w.Start(ctx)
	defer w.Stop()

	// Enqueue more tasks than concurrency
	for i := 0; i < 6; i++ {
		task := queue.NewTask(queue.TaskTypeCustom, json.RawMessage(`{}`))
		q.Enqueue(ctx, task)
	}

	// Wait for at least 3 to complete
	for atomic.LoadInt32(&completedCount) < 3 {
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	observed := maxActive
	mu.Unlock()

	if observed > 3 {
		t.Errorf("expected max 3 concurrent, got %d", observed)
	}
}

func TestWorkerTaskCallback(t *testing.T) {
	q := queue.NewMemoryQueue()
	w := NewWorker(WorkerConfig{Queue: q})

	w.RegisterHandler(queue.TaskTypeCustom, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	})

	taskSeenCh := make(chan struct{}, 1)
	w.onTask = func(task queue.Task) {
		select {
		case taskSeenCh <- struct{}{}:
		default:
		}
	}

	ctx := context.Background()
	w.Start(ctx)
	defer w.Stop()

	task := queue.NewTask(queue.TaskTypeCustom, json.RawMessage(`{"key": "value"}`))
	q.Enqueue(ctx, task)

	select {
	case <-taskSeenCh:
		// Task callback was called
	case <-time.After(100 * time.Millisecond):
		t.Error("expected task callback to be called")
	}
}

func TestWorkerOptions(t *testing.T) {
	q := queue.NewMemoryQueue()
	w := NewWorker(WorkerConfig{Queue: q},
		WithConcurrency(5),
		WithPollInterval(200*time.Millisecond),
		WithTaskTimeout(1*time.Minute),
		WithLockTTL(2*time.Minute),
	)

	if w.concurrency != 5 {
		t.Errorf("expected concurrency 5, got %d", w.concurrency)
	}
	if w.pollInterval != 200*time.Millisecond {
		t.Errorf("expected poll interval 200ms, got %v", w.pollInterval)
	}
	if w.taskTimeout != 1*time.Minute {
		t.Errorf("expected task timeout 1m, got %v", w.taskTimeout)
	}
	if w.lockTTL != 2*time.Minute {
		t.Errorf("expected lock TTL 2m, got %v", w.lockTTL)
	}
}

func TestCoordinatorStartStop(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{
		Queue:   q,
		Workers: 2,
	})

	ctx := context.Background()
	err := c.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	if c.WorkerCount() != 2 {
		t.Errorf("expected 2 workers, got %d", c.WorkerCount())
	}

	err = c.Stop()
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

func TestCoordinatorDoubleStart(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{Queue: q})

	ctx := context.Background()
	c.Start(ctx)
	defer c.Stop()

	err := c.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}
}

func TestCoordinatorSubmitRun(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{Queue: q})

	ctx := context.Background()
	c.Start(ctx)
	defer c.Stop()

	task := queue.NewTask(queue.TaskTypeCustom, json.RawMessage(`{}`))
	err := c.SubmitRun(ctx, "run-123", []queue.Task{task})
	if err != nil {
		t.Fatalf("submit run failed: %v", err)
	}

	state, exists := c.GetRunState("run-123")
	if !exists {
		t.Error("expected run state to exist")
	}
	if state.Status != RunStatusRunning {
		t.Errorf("expected status running, got %v", state.Status)
	}
	if state.TasksPending != 1 {
		t.Errorf("expected 1 pending task, got %d", state.TasksPending)
	}
}

func TestCoordinatorDuplicateRun(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{Queue: q})

	ctx := context.Background()
	c.Start(ctx)
	defer c.Stop()

	c.SubmitRun(ctx, "run-123", []queue.Task{})

	err := c.SubmitRun(ctx, "run-123", []queue.Task{})
	if err == nil {
		t.Error("expected error for duplicate run")
	}
}

func TestCoordinatorRunCallbacks(t *testing.T) {
	q := queue.NewMemoryQueue()

	var mu sync.Mutex
	startCalled := false
	completeCalled := false
	failedCalled := false

	c := NewCoordinator(CoordinatorConfig{Queue: q},
		WithRunStartCallback(func(runID string) {
			mu.Lock()
			startCalled = true
			mu.Unlock()
		}),
		WithRunCompleteCallback(func(runID string, result json.RawMessage) {
			mu.Lock()
			completeCalled = true
			mu.Unlock()
		}),
		WithRunFailedCallback(func(runID string, err error) {
			mu.Lock()
			failedCalled = true
			mu.Unlock()
		}),
	)

	ctx := context.Background()
	c.Start(ctx)
	defer c.Stop()

	c.SubmitRun(ctx, "run-1", []queue.Task{})
	mu.Lock()
	if !startCalled {
		t.Error("expected start callback to be called")
	}
	mu.Unlock()

	c.CompleteRun("run-1", json.RawMessage(`{"done": true}`))
	mu.Lock()
	if !completeCalled {
		t.Error("expected complete callback to be called")
	}
	mu.Unlock()

	c.SubmitRun(ctx, "run-2", []queue.Task{})
	c.FailRun("run-2", errors.New("test error"))
	mu.Lock()
	if !failedCalled {
		t.Error("expected failed callback to be called")
	}
	mu.Unlock()
}

func TestCoordinatorRunState(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{Queue: q})

	ctx := context.Background()
	c.Start(ctx)
	defer c.Stop()

	c.SubmitRun(ctx, "run-123", []queue.Task{})

	c.TaskCompleted("run-123")
	c.TaskCompleted("run-123")

	state, _ := c.GetRunState("run-123")
	if state.TasksDone != 2 {
		t.Errorf("expected 2 done tasks, got %d", state.TasksDone)
	}
}

func TestCoordinatorEnqueueTask(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{Queue: q})

	ctx := context.Background()
	c.Start(ctx)
	defer c.Stop()

	c.SubmitRun(ctx, "run-123", []queue.Task{})

	task := queue.NewTask(queue.TaskTypeCustom, json.RawMessage(`{}`))
	err := c.EnqueueTask(ctx, "run-123", task)
	if err != nil {
		t.Fatalf("enqueue task failed: %v", err)
	}

	state, _ := c.GetRunState("run-123")
	if state.TasksPending != 1 {
		t.Errorf("expected 1 pending task, got %d", state.TasksPending)
	}
}

func TestCoordinatorActiveRuns(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{Queue: q})

	ctx := context.Background()
	c.Start(ctx)
	defer c.Stop()

	c.SubmitRun(ctx, "run-1", []queue.Task{})
	c.SubmitRun(ctx, "run-2", []queue.Task{})

	if c.ActiveRuns() != 2 {
		t.Errorf("expected 2 active runs, got %d", c.ActiveRuns())
	}

	c.CompleteRun("run-1", nil)

	if c.ActiveRuns() != 1 {
		t.Errorf("expected 1 active run, got %d", c.ActiveRuns())
	}
}

func TestCoordinatorMetrics(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{
		Queue:   q,
		Workers: 2,
	})

	metrics := c.Metrics()
	if metrics.WorkerCount != 2 {
		t.Errorf("expected 2 workers, got %d", metrics.WorkerCount)
	}
}

func TestCoordinatorAddWorker(t *testing.T) {
	q := queue.NewMemoryQueue()
	c := NewCoordinator(CoordinatorConfig{Queue: q, Workers: 1})

	if c.WorkerCount() != 1 {
		t.Errorf("expected 1 worker, got %d", c.WorkerCount())
	}

	w := NewWorker(WorkerConfig{Queue: q})
	c.AddWorker(w)

	if c.WorkerCount() != 2 {
		t.Errorf("expected 2 workers, got %d", c.WorkerCount())
	}
}

func TestRunStatuses(t *testing.T) {
	statuses := []RunStatus{
		RunStatusPending,
		RunStatusRunning,
		RunStatusCompleted,
		RunStatusFailed,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Error("run status should not be empty")
		}
	}
}

func TestWorkerMetricsZero(t *testing.T) {
	m := &WorkerMetrics{}

	if m.AverageTaskDuration() != 0 {
		t.Error("expected zero average duration for empty metrics")
	}
	if m.SuccessRate() != 0 {
		t.Error("expected zero success rate for empty metrics")
	}
}

func TestCoordinatorMetricsZero(t *testing.T) {
	m := &CoordinatorMetrics{}

	if m.AverageTaskDuration() != 0 {
		t.Error("expected zero average duration for empty metrics")
	}
	if m.SuccessRate() != 0 {
		t.Error("expected zero success rate for empty metrics")
	}
}
