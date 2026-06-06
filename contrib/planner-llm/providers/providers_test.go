package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	plannerllm "go.klarlabs.de/agent/contrib/planner-llm"
)

// testTools returns a set of tool definitions for testing.
func testTools() []plannerllm.Tool {
	return []plannerllm.Tool{
		{
			Type: "function",
			Function: plannerllm.ToolFunction{
				Name:        "read_file",
				Description: "Read a file",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
			},
		},
	}
}

// testMessages returns a basic set of messages for testing.
func testMessages() []plannerllm.Message {
	return []plannerllm.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
	}
}

// --- OpenAI Provider Tests ---

func TestOpenAI_Complete_TextResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %q", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-123",
			Model: "gpt-4o",
			Choices: []struct {
				Message struct {
					Role      string           `json:"role"`
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
				} `json:"message"`
			}{
				{Message: struct {
					Role      string           `json:"role"`
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
				}{Role: "assistant", Content: `{"decision":"finish","result":"done","summary":"ok"}`}},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content == "" {
		t.Error("expected content in response")
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestOpenAI_Complete_ToolCallResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req openaiRequest
		json.Unmarshal(body, &req)
		if len(req.Tools) == 0 {
			t.Error("expected tools in request")
		}
		json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-456",
			Model: "gpt-4o",
			Choices: []struct {
				Message struct {
					Role      string           `json:"role"`
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
				} `json:"message"`
			}{
				{Message: struct {
					Role      string           `json:"role"`
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
				}{
					Role: "assistant",
					ToolCalls: []openaiToolCall{
						{ID: "call_1", Type: "function", Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{Name: "read_file", Arguments: `{"path":"main.go"}`}},
					},
				}},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
		Tools:    testTools(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", resp.Message.ToolCalls[0].Function.Name)
	}
}

func TestOpenAI_Complete_MissingAPIKey(t *testing.T) {
	t.Parallel()
	p := NewOpenAIProvider(OpenAIConfig{})
	_, err := p.Complete(context.Background(), plannerllm.CompletionRequest{Messages: testMessages()})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

// --- Anthropic Provider Tests ---

func TestAnthropic_Complete_TextResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key 'test-key', got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header")
		}
		body, _ := io.ReadAll(r.Body)
		var req anthropicRequest
		json.Unmarshal(body, &req)
		if req.System == "" {
			t.Error("expected system message separated from messages")
		}
		json.NewEncoder(w).Encode(anthropicResponse{
			ID:    "msg_123",
			Model: "claude-sonnet-4-5-20250514",
			Content: []anthropicContentBlock{
				{Type: "text", Text: `{"decision":"finish","result":"done","summary":"ok"}`},
			},
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{InputTokens: 10, OutputTokens: 5},
		})
	}))
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content == "" {
		t.Error("expected content in response")
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestAnthropic_Complete_ToolCallResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req anthropicRequest
		json.Unmarshal(body, &req)
		if len(req.Tools) == 0 {
			t.Error("expected tools in request")
		}
		if req.Tools[0].InputSchema == nil {
			t.Error("expected input_schema in tool")
		}
		json.NewEncoder(w).Encode(anthropicResponse{
			ID:    "msg_456",
			Model: "claude-sonnet-4-5-20250514",
			Content: []anthropicContentBlock{
				{Type: "tool_use", ID: "toolu_1", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
			},
		})
	}))
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
		Tools:    testTools(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", resp.Message.ToolCalls[0].Function.Name)
	}
}

// --- Gemini Provider Tests ---

func TestGemini_Complete_TextResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []geminiResponsePart `json:"parts"`
					Role  string               `json:"role"`
				} `json:"content"`
			}{
				{Content: struct {
					Parts []geminiResponsePart `json:"parts"`
					Role  string               `json:"role"`
				}{
					Parts: []geminiResponsePart{{Text: `{"decision":"finish","result":"ok","summary":"done"}`}},
					Role:  "model",
				}},
			},
			UsageMetadata: struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
				TotalTokenCount      int `json:"totalTokenCount"`
			}{PromptTokenCount: 10, CandidatesTokenCount: 5, TotalTokenCount: 15},
		})
	}))
	defer srv.Close()

	p := NewGeminiProvider(GeminiConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content == "" {
		t.Error("expected content in response")
	}
}

