package jira_test

import (
	"testing"

	jira "go.klarlabs.de/agent/contrib/pack-jira"
	"go.klarlabs.de/agent/domain/tool"
)

func TestRegister(t *testing.T) {
	p := jira.Pack(jira.Config{
		BaseURL: "https://example.atlassian.net",
	})
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "jira" {
		t.Errorf("expected pack name %q, got %q", "jira", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := jira.Pack(jira.Config{
		BaseURL: "https://example.atlassian.net",
	})
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
