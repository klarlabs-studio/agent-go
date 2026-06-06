package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	packhttp "go.klarlabs.de/agent/contrib/pack-http"
	"go.klarlabs.de/agent/domain/tool"
)

func TestRegister(t *testing.T) {
	p := packhttp.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() returned no tools")
	}
	if p.Name != "http" {
		t.Errorf("expected pack name %q, got %q", "http", p.Name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := packhttp.Pack()
	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
		if tt.Description() == "" {
			t.Errorf("tool %q has empty description", tt.Name())
		}
	}
}

// findTool looks up a tool by name from the pack.
func findTool(t *testing.T, name string) tool.Tool {
	t.Helper()
	p := packhttp.Pack()
	for _, tt := range p.Tools {
		if tt.Name() == name {
			return tt
		}
	}
	t.Fatalf("tool %q not found in pack", name)
	return nil
}

// responseOutput mirrors the handler output for test assertions.
type responseOutput struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type"`
	DurationMs  float64           `json:"duration_ms"`
}

func parseOutput(t *testing.T, result tool.Result) responseOutput {
	t.Helper()
	var out responseOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v (raw: %s)", err, string(result.Output))
	}
	return out
}

func TestHTTPGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"message":"hello"}`)
	}))
	defer srv.Close()

	tl := findTool(t, "http_get")
	input, _ := json.Marshal(map[string]any{"url": srv.URL})
	result, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := parseOutput(t, result)
	if out.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", out.StatusCode)
	}
	if out.Body != `{"message":"hello"}` {
		t.Errorf("unexpected body: %s", out.Body)
	}
	if out.Headers["X-Custom"] != "test-value" {
		t.Errorf("expected header X-Custom=test-value, got %q", out.Headers["X-Custom"])
	}
	if out.DurationMs < 0 {
		t.Error("duration should be non-negative")
	}
}

func TestHTTPPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"received":%s}`, string(body))
	}))
	defer srv.Close()

	tl := findTool(t, "http_post")
	input, _ := json.Marshal(map[string]any{
		"url":  srv.URL,
		"body": `{"name":"test"}`,
	})
	result, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := parseOutput(t, result)
	if out.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", out.StatusCode)
	}
}

func TestHTTPPostCustomContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "text/plain" {
			t.Errorf("expected Content-Type text/plain, got %q", ct)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tl := findTool(t, "http_post")
	input, _ := json.Marshal(map[string]any{
		"url":          srv.URL,
		"body":         "plain text body",
		"content_type": "text/plain",
	})
	_, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPPut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	tl := findTool(t, "http_put")
	input, _ := json.Marshal(map[string]any{
		"url":  srv.URL,
		"body": `{"update":"data"}`,
	})
	result, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := parseOutput(t, result)
	if out.StatusCode != 204 {
		t.Errorf("expected status 204, got %d", out.StatusCode)
	}
}

func TestHTTPDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"deleted":true}`)
	}))
	defer srv.Close()

	tl := findTool(t, "http_delete")
	input, _ := json.Marshal(map[string]any{"url": srv.URL})
	result, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := parseOutput(t, result)
	if out.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", out.StatusCode)
	}
}

func TestHTTPHead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.Header().Set("X-Total-Count", "42")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tl := findTool(t, "http_head")
	input, _ := json.Marshal(map[string]any{"url": srv.URL})
	result, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := parseOutput(t, result)
	if out.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", out.StatusCode)
	}
	if out.Body != "" {
		t.Errorf("HEAD response should have no body, got %q", out.Body)
	}
	if out.Headers["X-Total-Count"] != "42" {
		t.Errorf("expected X-Total-Count=42, got %q", out.Headers["X-Total-Count"])
	}
}

func TestHTTPPatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"patched":true}`)
	}))
	defer srv.Close()

	tl := findTool(t, "http_patch")
	input, _ := json.Marshal(map[string]any{
		"url":  srv.URL,
		"body": `{"field":"value"}`,
	})
	result, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := parseOutput(t, result)
	if out.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", out.StatusCode)
	}
}

func TestCustomHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-token" {
			t.Errorf("expected Authorization header, got %q", auth)
		}
		accept := r.Header.Get("Accept")
		if accept != "text/xml" {
			t.Errorf("expected Accept=text/xml, got %q", accept)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tl := findTool(t, "http_get")
	input, _ := json.Marshal(map[string]any{
		"url": srv.URL,
		"headers": map[string]string{
			"Authorization": "Bearer my-token",
			"Accept":        "text/xml",
		},
	})
	_, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCustomTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tl := findTool(t, "http_get")
	timeoutSecs := 0.05 // 50ms — should time out
	input, _ := json.Marshal(map[string]any{
		"url":          srv.URL,
		"timeout_secs": timeoutSecs,
	})
	_, err := tl.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("expected request failure error, got: %v", err)
	}
}

func TestEmptyURL(t *testing.T) {
	tl := findTool(t, "http_get")
	input, _ := json.Marshal(map[string]any{"url": ""})
	_, err := tl.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestInvalidJSON(t *testing.T) {
	tl := findTool(t, "http_get")
	_, err := tl.Execute(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tl := findTool(t, "http_get")
	input, _ := json.Marshal(map[string]any{"url": srv.URL})
	_, err := tl.Execute(ctx, input)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestResponseBodyTruncation(t *testing.T) {
	// Create a response larger than 1MB.
	largeBody := strings.Repeat("x", 1<<20+100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, largeBody)
	}))
	defer srv.Close()

	tl := findTool(t, "http_get")
	input, _ := json.Marshal(map[string]any{"url": srv.URL})
	result, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := parseOutput(t, result)
	if len(out.Body) != 1<<20 {
		t.Errorf("expected body to be truncated to 1MB, got %d bytes", len(out.Body))
	}
	if out.Headers["X-Agent-Truncated"] != "true" {
		t.Error("expected X-Agent-Truncated header on truncated response")
	}
}

func TestRedirectLimit(t *testing.T) {
	redirectCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		// Always redirect — should be stopped by the client after 10 hops.
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
	}))
	defer srv.Close()

	tl := findTool(t, "http_get")
	input, _ := json.Marshal(map[string]any{"url": srv.URL})
	_, err := tl.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error from redirect limit")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("expected request failure error, got: %v", err)
	}
}

func TestNon200StatusIsNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	}))
	defer srv.Close()

	tl := findTool(t, "http_get")
	input, _ := json.Marshal(map[string]any{"url": srv.URL})
	result, err := tl.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("non-200 status should not return error, got: %v", err)
	}

	out := parseOutput(t, result)
	if out.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", out.StatusCode)
	}
}

func TestConnectionRefused(t *testing.T) {
	tl := findTool(t, "http_get")
	// Port 1 is unlikely to have anything listening.
	input, _ := json.Marshal(map[string]any{
		"url":          "http://127.0.0.1:1/never",
		"timeout_secs": 2.0,
	})
	_, err := tl.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestAllToolsHaveHandlers(t *testing.T) {
	p := packhttp.Pack()
	for _, tl := range p.Tools {
		t.Run(tl.Name(), func(t *testing.T) {
			// Try to execute with a bad URL — should fail with a meaningful error,
			// not with ErrNoHandler.
			input, _ := json.Marshal(map[string]any{"url": "http://127.0.0.1:1/test"})
			_, err := tl.Execute(context.Background(), input)
			if err != nil && strings.Contains(err.Error(), "tool has no handler") {
				t.Errorf("tool %q has no handler implemented", tl.Name())
			}
		})
	}
}
