package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/agent/domain/tool"
	api "go.klarlabs.de/agent/interfaces/api"
)

// ReadFileInput is the input schema for the read_file tool.
type ReadFileInput struct {
	Path string `json:"path"`
}

// ReadFileOutput is the output schema for the read_file tool.
type ReadFileOutput struct {
	Content string `json:"content"`
	Size    int    `json:"size"`
}

// WriteFileInput is the input schema for the write_file tool.
type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteFileOutput is the output schema for the write_file tool.
type WriteFileOutput struct {
	BytesWritten int  `json:"bytes_written"`
	Created      bool `json:"created"`
}

// DeleteFileInput is the input schema for the delete_file tool.
type DeleteFileInput struct {
	Path string `json:"path"`
}

// DeleteFileOutput is the output schema for the delete_file tool.
type DeleteFileOutput struct {
	Deleted bool `json:"deleted"`
}

// ListDirInput is the input schema for the list_dir tool.
type ListDirInput struct {
	Path string `json:"path"`
}

// ListDirOutput is the output schema for the list_dir tool.
type ListDirOutput struct {
	Files []FileInfo `json:"files"`
	Count int        `json:"count"`
}

// FileInfo represents file metadata.
type FileInfo struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

// NewReadFileTool creates a tool for reading files.
func NewReadFileTool(baseDir string) tool.Tool {
	return api.NewToolBuilder("read_file").
		WithDescription("Reads the content of a file").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file to read"}
			},
			"required": ["path"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"content": {"type": "string", "description": "File content"},
				"size": {"type": "integer", "description": "File size in bytes"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in ReadFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			// Sanitize path
			fullPath := filepath.Join(baseDir, filepath.Clean(in.Path))
			if !isSubPath(baseDir, fullPath) {
				return tool.Result{}, fmt.Errorf("path traversal attempt: %s", in.Path)
			}

			// #nosec G304 -- path is sanitized above with filepath.Clean and isSubPath check
			content, err := os.ReadFile(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
			}

			output := ReadFileOutput{
				Content: string(content),
				Size:    len(content),
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewWriteFileTool creates a tool for writing files.
func NewWriteFileTool(baseDir string) tool.Tool {
	return api.NewToolBuilder("write_file").
		WithDescription("Writes content to a file").
		WithAnnotations(api.Annotations{
			ReadOnly:    false,
			Destructive: false,
			Idempotent:  true,
			RiskLevel:   api.RiskMedium,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file to write"},
				"content": {"type": "string", "description": "Content to write"}
			},
			"required": ["path", "content"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"bytes_written": {"type": "integer", "description": "Number of bytes written"},
				"created": {"type": "boolean", "description": "Whether file was created"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in WriteFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath := filepath.Join(baseDir, filepath.Clean(in.Path))
			if !isSubPath(baseDir, fullPath) {
				return tool.Result{}, fmt.Errorf("path traversal attempt: %s", in.Path)
			}

			// Check if file exists
			_, err := os.Stat(fullPath)
			created := os.IsNotExist(err)

			// Ensure directory exists
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create directory: %w", err)
			}

			if err := os.WriteFile(fullPath, []byte(in.Content), 0600); err != nil {
				return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
			}

			output := WriteFileOutput{
				BytesWritten: len(in.Content),
				Created:      created,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewDeleteFileTool creates a tool for deleting files.
func NewDeleteFileTool(baseDir string) tool.Tool {
	return api.NewToolBuilder("delete_file").
		WithDescription("Deletes a file").
		WithAnnotations(api.Annotations{
			ReadOnly:    false,
			Destructive: true,
			Idempotent:  false,
			RiskLevel:   api.RiskHigh,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file to delete"}
			},
			"required": ["path"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"deleted": {"type": "boolean", "description": "Whether file was deleted"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in DeleteFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath := filepath.Join(baseDir, filepath.Clean(in.Path))
			if !isSubPath(baseDir, fullPath) {
				return tool.Result{}, fmt.Errorf("path traversal attempt: %s", in.Path)
			}

			if err := os.Remove(fullPath); err != nil {
				if os.IsNotExist(err) {
					output := DeleteFileOutput{Deleted: false}
					outputBytes, _ := json.Marshal(output)
					return tool.Result{Output: outputBytes}, nil
				}
				return tool.Result{}, fmt.Errorf("failed to delete file: %w", err)
			}

			output := DeleteFileOutput{Deleted: true}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewListDirTool creates a tool for listing directory contents.
func NewListDirTool(baseDir string) tool.Tool {
	return api.NewToolBuilder("list_dir").
		WithDescription("Lists contents of a directory").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the directory to list"}
			},
			"required": ["path"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"files": {"type": "array", "description": "List of files"},
				"count": {"type": "integer", "description": "Number of files"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in ListDirInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath := filepath.Join(baseDir, filepath.Clean(in.Path))
			if !isSubPath(baseDir, fullPath) {
				return tool.Result{}, fmt.Errorf("path traversal attempt: %s", in.Path)
			}

			entries, err := os.ReadDir(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read directory: %w", err)
			}

			files := make([]FileInfo, 0, len(entries))
			for _, entry := range entries {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				files = append(files, FileInfo{
					Name:  entry.Name(),
					Size:  info.Size(),
					IsDir: entry.IsDir(),
				})
			}

			output := ListDirOutput{
				Files: files,
				Count: len(files),
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// isSubPath checks if the given path is under the base directory.
func isSubPath(base, path string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	// "." means same directory, which is allowed
	// ".." or paths starting with ".." are not allowed
	if rel == "." {
		return true
	}
	return !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..")
}
