package artifact_test

import (
	"testing"

	"go.klarlabs.de/agent/domain/artifact"
)

func TestNewRef(t *testing.T) {
	t.Parallel()

	t.Run("creates ref with ID", func(t *testing.T) {
		t.Parallel()

		ref := artifact.NewRef("art-123")
		if ref.ID != "art-123" {
			t.Errorf("NewRef() ID = %s, want art-123", ref.ID)
		}
	})

	t.Run("sets CreatedAt", func(t *testing.T) {
		t.Parallel()

		ref := artifact.NewRef("art-123")
		if ref.CreatedAt.IsZero() {
			t.Error("NewRef() CreatedAt should not be zero")
		}
	})

	t.Run("initializes Metadata map", func(t *testing.T) {
		t.Parallel()

		ref := artifact.NewRef("art-123")
		if ref.Metadata == nil {
			t.Error("NewRef() Metadata should be initialized")
		}
	})
}

func TestRef_WithName(t *testing.T) {
	t.Parallel()

	ref := artifact.NewRef("art-123").WithName("report.pdf")
	if ref.Name != "report.pdf" {
		t.Errorf("WithName() = %s, want report.pdf", ref.Name)
	}
	if ref.ID != "art-123" {
		t.Errorf("WithName() should preserve ID, got %s", ref.ID)
	}
}

func TestRef_WithContentType(t *testing.T) {
	t.Parallel()

	ref := artifact.NewRef("art-123").WithContentType("application/pdf")
	if ref.ContentType != "application/pdf" {
		t.Errorf("WithContentType() = %s, want application/pdf", ref.ContentType)
	}
}

func TestRef_WithSize(t *testing.T) {
	t.Parallel()

	ref := artifact.NewRef("art-123").WithSize(1024)
	if ref.Size != 1024 {
		t.Errorf("WithSize() = %d, want 1024", ref.Size)
	}
}

func TestRef_WithChecksum(t *testing.T) {
	t.Parallel()

	ref := artifact.NewRef("art-123").WithChecksum("sha256:abc123")
	if ref.Checksum != "sha256:abc123" {
		t.Errorf("WithChecksum() = %s, want sha256:abc123", ref.Checksum)
	}
}

func TestRef_WithMetadata(t *testing.T) {
	t.Parallel()

	t.Run("adds metadata to initialized map", func(t *testing.T) {
		t.Parallel()

		ref := artifact.NewRef("art-123").WithMetadata("author", "alice")
		if ref.Metadata["author"] != "alice" {
			t.Errorf("WithMetadata() author = %s, want alice", ref.Metadata["author"])
		}
	})

	t.Run("initializes nil metadata map", func(t *testing.T) {
		t.Parallel()

		ref := artifact.Ref{ID: "art-123"}
		ref = ref.WithMetadata("key", "value")
		if ref.Metadata == nil {
			t.Error("WithMetadata() should initialize nil map")
		}
		if ref.Metadata["key"] != "value" {
			t.Errorf("WithMetadata() key = %s, want value", ref.Metadata["key"])
		}
	})

	t.Run("supports method chaining", func(t *testing.T) {
		t.Parallel()

		ref := artifact.NewRef("art-123").
			WithMetadata("key1", "value1").
			WithMetadata("key2", "value2")

		if ref.Metadata["key1"] != "value1" {
			t.Error("WithMetadata() should preserve previous metadata")
		}
		if ref.Metadata["key2"] != "value2" {
			t.Error("WithMetadata() should add new metadata")
		}
	})
}

