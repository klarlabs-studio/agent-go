package config

import (
	"strings"
	"testing"
)

func TestValidator_ValidateMinimal(t *testing.T) {
	tests := []struct {
		name   string
		config *AgentConfig
	}{
		{
			name: "minimal valid config",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
			},
		},
		{
			name: "valid config with description",
			config: &AgentConfig{
				Name:        "test-agent",
				Version:     "1.0.0",
				Description: "A test agent",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			errs := v.Validate(tt.config)
			if errs.HasErrors() {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

func TestValidator_ValidateRequired(t *testing.T) {
	tests := []struct {
		name         string
		config       *AgentConfig
		wantErrPaths []string
	}{
		{
			name: "missing name",
			config: &AgentConfig{
				Version: "1.0.0",
			},
			wantErrPaths: []string{"name"},
		},
		{
			name: "missing version",
			config: &AgentConfig{
				Name: "test-agent",
			},
			wantErrPaths: []string{"version"},
		},
		{
			name:         "missing both name and version",
			config:       &AgentConfig{},
			wantErrPaths: []string{"name", "version"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			errs := v.Validate(tt.config)
			if !errs.HasErrors() {
				t.Fatal("expected errors, got none")
			}
			assertErrorPaths(t, errs, tt.wantErrPaths)
		})
	}
}

func TestValidator_ValidateAgent(t *testing.T) {
	tests := []struct {
		name         string
		config       *AgentConfig
		wantErrPaths []string
	}{
		{
			name: "valid max_steps positive",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Agent: AgentSettings{
					MaxSteps: 100,
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid max_steps zero (unlimited)",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Agent: AgentSettings{
					MaxSteps: 0,
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "invalid max_steps negative",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Agent: AgentSettings{
					MaxSteps: -1,
				},
			},
			wantErrPaths: []string{"agent.max_steps"},
		},
		{
			name: "valid initial_state intake",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Agent: AgentSettings{
					InitialState: "intake",
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid initial_state explore",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Agent: AgentSettings{
					InitialState: "explore",
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "invalid initial_state",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Agent: AgentSettings{
					InitialState: "invalid_state",
				},
			},
			wantErrPaths: []string{"agent.initial_state"},
		},
		{
			name: "multiple agent errors",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Agent: AgentSettings{
					MaxSteps:     -5,
					InitialState: "bad_state",
				},
			},
			wantErrPaths: []string{"agent.max_steps", "agent.initial_state"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			errs := v.Validate(tt.config)
			if len(tt.wantErrPaths) == 0 {
				if errs.HasErrors() {
					t.Errorf("expected no errors, got: %v", errs)
				}
			} else {
				if !errs.HasErrors() {
					t.Fatal("expected errors, got none")
				}
				assertErrorPaths(t, errs, tt.wantErrPaths)
			}
		})
	}
}

func TestValidator_ValidateTools(t *testing.T) {
	tests := []struct {
		name         string
		config       *AgentConfig
		wantErrPaths []string
	}{
		{
			name: "valid tool pack",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Packs: []ToolPackConfig{
						{Name: "fileops"},
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "missing pack name",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Packs: []ToolPackConfig{
						{Version: "1.0.0"},
					},
				},
			},
			wantErrPaths: []string{"tools.packs[0].name"},
		},
		{
			name: "multiple packs with missing names",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Packs: []ToolPackConfig{
						{Name: "valid"},
						{Version: "1.0.0"},
						{Name: "valid2"},
						{},
					},
				},
			},
			wantErrPaths: []string{"tools.packs[1].name", "tools.packs[3].name"},
		},
		{
			name: "valid inline tool with http handler",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "webhook",
							Description: "Call webhook",
							Handler: ToolHandlerConfig{
								Type: "http",
								URL:  "https://example.com/webhook",
							},
						},
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "inline tool missing name",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Description: "Missing name",
							Handler: ToolHandlerConfig{
								Type: "http",
								URL:  "https://example.com",
							},
						},
					},
				},
			},
			wantErrPaths: []string{"tools.inline[0].name"},
		},
		{
			name: "inline tool missing description",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name: "tool1",
							Handler: ToolHandlerConfig{
								Type: "http",
								URL:  "https://example.com",
							},
						},
					},
				},
			},
			wantErrPaths: []string{"tools.inline[0].description"},
		},
		{
			name: "inline tool missing handler type",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "tool1",
							Description: "Test tool",
							Handler:     ToolHandlerConfig{},
						},
					},
				},
			},
			wantErrPaths: []string{"tools.inline[0].handler.type"},
		},
		{
			name: "http handler missing URL",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "webhook",
							Description: "Call webhook",
							Handler: ToolHandlerConfig{
								Type: "http",
							},
						},
					},
				},
			},
			wantErrPaths: []string{"tools.inline[0].handler.url"},
		},
		{
			name: "exec handler missing command",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "script",
							Description: "Run script",
							Handler: ToolHandlerConfig{
								Type: "exec",
							},
						},
					},
				},
			},
			wantErrPaths: []string{"tools.inline[0].handler.command"},
		},
		{
			name: "wasm handler missing path",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "wasm_tool",
							Description: "WASM tool",
							Handler: ToolHandlerConfig{
								Type: "wasm",
							},
						},
					},
				},
			},
			wantErrPaths: []string{"tools.inline[0].handler.path"},
		},
		{
			name: "unknown handler type",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "tool1",
							Description: "Test tool",
							Handler: ToolHandlerConfig{
								Type: "unknown",
							},
						},
					},
				},
			},
			wantErrPaths: []string{"tools.inline[0].handler.type"},
		},
		{
			name: "valid exec handler with all fields",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "script",
							Description: "Run script",
							Handler: ToolHandlerConfig{
								Type:    "exec",
								Command: "/bin/bash",
								Args:    []string{"-c", "echo hello"},
								Env:     map[string]string{"FOO": "bar"},
							},
						},
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "invalid risk level",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "tool1",
							Description: "Test tool",
							Handler: ToolHandlerConfig{
								Type: "http",
								URL:  "https://example.com",
							},
							Annotations: ToolAnnotationsConfig{
								RiskLevel: "extreme",
							},
						},
					},
				},
			},
			wantErrPaths: []string{"tools.inline[0].annotations.risk_level"},
		},
		{
			name: "valid risk levels",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Inline: []InlineToolConfig{
						{
							Name:        "tool1",
							Description: "Low risk",
							Handler:     ToolHandlerConfig{Type: "http", URL: "https://example.com"},
							Annotations: ToolAnnotationsConfig{RiskLevel: "low"},
						},
						{
							Name:        "tool2",
							Description: "Medium risk",
							Handler:     ToolHandlerConfig{Type: "http", URL: "https://example.com"},
							Annotations: ToolAnnotationsConfig{RiskLevel: "medium"},
						},
						{
							Name:        "tool3",
							Description: "High risk",
							Handler:     ToolHandlerConfig{Type: "http", URL: "https://example.com"},
							Annotations: ToolAnnotationsConfig{RiskLevel: "high"},
						},
						{
							Name:        "tool4",
							Description: "Critical risk",
							Handler:     ToolHandlerConfig{Type: "http", URL: "https://example.com"},
							Annotations: ToolAnnotationsConfig{RiskLevel: "critical"},
						},
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "invalid eligibility state",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Eligibility: map[string][]string{
						"invalid_state": {"tool1"},
					},
				},
			},
			wantErrPaths: []string{"tools.eligibility.invalid_state"},
		},
		{
			name: "valid eligibility states",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Eligibility: map[string][]string{
						"intake":   {"tool1"},
						"explore":  {"tool2"},
						"decide":   {"tool3"},
						"act":      {"tool4"},
						"validate": {"tool5"},
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "multiple tool errors",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Tools: ToolsConfig{
					Packs: []ToolPackConfig{
						{}, // missing name
					},
					Inline: []InlineToolConfig{
						{
							// missing name and description
							Handler: ToolHandlerConfig{
								Type: "http",
								// missing URL
							},
						},
					},
					Eligibility: map[string][]string{
						"bad_state": {"tool1"},
					},
				},
			},
			wantErrPaths: []string{
				"tools.packs[0].name",
				"tools.inline[0].name",
				"tools.inline[0].description",
				"tools.inline[0].handler.url",
				"tools.eligibility.bad_state",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			errs := v.Validate(tt.config)
			if len(tt.wantErrPaths) == 0 {
				if errs.HasErrors() {
					t.Errorf("expected no errors, got: %v", errs)
				}
			} else {
				if !errs.HasErrors() {
					t.Fatal("expected errors, got none")
				}
				assertErrorPaths(t, errs, tt.wantErrPaths)
			}
		})
	}
}

