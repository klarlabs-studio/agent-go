// Package etl provides ETL pipeline tools for agent-go.
//
// The pack uses an interface-based approach, allowing any ETL engine
// (custom pipelines, Apache Beam, cloud services, etc.) to be plugged in.
package etl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Extractor provides data extraction capabilities.
type Extractor interface {
	Extract(ctx context.Context, source DataSource, opts ExtractOptions) (*ExtractResult, error)
}

// Transformer provides data transformation capabilities.
type Transformer interface {
	Transform(ctx context.Context, data []Record, steps []TransformStep) (*TransformResult, error)
}

// Loader provides data loading capabilities.
type Loader interface {
	Load(ctx context.Context, target DataTarget, records []Record, opts LoadOptions) (*LoadResult, error)
}

// SchemaValidator validates data against a schema.
type SchemaValidator interface {
	Validate(ctx context.Context, records []Record, schema Schema) (*ValidationResult, error)
}

// PipelineRunner executes end-to-end ETL pipelines.
type PipelineRunner interface {
	RunPipeline(ctx context.Context, pipeline Pipeline) (*PipelineResult, error)
}

// DataSource describes where to extract data from.
type DataSource struct {
	Type       string            `json:"type"` // "database", "file", "api", "stream"
	URI        string            `json:"uri"`
	Format     string            `json:"format,omitempty"`
	Query      string            `json:"query,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// DataTarget describes where to load data to.
type DataTarget struct {
	Type       string            `json:"type"` // "database", "file", "api", "stream"
	URI        string            `json:"uri"`
	Format     string            `json:"format,omitempty"`
	Table      string            `json:"table,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// Record represents a data record in the ETL pipeline.
type Record struct {
	ID     string         `json:"id,omitempty"`
	Fields map[string]any `json:"fields"`
}

// ExtractOptions configures extraction.
type ExtractOptions struct {
	BatchSize int               `json:"batch_size,omitempty"`
	Filter    map[string]string `json:"filter,omitempty"`
	Limit     int               `json:"limit,omitempty"`
	Offset    int               `json:"offset,omitempty"`
}

// ExtractResult contains extraction output.
type ExtractResult struct {
	Records    []Record `json:"records"`
	TotalCount int      `json:"total_count"`
	HasMore    bool     `json:"has_more"`
}

// TransformStep describes a transformation operation.
type TransformStep struct {
	Type   string         `json:"type"` // "map", "filter", "rename", "cast", "aggregate", "join", "deduplicate"
	Config map[string]any `json:"config"`
}

// TransformResult contains transformation output.
type TransformResult struct {
	Records      []Record `json:"records"`
	InputCount   int      `json:"input_count"`
	OutputCount  int      `json:"output_count"`
	DroppedCount int      `json:"dropped_count"`
}

// LoadOptions configures loading behavior.
type LoadOptions struct {
	BatchSize int    `json:"batch_size,omitempty"`
	Mode      string `json:"mode,omitempty"` // "insert", "upsert", "replace", "append"
	OnError   string `json:"on_error,omitempty"` // "fail", "skip", "log"
}

// LoadResult contains loading output.
type LoadResult struct {
	Loaded  int `json:"loaded"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// Schema describes the expected structure of records.
type Schema struct {
	Fields []SchemaField `json:"fields"`
}

// SchemaField describes a field in the schema.
type SchemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "string", "int", "float", "bool", "datetime", "json"
	Required bool   `json:"required,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
}

// ValidationResult contains schema validation output.
type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Errors  []ValidationError `json:"errors,omitempty"`
	Checked int               `json:"checked"`
}

// ValidationError describes a validation failure.
type ValidationError struct {
	RecordIndex int    `json:"record_index"`
	Field       string `json:"field"`
	Message     string `json:"message"`
}

// Pipeline describes an end-to-end ETL pipeline.
type Pipeline struct {
	Name       string          `json:"name"`
	Source     DataSource      `json:"source"`
	Target     DataTarget      `json:"target"`
	Transforms []TransformStep `json:"transforms,omitempty"`
	Schema     *Schema         `json:"schema,omitempty"`
	LoadOpts   LoadOptions     `json:"load_options,omitempty"`
}

// PipelineResult contains pipeline execution output.
type PipelineResult struct {
	Name       string          `json:"name"`
	Extracted  int             `json:"extracted"`
	Transformed int            `json:"transformed"`
	Loaded     int             `json:"loaded"`
	Failed     int             `json:"failed"`
	Validation *ValidationResult `json:"validation,omitempty"`
}

