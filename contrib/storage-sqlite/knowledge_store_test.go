package sqlite

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/knowledge"
)

func TestKnowledgeStore_EnsureSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	err := s.EnsureSchema(ctx)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify table exists.
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='knowledge_vectors'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query table: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 table, got %d", count)
	}
}

func TestKnowledgeStore_UpsertGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	v := &knowledge.Vector{
		ID:        "vec-1",
		Embedding: []float32{0.1, 0.2, 0.3},
		Text:      "hello world",
		Metadata:  map[string]string{"source": "test", "tag": "greeting"},
		CreatedAt: now,
	}

	// Upsert.
	err := s.Upsert(ctx, v)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Get.
	got, err := s.Get(ctx, "vec-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.ID != v.ID {
		t.Errorf("expected ID %s, got %s", v.ID, got.ID)
	}
	if got.Text != v.Text {
		t.Errorf("expected Text %s, got %s", v.Text, got.Text)
	}
	if len(got.Embedding) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(got.Embedding))
	}
	if got.Embedding[0] != 0.1 {
		t.Errorf("expected embedding[0] = 0.1, got %f", got.Embedding[0])
	}
	if got.Metadata["source"] != "test" {
		t.Errorf("expected metadata source=test, got %s", got.Metadata["source"])
	}
	if got.Metadata["tag"] != "greeting" {
		t.Errorf("expected metadata tag=greeting, got %s", got.Metadata["tag"])
	}
}

func TestKnowledgeStore_UpsertUpdate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	v := &knowledge.Vector{
		ID:        "vec-update",
		Embedding: []float32{1.0, 0.0},
		Text:      "original",
		CreatedAt: time.Now(),
	}

	if err := s.Upsert(ctx, v); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Update with same ID.
	v.Text = "updated"
	v.Embedding = []float32{0.0, 1.0}
	if err := s.Upsert(ctx, v); err != nil {
		t.Fatalf("Upsert update failed: %v", err)
	}

	got, err := s.Get(ctx, "vec-update")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Text != "updated" {
		t.Errorf("expected updated text, got %s", got.Text)
	}
	if got.Embedding[0] != 0.0 || got.Embedding[1] != 1.0 {
		t.Errorf("expected updated embedding, got %v", got.Embedding)
	}

	// Count should still be 1.
	count, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1 after upsert, got %d", count)
	}
}

func TestKnowledgeStore_UpsertValidation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Empty ID.
	err := s.Upsert(ctx, &knowledge.Vector{
		Embedding: []float32{1.0},
		Text:      "test",
	})
	if err != knowledge.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}

	// Empty embedding.
	err = s.Upsert(ctx, &knowledge.Vector{
		ID:   "vec-no-emb",
		Text: "test",
	})
	if err != knowledge.ErrInvalidEmbedding {
		t.Errorf("expected ErrInvalidEmbedding, got %v", err)
	}
}

func TestKnowledgeStore_GetNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	_, err := s.Get(ctx, "nonexistent")
	if err != knowledge.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = s.Get(ctx, "")
	if err != knowledge.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestKnowledgeStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	v := &knowledge.Vector{
		ID:        "vec-del",
		Embedding: []float32{1.0, 0.0},
		Text:      "to delete",
		CreatedAt: time.Now(),
	}

	if err := s.Upsert(ctx, v); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	if err := s.Delete(ctx, "vec-del"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := s.Get(ctx, "vec-del")
	if err != knowledge.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete non-existent should return ErrNotFound.
	err = s.Delete(ctx, "nonexistent")
	if err != knowledge.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing vector, got %v", err)
	}

	// Delete empty ID should return ErrInvalidID.
	err = s.Delete(ctx, "")
	if err != knowledge.ErrInvalidID {
		t.Errorf("expected ErrInvalidID, got %v", err)
	}
}