func TestRef_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		ref   artifact.Ref
		valid bool
	}{
		{
			name:  "valid with ID",
			ref:   artifact.Ref{ID: "art-123"},
			valid: true,
		},
		{
			name:  "invalid with empty ID",
			ref:   artifact.Ref{ID: ""},
			valid: false,
		},
		{
			name:  "invalid with zero value",
			ref:   artifact.Ref{},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.ref.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestRef_String(t *testing.T) {
	t.Parallel()

	t.Run("returns name and ID when name is set", func(t *testing.T) {
		t.Parallel()

		ref := artifact.Ref{ID: "art-123", Name: "report.pdf"}
		expected := "report.pdf (art-123)"
		if got := ref.String(); got != expected {
			t.Errorf("String() = %s, want %s", got, expected)
		}
	})

	t.Run("returns only ID when name is empty", func(t *testing.T) {
		t.Parallel()

		ref := artifact.Ref{ID: "art-123"}
		if got := ref.String(); got != "art-123" {
			t.Errorf("String() = %s, want art-123", got)
		}
	})
}

func TestRef_FluentBuilder(t *testing.T) {
	t.Parallel()

	ref := artifact.NewRef("art-123").
		WithName("report.pdf").
		WithContentType("application/pdf").
		WithSize(2048).
		WithChecksum("sha256:def456").
		WithMetadata("author", "bob")

	if ref.ID != "art-123" {
		t.Errorf("fluent builder ID = %s, want art-123", ref.ID)
	}
	if ref.Name != "report.pdf" {
		t.Errorf("fluent builder Name = %s, want report.pdf", ref.Name)
	}
	if ref.ContentType != "application/pdf" {
		t.Errorf("fluent builder ContentType = %s, want application/pdf", ref.ContentType)
	}
	if ref.Size != 2048 {
		t.Errorf("fluent builder Size = %d, want 2048", ref.Size)
	}
	if ref.Checksum != "sha256:def456" {
		t.Errorf("fluent builder Checksum = %s, want sha256:def456", ref.Checksum)
	}
	if ref.Metadata["author"] != "bob" {
		t.Errorf("fluent builder Metadata[author] = %s, want bob", ref.Metadata["author"])
	}
}

func TestDefaultStoreOptions(t *testing.T) {
	t.Parallel()

	opts := artifact.DefaultStoreOptions()

	if opts.ContentType != "application/octet-stream" {
		t.Errorf("DefaultStoreOptions() ContentType = %s, want application/octet-stream", opts.ContentType)
	}
	if !opts.ComputeChecksum {
		t.Error("DefaultStoreOptions() ComputeChecksum should be true")
	}
}

func TestStoreOptions_WithName(t *testing.T) {
	t.Parallel()

	opts := artifact.DefaultStoreOptions().WithName("data.json")
	if opts.Name != "data.json" {
		t.Errorf("WithName() = %s, want data.json", opts.Name)
	}
}

func TestStoreOptions_WithContentType(t *testing.T) {
	t.Parallel()

	opts := artifact.DefaultStoreOptions().WithContentType("application/json")
	if opts.ContentType != "application/json" {
		t.Errorf("WithContentType() = %s, want application/json", opts.ContentType)
	}
}

func TestStoreOptions_WithMetadata(t *testing.T) {
	t.Parallel()

	t.Run("initializes nil metadata map", func(t *testing.T) {
		t.Parallel()

		opts := artifact.StoreOptions{}
		opts = opts.WithMetadata("key", "value")
		if opts.Metadata == nil {
			t.Error("WithMetadata() should initialize nil map")
		}
		if opts.Metadata["key"] != "value" {
			t.Errorf("WithMetadata() key = %s, want value", opts.Metadata["key"])
		}
	})

	t.Run("adds to existing metadata", func(t *testing.T) {
		t.Parallel()

		opts := artifact.DefaultStoreOptions().
			WithMetadata("key1", "value1").
			WithMetadata("key2", "value2")

		if opts.Metadata["key1"] != "value1" {
			t.Error("WithMetadata() should preserve existing metadata")
		}
		if opts.Metadata["key2"] != "value2" {
			t.Error("WithMetadata() should add new metadata")
		}
	})
}

func TestStoreOptions_FluentBuilder(t *testing.T) {
	t.Parallel()

	opts := artifact.DefaultStoreOptions().
		WithName("report.json").
		WithContentType("application/json").
		WithMetadata("author", "alice").
		WithMetadata("version", "1.0")

	if opts.Name != "report.json" {
		t.Errorf("fluent builder Name = %s, want report.json", opts.Name)
	}
	if opts.ContentType != "application/json" {
		t.Errorf("fluent builder ContentType = %s, want application/json", opts.ContentType)
	}
	if opts.Metadata["author"] != "alice" {
		t.Errorf("fluent builder Metadata[author] = %s, want alice", opts.Metadata["author"])
	}
	if opts.Metadata["version"] != "1.0" {
		t.Errorf("fluent builder Metadata[version] = %s, want 1.0", opts.Metadata["version"])
	}
	if !opts.ComputeChecksum {
		t.Error("fluent builder should preserve ComputeChecksum from defaults")
	}
}

func TestDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrArtifactNotFound",
			err:  artifact.ErrArtifactNotFound,
			msg:  "artifact not found",
		},
		{
			name: "ErrArtifactExists",
			err:  artifact.ErrArtifactExists,
			msg:  "artifact already exists",
		},
		{
			name: "ErrInvalidRef",
			err:  artifact.ErrInvalidRef,
			msg:  "invalid artifact reference",
		},
		{
			name: "ErrStoreFull",
			err:  artifact.ErrStoreFull,
			msg:  "artifact store full",
		},
		{
			name: "ErrChecksumMismatch",
			err:  artifact.ErrChecksumMismatch,
			msg:  "checksum mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err.Error() != tt.msg {
				t.Errorf("%s.Error() = %s, want %s", tt.name, tt.err.Error(), tt.msg)
			}
		})
	}
}
