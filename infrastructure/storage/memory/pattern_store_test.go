package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/pattern"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

func TestNewPatternStore(t *testing.T) {
	t.Parallel()

	store := memory.NewPatternStore()
	if store == nil {
		t.Fatal("NewPatternStore() returned nil")
	}
}

func TestPatternStore_Save(t *testing.T) {
	t.Parallel()

	t.Run("saves valid pattern", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		p := &pattern.Pattern{
			ID:         "pattern-1",
			Type:       pattern.PatternTypeToolSequence,
			Name:       "Test Pattern",
			Confidence: 0.8,
			Frequency:  5,
			FirstSeen:  time.Now(),
			LastSeen:   time.Now(),
		}

		err := store.Save(ctx, p)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		p := &pattern.Pattern{
			ID:   "",
			Type: pattern.PatternTypeToolSequence,
			Name: "Test Pattern",
		}

		err := store.Save(ctx, p)
		if err != pattern.ErrInvalidPattern {
			t.Errorf("Save() error = %v, want ErrInvalidPattern", err)
		}
	})

	t.Run("returns error for duplicate ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		p := &pattern.Pattern{
			ID:   "pattern-1",
			Type: pattern.PatternTypeToolSequence,
			Name: "Test Pattern",
		}

		store.Save(ctx, p)
		err := store.Save(ctx, p)
		if err != pattern.ErrPatternExists {
			t.Errorf("Save() error = %v, want ErrPatternExists", err)
		}
	})
}

func TestPatternStore_Get(t *testing.T) {
	t.Parallel()

	t.Run("gets existing pattern", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		p := &pattern.Pattern{
			ID:         "pattern-1",
			Type:       pattern.PatternTypeToolSequence,
			Name:       "Test Pattern",
			Confidence: 0.8,
		}

		store.Save(ctx, p)

		result, err := store.Get(ctx, "pattern-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if result.ID != "pattern-1" {
			t.Errorf("Get() ID = %s, want pattern-1", result.ID)
		}
		if result.Confidence != 0.8 {
			t.Errorf("Get() Confidence = %f, want 0.8", result.Confidence)
		}
	})

	t.Run("returns error for non-existent pattern", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		_, err := store.Get(ctx, "nonexistent")
		if err != pattern.ErrPatternNotFound {
			t.Errorf("Get() error = %v, want ErrPatternNotFound", err)
		}
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		p := &pattern.Pattern{
			ID:   "pattern-1",
			Name: "Original",
		}
		store.Save(ctx, p)

		result, _ := store.Get(ctx, "pattern-1")
		result.Name = "Modified"

		result2, _ := store.Get(ctx, "pattern-1")
		if result2.Name != "Original" {
			t.Error("Get() should return a copy, not reference")
		}
	})
}

