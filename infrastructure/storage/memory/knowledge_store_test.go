package memory

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/knowledge"
)

func TestKnowledgeStore_Upsert(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	v := &knowledge.Vector{
		ID:        "doc-1",
		Embedding: []float32{1.0, 0.0, 0.0},
		Text:      "Hello world",
		Metadata:  map[string]string{"source": "test"},
	}

	err := store.Upsert(ctx, v)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Verify it was stored
	got, err := store.Get(ctx, "doc-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.ID != v.ID {
		t.Errorf("expected ID %s, got %s", v.ID, got.ID)
	}
	if got.Text != v.Text {
		t.Errorf("expected Text %s, got %s", v.Text, got.Text)
	}
}

func TestKnowledgeStore_Upsert_AutoDetectDimension(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	// First vector sets dimension
	v1 := &knowledge.Vector{
		ID:        "doc-1",
		Embedding: []float32{1.0, 0.0, 0.0},
		Text:      "First",
	}
	if err := store.Upsert(ctx, v1); err != nil {
		t.Fatalf("First Upsert failed: %v", err)
	}

	stats, _ := store.Stats(ctx)
	if stats.Dimension != 3 {
		t.Errorf("expected dimension 3, got %d", stats.Dimension)
	}

	// Second vector with same dimension succeeds
	v2 := &knowledge.Vector{
		ID:        "doc-2",
		Embedding: []float32{0.0, 1.0, 0.0},
		Text:      "Second",
	}
	if err := store.Upsert(ctx, v2); err != nil {
		t.Fatalf("Second Upsert failed: %v", err)
	}
}

func TestKnowledgeStore_Upsert_DimensionMismatch(t *testing.T) {
	store := NewKnowledgeStore(3) // Explicitly set dimension
	ctx := context.Background()

	v := &knowledge.Vector{
		ID:        "doc-1",
		Embedding: []float32{1.0, 0.0}, // Wrong dimension
		Text:      "Test",
	}

	err := store.Upsert(ctx, v)
	if err != knowledge.ErrDimensionMismatch {
		t.Errorf("expected ErrDimensionMismatch, got %v", err)
	}
}

