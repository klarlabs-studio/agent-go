package spreadsheet

import (
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestPack(t *testing.T) {
	p := Pack(DefaultConfig())
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Name != "spreadsheet" {
		t.Errorf("expected pack name 'spreadsheet', got %q", p.Name)
	}
	if len(p.Tools) == 0 {
		t.Fatal("expected at least 1 tool, got 0")
	}
}

func TestToolsImplementInterface(t *testing.T) {
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
