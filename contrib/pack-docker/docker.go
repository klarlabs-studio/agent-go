// Package docker provides container management tools for agent-go.
//
// The pack uses an interface-based approach, allowing any container runtime
// (Docker Engine, Podman, containerd, etc.) to be plugged in.
package docker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// ContainerRuntime provides container management operations.
type ContainerRuntime interface {
	Build(ctx context.Context, opts BuildOptions) (*BuildResult, error)
	Push(ctx context.Context, image, registry string) error
	Run(ctx context.Context, opts RunOptions) (*Container, error)
	Stop(ctx context.Context, containerID string, timeout int) error
	Logs(ctx context.Context, containerID string, opts LogOptions) (string, error)
	Exec(ctx context.Context, containerID string, cmd []string) (*ExecResult, error)
	ListContainers(ctx context.Context, all bool) ([]Container, error)
}

// ComposeRuntime provides docker-compose operations.
type ComposeRuntime interface {
	ComposeUp(ctx context.Context, file string, opts ComposeOptions) error
	ComposeDown(ctx context.Context, file string, removeVolumes bool) error
}

// BuildOptions configures an image build.
type BuildOptions struct {
	Context    string            `json:"context"`
	Dockerfile string            `json:"dockerfile,omitempty"`
	Tag        string            `json:"tag"`
	BuildArgs  map[string]string `json:"build_args,omitempty"`
	NoCache    bool              `json:"no_cache,omitempty"`
	Target     string            `json:"target,omitempty"`
}

// BuildResult contains build output.
type BuildResult struct {
	ImageID string `json:"image_id"`
	Tag     string `json:"tag"`
	Size    int64  `json:"size_bytes,omitempty"`
}

