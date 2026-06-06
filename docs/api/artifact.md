# Package `artifact`

**Import path:** `go.klarlabs.de/agent/domain/artifact`

## Overview

package artifact // import "go.klarlabs.de/agent/domain/artifact"

Package artifact provides domain models for artifact storage.

## Full API Reference

```
package artifact // import "go.klarlabs.de/agent/domain/artifact"

Package artifact provides domain models for artifact storage.

VARIABLES

var (
	// ErrArtifactNotFound indicates the artifact was not found.
	ErrArtifactNotFound = errors.New("artifact not found")

	// ErrArtifactExists indicates an artifact with the same ID exists.
	ErrArtifactExists = errors.New("artifact already exists")

	// ErrInvalidRef indicates the artifact reference is invalid.
	ErrInvalidRef = errors.New("invalid artifact reference")

	// ErrStoreFull indicates the store has reached capacity.
	ErrStoreFull = errors.New("artifact store full")

	// ErrChecksumMismatch indicates the content checksum doesn't match.
	ErrChecksumMismatch = errors.New("checksum mismatch")
)
    Domain errors for artifact storage.


TYPES

type Ref struct {
	// ID is the unique identifier for the artifact.
	ID string `json:"id"`

	// Name is an optional human-readable name.
	Name string `json:"name,omitempty"`

	// ContentType is the MIME type of the artifact.
	ContentType string `json:"content_type,omitempty"`

	// Size is the size of the artifact in bytes.
	Size int64 `json:"size"`

	// Checksum is the content hash for integrity verification.
	Checksum string `json:"checksum,omitempty"`

	// CreatedAt is when the artifact was stored.
	CreatedAt time.Time `json:"created_at"`

	// Metadata contains arbitrary key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}
    Ref is a stable reference to a stored artifact.

func NewRef(id string) Ref
    NewRef creates a new artifact reference.

func (r Ref) IsValid() bool
    IsValid returns true if the reference has a valid ID.

func (r Ref) String() string
    String returns a string representation of the reference.

func (r Ref) WithChecksum(checksum string) Ref
    WithChecksum sets the checksum.

func (r Ref) WithContentType(contentType string) Ref
    WithContentType sets the content type.

func (r Ref) WithMetadata(key, value string) Ref
    WithMetadata adds metadata to the artifact.

func (r Ref) WithName(name string) Ref
    WithName sets the artifact name.

func (r Ref) WithSize(size int64) Ref
    WithSize sets the artifact size.

type Store interface {
	// Store saves content and returns a stable reference.
	Store(ctx context.Context, content io.Reader, opts StoreOptions) (Ref, error)

	// Retrieve retrieves the content for an artifact reference.
	Retrieve(ctx context.Context, ref Ref) (io.ReadCloser, error)

	// Delete removes an artifact.
	Delete(ctx context.Context, ref Ref) error

	// Exists checks if an artifact exists.
	Exists(ctx context.Context, ref Ref) (bool, error)

	// Metadata retrieves the metadata for an artifact without content.
	Metadata(ctx context.Context, ref Ref) (Ref, error)
}
    Store defines the interface for artifact storage. Implementations are in
    infrastructure.

type StoreOptions struct {
	// Name is an optional human-readable name.
	Name string

	// ContentType is the MIME type of the content.
	ContentType string

	// Metadata contains arbitrary key-value pairs.
	Metadata map[string]string

	// ComputeChecksum enables checksum computation.
	ComputeChecksum bool
}
    StoreOptions configures artifact storage.

func DefaultStoreOptions() StoreOptions
    DefaultStoreOptions returns options with sensible defaults.

func (o StoreOptions) WithContentType(contentType string) StoreOptions
    WithContentType sets the content type.

func (o StoreOptions) WithMetadata(key, value string) StoreOptions
    WithMetadata adds metadata.

func (o StoreOptions) WithName(name string) StoreOptions
    WithName sets the artifact name.
```
