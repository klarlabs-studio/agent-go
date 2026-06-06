// Package graphql provides GraphQL query building and parsing tools for agents.
package graphql

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

// Pack returns the GraphQL tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("graphql").
		WithDescription("GraphQL query building and parsing tools").
		AddTools(
			buildQueryTool(),
			buildMutationTool(),
			buildSubscriptionTool(),
			parseTool(),
			validateTool(),
			formatTool(),
			extractVariablesTool(),
			buildFragmentTool(),
			minifyTool(),
			introspectionTool(),
			buildBatchTool(),
			analyzeComplexityTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func buildQueryTool() tool.Tool {
	return tool.NewBuilder("graphql_query").
		WithDescription("Build a GraphQL query").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name      string            `json:"name,omitempty"`
				Fields    []string          `json:"fields"`
				Arguments map[string]any    `json:"arguments,omitempty"`
				Variables map[string]string `json:"variables,omitempty"` // name -> type
				Fragments []string          `json:"fragments,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var query strings.Builder

			// Build variable definitions
			if len(params.Variables) > 0 {
				query.WriteString("query")
				if params.Name != "" {
					query.WriteString(" ")
					query.WriteString(params.Name)
				}
				query.WriteString("(")
				first := true
				for name, varType := range params.Variables {
					if !first {
						query.WriteString(", ")
					}
					query.WriteString("$")
					query.WriteString(name)
					query.WriteString(": ")
					query.WriteString(varType)
					first = false
				}
				query.WriteString(") ")
			} else if params.Name != "" {
				query.WriteString("query ")
				query.WriteString(params.Name)
				query.WriteString(" ")
			}

			query.WriteString("{\n")

			// Build fields with arguments
			for _, field := range params.Fields {
				query.WriteString("  ")
				query.WriteString(field)

				if args, ok := params.Arguments[field]; ok {
					query.WriteString("(")
					writeArgs(&query, args)
					query.WriteString(")")
				}

				query.WriteString("\n")
			}

			query.WriteString("}")

			// Add fragments
			if len(params.Fragments) > 0 {
				query.WriteString("\n\n")
				for _, frag := range params.Fragments {
					query.WriteString(frag)
					query.WriteString("\n")
				}
			}

			result := map[string]any{
				"query":     query.String(),
				"type":      "query",
				"name":      params.Name,
				"variables": params.Variables,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func writeArgs(sb *strings.Builder, args any) {
	switch v := args.(type) {
	case map[string]any:
		first := true
		for key, val := range v {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(key)
			sb.WriteString(": ")
			writeValue(sb, val)
			first = false
		}
	default:
		writeValue(sb, v)
	}
}

func writeValue(sb *strings.Builder, val any) {
	switch v := val.(type) {
	case string:
		if strings.HasPrefix(v, "$") {
			sb.WriteString(v)
		} else {
			sb.WriteString("\"")
			sb.WriteString(v)
			sb.WriteString("\"")
		}
	case float64:
		sb.WriteString(fmt.Sprintf("%v", v))
	case bool:
		sb.WriteString(fmt.Sprintf("%v", v))
	case nil:
		sb.WriteString("null")
	case []any:
		sb.WriteString("[")
		for i, item := range v {
			if i > 0 {
				sb.WriteString(", ")
			}
			writeValue(sb, item)
		}
		sb.WriteString("]")
	case map[string]any:
		sb.WriteString("{")
		first := true
		for key, item := range v {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(key)
			sb.WriteString(": ")
			writeValue(sb, item)
			first = false
		}
		sb.WriteString("}")
	default:
		sb.WriteString(fmt.Sprintf("%v", v))
	}
}

func buildMutationTool() tool.Tool {
	return tool.NewBuilder("graphql_mutation").
		WithDescription("Build a GraphQL mutation").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name      string            `json:"name"`
				Input     map[string]any    `json:"input"`
				Fields    []string          `json:"return_fields,omitempty"`
				Variables map[string]string `json:"variables,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var mutation strings.Builder

			mutation.WriteString("mutation")

			// Build variable definitions
			if len(params.Variables) > 0 {
				mutation.WriteString("(")
				first := true
				for name, varType := range params.Variables {
					if !first {
						mutation.WriteString(", ")
					}
					mutation.WriteString("$")
					mutation.WriteString(name)
					mutation.WriteString(": ")
					mutation.WriteString(varType)
					first = false
				}
				mutation.WriteString(")")
			}

			mutation.WriteString(" {\n  ")
			mutation.WriteString(params.Name)
			mutation.WriteString("(")

			// Input arguments
			if len(params.Input) > 0 {
				mutation.WriteString("input: ")
				writeValue(&mutation, params.Input)
			}

			mutation.WriteString(")")

			// Return fields
			if len(params.Fields) > 0 {
				mutation.WriteString(" {\n")
				for _, field := range params.Fields {
					mutation.WriteString("    ")
					mutation.WriteString(field)
					mutation.WriteString("\n")
				}
				mutation.WriteString("  }")
			}

			mutation.WriteString("\n}")

			result := map[string]any{
				"mutation":  mutation.String(),
				"type":      "mutation",
				"name":      params.Name,
				"variables": params.Variables,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func buildSubscriptionTool() tool.Tool {
	return tool.NewBuilder("graphql_subscription").
		WithDescription("Build a GraphQL subscription").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name      string            `json:"name"`
				Fields    []string          `json:"fields"`
				Arguments map[string]any    `json:"arguments,omitempty"`
				Variables map[string]string `json:"variables,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var sub strings.Builder

			sub.WriteString("subscription")

			// Build variable definitions
			if len(params.Variables) > 0 {
				sub.WriteString("(")
				first := true
				for name, varType := range params.Variables {
					if !first {
						sub.WriteString(", ")
					}
					sub.WriteString("$")
					sub.WriteString(name)
					sub.WriteString(": ")
					sub.WriteString(varType)
					first = false
				}
				sub.WriteString(")")
			}

			sub.WriteString(" {\n  ")
			sub.WriteString(params.Name)

			// Arguments
			if len(params.Arguments) > 0 {
				sub.WriteString("(")
				writeArgs(&sub, params.Arguments)
				sub.WriteString(")")
			}

			// Fields
			if len(params.Fields) > 0 {
				sub.WriteString(" {\n")
				for _, field := range params.Fields {
					sub.WriteString("    ")
					sub.WriteString(field)
					sub.WriteString("\n")
				}
				sub.WriteString("  }")
			}

			sub.WriteString("\n}")

			result := map[string]any{
				"subscription": sub.String(),
				"type":         "subscription",
				"name":         params.Name,
				"variables":    params.Variables,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("graphql_parse").
		WithDescription("Parse a GraphQL query and extract structure").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			query := strings.TrimSpace(params.Query)

			// Determine operation type
			opType := "query"
			if strings.HasPrefix(query, "mutation") {
				opType = "mutation"
			} else if strings.HasPrefix(query, "subscription") {
				opType = "subscription"
			}

			// Extract operation name
			opNamePattern := regexp.MustCompile(`^(?:query|mutation|subscription)\s+(\w+)`)
			opNameMatch := opNamePattern.FindStringSubmatch(query)
			opName := ""
			if len(opNameMatch) > 1 {
				opName = opNameMatch[1]
			}

			// Extract variables
			varPattern := regexp.MustCompile(`\$(\w+)\s*:\s*(\w+!?)`)
			varMatches := varPattern.FindAllStringSubmatch(query, -1)
			variables := make(map[string]string)
			for _, match := range varMatches {
				variables[match[1]] = match[2]
			}

			// Extract field names (simplified)
			fieldPattern := regexp.MustCompile(`\b(\w+)\s*(?:\(|{|\n)`)
			fieldMatches := fieldPattern.FindAllStringSubmatch(query, -1)
			var fields []string
			seen := make(map[string]bool)
			keywords := map[string]bool{"query": true, "mutation": true, "subscription": true, "fragment": true, "on": true}
			for _, match := range fieldMatches {
				field := match[1]
				if !keywords[field] && !seen[field] {
					fields = append(fields, field)
					seen[field] = true
				}
			}

			// Extract fragments
			fragPattern := regexp.MustCompile(`fragment\s+(\w+)\s+on\s+(\w+)`)
			fragMatches := fragPattern.FindAllStringSubmatch(query, -1)
			var fragments []map[string]string
			for _, match := range fragMatches {
				fragments = append(fragments, map[string]string{
					"name": match[1],
					"type": match[2],
				})
			}

			result := map[string]any{
				"operation_type": opType,
				"operation_name": opName,
				"variables":      variables,
				"fields":         fields,
				"fragments":      fragments,
				"valid":          true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("graphql_validate").
		WithDescription("Validate GraphQL query syntax").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			query := strings.TrimSpace(params.Query)
			var errors []string

			// Basic syntax checks
			// Check braces balance
			braceCount := 0
			parenCount := 0
			for _, r := range query {
				switch r {
				case '{':
					braceCount++
				case '}':
					braceCount--
				case '(':
					parenCount++
				case ')':
					parenCount--
				}
			}

			if braceCount != 0 {
				errors = append(errors, "Unbalanced braces")
			}
			if parenCount != 0 {
				errors = append(errors, "Unbalanced parentheses")
			}

			// Check for valid operation type
			if !strings.HasPrefix(query, "query") &&
				!strings.HasPrefix(query, "mutation") &&
				!strings.HasPrefix(query, "subscription") &&
				!strings.HasPrefix(query, "{") &&
				!strings.HasPrefix(query, "fragment") {
				errors = append(errors, "Invalid operation type")
			}

			// Check for empty selection sets
			if regexp.MustCompile(`{\s*}`).MatchString(query) {
				errors = append(errors, "Empty selection set")
			}

			result := map[string]any{
				"valid":  len(errors) == 0,
				"errors": errors,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatTool() tool.Tool {
	return tool.NewBuilder("graphql_format").
		WithDescription("Format GraphQL query with proper indentation").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Simple formatting: normalize whitespace and indent
			query := strings.TrimSpace(params.Query)

			// Remove extra whitespace
			query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

			// Add newlines after { and before }
			var formatted strings.Builder
			indent := 0
			inString := false

			for i := 0; i < len(query); i++ {
				c := query[i]

				if c == '"' && (i == 0 || query[i-1] != '\\') {
					inString = !inString
				}

				if inString {
					formatted.WriteByte(c)
					continue
				}

				switch c {
				case '{':
					formatted.WriteString(" {\n")
					indent++
					formatted.WriteString(strings.Repeat("  ", indent))
				case '}':
					formatted.WriteString("\n")
					indent--
					formatted.WriteString(strings.Repeat("  ", indent))
					formatted.WriteByte(c)
				case ',':
					formatted.WriteString(",\n")
					formatted.WriteString(strings.Repeat("  ", indent))
					// Skip following space
					if i+1 < len(query) && query[i+1] == ' ' {
						i++
					}
				case ' ':
					// Skip multiple spaces
					if i > 0 && query[i-1] != ' ' && query[i-1] != '\n' {
						formatted.WriteByte(c)
					}
				default:
					formatted.WriteByte(c)
				}
			}

			result := map[string]any{
				"formatted": formatted.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractVariablesTool() tool.Tool {
	return tool.NewBuilder("graphql_extract_variables").
		WithDescription("Extract variable definitions from a query").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Extract variable definitions
			defPattern := regexp.MustCompile(`\$(\w+)\s*:\s*(\w+!?(?:\[\w+!?\]!?)?)(?:\s*=\s*([^,)]+))?`)
			defMatches := defPattern.FindAllStringSubmatch(params.Query, -1)

			var variables []map[string]any
			for _, match := range defMatches {
				variable := map[string]any{
					"name":     match[1],
					"type":     match[2],
					"required": strings.HasSuffix(match[2], "!"),
				}
				if len(match) > 3 && match[3] != "" {
					variable["default"] = strings.TrimSpace(match[3])
				}
				variables = append(variables, variable)
			}

			// Extract variable usages
			usagePattern := regexp.MustCompile(`\$(\w+)`)
			usageMatches := usagePattern.FindAllStringSubmatch(params.Query, -1)
			usages := make(map[string]int)
			for _, match := range usageMatches {
				usages[match[1]]++
			}

			result := map[string]any{
				"variables": variables,
				"usages":    usages,
				"count":     len(variables),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func buildFragmentTool() tool.Tool {
	return tool.NewBuilder("graphql_fragment").
		WithDescription("Build a GraphQL fragment").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name   string   `json:"name"`
				Type   string   `json:"type"`
				Fields []string `json:"fields"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var fragment strings.Builder
			fragment.WriteString("fragment ")
			fragment.WriteString(params.Name)
			fragment.WriteString(" on ")
			fragment.WriteString(params.Type)
			fragment.WriteString(" {\n")

			for _, field := range params.Fields {
				fragment.WriteString("  ")
				fragment.WriteString(field)
				fragment.WriteString("\n")
			}

			fragment.WriteString("}")

			// Generate spread syntax
			spread := "..." + params.Name

			result := map[string]any{
				"fragment": fragment.String(),
				"spread":   spread,
				"name":     params.Name,
				"type":     params.Type,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func minifyTool() tool.Tool {
	return tool.NewBuilder("graphql_minify").
		WithDescription("Minify GraphQL query").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Remove comments
			query := regexp.MustCompile(`#[^\n]*`).ReplaceAllString(params.Query, "")

			// Collapse whitespace
			query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

			// Remove spaces around punctuation
			query = regexp.MustCompile(`\s*([{}():])\s*`).ReplaceAllString(query, "$1")

			// Remove trailing commas
			query = regexp.MustCompile(`,\s*}`).ReplaceAllString(query, "}")

			query = strings.TrimSpace(query)

			result := map[string]any{
				"minified":        query,
				"original_length": len(params.Query),
				"minified_length": len(query),
				"reduction":       float64(len(params.Query)-len(query)) / float64(len(params.Query)) * 100,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func introspectionTool() tool.Tool {
	return tool.NewBuilder("graphql_introspection_query").
		WithDescription("Generate GraphQL introspection query").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Type string `json:"type,omitempty"` // full, types, directives
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var query string
			switch strings.ToLower(params.Type) {
			case "types":
				query = `{
  __schema {
    types {
      name
      kind
      description
      fields {
        name
        type {
          name
          kind
        }
      }
    }
  }
}`
			case "directives":
				query = `{
  __schema {
    directives {
      name
      description
      locations
      args {
        name
        type {
          name
          kind
        }
      }
    }
  }
}`
			default:
				query = `{
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      kind
      name
      description
      fields(includeDeprecated: true) {
        name
        description
        args {
          name
          description
          type { ...TypeRef }
          defaultValue
        }
        type { ...TypeRef }
        isDeprecated
        deprecationReason
      }
      inputFields {
        name
        description
        type { ...TypeRef }
        defaultValue
      }
      interfaces { ...TypeRef }
      enumValues(includeDeprecated: true) {
        name
        description
        isDeprecated
        deprecationReason
      }
      possibleTypes { ...TypeRef }
    }
    directives {
      name
      description
      locations
      args {
        name
        description
        type { ...TypeRef }
        defaultValue
      }
    }
  }
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
      }
    }
  }
}`
			}

			result := map[string]any{
				"query": query,
				"type":  params.Type,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func buildBatchTool() tool.Tool {
	return tool.NewBuilder("graphql_batch").
		WithDescription("Build a batch of GraphQL queries").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Queries []struct {
					Name      string         `json:"name"`
					Query     string         `json:"query"`
					Variables map[string]any `json:"variables,omitempty"`
				} `json:"queries"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var batch []map[string]any
			for _, q := range params.Queries {
				item := map[string]any{
					"query": q.Query,
				}
				if q.Name != "" {
					item["operationName"] = q.Name
				}
				if len(q.Variables) > 0 {
					item["variables"] = q.Variables
				}
				batch = append(batch, item)
			}

			batchJSON, _ := json.Marshal(batch)

			result := map[string]any{
				"batch": batch,
				"json":  string(batchJSON),
				"count": len(batch),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func analyzeComplexityTool() tool.Tool {
	return tool.NewBuilder("graphql_analyze_complexity").
		WithDescription("Analyze GraphQL query complexity").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			query := params.Query

			// Count depth
			maxDepth := 0
			currentDepth := 0
			for _, c := range query {
				if c == '{' {
					currentDepth++
					if currentDepth > maxDepth {
						maxDepth = currentDepth
					}
				} else if c == '}' {
					currentDepth--
				}
			}

			// Count fields
			fieldPattern := regexp.MustCompile(`\b\w+\s*(?:{|\(|$)`)
			fieldCount := len(fieldPattern.FindAllString(query, -1))

			// Count arguments
			argPattern := regexp.MustCompile(`\w+\s*:`)
			argCount := len(argPattern.FindAllString(query, -1))

			// Count variables
			varPattern := regexp.MustCompile(`\$\w+`)
			varCount := len(varPattern.FindAllString(query, -1))

			// Check for pagination/lists (potential N+1)
			hasFirst := strings.Contains(query, "first:")
			hasAfter := strings.Contains(query, "after:")
			hasLast := strings.Contains(query, "last:")
			hasBefore := strings.Contains(query, "before:")

			// Simple complexity score
			complexity := fieldCount + (maxDepth * 2) + argCount

			result := map[string]any{
				"depth":      maxDepth,
				"fields":     fieldCount,
				"arguments":  argCount,
				"variables":  varCount,
				"complexity": complexity,
				"pagination": map[string]bool{
					"has_first":  hasFirst,
					"has_after":  hasAfter,
					"has_last":   hasLast,
					"has_before": hasBefore,
				},
				"recommendations": getRecommendations(maxDepth, fieldCount, complexity),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getRecommendations(depth, fields, complexity int) []string {
	var recs []string

	if depth > 5 {
		recs = append(recs, "Query depth exceeds 5 levels - consider flattening or using fragments")
	}

	if fields > 20 {
		recs = append(recs, "Large number of fields - consider splitting into multiple queries")
	}

	if complexity > 50 {
		recs = append(recs, "High complexity score - may cause performance issues")
	}

	if len(recs) == 0 {
		recs = append(recs, "Query complexity is within acceptable limits")
	}

	return recs
}
