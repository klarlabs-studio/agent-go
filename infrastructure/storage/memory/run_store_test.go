package memory_test

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/run"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

func TestNewRunStore(t *testing.T) {
	t.Parallel()

	store := memory.NewRunStore()
	if store == nil {
		t.Fatal("NewRunStore() returned nil")
	}
	if store.Len() != 0 {
		t.Errorf("Len() = %d, want 0 for new store", store.Len())
	}
}

func TestRunStore_Save(t *testing.T) {
	t.Parallel()

	t.Run("saves new run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		r := &agent.Run{
			ID:           "run-1",
			Goal:         "test goal",
			StartTime:    time.Now(),
			Status:       agent.RunStatusRunning,
			CurrentState: agent.StateIntake,
		}

		err := store.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		if store.Len() != 1 {
			t.Errorf("Len() = %d, want 1", store.Len())
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		r := &agent.Run{
			ID:   "",
			Goal: "test",
		}

		err := store.Save(ctx, r)
		if err != run.ErrInvalidRunID {
			t.Errorf("Save() error = %v, want ErrInvalidRunID", err)
		}
	})

	t.Run("returns error for duplicate ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		r := &agent.Run{ID: "run-1", Goal: "test"}
		store.Save(ctx, r)

		r2 := &agent.Run{ID: "run-1", Goal: "another"}
		err := store.Save(ctx, r2)
		if err != run.ErrRunExists {
			t.Errorf("Save() error = %v, want ErrRunExists", err)
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		r := &agent.Run{ID: "run-1", Goal: "test"}
		err := store.Save(ctx, r)
		if err == nil {
			t.Error("Save() should return error for cancelled context")
		}
	})
}

func TestRunStore_Get(t *testing.T) {
	t.Parallel()

	t.Run("retrieves existing run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		r := &agent.Run{ID: "run-1", Goal: "test goal"}
		store.Save(ctx, r)

		retrieved, err := store.Get(ctx, "run-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if retrieved.ID != "run-1" {
			t.Errorf("Get() ID = %s, want run-1", retrieved.ID)
		}
		if retrieved.Goal != "test goal" {
			t.Errorf("Get() Goal = %s, want 'test goal'", retrieved.Goal)
		}
	})

	t.Run("returns error for non-existent run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		_, err := store.Get(ctx, "nonexistent")
		if err != run.ErrRunNotFound {
			t.Errorf("Get() error = %v, want ErrRunNotFound", err)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		_, err := store.Get(ctx, "")
		if err != run.ErrInvalidRunID {
			t.Errorf("Get() error = %v, want ErrInvalidRunID", err)
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := store.Get(ctx, "run-1")
		if err == nil {
			t.Error("Get() should return error for cancelled context")
		}
	})
}

func TestRunStore_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates existing run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		r := &agent.Run{ID: "run-1", Goal: "original", Status: agent.RunStatusRunning}
		store.Save(ctx, r)

		r.Status = agent.RunStatusCompleted
		err := store.Update(ctx, r)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		updated, _ := store.Get(ctx, "run-1")
		if updated.Status != agent.RunStatusCompleted {
			t.Errorf("Status = %s, want completed", updated.Status)
		}
	})

	t.Run("returns error for non-existent run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		r := &agent.Run{ID: "nonexistent", Goal: "test"}
		err := store.Update(ctx, r)
		if err != run.ErrRunNotFound {
			t.Errorf("Update() error = %v, want ErrRunNotFound", err)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		r := &agent.Run{ID: "", Goal: "test"}
		err := store.Update(ctx, r)
		if err != run.ErrInvalidRunID {
			t.Errorf("Update() error = %v, want ErrInvalidRunID", err)
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		r := &agent.Run{ID: "run-1", Goal: "test"}
		err := store.Update(ctx, r)
		if err == nil {
			t.Error("Update() should return error for cancelled context")
		}
	})
}

func TestRunStore_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		r := &agent.Run{ID: "run-1", Goal: "test"}
		store.Save(ctx, r)

		err := store.Delete(ctx, "run-1")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err = store.Get(ctx, "run-1")
		if err != run.ErrRunNotFound {
			t.Error("Run should be deleted")
		}
	})

	t.Run("returns error for non-existent run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		err := store.Delete(ctx, "nonexistent")
		if err != run.ErrRunNotFound {
			t.Errorf("Delete() error = %v, want ErrRunNotFound", err)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		err := store.Delete(ctx, "")
		if err != run.ErrInvalidRunID {
			t.Errorf("Delete() error = %v, want ErrInvalidRunID", err)
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := store.Delete(ctx, "run-1")
		if err == nil {
			t.Error("Delete() should return error for cancelled context")
		}
	})
}

