package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	plannerllm "go.klarlabs.de/agent/contrib/planner-llm"
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

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Tools       []openaiTool    `json:"tools,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiTool struct {
	Type     string             `json:"type"`
	Function openaiToolFunction `json:"function"`
}

type openaiToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a completion request to OpenAI's chat completions API.
func (p *OpenAIProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	if p.config.APIKey == "" {
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

	// Convert tool definitions
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
		"Authorization": "Bearer " + p.config.APIKey,
	}
	if p.config.Organization != "" {
		headers["OpenAI-Organization"] = p.config.Organization
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
func (p *OpenAIProvider) Name() string { return "openai" }
