// Package diff provides text diffing and patching tools for agents.
package diff

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the diff tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("diff").
		WithDescription("Text diffing and patching tools").
		AddTools(
			diffTextTool(),
			diffLinesTool(),
			patchTool(),
			unifiedDiffTool(),
			levenshteinTool(),
			commonPrefixTool(),
			commonSuffixTool(),
			semanticCleanupTool(),
			prettyHtmlTool(),
			deltaEncodeTool(),
			deltaDecodeTool(),
			matchTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func diffTextTool() tool.Tool {
	return tool.NewBuilder("diff_text").
		WithDescription("Compute character-level diff between two texts").
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

			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(params.Text1, params.Text2, true)

			var changes []map[string]any
			for _, d := range diffs {
				op := "equal"
				switch d.Type {
				case diffmatchpatch.DiffInsert:
					op = "insert"
				case diffmatchpatch.DiffDelete:
					op = "delete"
				}
				changes = append(changes, map[string]any{
					"op":   op,
					"text": d.Text,
				})
			}

			result := map[string]any{
				"diffs": changes,
				"count": len(changes),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func diffLinesTool() tool.Tool {
	return tool.NewBuilder("diff_lines").
		WithDescription("Compute line-level diff between two texts").
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

			dmp := diffmatchpatch.New()
			a, b, c := dmp.DiffLinesToChars(params.Text1, params.Text2)
			diffs := dmp.DiffMain(a, b, false)
			diffs = dmp.DiffCharsToLines(diffs, c)

			var changes []map[string]any
			for _, d := range diffs {
				op := "equal"
				switch d.Type {
				case diffmatchpatch.DiffInsert:
					op = "insert"
				case diffmatchpatch.DiffDelete:
					op = "delete"
				}
				lines := strings.Split(strings.TrimSuffix(d.Text, "\n"), "\n")
				changes = append(changes, map[string]any{
					"op":    op,
					"lines": lines,
				})
			}

			result := map[string]any{
				"diffs": changes,
				"count": len(changes),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func patchTool() tool.Tool {
	return tool.NewBuilder("diff_patch").
		WithDescription("Apply a patch to text").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text    string `json:"text"`
				Patches string `json:"patches"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			dmp := diffmatchpatch.New()
			patches, err := dmp.PatchFromText(params.Patches)
			if err != nil {
				return tool.Result{}, err
			}

			newText, applied := dmp.PatchApply(patches, params.Text)

			allApplied := true
			for _, a := range applied {
				if !a {
					allApplied = false
					break
				}
			}

			result := map[string]any{
				"text":        newText,
				"all_applied": allApplied,
				"applied":     applied,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func unifiedDiffTool() tool.Tool {
	return tool.NewBuilder("diff_unified").
		WithDescription("Create unified diff format").
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

			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(params.Text1, params.Text2, true)
			patches := dmp.PatchMake(params.Text1, diffs)
			patchText := dmp.PatchToText(patches)

			result := map[string]any{
				"patch": patchText,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func levenshteinTool() tool.Tool {
	return tool.NewBuilder("diff_levenshtein").
		WithDescription("Calculate Levenshtein distance between two texts").
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

			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(params.Text1, params.Text2, false)
			distance := dmp.DiffLevenshtein(diffs)

			maxLen := len(params.Text1)
			if len(params.Text2) > maxLen {
				maxLen = len(params.Text2)
			}
			similarity := 0.0
			if maxLen > 0 {
				similarity = 1.0 - float64(distance)/float64(maxLen)
			}

			result := map[string]any{
				"distance":   distance,
				"similarity": similarity,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func commonPrefixTool() tool.Tool {
	return tool.NewBuilder("diff_common_prefix").
		WithDescription("Find common prefix of two texts").
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

			dmp := diffmatchpatch.New()
			prefixLen := dmp.DiffCommonPrefix(params.Text1, params.Text2)

			result := map[string]any{
				"prefix": params.Text1[:prefixLen],
				"length": prefixLen,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func commonSuffixTool() tool.Tool {
	return tool.NewBuilder("diff_common_suffix").
		WithDescription("Find common suffix of two texts").
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

			dmp := diffmatchpatch.New()
			suffixLen := dmp.DiffCommonSuffix(params.Text1, params.Text2)

			suffix := ""
			if suffixLen > 0 {
				suffix = params.Text1[len(params.Text1)-suffixLen:]
			}

			result := map[string]any{
				"suffix": suffix,
				"length": suffixLen,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func semanticCleanupTool() tool.Tool {
	return tool.NewBuilder("diff_semantic_cleanup").
		WithDescription("Clean up diff for human readability").
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

			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(params.Text1, params.Text2, true)
			diffs = dmp.DiffCleanupSemantic(diffs)

			var changes []map[string]any
			for _, d := range diffs {
				op := "equal"
				switch d.Type {
				case diffmatchpatch.DiffInsert:
					op = "insert"
				case diffmatchpatch.DiffDelete:
					op = "delete"
				}
				changes = append(changes, map[string]any{
					"op":   op,
					"text": d.Text,
				})
			}

			result := map[string]any{
				"diffs": changes,
				"count": len(changes),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func prettyHtmlTool() tool.Tool {
	return tool.NewBuilder("diff_pretty_html").
		WithDescription("Generate HTML visualization of diff").
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

			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(params.Text1, params.Text2, true)
			diffs = dmp.DiffCleanupSemantic(diffs)
			html := dmp.DiffPrettyHtml(diffs)

			result := map[string]any{
				"html": html,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deltaEncodeTool() tool.Tool {
	return tool.NewBuilder("diff_delta_encode").
		WithDescription("Encode diff as delta format").
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

			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(params.Text1, params.Text2, true)
			delta := dmp.DiffToDelta(diffs)

			result := map[string]any{
				"delta": delta,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deltaDecodeTool() tool.Tool {
	return tool.NewBuilder("diff_delta_decode").
		WithDescription("Decode delta format to apply to text").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text  string `json:"text"`
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			dmp := diffmatchpatch.New()
			diffs, err := dmp.DiffFromDelta(params.Text, params.Delta)
			if err != nil {
				return tool.Result{}, err
			}

			text2 := dmp.DiffText2(diffs)

			result := map[string]any{
				"text": text2,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func matchTool() tool.Tool {
	return tool.NewBuilder("diff_match").
		WithDescription("Find best match location for pattern in text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text    string `json:"text"`
				Pattern string `json:"pattern"`
				Loc     int    `json:"loc,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			dmp := diffmatchpatch.New()
			loc := dmp.MatchMain(params.Text, params.Pattern, params.Loc)

			result := map[string]any{
				"location": loc,
				"found":    loc >= 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
