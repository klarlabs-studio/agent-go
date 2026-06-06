// Package copilot provides code assistance tools for agent-go.
//
// This pack includes tools for AI-powered code assistance:
//   - copilot_complete: Get code completions
//   - copilot_explain: Get explanations for code snippets
//   - copilot_suggest: Get code suggestions for a task
//   - copilot_review: Get code review suggestions
//   - copilot_test: Generate test cases for code
//   - copilot_doc: Generate documentation for code
//
// Provider-agnostic via the CodeAssistant interface. Implement it for
// GitHub Copilot, OpenAI, Anthropic, or any compatible LLM backend.
// Respects rate limits and usage quotas.
package copilot

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Completion represents a code completion result.
type Completion struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence,omitempty"`
	Language   string  `json:"language,omitempty"`
}

// Explanation represents a code explanation result.
type Explanation struct {
	Summary  string   `json:"summary"`
	Details  string   `json:"details,omitempty"`
	Concepts []string `json:"concepts,omitempty"`
}

// Suggestion represents a code suggestion result.
type Suggestion struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Language    string `json:"language,omitempty"`
}

// ReviewComment represents a single code review comment.
type ReviewComment struct {
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity"` // "info", "warning", "error"
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

// TestCase represents a generated test case.
type TestCase struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
}

// Documentation represents generated documentation.
type Documentation struct {
	Content string `json:"content"`
	Format  string `json:"format"` // "markdown", "jsdoc", "godoc", etc.
}

// CodeAssistant is the provider-agnostic interface for code assistance.
type CodeAssistant interface {
	// Complete returns code completions for the given context.
	Complete(ctx context.Context, code, language, prompt string) ([]Completion, error)

	// Explain returns an explanation for the given code.
	Explain(ctx context.Context, code, language string) (*Explanation, error)

	// Suggest returns code suggestions for the given task.
	Suggest(ctx context.Context, task, language, context_ string) ([]Suggestion, error)

	// Review returns code review comments.
	Review(ctx context.Context, code, language string) ([]ReviewComment, error)

	// GenerateTests returns generated test cases.
	GenerateTests(ctx context.Context, code, language, framework string) ([]TestCase, error)

	// GenerateDoc returns generated documentation.
	GenerateDoc(ctx context.Context, code, language, format string) (*Documentation, error)
}

// Config holds code assistance configuration.
type Config struct {
	// Assistant is the CodeAssistant implementation. Required.
	Assistant CodeAssistant

	// DefaultLanguage is used when no language is specified.
	DefaultLanguage string

	// MaxCompletions limits completions returned (default: 5).
	MaxCompletions int
}

type copilotPack struct {
	cfg Config
}

// Pack returns the code assistance tools pack.
func Pack(cfg Config) *pack.Pack {
	if cfg.MaxCompletions == 0 {
		cfg.MaxCompletions = 5
	}

	p := &copilotPack{cfg: cfg}

	return pack.NewBuilder("copilot").
		WithDescription("AI-powered code assistance tools").
		WithVersion("0.1.0").
		AddTools(
			p.copilotComplete(),
			p.copilotExplain(),
			p.copilotSuggest(),
			p.copilotReview(),
			p.copilotTest(),
			p.copilotDoc(),
		).
		AllowInState(agent.StateExplore, "copilot_explain", "copilot_review").
		AllowInState(agent.StateAct, "copilot_complete", "copilot_explain", "copilot_suggest", "copilot_review", "copilot_test", "copilot_doc").
		AllowInState(agent.StateDecide, "copilot_suggest").
		Build()
}

