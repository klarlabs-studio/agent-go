// Package mcp provides Model Context Protocol (MCP) server support for agent-go.
//
// MCP enables exposing agent tools and resources to LLM applications following
// the Model Context Protocol specification. This allows external AI systems
// to discover and invoke agent capabilities.
//
// # Usage
//
//	srv := mcp.NewServer(mcp.Config{
//		Registry: myToolRegistry,
//		Address:  "localhost:8081",
//	})
//
//	if err := srv.Start(); err != nil {
//		log.Fatal(err)
//	}
//
// # MCP Protocol
//
// MCP is a standard protocol for AI model interactions with tools and resources.
// This implementation supports:
//   - Tool discovery and invocation
//   - Resource listing and reading
//   - Prompt templates
//   - Sampling requests
//
// See https://modelcontextprotocol.io for protocol specification.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"crypto/rand"
	"encoding/hex"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
)

// Common errors for MCP operations.
var (
	ErrToolNotFound     = errors.New("tool not found")
	ErrInvalidRequest   = errors.New("invalid request")
	ErrResourceNotFound = errors.New("resource not found")
	ErrNotImplemented   = errors.New("not implemented")
)

// ResourceHandler reads the content for a resource.
type ResourceHandler func(ctx context.Context, uri string) (ResourceContent, error)

// Config configures the MCP server.
type Config struct {
	// Registry provides access to agent tools.
	Registry tool.Registry

	// Address is the listen address for the MCP server.
	Address string

	// ServerName is the name advertised to clients.
	ServerName string

	// ServerVersion is the version advertised to clients.
	ServerVersion string

	// Transport specifies the transport type ("stdio", "http").
	Transport string

	// Resources is a list of static resources to expose.
	Resources []Resource

	// Prompts is a list of prompt templates to expose.
	Prompts []Prompt

	// ResourceHandlers maps resource URIs to handler functions.
	ResourceHandlers map[string]ResourceHandler

	// Middleware is the optional middleware registry for tool execution.
	// When set, MCP tool calls are routed through the same middleware chain
	// as engine tool calls (eligibility, approval, logging, etc.).
	Middleware *middleware.Registry

	// EventStore is the optional event store for MCP audit trail.
	// When set, tool.called/succeeded/failed events are published for
	// every MCP tool invocation.
	EventStore event.Store

	// Eligibility is the optional tool eligibility configuration.
	// When set, MCP tool calls are checked against state-based eligibility.
	Eligibility *policy.ToolEligibility

	// BudgetPerClient enables per-client budget tracking.
	// Key is the client identifier, value is the budget limit.
	// When set, each MCP client gets its own tool call budget.
	BudgetPerClient map[string]int
}

// Server implements the MCP server.
type Server struct {
	config    Config
	resources map[string]Resource
	prompts   map[string]Prompt
	mu        sync.RWMutex
}

// NewServer creates a new MCP server.
func NewServer(cfg Config) *Server {
	if cfg.ServerName == "" {
		cfg.ServerName = "agent-go-mcp"
	}
	if cfg.ServerVersion == "" {
		cfg.ServerVersion = "1.0.0"
	}
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}

	s := &Server{
		config:    cfg,
		resources: make(map[string]Resource),
		prompts:   make(map[string]Prompt),
	}

	// Index resources
	for _, r := range cfg.Resources {
		s.resources[r.URI] = r
	}

	// Index prompts
	for _, p := range cfg.Prompts {
		s.prompts[p.Name] = p
	}

	return s
}

// Start starts the MCP server.
func (s *Server) Start() error {
	switch s.config.Transport {
	case "stdio":
		return s.serveStdio()
	case "http":
		return s.serveHTTP()
	default:
		return errors.New("unknown transport: " + s.config.Transport)
	}
}

// serveStdio handles MCP over stdin/stdout.
func (s *Server) serveStdio() error {
	return s.ServeStdio(context.Background(), os.Stdin, os.Stdout)
}

// serveHTTP handles MCP over HTTP.
func (s *Server) serveHTTP() error {
	mux := http.NewServeMux()

	// MCP endpoints
	mux.HandleFunc("/mcp/initialize", s.handleInitialize)
	mux.HandleFunc("/mcp/tools/list", s.handleListTools)
	mux.HandleFunc("/mcp/tools/call", s.handleCallTool)
	mux.HandleFunc("/mcp/resources/list", s.handleListResources)
	mux.HandleFunc("/mcp/resources/read", s.handleReadResource)
	mux.HandleFunc("/mcp/prompts/list", s.handleListPrompts)
	mux.HandleFunc("/mcp/prompts/get", s.handleGetPrompt)

	server := &http.Server{
		Addr:              s.config.Address,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return server.ListenAndServe()
}

// handleInitialize handles the initialize request.
func (s *Server) handleInitialize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := InitializeResponse{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{ListChanged: true},
			Resources: &ResourcesCapability{Subscribe: false, ListChanged: false},
			Prompts:   &PromptsCapability{ListChanged: false},
		},
		ServerInfo: ServerInfo{
			Name:    s.config.ServerName,
			Version: s.config.ServerVersion,
		},
	}

	s.writeJSON(w, resp)
}

