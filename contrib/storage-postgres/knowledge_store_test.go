package postgres

import (
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/knowledge"
)

func TestKnowledgeStore_tableName(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		expected string
	}{
		{
			name:     "default schema",
			schema:   "",
			expected: "public.knowledge_vectors",
		},
		{
			name:     "custom schema",
			schema:   "agent",
			expected: "agent.knowledge_vectors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewKnowledgeStore(nil, tt.schema)
			got := store.tableName()
			if got != tt.expected {
				t.Errorf("tableName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestKnowledgeStore_Upsert_Validation(t *testing.T) {
	store := NewKnowledgeStore(nil, "public")

	t.Run("empty ID returns ErrInvalidID", func(t *testing.T) {
		v := &knowledge.Vector{
			ID:        "",
			Embedding: []float32{0.1, 0.2},
		}
		err := store.Upsert(t.Context(), v)
		if err != knowledge.ErrInvalidID {
			t.Errorf("Upsert() error = %v, want %v", err, knowledge.ErrInvalidID)
		}
	})

	t.Run("empty embedding returns ErrInvalidEmbedding", func(t *testing.T) {
		v := &knowledge.Vector{
			ID:        "vec-1",
			Embedding: nil,
		}
		err := store.Upsert(t.Context(), v)
		if err != knowledge.ErrInvalidEmbedding {
			t.Errorf("Upsert() error = %v, want %v", err, knowledge.ErrInvalidEmbedding)
		}
	})
}

func TestKnowledgeStore_Get_Validation(t *testing.T) {
	store := NewKnowledgeStore(nil, "public")

	t.Run("empty ID returns ErrInvalidID", func(t *testing.T) {
		_, err := store.Get(t.Context(), "")
		if err != knowledge.ErrInvalidID {
			t.Errorf("Get() error = %v, want %v", err, knowledge.ErrInvalidID)
		}
	})
}

func TestKnowledgeStore_Delete_Validation(t *testing.T) {
	store := NewKnowledgeStore(nil, "public")

	t.Run("empty ID returns ErrInvalidID", func(t *testing.T) {
		err := store.Delete(t.Context(), "")
		if err != knowledge.ErrInvalidID {
			t.Errorf("Delete() error = %v, want %v", err, knowledge.ErrInvalidID)
		}
	})
}

func TestKnowledgeStore_Search_Validation(t *testing.T) {
	store := NewKnowledgeStore(nil, "public")

	t.Run("empty embedding returns ErrInvalidEmbedding", func(t *testing.T) {
		_, err := store.Search(t.Context(), nil, 10)
		if err != knowledge.ErrInvalidEmbedding {
			t.Errorf("Search() error = %v, want %v", err, knowledge.ErrInvalidEmbedding)
		}
	})
}

func TestKnowledgeStore_UpsertBatch_Validation(t *testing.T) {
	store := NewKnowledgeStore(nil, "public")

	t.Run("empty batch returns nil", func(t *testing.T) {
		err := store.UpsertBatch(t.Context(), nil)
		if err != nil {
			t.Errorf("UpsertBatch() error = %v, want nil", err)
		}
	})

	t.Run("vector with empty ID returns ErrInvalidID", func(t *testing.T) {
		vectors := []*knowledge.Vector{
			{ID: "valid", Embedding: []float32{0.1}},
			{ID: "", Embedding: []float32{0.2}},
		}
		err := store.UpsertBatch(t.Context(), vectors)
		if err != knowledge.ErrInvalidID {
			t.Errorf("UpsertBatch() error = %v, want %v", err, knowledge.ErrInvalidID)
		}
	})

	t.Run("vector with empty embedding returns ErrInvalidEmbedding", func(t *testing.T) {
		vectors := []*knowledge.Vector{
			{ID: "vec-1", Embedding: nil},
		}
		err := store.UpsertBatch(t.Context(), vectors)
		if err != knowledge.ErrInvalidEmbedding {
			t.Errorf("UpsertBatch() error = %v, want %v", err, knowledge.ErrInvalidEmbedding)
		}
	})
}

func TestKnowledgeStore_DeleteBatch_EmptySlice(t *testing.T) {
	store := NewKnowledgeStore(nil, "public")

	err := store.DeleteBatch(t.Context(), nil)
	if err != nil {
		t.Errorf("DeleteBatch() error = %v, want nil", err)
	}
}

func TestKnowledgeStore_buildListQuery(t *testing.T) {
	store := NewKnowledgeStore(nil, "public")

	tests := []struct {
		name           string
		filter         knowledge.ListFilter
		expectContains []string
		expectArgCount int
	}{
		{
			name:           "empty filter",
			filter:         knowledge.ListFilter{},
			expectContains: []string{"SELECT", "FROM public.knowledge_vectors", "ORDER BY created_at DESC"},
			expectArgCount: 0,
		},
		{
			name: "with ID prefix",
			filter: knowledge.ListFilter{
				IDPrefix: "doc-",
			},
			expectContains: []string{"id LIKE"},
			expectArgCount: 1,
		},
		{
			name: "with metadata",
			filter: knowledge.ListFilter{
				Metadata: map[string]string{"source": "wiki"},
			},
			expectContains: []string{"metadata"},
			expectArgCount: 1,
		},
		{
			name: "with time range",
			filter: knowledge.ListFilter{
				FromTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				ToTime:   time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			expectContains: []string{"created_at >=", "created_at <="},
			expectArgCount: 2,
		},
		{
			name: "with limit and offset",
			filter: knowledge.ListFilter{
				Limit:  10,
				Offset: 20,
			},
			expectContains: []string{"LIMIT", "OFFSET"},
			expectArgCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := store.buildListQuery(tt.filter)

			for _, substr := range tt.expectContains {
				if !containsString(query, substr) {
					t.Errorf("query %q does not contain %q", query, substr)
				}
			}

			if len(args) != tt.expectArgCount {
				t.Errorf("args count = %d, want %d", len(args), tt.expectArgCount)
			}
		})
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"source", "source"},
		{"my_key", "my_key"},
		{"my-key", "my-key"},
		{"key'; DROP TABLE --", "keyDROPTABLE--"},
		{"a1b2c3", "a1b2c3"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeKey(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeKey(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestKnowledgeStore_InterfaceCompliance(t *testing.T) {
	// Verify the KnowledgeStore implements the required interfaces.
	var _ knowledge.Store = (*KnowledgeStore)(nil)
	var _ knowledge.StatsProvider = (*KnowledgeStore)(nil)
	var _ knowledge.BatchStore = (*KnowledgeStore)(nil)
}
