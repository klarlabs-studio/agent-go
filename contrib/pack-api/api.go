// Package api provides API consumption tools for agent-go.
//
// The pack uses an interface-based approach, allowing any HTTP client or
// API gateway to be plugged in. It supports OpenAPI, GraphQL, and SOAP.
package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// APIClient provides API consumption capabilities.
type APIClient interface {
	// Call makes an HTTP API call based on an OpenAPI operation.
	Call(ctx context.Context, opts CallOptions) (*APIResponse, error)

	// GraphQL executes a GraphQL query or mutation.
	GraphQL(ctx context.Context, endpoint string, query string, variables map[string]any, headers map[string]string) (*APIResponse, error)

	// ParseSpec parses an OpenAPI specification and returns available operations.
	ParseSpec(ctx context.Context, specURL string) (*APISpec, error)
}

// SOAPClient provides SOAP API capabilities.
type SOAPClient interface {
	// SOAPRequest makes a SOAP request.
	SOAPRequest(ctx context.Context, endpoint, action string, body string, headers map[string]string) (*APIResponse, error)
}

// CallOptions configures an API call.
type CallOptions struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	Body        any               `json:"body,omitempty"`
	Auth        *AuthConfig       `json:"auth,omitempty"`
	Timeout     int               `json:"timeout_seconds,omitempty"`
}

// AuthConfig configures request authentication.
type AuthConfig struct {
	Type   string `json:"type"` // "bearer", "basic", "api_key"
	Token  string `json:"token,omitempty"`
	User   string `json:"user,omitempty"`
	Pass   string `json:"pass,omitempty"`
	Key    string `json:"key,omitempty"`
	Header string `json:"header,omitempty"`
}

// APIResponse contains the API response.
type APIResponse struct {
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      any               `json:"body"`
	BodyRaw   string            `json:"body_raw,omitempty"`
	LatencyMS int64             `json:"latency_ms"`
}

// APISpec represents a parsed API specification.
type APISpec struct {
	Title       string      `json:"title"`
	Version     string      `json:"version"`
	Description string      `json:"description,omitempty"`
	BaseURL     string      `json:"base_url,omitempty"`
	Operations  []Operation `json:"operations"`
}

// Operation represents a single API operation.
type Operation struct {
	ID          string      `json:"id"`
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Parameters  []Parameter `json:"parameters,omitempty"`
	RequestBody *SchemaRef  `json:"request_body,omitempty"`
}

// Parameter represents an API parameter.
type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"` // "query", "path", "header"
	Required bool   `json:"required"`
	Type     string `json:"type,omitempty"`
}

// SchemaRef represents a JSON schema reference.
type SchemaRef struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
}

// Config holds API pack configuration.
type Config struct {
	// Client is the API client (required).
	Client APIClient

	// SOAP is an optional SOAP client.
	SOAP SOAPClient

	// DefaultHeaders are headers added to all requests.
	DefaultHeaders map[string]string

	// DefaultAuth is the default authentication config.
	DefaultAuth *AuthConfig
}

// Pack returns the API consumption tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &apiPack{cfg: cfg}

	tools := []tool.Tool{
		p.callTool(),
		p.graphqlTool(),
		p.parseSpecTool(),
	}

	if cfg.SOAP != nil {
		tools = append(tools, p.soapTool())
	}

	return pack.NewBuilder("api").
		WithDescription("API consumption tools: REST calls, GraphQL queries, OpenAPI spec parsing, SOAP requests").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type apiPack struct {
	cfg Config
}

func (p *apiPack) callTool() tool.Tool {
	return tool.NewBuilder("api_call").
		WithDescription("Make an HTTP API call").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Method      string            `json:"method"`
				URL         string            `json:"url"`
				Headers     map[string]string `json:"headers,omitempty"`
				QueryParams map[string]string `json:"query_params,omitempty"`
				Body        any               `json:"body,omitempty"`
				Auth        *AuthConfig       `json:"auth,omitempty"`
				Timeout     int               `json:"timeout_seconds,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}
			if in.Method == "" {
				in.Method = "GET"
			}

			headers := make(map[string]string)
			for k, v := range p.cfg.DefaultHeaders {
				headers[k] = v
			}
			for k, v := range in.Headers {
				headers[k] = v
			}

			auth := in.Auth
			if auth == nil {
				auth = p.cfg.DefaultAuth
			}

			resp, err := p.cfg.Client.Call(ctx, CallOptions{
				Method:      in.Method,
				URL:         in.URL,
				Headers:     headers,
				QueryParams: in.QueryParams,
				Body:        in.Body,
				Auth:        auth,
				Timeout:     in.Timeout,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("api call failed: %w", err)
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *apiPack) graphqlTool() tool.Tool {
	return tool.NewBuilder("api_graphql").
		WithDescription("Execute a GraphQL query or mutation").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Endpoint  string            `json:"endpoint"`
				Query     string            `json:"query"`
				Variables map[string]any    `json:"variables,omitempty"`
				Headers   map[string]string `json:"headers,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Endpoint == "" {
				return tool.Result{}, fmt.Errorf("endpoint is required")
			}
			if in.Query == "" {
				return tool.Result{}, fmt.Errorf("query is required")
			}

			headers := make(map[string]string)
			for k, v := range p.cfg.DefaultHeaders {
				headers[k] = v
			}
			for k, v := range in.Headers {
				headers[k] = v
			}

			resp, err := p.cfg.Client.GraphQL(ctx, in.Endpoint, in.Query, in.Variables, headers)
			if err != nil {
				return tool.Result{}, fmt.Errorf("graphql call failed: %w", err)
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *apiPack) parseSpecTool() tool.Tool {
	return tool.NewBuilder("api_parse_spec").
		WithDescription("Parse an OpenAPI specification and list available operations").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				SpecURL string `json:"spec_url"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.SpecURL == "" {
				return tool.Result{}, fmt.Errorf("spec_url is required")
			}

			spec, err := p.cfg.Client.ParseSpec(ctx, in.SpecURL)
			if err != nil {
				return tool.Result{}, fmt.Errorf("parse spec failed: %w", err)
			}

			output, _ := json.Marshal(spec)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *apiPack) soapTool() tool.Tool {
	return tool.NewBuilder("api_soap").
		WithDescription("Make a SOAP request").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Endpoint string            `json:"endpoint"`
				Action   string            `json:"action"`
				Body     string            `json:"body"`
				Headers  map[string]string `json:"headers,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Endpoint == "" {
				return tool.Result{}, fmt.Errorf("endpoint is required")
			}
			if in.Action == "" {
				return tool.Result{}, fmt.Errorf("action is required")
			}
			if in.Body == "" {
				return tool.Result{}, fmt.Errorf("body is required")
			}

			resp, err := p.cfg.SOAP.SOAPRequest(ctx, in.Endpoint, in.Action, in.Body, in.Headers)
			if err != nil {
				return tool.Result{}, fmt.Errorf("soap request failed: %w", err)
			}

			output, _ := json.Marshal(resp)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
