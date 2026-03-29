// Package regex provides regular expression tools for pattern matching and text manipulation.
package regex

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

type regexPack struct{}

// Pack creates a new regex tools pack.
func Pack() *pack.Pack {
	p := &regexPack{}

	return pack.NewBuilder("regex").
		WithDescription("Regular expression tools for pattern matching and text manipulation").
		WithVersion("1.0.0").
		AddTools(
			p.matchTool(),
			p.matchAllTool(),
			p.testTool(),
			p.replaceTool(),
			p.replaceAllTool(),
			p.splitTool(),
			p.findTool(),
			p.findAllTool(),
			p.extractGroupsTool(),
			p.extractAllGroupsTool(),
			p.validateTool(),
			p.escapeTool(),
			p.countMatchesTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// matchTool checks if pattern matches text.
func (p *regexPack) matchTool() tool.Tool {
	return tool.NewBuilder("regex_match").
		WithDescription("Check if a regular expression matches the text").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string `json:"pattern"`
				Text       string `json:"text"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
				Multiline  bool   `json:"multiline,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}
			if params.Multiline {
				pattern = "(?m)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			match := re.FindString(params.Text)
			matches := match != ""

			result := map[string]interface{}{
				"matches": matches,
				"match":   match,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// matchAllTool finds all matches.
func (p *regexPack) matchAllTool() tool.Tool {
	return tool.NewBuilder("regex_match_all").
		WithDescription("Find all matches of a pattern in text").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string `json:"pattern"`
				Text       string `json:"text"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
				Limit      int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			limit := params.Limit
			if limit <= 0 {
				limit = -1
			}

			matches := re.FindAllString(params.Text, limit)

			result := map[string]interface{}{
				"matches": matches,
				"count":   len(matches),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// testTool tests if pattern matches anywhere in text.
func (p *regexPack) testTool() tool.Tool {
	return tool.NewBuilder("regex_test").
		WithDescription("Test if pattern matches anywhere in text (returns boolean)").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string `json:"pattern"`
				Text       string `json:"text"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			matches := re.MatchString(params.Text)

			result := map[string]interface{}{
				"matches": matches,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// replaceTool replaces first match.
func (p *regexPack) replaceTool() tool.Tool {
	return tool.NewBuilder("regex_replace").
		WithDescription("Replace first match of pattern").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern     string `json:"pattern"`
				Text        string `json:"text"`
				Replacement string `json:"replacement"`
				IgnoreCase  bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			// Find first match location
			loc := re.FindStringIndex(params.Text)
			if loc == nil {
				result := map[string]interface{}{
					"output":   params.Text,
					"replaced": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Replace first match
			match := params.Text[loc[0]:loc[1]]
			replacement := re.ReplaceAllString(match, params.Replacement)
			resultText := params.Text[:loc[0]] + replacement + params.Text[loc[1]:]

			result := map[string]interface{}{
				"output":   resultText,
				"replaced": true,
				"match":    match,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// replaceAllTool replaces all matches.
func (p *regexPack) replaceAllTool() tool.Tool {
	return tool.NewBuilder("regex_replace_all").
		WithDescription("Replace all matches of pattern").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern     string `json:"pattern"`
				Text        string `json:"text"`
				Replacement string `json:"replacement"`
				IgnoreCase  bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			// Count matches before replacement
			matches := re.FindAllString(params.Text, -1)

			resultText := re.ReplaceAllString(params.Text, params.Replacement)

			result := map[string]interface{}{
				"output":       resultText,
				"replacements": len(matches),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// splitTool splits text by pattern.
func (p *regexPack) splitTool() tool.Tool {
	return tool.NewBuilder("regex_split").
		WithDescription("Split text by regular expression pattern").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string `json:"pattern"`
				Text       string `json:"text"`
				Limit      int    `json:"limit,omitempty"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			limit := params.Limit
			if limit <= 0 {
				limit = -1
			}

			parts := re.Split(params.Text, limit)

			result := map[string]interface{}{
				"parts": parts,
				"count": len(parts),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// findTool finds first match with position.
func (p *regexPack) findTool() tool.Tool {
	return tool.NewBuilder("regex_find").
		WithDescription("Find first match with position information").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string `json:"pattern"`
				Text       string `json:"text"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			loc := re.FindStringIndex(params.Text)
			if loc == nil {
				result := map[string]interface{}{
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]interface{}{
				"found": true,
				"match": params.Text[loc[0]:loc[1]],
				"start": loc[0],
				"end":   loc[1],
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// findAllTool finds all matches with positions.
func (p *regexPack) findAllTool() tool.Tool {
	return tool.NewBuilder("regex_find_all").
		WithDescription("Find all matches with position information").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string `json:"pattern"`
				Text       string `json:"text"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
				Limit      int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			limit := params.Limit
			if limit <= 0 {
				limit = -1
			}

			locs := re.FindAllStringIndex(params.Text, limit)

			var matches []map[string]interface{}
			for _, loc := range locs {
				matches = append(matches, map[string]interface{}{
					"match": params.Text[loc[0]:loc[1]],
					"start": loc[0],
					"end":   loc[1],
				})
			}

			result := map[string]interface{}{
				"matches": matches,
				"count":   len(matches),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// extractGroupsTool extracts capture groups from first match.
func (p *regexPack) extractGroupsTool() tool.Tool {
	return tool.NewBuilder("regex_extract_groups").
		WithDescription("Extract capture groups from first match").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string `json:"pattern"`
				Text       string `json:"text"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			match := re.FindStringSubmatch(params.Text)
			if match == nil {
				result := map[string]interface{}{
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			names := re.SubexpNames()
			groups := make(map[string]string)
			indexedGroups := make([]string, 0)

			for i, m := range match {
				if i == 0 {
					continue // Skip full match
				}
				indexedGroups = append(indexedGroups, m)
				if i < len(names) && names[i] != "" {
					groups[names[i]] = m
				}
			}

			result := map[string]interface{}{
				"found":        true,
				"full_match":   match[0],
				"groups":       indexedGroups,
				"named_groups": groups,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// extractAllGroupsTool extracts capture groups from all matches.
func (p *regexPack) extractAllGroupsTool() tool.Tool {
	return tool.NewBuilder("regex_extract_all_groups").
		WithDescription("Extract capture groups from all matches").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string `json:"pattern"`
				Text       string `json:"text"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
				Limit      int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			limit := params.Limit
			if limit <= 0 {
				limit = -1
			}

			allMatches := re.FindAllStringSubmatch(params.Text, limit)
			names := re.SubexpNames()

			var results []map[string]interface{}
			for _, match := range allMatches {
				groups := make(map[string]string)
				indexedGroups := make([]string, 0)

				for i, m := range match {
					if i == 0 {
						continue
					}
					indexedGroups = append(indexedGroups, m)
					if i < len(names) && names[i] != "" {
						groups[names[i]] = m
					}
				}

				results = append(results, map[string]interface{}{
					"full_match":   match[0],
					"groups":       indexedGroups,
					"named_groups": groups,
				})
			}

			result := map[string]interface{}{
				"matches": results,
				"count":   len(results),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// validateTool validates regex syntax.
func (p *regexPack) validateTool() tool.Tool {
	return tool.NewBuilder("regex_validate").
		WithDescription("Validate regular expression syntax").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern string `json:"pattern"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			_, err := regexp.Compile(params.Pattern)

			if err != nil {
				result := map[string]interface{}{
					"valid": false,
					"error": err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]interface{}{
				"valid": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// escapeTool escapes special regex characters.
func (p *regexPack) escapeTool() tool.Tool {
	return tool.NewBuilder("regex_escape").
		WithDescription("Escape special regex characters in a string").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			escaped := regexp.QuoteMeta(params.Text)

			result := map[string]interface{}{
				"escaped": escaped,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// countMatchesTool counts pattern matches.
func (p *regexPack) countMatchesTool() tool.Tool {
	return tool.NewBuilder("regex_count_matches").
		WithDescription("Count the number of pattern matches in text").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern     string `json:"pattern"`
				Text        string `json:"text"`
				IgnoreCase  bool   `json:"ignore_case,omitempty"`
				Overlapping bool   `json:"overlapping,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern is required")
			}

			pattern := params.Pattern
			if params.IgnoreCase {
				pattern = "(?i)" + pattern
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid regex: %w", err)
			}

			var count int
			if params.Overlapping {
				// For overlapping matches, we need to search from each position
				text := params.Text
				for i := 0; i < len(text); i++ {
					if re.MatchString(text[i:]) {
						loc := re.FindStringIndex(text[i:])
						if loc != nil && loc[0] == 0 {
							count++
						}
					}
				}
			} else {
				matches := re.FindAllString(params.Text, -1)
				count = len(matches)
			}

			result := map[string]interface{}{
				"count": count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Common regex patterns as helper
var CommonPatterns = map[string]string{
	"email":       `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
	"url":         `https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`,
	"ipv4":        `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
	"ipv6":        `([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}`,
	"phone":       `\+?[\d\s\-\(\)]{10,}`,
	"date_iso":    `\d{4}-\d{2}-\d{2}`,
	"time_24h":    `([01]?\d|2[0-3]):[0-5]\d(:[0-5]\d)?`,
	"hex_color":   `#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})\b`,
	"uuid":        `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`,
	"credit_card": `\b(?:\d{4}[- ]?){3}\d{4}\b`,
	"ssn":         `\b\d{3}-\d{2}-\d{4}\b`,
	"zip_us":      `\b\d{5}(-\d{4})?\b`,
	"word":        `\b\w+\b`,
	"integer":     `[+-]?\d+`,
	"decimal":     `[+-]?\d+\.?\d*`,
	"whitespace":  `\s+`,
	"newline":     `\r?\n`,
}

// Ensure strings import is used
var _ = strings.TrimSpace
