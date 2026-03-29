// Package xml provides XML processing tools for agents.
package xml

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"regexp"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the XML tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("xml").
		WithDescription("XML processing and manipulation tools").
		AddTools(
			parseTool(),
			validateTool(),
			toJSONTool(),
			fromJSONTool(),
			xpathTool(),
			extractElementsTool(),
			extractAttributesTool(),
			prettifyTool(),
			minifyTool(),
			extractTextTool(),
			countElementsTool(),
			getNamespacesTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Node represents an XML element
type Node struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Content  string     `xml:",chardata"`
	Children []Node     `xml:",any"`
}

func parseTool() tool.Tool {
	return tool.NewBuilder("xml_parse").
		WithDescription("Parse XML and return structure info").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML string `json:"xml"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var root Node
			if err := xml.Unmarshal([]byte(params.XML), &root); err != nil {
				result := map[string]any{"valid": false, "error": err.Error()}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var countNodes func(Node) int
			countNodes = func(n Node) int {
				count := 1
				for _, child := range n.Children {
					count += countNodes(child)
				}
				return count
			}

			result := map[string]any{
				"valid":      true,
				"root":       root.XMLName.Local,
				"namespace":  root.XMLName.Space,
				"node_count": countNodes(root),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("xml_validate").
		WithDescription("Validate XML syntax").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML string `json:"xml"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var root Node
			err := xml.Unmarshal([]byte(params.XML), &root)

			result := map[string]any{"valid": err == nil}
			if err != nil {
				result["error"] = err.Error()
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func toJSONTool() tool.Tool {
	return tool.NewBuilder("xml_to_json").
		WithDescription("Convert XML to JSON").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML string `json:"xml"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var root Node
			if err := xml.Unmarshal([]byte(params.XML), &root); err != nil {
				return tool.Result{}, err
			}

			var nodeToMap func(Node) map[string]any
			nodeToMap = func(n Node) map[string]any {
				result := make(map[string]any)

				// Add attributes
				if len(n.Attrs) > 0 {
					attrs := make(map[string]string)
					for _, attr := range n.Attrs {
						attrs[attr.Name.Local] = attr.Value
					}
					result["@attributes"] = attrs
				}

				// Add text content
				text := strings.TrimSpace(n.Content)
				if text != "" {
					result["#text"] = text
				}

				// Add children
				childMap := make(map[string][]map[string]any)
				for _, child := range n.Children {
					name := child.XMLName.Local
					childMap[name] = append(childMap[name], nodeToMap(child))
				}
				for name, children := range childMap {
					if len(children) == 1 {
						result[name] = children[0]
					} else {
						result[name] = children
					}
				}

				return result
			}

			jsonData := map[string]any{root.XMLName.Local: nodeToMap(root)}
			output, _ := json.Marshal(jsonData)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fromJSONTool() tool.Tool {
	return tool.NewBuilder("xml_from_json").
		WithDescription("Convert JSON to XML").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				JSON   map[string]any `json:"json"`
				Root   string         `json:"root,omitempty"`
				Indent string         `json:"indent,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Root == "" {
				params.Root = "root"
			}
			if params.Indent == "" {
				params.Indent = "  "
			}

			var toXML func(key string, value any, depth int) string
			toXML = func(key string, value any, depth int) string {
				indent := strings.Repeat(params.Indent, depth)

				switch v := value.(type) {
				case map[string]any:
					var sb strings.Builder
					sb.WriteString(indent + "<" + key + ">\n")
					for k, val := range v {
						sb.WriteString(toXML(k, val, depth+1))
					}
					sb.WriteString(indent + "</" + key + ">\n")
					return sb.String()
				case []any:
					var sb strings.Builder
					for _, item := range v {
						sb.WriteString(toXML(key, item, depth))
					}
					return sb.String()
				default:
					return indent + "<" + key + ">" + escapeXML(v) + "</" + key + ">\n"
				}
			}

			xmlStr := `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
			xmlStr += toXML(params.Root, params.JSON, 0)

			result := map[string]string{"xml": xmlStr}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func escapeXML(v any) string {
	s := ""
	switch val := v.(type) {
	case string:
		s = val
	case float64:
		s = strings.TrimRight(strings.TrimRight(strings.Replace(json.Number(json.RawMessage{}).String(), "", "", 1), "0"), ".")
		if s == "" {
			s = "0"
		}
		// Use fmt for proper formatting
		b, _ := json.Marshal(val)
		s = string(b)
	default:
		b, _ := json.Marshal(val)
		s = string(b)
	}
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func xpathTool() tool.Tool {
	return tool.NewBuilder("xml_xpath").
		WithDescription("Query XML using simple XPath-like expressions").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML  string `json:"xml"`
				Path string `json:"path"` // Simple path like /root/child or //element
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Simple implementation: split path and traverse
			path := strings.TrimPrefix(params.Path, "/")
			parts := strings.Split(path, "/")

			// Check for descendant search
			descendant := false
			if len(parts) > 0 && parts[0] == "" {
				descendant = true
				parts = parts[1:]
			}

			var root Node
			if err := xml.Unmarshal([]byte(params.XML), &root); err != nil {
				return tool.Result{}, err
			}

			var findElements func(Node, []string, bool) []Node
			findElements = func(n Node, path []string, desc bool) []Node {
				if len(path) == 0 {
					return []Node{n}
				}

				var results []Node
				target := path[0]
				remaining := path[1:]

				if desc {
					// Search all descendants
					var searchAll func(Node)
					searchAll = func(node Node) {
						if node.XMLName.Local == target {
							if len(remaining) == 0 {
								results = append(results, node)
							} else {
								for _, child := range node.Children {
									results = append(results, findElements(child, remaining, false)...)
								}
							}
						}
						for _, child := range node.Children {
							searchAll(child)
						}
					}
					searchAll(n)
				} else {
					if n.XMLName.Local == target {
						if len(remaining) == 0 {
							return []Node{n}
						}
						for _, child := range n.Children {
							results = append(results, findElements(child, remaining, false)...)
						}
					}
					for _, child := range n.Children {
						if child.XMLName.Local == target {
							if len(remaining) == 0 {
								results = append(results, child)
							} else {
								for _, grandChild := range child.Children {
									results = append(results, findElements(grandChild, remaining, false)...)
								}
							}
						}
					}
				}
				return results
			}

			elements := findElements(root, parts, descendant)

			var nodeToResult func(Node) map[string]any
			nodeToResult = func(n Node) map[string]any {
				r := map[string]any{
					"name":    n.XMLName.Local,
					"content": strings.TrimSpace(n.Content),
				}
				if len(n.Attrs) > 0 {
					attrs := make(map[string]string)
					for _, attr := range n.Attrs {
						attrs[attr.Name.Local] = attr.Value
					}
					r["attributes"] = attrs
				}
				return r
			}

			var results []map[string]any
			for _, elem := range elements {
				results = append(results, nodeToResult(elem))
			}

			result := map[string]any{"matches": results, "count": len(results)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractElementsTool() tool.Tool {
	return tool.NewBuilder("xml_extract_elements").
		WithDescription("Extract all elements with a specific tag name").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML string `json:"xml"`
				Tag string `json:"tag"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var root Node
			if err := xml.Unmarshal([]byte(params.XML), &root); err != nil {
				return tool.Result{}, err
			}

			var elements []map[string]any
			var extract func(Node)
			extract = func(n Node) {
				if n.XMLName.Local == params.Tag {
					elem := map[string]any{
						"content": strings.TrimSpace(n.Content),
					}
					if len(n.Attrs) > 0 {
						attrs := make(map[string]string)
						for _, attr := range n.Attrs {
							attrs[attr.Name.Local] = attr.Value
						}
						elem["attributes"] = attrs
					}
					elements = append(elements, elem)
				}
				for _, child := range n.Children {
					extract(child)
				}
			}
			extract(root)

			result := map[string]any{"elements": elements, "count": len(elements)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractAttributesTool() tool.Tool {
	return tool.NewBuilder("xml_extract_attributes").
		WithDescription("Extract all values of a specific attribute").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML       string `json:"xml"`
				Attribute string `json:"attribute"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var root Node
			if err := xml.Unmarshal([]byte(params.XML), &root); err != nil {
				return tool.Result{}, err
			}

			var values []string
			var extract func(Node)
			extract = func(n Node) {
				for _, attr := range n.Attrs {
					if attr.Name.Local == params.Attribute {
						values = append(values, attr.Value)
					}
				}
				for _, child := range n.Children {
					extract(child)
				}
			}
			extract(root)

			result := map[string]any{"values": values, "count": len(values)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func prettifyTool() tool.Tool {
	return tool.NewBuilder("xml_prettify").
		WithDescription("Format XML with proper indentation").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML    string `json:"xml"`
				Indent string `json:"indent,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Indent == "" {
				params.Indent = "  "
			}

			var root Node
			if err := xml.Unmarshal([]byte(params.XML), &root); err != nil {
				return tool.Result{}, err
			}

			var render func(Node, int) string
			render = func(n Node, depth int) string {
				indent := strings.Repeat(params.Indent, depth)
				var sb strings.Builder

				sb.WriteString(indent + "<" + n.XMLName.Local)
				for _, attr := range n.Attrs {
					sb.WriteString(" " + attr.Name.Local + `="` + attr.Value + `"`)
				}

				text := strings.TrimSpace(n.Content)
				if len(n.Children) == 0 && text == "" {
					sb.WriteString("/>\n")
					return sb.String()
				}

				sb.WriteString(">")

				if len(n.Children) == 0 {
					sb.WriteString(text + "</" + n.XMLName.Local + ">\n")
					return sb.String()
				}

				sb.WriteString("\n")
				if text != "" {
					sb.WriteString(indent + params.Indent + text + "\n")
				}
				for _, child := range n.Children {
					sb.WriteString(render(child, depth+1))
				}
				sb.WriteString(indent + "</" + n.XMLName.Local + ">\n")
				return sb.String()
			}

			xmlStr := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + render(root, 0)

			result := map[string]string{"xml": xmlStr}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func minifyTool() tool.Tool {
	return tool.NewBuilder("xml_minify").
		WithDescription("Minify XML by removing unnecessary whitespace").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML string `json:"xml"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			minified := params.XML

			// Remove comments
			minified = regexp.MustCompile(`<!--.*?-->`).ReplaceAllString(minified, "")

			// Remove whitespace between tags
			minified = regexp.MustCompile(`>\s+<`).ReplaceAllString(minified, "><")

			// Collapse multiple spaces
			minified = regexp.MustCompile(`\s{2,}`).ReplaceAllString(minified, " ")

			// Trim
			minified = strings.TrimSpace(minified)

			result := map[string]any{
				"xml":         minified,
				"original":    len(params.XML),
				"minified":    len(minified),
				"saved_bytes": len(params.XML) - len(minified),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractTextTool() tool.Tool {
	return tool.NewBuilder("xml_extract_text").
		WithDescription("Extract all text content from XML").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML string `json:"xml"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var root Node
			if err := xml.Unmarshal([]byte(params.XML), &root); err != nil {
				return tool.Result{}, err
			}

			var extractText func(Node) string
			extractText = func(n Node) string {
				var sb strings.Builder
				text := strings.TrimSpace(n.Content)
				if text != "" {
					sb.WriteString(text + " ")
				}
				for _, child := range n.Children {
					sb.WriteString(extractText(child))
				}
				return sb.String()
			}

			text := strings.TrimSpace(extractText(root))
			text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

			result := map[string]string{"text": text}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countElementsTool() tool.Tool {
	return tool.NewBuilder("xml_count_elements").
		WithDescription("Count occurrences of each element type").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML string `json:"xml"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var root Node
			if err := xml.Unmarshal([]byte(params.XML), &root); err != nil {
				return tool.Result{}, err
			}

			counts := make(map[string]int)
			var count func(Node)
			count = func(n Node) {
				counts[n.XMLName.Local]++
				for _, child := range n.Children {
					count(child)
				}
			}
			count(root)

			total := 0
			for _, c := range counts {
				total += c
			}

			result := map[string]any{"elements": counts, "total": total}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getNamespacesTool() tool.Tool {
	return tool.NewBuilder("xml_get_namespaces").
		WithDescription("Extract all namespace declarations from XML").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				XML string `json:"xml"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			namespaces := make(map[string]string)

			// Find xmlns declarations
			nsRe := regexp.MustCompile(`xmlns(?::(\w+))?=["']([^"']+)["']`)
			for _, match := range nsRe.FindAllStringSubmatch(params.XML, -1) {
				prefix := match[1]
				if prefix == "" {
					prefix = "default"
				}
				namespaces[prefix] = match[2]
			}

			result := map[string]any{"namespaces": namespaces, "count": len(namespaces)}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
