// Package json provides tools for JSON manipulation, JSONPath queries, and schema validation.
package json

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
	"github.com/santhosh-tekuri/jsonschema/v5"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

type jsonPack struct{}

// Pack creates a new JSON tools pack.
func Pack() *pack.Pack {
	p := &jsonPack{}

	return pack.NewBuilder("json").
		WithDescription("Tools for JSON manipulation, JSONPath queries, and schema validation").
		WithVersion("1.0.0").
		AddTools(
			// JSONPath tools
			p.queryTool(),
			p.setTool(),
			p.deleteTool(),
			// Manipulation tools
			p.mergeTool(),
			p.diffTool(),
			p.patchTool(),
			p.transformTool(),
			p.flattenTool(),
			p.unflattenTool(),
			// Validation tools
			p.validateSchemaTool(),
			p.inferSchemaTool(),
			// Format tools
			p.formatTool(),
			p.minifyTool(),
			p.sortKeysTool(),
			// Conversion tools
			p.toArrayTool(),
			p.fromArrayTool(),
			p.keysTool(),
			p.valuesTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// queryTool queries JSON using JSONPath.
func (p *jsonPack) queryTool() tool.Tool {
	return tool.NewBuilder("json_query").
		WithDescription("Query JSON data using JSONPath expression").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data json.RawMessage `json:"data"`
				Path string          `json:"path"`
				File string          `json:"file,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			var data interface{}
			if params.File != "" {
				content, err := os.ReadFile(params.File)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
				}
				if err := json.Unmarshal(content, &data); err != nil {
					return tool.Result{}, fmt.Errorf("failed to parse JSON: %w", err)
				}
			} else {
				if err := json.Unmarshal(params.Data, &data); err != nil {
					return tool.Result{}, fmt.Errorf("failed to parse JSON: %w", err)
				}
			}

			expr, err := jp.ParseString(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid JSONPath: %w", err)
			}

			results := expr.Get(data)

			result := map[string]interface{}{
				"results": results,
				"count":   len(results),
				"path":    params.Path,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// setTool sets a value at a JSONPath.
func (p *jsonPack) setTool() tool.Tool {
	return tool.NewBuilder("json_set").
		WithDescription("Set a value at a JSONPath location").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data  json.RawMessage `json:"data"`
				Path  string          `json:"path"`
				Value interface{}     `json:"value"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse JSON: %w", err)
			}

			expr, err := jp.ParseString(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid JSONPath: %w", err)
			}

			if err := expr.Set(data, params.Value); err != nil {
				return tool.Result{}, fmt.Errorf("failed to set value: %w", err)
			}

			result := map[string]interface{}{
				"result":  data,
				"path":    params.Path,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// deleteTool deletes a value at a JSONPath.
func (p *jsonPack) deleteTool() tool.Tool {
	return tool.NewBuilder("json_delete").
		WithDescription("Delete a value at a JSONPath location").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data json.RawMessage `json:"data"`
				Path string          `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Path == "" {
				return tool.Result{}, fmt.Errorf("path is required")
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse JSON: %w", err)
			}

			expr, err := jp.ParseString(params.Path)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid JSONPath: %w", err)
			}

			if err := expr.Del(data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to delete value: %w", err)
			}

			result := map[string]interface{}{
				"result":  data,
				"path":    params.Path,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// mergeTool merges multiple JSON objects.
func (p *jsonPack) mergeTool() tool.Tool {
	return tool.NewBuilder("json_merge").
		WithDescription("Merge multiple JSON objects (later objects override earlier ones)").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Objects []json.RawMessage `json:"objects"`
				Deep    bool              `json:"deep,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if len(params.Objects) < 2 {
				return tool.Result{}, fmt.Errorf("at least 2 objects are required")
			}

			result := make(map[string]interface{})

			for _, obj := range params.Objects {
				var m map[string]interface{}
				if err := json.Unmarshal(obj, &m); err != nil {
					return tool.Result{}, fmt.Errorf("all items must be objects: %w", err)
				}

				if params.Deep {
					deepMerge(result, m)
				} else {
					for k, v := range m {
						result[k] = v
					}
				}
			}

			output, _ := json.Marshal(map[string]interface{}{
				"result": result,
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

// diffTool compares two JSON values.
func (p *jsonPack) diffTool() tool.Tool {
	return tool.NewBuilder("json_diff").
		WithDescription("Compare two JSON values and show differences").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A json.RawMessage `json:"a"`
				B json.RawMessage `json:"b"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var a, b interface{}
			if err := json.Unmarshal(params.A, &a); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse a: %w", err)
			}
			if err := json.Unmarshal(params.B, &b); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse b: %w", err)
			}

			diffs := computeDiff("$", a, b)

			result := map[string]interface{}{
				"equal":      len(diffs) == 0,
				"diff_count": len(diffs),
				"diffs":      diffs,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func computeDiff(path string, a, b interface{}) []map[string]interface{} {
	var diffs []map[string]interface{}

	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	if string(aJSON) == string(bJSON) {
		return diffs
	}

	switch aVal := a.(type) {
	case map[string]interface{}:
		bVal, ok := b.(map[string]interface{})
		if !ok {
			diffs = append(diffs, map[string]interface{}{
				"path":   path,
				"type":   "type_change",
				"before": a,
				"after":  b,
			})
			return diffs
		}

		// Check for removed keys
		for k := range aVal {
			if _, exists := bVal[k]; !exists {
				diffs = append(diffs, map[string]interface{}{
					"path":   path + "." + k,
					"type":   "removed",
					"before": aVal[k],
				})
			}
		}

		// Check for added/changed keys
		for k, v := range bVal {
			if aV, exists := aVal[k]; exists {
				diffs = append(diffs, computeDiff(path+"."+k, aV, v)...)
			} else {
				diffs = append(diffs, map[string]interface{}{
					"path":  path + "." + k,
					"type":  "added",
					"after": v,
				})
			}
		}

	case []interface{}:
		bVal, ok := b.([]interface{})
		if !ok {
			diffs = append(diffs, map[string]interface{}{
				"path":   path,
				"type":   "type_change",
				"before": a,
				"after":  b,
			})
			return diffs
		}

		maxLen := len(aVal)
		if len(bVal) > maxLen {
			maxLen = len(bVal)
		}

		for i := 0; i < maxLen; i++ {
			elemPath := fmt.Sprintf("%s[%d]", path, i)
			switch {
			case i >= len(aVal):
				diffs = append(diffs, map[string]interface{}{
					"path":  elemPath,
					"type":  "added",
					"after": bVal[i],
				})
			case i >= len(bVal):
				diffs = append(diffs, map[string]interface{}{
					"path":   elemPath,
					"type":   "removed",
					"before": aVal[i],
				})
			default:
				diffs = append(diffs, computeDiff(elemPath, aVal[i], bVal[i])...)
			}
		}

	default:
		diffs = append(diffs, map[string]interface{}{
			"path":   path,
			"type":   "changed",
			"before": a,
			"after":  b,
		})
	}

	return diffs
}

// patchTool applies a JSON patch.
func (p *jsonPack) patchTool() tool.Tool {
	return tool.NewBuilder("json_patch").
		WithDescription("Apply a JSON Patch (RFC 6902) to a document").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data    json.RawMessage   `json:"data"`
				Patches []json.RawMessage `json:"patches"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse data: %w", err)
			}

			for _, patchRaw := range params.Patches {
				var patch struct {
					Op    string      `json:"op"`
					Path  string      `json:"path"`
					Value interface{} `json:"value,omitempty"`
					From  string      `json:"from,omitempty"`
				}
				if err := json.Unmarshal(patchRaw, &patch); err != nil {
					return tool.Result{}, fmt.Errorf("invalid patch: %w", err)
				}

				// Convert JSON Pointer to JSONPath
				path := jsonPointerToPath(patch.Path)
				expr, err := jp.ParseString(path)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid path: %w", err)
				}

				switch patch.Op {
				case "add", "replace":
					if err := expr.Set(data, patch.Value); err != nil {
						return tool.Result{}, fmt.Errorf("failed to %s: %w", patch.Op, err)
					}
				case "remove":
					if err := expr.Del(data); err != nil {
						return tool.Result{}, fmt.Errorf("failed to remove: %w", err)
					}
				case "copy", "move":
					fromPath := jsonPointerToPath(patch.From)
					fromExpr, err := jp.ParseString(fromPath)
					if err != nil {
						return tool.Result{}, fmt.Errorf("invalid from path: %w", err)
					}
					values := fromExpr.Get(data)
					if len(values) == 0 {
						return tool.Result{}, fmt.Errorf("source path not found: %s", patch.From)
					}
					if err := expr.Set(data, values[0]); err != nil {
						return tool.Result{}, fmt.Errorf("failed to %s: %w", patch.Op, err)
					}
					if patch.Op == "move" {
						if err := fromExpr.Del(data); err != nil {
							return tool.Result{}, fmt.Errorf("failed to remove source: %w", err)
						}
					}
				default:
					return tool.Result{}, fmt.Errorf("unsupported operation: %s", patch.Op)
				}
			}

			result := map[string]interface{}{
				"result":  data,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func jsonPointerToPath(pointer string) string {
	if pointer == "" || pointer == "/" {
		return "$"
	}
	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	path := "$"
	for _, part := range parts {
		// Unescape JSON Pointer
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		path += "." + part
	}
	return path
}

// transformTool transforms JSON using a template.
func (p *jsonPack) transformTool() tool.Tool {
	return tool.NewBuilder("json_transform").
		WithDescription("Transform JSON by selecting and renaming fields").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data     json.RawMessage   `json:"data"`
				Mappings map[string]string `json:"mappings"` // newKey -> JSONPath
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse data: %w", err)
			}

			result := make(map[string]interface{})
			for newKey, path := range params.Mappings {
				expr, err := jp.ParseString(path)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid path for %s: %w", newKey, err)
				}
				values := expr.Get(data)
				if len(values) == 1 {
					result[newKey] = values[0]
				} else if len(values) > 1 {
					result[newKey] = values
				}
			}

			output, _ := json.Marshal(map[string]interface{}{
				"result": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// flattenTool flattens nested JSON.
func (p *jsonPack) flattenTool() tool.Tool {
	return tool.NewBuilder("json_flatten").
		WithDescription("Flatten nested JSON into a single-level object with dot-notation keys").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data      json.RawMessage `json:"data"`
				Separator string          `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			sep := params.Separator
			if sep == "" {
				sep = "."
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse data: %w", err)
			}

			result := make(map[string]interface{})
			flatten("", data, result, sep)

			output, _ := json.Marshal(map[string]interface{}{
				"result": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func flatten(prefix string, data interface{}, result map[string]interface{}, sep string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for k, val := range v {
			newKey := k
			if prefix != "" {
				newKey = prefix + sep + k
			}
			flatten(newKey, val, result, sep)
		}
	case []interface{}:
		for i, val := range v {
			newKey := fmt.Sprintf("%s[%d]", prefix, i)
			if prefix == "" {
				newKey = fmt.Sprintf("[%d]", i)
			}
			flatten(newKey, val, result, sep)
		}
	default:
		result[prefix] = v
	}
}

// unflattenTool unflattens a flattened JSON.
func (p *jsonPack) unflattenTool() tool.Tool {
	return tool.NewBuilder("json_unflatten").
		WithDescription("Unflatten a dot-notation object back to nested JSON").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data      json.RawMessage `json:"data"`
				Separator string          `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			sep := params.Separator
			if sep == "" {
				sep = "."
			}

			var flat map[string]interface{}
			if err := json.Unmarshal(params.Data, &flat); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an object: %w", err)
			}

			result := make(map[string]interface{})
			for key, value := range flat {
				parts := strings.Split(key, sep)
				setNested(result, parts, value)
			}

			output, _ := json.Marshal(map[string]interface{}{
				"result": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func setNested(obj map[string]interface{}, path []string, value interface{}) {
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		if _, exists := obj[key]; !exists {
			obj[key] = make(map[string]interface{})
		}
		if m, ok := obj[key].(map[string]interface{}); ok {
			obj = m
		} else {
			return
		}
	}
	obj[path[len(path)-1]] = value
}

// validateSchemaTool validates JSON against a schema.
func (p *jsonPack) validateSchemaTool() tool.Tool {
	return tool.NewBuilder("json_validate_schema").
		WithDescription("Validate JSON data against a JSON Schema").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data   json.RawMessage `json:"data"`
				Schema json.RawMessage `json:"schema"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			compiler := jsonschema.NewCompiler()
			if err := compiler.AddResource("schema.json", bytes.NewReader(params.Schema)); err != nil {
				return tool.Result{}, fmt.Errorf("invalid schema: %w", err)
			}

			schema, err := compiler.Compile("schema.json")
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to compile schema: %w", err)
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse data: %w", err)
			}

			err = schema.Validate(data)
			if err != nil {
				validationErr, ok := err.(*jsonschema.ValidationError)
				if ok {
					var errors []map[string]interface{}
					for _, e := range validationErr.Causes {
						errors = append(errors, map[string]interface{}{
							"path":    e.InstanceLocation,
							"message": e.Message,
						})
					}
					result := map[string]interface{}{
						"valid":  false,
						"errors": errors,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
				return tool.Result{}, fmt.Errorf("validation failed: %w", err)
			}

			result := map[string]interface{}{
				"valid":  true,
				"errors": []interface{}{},
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// inferSchemaTool infers a JSON Schema from data.
func (p *jsonPack) inferSchemaTool() tool.Tool {
	return tool.NewBuilder("json_infer_schema").
		WithDescription("Infer a JSON Schema from example data").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse data: %w", err)
			}

			schema := inferSchema(data)

			result := map[string]interface{}{
				"schema": schema,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func inferSchema(data interface{}) map[string]interface{} {
	switch v := data.(type) {
	case nil:
		return map[string]interface{}{"type": "null"}
	case bool:
		return map[string]interface{}{"type": "boolean"}
	case float64:
		if v == float64(int(v)) {
			return map[string]interface{}{"type": "integer"}
		}
		return map[string]interface{}{"type": "number"}
	case string:
		return map[string]interface{}{"type": "string"}
	case []interface{}:
		if len(v) == 0 {
			return map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{},
			}
		}
		return map[string]interface{}{
			"type":  "array",
			"items": inferSchema(v[0]),
		}
	case map[string]interface{}:
		properties := make(map[string]interface{})
		required := make([]string, 0)
		for k, val := range v {
			properties[k] = inferSchema(val)
			required = append(required, k)
		}
		sort.Strings(required)
		return map[string]interface{}{
			"type":       "object",
			"properties": properties,
			"required":   required,
		}
	default:
		return map[string]interface{}{}
	}
}

// formatTool formats JSON with indentation.
func (p *jsonPack) formatTool() tool.Tool {
	return tool.NewBuilder("json_format").
		WithDescription("Format JSON with pretty indentation").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data   json.RawMessage `json:"data"`
				Indent string          `json:"indent,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			indent := params.Indent
			if indent == "" {
				indent = "  "
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse data: %w", err)
			}

			formatted, err := json.MarshalIndent(data, "", indent)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to format: %w", err)
			}

			result := map[string]interface{}{
				"formatted": string(formatted),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// minifyTool minifies JSON.
func (p *jsonPack) minifyTool() tool.Tool {
	return tool.NewBuilder("json_minify").
		WithDescription("Minify JSON by removing whitespace").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var data interface{}
			if err := json.Unmarshal(params.Data, &data); err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse data: %w", err)
			}

			minified, err := json.Marshal(data)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to minify: %w", err)
			}

			result := map[string]interface{}{
				"minified":      string(minified),
				"original_size": len(params.Data),
				"minified_size": len(minified),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// sortKeysTool sorts object keys.
func (p *jsonPack) sortKeysTool() tool.Tool {
	return tool.NewBuilder("json_sort_keys").
		WithDescription("Sort object keys alphabetically").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data      json.RawMessage `json:"data"`
				Recursive bool            `json:"recursive,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			// Parse with ojg to preserve order
			data, err := oj.Parse(params.Data)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to parse data: %w", err)
			}

			sorted := sortKeys(data, params.Recursive)

			// Re-encode with sorted keys
			output, _ := json.Marshal(map[string]interface{}{
				"result": sorted,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sortKeys(data interface{}, recursive bool) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		result := make(map[string]interface{})
		for _, k := range keys {
			if recursive {
				result[k] = sortKeys(v[k], recursive)
			} else {
				result[k] = v[k]
			}
		}
		return result
	case []interface{}:
		if recursive {
			result := make([]interface{}, len(v))
			for i, item := range v {
				result[i] = sortKeys(item, recursive)
			}
			return result
		}
		return v
	default:
		return v
	}
}

// toArrayTool converts an object to an array.
func (p *jsonPack) toArrayTool() tool.Tool {
	return tool.NewBuilder("json_to_array").
		WithDescription("Convert an object to an array of key-value pairs").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var obj map[string]interface{}
			if err := json.Unmarshal(params.Data, &obj); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an object: %w", err)
			}

			var array []map[string]interface{}
			for k, v := range obj {
				array = append(array, map[string]interface{}{
					"key":   k,
					"value": v,
				})
			}

			result := map[string]interface{}{
				"result": array,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// fromArrayTool converts an array to an object.
func (p *jsonPack) fromArrayTool() tool.Tool {
	return tool.NewBuilder("json_from_array").
		WithDescription("Convert an array of key-value pairs to an object").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data     json.RawMessage `json:"data"`
				KeyField string          `json:"key_field,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			keyField := params.KeyField
			if keyField == "" {
				keyField = "key"
			}

			var array []map[string]interface{}
			if err := json.Unmarshal(params.Data, &array); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an array of objects: %w", err)
			}

			obj := make(map[string]interface{})
			for _, item := range array {
				if key, ok := item[keyField].(string); ok {
					if value, exists := item["value"]; exists {
						obj[key] = value
					} else {
						// Use entire object minus key field
						copy := make(map[string]interface{})
						for k, v := range item {
							if k != keyField {
								copy[k] = v
							}
						}
						obj[key] = copy
					}
				}
			}

			result := map[string]interface{}{
				"result": obj,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// keysTool gets all keys from an object.
func (p *jsonPack) keysTool() tool.Tool {
	return tool.NewBuilder("json_keys").
		WithDescription("Get all keys from a JSON object").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var obj map[string]interface{}
			if err := json.Unmarshal(params.Data, &obj); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an object: %w", err)
			}

			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			result := map[string]interface{}{
				"keys":  keys,
				"count": len(keys),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// valuesTool gets all values from an object.
func (p *jsonPack) valuesTool() tool.Tool {
	return tool.NewBuilder("json_values").
		WithDescription("Get all values from a JSON object").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			var obj map[string]interface{}
			if err := json.Unmarshal(params.Data, &obj); err != nil {
				return tool.Result{}, fmt.Errorf("data must be an object: %w", err)
			}

			values := make([]interface{}, 0, len(obj))
			for _, v := range obj {
				values = append(values, v)
			}

			result := map[string]interface{}{
				"values": values,
				"count":  len(values),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
