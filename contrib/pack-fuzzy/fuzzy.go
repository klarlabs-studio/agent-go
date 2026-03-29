// Package fuzzy provides fuzzy matching and search tools for agents.
package fuzzy

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the fuzzy matching tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("fuzzy").
		WithDescription("Fuzzy matching and search utilities").
		AddTools(
			levenshteinTool(),
			jaroWinklerTool(),
			soundexTool(),
			metaphoneTool(),
			fuzzyMatchTool(),
			fuzzySearchTool(),
			similarityTool(),
			ngramTool(),
			diceCoefficientTool(),
			hammingTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func levenshteinTool() tool.Tool {
	return tool.NewBuilder("fuzzy_levenshtein").
		WithDescription("Calculate Levenshtein distance between strings").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A          string `json:"a"`
				B          string `json:"b"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			a, b := params.A, params.B
			if params.IgnoreCase {
				a = strings.ToLower(a)
				b = strings.ToLower(b)
			}

			distance := levenshteinDistance(a, b)
			maxLen := max(len(a), len(b))
			var similarity float64
			if maxLen > 0 {
				similarity = 1 - float64(distance)/float64(maxLen)
			} else {
				similarity = 1
			}

			result := map[string]any{
				"distance":   distance,
				"similarity": similarity,
				"a":          params.A,
				"b":          params.B,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	aRunes := []rune(a)
	bRunes := []rune(b)

	if len(aRunes) > len(bRunes) {
		aRunes, bRunes = bRunes, aRunes
	}

	prev := make([]int, len(aRunes)+1)
	curr := make([]int, len(aRunes)+1)

	for i := range prev {
		prev[i] = i
	}

	for j := 1; j <= len(bRunes); j++ {
		curr[0] = j
		for i := 1; i <= len(aRunes); i++ {
			cost := 0
			if aRunes[i-1] != bRunes[j-1] {
				cost = 1
			}
			curr[i] = min(curr[i-1]+1, min(prev[i]+1, prev[i-1]+cost))
		}
		prev, curr = curr, prev
	}

	return prev[len(aRunes)]
}

func jaroWinklerTool() tool.Tool {
	return tool.NewBuilder("fuzzy_jaro_winkler").
		WithDescription("Calculate Jaro-Winkler similarity").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A          string  `json:"a"`
				B          string  `json:"b"`
				IgnoreCase bool    `json:"ignore_case,omitempty"`
				Prefix     float64 `json:"prefix_scale,omitempty"` // default 0.1
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			a, b := params.A, params.B
			if params.IgnoreCase {
				a = strings.ToLower(a)
				b = strings.ToLower(b)
			}

			prefixScale := params.Prefix
			if prefixScale == 0 {
				prefixScale = 0.1
			}

			jaro := jaroSimilarity(a, b)

			// Find common prefix (max 4 chars)
			prefixLen := 0
			for i := 0; i < min(len(a), min(len(b), 4)); i++ {
				if a[i] == b[i] {
					prefixLen++
				} else {
					break
				}
			}

			jaroWinkler := jaro + float64(prefixLen)*prefixScale*(1-jaro)

			result := map[string]any{
				"jaro":         jaro,
				"jaro_winkler": jaroWinkler,
				"prefix_len":   prefixLen,
				"a":            params.A,
				"b":            params.B,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func jaroSimilarity(a, b string) float64 {
	if a == b {
		return 1
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	matchDistance := max(len(a), len(b))/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	aMatches := make([]bool, len(a))
	bMatches := make([]bool, len(b))

	matches := 0
	transpositions := 0

	for i := range a {
		start := max(0, i-matchDistance)
		end := min(len(b), i+matchDistance+1)

		for j := start; j < end; j++ {
			if bMatches[j] || a[i] != b[j] {
				continue
			}
			aMatches[i] = true
			bMatches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0
	}

	k := 0
	for i := range a {
		if !aMatches[i] {
			continue
		}
		for !bMatches[k] {
			k++
		}
		if a[i] != b[k] {
			transpositions++
		}
		k++
	}

	return (float64(matches)/float64(len(a)) +
		float64(matches)/float64(len(b)) +
		float64(matches-transpositions/2)/float64(matches)) / 3
}

func soundexTool() tool.Tool {
	return tool.NewBuilder("fuzzy_soundex").
		WithDescription("Generate Soundex code for string").
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

			code := soundex(params.Text)

			result := map[string]any{
				"text":    params.Text,
				"soundex": code,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func soundex(s string) string {
	if len(s) == 0 {
		return ""
	}

	// Soundex mapping
	mapping := map[rune]byte{
		'B': '1', 'F': '1', 'P': '1', 'V': '1',
		'C': '2', 'G': '2', 'J': '2', 'K': '2', 'Q': '2', 'S': '2', 'X': '2', 'Z': '2',
		'D': '3', 'T': '3',
		'L': '4',
		'M': '5', 'N': '5',
		'R': '6',
	}

	s = strings.ToUpper(s)
	result := make([]byte, 4)
	result[0] = s[0]
	resultIdx := 1
	lastCode := mapping[rune(s[0])]

	for i := 1; i < len(s) && resultIdx < 4; i++ {
		code, ok := mapping[rune(s[i])]
		if ok && code != lastCode {
			result[resultIdx] = code
			resultIdx++
			lastCode = code
		} else if !ok {
			lastCode = 0
		}
	}

	for resultIdx < 4 {
		result[resultIdx] = '0'
		resultIdx++
	}

	return string(result)
}

func metaphoneTool() tool.Tool {
	return tool.NewBuilder("fuzzy_metaphone").
		WithDescription("Generate Metaphone code for string").
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

			code := metaphone(params.Text)

			result := map[string]any{
				"text":      params.Text,
				"metaphone": code,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func metaphone(s string) string {
	if len(s) == 0 {
		return ""
	}

	s = strings.ToUpper(s)
	var result strings.Builder

	// Simple metaphone implementation
	for i := 0; i < len(s); i++ {
		c := s[i]

		// Skip duplicate adjacent letters
		if i > 0 && c == s[i-1] && c != 'C' {
			continue
		}

		switch c {
		case 'A', 'E', 'I', 'O', 'U':
			if i == 0 {
				result.WriteByte(c)
			}
		case 'B':
			if i == len(s)-1 && i > 0 && s[i-1] == 'M' {
				continue
			}
			result.WriteByte('B')
		case 'C':
			if i+1 < len(s) && (s[i+1] == 'I' || s[i+1] == 'E' || s[i+1] == 'Y') {
				result.WriteByte('S')
			} else if i+1 < len(s) && s[i+1] == 'H' {
				result.WriteByte('X')
				i++
			} else {
				result.WriteByte('K')
			}
		case 'D':
			if i+1 < len(s) && s[i+1] == 'G' {
				result.WriteByte('J')
				i++
			} else {
				result.WriteByte('T')
			}
		case 'F', 'J', 'L', 'M', 'N', 'R':
			result.WriteByte(c)
		case 'G':
			if i+1 < len(s) && (s[i+1] == 'I' || s[i+1] == 'E' || s[i+1] == 'Y') {
				result.WriteByte('J')
			} else {
				result.WriteByte('K')
			}
		case 'H':
			if i > 0 && strings.ContainsRune("AEIOU", rune(s[i-1])) {
				continue
			}
			if i+1 < len(s) && strings.ContainsRune("AEIOU", rune(s[i+1])) {
				result.WriteByte('H')
			}
		case 'K':
			if i > 0 && s[i-1] == 'C' {
				continue
			}
			result.WriteByte('K')
		case 'P':
			if i+1 < len(s) && s[i+1] == 'H' {
				result.WriteByte('F')
				i++
			} else {
				result.WriteByte('P')
			}
		case 'Q':
			result.WriteByte('K')
		case 'S':
			if i+1 < len(s) && s[i+1] == 'H' {
				result.WriteByte('X')
				i++
			} else {
				result.WriteByte('S')
			}
		case 'T':
			if i+1 < len(s) && s[i+1] == 'H' {
				result.WriteByte('0')
				i++
			} else if i+2 < len(s) && s[i+1] == 'I' && (s[i+2] == 'O' || s[i+2] == 'A') {
				result.WriteByte('X')
			} else {
				result.WriteByte('T')
			}
		case 'V':
			result.WriteByte('F')
		case 'W', 'Y':
			if i+1 < len(s) && strings.ContainsRune("AEIOU", rune(s[i+1])) {
				result.WriteByte(c)
			}
		case 'X':
			result.WriteString("KS")
		case 'Z':
			result.WriteByte('S')
		}
	}

	return result.String()
}

func fuzzyMatchTool() tool.Tool {
	return tool.NewBuilder("fuzzy_match").
		WithDescription("Check if pattern fuzzy matches text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Pattern    string  `json:"pattern"`
				Text       string  `json:"text"`
				Threshold  float64 `json:"threshold,omitempty"` // default 0.6
				IgnoreCase bool    `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			threshold := params.Threshold
			if threshold == 0 {
				threshold = 0.6
			}

			pattern, text := params.Pattern, params.Text
			if params.IgnoreCase {
				pattern = strings.ToLower(pattern)
				text = strings.ToLower(text)
			}

			// Calculate various similarity metrics
			levDist := levenshteinDistance(pattern, text)
			maxLen := max(len(pattern), len(text))
			var levSim float64
			if maxLen > 0 {
				levSim = 1 - float64(levDist)/float64(maxLen)
			} else {
				levSim = 1
			}

			jaro := jaroSimilarity(pattern, text)

			// Average similarity
			avgSim := (levSim + jaro) / 2
			matches := avgSim >= threshold

			result := map[string]any{
				"matches":     matches,
				"score":       avgSim,
				"levenshtein": levSim,
				"jaro":        jaro,
				"threshold":   threshold,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func fuzzySearchTool() tool.Tool {
	return tool.NewBuilder("fuzzy_search").
		WithDescription("Search for best matches in a list").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query      string   `json:"query"`
				Candidates []string `json:"candidates"`
				Limit      int      `json:"limit,omitempty"`
				Threshold  float64  `json:"threshold,omitempty"`
				IgnoreCase bool     `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			limit := params.Limit
			if limit <= 0 {
				limit = 5
			}

			threshold := params.Threshold
			if threshold == 0 {
				threshold = 0.3
			}

			query := params.Query
			if params.IgnoreCase {
				query = strings.ToLower(query)
			}

			type Match struct {
				Text  string  `json:"text"`
				Score float64 `json:"score"`
				Index int     `json:"index"`
			}

			var matches []Match

			for i, candidate := range params.Candidates {
				text := candidate
				if params.IgnoreCase {
					text = strings.ToLower(text)
				}

				levDist := levenshteinDistance(query, text)
				maxLen := max(len(query), len(text))
				var score float64
				if maxLen > 0 {
					score = 1 - float64(levDist)/float64(maxLen)
				} else {
					score = 1
				}

				if score >= threshold {
					matches = append(matches, Match{
						Text:  candidate,
						Score: score,
						Index: i,
					})
				}
			}

			// Sort by score descending
			sort.Slice(matches, func(i, j int) bool {
				return matches[i].Score > matches[j].Score
			})

			if len(matches) > limit {
				matches = matches[:limit]
			}

			result := map[string]any{
				"query":   params.Query,
				"matches": matches,
				"count":   len(matches),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func similarityTool() tool.Tool {
	return tool.NewBuilder("fuzzy_similarity").
		WithDescription("Compare multiple strings for similarity").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Strings    []string `json:"strings"`
				IgnoreCase bool     `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := len(params.Strings)
			if n < 2 {
				result := map[string]any{"error": "need at least 2 strings"}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Create similarity matrix
			matrix := make([][]float64, n)
			for i := range matrix {
				matrix[i] = make([]float64, n)
			}

			strs := params.Strings
			if params.IgnoreCase {
				strs = make([]string, n)
				for i, s := range params.Strings {
					strs[i] = strings.ToLower(s)
				}
			}

			var totalSim float64
			comparisons := 0

			for i := 0; i < n; i++ {
				matrix[i][i] = 1.0
				for j := i + 1; j < n; j++ {
					sim := jaroSimilarity(strs[i], strs[j])
					matrix[i][j] = sim
					matrix[j][i] = sim
					totalSim += sim
					comparisons++
				}
			}

			avgSim := 0.0
			if comparisons > 0 {
				avgSim = totalSim / float64(comparisons)
			}

			result := map[string]any{
				"strings":            params.Strings,
				"matrix":             matrix,
				"average_similarity": avgSim,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func ngramTool() tool.Tool {
	return tool.NewBuilder("fuzzy_ngram").
		WithDescription("Generate n-grams from text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
				N    int    `json:"n,omitempty"` // default 2 (bigrams)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n <= 0 {
				n = 2
			}

			text := params.Text
			runes := []rune(text)

			var ngrams []string
			for i := 0; i <= len(runes)-n; i++ {
				ngrams = append(ngrams, string(runes[i:i+n]))
			}

			// Count unique ngrams
			counts := make(map[string]int)
			for _, ng := range ngrams {
				counts[ng]++
			}

			result := map[string]any{
				"text":   params.Text,
				"n":      n,
				"ngrams": ngrams,
				"unique": len(counts),
				"counts": counts,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func diceCoefficientTool() tool.Tool {
	return tool.NewBuilder("fuzzy_dice").
		WithDescription("Calculate Dice coefficient between strings").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A          string `json:"a"`
				B          string `json:"b"`
				N          int    `json:"n,omitempty"` // n-gram size, default 2
				IgnoreCase bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.N
			if n <= 0 {
				n = 2
			}

			a, b := params.A, params.B
			if params.IgnoreCase {
				a = strings.ToLower(a)
				b = strings.ToLower(b)
			}

			// Generate n-grams
			ngramsA := make(map[string]bool)
			ngramsB := make(map[string]bool)

			runesA := []rune(a)
			runesB := []rune(b)

			for i := 0; i <= len(runesA)-n; i++ {
				ngramsA[string(runesA[i:i+n])] = true
			}
			for i := 0; i <= len(runesB)-n; i++ {
				ngramsB[string(runesB[i:i+n])] = true
			}

			// Count intersection
			intersection := 0
			for ng := range ngramsA {
				if ngramsB[ng] {
					intersection++
				}
			}

			var dice float64
			total := len(ngramsA) + len(ngramsB)
			if total > 0 {
				dice = 2 * float64(intersection) / float64(total)
			}

			result := map[string]any{
				"dice":         dice,
				"intersection": intersection,
				"a_ngrams":     len(ngramsA),
				"b_ngrams":     len(ngramsB),
				"a":            params.A,
				"b":            params.B,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hammingTool() tool.Tool {
	return tool.NewBuilder("fuzzy_hamming").
		WithDescription("Calculate Hamming distance between strings").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A          string `json:"a"`
				B          string `json:"b"`
				IgnoreCase bool   `json:"ignore_case,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			a, b := params.A, params.B
			if params.IgnoreCase {
				a = strings.ToLower(a)
				b = strings.ToLower(b)
			}

			runesA := []rune(a)
			runesB := []rune(b)

			if len(runesA) != len(runesB) {
				result := map[string]any{
					"error":    "strings must be of equal length for Hamming distance",
					"a_length": len(runesA),
					"b_length": len(runesB),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			distance := 0
			for i := range runesA {
				if runesA[i] != runesB[i] {
					distance++
				}
			}

			var similarity float64
			if len(runesA) > 0 {
				similarity = 1 - float64(distance)/float64(len(runesA))
			} else {
				similarity = 1
			}

			result := map[string]any{
				"distance":   distance,
				"similarity": similarity,
				"length":     len(runesA),
				"a":          params.A,
				"b":          params.B,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Helper to filter non-letters
func filterLetters(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}
