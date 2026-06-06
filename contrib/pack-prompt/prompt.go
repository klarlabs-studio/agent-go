// Package prompt provides LLM prompt utilities for agents.
package prompt

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the prompt tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("prompt").
		WithDescription("LLM prompt utilities").
		AddTools(
			templateTool(),
			formatTool(),
			chainTool(),
			fewShotTool(),
			systemTool(),
			chatTool(),
			extractVarsTool(),
			validateTool(),
			compressTool(),
			countTokensTool(),
			splitTool(),
			roleTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func templateTool() tool.Tool {
	return tool.NewBuilder("prompt_template").
		WithDescription("Create a prompt from a template").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Template  string         `json:"template"`
				Variables map[string]any `json:"variables,omitempty"`
				Prefix    string         `json:"prefix,omitempty"` // Default {{
				Suffix    string         `json:"suffix,omitempty"` // Default }}
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			prefix := params.Prefix
			if prefix == "" {
				prefix = "{{"
			}
			suffix := params.Suffix
			if suffix == "" {
				suffix = "}}"
			}

			prompt := params.Template

			// Replace variables
			for key, value := range params.Variables {
				placeholder := prefix + key + suffix
				valueStr := fmt.Sprintf("%v", value)
				prompt = strings.ReplaceAll(prompt, placeholder, valueStr)
			}

			// Find unreplaced variables
			pattern := regexp.MustCompile(regexp.QuoteMeta(prefix) + `(\w+)` + regexp.QuoteMeta(suffix))
			unreplaced := pattern.FindAllStringSubmatch(prompt, -1)
			var missing []string
			for _, match := range unreplaced {
				missing = append(missing, match[1])
			}

			result := map[string]any{
				"prompt":   prompt,
				"template": params.Template,
				"missing":  missing,
				"complete": len(missing) == 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatTool() tool.Tool {
	return tool.NewBuilder("prompt_format").
		WithDescription("Format prompt for different LLM providers").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
				Format string `json:"format"` // openai, anthropic, llama, mistral
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var formatted string

			switch strings.ToLower(params.Format) {
			case "openai":
				// Already in correct format
				msgJSON, _ := json.Marshal(params.Messages)
				formatted = string(msgJSON)

			case "anthropic":
				// Anthropic format with Human/Assistant
				var sb strings.Builder
				for _, msg := range params.Messages {
					switch msg.Role {
					case "system":
						// System goes in separate parameter for Anthropic
						continue
					case "user":
						sb.WriteString("\n\nHuman: ")
						sb.WriteString(msg.Content)
					case "assistant":
						sb.WriteString("\n\nAssistant: ")
						sb.WriteString(msg.Content)
					}
				}
				sb.WriteString("\n\nAssistant:")
				formatted = strings.TrimPrefix(sb.String(), "\n\n")

			case "llama", "llama2":
				// Llama 2 chat format
				var sb strings.Builder
				for _, msg := range params.Messages {
					switch msg.Role {
					case "system":
						sb.WriteString("[INST] <<SYS>>\n")
						sb.WriteString(msg.Content)
						sb.WriteString("\n<</SYS>>\n\n")
					case "user":
						if !strings.Contains(sb.String(), "[INST]") {
							sb.WriteString("[INST] ")
						}
						sb.WriteString(msg.Content)
						sb.WriteString(" [/INST]")
					case "assistant":
						sb.WriteString(" ")
						sb.WriteString(msg.Content)
						sb.WriteString(" ")
					}
				}
				formatted = sb.String()

			case "mistral":
				// Mistral format
				var sb strings.Builder
				for _, msg := range params.Messages {
					switch msg.Role {
					case "user":
						sb.WriteString("[INST] ")
						sb.WriteString(msg.Content)
						sb.WriteString(" [/INST]")
					case "assistant":
						sb.WriteString(msg.Content)
						sb.WriteString("</s>")
					}
				}
				formatted = sb.String()

			default:
				// Generic format
				var sb strings.Builder
				for _, msg := range params.Messages {
					sb.WriteString(strings.ToUpper(msg.Role))
					sb.WriteString(": ")
					sb.WriteString(msg.Content)
					sb.WriteString("\n\n")
				}
				formatted = strings.TrimSpace(sb.String())
			}

			result := map[string]any{
				"formatted":     formatted,
				"format":        params.Format,
				"message_count": len(params.Messages),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func chainTool() tool.Tool {
	return tool.NewBuilder("prompt_chain").
		WithDescription("Chain multiple prompts together").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Prompts   []string `json:"prompts"`
				Separator string   `json:"separator,omitempty"`
				Prefix    string   `json:"prefix,omitempty"`
				Suffix    string   `json:"suffix,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			separator := params.Separator
			if separator == "" {
				separator = "\n\n"
			}

			chained := strings.Join(params.Prompts, separator)
			if params.Prefix != "" {
				chained = params.Prefix + separator + chained
			}
			if params.Suffix != "" {
				chained = chained + separator + params.Suffix
			}

			result := map[string]any{
				"prompt": chained,
				"parts":  len(params.Prompts),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fewShotTool() tool.Tool {
	return tool.NewBuilder("prompt_few_shot").
		WithDescription("Create a few-shot prompt with examples").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Task     string `json:"task"`
				Examples []struct {
					Input  string `json:"input"`
					Output string `json:"output"`
				} `json:"examples"`
				Query       string `json:"query"`
				InputLabel  string `json:"input_label,omitempty"`
				OutputLabel string `json:"output_label,omitempty"`
				Separator   string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			inputLabel := params.InputLabel
			if inputLabel == "" {
				inputLabel = "Input"
			}
			outputLabel := params.OutputLabel
			if outputLabel == "" {
				outputLabel = "Output"
			}
			separator := params.Separator
			if separator == "" {
				separator = "\n\n"
			}

			var sb strings.Builder

			// Task description
			if params.Task != "" {
				sb.WriteString(params.Task)
				sb.WriteString(separator)
			}

			// Examples
			for i, ex := range params.Examples {
				if i > 0 {
					sb.WriteString(separator)
				}
				sb.WriteString(inputLabel)
				sb.WriteString(": ")
				sb.WriteString(ex.Input)
				sb.WriteString("\n")
				sb.WriteString(outputLabel)
				sb.WriteString(": ")
				sb.WriteString(ex.Output)
			}

			// Query
			sb.WriteString(separator)
			sb.WriteString(inputLabel)
			sb.WriteString(": ")
			sb.WriteString(params.Query)
			sb.WriteString("\n")
			sb.WriteString(outputLabel)
			sb.WriteString(": ")

			result := map[string]any{
				"prompt":        sb.String(),
				"example_count": len(params.Examples),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func systemTool() tool.Tool {
	return tool.NewBuilder("prompt_system").
		WithDescription("Create a system prompt").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Role         string   `json:"role,omitempty"`
				Personality  string   `json:"personality,omitempty"`
				Instructions []string `json:"instructions,omitempty"`
				Constraints  []string `json:"constraints,omitempty"`
				Context      string   `json:"context,omitempty"`
				Format       string   `json:"output_format,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var sb strings.Builder

			if params.Role != "" {
				sb.WriteString("You are ")
				sb.WriteString(params.Role)
				sb.WriteString(".\n\n")
			}

			if params.Personality != "" {
				sb.WriteString(params.Personality)
				sb.WriteString("\n\n")
			}

			if len(params.Instructions) > 0 {
				sb.WriteString("Instructions:\n")
				for _, inst := range params.Instructions {
					sb.WriteString("- ")
					sb.WriteString(inst)
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}

			if len(params.Constraints) > 0 {
				sb.WriteString("Constraints:\n")
				for _, con := range params.Constraints {
					sb.WriteString("- ")
					sb.WriteString(con)
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}

			if params.Context != "" {
				sb.WriteString("Context:\n")
				sb.WriteString(params.Context)
				sb.WriteString("\n\n")
			}

			if params.Format != "" {
				sb.WriteString("Output Format:\n")
				sb.WriteString(params.Format)
				sb.WriteString("\n")
			}

			result := map[string]any{
				"system_prompt": strings.TrimSpace(sb.String()),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func chatTool() tool.Tool {
	return tool.NewBuilder("prompt_chat").
		WithDescription("Build chat messages").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				System   string `json:"system,omitempty"`
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages,omitempty"`
				User string `json:"user,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var messages []map[string]string

			if params.System != "" {
				messages = append(messages, map[string]string{
					"role":    "system",
					"content": params.System,
				})
			}

			for _, msg := range params.Messages {
				messages = append(messages, map[string]string{
					"role":    msg.Role,
					"content": msg.Content,
				})
			}

			if params.User != "" {
				messages = append(messages, map[string]string{
					"role":    "user",
					"content": params.User,
				})
			}

			result := map[string]any{
				"messages": messages,
				"count":    len(messages),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractVarsTool() tool.Tool {
	return tool.NewBuilder("prompt_extract_vars").
		WithDescription("Extract variables from a prompt template").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Template string `json:"template"`
				Prefix   string `json:"prefix,omitempty"`
				Suffix   string `json:"suffix,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			prefix := params.Prefix
			if prefix == "" {
				prefix = "{{"
			}
			suffix := params.Suffix
			if suffix == "" {
				suffix = "}}"
			}

			pattern := regexp.MustCompile(regexp.QuoteMeta(prefix) + `(\w+)` + regexp.QuoteMeta(suffix))
			matches := pattern.FindAllStringSubmatch(params.Template, -1)

			seen := make(map[string]bool)
			var variables []string
			for _, match := range matches {
				varName := match[1]
				if !seen[varName] {
					variables = append(variables, varName)
					seen[varName] = true
				}
			}

			result := map[string]any{
				"variables": variables,
				"count":     len(variables),
				"pattern":   prefix + "VAR" + suffix,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("prompt_validate").
		WithDescription("Validate prompt for common issues").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Prompt   string `json:"prompt"`
				MaxChars int    `json:"max_chars,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var issues []string
			var warnings []string

			// Check length
			charCount := len(params.Prompt)
			if params.MaxChars > 0 && charCount > params.MaxChars {
				issues = append(issues, fmt.Sprintf("Prompt exceeds max chars (%d > %d)", charCount, params.MaxChars))
			}

			// Check for empty
			if strings.TrimSpace(params.Prompt) == "" {
				issues = append(issues, "Prompt is empty or whitespace only")
			}

			// Check for unbalanced brackets
			openBrackets := strings.Count(params.Prompt, "{{")
			closeBrackets := strings.Count(params.Prompt, "}}")
			if openBrackets != closeBrackets {
				issues = append(issues, fmt.Sprintf("Unbalanced template brackets (%d opens, %d closes)", openBrackets, closeBrackets))
			}

			// Check for potential injection patterns
			if strings.Contains(params.Prompt, "ignore previous") ||
				strings.Contains(params.Prompt, "disregard") ||
				strings.Contains(params.Prompt, "forget your instructions") {
				warnings = append(warnings, "Prompt contains potential injection patterns")
			}

			// Estimate tokens (~4 chars per token)
			estimatedTokens := charCount / 4

			result := map[string]any{
				"valid":            len(issues) == 0,
				"issues":           issues,
				"warnings":         warnings,
				"char_count":       charCount,
				"estimated_tokens": estimatedTokens,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compressTool() tool.Tool {
	return tool.NewBuilder("prompt_compress").
		WithDescription("Compress prompt by removing redundancy").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			original := params.Prompt
			compressed := original

			// Remove multiple spaces
			compressed = regexp.MustCompile(`\s+`).ReplaceAllString(compressed, " ")

			// Remove multiple newlines
			compressed = regexp.MustCompile(`\n{3,}`).ReplaceAllString(compressed, "\n\n")

			// Trim each line
			lines := strings.Split(compressed, "\n")
			for i, line := range lines {
				lines[i] = strings.TrimSpace(line)
			}
			compressed = strings.Join(lines, "\n")

			// Remove empty bullet points
			compressed = regexp.MustCompile(`(?m)^[-*]\s*$`).ReplaceAllString(compressed, "")

			compressed = strings.TrimSpace(compressed)

			result := map[string]any{
				"compressed":      compressed,
				"original_length": len(original),
				"new_length":      len(compressed),
				"reduction":       float64(len(original)-len(compressed)) / float64(len(original)) * 100,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countTokensTool() tool.Tool {
	return tool.NewBuilder("prompt_count_tokens").
		WithDescription("Estimate token count for prompt").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Prompt string `json:"prompt"`
				Model  string `json:"model,omitempty"` // gpt-3.5, gpt-4, claude, etc.
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			charCount := len(params.Prompt)
			wordCount := len(strings.Fields(params.Prompt))

			// Different models have different tokenization
			var tokensPerChar float64
			switch strings.ToLower(params.Model) {
			case "gpt-3.5", "gpt-4", "gpt-4o":
				tokensPerChar = 0.25 // ~4 chars per token
			case "claude", "claude-3":
				tokensPerChar = 0.28 // Slightly different tokenization
			default:
				tokensPerChar = 0.25
			}

			estimatedTokens := int(float64(charCount) * tokensPerChar)
			tokensFromWords := int(float64(wordCount) * 1.3)

			// Average of both methods
			avgEstimate := (estimatedTokens + tokensFromWords) / 2

			result := map[string]any{
				"estimated_tokens": avgEstimate,
				"by_chars":         estimatedTokens,
				"by_words":         tokensFromWords,
				"char_count":       charCount,
				"word_count":       wordCount,
				"model":            params.Model,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func splitTool() tool.Tool {
	return tool.NewBuilder("prompt_split").
		WithDescription("Split prompt into chunks by token limit").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Prompt    string `json:"prompt"`
				MaxTokens int    `json:"max_tokens,omitempty"`
				Overlap   int    `json:"overlap_tokens,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			maxTokens := params.MaxTokens
			if maxTokens <= 0 {
				maxTokens = 2000
			}

			overlap := params.Overlap
			if overlap < 0 {
				overlap = 0
			}

			// Estimate ~4 chars per token
			charsPerChunk := maxTokens * 4
			overlapChars := overlap * 4
			step := charsPerChunk - overlapChars

			var chunks []map[string]any
			text := params.Prompt

			for i := 0; i < len(text); i += step {
				end := i + charsPerChunk
				if end > len(text) {
					end = len(text)
				}

				chunk := text[i:end]

				// Try to break at sentence boundary
				if end < len(text) {
					lastPeriod := strings.LastIndex(chunk, ". ")
					if lastPeriod > len(chunk)/2 {
						chunk = chunk[:lastPeriod+1]
						end = i + lastPeriod + 1
					}
				}

				chunks = append(chunks, map[string]any{
					"text":             chunk,
					"estimated_tokens": len(chunk) / 4,
					"start":            i,
					"end":              end,
				})

				if end >= len(text) {
					break
				}
			}

			result := map[string]any{
				"chunks":     chunks,
				"count":      len(chunks),
				"max_tokens": maxTokens,
				"overlap":    overlap,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func roleTool() tool.Tool {
	return tool.NewBuilder("prompt_role").
		WithDescription("Create role-playing prompt").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Role        string   `json:"role"`
				Expertise   []string `json:"expertise,omitempty"`
				Style       string   `json:"style,omitempty"`
				Limitations []string `json:"limitations,omitempty"`
				Task        string   `json:"task,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var sb strings.Builder

			sb.WriteString("You are ")
			sb.WriteString(params.Role)
			sb.WriteString(".")

			if len(params.Expertise) > 0 {
				sb.WriteString(" You have expertise in: ")
				sb.WriteString(strings.Join(params.Expertise, ", "))
				sb.WriteString(".")
			}

			if params.Style != "" {
				sb.WriteString(" ")
				sb.WriteString(params.Style)
			}

			if len(params.Limitations) > 0 {
				sb.WriteString("\n\nLimitations:\n")
				for _, lim := range params.Limitations {
					sb.WriteString("- ")
					sb.WriteString(lim)
					sb.WriteString("\n")
				}
			}

			if params.Task != "" {
				sb.WriteString("\n\nYour task: ")
				sb.WriteString(params.Task)
			}

			result := map[string]any{
				"prompt": sb.String(),
				"role":   params.Role,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
