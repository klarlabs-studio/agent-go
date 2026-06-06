package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	plannerllm "go.klarlabs.de/agent/contrib/planner-llm"
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
	Model       string          `json:"model"`
	Messages    []cohereMessage `json:"messages"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Tools       []cohereTool    `json:"tools,omitempty"`
}

type cohereMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cohereTool struct {
	Type     string             `json:"type"`
	Function cohereToolFunction `json:"function"`
}

type cohereToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type cohereToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type cohereResponse struct {
	ID      string `json:"id"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		ToolCalls []cohereToolCall `json:"tool_calls,omitempty"`
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

	// Convert tool definitions
	if len(req.Tools) > 0 {
		tools := make([]cohereTool, len(req.Tools))
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
			tools[i] = cohereTool{
				Type: "function",
				Function: cohereToolFunction{
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

	var toolCalls []plannerllm.ToolCall
	for _, tc := range cResp.Message.ToolCalls {
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

	inputTokens := cResp.Usage.Tokens.InputTokens
	outputTokens := cResp.Usage.Tokens.OutputTokens

	return plannerllm.CompletionResponse{
		ID:    cResp.ID,
		Model: model,
		Message: plannerllm.Message{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
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