func TestValidator_ValidatePolicy(t *testing.T) {
	tests := []struct {
		name         string
		config       *AgentConfig
		wantErrPaths []string
	}{
		{
			name: "valid budgets",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Budgets: map[string]int{
						"tool_calls": 100,
						"tokens":     5000,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "negative budget",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Budgets: map[string]int{
						"tool_calls": -1,
					},
				},
			},
			wantErrPaths: []string{"policy.budgets.tool_calls"},
		},
		{
			name: "multiple negative budgets",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Budgets: map[string]int{
						"tool_calls": -1,
						"tokens":     -100,
					},
				},
			},
			wantErrPaths: []string{"policy.budgets.tool_calls", "policy.budgets.tokens"},
		},
		{
			name: "valid approval mode auto",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Approval: ApprovalConfig{
						Mode: "auto",
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid approval mode manual",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Approval: ApprovalConfig{
						Mode: "manual",
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid approval mode none",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Approval: ApprovalConfig{
						Mode: "none",
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "invalid approval mode",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Approval: ApprovalConfig{
						Mode: "invalid",
					},
				},
			},
			wantErrPaths: []string{"policy.approval.mode"},
		},
		{
			name: "valid approval risk level",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Approval: ApprovalConfig{
						RequireForRiskLevel: "high",
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "invalid approval risk level",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Approval: ApprovalConfig{
						RequireForRiskLevel: "extreme",
					},
				},
			},
			wantErrPaths: []string{"policy.approval.require_for_risk_level"},
		},
		{
			name: "valid transition",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Transitions: []TransitionConfig{
						{From: "intake", To: "explore"},
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "transition missing from",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Transitions: []TransitionConfig{
						{To: "explore"},
					},
				},
			},
			wantErrPaths: []string{"policy.transitions[0].from"},
		},
		{
			name: "transition missing to",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Transitions: []TransitionConfig{
						{From: "intake"},
					},
				},
			},
			wantErrPaths: []string{"policy.transitions[0].to"},
		},
		{
			name: "transition invalid from state",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Transitions: []TransitionConfig{
						{From: "invalid", To: "explore"},
					},
				},
			},
			wantErrPaths: []string{"policy.transitions[0].from"},
		},
		{
			name: "transition invalid to state",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Transitions: []TransitionConfig{
						{From: "intake", To: "invalid"},
					},
				},
			},
			wantErrPaths: []string{"policy.transitions[0].to"},
		},
		{
			name: "transition both states invalid",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Transitions: []TransitionConfig{
						{From: "bad", To: "worse"},
					},
				},
			},
			wantErrPaths: []string{"policy.transitions[0].from", "policy.transitions[0].to"},
		},
		{
			name: "multiple transitions with errors",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Transitions: []TransitionConfig{
						{From: "intake", To: "explore"},
						{From: "bad", To: "explore"},
						{From: "explore", To: "worse"},
					},
				},
			},
			wantErrPaths: []string{"policy.transitions[1].from", "policy.transitions[2].to"},
		},
		{
			name: "valid rate limit disabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					RateLimit: RateLimitConfig{
						Enabled: false,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid rate limit enabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					RateLimit: RateLimitConfig{
						Enabled: true,
						Rate:    10,
						Burst:   20,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "rate limit enabled with zero rate",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					RateLimit: RateLimitConfig{
						Enabled: true,
						Rate:    0,
						Burst:   10,
					},
				},
			},
			wantErrPaths: []string{"policy.rate_limit.rate"},
		},
		{
			name: "rate limit enabled with negative rate",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					RateLimit: RateLimitConfig{
						Enabled: true,
						Rate:    -5,
						Burst:   10,
					},
				},
			},
			wantErrPaths: []string{"policy.rate_limit.rate"},
		},
		{
			name: "rate limit enabled with zero burst",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					RateLimit: RateLimitConfig{
						Enabled: true,
						Rate:    10,
						Burst:   0,
					},
				},
			},
			wantErrPaths: []string{"policy.rate_limit.burst"},
		},
		{
			name: "rate limit enabled with invalid rate and burst",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					RateLimit: RateLimitConfig{
						Enabled: true,
						Rate:    -1,
						Burst:   -5,
					},
				},
			},
			wantErrPaths: []string{"policy.rate_limit.rate", "policy.rate_limit.burst"},
		},
		{
			name: "multiple policy errors",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Policy: PolicyConfig{
					Budgets: map[string]int{
						"tool_calls": -1,
					},
					Approval: ApprovalConfig{
						Mode:                "bad_mode",
						RequireForRiskLevel: "extreme",
					},
					Transitions: []TransitionConfig{
						{From: "bad", To: "worse"},
					},
					RateLimit: RateLimitConfig{
						Enabled: true,
						Rate:    -1,
						Burst:   0,
					},
				},
			},
			wantErrPaths: []string{
				"policy.budgets.tool_calls",
				"policy.approval.mode",
				"policy.approval.require_for_risk_level",
				"policy.transitions[0].from",
				"policy.transitions[0].to",
				"policy.rate_limit.rate",
				"policy.rate_limit.burst",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			errs := v.Validate(tt.config)
			if len(tt.wantErrPaths) == 0 {
				if errs.HasErrors() {
					t.Errorf("expected no errors, got: %v", errs)
				}
			} else {
				if !errs.HasErrors() {
					t.Fatal("expected errors, got none")
				}
				assertErrorPaths(t, errs, tt.wantErrPaths)
			}
		})
	}
}

