package copilot

import (
	"context"
	"encoding/json"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// ConvertTool converts an agent-go Tool to a Copilot SDK Tool.
// This enables agent-go tools to be used in Copilot sessions.
func ConvertTool(agentTool tool.Tool) Tool {
	return Tool{
		Name:        agentTool.Name(),
		Description: agentTool.Description(),
		Parameters:  schemaToParameters(agentTool.InputSchema()),
		Handler:     createHandler(agentTool),
	}
}

// ConvertTools converts multiple agent-go Tools to Copilot SDK Tools.
func ConvertTools(agentTools []tool.Tool) []Tool {
	result := make([]Tool, len(agentTools))
	for i, t := range agentTools {
		result[i] = ConvertTool(t)
	}
	return result
}

// ConvertFromRegistry converts all tools from a registry to Copilot SDK Tools.
func ConvertFromRegistry(registry tool.Registry) []Tool {
	return ConvertTools(registry.List())
}

// schemaToParameters converts a tool.Schema to Copilot parameters format.
// The Copilot SDK expects a map[string]interface{} representing the JSON Schema.
func schemaToParameters(schema tool.Schema) map[string]interface{} {
	if schema.IsEmpty() {
		return map[string]interface{}{
			"type": "object",
		}
	}

	var params map[string]interface{}
	if err := json.Unmarshal(schema.Raw(), &params); err != nil {
		return map[string]interface{}{
			"type": "object",
		}
	}

	return params
}

// createHandler creates a Copilot ToolHandler from an agent-go Tool.
func createHandler(agentTool tool.Tool) ToolHandler {
	return func(invocation ToolInvocation) (ToolResult, error) {
		input, err := json.Marshal(invocation.Arguments)
		if err != nil {
			return ToolResult{
				ResultType: "error",
				Error:      "failed to marshal arguments: " + err.Error(),
			}, nil
		}

		ctx := context.Background()
		result, err := agentTool.Execute(ctx, input)
		if err != nil {
			return ToolResult{
				ResultType: "error",
				Error:      err.Error(),
			}, nil
		}

		return ToolResult{
			TextResultForLLM: string(result.Output),
			ResultType:       "success",
		}, nil
	}
}

// ResultToText converts a tool.Result to a text representation for Copilot.
func ResultToText(result tool.Result) string {
	if len(result.Output) == 0 {
		return ""
	}

	var v interface{}
	if err := json.Unmarshal(result.Output, &v); err == nil {
		pretty, err := json.MarshalIndent(v, "", "  ")
		if err == nil {
			return string(pretty)
		}
	}

	return string(result.Output)
}
