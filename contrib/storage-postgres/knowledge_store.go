package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/knowledge"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// KnowledgeStore is a PostgreSQL-backed implementation of knowledge.Store.
// It uses pgvector for efficient cosine similarity search on embeddings.
type KnowledgeStore struct {
	pool   *pgxpool.Pool
	schema string
}

// NewKnowledgeStore creates a new PostgreSQL knowledge store.
func NewKnowledgeStore(pool *pgxpool.Pool, schema string) *KnowledgeStore {
	if schema == "" {
		schema = "public"
	}
	return &KnowledgeStore{
		pool:   pool,
		schema: schema,
	}
}

// tableName returns the fully qualified table name.
func (s *KnowledgeStore) tableName() string {
	return fmt.Sprintf("%s.knowledge_vectors", s.schema)
}

// Upsert stores or updates a vector.
func (s *KnowledgeStore) Upsert(ctx context.Context, vector *knowledge.Vector) error {
	if vector.ID == "" {
		return knowledge.ErrInvalidID
	}
	if len(vector.Embedding) == 0 {
		return knowledge.ErrInvalidEmbedding
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, embedding, text, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			text = EXCLUDED.text,
			metadata = EXCLUDED.metadata,
			created_at = EXCLUDED.created_at
	`, s.tableName())

	_, err := s.pool.Exec(ctx, query,
		vector.ID,
		vector.Embedding,
		vector.Text,
		vector.Metadata,
		vector.CreatedAt,
	)
	if err != nil {
		return s.wrapError(err)
	}

	return nil
}

// Search finds similar vectors by embedding using cosine similarity.
// Results are sorted by similarity score (highest first).
func (s *KnowledgeStore) Search(ctx context.Context, embedding []float32, topK int) ([]knowledge.SearchResult, error) {
	if len(embedding) == 0 {
		return nil, knowledge.ErrInvalidEmbedding
	}
	if topK <= 0 {
		topK = 10
	}

	// Use cosine distance operator from pgvector: 1 - (a <=> b) gives cosine similarity.
	query := fmt.Sprintf(`
		SELECT id, text, 1 - (embedding <=> $1::vector) AS score, metadata
		FROM %s
		ORDER BY embedding <=> $1::vector
		LIMIT $2
	`, s.tableName())

	rows, err := s.pool.Query(ctx, query, embedding, topK)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer rows.Close()

	var results []knowledge.SearchResult
	for rows.Next() {
		var r knowledge.SearchResult
		if err := rows.Scan(&r.ID, &r.Text, &r.Score, &r.Metadata); err != nil {
			return nil, s.wrapError(err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, s.wrapError(err)
	}

	return results, nil
}

// Get retrieves a vector by ID.
func (s *KnowledgeStore) Get(ctx context.Context, id string) (*knowledge.Vector, error) {
	if id == "" {
		return nil, knowledge.ErrInvalidID
	}

	query := fmt.Sprintf(`
		SELECT id, embedding, text, metadata, created_at
		FROM %s
		WHERE id = $1
	`, s.tableName())

	var v knowledge.Vector
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&v.ID,
		&v.Embedding,
		&v.Text,
		&v.Metadata,
		&v.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, knowledge.ErrNotFound
		}
		return nil, s.wrapError(err)
	}

	return &v, nil
}

// Delete removes a vector by ID.
func (s *KnowledgeStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return knowledge.ErrInvalidID
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, s.tableName())

	result, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return s.wrapError(err)
	}

	if result.RowsAffected() == 0 {
		return knowledge.ErrNotFound
	}

	return nil
}

// List returns vectors matching the filter criteria.
func (s *KnowledgeStore) List(ctx context.Context, filter knowledge.ListFilter) ([]*knowledge.Vector, error) {
	query, args := s.buildListQuery(filter)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer rows.Close()

	var vectors []*knowledge.Vector
	for rows.Next() {
		var v knowledge.Vector
		if err := rows.Scan(&v.ID, &v.Embedding, &v.Text, &v.Metadata, &v.CreatedAt); err != nil {
			return nil, s.wrapError(err)
		}
		vectors = append(vectors, &v)
	}

	if err := rows.Err(); err != nil {
		return nil, s.wrapError(err)
	}

	return vectors, nil
}

// Count returns the total number of vectors in the store.
func (s *KnowledgeStore) Count(ctx context.Context) (int64, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, s.tableName())

	var count int64
	if err := s.pool.QueryRow(ctx, query).Scan(&count); err != nil {
		return 0, s.wrapError(err)
	}

	return count, nil
}

// Stats returns statistics about the knowledge store.
func (s *KnowledgeStore) Stats(ctx context.Context) (knowledge.Stats, error) {
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, s.tableName())

	var stats knowledge.Stats
	if err := s.pool.QueryRow(ctx, countQuery).Scan(&stats.VectorCount); err != nil {
		return knowledge.Stats{}, s.wrapError(err)
	}

	// Get dimension from the first vector if available.
	dimQuery := fmt.Sprintf(`SELECT array_length(embedding, 1) FROM %s LIMIT 1`, s.tableName())
	err := s.pool.QueryRow(ctx, dimQuery).Scan(&stats.Dimension)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return knowledge.Stats{}, s.wrapError(err)
	}

	return stats, nil
}

// UpsertBatch stores or updates multiple vectors in a single transaction.
func (s *KnowledgeStore) UpsertBatch(ctx context.Context, vectors []*knowledge.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	for _, v := range vectors {
		if v.ID == "" {
			return knowledge.ErrInvalidID
		}
		if len(v.Embedding) == 0 {
			return knowledge.ErrInvalidEmbedding
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return s.wrapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, embedding, text, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			embedding = EXCLUDED.embedding,
			text = EXCLUDED.text,
			metadata = EXCLUDED.metadata,
			created_at = EXCLUDED.created_at
	`, s.tableName())

	batch := &pgx.Batch{}
	for _, v := range vectors {
		batch.Queue(query, v.ID, v.Embedding, v.Text, v.Metadata, v.CreatedAt)
	}

	br := tx.SendBatch(ctx, batch)
	for range vectors {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return s.wrapError(err)
		}
	}
	if err := br.Close(); err != nil {
		return s.wrapError(err)
	}

	return tx.Commit(ctx)
}

