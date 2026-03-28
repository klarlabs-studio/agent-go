// Package kubernetes provides Kubernetes operation tools for agent-go.
//
// This pack shells out to kubectl for cluster operations, avoiding heavy
// client-go dependencies. kubectl must be installed and available on PATH.
//
// Tools provided:
//   - kubectl_get: Get Kubernetes resources with -o json output
//   - kubectl_describe: Describe a Kubernetes resource
//   - kubectl_logs: Get pod logs (with optional container, tail lines)
//   - kubectl_apply: Apply a manifest to the cluster (Destructive)
//   - kubectl_delete: Delete a Kubernetes resource (Destructive)
//   - kubectl_exec: Execute a command in a pod container
//   - kubectl_port_forward: NOT SUPPORTED (long-running, incompatible with tool model)
//   - kubectl_get_contexts: List available kubeconfig contexts
//   - kubectl_get_namespaces: List all namespaces with -o json output
//
// Authentication is handled by the ambient kubeconfig (~/.kube/config or
// KUBECONFIG env var). In-cluster config works automatically when running
// inside a Kubernetes pod.
package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// defaultTimeout is the default command execution timeout.
const defaultTimeout = 30 * time.Second

// Pack returns the Kubernetes tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("kubernetes").
		WithDescription("Kubernetes cluster management tools via kubectl CLI").
		WithVersion("0.2.0").
		AddTools(
			kubectlGet(),
			kubectlDescribe(),
			kubectlLogs(),
			kubectlApply(),
			kubectlDelete(),
			kubectlExec(),
			kubectlPortForward(),
			kubectlGetContexts(),
			kubectlGetNamespaces(),
		).
		AllowInState(agent.StateExplore,
			"kubectl_get", "kubectl_describe", "kubectl_logs",
			"kubectl_get_contexts", "kubectl_get_namespaces",
		).
		AllowInState(agent.StateAct,
			"kubectl_get", "kubectl_describe", "kubectl_logs",
			"kubectl_apply", "kubectl_delete", "kubectl_exec",
			"kubectl_get_contexts", "kubectl_get_namespaces",
		).
		AllowInState(agent.StateValidate,
			"kubectl_get", "kubectl_describe", "kubectl_logs",
			"kubectl_get_contexts", "kubectl_get_namespaces",
		).
		Build()
}

// --- Input types ---

// getInput is the input for kubectl_get.
type getInput struct {
	Resource  string `json:"resource"`                 // e.g. "pods", "services", "deployments"
	Name      string `json:"name,omitempty"`           // optional: specific resource name
	Namespace string `json:"namespace,omitempty"`      // optional: defaults to current context namespace
	Context   string `json:"context,omitempty"`        // optional: kubeconfig context
	Selector  string `json:"selector,omitempty"`       // optional: label selector
	AllNS     bool   `json:"all_namespaces,omitempty"` // optional: search all namespaces
}

// describeInput is the input for kubectl_describe.
type describeInput struct {
	Resource  string `json:"resource"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Context   string `json:"context,omitempty"`
}

// logsInput is the input for kubectl_logs.
type logsInput struct {
	Pod       string `json:"pod"`
	Container string `json:"container,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Context   string `json:"context,omitempty"`
	Tail      int    `json:"tail,omitempty"`     // number of lines from end; 0 means all
	Since     string `json:"since,omitempty"`    // e.g. "1h", "30m"
	Previous  bool   `json:"previous,omitempty"` // show previous container logs
}

// applyInput is the input for kubectl_apply.
type applyInput struct {
	Manifest  string `json:"manifest"` // YAML or JSON manifest content
	Namespace string `json:"namespace,omitempty"`
	Context   string `json:"context,omitempty"`
	DryRun    string `json:"dry_run,omitempty"` // "none", "client", "server"
}

