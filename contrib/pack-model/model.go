// Package model provides ML model management tools for agent-go.
//
// The pack uses an interface-based approach, allowing any model serving
// platform (MLflow, SageMaker, Vertex AI, local inference, etc.) to be plugged in.
package model

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// ModelPlatform provides model management and inference operations.
type ModelPlatform interface {
	// Inference runs inference on a deployed model.
	Inference(ctx context.Context, modelID string, input any, opts InferenceOptions) (*InferenceResult, error)

	// ListModels lists available models.
	ListModels(ctx context.Context, opts ListOptions) ([]ModelInfo, error)

	// GetModel retrieves detailed model information.
	GetModel(ctx context.Context, modelID string) (*ModelInfo, error)

	// DeployModel deploys a model for inference.
	DeployModel(ctx context.Context, modelID string, opts DeployOptions) (*DeploymentInfo, error)

	// UndeployModel removes a model deployment.
	UndeployModel(ctx context.Context, deploymentID string) error

	// Evaluate evaluates a model against a dataset.
	Evaluate(ctx context.Context, modelID string, dataset any, metrics []string) (*EvalResult, error)
}

// InferenceOptions configures model inference.
type InferenceOptions struct {
	Version     string         `json:"version,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// InferenceResult contains inference output.
type InferenceResult struct {
	Output     any            `json:"output"`
	ModelID    string         `json:"model_id"`
	Version    string         `json:"version,omitempty"`
	LatencyMS  int64          `json:"latency_ms"`
	TokensUsed int            `json:"tokens_used,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ListOptions configures model listing.
type ListOptions struct {
	Framework  string `json:"framework,omitempty"`
	Task       string `json:"task,omitempty"`
	Status     string `json:"status,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// ModelInfo describes a model.
type ModelInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Version     string         `json:"version,omitempty"`
	Framework   string         `json:"framework,omitempty"`
	Task        string         `json:"task,omitempty"`
	Status      string         `json:"status,omitempty"`
	Description string         `json:"description,omitempty"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	CreatedAt   string         `json:"created_at,omitempty"`
}

// DeployOptions configures model deployment.
type DeployOptions struct {
	InstanceType string `json:"instance_type,omitempty"`
	MinReplicas  int    `json:"min_replicas,omitempty"`
	MaxReplicas  int    `json:"max_replicas,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
}

// DeploymentInfo describes a model deployment.
type DeploymentInfo struct {
	ID        string `json:"id"`
	ModelID   string `json:"model_id"`
	Status    string `json:"status"`
	Endpoint  string `json:"endpoint,omitempty"`
	Replicas  int    `json:"replicas,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// EvalResult contains model evaluation results.
type EvalResult struct {
	ModelID  string             `json:"model_id"`
	Metrics  map[string]float64 `json:"metrics"`
	Samples  int                `json:"samples"`
	Duration string             `json:"duration,omitempty"`
}

// Config holds model pack configuration.
type Config struct {
	// Platform is the model management platform (required).
	Platform ModelPlatform
}

// Pack returns the model management tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &modelPack{cfg: cfg}

	return pack.NewBuilder("model").
		WithDescription("ML model management tools: inference, deployment, evaluation, listing").
		WithVersion("1.0.0").
		AddTools(
			p.inferenceTool(),
			p.listModelsTool(),
			p.getModelTool(),
			p.deployModelTool(),
			p.undeployModelTool(),
			p.evaluateTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type modelPack struct {
	cfg Config
}

func (p *modelPack) inferenceTool() tool.Tool {
	return tool.NewBuilder("model_inference").
		WithDescription("Run inference on a deployed model").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ModelID     string         `json:"model_id"`
				Input       any            `json:"input"`
				Version     string         `json:"version,omitempty"`
				MaxTokens   int            `json:"max_tokens,omitempty"`
				Temperature float64        `json:"temperature,omitempty"`
				Parameters  map[string]any `json:"parameters,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ModelID == "" {
				return tool.Result{}, fmt.Errorf("model_id is required")
			}
			if in.Input == nil {
				return tool.Result{}, fmt.Errorf("input is required")
			}

			result, err := p.cfg.Platform.Inference(ctx, in.ModelID, in.Input, InferenceOptions{
				Version:     in.Version,
				MaxTokens:   in.MaxTokens,
				Temperature: in.Temperature,
				Parameters:  in.Parameters,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("inference failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *modelPack) listModelsTool() tool.Tool {
	return tool.NewBuilder("model_list_models").
		WithDescription("List available ML models").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Framework  string `json:"framework,omitempty"`
				Task       string `json:"task,omitempty"`
				Status     string `json:"status,omitempty"`
				MaxResults int    `json:"max_results,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.MaxResults == 0 {
				in.MaxResults = 20
			}

			models, err := p.cfg.Platform.ListModels(ctx, ListOptions{
				Framework:  in.Framework,
				Task:       in.Task,
				Status:     in.Status,
				MaxResults: in.MaxResults,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("list models failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(models),
				"models": models,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *modelPack) getModelTool() tool.Tool {
	return tool.NewBuilder("model_get_model").
		WithDescription("Get detailed information about a model").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ModelID string `json:"model_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ModelID == "" {
				return tool.Result{}, fmt.Errorf("model_id is required")
			}

			model, err := p.cfg.Platform.GetModel(ctx, in.ModelID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get model failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"model": model,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *modelPack) deployModelTool() tool.Tool {
	return tool.NewBuilder("model_deploy").
		WithDescription("Deploy a model for inference").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ModelID      string `json:"model_id"`
				InstanceType string `json:"instance_type,omitempty"`
				MinReplicas  int    `json:"min_replicas,omitempty"`
				MaxReplicas  int    `json:"max_replicas,omitempty"`
				Endpoint     string `json:"endpoint,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ModelID == "" {
				return tool.Result{}, fmt.Errorf("model_id is required")
			}

			deployment, err := p.cfg.Platform.DeployModel(ctx, in.ModelID, DeployOptions{
				InstanceType: in.InstanceType,
				MinReplicas:  in.MinReplicas,
				MaxReplicas:  in.MaxReplicas,
				Endpoint:     in.Endpoint,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("deploy model failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"deployment": deployment,
				"success":    true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *modelPack) undeployModelTool() tool.Tool {
	return tool.NewBuilder("model_undeploy").
		WithDescription("Remove a model deployment").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				DeploymentID string `json:"deployment_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.DeploymentID == "" {
				return tool.Result{}, fmt.Errorf("deployment_id is required")
			}

			err := p.cfg.Platform.UndeployModel(ctx, in.DeploymentID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("undeploy failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"deployment_id": in.DeploymentID,
				"success":       true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *modelPack) evaluateTool() tool.Tool {
	return tool.NewBuilder("model_evaluate").
		WithDescription("Evaluate a model against a dataset with specified metrics").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ModelID string   `json:"model_id"`
				Dataset any      `json:"dataset"`
				Metrics []string `json:"metrics,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ModelID == "" {
				return tool.Result{}, fmt.Errorf("model_id is required")
			}
			if in.Dataset == nil {
				return tool.Result{}, fmt.Errorf("dataset is required")
			}
			if len(in.Metrics) == 0 {
				in.Metrics = []string{"accuracy", "f1", "precision", "recall"}
			}

			result, err := p.cfg.Platform.Evaluate(ctx, in.ModelID, in.Dataset, in.Metrics)
			if err != nil {
				return tool.Result{}, fmt.Errorf("evaluate failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
