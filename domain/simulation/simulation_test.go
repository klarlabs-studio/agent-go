package simulation_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/simulation"
	"go.klarlabs.de/agent/domain/tool"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	config := simulation.DefaultConfig()

	if !config.Enabled {
		t.Error("DefaultConfig() Enabled should be true")
	}
	if !config.RecordIntents {
		t.Error("DefaultConfig() RecordIntents should be true")
	}
	if config.MockResults == nil {
		t.Error("DefaultConfig() MockResults should be initialized")
	}
	if !config.AllowReadOnly {
		t.Error("DefaultConfig() AllowReadOnly should be true")
	}
	if config.AllowIdempotent {
		t.Error("DefaultConfig() AllowIdempotent should be false")
	}
	if config.DefaultResult != nil {
		t.Error("DefaultConfig() DefaultResult should be nil")
	}
	if config.Recorder != nil {
		t.Error("DefaultConfig() Recorder should be nil")
	}
}

func TestWithMockResult(t *testing.T) {
	t.Parallel()

	t.Run("sets mock result for tool", func(t *testing.T) {
		t.Parallel()

		result := tool.Result{Output: json.RawMessage(`"mocked"`)}
		option := simulation.WithMockResult("read_file", result)

		config := simulation.Config{}
		option(&config)

		if config.MockResults == nil {
			t.Fatal("MockResults should be initialized")
		}
		if mock, ok := config.MockResults["read_file"]; !ok {
			t.Error("MockResults should contain read_file")
		} else if string(mock.Output) != `"mocked"` {
			t.Errorf("MockResults[read_file] = %s, want \"mocked\"", string(mock.Output))
		}
	})

	t.Run("initializes nil MockResults map", func(t *testing.T) {
		t.Parallel()

		config := simulation.Config{MockResults: nil}
		option := simulation.WithMockResult("tool", tool.Result{})
		option(&config)

		if config.MockResults == nil {
			t.Error("WithMockResult should initialize nil map")
		}
	})
}

func TestWithDefaultResult(t *testing.T) {
	t.Parallel()

	result := tool.Result{Output: json.RawMessage(`"default"`)}
	option := simulation.WithDefaultResult(result)

	config := simulation.Config{}
	option(&config)

	if config.DefaultResult == nil {
		t.Fatal("DefaultResult should be set")
	}
	if string(config.DefaultResult.Output) != `"default"` {
		t.Errorf("DefaultResult = %s, want \"default\"", string(config.DefaultResult.Output))
	}
}

func TestWithAllowReadOnly(t *testing.T) {
	t.Parallel()

	t.Run("sets AllowReadOnly to true", func(t *testing.T) {
		t.Parallel()

		config := simulation.Config{AllowReadOnly: false}
		simulation.WithAllowReadOnly(true)(&config)

		if !config.AllowReadOnly {
			t.Error("AllowReadOnly should be true")
		}
	})

	t.Run("sets AllowReadOnly to false", func(t *testing.T) {
		t.Parallel()

		config := simulation.Config{AllowReadOnly: true}
		simulation.WithAllowReadOnly(false)(&config)

		if config.AllowReadOnly {
			t.Error("AllowReadOnly should be false")
		}
	})
}

func TestWithAllowIdempotent(t *testing.T) {
	t.Parallel()

	t.Run("sets AllowIdempotent to true", func(t *testing.T) {
		t.Parallel()

		config := simulation.Config{AllowIdempotent: false}
		simulation.WithAllowIdempotent(true)(&config)

		if !config.AllowIdempotent {
			t.Error("AllowIdempotent should be true")
		}
	})

	t.Run("sets AllowIdempotent to false", func(t *testing.T) {
		t.Parallel()

		config := simulation.Config{AllowIdempotent: true}
		simulation.WithAllowIdempotent(false)(&config)

		if config.AllowIdempotent {
			t.Error("AllowIdempotent should be false")
		}
	})
}

func TestWithRecorder(t *testing.T) {
	t.Parallel()

	recorder := simulation.NewMemoryRecorder()
	option := simulation.WithRecorder(recorder)

	config := simulation.Config{RecordIntents: false}
	option(&config)

	if config.Recorder != recorder {
		t.Error("Recorder should be set")
	}
	if !config.RecordIntents {
		t.Error("WithRecorder should enable RecordIntents")
	}
}

func TestNewMemoryRecorder(t *testing.T) {
	t.Parallel()

	recorder := simulation.NewMemoryRecorder()
	if recorder == nil {
		t.Fatal("NewMemoryRecorder() returned nil")
	}

	intents := recorder.Intents()
	if len(intents) != 0 {
		t.Errorf("NewMemoryRecorder() Intents() len = %d, want 0", len(intents))
	}
}

