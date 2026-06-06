package validate

import (
	"context"
	"encoding/json"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestPack(t *testing.T) {
	p := Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Name != "validate" {
		t.Errorf("expected pack name 'validate', got %q", p.Name)
	}
	if len(p.Tools) == 0 {
		t.Fatal("expected at least 1 tool, got 0")
	}
}

func TestToolsImplementInterface(t *testing.T) {
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

func TestExecute_ValidateEmail(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "validate_email")

	t.Run("valid email", func(t *testing.T) {
		input := json.RawMessage(`{"email":"user@example.com"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Error("expected valid=true for valid email")
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		input := json.RawMessage(`{"email":"not-an-email"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for invalid email")
		}
	})
}

func TestExecute_ValidateURL(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "validate_url")

	t.Run("valid URL", func(t *testing.T) {
		input := json.RawMessage(`{"url":"https://example.com/path?q=1"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Error("expected valid=true for valid URL")
		}
		if out["scheme"] != "https" {
			t.Errorf("expected scheme=https, got %v", out["scheme"])
		}
	})

	t.Run("invalid URL - no host", func(t *testing.T) {
		input := json.RawMessage(`{"url":"not a url"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for URL without host")
		}
	})
}

func TestExecute_ValidateIP(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "validate_ip")

	t.Run("valid IPv4", func(t *testing.T) {
		input := json.RawMessage(`{"ip":"192.168.1.1"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Error("expected valid=true for valid IPv4")
		}
		if out["version"] != float64(4) {
			t.Errorf("expected version=4, got %v", out["version"])
		}
	})

	t.Run("valid IPv6", func(t *testing.T) {
		input := json.RawMessage(`{"ip":"::1"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Error("expected valid=true for valid IPv6")
		}
		if out["version"] != float64(6) {
			t.Errorf("expected version=6, got %v", out["version"])
		}
	})

	t.Run("invalid IP", func(t *testing.T) {
		input := json.RawMessage(`{"ip":"999.999.999.999"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for invalid IP")
		}
	})
}

func TestExecute_ValidateJSON(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "validate_json")

	t.Run("valid JSON object", func(t *testing.T) {
		input := json.RawMessage(`{"json":"{\"key\":\"value\"}"}`)
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
		if out["type"] != "object" {
			t.Errorf("expected type=object, got %v", out["type"])
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		input := json.RawMessage(`{"json":"{invalid}"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for invalid JSON")
		}
	})
}
