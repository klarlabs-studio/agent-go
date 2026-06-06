// Package similarity provides text similarity and comparison tools for agents.
package similarity

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"unicode"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the text similarity tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("similarity").
		WithDescription("Text similarity and comparison tools").
		AddTools(
			levenshteinTool(),
			jaroWinklerTool(),
			hammingTool(),
			cosineTool(),
			jaccardTool(),
			sorensenDiceTool(),
			ngramTool(),
			fuzzyMatchTool(),
			bestMatchTool(),
			normalizeTextTool(),
			tokensCompareTool(),
			ratioTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func levenshteinTool() tool.Tool {
	return tool.NewBuilder("similarity_levenshtein").
		WithDescription("Calculate Levenshtein (edit) distance").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			distance := levenshteinDistance(params.Text1, params.Text2)
			maxLen := max(len(params.Text1), len(params.Text2))
			similarity := 0.0
			if maxLen > 0 {
				similarity = 1.0 - float64(distance)/float64(maxLen)
			}

			result := map[string]any{
				"distance":   distance,
				"similarity": similarity,
				"length1":    len(params.Text1),
				"length2":    len(params.Text2),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func jaroWinklerTool() tool.Tool {
	return tool.NewBuilder("similarity_jaro_winkler").
		WithDescription("Calculate Jaro-Winkler similarity").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			jaro := jaroSimilarity(params.Text1, params.Text2)
			jaroWinkler := jaroWinklerSimilarity(params.Text1, params.Text2, jaro)

			result := map[string]any{
				"jaro":         jaro,
				"jaro_winkler": jaroWinkler,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hammingTool() tool.Tool {
	return tool.NewBuilder("similarity_hamming").
		WithDescription("Calculate Hamming distance (for equal length strings)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			r1 := []rune(params.Text1)
			r2 := []rune(params.Text2)

			if len(r1) != len(r2) {
				result := map[string]any{
					"error":   "strings must be of equal length",
					"length1": len(r1),
					"length2": len(r2),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			distance := 0
			for i := range r1 {
				if r1[i] != r2[i] {
					distance++
				}
			}

			similarity := 1.0 - float64(distance)/float64(len(r1))

			result := map[string]any{
				"distance":   distance,
				"similarity": similarity,
				"length":     len(r1),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func cosineTool() tool.Tool {
	return tool.NewBuilder("similarity_cosine").
		WithDescription("Calculate cosine similarity based on word vectors").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			words1 := tokenize(params.Text1)
			words2 := tokenize(params.Text2)

			// Build word frequency vectors
			allWords := make(map[string]bool)
			freq1 := make(map[string]int)
			freq2 := make(map[string]int)

			for _, w := range words1 {
				freq1[w]++
				allWords[w] = true
			}
			for _, w := range words2 {
				freq2[w]++
				allWords[w] = true
			}

			// Calculate cosine similarity
			var dotProduct, norm1, norm2 float64
			for word := range allWords {
				v1 := float64(freq1[word])
				v2 := float64(freq2[word])
				dotProduct += v1 * v2
				norm1 += v1 * v1
				norm2 += v2 * v2
			}

			similarity := 0.0
			if norm1 > 0 && norm2 > 0 {
				similarity = dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
			}

			result := map[string]any{
				"similarity":   similarity,
				"unique_words": len(allWords),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func jaccardTool() tool.Tool {
	return tool.NewBuilder("similarity_jaccard").
		WithDescription("Calculate Jaccard similarity coefficient").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			set1 := make(map[string]bool)
			set2 := make(map[string]bool)

			for _, w := range tokenize(params.Text1) {
				set1[w] = true
			}
			for _, w := range tokenize(params.Text2) {
				set2[w] = true
			}

			intersection := 0
			for w := range set1 {
				if set2[w] {
					intersection++
				}
			}

			union := len(set1) + len(set2) - intersection

			similarity := 0.0
			if union > 0 {
				similarity = float64(intersection) / float64(union)
			}

			result := map[string]any{
				"similarity":   similarity,
				"intersection": intersection,
				"union":        union,
				"size1":        len(set1),
				"size2":        len(set2),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sorensenDiceTool() tool.Tool {
	return tool.NewBuilder("similarity_sorensen_dice").
		WithDescription("Calculate Sorensen-Dice coefficient").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			set1 := make(map[string]bool)
			set2 := make(map[string]bool)

			for _, w := range tokenize(params.Text1) {
				set1[w] = true
			}
			for _, w := range tokenize(params.Text2) {
				set2[w] = true
			}

			intersection := 0
			for w := range set1 {
				if set2[w] {
					intersection++
				}
			}

			similarity := 0.0
			if len(set1)+len(set2) > 0 {
				similarity = (2.0 * float64(intersection)) / float64(len(set1)+len(set2))
			}

			result := map[string]any{
				"similarity":   similarity,
				"intersection": intersection,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func ngramTool() tool.Tool {
	return tool.NewBuilder("similarity_ngram").
		WithDescription("Calculate n-gram similarity").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
				N     int    `json:"n,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n <= 0 {
				n = 2 // bigrams by default
			}

			ngrams1 := getNgrams(params.Text1, n)
			ngrams2 := getNgrams(params.Text2, n)

			intersection := 0
			for ng := range ngrams1 {
				if ngrams2[ng] {
					intersection++
				}
			}

			union := len(ngrams1) + len(ngrams2) - intersection
			similarity := 0.0
			if union > 0 {
				similarity = float64(intersection) / float64(union)
			}

			result := map[string]any{
				"similarity":   similarity,
				"n":            n,
				"ngrams1":      len(ngrams1),
				"ngrams2":      len(ngrams2),
				"intersection": intersection,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fuzzyMatchTool() tool.Tool {
	return tool.NewBuilder("similarity_fuzzy_match").
		WithDescription("Check if strings match with a threshold").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1     string  `json:"text1"`
				Text2     string  `json:"text2"`
				Threshold float64 `json:"threshold,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			threshold := params.Threshold
			if threshold == 0 {
				threshold = 0.8
			}

			distance := levenshteinDistance(strings.ToLower(params.Text1), strings.ToLower(params.Text2))
			maxLen := max(len(params.Text1), len(params.Text2))
			similarity := 0.0
			if maxLen > 0 {
				similarity = 1.0 - float64(distance)/float64(maxLen)
			}

			result := map[string]any{
				"matches":    similarity >= threshold,
				"similarity": similarity,
				"threshold":  threshold,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func bestMatchTool() tool.Tool {
	return tool.NewBuilder("similarity_best_match").
		WithDescription("Find best match from a list of candidates").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query      string   `json:"query"`
				Candidates []string `json:"candidates"`
				Threshold  float64  `json:"threshold,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			threshold := params.Threshold
			query := strings.ToLower(params.Query)

			type match struct {
				Text       string  `json:"text"`
				Similarity float64 `json:"similarity"`
				Index      int     `json:"index"`
			}

			var matches []match
			for i, c := range params.Candidates {
				distance := levenshteinDistance(query, strings.ToLower(c))
				maxLen := max(len(query), len(c))
				similarity := 0.0
				if maxLen > 0 {
					similarity = 1.0 - float64(distance)/float64(maxLen)
				}

				if threshold == 0 || similarity >= threshold {
					matches = append(matches, match{
						Text:       c,
						Similarity: similarity,
						Index:      i,
					})
				}
			}

			// Sort by similarity descending
			sort.Slice(matches, func(i, j int) bool {
				return matches[i].Similarity > matches[j].Similarity
			})

			result := map[string]any{
				"query":   params.Query,
				"matches": matches,
				"count":   len(matches),
			}

			if len(matches) > 0 {
				result["best"] = matches[0]
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func normalizeTextTool() tool.Tool {
	return tool.NewBuilder("similarity_normalize").
		WithDescription("Normalize text for comparison").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text        string `json:"text"`
				Lowercase   bool   `json:"lowercase,omitempty"`
				RemovePunct bool   `json:"remove_punct,omitempty"`
				Trim        bool   `json:"trim,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			text := params.Text

			if params.Lowercase {
				text = strings.ToLower(text)
			}

			if params.RemovePunct {
				var sb strings.Builder
				for _, r := range text {
					if !unicode.IsPunct(r) {
						sb.WriteRune(r)
					}
				}
				text = sb.String()
			}

			if params.Trim {
				text = strings.TrimSpace(text)
				// Also normalize whitespace
				words := strings.Fields(text)
				text = strings.Join(words, " ")
			}

			result := map[string]any{
				"original":   params.Text,
				"normalized": text,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func tokensCompareTool() tool.Tool {
	return tool.NewBuilder("similarity_tokens_compare").
		WithDescription("Compare tokenized versions of texts").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			tokens1 := tokenize(params.Text1)
			tokens2 := tokenize(params.Text2)

			// Find common tokens
			set1 := make(map[string]bool)
			for _, t := range tokens1 {
				set1[t] = true
			}

			var common []string
			var onlyIn2 []string
			set2 := make(map[string]bool)
			for _, t := range tokens2 {
				set2[t] = true
				if set1[t] {
					common = append(common, t)
				} else {
					onlyIn2 = append(onlyIn2, t)
				}
			}

			var onlyIn1 []string
			for _, t := range tokens1 {
				if !set2[t] {
					onlyIn1 = append(onlyIn1, t)
				}
			}

			result := map[string]any{
				"tokens1":   tokens1,
				"tokens2":   tokens2,
				"common":    common,
				"only_in_1": onlyIn1,
				"only_in_2": onlyIn2,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func ratioTool() tool.Tool {
	return tool.NewBuilder("similarity_ratio").
		WithDescription("Calculate multiple similarity metrics at once").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text1 string `json:"text1"`
				Text2 string `json:"text2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Levenshtein
			distance := levenshteinDistance(params.Text1, params.Text2)
			maxLen := max(len(params.Text1), len(params.Text2))
			levenshtein := 0.0
			if maxLen > 0 {
				levenshtein = 1.0 - float64(distance)/float64(maxLen)
			}

			// Jaro-Winkler
			jaro := jaroSimilarity(params.Text1, params.Text2)
			jaroWinkler := jaroWinklerSimilarity(params.Text1, params.Text2, jaro)

			// Jaccard
			set1 := make(map[string]bool)
			set2 := make(map[string]bool)
			for _, w := range tokenize(params.Text1) {
				set1[w] = true
			}
			for _, w := range tokenize(params.Text2) {
				set2[w] = true
			}
			intersection := 0
			for w := range set1 {
				if set2[w] {
					intersection++
				}
			}
			union := len(set1) + len(set2) - intersection
			jaccard := 0.0
			if union > 0 {
				jaccard = float64(intersection) / float64(union)
			}

			result := map[string]any{
				"levenshtein":  levenshtein,
				"jaro":         jaro,
				"jaro_winkler": jaroWinkler,
				"jaccard":      jaccard,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Helper functions

func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)

	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	// Create matrix
	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
		d[i][0] = i
	}
	for j := 0; j <= m; j++ {
		d[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
		}
	}

	return d[n][m]
}

func jaroSimilarity(s1, s2 string) float64 {
	r1, r2 := []rune(s1), []rune(s2)
	if len(r1) == 0 && len(r2) == 0 {
		return 1.0
	}
	if len(r1) == 0 || len(r2) == 0 {
		return 0.0
	}

	matchDistance := max(len(r1), len(r2))/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	s1Matches := make([]bool, len(r1))
	s2Matches := make([]bool, len(r2))

	matches := 0
	transpositions := 0

	for i := range r1 {
		start := max(0, i-matchDistance)
		end := min(i+matchDistance+1, len(r2))

		for j := start; j < end; j++ {
			if s2Matches[j] || r1[i] != r2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	k := 0
	for i := range r1 {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if r1[i] != r2[k] {
			transpositions++
		}
		k++
	}

	m := float64(matches)
	return (m/float64(len(r1)) + m/float64(len(r2)) + (m-float64(transpositions)/2)/m) / 3
}

func jaroWinklerSimilarity(s1, s2 string, jaro float64) float64 {
	r1, r2 := []rune(s1), []rune(s2)
	prefixLen := 0
	maxPrefix := min(4, min(len(r1), len(r2)))

	for i := 0; i < maxPrefix; i++ {
		if r1[i] == r2[i] {
			prefixLen++
		} else {
			break
		}
	}

	return jaro + float64(prefixLen)*0.1*(1-jaro)
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func getNgrams(text string, n int) map[string]bool {
	ngrams := make(map[string]bool)
	runes := []rune(strings.ToLower(text))

	if len(runes) < n {
		return ngrams
	}

	for i := 0; i <= len(runes)-n; i++ {
		ngram := string(runes[i : i+n])
		ngrams[ngram] = true
	}

	return ngrams
}
