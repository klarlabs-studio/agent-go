package config

import (
	"encoding/json"
)

// JSONSchema represents a JSON Schema document.
type JSONSchema struct {
	Schema               string                 `json:"$schema,omitempty"`
	ID                   string                 `json:"$id,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Type                 string                 `json:"type,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	AdditionalProperties *JSONSchema            `json:"additionalProperties,omitempty"`
	Enum                 []string               `json:"enum,omitempty"`
	Default              any                    `json:"default,omitempty"`
	Minimum              *float64               `json:"minimum,omitempty"`
	Maximum              *float64               `json:"maximum,omitempty"`
	MinLength            *int                   `json:"minLength,omitempty"`
	MaxLength            *int                   `json:"maxLength,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
	Format               string                 `json:"format,omitempty"`
	Ref                  string                 `json:"$ref,omitempty"`
	Definitions          map[string]*JSONSchema `json:"$defs,omitempty"`
	OneOf                []*JSONSchema          `json:"oneOf,omitempty"`
	AnyOf                []*JSONSchema          `json:"anyOf,omitempty"`
	AllOf                []*JSONSchema          `json:"allOf,omitempty"`
}

// GenerateSchema generates a JSON Schema for the AgentConfig.
func GenerateSchema() *JSONSchema {
	return &JSONSchema{
		Schema:      "https://json-schema.org/draft/2020-12/schema",
		ID:          "https://github.com/felixgeelhaar/agent-go/agent-config.schema.json",
		Title:       "Agent Configuration",
		Description: "Configuration schema for agent-go runtime",
		Type:        "object",
		Required:    []string{"name", "version"},
		Properties: map[string]*JSONSchema{
			"name": {
				Type:        "string",
				Description: "A human-readable name for this configuration",
			},
			"version": {
				Type:        "string",
				Description: "The configuration schema version",
				Default:     "1.0",
			},
			"description": {
				Type:        "string",
				Description: "Describes the agent's purpose",
			},
			"agent":        generateAgentSchema(),
			"tools":        generateToolsSchema(),
			"policy":       generatePolicySchema(),
			"resilience":   generateResilienceSchema(),
			"notification": generateNotificationSchema(),
			"variables": {
				Type:        "object",
				Description: "Initial variables for the agent",
				AdditionalProperties: &JSONSchema{
					Description: "Variable value (any type)",
				},
			},
		},
	}
}

func generateAgentSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Core agent behavior settings",
		Properties: map[string]*JSONSchema{
			"max_steps": {
				Type:        "integer",
				Description: "Maximum number of execution steps",
				Default:     100,
				Minimum:     floatPtr(0),
			},
			"default_goal": {
				Type:        "string",
				Description: "Default goal if none is provided",
			},
			"initial_state": {
				Type:        "string",
				Description: "Starting state (default: intake)",
				Enum:        []string{"intake", "explore", "decide", "act", "validate"},
				Default:     "intake",
			},
		},
	}
}

func generateToolsSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Tool-related configuration",
		Properties: map[string]*JSONSchema{
			"packs": {
				Type:        "array",
				Description: "List of tool packs to load",
				Items:       generateToolPackSchema(),
			},
			"inline": {
				Type:        "array",
				Description: "Inline tool definitions",
				Items:       generateInlineToolSchema(),
			},
			"eligibility": {
				Type:        "object",
				Description: "Maps states to allowed tools",
				AdditionalProperties: &JSONSchema{
					Type:        "array",
					Description: "List of tool names allowed in this state",
					Items: &JSONSchema{
						Type: "string",
					},
				},
			},
		},
	}
}

func generateToolPackSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Tool pack configuration",
		Required:    []string{"name"},
		Properties: map[string]*JSONSchema{
			"name": {
				Type:        "string",
				Description: "Pack name",
			},
			"version": {
				Type:        "string",
				Description: "Required version (optional)",
			},
			"config": {
				Type:                 "object",
				Description:          "Pack-specific configuration",
				AdditionalProperties: &JSONSchema{},
			},
			"enabled": {
				Type:        "array",
				Description: "Tools to enable (empty = all)",
				Items:       &JSONSchema{Type: "string"},
			},
			"disabled": {
				Type:        "array",
				Description: "Tools to disable",
				Items:       &JSONSchema{Type: "string"},
			},
		},
	}
}

func generateInlineToolSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Inline tool definition",
		Required:    []string{"name", "description", "handler"},
		Properties: map[string]*JSONSchema{
			"name": {
				Type:        "string",
				Description: "Tool identifier",
			},
			"description": {
				Type:        "string",
				Description: "Tool description",
			},
			"annotations": {
				Type:        "object",
				Description: "Tool behavior annotations",
				Properties: map[string]*JSONSchema{
					"read_only": {
						Type:        "boolean",
						Description: "Tool doesn't modify state",
						Default:     false,
					},
					"destructive": {
						Type:        "boolean",
						Description: "Tool performs irreversible operations",
						Default:     false,
					},
					"idempotent": {
						Type:        "boolean",
						Description: "Repeated calls produce same result",
						Default:     false,
					},
					"cacheable": {
						Type:        "boolean",
						Description: "Results can be cached",
						Default:     false,
					},
					"risk_level": {
						Type:        "string",
						Description: "Potential impact level",
						Enum:        []string{"none", "low", "medium", "high", "critical"},
						Default:     "none",
					},
				},
			},
			"input_schema": {
				Type:        "object",
				Description: "JSON schema for input validation",
			},
			"output_schema": {
				Type:        "object",
				Description: "JSON schema for output validation",
			},
			"handler": generateHandlerSchema(),
		},
	}
}

func generateHandlerSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Tool execution handler",
		Required:    []string{"type"},
		Properties: map[string]*JSONSchema{
			"type": {
				Type:        "string",
				Description: "Handler type",
				Enum:        []string{"http", "exec", "wasm"},
			},
			"url": {
				Type:        "string",
				Description: "Endpoint for HTTP handlers",
				Format:      "uri",
			},
			"method": {
				Type:        "string",
				Description: "HTTP method (default: POST)",
				Default:     "POST",
			},
			"headers": {
				Type:                 "object",
				Description:          "Additional HTTP headers",
				AdditionalProperties: &JSONSchema{Type: "string"},
			},
			"command": {
				Type:        "string",
				Description: "Command for exec handlers",
			},
			"args": {
				Type:        "array",
				Description: "Command arguments",
				Items:       &JSONSchema{Type: "string"},
			},
			"env": {
				Type:                 "object",
				Description:          "Environment variables for exec handlers",
				AdditionalProperties: &JSONSchema{Type: "string"},
			},
			"path": {
				Type:        "string",
				Description: "WASM module path",
			},
		},
	}
}

func generatePolicySchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Policy settings",
		Properties: map[string]*JSONSchema{
			"budgets": {
				Type:        "object",
				Description: "Budget limits",
				AdditionalProperties: &JSONSchema{
					Type:    "integer",
					Minimum: floatPtr(0),
				},
			},
			"approval": {
				Type:        "object",
				Description: "Approval behavior",
				Properties: map[string]*JSONSchema{
					"mode": {
						Type:        "string",
						Description: "Approval mode",
						Enum:        []string{"auto", "manual", "none"},
						Default:     "auto",
					},
					"require_for_destructive": {
						Type:        "boolean",
						Description: "Require approval for destructive tools",
						Default:     true,
					},
					"require_for_risk_level": {
						Type:        "string",
						Description: "Require approval above this risk level",
						Enum:        []string{"none", "low", "medium", "high", "critical"},
					},
				},
			},
			"transitions": {
				Type:        "array",
				Description: "Custom state transitions",
				Items: &JSONSchema{
					Type:     "object",
					Required: []string{"from", "to"},
					Properties: map[string]*JSONSchema{
						"from": {
							Type: "string",
							Enum: []string{"intake", "explore", "decide", "act", "validate", "done", "failed"},
						},
						"to": {
							Type: "string",
							Enum: []string{"intake", "explore", "decide", "act", "validate", "done", "failed"},
						},
						"guard": {
							Type:        "string",
							Description: "Optional guard condition",
						},
					},
				},
			},
			"rate_limit": generateRateLimitSchema(),
		},
	}
}

func generateRateLimitSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Rate limiting configuration",
		Properties: map[string]*JSONSchema{
			"enabled": {
				Type:        "boolean",
				Description: "Enable rate limiting",
				Default:     false,
			},
			"rate": {
				Type:        "integer",
				Description: "Tokens per second",
				Minimum:     floatPtr(1),
			},
			"burst": {
				Type:        "integer",
				Description: "Maximum burst size",
				Minimum:     floatPtr(1),
			},
			"per_tool": {
				Type:        "boolean",
				Description: "Enable per-tool rate limiting",
				Default:     false,
			},
			"tool_rates": {
				Type:        "object",
				Description: "Per-tool rate limits",
				AdditionalProperties: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"rate":  {Type: "integer", Minimum: floatPtr(1)},
						"burst": {Type: "integer", Minimum: floatPtr(1)},
					},
				},
			},
		},
	}
}

func generateResilienceSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Resilience settings",
		Properties: map[string]*JSONSchema{
			"timeout": {
				Type:        "string",
				Description: "Default tool timeout (e.g., '30s', '1m')",
				Format:      "duration",
				Default:     "30s",
			},
			"retry": {
				Type:        "object",
				Description: "Retry behavior",
				Properties: map[string]*JSONSchema{
					"enabled": {
						Type:    "boolean",
						Default: true,
					},
					"max_attempts": {
						Type:    "integer",
						Minimum: floatPtr(1),
						Default: 3,
					},
					"initial_delay": {
						Type:    "string",
						Format:  "duration",
						Default: "1s",
					},
					"max_delay": {
						Type:   "string",
						Format: "duration",
					},
					"multiplier": {
						Type:    "number",
						Minimum: floatPtr(1),
						Default: 2.0,
					},
				},
			},
			"circuit_breaker": {
				Type:        "object",
				Description: "Circuit breaker behavior",
				Properties: map[string]*JSONSchema{
					"enabled": {
						Type:    "boolean",
						Default: true,
					},
					"threshold": {
						Type:        "integer",
						Description: "Failures before opening",
						Minimum:     floatPtr(1),
						Default:     5,
					},
					"timeout": {
						Type:        "string",
						Description: "How long circuit stays open",
						Format:      "duration",
						Default:     "30s",
					},
				},
			},
			"bulkhead": {
				Type:        "object",
				Description: "Bulkhead behavior",
				Properties: map[string]*JSONSchema{
					"enabled": {
						Type:    "boolean",
						Default: true,
					},
					"max_concurrent": {
						Type:        "integer",
						Description: "Maximum concurrent executions",
						Minimum:     floatPtr(1),
						Default:     10,
					},
				},
			},
		},
	}
}

func generateNotificationSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Notification settings",
		Properties: map[string]*JSONSchema{
			"enabled": {
				Type:        "boolean",
				Description: "Enable notifications",
				Default:     false,
			},
			"endpoints": {
				Type:        "array",
				Description: "Webhook endpoints",
				Items: &JSONSchema{
					Type:     "object",
					Required: []string{"url"},
					Properties: map[string]*JSONSchema{
						"name": {
							Type:        "string",
							Description: "Human-readable name",
						},
						"url": {
							Type:        "string",
							Description: "Webhook URL",
							Format:      "uri",
						},
						"enabled": {
							Type:    "boolean",
							Default: true,
						},
						"secret": {
							Type:        "string",
							Description: "HMAC signing secret",
						},
						"headers": {
							Type:                 "object",
							Description:          "Additional HTTP headers",
							AdditionalProperties: &JSONSchema{Type: "string"},
						},
						"event_filter": {
							Type:        "array",
							Description: "Event types to send",
							Items:       &JSONSchema{Type: "string"},
						},
					},
				},
			},
			"batching": {
				Type:        "object",
				Description: "Event batching",
				Properties: map[string]*JSONSchema{
					"enabled": {
						Type:    "boolean",
						Default: true,
					},
					"max_size": {
						Type:    "integer",
						Minimum: floatPtr(1),
						Default: 100,
					},
					"max_wait": {
						Type:    "string",
						Format:  "duration",
						Default: "5s",
					},
				},
			},
			"event_filter": {
				Type:        "array",
				Description: "Global event type filter",
				Items:       &JSONSchema{Type: "string"},
			},
		},
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

// SchemaJSON returns the JSON Schema as a JSON string.
func SchemaJSON() (string, error) {
	schema := GenerateSchema()
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
