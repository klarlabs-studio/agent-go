// Package template provides tools for template rendering and text generation.
package template

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	texttemplate "text/template"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

type templatePack struct{}

// Pack creates a new template tools pack.
func Pack() *pack.Pack {
	p := &templatePack{}

	return pack.NewBuilder("template").
		WithDescription("Tools for template rendering and text generation").
		WithVersion("1.0.0").
		AddTools(
			// Go template tools
			p.renderTool(),
			p.renderFileTool(),
			p.renderHTMLTool(),
			p.validateTool(),
			// String interpolation tools
			p.interpolateTool(),
			p.formatTool(),
			// Text manipulation tools
			p.replaceTool(),
			p.replaceAllTool(),
			p.regexReplaceTool(),
			p.wrapTool(),
			p.trimTool(),
			p.padTool(),
			p.caseTool(),
			p.joinTool(),
			p.splitTool(),
			p.repeatTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Custom template functions
var templateFuncs = texttemplate.FuncMap{
	"upper":      strings.ToUpper,
	"lower":      strings.ToLower,
	"title":      strings.Title,
	"trim":       strings.TrimSpace,
	"trimPrefix": strings.TrimPrefix,
	"trimSuffix": strings.TrimSuffix,
	"replace":    strings.ReplaceAll,
	"split":      strings.Split,
	"join":       strings.Join,
	"contains":   strings.Contains,
	"hasPrefix":  strings.HasPrefix,
	"hasSuffix":  strings.HasSuffix,
	"repeat":     strings.Repeat,
	"default": func(def, val interface{}) interface{} {
		if val == nil || val == "" {
			return def
		}
		return val
	},
	"coalesce": func(vals ...interface{}) interface{} {
		for _, v := range vals {
			if v != nil && v != "" {
				return v
			}
		}
		return nil
	},
	"ternary": func(cond bool, t, f interface{}) interface{} {
		if cond {
			return t
		}
		return f
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"mul": func(a, b int) int { return a * b },
	"div": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"mod": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a % b
	},
	"seq": func(start, end int) []int {
		if end < start {
			return nil
		}
		result := make([]int, end-start+1)
		for i := range result {
			result[i] = start + i
		}
		return result
	},
	"list": func(args ...interface{}) []interface{} {
		return args
	},
	"dict": func(args ...interface{}) map[string]interface{} {
		result := make(map[string]interface{})
		for i := 0; i < len(args)-1; i += 2 {
			if key, ok := args[i].(string); ok {
				result[key] = args[i+1]
			}
		}
		return result
	},
	"first": func(list []interface{}) interface{} {
		if len(list) > 0 {
			return list[0]
		}
		return nil
	},
	"last": func(list []interface{}) interface{} {
		if len(list) > 0 {
			return list[len(list)-1]
		}
		return nil
	},
	"indent": func(spaces int, s string) string {
		pad := strings.Repeat(" ", spaces)
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = pad + line
			}
		}
		return strings.Join(lines, "\n")
	},
	"nindent": func(spaces int, s string) string {
		return "\n" + strings.Repeat(" ", spaces) + strings.ReplaceAll(s, "\n", "\n"+strings.Repeat(" ", spaces))
	},
	"quote": func(s string) string {
		return fmt.Sprintf("%q", s)
	},
	"squote": func(s string) string {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	},
}

