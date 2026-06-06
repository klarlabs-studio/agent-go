package regex

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
	if p.Name != "regex" {
		t.Errorf("expected pack name 'regex', got %q", p.Name)
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

func TestExecute_RegexMatch(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "regex_match")

	t.Run("match found", func(t *testing.T) {
		input := json.RawMessage(`{"pattern":"\\d+","text":"abc123def"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["matches"] != true {
			t.Error("expected matches=true")
		}
		if out["match"] != "123" {
			t.Errorf("expected match=123, got %v", out["match"])
		}
	})

	t.Run("no match", func(t *testing.T) {
		input := json.RawMessage(`{"pattern":"\\d+","text":"abcdef"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["matches"] != false {
			t.Error("expected matches=false")
		}
	})

	t.Run("missing pattern", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello"}`)
		_, err := tl.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error for missing pattern")
		}
	})
}

func TestExecute_RegexExtractGroups(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "regex_extract_groups")

	t.Run("named groups", func(t *testing.T) {
		input := json.RawMessage(`{"pattern":"(?P<year>\\d{4})-(?P<month>\\d{2})-(?P<day>\\d{2})","text":"date: 2024-01-15"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["found"] != true {
			t.Fatal("expected found=true")
		}
		named, ok := out["named_groups"].(map[string]interface{})
		if !ok {
			t.Fatal("expected named_groups to be a map")
		}
		if named["year"] != "2024" {
			t.Errorf("expected year=2024, got %v", named["year"])
		}
	})
}

func TestExecute_RegexReplace(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "regex_replace")

	t.Run("replace first match", func(t *testing.T) {
		input := json.RawMessage(`{"pattern":"\\d+","text":"abc123def456","replacement":"NUM"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["output"] != "abcNUMdef456" {
			t.Errorf("expected abcNUMdef456, got %v", out["output"])
		}
		if out["replaced"] != true {
			t.Error("expected replaced=true")
		}
	})
}

func TestExecute_RegexReplaceAll(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "regex_replace_all")

	t.Run("replace all matches", func(t *testing.T) {
		input := json.RawMessage(`{"pattern":"\\d+","text":"abc123def456","replacement":"NUM"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["output"] != "abcNUMdefNUM" {
			t.Errorf("expected abcNUMdefNUM, got %v", out["output"])
		}
	})
}

func TestExecute_RegexCountMatches(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "regex_count_matches")

	t.Run("count matches", func(t *testing.T) {
		input := json.RawMessage(`{"pattern":"\\d+","text":"a1b2c3"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["count"] != float64(3) {
			t.Errorf("expected count=3, got %v", out["count"])
		}
	})
}
