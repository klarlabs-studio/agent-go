package tool_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/tool"
)

func TestNewResult(t *testing.T) {
	t.Parallel()

	output := json.RawMessage(`{"key": "value"}`)
	result := tool.NewResult(output)

	if string(result.Output) != string(output) {
		t.Errorf("Output = %s, want %s", result.Output, output)
	}
	if result.Duration != 0 {
		t.Errorf("Duration = %v, want 0", result.Duration)
	}
	if result.Cached {
		t.Error("Cached should be false")
	}
	if result.IsError() {
		t.Error("IsError should be false")
	}
}

func TestNewResultWithDuration(t *testing.T) {
	t.Parallel()

	output := json.RawMessage(`{"data": "test"}`)
	duration := 100 * time.Millisecond

	result := tool.NewResultWithDuration(output, duration)

	if string(result.Output) != string(output) {
		t.Errorf("Output = %s, want %s", result.Output, output)
	}
	if result.Duration != duration {
		t.Errorf("Duration = %v, want %v", result.Duration, duration)
	}
}

func TestNewErrorResult(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("test error")
	result := tool.NewErrorResult(expectedErr)

	if !result.IsError() {
		t.Error("IsError should be true")
	}
	if result.Error != expectedErr {
		t.Errorf("Error = %v, want %v", result.Error, expectedErr)
	}
}

func TestNewCachedResult(t *testing.T) {
	t.Parallel()

	output := json.RawMessage(`{"cached": true}`)
	result := tool.NewCachedResult(output)

	if string(result.Output) != string(output) {
		t.Errorf("Output = %s, want %s", result.Output, output)
	}
	if !result.Cached {
		t.Error("Cached should be true")
	}
}

func TestResult_IsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result tool.Result
		want   bool
	}{
		{
			name:   "no error",
			result: tool.NewResult(json.RawMessage(`{}`)),
			want:   false,
		},
		{
			name:   "with error",
			result: tool.NewErrorResult(errors.New("error")),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.result.IsError(); got != tt.want {
				t.Errorf("IsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResult_HasArtifacts(t *testing.T) {
	t.Parallel()

	t.Run("no artifacts", func(t *testing.T) {
		t.Parallel()

		result := tool.NewResult(json.RawMessage(`{}`))
		if result.HasArtifacts() {
			t.Error("HasArtifacts should be false")
		}
	})

	t.Run("with artifacts", func(t *testing.T) {
		t.Parallel()

		result := tool.NewResult(json.RawMessage(`{}`))
		result = result.WithArtifact(tool.ArtifactRef{ID: "art-1", Name: "test"})

		if !result.HasArtifacts() {
			t.Error("HasArtifacts should be true")
		}
	})
}

func TestResult_WithArtifact(t *testing.T) {
	t.Parallel()

	result := tool.NewResult(json.RawMessage(`{}`))

	artifact1 := tool.ArtifactRef{ID: "art-1", Name: "artifact1"}
	artifact2 := tool.ArtifactRef{ID: "art-2", Name: "artifact2"}

	result = result.WithArtifact(artifact1)
	result = result.WithArtifact(artifact2)

	if len(result.Artifacts) != 2 {
		t.Errorf("Artifacts count = %d, want 2", len(result.Artifacts))
	}
	if result.Artifacts[0].ID != "art-1" {
		t.Errorf("Artifact[0].ID = %s, want art-1", result.Artifacts[0].ID)
	}
	if result.Artifacts[1].ID != "art-2" {
		t.Errorf("Artifact[1].ID = %s, want art-2", result.Artifacts[1].ID)
	}
}

func TestResult_OutputString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output json.RawMessage
		want   string
	}{
		{
			name:   "json object",
			output: json.RawMessage(`{"key": "value"}`),
			want:   `{"key": "value"}`,
		},
		{
			name:   "json array",
			output: json.RawMessage(`[1, 2, 3]`),
			want:   `[1, 2, 3]`,
		},
		{
			name:   "empty",
			output: json.RawMessage(`{}`),
			want:   `{}`,
		},
		{
			name:   "string value",
			output: json.RawMessage(`"hello"`),
			want:   `"hello"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tool.NewResult(tt.output)
			if got := result.OutputString(); got != tt.want {
				t.Errorf("OutputString() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestArtifactRef_Fields(t *testing.T) {
	t.Parallel()

	ref := tool.ArtifactRef{
		ID:   "artifact-123",
		Name: "test-artifact",
	}

	if ref.ID != "artifact-123" {
		t.Errorf("ID = %s, want artifact-123", ref.ID)
	}
	if ref.Name != "test-artifact" {
		t.Errorf("Name = %s, want test-artifact", ref.Name)
	}
}
