package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/knowledge"
)

// KnowledgeStore is a SQLite-backed implementation of knowledge.Store.
// It stores vectors with embeddings and metadata, providing cosine similarity
// search using pure SQL computation.
//
// Note: SQLite does not natively support vector operations. Cosine similarity
// is computed in Go after loading candidate vectors. This is suitable for
// small-to-medium datasets (up to ~100K vectors). For larger datasets,
// consider a dedicated vector database.
type KnowledgeStore struct {
	db *sql.DB
}

// NewKnowledgeStore creates a new SQLite knowledge store with the given database connection.
// The caller is responsible for managing the database connection lifecycle.
func NewKnowledgeStore(db *sql.DB) *KnowledgeStore {
	return &KnowledgeStore{db: db}
}

// EnsureSchema creates the knowledge tables if they don't exist.
func (s *KnowledgeStore) EnsureSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS knowledge_vectors (
		id TEXT PRIMARY KEY,
		embedding BLOB NOT NULL,
		text TEXT NOT NULL,
		metadata TEXT,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_knowledge_vectors_created_at ON knowledge_vectors(created_at);`

	_, err := s.db.ExecContext(ctx, query)
	return err
}

// Upsert stores or updates a vector.
func (s *KnowledgeStore) Upsert(ctx context.Context, vector *knowledge.Vector) error {
	if vector.ID == "" {
		return knowledge.ErrInvalidID
	}
	if len(vector.Embedding) == 0 {
		return knowledge.ErrInvalidEmbedding
	}

	embeddingBytes, err := json.Marshal(vector.Embedding)
	if err != nil {
		return err
	}

	var metadataBytes []byte
	if len(vector.Metadata) > 0 {
		metadataBytes, err = json.Marshal(vector.Metadata)
		if err != nil {
			return err
		}
	}

	if vector.CreatedAt.IsZero() {
		vector.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO knowledge_vectors (id, embedding, text, metadata, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			embedding = excluded.embedding,
			text = excluded.text,
			metadata = excluded.metadata`

	_, err = s.db.ExecContext(ctx, query,
		vector.ID,
		embeddingBytes,
		vector.Text,
		metadataBytes,
		vector.CreatedAt,
	)
	return err
}

// Search finds similar vectors by embedding using cosine similarity.
// Returns results sorted by similarity score (highest first).
func (s *KnowledgeStore) Search(ctx context.Context, embedding []float32, topK int) ([]knowledge.SearchResult, error) {
	if len(embedding) == 0 {
		return nil, knowledge.ErrInvalidEmbedding
	}
	if topK <= 0 {
		topK = 10
	}

	// Load all vectors and compute cosine similarity in Go.
	// SQLite lacks native vector operations, so this is the pragmatic approach
	// for small-to-medium datasets.
	query := `SELECT id, embedding, text, metadata FROM knowledge_vectors`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		id       string
		text     string
		metadata map[string]string
		score    float32
	}

	var candidates []scored

	for rows.Next() {
		var id, text string
		var embBytes, metaBytes []byte

		if err := rows.Scan(&id, &embBytes, &text, &metaBytes); err != nil {
			return nil, err
		}

		var storedEmb []float32
		if err := json.Unmarshal(embBytes, &storedEmb); err != nil {
			return nil, err
		}

		score := cosineSimilarity(embedding, storedEmb)

		var metadata map[string]string
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &metadata)
		}

		candidates = append(candidates, scored{
			id:       id,
			text:     text,
			metadata: metadata,
			score:    score,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by score descending using insertion sort (stable, good for small N).
	for i := 1; i < len(candidates); i++ {
		key := candidates[i]
		j := i - 1
		for j >= 0 && candidates[j].score < key.score {
			candidates[j+1] = candidates[j]
			j--
		}
		candidates[j+1] = key
	}

	// Trim to topK.
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	results := make([]knowledge.SearchResult, len(candidates))
	for i, c := range candidates {
		results[i] = knowledge.SearchResult{
			ID:       c.id,
			Text:     c.text,
			Score:    c.score,
			Metadata: c.metadata,
		}
	}

	return results, nil
}

// Get retrieves a vector by ID.
func (s *KnowledgeStore) Get(ctx context.Context, id string) (*knowledge.Vector, error) {
	if id == "" {
		return nil, knowledge.ErrInvalidID
	}

	query := `SELECT id, embedding, text, metadata, created_at FROM knowledge_vectors WHERE id = ?`

	var embBytes, metaBytes []byte
	var v knowledge.Vector

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&v.ID,
		&embBytes,
		&v.Text,
		&metaBytes,
		&v.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, knowledge.ErrNotFound
		}
		return nil, err
	}

	if err := json.Unmarshal(embBytes, &v.Embedding); err != nil {
		return nil, err
	}

	if len(metaBytes) > 0 {
		v.Metadata = make(map[string]string)
		if err := json.Unmarshal(metaBytes, &v.Metadata); err != nil {
			return nil, err
		}
	}

	return &v, nil
}

