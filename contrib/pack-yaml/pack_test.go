package yaml

import (
	"context"
	"encoding/json"
	stdtesting "testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestPack(t *stdtesting.T) {
	p := Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Name != "yaml" {
		t.Errorf("expected pack name 'yaml', got %q", p.Name)
	}
	if len(p.Tools) == 0 {
		t.Fatal("expected at least 1 tool, got 0")
	}
}

func TestToolsImplementInterface(t *stdtesting.T) {
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
func findTool(t *stdtesting.T, name string) tool.Tool {
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

func TestExecute_YAMLParse(t *stdtesting.T) {
	ctx := context.Background()
	tl := findTool(t, "yaml_parse")

	t.Run("valid YAML", func(t *stdtesting.T) {
		input := json.RawMessage(`{"content":"name: alice\nage: 30"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		data, ok := out["data"].(map[string]interface{})
		if !ok {
			t.Fatal("expected data to be an object")
		}
		if data["name"] != "alice" {
			t.Errorf("expected name=alice, got %v", data["name"])
		}
	})

	t.Run("empty content", func(t *stdtesting.T) {
		input := json.RawMessage(`{"content":""}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for empty content")
		}
	})
}

func TestExecute_YAMLStringify(t *stdtesting.T) {
	ctx := context.Background()
	tl := findTool(t, "yaml_stringify")

	t.Run("object to YAML", func(t *stdtesting.T) {
		input := json.RawMessage(`{"data":{"name":"bob","age":25}}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		yamlStr, ok := out["yaml"].(string)
		if !ok || yamlStr == "" {
			t.Error("expected non-empty yaml output")
		}
	})
}

func TestExecute_YAMLToJSON(t *stdtesting.T) {
	ctx := context.Background()
	tl := findTool(t, "yaml_to_json")

	t.Run("convert YAML to JSON", func(t *stdtesting.T) {
		input := json.RawMessage(`{"yaml":"name: alice\nage: 30","pretty":true}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		jsonStr, ok := out["json"].(string)
		if !ok || jsonStr == "" {
			t.Error("expected non-empty json output")
		}
	})

	t.Run("empty YAML", func(t *stdtesting.T) {
		input := json.RawMessage(`{"yaml":""}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for empty yaml")
		}
	})
}

func TestExecute_YAMLFromJSON(t *stdtesting.T) {
	ctx := context.Background()
	tl := findTool(t, "yaml_from_json")

	t.Run("convert JSON to YAML", func(t *stdtesting.T) {
		input := json.RawMessage(`{"json":{"name":"alice","age":30}}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		yamlStr, ok := out["yaml"].(string)
		if !ok || yamlStr == "" {
			t.Error("expected non-empty yaml output")
		}
	})
}

func TestExecute_YAMLValidate(t *stdtesting.T) {
	ctx := context.Background()
	tl := findTool(t, "yaml_validate")

	t.Run("valid YAML", func(t *stdtesting.T) {
		input := json.RawMessage(`{"content":"key: value\nlist:\n  - item1\n  - item2"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] != true {
			t.Error("expected valid=true for valid YAML")
		}
	})

	t.Run("invalid YAML", func(t *stdtesting.T) {
		input := json.RawMessage(`{"content":":\n  - :\n    bad: ["}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["valid"] == true {
			t.Error("expected valid=false for invalid YAML")
		}
	})
}

func TestExecute_YAMLMerge(t *stdtesting.T) {
	ctx := context.Background()
	tl := findTool(t, "yaml_merge")

	t.Run("merge two documents", func(t *stdtesting.T) {
		input := json.RawMessage(`{"documents":["a: 1","b: 2"]}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["yaml"] == nil {
			t.Error("expected yaml field in output")
		}
	})

	t.Run("too few documents", func(t *stdtesting.T) {
		input := json.RawMessage(`{"documents":["a: 1"]}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for fewer than 2 documents")
		}
	})
}
