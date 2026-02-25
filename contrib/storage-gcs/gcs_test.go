package gcs

import (
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/artifact"
)

// TestArtifactStoreCompiles verifies that ArtifactStore implements artifact.Store interface.
// This ensures compile-time verification of interface compliance.
func TestArtifactStoreCompiles(t *testing.T) {
	var _ artifact.Store = (*ArtifactStore)(nil)
}
