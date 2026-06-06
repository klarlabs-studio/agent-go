package messaging_test

import (
	"testing"

	messaging "go.klarlabs.de/agent/contrib/pack-messaging"
	"go.klarlabs.de/agent/domain/tool"
)

func TestRegister(t *testing.T) {
	p := messaging.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "messaging" {
		t.Errorf("expected pack name %q, got %q", "messaging", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := messaging.Pack()
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