func TestPatternStore_List(t *testing.T) {
	t.Parallel()

	t.Run("lists all patterns", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &pattern.Pattern{ID: "p1", Type: pattern.PatternTypeToolSequence, FirstSeen: now})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Type: pattern.PatternTypeStateLoop, FirstSeen: now.Add(time.Second)})
		store.Save(ctx, &pattern.Pattern{ID: "p3", Type: pattern.PatternTypeToolSequence, FirstSeen: now.Add(2 * time.Second)})

		results, err := store.List(ctx, pattern.ListFilter{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 3 {
			t.Errorf("List() count = %d, want 3", len(results))
		}
	})

	t.Run("filters by type", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", Type: pattern.PatternTypeToolSequence})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Type: pattern.PatternTypeStateLoop})
		store.Save(ctx, &pattern.Pattern{ID: "p3", Type: pattern.PatternTypeToolSequence})

		results, err := store.List(ctx, pattern.ListFilter{
			Types: []pattern.PatternType{pattern.PatternTypeToolSequence},
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by minimum confidence", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", Confidence: 0.3})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Confidence: 0.7})
		store.Save(ctx, &pattern.Pattern{ID: "p3", Confidence: 0.9})

		results, err := store.List(ctx, pattern.ListFilter{MinConfidence: 0.5})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by minimum frequency", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", Frequency: 2})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Frequency: 5})
		store.Save(ctx, &pattern.Pattern{ID: "p3", Frequency: 10})

		results, err := store.List(ctx, pattern.ListFilter{MinFrequency: 5})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by time range", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		now := time.Now()
		// FromTime filters: FirstSeen >= FromTime
		// ToTime filters: LastSeen <= ToTime
		store.Save(ctx, &pattern.Pattern{ID: "p1", FirstSeen: now.Add(-2 * time.Hour), LastSeen: now.Add(-90 * time.Minute)})    // FirstSeen before FromTime - filtered out
		store.Save(ctx, &pattern.Pattern{ID: "p2", FirstSeen: now.Add(-30 * time.Minute), LastSeen: now})                        // Matches: FirstSeen >= FromTime, LastSeen <= ToTime
		store.Save(ctx, &pattern.Pattern{ID: "p3", FirstSeen: now.Add(-45 * time.Minute), LastSeen: now.Add(-15 * time.Minute)}) // Matches both

		results, err := store.List(ctx, pattern.ListFilter{
			FromTime: now.Add(-1 * time.Hour),
			ToTime:   now.Add(30 * time.Minute),
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by run ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", RunIDs: []string{"run-1", "run-2"}})
		store.Save(ctx, &pattern.Pattern{ID: "p2", RunIDs: []string{"run-2", "run-3"}})
		store.Save(ctx, &pattern.Pattern{ID: "p3", RunIDs: []string{"run-3"}})

		results, err := store.List(ctx, pattern.ListFilter{RunID: "run-2"})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("applies offset and limit", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		for i := 0; i < 10; i++ {
			store.Save(ctx, &pattern.Pattern{ID: string(rune('a' + i))})
		}

		results, err := store.List(ctx, pattern.ListFilter{Offset: 3, Limit: 4})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 4 {
			t.Errorf("List() count = %d, want 4", len(results))
		}
	})

	t.Run("returns empty for large offset", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1"})

		results, err := store.List(ctx, pattern.ListFilter{Offset: 100})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("List() count = %d, want 0", len(results))
		}
	})

	t.Run("sorts by first seen", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &pattern.Pattern{ID: "p3", FirstSeen: now.Add(2 * time.Hour)})
		store.Save(ctx, &pattern.Pattern{ID: "p1", FirstSeen: now})
		store.Save(ctx, &pattern.Pattern{ID: "p2", FirstSeen: now.Add(time.Hour)})

		results, err := store.List(ctx, pattern.ListFilter{OrderBy: pattern.OrderByFirstSeen})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].ID != "p1" || results[1].ID != "p2" || results[2].ID != "p3" {
			t.Error("List() did not sort by first seen correctly")
		}
	})

	t.Run("sorts by last seen", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &pattern.Pattern{ID: "p3", LastSeen: now.Add(2 * time.Hour)})
		store.Save(ctx, &pattern.Pattern{ID: "p1", LastSeen: now})
		store.Save(ctx, &pattern.Pattern{ID: "p2", LastSeen: now.Add(time.Hour)})

		results, err := store.List(ctx, pattern.ListFilter{OrderBy: pattern.OrderByLastSeen})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].ID != "p1" || results[1].ID != "p2" || results[2].ID != "p3" {
			t.Error("List() did not sort by last seen correctly")
		}
	})

	t.Run("sorts by frequency", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p3", Frequency: 30})
		store.Save(ctx, &pattern.Pattern{ID: "p1", Frequency: 10})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Frequency: 20})

		results, err := store.List(ctx, pattern.ListFilter{OrderBy: pattern.OrderByFrequency})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].Frequency != 10 || results[1].Frequency != 20 || results[2].Frequency != 30 {
			t.Error("List() did not sort by frequency correctly")
		}
	})

	t.Run("sorts by confidence", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p3", Confidence: 0.9})
		store.Save(ctx, &pattern.Pattern{ID: "p1", Confidence: 0.3})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Confidence: 0.6})

		results, err := store.List(ctx, pattern.ListFilter{OrderBy: pattern.OrderByConfidence})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].Confidence != 0.3 || results[1].Confidence != 0.6 || results[2].Confidence != 0.9 {
			t.Error("List() did not sort by confidence correctly")
		}
	})

	t.Run("sorts descending", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", Frequency: 10})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Frequency: 20})
		store.Save(ctx, &pattern.Pattern{ID: "p3", Frequency: 30})

		results, err := store.List(ctx, pattern.ListFilter{OrderBy: pattern.OrderByFrequency, Descending: true})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].Frequency != 30 || results[1].Frequency != 20 || results[2].Frequency != 10 {
			t.Error("List() did not sort descending correctly")
		}
	})
}

func TestPatternStore_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing pattern", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1"})

		err := store.Delete(ctx, "p1")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err = store.Get(ctx, "p1")
		if err != pattern.ErrPatternNotFound {
			t.Error("Pattern should be deleted")
		}
	})

	t.Run("returns error for non-existent pattern", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		err := store.Delete(ctx, "nonexistent")
		if err != pattern.ErrPatternNotFound {
			t.Errorf("Delete() error = %v, want ErrPatternNotFound", err)
		}
	})
}

