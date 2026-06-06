package memory_test

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/suggestion"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

func TestNewSuggestionStore(t *testing.T) {
	t.Parallel()

	store := memory.NewSuggestionStore()
	if store == nil {
		t.Fatal("NewSuggestionStore() returned nil")
	}
}

func TestSuggestionStore_Save(t *testing.T) {
	t.Parallel()

	t.Run("saves valid suggestion", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		sug := &suggestion.Suggestion{
			ID:         "sug-1",
			Type:       suggestion.SuggestionTypeAddEligibility,
			Title:      "Test Suggestion",
			Confidence: 0.8,
			Impact:     suggestion.ImpactLevelMedium,
			Status:     suggestion.SuggestionStatusPending,
			CreatedAt:  time.Now(),
		}

		err := store.Save(ctx, sug)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		sug := &suggestion.Suggestion{
			ID:    "",
			Type:  suggestion.SuggestionTypeAddEligibility,
			Title: "Test Suggestion",
		}

		err := store.Save(ctx, sug)
		if err != suggestion.ErrInvalidSuggestion {
			t.Errorf("Save() error = %v, want ErrInvalidSuggestion", err)
		}
	})

	t.Run("returns error for duplicate ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		sug := &suggestion.Suggestion{
			ID:    "sug-1",
			Type:  suggestion.SuggestionTypeAddEligibility,
			Title: "Test Suggestion",
		}

		store.Save(ctx, sug)
		err := store.Save(ctx, sug)
		if err != suggestion.ErrSuggestionExists {
			t.Errorf("Save() error = %v, want ErrSuggestionExists", err)
		}
	})
}

func TestSuggestionStore_Get(t *testing.T) {
	t.Parallel()

	t.Run("gets existing suggestion", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		sug := &suggestion.Suggestion{
			ID:         "sug-1",
			Type:       suggestion.SuggestionTypeAddEligibility,
			Title:      "Test Suggestion",
			Confidence: 0.8,
		}

		store.Save(ctx, sug)

		result, err := store.Get(ctx, "sug-1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if result.ID != "sug-1" {
			t.Errorf("Get() ID = %s, want sug-1", result.ID)
		}
		if result.Confidence != 0.8 {
			t.Errorf("Get() Confidence = %f, want 0.8", result.Confidence)
		}
	})

	t.Run("returns error for non-existent suggestion", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		_, err := store.Get(ctx, "nonexistent")
		if err != suggestion.ErrSuggestionNotFound {
			t.Errorf("Get() error = %v, want ErrSuggestionNotFound", err)
		}
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		sug := &suggestion.Suggestion{
			ID:    "sug-1",
			Title: "Original",
		}
		store.Save(ctx, sug)

		result, _ := store.Get(ctx, "sug-1")
		result.Title = "Modified"

		result2, _ := store.Get(ctx, "sug-1")
		if result2.Title != "Original" {
			t.Error("Get() should return a copy, not reference")
		}
	})
}

