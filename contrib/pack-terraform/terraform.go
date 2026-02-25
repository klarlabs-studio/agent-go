// Package terraform provides Infrastructure as Code tools for agent-go.
//
// The pack uses an interface-based approach, allowing any IaC executor
// (Terraform CLI, OpenTofu, Pulumi adapter, etc.) to be plugged in.
package terraform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// IaCExecutor provides Infrastructure as Code operations.
type IaCExecutor interface {
	// Plan generates an execution plan showing what changes will be made.
	Plan(ctx context.Context, opts PlanOptions) (*PlanResult, error)

	// Apply applies the planned changes to infrastructure.
	Apply(ctx context.Context, opts ApplyOptions) (*ApplyResult, error)

	// Destroy tears down managed infrastructure.
	Destroy(ctx context.Context, opts DestroyOptions) (*ApplyResult, error)

	// Import imports existing infrastructure into state.
	Import(ctx context.Context, resourceType, resourceID, address string) error

	// StateList lists all resources in the current state.
	StateList(ctx context.Context) ([]StateResource, error)

	// Output retrieves output values from the state.
	Output(ctx context.Context, name string) (*OutputValue, error)

	// Validate validates the configuration files.
	Validate(ctx context.Context) (*ValidateResult, error)
}

// PlanOptions configures plan generation.
type PlanOptions struct {
	Targets   []string          `json:"targets,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	VarFile   string            `json:"var_file,omitempty"`
	Destroy   bool              `json:"destroy,omitempty"`
}

// PlanResult contains the execution plan.
type PlanResult struct {
	HasChanges bool             `json:"has_changes"`
	Add        int              `json:"add"`
	Change     int              `json:"change"`
	Destroy    int              `json:"destroy"`
	Resources  []PlannedChange  `json:"resources,omitempty"`
	PlanFile   string           `json:"plan_file,omitempty"`
	Summary    string           `json:"summary"`
}

// PlannedChange represents a single planned resource change.
type PlannedChange struct {
	Address    string         `json:"address"`
	Type       string         `json:"type"`
	Action     string         `json:"action"` // "create", "update", "delete", "replace"
	Before     map[string]any `json:"before,omitempty"`
	After      map[string]any `json:"after,omitempty"`
}

// ApplyOptions configures apply execution.
type ApplyOptions struct {
	PlanFile  string            `json:"plan_file,omitempty"`
	Targets   []string          `json:"targets,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	AutoApprove bool           `json:"auto_approve,omitempty"`
}

// ApplyResult contains apply execution results.
type ApplyResult struct {
	Success   bool            `json:"success"`
	Add       int             `json:"add"`
	Change    int             `json:"change"`
	Destroy   int             `json:"destroy"`
	Resources []AppliedChange `json:"resources,omitempty"`
	Summary   string          `json:"summary"`
}

// AppliedChange represents a resource that was changed.
type AppliedChange struct {
	Address string `json:"address"`
	Type    string `json:"type"`
	Action  string `json:"action"`
	ID      string `json:"id,omitempty"`
}

