// Package compression provides data compression tools for agents.
package compression

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the compression tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("compression").
		WithDescription("Data compression tools").
		AddTools(
			gzipCompressTool(),
			gzipDecompressTool(),
			zlibCompressTool(),
			zlibDecompressTool(),
			deflateCompressTool(),
			deflateDecompressTool(),
			detectTool(),
			ratioTool(),
			compareTool(),
			benchmarkTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func gzipCompressTool() tool.Tool {
	return tool.NewBuilder("gzip_compress").
		WithDescription("Compress data using gzip").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data  string `json:"data"`
				Level int    `json:"level,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			level := params.Level
			if level == 0 {
				level = gzip.DefaultCompression
			}

			var buf bytes.Buffer
			writer, err := gzip.NewWriterLevel(&buf, level)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			_, err = writer.Write([]byte(params.Data))
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}
			if err := writer.Close(); err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			compressed := buf.Bytes()
			encoded := base64.StdEncoding.EncodeToString(compressed)

			result := map[string]any{
				"compressed":      encoded,
				"original_size":   len(params.Data),
				"compressed_size": len(compressed),
				"ratio":           float64(len(compressed)) / float64(len(params.Data)),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func gzipDecompressTool() tool.Tool {
	return tool.NewBuilder("gzip_decompress").
		WithDescription("Decompress gzip data").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			compressed, err := base64.StdEncoding.DecodeString(params.Data)
			if err != nil {
				result := map[string]any{
					"error": "invalid base64: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			reader, err := gzip.NewReader(bytes.NewReader(compressed))
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}
			defer reader.Close()

			decompressed, err := io.ReadAll(reader)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"decompressed":      string(decompressed),
				"compressed_size":   len(compressed),
				"decompressed_size": len(decompressed),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func zlibCompressTool() tool.Tool {
	return tool.NewBuilder("zlib_compress").
		WithDescription("Compress data using zlib").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data  string `json:"data"`
				Level int    `json:"level,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			level := params.Level
			if level == 0 {
				level = zlib.DefaultCompression
			}

			var buf bytes.Buffer
			writer, err := zlib.NewWriterLevel(&buf, level)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			_, err = writer.Write([]byte(params.Data))
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}
			if err := writer.Close(); err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			compressed := buf.Bytes()
			encoded := base64.StdEncoding.EncodeToString(compressed)

			result := map[string]any{
				"compressed":      encoded,
				"original_size":   len(params.Data),
				"compressed_size": len(compressed),
				"ratio":           float64(len(compressed)) / float64(len(params.Data)),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func zlibDecompressTool() tool.Tool {
	return tool.NewBuilder("zlib_decompress").
		WithDescription("Decompress zlib data").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			compressed, err := base64.StdEncoding.DecodeString(params.Data)
			if err != nil {
				result := map[string]any{
					"error": "invalid base64: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			reader, err := zlib.NewReader(bytes.NewReader(compressed))
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}
			defer reader.Close()

			decompressed, err := io.ReadAll(reader)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"decompressed":      string(decompressed),
				"compressed_size":   len(compressed),
				"decompressed_size": len(decompressed),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deflateCompressTool() tool.Tool {
	return tool.NewBuilder("deflate_compress").
		WithDescription("Compress data using DEFLATE").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data  string `json:"data"`
				Level int    `json:"level,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			level := params.Level
			if level == 0 {
				level = flate.DefaultCompression
			}

			var buf bytes.Buffer
			writer, err := flate.NewWriter(&buf, level)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			_, err = writer.Write([]byte(params.Data))
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}
			if err := writer.Close(); err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			compressed := buf.Bytes()
			encoded := base64.StdEncoding.EncodeToString(compressed)

			result := map[string]any{
				"compressed":      encoded,
				"original_size":   len(params.Data),
				"compressed_size": len(compressed),
				"ratio":           float64(len(compressed)) / float64(len(params.Data)),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deflateDecompressTool() tool.Tool {
	return tool.NewBuilder("deflate_decompress").
		WithDescription("Decompress DEFLATE data").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			compressed, err := base64.StdEncoding.DecodeString(params.Data)
			if err != nil {
				result := map[string]any{
					"error": "invalid base64: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			reader := flate.NewReader(bytes.NewReader(compressed))
			defer reader.Close()

			decompressed, err := io.ReadAll(reader)
			if err != nil {
				result := map[string]any{
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"decompressed":      string(decompressed),
				"compressed_size":   len(compressed),
				"decompressed_size": len(decompressed),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func detectTool() tool.Tool {
	return tool.NewBuilder("compression_detect").
		WithDescription("Detect compression format").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data, err := base64.StdEncoding.DecodeString(params.Data)
			if err != nil {
				result := map[string]any{
					"error": "invalid base64: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var format string
			var valid bool

			// Check gzip magic bytes (1f 8b)
			if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
				format = "gzip"
				_, err := gzip.NewReader(bytes.NewReader(data))
				valid = err == nil
			} else if len(data) >= 2 {
				// Check zlib header
				cmf := data[0]
				flg := data[1]
				if (cmf&0x0f) == 8 && ((int(cmf)*256+int(flg))%31) == 0 {
					format = "zlib"
					_, err := zlib.NewReader(bytes.NewReader(data))
					valid = err == nil
				} else {
					// Try deflate
					reader := flate.NewReader(bytes.NewReader(data))
					_, err := io.ReadAll(reader)
					_ = reader.Close() // #nosec G104 -- best-effort close
					if err == nil {
						format = "deflate"
						valid = true
					} else {
						format = "unknown"
						valid = false
					}
				}
			}

			result := map[string]any{
				"format": format,
				"valid":  valid,
				"size":   len(data),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func ratioTool() tool.Tool {
	return tool.NewBuilder("compression_ratio").
		WithDescription("Calculate compression ratio").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				OriginalSize   int `json:"original_size"`
				CompressedSize int `json:"compressed_size"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.OriginalSize == 0 {
				result := map[string]any{
					"error": "original size cannot be zero",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			ratio := float64(params.CompressedSize) / float64(params.OriginalSize)
			savings := 1.0 - ratio
			percentSaved := savings * 100

			result := map[string]any{
				"ratio":         ratio,
				"savings":       savings,
				"percent_saved": percentSaved,
				"bytes_saved":   params.OriginalSize - params.CompressedSize,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareTool() tool.Tool {
	return tool.NewBuilder("compression_compare").
		WithDescription("Compare compression methods on data").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			originalSize := len(params.Data)

			// Try gzip
			var gzipBuf bytes.Buffer
			gzipWriter := gzip.NewWriter(&gzipBuf)
			_, _ = gzipWriter.Write([]byte(params.Data)) // #nosec G104 -- comparison tool, errors don't affect result validity
			_ = gzipWriter.Close()                       // #nosec G104
			gzipSize := gzipBuf.Len()

			// Try zlib
			var zlibBuf bytes.Buffer
			zlibWriter := zlib.NewWriter(&zlibBuf)
			_, _ = zlibWriter.Write([]byte(params.Data)) // #nosec G104 -- comparison tool, errors don't affect result validity
			_ = zlibWriter.Close()                       // #nosec G104
			zlibSize := zlibBuf.Len()

			// Try deflate
			var deflateBuf bytes.Buffer
			deflateWriter, _ := flate.NewWriter(&deflateBuf, flate.DefaultCompression)
			_, _ = deflateWriter.Write([]byte(params.Data)) // #nosec G104 -- comparison tool, errors don't affect result validity
			_ = deflateWriter.Close()                       // #nosec G104
			deflateSize := deflateBuf.Len()

			// Find best
			best := "gzip"
			bestSize := gzipSize
			if zlibSize < bestSize {
				best = "zlib"
				bestSize = zlibSize
			}
			if deflateSize < bestSize {
				best = "deflate"
				bestSize = deflateSize
			}

			result := map[string]any{
				"original_size": originalSize,
				"gzip": map[string]any{
					"size":  gzipSize,
					"ratio": float64(gzipSize) / float64(originalSize),
				},
				"zlib": map[string]any{
					"size":  zlibSize,
					"ratio": float64(zlibSize) / float64(originalSize),
				},
				"deflate": map[string]any{
					"size":  deflateSize,
					"ratio": float64(deflateSize) / float64(originalSize),
				},
				"best":       best,
				"best_ratio": float64(bestSize) / float64(originalSize),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func benchmarkTool() tool.Tool {
	return tool.NewBuilder("compression_benchmark").
		WithDescription("Benchmark compression at different levels").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Data   string `json:"data"`
				Method string `json:"method,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			method := params.Method
			if method == "" {
				method = "gzip"
			}

			originalSize := len(params.Data)
			levels := []int{1, 5, 9}
			results := make([]map[string]any, 0)

			for _, level := range levels {
				var buf bytes.Buffer
				var err error

				switch method {
				case "gzip":
					var w *gzip.Writer
					w, err = gzip.NewWriterLevel(&buf, level)
					if err == nil {
						_, _ = w.Write([]byte(params.Data)) // #nosec G104 -- benchmark tool, errors don't affect result validity
						_ = w.Close()                       // #nosec G104
					}
				case "zlib":
					var w *zlib.Writer
					w, err = zlib.NewWriterLevel(&buf, level)
					if err == nil {
						_, _ = w.Write([]byte(params.Data)) // #nosec G104 -- benchmark tool, errors don't affect result validity
						_ = w.Close()                       // #nosec G104
					}
				case "deflate":
					var w *flate.Writer
					w, err = flate.NewWriter(&buf, level)
					if err == nil {
						_, _ = w.Write([]byte(params.Data)) // #nosec G104 -- benchmark tool, errors don't affect result validity
						_ = w.Close()                       // #nosec G104
					}
				}

				if err != nil {
					continue
				}

				compressedSize := buf.Len()
				results = append(results, map[string]any{
					"level": level,
					"size":  compressedSize,
					"ratio": float64(compressedSize) / float64(originalSize),
				})
			}

			result := map[string]any{
				"method":        method,
				"original_size": originalSize,
				"levels":        results,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
