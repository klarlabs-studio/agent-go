package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeStdio_Initialize(t *testing.T) {
	srv := NewServer(Config{
		ServerName:    "test-server",
		ServerVersion: "1.0.0",
	})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	// Send initialize request
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	// Read response
	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan(), "expected response line")

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	var initResp InitializeResponse
	err = json.Unmarshal(resp.Result, &initResp)
	require.NoError(t, err)
	assert.Equal(t, "2024-11-05", initResp.ProtocolVersion)
	assert.Equal(t, "test-server", initResp.ServerInfo.Name)
	assert.Equal(t, "1.0.0", initResp.ServerInfo.Version)

	// Close pipes to cause scanner to finish
	stdinW.Close()
	stdoutW.Close()

	select {
	case err := <-errCh:
		if err != nil && err != io.ErrClosedPipe {
			t.Errorf("ServeStdio returned unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("ServeStdio did not exit after closing pipes")
	}
}

func TestServeStdio_ListTools(t *testing.T) {
	srv := NewServer(Config{
		ServerName: "test",
		Registry:   nil, // No tools
	})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan())

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	var listResp ListToolsResponse
	err = json.Unmarshal(resp.Result, &listResp)
	require.NoError(t, err)
	assert.Empty(t, listResp.Tools)

	cancel()
}

func TestServeStdio_ListResources(t *testing.T) {
	srv := NewServer(Config{
		ServerName: "test",
		Resources: []Resource{
			{URI: "test://example", Name: "Example", MimeType: "text/plain"},
		},
	})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan())

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	var listResp ListResourcesResponse
	err = json.Unmarshal(resp.Result, &listResp)
	require.NoError(t, err)
	require.Len(t, listResp.Resources, 1)
	assert.Equal(t, "test://example", listResp.Resources[0].URI)

	cancel()
}

func TestServeStdio_ReadResource_WithHandler(t *testing.T) {
	handlerCalled := false
	srv := NewServer(Config{
		ServerName: "test",
		Resources: []Resource{
			{URI: "test://example", Name: "Example", MimeType: "text/plain"},
		},
		ResourceHandlers: map[string]ResourceHandler{
			"test://example": func(ctx context.Context, uri string) (ResourceContent, error) {
				handlerCalled = true
				return ResourceContent{
					URI:      uri,
					MimeType: "text/plain",
					Text:     "Handler content",
				}, nil
			},
		},
	})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"test://example"}}` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan())

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	var readResp ReadResourceResponse
	err = json.Unmarshal(resp.Result, &readResp)
	require.NoError(t, err)
	require.Len(t, readResp.Contents, 1)
	assert.Equal(t, "test://example", readResp.Contents[0].URI)
	assert.Equal(t, "Handler content", readResp.Contents[0].Text)
	assert.True(t, handlerCalled)

	cancel()
}

func TestServeStdio_ListPrompts(t *testing.T) {
	srv := NewServer(Config{
		ServerName: "test",
		Prompts: []Prompt{
			{Name: "example", Description: "Example prompt"},
		},
	})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"prompts/list","params":{}}` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan())

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	var listResp ListPromptsResponse
	err = json.Unmarshal(resp.Result, &listResp)
	require.NoError(t, err)
	require.Len(t, listResp.Prompts, 1)
	assert.Equal(t, "example", listResp.Prompts[0].Name)

	cancel()
}

func TestServeStdio_GetPrompt(t *testing.T) {
	srv := NewServer(Config{
		ServerName: "test",
		Prompts: []Prompt{
			{
				Name:        "example",
				Description: "Example prompt",
				Arguments: []PromptArgument{
					{Name: "arg1", Description: "First arg", Required: true},
				},
			},
		},
	})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"example"}}` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan())

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	var prompt Prompt
	err = json.Unmarshal(resp.Result, &prompt)
	require.NoError(t, err)
	assert.Equal(t, "example", prompt.Name)
	assert.Equal(t, "Example prompt", prompt.Description)
	require.Len(t, prompt.Arguments, 1)
	assert.Equal(t, "arg1", prompt.Arguments[0].Name)

	cancel()
}

func TestServeStdio_GetPrompt_NotFound(t *testing.T) {
	srv := NewServer(Config{ServerName: "test"})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"nonexistent"}}` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan())

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "not found")

	cancel()
}