// Config holds ETL pack configuration.
type Config struct {
	Extractor  Extractor
	Transformer Transformer
	Loader     Loader
	Validator  SchemaValidator  // optional
	Runner     PipelineRunner   // optional
}

// Pack returns the ETL pipeline tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &etlPack{cfg: cfg}

	tools := []tool.Tool{
		p.extractTool(), p.transformTool(), p.loadTool(), p.validateTool(),
	}

	if cfg.Runner != nil {
		tools = append(tools, p.runPipelineTool())
	}

	return pack.NewBuilder("etl").
		WithDescription("ETL pipeline tools: extract, transform, load, validate_schema, run_pipeline").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type etlPack struct{ cfg Config }

func (p *etlPack) extractTool() tool.Tool {
	return tool.NewBuilder("etl_extract").
		WithDescription("Extract data from a source").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Source    DataSource     `json:"source"`
				BatchSize int           `json:"batch_size,omitempty"`
				Filter   map[string]string `json:"filter,omitempty"`
				Limit    int            `json:"limit,omitempty"`
				Offset   int            `json:"offset,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Source.Type == "" || in.Source.URI == "" {
				return tool.Result{}, fmt.Errorf("source type and uri are required")
			}
			result, err := p.cfg.Extractor.Extract(ctx, in.Source, ExtractOptions{
				BatchSize: in.BatchSize, Filter: in.Filter,
				Limit: in.Limit, Offset: in.Offset,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("extract failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *etlPack) transformTool() tool.Tool {
	return tool.NewBuilder("etl_transform").
		WithDescription("Transform data using a series of steps").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Records []Record        `json:"records"`
				Steps   []TransformStep `json:"steps"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.Records) == 0 {
				return tool.Result{}, fmt.Errorf("records are required")
			}
			if len(in.Steps) == 0 {
				return tool.Result{}, fmt.Errorf("at least one transform step is required")
			}
			result, err := p.cfg.Transformer.Transform(ctx, in.Records, in.Steps)
			if err != nil {
				return tool.Result{}, fmt.Errorf("transform failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *etlPack) loadTool() tool.Tool {
	return tool.NewBuilder("etl_load").
		WithDescription("Load data into a target").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Target    DataTarget  `json:"target"`
				Records   []Record    `json:"records"`
				BatchSize int         `json:"batch_size,omitempty"`
				Mode      string      `json:"mode,omitempty"`
				OnError   string      `json:"on_error,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Target.Type == "" || in.Target.URI == "" {
				return tool.Result{}, fmt.Errorf("target type and uri are required")
			}
			if len(in.Records) == 0 {
				return tool.Result{}, fmt.Errorf("records are required")
			}
			result, err := p.cfg.Loader.Load(ctx, in.Target, in.Records, LoadOptions{
				BatchSize: in.BatchSize, Mode: in.Mode, OnError: in.OnError,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("load failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *etlPack) validateTool() tool.Tool {
	return tool.NewBuilder("etl_validate_schema").
		WithDescription("Validate data records against a schema").
		ReadOnly().Idempotent().Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Records []Record `json:"records"`
				Schema  Schema   `json:"schema"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if len(in.Records) == 0 {
				return tool.Result{}, fmt.Errorf("records are required")
			}
			if len(in.Schema.Fields) == 0 {
				return tool.Result{}, fmt.Errorf("schema fields are required")
			}
			if p.cfg.Validator == nil {
				return tool.Result{}, fmt.Errorf("schema validator not configured")
			}
			result, err := p.cfg.Validator.Validate(ctx, in.Records, in.Schema)
			if err != nil {
				return tool.Result{}, fmt.Errorf("validate failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *etlPack) runPipelineTool() tool.Tool {
	return tool.NewBuilder("etl_run_pipeline").
		WithDescription("Execute an end-to-end ETL pipeline").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in Pipeline
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Name == "" {
				return tool.Result{}, fmt.Errorf("pipeline name is required")
			}
			if in.Source.Type == "" || in.Source.URI == "" {
				return tool.Result{}, fmt.Errorf("source type and uri are required")
			}
			if in.Target.Type == "" || in.Target.URI == "" {
				return tool.Result{}, fmt.Errorf("target type and uri are required")
			}
			result, err := p.cfg.Runner.RunPipeline(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("pipeline failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
