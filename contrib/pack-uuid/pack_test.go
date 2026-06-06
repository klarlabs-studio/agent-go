package uuid

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestPack(t *testing.T) {
	p := Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Name != "uuid" {
		t.Errorf("expected pack name 'uuid', got %q", p.Name)
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

func TestExecute_GenerateV4(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "uuid_generate_v4")

	t.Run("generate UUID v4", func(t *testing.T) {
		input := json.RawMessage(`{}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		uuidStr, ok := out["uuid"].(string)
		if !ok || uuidStr == "" {
			t.Fatal("expected non-empty uuid")
		}
		// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
		parts := strings.Split(uuidStr, "-")
		if len(parts) != 5 {
			t.Errorf("expected 5 parts in UUID, got %d", len(parts))
		}
		if out["version"] != float64(4) {
			t.Errorf("expected version=4, got %v", out["version"])
		}
	})
}

func TestExecute_Validate(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "uuid_validate")

	t.Run("valid UUID", func(t *testing.T) {
		input := json.RawMessage(`{"uuid":"550e8400-e29b-41d4-a716-446655440000"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Error("expected valid=true for valid UUID")
		}
	})

	t.Run("invalid UUID", func(t *testing.T) {
		input := json.RawMessage(`{"uuid":"not-a-uuid"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for invalid UUID")
		}
	})
}

func TestExecute_Parse(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "uuid_parse")

	t.Run("parse valid UUID", func(t *testing.T) {
		input := json.RawMessage(`{"uuid":"550e8400-e29b-41d4-a716-446655440000"}`)
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
		if out["version"] == nil {
			t.Error("expected version in output")
		}
	})

	t.Run("parse invalid UUID", func(t *testing.T) {
		input := json.RawMessage(`{"uuid":"invalid"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != false {
			t.Error("expected valid=false for invalid UUID")
		}
	})
}

func TestExecute_NilUUID(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "uuid_nil")

	t.Run("get nil UUID", func(t *testing.T) {
		input := json.RawMessage(`{}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["nil_uuid"] != "00000000-0000-0000-0000-000000000000" {
			t.Errorf("expected nil UUID, got %v", out["nil_uuid"])
		}
	})
}
