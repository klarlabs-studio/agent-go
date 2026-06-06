// Package pathutil provides file path utilities for agents.
package pathutil

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the path utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("path").
		WithDescription("File path utilities").
		AddTools(
			joinTool(),
			baseTool(),
			dirTool(),
			extTool(),
			absTool(),
			relTool(),
			cleanTool(),
			matchTool(),
			splitTool(),
			existsTool(),
			expandTool(),
			normalizeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func joinTool() tool.Tool {
	return tool.NewBuilder("path_join").
		WithDescription("Join path elements").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Parts []string `json:"parts"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			joined := filepath.Join(params.Parts...)

			result := map[string]any{
				"path": joined,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func baseTool() tool.Tool {
	return tool.NewBuilder("path_base").
		WithDescription("Get base name of path").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			base := filepath.Base(params.Path)
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)

			result := map[string]any{
				"base":      base,
				"name":      name,
				"extension": ext,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func dirTool() tool.Tool {
	return tool.NewBuilder("path_dir").
		WithDescription("Get directory of path").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			dir := filepath.Dir(params.Path)

			result := map[string]any{
				"dir": dir,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extTool() tool.Tool {
	return tool.NewBuilder("path_ext").
		WithDescription("Get file extension").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ext := filepath.Ext(params.Path)

			result := map[string]any{
				"extension": ext,
				"has_ext":   ext != "",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func absTool() tool.Tool {
	return tool.NewBuilder("path_abs").
		WithDescription("Get absolute path").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			abs, err := filepath.Abs(params.Path)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"absolute": abs,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func relTool() tool.Tool {
	return tool.NewBuilder("path_rel").
		WithDescription("Get relative path").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Base   string `json:"base"`
				Target string `json:"target"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			rel, err := filepath.Rel(params.Base, params.Target)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"relative": rel,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cleanTool() tool.Tool {
	return tool.NewBuilder("path_clean").
		WithDescription("Clean path (remove . and ..)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			cleaned := filepath.Clean(params.Path)

			result := map[string]any{
				"cleaned": cleaned,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func matchTool() tool.Tool {
	return tool.NewBuilder("path_match").
		WithDescription("Check if path matches pattern").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern string `json:"pattern"`
				Path    string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			matched, err := filepath.Match(params.Pattern, params.Path)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"pattern": params.Pattern,
				"path":    params.Path,
				"matches": matched,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func splitTool() tool.Tool {
	return tool.NewBuilder("path_split").
		WithDescription("Split path into components").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			dir, file := filepath.Split(params.Path)

			// Also split into all parts
			cleaned := filepath.Clean(params.Path)
			var parts []string
			for cleaned != "" && cleaned != "." && cleaned != "/" {
				base := filepath.Base(cleaned)
				parts = append([]string{base}, parts...)
				parent := filepath.Dir(cleaned)
				if parent == cleaned {
					break
				}
				cleaned = parent
			}
			if strings.HasPrefix(params.Path, "/") {
				parts = append([]string{"/"}, parts...)
			}

			result := map[string]any{
				"dir":   dir,
				"file":  file,
				"parts": parts,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func existsTool() tool.Tool {
	return tool.NewBuilder("path_exists").
		WithDescription("Check if path exists").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			info, err := os.Stat(params.Path)
			exists := err == nil

			result := map[string]any{
				"path":   params.Path,
				"exists": exists,
			}
			if exists {
				result["is_dir"] = info.IsDir()
				result["is_file"] = !info.IsDir()
				result["size"] = info.Size()
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func expandTool() tool.Tool {
	return tool.NewBuilder("path_expand").
		WithDescription("Expand ~ to home directory").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			path := params.Path
			if strings.HasPrefix(path, "~") {
				home, err := os.UserHomeDir()
				if err == nil {
					path = filepath.Join(home, path[1:])
				}
			}

			result := map[string]any{
				"expanded": path,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func normalizeTool() tool.Tool {
	return tool.NewBuilder("path_normalize").
		WithDescription("Normalize path separators").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Path      string `json:"path"`
				Separator string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// First clean the path
			normalized := filepath.Clean(params.Path)

			// Convert to specified separator if requested
			if params.Separator == "/" {
				normalized = filepath.ToSlash(normalized)
			} else if params.Separator == "\\" {
				normalized = strings.ReplaceAll(normalized, "/", "\\")
			}

			result := map[string]any{
				"normalized": normalized,
				"separator":  string(filepath.Separator),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
