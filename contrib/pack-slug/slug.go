// Package slug provides URL slug generation tools for agents.
package slug

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

var (
	multiDash   = regexp.MustCompile(`-+`)
	nonAlphaNum = regexp.MustCompile(`[^a-z0-9-]`)
)

// Pack returns the slug tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("slug").
		WithDescription("URL slug generation tools").
		AddTools(
			generateTool(),
			fromTitleTool(),
			validateTool(),
			truncateTool(),
			joinTool(),
			uniqueTool(),
			reverseTool(),
			extractWordsTool(),
			countWordsTool(),
			compareTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func generateTool() tool.Tool {
	return tool.NewBuilder("slug_generate").
		WithDescription("Generate a URL slug from text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				MaxLength int    `json:"max_length,omitempty"`
				Separator string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sep := params.Separator
			if sep == "" {
				sep = "-"
			}

			slug := slugify(params.Text, sep)

			if params.MaxLength > 0 && len(slug) > params.MaxLength {
				slug = truncateSlug(slug, params.MaxLength, sep)
			}

			result := map[string]any{
				"slug":   slug,
				"length": len(slug),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fromTitleTool() tool.Tool {
	return tool.NewBuilder("slug_from_title").
		WithDescription("Generate slug from article title").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Title     string `json:"title"`
				MaxLength int    `json:"max_length,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Remove common stop words for cleaner slugs
			stopWords := map[string]bool{
				"a": true, "an": true, "the": true, "and": true, "or": true,
				"but": true, "in": true, "on": true, "at": true, "to": true,
				"for": true, "of": true, "with": true, "by": true, "is": true,
				"are": true, "was": true, "were": true, "be": true, "been": true,
			}

			words := strings.Fields(strings.ToLower(params.Title))
			var filtered []string
			for _, word := range words {
				if !stopWords[word] {
					filtered = append(filtered, word)
				}
			}

			text := strings.Join(filtered, " ")
			slug := slugify(text, "-")

			if params.MaxLength > 0 && len(slug) > params.MaxLength {
				slug = truncateSlug(slug, params.MaxLength, "-")
			}

			result := map[string]any{
				"title":  params.Title,
				"slug":   slug,
				"length": len(slug),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("slug_validate").
		WithDescription("Validate a URL slug").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Slug      string `json:"slug"`
				MaxLength int    `json:"max_length,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			slug := params.Slug
			var issues []string

			// Check for valid characters
			if nonAlphaNum.MatchString(slug) {
				issues = append(issues, "contains invalid characters")
			}

			// Check for double dashes
			if strings.Contains(slug, "--") {
				issues = append(issues, "contains consecutive dashes")
			}

			// Check for leading/trailing dashes
			if strings.HasPrefix(slug, "-") {
				issues = append(issues, "starts with dash")
			}
			if strings.HasSuffix(slug, "-") {
				issues = append(issues, "ends with dash")
			}

			// Check length
			if params.MaxLength > 0 && len(slug) > params.MaxLength {
				issues = append(issues, "exceeds max length")
			}

			// Check for empty
			if slug == "" {
				issues = append(issues, "empty slug")
			}

			valid := len(issues) == 0

			result := map[string]any{
				"slug":   slug,
				"valid":  valid,
				"issues": issues,
				"length": len(slug),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func truncateTool() tool.Tool {
	return tool.NewBuilder("slug_truncate").
		WithDescription("Truncate a slug to max length").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Slug      string `json:"slug"`
				MaxLength int    `json:"max_length"`
				Separator string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sep := params.Separator
			if sep == "" {
				sep = "-"
			}

			truncated := truncateSlug(params.Slug, params.MaxLength, sep)

			result := map[string]any{
				"original":  params.Slug,
				"truncated": truncated,
				"length":    len(truncated),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func joinTool() tool.Tool {
	return tool.NewBuilder("slug_join").
		WithDescription("Join multiple slugs").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Slugs     []string `json:"slugs"`
				Separator string   `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sep := params.Separator
			if sep == "" {
				sep = "-"
			}

			// Filter empty slugs
			var valid []string
			for _, s := range params.Slugs {
				if s != "" {
					valid = append(valid, s)
				}
			}

			joined := strings.Join(valid, sep)

			result := map[string]any{
				"slug":   joined,
				"length": len(joined),
				"parts":  len(valid),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func uniqueTool() tool.Tool {
	return tool.NewBuilder("slug_unique").
		WithDescription("Generate unique slug with suffix").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Slug      string   `json:"slug"`
				Existing  []string `json:"existing"`
				Separator string   `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sep := params.Separator
			if sep == "" {
				sep = "-"
			}

			existing := make(map[string]bool)
			for _, s := range params.Existing {
				existing[s] = true
			}

			slug := params.Slug
			if !existing[slug] {
				result := map[string]any{
					"slug":     slug,
					"suffix":   0,
					"modified": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			for i := 2; i < 1000; i++ {
				candidate := slug + sep + string(rune('0'+i/100)) + string(rune('0'+(i/10)%10)) + string(rune('0'+i%10))
				// Simplify: just use the number
				candidate = slug + sep + strings.TrimLeft(candidate[len(slug)+1:], "0")
				if candidate == slug+sep {
					candidate = slug + sep + "2"
				}
				if !existing[candidate] {
					result := map[string]any{
						"slug":     candidate,
						"suffix":   i,
						"modified": true,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]any{
				"error": "could not generate unique slug",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func reverseTool() tool.Tool {
	return tool.NewBuilder("slug_reverse").
		WithDescription("Reverse slug to readable text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Slug      string `json:"slug"`
				Separator string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sep := params.Separator
			if sep == "" {
				sep = "-"
			}

			// Replace separator with space and title case
			text := strings.ReplaceAll(params.Slug, sep, " ")
			words := strings.Fields(text)
			for i, word := range words {
				if len(word) > 0 {
					words[i] = strings.ToUpper(word[:1]) + word[1:]
				}
			}
			text = strings.Join(words, " ")

			result := map[string]any{
				"slug": params.Slug,
				"text": text,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractWordsTool() tool.Tool {
	return tool.NewBuilder("slug_extract_words").
		WithDescription("Extract words from slug").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Slug      string `json:"slug"`
				Separator string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sep := params.Separator
			if sep == "" {
				sep = "-"
			}

			words := strings.Split(params.Slug, sep)
			var nonEmpty []string
			for _, w := range words {
				if w != "" {
					nonEmpty = append(nonEmpty, w)
				}
			}

			result := map[string]any{
				"slug":  params.Slug,
				"words": nonEmpty,
				"count": len(nonEmpty),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countWordsTool() tool.Tool {
	return tool.NewBuilder("slug_count_words").
		WithDescription("Count words in slug").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Slug      string `json:"slug"`
				Separator string `json:"separator,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			sep := params.Separator
			if sep == "" {
				sep = "-"
			}

			words := strings.Split(params.Slug, sep)
			count := 0
			for _, w := range words {
				if w != "" {
					count++
				}
			}

			result := map[string]any{
				"slug":  params.Slug,
				"count": count,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareTool() tool.Tool {
	return tool.NewBuilder("slug_compare").
		WithDescription("Compare two slugs").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Slug1 string `json:"slug1"`
				Slug2 string `json:"slug2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			equal := params.Slug1 == params.Slug2

			// Check if one is prefix of other
			isPrefix := strings.HasPrefix(params.Slug2, params.Slug1) ||
				strings.HasPrefix(params.Slug1, params.Slug2)

			result := map[string]any{
				"slug1":     params.Slug1,
				"slug2":     params.Slug2,
				"equal":     equal,
				"is_prefix": isPrefix,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func slugify(text string, sep string) string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Replace spaces and underscores with separator
	text = strings.ReplaceAll(text, " ", sep)
	text = strings.ReplaceAll(text, "_", sep)

	// Remove accents and special characters
	var result strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				result.WriteRune(r)
			} else {
				// Skip non-ASCII
			}
		} else if r == '-' || (sep != "" && string(r) == sep) {
			result.WriteString(sep)
		}
	}

	slug := result.String()

	// Replace multiple separators with single
	if sep == "-" {
		slug = multiDash.ReplaceAllString(slug, "-")
	}

	// Trim separators from ends
	slug = strings.Trim(slug, sep)

	return slug
}

func truncateSlug(slug string, maxLength int, sep string) string {
	if len(slug) <= maxLength {
		return slug
	}

	// Try to cut at separator
	truncated := slug[:maxLength]
	lastSep := strings.LastIndex(truncated, sep)
	if lastSep > 0 {
		truncated = truncated[:lastSep]
	}

	return strings.TrimSuffix(truncated, sep)
}
