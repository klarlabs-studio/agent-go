package fileops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestPack_RegistersTools(t *testing.T) {
	p := Pack(t.TempDir())

	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if len(p.Tools) == 0 {
		t.Fatal("Pack() registered no tools")
	}
	if p.Name != "fileops" {
		t.Errorf("expected pack name %q, got %q", "fileops", p.Name)
	}
}

func TestPack_ToolsImplementInterface(t *testing.T) {
	p := Pack(t.TempDir())

	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
		if tt.Description() == "" {
			t.Errorf("tool %q has empty description", tt.Name())
		}
	}
}

func TestPack_ExpectedToolCount(t *testing.T) {
	p := Pack(t.TempDir())
	expected := 9
	if got := len(p.Tools); got != expected {
		t.Errorf("expected %d tools, got %d", expected, got)
	}
}

// --- Handler tests ---

func getTool(t *testing.T, baseDir, name string) tool.Tool {
	t.Helper()
	p := Pack(baseDir)
	tt, ok := p.GetTool(name)
	if !ok {
		t.Fatalf("tool %q not found in pack", name)
	}
	return tt
}

func execTool(t *testing.T, tt tool.Tool, input any) json.RawMessage {
	t.Helper()
	inputBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	result, err := tt.Execute(context.Background(), inputBytes)
	if err != nil {
		t.Fatalf("tool %q execution failed: %v", tt.Name(), err)
	}
	return result.Output
}

func TestFileopsRead(t *testing.T) {
	baseDir := t.TempDir()
	content := "hello world\nsecond line"
	os.WriteFile(filepath.Join(baseDir, "test.txt"), []byte(content), 0600)

	tt := getTool(t, baseDir, "fileops_read")

	t.Run("read full file", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "test.txt"})
		var result readOutput
		json.Unmarshal(out, &result)
		if result.Content != content {
			t.Errorf("expected content %q, got %q", content, result.Content)
		}
		if result.TotalBytes != len(content) {
			t.Errorf("expected total_bytes %d, got %d", len(content), result.TotalBytes)
		}
	})

	t.Run("read with offset and limit", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "test.txt", "offset": 6, "limit": 5})
		var result readOutput
		json.Unmarshal(out, &result)
		if result.Content != "world" {
			t.Errorf("expected content %q, got %q", "world", result.Content)
		}
		if !result.Truncated {
			t.Error("expected truncated to be true")
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{"path": "../../etc/passwd"})
		_, err := tt.Execute(context.Background(), input)
		if err == nil {
			t.Error("expected error for path traversal")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{"path": "nonexistent.txt"})
		_, err := tt.Execute(context.Background(), input)
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestFileopsWrite(t *testing.T) {
	baseDir := t.TempDir()
	tt := getTool(t, baseDir, "fileops_write")

	t.Run("create new file", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "new.txt", "content": "hello"})
		var result writeOutput
		json.Unmarshal(out, &result)
		if !result.Created {
			t.Error("expected created to be true")
		}
		if result.BytesWritten != 5 {
			t.Errorf("expected 5 bytes written, got %d", result.BytesWritten)
		}
		data, _ := os.ReadFile(filepath.Join(baseDir, "new.txt"))
		if string(data) != "hello" {
			t.Errorf("file content mismatch: %q", string(data))
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "new.txt", "content": "updated"})
		var result writeOutput
		json.Unmarshal(out, &result)
		if result.Created {
			t.Error("expected created to be false for overwrite")
		}
	})

	t.Run("create with backup", func(t *testing.T) {
		os.WriteFile(filepath.Join(baseDir, "backup.txt"), []byte("original"), 0600)
		out := execTool(t, tt, map[string]any{"path": "backup.txt", "content": "new content", "backup": true})
		var result writeOutput
		json.Unmarshal(out, &result)
		if result.BackupPath == "" {
			t.Error("expected backup_path to be set")
		}
		backup, _ := os.ReadFile(filepath.Join(baseDir, "backup.txt.bak"))
		if string(backup) != "original" {
			t.Errorf("backup content mismatch: %q", string(backup))
		}
	})

	t.Run("create nested directories", func(t *testing.T) {
		execTool(t, tt, map[string]any{"path": "sub/dir/file.txt", "content": "nested"})
		data, err := os.ReadFile(filepath.Join(baseDir, "sub", "dir", "file.txt"))
		if err != nil {
			t.Fatalf("failed to read nested file: %v", err)
		}
		if string(data) != "nested" {
			t.Errorf("nested file content mismatch: %q", string(data))
		}
	})
}

func TestFileopsAppend(t *testing.T) {
	baseDir := t.TempDir()
	tt := getTool(t, baseDir, "fileops_append")

	t.Run("append to new file", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "log.txt", "content": "line1\n"})
		var result appendOutput
		json.Unmarshal(out, &result)
		if result.BytesWritten != 6 {
			t.Errorf("expected 6 bytes written, got %d", result.BytesWritten)
		}
	})

	t.Run("append to existing file", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "log.txt", "content": "line2\n"})
		var result appendOutput
		json.Unmarshal(out, &result)
		if result.TotalSize != 12 {
			t.Errorf("expected total size 12, got %d", result.TotalSize)
		}
		data, _ := os.ReadFile(filepath.Join(baseDir, "log.txt"))
		if string(data) != "line1\nline2\n" {
			t.Errorf("appended content mismatch: %q", string(data))
		}
	})
}

