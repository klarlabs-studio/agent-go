package gpio_test

import (
	"testing"

	gpio "github.com/felixgeelhaar/agent-go/contrib/pack-gpio"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestRegister(t *testing.T) {
	p := gpio.Pack(gpio.Config{})
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "gpio" {
		t.Errorf("expected pack name %q, got %q", "gpio", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := gpio.Pack(gpio.Config{})
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
