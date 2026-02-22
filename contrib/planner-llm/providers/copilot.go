package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	plannerllm "github.com/felixgeelhaar/agent-go/contrib/planner-llm"
)

// CopilotConfig configures the GitHub Copilot provider.
type CopilotConfig struct {
	// Token is the GitHub Copilot token.
	Token string

	// BaseURL overrides the default API endpoint.
	BaseURL string

	// Model is the model to use (e.g., "gpt-4o", "claude-sonnet-4-5-20250514").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// CopilotProvider implements Provider for GitHub Copilot's OpenAI-compatible API.
type CopilotProvider struct {
	config CopilotConfig
}

// NewCopilotProvider creates a new GitHub Copilot provider.
func NewCopilotProvider(cfg CopilotConfig) *CopilotProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.githubcopilot.com"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &CopilotProvider{config: cfg}
}

// Complete sends a completion request to GitHub Copilot's chat completions API.
// Copilot uses an OpenAI-compatible format.
func (p *CopilotProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	if p.config.Token == "" {
		return plannerllm.CompletionResponse{}, ErrMissingAPIKey
	}

	model := resolveModel(req.Model, p.config.Model, "gpt-4o")

	msgs := make([]openaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openaiMessage{Role: m.Role, Content: m.Content}
	}

	body := openaiRequest{Model: model, Messages: msgs}
	if req.Temperature > 0 {
		t := req.Temperature
		body.Temperature = &t
	}
	if req.MaxTokens > 0 {
		body.MaxTokens = req.MaxTokens
	}

	// Convert tool definitions (Copilot uses OpenAI-compatible format)
	if len(req.Tools) > 0 {
		tools := make([]openaiTool, len(req.Tools))
		for i, t := range req.Tools {
			var params json.RawMessage
			if t.Function.Parameters != nil {
				switch v := t.Function.Parameters.(type) {
				case json.RawMessage:
					params = v
				case []byte:
					params = json.RawMessage(v)
				default:
					b, err := json.Marshal(v)
					if err != nil {
						return plannerllm.CompletionResponse{}, fmt.Errorf("marshal tool parameters: %w", err)
					}
					params = b
				}
			}
			tools[i] = openaiTool{
				Type: "function",
				Function: openaiToolFunction{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  params,
				},
			}
		}
		body.Tools = tools
	}

	headers := map[string]string{
		"Authorization":          "Bearer " + p.config.Token,
		"Copilot-Integration-Id": "agent-go",
		"Editor-Version":         "agent-go/1.0",
	}

	url := strings.TrimRight(p.config.BaseURL, "/") + "/chat/completions"
	respBody, err := doRequest(ctx, "POST", url, headers, body, p.config.Timeout)
	if err != nil {
		return plannerllm.CompletionResponse{}, err
	}

	var oaiResp openaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if oaiResp.Error != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("API error: %s", oaiResp.Error.Message)
	}

	var content string
	var toolCalls []plannerllm.ToolCall
	if len(oaiResp.Choices) > 0 {
		choice := oaiResp.Choices[0].Message
		content = choice.Content
		for _, tc := range choice.ToolCalls {
			toolCalls = append(toolCalls, plannerllm.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return plannerllm.CompletionResponse{
		ID:    oaiResp.ID,
		Model: oaiResp.Model,
		Message: plannerllm.Message{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		},
		Usage: plannerllm.Usage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
			TotalTokens:      oaiResp.Usage.TotalTokens,
		},
	}, nil
}

// Name returns the provider name.
func (p *CopilotProvider) Name() string { return "copilot" }
