// Package ci provides CI/CD integration tools for agent-go.
//
// The pack uses an interface-based approach, allowing any CI/CD platform
// (GitHub Actions, GitLab CI, Jenkins, CircleCI, etc.) to be plugged in.
package ci

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// CIProvider provides CI/CD pipeline operations.
type CIProvider interface {
	// TriggerPipeline triggers a new pipeline run.
	TriggerPipeline(ctx context.Context, opts TriggerOptions) (*PipelineRun, error)

	// GetPipelineRun retrieves the status of a pipeline run.
	GetPipelineRun(ctx context.Context, runID string) (*PipelineRun, error)

	// ListPipelineRuns lists recent pipeline runs.
	ListPipelineRuns(ctx context.Context, opts ListOptions) ([]PipelineRun, error)

	// GetBuildLog retrieves logs for a specific build step.
	GetBuildLog(ctx context.Context, runID, stepName string) (string, error)

	// CancelPipelineRun cancels a running pipeline.
	CancelPipelineRun(ctx context.Context, runID string) error

	// GetArtifacts lists artifacts produced by a pipeline run.
	GetArtifacts(ctx context.Context, runID string) ([]Artifact, error)

	// Deploy triggers a deployment to the specified environment.
	Deploy(ctx context.Context, opts DeployOptions) (*Deployment, error)

	// Rollback rolls back a deployment to a previous version.
	Rollback(ctx context.Context, environment, targetVersion string) (*Deployment, error)
}

// TriggerOptions configures a pipeline trigger.
type TriggerOptions struct {
	Pipeline  string            `json:"pipeline"`
	Branch    string            `json:"branch,omitempty"`
	Commit    string            `json:"commit,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

// ListOptions configures pipeline run listing.
type ListOptions struct {
	Pipeline   string `json:"pipeline,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Status     string `json:"status,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// PipelineRun represents a CI pipeline execution.
type PipelineRun struct {
	ID         string    `json:"id"`
	Pipeline   string    `json:"pipeline"`
	Branch     string    `json:"branch,omitempty"`
	Commit     string    `json:"commit,omitempty"`
	Status     string    `json:"status"` // "pending", "running", "success", "failed", "cancelled"
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Duration   string    `json:"duration,omitempty"`
	Steps      []Step    `json:"steps,omitempty"`
	URL        string    `json:"url,omitempty"`
}

// Step represents a pipeline step or job.
type Step struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Duration   string    `json:"duration,omitempty"`
}

// Artifact represents a build artifact.
type Artifact struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	URL       string `json:"url,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// DeployOptions configures a deployment.
type DeployOptions struct {
	Environment string            `json:"environment"`
	Version     string            `json:"version,omitempty"`
	RunID       string            `json:"run_id,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
}

// Deployment represents a deployment operation.
type Deployment struct {
	ID          string    `json:"id"`
	Environment string    `json:"environment"`
	Version     string    `json:"version"`
	Status      string    `json:"status"`
	DeployedAt  time.Time `json:"deployed_at,omitempty"`
	DeployedBy  string    `json:"deployed_by,omitempty"`
	URL         string    `json:"url,omitempty"`
}

// Config holds CI pack configuration.
type Config struct {
	// Provider is the CI/CD provider (required).
	Provider CIProvider

	// DefaultPipeline is the default pipeline name.
	DefaultPipeline string

	// DefaultBranch is the default branch for triggers.
	DefaultBranch string
}