// deleteInput is the input for kubectl_delete.
type deleteInput struct {
	Resource  string `json:"resource"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Context   string `json:"context,omitempty"`
	Force     bool   `json:"force,omitempty"`
}

// execInput is the input for kubectl_exec.
type execInput struct {
	Pod       string   `json:"pod"`
	Command   []string `json:"command"`
	Container string   `json:"container,omitempty"`
	Namespace string   `json:"namespace,omitempty"`
	Context   string   `json:"context,omitempty"`
}

// contextInput is a minimal input for context-scoped commands.
type contextInput struct {
	Context string `json:"context,omitempty"`
}

// --- Tool constructors ---

func kubectlGet() tool.Tool {
	return tool.NewBuilder("kubectl_get").
		WithDescription("Get Kubernetes resources with structured JSON output").
		ReadOnly().
		Cacheable().
		WithHandler(handleGet).
		MustBuild()
}

func kubectlDescribe() tool.Tool {
	return tool.NewBuilder("kubectl_describe").
		WithDescription("Describe a Kubernetes resource with detailed human-readable output").
		ReadOnly().
		WithHandler(handleDescribe).
		MustBuild()
}

func kubectlLogs() tool.Tool {
	return tool.NewBuilder("kubectl_logs").
		WithDescription("Get logs from a pod (supports container selection and tail)").
		ReadOnly().
		WithHandler(handleLogs).
		MustBuild()
}

func kubectlApply() tool.Tool {
	return tool.NewBuilder("kubectl_apply").
		WithDescription("Apply a YAML/JSON manifest to the Kubernetes cluster").
		Destructive().
		Idempotent().
		WithHandler(handleApply).
		MustBuild()
}

func kubectlDelete() tool.Tool {
	return tool.NewBuilder("kubectl_delete").
		WithDescription("Delete a Kubernetes resource").
		Destructive().
		WithHandler(handleDelete).
		MustBuild()
}

func kubectlExec() tool.Tool {
	return tool.NewBuilder("kubectl_exec").
		WithDescription("Execute a command in a pod container").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithHandler(handleExec).
		MustBuild()
}

// kubectlPortForward returns a tool that documents port-forward as unsupported.
// Port forwarding is a long-running streaming operation that is incompatible
// with the request/response tool execution model. Use kubectl_exec with
// network diagnostic commands, or configure Kubernetes Services/Ingresses
// for connectivity instead.
func kubectlPortForward() tool.Tool {
	return tool.NewBuilder("kubectl_port_forward").
		WithDescription("NOT SUPPORTED: port-forward is a long-running operation incompatible with tool execution. Use Services or Ingresses instead.").
		WithRiskLevel(tool.RiskLow).
		WithHandler(handlePortForward).
		MustBuild()
}

func kubectlGetContexts() tool.Tool {
	return tool.NewBuilder("kubectl_get_contexts").
		WithDescription("List available kubeconfig contexts").
		ReadOnly().
		Cacheable().
		WithHandler(handleGetContexts).
		MustBuild()
}

func kubectlGetNamespaces() tool.Tool {
	return tool.NewBuilder("kubectl_get_namespaces").
		WithDescription("List all namespaces with structured JSON output").
		ReadOnly().
		Cacheable().
		WithHandler(handleGetNamespaces).
		MustBuild()
}

// --- Handlers ---

func handleGet(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in getInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parsing input: %w", err)
	}
	if in.Resource == "" {
		return tool.Result{}, fmt.Errorf("resource is required")
	}

	args := []string{"get", in.Resource}
	if in.Name != "" {
		args = append(args, in.Name)
	}
	args = append(args, "-o", "json")
	args = appendNamespace(args, in.Namespace)
	args = appendContext(args, in.Context)
	if in.Selector != "" {
		args = append(args, "-l", in.Selector)
	}
	if in.AllNS {
		args = append(args, "--all-namespaces")
	}

	return runKubectl(ctx, args)
}

func handleDescribe(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in describeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parsing input: %w", err)
	}
	if in.Resource == "" {
		return tool.Result{}, fmt.Errorf("resource is required")
	}
	if in.Name == "" {
		return tool.Result{}, fmt.Errorf("name is required")
	}

	args := []string{"describe", in.Resource, in.Name}
	args = appendNamespace(args, in.Namespace)
	args = appendContext(args, in.Context)

	return runKubectl(ctx, args)
}

func handleLogs(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in logsInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parsing input: %w", err)
	}
	if in.Pod == "" {
		return tool.Result{}, fmt.Errorf("pod is required")
	}

	args := []string{"logs", in.Pod}
	if in.Container != "" {
		args = append(args, "-c", in.Container)
	}
	if in.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", in.Tail))
	}
	if in.Since != "" {
		args = append(args, "--since", in.Since)
	}
	if in.Previous {
		args = append(args, "--previous")
	}
	args = appendNamespace(args, in.Namespace)
	args = appendContext(args, in.Context)

	return runKubectl(ctx, args)
}

func handleApply(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in applyInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parsing input: %w", err)
	}
	if in.Manifest == "" {
		return tool.Result{}, fmt.Errorf("manifest is required")
	}

	args := []string{"apply", "-f", "-"}
	args = appendNamespace(args, in.Namespace)
	args = appendContext(args, in.Context)
	if in.DryRun != "" && in.DryRun != "none" {
		args = append(args, "--dry-run="+in.DryRun)
	}

	return runKubectlWithStdin(ctx, args, in.Manifest)
}

func handleDelete(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in deleteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parsing input: %w", err)
	}
	if in.Resource == "" {
		return tool.Result{}, fmt.Errorf("resource is required")
	}
	if in.Name == "" {
		return tool.Result{}, fmt.Errorf("name is required")
	}

	args := []string{"delete", in.Resource, in.Name}
	args = appendNamespace(args, in.Namespace)
	args = appendContext(args, in.Context)
	if in.Force {
		args = append(args, "--force", "--grace-period=0")
	}

	return runKubectl(ctx, args)
}

func handleExec(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in execInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("parsing input: %w", err)
	}
	if in.Pod == "" {
		return tool.Result{}, fmt.Errorf("pod is required")
	}
	if len(in.Command) == 0 {
		return tool.Result{}, fmt.Errorf("command is required")
	}

	args := []string{"exec", in.Pod}
	if in.Container != "" {
		args = append(args, "-c", in.Container)
	}
	args = appendNamespace(args, in.Namespace)
	args = appendContext(args, in.Context)
	args = append(args, "--")
	args = append(args, in.Command...)

	return runKubectl(ctx, args)
}

func handlePortForward(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	msg := map[string]string{
		"error":       "unsupported",
		"description": "kubectl port-forward is a long-running streaming operation that is incompatible with the synchronous tool execution model",
		"alternatives": "Use kubectl_exec with network diagnostic commands (curl, wget, nc), " +
			"or configure Kubernetes Services/Ingresses for connectivity",
	}
	out, _ := json.Marshal(msg)
	return tool.NewResult(out), fmt.Errorf("kubectl_port_forward is not supported: port-forward is a long-running operation incompatible with tool execution")
}

func handleGetContexts(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	// contextInput is optional; ignore parse errors on empty input.
	var in contextInput
	if len(input) > 0 {
		_ = json.Unmarshal(input, &in)
	}
	_ = in // context field is not used for get-contexts itself

	args := []string{"config", "get-contexts", "-o", "name"}
	return runKubectl(ctx, args)
}

func handleGetNamespaces(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in contextInput
	if len(input) > 0 {
		_ = json.Unmarshal(input, &in)
	}

	args := []string{"get", "namespaces", "-o", "json"}
	args = appendContext(args, in.Context)

	return runKubectl(ctx, args)
}

// --- Helpers ---

// appendNamespace adds --namespace flag if namespace is non-empty.
func appendNamespace(args []string, ns string) []string {
	if ns != "" {
		args = append(args, "--namespace", ns)
	}
	return args
}

// appendContext adds --context flag if context is non-empty.
func appendContext(args []string, ctx string) []string {
	if ctx != "" {
		args = append(args, "--context", ctx)
	}
	return args
}

// kubectlPath resolves the kubectl binary. It can be overridden in tests.
var kubectlPath = resolveKubectl

func resolveKubectl() (string, error) {
	return exec.LookPath("kubectl")
}

// runKubectl executes a kubectl command and returns the output as a tool.Result.
func runKubectl(ctx context.Context, args []string) (tool.Result, error) {
	return runKubectlWithStdin(ctx, args, "")
}

// runKubectlWithStdin executes a kubectl command with optional stdin and returns
// the output as a tool.Result.
func runKubectlWithStdin(ctx context.Context, args []string, stdin string) (tool.Result, error) {
	start := time.Now()

	bin, err := kubectlPath()
	if err != nil {
		return tool.Result{}, fmt.Errorf("kubectl not found on PATH: %w", err)
	}

	// Apply timeout if context has no deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		// Include stderr/stdout in the error for diagnostic value.
		errMsg := map[string]string{
			"error":   err.Error(),
			"output":  string(output),
			"command": "kubectl " + strings.Join(args, " "),
		}
		errJSON, _ := json.Marshal(errMsg)
		return tool.NewResultWithDuration(errJSON, duration),
			fmt.Errorf("kubectl command failed: %w", err)
	}

	// If output is valid JSON, pass it through directly.
	// Otherwise, wrap it as a string in a JSON object.
	trimmed := strings.TrimSpace(string(output))
	if json.Valid([]byte(trimmed)) {
		return tool.NewResultWithDuration(json.RawMessage(trimmed), duration), nil
	}

	wrapped := map[string]string{"output": trimmed}
	wrappedJSON, _ := json.Marshal(wrapped)
	return tool.NewResultWithDuration(wrappedJSON, duration), nil
}
