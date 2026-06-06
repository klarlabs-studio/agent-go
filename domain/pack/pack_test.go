package pack_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// mockTool implements tool.Tool for testing
type mockTool struct {
	name string
}

func (m mockTool) Name() string                  { return m.name }
func (m mockTool) Description() string           { return "mock tool" }
func (m mockTool) Annotations() tool.Annotations { return tool.Annotations{} }
func (m mockTool) InputSchema() tool.Schema      { return tool.Schema{} }
func (m mockTool) OutputSchema() tool.Schema     { return tool.Schema{} }
func (m mockTool) Execute(context.Context, json.RawMessage) (tool.Result, error) {
	return tool.Result{}, nil
}

func TestPack_ToolNames(t *testing.T) {
	t.Parallel()

	t.Run("returns empty slice for pack with no tools", func(t *testing.T) {
		t.Parallel()

		p := &pack.Pack{}
		names := p.ToolNames()
		if len(names) != 0 {
			t.Errorf("ToolNames() len = %d, want 0", len(names))
		}
	})

	t.Run("returns all tool names", func(t *testing.T) {
		t.Parallel()

		p := &pack.Pack{
			Tools: []tool.Tool{
				mockTool{name: "read_file"},
				mockTool{name: "write_file"},
				mockTool{name: "delete_file"},
			},
		}

		names := p.ToolNames()
		if len(names) != 3 {
			t.Fatalf("ToolNames() len = %d, want 3", len(names))
		}
		if names[0] != "read_file" {
			t.Errorf("ToolNames()[0] = %s, want read_file", names[0])
		}
		if names[1] != "write_file" {
			t.Errorf("ToolNames()[1] = %s, want write_file", names[1])
		}
		if names[2] != "delete_file" {
			t.Errorf("ToolNames()[2] = %s, want delete_file", names[2])
		}
	})
}

func TestPack_GetTool(t *testing.T) {
	t.Parallel()

	t.Run("returns tool by name", func(t *testing.T) {
		t.Parallel()

		p := &pack.Pack{
			Tools: []tool.Tool{
				mockTool{name: "read_file"},
				mockTool{name: "write_file"},
			},
		}

		found, ok := p.GetTool("write_file")
		if !ok {
			t.Fatal("GetTool() should find write_file")
		}
		if found.Name() != "write_file" {
			t.Errorf("GetTool() Name() = %s, want write_file", found.Name())
		}
	})

	t.Run("returns false for non-existent tool", func(t *testing.T) {
		t.Parallel()

		p := &pack.Pack{
			Tools: []tool.Tool{
				mockTool{name: "read_file"},
			},
		}

		_, ok := p.GetTool("nonexistent")
		if ok {
			t.Error("GetTool() should return false for nonexistent tool")
		}
	})

	t.Run("returns false for empty pack", func(t *testing.T) {
		t.Parallel()

		p := &pack.Pack{}
		_, ok := p.GetTool("any")
		if ok {
			t.Error("GetTool() should return false for empty pack")
		}
	})
}

func TestPack_AllowedInState(t *testing.T) {
	t.Parallel()

	t.Run("returns allowed tools for state", func(t *testing.T) {
		t.Parallel()

		p := &pack.Pack{
			Eligibility: map[agent.State][]string{
				agent.StateExplore: {"read_file", "list_files"},
				agent.StateAct:     {"write_file"},
			},
		}

		allowed := p.AllowedInState(agent.StateExplore)
		if len(allowed) != 2 {
			t.Fatalf("AllowedInState() len = %d, want 2", len(allowed))
		}
		if allowed[0] != "read_file" {
			t.Errorf("AllowedInState()[0] = %s, want read_file", allowed[0])
		}
	})

	t.Run("returns nil for state with no allowed tools", func(t *testing.T) {
		t.Parallel()

		p := &pack.Pack{
			Eligibility: map[agent.State][]string{
				agent.StateExplore: {"read_file"},
			},
		}

		allowed := p.AllowedInState(agent.StateAct)
		if allowed != nil {
			t.Errorf("AllowedInState() = %v, want nil", allowed)
		}
	})

	t.Run("returns nil for empty eligibility map", func(t *testing.T) {
		t.Parallel()

		p := &pack.Pack{}
		allowed := p.AllowedInState(agent.StateExplore)
		if allowed != nil {
			t.Errorf("AllowedInState() = %v, want nil", allowed)
		}
	})
}

