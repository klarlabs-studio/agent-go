package tool

import (
	"context"
	"encoding/json"
)

// Tool represents a registered capability the agent can invoke.
type Tool interface {
	// Name returns the stable string identifier for the tool.
	Name() string

	// Description returns a human-readable description of what the tool does.
	Description() string

	// InputSchema returns the JSON Schema for validating input.
	InputSchema() Schema

	// OutputSchema returns the JSON Schema for validating output.
	OutputSchema() Schema

	// Annotations returns the tool's behavioral annotations.
	Annotations() Annotations

	// Execute runs the tool with the given input.
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

// Handler is the function signature for tool execution.
type Handler func(ctx context.Context, input json.RawMessage) (Result, error)

// Definition is a concrete implementation of Tool.
type Definition struct {
	name         string
	description  string
	inputSchema  Schema
	outputSchema Schema
	annotations  Annotations
	handler      Handler
}

// Name returns the tool name.
func (d *Definition) Name() string {
	return d.name
}

// Description returns the tool description.
func (d *Definition) Description() string {
	return d.description
}

// InputSchema returns the input schema.
func (d *Definition) InputSchema() Schema {
	return d.inputSchema
}

// OutputSchema returns the output schema.
func (d *Definition) OutputSchema() Schema {
	return d.outputSchema
}

// Annotations returns the tool annotations.
func (d *Definition) Annotations() Annotations {
	return d.annotations
}

// Execute runs the tool handler.
func (d *Definition) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if d.handler == nil {
		return Result{}, ErrNoHandler
	}
	return d.handler(ctx, input)
}

// Builder provides a fluent API for constructing tools.
type Builder struct {
	def *Definition
	err error
}

// NewBuilder creates a new tool builder with the given name.
func NewBuilder(name string) *Builder {
	return &Builder{
		def: &Definition{
			name:        name,
			annotations: DefaultAnnotations(),
		},
	}
}

// WithDescription sets the tool description.
func (b *Builder) WithDescription(desc string) *Builder {
	if b.err != nil {
		return b
	}
	b.def.description = desc
	return b
}

// WithInputSchema sets the input schema.
func (b *Builder) WithInputSchema(schema Schema) *Builder {
	if b.err != nil {
		return b
	}
	b.def.inputSchema = schema
	return b
}

// WithOutputSchema sets the output schema.
func (b *Builder) WithOutputSchema(schema Schema) *Builder {
	if b.err != nil {
		return b
	}
	b.def.outputSchema = schema
	return b
}

// WithAnnotations sets the tool annotations.
func (b *Builder) WithAnnotations(annotations Annotations) *Builder {
	if b.err != nil {
		return b
	}
	b.def.annotations = annotations
	return b
}

// ReadOnly marks the tool as read-only.
func (b *Builder) ReadOnly() *Builder {
	if b.err != nil {
		return b
	}
	b.def.annotations.ReadOnly = true
	b.def.annotations.RiskLevel = RiskNone
	return b
}

// Destructive marks the tool as destructive.
func (b *Builder) Destructive() *Builder {
	if b.err != nil {
		return b
	}
	b.def.annotations.Destructive = true
	b.def.annotations.RequiresApproval = true
	if b.def.annotations.RiskLevel < RiskHigh {
		b.def.annotations.RiskLevel = RiskHigh
	}
	return b
}

// Idempotent marks the tool as idempotent.
func (b *Builder) Idempotent() *Builder {
	if b.err != nil {
		return b
	}
	b.def.annotations.Idempotent = true
	return b
}

// Cacheable marks the tool as cacheable.
func (b *Builder) Cacheable() *Builder {
	if b.err != nil {
		return b
	}
	b.def.annotations.Cacheable = true
	return b
}

// WithRiskLevel sets the risk level.
func (b *Builder) WithRiskLevel(level RiskLevel) *Builder {
	if b.err != nil {
		return b
	}
	b.def.annotations.RiskLevel = level
	return b
}

// RequiresApproval marks the tool as requiring approval.
func (b *Builder) RequiresApproval() *Builder {
	if b.err != nil {
		return b
	}
	b.def.annotations.RequiresApproval = true
	return b
}

// WithHandler sets the tool handler function.
func (b *Builder) WithHandler(handler Handler) *Builder {
	if b.err != nil {
		return b
	}
	b.def.handler = handler
	return b
}

// WithTags adds tags to the tool.
func (b *Builder) WithTags(tags ...string) *Builder {
	if b.err != nil {
		return b
	}
	b.def.annotations.Tags = append(b.def.annotations.Tags, tags...)
	return b
}

// Build constructs the tool definition.
func (b *Builder) Build() (Tool, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.def.name == "" {
		return nil, ErrEmptyName
	}
	return b.def, nil
}

// MustBuild constructs the tool definition or panics on error.
// This is intentional — use MustBuild only for static tool definitions
// where a build failure indicates a programming error (e.g., missing name).
// For dynamic tool creation where errors are expected, use Build instead.
func (b *Builder) MustBuild() Tool {
	tool, err := b.Build()
	if err != nil {
		panic(err)
	}
	return tool
}