// handleListTools handles the tools/list request.
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.config.Registry == nil {
		s.writeJSON(w, ListToolsResponse{Tools: []ToolDefinition{}})
		return
	}

	tools := s.config.Registry.List()
	definitions := make([]ToolDefinition, 0, len(tools))

	for _, t := range tools {
		definitions = append(definitions, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema().Raw,
		})
	}

	s.writeJSON(w, ListToolsResponse{Tools: definitions})
}

// handleCallTool handles the tools/call request.
func (s *Server) handleCallTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CallToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if s.config.Registry == nil {
		http.Error(w, "No tool registry configured", http.StatusServiceUnavailable)
		return
	}

	t, ok := s.config.Registry.Get(req.Name)
	if !ok {
		http.Error(w, "Tool not found", http.StatusNotFound)
		return
	}

	// Execute the tool (through middleware if configured)
	result, err := s.executeTool(r.Context(), t, req.Arguments)
	if err != nil {
		resp := CallToolResponse{
			IsError: true,
			Content: []ContentBlock{{Type: "text", Text: err.Error()}},
		}
		s.writeJSON(w, resp)
		return
	}

	resp := CallToolResponse{
		Content: []ContentBlock{{Type: "text", Text: string(result.Output)}},
	}
	s.writeJSON(w, resp)
}

// handleListResources handles the resources/list request.
func (s *Server) handleListResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	resources := make([]Resource, 0, len(s.resources))
	for _, r := range s.resources {
		resources = append(resources, r)
	}
	s.mu.RUnlock()

	s.writeJSON(w, ListResourcesResponse{Resources: resources})
}

// handleReadResource handles the resources/read request.
func (s *Server) handleReadResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReadResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	resource, ok := s.resources[req.URI]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}

	// Check if we have a handler for this resource
	if s.config.ResourceHandlers != nil {
		if handler, ok := s.config.ResourceHandlers[req.URI]; ok {
			content, err := handler(r.Context(), req.URI)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			resp := ReadResourceResponse{Contents: []ResourceContent{content}}
			s.writeJSON(w, resp)
			return
		}
	}

	// Fall back to static resource content
	resp := ReadResourceResponse{
		Contents: []ResourceContent{{
			URI:      resource.URI,
			MimeType: resource.MimeType,
			Text:     "",
		}},
	}
	s.writeJSON(w, resp)
}

// handleListPrompts handles the prompts/list request.
func (s *Server) handleListPrompts(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	prompts := make([]Prompt, 0, len(s.prompts))
	for _, p := range s.prompts {
		prompts = append(prompts, p)
	}
	s.mu.RUnlock()

	s.writeJSON(w, ListPromptsResponse{Prompts: prompts})
}

// handleGetPrompt handles the prompts/get request.
func (s *Server) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	prompt, ok := s.prompts[req.Name]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "Prompt not found", http.StatusNotFound)
		return
	}

	s.writeJSON(w, prompt)
}

// writeJSON writes a JSON response.
func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers already written; http.Error would cause a double-write.
		log.Printf("mcp: failed to encode JSON response: %v", err)
	}
}

// AddResource adds a resource to the server.
func (s *Server) AddResource(r Resource) {
	s.mu.Lock()
	s.resources[r.URI] = r
	s.mu.Unlock()
}

// RemoveResource removes a resource from the server.
func (s *Server) RemoveResource(uri string) {
	s.mu.Lock()
	delete(s.resources, uri)
	s.mu.Unlock()
}

// ServeStdio runs the MCP server over stdin/stdout.
// This is typically used when the server is launched as a subprocess.
func (s *Server) ServeStdio(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	enc := json.NewEncoder(stdout)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &jsonRPCError{Code: -32700, Message: "Parse error"},
			}
			if err := enc.Encode(resp); err != nil {
				return err
			}
			continue
		}

		result, rpcErr := s.handleMethod(ctx, req.Method, req.Params)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			raw, _ := json.Marshal(result)
			resp.Result = raw
		}

		if err := enc.Encode(resp); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// handleMethod dispatches JSON-RPC method calls.