func TestMemoryRecorder_Record(t *testing.T) {
	t.Parallel()

	recorder := simulation.NewMemoryRecorder()

	intent := simulation.Intent{
		ToolName:  "read_file",
		Input:     json.RawMessage(`{"path":"/test"}`),
		State:     agent.StateExplore,
		Timestamp: time.Now(),
	}

	recorder.Record(intent)

	intents := recorder.Intents()
	if len(intents) != 1 {
		t.Fatalf("Record() Intents() len = %d, want 1", len(intents))
	}
	if intents[0].ToolName != "read_file" {
		t.Errorf("Record() ToolName = %s, want read_file", intents[0].ToolName)
	}
}

func TestMemoryRecorder_Intents(t *testing.T) {
	t.Parallel()

	t.Run("returns copy of intents", func(t *testing.T) {
		t.Parallel()

		recorder := simulation.NewMemoryRecorder()
		recorder.Record(simulation.Intent{ToolName: "tool1"})

		intents := recorder.Intents()
		intents[0].ToolName = "modified"

		originalIntents := recorder.Intents()
		if originalIntents[0].ToolName != "tool1" {
			t.Error("Intents() should return a copy")
		}
	})

	t.Run("returns empty slice for empty recorder", func(t *testing.T) {
		t.Parallel()

		recorder := simulation.NewMemoryRecorder()
		intents := recorder.Intents()

		if intents == nil {
			t.Error("Intents() should return empty slice, not nil")
		}
		if len(intents) != 0 {
			t.Errorf("Intents() len = %d, want 0", len(intents))
		}
	})
}

func TestMemoryRecorder_Clear(t *testing.T) {
	t.Parallel()

	recorder := simulation.NewMemoryRecorder()
	recorder.Record(simulation.Intent{ToolName: "tool1"})
	recorder.Record(simulation.Intent{ToolName: "tool2"})

	if recorder.Len() != 2 {
		t.Fatalf("before Clear() Len() = %d, want 2", recorder.Len())
	}

	recorder.Clear()

	if recorder.Len() != 0 {
		t.Errorf("after Clear() Len() = %d, want 0", recorder.Len())
	}
}

func TestMemoryRecorder_Len(t *testing.T) {
	t.Parallel()

	recorder := simulation.NewMemoryRecorder()

	if recorder.Len() != 0 {
		t.Errorf("empty recorder Len() = %d, want 0", recorder.Len())
	}

	recorder.Record(simulation.Intent{ToolName: "tool1"})
	if recorder.Len() != 1 {
		t.Errorf("recorder with 1 intent Len() = %d, want 1", recorder.Len())
	}

	recorder.Record(simulation.Intent{ToolName: "tool2"})
	recorder.Record(simulation.Intent{ToolName: "tool3"})
	if recorder.Len() != 3 {
		t.Errorf("recorder with 3 intents Len() = %d, want 3", recorder.Len())
	}
}

func TestMemoryRecorder_IntentsByTool(t *testing.T) {
	t.Parallel()

	t.Run("groups intents by tool name", func(t *testing.T) {
		t.Parallel()

		recorder := simulation.NewMemoryRecorder()
		recorder.Record(simulation.Intent{ToolName: "read_file", Input: json.RawMessage(`{"path":"/a"}`)})
		recorder.Record(simulation.Intent{ToolName: "write_file", Input: json.RawMessage(`{"path":"/b"}`)})
		recorder.Record(simulation.Intent{ToolName: "read_file", Input: json.RawMessage(`{"path":"/c"}`)})

		byTool := recorder.IntentsByTool()

		if len(byTool) != 2 {
			t.Fatalf("IntentsByTool() len = %d, want 2", len(byTool))
		}
		if len(byTool["read_file"]) != 2 {
			t.Errorf("IntentsByTool()[read_file] len = %d, want 2", len(byTool["read_file"]))
		}
		if len(byTool["write_file"]) != 1 {
			t.Errorf("IntentsByTool()[write_file] len = %d, want 1", len(byTool["write_file"]))
		}
	})

	t.Run("returns empty map for empty recorder", func(t *testing.T) {
		t.Parallel()

		recorder := simulation.NewMemoryRecorder()
		byTool := recorder.IntentsByTool()

		if byTool == nil {
			t.Error("IntentsByTool() should return empty map, not nil")
		}
		if len(byTool) != 0 {
			t.Errorf("IntentsByTool() len = %d, want 0", len(byTool))
		}
	})
}

