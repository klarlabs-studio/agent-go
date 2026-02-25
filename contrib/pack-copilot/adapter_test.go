package copilot

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

func createTestTool(name string, readonly bool) tool.Tool {
	b := tool.NewBuilder(name).
		WithDescription("Test tool " + name).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: []byte(`{"result": "success"}`)}, nil
		})

	if readonly {
		b = b.ReadOnly()
	}

	return b.MustBuild()
}

func TestNewAdapter(t *testing.T) {
	t.Parallel()

	registry := memory.NewToolRegistry()
	adapter := NewAdapter(registry)

	if adapter == nil {
		t.Fatal("NewAdapter() returned nil")
	}

	if adapter.registry != registry {
		t.Error("adapter.registry not set correctly")
	}
}

func TestNewAdapter_WithOptions(t *testing.T) {
	t.Parallel()

	registry := memory.NewToolRegistry()
	filterCalled := false

	adapter := NewAdapter(registry,
		WithToolFilter(func(t tool.Tool) bool {
			filterCalled = true
			return true
		}),
	)

	// Register a tool to trigger filter
	_ = registry.Register(createTestTool("test", true))

	// GetCopilotTools should call the filter
	_ = adapter.GetCopilotTools()

	if !filterCalled {
		t.Error("tool filter was not called")
	}
}

func TestAdapter_GetCopilotTools(t *testing.T) {
	t.Parallel()

	registry := memory.NewToolRegistry()
	_ = registry.Register(createTestTool("tool1", true))
	_ = registry.Register(createTestTool("tool2", false))

	adapter := NewAdapter(registry)
	tools := adapter.GetCopilotTools()

	if len(tools) != 2 {
		t.Errorf("GetCopilotTools() returned %d tools, want 2", len(tools))
	}
}

func TestAdapter_GetCopilotTools_WithFilter(t *testing.T) {
	t.Parallel()

	registry := memory.NewToolRegistry()
	_ = registry.Register(createTestTool("readonly", true))
	_ = registry.Register(createTestTool("writable", false))

	adapter := NewAdapter(registry, WithToolFilter(OnlyReadOnly()))
	tools := adapter.GetCopilotTools()

	if len(tools) != 1 {
		t.Errorf("GetCopilotTools() returned %d tools, want 1", len(tools))
	}

	if tools[0].Name != "readonly" {
		t.Errorf("GetCopilotTools()[0].Name = %s, want readonly", tools[0].Name)
	}
}

func TestAdapter_CreateSessionConfig(t *testing.T) {
	t.Parallel()

	registry := memory.NewToolRegistry()
	_ = registry.Register(createTestTool("test", true))

	adapter := NewAdapter(registry)
	config := adapter.CreateSessionConfig("gpt-4", true)

	if config.Model != "gpt-4" {
		t.Errorf("config.Model = %s, want gpt-4", config.Model)
	}

	if !config.Streaming {
		t.Error("config.Streaming = false, want true")
	}

	if len(config.Tools) != 1 {
		t.Errorf("len(config.Tools) = %d, want 1", len(config.Tools))
	}
}

func TestConvertTool(t *testing.T) {
	t.Parallel()

	agentTool := createTestTool("test_tool", true)
	copilotTool := ConvertTool(agentTool)

	if copilotTool.Name != "test_tool" {
		t.Errorf("copilotTool.Name = %s, want test_tool", copilotTool.Name)
	}

	if copilotTool.Description != "Test tool test_tool" {
		t.Errorf("copilotTool.Description = %s, want 'Test tool test_tool'", copilotTool.Description)
	}

	if copilotTool.Handler == nil {
		t.Error("copilotTool.Handler is nil")
	}
}

func TestConvertTool_Handler(t *testing.T) {
	t.Parallel()

	agentTool := tool.NewBuilder("echo").
		WithDescription("Echo tool").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	copilotTool := ConvertTool(agentTool)

	invocation := ToolInvocation{
		ToolCallID: "test-123",
		ToolName:   "echo",
		Arguments:  map[string]string{"message": "hello"},
	}

	result, err := copilotTool.Handler(invocation)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if result.ResultType != "success" {
		t.Errorf("result.ResultType = %s, want success", result.ResultType)
	}

	if result.TextResultForLLM == "" {
		t.Error("result.TextResultForLLM is empty")
	}
}

func TestConvertTools(t *testing.T) {
	t.Parallel()

	agentTools := []tool.Tool{
		createTestTool("tool1", true),
		createTestTool("tool2", false),
	}

	copilotTools := ConvertTools(agentTools)

	if len(copilotTools) != 2 {
		t.Errorf("ConvertTools() returned %d tools, want 2", len(copilotTools))
	}
}

func TestConvertFromRegistry(t *testing.T) {
	t.Parallel()

	registry := memory.NewToolRegistry()
	_ = registry.Register(createTestTool("tool1", true))
	_ = registry.Register(createTestTool("tool2", false))

	copilotTools := ConvertFromRegistry(registry)

	if len(copilotTools) != 2 {
		t.Errorf("ConvertFromRegistry() returned %d tools, want 2", len(copilotTools))
	}
}

func TestOnlyReadOnly(t *testing.T) {
	t.Parallel()

	filter := OnlyReadOnly()

	readonlyTool := createTestTool("readonly", true)
	writableTool := createTestTool("writable", false)

	if !filter(readonlyTool) {
		t.Error("OnlyReadOnly() should accept readonly tool")
	}

	if filter(writableTool) {
		t.Error("OnlyReadOnly() should reject writable tool")
	}
}

