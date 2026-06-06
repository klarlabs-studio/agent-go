// Package yaml provides tools for YAML parsing, manipulation, and conversion.
package yaml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

type yamlPack struct{}

// Pack creates a new YAML tools pack.
func Pack() *pack.Pack {
	p := &yamlPack{}

	return pack.NewBuilder("yaml").
		WithDescription("Tools for YAML parsing, manipulation, and conversion").
		WithVersion("1.0.0").
		AddTools(
			p.parseTool(),
			p.parseFileTool(),
			p.stringifyTool(),
			p.writeFileTool(),
			p.toJSONTool(),
			p.fromJSONTool(),
			p.mergeTool(),
			p.getTool(),
			p.setTool(),
			p.deleteTool(),
			p.validateTool(),
			p.multiDocParseTool(),
			p.multiDocStringifyTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// parseTool parses YAML string.
func (p *yamlPack) parseTool() tool.Tool {
	return tool.NewBuilder("yaml_parse").
		WithDescription("Parse YAML string to JSON").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Content == "" {
				return tool.Result{}, fmt.Errorf("content is required")
			}

			var data interface{}
			if err := yaml.Unmarshal([]byte(params.Content), &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse YAML: %w", err)
			}

			// Convert to JSON-compatible format
			data = convertToJSONCompatible(data)

			result := map[string]interface{}{
				"data": data,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// parseFileTool parses YAML file.
func (p *yamlPack) parseFileTool() tool.Tool {
	return tool.NewBuilder("yaml_parse_file").
		WithDescription("Parse YAML file to JSON").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			content, err := os.ReadFile(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
			}

			var data interface{}
			if err := yaml.Unmarshal(content, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse YAML: %w", err)
			}

			data = convertToJSONCompatible(data)

			result := map[string]interface{}{
				"data": data,
				"path": params.Path,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// stringifyTool converts data to YAML string.
func (p *yamlPack) stringifyTool() tool.Tool {
	return tool.NewBuilder("yaml_stringify").
		WithDescription("Convert data to YAML string").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data   interface{} `json:"data"`
				Indent int         `json:"indent,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			indent := params.Indent
			if indent <= 0 {
				indent = 2
			}

			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(indent)
			if err := encoder.Encode(params.Data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to encode YAML: %w", err)
			}
			_ = encoder.Close() // #nosec G104 -- best-effort close

			result := map[string]interface{}{
				"yaml": buf.String(),
			}
			output, _ := json.Marshal(result) // #nosec G104 -- marshaling simple map
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// writeFileTool writes data to YAML file.
func (p *yamlPack) writeFileTool() tool.Tool {
	return tool.NewBuilder("yaml_write_file").
		WithDescription("Write data to YAML file").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path   string      `json:"path"`
				Data   interface{} `json:"data"`
				Indent int         `json:"indent,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			indent := params.Indent
			if indent <= 0 {
				indent = 2
			}

			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(indent)
			if err := encoder.Encode(params.Data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to encode YAML: %w", err)
			}
			_ = encoder.Close() // #nosec G104 -- best-effort close

			if err := os.WriteFile(params.Path, buf.Bytes(), 0600); err != nil {
				return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
			}

			result := map[string]interface{}{
				"path":    params.Path,
				"size":    buf.Len(),
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// toJSONTool converts YAML to JSON.
func (p *yamlPack) toJSONTool() tool.Tool {
	return tool.NewBuilder("yaml_to_json").
		WithDescription("Convert YAML to JSON").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Yaml   string `json:"yaml"`
				Pretty bool   `json:"pretty,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Yaml == "" {
				return tool.Result{}, fmt.Errorf("yaml is required")
			}

			var data interface{}
			if err := yaml.Unmarshal([]byte(params.Yaml), &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse YAML: %w", err)
			}

			data = convertToJSONCompatible(data)

			var jsonBytes []byte
			var err error
			if params.Pretty {
				jsonBytes, err = json.MarshalIndent(data, "", "  ")
			} else {
				jsonBytes, err = json.Marshal(data)
			}
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to encode JSON: %w", err)
			}

			result := map[string]interface{}{
				"json": string(jsonBytes),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// fromJSONTool converts JSON to YAML.
func (p *yamlPack) fromJSONTool() tool.Tool {
	return tool.NewBuilder("yaml_from_json").
		WithDescription("Convert JSON to YAML").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				JSON   json.RawMessage `json:"json"`
				Indent int             `json:"indent,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var data interface{}
			if err := json.Unmarshal(params.JSON, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse JSON: %w", err)
			}

			indent := params.Indent
			if indent <= 0 {
				indent = 2
			}

			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(indent)
			if err := encoder.Encode(data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to encode YAML: %w", err)
			}
			_ = encoder.Close() // #nosec G104 -- best-effort close

			result := map[string]interface{}{
				"yaml": buf.String(),
			}
			output, _ := json.Marshal(result) // #nosec G104 -- marshaling simple map
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// mergeTool merges multiple YAML documents.
func (p *yamlPack) mergeTool() tool.Tool {
	return tool.NewBuilder("yaml_merge").
		WithDescription("Merge multiple YAML documents (later documents override earlier ones)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Documents []string `json:"documents"`
				Deep      bool     `json:"deep,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if len(params.Documents) < 2 {
				return tool.Result{}, fmt.Errorf("at least 2 documents required")
			}

			result := make(map[string]interface{})

			for _, doc := range params.Documents {
				var data map[string]interface{}
				if err := yaml.Unmarshal([]byte(doc), &data); err != nil {
					return tool.Result{}, fmt.Errorf("failed to parse YAML: %w", err)
				}

				data = convertToJSONCompatible(data).(map[string]interface{})

				if params.Deep {
					deepMerge(result, data)
				} else {
					for k, v := range data {
						result[k] = v
					}
				}
			}

			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(2)
			if err := encoder.Encode(result); err != nil {
				return tool.Result{}, fmt.Errorf("failed to encode YAML: %w", err)
			}
			_ = encoder.Close() // #nosec G104 -- best-effort close

			output, _ := json.Marshal(map[string]interface{}{
				"yaml":   buf.String(),
				"merged": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deepMerge(dst, src map[string]interface{}) {
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				deepMerge(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
}

// getTool gets value at path.
func (p *yamlPack) getTool() tool.Tool {
	return tool.NewBuilder("yaml_get").
		WithDescription("Get value at a dot-notation path in YAML").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
				Path    string `json:"path"` // dot notation: "foo.bar.baz"
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Content == "" || params.Path == "" {
				return tool.Result{}, fmt.Errorf("content and path are required")
			}

			var data interface{}
			if err := yaml.Unmarshal([]byte(params.Content), &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse YAML: %w", err)
			}

			data = convertToJSONCompatible(data)

			parts := strings.Split(params.Path, ".")
			current := data
			for _, part := range parts {
				switch v := current.(type) {
				case map[string]interface{}:
					var ok bool
					current, ok = v[part]
					if !ok {
						result := map[string]interface{}{
							"found": false,
							"path":  params.Path,
						}
						output, _ := json.Marshal(result)
						return tool.Result{Output: output}, nil
					}
				default:
					result := map[string]interface{}{
						"found": false,
						"path":  params.Path,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]interface{}{
				"found": true,
				"value": current,
				"path":  params.Path,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// setTool sets value at path.
func (p *yamlPack) setTool() tool.Tool {
	return tool.NewBuilder("yaml_set").
		WithDescription("Set value at a dot-notation path in YAML").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string      `json:"content"`
				Path    string      `json:"path"`
				Value   interface{} `json:"value"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Content == "" || params.Path == "" {
				return tool.Result{}, fmt.Errorf("content and path are required")
			}

			var data interface{}
			if err := yaml.Unmarshal([]byte(params.Content), &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse YAML: %w", err)
			}

			data = convertToJSONCompatible(data)

			dataMap, ok := data.(map[string]interface{})
			if !ok {
				return tool.Result{}, fmt.Errorf("root must be an object")
			}

			parts := strings.Split(params.Path, ".")
			current := dataMap
			for i := 0; i < len(parts)-1; i++ {
				part := parts[i]
				if next, ok := current[part].(map[string]interface{}); ok {
					current = next
				} else {
					newMap := make(map[string]interface{})
					current[part] = newMap
					current = newMap
				}
			}
			current[parts[len(parts)-1]] = params.Value

			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(2)
			if err := encoder.Encode(dataMap); err != nil {
				return tool.Result{}, fmt.Errorf("failed to encode YAML: %w", err)
			}
			_ = encoder.Close() // #nosec G104 -- best-effort close

			result := map[string]interface{}{
				"yaml":    buf.String(),
				"path":    params.Path,
				"success": true,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- marshaling simple map
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// deleteTool deletes value at path.
func (p *yamlPack) deleteTool() tool.Tool {
	return tool.NewBuilder("yaml_delete").
		WithDescription("Delete value at a dot-notation path in YAML").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
				Path    string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Content == "" || params.Path == "" {
				return tool.Result{}, fmt.Errorf("content and path are required")
			}

			var data interface{}
			if err := yaml.Unmarshal([]byte(params.Content), &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse YAML: %w", err)
			}

			data = convertToJSONCompatible(data)

			dataMap, ok := data.(map[string]interface{})
			if !ok {
				return tool.Result{}, fmt.Errorf("root must be an object")
			}

			parts := strings.Split(params.Path, ".")
			current := dataMap
			for i := 0; i < len(parts)-1; i++ {
				part := parts[i]
				if next, ok := current[part].(map[string]interface{}); ok {
					current = next
				} else {
					result := map[string]interface{}{
						"yaml":    params.Content,
						"deleted": false,
						"path":    params.Path,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			delete(current, parts[len(parts)-1])

			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(2)
			if err := encoder.Encode(dataMap); err != nil {
				return tool.Result{}, fmt.Errorf("failed to encode YAML: %w", err)
			}
			_ = encoder.Close() // #nosec G104 -- best-effort close

			result := map[string]interface{}{
				"yaml":    buf.String(),
				"deleted": true,
				"path":    params.Path,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- marshaling simple map
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// validateTool validates YAML syntax.
func (p *yamlPack) validateTool() tool.Tool {
	return tool.NewBuilder("yaml_validate").
		WithDescription("Validate YAML syntax").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Content == "" {
				return tool.Result{}, fmt.Errorf("content is required")
			}

			var data interface{}
			err := yaml.Unmarshal([]byte(params.Content), &data)

			if err != nil {
				result := map[string]interface{}{
					"valid": false,
					"error": err.Error(),
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

// multiDocParseTool parses multi-document YAML.
func (p *yamlPack) multiDocParseTool() tool.Tool {
	return tool.NewBuilder("yaml_multi_doc_parse").
		WithDescription("Parse multi-document YAML (separated by ---)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Content == "" {
				return tool.Result{}, fmt.Errorf("content is required")
			}

			decoder := yaml.NewDecoder(strings.NewReader(params.Content))

			var documents []interface{}
			for {
				var doc interface{}
				err := decoder.Decode(&doc)
				if err != nil {
					break
				}
				if doc != nil {
					documents = append(documents, convertToJSONCompatible(doc))
				}
			}

			result := map[string]interface{}{
				"documents": documents,
				"count":     len(documents),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// multiDocStringifyTool creates multi-document YAML.
func (p *yamlPack) multiDocStringifyTool() tool.Tool {
	return tool.NewBuilder("yaml_multi_doc_stringify").
		WithDescription("Create multi-document YAML from array of documents").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Documents []interface{} `json:"documents"`
				Indent    int           `json:"indent,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if len(params.Documents) == 0 {
				return tool.Result{}, fmt.Errorf("documents are required")
			}

			indent := params.Indent
			if indent <= 0 {
				indent = 2
			}

			var buf bytes.Buffer
			encoder := yaml.NewEncoder(&buf)
			encoder.SetIndent(indent)

			for _, doc := range params.Documents {
				if err := encoder.Encode(doc); err != nil {
					return tool.Result{}, fmt.Errorf("failed to encode document: %w", err)
				}
			}
			_ = encoder.Close() // #nosec G104 -- best-effort close

			result := map[string]interface{}{
				"yaml":  buf.String(),
				"count": len(params.Documents),
			}
			output, _ := json.Marshal(result) // #nosec G104 -- marshaling simple map
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// convertToJSONCompatible converts YAML data to JSON-compatible format.
// YAML supports additional types that JSON doesn't (like map[interface{}]interface{}).
func convertToJSONCompatible(data interface{}) interface{} {
	switch v := data.(type) {
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			strKey := fmt.Sprintf("%v", key)
			result[strKey] = convertToJSONCompatible(value)
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = convertToJSONCompatible(value)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertToJSONCompatible(item)
		}
		return result
	default:
		return v
	}
}
