package cache

import (
	"errors"
	"testing"
	"time"
)

func TestSetOptions_Defaults(t *testing.T) {
	opts := SetOptions{}
	if opts.TTL != 0 {
		t.Errorf("default TTL: got %v, want 0", opts.TTL)
	}
}

func TestSetOptions_WithTTL(t *testing.T) {
	opts := SetOptions{TTL: 5 * time.Minute}
	if opts.TTL != 5*time.Minute {
		t.Errorf("TTL: got %v, want 5m", opts.TTL)
	}
}

func TestStats_ZeroValue(t *testing.T) {
	var s Stats
	if s.Hits != 0 || s.Misses != 0 || s.Size != 0 || s.MaxSize != 0 {
		t.Error("expected zero-value Stats")
	}
}

func TestErrorSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrKeyNotFound", ErrKeyNotFound},
		{"ErrCacheFull", ErrCacheFull},
		{"ErrInvalidKey", ErrInvalidKey},
		{"ErrOperationTimeout", ErrOperationTimeout},
		{"ErrConnectionFailed", ErrConnectionFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error sentinel is nil")
			}
			// Wrap and unwrap
			wrapped := errors.Join(tt.err, errors.New("detail"))
			if !errors.Is(wrapped, tt.err) {
				t.Error("errors.Is failed on wrapped error")
			}
		})
	}
}

func TestCacheInterface(t *testing.T) {
	var _ Cache = nil
}

func TestStatsProviderInterface(t *testing.T) {
	var _ StatsProvider = nil
}
