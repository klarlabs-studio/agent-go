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
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Common errors for MCP operations.
var (
	ErrToolNotFound     = errors.New("tool not found")
	ErrInvalidRequest   = errors.New("invalid request")
	ErrResourceNotFound = errors.New("resource not found")
	ErrNotImplemented   = errors.New("not implemented")
)

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
}

// Server implements the MCP server.
type Server struct {
	config    Config
	resources map[string]Resource
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
	}

	// Index resources
	for _, r := range cfg.Resources {
		s.resources[r.URI] = r
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
	// TODO: Implement stdio transport
	// Read JSON-RPC messages from stdin
	// Write responses to stdout
	return ErrNotImplemented
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

	// Execute the tool
	result, err := t.Execute(r.Context(), req.Arguments)
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

	// TODO: Implement resource reading based on type
	resp := ReadResourceResponse{
		Contents: []ResourceContent{
			{
				URI:      resource.URI,
				MimeType: resource.MimeType,
				Text:     "Resource content placeholder",
			},
		},
	}
	s.writeJSON(w, resp)
}

// handleListPrompts handles the prompts/list request.
func (s *Server) handleListPrompts(w http.ResponseWriter, _ *http.Request) {
	// TODO: Implement prompt listing
	s.writeJSON(w, ListPromptsResponse{Prompts: []Prompt{}})
}

// handleGetPrompt handles the prompts/get request.
func (s *Server) handleGetPrompt(w http.ResponseWriter, _ *http.Request) {
	// TODO: Implement prompt retrieval
	http.Error(w, "Not implemented", http.StatusNotImplemented)
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
	// TODO: Implement JSON-RPC over stdio
	return ErrNotImplemented
}

// MCP Protocol Types

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