func TestValidator_ValidateResilience(t *testing.T) {
	tests := []struct {
		name         string
		config       *AgentConfig
		wantErrPaths []string
	}{
		{
			name: "valid retry disabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Retry: RetryConfig{
						Enabled: false,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid retry enabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Retry: RetryConfig{
						Enabled:     true,
						MaxAttempts: 3,
						Multiplier:  2.0,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "retry enabled with zero max_attempts",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Retry: RetryConfig{
						Enabled:     true,
						MaxAttempts: 0,
						Multiplier:  2.0, // Valid multiplier to isolate max_attempts error
					},
				},
			},
			wantErrPaths: []string{"resilience.retry.max_attempts"},
		},
		{
			name: "retry enabled with negative max_attempts",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Retry: RetryConfig{
						Enabled:     true,
						MaxAttempts: -1,
						Multiplier:  2.0, // Valid multiplier to isolate max_attempts error
					},
				},
			},
			wantErrPaths: []string{"resilience.retry.max_attempts"},
		},
		{
			name: "retry enabled with multiplier less than 1",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Retry: RetryConfig{
						Enabled:     true,
						MaxAttempts: 3, // Valid max_attempts to isolate multiplier error
						Multiplier:  0.5,
					},
				},
			},
			wantErrPaths: []string{"resilience.retry.multiplier"},
		},
		{
			name: "retry with multiplier exactly 1",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Retry: RetryConfig{
						Enabled:     true,
						MaxAttempts: 3,
						Multiplier:  1.0,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid circuit breaker disabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					CircuitBreaker: CircuitBreakerConfig{
						Enabled: false,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid circuit breaker enabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					CircuitBreaker: CircuitBreakerConfig{
						Enabled:   true,
						Threshold: 5,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "circuit breaker enabled with zero threshold",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					CircuitBreaker: CircuitBreakerConfig{
						Enabled:   true,
						Threshold: 0,
					},
				},
			},
			wantErrPaths: []string{"resilience.circuit_breaker.threshold"},
		},
		{
			name: "circuit breaker enabled with negative threshold",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					CircuitBreaker: CircuitBreakerConfig{
						Enabled:   true,
						Threshold: -1,
					},
				},
			},
			wantErrPaths: []string{"resilience.circuit_breaker.threshold"},
		},
		{
			name: "valid bulkhead disabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Bulkhead: BulkheadConfig{
						Enabled: false,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid bulkhead enabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Bulkhead: BulkheadConfig{
						Enabled:       true,
						MaxConcurrent: 10,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "bulkhead enabled with zero max_concurrent",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Bulkhead: BulkheadConfig{
						Enabled:       true,
						MaxConcurrent: 0,
					},
				},
			},
			wantErrPaths: []string{"resilience.bulkhead.max_concurrent"},
		},
		{
			name: "bulkhead enabled with negative max_concurrent",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Bulkhead: BulkheadConfig{
						Enabled:       true,
						MaxConcurrent: -5,
					},
				},
			},
			wantErrPaths: []string{"resilience.bulkhead.max_concurrent"},
		},
		{
			name: "multiple resilience errors",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Resilience: ResilienceConfig{
					Retry: RetryConfig{
						Enabled:     true,
						MaxAttempts: -1,
						Multiplier:  0.5,
					},
					CircuitBreaker: CircuitBreakerConfig{
						Enabled:   true,
						Threshold: -1,
					},
					Bulkhead: BulkheadConfig{
						Enabled:       true,
						MaxConcurrent: 0,
					},
				},
			},
			wantErrPaths: []string{
				"resilience.retry.max_attempts",
				"resilience.retry.multiplier",
				"resilience.circuit_breaker.threshold",
				"resilience.bulkhead.max_concurrent",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			errs := v.Validate(tt.config)
			if len(tt.wantErrPaths) == 0 {
				if errs.HasErrors() {
					t.Errorf("expected no errors, got: %v", errs)
				}
			} else {
				if !errs.HasErrors() {
					t.Fatal("expected errors, got none")
				}
				assertErrorPaths(t, errs, tt.wantErrPaths)
			}
		})
	}
}

