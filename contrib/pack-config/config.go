// Package config provides configuration management tools for agents.
package config

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// ConfigStore manages configuration values.
type ConfigStore struct {
	mu     sync.RWMutex
	values map[string]any
}

var store = &ConfigStore{
	values: make(map[string]any),
}

// Pack returns the config tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("config").
		WithDescription("Configuration management tools").
		AddTools(
			setTool(),
			getTool(),
			deleteTool(),
			listTool(),
			loadEnvTool(),
			mergeTool(),
			defaultsTool(),
			validateTool(),
			exportTool(),
			clearTool(),
			hasTool(),
			getPathTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func setTool() tool.Tool {
	return tool.NewBuilder("config_set").
		WithDescription("Set a configuration value").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key   string `json:"key"`
				Value any    `json:"value"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			store.values[params.Key] = params.Value
			store.mu.Unlock()

			result := map[string]any{
				"key": params.Key,
				"set": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getTool() tool.Tool {
	return tool.NewBuilder("config_get").
		WithDescription("Get a configuration value").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key     string `json:"key"`
				Default any    `json:"default,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			value, ok := store.values[params.Key]
			store.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"key":   params.Key,
					"found": false,
					"value": params.Default,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"key":   params.Key,
				"found": true,
				"value": value,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deleteTool() tool.Tool {
	return tool.NewBuilder("config_delete").
		WithDescription("Delete a configuration value").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			_, existed := store.values[params.Key]
			delete(store.values, params.Key)
			store.mu.Unlock()

			result := map[string]any{
				"key":     params.Key,
				"deleted": existed,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("config_list").
		WithDescription("List all configuration keys").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Prefix string `json:"prefix,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			var keys []string
			for key := range store.values {
				if params.Prefix == "" || strings.HasPrefix(key, params.Prefix) {
					keys = append(keys, key)
				}
			}
			store.mu.RUnlock()

			result := map[string]any{
				"keys":  keys,
				"count": len(keys),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func loadEnvTool() tool.Tool {
	return tool.NewBuilder("config_load_env").
		WithDescription("Load configuration from environment").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Prefix string `json:"prefix,omitempty"`
				Strip  bool   `json:"strip_prefix,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			loaded := 0
			store.mu.Lock()
			for _, env := range os.Environ() {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key, value := parts[0], parts[1]

				if params.Prefix != "" {
					if !strings.HasPrefix(key, params.Prefix) {
						continue
					}
					if params.Strip {
						key = strings.TrimPrefix(key, params.Prefix)
					}
				}

				store.values[key] = value
				loaded++
			}
			store.mu.Unlock()

			result := map[string]any{
				"loaded": loaded,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mergeTool() tool.Tool {
	return tool.NewBuilder("config_merge").
		WithDescription("Merge configuration values").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Values    map[string]any `json:"values"`
				Overwrite bool           `json:"overwrite,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			merged := 0
			skipped := 0

			store.mu.Lock()
			for key, value := range params.Values {
				if _, exists := store.values[key]; exists && !params.Overwrite {
					skipped++
					continue
				}
				store.values[key] = value
				merged++
			}
			store.mu.Unlock()

			result := map[string]any{
				"merged":  merged,
				"skipped": skipped,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func defaultsTool() tool.Tool {
	return tool.NewBuilder("config_defaults").
		WithDescription("Set default values if not present").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Defaults map[string]any `json:"defaults"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			applied := 0

			store.mu.Lock()
			for key, value := range params.Defaults {
				if _, exists := store.values[key]; !exists {
					store.values[key] = value
					applied++
				}
			}
			store.mu.Unlock()

			result := map[string]any{
				"applied": applied,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("config_validate").
		WithDescription("Validate required configuration").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Required []string `json:"required"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var missing []string

			store.mu.RLock()
			for _, key := range params.Required {
				if _, ok := store.values[key]; !ok {
					missing = append(missing, key)
				}
			}
			store.mu.RUnlock()

			valid := len(missing) == 0

			result := map[string]any{
				"valid":   valid,
				"missing": missing,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func exportTool() tool.Tool {
	return tool.NewBuilder("config_export").
		WithDescription("Export all configuration").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			store.mu.RLock()
			values := make(map[string]any)
			for k, v := range store.values {
				values[k] = v
			}
			store.mu.RUnlock()

			result := map[string]any{
				"values": values,
				"count":  len(values),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func clearTool() tool.Tool {
	return tool.NewBuilder("config_clear").
		WithDescription("Clear all configuration").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			store.mu.Lock()
			count := len(store.values)
			store.values = make(map[string]any)
			store.mu.Unlock()

			result := map[string]any{
				"cleared": count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hasTool() tool.Tool {
	return tool.NewBuilder("config_has").
		WithDescription("Check if configuration key exists").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			_, exists := store.values[params.Key]
			store.mu.RUnlock()

			result := map[string]any{
				"key":    params.Key,
				"exists": exists,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getPathTool() tool.Tool {
	return tool.NewBuilder("config_get_path").
		WithDescription("Get nested configuration value by path").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string `json:"path"`
				Separator string `json:"separator,omitempty"`
				Default   any    `json:"default,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sep := params.Separator
			if sep == "" {
				sep = "."
			}

			parts := strings.Split(params.Path, sep)

			store.mu.RLock()
			var current any = store.values
			found := true
			for _, part := range parts {
				if m, ok := current.(map[string]any); ok {
					if val, exists := m[part]; exists {
						current = val
					} else {
						found = false
						break
					}
				} else {
					found = false
					break
				}
			}
			store.mu.RUnlock()

			if !found {
				result := map[string]any{
					"path":  params.Path,
					"found": false,
					"value": params.Default,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"path":  params.Path,
				"found": true,
				"value": current,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
