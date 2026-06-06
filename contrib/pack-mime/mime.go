// Package mime provides MIME type handling tools for agents.
package mime

import (
	"context"
	"encoding/json"
	"mime"
	"path/filepath"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Common MIME types database
var mimeTypes = map[string]string{
	// Text
	".txt":  "text/plain",
	".html": "text/html",
	".htm":  "text/html",
	".css":  "text/css",
	".js":   "text/javascript",
	".json": "application/json",
	".xml":  "application/xml",
	".csv":  "text/csv",
	".md":   "text/markdown",

	// Images
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
	".ico":  "image/x-icon",
	".bmp":  "image/bmp",

	// Audio
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".flac": "audio/flac",

	// Video
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",

	// Documents
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",

	// Archives
	".zip": "application/zip",
	".tar": "application/x-tar",
	".gz":  "application/gzip",
	".rar": "application/vnd.rar",
	".7z":  "application/x-7z-compressed",

	// Code
	".go":   "text/x-go",
	".py":   "text/x-python",
	".java": "text/x-java",
	".rs":   "text/x-rust",
	".ts":   "text/typescript",
	".tsx":  "text/typescript-jsx",
	".jsx":  "text/javascript-jsx",

	// Data
	".yaml": "application/x-yaml",
	".yml":  "application/x-yaml",
	".toml": "application/toml",

	// Fonts
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
}

// Reverse lookup
var extByMime = func() map[string][]string {
	result := make(map[string][]string)
	for ext, mimeType := range mimeTypes {
		result[mimeType] = append(result[mimeType], ext)
	}
	return result
}()

// Pack returns the MIME type tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("mime").
		WithDescription("MIME type handling tools").
		AddTools(
			detectTool(),
			extensionTool(),
			parseTool(),
			formatTool(),
			categoryTool(),
			isTextTool(),
			isBinaryTool(),
			validateTool(),
			lookupTool(),
			listCategoryTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func detectTool() tool.Tool {
	return tool.NewBuilder("mime_detect").
		WithDescription("Detect MIME type from filename").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Filename string `json:"filename"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ext := strings.ToLower(filepath.Ext(params.Filename))

			// Try our database first
			mimeType, ok := mimeTypes[ext]
			if !ok {
				// Fall back to stdlib
				mimeType = mime.TypeByExtension(ext)
			}

			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			result := map[string]any{
				"filename":  params.Filename,
				"extension": ext,
				"mime_type": mimeType,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extensionTool() tool.Tool {
	return tool.NewBuilder("mime_extension").
		WithDescription("Get file extension for MIME type").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				MimeType string `json:"mime_type"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Try our database first
			extensions := extByMime[params.MimeType]

			// Fall back to stdlib if not found
			if len(extensions) == 0 {
				exts, _ := mime.ExtensionsByType(params.MimeType)
				extensions = exts
			}

			var primaryExt string
			if len(extensions) > 0 {
				primaryExt = extensions[0]
			}

			result := map[string]any{
				"mime_type":  params.MimeType,
				"extension":  primaryExt,
				"extensions": extensions,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("mime_parse").
		WithDescription("Parse a MIME type string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				MimeType string `json:"mime_type"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			mediaType, params2, err := mime.ParseMediaType(params.MimeType)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			parts := strings.SplitN(mediaType, "/", 2)
			mainType := parts[0]
			subType := ""
			if len(parts) > 1 {
				subType = parts[1]
			}

			result := map[string]any{
				"valid":      true,
				"media_type": mediaType,
				"type":       mainType,
				"subtype":    subType,
				"params":     params2,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatTool() tool.Tool {
	return tool.NewBuilder("mime_format").
		WithDescription("Format a MIME type with parameters").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				MimeType string            `json:"mime_type"`
				Params   map[string]string `json:"params,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			formatted := mime.FormatMediaType(params.MimeType, params.Params)

			result := map[string]any{
				"formatted": formatted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func categoryTool() tool.Tool {
	return tool.NewBuilder("mime_category").
		WithDescription("Get category for MIME type").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				MimeType string `json:"mime_type"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			parts := strings.SplitN(params.MimeType, "/", 2)
			mainType := parts[0]

			var category string
			switch mainType {
			case "text":
				category = "text"
			case "image":
				category = "image"
			case "audio":
				category = "audio"
			case "video":
				category = "video"
			case "application":
				// Check for specific application types
				if len(parts) > 1 {
					subType := parts[1]
					switch {
					case strings.Contains(subType, "json") || strings.Contains(subType, "xml") ||
						strings.Contains(subType, "yaml") || strings.Contains(subType, "text"):
						category = "data"
					case strings.Contains(subType, "zip") || strings.Contains(subType, "tar") ||
						strings.Contains(subType, "gzip") || strings.Contains(subType, "compressed"):
						category = "archive"
					case strings.Contains(subType, "pdf") || strings.Contains(subType, "word") ||
						strings.Contains(subType, "excel") || strings.Contains(subType, "powerpoint") ||
						strings.Contains(subType, "document") || strings.Contains(subType, "spreadsheet"):
						category = "document"
					case strings.Contains(subType, "javascript") || strings.Contains(subType, "wasm"):
						category = "code"
					default:
						category = "application"
					}
				} else {
					category = "application"
				}
			case "font":
				category = "font"
			case "model":
				category = "3d"
			case "multipart":
				category = "multipart"
			case "message":
				category = "message"
			default:
				category = "unknown"
			}

			result := map[string]any{
				"mime_type": params.MimeType,
				"type":      mainType,
				"category":  category,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isTextTool() tool.Tool {
	return tool.NewBuilder("mime_is_text").
		WithDescription("Check if MIME type is text-based").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				MimeType string `json:"mime_type"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			isText := strings.HasPrefix(params.MimeType, "text/") ||
				params.MimeType == "application/json" ||
				params.MimeType == "application/xml" ||
				params.MimeType == "application/javascript" ||
				strings.HasSuffix(params.MimeType, "+json") ||
				strings.HasSuffix(params.MimeType, "+xml")

			result := map[string]any{
				"mime_type": params.MimeType,
				"is_text":   isText,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isBinaryTool() tool.Tool {
	return tool.NewBuilder("mime_is_binary").
		WithDescription("Check if MIME type is binary").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				MimeType string `json:"mime_type"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			isText := strings.HasPrefix(params.MimeType, "text/") ||
				params.MimeType == "application/json" ||
				params.MimeType == "application/xml" ||
				params.MimeType == "application/javascript" ||
				strings.HasSuffix(params.MimeType, "+json") ||
				strings.HasSuffix(params.MimeType, "+xml")

			result := map[string]any{
				"mime_type": params.MimeType,
				"is_binary": !isText,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("mime_validate").
		WithDescription("Validate a MIME type string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				MimeType string `json:"mime_type"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			_, _, err := mime.ParseMediaType(params.MimeType)
			valid := err == nil

			// Additional checks
			if valid {
				parts := strings.SplitN(params.MimeType, "/", 2)
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					valid = false
				}
			}

			result := map[string]any{
				"mime_type": params.MimeType,
				"valid":     valid,
			}
			if err != nil {
				result["error"] = err.Error()
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lookupTool() tool.Tool {
	return tool.NewBuilder("mime_lookup").
		WithDescription("Lookup MIME type in database").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Extension string `json:"extension,omitempty"`
				MimeType  string `json:"mime_type,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Extension != "" {
				ext := params.Extension
				if !strings.HasPrefix(ext, ".") {
					ext = "." + ext
				}
				ext = strings.ToLower(ext)

				mimeType, found := mimeTypes[ext]

				result := map[string]any{
					"extension": ext,
					"found":     found,
					"mime_type": mimeType,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			if params.MimeType != "" {
				extensions, found := extByMime[params.MimeType]

				result := map[string]any{
					"mime_type":  params.MimeType,
					"found":      found && len(extensions) > 0,
					"extensions": extensions,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"error": "provide either extension or mime_type",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listCategoryTool() tool.Tool {
	return tool.NewBuilder("mime_list_category").
		WithDescription("List MIME types by category").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Category string `json:"category"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var prefix string
			switch params.Category {
			case "text":
				prefix = "text/"
			case "image":
				prefix = "image/"
			case "audio":
				prefix = "audio/"
			case "video":
				prefix = "video/"
			case "font":
				prefix = "font/"
			default:
				prefix = params.Category + "/"
			}

			var types []map[string]any
			for ext, mimeType := range mimeTypes {
				if strings.HasPrefix(mimeType, prefix) {
					types = append(types, map[string]any{
						"extension": ext,
						"mime_type": mimeType,
					})
				}
			}

			result := map[string]any{
				"category": params.Category,
				"types":    types,
				"count":    len(types),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
