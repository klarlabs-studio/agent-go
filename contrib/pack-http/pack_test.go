package http_test

import (
	"testing"

	packhttp "github.com/felixgeelhaar/agent-go/contrib/pack-http"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestRegister(t *testing.T) {
	p := packhttp.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "http" {
		t.Errorf("expected pack name %q, got %q", "http", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := packhttp.Pack()
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