func TestMemoryRecorder_BlockedIntents(t *testing.T) {
	t.Parallel()

	t.Run("returns only blocked intents", func(t *testing.T) {
		t.Parallel()

		recorder := simulation.NewMemoryRecorder()
		recorder.Record(simulation.Intent{ToolName: "tool1", Blocked: false})
		recorder.Record(simulation.Intent{ToolName: "tool2", Blocked: true, BlockReason: "destructive"})
		recorder.Record(simulation.Intent{ToolName: "tool3", Blocked: false})
		recorder.Record(simulation.Intent{ToolName: "tool4", Blocked: true, BlockReason: "no permission"})

		blocked := recorder.BlockedIntents()

		if len(blocked) != 2 {
			t.Fatalf("BlockedIntents() len = %d, want 2", len(blocked))
		}
		if blocked[0].ToolName != "tool2" {
			t.Errorf("BlockedIntents()[0].ToolName = %s, want tool2", blocked[0].ToolName)
		}
		if blocked[1].ToolName != "tool4" {
			t.Errorf("BlockedIntents()[1].ToolName = %s, want tool4", blocked[1].ToolName)
		}
	})

	t.Run("returns nil for no blocked intents", func(t *testing.T) {
		t.Parallel()

		recorder := simulation.NewMemoryRecorder()
		recorder.Record(simulation.Intent{ToolName: "tool1", Blocked: false})

		blocked := recorder.BlockedIntents()
		if len(blocked) != 0 {
			t.Errorf("BlockedIntents() len = %d, want 0", len(blocked))
		}
	})
}

func TestMemoryRecorder_Concurrency(t *testing.T) {
	t.Parallel()

	recorder := simulation.NewMemoryRecorder()

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			recorder.Record(simulation.Intent{ToolName: "tool"})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = recorder.Intents()
			_ = recorder.Len()
			_ = recorder.IntentsByTool()
			_ = recorder.BlockedIntents()
		}()
	}

	wg.Wait()

	if recorder.Len() != numGoroutines {
		t.Errorf("after concurrent operations Len() = %d, want %d", recorder.Len(), numGoroutines)
	}
}

func TestIntent(t *testing.T) {
	t.Parallel()

	t.Run("holds all intent data", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		intent := simulation.Intent{
			ToolName:    "delete_file",
			Input:       json.RawMessage(`{"path":"/tmp/test"}`),
			State:       agent.StateAct,
			Timestamp:   now,
			Blocked:     true,
			BlockReason: "destructive action",
			MockResult:  false,
		}

		if intent.ToolName != "delete_file" {
			t.Errorf("ToolName = %s, want delete_file", intent.ToolName)
		}
		if intent.State != agent.StateAct {
			t.Errorf("State = %s, want act", intent.State)
		}
		if !intent.Blocked {
			t.Error("Blocked should be true")
		}
		if intent.BlockReason != "destructive action" {
			t.Errorf("BlockReason = %s", intent.BlockReason)
		}
		if intent.MockResult {
			t.Error("MockResult should be false")
		}
	})

	t.Run("marshals to JSON correctly", func(t *testing.T) {
		t.Parallel()

		intent := simulation.Intent{
			ToolName:    "read_file",
			Input:       json.RawMessage(`{"path":"/test"}`),
			State:       agent.StateExplore,
			Timestamp:   time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Blocked:     false,
			BlockReason: "",
			MockResult:  true,
		}

		data, err := json.Marshal(intent)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		var decoded simulation.Intent
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if decoded.ToolName != intent.ToolName {
			t.Errorf("decoded ToolName = %s, want %s", decoded.ToolName, intent.ToolName)
		}
		if decoded.MockResult != intent.MockResult {
			t.Errorf("decoded MockResult = %v, want %v", decoded.MockResult, intent.MockResult)
		}
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("can be constructed manually", func(t *testing.T) {
		t.Parallel()

		config := simulation.Config{
			Enabled:         true,
			RecordIntents:   true,
			AllowReadOnly:   true,
			AllowIdempotent: true,
			MockResults: map[string]tool.Result{
				"read_file": {Output: json.RawMessage(`"content"`)},
			},
		}

		if !config.Enabled {
			t.Error("Enabled should be true")
		}
		if !config.AllowIdempotent {
			t.Error("AllowIdempotent should be true")
		}
		if config.MockResults["read_file"].Output == nil {
			t.Error("MockResults should contain read_file")
		}
	})
}
