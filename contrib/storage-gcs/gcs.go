// Package gcs provides Google Cloud Storage-backed implementations of agent-go storage interfaces.
//
// GCS is a scalable, fully managed object storage service that offers industry-leading
// durability and availability. It is ideal for storing large artifacts, backups, and
// any content that benefits from object storage semantics.
//
// # Usage
//
//	client, err := storage.NewClient(ctx)
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
//	store := gcs.NewArtifactStore(client, "my-bucket")
package gcs

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"go.klarlabs.de/agent/domain/artifact"
)

// ArtifactStore is a GCS-backed implementation of artifact.Store.
// It stores artifacts as objects in a GCS bucket with metadata support.
type ArtifactStore struct {
	client     *storage.Client
	bucketName string
	prefix     string
}

// bucket returns a handle to the configured GCS bucket.
func (s *ArtifactStore) bucket() *storage.BucketHandle {
	return s.client.Bucket(s.bucketName)
}

// ArtifactStoreConfig holds configuration for the GCS artifact store.
type ArtifactStoreConfig struct {
	// BucketName is the GCS bucket name.
	BucketName string

	// Prefix is an optional prefix for all object keys.
	Prefix string

	// ComputeChecksum enables MD5 checksum computation on upload.
	ComputeChecksum bool
}

// NewArtifactStore creates a new GCS artifact store with the given client and bucket.
func NewArtifactStore(client *storage.Client, bucketName string) *ArtifactStore {
	return &ArtifactStore{
		client:     client,
		bucketName: bucketName,
	}
}

// NewArtifactStoreWithConfig creates a new GCS artifact store with full configuration.
func NewArtifactStoreWithConfig(client *storage.Client, cfg ArtifactStoreConfig) *ArtifactStore {
	return &ArtifactStore{
		client:     client,
		bucketName: cfg.BucketName,
		prefix:     cfg.Prefix,
	}
}

// Store saves content and returns a stable reference.
// The content is uploaded to GCS as an object with metadata stored as custom headers.
func (s *ArtifactStore) Store(ctx context.Context, content io.Reader, opts artifact.StoreOptions) (artifact.Ref, error) {
	// Check context validity
	if err := ctx.Err(); err != nil {
		return artifact.Ref{}, err
	}

	// Generate unique ID for this artifact
	id := uuid.New().String()

	// Get object handle
	obj := s.bucket().Object(s.objectKey(id))
	w := obj.NewWriter(ctx)

	// Set content type
	if opts.ContentType != "" {
		w.ContentType = opts.ContentType
	}

	// Set metadata
	if len(opts.Metadata) > 0 {
		w.Metadata = opts.Metadata
	}

	// Set up checksum computation if requested
	var hasher hash.Hash
	reader := content
	if opts.ComputeChecksum {
		hasher = md5.New()
		reader = io.TeeReader(content, hasher)
	}

	// Copy content to GCS
	written, err := io.Copy(w, reader)
	if err != nil {
		w.Close()
		return artifact.Ref{}, fmt.Errorf("failed to write content: %w", err)
	}

	// Close writer to complete upload
	if err := w.Close(); err != nil {
		return artifact.Ref{}, fmt.Errorf("failed to close writer: %w", err)
	}

	// Build artifact reference
	ref := artifact.Ref{
		ID:          id,
		Name:        opts.Name,
		ContentType: opts.ContentType,
		Size:        written,
		CreatedAt:   time.Now(),
		Metadata:    opts.Metadata,
	}

	// Add checksum if computed
	if opts.ComputeChecksum && hasher != nil {
		ref.Checksum = hex.EncodeToString(hasher.Sum(nil))
	}

	return ref, nil
}

// Retrieve retrieves the content for an artifact reference.
// Returns an io.ReadCloser that must be closed by the caller.
func (s *ArtifactStore) Retrieve(ctx context.Context, ref artifact.Ref) (io.ReadCloser, error) {
	// Check context validity
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Validate reference
	if ref.ID == "" {
		return nil, artifact.ErrInvalidRef
	}

	// Get object handle
	obj := s.bucket().Object(s.objectKey(ref.ID))

	// Create reader
	reader, err := obj.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, artifact.ErrArtifactNotFound
		}
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}

	return reader, nil
}

// Delete removes an artifact from the bucket.
func (s *ArtifactStore) Delete(ctx context.Context, ref artifact.Ref) error {
	// Check context validity
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate reference
	if ref.ID == "" {
		return artifact.ErrInvalidRef
	}

	// Delete object
	obj := s.bucket().Object(s.objectKey(ref.ID))
	err := obj.Delete(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return artifact.ErrArtifactNotFound
		}
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists in the bucket.
func (s *ArtifactStore) Exists(ctx context.Context, ref artifact.Ref) (bool, error) {
	// Check context validity
	if err := ctx.Err(); err != nil {
		return false, err
	}

	// Validate reference
	if ref.ID == "" {
		return false, artifact.ErrInvalidRef
	}

	// Check object existence via Attrs
	obj := s.bucket().Object(s.objectKey(ref.ID))
	_, err := obj.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return true, nil
}

// Metadata retrieves the metadata for an artifact without content.
// Returns a Ref populated with object attributes from GCS.
func (s *ArtifactStore) Metadata(ctx context.Context, ref artifact.Ref) (artifact.Ref, error) {
	// Check context validity
	if err := ctx.Err(); err != nil {
		return artifact.Ref{}, err
	}

	// Validate reference
	if ref.ID == "" {
		return artifact.Ref{}, artifact.ErrInvalidRef
	}

	// Get object handle and attributes
	obj := s.bucket().Object(s.objectKey(ref.ID))
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return artifact.Ref{}, artifact.ErrArtifactNotFound
		}
		return artifact.Ref{}, fmt.Errorf("failed to get attributes: %w", err)
	}

	// Map GCS attributes to artifact reference
	result := artifact.Ref{
		ID:          ref.ID,
		Name:        attrs.Name,
		ContentType: attrs.ContentType,
		Size:        attrs.Size,
		CreatedAt:   attrs.Created,
		Metadata:    attrs.Metadata,
	}

	// Add MD5 checksum if available
	if len(attrs.MD5) > 0 {
		result.Checksum = hex.EncodeToString(attrs.MD5)
	}

	return result, nil
}

// objectKey returns the full object key including prefix.
func (s *ArtifactStore) objectKey(id string) string {
	if s.prefix == "" {
		return id
	}
	return s.prefix + "/" + id
}

// Ensure interface is implemented.
var _ artifact.Store = (*ArtifactStore)(nil)