// Pack returns the CI/CD integration tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &ciPack{cfg: cfg}
	if p.cfg.DefaultBranch == "" {
		p.cfg.DefaultBranch = "main"
	}

	return pack.NewBuilder("ci").
		WithDescription("CI/CD integration tools: trigger pipelines, check build status, deploy, rollback").
		WithVersion("1.0.0").
		AddTools(
			p.triggerPipelineTool(),
			p.getBuildStatusTool(),
			p.listPipelineRunsTool(),
			p.getBuildLogTool(),
			p.cancelPipelineTool(),
			p.getArtifactsTool(),
			p.deployTool(),
			p.rollbackTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type ciPack struct {
	cfg Config
}

func (p *ciPack) triggerPipelineTool() tool.Tool {
	return tool.NewBuilder("ci_trigger_pipeline").
		WithDescription("Trigger a CI/CD pipeline run").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pipeline  string            `json:"pipeline,omitempty"`
				Branch    string            `json:"branch,omitempty"`
				Commit    string            `json:"commit,omitempty"`
				Variables map[string]string `json:"variables,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			pipeline := in.Pipeline
			if pipeline == "" {
				pipeline = p.cfg.DefaultPipeline
			}
			if pipeline == "" {
				return tool.Result{}, fmt.Errorf("pipeline is required")
			}

			branch := in.Branch
			if branch == "" {
				branch = p.cfg.DefaultBranch
			}

			run, err := p.cfg.Provider.TriggerPipeline(ctx, TriggerOptions{
				Pipeline:  pipeline,
				Branch:    branch,
				Commit:    in.Commit,
				Variables: in.Variables,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("trigger pipeline failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"run":     run,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ciPack) getBuildStatusTool() tool.Tool {
	return tool.NewBuilder("ci_get_build_status").
		WithDescription("Get the status of a CI/CD pipeline run").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.RunID == "" {
				return tool.Result{}, fmt.Errorf("run_id is required")
			}

			run, err := p.cfg.Provider.GetPipelineRun(ctx, in.RunID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get build status failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"run": run,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ciPack) listPipelineRunsTool() tool.Tool {
	return tool.NewBuilder("ci_list_pipeline_runs").
		WithDescription("List recent CI/CD pipeline runs").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pipeline   string `json:"pipeline,omitempty"`
				Branch     string `json:"branch,omitempty"`
				Status     string `json:"status,omitempty"`
				MaxResults int    `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.MaxResults == 0 {
				in.MaxResults = 10
			}

			runs, err := p.cfg.Provider.ListPipelineRuns(ctx, ListOptions{
				Pipeline:   in.Pipeline,
				Branch:     in.Branch,
				Status:     in.Status,
				MaxResults: in.MaxResults,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("list pipeline runs failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count": len(runs),
				"runs":  runs,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ciPack) getBuildLogTool() tool.Tool {
	return tool.NewBuilder("ci_get_build_log").
		WithDescription("Get logs for a specific build step").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				RunID    string `json:"run_id"`
				StepName string `json:"step_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.RunID == "" {
				return tool.Result{}, fmt.Errorf("run_id is required")
			}
			if in.StepName == "" {
				return tool.Result{}, fmt.Errorf("step_name is required")
			}

			log, err := p.cfg.Provider.GetBuildLog(ctx, in.RunID, in.StepName)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get build log failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"run_id":    in.RunID,
				"step_name": in.StepName,
				"log":       log,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ciPack) cancelPipelineTool() tool.Tool {
	return tool.NewBuilder("ci_cancel_pipeline").
		WithDescription("Cancel a running CI/CD pipeline").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.RunID == "" {
				return tool.Result{}, fmt.Errorf("run_id is required")
			}

			err := p.cfg.Provider.CancelPipelineRun(ctx, in.RunID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("cancel pipeline failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"run_id":  in.RunID,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ciPack) getArtifactsTool() tool.Tool {
	return tool.NewBuilder("ci_get_artifacts").
		WithDescription("List artifacts produced by a pipeline run").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				RunID string `json:"run_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.RunID == "" {
				return tool.Result{}, fmt.Errorf("run_id is required")
			}

			artifacts, err := p.cfg.Provider.GetArtifacts(ctx, in.RunID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get artifacts failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"run_id":    in.RunID,
				"count":     len(artifacts),
				"artifacts": artifacts,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ciPack) deployTool() tool.Tool {
	return tool.NewBuilder("ci_deploy").
		WithDescription("Trigger a deployment to the specified environment").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Environment string            `json:"environment"`
				Version     string            `json:"version,omitempty"`
				RunID       string            `json:"run_id,omitempty"`
				Variables   map[string]string `json:"variables,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Environment == "" {
				return tool.Result{}, fmt.Errorf("environment is required")
			}

			deployment, err := p.cfg.Provider.Deploy(ctx, DeployOptions{
				Environment: in.Environment,
				Version:     in.Version,
				RunID:       in.RunID,
				Variables:   in.Variables,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("deploy failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"deployment": deployment,
				"success":    true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ciPack) rollbackTool() tool.Tool {
	return tool.NewBuilder("ci_rollback").
		WithDescription("Rollback a deployment to a previous version").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Environment   string `json:"environment"`
				TargetVersion string `json:"target_version"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Environment == "" {
				return tool.Result{}, fmt.Errorf("environment is required")
			}
			if in.TargetVersion == "" {
				return tool.Result{}, fmt.Errorf("target_version is required")
			}

			deployment, err := p.cfg.Provider.Rollback(ctx, in.Environment, in.TargetVersion)
			if err != nil {
				return tool.Result{}, fmt.Errorf("rollback failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"deployment": deployment,
				"success":    true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