func TestNewBuilder(t *testing.T) {
	t.Parallel()

	builder := pack.NewBuilder("test-pack")
	if builder == nil {
		t.Fatal("NewBuilder() returned nil")
	}

	p := builder.Build()
	if p.Name != "test-pack" {
		t.Errorf("Build() Name = %s, want test-pack", p.Name)
	}
	if p.Tools == nil {
		t.Error("Build() Tools should be initialized")
	}
	if p.Eligibility == nil {
		t.Error("Build() Eligibility should be initialized")
	}
	if p.Metadata == nil {
		t.Error("Build() Metadata should be initialized")
	}
}

func TestBuilder_WithDescription(t *testing.T) {
	t.Parallel()

	p := pack.NewBuilder("test").
		WithDescription("A test pack").
		Build()

	if p.Description != "A test pack" {
		t.Errorf("Description = %s, want 'A test pack'", p.Description)
	}
}

func TestBuilder_WithVersion(t *testing.T) {
	t.Parallel()

	p := pack.NewBuilder("test").
		WithVersion("1.0.0").
		Build()

	if p.Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", p.Version)
	}
}

func TestBuilder_AddTool(t *testing.T) {
	t.Parallel()

	p := pack.NewBuilder("test").
		AddTool(mockTool{name: "tool1"}).
		AddTool(mockTool{name: "tool2"}).
		Build()

	if len(p.Tools) != 2 {
		t.Fatalf("Tools len = %d, want 2", len(p.Tools))
	}
	if p.Tools[0].Name() != "tool1" {
		t.Errorf("Tools[0].Name() = %s, want tool1", p.Tools[0].Name())
	}
}

func TestBuilder_AddTools(t *testing.T) {
	t.Parallel()

	p := pack.NewBuilder("test").
		AddTools(
			mockTool{name: "tool1"},
			mockTool{name: "tool2"},
			mockTool{name: "tool3"},
		).
		Build()

	if len(p.Tools) != 3 {
		t.Fatalf("Tools len = %d, want 3", len(p.Tools))
	}
}

func TestBuilder_AllowInState(t *testing.T) {
	t.Parallel()

	t.Run("allows specific tools in state", func(t *testing.T) {
		t.Parallel()

		p := pack.NewBuilder("test").
			AllowInState(agent.StateExplore, "read_file", "list_files").
			Build()

		allowed := p.Eligibility[agent.StateExplore]
		if len(allowed) != 2 {
			t.Fatalf("Eligibility[explore] len = %d, want 2", len(allowed))
		}
	})

	t.Run("accumulates tools for same state", func(t *testing.T) {
		t.Parallel()

		p := pack.NewBuilder("test").
			AllowInState(agent.StateExplore, "tool1").
			AllowInState(agent.StateExplore, "tool2").
			Build()

		allowed := p.Eligibility[agent.StateExplore]
		if len(allowed) != 2 {
			t.Fatalf("Eligibility[explore] len = %d, want 2", len(allowed))
		}
	})
}

func TestBuilder_AllowAllInState(t *testing.T) {
	t.Parallel()

	p := pack.NewBuilder("test").
		AddTool(mockTool{name: "tool1"}).
		AddTool(mockTool{name: "tool2"}).
		AddTool(mockTool{name: "tool3"}).
		AllowAllInState(agent.StateExplore).
		Build()

	allowed := p.Eligibility[agent.StateExplore]
	if len(allowed) != 3 {
		t.Fatalf("AllowAllInState() len = %d, want 3", len(allowed))
	}

	// Verify all tools are allowed
	expectedTools := map[string]bool{"tool1": true, "tool2": true, "tool3": true}
	for _, name := range allowed {
		if !expectedTools[name] {
			t.Errorf("Unexpected tool allowed: %s", name)
		}
	}
}

