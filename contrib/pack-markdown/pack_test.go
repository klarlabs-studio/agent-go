package markdown_test

import (
	"testing"

	markdown "go.klarlabs.de/agent/contrib/pack-markdown"
	"go.klarlabs.de/agent/domain/tool"
)

func TestRegister(t *testing.T) {
	p := markdown.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "markdown" {
		t.Errorf("expected pack name %q, got %q", "markdown", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := markdown.Pack()
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