func (p *copilotPack) copilotComplete() tool.Tool {
	return tool.NewBuilder("copilot_complete").
		WithDescription("Get code completions").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code     string `json:"code"`
				Language string `json:"language,omitempty"`
				Prompt   string `json:"prompt,omitempty"`
				Max      int    `json:"max,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Code == "" {
				return tool.Result{}, fmt.Errorf("code is required")
			}

			lang := params.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			completions, err := p.cfg.Assistant.Complete(ctx, params.Code, lang, params.Prompt)
			if err != nil {
				return tool.Result{}, fmt.Errorf("completion failed: %w", err)
			}

			max := params.Max
			if max <= 0 || max > p.cfg.MaxCompletions {
				max = p.cfg.MaxCompletions
			}
			if len(completions) > max {
				completions = completions[:max]
			}

			result := map[string]any{
				"completions": completions,
				"count":       len(completions),
				"language":    lang,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *copilotPack) copilotExplain() tool.Tool {
	return tool.NewBuilder("copilot_explain").
		WithDescription("Get explanations for code snippets").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code     string `json:"code"`
				Language string `json:"language,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Code == "" {
				return tool.Result{}, fmt.Errorf("code is required")
			}

			lang := params.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			explanation, err := p.cfg.Assistant.Explain(ctx, params.Code, lang)
			if err != nil {
				return tool.Result{}, fmt.Errorf("explain failed: %w", err)
			}

			result := map[string]any{
				"summary":  explanation.Summary,
				"details":  explanation.Details,
				"concepts": explanation.Concepts,
				"language": lang,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *copilotPack) copilotSuggest() tool.Tool {
	return tool.NewBuilder("copilot_suggest").
		WithDescription("Get code suggestions for implementing a task").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Task     string `json:"task"`
				Language string `json:"language,omitempty"`
				Context  string `json:"context,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Task == "" {
				return tool.Result{}, fmt.Errorf("task is required")
			}

			lang := params.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			suggestions, err := p.cfg.Assistant.Suggest(ctx, params.Task, lang, params.Context)
			if err != nil {
				return tool.Result{}, fmt.Errorf("suggest failed: %w", err)
			}

			result := map[string]any{
				"suggestions": suggestions,
				"count":       len(suggestions),
				"language":    lang,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *copilotPack) copilotReview() tool.Tool {
	return tool.NewBuilder("copilot_review").
		WithDescription("Get code review suggestions and improvements").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code     string `json:"code"`
				Language string `json:"language,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Code == "" {
				return tool.Result{}, fmt.Errorf("code is required")
			}

			lang := params.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			comments, err := p.cfg.Assistant.Review(ctx, params.Code, lang)
			if err != nil {
				return tool.Result{}, fmt.Errorf("review failed: %w", err)
			}

			var warnings, errors int
			for _, c := range comments {
				switch c.Severity {
				case "warning":
					warnings++
				case "error":
					errors++
				}
			}

			result := map[string]any{
				"comments": comments,
				"count":    len(comments),
				"warnings": warnings,
				"errors":   errors,
				"language": lang,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *copilotPack) copilotTest() tool.Tool {
	return tool.NewBuilder("copilot_test").
		WithDescription("Generate test cases for code").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code      string `json:"code"`
				Language  string `json:"language,omitempty"`
				Framework string `json:"framework,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Code == "" {
				return tool.Result{}, fmt.Errorf("code is required")
			}

			lang := params.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			tests, err := p.cfg.Assistant.GenerateTests(ctx, params.Code, lang, params.Framework)
			if err != nil {
				return tool.Result{}, fmt.Errorf("test generation failed: %w", err)
			}

			result := map[string]any{
				"tests":     tests,
				"count":     len(tests),
				"language":  lang,
				"framework": params.Framework,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *copilotPack) copilotDoc() tool.Tool {
	return tool.NewBuilder("copilot_doc").
		WithDescription("Generate documentation for code").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code     string `json:"code"`
				Language string `json:"language,omitempty"`
				Format   string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Code == "" {
				return tool.Result{}, fmt.Errorf("code is required")
			}

			lang := params.Language
			if lang == "" {
				lang = p.cfg.DefaultLanguage
			}

			format := params.Format
			if format == "" {
				format = "markdown"
			}

			doc, err := p.cfg.Assistant.GenerateDoc(ctx, params.Code, lang, format)
			if err != nil {
				return tool.Result{}, fmt.Errorf("doc generation failed: %w", err)
			}

			result := map[string]any{
				"content":  doc.Content,
				"format":   doc.Format,
				"language": lang,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
