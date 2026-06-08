package config

import (
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	domainconfig "go.klarlabs.de/agent/domain/config"
	"go.klarlabs.de/agent/domain/notification"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
	infranotif "go.klarlabs.de/agent/infrastructure/notification"
)

// Builder builds engine options from configuration.
type Builder struct {
	config *domainconfig.AgentConfig
}

// NewBuilder creates a new configuration builder.
func NewBuilder(config *domainconfig.AgentConfig) *Builder {
	return &Builder{config: config}
}

// BuildResult contains the built components from configuration.
type BuildResult struct {
	// Eligibility is the tool eligibility policy.
	Eligibility *policy.ToolEligibility
	// Transitions is the state transition policy.
	Transitions *policy.StateTransitions
	// Budgets maps budget names to limits.
	Budgets map[string]int
	// MaxSteps is the maximum execution steps.
	MaxSteps int
	// Variables are the initial variables.
	Variables map[string]any
	// Notifier is the configured webhook notifier (if enabled).
	Notifier notification.Notifier
	// ToolPacks are the requested tool packs to load.
	ToolPacks []ToolPackRequest
	// InlineTools are inline tool definitions to build.
	InlineTools []InlineToolDef
	// RateLimitConfig contains rate limiting configuration.
	RateLimitConfig *RateLimitBuildResult
}

// ToolPackRequest represents a request to load a tool pack.
type ToolPackRequest struct {
	Name     string
	Version  string
	Config   map[string]any
	Enabled  []string
	Disabled []string
}

// InlineToolDef represents an inline tool definition.
type InlineToolDef struct {
	Name         string
	Description  string
	Annotations  tool.Annotations
	Handler      domainconfig.ToolHandlerConfig
	InputSchema  map[string]any
	OutputSchema map[string]any
}

// RateLimitBuildResult contains rate limiting build configuration.
type RateLimitBuildResult struct {
	Enabled   bool
	Rate      int
	Burst     int
	PerTool   bool
	ToolRates map[string]struct{ Rate, Burst int }
}

// Build builds the engine components from configuration.
func (b *Builder) Build() (*BuildResult, error) {
	result := &BuildResult{
		Budgets:   make(map[string]int),
		Variables: make(map[string]any),
	}

	// Build eligibility
	if err := b.buildEligibility(result); err != nil {
		return nil, fmt.Errorf("building eligibility: %w", err)
	}

	// Build transitions
	if err := b.buildTransitions(result); err != nil {
		return nil, fmt.Errorf("building transitions: %w", err)
	}

	// Build policy settings
	b.buildPolicy(result)

	// Build notification
	b.buildNotification(result)

	// Build tool packs
	b.buildToolPacks(result)

	// Build inline tools
	if err := b.buildInlineTools(result); err != nil {
		return nil, fmt.Errorf("building inline tools: %w", err)
	}

	// Set agent settings
	result.MaxSteps = b.config.Agent.MaxSteps
	if result.MaxSteps <= 0 {
		result.MaxSteps = 100 // Default
	}

	// Copy variables
	for k, v := range b.config.Variables {
		result.Variables[k] = v
	}

	return result, nil
}

func (b *Builder) buildEligibility(result *BuildResult) error {
	eligibility := policy.NewToolEligibility()

	// Map state strings to agent.State
	stateMap := map[string]agent.State{
		"intake":   agent.StateIntake,
		"explore":  agent.StateExplore,
		"decide":   agent.StateDecide,
		"act":      agent.StateAct,
		"validate": agent.StateValidate,
	}

	for stateStr, tools := range b.config.Tools.Eligibility {
		state, ok := stateMap[stateStr]
		if !ok {
			return fmt.Errorf("unknown state: %s", stateStr)
		}
		for _, toolName := range tools {
			eligibility.Allow(state, toolName)
		}
	}

	result.Eligibility = eligibility
	return nil
}

func (b *Builder) buildTransitions(result *BuildResult) error {
	transitions := policy.DefaultTransitions()

	// Add custom transitions
	for _, t := range b.config.Policy.Transitions {
		fromState, err := parseState(t.From)
		if err != nil {
			return err
		}
		toState, err := parseState(t.To)
		if err != nil {
			return err
		}
		transitions.Allow(fromState, toState)
	}

	result.Transitions = transitions
	return nil
}

func (b *Builder) buildPolicy(result *BuildResult) {
	// Copy budgets
	for name, limit := range b.config.Policy.Budgets {
		result.Budgets[name] = limit
	}

	// Build rate limit config
	if b.config.Policy.RateLimit.Enabled {
		result.RateLimitConfig = &RateLimitBuildResult{
			Enabled:   true,
			Rate:      b.config.Policy.RateLimit.Rate,
			Burst:     b.config.Policy.RateLimit.Burst,
			PerTool:   b.config.Policy.RateLimit.PerTool,
			ToolRates: make(map[string]struct{ Rate, Burst int }),
		}
		for toolName, cfg := range b.config.Policy.RateLimit.ToolRates {
			result.RateLimitConfig.ToolRates[toolName] = struct{ Rate, Burst int }{
				Rate:  cfg.Rate,
				Burst: cfg.Burst,
			}
		}
	}
}