func TestFileopsSearch(t *testing.T) {
	baseDir := t.TempDir()
	os.WriteFile(filepath.Join(baseDir, "a.txt"), []byte("hello world\nfoo bar\nhello again"), 0600)
	os.MkdirAll(filepath.Join(baseDir, "sub"), 0750)
	os.WriteFile(filepath.Join(baseDir, "sub", "b.txt"), []byte("hello sub\nno match"), 0600)

	tt := getTool(t, baseDir, "fileops_search")

	t.Run("search single file", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "a.txt", "pattern": "hello"})
		var result searchOutput
		json.Unmarshal(out, &result)
		if result.MatchCount != 2 {
			t.Errorf("expected 2 matches, got %d", result.MatchCount)
		}
	})

	t.Run("search directory non-recursive", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": ".", "pattern": "hello"})
		var result searchOutput
		json.Unmarshal(out, &result)
		// Only matches in a.txt (root), not sub/b.txt
		if result.MatchCount != 2 {
			t.Errorf("expected 2 matches in root dir, got %d", result.MatchCount)
		}
	})

	t.Run("search directory recursive", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": ".", "pattern": "hello", "recursive": true})
		var result searchOutput
		json.Unmarshal(out, &result)
		if result.MatchCount != 3 {
			t.Errorf("expected 3 matches recursively, got %d", result.MatchCount)
		}
	})

	t.Run("max results", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "a.txt", "pattern": "hello", "max_results": 1})
		var result searchOutput
		json.Unmarshal(out, &result)
		if result.MatchCount != 1 {
			t.Errorf("expected 1 match with max_results=1, got %d", result.MatchCount)
		}
	})

	t.Run("invalid regex", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{"path": "a.txt", "pattern": "["})
		_, err := tt.Execute(context.Background(), input)
		if err == nil {
			t.Error("expected error for invalid regex")
		}
	})
}

func TestFileopsReplace(t *testing.T) {
	baseDir := t.TempDir()
	tt := getTool(t, baseDir, "fileops_replace")

	t.Run("replace first occurrence", func(t *testing.T) {
		os.WriteFile(filepath.Join(baseDir, "r.txt"), []byte("foo bar foo baz"), 0600)
		out := execTool(t, tt, map[string]any{"path": "r.txt", "pattern": "foo", "replace": "qux"})
		var result replaceOutput
		json.Unmarshal(out, &result)
		if result.Replacements != 1 {
			t.Errorf("expected 1 replacement, got %d", result.Replacements)
		}
		data, _ := os.ReadFile(filepath.Join(baseDir, "r.txt"))
		if string(data) != "qux bar foo baz" {
			t.Errorf("unexpected content: %q", string(data))
		}
	})

	t.Run("replace all occurrences", func(t *testing.T) {
		os.WriteFile(filepath.Join(baseDir, "ra.txt"), []byte("foo bar foo baz"), 0600)
		out := execTool(t, tt, map[string]any{"path": "ra.txt", "pattern": "foo", "replace": "qux", "all": true})
		var result replaceOutput
		json.Unmarshal(out, &result)
		if result.Replacements != 2 {
			t.Errorf("expected 2 replacements, got %d", result.Replacements)
		}
		data, _ := os.ReadFile(filepath.Join(baseDir, "ra.txt"))
		if string(data) != "qux bar qux baz" {
			t.Errorf("unexpected content: %q", string(data))
		}
	})

	t.Run("no match", func(t *testing.T) {
		os.WriteFile(filepath.Join(baseDir, "nm.txt"), []byte("hello"), 0600)
		out := execTool(t, tt, map[string]any{"path": "nm.txt", "pattern": "xyz", "replace": "abc"})
		var result replaceOutput
		json.Unmarshal(out, &result)
		if result.Modified {
			t.Error("expected modified to be false")
		}
	})
}

func TestFileopsDiff(t *testing.T) {
	baseDir := t.TempDir()
	tt := getTool(t, baseDir, "fileops_diff")

	t.Run("identical files", func(t *testing.T) {
		os.WriteFile(filepath.Join(baseDir, "a.txt"), []byte("same content"), 0600)
		os.WriteFile(filepath.Join(baseDir, "b.txt"), []byte("same content"), 0600)
		out := execTool(t, tt, map[string]any{"path_a": "a.txt", "path_b": "b.txt"})
		var result diffOutput
		json.Unmarshal(out, &result)
		if !result.Identical {
			t.Error("expected identical to be true")
		}
	})

	t.Run("different files", func(t *testing.T) {
		os.WriteFile(filepath.Join(baseDir, "c.txt"), []byte("line1\nline2"), 0600)
		os.WriteFile(filepath.Join(baseDir, "d.txt"), []byte("line1\nchanged"), 0600)
		out := execTool(t, tt, map[string]any{"path_a": "c.txt", "path_b": "d.txt"})
		var result diffOutput
		json.Unmarshal(out, &result)
		if result.Identical {
			t.Error("expected identical to be false")
		}
		if len(result.Changes) == 0 {
			t.Error("expected changes to be non-empty")
		}
	})
}

