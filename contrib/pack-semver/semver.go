// Package semver provides semantic versioning tools for agents.
package semver

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/Masterminds/semver/v3"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the semver tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("semver").
		WithDescription("Semantic versioning tools").
		AddTools(
			parseTool(),
			validateTool(),
			compareTool(),
			bumpMajorTool(),
			bumpMinorTool(),
			bumpPatchTool(),
			setPrereleaseToool(),
			setMetadataTool(),
			constraintTool(),
			sortTool(),
			latestTool(),
			rangeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("semver_parse").
		WithDescription("Parse a semantic version string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			v, err := semver.NewVersion(params.Version)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"major":      v.Major(),
				"minor":      v.Minor(),
				"patch":      v.Patch(),
				"prerelease": v.Prerelease(),
				"metadata":   v.Metadata(),
				"original":   v.Original(),
				"string":     v.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("semver_validate").
		WithDescription("Validate a semantic version string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			_, err := semver.NewVersion(params.Version)
			valid := err == nil
			errorMsg := ""
			if err != nil {
				errorMsg = err.Error()
			}

			result := map[string]any{
				"valid": valid,
				"error": errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareTool() tool.Tool {
	return tool.NewBuilder("semver_compare").
		WithDescription("Compare two semantic versions").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version1 string `json:"version1"`
				Version2 string `json:"version2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			v1, err := semver.NewVersion(params.Version1)
			if err != nil {
				return tool.Result{}, err
			}
			v2, err := semver.NewVersion(params.Version2)
			if err != nil {
				return tool.Result{}, err
			}

			cmp := v1.Compare(v2)
			relation := "equal"
			if cmp < 0 {
				relation = "less"
			} else if cmp > 0 {
				relation = "greater"
			}

			result := map[string]any{
				"comparison": cmp,
				"relation":   relation,
				"equal":      v1.Equal(v2),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func bumpMajorTool() tool.Tool {
	return tool.NewBuilder("semver_bump_major").
		WithDescription("Increment major version").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			v, err := semver.NewVersion(params.Version)
			if err != nil {
				return tool.Result{}, err
			}

			newV := v.IncMajor()

			result := map[string]any{
				"original": v.String(),
				"bumped":   newV.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func bumpMinorTool() tool.Tool {
	return tool.NewBuilder("semver_bump_minor").
		WithDescription("Increment minor version").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			v, err := semver.NewVersion(params.Version)
			if err != nil {
				return tool.Result{}, err
			}

			newV := v.IncMinor()

			result := map[string]any{
				"original": v.String(),
				"bumped":   newV.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func bumpPatchTool() tool.Tool {
	return tool.NewBuilder("semver_bump_patch").
		WithDescription("Increment patch version").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			v, err := semver.NewVersion(params.Version)
			if err != nil {
				return tool.Result{}, err
			}

			newV := v.IncPatch()

			result := map[string]any{
				"original": v.String(),
				"bumped":   newV.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func setPrereleaseToool() tool.Tool {
	return tool.NewBuilder("semver_set_prerelease").
		WithDescription("Set prerelease identifier").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version    string `json:"version"`
				Prerelease string `json:"prerelease"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			v, err := semver.NewVersion(params.Version)
			if err != nil {
				return tool.Result{}, err
			}

			newV, err := v.SetPrerelease(params.Prerelease)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"original": v.String(),
				"updated":  newV.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func setMetadataTool() tool.Tool {
	return tool.NewBuilder("semver_set_metadata").
		WithDescription("Set build metadata").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version  string `json:"version"`
				Metadata string `json:"metadata"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			v, err := semver.NewVersion(params.Version)
			if err != nil {
				return tool.Result{}, err
			}

			newV, err := v.SetMetadata(params.Metadata)
			if err != nil {
				return tool.Result{}, err
			}

			result := map[string]any{
				"original": v.String(),
				"updated":  newV.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func constraintTool() tool.Tool {
	return tool.NewBuilder("semver_constraint").
		WithDescription("Check if version satisfies constraint").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Version    string `json:"version"`
				Constraint string `json:"constraint"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			v, err := semver.NewVersion(params.Version)
			if err != nil {
				return tool.Result{}, err
			}

			c, err := semver.NewConstraint(params.Constraint)
			if err != nil {
				return tool.Result{}, err
			}

			satisfies, reasons := c.Validate(v)

			var errorMsgs []string
			for _, r := range reasons {
				errorMsgs = append(errorMsgs, r.Error())
			}

			result := map[string]any{
				"satisfies": satisfies,
				"errors":    errorMsgs,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sortTool() tool.Tool {
	return tool.NewBuilder("semver_sort").
		WithDescription("Sort a list of versions").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Versions   []string `json:"versions"`
				Descending bool     `json:"descending,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var vs []*semver.Version
			for _, vStr := range params.Versions {
				v, err := semver.NewVersion(vStr)
				if err != nil {
					return tool.Result{}, err
				}
				vs = append(vs, v)
			}

			if params.Descending {
				sort.Sort(sort.Reverse(semver.Collection(vs)))
			} else {
				sort.Sort(semver.Collection(vs))
			}

			var sorted []string
			for _, v := range vs {
				sorted = append(sorted, v.String())
			}

			result := map[string]any{
				"sorted": sorted,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func latestTool() tool.Tool {
	return tool.NewBuilder("semver_latest").
		WithDescription("Find the latest version from a list").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Versions         []string `json:"versions"`
				IncludePrerelase bool     `json:"include_prerelease,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var vs []*semver.Version
			for _, vStr := range params.Versions {
				v, err := semver.NewVersion(vStr)
				if err != nil {
					continue // skip invalid
				}
				if !params.IncludePrerelase && v.Prerelease() != "" {
					continue
				}
				vs = append(vs, v)
			}

			if len(vs) == 0 {
				result := map[string]any{
					"latest": nil,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			sort.Sort(sort.Reverse(semver.Collection(vs)))

			result := map[string]any{
				"latest": vs[0].String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func rangeTool() tool.Tool {
	return tool.NewBuilder("semver_range").
		WithDescription("Filter versions by constraint range").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Versions   []string `json:"versions"`
				Constraint string   `json:"constraint"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			c, err := semver.NewConstraint(params.Constraint)
			if err != nil {
				return tool.Result{}, err
			}

			var matching []string
			for _, vStr := range params.Versions {
				v, err := semver.NewVersion(vStr)
				if err != nil {
					continue
				}
				if c.Check(v) {
					matching = append(matching, v.String())
				}
			}

			result := map[string]any{
				"matching": matching,
				"count":    len(matching),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
