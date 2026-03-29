// Package base64 provides Base64 encoding and decoding tools for agents.
package base64

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the Base64 tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("base64").
		WithDescription("Base64 encoding and decoding tools").
		AddTools(
			encodeTool(),
			decodeTool(),
			encodeURLTool(),
			decodeURLTool(),
			encodeRawTool(),
			decodeRawTool(),
			validateTool(),
			detectTool(),
			chunkEncodeTool(),
			chunkDecodeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func encodeTool() tool.Tool {
	return tool.NewBuilder("base64_encode").
		WithDescription("Encode text to standard Base64").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			encoded := base64.StdEncoding.EncodeToString([]byte(params.Text))

			result := map[string]any{
				"encoded": encoded,
				"length":  len(encoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func decodeTool() tool.Tool {
	return tool.NewBuilder("base64_decode").
		WithDescription("Decode standard Base64 to text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Encoded string `json:"encoded"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			decoded, err := base64.StdEncoding.DecodeString(params.Encoded)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"decoded": string(decoded),
				"valid":   true,
				"length":  len(decoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func encodeURLTool() tool.Tool {
	return tool.NewBuilder("base64_encode_url").
		WithDescription("Encode text to URL-safe Base64").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			encoded := base64.URLEncoding.EncodeToString([]byte(params.Text))

			result := map[string]any{
				"encoded": encoded,
				"length":  len(encoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func decodeURLTool() tool.Tool {
	return tool.NewBuilder("base64_decode_url").
		WithDescription("Decode URL-safe Base64 to text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Encoded string `json:"encoded"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			decoded, err := base64.URLEncoding.DecodeString(params.Encoded)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"decoded": string(decoded),
				"valid":   true,
				"length":  len(decoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func encodeRawTool() tool.Tool {
	return tool.NewBuilder("base64_encode_raw").
		WithDescription("Encode text to raw Base64 (no padding)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text    string `json:"text"`
				URLSafe bool   `json:"url_safe,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var encoding *base64.Encoding
			if params.URLSafe {
				encoding = base64.RawURLEncoding
			} else {
				encoding = base64.RawStdEncoding
			}

			encoded := encoding.EncodeToString([]byte(params.Text))

			result := map[string]any{
				"encoded": encoded,
				"length":  len(encoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func decodeRawTool() tool.Tool {
	return tool.NewBuilder("base64_decode_raw").
		WithDescription("Decode raw Base64 (no padding) to text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Encoded string `json:"encoded"`
				URLSafe bool   `json:"url_safe,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var encoding *base64.Encoding
			if params.URLSafe {
				encoding = base64.RawURLEncoding
			} else {
				encoding = base64.RawStdEncoding
			}

			decoded, err := encoding.DecodeString(params.Encoded)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"decoded": string(decoded),
				"valid":   true,
				"length":  len(decoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("base64_validate").
		WithDescription("Validate a Base64 encoded string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Encoded string `json:"encoded"`
				URLSafe bool   `json:"url_safe,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var encoding *base64.Encoding
			if params.URLSafe {
				encoding = base64.URLEncoding
			} else {
				encoding = base64.StdEncoding
			}

			_, err := encoding.DecodeString(params.Encoded)
			valid := err == nil

			result := map[string]any{
				"valid":  valid,
				"length": len(params.Encoded),
			}
			if err != nil {
				result["error"] = err.Error()
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func detectTool() tool.Tool {
	return tool.NewBuilder("base64_detect").
		WithDescription("Detect Base64 encoding type").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Encoded string `json:"encoded"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			encoded := params.Encoded

			// Check for URL-safe characters
			hasURLChars := strings.ContainsAny(encoded, "-_")
			hasStdChars := strings.ContainsAny(encoded, "+/")
			hasPadding := strings.HasSuffix(encoded, "=")

			var encodingType string
			var valid bool

			// Try to detect and validate
			if hasURLChars && !hasStdChars {
				if hasPadding {
					_, err := base64.URLEncoding.DecodeString(encoded)
					valid = err == nil
					encodingType = "url"
				} else {
					_, err := base64.RawURLEncoding.DecodeString(encoded)
					valid = err == nil
					encodingType = "raw_url"
				}
			} else if hasStdChars || (!hasURLChars && !hasStdChars) {
				if hasPadding {
					_, err := base64.StdEncoding.DecodeString(encoded)
					valid = err == nil
					encodingType = "standard"
				} else {
					_, err := base64.RawStdEncoding.DecodeString(encoded)
					valid = err == nil
					encodingType = "raw_standard"
				}
			}

			result := map[string]any{
				"valid":       valid,
				"type":        encodingType,
				"has_padding": hasPadding,
				"url_safe":    hasURLChars && !hasStdChars,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func chunkEncodeTool() tool.Tool {
	return tool.NewBuilder("base64_chunk_encode").
		WithDescription("Encode text to Base64 with line breaks").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				LineWidth int    `json:"line_width,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			lineWidth := params.LineWidth
			if lineWidth <= 0 {
				lineWidth = 76 // MIME standard
			}

			encoded := base64.StdEncoding.EncodeToString([]byte(params.Text))

			// Split into lines
			var lines []string
			for i := 0; i < len(encoded); i += lineWidth {
				end := i + lineWidth
				if end > len(encoded) {
					end = len(encoded)
				}
				lines = append(lines, encoded[i:end])
			}

			chunked := strings.Join(lines, "\n")

			result := map[string]any{
				"encoded":    chunked,
				"lines":      len(lines),
				"line_width": lineWidth,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func chunkDecodeTool() tool.Tool {
	return tool.NewBuilder("base64_chunk_decode").
		WithDescription("Decode Base64 with line breaks removed").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Encoded string `json:"encoded"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Remove all whitespace
			cleaned := strings.Map(func(r rune) rune {
				if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
					return -1
				}
				return r
			}, params.Encoded)

			decoded, err := base64.StdEncoding.DecodeString(cleaned)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"decoded": string(decoded),
				"valid":   true,
				"length":  len(decoded),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