func TestBuilder_WithDependency(t *testing.T) {
	t.Parallel()

	p := pack.NewBuilder("test").
		WithDependency("core-pack").
		WithDependency("utils-pack").
		Build()

	if len(p.Dependencies) != 2 {
		t.Fatalf("Dependencies len = %d, want 2", len(p.Dependencies))
	}
	if p.Dependencies[0] != "core-pack" {
		t.Errorf("Dependencies[0] = %s, want core-pack", p.Dependencies[0])
	}
	if p.Dependencies[1] != "utils-pack" {
		t.Errorf("Dependencies[1] = %s, want utils-pack", p.Dependencies[1])
	}
}

func TestBuilder_WithMetadata(t *testing.T) {
	t.Parallel()

	p := pack.NewBuilder("test").
		WithMetadata("author", "alice").
		WithMetadata("license", "MIT").
		Build()

	if p.Metadata["author"] != "alice" {
		t.Errorf("Metadata[author] = %s, want alice", p.Metadata["author"])
	}
	if p.Metadata["license"] != "MIT" {
		t.Errorf("Metadata[license] = %s, want MIT", p.Metadata["license"])
	}
}

func TestBuilder_Build(t *testing.T) {
	t.Parallel()

	p := pack.NewBuilder("file-ops").
		WithDescription("File operations pack").
		WithVersion("2.0.0").
		AddTool(mockTool{name: "read_file"}).
		AddTool(mockTool{name: "write_file"}).
		AllowInState(agent.StateExplore, "read_file").
		AllowInState(agent.StateAct, "read_file", "write_file").
		WithDependency("core").
		WithMetadata("category", "filesystem").
		Build()

	if p.Name != "file-ops" {
		t.Errorf("Name = %s, want file-ops", p.Name)
	}
	if p.Description != "File operations pack" {
		t.Errorf("Description = %s", p.Description)
	}
	if p.Version != "2.0.0" {
		t.Errorf("Version = %s", p.Version)
	}
	if len(p.Tools) != 2 {
		t.Errorf("Tools len = %d", len(p.Tools))
	}
	if len(p.Eligibility[agent.StateExplore]) != 1 {
		t.Errorf("Eligibility[explore] len = %d", len(p.Eligibility[agent.StateExplore]))
	}
	if len(p.Eligibility[agent.StateAct]) != 2 {
		t.Errorf("Eligibility[act] len = %d", len(p.Eligibility[agent.StateAct]))
	}
	if len(p.Dependencies) != 1 {
		t.Errorf("Dependencies len = %d", len(p.Dependencies))
	}
	if p.Metadata["category"] != "filesystem" {
		t.Errorf("Metadata[category] = %s", p.Metadata["category"])
	}
}

func TestDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrPackNotFound",
			err:  pack.ErrPackNotFound,
			msg:  "pack not found",
		},
		{
			name: "ErrPackExists",
			err:  pack.ErrPackExists,
			msg:  "pack already exists",
		},
		{
			name: "ErrInvalidPack",
			err:  pack.ErrInvalidPack,
			msg:  "invalid pack",
		},
		{
			name: "ErrDependencyNotFound",
			err:  pack.ErrDependencyNotFound,
			msg:  "pack dependency not found",
		},
		{
			name: "ErrCircularDependency",
			err:  pack.ErrCircularDependency,
			msg:  "circular pack dependency detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err.Error() != tt.msg {
				t.Errorf("%s.Error() = %s, want %s", tt.name, tt.err.Error(), tt.msg)
			}
		})
	}
}
