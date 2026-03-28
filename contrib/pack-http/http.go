// Package http provides HTTP request tools for agent-go.
//
// This pack includes tools for making HTTP requests:
//   - http_get: Perform HTTP GET requests
//   - http_post: Perform HTTP POST requests with JSON body
//   - http_put: Perform HTTP PUT requests with JSON body
//   - http_delete: Perform HTTP DELETE requests
//   - http_head: Perform HTTP HEAD requests
//   - http_patch: Perform HTTP PATCH requests with JSON body
//
// All tools support custom headers, timeouts, and return structured responses
// including status code, headers, body, content type, and request duration.
//
// Security: Response bodies are capped at 1MB to prevent OOM. Redirect chains
// are limited to 10 hops. All requests respect context cancellation.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

const (
	// defaultTimeout is the default request timeout when none is specified.
	defaultTimeout = 30 * time.Second

	// maxResponseBody is the maximum response body size (1MB).
	maxResponseBody = 1 << 20

	// maxRedirects is the maximum number of redirects to follow.
	maxRedirects = 10
)

// requestInput is the common input structure for all HTTP tools.
type requestInput struct {
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	TimeoutSecs *float64          `json:"timeout_secs,omitempty"`
}

// responseOutput is the structured response returned by all HTTP tools.
type responseOutput struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type"`
	DurationMs  float64           `json:"duration_ms"`
}

// newClient creates an http.Client configured with timeout and redirect policy.
func newClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
}

// parseTimeout returns the timeout duration from input, or the default.
func parseTimeout(secs *float64) time.Duration {
	if secs != nil && *secs > 0 {
		return time.Duration(*secs * float64(time.Second))
	}
	return defaultTimeout
}

// doRequest executes an HTTP request and returns a structured tool result.
// The method parameter determines the HTTP method. If body is non-empty it is
// sent as the request body. The includeBody flag controls whether the response
// body is read and included (false for HEAD requests).
func doRequest(ctx context.Context, method string, input requestInput, includeBody bool) (tool.Result, error) {
	if input.URL == "" {
		return tool.Result{}, fmt.Errorf("url is required")
	}

	timeout := parseTimeout(input.TimeoutSecs)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if input.Body != "" {
		bodyReader = strings.NewReader(input.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, input.URL, bodyReader)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type for requests that carry a body.
	if input.Body != "" && input.ContentType != "" {
		req.Header.Set("Content-Type", input.ContentType)
	} else if input.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply custom headers (after defaults so callers can override).
	for k, v := range input.Headers {
		req.Header.Set(k, v)
	}

	client := newClient(timeout)

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return tool.Result{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	out := responseOutput{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		DurationMs:  float64(duration.Milliseconds()),
		Headers:     flattenHeaders(resp.Header),
	}

	if includeBody {
		limited := io.LimitReader(resp.Body, maxResponseBody+1)
		data, err := io.ReadAll(limited)
		if err != nil {
			return tool.Result{}, fmt.Errorf("failed to read response body: %w", err)
		}
		if len(data) > maxResponseBody {
			data = data[:maxResponseBody]
			out.Body = string(data)
			out.Headers["X-Agent-Truncated"] = "true"
		} else {
			out.Body = string(data)
		}
	}

	raw, err := json.Marshal(out)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to marshal response: %w", err)
	}

	return tool.NewResultWithDuration(raw, duration), nil
}

// flattenHeaders converts multi-value headers to single-value by joining with ", ".
func flattenHeaders(h http.Header) map[string]string {
	flat := make(map[string]string, len(h))
	for k, v := range h {
		flat[k] = strings.Join(v, ", ")
	}
	return flat
}

// parseInput unmarshals the JSON input into a requestInput struct.
func parseInput(raw json.RawMessage) (requestInput, error) {
	var in requestInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return requestInput{}, fmt.Errorf("invalid input: %w", err)
	}
	return in, nil
}

// Pack returns the HTTP tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("http").
		WithDescription("HTTP request tools for making web API calls").
		WithVersion("0.1.0").
		AddTools(
			httpGet(),
			httpPost(),
			httpPut(),
			httpDelete(),
			httpHead(),
			httpPatch(),
		).
		AllowInState(agent.StateExplore, "http_get", "http_head").
		AllowInState(agent.StateAct, "http_get", "http_post", "http_put", "http_delete", "http_head", "http_patch").
		Build()
}

func httpGet() tool.Tool {
	return tool.NewBuilder("http_get").
		WithDescription("Perform an HTTP GET request").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := parseInput(input)
			if err != nil {
				return tool.Result{}, err
			}
			return doRequest(ctx, http.MethodGet, in, true)
		}).
		MustBuild()
}

func httpPost() tool.Tool {
	return tool.NewBuilder("http_post").
		WithDescription("Perform an HTTP POST request with JSON body").
		WithRiskLevel(tool.RiskMedium).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := parseInput(input)
			if err != nil {
				return tool.Result{}, err
			}
			return doRequest(ctx, http.MethodPost, in, true)
		}).
		MustBuild()
}

func httpPut() tool.Tool {
	return tool.NewBuilder("http_put").
		WithDescription("Perform an HTTP PUT request with JSON body").
		WithRiskLevel(tool.RiskMedium).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := parseInput(input)
			if err != nil {
				return tool.Result{}, err
			}
			return doRequest(ctx, http.MethodPut, in, true)
		}).
		MustBuild()
}

func httpDelete() tool.Tool {
	return tool.NewBuilder("http_delete").
		WithDescription("Perform an HTTP DELETE request").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := parseInput(input)
			if err != nil {
				return tool.Result{}, err
			}
			return doRequest(ctx, http.MethodDelete, in, true)
		}).
		MustBuild()
}

func httpHead() tool.Tool {
	return tool.NewBuilder("http_head").
		WithDescription("Perform an HTTP HEAD request").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := parseInput(input)
			if err != nil {
				return tool.Result{}, err
			}
			return doRequest(ctx, http.MethodHead, in, false)
		}).
		MustBuild()
}

func httpPatch() tool.Tool {
	return tool.NewBuilder("http_patch").
		WithDescription("Perform an HTTP PATCH request with JSON body").
		WithRiskLevel(tool.RiskMedium).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := parseInput(input)
			if err != nil {
				return tool.Result{}, err
			}
			return doRequest(ctx, http.MethodPatch, in, true)
		}).
		MustBuild()
}