func TestKnowledgeStore_Upsert_InvalidID(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	v := &knowledge.Vector{
		ID:        "",
		Embedding: []float32{1.0},
	}

	err := store.Upsert(ctx, v)
	if err != knowledge.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestKnowledgeStore_Upsert_InvalidEmbedding(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	v := &knowledge.Vector{
		ID:        "doc-1",
		Embedding: nil,
	}

	err := store.Upsert(ctx, v)
	if err != knowledge.ErrInvalidEmbedding {
		t.Errorf("expected ErrInvalidEmbedding, got %v", err)
	}
}

func TestKnowledgeStore_Get(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	v := &knowledge.Vector{
		ID:        "doc-1",
		Embedding: []float32{1.0, 0.0, 0.0},
		Text:      "Hello world",
		Metadata:  map[string]string{"key": "value"},
	}
	_ = store.Upsert(ctx, v)

	got, err := store.Get(ctx, "doc-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Verify deep copy - modifying retrieved vector doesn't affect stored
	got.Text = "Modified"
	original, _ := store.Get(ctx, "doc-1")
	if original.Text == "Modified" {
		t.Error("Get should return deep copy")
	}
}

func TestKnowledgeStore_Get_NotFound(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err != knowledge.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestKnowledgeStore_Delete(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	v := &knowledge.Vector{
		ID:        "doc-1",
		Embedding: []float32{1.0, 0.0, 0.0},
		Text:      "To be deleted",
	}
	_ = store.Upsert(ctx, v)

	err := store.Delete(ctx, "doc-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(ctx, "doc-1")
	if err != knowledge.ErrNotFound {
		t.Error("expected vector to be deleted")
	}
}

func TestKnowledgeStore_Delete_NotFound(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err != knowledge.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestKnowledgeStore_Search_CosineSimilarity(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	// Create orthogonal vectors
	vectors := []*knowledge.Vector{
		{ID: "x", Embedding: []float32{1.0, 0.0, 0.0}, Text: "X axis"},
		{ID: "y", Embedding: []float32{0.0, 1.0, 0.0}, Text: "Y axis"},
		{ID: "z", Embedding: []float32{0.0, 0.0, 1.0}, Text: "Z axis"},
	}

	for _, v := range vectors {
		_ = store.Upsert(ctx, v)
	}

	// Search for X direction
	results, err := store.Search(ctx, []float32{1.0, 0.0, 0.0}, 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// X should be most similar (score = 1.0)
	if results[0].ID != "x" {
		t.Errorf("expected first result to be 'x', got '%s'", results[0].ID)
	}
	if math.Abs(float64(results[0].Score)-1.0) > 0.0001 {
		t.Errorf("expected score ~1.0, got %f", results[0].Score)
	}

	// Y and Z should have score ~0 (orthogonal)
	if math.Abs(float64(results[1].Score)) > 0.0001 {
		t.Errorf("expected orthogonal vectors to have score ~0, got %f", results[1].Score)
	}
}

func TestKnowledgeStore_Search_TopK(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	// Add 10 vectors
	for i := 0; i < 10; i++ {
		v := &knowledge.Vector{
			ID:        "doc-" + string(rune('0'+i)),
			Embedding: []float32{float32(i), 0, 0},
			Text:      "Doc",
		}
		_ = store.Upsert(ctx, v)
	}

	// Request top 3
	results, err := store.Search(ctx, []float32{9, 0, 0}, 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestKnowledgeStore_Search_EmptyStore(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	results, err := store.Search(ctx, []float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results from empty store, got %d", len(results))
	}
}

func TestKnowledgeStore_List_WithFilters(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	now := time.Now()

	vectors := []*knowledge.Vector{
		{ID: "doc-a", Embedding: []float32{1}, Text: "A", Metadata: map[string]string{"type": "article"}, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "doc-b", Embedding: []float32{1}, Text: "B", Metadata: map[string]string{"type": "article"}, CreatedAt: now.Add(-1 * time.Hour)},
		{ID: "note-c", Embedding: []float32{1}, Text: "C", Metadata: map[string]string{"type": "note"}, CreatedAt: now},
	}

	for _, v := range vectors {
		_ = store.Upsert(ctx, v)
	}

	// Filter by ID prefix
	results, _ := store.List(ctx, knowledge.ListFilter{IDPrefix: "doc-"})
	if len(results) != 2 {
		t.Errorf("expected 2 results with prefix 'doc-', got %d", len(results))
	}

	// Filter by metadata
	results, _ = store.List(ctx, knowledge.ListFilter{Metadata: map[string]string{"type": "article"}})
	if len(results) != 2 {
		t.Errorf("expected 2 articles, got %d", len(results))
	}

	// Filter by time
	results, _ = store.List(ctx, knowledge.ListFilter{FromTime: now.Add(-90 * time.Minute)})
	if len(results) != 2 {
		t.Errorf("expected 2 recent results, got %d", len(results))
	}
}

func TestKnowledgeStore_List_WithPagination(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	// Add 5 vectors
	for i := 0; i < 5; i++ {
		v := &knowledge.Vector{
			ID:        "doc-" + string(rune('a'+i)),
			Embedding: []float32{1},
			Text:      "Doc",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Hour),
		}
		_ = store.Upsert(ctx, v)
	}

	// Get first 2
	results, _ := store.List(ctx, knowledge.ListFilter{Limit: 2})
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Get next 2
	results, _ = store.List(ctx, knowledge.ListFilter{Limit: 2, Offset: 2})
	if len(results) != 2 {
		t.Errorf("expected 2 results with offset, got %d", len(results))
	}

	// Offset beyond count
	results, _ = store.List(ctx, knowledge.ListFilter{Offset: 10})
	if len(results) != 0 {
		t.Errorf("expected 0 results with large offset, got %d", len(results))
	}
}

func TestKnowledgeStore_Count(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	count, _ := store.Count(ctx)
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	for i := 0; i < 5; i++ {
		v := &knowledge.Vector{
			ID:        "doc-" + string(rune('a'+i)),
			Embedding: []float32{1},
		}
		_ = store.Upsert(ctx, v)
	}

	count, _ = store.Count(ctx)
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

func TestKnowledgeStore_Stats(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	// Empty store
	stats, _ := store.Stats(ctx)
	if stats.VectorCount != 0 || stats.Dimension != 0 {
		t.Error("expected empty stats for empty store")
	}

	// After adding vectors
	v := &knowledge.Vector{
		ID:        "doc-1",
		Embedding: []float32{1, 2, 3},
	}
	_ = store.Upsert(ctx, v)

	stats, _ = store.Stats(ctx)
	if stats.VectorCount != 1 {
		t.Errorf("expected count 1, got %d", stats.VectorCount)
	}
	if stats.Dimension != 3 {
		t.Errorf("expected dimension 3, got %d", stats.Dimension)
	}
}

func TestKnowledgeStore_BatchOperations(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	vectors := []*knowledge.Vector{
		{ID: "batch-1", Embedding: []float32{1, 0, 0}, Text: "One"},
		{ID: "batch-2", Embedding: []float32{0, 1, 0}, Text: "Two"},
		{ID: "batch-3", Embedding: []float32{0, 0, 1}, Text: "Three"},
	}

	// UpsertBatch
	err := store.UpsertBatch(ctx, vectors)
	if err != nil {
		t.Fatalf("UpsertBatch failed: %v", err)
	}

	count, _ := store.Count(ctx)
	if count != 3 {
		t.Errorf("expected 3 vectors after batch upsert, got %d", count)
	}

	// DeleteBatch
	err = store.DeleteBatch(ctx, []string{"batch-1", "batch-2"})
	if err != nil {
		t.Fatalf("DeleteBatch failed: %v", err)
	}

	count, _ = store.Count(ctx)
	if count != 1 {
		t.Errorf("expected 1 vector after batch delete, got %d", count)
	}
}

func TestKnowledgeStore_ConcurrentAccess(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx := context.Background()

	var wg sync.WaitGroup
	concurrency := 100

	// Concurrent writes
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			v := &knowledge.Vector{
				ID:        "doc-" + string(rune(n)),
				Embedding: []float32{float32(n), 0, 0},
			}
			_ = store.Upsert(ctx, v)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.Search(ctx, []float32{1, 0, 0}, 5)
		}()
	}

	wg.Wait()

	// Verify store is consistent
	count, _ := store.Count(ctx)
	if count != int64(concurrency) {
		t.Errorf("expected %d vectors after concurrent access, got %d", concurrency, count)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
		epsilon  float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
			epsilon:  0.0001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
			epsilon:  0.0001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
			epsilon:  0.0001,
		},
		{
			name:     "different lengths",
			a:        []float32{1, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
			epsilon:  0.0001,
		},
		{
			name:     "zero vector",
			a:        []float32{0, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
			epsilon:  0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if math.Abs(float64(got-tt.expected)) > float64(tt.epsilon) {
				t.Errorf("cosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestKnowledgeStore_ContextCancellation(t *testing.T) {
	store := NewKnowledgeStore(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	v := &knowledge.Vector{
		ID:        "doc-1",
		Embedding: []float32{1, 0, 0},
	}

	if err := store.Upsert(ctx, v); err == nil {
		t.Error("expected error from cancelled context")
	}

	if _, err := store.Search(ctx, []float32{1, 0, 0}, 5); err == nil {
		t.Error("expected error from cancelled context")
	}

	if _, err := store.Get(ctx, "doc-1"); err == nil {
		t.Error("expected error from cancelled context")
	}
}
