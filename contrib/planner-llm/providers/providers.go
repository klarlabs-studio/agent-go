// Package providers contains LLM provider implementations for the planner-llm module.
//
// This package provides ready-to-use implementations for various LLM providers:
//
//   - OpenAI (GPT-4o, GPT-4, GPT-3.5-turbo) — also supports Azure OpenAI via BaseURL
//   - Anthropic (Claude 4, Claude 3.5)
//   - Google Gemini (Gemini Pro, Gemini Ultra)
//   - Cohere (Command-R, Command-R+)
//   - AWS Bedrock (Claude, Llama, Mistral via SigV4)
//   - GitHub Copilot (OpenAI-compatible endpoint)
//   - Ollama (local models via native /api/chat)
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
	"time"

	plannerllm "go.klarlabs.de/agent/contrib/planner-llm"
)

// Common errors for providers.
var (
	ErrMissingAPIKey    = errors.New("missing API key")
	ErrInvalidModel     = errors.New("invalid model")
	ErrRateLimited      = errors.New("rate limited")
	ErrContextCanceled  = errors.New("context canceled")
	ErrConnectionFailed = errors.New("connection failed")
)

// doRequest performs an HTTP request with standard error handling.
// It marshals the body, sets headers, executes the request, and returns the raw response bytes.
func doRequest(ctx context.Context, method, url string, headers map[string]string, body any, timeoutSec int) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ErrContextCanceled
		}
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// resolveModel returns the first non-empty model string.
func resolveModel(requestModel, configModel, defaultModel string) string {
	if requestModel != "" {
		return requestModel
	}
	if configModel != "" {
		return configModel
	}
	return defaultModel
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
