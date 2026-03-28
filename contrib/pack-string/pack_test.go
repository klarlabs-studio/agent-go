package stringutil

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestPack(t *testing.T) {
	p := Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Name != "string" {
		t.Errorf("expected pack name 'string', got %q", p.Name)
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

func TestExecute_StringReverse(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "string_reverse")

	t.Run("reverse string", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["reversed"] != "olleh" {
			t.Errorf("expected reversed=olleh, got %v", out["reversed"])
		}
	})
}

func TestExecute_StringTruncate(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "string_truncate")

	t.Run("truncate long string", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello world this is long","length":10}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["was_truncated"] != true {
			t.Error("expected was_truncated=true")
		}
	})

	t.Run("short string unchanged", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hi","length":10}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["was_truncated"] != false {
			t.Error("expected was_truncated=false for short string")
		}
	})
}

func TestExecute_StringPad(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "string_pad")

	t.Run("pad left with zeros", func(t *testing.T) {
		input := json.RawMessage(`{"text":"42","length":5,"char":"0","side":"left"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["padded"] != "00042" {
			t.Errorf("expected padded=00042, got %v", out["padded"])
		}
	})
}

func TestExecute_StringCount(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "string_count")

	t.Run("count text metrics", func(t *testing.T) {
		input := json.RawMessage(`{"text":"Hello world. How are you?"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["words"] != float64(5) {
			t.Errorf("expected words=5, got %v", out["words"])
		}
		if out["characters"] != float64(25) {
			t.Errorf("expected characters=25, got %v", out["characters"])
		}
	})
}

func TestExecute_StringContains(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "string_contains")

	t.Run("contains substring", func(t *testing.T) {
		input := json.RawMessage(`{"text":"hello world","substring":"world"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["contains"] != true {
			t.Error("expected contains=true")
		}
		if out["count"] != float64(1) {
			t.Errorf("expected count=1, got %v", out["count"])
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		input := json.RawMessage(`{"text":"Hello World","substring":"hello","ignore_case":true}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["contains"] != true {
			t.Error("expected contains=true for case-insensitive match")
		}
	})
}

func TestExecute_StringReplace(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "string_replace")

	t.Run("replace all occurrences", func(t *testing.T) {
		input := json.RawMessage(`{"text":"aabaa","old":"a","new":"x"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["replaced"] != "xxbxx" {
			t.Errorf("expected replaced=xxbxx, got %v", out["replaced"])
		}
	})
}
