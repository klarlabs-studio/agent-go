package memory

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/knowledge"
)

// KnowledgeStore is an in-memory vector store with cosine similarity search.
type KnowledgeStore struct {
	vectors   map[string]*knowledge.Vector
	dimension int // 0 = auto-detect from first vector
	mu        sync.RWMutex
}

// NewKnowledgeStore creates a new in-memory knowledge store.
// If dimension is 0, it will be auto-detected from the first vector.
func NewKnowledgeStore(dimension int) *KnowledgeStore {
	return &KnowledgeStore{
		vectors:   make(map[string]*knowledge.Vector),
		dimension: dimension,
	}
}

// Upsert stores or updates a vector.
func (s *KnowledgeStore) Upsert(ctx context.Context, v *knowledge.Vector) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if v.ID == "" {
		return knowledge.ErrInvalidID
	}
	if len(v.Embedding) == 0 {
		return knowledge.ErrInvalidEmbedding
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Auto-detect or validate dimension
	if s.dimension == 0 {
		s.dimension = len(v.Embedding)
	} else if len(v.Embedding) != s.dimension {
		return knowledge.ErrDimensionMismatch
	}

	// Deep copy
	stored := &knowledge.Vector{
		ID:        v.ID,
		Embedding: make([]float32, len(v.Embedding)),
		Text:      v.Text,
		Metadata:  copyKnowledgeMetadata(v.Metadata),
		CreatedAt: v.CreatedAt,
	}
	copy(stored.Embedding, v.Embedding)

	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = time.Now()
	}

	s.vectors[v.ID] = stored
	return nil
}

// Search finds similar vectors by embedding using cosine similarity.
func (s *KnowledgeStore) Search(ctx context.Context, embedding []float32, topK int) ([]knowledge.SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(embedding) == 0 {
		return nil, knowledge.ErrInvalidEmbedding
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.dimension > 0 && len(embedding) != s.dimension {
		return nil, knowledge.ErrDimensionMismatch
	}

	// Calculate similarity for all vectors
	type scored struct {
		id    string
		text  string
		meta  map[string]string
		score float32
	}

	results := make([]scored, 0, len(s.vectors))
	for _, v := range s.vectors {
		sim := cosineSimilarity(embedding, v.Embedding)
		results = append(results, scored{
			id:    v.ID,
			text:  v.Text,
			meta:  v.Metadata,
			score: sim,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Take top K
	if topK > len(results) {
		topK = len(results)
	}

	output := make([]knowledge.SearchResult, topK)
	for i := 0; i < topK; i++ {
		output[i] = knowledge.SearchResult{
			ID:       results[i].id,
			Text:     results[i].text,
			Score:    results[i].score,
			Metadata: copyKnowledgeMetadata(results[i].meta),
		}
	}

	return output, nil
}

// Get retrieves a vector by ID.
func (s *KnowledgeStore) Get(ctx context.Context, id string) (*knowledge.Vector, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, knowledge.ErrInvalidID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.vectors[id]
	if !ok {
		return nil, knowledge.ErrNotFound
	}

	// Return deep copy
	return copyKnowledgeVector(v), nil
}

// Delete removes a vector by ID.
func (s *KnowledgeStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id == "" {
		return knowledge.ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.vectors[id]; !ok {
		return knowledge.ErrNotFound
	}

	delete(s.vectors, id)
	return nil
}

// List returns vectors matching the filter criteria.
func (s *KnowledgeStore) List(ctx context.Context, filter knowledge.ListFilter) ([]*knowledge.Vector, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*knowledge.Vector
	for _, v := range s.vectors {
		if vectorMatchesFilter(v, filter) {
			results = append(results, copyKnowledgeVector(v))
		}
	}

	// Sort by created_at descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// Apply offset
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	} else if filter.Offset >= len(results) {
		return []*knowledge.Vector{}, nil
	}

	// Apply limit
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// Count returns the total number of vectors in the store.
func (s *KnowledgeStore) Count(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return int64(len(s.vectors)), nil
}

// Stats implements StatsProvider.
func (s *KnowledgeStore) Stats(ctx context.Context) (knowledge.Stats, error) {
	if err := ctx.Err(); err != nil {
		return knowledge.Stats{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return knowledge.Stats{
		VectorCount: int64(len(s.vectors)),
		Dimension:   s.dimension,
	}, nil
}

// UpsertBatch stores or updates multiple vectors.
func (s *KnowledgeStore) UpsertBatch(ctx context.Context, vectors []*knowledge.Vector) error {
	for _, v := range vectors {
		if err := s.Upsert(ctx, v); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBatch removes multiple vectors by ID.
func (s *KnowledgeStore) DeleteBatch(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if err := s.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical direction.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// vectorMatchesFilter checks if a vector matches the filter criteria.
func vectorMatchesFilter(v *knowledge.Vector, f knowledge.ListFilter) bool {
	// ID prefix filter
	if f.IDPrefix != "" && !strings.HasPrefix(v.ID, f.IDPrefix) {
		return false
	}

	// Time filters
	if !f.FromTime.IsZero() && v.CreatedAt.Before(f.FromTime) {
		return false
	}
	if !f.ToTime.IsZero() && v.CreatedAt.After(f.ToTime) {
		return false
	}

	// Metadata filter (all must match)
	for k, want := range f.Metadata {
		if got, ok := v.Metadata[k]; !ok || got != want {
			return false
		}
	}

	return true
}

// copyKnowledgeVector creates a deep copy of a vector.
func copyKnowledgeVector(v *knowledge.Vector) *knowledge.Vector {
	c := &knowledge.Vector{
		ID:        v.ID,
		Embedding: make([]float32, len(v.Embedding)),
		Text:      v.Text,
		Metadata:  copyKnowledgeMetadata(v.Metadata),
		CreatedAt: v.CreatedAt,
	}
	copy(c.Embedding, v.Embedding)
	return c
}

// copyKnowledgeMetadata creates a deep copy of metadata.
func copyKnowledgeMetadata(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// Ensure KnowledgeStore implements the interfaces.
var (
	_ knowledge.Store         = (*KnowledgeStore)(nil)
	_ knowledge.StatsProvider = (*KnowledgeStore)(nil)
	_ knowledge.BatchStore    = (*KnowledgeStore)(nil)
)
