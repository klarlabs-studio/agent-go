// Package fileops provides file operation tools for agent-go.
//
// This pack includes high-level tools for file operations:
//   - fileops_read: Read file contents with optional offset/limit
//   - fileops_write: Write content to a file with optional backup
//   - fileops_append: Append content to the end of a file
//   - fileops_search: Search for text patterns in files using regex
//   - fileops_replace: Find and replace text patterns in files
//   - fileops_diff: Compare two files and show differences
//   - fileops_archive: Create archives (zip, tar, tar.gz)
//   - fileops_extract: Extract archives
//   - fileops_checksum: Calculate file checksums (MD5, SHA256)
//
// All file paths are sandboxed to the configured base directory to prevent
// path traversal attacks. Destructive operations (write, replace, archive,
// extract) are annotated accordingly for policy enforcement.
package fileops

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the file operations tools pack.
// The baseDir parameter restricts all file operations to the given directory
// to prevent path traversal attacks. All paths provided by tool inputs are
// resolved relative to baseDir.
func Pack(baseDir string) *pack.Pack {
	return pack.NewBuilder("fileops").
		WithDescription("High-level file operation tools").
		WithVersion("0.1.0").
		AddTools(
			fileopsRead(baseDir),
			fileopsWrite(baseDir),
			fileopsAppend(baseDir),
			fileopsSearch(baseDir),
			fileopsReplace(baseDir),
			fileopsDiff(baseDir),
			fileopsArchive(baseDir),
			fileopsExtract(baseDir),
			fileopsChecksum(baseDir),
		).
		AllowInState(agent.StateExplore, "fileops_read", "fileops_search", "fileops_diff", "fileops_checksum").
		AllowInState(agent.StateAct, "fileops_read", "fileops_write", "fileops_append", "fileops_search", "fileops_replace", "fileops_diff", "fileops_archive", "fileops_extract", "fileops_checksum").
		AllowInState(agent.StateValidate, "fileops_read", "fileops_diff", "fileops_checksum").
		Build()
}

// --- Path security ---

// safePath resolves a user-provided path relative to baseDir and ensures
// it does not escape the sandbox. Returns the absolute path or an error.
func safePath(baseDir, userPath string) (string, error) {
	fullPath := filepath.Join(baseDir, filepath.Clean(userPath))
	if !isSubPath(baseDir, fullPath) {
		return "", fmt.Errorf("path traversal attempt: %s", userPath)
	}
	return fullPath, nil
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
	if rel == "." {
		return true
	}
	return !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..")
}

// --- Input/Output types ---

type readInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type readOutput struct {
	Content    string `json:"content"`
	Size       int    `json:"size"`
	Truncated  bool   `json:"truncated,omitempty"`
	TotalBytes int    `json:"total_bytes"`
}

type writeInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Backup  bool   `json:"backup,omitempty"`
}

type writeOutput struct {
	BytesWritten int    `json:"bytes_written"`
	Created      bool   `json:"created"`
	BackupPath   string `json:"backup_path,omitempty"`
}

type appendInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type appendOutput struct {
	BytesWritten int `json:"bytes_written"`
	TotalSize    int `json:"total_size"`
}

