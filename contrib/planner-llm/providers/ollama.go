package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	plannerllm "github.com/felixgeelhaar/agent-go/contrib/planner-llm"
)

// OllamaConfig configures the Ollama provider for local models.
type OllamaConfig struct {
	// BaseURL is the Ollama API endpoint.
	BaseURL string

	// Model is the model name (e.g., "llama3", "mistral", "codellama").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// OllamaProvider implements Provider for local Ollama models via /api/chat.
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

type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Options  *ollamaOptions   `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Model   string `json:"model"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done            bool `json:"done"`
	PromptEvalCount int  `json:"prompt_eval_count"`
	EvalCount       int  `json:"eval_count"`
	Error           string `json:"error,omitempty"`
}

// Complete sends a completion request to Ollama's /api/chat endpoint.
func (p *OllamaProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	model := resolveModel(req.Model, p.config.Model, "llama3")

	msgs := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}

	body := ollamaRequest{
		Model:    model,
		Messages: msgs,
		Stream:   false,
	}

	if req.Temperature > 0 || req.MaxTokens > 0 {
		opts := &ollamaOptions{}
		if req.Temperature > 0 {
			opts.Temperature = req.Temperature
		}
		if req.MaxTokens > 0 {
			opts.NumPredict = req.MaxTokens
		}
		body.Options = opts
	}

	url := strings.TrimRight(p.config.BaseURL, "/") + "/api/chat"
	respBody, err := doRequest(ctx, "POST", url, nil, body, p.config.Timeout)
	if err != nil {
		return plannerllm.CompletionResponse{}, err
	}

	var oResp ollamaResponse
	if err := json.Unmarshal(respBody, &oResp); err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if oResp.Error != "" {
		return plannerllm.CompletionResponse{}, fmt.Errorf("Ollama error: %s", oResp.Error)
	}

	return plannerllm.CompletionResponse{
		Model: oResp.Model,
		Message: plannerllm.Message{
			Role:    oResp.Message.Role,
			Content: oResp.Message.Content,
		},
		Usage: plannerllm.Usage{
			PromptTokens:     oResp.PromptEvalCount,
			CompletionTokens: oResp.EvalCount,
			TotalTokens:      oResp.PromptEvalCount + oResp.EvalCount,
		},
	}, nil
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string { return "ollama" }
