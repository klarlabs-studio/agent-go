package pack_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	domainpack "go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
	infrapack "go.klarlabs.de/agent/infrastructure/pack"
)

// mockTool implements tool.Tool for testing.
type mockTool struct {
	name string
}

func (m *mockTool) Name() string              { return m.name }
func (m *mockTool) Description() string       { return "mock tool" }
func (m *mockTool) InputSchema() tool.Schema  { return tool.Schema{} }
func (m *mockTool) OutputSchema() tool.Schema { return tool.Schema{} }
func (m *mockTool) Annotations() tool.Annotations {
	return tool.Annotations{}
}
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return tool.Result{}, nil
}

// mockToolRegistry implements tool.Registry for testing.
type mockToolRegistry struct {
	tools map[string]tool.Tool
}

func newMockToolRegistry() *mockToolRegistry {
	return &mockToolRegistry{tools: make(map[string]tool.Tool)}
}

func (r *mockToolRegistry) Register(t tool.Tool) error {
	if _, exists := r.tools[t.Name()]; exists {
		return errors.New("tool already exists")
	}
	r.tools[t.Name()] = t
	return nil
}

func (r *mockToolRegistry) Get(name string) (tool.Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *mockToolRegistry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

func (r *mockToolRegistry) List() []tool.Tool {
	result := make([]tool.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *mockToolRegistry) Names() []string {
	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}

func (r *mockToolRegistry) Unregister(name string) error {
	delete(r.tools, name)
	return nil
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	t.Run("creates empty registry", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		if reg == nil {
			t.Fatal("NewRegistry() returned nil")
		}
		if reg.Len() != 0 {
			t.Errorf("Len() = %d, want 0", reg.Len())
		}
	})
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("registers valid pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		p := &domainpack.Pack{
			Name:        "test-pack",
			Description: "A test pack",
		}

		err := reg.Register(p)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if reg.Len() != 1 {
			t.Errorf("Len() = %d, want 1", reg.Len())
		}
	})

	t.Run("returns error for nil pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()

		err := reg.Register(nil)
		if err == nil {
			t.Error("Register(nil) should return error")
		}
		if !errors.Is(err, domainpack.ErrInvalidPack) {
			t.Errorf("error = %v, want ErrInvalidPack", err)
		}
	})

	t.Run("returns error for pack with empty name", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		p := &domainpack.Pack{Name: ""}

		err := reg.Register(p)
		if err == nil {
			t.Error("Register(empty name) should return error")
		}
		if !errors.Is(err, domainpack.ErrInvalidPack) {
			t.Errorf("error = %v, want ErrInvalidPack", err)
		}
	})

	t.Run("returns error for duplicate pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		p := &domainpack.Pack{Name: "test-pack"}

		_ = reg.Register(p)
		err := reg.Register(p)

		if err == nil {
			t.Error("Register(duplicate) should return error")
		}
		if !errors.Is(err, domainpack.ErrPackExists) {
			t.Errorf("error = %v, want ErrPackExists", err)
		}
	})
}

func TestRegistry_Get(t *testing.T) {
	t.Parallel()

	t.Run("retrieves registered pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		p := &domainpack.Pack{Name: "test-pack", Description: "desc"}
		_ = reg.Register(p)

		got, ok := reg.Get("test-pack")
		if !ok {
			t.Fatal("Get() returned false")
		}
		if got.Name != "test-pack" {
			t.Errorf("Name = %s, want test-pack", got.Name)
		}
	})

	t.Run("returns false for missing pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()

		_, ok := reg.Get("nonexistent")
		if ok {
			t.Error("Get(nonexistent) should return false")
		}
	})
}

func TestRegistry_List(t *testing.T) {
	t.Parallel()

	t.Run("lists all registered packs", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		_ = reg.Register(&domainpack.Pack{Name: "pack1"})
		_ = reg.Register(&domainpack.Pack{Name: "pack2"})
		_ = reg.Register(&domainpack.Pack{Name: "pack3"})

		packs := reg.List()
		if len(packs) != 3 {
			t.Errorf("List() returned %d packs, want 3", len(packs))
		}
	})

	t.Run("returns empty list for empty registry", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()

		packs := reg.List()
		if len(packs) != 0 {
			t.Errorf("List() returned %d packs, want 0", len(packs))
		}
	})
}

func TestRegistry_Unregister(t *testing.T) {
	t.Parallel()

	t.Run("removes registered pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		_ = reg.Register(&domainpack.Pack{Name: "test-pack"})

		err := reg.Unregister("test-pack")
		if err != nil {
			t.Fatalf("Unregister() error = %v", err)
		}

		_, ok := reg.Get("test-pack")
		if ok {
			t.Error("pack should be unregistered")
		}
	})

	t.Run("returns error for missing pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()

		err := reg.Unregister("nonexistent")
		if err == nil {
			t.Error("Unregister(nonexistent) should return error")
		}
		if !errors.Is(err, domainpack.ErrPackNotFound) {
			t.Errorf("error = %v, want ErrPackNotFound", err)
		}
	})
}

