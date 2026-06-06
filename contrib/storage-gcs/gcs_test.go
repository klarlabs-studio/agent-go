package gcs

import (
	"context"
	"testing"

	"go.klarlabs.de/agent/domain/artifact"
)

// --- Interface compliance tests ---

func TestArtifactStoreCompiles(t *testing.T) {
	var _ artifact.Store = (*ArtifactStore)(nil)
}

// --- Constructor tests ---

func TestNewArtifactStore(t *testing.T) {
	// We pass nil client since we are only testing struct construction.
	// No GCS operations will be called.
	store := NewArtifactStore(nil, "test-bucket")
	if store == nil {
		t.Fatal("expected non-nil ArtifactStore")
	}
	if store.bucketName != "test-bucket" {
		t.Errorf("expected bucket name %q, got %q", "test-bucket", store.bucketName)
	}
	if store.prefix != "" {
		t.Errorf("expected empty prefix, got %q", store.prefix)
	}
}

func TestNewArtifactStoreWithConfig(t *testing.T) {
	store := NewArtifactStoreWithConfig(nil, ArtifactStoreConfig{
		BucketName:      "config-bucket",
		Prefix:          "artifacts/v1",
		ComputeChecksum: true,
	})
	if store == nil {
		t.Fatal("expected non-nil ArtifactStore")
	}
	if store.bucketName != "config-bucket" {
		t.Errorf("expected bucket name %q, got %q", "config-bucket", store.bucketName)
	}
	if store.prefix != "artifacts/v1" {
		t.Errorf("expected prefix %q, got %q", "artifacts/v1", store.prefix)
	}
}

// --- objectKey tests ---

func TestObjectKey_NoPrefix(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
		prefix:     "",
	}

	key := store.objectKey("abc-123")
	if key != "abc-123" {
		t.Errorf("expected %q, got %q", "abc-123", key)
	}
}

func TestObjectKey_WithPrefix(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
		prefix:     "artifacts/v1",
	}

	key := store.objectKey("abc-123")
	expected := "artifacts/v1/abc-123"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestObjectKey_PrefixVariations(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		id       string
		expected string
	}{
		{
			name:     "empty prefix",
			prefix:   "",
			id:       "file-1",
			expected: "file-1",
		},
		{
			name:     "simple prefix",
			prefix:   "data",
			id:       "file-1",
			expected: "data/file-1",
		},
		{
			name:     "nested prefix",
			prefix:   "org/project/artifacts",
			id:       "file-1",
			expected: "org/project/artifacts/file-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &ArtifactStore{prefix: tt.prefix}
			result := store.objectKey(tt.id)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// --- Input validation tests (context and ref) ---

func TestRetrieve_EmptyRefID(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	_, err := store.Retrieve(context.Background(), artifact.Ref{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty ref ID")
	}
	if err != artifact.ErrInvalidRef {
		t.Errorf("expected ErrInvalidRef, got %v", err)
	}
}

func TestDelete_EmptyRefID(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	err := store.Delete(context.Background(), artifact.Ref{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty ref ID")
	}
	if err != artifact.ErrInvalidRef {
		t.Errorf("expected ErrInvalidRef, got %v", err)
	}
}

func TestExists_EmptyRefID(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	_, err := store.Exists(context.Background(), artifact.Ref{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty ref ID")
	}
	if err != artifact.ErrInvalidRef {
		t.Errorf("expected ErrInvalidRef, got %v", err)
	}
}

func TestMetadata_EmptyRefID(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	_, err := store.Metadata(context.Background(), artifact.Ref{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty ref ID")
	}
	if err != artifact.ErrInvalidRef {
		t.Errorf("expected ErrInvalidRef, got %v", err)
	}
}

// --- Context cancellation tests ---

func TestStore_CancelledContext(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Store(ctx, nil, artifact.StoreOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context on Store")
	}
}

func TestRetrieve_CancelledContext(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Retrieve(ctx, artifact.Ref{ID: "test"})
	if err == nil {
		t.Fatal("expected error for cancelled context on Retrieve")
	}
}

func TestDelete_CancelledContext(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Delete(ctx, artifact.Ref{ID: "test"})
	if err == nil {
		t.Fatal("expected error for cancelled context on Delete")
	}
}

func TestExists_CancelledContext(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Exists(ctx, artifact.Ref{ID: "test"})
	if err == nil {
		t.Fatal("expected error for cancelled context on Exists")
	}
}

func TestMetadata_CancelledContext(t *testing.T) {
	store := &ArtifactStore{
		bucketName: "test-bucket",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Metadata(ctx, artifact.Ref{ID: "test"})
	if err == nil {
		t.Fatal("expected error for cancelled context on Metadata")
	}
}

// --- ArtifactStoreConfig validation tests ---

func TestArtifactStoreConfig_Defaults(t *testing.T) {
	cfg := ArtifactStoreConfig{}

	if cfg.BucketName != "" {
		t.Errorf("expected empty default bucket name, got %q", cfg.BucketName)
	}
	if cfg.Prefix != "" {
		t.Errorf("expected empty default prefix, got %q", cfg.Prefix)
	}
	if cfg.ComputeChecksum {
		t.Error("expected ComputeChecksum to default to false")
	}
}