func TestFileopsArchiveAndExtract(t *testing.T) {
	baseDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(baseDir, "file1.txt"), []byte("content1"), 0600)
	os.MkdirAll(filepath.Join(baseDir, "subdir"), 0750)
	os.WriteFile(filepath.Join(baseDir, "subdir", "file2.txt"), []byte("content2"), 0600)

	archiveTool := getTool(t, baseDir, "fileops_archive")
	extractTool := getTool(t, baseDir, "fileops_extract")

	t.Run("zip archive and extract", func(t *testing.T) {
		// Archive
		out := execTool(t, archiveTool, map[string]any{
			"paths":  []string{"file1.txt", "subdir"},
			"output": "archive.zip",
			"format": "zip",
		})
		var archResult archiveOutput
		json.Unmarshal(out, &archResult)
		if archResult.FileCount != 2 {
			t.Errorf("expected 2 files archived, got %d", archResult.FileCount)
		}
		if archResult.Size <= 0 {
			t.Error("expected archive size > 0")
		}

		// Extract
		out = execTool(t, extractTool, map[string]any{
			"path":   "archive.zip",
			"output": "extracted",
		})
		var extResult extractOutput
		json.Unmarshal(out, &extResult)
		if extResult.FileCount != 2 {
			t.Errorf("expected 2 files extracted, got %d", extResult.FileCount)
		}

		// Verify extracted content
		data, err := os.ReadFile(filepath.Join(baseDir, "extracted", "file1.txt"))
		if err != nil {
			t.Fatalf("failed to read extracted file: %v", err)
		}
		if string(data) != "content1" {
			t.Errorf("extracted content mismatch: %q", string(data))
		}
	})

	t.Run("tar.gz archive and extract", func(t *testing.T) {
		out := execTool(t, archiveTool, map[string]any{
			"paths":  []string{"file1.txt"},
			"output": "archive.tar.gz",
			"format": "tar.gz",
		})
		var archResult archiveOutput
		json.Unmarshal(out, &archResult)
		if archResult.FileCount != 1 {
			t.Errorf("expected 1 file archived, got %d", archResult.FileCount)
		}

		out = execTool(t, extractTool, map[string]any{
			"path":   "archive.tar.gz",
			"output": "extracted_tgz",
		})
		var extResult extractOutput
		json.Unmarshal(out, &extResult)
		if extResult.FileCount != 1 {
			t.Errorf("expected 1 file extracted, got %d", extResult.FileCount)
		}
	})
}

func TestFileopsChecksum(t *testing.T) {
	baseDir := t.TempDir()
	content := []byte("test content for checksum")
	os.WriteFile(filepath.Join(baseDir, "check.txt"), content, 0600)

	tt := getTool(t, baseDir, "fileops_checksum")

	t.Run("sha256 default", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "check.txt"})
		var result checksumOutput
		json.Unmarshal(out, &result)
		if result.Algorithm != "sha256" {
			t.Errorf("expected algorithm sha256, got %s", result.Algorithm)
		}
		expected := sha256.Sum256(content)
		expectedHex := hex.EncodeToString(expected[:])
		if result.Checksum != expectedHex {
			t.Errorf("checksum mismatch: expected %s, got %s", expectedHex, result.Checksum)
		}
		if result.Size != int64(len(content)) {
			t.Errorf("expected size %d, got %d", len(content), result.Size)
		}
	})

	t.Run("md5", func(t *testing.T) {
		out := execTool(t, tt, map[string]any{"path": "check.txt", "algorithm": "md5"})
		var result checksumOutput
		json.Unmarshal(out, &result)
		if result.Algorithm != "md5" {
			t.Errorf("expected algorithm md5, got %s", result.Algorithm)
		}
		if result.Checksum == "" {
			t.Error("expected non-empty checksum")
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		input, _ := json.Marshal(map[string]any{"path": "check.txt", "algorithm": "crc32"})
		_, err := tt.Execute(context.Background(), input)
		if err == nil {
			t.Error("expected error for unsupported algorithm")
		}
	})
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		path     string
		expected bool
	}{
		{"same dir", "/base", "/base", true},
		{"subdir", "/base", "/base/sub", true},
		{"parent escape", "/base", "/base/../etc", false},
		{"absolute escape", "/base", "/etc/passwd", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSubPath(tc.base, tc.path); got != tc.expected {
				t.Errorf("isSubPath(%q, %q) = %v, want %v", tc.base, tc.path, got, tc.expected)
			}
		})
	}
}
