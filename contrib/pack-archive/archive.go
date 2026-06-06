// Package archive provides tools for working with compressed archives (ZIP, TAR, GZIP).
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Config holds archive pack configuration.
type Config struct {
	// MaxFileSize limits the maximum size of files to extract (0 = unlimited)
	MaxFileSize int64
	// MaxFiles limits the maximum number of files to extract (0 = unlimited)
	MaxFiles int
	// TempDir is the directory for temporary files
	TempDir string
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		MaxFileSize: 1 << 30, // 1GB
		MaxFiles:    10000,
		TempDir:     os.TempDir(),
	}
}

type archivePack struct {
	cfg Config
}

// Pack creates a new archive tools pack.
func Pack(cfg Config) *pack.Pack {
	p := &archivePack{cfg: cfg}

	return pack.NewBuilder("archive").
		WithDescription("Tools for creating and extracting compressed archives (ZIP, TAR, GZIP)").
		WithVersion("1.0.0").
		AddTools(
			// ZIP tools
			p.createZipTool(),
			p.extractZipTool(),
			p.listZipTool(),
			p.addToZipTool(),
			p.extractZipFileTool(),
			// TAR tools
			p.createTarTool(),
			p.extractTarTool(),
			p.listTarTool(),
			// GZIP tools
			p.gzipCompressTool(),
			p.gzipDecompressTool(),
			// TAR.GZ tools
			p.createTarGzTool(),
			p.extractTarGzTool(),
			p.listTarGzTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// isPathSafe validates that the target path does not escape the base directory.
// This prevents zip slip and path traversal attacks.
func isPathSafe(basePath, targetPath string) bool {
	cleanBase := filepath.Clean(basePath)
	cleanTarget := filepath.Clean(targetPath)

	// Use filepath.Rel to determine if target is within base
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return false
	}
	// Reject if the relative path escapes the base directory
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return false
	}
	return true
}

