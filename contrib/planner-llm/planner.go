// Package plannerllm provides LLM-based planner implementations for agent-go.
//
// This package wraps the core planner interface to provide integrations with
// various LLM providers including OpenAI, Anthropic, Google Gemini, Cohere,
// AWS Bedrock, and GitHub Copilot.
//
// # Usage
//
//	provider := plannerllm.NewOpenAIProvider(plannerllm.OpenAIConfig{
//		APIKey: os.Getenv("OPENAI_API_KEY"),
//		Model:  "gpt-4",
//	})
//
//	llmPlanner := plannerllm.NewPlanner(plannerllm.Config{
//		Provider:    provider,
//		Temperature: 0.7,
//		MaxTokens:   4096,
//	})
//
//	// Use planner with agent engine
//	engine, err := api.New(api.WithPlanner(llmPlanner))
package plannerllm

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/infrastructure/planner"
)

// Provider defines the interface for LLM providers.
// Each provider implementation handles the specifics of communicating
// with a particular LLM service (OpenAI, Anthropic, etc.).
type Provider interface {
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// Name returns the provider name for logging and metrics.
	Name() string
}

// CompletionRequest represents a chat completion request.
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// CompletionResponse represents a chat completion response.
type CompletionResponse struct {
	ID      string  `json:"id"`
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Usage   Usage   `json:"usage"`
	Error   error   `json:"error,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role      string     `json:"role"` // system, user, assistant, tool
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Tool represents a tool definition for function calling.
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ToolCall represents a tool invocation from the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Usage contains token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Config configures the LLM planner.
type Config struct {
	// Provider is the LLM provider to use.
	Provider Provider

	// Model is the model identifier (provider-specific).
	Model string

	// Temperature controls randomness (0.0 to 1.0).
	Temperature float64

	// MaxTokens limits the response length.
	MaxTokens int

	// SystemPrompt overrides the default system prompt.
	SystemPrompt string
}

// LLMPlanner uses an LLM provider to make planning decisions.
type LLMPlanner struct {
	provider     Provider
	model        string
	temperature  float64
	maxTokens    int
	systemPrompt string
}

// NewPlanner creates a new LLM-based planner with the given configuration.
func NewPlanner(cfg Config) *LLMPlanner {
	temperature := cfg.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	return &LLMPlanner{
		provider:     cfg.Provider,
		model:        cfg.Model,
		temperature:  temperature,
		maxTokens:    maxTokens,
		systemPrompt: systemPrompt,
	}
}

// Plan implements the planner.Planner interface.
// It builds a prompt from the plan request, sends it to the LLM provider,
// and parses the response into a Decision.
func (p *LLMPlanner) Plan(ctx context.Context, req planner.PlanRequest) (agent.Decision, error) {
	// Build messages from request context
	messages := BuildMessages(p.systemPrompt, req)

	// Convert tool defs for providers that support native tool calling
	var tools []Tool
	for _, td := range req.ToolDefs {
		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        td.Name,
				Description: td.Description,
				Parameters:  json.RawMessage(td.InputSchema),
			},
		})
	}

	// Call provider
	resp, err := p.provider.Complete(ctx, CompletionRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: p.temperature,
		MaxTokens:   p.maxTokens,
		Tools:       tools,
	})
	if err != nil {
		return agent.Decision{}, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse response — native tool calls take priority over text JSON
	if len(resp.Message.ToolCalls) > 0 {
		return ParseToolCalls(resp.Message.ToolCalls)
	}
	return ParseDecisionJSON(resp.Message.Content)
}

// DefaultSystemPrompt is the default system prompt for the agent planner.
const DefaultSystemPrompt = `You are an AI agent that helps accomplish goals by making decisions and using tools.

Your role is to analyze the current state, evidence, and available tools to decide the next action.

## Response Format

You MUST respond with a JSON object in one of these formats:

### 1. Call a Tool
{"decision": "call_tool", "tool_name": "<name>", "input": {...}, "reason": "<why>"}

### 2. Transition State
{"decision": "transition", "to_state": "<state>", "reason": "<why>"}

Valid states: intake, explore, decide, act, validate, done, failed

### 3. Finish Successfully
{"decision": "finish", "result": <any>, "summary": "<brief summary>"}

### 4. Fail
{"decision": "fail", "reason": "<why failed>"}

### 5. Ask Human
{"decision": "ask_human", "question": "<what to ask>", "options": ["opt1", "opt2"]}

## Guidelines

1. In "explore" state: Gather information using read-only tools
2. In "act" state: Execute actions using available tools
3. In "validate" state: Verify results and decide if goal is achieved
4. Always provide a reason for your decisions
5. Respond ONLY with valid JSON, no additional text`

// Ensure LLMPlanner implements the infrastructure planner.Planner interface.
var _ planner.Planner = (*LLMPlanner)(nil)