// DestroyOptions configures destroy execution.
type DestroyOptions struct {
	Targets     []string          `json:"targets,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	AutoApprove bool             `json:"auto_approve,omitempty"`
}

// StateResource represents a resource in the state.
type StateResource struct {
	Address    string         `json:"address"`
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Provider   string         `json:"provider"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// OutputValue represents a terraform output.
type OutputValue struct {
	Name      string `json:"name"`
	Value     any    `json:"value"`
	Type      string `json:"type"`
	Sensitive bool   `json:"sensitive"`
}

// ValidateResult contains validation results.
type ValidateResult struct {
	Valid        bool         `json:"valid"`
	ErrorCount   int          `json:"error_count"`
	WarningCount int          `json:"warning_count"`
	Diagnostics  []Diagnostic `json:"diagnostics,omitempty"`
}

// Diagnostic represents a validation diagnostic.
type Diagnostic struct {
	Severity string `json:"severity"` // "error", "warning"
	Summary  string `json:"summary"`
	Detail   string `json:"detail,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// Config holds terraform pack configuration.
type Config struct {
	// Executor is the IaC executor (required).
	Executor IaCExecutor
}

// Pack returns the IaC tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &terraformPack{cfg: cfg}

	return pack.NewBuilder("terraform").
		WithDescription("Infrastructure as Code tools: plan, apply, destroy, import, state management").
		WithVersion("1.0.0").
		AddTools(
			p.planTool(),
			p.applyTool(),
			p.destroyTool(),
			p.importTool(),
			p.stateListTool(),
			p.outputTool(),
			p.validateTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type terraformPack struct {
	cfg Config
}

func (p *terraformPack) planTool() tool.Tool {
	return tool.NewBuilder("terraform_plan").
		WithDescription("Generate an execution plan showing infrastructure changes").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Targets   []string          `json:"targets,omitempty"`
				Variables map[string]string `json:"variables,omitempty"`
				VarFile   string            `json:"var_file,omitempty"`
				Destroy   bool              `json:"destroy,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			result, err := p.cfg.Executor.Plan(ctx, PlanOptions{
				Targets:   in.Targets,
				Variables: in.Variables,
				VarFile:   in.VarFile,
				Destroy:   in.Destroy,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("plan failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *terraformPack) applyTool() tool.Tool {
	return tool.NewBuilder("terraform_apply").
		WithDescription("Apply planned infrastructure changes").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				PlanFile    string            `json:"plan_file,omitempty"`
				Targets     []string          `json:"targets,omitempty"`
				Variables   map[string]string `json:"variables,omitempty"`
				AutoApprove bool             `json:"auto_approve,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			result, err := p.cfg.Executor.Apply(ctx, ApplyOptions{
				PlanFile:    in.PlanFile,
				Targets:     in.Targets,
				Variables:   in.Variables,
				AutoApprove: in.AutoApprove,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("apply failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *terraformPack) destroyTool() tool.Tool {
	return tool.NewBuilder("terraform_destroy").
		WithDescription("Destroy managed infrastructure").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Targets     []string          `json:"targets,omitempty"`
				Variables   map[string]string `json:"variables,omitempty"`
				AutoApprove bool             `json:"auto_approve,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			result, err := p.cfg.Executor.Destroy(ctx, DestroyOptions{
				Targets:     in.Targets,
				Variables:   in.Variables,
				AutoApprove: in.AutoApprove,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("destroy failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *terraformPack) importTool() tool.Tool {
	return tool.NewBuilder("terraform_import").
		WithDescription("Import existing infrastructure into Terraform state").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ResourceType string `json:"resource_type"`
				ResourceID   string `json:"resource_id"`
				Address      string `json:"address"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ResourceType == "" {
				return tool.Result{}, fmt.Errorf("resource_type is required")
			}
			if in.ResourceID == "" {
				return tool.Result{}, fmt.Errorf("resource_id is required")
			}
			if in.Address == "" {
				return tool.Result{}, fmt.Errorf("address is required")
			}

			err := p.cfg.Executor.Import(ctx, in.ResourceType, in.ResourceID, in.Address)
			if err != nil {
				return tool.Result{}, fmt.Errorf("import failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"address":       in.Address,
				"resource_type": in.ResourceType,
				"resource_id":   in.ResourceID,
				"success":       true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *terraformPack) stateListTool() tool.Tool {
	return tool.NewBuilder("terraform_state_list").
		WithDescription("List all resources in the Terraform state").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			resources, err := p.cfg.Executor.StateList(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("state list failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":     len(resources),
				"resources": resources,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *terraformPack) outputTool() tool.Tool {
	return tool.NewBuilder("terraform_output").
		WithDescription("Get a Terraform output value").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Name == "" {
				return tool.Result{}, fmt.Errorf("name is required")
			}

			val, err := p.cfg.Executor.Output(ctx, in.Name)
			if err != nil {
				return tool.Result{}, fmt.Errorf("output failed: %w", err)
			}

			output, _ := json.Marshal(val)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *terraformPack) validateTool() tool.Tool {
	return tool.NewBuilder("terraform_validate").
		WithDescription("Validate Terraform configuration files").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			result, err := p.cfg.Executor.Validate(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("validate failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
