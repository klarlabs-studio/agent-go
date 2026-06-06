// Package pluralize provides word pluralization tools for agents.
package pluralize

import (
	"context"
	"encoding/json"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Common irregular plurals
var irregularPlurals = map[string]string{
	"child":      "children",
	"person":     "people",
	"man":        "men",
	"woman":      "women",
	"foot":       "feet",
	"tooth":      "teeth",
	"goose":      "geese",
	"mouse":      "mice",
	"louse":      "lice",
	"ox":         "oxen",
	"sheep":      "sheep",
	"deer":       "deer",
	"fish":       "fish",
	"species":    "species",
	"series":     "series",
	"aircraft":   "aircraft",
	"salmon":     "salmon",
	"trout":      "trout",
	"moose":      "moose",
	"bison":      "bison",
	"criterion":  "criteria",
	"phenomenon": "phenomena",
	"datum":      "data",
	"medium":     "media",
	"analysis":   "analyses",
	"thesis":     "theses",
	"crisis":     "crises",
	"diagnosis":  "diagnoses",
	"hypothesis": "hypotheses",
	"oasis":      "oases",
	"index":      "indices",
	"appendix":   "appendices",
	"matrix":     "matrices",
	"vertex":     "vertices",
	"focus":      "foci",
	"radius":     "radii",
	"stimulus":   "stimuli",
	"cactus":     "cacti",
	"fungus":     "fungi",
	"nucleus":    "nuclei",
	"syllabus":   "syllabi",
	"alumnus":    "alumni",
}

// Reverse mapping for singularization
var irregularSingulars = func() map[string]string {
	result := make(map[string]string)
	for singular, plural := range irregularPlurals {
		result[plural] = singular
	}
	return result
}()

// Pack returns the pluralization tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("pluralize").
		WithDescription("Word pluralization tools").
		AddTools(
			pluralizeTool(),
			singularizeTool(),
			isPluralTool(),
			isSingularTool(),
			countTool(),
			pluralRuleTool(),
			irregularsTool(),
			addIrregularTool(),
			pluralizeListTool(),
			singularizeListTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func pluralizeTool() tool.Tool {
	return tool.NewBuilder("pluralize").
		WithDescription("Convert word to plural form").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Word string `json:"word"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			plural := pluralizeWord(params.Word)

			result := map[string]any{
				"singular": params.Word,
				"plural":   plural,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func singularizeTool() tool.Tool {
	return tool.NewBuilder("singularize").
		WithDescription("Convert word to singular form").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Word string `json:"word"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			singular := singularizeWord(params.Word)

			result := map[string]any{
				"plural":   params.Word,
				"singular": singular,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isPluralTool() tool.Tool {
	return tool.NewBuilder("is_plural").
		WithDescription("Check if word is plural").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Word string `json:"word"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			word := strings.ToLower(params.Word)

			// Check if it's a known plural
			_, isIrregularPlural := irregularSingulars[word]

			// Check common plural patterns
			isPlural := isIrregularPlural ||
				strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") ||
				strings.HasSuffix(word, "ies") ||
				strings.HasSuffix(word, "es")

			result := map[string]any{
				"word":      params.Word,
				"is_plural": isPlural,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isSingularTool() tool.Tool {
	return tool.NewBuilder("is_singular").
		WithDescription("Check if word is singular").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Word string `json:"word"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			word := strings.ToLower(params.Word)

			// Check if it's a known singular
			_, isIrregularSingular := irregularPlurals[word]

			// Words that don't end in common plural patterns are likely singular
			isSingular := isIrregularSingular ||
				(!strings.HasSuffix(word, "s") ||
					strings.HasSuffix(word, "ss") ||
					strings.HasSuffix(word, "us") ||
					strings.HasSuffix(word, "is"))

			result := map[string]any{
				"word":        params.Word,
				"is_singular": isSingular,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countTool() tool.Tool {
	return tool.NewBuilder("pluralize_count").
		WithDescription("Pluralize based on count").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Word  string `json:"word"`
				Count int    `json:"count"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var word string
			if params.Count == 1 {
				word = params.Word
			} else {
				word = pluralizeWord(params.Word)
			}

			result := map[string]any{
				"word":  word,
				"count": params.Count,
				"text":  word,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pluralRuleTool() tool.Tool {
	return tool.NewBuilder("pluralize_rule").
		WithDescription("Get pluralization rule for word").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Word string `json:"word"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			word := strings.ToLower(params.Word)
			var rule string

			switch {
			case irregularPlurals[word] != "":
				rule = "irregular"
			case strings.HasSuffix(word, "y") && !isVowel(rune(word[len(word)-2])):
				rule = "y_to_ies"
			case strings.HasSuffix(word, "s") || strings.HasSuffix(word, "x") ||
				strings.HasSuffix(word, "z") || strings.HasSuffix(word, "ch") ||
				strings.HasSuffix(word, "sh"):
				rule = "add_es"
			case strings.HasSuffix(word, "f"):
				rule = "f_to_ves"
			case strings.HasSuffix(word, "fe"):
				rule = "fe_to_ves"
			default:
				rule = "add_s"
			}

			result := map[string]any{
				"word": params.Word,
				"rule": rule,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func irregularsTool() tool.Tool {
	return tool.NewBuilder("pluralize_irregulars").
		WithDescription("List irregular plurals").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Limit int `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var irregulars []map[string]string
			count := 0
			for singular, plural := range irregularPlurals {
				if params.Limit > 0 && count >= params.Limit {
					break
				}
				irregulars = append(irregulars, map[string]string{
					"singular": singular,
					"plural":   plural,
				})
				count++
			}

			result := map[string]any{
				"irregulars": irregulars,
				"count":      len(irregulars),
				"total":      len(irregularPlurals),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func addIrregularTool() tool.Tool {
	return tool.NewBuilder("pluralize_add_irregular").
		WithDescription("Add custom irregular plural").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Singular string `json:"singular"`
				Plural   string `json:"plural"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			singular := strings.ToLower(params.Singular)
			plural := strings.ToLower(params.Plural)

			irregularPlurals[singular] = plural
			irregularSingulars[plural] = singular

			result := map[string]any{
				"singular": singular,
				"plural":   plural,
				"added":    true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pluralizeListTool() tool.Tool {
	return tool.NewBuilder("pluralize_list").
		WithDescription("Pluralize a list of words").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Words []string `json:"words"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var plurals []string
			for _, word := range params.Words {
				plurals = append(plurals, pluralizeWord(word))
			}

			result := map[string]any{
				"plurals": plurals,
				"count":   len(plurals),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func singularizeListTool() tool.Tool {
	return tool.NewBuilder("singularize_list").
		WithDescription("Singularize a list of words").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Words []string `json:"words"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var singulars []string
			for _, word := range params.Words {
				singulars = append(singulars, singularizeWord(word))
			}

			result := map[string]any{
				"singulars": singulars,
				"count":     len(singulars),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pluralizeWord(word string) string {
	lower := strings.ToLower(word)

	// Check irregular
	if plural, ok := irregularPlurals[lower]; ok {
		return preserveCase(word, plural)
	}

	// Apply rules
	switch {
	case strings.HasSuffix(lower, "y") && len(lower) > 1 && !isVowel(rune(lower[len(lower)-2])):
		return word[:len(word)-1] + "ies"
	case strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") ||
		strings.HasSuffix(lower, "z") || strings.HasSuffix(lower, "ch") ||
		strings.HasSuffix(lower, "sh"):
		return word + "es"
	case strings.HasSuffix(lower, "f"):
		return word[:len(word)-1] + "ves"
	case strings.HasSuffix(lower, "fe"):
		return word[:len(word)-2] + "ves"
	default:
		return word + "s"
	}
}

func singularizeWord(word string) string {
	lower := strings.ToLower(word)

	// Check irregular
	if singular, ok := irregularSingulars[lower]; ok {
		return preserveCase(word, singular)
	}

	// Apply reverse rules
	switch {
	case strings.HasSuffix(lower, "ies"):
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(lower, "ves"):
		// Could be f or fe
		base := word[:len(word)-3]
		return base + "f"
	case strings.HasSuffix(lower, "es"):
		base := word[:len(word)-2]
		// Check if base ends in s, x, z, ch, sh
		baseLower := strings.ToLower(base)
		if strings.HasSuffix(baseLower, "s") || strings.HasSuffix(baseLower, "x") ||
			strings.HasSuffix(baseLower, "z") || strings.HasSuffix(baseLower, "ch") ||
			strings.HasSuffix(baseLower, "sh") {
			return base
		}
		return word[:len(word)-1] // Just remove s
	case strings.HasSuffix(lower, "s") && len(lower) > 1:
		return word[:len(word)-1]
	default:
		return word
	}
}

func isVowel(r rune) bool {
	return r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u'
}

func preserveCase(original, replacement string) string {
	if len(original) == 0 || len(replacement) == 0 {
		return replacement
	}

	// If original is all uppercase, return uppercase
	if strings.ToUpper(original) == original {
		return strings.ToUpper(replacement)
	}

	// If first letter is uppercase, capitalize replacement
	if original[0] >= 'A' && original[0] <= 'Z' {
		return strings.ToUpper(replacement[:1]) + replacement[1:]
	}

	return replacement
}
