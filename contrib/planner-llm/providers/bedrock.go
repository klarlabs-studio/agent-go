package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	plannerllm "go.klarlabs.de/agent/contrib/planner-llm"
)

// BedrockConfig configures the AWS Bedrock provider.
type BedrockConfig struct {
	// Region is the AWS region (e.g., "us-east-1").
	Region string

	// AccessKeyID is the AWS access key ID.
	AccessKeyID string

	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string

	// SessionToken is the optional AWS session token (for temporary credentials).
	SessionToken string

	// Model is the Bedrock model ID (e.g., "anthropic.claude-3-sonnet-20240229-v1:0").
	Model string

	// Timeout is the request timeout in seconds.
	Timeout int
}

// BedrockProvider implements Provider for AWS Bedrock's Converse API.
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

type bedrockRequest struct {
	Messages        []bedrockMessage `json:"messages"`
	System          []bedrockBlock   `json:"system,omitempty"`
	InferenceConfig *bedrockInfCfg   `json:"inferenceConfig,omitempty"`
	ToolConfig      *bedrockToolCfg  `json:"toolConfig,omitempty"`
}

type bedrockMessage struct {
	Role    string         `json:"role"`
	Content []bedrockBlock `json:"content"`
}

type bedrockBlock struct {
	Text    string          `json:"text,omitempty"`
	ToolUse *bedrockToolUse `json:"toolUse,omitempty"`
}

type bedrockToolUse struct {
	ToolUseID string          `json:"toolUseId"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
}

type bedrockToolCfg struct {
	Tools []bedrockToolSpec `json:"tools"`
}

type bedrockToolSpec struct {
	ToolSpec bedrockToolSpecDef `json:"toolSpec"`
}

type bedrockToolSpecDef struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	InputSchema bedrockSchema `json:"inputSchema"`
}

type bedrockSchema struct {
	JSON json.RawMessage `json:"json"`
}

type bedrockInfCfg struct {
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   int      `json:"maxTokens,omitempty"`
}

type bedrockResponseBlock struct {
	Text    string          `json:"text,omitempty"`
	ToolUse *bedrockToolUse `json:"toolUse,omitempty"`
}

type bedrockResponse struct {
	Output struct {
		Message struct {
			Role    string                 `json:"role"`
			Content []bedrockResponseBlock `json:"content"`
		} `json:"message"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
		TotalTokens  int `json:"totalTokens"`
	} `json:"usage"`
}

// Complete sends a Converse request to AWS Bedrock with SigV4 authentication.
func (p *BedrockProvider) Complete(ctx context.Context, req plannerllm.CompletionRequest) (plannerllm.CompletionResponse, error) {
	if p.config.AccessKeyID == "" || p.config.SecretAccessKey == "" {
		return plannerllm.CompletionResponse{}, fmt.Errorf("%w: AWS credentials required (AccessKeyID, SecretAccessKey)", ErrMissingAPIKey)
	}

	model := resolveModel(req.Model, p.config.Model, "anthropic.claude-3-sonnet-20240229-v1:0")

	var system []bedrockBlock
	var msgs []bedrockMessage
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = append(system, bedrockBlock{Text: m.Content})
			continue
		}
		msgs = append(msgs, bedrockMessage{
			Role:    m.Role,
			Content: []bedrockBlock{{Text: m.Content}},
		})
	}

	body := bedrockRequest{
		Messages: msgs,
		System:   system,
	}
	if req.Temperature > 0 || req.MaxTokens > 0 {
		cfg := &bedrockInfCfg{}
		if req.Temperature > 0 {
			t := req.Temperature
			cfg.Temperature = &t
		}
		if req.MaxTokens > 0 {
			cfg.MaxTokens = req.MaxTokens
		}
		body.InferenceConfig = cfg
	}

	// Convert tool definitions
	if len(req.Tools) > 0 {
		specs := make([]bedrockToolSpec, len(req.Tools))
		for i, t := range req.Tools {
			var schema json.RawMessage
			if t.Function.Parameters != nil {
				switch v := t.Function.Parameters.(type) {
				case json.RawMessage:
					schema = v
				case []byte:
					schema = json.RawMessage(v)
				default:
					b, err := json.Marshal(v)
					if err != nil {
						return plannerllm.CompletionResponse{}, fmt.Errorf("marshal tool schema: %w", err)
					}
					schema = b
				}
			}
			if schema == nil {
				schema = json.RawMessage(`{"type":"object"}`)
			}
			specs[i] = bedrockToolSpec{
				ToolSpec: bedrockToolSpecDef{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					InputSchema: bedrockSchema{JSON: schema},
				},
			}
		}
		body.ToolConfig = &bedrockToolCfg{Tools: specs}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	host := fmt.Sprintf("bedrock-runtime.%s.amazonaws.com", p.config.Region)
	path := fmt.Sprintf("/model/%s/converse", model)
	url := fmt.Sprintf("https://%s%s", host, path)

	now := time.Now().UTC()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Host", host)
	httpReq.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	if p.config.SessionToken != "" {
		httpReq.Header.Set("X-Amz-Security-Token", p.config.SessionToken)
	}

	p.signV4(httpReq, payload, now, host, path)

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
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return plannerllm.CompletionResponse{}, fmt.Errorf("Bedrock API error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	var bResp bedrockResponse
	if err := json.Unmarshal(respBody, &bResp); err != nil {
		return plannerllm.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	var content string
	var toolCalls []plannerllm.ToolCall
	for _, c := range bResp.Output.Message.Content {
		if c.Text != "" {
			content += c.Text
		}
		if c.ToolUse != nil {
			args := string(c.ToolUse.Input)
			toolCalls = append(toolCalls, plannerllm.ToolCall{
				ID:   c.ToolUse.ToolUseID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      c.ToolUse.Name,
					Arguments: args,
				},
			})
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
			PromptTokens:     bResp.Usage.InputTokens,
			CompletionTokens: bResp.Usage.OutputTokens,
			TotalTokens:      bResp.Usage.TotalTokens,
		},
	}, nil
}

// signV4 adds AWS Signature Version 4 headers to the request.
func (p *BedrockProvider) signV4(req *http.Request, payload []byte, now time.Time, host, path string) {
	service := "bedrock"
	datestamp := now.Format("20060102")
	amzdate := now.Format("20060102T150405Z")
	scope := datestamp + "/" + p.config.Region + "/" + service + "/aws4_request"

	payloadHash := sha256Hex(payload)

	// Canonical headers — must be sorted. content-type, host, x-amz-date are sufficient.
	signedHeaders := "content-type;host;x-amz-date"
	canonicalHeaders := "content-type:application/json\n" +
		"host:" + host + "\n" +
		"x-amz-date:" + amzdate + "\n"

	if p.config.SessionToken != "" {
		signedHeaders = "content-type;host;x-amz-date;x-amz-security-token"
		canonicalHeaders = "content-type:application/json\n" +
			"host:" + host + "\n" +
			"x-amz-date:" + amzdate + "\n" +
			"x-amz-security-token:" + p.config.SessionToken + "\n"
	}

	canonicalRequest := strings.Join([]string{
		"POST",
		path,
		"", // query string
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzdate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(p.config.SecretAccessKey, datestamp, p.config.Region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		p.config.AccessKeyID, scope, signedHeaders, signature,
	)
	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func deriveSigningKey(secret, datestamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(datestamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

// Name returns the provider name.
func (p *BedrockProvider) Name() string { return "bedrock" }
