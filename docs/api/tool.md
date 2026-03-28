# Package `tool`

**Import path:** `github.com/felixgeelhaar/agent-go/domain/tool`

## Overview

package tool // import "github.com/felixgeelhaar/agent-go/domain/tool"

Package tool provides the domain model for agent tools.

## Full API Reference

```
package tool // import "github.com/felixgeelhaar/agent-go/domain/tool"

Package tool provides the domain model for agent tools.

VARIABLES

var (
	// ErrEmptyName indicates a tool was created with an empty name.
	ErrEmptyName = errors.New("tool name cannot be empty")

	// ErrNoHandler indicates a tool was created without a handler.
	ErrNoHandler = errors.New("tool has no handler")

	// ErrToolNotFound indicates the requested tool was not found.
	ErrToolNotFound = errors.New("tool not found")

	// ErrToolExists indicates a tool with the same name already exists.
	ErrToolExists = errors.New("tool already exists")

	// ErrToolNotAllowed indicates the tool is not allowed in the current state.
	ErrToolNotAllowed = errors.New("tool not allowed in current state")

	// ErrInvalidInput indicates the input failed schema validation.
	ErrInvalidInput = errors.New("invalid tool input")

	// ErrInvalidOutput indicates the output failed schema validation.
	ErrInvalidOutput = errors.New("invalid tool output")

	// ErrApprovalRequired indicates the tool requires approval to execute.
	ErrApprovalRequired = errors.New("approval required for tool execution")

	// ErrApprovalDenied indicates approval was denied for tool execution.
	ErrApprovalDenied = errors.New("approval denied for tool execution")

	// ErrExecutionTimeout indicates the tool execution timed out.
	ErrExecutionTimeout = errors.New("tool execution timed out")
)
    Domain errors for the tool system.


TYPES

type Annotations struct {
	// ReadOnly indicates the tool has no side effects.
	ReadOnly bool `json:"read_only"`

	// Destructive indicates the tool may cause irreversible changes.
	Destructive bool `json:"destructive"`

	// Idempotent indicates multiple calls with same input yield same result.
	Idempotent bool `json:"idempotent"`

	// Cacheable indicates results can be cached.
	Cacheable bool `json:"cacheable"`

	// RiskLevel indicates the potential impact of execution.
	RiskLevel RiskLevel `json:"risk_level"`

	// RequiresApproval indicates human approval is required.
	RequiresApproval bool `json:"requires_approval"`

	// Timeout is the maximum execution time in seconds (0 = default).
	Timeout int `json:"timeout,omitempty"`

	// Sandboxed indicates the tool should execute in an isolated sandbox.
	Sandboxed bool `json:"sandboxed"`

	// Tags are arbitrary labels for categorization.
	Tags []string `json:"tags,omitempty"`
}
    Annotations describe tool behavior for policy enforcement, caching,
    and planning.

func DefaultAnnotations() Annotations
    DefaultAnnotations returns annotations with safe defaults.

func DestructiveAnnotations() Annotations
    DestructiveAnnotations returns annotations for a destructive tool.

func ReadOnlyAnnotations() Annotations
    ReadOnlyAnnotations returns annotations for a read-only tool.

func (a Annotations) CanCache() bool
    CanCache returns true if the tool result can be cached.

func (a Annotations) CanRetry() bool
    CanRetry returns true if the tool can be safely retried on failure.

func (a Annotations) ShouldRequireApproval() bool
    ShouldRequireApproval returns true if the tool should require approval.

type ArtifactRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}
    ArtifactRef is a reference to a stored artifact. This is a lightweight
    reference; the full artifact is in domain/artifact.

type Builder struct {
	// Has unexported fields.
}
    Builder provides a fluent API for constructing tools.

func NewBuilder(name string) *Builder
    NewBuilder creates a new tool builder with the given name.

func (b *Builder) Build() (Tool, error)
    Build constructs the tool definition.

func (b *Builder) Cacheable() *Builder
    Cacheable marks the tool as cacheable.

func (b *Builder) Destructive() *Builder
    Destructive marks the tool as destructive.

func (b *Builder) Idempotent() *Builder
    Idempotent marks the tool as idempotent.

func (b *Builder) MustBuild() Tool
    MustBuild constructs the tool definition or panics on error.

func (b *Builder) ReadOnly() *Builder
    ReadOnly marks the tool as read-only.

func (b *Builder) RequiresApproval() *Builder
    RequiresApproval marks the tool as requiring approval.

func (b *Builder) WithAnnotations(annotations Annotations) *Builder
    WithAnnotations sets the tool annotations.

func (b *Builder) WithDescription(desc string) *Builder
    WithDescription sets the tool description.

func (b *Builder) WithHandler(handler Handler) *Builder
    WithHandler sets the tool handler function.

func (b *Builder) WithInputSchema(schema Schema) *Builder
    WithInputSchema sets the input schema.

func (b *Builder) WithOutputSchema(schema Schema) *Builder
    WithOutputSchema sets the output schema.

func (b *Builder) WithRiskLevel(level RiskLevel) *Builder
    WithRiskLevel sets the risk level.

func (b *Builder) WithTags(tags ...string) *Builder
    WithTags adds tags to the tool.

type Definition struct {
	// Has unexported fields.
}
    Definition is a concrete implementation of Tool.

func (d *Definition) Annotations() Annotations
    Annotations returns the tool annotations.

func (d *Definition) Description() string
    Description returns the tool description.

func (d *Definition) Execute(ctx context.Context, input json.RawMessage) (Result, error)
    Execute runs the tool handler.

func (d *Definition) InputSchema() Schema
    InputSchema returns the input schema.

func (d *Definition) Name() string
    Name returns the tool name.

func (d *Definition) OutputSchema() Schema
    OutputSchema returns the output schema.

type Handler func(ctx context.Context, input json.RawMessage) (Result, error)
    Handler is the function signature for tool execution.

type Registry interface {
	// Register adds a tool to the registry.
	Register(tool Tool) error

	// Get retrieves a tool by name.
	Get(name string) (Tool, bool)

	// List returns all registered tools.
	List() []Tool

	// Names returns all registered tool names.
	Names() []string

	// Has checks if a tool is registered.
	Has(name string) bool

	// Unregister removes a tool from the registry.
	Unregister(name string) error
}
    Registry defines the interface for tool registration and lookup. This is a
    repository interface - implementations are in infrastructure.

type Result struct {
	// Output is the primary result data.
	Output json.RawMessage `json:"output"`

	// Artifacts are optional large outputs produced by the tool.
	Artifacts []ArtifactRef `json:"artifacts,omitempty"`

	// Duration is how long the execution took.
	Duration time.Duration `json:"duration"`

	// Cached indicates if this result was served from cache.
	Cached bool `json:"cached,omitempty"`

	// Error is a tool-level error (distinct from execution error).
	Error error `json:"-"`
}
    Result contains the output of a tool execution.

func NewCachedResult(output json.RawMessage) Result
    NewCachedResult creates a result marked as cached.

func NewErrorResult(err error) Result
    NewErrorResult creates a result representing an error.

func NewResult(output json.RawMessage) Result
    NewResult creates a successful result with the given output.

func NewResultWithDuration(output json.RawMessage, duration time.Duration) Result
    NewResultWithDuration creates a result with timing information.

func (r Result) HasArtifacts() bool
    HasArtifacts returns true if the result includes artifacts.

func (r Result) IsError() bool
    IsError returns true if the result represents an error.

func (r Result) OutputString() string
    OutputString returns the output as a string for convenience.

func (r Result) WithArtifact(ref ArtifactRef) Result
    WithArtifact adds an artifact reference to the result.

type RiskLevel int
    RiskLevel indicates the potential impact of a tool execution.

const (
	RiskNone     RiskLevel = iota // No risk - purely informational
	RiskLow                       // Low risk - reversible changes
	RiskMedium                    // Medium risk - may require cleanup
	RiskHigh                      // High risk - difficult to reverse
	RiskCritical                  // Critical risk - irreversible or destructive
)
func (r RiskLevel) String() string
    String returns the string representation of the risk level.

type Schema struct {
	// Has unexported fields.
}
    Schema wraps JSON Schema for input/output validation.

func EmptySchema() Schema
    EmptySchema returns a schema that accepts any input.

func NewSchema(raw json.RawMessage) Schema
    NewSchema creates a schema from raw JSON.

func ObjectSchema(properties map[string]json.RawMessage, required []string) Schema
    ObjectSchema returns a schema for an object with the given properties.

func (s Schema) IsEmpty() bool
    IsEmpty returns true if the schema is empty or nil.

func (s Schema) MarshalJSON() ([]byte, error)
    MarshalJSON implements json.Marshaler.

func (s Schema) Raw() json.RawMessage
    Raw returns the underlying JSON schema.

func (s *Schema) UnmarshalJSON(data []byte) error
    UnmarshalJSON implements json.Unmarshaler.

func (s Schema) Validate(data json.RawMessage) error
    Validate validates data against the schema. For now, this is a placeholder -
    full JSON Schema validation will be implemented in internal/schema.

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
    Tool represents a registered capability the agent can invoke.
```
