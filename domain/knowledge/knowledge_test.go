package knowledge

import (
	"errors"
	"testing"
	"time"
)

func TestVector_ZeroValue(t *testing.T) {
	var v Vector
	if v.ID != "" || v.Text != "" || len(v.Embedding) != 0 {
		t.Error("expected zero-value Vector")
	}
}

func TestSearchResult_ZeroValue(t *testing.T) {
	var sr SearchResult
	if sr.Score != 0 || sr.ID != "" {
		t.Error("expected zero-value SearchResult")
	}
}

func TestListFilter_Defaults(t *testing.T) {
	var f ListFilter
	if f.Limit != 0 || f.Offset != 0 || f.IDPrefix != "" {
		t.Error("expected zero-value ListFilter")
	}
}

func TestListFilter_WithTimeRange(t *testing.T) {
	now := time.Now()
	f := ListFilter{
		FromTime: now.Add(-1 * time.Hour),
		ToTime:   now,
		Limit:    10,
	}
	if f.Limit != 10 {
		t.Errorf("Limit: got %d, want 10", f.Limit)
	}
}

func TestStats_ZeroValue(t *testing.T) {
	var s Stats
	if s.VectorCount != 0 || s.Dimension != 0 {
		t.Error("expected zero-value Stats")
	}
}

func TestErrorSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrInvalidID", ErrInvalidID},
		{"ErrInvalidEmbedding", ErrInvalidEmbedding},
		{"ErrDimensionMismatch", ErrDimensionMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error sentinel is nil")
			}
			wrapped := errors.Join(tt.err, errors.New("detail"))
			if !errors.Is(wrapped, tt.err) {
				t.Error("errors.Is failed on wrapped error")
			}
		})
	}
}

func TestInterfaces(t *testing.T) {
	var _ Store = nil
	var _ StatsProvider = nil
	var _ BatchStore = nil
}
