package config

import (
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	domainconfig "go.klarlabs.de/agent/domain/config"
)

func TestBuilder_BasicBuild(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Agent: domainconfig.AgentSettings{
			MaxSteps:     50,
			InitialState: "intake",
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.MaxSteps != 50 {
		t.Errorf("MaxSteps = %d, want 50", result.MaxSteps)
	}
}

func TestBuilder_DefaultMaxSteps(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Agent: domainconfig.AgentSettings{
			MaxSteps: 0, // Should default to 100
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.MaxSteps != 100 {
		t.Errorf("MaxSteps = %d, want 100 (default)", result.MaxSteps)
	}
}

func TestBuilder_Eligibility(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Tools: domainconfig.ToolsConfig{
			Eligibility: map[string][]string{
				"explore": {"read_file", "list_dir"},
				"act":     {"write_file"},
			},
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.Eligibility == nil {
		t.Fatal("Eligibility is nil")
	}

	if !result.Eligibility.IsAllowed(agent.StateExplore, "read_file") {
		t.Error("read_file should be allowed in explore")
	}
	if !result.Eligibility.IsAllowed(agent.StateExplore, "list_dir") {
		t.Error("list_dir should be allowed in explore")
	}
	if !result.Eligibility.IsAllowed(agent.StateAct, "write_file") {
		t.Error("write_file should be allowed in act")
	}
	if result.Eligibility.IsAllowed(agent.StateExplore, "write_file") {
		t.Error("write_file should NOT be allowed in explore")
	}
}

func TestBuilder_Budgets(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Policy: domainconfig.PolicyConfig{
			Budgets: map[string]int{
				"tool_calls": 100,
				"tokens":     10000,
			},
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.Budgets["tool_calls"] != 100 {
		t.Errorf("Budgets[tool_calls] = %d, want 100", result.Budgets["tool_calls"])
	}
	if result.Budgets["tokens"] != 10000 {
		t.Errorf("Budgets[tokens] = %d, want 10000", result.Budgets["tokens"])
	}
}

func TestBuilder_Variables(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Variables: map[string]any{
			"env":   "test",
			"debug": true,
			"count": 42,
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.Variables["env"] != "test" {
		t.Errorf("Variables[env] = %v, want test", result.Variables["env"])
	}
	if result.Variables["debug"] != true {
		t.Errorf("Variables[debug] = %v, want true", result.Variables["debug"])
	}
	if result.Variables["count"] != 42 {
		t.Errorf("Variables[count] = %v, want 42", result.Variables["count"])
	}
}

func TestBuilder_RateLimit(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Policy: domainconfig.PolicyConfig{
			RateLimit: domainconfig.RateLimitConfig{
				Enabled: true,
				Rate:    10,
				Burst:   20,
				PerTool: true,
				ToolRates: map[string]domainconfig.ToolRateLimitConfig{
					"expensive_tool": {Rate: 1, Burst: 2},
				},
			},
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.RateLimitConfig == nil {
		t.Fatal("RateLimitConfig is nil")
	}
	if !result.RateLimitConfig.Enabled {
		t.Error("RateLimitConfig.Enabled should be true")
	}
	if result.RateLimitConfig.Rate != 10 {
		t.Errorf("Rate = %d, want 10", result.RateLimitConfig.Rate)
	}
	if result.RateLimitConfig.Burst != 20 {
		t.Errorf("Burst = %d, want 20", result.RateLimitConfig.Burst)
	}
	if !result.RateLimitConfig.PerTool {
		t.Error("PerTool should be true")
	}
	if tr, ok := result.RateLimitConfig.ToolRates["expensive_tool"]; !ok {
		t.Error("ToolRates should have expensive_tool")
	} else if tr.Rate != 1 || tr.Burst != 2 {
		t.Errorf("ToolRates[expensive_tool] = %v, want {1, 2}", tr)
	}
}

func TestBuilder_ToolPacks(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Tools: domainconfig.ToolsConfig{
			Packs: []domainconfig.ToolPackConfig{
				{
					Name:    "fileops",
					Version: "1.0",
					Config: map[string]any{
						"root_dir": "/tmp",
					},
					Enabled:  []string{"read_file"},
					Disabled: []string{"delete_file"},
				},
			},
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.ToolPacks) != 1 {
		t.Fatalf("ToolPacks has %d packs, want 1", len(result.ToolPacks))
	}

	pack := result.ToolPacks[0]
	if pack.Name != "fileops" {
		t.Errorf("Pack.Name = %s, want fileops", pack.Name)
	}
	if pack.Version != "1.0" {
		t.Errorf("Pack.Version = %s, want 1.0", pack.Version)
	}
	if pack.Config["root_dir"] != "/tmp" {
		t.Errorf("Pack.Config[root_dir] = %v, want /tmp", pack.Config["root_dir"])
	}
	if len(pack.Enabled) != 1 || pack.Enabled[0] != "read_file" {
		t.Errorf("Pack.Enabled = %v, want [read_file]", pack.Enabled)
	}
	if len(pack.Disabled) != 1 || pack.Disabled[0] != "delete_file" {
		t.Errorf("Pack.Disabled = %v, want [delete_file]", pack.Disabled)
	}
}

func TestBuilder_InlineTools(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Tools: domainconfig.ToolsConfig{
			Inline: []domainconfig.InlineToolConfig{
				{
					Name:        "echo",
					Description: "Echo input",
					Annotations: domainconfig.ToolAnnotationsConfig{
						ReadOnly:  true,
						RiskLevel: "low",
					},
					Handler: domainconfig.ToolHandlerConfig{
						Type:    "exec",
						Command: "echo",
						Args:    []string{"-n"},
					},
				},
			},
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(result.InlineTools) != 1 {
		t.Fatalf("InlineTools has %d tools, want 1", len(result.InlineTools))
	}

	tool := result.InlineTools[0]
	if tool.Name != "echo" {
		t.Errorf("Tool.Name = %s, want echo", tool.Name)
	}
	if tool.Description != "Echo input" {
		t.Errorf("Tool.Description = %s, want Echo input", tool.Description)
	}
	if !tool.Annotations.ReadOnly {
		t.Error("Tool.Annotations.ReadOnly should be true")
	}
	if tool.Handler.Type != "exec" {
		t.Errorf("Tool.Handler.Type = %s, want exec", tool.Handler.Type)
	}
}

func TestBuilder_Notification(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Notification: domainconfig.NotificationConfig{
			Enabled: true,
			Endpoints: []domainconfig.EndpointConfig{
				{
					Name:    "slack",
					URL:     "https://hooks.slack.com/test",
					Enabled: true,
					Secret:  "secret-key",
				},
			},
			Batching: domainconfig.BatchingConfig{
				Enabled: true,
				MaxSize: 50,
				MaxWait: domainconfig.Duration(10 * time.Second),
			},
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.Notifier == nil {
		t.Error("Notifier should not be nil when notification is enabled")
	}
}

func TestBuilder_NotificationDisabled(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Notification: domainconfig.NotificationConfig{
			Enabled: false,
		},
	}

	builder := NewBuilder(cfg)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if result.Notifier != nil {
		t.Error("Notifier should be nil when notification is disabled")
	}
}

func TestBuilder_InvalidState(t *testing.T) {
	cfg := &domainconfig.AgentConfig{
		Name:    "test-agent",
		Version: "1.0",
		Tools: domainconfig.ToolsConfig{
			Eligibility: map[string][]string{
				"invalid_state": {"read_file"},
			},
		},
	}

	builder := NewBuilder(cfg)
	_, err := builder.Build()
	if err == nil {
		t.Error("Build() should return error for invalid state")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Name != "agent" {
		t.Errorf("Name = %s, want agent", cfg.Name)
	}
	if cfg.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", cfg.Version)
	}
	if cfg.Agent.MaxSteps != 100 {
		t.Errorf("MaxSteps = %d, want 100", cfg.Agent.MaxSteps)
	}
	if cfg.Policy.Budgets["tool_calls"] != 100 {
		t.Errorf("Budgets[tool_calls] = %d, want 100", cfg.Policy.Budgets["tool_calls"])
	}
}