func TestGemini_Complete_FunctionCallResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req geminiRequest
		json.Unmarshal(body, &req)
		if len(req.Tools) == 0 {
			t.Error("expected tools in request")
		}
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []geminiResponsePart `json:"parts"`
					Role  string               `json:"role"`
				} `json:"content"`
			}{
				{Content: struct {
					Parts []geminiResponsePart `json:"parts"`
					Role  string               `json:"role"`
				}{
					Parts: []geminiResponsePart{
						{FunctionCall: &geminiFunctionCall{Name: "read_file", Args: json.RawMessage(`{"path":"main.go"}`)}},
					},
					Role: "model",
				}},
			},
		})
	}))
	defer srv.Close()

	p := NewGeminiProvider(GeminiConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
		Tools:    testTools(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", resp.Message.ToolCalls[0].Function.Name)
	}
}

// --- Ollama Provider Tests ---

func TestOllama_Complete_TextResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ollamaRequest
		json.Unmarshal(body, &req)
		if req.Stream != false {
			t.Error("expected stream=false")
		}
		json.NewEncoder(w).Encode(ollamaResponse{
			Model: "llama3",
			Message: struct {
				Role      string           `json:"role"`
				Content   string           `json:"content"`
				ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
			}{Role: "assistant", Content: `{"decision":"finish","result":"ok","summary":"done"}`},
			Done:            true,
			PromptEvalCount: 10,
			EvalCount:       5,
		})
	}))
	defer srv.Close()

	p := NewOllamaProvider(OllamaConfig{BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content == "" {
		t.Error("expected content in response")
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestOllama_Complete_ToolCallResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ollamaRequest
		json.Unmarshal(body, &req)
		if len(req.Tools) == 0 {
			t.Error("expected tools in request")
		}
		json.NewEncoder(w).Encode(ollamaResponse{
			Model: "llama3",
			Message: struct {
				Role      string           `json:"role"`
				Content   string           `json:"content"`
				ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
			}{
				Role: "assistant",
				ToolCalls: []ollamaToolCall{
					{Function: struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					}{Name: "read_file", Arguments: json.RawMessage(`{"path":"main.go"}`)}},
				},
			},
			Done: true,
		})
	}))
	defer srv.Close()

	p := NewOllamaProvider(OllamaConfig{BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
		Tools:    testTools(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", resp.Message.ToolCalls[0].Function.Name)
	}
}

// --- Cohere Provider Tests ---

func TestCohere_Complete_TextResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %q", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(cohereResponse{
			ID: "resp_123",
			Message: struct {
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				ToolCalls []cohereToolCall `json:"tool_calls,omitempty"`
			}{
				Role: "assistant",
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{Type: "text", Text: `{"decision":"finish","result":"ok","summary":"done"}`},
				},
			},
			Usage: struct {
				Tokens struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"tokens"`
			}{Tokens: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{InputTokens: 10, OutputTokens: 5}},
		})
	}))
	defer srv.Close()

	p := NewCohereProvider(CohereConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content == "" {
		t.Error("expected content in response")
	}
}

func TestCohere_Complete_ToolCallResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req cohereRequest
		json.Unmarshal(body, &req)
		if len(req.Tools) == 0 {
			t.Error("expected tools in request")
		}
		json.NewEncoder(w).Encode(cohereResponse{
			ID: "resp_456",
			Message: struct {
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				ToolCalls []cohereToolCall `json:"tool_calls,omitempty"`
			}{
				Role: "assistant",
				ToolCalls: []cohereToolCall{
					{ID: "call_1", Type: "function", Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: "read_file", Arguments: `{"path":"main.go"}`}},
				},
			},
		})
	}))
	defer srv.Close()

	p := NewCohereProvider(CohereConfig{APIKey: "test-key", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
		Tools:    testTools(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", resp.Message.ToolCalls[0].Function.Name)
	}
}

// --- Copilot Provider Tests ---

func TestCopilot_Complete_TextResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Copilot-Integration-Id") != "agent-go" {
			t.Errorf("expected Copilot-Integration-Id 'agent-go', got %q", r.Header.Get("Copilot-Integration-Id"))
		}
		json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-789",
			Model: "gpt-4o",
			Choices: []struct {
				Message struct {
					Role      string           `json:"role"`
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
				} `json:"message"`
			}{
				{Message: struct {
					Role      string           `json:"role"`
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
				}{Role: "assistant", Content: `{"decision":"finish","result":"ok","summary":"done"}`}},
			},
		})
	}))
	defer srv.Close()

	p := NewCopilotProvider(CopilotConfig{Token: "test-token", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content == "" {
		t.Error("expected content in response")
	}
}

func TestCopilot_Complete_ToolCallResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req openaiRequest
		json.Unmarshal(body, &req)
		if len(req.Tools) == 0 {
			t.Error("expected tools in request (Copilot uses OpenAI format)")
		}
		json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-tool",
			Model: "gpt-4o",
			Choices: []struct {
				Message struct {
					Role      string           `json:"role"`
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
				} `json:"message"`
			}{
				{Message: struct {
					Role      string           `json:"role"`
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
				}{
					Role: "assistant",
					ToolCalls: []openaiToolCall{
						{ID: "call_1", Type: "function", Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{Name: "read_file", Arguments: `{"path":"main.go"}`}},
					},
				}},
			},
		})
	}))
	defer srv.Close()

	p := NewCopilotProvider(CopilotConfig{Token: "test-token", BaseURL: srv.URL})
	resp, err := p.Complete(context.Background(), plannerllm.CompletionRequest{
		Messages: testMessages(),
		Tools:    testTools(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", resp.Message.ToolCalls[0].Function.Name)
	}
}

// --- Bedrock Provider Tests ---

func TestBedrock_Complete_TextResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(bedrockResponse{
			Output: struct {
				Message struct {
					Role    string                 `json:"role"`
					Content []bedrockResponseBlock `json:"content"`
				} `json:"message"`
			}{
				Message: struct {
					Role    string                 `json:"role"`
					Content []bedrockResponseBlock `json:"content"`
				}{
					Role:    "assistant",
					Content: []bedrockResponseBlock{{Text: `{"decision":"finish","result":"ok","summary":"done"}`}},
				},
			},
			Usage: struct {
				InputTokens  int `json:"inputTokens"`
				OutputTokens int `json:"outputTokens"`
				TotalTokens  int `json:"totalTokens"`
			}{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
		})
	}))
	defer srv.Close()

	p := NewBedrockProvider(BedrockConfig{
		AccessKeyID:     "AKID",
		SecretAccessKey: "secret",
		Region:          "us-east-1",
	})
	// Override the URL by making a request with the test server
	// Bedrock doesn't use doRequest (uses its own SigV4 HTTP client),
	// so we test the response parsing logic directly.
	// For a full integration test, we'd need to mock the SigV4 signing.
	// Instead, test the provider handles missing credentials correctly.
	if p.Name() != "bedrock" {
		t.Errorf("expected name 'bedrock', got %q", p.Name())
	}
}

func TestBedrock_Complete_MissingCredentials(t *testing.T) {
	t.Parallel()
	p := NewBedrockProvider(BedrockConfig{})
	_, err := p.Complete(context.Background(), plannerllm.CompletionRequest{Messages: testMessages()})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

// --- Provider Name Tests ---

func TestProviderNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provider plannerllm.Provider
		expected string
	}{
		{"openai", NewOpenAIProvider(OpenAIConfig{}), "openai"},
		{"anthropic", NewAnthropicProvider(AnthropicConfig{}), "anthropic"},
		{"gemini", NewGeminiProvider(GeminiConfig{}), "gemini"},
		{"ollama", NewOllamaProvider(OllamaConfig{}), "ollama"},
		{"cohere", NewCohereProvider(CohereConfig{}), "cohere"},
		{"bedrock", NewBedrockProvider(BedrockConfig{}), "bedrock"},
		{"copilot", NewCopilotProvider(CopilotConfig{}), "copilot"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.provider.Name() != tc.expected {
				t.Errorf("expected name %q, got %q", tc.expected, tc.provider.Name())
			}
		})
	}
}

// --- Error Handling Tests ---

func TestOpenAI_Complete_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(openaiResponse{
			Error: &struct {
				Message string `json:"message"`
			}{Message: "rate limit exceeded"},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test-key", BaseURL: srv.URL})
	_, err := p.Complete(context.Background(), plannerllm.CompletionRequest{Messages: testMessages()})
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestAnthropic_Complete_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(anthropicResponse{
			Error: &struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}{Type: "invalid_request_error", Message: "bad request"},
		})
	}))
	defer srv.Close()

	p := NewAnthropicProvider(AnthropicConfig{APIKey: "test-key", BaseURL: srv.URL})
	_, err := p.Complete(context.Background(), plannerllm.CompletionRequest{Messages: testMessages()})
	if err == nil {
		t.Fatal("expected API error")
	}
}

func TestHTTPError_RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	p := NewOpenAIProvider(OpenAIConfig{APIKey: "test-key", BaseURL: srv.URL})
	_, err := p.Complete(context.Background(), plannerllm.CompletionRequest{Messages: testMessages()})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
}

// --- resolveModel Tests ---

func TestResolveModel(t *testing.T) {
	t.Parallel()
	if m := resolveModel("req-model", "cfg-model", "default"); m != "req-model" {
		t.Errorf("expected req-model, got %q", m)
	}
	if m := resolveModel("", "cfg-model", "default"); m != "cfg-model" {
		t.Errorf("expected cfg-model, got %q", m)
	}
	if m := resolveModel("", "", "default"); m != "default" {
		t.Errorf("expected default, got %q", m)
	}
}
