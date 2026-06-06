// Package caseconv provides text case conversion tools for agents.
package caseconv

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

var (
	camelCaseSplit = regexp.MustCompile(`([a-z])([A-Z])`)
	wordBoundary   = regexp.MustCompile(`[\s_\-]+`)
)

// Pack returns the case conversion tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("case").
		WithDescription("Text case conversion tools").
		AddTools(
			camelCaseTool(),
			pascalCaseTool(),
			snakeCaseTool(),
			kebabCaseTool(),
			titleCaseTool(),
			upperCaseTool(),
			lowerCaseTool(),
			constantCaseTool(),
			sentenceCaseTool(),
			detectCaseTool(),
			convertTool(),
			splitWordsTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func camelCaseTool() tool.Tool {
	return tool.NewBuilder("case_camel").
		WithDescription("Convert to camelCase").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)
			if len(words) == 0 {
				result := map[string]any{
					"result": "",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var parts []string
			for i, word := range words {
				if i == 0 {
					parts = append(parts, strings.ToLower(word))
				} else {
					parts = append(parts, capitalize(word))
				}
			}

			result := map[string]any{
				"result": strings.Join(parts, ""),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pascalCaseTool() tool.Tool {
	return tool.NewBuilder("case_pascal").
		WithDescription("Convert to PascalCase").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)
			var parts []string
			for _, word := range words {
				parts = append(parts, capitalize(word))
			}

			result := map[string]any{
				"result": strings.Join(parts, ""),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func snakeCaseTool() tool.Tool {
	return tool.NewBuilder("case_snake").
		WithDescription("Convert to snake_case").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text"`
				Upper bool   `json:"upper,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)
			var parts []string
			for _, word := range words {
				if params.Upper {
					parts = append(parts, strings.ToUpper(word))
				} else {
					parts = append(parts, strings.ToLower(word))
				}
			}

			result := map[string]any{
				"result": strings.Join(parts, "_"),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func kebabCaseTool() tool.Tool {
	return tool.NewBuilder("case_kebab").
		WithDescription("Convert to kebab-case").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)
			var parts []string
			for _, word := range words {
				parts = append(parts, strings.ToLower(word))
			}

			result := map[string]any{
				"result": strings.Join(parts, "-"),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func titleCaseTool() tool.Tool {
	return tool.NewBuilder("case_title").
		WithDescription("Convert to Title Case").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)
			var parts []string
			for _, word := range words {
				parts = append(parts, capitalize(word))
			}

			result := map[string]any{
				"result": strings.Join(parts, " "),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func upperCaseTool() tool.Tool {
	return tool.NewBuilder("case_upper").
		WithDescription("Convert to UPPERCASE").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"result": strings.ToUpper(params.Text),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lowerCaseTool() tool.Tool {
	return tool.NewBuilder("case_lower").
		WithDescription("Convert to lowercase").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"result": strings.ToLower(params.Text),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func constantCaseTool() tool.Tool {
	return tool.NewBuilder("case_constant").
		WithDescription("Convert to CONSTANT_CASE").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)
			var parts []string
			for _, word := range words {
				parts = append(parts, strings.ToUpper(word))
			}

			result := map[string]any{
				"result": strings.Join(parts, "_"),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sentenceCaseTool() tool.Tool {
	return tool.NewBuilder("case_sentence").
		WithDescription("Convert to Sentence case").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)
			var parts []string
			for i, word := range words {
				if i == 0 {
					parts = append(parts, capitalize(word))
				} else {
					parts = append(parts, strings.ToLower(word))
				}
			}

			result := map[string]any{
				"result": strings.Join(parts, " "),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func detectCaseTool() tool.Tool {
	return tool.NewBuilder("case_detect").
		WithDescription("Detect case style of text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			text := params.Text
			var caseType string

			switch {
			case strings.Contains(text, "_") && text == strings.ToUpper(text):
				caseType = "constant"
			case strings.Contains(text, "_"):
				caseType = "snake"
			case strings.Contains(text, "-"):
				caseType = "kebab"
			case text == strings.ToUpper(text):
				caseType = "upper"
			case text == strings.ToLower(text):
				caseType = "lower"
			case len(text) > 0 && unicode.IsUpper(rune(text[0])) && !strings.Contains(text, " "):
				caseType = "pascal"
			case len(text) > 0 && unicode.IsLower(rune(text[0])) && camelCaseSplit.MatchString(text):
				caseType = "camel"
			case strings.Contains(text, " "):
				// Check if title case
				words := strings.Fields(text)
				isTitle := true
				for _, w := range words {
					if len(w) > 0 && !unicode.IsUpper(rune(w[0])) {
						isTitle = false
						break
					}
				}
				if isTitle {
					caseType = "title"
				} else {
					caseType = "sentence"
				}
			default:
				caseType = "unknown"
			}

			result := map[string]any{
				"text": params.Text,
				"case": caseType,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func convertTool() tool.Tool {
	return tool.NewBuilder("case_convert").
		WithDescription("Convert between case styles").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
				To   string `json:"to"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)
			var converted string

			switch params.To {
			case "camel":
				var parts []string
				for i, word := range words {
					if i == 0 {
						parts = append(parts, strings.ToLower(word))
					} else {
						parts = append(parts, capitalize(word))
					}
				}
				converted = strings.Join(parts, "")
			case "pascal":
				var parts []string
				for _, word := range words {
					parts = append(parts, capitalize(word))
				}
				converted = strings.Join(parts, "")
			case "snake":
				var parts []string
				for _, word := range words {
					parts = append(parts, strings.ToLower(word))
				}
				converted = strings.Join(parts, "_")
			case "kebab":
				var parts []string
				for _, word := range words {
					parts = append(parts, strings.ToLower(word))
				}
				converted = strings.Join(parts, "-")
			case "constant":
				var parts []string
				for _, word := range words {
					parts = append(parts, strings.ToUpper(word))
				}
				converted = strings.Join(parts, "_")
			case "title":
				var parts []string
				for _, word := range words {
					parts = append(parts, capitalize(word))
				}
				converted = strings.Join(parts, " ")
			case "sentence":
				var parts []string
				for i, word := range words {
					if i == 0 {
						parts = append(parts, capitalize(word))
					} else {
						parts = append(parts, strings.ToLower(word))
					}
				}
				converted = strings.Join(parts, " ")
			case "upper":
				converted = strings.ToUpper(params.Text)
			case "lower":
				converted = strings.ToLower(params.Text)
			default:
				result := map[string]any{
					"error": "unknown target case: " + params.To,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"result": converted,
				"from":   params.Text,
				"to":     params.To,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func splitWordsTool() tool.Tool {
	return tool.NewBuilder("case_split_words").
		WithDescription("Split text into words").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words := splitWords(params.Text)

			result := map[string]any{
				"text":  params.Text,
				"words": words,
				"count": len(words),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// splitWords splits text into words regardless of case style
func splitWords(text string) []string {
	// Handle camelCase and PascalCase
	text = camelCaseSplit.ReplaceAllString(text, "${1} ${2}")

	// Handle other separators
	text = wordBoundary.ReplaceAllString(text, " ")

	// Split and filter
	parts := strings.Fields(text)
	var words []string
	for _, p := range parts {
		if p != "" {
			words = append(words, p)
		}
	}
	return words
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
