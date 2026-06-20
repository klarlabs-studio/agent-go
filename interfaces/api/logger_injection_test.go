package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.klarlabs.de/agent/infrastructure/logging"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
	api "go.klarlabs.de/agent/interfaces/api"
	"go.klarlabs.de/bolt"
)

// TestWithLogger_DefaultChain_NoGlobalSink proves the release-blocker fix: a
// default-config engine built with api.WithLogger routes ALL execution-path
// logs (including the built-in Logging middleware that runs on every tool call)
// to the injected logger and NOTHING to the package-level global sink.
//
// The global sink is captured via a test seam so any leak is detectable; the
// injected logger writes to its own buffer. A real tool call exercises the
// Logging middleware, which previously called the global logging.Info()/Error().
func TestWithLogger_DefaultChain_NoGlobalSink(t *testing.T) {
	// Capture the package-level global sink. If ANY execution-path code reaches
	// logging.Get()/Info()/Error(), it lands here.
	var globalBuf bytes.Buffer
	restore := logging.SetGlobalSinkForTest(
		bolt.New(bolt.NewJSONHandler(&globalBuf)).SetLevel(bolt.TRACE),
	)
	defer restore()

	// Injected logger writes to its own buffer.
	var customBuf bytes.Buffer
	custom := api.NewLogger(bolt.New(bolt.NewJSONHandler(&customBuf)).SetLevel(bolt.TRACE))

	readTool := api.NewToolBuilder("read_file").
		WithDescription("reads a file").
		WithAnnotations(api.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (api.ToolResult, error) {
			return api.ToolResult{Output: json.RawMessage(`{"ok":true}`)}, nil
		}).
		MustBuild()

	eligibility := api.NewToolEligibility()
	eligibility.Allow(api.StateExplore, "read_file")

	// A run that actually invokes a tool so the Logging middleware fires.
	p := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "begin")},
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewCallToolDecision("read_file", json.RawMessage(`{}`), "gather")},
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "decide")},
		api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("done", json.RawMessage(`{}`))},
	)

	engine, err := api.New(
		api.WithTool(readTool),
		api.WithPlanner(p),
		api.WithToolEligibility(eligibility),
		api.WithEventStore(memory.NewEventStore()),
		api.WithLogger(custom),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	run, err := engine.Run(context.Background(), "logger injection")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.Status != api.StatusCompleted {
		t.Fatalf("run status = %s, want completed", run.Status)
	}

	customOut := customBuf.String()
	globalOut := globalBuf.String()

	// The injected logger must have captured execution-path logs, including the
	// per-tool-call Logging middleware line ("tool executed") and the engine's
	// run lifecycle logs ("run started" / "run completed").
	if customOut == "" {
		t.Fatal("injected logger captured nothing; expected execution-path logs")
	}
	for _, want := range []string{"run started", "run completed", "tool executed"} {
		if !strings.Contains(customOut, want) {
			t.Errorf("injected logger missing %q; got:\n%s", want, customOut)
		}
	}

	// NOTHING may reach the global sink from the execution path.
	if globalOut != "" {
		t.Errorf("execution path leaked to the global logging sink:\n%s", globalOut)
	}
}