func TestValidator_ValidateNotification(t *testing.T) {
	tests := []struct {
		name         string
		config       *AgentConfig
		wantErrPaths []string
	}{
		{
			name: "notification disabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: false,
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid notification with endpoint",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: true,
					Endpoints: []EndpointConfig{
						{URL: "https://example.com/webhook"},
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "notification enabled with missing endpoint URL",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: true,
					Endpoints: []EndpointConfig{
						{Name: "webhook1"},
					},
				},
			},
			wantErrPaths: []string{"notification.endpoints[0].url"},
		},
		{
			name: "multiple endpoints with missing URLs",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: true,
					Endpoints: []EndpointConfig{
						{URL: "https://example.com/webhook1"},
						{Name: "missing-url"},
						{URL: "https://example.com/webhook2"},
						{},
					},
				},
			},
			wantErrPaths: []string{"notification.endpoints[1].url", "notification.endpoints[3].url"},
		},
		{
			name: "valid batching disabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: true,
					Endpoints: []EndpointConfig{
						{URL: "https://example.com/webhook"},
					},
					Batching: BatchingConfig{
						Enabled: false,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "valid batching enabled",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: true,
					Endpoints: []EndpointConfig{
						{URL: "https://example.com/webhook"},
					},
					Batching: BatchingConfig{
						Enabled: true,
						MaxSize: 100,
					},
				},
			},
			wantErrPaths: nil,
		},
		{
			name: "batching enabled with zero max_size",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: true,
					Endpoints: []EndpointConfig{
						{URL: "https://example.com/webhook"},
					},
					Batching: BatchingConfig{
						Enabled: true,
						MaxSize: 0,
					},
				},
			},
			wantErrPaths: []string{"notification.batching.max_size"},
		},
		{
			name: "batching enabled with negative max_size",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: true,
					Endpoints: []EndpointConfig{
						{URL: "https://example.com/webhook"},
					},
					Batching: BatchingConfig{
						Enabled: true,
						MaxSize: -1,
					},
				},
			},
			wantErrPaths: []string{"notification.batching.max_size"},
		},
		{
			name: "multiple notification errors",
			config: &AgentConfig{
				Name:    "test-agent",
				Version: "1.0.0",
				Notification: NotificationConfig{
					Enabled: true,
					Endpoints: []EndpointConfig{
						{},
						{Name: "ep2"},
					},
					Batching: BatchingConfig{
						Enabled: true,
						MaxSize: -10,
					},
				},
			},
			wantErrPaths: []string{
				"notification.endpoints[0].url",
				"notification.endpoints[1].url",
				"notification.batching.max_size",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			errs := v.Validate(tt.config)
			if len(tt.wantErrPaths) == 0 {
				if errs.HasErrors() {
					t.Errorf("expected no errors, got: %v", errs)
				}
			} else {
				if !errs.HasErrors() {
					t.Fatal("expected errors, got none")
				}
				assertErrorPaths(t, errs, tt.wantErrPaths)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "with path",
			err:  ValidationError{Path: "agent.max_steps", Message: "must be positive"},
			want: "agent.max_steps: must be positive",
		},
		{
			name: "without path",
			err:  ValidationError{Path: "", Message: "general error"},
			want: "general error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("ValidationError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationErrors_Error(t *testing.T) {
	tests := []struct {
		name string
		errs ValidationErrors
		want string
	}{
		{
			name: "no errors",
			errs: ValidationErrors{},
			want: "no validation errors",
		},
		{
			name: "single error",
			errs: ValidationErrors{
				{Path: "name", Message: "name is required"},
			},
			want: "name: name is required",
		},
		{
			name: "multiple errors",
			errs: ValidationErrors{
				{Path: "name", Message: "name is required"},
				{Path: "version", Message: "version is required"},
			},
			want: "2 validation errors:\n  - name: name is required\n  - version: version is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errs.Error()
			if got != tt.want {
				t.Errorf("ValidationErrors.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationErrors_HasErrors(t *testing.T) {
	tests := []struct {
		name string
		errs ValidationErrors
		want bool
	}{
		{
			name: "no errors",
			errs: ValidationErrors{},
			want: false,
		},
		{
			name: "has errors",
			errs: ValidationErrors{
				{Path: "name", Message: "name is required"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errs.HasErrors()
			if got != tt.want {
				t.Errorf("ValidationErrors.HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidator_CompleteConfig(t *testing.T) {
	// Test a comprehensive valid config with all fields
	config := &AgentConfig{
		Name:        "complete-agent",
		Version:     "1.0.0",
		Description: "A complete test configuration",
		Agent: AgentSettings{
			MaxSteps:     100,
			DefaultGoal:  "test goal",
			InitialState: "intake",
		},
		Tools: ToolsConfig{
			Packs: []ToolPackConfig{
				{Name: "fileops", Version: "1.0.0"},
			},
			Inline: []InlineToolConfig{
				{
					Name:        "webhook",
					Description: "HTTP webhook",
					Handler: ToolHandlerConfig{
						Type:   "http",
						URL:    "https://example.com/webhook",
						Method: "POST",
					},
					Annotations: ToolAnnotationsConfig{
						ReadOnly:  true,
						RiskLevel: "low",
					},
				},
			},
			Eligibility: map[string][]string{
				"explore": {"webhook", "fileops.read"},
			},
		},
		Policy: PolicyConfig{
			Budgets: map[string]int{
				"tool_calls": 100,
				"tokens":     5000,
			},
			Approval: ApprovalConfig{
				Mode:                  "manual",
				RequireForDestructive: true,
				RequireForRiskLevel:   "high",
			},
			Transitions: []TransitionConfig{
				{From: "intake", To: "explore"},
				{From: "explore", To: "decide"},
			},
			RateLimit: RateLimitConfig{
				Enabled: true,
				Rate:    10,
				Burst:   20,
			},
		},
		Resilience: ResilienceConfig{
			Retry: RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
				Multiplier:  2.0,
			},
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:   true,
				Threshold: 5,
			},
			Bulkhead: BulkheadConfig{
				Enabled:       true,
				MaxConcurrent: 10,
			},
		},
		Notification: NotificationConfig{
			Enabled: true,
			Endpoints: []EndpointConfig{
				{
					Name:    "webhook1",
					URL:     "https://example.com/webhook",
					Enabled: true,
				},
			},
			Batching: BatchingConfig{
				Enabled: true,
				MaxSize: 100,
			},
		},
		Variables: map[string]any{
			"env": "test",
		},
	}

	v := NewValidator()
	errs := v.Validate(config)
	if errs.HasErrors() {
		t.Errorf("expected no errors for complete valid config, got: %v", errs)
	}
}

// assertErrorPaths verifies that the actual errors contain all expected paths.
func assertErrorPaths(t *testing.T, errs ValidationErrors, wantPaths []string) {
	t.Helper()

	if len(errs) != len(wantPaths) {
		t.Errorf("got %d errors, want %d:\n%v", len(errs), len(wantPaths), errs)
		return
	}

	// Check each expected path is present
	for _, wantPath := range wantPaths {
		found := false
		for _, err := range errs {
			if err.Path == wantPath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected error path %q in errors:\n%v", wantPath, errs)
		}
	}

	// Check for unexpected paths
	for _, err := range errs {
		found := false
		for _, wantPath := range wantPaths {
			if err.Path == wantPath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected error path %q: %s", err.Path, err.Message)
		}
	}
}

// TestValidator_AllErrorsReturned verifies that validation returns all errors, not just the first one.
func TestValidator_AllErrorsReturned(t *testing.T) {
	config := &AgentConfig{
		// Missing name and version
		Agent: AgentSettings{
			MaxSteps:     -1,              // Invalid
			InitialState: "invalid_state", // Invalid
		},
		Tools: ToolsConfig{
			Packs: []ToolPackConfig{
				{}, // Missing name
			},
			Inline: []InlineToolConfig{
				{
					// Missing name, description, handler fields
					Handler: ToolHandlerConfig{
						Type: "http", // Missing URL
					},
				},
			},
			Eligibility: map[string][]string{
				"bad_state": {"tool1"}, // Invalid state
			},
		},
		Policy: PolicyConfig{
			Budgets: map[string]int{
				"tokens": -100, // Negative
			},
			Approval: ApprovalConfig{
				Mode: "invalid_mode", // Invalid
			},
			Transitions: []TransitionConfig{
				{From: "bad", To: "worse"}, // Both invalid
			},
			RateLimit: RateLimitConfig{
				Enabled: true,
				Rate:    -1, // Invalid
				Burst:   0,  // Invalid
			},
		},
		Resilience: ResilienceConfig{
			Retry: RetryConfig{
				Enabled:     true,
				MaxAttempts: -1,  // Invalid
				Multiplier:  0.5, // Invalid
			},
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:   true,
				Threshold: -1, // Invalid
			},
			Bulkhead: BulkheadConfig{
				Enabled:       true,
				MaxConcurrent: 0, // Invalid
			},
		},
		Notification: NotificationConfig{
			Enabled: true,
			Endpoints: []EndpointConfig{
				{Name: "ep1"}, // Missing URL
			},
			Batching: BatchingConfig{
				Enabled: true,
				MaxSize: -1, // Invalid
			},
		},
	}

	v := NewValidator()
	errs := v.Validate(config)

	if !errs.HasErrors() {
		t.Fatal("expected errors, got none")
	}

	// Should have many errors
	minExpectedErrors := 15
	if len(errs) < minExpectedErrors {
		t.Errorf("expected at least %d errors, got %d: %v", minExpectedErrors, len(errs), errs)
	}

	// Verify error message format for multiple errors
	errMsg := errs.Error()
	if !strings.Contains(errMsg, "validation errors:") {
		t.Errorf("expected multiple errors message format, got: %q", errMsg)
	}
}
