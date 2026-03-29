package memory

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// mockTool implements tool.Tool for testing.
type mockTool struct {
	name        string
	description string
	annotations tool.Annotations
}

func (m *mockTool) Name() string                  { return m.name }
func (m *mockTool) Description() string           { return m.description }
func (m *mockTool) InputSchema() tool.Schema      { return tool.Schema{} }
func (m *mockTool) OutputSchema() tool.Schema     { return tool.Schema{} }
func (m *mockTool) Annotations() tool.Annotations { return m.annotations }
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return tool.Result{}, nil
}

func newMockTool(name string) *mockTool {
	return &mockTool{name: name, description: "Mock " + name}
}

func TestNewToolRegistry(t *testing.T) {
	registry := NewToolRegistry()
	if registry == nil {
		t.Fatal("NewToolRegistry() returned nil")
	}
	if registry.Count() != 0 {
		t.Errorf("NewToolRegistry().Count() = %d, want 0", registry.Count())
	}
}

func TestToolRegistry_Register(t *testing.T) {
	registry := NewToolRegistry()

	t.Run("successful registration", func(t *testing.T) {
		err := registry.Register(newMockTool("test_tool"))
		if err != nil {
			t.Errorf("Register() error = %v, want nil", err)
		}
		if registry.Count() != 1 {
			t.Errorf("Count() = %d, want 1", registry.Count())
		}
	})

	t.Run("duplicate registration", func(t *testing.T) {
		err := registry.Register(newMockTool("test_tool"))
		if err != tool.ErrToolExists {
			t.Errorf("Register() error = %v, want ErrToolExists", err)
		}
	})
}

func TestToolRegistry_Get(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(newMockTool("my_tool"))

	t.Run("existing tool", func(t *testing.T) {
		got, ok := registry.Get("my_tool")
		if !ok {
			t.Error("Get() returned false for existing tool")
		}
		if got.Name() != "my_tool" {
			t.Errorf("Get() name = %q, want %q", got.Name(), "my_tool")
		}
	})

	t.Run("non-existing tool", func(t *testing.T) {
		_, ok := registry.Get("nonexistent")
		if ok {
			t.Error("Get() returned true for non-existing tool")
		}
	})
}

func TestToolRegistry_List(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(newMockTool("tool1"))
	registry.Register(newMockTool("tool2"))
	registry.Register(newMockTool("tool3"))

	tools := registry.List()
	if len(tools) != 3 {
		t.Errorf("List() returned %d tools, want 3", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name()] = true
	}

	for _, expected := range []string{"tool1", "tool2", "tool3"} {
		if !names[expected] {
			t.Errorf("List() missing tool %q", expected)
		}
	}
}

func TestToolRegistry_Names(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(newMockTool("alpha"))
	registry.Register(newMockTool("beta"))

	names := registry.Names()
	if len(names) != 2 {
		t.Errorf("Names() returned %d names, want 2", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	if !nameSet["alpha"] || !nameSet["beta"] {
		t.Errorf("Names() = %v, want [alpha, beta]", names)
	}
}

func TestToolRegistry_Has(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(newMockTool("exists"))

	if !registry.Has("exists") {
		t.Error("Has() returned false for existing tool")
	}
	if registry.Has("not_exists") {
		t.Error("Has() returned true for non-existing tool")
	}
}

func TestToolRegistry_Unregister(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(newMockTool("to_remove"))

	t.Run("unregister existing", func(t *testing.T) {
		err := registry.Unregister("to_remove")
		if err != nil {
			t.Errorf("Unregister() error = %v, want nil", err)
		}
		if registry.Has("to_remove") {
			t.Error("Tool still exists after Unregister()")
		}
	})

	t.Run("unregister non-existing", func(t *testing.T) {
		err := registry.Unregister("nonexistent")
		if err != tool.ErrToolNotFound {
			t.Errorf("Unregister() error = %v, want ErrToolNotFound", err)
		}
	})
}

func TestToolRegistry_Clear(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(newMockTool("tool1"))
	registry.Register(newMockTool("tool2"))

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", registry.Count())
	}
}

func TestToolRegistry_Concurrency(t *testing.T) {
	registry := NewToolRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('a' + i%26))
			registry.Register(newMockTool(name))
			registry.Get(name)
			registry.Has(name)
			registry.List()
			registry.Names()
			registry.Count()
		}(i)
	}
	wg.Wait()

	// Should have at most 26 unique tools
	if registry.Count() > 26 {
		t.Errorf("Count() = %d, want <= 26", registry.Count())
	}
}