func (b *Builder) buildNotification(result *BuildResult) {
	if !b.config.Notification.Enabled {
		return
	}

	// Build endpoints
	endpoints := make([]*notification.Endpoint, 0, len(b.config.Notification.Endpoints))
	for _, ep := range b.config.Notification.Endpoints {
		endpoint := &notification.Endpoint{
			Name:    ep.Name,
			URL:     ep.URL,
			Enabled: ep.Enabled,
			Secret:  ep.Secret,
			Headers: ep.Headers,
		}
		// Build event filter if specified
		if len(ep.EventFilter) > 0 {
			endpoint.Filter = buildEventFilter(ep.EventFilter)
		}
		endpoints = append(endpoints, endpoint)
	}

	// Build notifier config
	notifierConfig := infranotif.WebhookNotifierConfig{
		Endpoints:      endpoints,
		EnableBatching: b.config.Notification.Batching.Enabled,
		SenderConfig:   infranotif.DefaultSenderConfig(),
	}

	if b.config.Notification.Batching.Enabled {
		notifierConfig.BatcherConfig = infranotif.BatcherConfig{
			MaxBatchSize: b.config.Notification.Batching.MaxSize,
			MaxWait:      b.config.Notification.Batching.MaxWait.Duration(),
		}
	}

	// Build global filter
	if len(b.config.Notification.EventFilter) > 0 {
		notifierConfig.GlobalFilter = buildEventFilter(b.config.Notification.EventFilter)
	}

	result.Notifier = infranotif.NewWebhookNotifier(notifierConfig)
}

func (b *Builder) buildToolPacks(result *BuildResult) {
	for _, pack := range b.config.Tools.Packs {
		result.ToolPacks = append(result.ToolPacks, ToolPackRequest{
			Name:     pack.Name,
			Version:  pack.Version,
			Config:   pack.Config,
			Enabled:  pack.Enabled,
			Disabled: pack.Disabled,
		})
	}
}

func (b *Builder) buildInlineTools(result *BuildResult) error {
	for _, t := range b.config.Tools.Inline {
		annotations := tool.Annotations{
			ReadOnly:    t.Annotations.ReadOnly,
			Destructive: t.Annotations.Destructive,
			Idempotent:  t.Annotations.Idempotent,
			Cacheable:   t.Annotations.Cacheable,
		}
		// Parse risk level
		if t.Annotations.RiskLevel != "" {
			riskLevel, err := parseRiskLevel(t.Annotations.RiskLevel)
			if err != nil {
				return err
			}
			annotations.RiskLevel = riskLevel
		}

		result.InlineTools = append(result.InlineTools, InlineToolDef{
			Name:         t.Name,
			Description:  t.Description,
			Annotations:  annotations,
			Handler:      t.Handler,
			InputSchema:  t.InputSchema,
			OutputSchema: t.OutputSchema,
		})
	}
	return nil
}

func parseState(s string) (agent.State, error) {
	stateMap := map[string]agent.State{
		"intake":   agent.StateIntake,
		"explore":  agent.StateExplore,
		"decide":   agent.StateDecide,
		"act":      agent.StateAct,
		"validate": agent.StateValidate,
		"done":     agent.StateDone,
		"failed":   agent.StateFailed,
	}
	state, ok := stateMap[s]
	if !ok {
		return "", fmt.Errorf("unknown state: %s", s)
	}
	return state, nil
}

func parseRiskLevel(s string) (tool.RiskLevel, error) {
	levelMap := map[string]tool.RiskLevel{
		"none":     tool.RiskNone,
		"low":      tool.RiskLow,
		"medium":   tool.RiskMedium,
		"high":     tool.RiskHigh,
		"critical": tool.RiskCritical,
	}
	level, ok := levelMap[s]
	if !ok {
		return tool.RiskNone, fmt.Errorf("unknown risk level: %s", s)
	}
	return level, nil
}

func buildEventFilter(types []string) notification.EventFilter {
	eventTypes := make([]notification.EventType, 0, len(types))
	for _, t := range types {
		eventTypes = append(eventTypes, notification.EventType(t))
	}
	return notification.FilterByType(eventTypes...)
}

// DefaultConfig returns a minimal default configuration.
func DefaultConfig() *domainconfig.AgentConfig {
	return &domainconfig.AgentConfig{
		Name:    "agent",
		Version: "1.0",
		Agent: domainconfig.AgentSettings{
			MaxSteps:     100,
			InitialState: "intake",
		},
		Policy: domainconfig.PolicyConfig{
			Budgets: map[string]int{
				"tool_calls": 100,
			},
		},
		Resilience: domainconfig.ResilienceConfig{
			Timeout: domainconfig.Duration(30 * time.Second),
			Retry: domainconfig.RetryConfig{
				Enabled:      true,
				MaxAttempts:  3,
				InitialDelay: domainconfig.Duration(1 * time.Second),
				Multiplier:   2.0,
			},
		},
	}
}
