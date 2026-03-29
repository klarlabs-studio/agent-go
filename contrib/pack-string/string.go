// Package stringutil provides string manipulation tools for agents.
package stringutil

import (
	"context"
	"crypto/md5"  // #nosec G501 -- MD5 used for checksums/fingerprinting, not cryptographic security
	"crypto/sha1" // #nosec G505 -- SHA1 used for checksums/fingerprinting, not cryptographic security
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the string utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("string").
		WithDescription("String manipulation utilities").
		AddTools(
			trimTool(),
			padTool(),
			repeatTool(),
			reverseTool(),
			truncateTool(),
			wordsTool(),
			linesTool(),
			containsTool(),
			replaceTool(),
			splitJoinTool(),
			hashTool(),
			countTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func trimTool() tool.Tool {
	return tool.NewBuilder("string_trim").
		WithDescription("Trim whitespace or characters from string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text   string `json:"text"`
				Chars  string `json:"chars,omitempty"`
				Side   string `json:"side,omitempty"` // left, right, both (default)
				Cutset string `json:"cutset,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			text := params.Text
			cutset := params.Cutset
			if cutset == "" {
				cutset = params.Chars
			}

			var trimmed string
			switch params.Side {
			case "left":
				if cutset != "" {
					trimmed = strings.TrimLeft(text, cutset)
				} else {
					trimmed = strings.TrimLeftFunc(text, unicode.IsSpace)
				}
			case "right":
				if cutset != "" {
					trimmed = strings.TrimRight(text, cutset)
				} else {
					trimmed = strings.TrimRightFunc(text, unicode.IsSpace)
				}
			default:
				if cutset != "" {
					trimmed = strings.Trim(text, cutset)
				} else {
					trimmed = strings.TrimSpace(text)
				}
			}

			result := map[string]any{
				"trimmed":  trimmed,
				"original": text,
				"removed":  len(text) - len(trimmed),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func padTool() tool.Tool {
	return tool.NewBuilder("string_pad").
		WithDescription("Pad string to specified length").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text   string `json:"text"`
				Length int    `json:"length"`
				Char   string `json:"char,omitempty"`
				Side   string `json:"side,omitempty"` // left, right (default), center
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			padChar := " "
			if params.Char != "" {
				padChar = params.Char
			}

			text := params.Text
			currentLen := utf8.RuneCountInString(text)

			if currentLen >= params.Length {
				result := map[string]any{
					"padded":   text,
					"original": text,
					"added":    0,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			needed := params.Length - currentLen
			var padded string

			switch params.Side {
			case "left":
				padded = strings.Repeat(padChar, needed) + text
			case "center":
				left := needed / 2
				right := needed - left
				padded = strings.Repeat(padChar, left) + text + strings.Repeat(padChar, right)
			default: // right
				padded = text + strings.Repeat(padChar, needed)
			}

			result := map[string]any{
				"padded":   padded,
				"original": text,
				"added":    needed,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func repeatTool() tool.Tool {
	return tool.NewBuilder("string_repeat").
		WithDescription("Repeat string n times").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				Count     int    `json:"count"`
				Separator string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Count <= 0 {
				result := map[string]any{
					"repeated": "",
					"length":   0,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			var repeated string
			if params.Separator != "" {
				parts := make([]string, params.Count)
				for i := range parts {
					parts[i] = params.Text
				}
				repeated = strings.Join(parts, params.Separator)
			} else {
				repeated = strings.Repeat(params.Text, params.Count)
			}

			result := map[string]any{
				"repeated": repeated,
				"length":   len(repeated),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func reverseTool() tool.Tool {
	return tool.NewBuilder("string_reverse").
		WithDescription("Reverse string").
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

			runes := []rune(params.Text)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}

			result := map[string]any{
				"reversed": string(runes),
				"original": params.Text,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func truncateTool() tool.Tool {
	return tool.NewBuilder("string_truncate").
		WithDescription("Truncate string to specified length").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text     string `json:"text"`
				Length   int    `json:"length"`
				Ellipsis string `json:"ellipsis,omitempty"`
				Word     bool   `json:"word_boundary,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ellipsis := params.Ellipsis
			if ellipsis == "" {
				ellipsis = "..."
			}

			runes := []rune(params.Text)
			if len(runes) <= params.Length {
				result := map[string]any{
					"truncated":     params.Text,
					"was_truncated": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			maxLen := params.Length - utf8.RuneCountInString(ellipsis)
			if maxLen < 0 {
				maxLen = 0
			}

			truncated := string(runes[:maxLen])

			// If word boundary, find last space
			if params.Word && maxLen > 0 {
				if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 0 {
					truncated = truncated[:lastSpace]
				}
			}

			truncated = strings.TrimSpace(truncated) + ellipsis

			result := map[string]any{
				"truncated":     truncated,
				"was_truncated": true,
				"original_len":  len(runes),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func wordsTool() tool.Tool {
	return tool.NewBuilder("string_words").
		WithDescription("Extract words from string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text   string `json:"text"`
				Unique bool   `json:"unique,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Split on non-letter characters
			wordRegex := regexp.MustCompile(`\p{L}+`)
			matches := wordRegex.FindAllString(params.Text, -1)

			words := matches
			if words == nil {
				words = []string{}
			}

			if params.Unique {
				seen := make(map[string]bool)
				unique := make([]string, 0)
				for _, w := range words {
					lower := strings.ToLower(w)
					if !seen[lower] {
						seen[lower] = true
						unique = append(unique, w)
					}
				}
				words = unique
			}

			result := map[string]any{
				"words": words,
				"count": len(words),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func linesTool() tool.Tool {
	return tool.NewBuilder("string_lines").
		WithDescription("Split string into lines").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text       string `json:"text"`
				TrimEmpty  bool   `json:"trim_empty,omitempty"`
				TrimSpaces bool   `json:"trim_spaces,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Normalize line endings
			text := strings.ReplaceAll(params.Text, "\r\n", "\n")
			text = strings.ReplaceAll(text, "\r", "\n")

			lines := strings.Split(text, "\n")

			if params.TrimSpaces {
				for i, line := range lines {
					lines[i] = strings.TrimSpace(line)
				}
			}

			if params.TrimEmpty {
				filtered := make([]string, 0, len(lines))
				for _, line := range lines {
					if line != "" {
						filtered = append(filtered, line)
					}
				}
				lines = filtered
			}

			result := map[string]any{
				"lines": lines,
				"count": len(lines),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func containsTool() tool.Tool {
	return tool.NewBuilder("string_contains").
		WithDescription("Check if string contains substring").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text       string `json:"text"`
				Substring  string `json:"substring"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			text := params.Text
			substring := params.Substring

			if params.IgnoreCase {
				text = strings.ToLower(text)
				substring = strings.ToLower(substring)
			}

			contains := strings.Contains(text, substring)
			count := strings.Count(text, substring)
			index := strings.Index(text, substring)

			result := map[string]any{
				"contains":    contains,
				"count":       count,
				"first_index": index,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func replaceTool() tool.Tool {
	return tool.NewBuilder("string_replace").
		WithDescription("Replace occurrences in string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text    string `json:"text"`
				Old     string `json:"old"`
				New     string `json:"new"`
				Count   int    `json:"count,omitempty"` // -1 for all
				IsRegex bool   `json:"regex,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			count := params.Count
			if count == 0 {
				count = -1 // Replace all by default
			}

			var replaced string
			var replacements int

			if params.IsRegex {
				re, err := regexp.Compile(params.Old)
				if err != nil {
					result := map[string]any{
						"error": err.Error(),
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}

				matches := re.FindAllStringIndex(params.Text, -1)
				replacements = len(matches)
				if count > 0 && replacements > count {
					replacements = count
				}

				if count > 0 {
					// Limited replacement
					result := params.Text
					for i := 0; i < count; i++ {
						loc := re.FindStringIndex(result)
						if loc == nil {
							break
						}
						result = result[:loc[0]] + params.New + result[loc[1]:]
					}
					replaced = result
				} else {
					replaced = re.ReplaceAllString(params.Text, params.New)
				}
			} else {
				replacements = strings.Count(params.Text, params.Old)
				if count > 0 && replacements > count {
					replacements = count
				}
				replaced = strings.Replace(params.Text, params.Old, params.New, count)
			}

			result := map[string]any{
				"replaced":     replaced,
				"original":     params.Text,
				"replacements": replacements,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func splitJoinTool() tool.Tool {
	return tool.NewBuilder("string_split_join").
		WithDescription("Split and optionally rejoin string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text        string `json:"text"`
				Separator   string `json:"separator"`
				JoinWith    string `json:"join_with,omitempty"`
				Limit       int    `json:"limit,omitempty"`
				TrimParts   bool   `json:"trim_parts,omitempty"`
				RemoveEmpty bool   `json:"remove_empty,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var parts []string
			if params.Limit > 0 {
				parts = strings.SplitN(params.Text, params.Separator, params.Limit)
			} else {
				parts = strings.Split(params.Text, params.Separator)
			}

			if params.TrimParts {
				for i, p := range parts {
					parts[i] = strings.TrimSpace(p)
				}
			}

			if params.RemoveEmpty {
				filtered := make([]string, 0, len(parts))
				for _, p := range parts {
					if p != "" {
						filtered = append(filtered, p)
					}
				}
				parts = filtered
			}

			result := map[string]any{
				"parts": parts,
				"count": len(parts),
			}

			if params.JoinWith != "" {
				result["joined"] = strings.Join(parts, params.JoinWith)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hashTool() tool.Tool {
	return tool.NewBuilder("string_hash").
		WithDescription("Generate hash of string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				Algorithm string `json:"algorithm,omitempty"` // md5, sha1, sha256 (default)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			data := []byte(params.Text)
			var hashHex string
			algorithm := params.Algorithm

			switch strings.ToLower(algorithm) {
			case "md5":
				hash := md5.Sum(data) // #nosec G401 -- MD5 used for checksums/fingerprinting, not cryptographic security
				hashHex = hex.EncodeToString(hash[:])
				algorithm = "md5"
			case "sha1":
				hash := sha1.Sum(data) // #nosec G401 -- SHA1 used for checksums/fingerprinting, not cryptographic security
				hashHex = hex.EncodeToString(hash[:])
				algorithm = "sha1"
			default:
				hash := sha256.Sum256(data)
				hashHex = hex.EncodeToString(hash[:])
				algorithm = "sha256"
			}

			result := map[string]any{
				"hash":      hashHex,
				"algorithm": algorithm,
				"length":    len(hashHex),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countTool() tool.Tool {
	return tool.NewBuilder("string_count").
		WithDescription("Count characters, words, lines in string").
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

			// Count characters (runes)
			chars := utf8.RuneCountInString(text)

			// Count bytes
			bytes := len(text)

			// Count words
			wordRegex := regexp.MustCompile(`\S+`)
			words := len(wordRegex.FindAllString(text, -1))

			// Count lines
			lines := 1
			if text == "" {
				lines = 0
			} else {
				lines = strings.Count(text, "\n") + 1
			}

			// Count sentences (rough estimate)
			sentenceRegex := regexp.MustCompile(`[.!?]+\s*`)
			sentences := len(sentenceRegex.FindAllString(text, -1))
			if sentences == 0 && text != "" {
				sentences = 1
			}

			result := map[string]any{
				"characters": chars,
				"bytes":      bytes,
				"words":      words,
				"lines":      lines,
				"sentences":  sentences,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