type searchInput struct {
	Path       string `json:"path"`
	Pattern    string `json:"pattern"`
	Recursive  bool   `json:"recursive,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type searchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type searchOutput struct {
	Matches    []searchMatch `json:"matches"`
	MatchCount int           `json:"match_count"`
	Truncated  bool          `json:"truncated,omitempty"`
}

type replaceInput struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
	All     bool   `json:"all,omitempty"`
}

type replaceOutput struct {
	Replacements int  `json:"replacements"`
	Modified     bool `json:"modified"`
}

type diffInput struct {
	PathA string `json:"path_a"`
	PathB string `json:"path_b"`
}

type diffLine struct {
	Number int    `json:"number"`
	Type   string `json:"type"`
	Text   string `json:"text"`
}

type diffOutput struct {
	Identical bool       `json:"identical"`
	Changes   []diffLine `json:"changes,omitempty"`
}

type archiveInput struct {
	Paths  []string `json:"paths"`
	Output string   `json:"output"`
	Format string   `json:"format"`
}

type archiveOutput struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	FileCount int    `json:"file_count"`
}

type extractInput struct {
	Path   string `json:"path"`
	Output string `json:"output"`
}

type extractOutput struct {
	Path      string `json:"path"`
	FileCount int    `json:"file_count"`
}

type checksumInput struct {
	Path      string `json:"path"`
	Algorithm string `json:"algorithm,omitempty"`
}

type checksumOutput struct {
	Checksum  string `json:"checksum"`
	Algorithm string `json:"algorithm"`
	Size      int64  `json:"size"`
}

// --- Tool constructors ---

func fileopsRead(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_read").
		WithDescription("Read file contents with optional offset and limit").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file to read (relative to base directory)"},
				"offset": {"type": "integer", "description": "Byte offset to start reading from (default: 0)"},
				"limit": {"type": "integer", "description": "Maximum bytes to read (default: entire file)"}
			},
			"required": ["path"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in readInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
			}

			totalBytes := len(data)
			truncated := false

			if in.Offset > 0 {
				if in.Offset >= len(data) {
					data = nil
				} else {
					data = data[in.Offset:]
				}
			}

			if in.Limit > 0 && in.Limit < len(data) {
				data = data[:in.Limit]
				truncated = true
			}

			out := readOutput{
				Content:    string(data),
				Size:       len(data),
				Truncated:  truncated,
				TotalBytes: totalBytes,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func fileopsWrite(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_write").
		WithDescription("Write content to a file with optional backup").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file to write (relative to base directory)"},
				"content": {"type": "string", "description": "Content to write"},
				"backup": {"type": "boolean", "description": "Create a backup of existing file before overwriting (default: false)"}
			},
			"required": ["path", "content"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in writeInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			_, statErr := os.Stat(fullPath)
			created := os.IsNotExist(statErr)
			var backupPath string

			// Create backup if requested and file exists
			if in.Backup && !created {
				backupPath = fullPath + ".bak"
				existing, readErr := os.ReadFile(fullPath)
				if readErr != nil {
					return tool.Result{}, fmt.Errorf("failed to read file for backup: %w", readErr)
				}
				if writeErr := os.WriteFile(backupPath, existing, 0600); writeErr != nil {
					return tool.Result{}, fmt.Errorf("failed to create backup: %w", writeErr)
				}
			}

			// Ensure parent directory exists
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create directory: %w", err)
			}

			if err := os.WriteFile(fullPath, []byte(in.Content), 0600); err != nil {
				return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
			}

			out := writeOutput{
				BytesWritten: len(in.Content),
				Created:      created,
				BackupPath:   backupPath,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func fileopsAppend(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_append").
		WithDescription("Append content to the end of a file").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file to append to (relative to base directory)"},
				"content": {"type": "string", "description": "Content to append"}
			},
			"required": ["path", "content"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in appendInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			// Ensure parent directory exists
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create directory: %w", err)
			}

			f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			n, err := f.WriteString(in.Content)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to append to file: %w", err)
			}

			info, err := f.Stat()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat file: %w", err)
			}

			out := appendOutput{
				BytesWritten: n,
				TotalSize:    int(info.Size()),
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func fileopsSearch(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_search").
		WithDescription("Search for text patterns in files using regex").
		ReadOnly().
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "File or directory path to search in (relative to base directory)"},
				"pattern": {"type": "string", "description": "Regular expression pattern to search for"},
				"recursive": {"type": "boolean", "description": "Search recursively in subdirectories (default: false)"},
				"max_results": {"type": "integer", "description": "Maximum number of matches to return (default: 100)"}
			},
			"required": ["path", "pattern"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in searchInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			re, err := regexp.Compile(in.Pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex pattern: %w", err)
			}

			maxResults := in.MaxResults
			if maxResults <= 0 {
				maxResults = 100
			}

			var matches []searchMatch
			truncated := false

			info, err := os.Stat(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat path: %w", err)
			}

			var files []string
			if info.IsDir() {
				if in.Recursive {
					err = filepath.Walk(fullPath, func(p string, fi os.FileInfo, walkErr error) error {
						if walkErr != nil {
							return nil // skip errors
						}
						if !fi.IsDir() {
							files = append(files, p)
						}
						return nil
					})
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to walk directory: %w", err)
					}
				} else {
					entries, readErr := os.ReadDir(fullPath)
					if readErr != nil {
						return tool.Result{}, fmt.Errorf("failed to read directory: %w", readErr)
					}
					for _, e := range entries {
						if !e.IsDir() {
							files = append(files, filepath.Join(fullPath, e.Name()))
						}
					}
				}
			} else {
				files = []string{fullPath}
			}

			for _, filePath := range files {
				if len(matches) >= maxResults {
					truncated = true
					break
				}
				fileMatches, searchErr := searchFile(filePath, re, maxResults-len(matches), baseDir)
				if searchErr != nil {
					continue // skip files that cannot be read
				}
				matches = append(matches, fileMatches...)
			}

			if len(matches) >= maxResults {
				truncated = true
			}

			out := searchOutput{
				Matches:    matches,
				MatchCount: len(matches),
				Truncated:  truncated,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// searchFile searches a single file for regex matches.
func searchFile(filePath string, re *regexp.Regexp, maxMatches int, baseDir string) ([]searchMatch, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	relPath, _ := filepath.Rel(baseDir, filePath)
	var matches []searchMatch
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, searchMatch{
				File:    relPath,
				Line:    lineNum,
				Content: line,
			})
			if len(matches) >= maxMatches {
				break
			}
		}
	}
	return matches, scanner.Err()
}

func fileopsReplace(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_replace").
		WithDescription("Find and replace text patterns in files").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file (relative to base directory)"},
				"pattern": {"type": "string", "description": "Regular expression pattern to find"},
				"replace": {"type": "string", "description": "Replacement string (supports regex group references like $1)"},
				"all": {"type": "boolean", "description": "Replace all occurrences (default: false, replaces first only)"}
			},
			"required": ["path", "pattern", "replace"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in replaceInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			re, err := regexp.Compile(in.Pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex pattern: %w", err)
			}

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
			}

			original := string(data)
			var replaced string
			var count int

			if in.All {
				replaced = re.ReplaceAllString(original, in.Replace)
				// Count matches
				count = len(re.FindAllStringIndex(original, -1))
			} else {
				// Replace only first occurrence
				loc := re.FindStringIndex(original)
				if loc != nil {
					match := original[loc[0]:loc[1]]
					replacement := re.ReplaceAllString(match, in.Replace)
					replaced = original[:loc[0]] + replacement + original[loc[1]:]
					count = 1
				} else {
					replaced = original
					count = 0
				}
			}

			modified := replaced != original
			if modified {
				if err := os.WriteFile(fullPath, []byte(replaced), 0600); err != nil {
					return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
				}
			}

			out := replaceOutput{
				Replacements: count,
				Modified:     modified,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func fileopsDiff(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_diff").
		WithDescription("Compare two files and show differences").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path_a": {"type": "string", "description": "Path to the first file (relative to base directory)"},
				"path_b": {"type": "string", "description": "Path to the second file (relative to base directory)"}
			},
			"required": ["path_a", "path_b"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in diffInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			pathA, err := safePath(baseDir, in.PathA)
			if err != nil {
				return tool.Result{}, err
			}
			pathB, err := safePath(baseDir, in.PathB)
			if err != nil {
				return tool.Result{}, err
			}

			dataA, err := os.ReadFile(pathA)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file A: %w", err)
			}
			dataB, err := os.ReadFile(pathB)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read file B: %w", err)
			}

			linesA := strings.Split(string(dataA), "\n")
			linesB := strings.Split(string(dataB), "\n")

			identical := string(dataA) == string(dataB)
			var changes []diffLine

			if !identical {
				changes = computeSimpleDiff(linesA, linesB)
			}

			out := diffOutput{
				Identical: identical,
				Changes:   changes,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// computeSimpleDiff produces a line-by-line diff showing removed and added lines.
// This is a simple O(n+m) approach that marks lines unique to A as removed and
// lines unique to B as added, with shared lines marked as context.
func computeSimpleDiff(linesA, linesB []string) []diffLine {
	var result []diffLine
	i, j := 0, 0
	for i < len(linesA) || j < len(linesB) {
		switch {
		case i < len(linesA) && j < len(linesB) && linesA[i] == linesB[j]:
			result = append(result, diffLine{
				Number: i + 1,
				Type:   "context",
				Text:   linesA[i],
			})
			i++
			j++
		case i < len(linesA) && (j >= len(linesB) || !containsFrom(linesB, j, linesA[i])):
			result = append(result, diffLine{
				Number: i + 1,
				Type:   "removed",
				Text:   linesA[i],
			})
			i++
		case j < len(linesB):
			result = append(result, diffLine{
				Number: j + 1,
				Type:   "added",
				Text:   linesB[j],
			})
			j++
		}
	}
	return result
}

// containsFrom checks if target appears anywhere in lines starting from index start.
func containsFrom(lines []string, start int, target string) bool {
	for k := start; k < len(lines); k++ {
		if lines[k] == target {
			return true
		}
	}
	return false
}

func fileopsArchive(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_archive").
		WithDescription("Create archives (zip, tar, tar.gz) from files").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"paths": {"type": "array", "items": {"type": "string"}, "description": "Paths to include in archive (relative to base directory)"},
				"output": {"type": "string", "description": "Output archive path (relative to base directory)"},
				"format": {"type": "string", "enum": ["zip", "tar", "tar.gz"], "description": "Archive format (default: zip)"}
			},
			"required": ["paths", "output"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in archiveInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			outputPath, err := safePath(baseDir, in.Output)
			if err != nil {
				return tool.Result{}, err
			}

			format := in.Format
			if format == "" {
				format = "zip"
			}

			// Resolve and validate all input paths
			var resolvedPaths []string
			for _, p := range in.Paths {
				fp, pathErr := safePath(baseDir, p)
				if pathErr != nil {
					return tool.Result{}, pathErr
				}
				resolvedPaths = append(resolvedPaths, fp)
			}

			// Ensure output directory exists
			if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output directory: %w", err)
			}

			var fileCount int
			switch format {
			case "zip":
				fileCount, err = createZipArchive(outputPath, resolvedPaths, baseDir)
			case "tar":
				fileCount, err = createTarArchive(outputPath, resolvedPaths, baseDir, false)
			case "tar.gz":
				fileCount, err = createTarArchive(outputPath, resolvedPaths, baseDir, true)
			default:
				return tool.Result{}, fmt.Errorf("unsupported format: %s", format)
			}
			if err != nil {
				return tool.Result{}, err
			}

			info, err := os.Stat(outputPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat archive: %w", err)
			}

			out := archiveOutput{
				Path:      in.Output,
				Size:      info.Size(),
				FileCount: fileCount,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func createZipArchive(outputPath string, paths []string, baseDir string) (int, error) {
	f, err := os.Create(outputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create archive: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer func() { _ = w.Close() }()

	count := 0
	for _, p := range paths {
		err := filepath.Walk(p, func(filePath string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}

			relPath, _ := filepath.Rel(baseDir, filePath)
			zf, createErr := w.Create(relPath)
			if createErr != nil {
				return createErr
			}

			data, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return readErr
			}
			if _, writeErr := zf.Write(data); writeErr != nil {
				return writeErr
			}
			count++
			return nil
		})
		if err != nil {
			return count, fmt.Errorf("failed to add to archive: %w", err)
		}
	}
	return count, nil
}

func createTarArchive(outputPath string, paths []string, baseDir string, compressed bool) (int, error) {
	f, err := os.Create(outputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create archive: %w", err)
	}
	defer f.Close()

	var w io.Writer = f
	var gzWriter *gzip.Writer
	if compressed {
		gzWriter = gzip.NewWriter(f)
		defer func() { _ = gzWriter.Close() }()
		w = gzWriter
	}

	tw := tar.NewWriter(w)
	defer func() { _ = tw.Close() }()

	count := 0
	for _, p := range paths {
		err := filepath.Walk(p, func(filePath string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}

			relPath, _ := filepath.Rel(baseDir, filePath)
			header := &tar.Header{
				Name: relPath,
				Size: info.Size(),
				Mode: int64(info.Mode()),
			}
			if writeErr := tw.WriteHeader(header); writeErr != nil {
				return writeErr
			}

			data, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return readErr
			}
			if _, writeErr := tw.Write(data); writeErr != nil {
				return writeErr
			}
			count++
			return nil
		})
		if err != nil {
			return count, fmt.Errorf("failed to add to archive: %w", err)
		}
	}
	return count, nil
}

func fileopsExtract(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_extract").
		WithDescription("Extract archives to a directory").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the archive file (relative to base directory)"},
				"output": {"type": "string", "description": "Output directory (relative to base directory)"}
			},
			"required": ["path", "output"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in extractInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			archivePath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}
			outputDir, err := safePath(baseDir, in.Output)
			if err != nil {
				return tool.Result{}, err
			}

			if err := os.MkdirAll(outputDir, 0750); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output directory: %w", err)
			}

			var fileCount int
			lower := strings.ToLower(archivePath)
			switch {
			case strings.HasSuffix(lower, ".zip"):
				fileCount, err = extractZip(archivePath, outputDir, baseDir)
			case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
				fileCount, err = extractTar(archivePath, outputDir, baseDir, true)
			case strings.HasSuffix(lower, ".tar"):
				fileCount, err = extractTar(archivePath, outputDir, baseDir, false)
			default:
				return tool.Result{}, fmt.Errorf("unsupported archive format: %s", filepath.Ext(archivePath))
			}
			if err != nil {
				return tool.Result{}, err
			}

			out := extractOutput{
				Path:      in.Output,
				FileCount: fileCount,
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

func extractZip(archivePath, outputDir, baseDir string) (int, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	count := 0
	for _, zf := range r.File {
		// Validate extracted path stays within sandbox
		destPath := filepath.Join(outputDir, filepath.Clean(zf.Name))
		if !isSubPath(baseDir, destPath) {
			return count, fmt.Errorf("zip contains path traversal entry: %s", zf.Name)
		}

		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0750); err != nil {
				return count, err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
			return count, err
		}

		rc, err := zf.Open()
		if err != nil {
			return count, err
		}

		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			rc.Close()
			return count, err
		}

		// Limit extraction size to prevent zip bombs (100MB per file)
		_, err = io.Copy(outFile, io.LimitReader(rc, 100*1024*1024))
		outFile.Close()
		rc.Close()
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func extractTar(archivePath, outputDir, baseDir string, compressed bool) (int, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open archive: %w", err)
	}
	defer f.Close()

	var r io.Reader = f
	if compressed {
		gzReader, gzErr := gzip.NewReader(f)
		if gzErr != nil {
			return 0, fmt.Errorf("failed to create gzip reader: %w", gzErr)
		}
		defer func() { _ = gzReader.Close() }()
		r = gzReader
	}

	tr := tar.NewReader(r)
	count := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("failed to read tar entry: %w", err)
		}

		destPath := filepath.Join(outputDir, filepath.Clean(header.Name))
		if !isSubPath(baseDir, destPath) {
			return count, fmt.Errorf("tar contains path traversal entry: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0750); err != nil {
				return count, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
				return count, err
			}
			outFile, createErr := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if createErr != nil {
				return count, createErr
			}
			// Limit extraction size to prevent tar bombs (100MB per file)
			_, copyErr := io.Copy(outFile, io.LimitReader(tr, 100*1024*1024))
			outFile.Close()
			if copyErr != nil {
				return count, copyErr
			}
			count++
		}
	}
	return count, nil
}

func fileopsChecksum(baseDir string) tool.Tool {
	return tool.NewBuilder("fileops_checksum").
		WithDescription("Calculate file checksums (MD5, SHA256, SHA512)").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path to the file (relative to base directory)"},
				"algorithm": {"type": "string", "enum": ["md5", "sha256", "sha512"], "description": "Hash algorithm (default: sha256)"}
			},
			"required": ["path"]
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in checksumInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			fullPath, err := safePath(baseDir, in.Path)
			if err != nil {
				return tool.Result{}, err
			}

			algorithm := in.Algorithm
			if algorithm == "" {
				algorithm = "sha256"
			}

			var h hash.Hash
			switch algorithm {
			case "md5":
				h = md5.New()
			case "sha256":
				h = sha256.New()
			case "sha512":
				h = sha512.New()
			default:
				return tool.Result{}, fmt.Errorf("unsupported algorithm: %s", algorithm)
			}

			f, err := os.Open(fullPath)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			if _, err := io.Copy(h, f); err != nil {
				return tool.Result{}, fmt.Errorf("failed to compute checksum: %w", err)
			}

			info, err := f.Stat()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat file: %w", err)
			}

			out := checksumOutput{
				Checksum:  hex.EncodeToString(h.Sum(nil)),
				Algorithm: algorithm,
				Size:      info.Size(),
			}
			outputBytes, _ := json.Marshal(out)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}
