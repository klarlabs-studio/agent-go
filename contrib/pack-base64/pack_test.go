package base64

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestPack_RegistersTools(t *testing.T) {
	p := Pack()

	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() registered no tools")
	}
	if p.Name != "base64" {
		t.Errorf("expected pack name %q, got %q", "base64", p.Name)
	}
}

func TestPack_ToolsImplementInterface(t *testing.T) {
	p := Pack()

	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
		if tt.Description() == "" {
			t.Errorf("tool %q has empty description", tt.Name())
		}
	}
}

// findTool looks up a tool by name in the pack.
func findTool(t *testing.T, name string) tool.Tool {
	t.Helper()
	p := Pack()
	for _, tt := range p.Tools {
		if tt.Name() == name {
			return tt
		}
	}
	t.Fatalf("tool %q not found in pack", name)
	return nil
}

func TestExecute_Encode(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "base64_encode")

	t.Run("encode hello", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		// base64("hello") = "aGVsbG8="
		if out["encoded"] != "aGVsbG8=" {
			t.Errorf("expected encoded=aGVsbG8=, got %v", out["encoded"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		input := json.RawMessage(`{bad json}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for invalid JSON input")
		}
	})
}

func TestExecute_Decode(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "base64_decode")

	t.Run("decode valid base64", func(t *testing.T) {
		input := json.RawMessage(`{"encoded":"aGVsbG8="}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["decoded"] != "hello" {
			t.Errorf("expected decoded=hello, got %v", out["decoded"])
		}
		if out["valid"] != true {
			t.Error("expected valid=true")
		}
	})

	t.Run("decode invalid base64", func(t *testing.T) {
		input := json.RawMessage(`{"encoded":"!!!not-base64!!!"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for invalid base64")
		}
	})
}

func TestExecute_EncodeURL(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "base64_encode_url")

	t.Run("URL-safe encode", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello world?"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		encoded, ok := out["encoded"].(string)
		if !ok || encoded == "" {
			t.Error("expected non-empty encoded output")
		}
	})
}

func TestExecute_Validate(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "base64_validate")

	t.Run("valid base64", func(t *testing.T) {
		input := json.RawMessage(`{"encoded":"aGVsbG8="}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Error("expected valid=true")
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		input := json.RawMessage(`{"encoded":"!!!"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for invalid base64")
		}
	})
}