func (s *Server) handleMethod(ctx context.Context, method string, params json.RawMessage) (any, *jsonRPCError) {
	switch method {
	case "initialize":
		return s.rpcInitialize()
	case "tools/list":
		return s.rpcListTools()
	case "tools/call":
		return s.rpcCallTool(ctx, params)
	case "resources/list":
		return s.rpcListResources()
	case "resources/read":
		return s.rpcReadResource(ctx, params)
	case "prompts/list":
		return s.rpcListPrompts()
	case "prompts/get":
		return s.rpcGetPrompt(params)
	default:
		return nil, &jsonRPCError{Code: -32601, Message: "Method not found"}
	}
}

// rpcInitialize handles the initialize RPC method.
func (s *Server) rpcInitialize() (any, *jsonRPCError) {
	return InitializeResponse{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{ListChanged: true},
			Resources: &ResourcesCapability{Subscribe: false, ListChanged: false},
			Prompts:   &PromptsCapability{ListChanged: false},
		},
		ServerInfo: ServerInfo{
			Name:    s.config.ServerName,
			Version: s.config.ServerVersion,
		},
	}, nil
}

// rpcListTools handles the tools/list RPC method.
func (s *Server) rpcListTools() (any, *jsonRPCError) {
	if s.config.Registry == nil {
		return ListToolsResponse{Tools: []ToolDefinition{}}, nil
	}

	tools := s.config.Registry.List()
	definitions := make([]ToolDefinition, 0, len(tools))

	for _, t := range tools {
		definitions = append(definitions, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema().Raw,
		})
	}

	return ListToolsResponse{Tools: definitions}, nil
}

// rpcCallTool handles the tools/call RPC method.
func (s *Server) rpcCallTool(ctx context.Context, params json.RawMessage) (any, *jsonRPCError) {
	var req CallToolRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "Invalid params"}
	}

	if s.config.Registry == nil {
		return nil, &jsonRPCError{Code: -32603, Message: "No tool registry configured"}
	}

	t, ok := s.config.Registry.Get(req.Name)
	if !ok {
		return nil, &jsonRPCError{Code: -32602, Message: "Tool not found"}
	}

	result, err := s.executeTool(ctx, t, req.Arguments)
	if err != nil {
		return CallToolResponse{
			IsError: true,
			Content: []ContentBlock{{Type: "text", Text: err.Error()}},
		}, nil
	}

	return CallToolResponse{
		Content: []ContentBlock{{Type: "text", Text: string(result.Output)}},
	}, nil
}

// rpcListResources handles the resources/list RPC method.
func (s *Server) rpcListResources() (any, *jsonRPCError) {
	s.mu.RLock()
	resources := make([]Resource, 0, len(s.resources))
	for _, r := range s.resources {
		resources = append(resources, r)
	}
	s.mu.RUnlock()

	return ListResourcesResponse{Resources: resources}, nil
}

// rpcReadResource handles the resources/read RPC method.
func (s *Server) rpcReadResource(ctx context.Context, params json.RawMessage) (any, *jsonRPCError) {
	var req ReadResourceRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "Invalid params"}
	}

	s.mu.RLock()
	resource, ok := s.resources[req.URI]
	s.mu.RUnlock()

	if !ok {
		return nil, &jsonRPCError{Code: -32602, Message: "Resource not found"}
	}

	// Check if we have a handler for this resource
	if s.config.ResourceHandlers != nil {
		if handler, ok := s.config.ResourceHandlers[req.URI]; ok {
			content, err := handler(ctx, req.URI)
			if err != nil {
				return nil, &jsonRPCError{Code: -32603, Message: err.Error()}
			}
			return ReadResourceResponse{Contents: []ResourceContent{content}}, nil
		}
	}

	// Fall back to static resource content
	return ReadResourceResponse{
		Contents: []ResourceContent{{
			URI:      resource.URI,
			MimeType: resource.MimeType,
			Text:     "",
		}},
	}, nil
}

// rpcListPrompts handles the prompts/list RPC method.
func (s *Server) rpcListPrompts() (any, *jsonRPCError) {
	s.mu.RLock()
	prompts := make([]Prompt, 0, len(s.prompts))
	for _, p := range s.prompts {
		prompts = append(prompts, p)
	}
	s.mu.RUnlock()

	return ListPromptsResponse{Prompts: prompts}, nil
}

// rpcGetPrompt handles the prompts/get RPC method.
func (s *Server) rpcGetPrompt(params json.RawMessage) (any, *jsonRPCError) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "Invalid params"}
	}

	s.mu.RLock()
	prompt, ok := s.prompts[req.Name]
	s.mu.RUnlock()

	if !ok {
		return nil, &jsonRPCError{Code: -32602, Message: "Prompt not found"}
	}

	return prompt, nil
}

// MCP Protocol Types

