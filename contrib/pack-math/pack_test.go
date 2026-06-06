package mathutil_test

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	mathutil "go.klarlabs.de/agent/contrib/pack-math"
	"go.klarlabs.de/agent/domain/tool"
)

func TestRegister(t *testing.T) {
	p := mathutil.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "math" {
		t.Errorf("expected pack name %q, got %q", "math", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := mathutil.Pack()
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
	p := mathutil.Pack()
	for _, tt := range p.Tools {
		if tt.Name() == name {
			return tt
		}
	}
	t.Fatalf("tool %q not found in pack", name)
	return nil
}

func TestExecute_BasicArithmetic(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "math_basic")

	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"add", `{"a":3,"b":4,"op":"add"}`, 7},
		{"sub", `{"a":10,"b":3,"op":"sub"}`, 7},
		{"mul", `{"a":3,"b":4,"op":"mul"}`, 12},
		{"div", `{"a":12,"b":4,"op":"div"}`, 3},
		{"mod", `{"a":10,"b":3,"op":"mod"}`, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tl.Execute(ctx, json.RawMessage(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]interface{}
			if err := json.Unmarshal(result.Output, &out); err != nil {
				t.Fatalf("failed to unmarshal output: %v", err)
			}
			if out["result"] != tc.expected {
				t.Errorf("expected result=%v, got %v", tc.expected, out["result"])
			}
		})
	}

	t.Run("division by zero", func(t *testing.T) {
		result, err := tl.Execute(ctx, json.RawMessage(`{"a":1,"b":0,"op":"div"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["error"] == nil {
			t.Error("expected error for division by zero")
		}
	})
}

func TestExecute_Round(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "math_round")

	t.Run("round to 2 decimals", func(t *testing.T) {
		input := json.RawMessage(`{"value":3.14159,"precision":2}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["rounded"] != 3.14 {
			t.Errorf("expected rounded=3.14, got %v", out["rounded"])
		}
	})

	t.Run("floor", func(t *testing.T) {
		input := json.RawMessage(`{"value":3.9,"precision":0,"mode":"floor"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["rounded"] != float64(3) {
			t.Errorf("expected rounded=3, got %v", out["rounded"])
		}
	})
}

func TestExecute_Stats(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "math_stats")

	t.Run("basic statistics", func(t *testing.T) {
		input := json.RawMessage(`{"values":[1,2,3,4,5]}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["mean"] != float64(3) {
			t.Errorf("expected mean=3, got %v", out["mean"])
		}
		if out["median"] != float64(3) {
			t.Errorf("expected median=3, got %v", out["median"])
		}
		if out["sum"] != float64(15) {
			t.Errorf("expected sum=15, got %v", out["sum"])
		}
		if out["min"] != float64(1) {
			t.Errorf("expected min=1, got %v", out["min"])
		}
		if out["max"] != float64(5) {
			t.Errorf("expected max=5, got %v", out["max"])
		}
		// Variance of [1,2,3,4,5] is 2.0
		if out["variance"] != float64(2) {
			t.Errorf("expected variance=2, got %v", out["variance"])
		}
		// Std dev should be sqrt(2)
		stdDev, ok := out["std_dev"].(float64)
		if !ok || math.Abs(stdDev-math.Sqrt(2)) > 0.0001 {
			t.Errorf("expected std_dev=sqrt(2), got %v", out["std_dev"])
		}
	})

	t.Run("empty values", func(t *testing.T) {
		input := json.RawMessage(`{"values":[]}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["error"] == nil {
			t.Error("expected error for empty values")
		}
	})
}

func TestExecute_Percent(t *testing.T) {
	ctx := context.Background()
	tl := findTool(t, "math_percent")

	t.Run("what percent is value of total", func(t *testing.T) {
		input := json.RawMessage(`{"value":25,"total":100,"op":"from"}`)
		result, err := tl.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(result.Output, &out); err != nil {
			t.Fatalf("failed to unmarshal output: %v", err)
		}
		if out["result"] != float64(25) {
			t.Errorf("expected result=25, got %v", out["result"])
		}
	})
}
