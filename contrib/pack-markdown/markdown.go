// Package markdown provides Markdown processing tools for agents.
package markdown

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the markdown tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("markdown").
		WithDescription("Markdown processing and conversion tools").
		AddTools(
			renderHTMLTool(),
			extractLinksTool(),
			extractHeadingsTool(),
			extractCodeBlocksTool(),
			generateTOCTool(),
			stripMarkdownTool(),
			convertFromHTMLTool(),
			validateTool(),
			formatTableTool(),
			wrapCodeBlockTool(),
			extractImagesTool(),
			countWordsTool(),
			splitSectionsTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func renderHTMLTool() tool.Tool {
	return tool.NewBuilder("md_render_html").
		WithDescription("Convert Markdown to HTML").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown  string `json:"markdown"`
				GFM       bool   `json:"gfm,omitempty"`
				Unsafe    bool   `json:"unsafe,omitempty"`
				Hardwraps bool   `json:"hardwraps,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			opts := []goldmark.Option{
				goldmark.WithParserOptions(parser.WithAutoHeadingID()),
			}

			if params.GFM {
				opts = append(opts, goldmark.WithExtensions(extension.GFM))
			}

			if params.Unsafe {
				opts = append(opts, goldmark.WithRendererOptions(html.WithUnsafe()))
			}
			if params.Hardwraps {
				opts = append(opts, goldmark.WithRendererOptions(html.WithHardWraps()))
			}

			md := goldmark.New(opts...)
			var buf bytes.Buffer
			if err := md.Convert([]byte(params.Markdown), &buf); err != nil {
				return tool.Result{}, err
			}

			result := map[string]string{"html": buf.String()}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractLinksTool() tool.Tool {
	return tool.NewBuilder("md_extract_links").
		WithDescription("Extract all links from Markdown").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Match [text](url) and bare URLs
			linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
			bareURLRe := regexp.MustCompile(`https?://[^\s\)]+`)

			type Link struct {
				Text string `json:"text"`
				URL  string `json:"url"`
			}

			var links []Link
			seen := make(map[string]bool)

			for _, match := range linkRe.FindAllStringSubmatch(params.Markdown, -1) {
				url := match[2]
				if !seen[url] {
					links = append(links, Link{Text: match[1], URL: url})
					seen[url] = true
				}
			}

			for _, url := range bareURLRe.FindAllString(params.Markdown, -1) {
				if !seen[url] {
					links = append(links, Link{Text: "", URL: url})
					seen[url] = true
				}
			}

			result := map[string]any{"links": links, "count": len(links)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractHeadingsTool() tool.Tool {
	return tool.NewBuilder("md_extract_headings").
		WithDescription("Extract all headings from Markdown").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			type Heading struct {
				Level int    `json:"level"`
				Text  string `json:"text"`
				ID    string `json:"id"`
			}

			headingRe := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
			var headings []Heading

			for _, match := range headingRe.FindAllStringSubmatch(params.Markdown, -1) {
				level := len(match[1])
				text := strings.TrimSpace(match[2])
				id := strings.ToLower(regexp.MustCompile(`[^\w\s-]`).ReplaceAllString(text, ""))
				id = strings.ReplaceAll(id, " ", "-")

				headings = append(headings, Heading{Level: level, Text: text, ID: id})
			}

			result := map[string]any{"headings": headings, "count": len(headings)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractCodeBlocksTool() tool.Tool {
	return tool.NewBuilder("md_extract_code_blocks").
		WithDescription("Extract code blocks from Markdown").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
				Language string `json:"language,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			type CodeBlock struct {
				Language string `json:"language"`
				Code     string `json:"code"`
			}

			codeRe := regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
			var blocks []CodeBlock

			for _, match := range codeRe.FindAllStringSubmatch(params.Markdown, -1) {
				lang := match[1]
				code := match[2]

				if params.Language == "" || lang == params.Language {
					blocks = append(blocks, CodeBlock{Language: lang, Code: strings.TrimSuffix(code, "\n")})
				}
			}

			result := map[string]any{"code_blocks": blocks, "count": len(blocks)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateTOCTool() tool.Tool {
	return tool.NewBuilder("md_generate_toc").
		WithDescription("Generate table of contents from Markdown").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
				MaxLevel int    `json:"max_level,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.MaxLevel == 0 {
				params.MaxLevel = 6
			}

			headingRe := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
			var toc strings.Builder

			for _, match := range headingRe.FindAllStringSubmatch(params.Markdown, -1) {
				level := len(match[1])
				if level > params.MaxLevel {
					continue
				}

				text := strings.TrimSpace(match[2])
				id := strings.ToLower(regexp.MustCompile(`[^\w\s-]`).ReplaceAllString(text, ""))
				id = strings.ReplaceAll(id, " ", "-")

				indent := strings.Repeat("  ", level-1)
				toc.WriteString(indent + "- [" + text + "](#" + id + ")\n")
			}

			result := map[string]string{"toc": toc.String()}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func stripMarkdownTool() tool.Tool {
	return tool.NewBuilder("md_strip").
		WithDescription("Remove Markdown formatting, leaving plain text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			text := params.Markdown

			// Remove code blocks
			text = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(text, "")
			text = regexp.MustCompile("`[^`]+`").ReplaceAllString(text, "")

			// Remove images
			text = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`).ReplaceAllString(text, "$1")

			// Convert links to text
			text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "$1")

			// Remove emphasis
			text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1")
			text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
			text = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(text, "$1")
			text = regexp.MustCompile(`_([^_]+)_`).ReplaceAllString(text, "$1")
			text = regexp.MustCompile(`~~([^~]+)~~`).ReplaceAllString(text, "$1")

			// Remove headings
			text = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(text, "")

			// Remove blockquotes
			text = regexp.MustCompile(`(?m)^>\s+`).ReplaceAllString(text, "")

			// Remove horizontal rules
			text = regexp.MustCompile(`(?m)^[-*_]{3,}$`).ReplaceAllString(text, "")

			// Remove list markers
			text = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`).ReplaceAllString(text, "")
			text = regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`).ReplaceAllString(text, "")

			// Clean up multiple newlines
			text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
			text = strings.TrimSpace(text)

			result := map[string]string{"text": text}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func convertFromHTMLTool() tool.Tool {
	return tool.NewBuilder("md_from_html").
		WithDescription("Convert HTML to Markdown (basic conversion)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				HTML string `json:"html"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			md := params.HTML

			// Convert headers
			for i := 6; i >= 1; i-- {
				tag := strings.Repeat("#", i)
				md = regexp.MustCompile(`(?i)<h`+string(rune('0'+i))+`[^>]*>([^<]+)</h`+string(rune('0'+i))+`>`).
					ReplaceAllString(md, tag+" $1\n\n")
			}

			// Convert paragraphs
			md = regexp.MustCompile(`(?i)<p[^>]*>([^<]+)</p>`).ReplaceAllString(md, "$1\n\n")

			// Convert links
			md = regexp.MustCompile(`(?i)<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`).ReplaceAllString(md, "[$2]($1)")

			// Convert emphasis
			md = regexp.MustCompile(`(?i)<(strong|b)[^>]*>([^<]+)</\1>`).ReplaceAllString(md, "**$2**")
			md = regexp.MustCompile(`(?i)<(em|i)[^>]*>([^<]+)</\1>`).ReplaceAllString(md, "*$2*")

			// Convert code
			md = regexp.MustCompile(`(?i)<code[^>]*>([^<]+)</code>`).ReplaceAllString(md, "`$1`")
			md = regexp.MustCompile(`(?is)<pre[^>]*><code[^>]*>(.+?)</code></pre>`).ReplaceAllString(md, "```\n$1\n```\n")

			// Convert lists
			md = regexp.MustCompile(`(?i)<li[^>]*>([^<]+)</li>`).ReplaceAllString(md, "- $1\n")
			md = regexp.MustCompile(`(?i)</?[uo]l[^>]*>`).ReplaceAllString(md, "")

			// Convert line breaks
			md = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(md, "\n")

			// Remove remaining tags
			md = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(md, "")

			// Decode HTML entities
			md = strings.ReplaceAll(md, "&amp;", "&")
			md = strings.ReplaceAll(md, "&lt;", "<")
			md = strings.ReplaceAll(md, "&gt;", ">")
			md = strings.ReplaceAll(md, "&quot;", "\"")
			md = strings.ReplaceAll(md, "&#39;", "'")
			md = strings.ReplaceAll(md, "&nbsp;", " ")

			// Clean up whitespace
			md = regexp.MustCompile(`\n{3,}`).ReplaceAllString(md, "\n\n")
			md = strings.TrimSpace(md)

			result := map[string]string{"markdown": md}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("md_validate").
		WithDescription("Validate Markdown structure and report issues").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var issues []string

			// Check for unclosed code blocks
			if strings.Count(params.Markdown, "```")%2 != 0 {
				issues = append(issues, "Unclosed code block (odd number of ```)")
			}

			// Check for unbalanced brackets
			if strings.Count(params.Markdown, "[") != strings.Count(params.Markdown, "]") {
				issues = append(issues, "Unbalanced square brackets")
			}

			// Check for broken links
			linkRe := regexp.MustCompile(`\[([^\]]+)\]\(\s*\)`)
			if linkRe.MatchString(params.Markdown) {
				issues = append(issues, "Empty link URL found")
			}

			// Check for consecutive blank lines (more than 2)
			if regexp.MustCompile(`\n{4,}`).MatchString(params.Markdown) {
				issues = append(issues, "Excessive blank lines (more than 2 consecutive)")
			}

			result := map[string]any{
				"valid":  len(issues) == 0,
				"issues": issues,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatTableTool() tool.Tool {
	return tool.NewBuilder("md_format_table").
		WithDescription("Generate a formatted Markdown table").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Headers []string   `json:"headers"`
				Rows    [][]string `json:"rows"`
				Align   []string   `json:"align,omitempty"` // left, center, right
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Headers) == 0 {
				result := map[string]string{"table": ""}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Calculate column widths
			widths := make([]int, len(params.Headers))
			for i, h := range params.Headers {
				widths[i] = len(h)
			}
			for _, row := range params.Rows {
				for i, cell := range row {
					if i < len(widths) && len(cell) > widths[i] {
						widths[i] = len(cell)
					}
				}
			}

			var sb strings.Builder

			// Header row
			sb.WriteString("|")
			for i, h := range params.Headers {
				sb.WriteString(" " + padRight(h, widths[i]) + " |")
			}
			sb.WriteString("\n")

			// Separator row
			sb.WriteString("|")
			for i := range params.Headers {
				align := "left"
				if i < len(params.Align) {
					align = params.Align[i]
				}

				sep := strings.Repeat("-", widths[i])
				switch align {
				case "center":
					sb.WriteString(":" + sep + ":|")
				case "right":
					sb.WriteString(" " + sep + ":|")
				default:
					sb.WriteString(" " + sep + " |")
				}
			}
			sb.WriteString("\n")

			// Data rows
			for _, row := range params.Rows {
				sb.WriteString("|")
				for i := 0; i < len(params.Headers); i++ {
					cell := ""
					if i < len(row) {
						cell = row[i]
					}
					sb.WriteString(" " + padRight(cell, widths[i]) + " |")
				}
				sb.WriteString("\n")
			}

			result := map[string]string{"table": sb.String()}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func wrapCodeBlockTool() tool.Tool {
	return tool.NewBuilder("md_wrap_code").
		WithDescription("Wrap text in a Markdown code block").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code     string `json:"code"`
				Language string `json:"language,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			block := "```" + params.Language + "\n" + params.Code + "\n```"

			result := map[string]string{"markdown": block}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractImagesTool() tool.Tool {
	return tool.NewBuilder("md_extract_images").
		WithDescription("Extract all images from Markdown").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			type Image struct {
				Alt string `json:"alt"`
				URL string `json:"url"`
			}

			imageRe := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
			var images []Image

			for _, match := range imageRe.FindAllStringSubmatch(params.Markdown, -1) {
				images = append(images, Image{Alt: match[1], URL: match[2]})
			}

			result := map[string]any{"images": images, "count": len(images)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countWordsTool() tool.Tool {
	return tool.NewBuilder("md_count_words").
		WithDescription("Count words in Markdown (excluding code and markup)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Remove code blocks
			text := regexp.MustCompile("(?s)```.*?```").ReplaceAllString(params.Markdown, "")
			text = regexp.MustCompile("`[^`]+`").ReplaceAllString(text, "")

			// Remove markdown syntax
			text = regexp.MustCompile(`[#*_\[\]()>-]`).ReplaceAllString(text, " ")

			// Count words
			words := strings.Fields(text)

			result := map[string]any{
				"words":      len(words),
				"characters": len(params.Markdown),
				"lines":      len(strings.Split(params.Markdown, "\n")),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func splitSectionsTool() tool.Tool {
	return tool.NewBuilder("md_split_sections").
		WithDescription("Split Markdown into sections by heading").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Markdown string `json:"markdown"`
				Level    int    `json:"level,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Level == 0 {
				params.Level = 2
			}

			type Section struct {
				Heading string `json:"heading"`
				Content string `json:"content"`
			}

			pattern := `(?m)^#{` + string(rune('0'+params.Level)) + `}\s+(.+)$`
			headingRe := regexp.MustCompile(pattern)

			matches := headingRe.FindAllStringSubmatchIndex(params.Markdown, -1)
			var sections []Section

			for i, match := range matches {
				heading := params.Markdown[match[2]:match[3]]

				start := match[1]
				end := len(params.Markdown)
				if i+1 < len(matches) {
					end = matches[i+1][0]
				}

				content := strings.TrimSpace(params.Markdown[start:end])
				sections = append(sections, Section{Heading: heading, Content: content})
			}

			result := map[string]any{"sections": sections, "count": len(sections)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
