// Package wasm provides a WASM-based sandbox for tool execution using wazero.
//
// Tools that implement the WASMExecutor interface have their WASM modules
// loaded and executed with memory and time limits enforced by the wazero
// runtime. Tools without WASM modules fall back to direct execution.
//
// Usage:
//
//	sb, err := wasm.NewSandbox(sandbox.WithMaxMemory(64<<20), sandbox.WithMaxExecTime(30*time.Second))
//	defer sb.Close()
//
//	result, err := sb.Execute(ctx, myTool, input)
package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/security/sandbox"
)

// WASMExecutor is an optional interface tools can implement to provide
// pre-compiled WASM modules for sandboxed execution.
type WASMExecutor interface {
	// WASMModule returns the compiled .wasm bytes for this tool.
	WASMModule() []byte
}

// Sandbox implements sandbox.Sandbox using the wazero WASM runtime.
type Sandbox struct {
	runtime wazero.Runtime
	config  sandbox.Config
}

// NewSandbox creates a WASM sandbox with the given options.
func NewSandbox(opts ...sandbox.Option) (*Sandbox, error) {
	cfg := sandbox.Config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Configure wazero runtime
	rtConfig := wazero.NewRuntimeConfig()
	if cfg.MaxMemory > 0 {
		pages := uint32(cfg.MaxMemory / 65536) // WASM page = 64KB
		if pages == 0 {
			pages = 1
		}
		rtConfig = rtConfig.WithMemoryLimitPages(pages)
	}

	ctx := context.Background()
	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)

	// Instantiate WASI for stdin/stdout support
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("wasm sandbox: failed to instantiate WASI: %w", err)
	}

	return &Sandbox{
		runtime: rt,
		config:  cfg,
	}, nil
}

// Execute runs a tool in the WASM sandbox.
// If the tool implements WASMExecutor, its WASM module is loaded and executed.
// Otherwise, the tool is executed directly (fallback to no isolation).
func (s *Sandbox) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	// Check if tool provides a WASM module
	wasmExec, ok := t.(WASMExecutor)
	if !ok {
		// Fallback: direct execution (no WASM isolation)
		return t.Execute(ctx, input)
	}

	moduleBytes := wasmExec.WASMModule()
	if len(moduleBytes) == 0 {
		return t.Execute(ctx, input)
	}

	// Apply time limit
	if s.config.MaxExecTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.MaxExecTime)
		defer cancel()
	}

	// Compile the module
	compiled, err := s.runtime.CompileModule(ctx, moduleBytes)
	if err != nil {
		return tool.Result{}, fmt.Errorf("wasm sandbox: failed to compile module: %w", err)
	}
	defer func() { _ = compiled.Close(ctx) }()

	// Set up stdin/stdout for JSON I/O
	stdin := bytes.NewReader(input)
	var stdout bytes.Buffer

	// Configure module with WASI and filesystem restrictions
	modConfig := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(&bytes.Buffer{}).
		WithName(t.Name())

	// Apply filesystem restrictions
	if s.config.AllowFilesystem {
		if s.config.FSRoot != "" {
			modConfig = modConfig.WithFSConfig(
				wazero.NewFSConfig().WithDirMount(s.config.FSRoot, "/"),
			)
		}
	}

	// Instantiate and run the module
	mod, err := s.runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return tool.Result{}, fmt.Errorf("wasm sandbox: execution failed: %w", err)
	}
	defer func() { _ = mod.Close(ctx) }()

	// Parse output from stdout
	output := stdout.Bytes()
	if len(output) == 0 {
		output = json.RawMessage(`{"status":"completed"}`)
	}

	return tool.Result{
		Output: json.RawMessage(output),
	}, nil
}

// Capabilities returns what this sandbox allows based on configuration.
func (s *Sandbox) Capabilities() sandbox.Capabilities {
	return sandbox.Capabilities{
		Network:       false, // WASM has no network access by default
		Filesystem:    s.config.AllowFilesystem,
		MaxMemory:     s.config.MaxMemory,
		MaxExecTime:   s.config.MaxExecTime,
		AllowedEnv:    s.config.AllowedEnv,
		ReadOnlyPaths: s.config.ReadOnlyPaths,
		WritePaths:    s.config.WritePaths,
	}
}

// Close releases the wazero runtime and all compiled modules.
func (s *Sandbox) Close() error {
	return s.runtime.Close(context.Background())
}

// Ensure Sandbox implements sandbox.Sandbox.
var _ sandbox.Sandbox = (*Sandbox)(nil)
