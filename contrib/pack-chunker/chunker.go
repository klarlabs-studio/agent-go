// Package chunker provides text chunking tools for RAG pipelines.
package chunker

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the chunker tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("chunker").
		WithDescription("Text chunking tools for RAG pipelines").
		AddTools(
			fixedSizeTool(),
			sentenceTool(),
			paragraphTool(),
			semanticTool(),
			slidingWindowTool(),
			recursiveTool(),
			markdownTool(),
			codeTool(),
			overlapTool(),
			mergeTool(),
			splitByTokensTool(),
			estimateTokensTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func fixedSizeTool() tool.Tool {
	return tool.NewBuilder("chunk_fixed_size").
		WithDescription("Split text into fixed-size chunks").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				ChunkSize int    `json:"chunk_size,omitempty"`
				Overlap   int    `json:"overlap,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			chunkSize := params.ChunkSize
			if chunkSize <= 0 {
				chunkSize = 1000
			}

			overlap := params.Overlap
			if overlap < 0 {
				overlap = 0
			}
			if overlap >= chunkSize {
				overlap = chunkSize / 2
			}

			var chunks []map[string]any
			text := params.Text
			step := chunkSize - overlap

			for i := 0; i < len(text); i += step {
				end := i + chunkSize
				if end > len(text) {
					end = len(text)
				}

				chunks = append(chunks, map[string]any{
					"text":  text[i:end],
					"start": i,
					"end":   end,
				})

				if end >= len(text) {
					break
				}
			}

			result := map[string]any{
				"chunks":     chunks,
				"count":      len(chunks),
				"chunk_size": chunkSize,
				"overlap":    overlap,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sentenceTool() tool.Tool {
	return tool.NewBuilder("chunk_by_sentence").
		WithDescription("Split text into sentence-based chunks").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text              string `json:"text"`
				SentencesPerChunk int    `json:"sentences_per_chunk,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sentencesPerChunk := params.SentencesPerChunk
			if sentencesPerChunk <= 0 {
				sentencesPerChunk = 3
			}

			// Simple sentence splitting
			sentenceEnders := regexp.MustCompile(`[.!?]+\s+`)
			parts := sentenceEnders.Split(params.Text, -1)

			var chunks []map[string]any
			var currentChunk []string

			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				currentChunk = append(currentChunk, part)

				if len(currentChunk) >= sentencesPerChunk {
					chunks = append(chunks, map[string]any{
						"text":      strings.Join(currentChunk, ". ") + ".",
						"sentences": len(currentChunk),
					})
					currentChunk = nil
				}
			}

			if len(currentChunk) > 0 {
				chunks = append(chunks, map[string]any{
					"text":      strings.Join(currentChunk, ". ") + ".",
					"sentences": len(currentChunk),
				})
			}

			result := map[string]any{
				"chunks":              chunks,
				"count":               len(chunks),
				"sentences_per_chunk": sentencesPerChunk,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func paragraphTool() tool.Tool {
	return tool.NewBuilder("chunk_by_paragraph").
		WithDescription("Split text into paragraph-based chunks").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text               string `json:"text"`
				ParagraphsPerChunk int    `json:"paragraphs_per_chunk,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			paragraphsPerChunk := params.ParagraphsPerChunk
			if paragraphsPerChunk <= 0 {
				paragraphsPerChunk = 1
			}

			// Split by double newlines
			paragraphSplitter := regexp.MustCompile(`\n\s*\n`)
			parts := paragraphSplitter.Split(params.Text, -1)

			var chunks []map[string]any
			var currentChunk []string

			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				currentChunk = append(currentChunk, part)

				if len(currentChunk) >= paragraphsPerChunk {
					chunks = append(chunks, map[string]any{
						"text":       strings.Join(currentChunk, "\n\n"),
						"paragraphs": len(currentChunk),
					})
					currentChunk = nil
				}
			}

			if len(currentChunk) > 0 {
				chunks = append(chunks, map[string]any{
					"text":       strings.Join(currentChunk, "\n\n"),
					"paragraphs": len(currentChunk),
				})
			}

			result := map[string]any{
				"chunks":               chunks,
				"count":                len(chunks),
				"paragraphs_per_chunk": paragraphsPerChunk,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func semanticTool() tool.Tool {
	return tool.NewBuilder("chunk_semantic").
		WithDescription("Split text at semantic boundaries (headings, sections)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text         string `json:"text"`
				MaxChunkSize int    `json:"max_chunk_size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			maxSize := params.MaxChunkSize
			if maxSize <= 0 {
				maxSize = 2000
			}

			// Split by common section markers
			sectionMarkers := regexp.MustCompile(`(?m)^(#{1,6}\s|[A-Z][A-Za-z\s]+:\s*$|\d+\.\s+[A-Z])`)
			indices := sectionMarkers.FindAllStringIndex(params.Text, -1)

			var chunks []map[string]any
			lastEnd := 0

			for _, idx := range indices {
				if idx[0] > lastEnd {
					chunk := strings.TrimSpace(params.Text[lastEnd:idx[0]])
					if chunk != "" {
						// Further split if too large
						for len(chunk) > maxSize {
							chunks = append(chunks, map[string]any{
								"text":         chunk[:maxSize],
								"is_truncated": true,
							})
							chunk = chunk[maxSize:]
						}
						if chunk != "" {
							chunks = append(chunks, map[string]any{
								"text":         chunk,
								"is_truncated": false,
							})
						}
					}
				}
				lastEnd = idx[0]
			}

			if lastEnd < len(params.Text) {
				chunk := strings.TrimSpace(params.Text[lastEnd:])
				if chunk != "" {
					for len(chunk) > maxSize {
						chunks = append(chunks, map[string]any{
							"text":         chunk[:maxSize],
							"is_truncated": true,
						})
						chunk = chunk[maxSize:]
					}
					if chunk != "" {
						chunks = append(chunks, map[string]any{
							"text":         chunk,
							"is_truncated": false,
						})
					}
				}
			}

			result := map[string]any{
				"chunks":         chunks,
				"count":          len(chunks),
				"max_chunk_size": maxSize,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func slidingWindowTool() tool.Tool {
	return tool.NewBuilder("chunk_sliding_window").
		WithDescription("Create overlapping chunks using a sliding window").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text       string `json:"text"`
				WindowSize int    `json:"window_size,omitempty"`
				StepSize   int    `json:"step_size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			windowSize := params.WindowSize
			if windowSize <= 0 {
				windowSize = 500
			}

			stepSize := params.StepSize
			if stepSize <= 0 {
				stepSize = windowSize / 2
			}

			var chunks []map[string]any
			text := params.Text

			for i := 0; i < len(text); i += stepSize {
				end := i + windowSize
				if end > len(text) {
					end = len(text)
				}

				chunks = append(chunks, map[string]any{
					"text":    text[i:end],
					"start":   i,
					"end":     end,
					"overlap": min(windowSize-stepSize, i),
				})

				if end >= len(text) {
					break
				}
			}

			result := map[string]any{
				"chunks":      chunks,
				"count":       len(chunks),
				"window_size": windowSize,
				"step_size":   stepSize,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func recursiveTool() tool.Tool {
	return tool.NewBuilder("chunk_recursive").
		WithDescription("Recursively split text using multiple separators").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text       string   `json:"text"`
				ChunkSize  int      `json:"chunk_size,omitempty"`
				Separators []string `json:"separators,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			chunkSize := params.ChunkSize
			if chunkSize <= 0 {
				chunkSize = 1000
			}

			separators := params.Separators
			if len(separators) == 0 {
				separators = []string{"\n\n", "\n", ". ", " ", ""}
			}

			chunks := recursiveSplit(params.Text, separators, chunkSize)

			var result []map[string]any
			for _, chunk := range chunks {
				result = append(result, map[string]any{
					"text": chunk,
					"size": len(chunk),
				})
			}

			output, _ := json.Marshal(map[string]any{
				"chunks":     result,
				"count":      len(result),
				"chunk_size": chunkSize,
				"separators": separators,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func recursiveSplit(text string, separators []string, chunkSize int) []string {
	if len(text) <= chunkSize {
		return []string{strings.TrimSpace(text)}
	}

	if len(separators) == 0 {
		// Last resort: hard split
		var chunks []string
		for i := 0; i < len(text); i += chunkSize {
			end := i + chunkSize
			if end > len(text) {
				end = len(text)
			}
			chunks = append(chunks, text[i:end])
		}
		return chunks
	}

	sep := separators[0]
	var chunks []string

	if sep == "" {
		// Character-level split
		for i := 0; i < len(text); i += chunkSize {
			end := i + chunkSize
			if end > len(text) {
				end = len(text)
			}
			chunks = append(chunks, text[i:end])
		}
		return chunks
	}

	parts := strings.Split(text, sep)
	var current strings.Builder

	for _, part := range parts {
		if current.Len()+len(part)+len(sep) <= chunkSize {
			if current.Len() > 0 {
				current.WriteString(sep)
			}
			current.WriteString(part)
		} else {
			if current.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(current.String()))
				current.Reset()
			}
			if len(part) <= chunkSize {
				current.WriteString(part)
			} else {
				// Recursively split with next separator
				subChunks := recursiveSplit(part, separators[1:], chunkSize)
				chunks = append(chunks, subChunks...)
			}
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	return chunks
}

func markdownTool() tool.Tool {
	return tool.NewBuilder("chunk_markdown").
		WithDescription("Split markdown by headings and structure").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text         string `json:"text"`
				MaxChunkSize int    `json:"max_chunk_size,omitempty"`
				ByHeading    int    `json:"by_heading,omitempty"` // Split at heading level (1-6)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			maxSize := params.MaxChunkSize
			if maxSize <= 0 {
				maxSize = 2000
			}

			headingLevel := params.ByHeading
			if headingLevel <= 0 || headingLevel > 6 {
				headingLevel = 2
			}

			// Build heading regex
			pattern := regexp.MustCompile(`(?m)^#{1,` + string(rune('0'+headingLevel)) + `}\s+.+$`)
			indices := pattern.FindAllStringIndex(params.Text, -1)

			var chunks []map[string]any
			lastEnd := 0

			for _, idx := range indices {
				if idx[0] > lastEnd {
					chunk := strings.TrimSpace(params.Text[lastEnd:idx[0]])
					if chunk != "" {
						chunks = append(chunks, map[string]any{
							"text":    truncateChunk(chunk, maxSize),
							"heading": "",
						})
					}
				}

				// Find the heading text
				lineEnd := strings.Index(params.Text[idx[0]:], "\n")
				if lineEnd == -1 {
					lineEnd = len(params.Text) - idx[0]
				}
				headingText := strings.TrimSpace(params.Text[idx[0] : idx[0]+lineEnd])

				lastEnd = idx[0]
				_ = headingText // Will be included in next chunk
			}

			if lastEnd < len(params.Text) {
				chunk := strings.TrimSpace(params.Text[lastEnd:])
				if chunk != "" {
					chunks = append(chunks, map[string]any{
						"text":    truncateChunk(chunk, maxSize),
						"heading": extractHeading(chunk),
					})
				}
			}

			result := map[string]any{
				"chunks":         chunks,
				"count":          len(chunks),
				"max_chunk_size": maxSize,
				"heading_level":  headingLevel,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func truncateChunk(text string, maxSize int) string {
	if len(text) <= maxSize {
		return text
	}
	return text[:maxSize]
}

func extractHeading(text string) string {
	lines := strings.SplitN(text, "\n", 2)
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#") {
		return strings.TrimLeft(lines[0], "# ")
	}
	return ""
}

func codeTool() tool.Tool {
	return tool.NewBuilder("chunk_code").
		WithDescription("Split code by functions/classes/blocks").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code         string `json:"code"`
				Language     string `json:"language,omitempty"`
				MaxChunkSize int    `json:"max_chunk_size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			maxSize := params.MaxChunkSize
			if maxSize <= 0 {
				maxSize = 2000
			}

			// Language-specific patterns
			var pattern *regexp.Regexp
			switch strings.ToLower(params.Language) {
			case "go", "golang":
				pattern = regexp.MustCompile(`(?m)^func\s+`)
			case "python", "py":
				pattern = regexp.MustCompile(`(?m)^(def|class)\s+`)
			case "javascript", "js", "typescript", "ts":
				pattern = regexp.MustCompile(`(?m)^(function|class|const\s+\w+\s*=\s*(?:async\s+)?(?:function|\())\s*`)
			case "java", "kotlin":
				pattern = regexp.MustCompile(`(?m)^(\s*public|\s*private|\s*protected)?\s*(class|interface|enum|void|static)\s+`)
			default:
				// Generic: split by blank lines
				pattern = regexp.MustCompile(`\n\s*\n`)
			}

			indices := pattern.FindAllStringIndex(params.Code, -1)

			var chunks []map[string]any
			lastEnd := 0

			for _, idx := range indices {
				if idx[0] > lastEnd {
					chunk := strings.TrimSpace(params.Code[lastEnd:idx[0]])
					if chunk != "" && len(chunk) > 10 {
						chunks = append(chunks, map[string]any{
							"text": truncateChunk(chunk, maxSize),
							"type": "block",
						})
					}
				}
				lastEnd = idx[0]
			}

			if lastEnd < len(params.Code) {
				chunk := strings.TrimSpace(params.Code[lastEnd:])
				if chunk != "" {
					chunks = append(chunks, map[string]any{
						"text": truncateChunk(chunk, maxSize),
						"type": "block",
					})
				}
			}

			result := map[string]any{
				"chunks":         chunks,
				"count":          len(chunks),
				"language":       params.Language,
				"max_chunk_size": maxSize,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func overlapTool() tool.Tool {
	return tool.NewBuilder("chunk_add_overlap").
		WithDescription("Add overlap between existing chunks").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Chunks      []string `json:"chunks"`
				OverlapSize int      `json:"overlap_size,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			overlapSize := params.OverlapSize
			if overlapSize <= 0 {
				overlapSize = 100
			}

			var result []map[string]any

			for i, chunk := range params.Chunks {
				var prefix, suffix string

				if i > 0 {
					prevChunk := params.Chunks[i-1]
					if len(prevChunk) > overlapSize {
						prefix = prevChunk[len(prevChunk)-overlapSize:]
					} else {
						prefix = prevChunk
					}
				}

				if i < len(params.Chunks)-1 {
					nextChunk := params.Chunks[i+1]
					if len(nextChunk) > overlapSize {
						suffix = nextChunk[:overlapSize]
					} else {
						suffix = nextChunk
					}
				}

				result = append(result, map[string]any{
					"text":         prefix + chunk + suffix,
					"original":     chunk,
					"prefix_added": len(prefix),
					"suffix_added": len(suffix),
				})
			}

			output, _ := json.Marshal(map[string]any{
				"chunks":       result,
				"count":        len(result),
				"overlap_size": overlapSize,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mergeTool() tool.Tool {
	return tool.NewBuilder("chunk_merge").
		WithDescription("Merge small chunks together").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Chunks       []string `json:"chunks"`
				MinChunkSize int      `json:"min_chunk_size,omitempty"`
				MaxChunkSize int      `json:"max_chunk_size,omitempty"`
				Separator    string   `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			minSize := params.MinChunkSize
			if minSize <= 0 {
				minSize = 200
			}

			maxSize := params.MaxChunkSize
			if maxSize <= 0 {
				maxSize = 2000
			}

			separator := params.Separator
			if separator == "" {
				separator = "\n\n"
			}

			var result []string
			var current strings.Builder

			for _, chunk := range params.Chunks {
				if current.Len()+len(chunk)+len(separator) <= maxSize {
					if current.Len() > 0 {
						current.WriteString(separator)
					}
					current.WriteString(chunk)
				} else {
					if current.Len() > 0 {
						result = append(result, current.String())
						current.Reset()
					}
					current.WriteString(chunk)
				}
			}

			if current.Len() > 0 {
				result = append(result, current.String())
			}

			output, _ := json.Marshal(map[string]any{
				"chunks":         result,
				"count":          len(result),
				"original_count": len(params.Chunks),
				"min_chunk_size": minSize,
				"max_chunk_size": maxSize,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func splitByTokensTool() tool.Tool {
	return tool.NewBuilder("chunk_by_tokens").
		WithDescription("Split text by estimated token count").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				MaxTokens int    `json:"max_tokens,omitempty"`
				Overlap   int    `json:"overlap_tokens,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			maxTokens := params.MaxTokens
			if maxTokens <= 0 {
				maxTokens = 500
			}

			overlap := params.Overlap
			if overlap < 0 {
				overlap = 0
			}

			// Estimate: ~4 characters per token for English
			charsPerToken := 4
			chunkSize := maxTokens * charsPerToken
			overlapChars := overlap * charsPerToken

			var chunks []map[string]any
			text := params.Text
			step := chunkSize - overlapChars

			for i := 0; i < len(text); i += step {
				end := i + chunkSize
				if end > len(text) {
					end = len(text)
				}

				chunk := text[i:end]
				estimatedTokens := len(chunk) / charsPerToken

				chunks = append(chunks, map[string]any{
					"text":             chunk,
					"estimated_tokens": estimatedTokens,
					"start":            i,
					"end":              end,
				})

				if end >= len(text) {
					break
				}
			}

			result := map[string]any{
				"chunks":         chunks,
				"count":          len(chunks),
				"max_tokens":     maxTokens,
				"overlap_tokens": overlap,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func estimateTokensTool() tool.Tool {
	return tool.NewBuilder("chunk_estimate_tokens").
		WithDescription("Estimate token count for text").
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

			text := params.Text

			// Multiple estimation methods
			charCount := len(text)
			wordCount := len(strings.Fields(text))

			// Count by character types
			var alphanumeric, whitespace, punctuation, other int
			for _, r := range text {
				switch {
				case unicode.IsLetter(r) || unicode.IsDigit(r):
					alphanumeric++
				case unicode.IsSpace(r):
					whitespace++
				case unicode.IsPunct(r):
					punctuation++
				default:
					other++
				}
			}

			// Different estimation methods
			byChars := charCount / 4                 // ~4 chars per token
			byWords := int(float64(wordCount) * 1.3) // ~1.3 tokens per word

			result := map[string]any{
				"estimated_tokens": (byChars + byWords) / 2, // Average of both methods
				"by_chars":         byChars,
				"by_words":         byWords,
				"char_count":       charCount,
				"word_count":       wordCount,
				"breakdown": map[string]int{
					"alphanumeric": alphanumeric,
					"whitespace":   whitespace,
					"punctuation":  punctuation,
					"other":        other,
				},
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
