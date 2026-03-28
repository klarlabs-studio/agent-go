package browser

import (
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestPack_RegistersTools(t *testing.T) {
	p := Pack(DefaultConfig())

	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() registered no tools")
	}
	if p.Name != "browser" {
		t.Errorf("expected pack name %q, got %q", "browser", p.Name)
	}
}

func TestPack_ToolsImplementInterface(t *testing.T) {
	p := Pack(DefaultConfig())

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