func TestSuggestionStore_List(t *testing.T) {
	t.Parallel()

	t.Run("lists all suggestions", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &suggestion.Suggestion{ID: "s1", CreatedAt: now})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", CreatedAt: now.Add(time.Second)})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", CreatedAt: now.Add(2 * time.Second)})

		results, err := store.List(ctx, suggestion.ListFilter{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 3 {
			t.Errorf("List() count = %d, want 3", len(results))
		}
	})

	t.Run("filters by type", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Type: suggestion.SuggestionTypeAddEligibility})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Type: suggestion.SuggestionTypeRemoveEligibility})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Type: suggestion.SuggestionTypeAddEligibility})

		results, err := store.List(ctx, suggestion.ListFilter{
			Types: []suggestion.SuggestionType{suggestion.SuggestionTypeAddEligibility},
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Status: suggestion.SuggestionStatusPending})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Status: suggestion.SuggestionStatusAccepted})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Status: suggestion.SuggestionStatusPending})

		results, err := store.List(ctx, suggestion.ListFilter{
			Status: []suggestion.SuggestionStatus{suggestion.SuggestionStatusPending},
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

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Confidence: 0.3})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Confidence: 0.7})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Confidence: 0.9})

		results, err := store.List(ctx, suggestion.ListFilter{MinConfidence: 0.5})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by impact", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Impact: suggestion.ImpactLevelLow})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Impact: suggestion.ImpactLevelMedium})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Impact: suggestion.ImpactLevelHigh})

		results, err := store.List(ctx, suggestion.ListFilter{
			Impact: []suggestion.ImpactLevel{suggestion.ImpactLevelMedium, suggestion.ImpactLevelHigh},
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by pattern ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", PatternIDs: []string{"p1", "p2"}})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", PatternIDs: []string{"p2", "p3"}})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", PatternIDs: []string{"p3"}})

		results, err := store.List(ctx, suggestion.ListFilter{PatternID: "p2"})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() count = %d, want 2", len(results))
		}
	})

	t.Run("filters by time range", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &suggestion.Suggestion{ID: "s1", CreatedAt: now.Add(-2 * time.Hour)})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", CreatedAt: now.Add(-30 * time.Minute)})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", CreatedAt: now.Add(time.Hour)})

		results, err := store.List(ctx, suggestion.ListFilter{
			FromTime: now.Add(-1 * time.Hour),
			ToTime:   now,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 1 {
			t.Errorf("List() count = %d, want 1", len(results))
		}
	})

	t.Run("applies offset and limit", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		for i := 0; i < 10; i++ {
			store.Save(ctx, &suggestion.Suggestion{ID: string(rune('a' + i))})
		}

		results, err := store.List(ctx, suggestion.ListFilter{Offset: 3, Limit: 4})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 4 {
			t.Errorf("List() count = %d, want 4", len(results))
		}
	})

	t.Run("returns empty for large offset", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1"})

		results, err := store.List(ctx, suggestion.ListFilter{Offset: 100})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("List() count = %d, want 0", len(results))
		}
	})

	t.Run("sorts by created at", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		now := time.Now()
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", CreatedAt: now.Add(2 * time.Hour)})
		store.Save(ctx, &suggestion.Suggestion{ID: "s1", CreatedAt: now})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", CreatedAt: now.Add(time.Hour)})

		results, err := store.List(ctx, suggestion.ListFilter{OrderBy: suggestion.OrderByCreatedAt})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].ID != "s1" || results[1].ID != "s2" || results[2].ID != "s3" {
			t.Error("List() did not sort by created at correctly")
		}
	})

	t.Run("sorts by confidence", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Confidence: 0.9})
		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Confidence: 0.3})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Confidence: 0.6})

		results, err := store.List(ctx, suggestion.ListFilter{OrderBy: suggestion.OrderByConfidence})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].Confidence != 0.3 || results[1].Confidence != 0.6 || results[2].Confidence != 0.9 {
			t.Error("List() did not sort by confidence correctly")
		}
	})

	t.Run("sorts by impact", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Impact: suggestion.ImpactLevelHigh})
		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Impact: suggestion.ImpactLevelLow})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Impact: suggestion.ImpactLevelMedium})

		results, err := store.List(ctx, suggestion.ListFilter{OrderBy: suggestion.OrderByImpact})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].Impact != suggestion.ImpactLevelLow ||
			results[1].Impact != suggestion.ImpactLevelMedium ||
			results[2].Impact != suggestion.ImpactLevelHigh {
			t.Error("List() did not sort by impact correctly")
		}
	})

	t.Run("sorts by status", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s4", Status: suggestion.SuggestionStatusSuperseded})
		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Status: suggestion.SuggestionStatusPending})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Status: suggestion.SuggestionStatusRejected})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Status: suggestion.SuggestionStatusAccepted})

		results, err := store.List(ctx, suggestion.ListFilter{OrderBy: suggestion.OrderByStatus})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].Status != suggestion.SuggestionStatusPending ||
			results[1].Status != suggestion.SuggestionStatusAccepted ||
			results[2].Status != suggestion.SuggestionStatusRejected ||
			results[3].Status != suggestion.SuggestionStatusSuperseded {
			t.Error("List() did not sort by status correctly")
		}
	})

	t.Run("sorts descending", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Confidence: 0.3})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Confidence: 0.6})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Confidence: 0.9})

		results, err := store.List(ctx, suggestion.ListFilter{OrderBy: suggestion.OrderByConfidence, Descending: true})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if results[0].Confidence != 0.9 || results[1].Confidence != 0.6 || results[2].Confidence != 0.3 {
			t.Error("List() did not sort descending correctly")
		}
	})
}

func TestSuggestionStore_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing suggestion", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1"})

		err := store.Delete(ctx, "s1")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err = store.Get(ctx, "s1")
		if err != suggestion.ErrSuggestionNotFound {
			t.Error("Suggestion should be deleted")
		}
	})

	t.Run("returns error for non-existent suggestion", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		err := store.Delete(ctx, "nonexistent")
		if err != suggestion.ErrSuggestionNotFound {
			t.Errorf("Delete() error = %v, want ErrSuggestionNotFound", err)
		}
	})
}

