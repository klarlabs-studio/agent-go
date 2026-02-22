package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	plannerllm "github.com/felixgeelhaar/agent-go/contrib/planner-llm"
)

// CohereConfig configures the Cohere provider.
type CohereConfig struct {
	// APIKey is the Cohere API key.
	APIKey string

	// BaseURL overrides the default API endpoint.
	BaseURL string

	// Model is the default model to use (e.g., "command-r", "command-r-plus").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// CohereProvider implements Provider for Cohere's Chat API (v2).
type CohereProvider struct {
	config CohereConfig
}

// NewCohereProvider creates a new Cohere provider.
func NewCohereProvider(cfg CohereConfig) *CohereProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.cohere.com/v2"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &CohereProvider{config: cfg}
}

type cohereRequest struct {
	Model       string           `json:"model"`
	Messages    []cohereMessage  `json:"messages"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

type cohereMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cohereResponse struct {
	ID      string `json:"id"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	Usage struct {
		Tokens struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"tokens"`
	} `json:"usage"`
	Error *string `json:"error,omitempty"`
}

// Complete sends a completion request to Cohere's v2 Chat API.
func (p *CohereProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	if p.config.APIKey == "" {
		return plannerllm.CompletionResponse{}, ErrMissingAPIKey
	}

	model := resolveModel(req.Model, p.config.Model, "command-r-plus")

	msgs := make([]cohereMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = cohereMessage{Role: m.Role, Content: m.Content}
	}

	body := cohereRequest{Model: model, Messages: msgs}
	if req.Temperature > 0 {
		t := req.Temperature
		body.Temperature = &t
	}
	if req.MaxTokens > 0 {
		body.MaxTokens = req.MaxTokens
	}

	headers := map[string]string{
		"Authorization": "Bearer " + p.config.APIKey,
	}

	url := strings.TrimRight(p.config.BaseURL, "/") + "/chat"
	respBody, err := doRequest(ctx, "POST", url, headers, body, p.config.Timeout)
	if err != nil {
		return plannerllm.CompletionResponse{}, err
	}

	var cResp cohereResponse
	if err := json.Unmarshal(respBody, &cResp); err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if cResp.Error != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("API error: %s", *cResp.Error)
	}

	var content string
	for _, c := range cResp.Message.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	inputTokens := cResp.Usage.Tokens.InputTokens
	outputTokens := cResp.Usage.Tokens.OutputTokens

	return plannerllm.CompletionResponse{
		ID:    cResp.ID,
		Model: model,
		Message: plannerllm.Message{
			Role:    "assistant",
			Content: content,
		},
		Usage: plannerllm.Usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
	}, nil
}

// Name returns the provider name.
func (p *CohereProvider) Name() string { return "cohere" }
