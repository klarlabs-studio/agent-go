package aiplugin

import (
	"sync"
	"testing"

	"go.klarlabs.de/statekit/plugin"
)

type ctx struct{}

func TestTokenCounter_AccumulatesTokens(t *testing.T) {
	t.Parallel()
	tc := NewTokenCounter[ctx]()

	tc.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Type: "LLM_CALL",
		Payload: map[string]any{
			KeyInputTokens:  100,
			KeyOutputTokens: 50,
		},
	})
	tc.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Type: "LLM_CALL",
		Payload: map[string]any{
			KeyInputTokens:  float64(200), // simulate JSON-decoded number
			KeyOutputTokens: float64(75),
		},
	})

	if got, want := tc.InputTokens(), int64(300); got != want {
		t.Errorf("InputTokens = %d, want %d", got, want)
	}
	if got, want := tc.OutputTokens(), int64(125); got != want {
		t.Errorf("OutputTokens = %d, want %d", got, want)
	}
	if got, want := tc.TotalTokens(), int64(425); got != want {
		t.Errorf("TotalTokens = %d, want %d", got, want)
	}
}

func TestTokenCounter_AccumulatesCost(t *testing.T) {
	t.Parallel()
	tc := NewTokenCounter[ctx]()

	tc.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Payload: map[string]any{
			KeyInputCost:  0.0015,
			KeyOutputCost: 0.0075,
		},
	})
	tc.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Payload: map[string]any{
			KeyInputCost:  0.0010,
			KeyOutputCost: 0.0050,
		},
	})

	got := tc.CostUSD()
	want := 0.015
	if abs(got-want) > 1e-9 {
		t.Errorf("CostUSD = %v, want %v", got, want)
	}
}

func TestTokenCounter_Reset(t *testing.T) {
	t.Parallel()
	tc := NewTokenCounter[ctx]()
	tc.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Payload: map[string]any{
			KeyInputTokens: 10,
			KeyInputCost:   1.0,
		},
	})

	tc.Reset()
	if tc.TotalTokens() != 0 {
		t.Errorf("after Reset, TotalTokens = %d, want 0", tc.TotalTokens())
	}
	if tc.CostUSD() != 0 {
		t.Errorf("after Reset, CostUSD = %v, want 0", tc.CostUSD())
	}
}

func TestTokenCounter_NoPayload_NoOp(t *testing.T) {
	t.Parallel()
	tc := NewTokenCounter[ctx]()
	tc.OnEvent(plugin.Context[ctx]{}, plugin.Event{Type: "TICK"})
	if tc.TotalTokens() != 0 {
		t.Errorf("expected 0 tokens for empty payload, got %d", tc.TotalTokens())
	}
}

func TestTokenCounter_BadTypes_Skipped(t *testing.T) {
	t.Parallel()
	tc := NewTokenCounter[ctx]()
	tc.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Payload: map[string]any{
			KeyInputTokens:  "not a number",
			KeyOutputTokens: []int{1},
			KeyInputCost:    "nope",
		},
	})
	if tc.TotalTokens() != 0 {
		t.Errorf("expected bad types to be skipped, got %d", tc.TotalTokens())
	}
}

func TestTokenCounter_Concurrent(t *testing.T) {
	t.Parallel()
	tc := NewTokenCounter[ctx]()

	var wg sync.WaitGroup
	const goroutines = 16
	const perG = 1000
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				tc.OnEvent(plugin.Context[ctx]{}, plugin.Event{
					Payload: map[string]any{
						KeyInputTokens: 1,
						KeyInputCost:   0.001,
					},
				})
			}
		}()
	}
	wg.Wait()

	if got := tc.InputTokens(); got != goroutines*perG {
		t.Errorf("InputTokens = %d, want %d", got, goroutines*perG)
	}
}

func TestPromptRecorder_Captures(t *testing.T) {
	t.Parallel()
	pr := NewPromptRecorder[ctx]()
	pr.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Payload: map[string]any{
			KeyModel:    "claude-opus-4-7",
			KeyPrompt:   "Hello",
			KeyResponse: "World",
		},
	})
	pr.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Payload: map[string]any{
			KeyPrompt: "second",
		},
	})

	if pr.Len() != 2 {
		t.Fatalf("Len = %d, want 2", pr.Len())
	}

	snaps := pr.Snapshots()
	if snaps[0].Model != "claude-opus-4-7" {
		t.Errorf("snap[0].Model = %q", snaps[0].Model)
	}
	if snaps[0].Prompt != "Hello" || snaps[0].Response != "World" {
		t.Errorf("snap[0] = %+v", snaps[0])
	}
	if snaps[1].Prompt != "second" {
		t.Errorf("snap[1].Prompt = %q", snaps[1].Prompt)
	}
}

func TestPromptRecorder_SkipsEmpty(t *testing.T) {
	t.Parallel()
	pr := NewPromptRecorder[ctx]()
	pr.OnEvent(plugin.Context[ctx]{}, plugin.Event{Type: "TICK"})
	pr.OnEvent(plugin.Context[ctx]{}, plugin.Event{Payload: map[string]any{}})
	pr.OnEvent(plugin.Context[ctx]{}, plugin.Event{Payload: map[string]any{KeyModel: "x"}})

	if pr.Len() != 0 {
		t.Errorf("expected 0 snapshots, got %d", pr.Len())
	}
}

func TestPromptRecorder_Reset(t *testing.T) {
	t.Parallel()
	pr := NewPromptRecorder[ctx]()
	pr.OnEvent(plugin.Context[ctx]{}, plugin.Event{
		Payload: map[string]any{KeyPrompt: "x"},
	})
	pr.Reset()
	if pr.Len() != 0 {
		t.Errorf("expected 0 after Reset, got %d", pr.Len())
	}
}

// PluginInterfaceCheck — compile-time verification that types satisfy
// the plugin OnEventHook interface.
func TestPluginInterfaces(t *testing.T) {
	t.Parallel()
	var _ plugin.OnEventHook[ctx] = NewTokenCounter[ctx]()
	var _ plugin.OnEventHook[ctx] = NewPromptRecorder[ctx]()

	if NewTokenCounter[ctx]().Name() != "ai-token-counter" {
		t.Error("token counter name mismatch")
	}
	if NewPromptRecorder[ctx]().Name() != "ai-prompt-recorder" {
		t.Error("prompt recorder name mismatch")
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
