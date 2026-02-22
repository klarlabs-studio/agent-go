package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	plannerllm "github.com/felixgeelhaar/agent-go/contrib/planner-llm"
)

// AnthropicConfig configures the Anthropic provider.
type AnthropicConfig struct {
	// APIKey is the Anthropic API key.
	APIKey string

	// BaseURL overrides the default API endpoint.
	BaseURL string

	// Model is the default model to use (e.g., "claude-sonnet-4-5-20250514").
	Model string

	// AnthropicVersion is the API version header. Defaults to "2023-06-01".
	AnthropicVersion string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// AnthropicProvider implements Provider for Anthropic's Messages API.
type AnthropicProvider struct {
	config AnthropicConfig
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(cfg AnthropicConfig) *AnthropicProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com/v1"
	}
	if cfg.AnthropicVersion == "" {
		cfg.AnthropicVersion = "2023-06-01"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &AnthropicProvider{config: cfg}
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a completion request to Anthropic's Messages API.
func (p *AnthropicProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	if p.config.APIKey == "" {
		return plannerllm.CompletionResponse{}, ErrMissingAPIKey
	}

	model := resolveModel(req.Model, p.config.Model, "claude-sonnet-4-5-20250514")

	// Anthropic separates system messages from the messages array.
	var system string
	var msgs []anthropicMessage
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	body := anthropicRequest{
		Model:     model,
		Messages:  msgs,
		System:    system,
		MaxTokens: maxTokens,
	}
	if req.Temperature > 0 {
		t := req.Temperature
		body.Temperature = &t
	}

	headers := map[string]string{
		"x-api-key":         p.config.APIKey,
		"anthropic-version": p.config.AnthropicVersion,
	}

	url := strings.TrimRight(p.config.BaseURL, "/") + "/messages"
	respBody, err := doRequest(ctx, "POST", url, headers, body, p.config.Timeout)
	if err != nil {
		return plannerllm.CompletionResponse{}, err
	}

	var aResp anthropicResponse
	if err := json.Unmarshal(respBody, &aResp); err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if aResp.Error != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("API error (%s): %s", aResp.Error.Type, aResp.Error.Message)
	}

	var content string
	for _, c := range aResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return plannerllm.CompletionResponse{
		ID:    aResp.ID,
		Model: aResp.Model,
		Message: plannerllm.Message{
			Role:    "assistant",
			Content: content,
		},
		Usage: plannerllm.Usage{
			PromptTokens:     aResp.Usage.InputTokens,
			CompletionTokens: aResp.Usage.OutputTokens,
			TotalTokens:      aResp.Usage.InputTokens + aResp.Usage.OutputTokens,
		},
	}, nil
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string { return "anthropic" }
