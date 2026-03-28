package json_test

import (
	"context"
	"encoding/json"
	"testing"

	packjson "github.com/felixgeelhaar/agent-go/contrib/pack-json"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestRegister(t *testing.T) {
	p := packjson.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "json" {
		t.Errorf("expected pack name %q, got %q", "json", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := packjson.Pack()
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
	p := packjson.Pack()
	for _, tt := range p.Tools {
		if tt.Name() == name {
			return tt
		}
	}
	t.Fatalf("tool %q not found in pack", name)
	return nil
}

func TestExecute_JSONQuery(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "json_query")

	t.Run("valid query", func(t *testing.T) {
		input := json.RawMessage(`{"data":{"name":"alice","age":30},"path":"$.name"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Output == nil {
			t.Fatal("expected non-nil output")
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["count"] == nil {
			t.Error("expected count in output")
		}
	})

	t.Run("missing path", func(t *testing.T) {
		input := json.RawMessage(`{"data":{"name":"alice"}}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for missing path")
		}
	})
}

func TestExecute_JSONFormat(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "json_format")

	t.Run("format compact JSON", func(t *testing.T) {
		input := json.RawMessage(`{"data":{"a":1,"b":2}}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		formatted, ok := out["formatted"].(string)
		if !ok || formatted == "" {
			t.Error("expected non-empty formatted output")
		}
	})
}

func TestExecute_JSONMerge(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "json_merge")

	t.Run("merge two objects", func(t *testing.T) {
		input := json.RawMessage(`{"objects":[{"a":1},{"b":2}]}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		merged, ok := out["result"].(map[string]interface{})
		if !ok {
			t.Fatal("expected result to be an object")
		}
		if merged["a"] == nil || merged["b"] == nil {
			t.Error("expected both keys in merged result")
		}
	})

	t.Run("too few objects", func(t *testing.T) {
		input := json.RawMessage(`{"objects":[{"a":1}]}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for fewer than 2 objects")
		}
	})
}

func TestExecute_JSONValidateSchema(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "json_validate_schema")

	t.Run("valid against schema", func(t *testing.T) {
		input := json.RawMessage(`{
			"data": {"name":"alice","age":30},
			"schema": {"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name"]}
		}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Errorf("expected valid=true, got %v", out["valid"])
		}
	})
}

func TestExecute_JSONMinify(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "json_minify")

	t.Run("minify JSON", func(t *testing.T) {
		input := json.RawMessage(`{"data":{"key": "value", "num": 42}}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["minified"] == nil {
			t.Error("expected minified field in output")
		}
	})
}

func TestExecute_JSONDiff(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "json_diff")

	t.Run("identical objects", func(t *testing.T) {
		input := json.RawMessage(`{"a":{"x":1},"b":{"x":1}}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["equal"] != true {
			t.Error("expected equal=true for identical objects")
		}
	})

	t.Run("different objects", func(t *testing.T) {
		input := json.RawMessage(`{"a":{"x":1},"b":{"x":2}}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["equal"] == true {
			t.Error("expected equal=false for different objects")
		}
	})
}