// renderTool renders a Go text template.
func (p *templatePack) renderTool() tool.Tool {
	return tool.NewBuilder("template_render").
		WithDescription("Render a Go text template with data").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Template string                 `json:"template"`
				Data     map[string]interface{} `json:"data"`
				Name     string                 `json:"name,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Template == "" {
				return tool.Result{}, fmt.Errorf("template is required")
			}

			name := params.Name
			if name == "" {
				name = "template"
			}

			tmpl, err := texttemplate.New(name).Funcs(templateFuncs).Parse(params.Template)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse template: %w", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, params.Data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to render template: %w", err)
			}

			result := map[string]interface{}{
				"output": buf.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// renderFileTool renders a template from a file.
func (p *templatePack) renderFileTool() tool.Tool {
	return tool.NewBuilder("template_render_file").
		WithDescription("Render a Go text template from a file").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path   string                 `json:"path"`
				Data   map[string]interface{} `json:"data"`
				Output string                 `json:"output,omitempty"` // Optional output file
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			content, err := os.ReadFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read template file: %w", err)
			}

			tmpl, err := texttemplate.New(filepath.Base(params.Path)).Funcs(templateFuncs).Parse(string(content))
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse template: %w", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, params.Data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to render template: %w", err)
			}

			if params.Output != "" {
				if err := os.WriteFile(params.Output, buf.Bytes(), 0600); err != nil {
					return tool.Result{}, fmt.Errorf("failed to write output: %w", err)
				}
			}

			result := map[string]interface{}{
				"output": buf.String(),
			}
			if params.Output != "" {
				result["file"] = params.Output
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// renderHTMLTool renders an HTML template with auto-escaping.
func (p *templatePack) renderHTMLTool() tool.Tool {
	return tool.NewBuilder("template_render_html").
		WithDescription("Render an HTML template with auto-escaping").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Template string                 `json:"template"`
				Data     map[string]interface{} `json:"data"`
				Name     string                 `json:"name,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Template == "" {
				return tool.Result{}, fmt.Errorf("template is required")
			}

			name := params.Name
			if name == "" {
				name = "template"
			}

			// Convert text/template funcs to html/template funcs
			htmlFuncs := template.FuncMap{}
			for k, v := range templateFuncs {
				htmlFuncs[k] = v
			}

			tmpl, err := template.New(name).Funcs(htmlFuncs).Parse(params.Template)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse template: %w", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, params.Data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to render template: %w", err)
			}

			result := map[string]interface{}{
				"output": buf.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// validateTool validates template syntax.
func (p *templatePack) validateTool() tool.Tool {
	return tool.NewBuilder("template_validate").
		WithDescription("Validate Go template syntax").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Template string `json:"template"`
				HTML     bool   `json:"html,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Template == "" {
				return tool.Result{}, fmt.Errorf("template is required")
			}

			var parseErr error
			if params.HTML {
				htmlFuncs := template.FuncMap{}
				for k, v := range templateFuncs {
					htmlFuncs[k] = v
				}
				_, parseErr = template.New("validate").Funcs(htmlFuncs).Parse(params.Template)
			} else {
				_, parseErr = texttemplate.New("validate").Funcs(templateFuncs).Parse(params.Template)
			}

			if parseErr != nil {
				result := map[string]interface{}{
					"valid": false,
					"error": parseErr.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]interface{}{
				"valid": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// interpolateTool performs simple string interpolation.
func (p *templatePack) interpolateTool() tool.Tool {
	return tool.NewBuilder("template_interpolate").
		WithDescription("Perform simple ${var} or {{var}} string interpolation").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string                 `json:"text"`
				Variables map[string]interface{} `json:"variables"`
				Style     string                 `json:"style,omitempty"` // dollar, mustache
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Text == "" {
				return tool.Result{}, fmt.Errorf("text is required")
			}

			style := params.Style
			if style == "" {
				style = "dollar"
			}

			result := params.Text

			var pattern *regexp.Regexp
			switch style {
			case "dollar":
				pattern = regexp.MustCompile(`\$\{([^}]+)\}`)
			case "mustache":
				pattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)
			default:
				return tool.Result{}, fmt.Errorf("unsupported style: %s (use 'dollar' or 'mustache')", style)
			}

			result = pattern.ReplaceAllStringFunc(result, func(match string) string {
				submatch := pattern.FindStringSubmatch(match)
				if len(submatch) < 2 {
					return match
				}
				key := strings.TrimSpace(submatch[1])
				if val, ok := params.Variables[key]; ok {
					return fmt.Sprintf("%v", val)
				}
				return match
			})

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// formatTool formats a string using printf-style formatting.
func (p *templatePack) formatTool() tool.Tool {
	return tool.NewBuilder("template_format").
		WithDescription("Format a string using printf-style formatting").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Format string        `json:"format"`
				Args   []interface{} `json:"args"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Format == "" {
				return tool.Result{}, fmt.Errorf("format is required")
			}

			result := fmt.Sprintf(params.Format, params.Args...)

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// replaceTool replaces first occurrence.
func (p *templatePack) replaceTool() tool.Tool {
	return tool.NewBuilder("template_replace").
		WithDescription("Replace first occurrence of a substring").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text"`
				Old   string `json:"old"`
				New   string `json:"new"`
				Count int    `json:"count,omitempty"` // Number of replacements, -1 for all
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			count := params.Count
			if count == 0 {
				count = 1
			}

			result := strings.Replace(params.Text, params.Old, params.New, count)

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// replaceAllTool replaces all occurrences.
func (p *templatePack) replaceAllTool() tool.Tool {
	return tool.NewBuilder("template_replace_all").
		WithDescription("Replace all occurrences of a substring").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
				Old  string `json:"old"`
				New  string `json:"new"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			result := strings.ReplaceAll(params.Text, params.Old, params.New)

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
				"count":  strings.Count(params.Text, params.Old),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// regexReplaceTool replaces using regex.
func (p *templatePack) regexReplaceTool() tool.Tool {
	return tool.NewBuilder("template_regex_replace").
		WithDescription("Replace text using regular expression").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text        string `json:"text"`
				Pattern     string `json:"pattern"`
				Replacement string `json:"replacement"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			re, err := regexp.Compile(params.Pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			result := re.ReplaceAllString(params.Text, params.Replacement)

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// wrapTool wraps text at specified width.
func (p *templatePack) wrapTool() tool.Tool {
	return tool.NewBuilder("template_wrap").
		WithDescription("Wrap text at specified width").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text"`
				Width int    `json:"width"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Width <= 0 {
				params.Width = 80
			}

			words := strings.Fields(params.Text)
			if len(words) == 0 {
				output, _ := json.Marshal(map[string]interface{}{
					"output": "",
				})
				return tool.Result{Output: output}, nil
			}

			var lines []string
			var currentLine strings.Builder
			currentLine.WriteString(words[0])

			for _, word := range words[1:] {
				if currentLine.Len()+1+len(word) > params.Width {
					lines = append(lines, currentLine.String())
					currentLine.Reset()
					currentLine.WriteString(word)
				} else {
					currentLine.WriteString(" ")
					currentLine.WriteString(word)
				}
			}
			lines = append(lines, currentLine.String())

			result := strings.Join(lines, "\n")

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
				"lines":  len(lines),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// trimTool trims whitespace or specific characters.
func (p *templatePack) trimTool() tool.Tool {
	return tool.NewBuilder("template_trim").
		WithDescription("Trim whitespace or specific characters from text").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text"`
				Chars string `json:"chars,omitempty"`
				Left  bool   `json:"left,omitempty"`
				Right bool   `json:"right,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			result := params.Text

			if params.Chars == "" {
				if params.Left && !params.Right {
					result = strings.TrimLeft(result, " \t\n\r")
				} else if params.Right && !params.Left {
					result = strings.TrimRight(result, " \t\n\r")
				} else {
					result = strings.TrimSpace(result)
				}
			} else {
				if params.Left && !params.Right {
					result = strings.TrimLeft(result, params.Chars)
				} else if params.Right && !params.Left {
					result = strings.TrimRight(result, params.Chars)
				} else {
					result = strings.Trim(result, params.Chars)
				}
			}

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// padTool pads text to specified length.
func (p *templatePack) padTool() tool.Tool {
	return tool.NewBuilder("template_pad").
		WithDescription("Pad text to specified length").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text   string `json:"text"`
				Length int    `json:"length"`
				Char   string `json:"char,omitempty"`
				Left   bool   `json:"left,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Length <= 0 {
				return tool.Result{}, fmt.Errorf("length must be positive")
			}

			padChar := " "
			if params.Char != "" {
				padChar = params.Char[:1]
			}

			result := params.Text
			for len(result) < params.Length {
				if params.Left {
					result = padChar + result
				} else {
					result = result + padChar
				}
			}

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// caseTool converts text case.
func (p *templatePack) caseTool() tool.Tool {
	return tool.NewBuilder("template_case").
		WithDescription("Convert text case (upper, lower, title, camel, snake, kebab)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
				Case string `json:"case"` // upper, lower, title, camel, snake, kebab
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Case == "" {
				return tool.Result{}, fmt.Errorf("case is required")
			}

			var result string
			switch strings.ToLower(params.Case) {
			case "upper":
				result = strings.ToUpper(params.Text)
			case "lower":
				result = strings.ToLower(params.Text)
			case "title":
				result = strings.Title(strings.ToLower(params.Text))
			case "camel":
				result = toCamelCase(params.Text)
			case "snake":
				result = toSnakeCase(params.Text)
			case "kebab":
				result = toKebabCase(params.Text)
			default:
				return tool.Result{}, fmt.Errorf("unsupported case: %s", params.Case)
			}

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func toCamelCase(s string) string {
	words := splitWords(s)
	for i, word := range words {
		if i == 0 {
			words[i] = strings.ToLower(word)
		} else {
			words[i] = strings.Title(strings.ToLower(word))
		}
	}
	return strings.Join(words, "")
}

func toSnakeCase(s string) string {
	words := splitWords(s)
	for i, word := range words {
		words[i] = strings.ToLower(word)
	}
	return strings.Join(words, "_")
}

func toKebabCase(s string) string {
	words := splitWords(s)
	for i, word := range words {
		words[i] = strings.ToLower(word)
	}
	return strings.Join(words, "-")
}

func splitWords(s string) []string {
	// Split on spaces, underscores, hyphens, and camelCase boundaries
	re := regexp.MustCompile(`[A-Z][a-z]+|[a-z]+|[A-Z]+(?=[A-Z][a-z]|\b)|[0-9]+`)
	matches := re.FindAllString(s, -1)
	if len(matches) == 0 {
		return []string{s}
	}
	return matches
}

// joinTool joins array elements.
func (p *templatePack) joinTool() tool.Tool {
	return tool.NewBuilder("template_join").
		WithDescription("Join array elements with separator").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Items     []interface{} `json:"items"`
				Separator string        `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			sep := params.Separator
			if sep == "" {
				sep = ", "
			}

			strs := make([]string, len(params.Items))
			for i, item := range params.Items {
				strs[i] = fmt.Sprintf("%v", item)
			}

			result := strings.Join(strs, sep)

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// splitTool splits text into array.
func (p *templatePack) splitTool() tool.Tool {
	return tool.NewBuilder("template_split").
		WithDescription("Split text into array by separator or regex").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				Separator string `json:"separator,omitempty"`
				Regex     bool   `json:"regex,omitempty"`
				Limit     int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			sep := params.Separator
			if sep == "" {
				sep = " "
			}

			var parts []string
			if params.Regex {
				re, err := regexp.Compile(sep)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
				}
				if params.Limit > 0 {
					parts = re.Split(params.Text, params.Limit)
				} else {
					parts = re.Split(params.Text, -1)
				}
			} else {
				if params.Limit > 0 {
					parts = strings.SplitN(params.Text, sep, params.Limit)
				} else {
					parts = strings.Split(params.Text, sep)
				}
			}

			output, _ := json.Marshal(map[string]interface{}{
				"parts": parts,
				"count": len(parts),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// repeatTool repeats text.
func (p *templatePack) repeatTool() tool.Tool {
	return tool.NewBuilder("template_repeat").
		WithDescription("Repeat text a specified number of times").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				Count     int    `json:"count"`
				Separator string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Count <= 0 || params.Count > 10000 {
				return tool.Result{}, fmt.Errorf("count must be 1-10000")
			}

			var result string
			if params.Separator != "" {
				parts := make([]string, params.Count)
				for i := 0; i < params.Count; i++ {
					parts[i] = params.Text
				}
				result = strings.Join(parts, params.Separator)
			} else {
				result = strings.Repeat(params.Text, params.Count)
			}

			output, _ := json.Marshal(map[string]interface{}{
				"output": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
