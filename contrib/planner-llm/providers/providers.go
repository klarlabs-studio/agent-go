// Package providers contains LLM provider implementations for the planner-llm module.
//
// This package provides ready-to-use implementations for various LLM providers:
//
//   - OpenAI (GPT-4, GPT-3.5-turbo)
//   - Anthropic (Claude 3, Claude 2)
//   - Google (Gemini Pro, Gemini Ultra)
//   - Cohere (Command, Command-R)
//   - AWS Bedrock (Claude, Llama, Mistral)
//   - GitHub Copilot (for GitHub integration)
//   - Ollama (local models)
//
// Each provider implements the plannerllm.Provider interface.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	plannerllm "github.com/felixgeelhaar/agent-go/contrib/planner-llm"
)

// Common errors for providers.
var (
	ErrMissingAPIKey    = errors.New("missing API key")
	ErrInvalidModel     = errors.New("invalid model")
	ErrRateLimited      = errors.New("rate limited")
	ErrContextCanceled  = errors.New("context canceled")
	ErrConnectionFailed = errors.New("connection failed")
)

// OpenAIConfig configures the OpenAI provider.
type OpenAIConfig struct {
	// APIKey is the OpenAI API key.
	APIKey string

	// BaseURL overrides the default API endpoint.
	// Useful for Azure OpenAI or compatible APIs.
	BaseURL string

	// Organization is the OpenAI organization ID (optional).
	Organization string

	// Model is the default model to use.
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// OpenAIProvider implements Provider for OpenAI's API.
type OpenAIProvider struct {
	config OpenAIConfig
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &OpenAIProvider{config: cfg}
}

// openaiRequest is the request body for the OpenAI chat completions API.
type openaiRequest struct {
	Model       string           `json:"model"`
	Messages    []openaiMessage  `json:"messages"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse is the response body from the OpenAI chat completions API.
type openaiResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Complete sends a completion request to OpenAI's chat completions API.
func (p *OpenAIProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	if p.config.APIKey == "" {
		return plannerllm.CompletionResponse{}, ErrMissingAPIKey
	}

	model := req.Model
	if model == "" {
		model = p.config.Model
	}
	if model == "" {
		model = "gpt-4o"
	}

	msgs := make([]openaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openaiMessage{Role: m.Role, Content: m.Content}
	}

	body := openaiRequest{
		Model:    model,
		Messages: msgs,
	}
	if req.Temperature > 0 {
		t := req.Temperature
		body.Temperature = &t
	}
	if req.MaxTokens > 0 {
		body.MaxTokens = req.MaxTokens
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(p.config.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	if p.config.Organization != "" {
		httpReq.Header.Set("OpenAI-Organization", p.config.Organization)
	}

	client := &http.Client{Timeout: time.Duration(p.config.Timeout) * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return plannerllm.CompletionResponse{}, ErrContextCanceled
		}
		return plannerllm.CompletionResponse{}, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode == http.StatusTooManyRequests {
		return plannerllm.CompletionResponse{}, ErrRateLimited
	}
	if httpResp.StatusCode != http.StatusOK {
		return plannerllm.CompletionResponse{}, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	var oaiResp openaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if oaiResp.Error != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("API error: %s", oaiResp.Error.Message)
	}

	var content string
	if len(oaiResp.Choices) > 0 {
		content = oaiResp.Choices[0].Message.Content
	}

	return plannerllm.CompletionResponse{
		ID:    oaiResp.ID,
		Model: oaiResp.Model,
		Message: plannerllm.Message{
			Role:    "assistant",
			Content: content,
		},
		Usage: plannerllm.Usage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
			TotalTokens:      oaiResp.Usage.TotalTokens,
		},
	}, nil
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// AnthropicConfig configures the Anthropic provider.
type AnthropicConfig struct {
	// APIKey is the Anthropic API key.
	APIKey string

	// BaseURL overrides the default API endpoint.
	BaseURL string

	// Model is the default model to use (e.g., "claude-3-opus-20240229").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// AnthropicProvider implements Provider for Anthropic's API.
type AnthropicProvider struct {
	config AnthropicConfig
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(cfg AnthropicConfig) *AnthropicProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &AnthropicProvider{config: cfg}
}

// Complete sends a completion request to Anthropic.
func (p *AnthropicProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	// TODO: Implement Anthropic API call
	return plannerllm.CompletionResponse{}, errors.New("Anthropic provider not yet implemented")
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// GeminiConfig configures the Google Gemini provider.
type GeminiConfig struct {
	// APIKey is the Google AI API key.
	APIKey string

	// ProjectID is the Google Cloud project ID (for Vertex AI).
	ProjectID string

	// Location is the Vertex AI location (e.g., "us-central1").
	Location string

	// Model is the default model to use (e.g., "gemini-pro").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// GeminiProvider implements Provider for Google's Gemini API.
type GeminiProvider struct {
	config GeminiConfig
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(cfg GeminiConfig) *GeminiProvider {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &GeminiProvider{config: cfg}
}

// Complete sends a completion request to Gemini.
func (p *GeminiProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	// TODO: Implement Gemini API call
	return plannerllm.CompletionResponse{}, errors.New("Gemini provider not yet implemented")
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// CohereConfig configures the Cohere provider.
type CohereConfig struct {
	// APIKey is the Cohere API key.
	APIKey string

	// BaseURL overrides the default API endpoint.
	BaseURL string

	// Model is the default model to use (e.g., "command-r").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// CohereProvider implements Provider for Cohere's API.
type CohereProvider struct {
	config CohereConfig
}

// NewCohereProvider creates a new Cohere provider.
func NewCohereProvider(cfg CohereConfig) *CohereProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.cohere.ai/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &CohereProvider{config: cfg}
}

// Complete sends a completion request to Cohere.
func (p *CohereProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	// TODO: Implement Cohere API call
	return plannerllm.CompletionResponse{}, errors.New("Cohere provider not yet implemented")
}

// Name returns the provider name.
func (p *CohereProvider) Name() string {
	return "cohere"
}

// BedrockConfig configures the AWS Bedrock provider.
type BedrockConfig struct {
	// Region is the AWS region.
	Region string

	// Model is the Bedrock model ID (e.g., "anthropic.claude-3-opus-20240229-v1:0").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// BedrockProvider implements Provider for AWS Bedrock.
type BedrockProvider struct {
	config BedrockConfig
}

// NewBedrockProvider creates a new Bedrock provider.
func NewBedrockProvider(cfg BedrockConfig) *BedrockProvider {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &BedrockProvider{config: cfg}
}

// Complete sends a completion request to Bedrock.
func (p *BedrockProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	// TODO: Implement Bedrock API call
	return plannerllm.CompletionResponse{}, errors.New("Bedrock provider not yet implemented")
}

// Name returns the provider name.
func (p *BedrockProvider) Name() string {
	return "bedrock"
}

// OllamaConfig configures the Ollama provider for local models.
type OllamaConfig struct {
	// BaseURL is the Ollama API endpoint.
	BaseURL string

	// Model is the model name (e.g., "llama3", "mistral").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// OllamaProvider implements Provider for local Ollama models.
type OllamaProvider struct {
	config OllamaConfig
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(cfg OllamaConfig) *OllamaProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 // Longer timeout for local models
	}
	return &OllamaProvider{config: cfg}
}

// Complete sends a completion request to Ollama.
func (p *OllamaProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	// TODO: Implement Ollama API call
	return plannerllm.CompletionResponse{}, errors.New("Ollama provider not yet implemented")
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// CopilotConfig configures the GitHub Copilot provider.
type CopilotConfig struct {
	// Token is the GitHub Copilot token.
	Token string

	// Model is the model to use (typically "copilot").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// CopilotProvider implements Provider for GitHub Copilot.
type CopilotProvider struct {
	config CopilotConfig
}

// NewCopilotProvider creates a new GitHub Copilot provider.
func NewCopilotProvider(cfg CopilotConfig) *CopilotProvider {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &CopilotProvider{config: cfg}
}

// Complete sends a completion request to GitHub Copilot.
func (p *CopilotProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	// TODO: Implement Copilot API call
	return plannerllm.CompletionResponse{}, errors.New("Copilot provider not yet implemented")
}

// Name returns the provider name.
func (p *CopilotProvider) Name() string {
	return "copilot"
}

// Ensure all providers implement the Provider interface.
var (
	_ plannerllm.Provider = (*OpenAIProvider)(nil)
	_ plannerllm.Provider = (*AnthropicProvider)(nil)
	_ plannerllm.Provider = (*GeminiProvider)(nil)
	_ plannerllm.Provider = (*CohereProvider)(nil)
	_ plannerllm.Provider = (*BedrockProvider)(nil)
	_ plannerllm.Provider = (*OllamaProvider)(nil)
	_ plannerllm.Provider = (*CopilotProvider)(nil)
)