// Delete removes a vector by ID.
func (s *KnowledgeStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return knowledge.ErrInvalidID
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM knowledge_vectors WHERE id = ?`, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return knowledge.ErrNotFound
	}

	return nil
}

// List returns vectors matching the filter criteria.
func (s *KnowledgeStore) List(ctx context.Context, filter knowledge.ListFilter) ([]*knowledge.Vector, error) {
	query, args := s.buildListQuery(filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vectors []*knowledge.Vector
	for rows.Next() {
		v, err := s.scanVector(rows)
		if err != nil {
			return nil, err
		}

		// Apply metadata filter in Go since metadata is stored as JSON.
		if len(filter.Metadata) > 0 && !matchesMetadata(v.Metadata, filter.Metadata) {
			continue
		}

		vectors = append(vectors, v)
	}

	return vectors, rows.Err()
}

// Count returns the total number of vectors in the store.
func (s *KnowledgeStore) Count(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_vectors`).Scan(&count)
	return count, err
}

// Stats returns statistics about the knowledge store.
func (s *KnowledgeStore) Stats(ctx context.Context) (knowledge.Stats, error) {
	count, err := s.Count(ctx)
	if err != nil {
		return knowledge.Stats{}, err
	}

	// Determine dimension from first vector, if any.
	var dimension int
	var embBytes []byte
	err = s.db.QueryRowContext(ctx, `SELECT embedding FROM knowledge_vectors LIMIT 1`).Scan(&embBytes)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return knowledge.Stats{}, err
	}
	if len(embBytes) > 0 {
		var emb []float32
		if err := json.Unmarshal(embBytes, &emb); err == nil {
			dimension = len(emb)
		}
	}

	return knowledge.Stats{
		VectorCount: count,
		Dimension:   dimension,
	}, nil
}

// Close closes the underlying database connection.
func (s *KnowledgeStore) Close() error {
	return s.db.Close()
}

// buildListQuery constructs the SELECT query for listing vectors.
func (s *KnowledgeStore) buildListQuery(filter knowledge.ListFilter) (string, []any) {
	var conditions []string
	var args []any

	if filter.IDPrefix != "" {
		args = append(args, filter.IDPrefix+"%")
		conditions = append(conditions, "id LIKE ?")
	}

	if !filter.FromTime.IsZero() {
		args = append(args, filter.FromTime)
		conditions = append(conditions, "created_at >= ?")
	}

	if !filter.ToTime.IsZero() {
		args = append(args, filter.ToTime)
		conditions = append(conditions, "created_at <= ?")
	}

	// Metadata filtering is done post-query since metadata is stored as JSON.
	// For small-to-medium datasets this is acceptable. For larger datasets,
	// consider a dedicated metadata index table.

	query := `SELECT id, embedding, text, metadata, created_at FROM knowledge_vectors`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	return query, args
}

// scanVector scans a row into a Vector struct.
func (s *KnowledgeStore) scanVector(rows *sql.Rows) (*knowledge.Vector, error) {
	var v knowledge.Vector
	var embBytes, metaBytes []byte

	if err := rows.Scan(&v.ID, &embBytes, &v.Text, &metaBytes, &v.CreatedAt); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(embBytes, &v.Embedding); err != nil {
		return nil, err
	}

	if len(metaBytes) > 0 {
		v.Metadata = make(map[string]string)
		_ = json.Unmarshal(metaBytes, &v.Metadata)
	}

	return &v, nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns a value in [0, 1] where 1 means identical direction.
// Returns 0 if vectors have different dimensions or zero magnitude.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// matchesMetadata returns true if all required metadata key-value pairs
// are present in the vector's metadata.
func matchesMetadata(vectorMeta, required map[string]string) bool {
	if len(vectorMeta) == 0 && len(required) > 0 {
		return false
	}
	for k, v := range required {
		if vectorMeta[k] != v {
			return false
		}
	}
	return true
}

// Ensure interfaces are implemented.
var (
	_ knowledge.Store         = (*KnowledgeStore)(nil)
	_ knowledge.StatsProvider = (*KnowledgeStore)(nil)
)
