package copilot

// SDK types for GitHub Copilot integration.
// These types mirror the GitHub Copilot SDK API. When the official
// github.com/github/copilot-sdk/go module is available, these can
// be replaced with the real SDK types.

// Tool represents a tool that can be used in a Copilot session.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Handler     ToolHandler            `json:"-"`
}

// ToolHandler is a function that handles a tool invocation.
type ToolHandler func(invocation ToolInvocation) (ToolResult, error)

// ToolInvocation represents a request to execute a tool.
type ToolInvocation struct {
	ToolCallID string      `json:"tool_call_id"`
	ToolName   string      `json:"tool_name"`
	Arguments  interface{} `json:"arguments"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	TextResultForLLM string `json:"text_result_for_llm,omitempty"`
	ResultType       string `json:"result_type"` // "success" or "error"
	Error            string `json:"error,omitempty"`
}

// Session represents a Copilot chat session.
type Session struct {
	Config *SessionConfig
}

// SessionConfig holds configuration for creating a Copilot session.
type SessionConfig struct {
	Model     string `json:"model"`
	Streaming bool   `json:"streaming"`
	Tools     []Tool `json:"tools,omitempty"`
}
