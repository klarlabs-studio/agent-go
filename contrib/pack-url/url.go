// Package url provides URL parsing and manipulation tools for agents.
package url

import (
	"context"
	"encoding/json"
	"net/url"
	"path"
	"sort"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the URL tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("url").
		WithDescription("URL parsing and manipulation tools").
		AddTools(
			parseTool(),
			buildTool(),
			joinTool(),
			resolveTool(),
			queryGetTool(),
			querySetTool(),
			queryDeleteTool(),
			queryParseTool(),
			queryBuildTool(),
			encodeTool(),
			decodeTool(),
			validateTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("url_parse").
		WithDescription("Parse a URL into components").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parsed, err := url.Parse(params.URL)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Parse query parameters
			queryParams := make(map[string]any)
			for key, values := range parsed.Query() {
				if len(values) == 1 {
					queryParams[key] = values[0]
				} else {
					queryParams[key] = values
				}
			}

			result := map[string]any{
				"valid":    true,
				"scheme":   parsed.Scheme,
				"host":     parsed.Host,
				"hostname": parsed.Hostname(),
				"port":     parsed.Port(),
				"path":     parsed.Path,
				"query":    parsed.RawQuery,
				"fragment": parsed.Fragment,
				"user":     nil,
				"params":   queryParams,
			}

			if parsed.User != nil {
				password, hasPassword := parsed.User.Password()
				result["user"] = map[string]any{
					"username":     parsed.User.Username(),
					"password":     password,
					"has_password": hasPassword,
				}
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func buildTool() tool.Tool {
	return tool.NewBuilder("url_build").
		WithDescription("Build a URL from components").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Scheme   string            `json:"scheme,omitempty"`
				Host     string            `json:"host"`
				Port     string            `json:"port,omitempty"`
				Path     string            `json:"path,omitempty"`
				Query    map[string]string `json:"query,omitempty"`
				Fragment string            `json:"fragment,omitempty"`
				Username string            `json:"username,omitempty"`
				Password string            `json:"password,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			u := &url.URL{
				Scheme:   params.Scheme,
				Host:     params.Host,
				Path:     params.Path,
				Fragment: params.Fragment,
			}

			if params.Scheme == "" {
				u.Scheme = "https"
			}

			if params.Port != "" {
				u.Host = params.Host + ":" + params.Port
			}

			if params.Username != "" {
				if params.Password != "" {
					u.User = url.UserPassword(params.Username, params.Password)
				} else {
					u.User = url.User(params.Username)
				}
			}

			if len(params.Query) > 0 {
				q := url.Values{}
				for k, v := range params.Query {
					q.Set(k, v)
				}
				u.RawQuery = q.Encode()
			}

			result := map[string]any{
				"url": u.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func joinTool() tool.Tool {
	return tool.NewBuilder("url_join").
		WithDescription("Join URL path segments").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Base     string   `json:"base"`
				Segments []string `json:"segments"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parsed, err := url.Parse(params.Base)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Join path segments
			segments := append([]string{parsed.Path}, params.Segments...)
			parsed.Path = path.Join(segments...)

			result := map[string]any{
				"url":  parsed.String(),
				"path": parsed.Path,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func resolveTool() tool.Tool {
	return tool.NewBuilder("url_resolve").
		WithDescription("Resolve a relative URL against a base URL").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Base     string `json:"base"`
				Relative string `json:"relative"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			base, err := url.Parse(params.Base)
			if err != nil {
				result := map[string]any{
					"error": "invalid base URL: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			ref, err := url.Parse(params.Relative)
			if err != nil {
				result := map[string]any{
					"error": "invalid relative URL: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			resolved := base.ResolveReference(ref)

			result := map[string]any{
				"url": resolved.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func queryGetTool() tool.Tool {
	return tool.NewBuilder("url_query_get").
		WithDescription("Get a query parameter from a URL").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL string `json:"url"`
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parsed, err := url.Parse(params.URL)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			values := parsed.Query()[params.Key]

			result := map[string]any{
				"key":    params.Key,
				"values": values,
				"value":  nil,
				"found":  len(values) > 0,
			}
			if len(values) > 0 {
				result["value"] = values[0]
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func querySetTool() tool.Tool {
	return tool.NewBuilder("url_query_set").
		WithDescription("Set query parameters on a URL").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL    string            `json:"url"`
				Params map[string]string `json:"params"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parsed, err := url.Parse(params.URL)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			q := parsed.Query()
			for k, v := range params.Params {
				q.Set(k, v)
			}
			parsed.RawQuery = q.Encode()

			result := map[string]any{
				"url": parsed.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func queryDeleteTool() tool.Tool {
	return tool.NewBuilder("url_query_delete").
		WithDescription("Remove query parameters from a URL").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL  string   `json:"url"`
				Keys []string `json:"keys"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parsed, err := url.Parse(params.URL)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			q := parsed.Query()
			for _, key := range params.Keys {
				q.Del(key)
			}
			parsed.RawQuery = q.Encode()

			result := map[string]any{
				"url":     parsed.String(),
				"deleted": params.Keys,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func queryParseTool() tool.Tool {
	return tool.NewBuilder("url_query_parse").
		WithDescription("Parse a query string into key-value pairs").
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

			// Remove leading ? if present
			query := strings.TrimPrefix(params.Query, "?")

			values, err := url.ParseQuery(query)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Convert to simpler format
			parsed := make(map[string]any)
			for key, vals := range values {
				if len(vals) == 1 {
					parsed[key] = vals[0]
				} else {
					parsed[key] = vals
				}
			}

			result := map[string]any{
				"params": parsed,
				"count":  len(values),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func queryBuildTool() tool.Tool {
	return tool.NewBuilder("url_query_build").
		WithDescription("Build a query string from key-value pairs").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Params map[string]any `json:"params"`
				Sorted bool           `json:"sorted,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			values := url.Values{}
			for k, v := range params.Params {
				switch val := v.(type) {
				case string:
					values.Add(k, val)
				case []any:
					for _, item := range val {
						if s, ok := item.(string); ok {
							values.Add(k, s)
						}
					}
				case float64:
					// Convert number to string
					numBytes, _ := json.Marshal(val)
					values.Add(k, string(numBytes))
				}
			}

			var query string
			if params.Sorted {
				// Sort keys for deterministic output
				keys := make([]string, 0, len(values))
				for k := range values {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				var parts []string
				for _, k := range keys {
					for _, v := range values[k] {
						parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
					}
				}
				query = strings.Join(parts, "&")
			} else {
				query = values.Encode()
			}

			result := map[string]any{
				"query": query,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func encodeTool() tool.Tool {
	return tool.NewBuilder("url_encode").
		WithDescription("URL encode a string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text     string `json:"text"`
				PathMode bool   `json:"path_mode,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var encoded string
			if params.PathMode {
				encoded = url.PathEscape(params.Text)
			} else {
				encoded = url.QueryEscape(params.Text)
			}

			result := map[string]any{
				"encoded": encoded,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func decodeTool() tool.Tool {
	return tool.NewBuilder("url_decode").
		WithDescription("URL decode a string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text     string `json:"text"`
				PathMode bool   `json:"path_mode,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var decoded string
			var err error
			if params.PathMode {
				decoded, err = url.PathUnescape(params.Text)
			} else {
				decoded, err = url.QueryUnescape(params.Text)
			}

			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"decoded": decoded,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("url_validate").
		WithDescription("Validate a URL").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL            string   `json:"url"`
				RequireScheme  bool     `json:"require_scheme,omitempty"`
				RequireHost    bool     `json:"require_host,omitempty"`
				AllowedSchemes []string `json:"allowed_schemes,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parsed, err := url.Parse(params.URL)
			if err != nil {
				result := map[string]any{
					"valid":  false,
					"reason": "parse error: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			if params.RequireScheme && parsed.Scheme == "" {
				result := map[string]any{
					"valid":  false,
					"reason": "missing scheme",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			if params.RequireHost && parsed.Host == "" {
				result := map[string]any{
					"valid":  false,
					"reason": "missing host",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			if len(params.AllowedSchemes) > 0 && parsed.Scheme != "" {
				allowed := false
				for _, s := range params.AllowedSchemes {
					if strings.EqualFold(parsed.Scheme, s) {
						allowed = true
						break
					}
				}
				if !allowed {
					result := map[string]any{
						"valid":  false,
						"reason": "scheme not allowed: " + parsed.Scheme,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]any{
				"valid":  true,
				"scheme": parsed.Scheme,
				"host":   parsed.Host,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