// DeleteBatch removes multiple vectors by their IDs in a single operation.
func (s *KnowledgeStore) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ANY($1)`, s.tableName())

	_, err := s.pool.Exec(ctx, query, ids)
	if err != nil {
		return s.wrapError(err)
	}

	return nil
}

// buildListQuery constructs the SELECT query for listing vectors.
func (s *KnowledgeStore) buildListQuery(filter knowledge.ListFilter) (string, []any) {
	var conditions []string
	var args []any
	argNum := 0

	if filter.IDPrefix != "" {
		argNum++
		args = append(args, filter.IDPrefix+"%")
		conditions = append(conditions, fmt.Sprintf("id LIKE $%d", argNum))
	}

	for k, v := range filter.Metadata {
		argNum++
		args = append(args, v)
		conditions = append(conditions, fmt.Sprintf("metadata->>'%s' = $%d", sanitizeKey(k), argNum))
	}

	if !filter.FromTime.IsZero() {
		argNum++
		args = append(args, filter.FromTime)
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argNum))
	}

	if !filter.ToTime.IsZero() {
		argNum++
		args = append(args, filter.ToTime)
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argNum))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, embedding, text, metadata, created_at
		FROM %s
		%s
		ORDER BY created_at DESC
	`, s.tableName(), whereClause)

	if filter.Limit > 0 {
		argNum++
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", argNum)
	}

	if filter.Offset > 0 {
		argNum++
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", argNum)
	}

	return query, args
}

// sanitizeKey removes characters that could be used for SQL injection in JSON key access.
func sanitizeKey(key string) string {
	// Only allow alphanumeric, underscore, and hyphen in metadata keys.
	var b strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// wrapError wraps database errors with domain-appropriate errors.
func (s *KnowledgeStore) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("knowledge store operation timeout: %w", err)
	}

	return fmt.Errorf("knowledge store error: %w", err)
}

// Ensure KnowledgeStore implements knowledge.Store, knowledge.StatsProvider, knowledge.BatchStore.
var (
	_ knowledge.Store         = (*KnowledgeStore)(nil)
	_ knowledge.StatsProvider = (*KnowledgeStore)(nil)
	_ knowledge.BatchStore    = (*KnowledgeStore)(nil)
)
