// Package html provides HTML processing tools for agents.
package html

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
	"golang.org/x/net/html"
)

// Pack returns the HTML tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("html").
		WithDescription("HTML processing and manipulation tools").
		AddTools(
			parseTool(),
			extractTextTool(),
			extractLinksTool(),
			extractImagesTool(),
			querySelectorTool(),
			extractMetaTool(),
			sanitizeTool(),
			minifyTool(),
			prettifyTool(),
			toMarkdownTool(),
			extractTablesTool(),
			extractFormsTool(),
			stripTagsTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("html_parse").
		WithDescription("Parse HTML and return structure info").
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

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				result := map[string]any{"valid": false, "error": err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var countNodes func(*html.Node) int
			countNodes = func(n *html.Node) int {
				count := 0
				if n.Type == html.ElementNode {
					count = 1
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					count += countNodes(c)
				}
				return count
			}

			result := map[string]any{"valid": true, "node_count": countNodes(doc)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractTextTool() tool.Tool {
	return tool.NewBuilder("html_extract_text").
		WithDescription("Extract all text content from HTML").
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

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				return tool.Result{}, err
			}

			var extractText func(*html.Node) string
			extractText = func(n *html.Node) string {
				if n.Type == html.TextNode {
					return n.Data
				}
				if n.Type == html.ElementNode {
					if n.Data == "script" || n.Data == "style" {
						return ""
					}
				}
				var sb strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					sb.WriteString(extractText(c))
				}
				return sb.String()
			}

			text := extractText(doc)
			text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
			text = strings.TrimSpace(text)

			result := map[string]string{"text": text}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractLinksTool() tool.Tool {
	return tool.NewBuilder("html_extract_links").
		WithDescription("Extract all links from HTML").
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

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				return tool.Result{}, err
			}

			type Link struct {
				Href string `json:"href"`
				Text string `json:"text"`
			}

			var links []Link
			var extract func(*html.Node)
			extract = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "a" {
					link := Link{}
					for _, attr := range n.Attr {
						if attr.Key == "href" {
							link.Href = attr.Val
							break
						}
					}
					var getText func(*html.Node) string
					getText = func(n *html.Node) string {
						if n.Type == html.TextNode {
							return n.Data
						}
						var sb strings.Builder
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							sb.WriteString(getText(c))
						}
						return sb.String()
					}
					link.Text = strings.TrimSpace(getText(n))
					links = append(links, link)
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					extract(c)
				}
			}
			extract(doc)

			result := map[string]any{"links": links, "count": len(links)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractImagesTool() tool.Tool {
	return tool.NewBuilder("html_extract_images").
		WithDescription("Extract all images from HTML").
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

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				return tool.Result{}, err
			}

			type Image struct {
				Src string `json:"src"`
				Alt string `json:"alt"`
			}

			var images []Image
			var extract func(*html.Node)
			extract = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "img" {
					img := Image{}
					for _, attr := range n.Attr {
						switch attr.Key {
						case "src":
							img.Src = attr.Val
						case "alt":
							img.Alt = attr.Val
						}
					}
					images = append(images, img)
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					extract(c)
				}
			}
			extract(doc)

			result := map[string]any{"images": images, "count": len(images)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func querySelectorTool() tool.Tool {
	return tool.NewBuilder("html_query_selector").
		WithDescription("Find elements by tag name").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				HTML  string `json:"html"`
				Tag   string `json:"tag"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				return tool.Result{}, err
			}

			if params.Limit == 0 {
				params.Limit = 100
			}

			var elements []map[string]any
			var extract func(*html.Node)
			extract = func(n *html.Node) {
				if len(elements) >= params.Limit {
					return
				}
				if n.Type == html.ElementNode && n.Data == params.Tag {
					elem := map[string]any{"tag": n.Data}
					attrs := make(map[string]string)
					for _, attr := range n.Attr {
						attrs[attr.Key] = attr.Val
					}
					if len(attrs) > 0 {
						elem["attributes"] = attrs
					}

					var getText func(*html.Node) string
					getText = func(n *html.Node) string {
						if n.Type == html.TextNode {
							return n.Data
						}
						var sb strings.Builder
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							sb.WriteString(getText(c))
						}
						return sb.String()
					}
					text := strings.TrimSpace(getText(n))
					if text != "" {
						elem["text"] = text
					}

					elements = append(elements, elem)
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					extract(c)
				}
			}
			extract(doc)

			result := map[string]any{"elements": elements, "count": len(elements)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractMetaTool() tool.Tool {
	return tool.NewBuilder("html_extract_meta").
		WithDescription("Extract meta tags from HTML").
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

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				return tool.Result{}, err
			}

			meta := make(map[string]string)
			var title string

			var extract func(*html.Node)
			extract = func(n *html.Node) {
				if n.Type == html.ElementNode {
					if n.Data == "title" {
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							if c.Type == html.TextNode {
								title = c.Data
							}
						}
					}
					if n.Data == "meta" {
						var name, content string
						for _, attr := range n.Attr {
							switch attr.Key {
							case "name", "property":
								name = attr.Val
							case "content":
								content = attr.Val
							}
						}
						if name != "" && content != "" {
							meta[name] = content
						}
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					extract(c)
				}
			}
			extract(doc)

			result := map[string]any{"title": title, "meta": meta}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sanitizeTool() tool.Tool {
	return tool.NewBuilder("html_sanitize").
		WithDescription("Remove potentially dangerous HTML elements and attributes").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				HTML string `json:"html"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sanitized := params.HTML

			// Remove script tags
			sanitized = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(sanitized, "")

			// Remove style tags
			sanitized = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(sanitized, "")

			// Remove event handlers
			sanitized = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*["'][^"']*["']`).ReplaceAllString(sanitized, "")
			sanitized = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*[^\s>]+`).ReplaceAllString(sanitized, "")

			// Remove javascript: URLs
			sanitized = regexp.MustCompile(`(?i)href\s*=\s*["']javascript:[^"']*["']`).ReplaceAllString(sanitized, `href="#"`)

			// Remove data: URLs in certain contexts
			sanitized = regexp.MustCompile(`(?i)src\s*=\s*["']data:[^"']*["']`).ReplaceAllString(sanitized, `src=""`)

			result := map[string]string{"html": sanitized}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func minifyTool() tool.Tool {
	return tool.NewBuilder("html_minify").
		WithDescription("Minify HTML by removing unnecessary whitespace").
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

			minified := params.HTML

			// Remove comments
			minified = regexp.MustCompile(`<!--.*?-->`).ReplaceAllString(minified, "")

			// Remove whitespace between tags
			minified = regexp.MustCompile(`>\s+<`).ReplaceAllString(minified, "><")

			// Collapse multiple spaces
			minified = regexp.MustCompile(`\s{2,}`).ReplaceAllString(minified, " ")

			// Trim
			minified = strings.TrimSpace(minified)

			result := map[string]any{
				"html":        minified,
				"original":    len(params.HTML),
				"minified":    len(minified),
				"saved_bytes": len(params.HTML) - len(minified),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func prettifyTool() tool.Tool {
	return tool.NewBuilder("html_prettify").
		WithDescription("Format HTML with proper indentation").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				HTML   string `json:"html"`
				Indent string `json:"indent,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Indent == "" {
				params.Indent = "  "
			}

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				return tool.Result{}, err
			}

			var buf bytes.Buffer
			var render func(*html.Node, int)
			render = func(n *html.Node, depth int) {
				switch n.Type {
				case html.ElementNode:
					indent := strings.Repeat(params.Indent, depth)
					buf.WriteString(indent + "<" + n.Data)
					for _, attr := range n.Attr {
						buf.WriteString(" " + attr.Key + `="` + attr.Val + `"`)
					}

					// Self-closing tags
					selfClosing := map[string]bool{
						"br": true, "hr": true, "img": true, "input": true,
						"meta": true, "link": true, "area": true, "base": true,
					}
					if selfClosing[n.Data] {
						buf.WriteString(" />\n")
						return
					}

					buf.WriteString(">\n")
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						render(c, depth+1)
					}
					buf.WriteString(indent + "</" + n.Data + ">\n")
				case html.TextNode:
					text := strings.TrimSpace(n.Data)
					if text != "" {
						indent := strings.Repeat(params.Indent, depth)
						buf.WriteString(indent + text + "\n")
					}
				case html.DocumentNode:
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						render(c, depth)
					}
				}
			}
			render(doc, 0)

			result := map[string]string{"html": buf.String()}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func toMarkdownTool() tool.Tool {
	return tool.NewBuilder("html_to_markdown").
		WithDescription("Convert HTML to Markdown").
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
				md = regexp.MustCompile(`(?is)<h`+string(rune('0'+i))+`[^>]*>(.*?)</h`+string(rune('0'+i))+`>`).
					ReplaceAllString(md, tag+" $1\n\n")
			}

			// Convert paragraphs
			md = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`).ReplaceAllString(md, "$1\n\n")

			// Convert links
			md = regexp.MustCompile(`(?is)<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`).ReplaceAllString(md, "[$2]($1)")

			// Convert emphasis
			md = regexp.MustCompile(`(?is)<(strong|b)[^>]*>(.*?)</\1>`).ReplaceAllString(md, "**$2**")
			md = regexp.MustCompile(`(?is)<(em|i)[^>]*>(.*?)</\1>`).ReplaceAllString(md, "*$2*")

			// Convert code
			md = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`).ReplaceAllString(md, "`$1`")
			md = regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`).ReplaceAllString(md, "```\n$1\n```\n")

			// Convert images
			md = regexp.MustCompile(`(?i)<img[^>]*src="([^"]+)"[^>]*alt="([^"]*)"[^>]*/?>`).ReplaceAllString(md, "![$2]($1)")
			md = regexp.MustCompile(`(?i)<img[^>]*alt="([^"]*)"[^>]*src="([^"]+)"[^>]*/?>`).ReplaceAllString(md, "![$1]($2)")

			// Convert lists
			md = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`).ReplaceAllString(md, "- $1\n")
			md = regexp.MustCompile(`(?is)</?[uo]l[^>]*>`).ReplaceAllString(md, "")

			// Convert line breaks
			md = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(md, "\n")

			// Remove remaining tags
			md = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(md, "")

			// Decode entities
			md = strings.ReplaceAll(md, "&amp;", "&")
			md = strings.ReplaceAll(md, "&lt;", "<")
			md = strings.ReplaceAll(md, "&gt;", ">")
			md = strings.ReplaceAll(md, "&quot;", "\"")
			md = strings.ReplaceAll(md, "&nbsp;", " ")

			// Clean up
			md = regexp.MustCompile(`\n{3,}`).ReplaceAllString(md, "\n\n")
			md = strings.TrimSpace(md)

			result := map[string]string{"markdown": md}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractTablesTool() tool.Tool {
	return tool.NewBuilder("html_extract_tables").
		WithDescription("Extract tables from HTML as structured data").
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

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				return tool.Result{}, err
			}

			type Table struct {
				Headers []string   `json:"headers"`
				Rows    [][]string `json:"rows"`
			}

			var tables []Table
			var extract func(*html.Node)
			extract = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "table" {
					table := Table{}

					var extractCells func(*html.Node, string) []string
					extractCells = func(row *html.Node, cellType string) []string {
						var cells []string
						for c := row.FirstChild; c != nil; c = c.NextSibling {
							if c.Type == html.ElementNode && c.Data == cellType {
								var getText func(*html.Node) string
								getText = func(n *html.Node) string {
									if n.Type == html.TextNode {
										return n.Data
									}
									var sb strings.Builder
									for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
										sb.WriteString(getText(ch))
									}
									return sb.String()
								}
								cells = append(cells, strings.TrimSpace(getText(c)))
							}
						}
						return cells
					}

					var processTable func(*html.Node)
					processTable = func(n *html.Node) {
						if n.Type == html.ElementNode && n.Data == "tr" {
							headerCells := extractCells(n, "th")
							if len(headerCells) > 0 {
								table.Headers = headerCells
							} else {
								dataCells := extractCells(n, "td")
								if len(dataCells) > 0 {
									table.Rows = append(table.Rows, dataCells)
								}
							}
						}
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							processTable(c)
						}
					}
					processTable(n)

					tables = append(tables, table)
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					extract(c)
				}
			}
			extract(doc)

			result := map[string]any{"tables": tables, "count": len(tables)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractFormsTool() tool.Tool {
	return tool.NewBuilder("html_extract_forms").
		WithDescription("Extract forms and their fields from HTML").
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

			doc, err := html.Parse(strings.NewReader(params.HTML))
			if err != nil {
				return tool.Result{}, err
			}

			type Field struct {
				Name  string `json:"name"`
				Type  string `json:"type"`
				Value string `json:"value,omitempty"`
			}

			type Form struct {
				Action string  `json:"action"`
				Method string  `json:"method"`
				Fields []Field `json:"fields"`
			}

			var forms []Form
			var extract func(*html.Node)
			extract = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "form" {
					form := Form{Method: "GET"}
					for _, attr := range n.Attr {
						switch attr.Key {
						case "action":
							form.Action = attr.Val
						case "method":
							form.Method = strings.ToUpper(attr.Val)
						}
					}

					var extractFields func(*html.Node)
					extractFields = func(n *html.Node) {
						if n.Type == html.ElementNode {
							if n.Data == "input" || n.Data == "select" || n.Data == "textarea" {
								field := Field{Type: "text"}
								for _, attr := range n.Attr {
									switch attr.Key {
									case "name":
										field.Name = attr.Val
									case "type":
										field.Type = attr.Val
									case "value":
										field.Value = attr.Val
									}
								}
								if n.Data == "select" {
									field.Type = "select"
								}
								if n.Data == "textarea" {
									field.Type = "textarea"
								}
								if field.Name != "" {
									form.Fields = append(form.Fields, field)
								}
							}
						}
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							extractFields(c)
						}
					}
					extractFields(n)

					forms = append(forms, form)
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					extract(c)
				}
			}
			extract(doc)

			result := map[string]any{"forms": forms, "count": len(forms)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func stripTagsTool() tool.Tool {
	return tool.NewBuilder("html_strip_tags").
		WithDescription("Remove all HTML tags, keeping only text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				HTML      string   `json:"html"`
				AllowTags []string `json:"allow_tags,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			stripped := params.HTML

			if len(params.AllowTags) > 0 {
				// Build pattern to match all tags except allowed ones
				for _, tag := range params.AllowTags {
					// Temporarily replace allowed tags
					stripped = regexp.MustCompile(`(?i)<`+tag+`([^>]*)>`).ReplaceAllString(stripped, "[[OPEN_"+tag+"$1]]")
					stripped = regexp.MustCompile(`(?i)</`+tag+`>`).ReplaceAllString(stripped, "[[CLOSE_"+tag+"]]")
				}
			}

			// Remove all tags
			stripped = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(stripped, "")

			if len(params.AllowTags) > 0 {
				// Restore allowed tags
				for _, tag := range params.AllowTags {
					stripped = regexp.MustCompile(`\[\[OPEN_`+tag+`([^\]]*)\]\]`).ReplaceAllString(stripped, "<"+tag+"$1>")
					stripped = regexp.MustCompile(`\[\[CLOSE_`+tag+`\]\]`).ReplaceAllString(stripped, "</"+tag+">")
				}
			}

			// Decode entities
			stripped = strings.ReplaceAll(stripped, "&amp;", "&")
			stripped = strings.ReplaceAll(stripped, "&lt;", "<")
			stripped = strings.ReplaceAll(stripped, "&gt;", ">")
			stripped = strings.ReplaceAll(stripped, "&nbsp;", " ")

			// Clean up whitespace
			stripped = regexp.MustCompile(`\s+`).ReplaceAllString(stripped, " ")
			stripped = strings.TrimSpace(stripped)

			result := map[string]string{"text": stripped}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
