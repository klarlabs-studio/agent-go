// Package scheduler provides task scheduling tools for agents.
package scheduler

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Scheduler manages scheduled tasks.
type Scheduler struct {
	mu    sync.RWMutex
	tasks map[string]*scheduledTask
}

type scheduledTask struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Cron      string    `json:"cron,omitempty"`
	Interval  int       `json:"interval_seconds,omitempty"`
	NextRun   time.Time `json:"next_run"`
	LastRun   time.Time `json:"last_run,omitempty"`
	RunCount  int       `json:"run_count"`
	Enabled   bool      `json:"enabled"`
	Data      any       `json:"data,omitempty"`
	MaxRuns   int       `json:"max_runs,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	cancel    context.CancelFunc
}

func (t *scheduledTask) isExpired() bool {
	if !t.ExpiresAt.IsZero() && time.Now().After(t.ExpiresAt) {
		return true
	}
	if t.MaxRuns > 0 && t.RunCount >= t.MaxRuns {
		return true
	}
	return false
}

var scheduler = &Scheduler{
	tasks: make(map[string]*scheduledTask),
}

// Pack returns the scheduler tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("scheduler").
		WithDescription("Task scheduling tools").
		AddTools(
			createTool(),
			cancelTool(),
			pauseTool(),
			resumeTool(),
			getTool(),
			listTool(),
			updateTool(),
			triggerTool(),
			nextRunTool(),
			historyTool(),
			clearTool(),
			statusTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func createTool() tool.Tool {
	return tool.NewBuilder("schedule_create").
		WithDescription("Create a scheduled task").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Interval int    `json:"interval_seconds"`
				Delay    int    `json:"delay_seconds,omitempty"`
				MaxRuns  int    `json:"max_runs,omitempty"`
				TTL      int    `json:"ttl_seconds,omitempty"`
				Data     any    `json:"data,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			now := time.Now()
			task := &scheduledTask{
				ID:        params.ID,
				Name:      params.Name,
				Interval:  params.Interval,
				Enabled:   true,
				Data:      params.Data,
				MaxRuns:   params.MaxRuns,
				CreatedAt: now,
			}

			if params.Delay > 0 {
				task.NextRun = now.Add(time.Duration(params.Delay) * time.Second)
			} else {
				task.NextRun = now.Add(time.Duration(params.Interval) * time.Second)
			}

			if params.TTL > 0 {
				task.ExpiresAt = now.Add(time.Duration(params.TTL) * time.Second)
			}

			scheduler.mu.Lock()
			if existing, ok := scheduler.tasks[params.ID]; ok {
				if existing.cancel != nil {
					existing.cancel()
				}
			}
			scheduler.tasks[params.ID] = task
			scheduler.mu.Unlock()

			result := map[string]any{
				"id":       task.ID,
				"name":     task.Name,
				"next_run": task.NextRun,
				"created":  true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cancelTool() tool.Tool {
	return tool.NewBuilder("schedule_cancel").
		WithDescription("Cancel a scheduled task").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.Lock()
			task, ok := scheduler.tasks[params.ID]
			if ok {
				if task.cancel != nil {
					task.cancel()
				}
				delete(scheduler.tasks, params.ID)
			}
			scheduler.mu.Unlock()

			result := map[string]any{
				"id":        params.ID,
				"cancelled": ok,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pauseTool() tool.Tool {
	return tool.NewBuilder("schedule_pause").
		WithDescription("Pause a scheduled task").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.Lock()
			task, ok := scheduler.tasks[params.ID]
			if ok {
				task.Enabled = false
			}
			scheduler.mu.Unlock()

			result := map[string]any{
				"id":     params.ID,
				"paused": ok,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func resumeTool() tool.Tool {
	return tool.NewBuilder("schedule_resume").
		WithDescription("Resume a paused scheduled task").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.Lock()
			task, ok := scheduler.tasks[params.ID]
			if ok {
				task.Enabled = true
				if task.NextRun.Before(time.Now()) {
					task.NextRun = time.Now().Add(time.Duration(task.Interval) * time.Second)
				}
			}
			scheduler.mu.Unlock()

			result := map[string]any{
				"id":      params.ID,
				"resumed": ok,
			}
			if ok {
				result["next_run"] = task.NextRun
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getTool() tool.Tool {
	return tool.NewBuilder("schedule_get").
		WithDescription("Get details of a scheduled task").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.RLock()
			task, ok := scheduler.tasks[params.ID]
			scheduler.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"id":    params.ID,
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"id":        task.ID,
				"name":      task.Name,
				"interval":  task.Interval,
				"next_run":  task.NextRun,
				"last_run":  task.LastRun,
				"run_count": task.RunCount,
				"enabled":   task.Enabled,
				"data":      task.Data,
				"expired":   task.isExpired(),
				"found":     true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("schedule_list").
		WithDescription("List all scheduled tasks").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				EnabledOnly bool `json:"enabled_only,omitempty"`
				Limit       int  `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.RLock()
			var tasks []map[string]any
			for _, task := range scheduler.tasks {
				if params.EnabledOnly && !task.Enabled {
					continue
				}
				if task.isExpired() {
					continue
				}
				tasks = append(tasks, map[string]any{
					"id":        task.ID,
					"name":      task.Name,
					"next_run":  task.NextRun,
					"enabled":   task.Enabled,
					"run_count": task.RunCount,
				})
				if params.Limit > 0 && len(tasks) >= params.Limit {
					break
				}
			}
			scheduler.mu.RUnlock()

			result := map[string]any{
				"tasks": tasks,
				"count": len(tasks),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func updateTool() tool.Tool {
	return tool.NewBuilder("schedule_update").
		WithDescription("Update a scheduled task").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID       string `json:"id"`
				Name     string `json:"name,omitempty"`
				Interval int    `json:"interval_seconds,omitempty"`
				Data     any    `json:"data,omitempty"`
				MaxRuns  int    `json:"max_runs,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.Lock()
			task, ok := scheduler.tasks[params.ID]
			if ok {
				if params.Name != "" {
					task.Name = params.Name
				}
				if params.Interval > 0 {
					task.Interval = params.Interval
					task.NextRun = time.Now().Add(time.Duration(params.Interval) * time.Second)
				}
				if params.Data != nil {
					task.Data = params.Data
				}
				if params.MaxRuns > 0 {
					task.MaxRuns = params.MaxRuns
				}
			}
			scheduler.mu.Unlock()

			result := map[string]any{
				"id":      params.ID,
				"updated": ok,
			}
			if ok {
				result["next_run"] = task.NextRun
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func triggerTool() tool.Tool {
	return tool.NewBuilder("schedule_trigger").
		WithDescription("Manually trigger a scheduled task").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID         string `json:"id"`
				Reschedule bool   `json:"reschedule,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.Lock()
			task, ok := scheduler.tasks[params.ID]
			if ok {
				task.LastRun = time.Now()
				task.RunCount++
				if params.Reschedule && task.Interval > 0 {
					task.NextRun = time.Now().Add(time.Duration(task.Interval) * time.Second)
				}
			}
			scheduler.mu.Unlock()

			if !ok {
				result := map[string]any{
					"id":        params.ID,
					"triggered": false,
					"error":     "task not found",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"id":        task.ID,
				"triggered": true,
				"run_count": task.RunCount,
				"data":      task.Data,
			}
			if params.Reschedule {
				result["next_run"] = task.NextRun
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func nextRunTool() tool.Tool {
	return tool.NewBuilder("schedule_next_run").
		WithDescription("Get next scheduled task to run").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			scheduler.mu.RLock()
			var nextTask *scheduledTask
			for _, task := range scheduler.tasks {
				if !task.Enabled || task.isExpired() {
					continue
				}
				if nextTask == nil || task.NextRun.Before(nextTask.NextRun) {
					nextTask = task
				}
			}
			scheduler.mu.RUnlock()

			if nextTask == nil {
				result := map[string]any{
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"found":    true,
				"id":       nextTask.ID,
				"name":     nextTask.Name,
				"next_run": nextTask.NextRun,
				"in":       time.Until(nextTask.NextRun).Seconds(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func historyTool() tool.Tool {
	return tool.NewBuilder("schedule_history").
		WithDescription("Get run history for a task").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.RLock()
			task, ok := scheduler.tasks[params.ID]
			scheduler.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"id":    params.ID,
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"id":         task.ID,
				"run_count":  task.RunCount,
				"last_run":   task.LastRun,
				"created_at": task.CreatedAt,
				"found":      true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func clearTool() tool.Tool {
	return tool.NewBuilder("schedule_clear").
		WithDescription("Clear all scheduled tasks").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ExpiredOnly bool `json:"expired_only,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			scheduler.mu.Lock()
			var cleared int
			if params.ExpiredOnly {
				for id, task := range scheduler.tasks {
					if task.isExpired() {
						if task.cancel != nil {
							task.cancel()
						}
						delete(scheduler.tasks, id)
						cleared++
					}
				}
			} else {
				cleared = len(scheduler.tasks)
				for _, task := range scheduler.tasks {
					if task.cancel != nil {
						task.cancel()
					}
				}
				scheduler.tasks = make(map[string]*scheduledTask)
			}
			scheduler.mu.Unlock()

			result := map[string]any{
				"cleared": cleared,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func statusTool() tool.Tool {
	return tool.NewBuilder("schedule_status").
		WithDescription("Get scheduler status").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			scheduler.mu.RLock()
			total := len(scheduler.tasks)
			var enabled, expired, paused int
			for _, task := range scheduler.tasks {
				switch {
				case task.isExpired():
					expired++
				case task.Enabled:
					enabled++
				default:
					paused++
				}
			}
			scheduler.mu.RUnlock()

			result := map[string]any{
				"total":   total,
				"enabled": enabled,
				"paused":  paused,
				"expired": expired,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
