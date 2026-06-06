package image_test

import (
	"testing"

	packimage "go.klarlabs.de/agent/contrib/pack-image"
	"go.klarlabs.de/agent/domain/tool"
)

func TestRegister(t *testing.T) {
	p := packimage.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "image" {
		t.Errorf("expected pack name %q, got %q", "image", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := packimage.Pack()
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