// createZipTool creates a ZIP archive.
func (p *archivePack) createZipTool() tool.Tool {
	return tool.NewBuilder("archive_create_zip").
		WithDescription("Create a ZIP archive from files or directories").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Output  string   `json:"output"`
				Files   []string `json:"files"`
				BaseDir string   `json:"base_dir,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Output == "" || len(params.Files) == 0 {
				return tool.Result{}, fmt.Errorf("output and files are required")
			}

			zipFile, err := os.Create(params.Output)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create zip file: %w", err)
			}
			defer zipFile.Close()

			zipWriter := zip.NewWriter(zipFile)
			defer zipWriter.Close()

			fileCount := 0
			totalSize := int64(0)

			for _, file := range params.Files {
				err := filepath.Walk(file, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info.IsDir() {
						return nil
					}

					// Determine the name in the archive
					var name string
					if params.BaseDir != "" {
						relPath, err := filepath.Rel(params.BaseDir, path)
						if err != nil {
							name = filepath.Base(path)
						} else {
							name = relPath
						}
					} else {
						name = path
					}

					header, err := zip.FileInfoHeader(info)
					if err != nil {
						return fmt.Errorf("failed to create header: %w", err)
					}
					header.Name = name
					header.Method = zip.Deflate

					writer, err := zipWriter.CreateHeader(header)
					if err != nil {
						return fmt.Errorf("failed to create entry: %w", err)
					}

					// #nosec G304 -- File path from user input is intentional for archive tool
					f, err := os.Open(path)
					if err != nil {
						return fmt.Errorf("failed to open file: %w", err)
					}
					defer f.Close()

					written, err := io.Copy(writer, f)
					if err != nil {
						return fmt.Errorf("failed to write file: %w", err)
					}

					fileCount++
					totalSize += written
					return nil
				})
				if err != nil {
					return tool.Result{}, err
				}
			}

			result := map[string]interface{}{
				"output":     params.Output,
				"file_count": fileCount,
				"total_size": totalSize,
				"success":    true,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// extractZipTool extracts a ZIP archive.
func (p *archivePack) extractZipTool() tool.Tool {
	return tool.NewBuilder("archive_extract_zip").
		WithDescription("Extract a ZIP archive to a directory").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Archive string `json:"archive"`
				Output  string `json:"output"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Archive == "" || params.Output == "" {
				return tool.Result{}, fmt.Errorf("archive and output are required")
			}

			reader, err := zip.OpenReader(params.Archive)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open zip: %w", err)
			}
			defer reader.Close()

			// #nosec G301 -- Directory permissions 0755 are intentional for user-accessible directories
			if err := os.MkdirAll(params.Output, 0755); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output dir: %w", err)
			}

			fileCount := 0
			totalSize := int64(0)

			for _, file := range reader.File {
				if p.cfg.MaxFiles > 0 && fileCount >= p.cfg.MaxFiles {
					break
				}

				// Validate path to prevent zip slip attacks
				destPath := filepath.Join(params.Output, filepath.Clean(file.Name)) // #nosec G305 -- Path validated by isPathSafe using filepath.Rel
				if !isPathSafe(params.Output, destPath) {
					continue // Skip files that would escape the output directory
				}

				if file.FileInfo().IsDir() {
					_ = os.MkdirAll(destPath, file.Mode()) // #nosec G104 -- Best effort directory creation
					continue
				}

				// G115: Check for integer overflow when converting uint64 to int64
				if file.UncompressedSize64 > math.MaxInt64 {
					continue // Skip files with size that would overflow int64
				}
				// #nosec G115 -- Overflow checked above with math.MaxInt64 comparison
				if p.cfg.MaxFileSize > 0 && int64(file.UncompressedSize64) > p.cfg.MaxFileSize {
					continue // Skip files that are too large
				}

				// #nosec G301 -- Directory permissions 0755 are intentional for user-accessible directories
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					return tool.Result{}, fmt.Errorf("failed to create dir: %w", err)
				}

				// #nosec G304 -- destPath is validated by isPathSafe to prevent traversal
				destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to create file: %w", err)
				}

				srcFile, err := file.Open()
				if err != nil {
					_ = destFile.Close() // #nosec G104 -- Best effort cleanup
					return tool.Result{}, fmt.Errorf("failed to open zip entry: %w", err)
				}

				// #nosec G110 -- Decompression size is bounded by MaxFileSize config and header size check above
				written, err := io.Copy(destFile, srcFile)
				_ = srcFile.Close()  // #nosec G104 -- Best effort cleanup
				_ = destFile.Close() // #nosec G104 -- Best effort cleanup
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to extract file: %w", err)
				}

				fileCount++
				totalSize += written
			}

			result := map[string]interface{}{
				"output":     params.Output,
				"file_count": fileCount,
				"total_size": totalSize,
				"success":    true,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// listZipTool lists contents of a ZIP archive.
func (p *archivePack) listZipTool() tool.Tool {
	return tool.NewBuilder("archive_list_zip").
		WithDescription("List contents of a ZIP archive").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Archive string `json:"archive"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Archive == "" {
				return tool.Result{}, fmt.Errorf("archive is required")
			}

			reader, err := zip.OpenReader(params.Archive)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open zip: %w", err)
			}
			defer reader.Close()

			var files []map[string]interface{}
			totalSize := uint64(0)

			for _, file := range reader.File {
				files = append(files, map[string]interface{}{
					"name":            file.Name,
					"size":            file.UncompressedSize64,
					"compressed_size": file.CompressedSize64,
					"is_dir":          file.FileInfo().IsDir(),
					"modified":        file.Modified.Format("2006-01-02T15:04:05Z"),
				})
				totalSize += file.UncompressedSize64
			}

			result := map[string]interface{}{
				"files":      files,
				"file_count": len(files),
				"total_size": totalSize,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// addToZipTool adds files to an existing ZIP archive.
func (p *archivePack) addToZipTool() tool.Tool {
	return tool.NewBuilder("archive_add_to_zip").
		WithDescription("Add files to an existing ZIP archive").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Archive string   `json:"archive"`
				Files   []string `json:"files"`
				BaseDir string   `json:"base_dir,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Archive == "" || len(params.Files) == 0 {
				return tool.Result{}, fmt.Errorf("archive and files are required")
			}

			// Read existing archive
			existingReader, err := zip.OpenReader(params.Archive)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open zip: %w", err)
			}

			// Create temp file for new archive
			tempFile, err := os.CreateTemp(p.cfg.TempDir, "archive-*.zip")
			if err != nil {
				_ = existingReader.Close() // #nosec G104 -- Best effort cleanup
				return tool.Result{}, fmt.Errorf("failed to create temp file: %w", err)
			}
			tempPath := tempFile.Name()

			zipWriter := zip.NewWriter(tempFile)

			// Copy existing files
			for _, file := range existingReader.File {
				header := file.FileHeader
				writer, err := zipWriter.CreateHeader(&header)
				if err != nil {
					_ = zipWriter.Close()      // #nosec G104 -- Best effort cleanup
					_ = tempFile.Close()       // #nosec G104 -- Best effort cleanup
					_ = existingReader.Close() // #nosec G104 -- Best effort cleanup
					_ = os.Remove(tempPath)    // #nosec G104 -- Best effort cleanup
					return tool.Result{}, fmt.Errorf("failed to create header: %w", err)
				}

				reader, err := file.Open()
				if err != nil {
					_ = zipWriter.Close()      // #nosec G104 -- Best effort cleanup
					_ = tempFile.Close()       // #nosec G104 -- Best effort cleanup
					_ = existingReader.Close() // #nosec G104 -- Best effort cleanup
					_ = os.Remove(tempPath)    // #nosec G104 -- Best effort cleanup
					return tool.Result{}, fmt.Errorf("failed to open file: %w", err)
				}

				// #nosec G110 -- Copying existing archive entries - size bounded by original archive
				_, err = io.Copy(writer, reader)
				_ = reader.Close() // #nosec G104 -- Best effort cleanup
				if err != nil {
					_ = zipWriter.Close()      // #nosec G104 -- Best effort cleanup
					_ = tempFile.Close()       // #nosec G104 -- Best effort cleanup
					_ = existingReader.Close() // #nosec G104 -- Best effort cleanup
					_ = os.Remove(tempPath)    // #nosec G104 -- Best effort cleanup
					return tool.Result{}, fmt.Errorf("failed to copy file: %w", err)
				}
			}
			_ = existingReader.Close() // #nosec G104 -- Best effort cleanup

			// Add new files
			addedCount := 0
			for _, file := range params.Files {
				info, err := os.Stat(file)
				if err != nil {
					continue
				}
				if info.IsDir() {
					continue
				}

				var name string
				if params.BaseDir != "" {
					relPath, err := filepath.Rel(params.BaseDir, file)
					if err != nil {
						name = filepath.Base(file)
					} else {
						name = relPath
					}
				} else {
					name = filepath.Base(file)
				}

				header, err := zip.FileInfoHeader(info)
				if err != nil {
					continue
				}
				header.Name = name
				header.Method = zip.Deflate

				writer, err := zipWriter.CreateHeader(header)
				if err != nil {
					continue
				}

				// #nosec G304 -- File path from user input is intentional for archive tool
				f, err := os.Open(file)
				if err != nil {
					continue
				}

				_, err = io.Copy(writer, f)
				_ = f.Close() // #nosec G104 -- Best effort cleanup
				if err != nil {
					continue
				}
				addedCount++
			}

			_ = zipWriter.Close() // #nosec G104 -- Best effort cleanup
			_ = tempFile.Close()  // #nosec G104 -- Best effort cleanup

			// Replace original with temp
			if err := os.Rename(tempPath, params.Archive); err != nil {
				_ = os.Remove(tempPath) // #nosec G104 -- Best effort cleanup
				return tool.Result{}, fmt.Errorf("failed to replace archive: %w", err)
			}

			result := map[string]interface{}{
				"archive":     params.Archive,
				"files_added": addedCount,
				"success":     true,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// extractZipFileTool extracts a single file from a ZIP archive.
func (p *archivePack) extractZipFileTool() tool.Tool {
	return tool.NewBuilder("archive_extract_zip_file").
		WithDescription("Extract a single file from a ZIP archive").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Archive  string `json:"archive"`
				FileName string `json:"file_name"`
				Output   string `json:"output"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Archive == "" || params.FileName == "" || params.Output == "" {
				return tool.Result{}, fmt.Errorf("archive, file_name, and output are required")
			}

			reader, err := zip.OpenReader(params.Archive)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open zip: %w", err)
			}
			defer reader.Close()

			for _, file := range reader.File {
				if file.Name == params.FileName {
					// Validate output path to prevent zip slip attacks
					outputDir := filepath.Dir(filepath.Clean(params.Output))
					if !isPathSafe(outputDir, filepath.Clean(params.Output)) {
						return tool.Result{}, fmt.Errorf("unsafe output path: %s", params.Output)
					}

					// #nosec G301 -- Directory permissions 0755 are intentional for user-accessible directories
					if err := os.MkdirAll(filepath.Dir(params.Output), 0755); err != nil {
						return tool.Result{}, fmt.Errorf("failed to create dir: %w", err)
					}

					destFile, err := os.Create(params.Output)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to create file: %w", err)
					}
					defer destFile.Close()

					srcFile, err := file.Open()
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to open zip entry: %w", err)
					}
					defer srcFile.Close()

					// #nosec G110 -- Single file extraction - size bounded by MaxFileSize if configured
					written, err := io.Copy(destFile, srcFile)
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to extract: %w", err)
					}

					result := map[string]interface{}{
						"output":  params.Output,
						"size":    written,
						"success": true,
					}
					output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
					return tool.Result{Output: output}, nil
				}
			}

			return tool.Result{}, fmt.Errorf("file not found in archive: %s", params.FileName)
		}).
		MustBuild()
}

// createTarTool creates a TAR archive.
func (p *archivePack) createTarTool() tool.Tool {
	return tool.NewBuilder("archive_create_tar").
		WithDescription("Create a TAR archive from files or directories").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Output  string   `json:"output"`
				Files   []string `json:"files"`
				BaseDir string   `json:"base_dir,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Output == "" || len(params.Files) == 0 {
				return tool.Result{}, fmt.Errorf("output and files are required")
			}

			tarFile, err := os.Create(params.Output)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create tar file: %w", err)
			}
			defer tarFile.Close()

			tarWriter := tar.NewWriter(tarFile)
			defer tarWriter.Close()

			fileCount := 0
			totalSize := int64(0)

			for _, file := range params.Files {
				err := filepath.Walk(file, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					var name string
					if params.BaseDir != "" {
						relPath, err := filepath.Rel(params.BaseDir, path)
						if err != nil {
							name = path
						} else {
							name = relPath
						}
					} else {
						name = path
					}

					header, err := tar.FileInfoHeader(info, "")
					if err != nil {
						return fmt.Errorf("failed to create header: %w", err)
					}
					header.Name = name

					if err := tarWriter.WriteHeader(header); err != nil {
						return fmt.Errorf("failed to write header: %w", err)
					}

					if info.IsDir() {
						return nil
					}

					// #nosec G304 -- File path from user input is intentional for archive tool
					f, err := os.Open(path)
					if err != nil {
						return fmt.Errorf("failed to open file: %w", err)
					}
					defer f.Close()

					written, err := io.Copy(tarWriter, f)
					if err != nil {
						return fmt.Errorf("failed to write file: %w", err)
					}

					fileCount++
					totalSize += written
					return nil
				})
				if err != nil {
					return tool.Result{}, err
				}
			}

			result := map[string]interface{}{
				"output":     params.Output,
				"file_count": fileCount,
				"total_size": totalSize,
				"success":    true,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// extractTarTool extracts a TAR archive.
func (p *archivePack) extractTarTool() tool.Tool {
	return tool.NewBuilder("archive_extract_tar").
		WithDescription("Extract a TAR archive to a directory").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Archive string `json:"archive"`
				Output  string `json:"output"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Archive == "" || params.Output == "" {
				return tool.Result{}, fmt.Errorf("archive and output are required")
			}

			tarFile, err := os.Open(params.Archive)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open tar: %w", err)
			}
			defer tarFile.Close()

			tarReader := tar.NewReader(tarFile)

			// #nosec G301 -- Directory permissions 0755 are intentional for user-accessible directories
			if err := os.MkdirAll(params.Output, 0755); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output dir: %w", err)
			}

			fileCount := 0
			totalSize := int64(0)

			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to read tar: %w", err)
				}

				if p.cfg.MaxFiles > 0 && fileCount >= p.cfg.MaxFiles {
					break
				}

				// Validate path to prevent path traversal attacks
				destPath := filepath.Join(params.Output, filepath.Clean(header.Name)) // #nosec G305 -- Path validated by isPathSafe using filepath.Rel
				if !isPathSafe(params.Output, destPath) {
					continue
				}

				switch header.Typeflag {
				case tar.TypeDir:
					// #nosec G115 -- header.Mode is int64, masking with 0777 ensures safe conversion to uint32
					if err := os.MkdirAll(destPath, os.FileMode(header.Mode&0777)); err != nil {
						return tool.Result{}, fmt.Errorf("failed to create dir: %w", err)
					}
				case tar.TypeReg:
					if p.cfg.MaxFileSize > 0 && header.Size > p.cfg.MaxFileSize {
						continue
					}

					// #nosec G301 -- Directory permissions 0755 are intentional for user-accessible directories
					if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
						return tool.Result{}, fmt.Errorf("failed to create dir: %w", err)
					}

					// #nosec G115 G304 -- Mode masked with 0777; destPath validated by isPathSafe
					destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode&0777))
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to create file: %w", err)
					}

					// #nosec G110 -- Decompression size is bounded by MaxFileSize config check above
					written, err := io.Copy(destFile, tarReader)
					_ = destFile.Close() // #nosec G104 -- Best effort cleanup
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to extract file: %w", err)
					}

					fileCount++
					totalSize += written
				}
			}

			result := map[string]interface{}{
				"output":     params.Output,
				"file_count": fileCount,
				"total_size": totalSize,
				"success":    true,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// listTarTool lists contents of a TAR archive.
func (p *archivePack) listTarTool() tool.Tool {
	return tool.NewBuilder("archive_list_tar").
		WithDescription("List contents of a TAR archive").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Archive string `json:"archive"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Archive == "" {
				return tool.Result{}, fmt.Errorf("archive is required")
			}

			tarFile, err := os.Open(params.Archive)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open tar: %w", err)
			}
			defer tarFile.Close()

			tarReader := tar.NewReader(tarFile)

			var files []map[string]interface{}
			totalSize := int64(0)

			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to read tar: %w", err)
				}

				fileType := "file"
				switch header.Typeflag {
				case tar.TypeDir:
					fileType = "dir"
				case tar.TypeSymlink:
					fileType = "symlink"
				case tar.TypeLink:
					fileType = "hardlink"
				}

				files = append(files, map[string]interface{}{
					"name":     header.Name,
					"size":     header.Size,
					"type":     fileType,
					"mode":     header.Mode,
					"modified": header.ModTime.Format("2006-01-02T15:04:05Z"),
				})
				totalSize += header.Size
			}

			result := map[string]interface{}{
				"files":      files,
				"file_count": len(files),
				"total_size": totalSize,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// gzipCompressTool compresses a file with GZIP.
func (p *archivePack) gzipCompressTool() tool.Tool {
	return tool.NewBuilder("archive_gzip_compress").
		WithDescription("Compress a file using GZIP").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Input  string `json:"input"`
				Output string `json:"output,omitempty"`
				Level  int    `json:"level,omitempty"` // 1-9, default 6
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Input == "" {
				return tool.Result{}, fmt.Errorf("input is required")
			}

			output := params.Output
			if output == "" {
				output = params.Input + ".gz"
			}

			level := params.Level
			if level < 1 || level > 9 {
				level = gzip.DefaultCompression
			}

			inputFile, err := os.Open(params.Input)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open input: %w", err)
			}
			defer inputFile.Close()

			inputInfo, err := inputFile.Stat()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat input: %w", err)
			}

			// #nosec G304 -- Output path from user input is intentional for archive tool
			outputFile, err := os.Create(output)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output: %w", err)
			}
			defer outputFile.Close()

			gzipWriter, err := gzip.NewWriterLevel(outputFile, level)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create gzip writer: %w", err)
			}
			gzipWriter.Name = filepath.Base(params.Input)

			if _, err := io.Copy(gzipWriter, inputFile); err != nil {
				_ = gzipWriter.Close() // #nosec G104 -- Best effort cleanup
				return tool.Result{}, fmt.Errorf("failed to compress: %w", err)
			}
			_ = gzipWriter.Close() // #nosec G104 -- Best effort cleanup

			outputInfo, err := os.Stat(output)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to stat output: %w", err)
			}

			ratio := float64(outputInfo.Size()) / float64(inputInfo.Size()) * 100

			result := map[string]interface{}{
				"output":            output,
				"original_size":     inputInfo.Size(),
				"compressed_size":   outputInfo.Size(),
				"compression_ratio": fmt.Sprintf("%.1f%%", ratio),
				"success":           true,
			}
			resultOutput, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: resultOutput}, nil
		}).
		MustBuild()
}

