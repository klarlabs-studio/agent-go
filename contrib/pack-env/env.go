// Package env provides environment variable tools for agents.
package env

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"sort"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the environment tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("env").
		WithDescription("Environment variable tools").
		AddTools(
			getTool(),
			listTool(),
			searchTool(),
			expandTool(),
			pathTool(),
			homeTool(),
			tempTool(),
			hostnameTool(),
			userTool(),
			goEnvTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func getTool() tool.Tool {
	return tool.NewBuilder("env_get").
		WithDescription("Get an environment variable value").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name    string `json:"name"`
				Default string `json:"default,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			value, exists := os.LookupEnv(params.Name)
			if !exists {
				value = params.Default
			}

			result := map[string]any{
				"name":   params.Name,
				"value":  value,
				"exists": exists,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("env_list").
		WithDescription("List all environment variables").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Prefix string `json:"prefix,omitempty"`
				Sorted bool   `json:"sorted,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			env := os.Environ()
			if params.Sorted {
				sort.Strings(env)
			}

			var vars []map[string]string
			for _, e := range env {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) != 2 {
					continue
				}
				name, value := parts[0], parts[1]

				if params.Prefix != "" && !strings.HasPrefix(name, params.Prefix) {
					continue
				}

				vars = append(vars, map[string]string{
					"name":  name,
					"value": value,
				})
			}

			result := map[string]any{
				"variables": vars,
				"count":     len(vars),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func searchTool() tool.Tool {
	return tool.NewBuilder("env_search").
		WithDescription("Search environment variables by name or value").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query      string `json:"query"`
				SearchName bool   `json:"search_name,omitempty"`
				SearchVal  bool   `json:"search_value,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Default to searching names
			if !params.SearchName && !params.SearchVal {
				params.SearchName = true
			}

			query := strings.ToLower(params.Query)
			env := os.Environ()

			var matches []map[string]string
			for _, e := range env {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) != 2 {
					continue
				}
				name, value := parts[0], parts[1]

				match := false
				if params.SearchName && strings.Contains(strings.ToLower(name), query) {
					match = true
				}
				if params.SearchVal && strings.Contains(strings.ToLower(value), query) {
					match = true
				}

				if match {
					matches = append(matches, map[string]string{
						"name":  name,
						"value": value,
					})
				}
			}

			result := map[string]any{
				"query":   params.Query,
				"matches": matches,
				"count":   len(matches),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func expandTool() tool.Tool {
	return tool.NewBuilder("env_expand").
		WithDescription("Expand environment variables in a string").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			expanded := os.ExpandEnv(params.Text)

			result := map[string]any{
				"original": params.Text,
				"expanded": expanded,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pathTool() tool.Tool {
	return tool.NewBuilder("env_path").
		WithDescription("Get and parse PATH environment variable").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			pathEnv := os.Getenv("PATH")
			separator := string(os.PathListSeparator)
			paths := strings.Split(pathEnv, separator)

			// Filter empty entries
			var cleanPaths []string
			for _, p := range paths {
				if p != "" {
					cleanPaths = append(cleanPaths, p)
				}
			}

			result := map[string]any{
				"path":      pathEnv,
				"paths":     cleanPaths,
				"separator": separator,
				"count":     len(cleanPaths),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func homeTool() tool.Tool {
	return tool.NewBuilder("env_home").
		WithDescription("Get user home directory").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			home, err := os.UserHomeDir()
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"home": home,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func tempTool() tool.Tool {
	return tool.NewBuilder("env_temp").
		WithDescription("Get temporary directory").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			temp := os.TempDir()

			result := map[string]any{
				"temp": temp,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hostnameTool() tool.Tool {
	return tool.NewBuilder("env_hostname").
		WithDescription("Get system hostname").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			hostname, err := os.Hostname()
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"hostname": hostname,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func userTool() tool.Tool {
	return tool.NewBuilder("env_user").
		WithDescription("Get current user information").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			user := os.Getenv("USER")
			if user == "" {
				user = os.Getenv("USERNAME")
			}

			home, _ := os.UserHomeDir()
			cwd, _ := os.Getwd()

			result := map[string]any{
				"user": user,
				"uid":  os.Getuid(),
				"gid":  os.Getgid(),
				"home": home,
				"cwd":  cwd,
				"pid":  os.Getpid(),
				"ppid": os.Getppid(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func goEnvTool() tool.Tool {
	return tool.NewBuilder("env_go").
		WithDescription("Get Go-related environment variables").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			goVars := []string{
				"GOROOT", "GOPATH", "GOBIN", "GOCACHE", "GOMODCACHE",
				"GOPROXY", "GONOPROXY", "GONOSUMDB", "GOPRIVATE",
				"CGO_ENABLED", "GO111MODULE", "GOFLAGS",
			}

			vars := make(map[string]string)
			for _, name := range goVars {
				if val := os.Getenv(name); val != "" {
					vars[name] = val
				}
			}

			result := map[string]any{
				"variables":  vars,
				"go_version": runtime.Version(),
				"go_os":      runtime.GOOS,
				"go_arch":    runtime.GOARCH,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
