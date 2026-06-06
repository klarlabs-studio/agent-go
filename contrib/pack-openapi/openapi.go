// Package openapi provides OpenAPI specification tools for agents.
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the OpenAPI tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("openapi").
		WithDescription("OpenAPI specification tools").
		AddTools(
			parseTool(),
			validateTool(),
			extractPathsTool(),
			extractSchemasTool(),
			generatePathTool(),
			generateSchemaTool(),
			convertTool(),
			mergeTool(),
			diffTool(),
			mockTool(),
			infoTool(),
			securityTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("openapi_parse").
		WithDescription("Parse OpenAPI specification").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Spec string `json:"spec"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var spec map[string]any
			if err := json.Unmarshal([]byte(params.Spec), &spec); err != nil {
				// Try YAML-like parsing (simplified)
				return tool.Result{}, fmt.Errorf("failed to parse spec: %w", err)
			}

			// Extract version
			version := ""
			if v, ok := spec["openapi"].(string); ok {
				version = v
			} else if v, ok := spec["swagger"].(string); ok {
				version = v
			}

			// Extract info
			info := map[string]any{}
			if i, ok := spec["info"].(map[string]any); ok {
				info = i
			}

			// Count paths
			pathCount := 0
			if paths, ok := spec["paths"].(map[string]any); ok {
				pathCount = len(paths)
			}

			// Count schemas
			schemaCount := 0
			if components, ok := spec["components"].(map[string]any); ok {
				if schemas, ok := components["schemas"].(map[string]any); ok {
					schemaCount = len(schemas)
				}
			} else if definitions, ok := spec["definitions"].(map[string]any); ok {
				schemaCount = len(definitions)
			}

			result := map[string]any{
				"version":      version,
				"info":         info,
				"path_count":   pathCount,
				"schema_count": schemaCount,
				"valid":        version != "",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("openapi_validate").
		WithDescription("Validate OpenAPI specification").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Spec string `json:"spec"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var spec map[string]any
			if err := json.Unmarshal([]byte(params.Spec), &spec); err != nil {
				result := map[string]any{
					"valid":  false,
					"errors": []string{"Failed to parse JSON"},
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var errors []string
			var warnings []string

			// Check version
			hasOpenAPI := false
			if v, ok := spec["openapi"].(string); ok {
				hasOpenAPI = true
				if !strings.HasPrefix(v, "3.") {
					warnings = append(warnings, fmt.Sprintf("OpenAPI version %s may have limited support", v))
				}
			} else if _, ok := spec["swagger"]; ok {
				hasOpenAPI = true
				warnings = append(warnings, "Swagger 2.0 spec detected - consider upgrading to OpenAPI 3.x")
			}

			if !hasOpenAPI {
				errors = append(errors, "Missing 'openapi' or 'swagger' version field")
			}

			// Check required fields
			if _, ok := spec["info"]; !ok {
				errors = append(errors, "Missing 'info' field")
			} else {
				info := spec["info"].(map[string]any)
				if _, ok := info["title"]; !ok {
					errors = append(errors, "Missing 'info.title' field")
				}
				if _, ok := info["version"]; !ok {
					errors = append(errors, "Missing 'info.version' field")
				}
			}

			if _, ok := spec["paths"]; !ok {
				errors = append(errors, "Missing 'paths' field")
			}

			// Check for undefined refs
			specJSON, _ := json.Marshal(spec)
			refPattern := regexp.MustCompile(`"\$ref"\s*:\s*"([^"]+)"`)
			refs := refPattern.FindAllStringSubmatch(string(specJSON), -1)
			for _, ref := range refs {
				refPath := ref[1]
				if strings.HasPrefix(refPath, "#/") {
					// Check if ref exists
					parts := strings.Split(strings.TrimPrefix(refPath, "#/"), "/")
					current := spec
					found := true
					for _, part := range parts {
						if m, ok := current[part].(map[string]any); ok {
							current = m
						} else {
							found = false
							break
						}
					}
					if !found {
						warnings = append(warnings, fmt.Sprintf("Potentially undefined $ref: %s", refPath))
					}
				}
			}

			result := map[string]any{
				"valid":    len(errors) == 0,
				"errors":   errors,
				"warnings": warnings,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractPathsTool() tool.Tool {
	return tool.NewBuilder("openapi_extract_paths").
		WithDescription("Extract paths/endpoints from spec").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Spec   string `json:"spec"`
				Filter string `json:"filter,omitempty"` // Optional path filter
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var spec map[string]any
			if err := json.Unmarshal([]byte(params.Spec), &spec); err != nil {
				return tool.Result{}, err
			}

			paths, ok := spec["paths"].(map[string]any)
			if !ok {
				result := map[string]any{
					"endpoints": []any{},
					"count":     0,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var endpoints []map[string]any
			for path, methods := range paths {
				if params.Filter != "" && !strings.Contains(path, params.Filter) {
					continue
				}

				methodsMap, ok := methods.(map[string]any)
				if !ok {
					continue
				}

				for method, details := range methodsMap {
					if method == "parameters" || method == "servers" {
						continue
					}

					endpoint := map[string]any{
						"path":   path,
						"method": strings.ToUpper(method),
					}

					if detailsMap, ok := details.(map[string]any); ok {
						if summary, ok := detailsMap["summary"].(string); ok {
							endpoint["summary"] = summary
						}
						if operationId, ok := detailsMap["operationId"].(string); ok {
							endpoint["operation_id"] = operationId
						}
						if tags, ok := detailsMap["tags"].([]any); ok {
							endpoint["tags"] = tags
						}
					}

					endpoints = append(endpoints, endpoint)
				}
			}

			// Sort by path then method
			sort.Slice(endpoints, func(i, j int) bool {
				pathI := endpoints[i]["path"].(string)
				pathJ := endpoints[j]["path"].(string)
				if pathI != pathJ {
					return pathI < pathJ
				}
				return endpoints[i]["method"].(string) < endpoints[j]["method"].(string)
			})

			result := map[string]any{
				"endpoints": endpoints,
				"count":     len(endpoints),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractSchemasTool() tool.Tool {
	return tool.NewBuilder("openapi_extract_schemas").
		WithDescription("Extract schemas/models from spec").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Spec   string `json:"spec"`
				Filter string `json:"filter,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var spec map[string]any
			if err := json.Unmarshal([]byte(params.Spec), &spec); err != nil {
				return tool.Result{}, err
			}

			var schemas map[string]any

			// OpenAPI 3.x
			if components, ok := spec["components"].(map[string]any); ok {
				if s, ok := components["schemas"].(map[string]any); ok {
					schemas = s
				}
			}
			// Swagger 2.0
			if schemas == nil {
				if defs, ok := spec["definitions"].(map[string]any); ok {
					schemas = defs
				}
			}

			if schemas == nil {
				result := map[string]any{
					"schemas": []any{},
					"count":   0,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var schemaList []map[string]any
			for name, schema := range schemas {
				if params.Filter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(params.Filter)) {
					continue
				}

				schemaMap, ok := schema.(map[string]any)
				if !ok {
					continue
				}

				schemaInfo := map[string]any{
					"name": name,
				}

				if t, ok := schemaMap["type"].(string); ok {
					schemaInfo["type"] = t
				}
				if props, ok := schemaMap["properties"].(map[string]any); ok {
					var propNames []string
					for propName := range props {
						propNames = append(propNames, propName)
					}
					sort.Strings(propNames)
					schemaInfo["properties"] = propNames
				}
				if required, ok := schemaMap["required"].([]any); ok {
					schemaInfo["required"] = required
				}
				if desc, ok := schemaMap["description"].(string); ok {
					schemaInfo["description"] = desc
				}

				schemaList = append(schemaList, schemaInfo)
			}

			// Sort by name
			sort.Slice(schemaList, func(i, j int) bool {
				return schemaList[i]["name"].(string) < schemaList[j]["name"].(string)
			})

			result := map[string]any{
				"schemas": schemaList,
				"count":   len(schemaList),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generatePathTool() tool.Tool {
	return tool.NewBuilder("openapi_generate_path").
		WithDescription("Generate OpenAPI path definition").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path        string   `json:"path"`
				Method      string   `json:"method"`
				Summary     string   `json:"summary,omitempty"`
				Description string   `json:"description,omitempty"`
				Tags        []string `json:"tags,omitempty"`
				OperationID string   `json:"operation_id,omitempty"`
				Parameters  []struct {
					Name     string `json:"name"`
					In       string `json:"in"` // query, path, header, cookie
					Required bool   `json:"required,omitempty"`
					Type     string `json:"type,omitempty"`
				} `json:"parameters,omitempty"`
				RequestBody string   `json:"request_body_ref,omitempty"`
				Responses   []string `json:"response_codes,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			method := strings.ToLower(params.Method)
			if method == "" {
				method = "get"
			}

			operation := map[string]any{}

			if params.Summary != "" {
				operation["summary"] = params.Summary
			}
			if params.Description != "" {
				operation["description"] = params.Description
			}
			if len(params.Tags) > 0 {
				operation["tags"] = params.Tags
			}
			if params.OperationID != "" {
				operation["operationId"] = params.OperationID
			}

			if len(params.Parameters) > 0 {
				var paramList []map[string]any
				for _, p := range params.Parameters {
					param := map[string]any{
						"name":     p.Name,
						"in":       p.In,
						"required": p.Required,
					}
					if p.Type != "" {
						param["schema"] = map[string]string{"type": p.Type}
					}
					paramList = append(paramList, param)
				}
				operation["parameters"] = paramList
			}

			if params.RequestBody != "" {
				operation["requestBody"] = map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]string{"$ref": params.RequestBody},
						},
					},
				}
			}

			// Default responses
			responses := map[string]any{
				"200": map[string]string{"description": "Successful response"},
			}
			for _, code := range params.Responses {
				if code != "200" {
					responses[code] = map[string]string{"description": "Response " + code}
				}
			}
			operation["responses"] = responses

			pathDef := map[string]any{
				params.Path: map[string]any{
					method: operation,
				},
			}

			pathJSON, _ := json.MarshalIndent(pathDef, "", "  ")

			result := map[string]any{
				"path":       params.Path,
				"method":     method,
				"definition": pathDef,
				"json":       string(pathJSON),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateSchemaTool() tool.Tool {
	return tool.NewBuilder("openapi_generate_schema").
		WithDescription("Generate OpenAPI schema definition").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
				Properties  []struct {
					Name        string `json:"name"`
					Type        string `json:"type"`
					Format      string `json:"format,omitempty"`
					Description string `json:"description,omitempty"`
					Required    bool   `json:"required,omitempty"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			schema := map[string]any{
				"type": "object",
			}

			if params.Description != "" {
				schema["description"] = params.Description
			}

			properties := map[string]any{}
			var required []string

			for _, prop := range params.Properties {
				propDef := map[string]any{
					"type": prop.Type,
				}
				if prop.Format != "" {
					propDef["format"] = prop.Format
				}
				if prop.Description != "" {
					propDef["description"] = prop.Description
				}
				properties[prop.Name] = propDef

				if prop.Required {
					required = append(required, prop.Name)
				}
			}

			schema["properties"] = properties
			if len(required) > 0 {
				schema["required"] = required
			}

			schemaDef := map[string]any{
				params.Name: schema,
			}

			schemaJSON, _ := json.MarshalIndent(schemaDef, "", "  ")

			result := map[string]any{
				"name":       params.Name,
				"definition": schemaDef,
				"json":       string(schemaJSON),
				"ref":        "#/components/schemas/" + params.Name,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func convertTool() tool.Tool {
	return tool.NewBuilder("openapi_convert").
		WithDescription("Convert between OpenAPI versions").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Spec      string `json:"spec"`
				ToVersion string `json:"to_version"` // 3.0, 3.1
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var spec map[string]any
			if err := json.Unmarshal([]byte(params.Spec), &spec); err != nil {
				return tool.Result{}, err
			}

			// Detect current version
			currentVersion := ""
			if v, ok := spec["openapi"].(string); ok {
				currentVersion = v
			} else if v, ok := spec["swagger"].(string); ok {
				currentVersion = "swagger-" + v
			}

			// Basic conversion (simplified)
			if strings.HasPrefix(currentVersion, "swagger") {
				// Convert Swagger 2.0 to OpenAPI 3.0
				spec["openapi"] = params.ToVersion
				delete(spec, "swagger")

				// Move definitions to components/schemas
				if defs, ok := spec["definitions"]; ok {
					spec["components"] = map[string]any{
						"schemas": defs,
					}
					delete(spec, "definitions")
				}

				// Note: Full conversion would require more transformations
			}

			// Update version
			spec["openapi"] = params.ToVersion

			convertedJSON, _ := json.MarshalIndent(spec, "", "  ")

			result := map[string]any{
				"from_version": currentVersion,
				"to_version":   params.ToVersion,
				"converted":    string(convertedJSON),
				"note":         "Basic conversion - manual review recommended",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mergeTool() tool.Tool {
	return tool.NewBuilder("openapi_merge").
		WithDescription("Merge multiple OpenAPI specs").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Specs []string `json:"specs"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if len(params.Specs) == 0 {
				return tool.Result{}, fmt.Errorf("no specs provided")
			}

			// Parse first spec as base
			var merged map[string]any
			if err := json.Unmarshal([]byte(params.Specs[0]), &merged); err != nil {
				return tool.Result{}, err
			}

			// Merge additional specs
			for i := 1; i < len(params.Specs); i++ {
				var spec map[string]any
				if err := json.Unmarshal([]byte(params.Specs[i]), &spec); err != nil {
					continue
				}

				// Merge paths
				if paths, ok := spec["paths"].(map[string]any); ok {
					if mergedPaths, ok := merged["paths"].(map[string]any); ok {
						for path, methods := range paths {
							mergedPaths[path] = methods
						}
					}
				}

				// Merge schemas
				if components, ok := spec["components"].(map[string]any); ok {
					if schemas, ok := components["schemas"].(map[string]any); ok {
						if mergedComponents, ok := merged["components"].(map[string]any); !ok {
							merged["components"] = map[string]any{"schemas": schemas}
						} else if mergedSchemas, ok := mergedComponents["schemas"].(map[string]any); ok {
							for name, schema := range schemas {
								mergedSchemas[name] = schema
							}
						}
					}
				}
			}

			mergedJSON, _ := json.MarshalIndent(merged, "", "  ")

			result := map[string]any{
				"merged":      string(mergedJSON),
				"specs_count": len(params.Specs),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func diffTool() tool.Tool {
	return tool.NewBuilder("openapi_diff").
		WithDescription("Compare two OpenAPI specs").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Spec1 string `json:"spec1"`
				Spec2 string `json:"spec2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var spec1, spec2 map[string]any
			if err := json.Unmarshal([]byte(params.Spec1), &spec1); err != nil {
				return tool.Result{}, err
			}
			if err := json.Unmarshal([]byte(params.Spec2), &spec2); err != nil {
				return tool.Result{}, err
			}

			var added, removed, modified []string

			// Compare paths
			paths1, _ := spec1["paths"].(map[string]any)
			paths2, _ := spec2["paths"].(map[string]any)

			for path := range paths2 {
				if _, exists := paths1[path]; !exists {
					added = append(added, "path: "+path)
				}
			}

			for path := range paths1 {
				if _, exists := paths2[path]; !exists {
					removed = append(removed, "path: "+path)
				}
			}

			// Compare schemas
			var schemas1, schemas2 map[string]any
			if components1, ok := spec1["components"].(map[string]any); ok {
				schemas1, _ = components1["schemas"].(map[string]any)
			}
			if components2, ok := spec2["components"].(map[string]any); ok {
				schemas2, _ = components2["schemas"].(map[string]any)
			}

			for name := range schemas2 {
				if _, exists := schemas1[name]; !exists {
					added = append(added, "schema: "+name)
				}
			}

			for name := range schemas1 {
				if _, exists := schemas2[name]; !exists {
					removed = append(removed, "schema: "+name)
				}
			}

			result := map[string]any{
				"added":         added,
				"removed":       removed,
				"modified":      modified,
				"added_count":   len(added),
				"removed_count": len(removed),
				"breaking":      len(removed) > 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mockTool() tool.Tool {
	return tool.NewBuilder("openapi_mock").
		WithDescription("Generate mock data from schema").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Schema map[string]any `json:"schema"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			mock := generateMockData(params.Schema)

			mockJSON, _ := json.MarshalIndent(mock, "", "  ")

			result := map[string]any{
				"mock": mock,
				"json": string(mockJSON),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateMockData(schema map[string]any) any {
	schemaType, _ := schema["type"].(string)
	format, _ := schema["format"].(string)

	switch schemaType {
	case "string":
		switch format {
		case "email":
			return "user@example.com"
		case "date":
			return "2024-01-15"
		case "date-time":
			return "2024-01-15T10:30:00Z"
		case "uri":
			return "https://example.com"
		case "uuid":
			return "550e8400-e29b-41d4-a716-446655440000"
		default:
			return "string"
		}
	case "integer":
		return 42
	case "number":
		return 3.14
	case "boolean":
		return true
	case "array":
		if items, ok := schema["items"].(map[string]any); ok {
			return []any{generateMockData(items)}
		}
		return []any{}
	case "object":
		obj := map[string]any{}
		if props, ok := schema["properties"].(map[string]any); ok {
			for name, propSchema := range props {
				if ps, ok := propSchema.(map[string]any); ok {
					obj[name] = generateMockData(ps)
				}
			}
		}
		return obj
	default:
		return nil
	}
}

func infoTool() tool.Tool {
	return tool.NewBuilder("openapi_info").
		WithDescription("Get API info from spec").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Spec string `json:"spec"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var spec map[string]any
			if err := json.Unmarshal([]byte(params.Spec), &spec); err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{}

			// Version
			if v, ok := spec["openapi"].(string); ok {
				result["openapi_version"] = v
			} else if v, ok := spec["swagger"].(string); ok {
				result["swagger_version"] = v
			}

			// Info
			if info, ok := spec["info"].(map[string]any); ok {
				result["title"] = info["title"]
				result["version"] = info["version"]
				result["description"] = info["description"]
				if contact, ok := info["contact"].(map[string]any); ok {
					result["contact"] = contact
				}
				if license, ok := info["license"].(map[string]any); ok {
					result["license"] = license
				}
			}

			// Servers
			if servers, ok := spec["servers"].([]any); ok {
				result["servers"] = servers
			}

			// Tags
			if tags, ok := spec["tags"].([]any); ok {
				result["tags"] = tags
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func securityTool() tool.Tool {
	return tool.NewBuilder("openapi_security").
		WithDescription("Extract security schemes from spec").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Spec string `json:"spec"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var spec map[string]any
			if err := json.Unmarshal([]byte(params.Spec), &spec); err != nil {
				return tool.Result{}, err
			}

			var schemes map[string]any

			// OpenAPI 3.x
			if components, ok := spec["components"].(map[string]any); ok {
				if s, ok := components["securitySchemes"].(map[string]any); ok {
					schemes = s
				}
			}
			// Swagger 2.0
			if schemes == nil {
				if s, ok := spec["securityDefinitions"].(map[string]any); ok {
					schemes = s
				}
			}

			// Global security requirements
			globalSecurity, _ := spec["security"].([]any)

			result := map[string]any{
				"schemes":         schemes,
				"scheme_count":    len(schemes),
				"global_security": globalSecurity,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