func TestOnlyNonDestructive(t *testing.T) {
	t.Parallel()

	filter := OnlyNonDestructive()

	normalTool := createTestTool("normal", false)

	destructiveTool := tool.NewBuilder("destructive").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, nil
		}).
		MustBuild()

	if !filter(normalTool) {
		t.Error("OnlyNonDestructive() should accept non-destructive tool")
	}

	if filter(destructiveTool) {
		t.Error("OnlyNonDestructive() should reject destructive tool")
	}
}

func TestOnlyWithTags(t *testing.T) {
	t.Parallel()

	filter := OnlyWithTags("api", "network")

	taggedTool := tool.NewBuilder("tagged").
		WithTags("api").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, nil
		}).
		MustBuild()

	untaggedTool := createTestTool("untagged", true)

	if !filter(taggedTool) {
		t.Error("OnlyWithTags() should accept tool with matching tag")
	}

	if filter(untaggedTool) {
		t.Error("OnlyWithTags() should reject tool without matching tag")
	}
}

func TestResultToText(t *testing.T) {
	t.Parallel()

	t.Run("empty result", func(t *testing.T) {
		t.Parallel()
		result := tool.Result{}
		text := ResultToText(result)
		if text != "" {
			t.Errorf("ResultToText() = %q, want empty", text)
		}
	})

	t.Run("JSON result", func(t *testing.T) {
		t.Parallel()
		result := tool.Result{Output: []byte(`{"key":"value"}`)}
		text := ResultToText(result)
		if text == "" {
			t.Error("ResultToText() returned empty for JSON")
		}
	})

	t.Run("plain text result", func(t *testing.T) {
		t.Parallel()
		result := tool.Result{Output: []byte("plain text")}
		text := ResultToText(result)
		if text != "plain text" {
			t.Errorf("ResultToText() = %q, want 'plain text'", text)
		}
	})
}

func TestSessionHandler_RecordExecution(t *testing.T) {
	t.Parallel()

	handler := &SessionHandler{
		executions: make(map[string]*Execution),
	}

	exec := &Execution{
		ToolCallID: "test-123",
		ToolName:   "test_tool",
		Completed:  true,
	}

	handler.RecordExecution(exec)

	recorded, ok := handler.GetExecution("test-123")
	if !ok {
		t.Fatal("GetExecution() returned false")
	}

	if recorded.ToolName != "test_tool" {
		t.Errorf("recorded.ToolName = %s, want test_tool", recorded.ToolName)
	}
}

func TestSessionHandler_GetMetrics(t *testing.T) {
	t.Parallel()

	handler := &SessionHandler{
		executions: make(map[string]*Execution),
	}

	handler.RecordExecution(&Execution{
		ToolCallID: "1",
		ToolName:   "tool1",
		Completed:  true,
		Result:     &ToolResult{ResultType: "success"},
	})
	handler.RecordExecution(&Execution{
		ToolCallID: "2",
		ToolName:   "tool1",
		Completed:  true,
		Result:     &ToolResult{ResultType: "error"},
	})
	handler.RecordExecution(&Execution{
		ToolCallID: "3",
		ToolName:   "tool2",
		Completed:  true,
		Result:     &ToolResult{ResultType: "success"},
	})

	metrics := handler.GetMetrics()

	if metrics.TotalExecutions != 3 {
		t.Errorf("metrics.TotalExecutions = %d, want 3", metrics.TotalExecutions)
	}

	if metrics.SuccessfulExecutions != 2 {
		t.Errorf("metrics.SuccessfulExecutions = %d, want 2", metrics.SuccessfulExecutions)
	}

	if metrics.FailedExecutions != 1 {
		t.Errorf("metrics.FailedExecutions = %d, want 1", metrics.FailedExecutions)
	}

	if metrics.ToolCounts["tool1"] != 2 {
		t.Errorf("metrics.ToolCounts[tool1] = %d, want 2", metrics.ToolCounts["tool1"])
	}
}

func TestSessionHandler_Clear(t *testing.T) {
	t.Parallel()

	handler := &SessionHandler{
		executions: make(map[string]*Execution),
	}

	handler.RecordExecution(&Execution{ToolCallID: "1"})
	handler.Clear()

	if len(handler.GetExecutions()) != 0 {
		t.Error("Clear() did not remove executions")
	}
}

func TestMarshalArguments(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		result, err := marshalArguments(nil)
		if err != nil {
			t.Fatalf("marshalArguments(nil) error = %v", err)
		}
		if string(result) != "{}" {
			t.Errorf("marshalArguments(nil) = %s, want {}", string(result))
		}
	})

	t.Run("map", func(t *testing.T) {
		t.Parallel()
		result, err := marshalArguments(map[string]string{"key": "value"})
		if err != nil {
			t.Fatalf("marshalArguments() error = %v", err)
		}
		if result == nil {
			t.Error("marshalArguments() returned nil")
		}
	})

	t.Run("json string", func(t *testing.T) {
		t.Parallel()
		result, err := marshalArguments(`{"key": "value"}`)
		if err != nil {
			t.Fatalf("marshalArguments() error = %v", err)
		}
		if string(result) != `{"key": "value"}` {
			t.Errorf("marshalArguments() = %s, want original JSON", string(result))
		}
	})
}