func TestRunStore_List(t *testing.T) {
	t.Parallel()

	t.Run("lists all runs without filter", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		store.Save(ctx, &agent.Run{ID: "run-1", Goal: "test 1", Status: agent.RunStatusRunning})
		store.Save(ctx, &agent.Run{ID: "run-2", Goal: "test 2", Status: agent.RunStatusCompleted})

		runs, err := store.List(ctx, run.ListFilter{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(runs) != 2 {
			t.Errorf("List() count = %d, want 2", len(runs))
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		store.Save(ctx, &agent.Run{ID: "run-1", Goal: "test", Status: agent.RunStatusRunning})
		store.Save(ctx, &agent.Run{ID: "run-2", Goal: "test", Status: agent.RunStatusCompleted})
		store.Save(ctx, &agent.Run{ID: "run-3", Goal: "test", Status: agent.RunStatusFailed})

		runs, err := store.List(ctx, run.ListFilter{
			Status: []agent.RunStatus{agent.RunStatusCompleted},
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(runs) != 1 {
			t.Errorf("List() count = %d, want 1", len(runs))
		}
	})

	t.Run("filters by state", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		store.Save(ctx, &agent.Run{ID: "run-1", Goal: "test", CurrentState: agent.StateIntake})
		store.Save(ctx, &agent.Run{ID: "run-2", Goal: "test", CurrentState: agent.StateExplore})

		runs, err := store.List(ctx, run.ListFilter{
			States: []agent.State{agent.StateIntake},
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(runs) != 1 {
			t.Errorf("List() count = %d, want 1", len(runs))
		}
	})

	t.Run("filters by time range", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &agent.Run{ID: "run-1", Goal: "test", StartTime: now.Add(-2 * time.Hour)})
		store.Save(ctx, &agent.Run{ID: "run-2", Goal: "test", StartTime: now.Add(-1 * time.Hour)})
		store.Save(ctx, &agent.Run{ID: "run-3", Goal: "test", StartTime: now})

		runs, err := store.List(ctx, run.ListFilter{
			FromTime: now.Add(-90 * time.Minute),
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(runs) != 2 {
			t.Errorf("List() count = %d, want 2", len(runs))
		}
	})

	t.Run("filters by goal pattern", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		store.Save(ctx, &agent.Run{ID: "run-1", Goal: "process files"})
		store.Save(ctx, &agent.Run{ID: "run-2", Goal: "analyze data"})
		store.Save(ctx, &agent.Run{ID: "run-3", Goal: "process data"})

		runs, err := store.List(ctx, run.ListFilter{
			GoalPattern: "process",
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(runs) != 2 {
			t.Errorf("List() count = %d, want 2", len(runs))
		}
	})

	t.Run("applies offset and limit", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			store.Save(ctx, &agent.Run{ID: "run-" + string(rune('0'+i)), Goal: "test"})
		}

		runs, err := store.List(ctx, run.ListFilter{
			Offset: 2,
			Limit:  2,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(runs) != 2 {
			t.Errorf("List() count = %d, want 2", len(runs))
		}
	})

	t.Run("returns empty for large offset", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		store.Save(ctx, &agent.Run{ID: "run-1", Goal: "test"})

		runs, err := store.List(ctx, run.ListFilter{
			Offset: 100,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(runs) != 0 {
			t.Errorf("List() count = %d, want 0", len(runs))
		}
	})

	t.Run("sorts by different fields", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &agent.Run{ID: "run-b", Goal: "test", StartTime: now.Add(-1 * time.Hour)})
		store.Save(ctx, &agent.Run{ID: "run-a", Goal: "test", StartTime: now})

		// Sort by ID ascending
		runs, _ := store.List(ctx, run.ListFilter{
			OrderBy: run.OrderByID,
		})
		if len(runs) == 2 && runs[0].ID != "run-a" {
			t.Errorf("First run ID = %s, want run-a", runs[0].ID)
		}

		// Sort by ID descending
		runs, _ = store.List(ctx, run.ListFilter{
			OrderBy:    run.OrderByID,
			Descending: true,
		})
		if len(runs) == 2 && runs[0].ID != "run-b" {
			t.Errorf("First run ID = %s, want run-b", runs[0].ID)
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewRunStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := store.List(ctx, run.ListFilter{})
		if err == nil {
			t.Error("List() should return error for cancelled context")
		}
	})
}

func TestRunStore_Count(t *testing.T) {
	t.Parallel()

	store := memory.NewRunStore()
	ctx := context.Background()

	store.Save(ctx, &agent.Run{ID: "run-1", Status: agent.RunStatusRunning})
	store.Save(ctx, &agent.Run{ID: "run-2", Status: agent.RunStatusCompleted})

	count, err := store.Count(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 2 {
		t.Errorf("Count() = %d, want 2", count)
	}

	// Count with filter
	count, _ = store.Count(ctx, run.ListFilter{
		Status: []agent.RunStatus{agent.RunStatusRunning},
	})
	if count != 1 {
		t.Errorf("Count() with filter = %d, want 1", count)
	}
}

func TestRunStore_Summary(t *testing.T) {
	t.Parallel()

	store := memory.NewRunStore()
	ctx := context.Background()

	now := time.Now()
	store.Save(ctx, &agent.Run{
		ID:        "run-1",
		Status:    agent.RunStatusRunning,
		StartTime: now,
	})
	store.Save(ctx, &agent.Run{
		ID:        "run-2",
		Status:    agent.RunStatusCompleted,
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
	})
	store.Save(ctx, &agent.Run{
		ID:        "run-3",
		Status:    agent.RunStatusFailed,
		StartTime: now.Add(-30 * time.Minute),
		EndTime:   now,
	})

	summary, err := store.Summary(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}

	if summary.TotalRuns != 3 {
		t.Errorf("TotalRuns = %d, want 3", summary.TotalRuns)
	}
	if summary.RunningRuns != 1 {
		t.Errorf("RunningRuns = %d, want 1", summary.RunningRuns)
	}
	if summary.CompletedRuns != 1 {
		t.Errorf("CompletedRuns = %d, want 1", summary.CompletedRuns)
	}
	if summary.FailedRuns != 1 {
		t.Errorf("FailedRuns = %d, want 1", summary.FailedRuns)
	}
}

func TestRunStore_Clear(t *testing.T) {
	t.Parallel()

	store := memory.NewRunStore()
	ctx := context.Background()

	store.Save(ctx, &agent.Run{ID: "run-1", Goal: "test"})
	store.Save(ctx, &agent.Run{ID: "run-2", Goal: "test"})

	store.Clear()

	if store.Len() != 0 {
		t.Errorf("Len() = %d, want 0 after Clear()", store.Len())
	}
}