func TestSuggestionStore_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates existing suggestion", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Title: "Original"})

		err := store.Update(ctx, &suggestion.Suggestion{ID: "s1", Title: "Updated"})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		result, _ := store.Get(ctx, "s1")
		if result.Title != "Updated" {
			t.Errorf("Update() Title = %s, want Updated", result.Title)
		}
	})

	t.Run("returns error for empty ID", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		err := store.Update(ctx, &suggestion.Suggestion{ID: ""})
		if err != suggestion.ErrInvalidSuggestion {
			t.Errorf("Update() error = %v, want ErrInvalidSuggestion", err)
		}
	})

	t.Run("returns error for non-existent suggestion", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		err := store.Update(ctx, &suggestion.Suggestion{ID: "nonexistent"})
		if err != suggestion.ErrSuggestionNotFound {
			t.Errorf("Update() error = %v, want ErrSuggestionNotFound", err)
		}
	})
}

func TestSuggestionStore_Count(t *testing.T) {
	t.Parallel()

	t.Run("counts all suggestions", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1"})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2"})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3"})

		count, err := store.Count(ctx, suggestion.ListFilter{})
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if count != 3 {
			t.Errorf("Count() = %d, want 3", count)
		}
	})

	t.Run("counts with filter", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Status: suggestion.SuggestionStatusPending})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Status: suggestion.SuggestionStatusAccepted})
		store.Save(ctx, &suggestion.Suggestion{ID: "s3", Status: suggestion.SuggestionStatusPending})

		count, err := store.Count(ctx, suggestion.ListFilter{
			Status: []suggestion.SuggestionStatus{suggestion.SuggestionStatusPending},
		})
		if err != nil {
			t.Fatalf("Count() error = %v", err)
		}
		if count != 2 {
			t.Errorf("Count() = %d, want 2", count)
		}
	})
}

func TestSuggestionStore_Summarize(t *testing.T) {
	t.Parallel()

	t.Run("summarizes suggestions", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{
			ID:         "s1",
			Type:       suggestion.SuggestionTypeAddEligibility,
			Status:     suggestion.SuggestionStatusPending,
			Impact:     suggestion.ImpactLevelLow,
			Confidence: 0.6,
		})
		store.Save(ctx, &suggestion.Suggestion{
			ID:         "s2",
			Type:       suggestion.SuggestionTypeAddEligibility,
			Status:     suggestion.SuggestionStatusAccepted,
			Impact:     suggestion.ImpactLevelMedium,
			Confidence: 0.8,
		})
		store.Save(ctx, &suggestion.Suggestion{
			ID:         "s3",
			Type:       suggestion.SuggestionTypeRemoveEligibility,
			Status:     suggestion.SuggestionStatusPending,
			Impact:     suggestion.ImpactLevelHigh,
			Confidence: 0.7,
		})

		summary, err := store.Summarize(ctx, suggestion.ListFilter{})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalSuggestions != 3 {
			t.Errorf("Summarize() TotalSuggestions = %d, want 3", summary.TotalSuggestions)
		}
		if summary.ByType[suggestion.SuggestionTypeAddEligibility] != 2 {
			t.Errorf("Summarize() ByType[AddEligibility] = %d, want 2", summary.ByType[suggestion.SuggestionTypeAddEligibility])
		}
		if summary.ByStatus[suggestion.SuggestionStatusPending] != 2 {
			t.Errorf("Summarize() ByStatus[Pending] = %d, want 2", summary.ByStatus[suggestion.SuggestionStatusPending])
		}
		if summary.ByImpact[suggestion.ImpactLevelMedium] != 1 {
			t.Errorf("Summarize() ByImpact[Medium] = %d, want 1", summary.ByImpact[suggestion.ImpactLevelMedium])
		}
		// Average confidence: (0.6 + 0.8 + 0.7) / 3 = 0.7
		if summary.AverageConfidence < 0.69 || summary.AverageConfidence > 0.71 {
			t.Errorf("Summarize() AverageConfidence = %f, want ~0.7", summary.AverageConfidence)
		}
	})

	t.Run("summarizes with filter", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		store.Save(ctx, &suggestion.Suggestion{ID: "s1", Status: suggestion.SuggestionStatusPending, Confidence: 0.8})
		store.Save(ctx, &suggestion.Suggestion{ID: "s2", Status: suggestion.SuggestionStatusAccepted, Confidence: 0.3})

		summary, err := store.Summarize(ctx, suggestion.ListFilter{
			Status: []suggestion.SuggestionStatus{suggestion.SuggestionStatusPending},
		})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalSuggestions != 1 {
			t.Errorf("Summarize() TotalSuggestions = %d, want 1", summary.TotalSuggestions)
		}
	})

	t.Run("handles empty store", func(t *testing.T) {
		t.Parallel()

		store := memory.NewSuggestionStore()
		ctx := context.Background()

		summary, err := store.Summarize(ctx, suggestion.ListFilter{})
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if summary.TotalSuggestions != 0 {
			t.Errorf("Summarize() TotalSuggestions = %d, want 0", summary.TotalSuggestions)
		}
		if summary.AverageConfidence != 0 {
			t.Errorf("Summarize() AverageConfidence = %f, want 0", summary.AverageConfidence)
		}
	})
}