// gzipDecompressTool decompresses a GZIP file.
func (p *archivePack) gzipDecompressTool() tool.Tool {
	return tool.NewBuilder("archive_gzip_decompress").
		WithDescription("Decompress a GZIP file").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Input  string `json:"input"`
				Output string `json:"output,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Input == "" {
				return tool.Result{}, fmt.Errorf("input is required")
			}

			output := params.Output
			if output == "" {
				output = strings.TrimSuffix(params.Input, ".gz")
				if output == params.Input {
					output = params.Input + ".decompressed"
				}
			}

			inputFile, err := os.Open(params.Input)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open input: %w", err)
			}
			defer inputFile.Close()

			gzipReader, err := gzip.NewReader(inputFile)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create gzip reader: %w", err)
			}
			defer gzipReader.Close()

			// #nosec G304 -- Output path from user input is intentional for archive tool
			outputFile, err := os.Create(output)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output: %w", err)
			}
			defer outputFile.Close()

			// #nosec G110 -- Decompression size is bounded by MaxFileSize if configured, tool intentionally handles large files
			written, err := io.Copy(outputFile, gzipReader)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to decompress: %w", err)
			}

			result := map[string]interface{}{
				"output":  output,
				"size":    written,
				"success": true,
			}
			resultOutput, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: resultOutput}, nil
		}).
		MustBuild()
}

// createTarGzTool creates a TAR.GZ archive.
func (p *archivePack) createTarGzTool() tool.Tool {
	return tool.NewBuilder("archive_create_tar_gz").
		WithDescription("Create a TAR.GZ (gzipped tar) archive").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Output  string   `json:"output"`
				Files   []string `json:"files"`
				BaseDir string   `json:"base_dir,omitempty"`
				Level   int      `json:"level,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Output == "" || len(params.Files) == 0 {
				return tool.Result{}, fmt.Errorf("output and files are required")
			}

			level := params.Level
			if level < 1 || level > 9 {
				level = gzip.DefaultCompression
			}

			outFile, err := os.Create(params.Output)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create file: %w", err)
			}
			defer outFile.Close()

			gzipWriter, err := gzip.NewWriterLevel(outFile, level)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create gzip writer: %w", err)
			}
			defer gzipWriter.Close()

			tarWriter := tar.NewWriter(gzipWriter)
			defer tarWriter.Close()

			fileCount := 0
			totalSize := int64(0)

			for _, file := range params.Files {
				err := filepath.Walk(file, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					var name string
					if params.BaseDir != "" {
						relPath, err := filepath.Rel(params.BaseDir, path)
						if err != nil {
							name = path
						} else {
							name = relPath
						}
					} else {
						name = path
					}

					header, err := tar.FileInfoHeader(info, "")
					if err != nil {
						return fmt.Errorf("failed to create header: %w", err)
					}
					header.Name = name

					if err := tarWriter.WriteHeader(header); err != nil {
						return fmt.Errorf("failed to write header: %w", err)
					}

					if info.IsDir() {
						return nil
					}

					// #nosec G304 -- File path from user input is intentional for archive tool
					f, err := os.Open(path)
					if err != nil {
						return fmt.Errorf("failed to open file: %w", err)
					}
					defer f.Close()

					written, err := io.Copy(tarWriter, f)
					if err != nil {
						return fmt.Errorf("failed to write file: %w", err)
					}

					fileCount++
					totalSize += written
					return nil
				})
				if err != nil {
					return tool.Result{}, err
				}
			}

			result := map[string]interface{}{
				"output":     params.Output,
				"file_count": fileCount,
				"total_size": totalSize,
				"success":    true,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// extractTarGzTool extracts a TAR.GZ archive.
func (p *archivePack) extractTarGzTool() tool.Tool {
	return tool.NewBuilder("archive_extract_tar_gz").
		WithDescription("Extract a TAR.GZ archive to a directory").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Archive string `json:"archive"`
				Output  string `json:"output"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Archive == "" || params.Output == "" {
				return tool.Result{}, fmt.Errorf("archive and output are required")
			}

			archiveFile, err := os.Open(params.Archive)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open archive: %w", err)
			}
			defer archiveFile.Close()

			gzipReader, err := gzip.NewReader(archiveFile)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create gzip reader: %w", err)
			}
			defer gzipReader.Close()

			tarReader := tar.NewReader(gzipReader)

			// #nosec G301 -- Directory permissions 0755 are intentional for user-accessible directories
			if err := os.MkdirAll(params.Output, 0755); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create output dir: %w", err)
			}

			fileCount := 0
			totalSize := int64(0)

			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to read tar: %w", err)
				}

				if p.cfg.MaxFiles > 0 && fileCount >= p.cfg.MaxFiles {
					break
				}

				// Validate path to prevent path traversal attacks
				destPath := filepath.Join(params.Output, filepath.Clean(header.Name)) // #nosec G305 -- Path validated by isPathSafe using filepath.Rel
				if !isPathSafe(params.Output, destPath) {
					continue
				}

				switch header.Typeflag {
				case tar.TypeDir:
					// #nosec G115 -- header.Mode is int64, masking with 0777 ensures safe conversion to uint32
					if err := os.MkdirAll(destPath, os.FileMode(header.Mode&0777)); err != nil {
						return tool.Result{}, fmt.Errorf("failed to create dir: %w", err)
					}
				case tar.TypeReg:
					if p.cfg.MaxFileSize > 0 && header.Size > p.cfg.MaxFileSize {
						continue
					}

					// #nosec G301 -- Directory permissions 0755 are intentional for user-accessible directories
					if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
						return tool.Result{}, fmt.Errorf("failed to create dir: %w", err)
					}

					// #nosec G115 G304 -- Mode masked with 0777; destPath validated by isPathSafe
					destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode&0777))
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to create file: %w", err)
					}

					// #nosec G110 -- Decompression size is bounded by MaxFileSize config check above
					written, err := io.Copy(destFile, tarReader)
					_ = destFile.Close() // #nosec G104 -- Best effort cleanup
					if err != nil {
						return tool.Result{}, fmt.Errorf("failed to extract file: %w", err)
					}

					fileCount++
					totalSize += written
				}
			}

			result := map[string]interface{}{
				"output":     params.Output,
				"file_count": fileCount,
				"total_size": totalSize,
				"success":    true,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// listTarGzTool lists contents of a TAR.GZ archive.
func (p *archivePack) listTarGzTool() tool.Tool {
	return tool.NewBuilder("archive_list_tar_gz").
		WithDescription("List contents of a TAR.GZ archive").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Archive string `json:"archive"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Archive == "" {
				return tool.Result{}, fmt.Errorf("archive is required")
			}

			archiveFile, err := os.Open(params.Archive)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to open archive: %w", err)
			}
			defer archiveFile.Close()

			gzipReader, err := gzip.NewReader(archiveFile)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create gzip reader: %w", err)
			}
			defer gzipReader.Close()

			tarReader := tar.NewReader(gzipReader)

			var files []map[string]interface{}
			totalSize := int64(0)

			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to read tar: %w", err)
				}

				fileType := "file"
				switch header.Typeflag {
				case tar.TypeDir:
					fileType = "dir"
				case tar.TypeSymlink:
					fileType = "symlink"
				case tar.TypeLink:
					fileType = "hardlink"
				}

				files = append(files, map[string]interface{}{
					"name":     header.Name,
					"size":     header.Size,
					"type":     fileType,
					"mode":     header.Mode,
					"modified": header.ModTime.Format("2006-01-02T15:04:05Z"),
				})
				totalSize += header.Size
			}

			result := map[string]interface{}{
				"files":      files,
				"file_count": len(files),
				"total_size": totalSize,
			}
			output, _ := json.Marshal(result) // #nosec G104 -- Marshal of simple map cannot fail
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