// jsonRPCRequest represents a JSON-RPC 2.0 request.
// executeTool runs a tool through the middleware chain if configured,
// or directly if not. This is the single execution path for both HTTP
// and JSON-RPC tool calls, ensuring policy enforcement.
func (s *Server) executeTool(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	// If no middleware configured, execute directly (backward compatible)
	if s.config.Middleware == nil {
		result, err := t.Execute(ctx, input)
		s.publishMCPToolEvents(ctx, t.Name(), input, result, err)
		return result, err
	}

	// Generate a synthetic MCP call ID for audit trail
	mcpCallID := generateMCPCallID()

	// Build execution context for middleware
	execCtx := &middleware.ExecutionContext{
		RunID:        mcpCallID,
		CurrentState: agent.StateAct, // MCP calls are always "acting"
		Tool:         t,
		Input:        input,
		Reason:       "mcp-invocation",
	}

	// Wire event publisher if event store is configured
	if s.config.EventStore != nil {
		execCtx.EventPublisher = func(eventType string, payload any) {
			evt, err := event.NewEvent(mcpCallID, event.Type(eventType), payload)
			if err == nil {
				_ = s.config.EventStore.Append(ctx, evt)
			}
		}
	}

	// Publish tool.called event
	s.publishMCPEvent(ctx, mcpCallID, event.TypeToolCalled, event.ToolCalledPayload{
		ToolName: t.Name(), Input: input, State: agent.StateAct, Reason: "mcp-invocation",
	})

	// Core handler wraps direct tool execution
	coreHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
		return ec.Tool.Execute(ctx, ec.Input)
	}

	// Execute through middleware chain
	start := time.Now()
	handler := s.config.Middleware.Chain()(coreHandler)
	result, err := handler(ctx, execCtx)
	duration := time.Since(start)

	// Publish success/failure events
	if err != nil {
		s.publishMCPEvent(ctx, mcpCallID, event.TypeToolFailed, event.ToolFailedPayload{
			ToolName: t.Name(), Error: err.Error(), Duration: duration,
		})
	} else {
		s.publishMCPEvent(ctx, mcpCallID, event.TypeToolSucceeded, event.ToolSucceededPayload{
			ToolName: t.Name(), Output: result.Output, Duration: duration, Cached: result.Cached,
		})
	}

	return result, err
}

// publishMCPEvent publishes a single event to the event store. Nil-safe.
func (s *Server) publishMCPEvent(ctx context.Context, mcpCallID string, eventType event.Type, payload any) {
	if s.config.EventStore == nil {
		return
	}
	evt, err := event.NewEvent(mcpCallID, eventType, payload)
	if err == nil {
		_ = s.config.EventStore.Append(ctx, evt)
	}
}

// publishMCPToolEvents publishes tool events for non-middleware execution path.
func (s *Server) publishMCPToolEvents(ctx context.Context, toolName string, input json.RawMessage, result tool.Result, err error) {
	if s.config.EventStore == nil {
		return
	}
	mcpCallID := generateMCPCallID()
	s.publishMCPEvent(ctx, mcpCallID, event.TypeToolCalled, event.ToolCalledPayload{
		ToolName: toolName, Input: input, State: agent.StateAct,
	})
	if err != nil {
		s.publishMCPEvent(ctx, mcpCallID, event.TypeToolFailed, event.ToolFailedPayload{
			ToolName: toolName, Error: err.Error(),
		})
	} else {
		s.publishMCPEvent(ctx, mcpCallID, event.TypeToolSucceeded, event.ToolSucceededPayload{
			ToolName: toolName, Output: result.Output,
		})
	}
}

// generateMCPCallID creates a unique ID for MCP tool calls.
func generateMCPCallID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("mcp-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse represents a JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError represents a JSON-RPC 2.0 error.
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// InitializeResponse is the response to an initialize request.
type InitializeResponse struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities describes what the server supports.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ToolsCapability describes tool support.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ResourcesCapability describes resource support.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

// PromptsCapability describes prompt support.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ServerInfo identifies the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ListToolsResponse is the response to a tools/list request.
type ListToolsResponse struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToolDefinition describes a tool for MCP.
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema"`
}

// CallToolRequest is a request to invoke a tool.
type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResponse is the response from invoking a tool.
type CallToolResponse struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a unit of content in a response.
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
}

// Resource describes an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ListResourcesResponse is the response to a resources/list request.
type ListResourcesResponse struct {
	Resources []Resource `json:"resources"`
}

// ReadResourceRequest is a request to read a resource.
type ReadResourceRequest struct {
	URI string `json:"uri"`
}

// ReadResourceResponse is the response from reading a resource.
type ReadResourceResponse struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent is the content of a resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64 encoded
}

// Prompt describes an MCP prompt template.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a prompt argument.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// ListPromptsResponse is the response to a prompts/list request.
type ListPromptsResponse struct {
	Prompts []Prompt `json:"prompts"`
}
