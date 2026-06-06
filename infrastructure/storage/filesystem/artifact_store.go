// Package filesystem provides filesystem-based storage implementations.
package filesystem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.klarlabs.de/agent/domain/artifact"
)

// ArtifactStore implements artifact.Store using the local filesystem.
type ArtifactStore struct {
	basePath string
}

// NewArtifactStore creates a new filesystem artifact store.
func NewArtifactStore(basePath string) (*ArtifactStore, error) {
	// Ensure base path exists with restrictive permissions (G301 fix)
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create artifact directory: %w", err)
	}

	return &ArtifactStore{basePath: basePath}, nil
}

// Store saves content and returns a stable reference.
func (s *ArtifactStore) Store(ctx context.Context, content io.Reader, opts artifact.StoreOptions) (artifact.Ref, error) {
	// Generate unique ID
	id := generateArtifactID()

	// Create artifact directory with restrictive permissions (G301 fix)
	artifactPath := s.artifactPath(id)
	if err := os.MkdirAll(artifactPath, 0750); err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to create artifact path: %w", err)
	}

	// Write content to file
	contentPath := filepath.Join(artifactPath, "content")
	// #nosec G304 -- path is constructed from internally generated artifact ID, not user input
	file, err := os.Create(contentPath)
	if err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to create content file: %w", err)
	}

	// Compute checksum while writing
	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)

	size, err := io.Copy(writer, content)
	if err != nil {
		file.Close()               // #nosec G104 -- best-effort cleanup in error path
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to write content: %w", err)
	}

	// Close content file and check for write errors (G104 fix)
	if err := file.Close(); err != nil {
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to close content file: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	// Create reference
	ref := artifact.NewRef(id).
		WithSize(size).
		WithContentType(opts.ContentType)

	if opts.Name != "" {
		ref = ref.WithName(opts.Name)
	}

	if opts.ComputeChecksum {
		ref = ref.WithChecksum(checksum)
	}

	for k, v := range opts.Metadata {
		ref = ref.WithMetadata(k, v)
	}

	// Write metadata file
	metaPath := filepath.Join(artifactPath, "metadata.json")
	// #nosec G304 -- path is constructed from internally generated artifact ID, not user input
	metaFile, err := os.Create(metaPath)
	if err != nil {
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to create metadata file: %w", err)
	}

	if err := json.NewEncoder(metaFile).Encode(ref); err != nil {
		metaFile.Close()           // #nosec G104 -- best-effort cleanup in error path
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Close metadata file and check for write errors (G104 fix)
	if err := metaFile.Close(); err != nil {
		os.RemoveAll(artifactPath) // #nosec G104 -- best-effort cleanup in error path
		return artifact.Ref{}, fmt.Errorf("failed to close metadata file: %w", err)
	}

	return ref, nil
}

// Retrieve retrieves the content for an artifact reference.
func (s *ArtifactStore) Retrieve(_ context.Context, ref artifact.Ref) (io.ReadCloser, error) {
	if !ref.IsValid() {
		return nil, artifact.ErrInvalidRef
	}

	contentPath := filepath.Join(s.artifactPath(ref.ID), "content")
	// #nosec G304 -- path is constructed from validated artifact ref, not user input
	file, err := os.Open(contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, artifact.ErrArtifactNotFound
		}
		return nil, fmt.Errorf("failed to open artifact: %w", err)
	}

	return file, nil
}

// Delete removes an artifact.
func (s *ArtifactStore) Delete(_ context.Context, ref artifact.Ref) error {
	if !ref.IsValid() {
		return artifact.ErrInvalidRef
	}

	artifactPath := s.artifactPath(ref.ID)
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		return artifact.ErrArtifactNotFound
	}

	if err := os.RemoveAll(artifactPath); err != nil {
		return fmt.Errorf("failed to delete artifact: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists.
func (s *ArtifactStore) Exists(_ context.Context, ref artifact.Ref) (bool, error) {
	if !ref.IsValid() {
		return false, artifact.ErrInvalidRef
	}

	contentPath := filepath.Join(s.artifactPath(ref.ID), "content")
	_, err := os.Stat(contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Metadata retrieves the metadata for an artifact without content.
func (s *ArtifactStore) Metadata(_ context.Context, ref artifact.Ref) (artifact.Ref, error) {
	if !ref.IsValid() {
		return artifact.Ref{}, artifact.ErrInvalidRef
	}

	metaPath := filepath.Join(s.artifactPath(ref.ID), "metadata.json")
	// #nosec G304 -- path is constructed from validated artifact ref, not user input
	file, err := os.Open(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return artifact.Ref{}, artifact.ErrArtifactNotFound
		}
		return artifact.Ref{}, fmt.Errorf("failed to open metadata: %w", err)
	}
	defer file.Close() // #nosec G104 -- read-only operation, close error is non-critical

	var storedRef artifact.Ref
	if err := json.NewDecoder(file).Decode(&storedRef); err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to decode metadata: %w", err)
	}

	return storedRef, nil
}

// artifactPath returns the directory path for an artifact.
func (s *ArtifactStore) artifactPath(id string) string {
	return filepath.Join(s.basePath, id)
}

// generateArtifactID creates a unique artifact ID.
func generateArtifactID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

// randomString generates a random alphanumeric string.
func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond) // Ensure uniqueness
	}
	return string(b)
}