func TestKnowledgeStore_Search(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Insert vectors with known directions.
	vectors := []*knowledge.Vector{
		{ID: "v1", Embedding: []float32{1.0, 0.0, 0.0}, Text: "x-axis", CreatedAt: time.Now()},
		{ID: "v2", Embedding: []float32{0.0, 1.0, 0.0}, Text: "y-axis", CreatedAt: time.Now()},
		{ID: "v3", Embedding: []float32{0.0, 0.0, 1.0}, Text: "z-axis", CreatedAt: time.Now()},
		{ID: "v4", Embedding: []float32{0.9, 0.1, 0.0}, Text: "near-x", CreatedAt: time.Now()},
	}

	for _, v := range vectors {
		if err := s.Upsert(ctx, v); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	}

	// Search for vectors similar to x-axis.
	results, err := s.Search(ctx, []float32{1.0, 0.0, 0.0}, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result should be exact match (v1).
	if results[0].ID != "v1" {
		t.Errorf("expected first result to be v1, got %s", results[0].ID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("expected score ~1.0 for exact match, got %f", results[0].Score)
	}

	// Second result should be near-x (v4).
	if results[1].ID != "v4" {
		t.Errorf("expected second result to be v4, got %s", results[1].ID)
	}

	// Search with empty embedding should fail.
	_, err = s.Search(ctx, []float32{}, 10)
	if err != knowledge.ErrInvalidEmbedding {
		t.Errorf("expected ErrInvalidEmbedding, got %v", err)
	}
}

func TestKnowledgeStore_SearchWithMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	vectors := []*knowledge.Vector{
		{
			ID:        "v1",
			Embedding: []float32{1.0, 0.0},
			Text:      "doc one",
			Metadata:  map[string]string{"type": "doc"},
			CreatedAt: time.Now(),
		},
		{
			ID:        "v2",
			Embedding: []float32{0.9, 0.1},
			Text:      "doc two",
			Metadata:  map[string]string{"type": "code"},
			CreatedAt: time.Now(),
		},
	}

	for _, v := range vectors {
		if err := s.Upsert(ctx, v); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	}

	results, err := s.Search(ctx, []float32{1.0, 0.0}, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify metadata is returned.
	if results[0].Metadata["type"] != "doc" {
		t.Errorf("expected metadata type=doc, got %s", results[0].Metadata["type"])
	}
}

func TestKnowledgeStore_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	base := time.Now().Truncate(time.Second)

	vectors := []*knowledge.Vector{
		{ID: "a-1", Embedding: []float32{1.0}, Text: "alpha one", Metadata: map[string]string{"group": "a"}, CreatedAt: base},
		{ID: "a-2", Embedding: []float32{1.0}, Text: "alpha two", Metadata: map[string]string{"group": "a"}, CreatedAt: base.Add(time.Second)},
		{ID: "b-1", Embedding: []float32{1.0}, Text: "beta one", Metadata: map[string]string{"group": "b"}, CreatedAt: base.Add(2 * time.Second)},
	}

	for _, v := range vectors {
		if err := s.Upsert(ctx, v); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	}

	// List all.
	all, err := s.List(ctx, knowledge.ListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 vectors, got %d", len(all))
	}

	// List with ID prefix.
	aVecs, err := s.List(ctx, knowledge.ListFilter{IDPrefix: "a-"})
	if err != nil {
		t.Fatalf("List with IDPrefix failed: %v", err)
	}
	if len(aVecs) != 2 {
		t.Fatalf("expected 2 vectors with prefix 'a-', got %d", len(aVecs))
	}

	// List with metadata filter.
	bVecs, err := s.List(ctx, knowledge.ListFilter{Metadata: map[string]string{"group": "b"}})
	if err != nil {
		t.Fatalf("List with metadata failed: %v", err)
	}
	if len(bVecs) != 1 {
		t.Fatalf("expected 1 vector with group=b, got %d", len(bVecs))
	}
	if bVecs[0].ID != "b-1" {
		t.Errorf("expected b-1, got %s", bVecs[0].ID)
	}

	// List with limit.
	limited, err := s.List(ctx, knowledge.ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("List with limit failed: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("expected 1 vector with limit, got %d", len(limited))
	}

	// List with offset.
	offset, err := s.List(ctx, knowledge.ListFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("List with offset failed: %v", err)
	}
	if len(offset) != 1 {
		t.Fatalf("expected 1 vector with offset, got %d", len(offset))
	}
}

func TestKnowledgeStore_Count(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	count, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 count, got %d", count)
	}

	for i := 0; i < 5; i++ {
		v := &knowledge.Vector{
			ID:        "v-" + string(rune('0'+i)),
			Embedding: []float32{float32(i)},
			Text:      "text",
			CreatedAt: time.Now(),
		}
		if err := s.Upsert(ctx, v); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	}

	count, err = s.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 count, got %d", count)
	}
}

func TestKnowledgeStore_Stats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewKnowledgeStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Stats on empty store.
	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.VectorCount != 0 {
		t.Errorf("expected 0 vectors, got %d", stats.VectorCount)
	}
	if stats.Dimension != 0 {
		t.Errorf("expected 0 dimension, got %d", stats.Dimension)
	}

	// Add a 3-dimensional vector.
	v := &knowledge.Vector{
		ID:        "stats-v1",
		Embedding: []float32{0.1, 0.2, 0.3},
		Text:      "test",
		CreatedAt: time.Now(),
	}
	if err := s.Upsert(ctx, v); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	stats, err = s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.VectorCount != 1 {
		t.Errorf("expected 1 vector, got %d", stats.VectorCount)
	}
	if stats.Dimension != 3 {
		t.Errorf("expected dimension 3, got %d", stats.Dimension)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float32
		epsilon  float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
			epsilon:  0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{0.0, 1.0},
			expected: 0.0,
			epsilon:  0.001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{-1.0, 0.0},
			expected: -1.0,
			epsilon:  0.001,
		},
		{
			name:     "different dimensions",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 0.0,
			epsilon:  0.001,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
			epsilon:  0.001,
		},
		{
			name:     "zero vector",
			a:        []float32{0.0, 0.0},
			b:        []float32{1.0, 0.0},
			expected: 0.0,
			epsilon:  0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			if float32(math.Abs(float64(result-tt.expected))) > tt.epsilon {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}
