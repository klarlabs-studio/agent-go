package providers

import (
	"context"
	"encoding/json"
	"fmt"

	plannerllm "go.klarlabs.de/agent/contrib/planner-llm"
)

// GeminiConfig configures the Google Gemini provider.
type GeminiConfig struct {
	// APIKey is the Google AI API key.
	APIKey string

	// BaseURL overrides the default API endpoint. Defaults to Google AI Studio.
	BaseURL string

	// Model is the default model to use (e.g., "gemini-pro", "gemini-1.5-pro").
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
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	return &GeminiProvider{config: cfg}
}

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	SystemInstruct   *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig *geminiGenConfig `json:"generationConfig,omitempty"`
	Tools            []geminiToolDef  `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type geminiToolDef struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiGenConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
}

type geminiResponsePart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiResponsePart `json:"parts"`
			Role  string               `json:"role"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a completion request to Google's Gemini API.
func (p *GeminiProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	if p.config.APIKey == "" {
		return plannerllm.CompletionResponse{}, ErrMissingAPIKey
	}

	model := resolveModel(req.Model, p.config.Model, "gemini-2.0-flash")

	var system *geminiContent
	var contents []geminiContent
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = &geminiContent{Parts: []geminiPart{{Text: m.Content}}}
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model" // Gemini uses "model" instead of "assistant"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	body := geminiRequest{
		Contents:       contents,
		SystemInstruct: system,
	}

	if req.Temperature > 0 || req.MaxTokens > 0 {
		gc := &geminiGenConfig{}
		if req.Temperature > 0 {
			t := req.Temperature
			gc.Temperature = &t
		}
		if req.MaxTokens > 0 {
			gc.MaxOutputTokens = req.MaxTokens
		}
		body.GenerationConfig = gc
	}

	// Convert tool definitions
	if len(req.Tools) > 0 {
		decls := make([]geminiFuncDecl, len(req.Tools))
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
			decls[i] = geminiFuncDecl{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  params,
			}
		}
		body.Tools = []geminiToolDef{{FunctionDeclarations: decls}}
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.config.BaseURL, model, p.config.APIKey)
	respBody, err := doRequest(ctx, "POST", url, nil, body, p.config.Timeout)
	if err != nil {
		return plannerllm.CompletionResponse{}, err
	}

	var gResp geminiResponse
	if err := json.Unmarshal(respBody, &gResp); err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if gResp.Error != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("API error (%d): %s", gResp.Error.Code, gResp.Error.Message)
	}

	var content string
	var toolCalls []plannerllm.ToolCall
	if len(gResp.Candidates) > 0 {
		for i, part := range gResp.Candidates[0].Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
			if part.FunctionCall != nil {
				args := string(part.FunctionCall.Args)
				toolCalls = append(toolCalls, plannerllm.ToolCall{
					ID:   fmt.Sprintf("call_%d", i),
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      part.FunctionCall.Name,
						Arguments: args,
					},
				})
			}
		}
	}

	return plannerllm.CompletionResponse{
		Model: model,
		Message: plannerllm.Message{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		},
		Usage: plannerllm.Usage{
			PromptTokens:     gResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: gResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gResp.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string { return "gemini" }
