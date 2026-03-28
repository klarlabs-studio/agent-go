package hash_test

import (
	"context"
	"encoding/json"
	"testing"

	hash "github.com/felixgeelhaar/agent-go/contrib/pack-hash"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestRegister(t *testing.T) {
	p := hash.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "hash" {
		t.Errorf("expected pack name %q, got %q", "hash", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := hash.Pack()
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
	p := hash.Pack()
	for _, tt := range p.Tools {
		if tt.Name() == name {
			return tt
		}
	}
	t.Fatalf("tool %q not found in pack", name)
	return nil
}

func TestExecute_SHA256(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "hash_sha256")

	t.Run("hash hello", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		// SHA256 of "hello" is well-known
		expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
		if out["hex"] != expected {
			t.Errorf("expected hex=%s, got %v", expected, out["hex"])
		}
	})

	t.Run("invalid JSON input", func(t *testing.T) {
		input := json.RawMessage(`{invalid}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestExecute_MD5(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "hash_md5")

	t.Run("hash hello", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		expected := "5d41402abc4b2a76b9719d911017c592"
		if out["hex"] != expected {
			t.Errorf("expected hex=%s, got %v", expected, out["hex"])
		}
	})
}

func TestExecute_Verify(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "hash_verify")

	t.Run("correct hash", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello","hash":"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824","algorithm":"sha256"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Error("expected valid=true for correct hash")
		}
	})

	t.Run("incorrect hash", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello","hash":"0000000000000000000000000000000000000000000000000000000000000000","algorithm":"sha256"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for incorrect hash")
		}
	})
}

func TestExecute_HMAC(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "hash_hmac")

	t.Run("compute HMAC", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello","key":"secret","algorithm":"sha256"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["hex"] == nil || out["hex"] == "" {
			t.Error("expected non-empty hex HMAC output")
		}
	})
}