// RunOptions configures a container run.
type RunOptions struct {
	Image       string            `json:"image"`
	Name        string            `json:"name,omitempty"`
	Ports       map[string]string `json:"ports,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Detach      bool              `json:"detach,omitempty"`
	Remove      bool              `json:"remove,omitempty"`
	Network     string            `json:"network,omitempty"`
}

// Container represents a container instance.
type Container struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	Ports   map[string]string `json:"ports,omitempty"`
	Created string            `json:"created,omitempty"`
}

// LogOptions configures log retrieval.
type LogOptions struct {
	Tail   int  `json:"tail,omitempty"`
	Follow bool `json:"follow,omitempty"`
	Since  string `json:"since,omitempty"`
}

// ExecResult contains command execution output.
type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// ComposeOptions configures compose operations.
type ComposeOptions struct {
	Build   bool     `json:"build,omitempty"`
	Detach  bool     `json:"detach,omitempty"`
	Services []string `json:"services,omitempty"`
}

// Config holds docker pack configuration.
type Config struct {
	Runtime ContainerRuntime
	Compose ComposeRuntime
}

// Pack returns the container management tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &dockerPack{cfg: cfg}

	tools := []tool.Tool{
		p.buildTool(),
		p.pushTool(),
		p.runTool(),
		p.stopTool(),
		p.logsTool(),
		p.execTool(),
		p.listTool(),
	}

	if cfg.Compose != nil {
		tools = append(tools, p.composeUpTool(), p.composeDownTool())
	}

	return pack.NewBuilder("docker").
		WithDescription("Container management tools: build, push, run, compose, logs, exec").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type dockerPack struct{ cfg Config }

func (p *dockerPack) buildTool() tool.Tool {
	return tool.NewBuilder("docker_build").
		WithDescription("Build a container image").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in BuildOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Context == "" {
				in.Context = "."
			}
			if in.Tag == "" {
				return tool.Result{}, fmt.Errorf("tag is required")
			}
			result, err := p.cfg.Runtime.Build(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("build failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *dockerPack) pushTool() tool.Tool {
	return tool.NewBuilder("docker_push").
		WithDescription("Push a container image to a registry").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Image    string `json:"image"`
				Registry string `json:"registry,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Image == "" {
				return tool.Result{}, fmt.Errorf("image is required")
			}
			err := p.cfg.Runtime.Push(ctx, in.Image, in.Registry)
			if err != nil {
				return tool.Result{}, fmt.Errorf("push failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"image": in.Image, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *dockerPack) runTool() tool.Tool {
	return tool.NewBuilder("docker_run").
		WithDescription("Run a container from an image").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in RunOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Image == "" {
				return tool.Result{}, fmt.Errorf("image is required")
			}
			container, err := p.cfg.Runtime.Run(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("run failed: %w", err)
			}
			output, _ := json.Marshal(container)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *dockerPack) stopTool() tool.Tool {
	return tool.NewBuilder("docker_stop").
		WithDescription("Stop a running container").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ContainerID string `json:"container_id"`
				Timeout     int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ContainerID == "" {
				return tool.Result{}, fmt.Errorf("container_id is required")
			}
			err := p.cfg.Runtime.Stop(ctx, in.ContainerID, in.Timeout)
			if err != nil {
				return tool.Result{}, fmt.Errorf("stop failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"container_id": in.ContainerID, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *dockerPack) logsTool() tool.Tool {
	return tool.NewBuilder("docker_logs").
		WithDescription("Get container logs").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ContainerID string `json:"container_id"`
				Tail        int    `json:"tail,omitempty"`
				Since       string `json:"since,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ContainerID == "" {
				return tool.Result{}, fmt.Errorf("container_id is required")
			}
			logs, err := p.cfg.Runtime.Logs(ctx, in.ContainerID, LogOptions{
				Tail: in.Tail, Since: in.Since,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("logs failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"container_id": in.ContainerID, "logs": logs})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *dockerPack) execTool() tool.Tool {
	return tool.NewBuilder("docker_exec").
		WithDescription("Execute a command in a running container").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ContainerID string   `json:"container_id"`
				Command     []string `json:"command"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.ContainerID == "" {
				return tool.Result{}, fmt.Errorf("container_id is required")
			}
			if len(in.Command) == 0 {
				return tool.Result{}, fmt.Errorf("command is required")
			}
			result, err := p.cfg.Runtime.Exec(ctx, in.ContainerID, in.Command)
			if err != nil {
				return tool.Result{}, fmt.Errorf("exec failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *dockerPack) listTool() tool.Tool {
	return tool.NewBuilder("docker_list_containers").
		WithDescription("List containers").
		ReadOnly().Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				All bool `json:"all,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			containers, err := p.cfg.Runtime.ListContainers(ctx, in.All)
			if err != nil {
				return tool.Result{}, fmt.Errorf("list containers failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"count": len(containers), "containers": containers})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *dockerPack) composeUpTool() tool.Tool {
	return tool.NewBuilder("docker_compose_up").
		WithDescription("Start services with docker-compose").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				File     string   `json:"file,omitempty"`
				Build    bool     `json:"build,omitempty"`
				Detach   bool     `json:"detach,omitempty"`
				Services []string `json:"services,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.File == "" {
				in.File = "docker-compose.yml"
			}
			err := p.cfg.Compose.ComposeUp(ctx, in.File, ComposeOptions{
				Build: in.Build, Detach: in.Detach, Services: in.Services,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("compose up failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"file": in.File, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *dockerPack) composeDownTool() tool.Tool {
	return tool.NewBuilder("docker_compose_down").
		WithDescription("Stop and remove docker-compose services").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				File          string `json:"file,omitempty"`
				RemoveVolumes bool   `json:"remove_volumes,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.File == "" {
				in.File = "docker-compose.yml"
			}
			err := p.cfg.Compose.ComposeDown(ctx, in.File, in.RemoveVolumes)
			if err != nil {
				return tool.Result{}, fmt.Errorf("compose down failed: %w", err)
			}
			output, _ := json.Marshal(map[string]any{"file": in.File, "success": true})
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