func TestPatternStore_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates existing pattern", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", Name: "Original"})

		err := store.Update(ctx, &pattern.Pattern{ID: "p1", Name: "Updated"})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		result, _ := store.Get(ctx, "p1")
		if result.Name != "Updated" {
			t.Errorf("Update() Name = %s, want Updated", result.Name)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		err := store.Update(ctx, &pattern.Pattern{ID: ""})
		if err != pattern.ErrInvalidPattern {
			t.Errorf("Update() error = %v, want ErrInvalidPattern", err)
		}
	})

	t.Run("returns error for non-existent pattern", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		err := store.Update(ctx, &pattern.Pattern{ID: "nonexistent"})
		if err != pattern.ErrPatternNotFound {
			t.Errorf("Update() error = %v, want ErrPatternNotFound", err)
		}
	})
}

func TestPatternStore_Count(t *testing.T) {
	t.Parallel()

	t.Run("counts all patterns", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1"})
		store.Save(ctx, &pattern.Pattern{ID: "p2"})
		store.Save(ctx, &pattern.Pattern{ID: "p3"})

		count, err := store.Count(ctx, pattern.ListFilter{})
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if count != 3 {
			t.Errorf("Count() = %d, want 3", count)
		}
	})

	t.Run("counts with filter", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", Type: pattern.PatternTypeToolSequence})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Type: pattern.PatternTypeStateLoop})
		store.Save(ctx, &pattern.Pattern{ID: "p3", Type: pattern.PatternTypeToolSequence})

		count, err := store.Count(ctx, pattern.ListFilter{
			Types: []pattern.PatternType{pattern.PatternTypeToolSequence},
		})
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if count != 2 {
			t.Errorf("Count() = %d, want 2", count)
		}
	})
}

func TestPatternStore_Summarize(t *testing.T) {
	t.Parallel()

	t.Run("summarizes patterns", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", Type: pattern.PatternTypeToolSequence, Confidence: 0.6, Frequency: 10})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Type: pattern.PatternTypeToolSequence, Confidence: 0.8, Frequency: 20})
		store.Save(ctx, &pattern.Pattern{ID: "p3", Type: pattern.PatternTypeStateLoop, Confidence: 0.7, Frequency: 15})

		summary, err := store.Summarize(ctx, pattern.ListFilter{})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalPatterns != 3 {
			t.Errorf("Summarize() TotalPatterns = %d, want 3", summary.TotalPatterns)
		}
		if summary.ByType[pattern.PatternTypeToolSequence] != 2 {
			t.Errorf("Summarize() ByType[ToolSequence] = %d, want 2", summary.ByType[pattern.PatternTypeToolSequence])
		}
		if summary.ByType[pattern.PatternTypeStateLoop] != 1 {
			t.Errorf("Summarize() ByType[StateLoop] = %d, want 1", summary.ByType[pattern.PatternTypeStateLoop])
		}
		// Average confidence: (0.6 + 0.8 + 0.7) / 3 = 0.7
		if summary.AverageConfidence < 0.69 || summary.AverageConfidence > 0.71 {
			t.Errorf("Summarize() AverageConfidence = %f, want ~0.7", summary.AverageConfidence)
		}
		// Average frequency: (10 + 20 + 15) / 3 = 15
		if summary.AverageFrequency != 15 {
			t.Errorf("Summarize() AverageFrequency = %f, want 15", summary.AverageFrequency)
		}
	})

	t.Run("summarizes with filter", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		store.Save(ctx, &pattern.Pattern{ID: "p1", Type: pattern.PatternTypeToolSequence, Confidence: 0.8})
		store.Save(ctx, &pattern.Pattern{ID: "p2", Type: pattern.PatternTypeStateLoop, Confidence: 0.3})

		summary, err := store.Summarize(ctx, pattern.ListFilter{MinConfidence: 0.5})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalPatterns != 1 {
			t.Errorf("Summarize() TotalPatterns = %d, want 1", summary.TotalPatterns)
		}
	})

	t.Run("handles empty store", func(t *testing.T) {
		t.Parallel()

		store := memory.NewPatternStore()
		ctx := context.Background()

		summary, err := store.Summarize(ctx, pattern.ListFilter{})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalPatterns != 0 {
			t.Errorf("Summarize() TotalPatterns = %d, want 0", summary.TotalPatterns)
		}
		if summary.AverageConfidence != 0 {
			t.Errorf("Summarize() AverageConfidence = %f, want 0", summary.AverageConfidence)
		}
	})
}