func TestRegistry_Install(t *testing.T) {
	t.Parallel()

	t.Run("installs pack tools and eligibility", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		toolReg := newMockToolRegistry()
		eligibility := policy.NewToolEligibility()

		testTool := &mockTool{name: "test-tool"}
		p := &domainpack.Pack{
			Name:  "test-pack",
			Tools: []tool.Tool{testTool},
			Eligibility: map[agent.State][]string{
				agent.StateExplore: {"test-tool"},
			},
		}
		_ = reg.Register(p)

		err := reg.Install("test-pack", toolReg, eligibility)
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		if !toolReg.Has("test-tool") {
			t.Error("tool should be registered")
		}
		if !eligibility.IsAllowed(agent.StateExplore, "test-tool") {
			t.Error("tool should be allowed in explore state")
		}
	})

	t.Run("returns error for missing pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		toolReg := newMockToolRegistry()
		eligibility := policy.NewToolEligibility()

		err := reg.Install("nonexistent", toolReg, eligibility)
		if err == nil {
			t.Error("Install(nonexistent) should return error")
		}
		if !errors.Is(err, domainpack.ErrPackNotFound) {
			t.Errorf("error = %v, want ErrPackNotFound", err)
		}
	})

	t.Run("returns error for missing dependency", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		toolReg := newMockToolRegistry()
		eligibility := policy.NewToolEligibility()

		p := &domainpack.Pack{
			Name:         "test-pack",
			Dependencies: []string{"missing-dep"},
		}
		_ = reg.Register(p)

		err := reg.Install("test-pack", toolReg, eligibility)
		if err == nil {
			t.Error("Install() with missing dependency should return error")
		}
		if !errors.Is(err, domainpack.ErrDependencyNotFound) {
			t.Errorf("error = %v, want ErrDependencyNotFound", err)
		}
	})

	t.Run("skips already registered tools", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		toolReg := newMockToolRegistry()
		eligibility := policy.NewToolEligibility()

		// Pre-register tool
		existingTool := &mockTool{name: "existing-tool"}
		_ = toolReg.Register(existingTool)

		p := &domainpack.Pack{
			Name:  "test-pack",
			Tools: []tool.Tool{&mockTool{name: "existing-tool"}},
		}
		_ = reg.Register(p)

		err := reg.Install("test-pack", toolReg, eligibility)
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}
	})
}

func TestRegistry_InstallPack(t *testing.T) {
	t.Parallel()

	t.Run("installs pack directly", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		toolReg := newMockToolRegistry()
		eligibility := policy.NewToolEligibility()

		testTool := &mockTool{name: "direct-tool"}
		p := &domainpack.Pack{
			Name:  "direct-pack",
			Tools: []tool.Tool{testTool},
			Eligibility: map[agent.State][]string{
				agent.StateAct: {"direct-tool"},
			},
		}

		err := reg.InstallPack(p, toolReg, eligibility)
		if err != nil {
			t.Fatalf("InstallPack() error = %v", err)
		}

		if !toolReg.Has("direct-tool") {
			t.Error("tool should be registered")
		}
		if !eligibility.IsAllowed(agent.StateAct, "direct-tool") {
			t.Error("tool should be allowed in act state")
		}
	})

	t.Run("returns error for nil pack", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		toolReg := newMockToolRegistry()
		eligibility := policy.NewToolEligibility()

		err := reg.InstallPack(nil, toolReg, eligibility)
		if err == nil {
			t.Error("InstallPack(nil) should return error")
		}
		if !errors.Is(err, domainpack.ErrInvalidPack) {
			t.Errorf("error = %v, want ErrInvalidPack", err)
		}
	})

	t.Run("skips already registered tools", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		toolReg := newMockToolRegistry()
		eligibility := policy.NewToolEligibility()

		// Pre-register tool
		existingTool := &mockTool{name: "existing"}
		_ = toolReg.Register(existingTool)

		p := &domainpack.Pack{
			Name:  "pack",
			Tools: []tool.Tool{&mockTool{name: "existing"}},
		}

		err := reg.InstallPack(p, toolReg, eligibility)
		if err != nil {
			t.Fatalf("InstallPack() error = %v", err)
		}
	})
}

func TestRegistry_Clear(t *testing.T) {
	t.Parallel()

	t.Run("clears all packs", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()
		_ = reg.Register(&domainpack.Pack{Name: "pack1"})
		_ = reg.Register(&domainpack.Pack{Name: "pack2"})

		reg.Clear()

		if reg.Len() != 0 {
			t.Errorf("Len() = %d after Clear, want 0", reg.Len())
		}
	})
}

func TestRegistry_Len(t *testing.T) {
	t.Parallel()

	t.Run("returns correct count", func(t *testing.T) {
		t.Parallel()

		reg := infrapack.NewRegistry()

		if reg.Len() != 0 {
			t.Errorf("Len() = %d, want 0", reg.Len())
		}

		_ = reg.Register(&domainpack.Pack{Name: "pack1"})
		if reg.Len() != 1 {
			t.Errorf("Len() = %d, want 1", reg.Len())
		}

		_ = reg.Register(&domainpack.Pack{Name: "pack2"})
		if reg.Len() != 2 {
			t.Errorf("Len() = %d, want 2", reg.Len())
		}
	})
}
