// Package tokenizer provides text tokenization tools for agents.
package tokenizer

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

// Pack returns the tokenizer tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("tokenizer").
		WithDescription("Text tokenization tools").
		AddTools(
			wordTool(),
			sentenceTool(),
			whitespaceTool(),
			regexTool(),
			ngramTool(),
			charTool(),
			punctuationTool(),
			normalizeTool(),
			stemTool(),
			stopwordsTool(),
			countTool(),
			frequencyTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func wordTool() tool.Tool {
	return tool.NewBuilder("tokenize_words").
		WithDescription("Tokenize text into words").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text      string `json:"text"`
				Lowercase bool   `json:"lowercase,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			text := params.Text
			if params.Lowercase {
				text = strings.ToLower(text)
			}

			// Split on non-word characters but keep contractions
			wordPattern := regexp.MustCompile(`[\w']+`)
			tokens := wordPattern.FindAllString(text, -1)

			result := map[string]any{
				"tokens": tokens,
				"count":  len(tokens),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sentenceTool() tool.Tool {
	return tool.NewBuilder("tokenize_sentences").
		WithDescription("Tokenize text into sentences").
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

			// Split on sentence-ending punctuation followed by space or end
			sentencePattern := regexp.MustCompile(`[^.!?]+[.!?]+`)
			matches := sentencePattern.FindAllString(params.Text, -1)

			var sentences []string
			for _, s := range matches {
				s = strings.TrimSpace(s)
				if s != "" {
					sentences = append(sentences, s)
				}
			}

			// Handle text without punctuation
			if len(sentences) == 0 && len(params.Text) > 0 {
				sentences = []string{strings.TrimSpace(params.Text)}
			}

			result := map[string]any{
				"sentences": sentences,
				"count":     len(sentences),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func whitespaceTool() tool.Tool {
	return tool.NewBuilder("tokenize_whitespace").
		WithDescription("Tokenize text by whitespace").
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

			tokens := strings.Fields(params.Text)

			result := map[string]any{
				"tokens": tokens,
				"count":  len(tokens),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func regexTool() tool.Tool {
	return tool.NewBuilder("tokenize_regex").
		WithDescription("Tokenize text using a custom regex pattern").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text    string `json:"text"`
				Pattern string `json:"pattern"`
				Split   bool   `json:"split,omitempty"` // If true, use pattern as delimiter
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			pattern, err := regexp.Compile(params.Pattern)
			if err != nil {
				return tool.Result{}, err
			}

			var tokens []string
			if params.Split {
				tokens = pattern.Split(params.Text, -1)
			} else {
				tokens = pattern.FindAllString(params.Text, -1)
			}

			// Filter empty strings
			var filtered []string
			for _, t := range tokens {
				if t != "" {
					filtered = append(filtered, t)
				}
			}

			result := map[string]any{
				"tokens":  filtered,
				"count":   len(filtered),
				"pattern": params.Pattern,
				"mode":    map[bool]string{true: "split", false: "match"}[params.Split],
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func ngramTool() tool.Tool {
	return tool.NewBuilder("tokenize_ngrams").
		WithDescription("Generate n-grams from text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
				N    int    `json:"n,omitempty"`
				Type string `json:"type,omitempty"` // word, char
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n <= 0 {
				n = 2
			}

			var ngrams []string

			if strings.ToLower(params.Type) == "char" {
				// Character n-grams
				text := params.Text
				for i := 0; i <= len(text)-n; i++ {
					ngrams = append(ngrams, text[i:i+n])
				}
			} else {
				// Word n-grams
				words := strings.Fields(params.Text)
				for i := 0; i <= len(words)-n; i++ {
					ngram := strings.Join(words[i:i+n], " ")
					ngrams = append(ngrams, ngram)
				}
			}

			result := map[string]any{
				"ngrams": ngrams,
				"count":  len(ngrams),
				"n":      n,
				"type":   params.Type,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func charTool() tool.Tool {
	return tool.NewBuilder("tokenize_chars").
		WithDescription("Tokenize text into individual characters").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text            string `json:"text"`
				IncludeSpaces   bool   `json:"include_spaces,omitempty"`
				GroupCategories bool   `json:"group_categories,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var tokens []string
			var categories map[string][]string
			if params.GroupCategories {
				categories = make(map[string][]string)
			}

			for _, r := range params.Text {
				if !params.IncludeSpaces && unicode.IsSpace(r) {
					continue
				}
				char := string(r)
				tokens = append(tokens, char)

				if params.GroupCategories {
					var cat string
					switch {
					case unicode.IsLetter(r):
						cat = "letter"
					case unicode.IsDigit(r):
						cat = "digit"
					case unicode.IsSpace(r):
						cat = "space"
					case unicode.IsPunct(r):
						cat = "punctuation"
					default:
						cat = "other"
					}
					categories[cat] = append(categories[cat], char)
				}
			}

			result := map[string]any{
				"tokens": tokens,
				"count":  len(tokens),
			}
			if params.GroupCategories {
				result["categories"] = categories
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func punctuationTool() tool.Tool {
	return tool.NewBuilder("tokenize_punctuation").
		WithDescription("Tokenize text keeping punctuation separate").
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

			// Split into words and punctuation
			pattern := regexp.MustCompile(`(\w+|[^\w\s])`)
			matches := pattern.FindAllString(params.Text, -1)

			var tokens []map[string]any
			for _, m := range matches {
				tokenType := "word"
				if len(m) == 1 && !unicode.IsLetter(rune(m[0])) && !unicode.IsDigit(rune(m[0])) {
					tokenType = "punctuation"
				}
				tokens = append(tokens, map[string]any{
					"token": m,
					"type":  tokenType,
				})
			}

			result := map[string]any{
				"tokens": tokens,
				"count":  len(tokens),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func normalizeTool() tool.Tool {
	return tool.NewBuilder("tokenize_normalize").
		WithDescription("Normalize and tokenize text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text             string `json:"text"`
				Lowercase        bool   `json:"lowercase,omitempty"`
				RemovePunct      bool   `json:"remove_punctuation,omitempty"`
				RemoveNumbers    bool   `json:"remove_numbers,omitempty"`
				RemoveWhitespace bool   `json:"collapse_whitespace,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			text := params.Text

			if params.Lowercase {
				text = strings.ToLower(text)
			}

			if params.RemovePunct {
				text = regexp.MustCompile(`[^\w\s]`).ReplaceAllString(text, "")
			}

			if params.RemoveNumbers {
				text = regexp.MustCompile(`\d+`).ReplaceAllString(text, "")
			}

			if params.RemoveWhitespace {
				text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
				text = strings.TrimSpace(text)
			}

			tokens := strings.Fields(text)

			result := map[string]any{
				"tokens":     tokens,
				"count":      len(tokens),
				"normalized": text,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func stemTool() tool.Tool {
	return tool.NewBuilder("tokenize_stem").
		WithDescription("Apply basic stemming to tokens").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Tokens []string `json:"tokens"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Simple Porter-like stemming rules
			suffixes := []string{"ing", "ed", "ly", "ness", "ment", "tion", "sion", "ity", "ies", "es", "s"}

			var stemmed []map[string]string
			for _, token := range params.Tokens {
				original := token
				lower := strings.ToLower(token)

				for _, suffix := range suffixes {
					if strings.HasSuffix(lower, suffix) && len(lower) > len(suffix)+2 {
						lower = lower[:len(lower)-len(suffix)]
						break
					}
				}

				stemmed = append(stemmed, map[string]string{
					"original": original,
					"stemmed":  lower,
				})
			}

			result := map[string]any{
				"tokens": stemmed,
				"count":  len(stemmed),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func stopwordsTool() tool.Tool {
	return tool.NewBuilder("tokenize_remove_stopwords").
		WithDescription("Remove common stopwords from tokens").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Tokens   []string `json:"tokens"`
				Language string   `json:"language,omitempty"`
				Custom   []string `json:"custom_stopwords,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Common English stopwords
			stopwords := map[string]bool{
				"a": true, "an": true, "and": true, "are": true, "as": true,
				"at": true, "be": true, "by": true, "for": true, "from": true,
				"has": true, "have": true, "he": true, "in": true, "is": true,
				"it": true, "its": true, "of": true, "on": true, "or": true,
				"that": true, "the": true, "to": true, "was": true, "were": true,
				"will": true, "with": true, "i": true, "me": true, "my": true,
				"you": true, "your": true, "we": true, "our": true, "they": true,
				"their": true, "this": true, "these": true, "those": true,
				"but": true, "not": true, "so": true, "if": true, "then": true,
				"than": true, "too": true, "very": true, "can": true, "just": true,
				"should": true, "now": true, "been": true, "being": true,
			}

			// Add custom stopwords
			for _, word := range params.Custom {
				stopwords[strings.ToLower(word)] = true
			}

			var filtered []string
			var removed []string
			for _, token := range params.Tokens {
				if stopwords[strings.ToLower(token)] {
					removed = append(removed, token)
				} else {
					filtered = append(filtered, token)
				}
			}

			result := map[string]any{
				"tokens":        filtered,
				"count":         len(filtered),
				"removed":       removed,
				"removed_count": len(removed),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countTool() tool.Tool {
	return tool.NewBuilder("tokenize_count").
		WithDescription("Count tokens, words, characters, and lines").
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
			words := strings.Fields(text)
			lines := strings.Split(text, "\n")

			// Count unique words
			unique := make(map[string]bool)
			for _, w := range words {
				unique[strings.ToLower(w)] = true
			}

			result := map[string]any{
				"characters":       len(text),
				"characters_no_ws": len(strings.ReplaceAll(text, " ", "")),
				"words":            len(words),
				"unique_words":     len(unique),
				"lines":            len(lines),
				"paragraphs":       len(regexp.MustCompile(`\n\s*\n`).Split(text, -1)),
				"estimated_tokens": len(text) / 4,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func frequencyTool() tool.Tool {
	return tool.NewBuilder("tokenize_frequency").
		WithDescription("Calculate token frequency").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Tokens    []string `json:"tokens"`
				TopN      int      `json:"top_n,omitempty"`
				Lowercase bool     `json:"lowercase,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			freq := make(map[string]int)
			for _, token := range params.Tokens {
				key := token
				if params.Lowercase {
					key = strings.ToLower(token)
				}
				freq[key]++
			}

			// Sort by frequency
			type tokenFreq struct {
				Token     string  `json:"token"`
				Count     int     `json:"count"`
				Frequency float64 `json:"frequency"`
			}

			var sorted []tokenFreq
			total := len(params.Tokens)
			for token, count := range freq {
				sorted = append(sorted, tokenFreq{
					Token:     token,
					Count:     count,
					Frequency: float64(count) / float64(total),
				})
			}

			// Simple sort by count (descending)
			for i := 0; i < len(sorted); i++ {
				for j := i + 1; j < len(sorted); j++ {
					if sorted[j].Count > sorted[i].Count {
						sorted[i], sorted[j] = sorted[j], sorted[i]
					}
				}
			}

			topN := params.TopN
			if topN <= 0 || topN > len(sorted) {
				topN = len(sorted)
			}

			result := map[string]any{
				"frequencies":  sorted[:topN],
				"unique_count": len(freq),
				"total_count":  total,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