func TestServeStdio_InvalidJSON(t *testing.T) {
	srv := NewServer(Config{ServerName: "test"})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	// Send invalid JSON
	req := `{invalid json` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan())

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32700, resp.Error.Code)
	assert.Equal(t, "Parse error", resp.Error.Message)

	cancel()
}

func TestServeStdio_UnknownMethod(t *testing.T) {
	srv := NewServer(Config{ServerName: "test"})

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = srv.ServeStdio(ctx, stdinR, stdoutW)
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"unknown/method","params":{}}` + "\n"
	_, err := stdinW.Write([]byte(req))
	require.NoError(t, err)

	scanner := bufio.NewScanner(stdoutR)
	require.True(t, scanner.Scan())

	var resp jsonRPCResponse
	err = json.Unmarshal(scanner.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Equal(t, "Method not found", resp.Error.Message)

	cancel()
}

func TestServeStdio_EmptyLines(t *testing.T) {
	srv := NewServer(Config{ServerName: "test"})

	input := "\n\n" + `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	stdinR := strings.NewReader(input)

	// Use a synchronized buffer to avoid race
	var mu sync.Mutex
	var output strings.Builder

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Wrap the output with a synchronized writer
		syncWriter := &syncWriter{w: &output, mu: &mu}
		_ = srv.ServeStdio(ctx, stdinR, syncWriter)
	}()

	// Wait for completion
	<-done

	// Read output safely
	mu.Lock()
	result := output.String()
	mu.Unlock()

	// Should have only one response (empty lines ignored)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	assert.Len(t, lines, 1)

	var resp jsonRPCResponse
	err := json.Unmarshal([]byte(lines[0]), &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)
}

// syncWriter wraps a writer with mutex protection
type syncWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (sw *syncWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

func TestHandleMethod_Coverage(t *testing.T) {
	srv := NewServer(Config{
		ServerName: "test",
		Resources: []Resource{
			{URI: "test://res", Name: "Test"},
		},
		Prompts: []Prompt{
			{Name: "test-prompt"},
		},
	})

	tests := []struct {
		name        string
		method      string
		params      string
		wantErr     bool
		errCode     int
		errContains string
	}{
		{
			name:    "initialize",
			method:  "initialize",
			params:  "{}",
			wantErr: false,
		},
		{
			name:    "tools/list",
			method:  "tools/list",
			params:  "{}",
			wantErr: false,
		},
		{
			name:    "resources/list",
			method:  "resources/list",
			params:  "{}",
			wantErr: false,
		},
		{
			name:    "resources/read valid",
			method:  "resources/read",
			params:  `{"uri":"test://res"}`,
			wantErr: false,
		},
		{
			name:        "resources/read invalid params",
			method:      "resources/read",
			params:      `{invalid}`,
			wantErr:     true,
			errCode:     -32602,
			errContains: "Invalid params",
		},
		{
			name:        "resources/read not found",
			method:      "resources/read",
			params:      `{"uri":"test://notfound"}`,
			wantErr:     true,
			errCode:     -32602,
			errContains: "not found",
		},
		{
			name:    "prompts/list",
			method:  "prompts/list",
			params:  "{}",
			wantErr: false,
		},
		{
			name:    "prompts/get valid",
			method:  "prompts/get",
			params:  `{"name":"test-prompt"}`,
			wantErr: false,
		},
		{
			name:        "prompts/get invalid params",
			method:      "prompts/get",
			params:      `{invalid}`,
			wantErr:     true,
			errCode:     -32602,
			errContains: "Invalid params",
		},
		{
			name:        "prompts/get not found",
			method:      "prompts/get",
			params:      `{"name":"nonexistent"}`,
			wantErr:     true,
			errCode:     -32602,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, rpcErr := srv.handleMethod(context.Background(), tt.method, json.RawMessage(tt.params))

			if tt.wantErr {
				require.NotNil(t, rpcErr, "expected error")
				assert.Equal(t, tt.errCode, rpcErr.Code)
				assert.Contains(t, rpcErr.Message, tt.errContains)
			} else {
				require.Nil(t, rpcErr, "expected no error")
				require.NotNil(t, result, "expected result")
			}
		})
	}
}
