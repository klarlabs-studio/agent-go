package linear_test

import (
	"testing"

	linear "go.klarlabs.de/agent/contrib/pack-linear"
	"go.klarlabs.de/agent/domain/tool"
)

func TestRegister(t *testing.T) {
	p := linear.Pack(linear.Config{})
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "linear" {
		t.Errorf("expected pack name %q, got %q", "linear", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := linear.Pack(linear.Config{})
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
