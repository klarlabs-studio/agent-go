// Package changelog provides changelog generation and parsing tools for agents.
package changelog

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the changelog tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("changelog").
		WithDescription("Changelog generation and parsing tools").
		AddTools(
			parseTool(),
			generateTool(),
			addEntryTool(),
			parseCommitsTool(),
			formatKeepAChangelogTool(),
			extractVersionTool(),
			validateTool(),
			mergeTool(),
			filterByTypeTool(),
			getUnreleasedTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("changelog_parse").
		WithDescription("Parse a changelog file").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			versions := parseChangelog(params.Content)

			result := map[string]any{
				"versions": versions,
				"count":    len(versions),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateTool() tool.Tool {
	return tool.NewBuilder("changelog_generate").
		WithDescription("Generate a changelog from entries").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Title   string `json:"title,omitempty"`
				Entries []struct {
					Version string `json:"version"`
					Date    string `json:"date,omitempty"`
					Changes []struct {
						Type    string `json:"type"` // added, changed, deprecated, removed, fixed, security
						Message string `json:"message"`
					} `json:"changes"`
				} `json:"entries"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			title := params.Title
			if title == "" {
				title = "Changelog"
			}

			var sb strings.Builder
			sb.WriteString("# " + title + "\n\n")
			sb.WriteString("All notable changes to this project will be documented in this file.\n\n")

			for _, entry := range params.Entries {
				date := entry.Date
				if date == "" {
					date = time.Now().Format("2006-01-02")
				}
				sb.WriteString("## [" + entry.Version + "] - " + date + "\n\n")

				// Group by type
				byType := make(map[string][]string)
				for _, c := range entry.Changes {
					byType[c.Type] = append(byType[c.Type], c.Message)
				}

				typeOrder := []string{"added", "changed", "deprecated", "removed", "fixed", "security"}
				for _, t := range typeOrder {
					if msgs, ok := byType[t]; ok {
						sb.WriteString("### " + strings.Title(t) + "\n\n")
						for _, m := range msgs {
							sb.WriteString("- " + m + "\n")
						}
						sb.WriteString("\n")
					}
				}
			}

			result := map[string]any{
				"changelog": sb.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func addEntryTool() tool.Tool {
	return tool.NewBuilder("changelog_add_entry").
		WithDescription("Add a new entry to a changelog").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
				Version string `json:"version"`
				Date    string `json:"date,omitempty"`
				Type    string `json:"type"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			date := params.Date
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			// Find or create version section
			versionHeader := "## [" + params.Version + "]"
			typeHeader := "### " + strings.Title(params.Type)
			newEntry := "- " + params.Message

			content := params.Content
			if !strings.Contains(content, versionHeader) {
				// Insert new version after main header
				insertPos := strings.Index(content, "\n\n")
				if insertPos == -1 {
					insertPos = len(content)
				} else {
					insertPos += 2
				}
				newSection := versionHeader + " - " + date + "\n\n" + typeHeader + "\n\n" + newEntry + "\n\n"
				content = content[:insertPos] + newSection + content[insertPos:]
			} else {
				// Find version section
				versionPos := strings.Index(content, versionHeader)
				nextVersionPos := strings.Index(content[versionPos+len(versionHeader):], "\n## ")
				if nextVersionPos == -1 {
					nextVersionPos = len(content) - versionPos - len(versionHeader)
				}
				sectionEnd := versionPos + len(versionHeader) + nextVersionPos

				section := content[versionPos:sectionEnd]
				if strings.Contains(section, typeHeader) {
					// Add to existing type section
					typePos := strings.Index(section, typeHeader)
					insertPos := typePos + len(typeHeader) + 1
					// Find next line
					for insertPos < len(section) && section[insertPos] != '\n' {
						insertPos++
					}
					insertPos++
					section = section[:insertPos] + newEntry + "\n" + section[insertPos:]
				} else {
					// Add new type section
					endOfHeader := strings.Index(section, "\n\n")
					if endOfHeader == -1 {
						endOfHeader = len(section)
					} else {
						endOfHeader += 2
					}
					section = section[:endOfHeader] + typeHeader + "\n\n" + newEntry + "\n\n" + section[endOfHeader:]
				}
				content = content[:versionPos] + section + content[sectionEnd:]
			}

			result := map[string]any{
				"changelog": content,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func parseCommitsTool() tool.Tool {
	return tool.NewBuilder("changelog_parse_commits").
		WithDescription("Parse conventional commits into changelog entries").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Commits []string `json:"commits"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Conventional commit pattern
			re := regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|chore|build|ci)(\(.+\))?!?:\s*(.+)$`)

			typeMap := map[string]string{
				"feat":     "added",
				"fix":      "fixed",
				"docs":     "changed",
				"style":    "changed",
				"refactor": "changed",
				"perf":     "changed",
				"test":     "changed",
				"chore":    "changed",
				"build":    "changed",
				"ci":       "changed",
			}

			var entries []map[string]string
			for _, commit := range params.Commits {
				matches := re.FindStringSubmatch(commit)
				if matches != nil {
					changeType := typeMap[matches[1]]
					scope := ""
					if matches[2] != "" {
						scope = strings.Trim(matches[2], "()")
					}
					message := matches[3]

					entries = append(entries, map[string]string{
						"type":    changeType,
						"scope":   scope,
						"message": message,
						"raw":     commit,
					})
				}
			}

			result := map[string]any{
				"entries": entries,
				"count":   len(entries),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatKeepAChangelogTool() tool.Tool {
	return tool.NewBuilder("changelog_format_keepachangelog").
		WithDescription("Format changelog according to Keep a Changelog standard").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ProjectName string `json:"project_name"`
				Versions    []struct {
					Version string              `json:"version"`
					Date    string              `json:"date"`
					Changes map[string][]string `json:"changes"`
				} `json:"versions"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var sb strings.Builder
			sb.WriteString("# Changelog\n\n")
			sb.WriteString("All notable changes to this project will be documented in this file.\n\n")
			sb.WriteString("The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),\n")
			sb.WriteString("and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).\n\n")

			typeOrder := []string{"Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"}

			for _, v := range params.Versions {
				if v.Version == "Unreleased" {
					sb.WriteString("## [Unreleased]\n\n")
				} else {
					sb.WriteString("## [" + v.Version + "] - " + v.Date + "\n\n")
				}

				for _, t := range typeOrder {
					if changes, ok := v.Changes[t]; ok && len(changes) > 0 {
						sb.WriteString("### " + t + "\n\n")
						for _, c := range changes {
							sb.WriteString("- " + c + "\n")
						}
						sb.WriteString("\n")
					}
				}
			}

			result := map[string]any{
				"changelog": sb.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractVersionTool() tool.Tool {
	return tool.NewBuilder("changelog_extract_version").
		WithDescription("Extract a specific version from changelog").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
				Version string `json:"version"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			versions := parseChangelog(params.Content)
			for _, v := range versions {
				if v["version"] == params.Version {
					result := map[string]any{
						"found":   true,
						"version": v,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]any{
				"found":   false,
				"version": nil,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("changelog_validate").
		WithDescription("Validate changelog format").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var issues []string
			content := params.Content

			// Check for main header
			if !strings.HasPrefix(content, "# ") {
				issues = append(issues, "Missing main header")
			}

			// Check for version format
			versionRe := regexp.MustCompile(`## \[.+\]`)
			if !versionRe.MatchString(content) {
				issues = append(issues, "No version sections found")
			}

			// Check for proper date format
			dateRe := regexp.MustCompile(`## \[.+\] - \d{4}-\d{2}-\d{2}`)
			if !dateRe.MatchString(content) && !strings.Contains(content, "[Unreleased]") {
				issues = append(issues, "Version sections missing dates in YYYY-MM-DD format")
			}

			result := map[string]any{
				"valid":  len(issues) == 0,
				"issues": issues,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mergeTool() tool.Tool {
	return tool.NewBuilder("changelog_merge").
		WithDescription("Merge two changelogs").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Changelog1 string `json:"changelog1"`
				Changelog2 string `json:"changelog2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			versions1 := parseChangelog(params.Changelog1)
			versions2 := parseChangelog(params.Changelog2)

			// Merge versions by version number
			merged := make(map[string]map[string]any)
			for _, v := range versions1 {
				merged[v["version"].(string)] = v
			}
			for _, v := range versions2 {
				ver := v["version"].(string)
				if existing, ok := merged[ver]; ok {
					// Merge changes
					for key, val := range v {
						if key != "version" && key != "date" {
							existing[key] = val
						}
					}
				} else {
					merged[ver] = v
				}
			}

			// Convert to slice and sort
			var result []map[string]any
			for _, v := range merged {
				result = append(result, v)
			}
			sort.Slice(result, func(i, j int) bool {
				return result[i]["version"].(string) > result[j]["version"].(string)
			})

			output, _ := json.Marshal(map[string]any{
				"merged": result,
				"count":  len(result),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func filterByTypeTool() tool.Tool {
	return tool.NewBuilder("changelog_filter_by_type").
		WithDescription("Filter changelog entries by change type").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string   `json:"content"`
				Types   []string `json:"types"` // added, changed, fixed, etc.
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			versions := parseChangelog(params.Content)

			typeSet := make(map[string]bool)
			for _, t := range params.Types {
				typeSet[strings.ToLower(t)] = true
			}

			var filtered []map[string]any
			for _, v := range versions {
				filteredV := map[string]any{
					"version": v["version"],
					"date":    v["date"],
				}
				for key, val := range v {
					if typeSet[strings.ToLower(key)] {
						filteredV[key] = val
					}
				}
				filtered = append(filtered, filteredV)
			}

			result := map[string]any{
				"versions": filtered,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getUnreleasedTool() tool.Tool {
	return tool.NewBuilder("changelog_get_unreleased").
		WithDescription("Get unreleased changes from changelog").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			versions := parseChangelog(params.Content)
			for _, v := range versions {
				if v["version"] == "Unreleased" {
					result := map[string]any{
						"found":      true,
						"unreleased": v,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]any{
				"found":      false,
				"unreleased": nil,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// parseChangelog parses a Keep a Changelog format into structured data
func parseChangelog(content string) []map[string]any {
	var versions []map[string]any

	versionRe := regexp.MustCompile(`## \[([^\]]+)\](?:\s*-\s*(\d{4}-\d{2}-\d{2}))?`)
	typeRe := regexp.MustCompile(`### (Added|Changed|Deprecated|Removed|Fixed|Security)`)

	lines := strings.Split(content, "\n")

	var currentVersion map[string]any
	var currentType string

	for _, line := range lines {
		if matches := versionRe.FindStringSubmatch(line); matches != nil {
			if currentVersion != nil {
				versions = append(versions, currentVersion)
			}
			currentVersion = map[string]any{
				"version": matches[1],
				"date":    matches[2],
			}
			currentType = ""
		} else if matches := typeRe.FindStringSubmatch(line); matches != nil {
			currentType = strings.ToLower(matches[1])
		} else if strings.HasPrefix(strings.TrimSpace(line), "- ") && currentVersion != nil && currentType != "" {
			entry := strings.TrimPrefix(strings.TrimSpace(line), "- ")
			if currentVersion[currentType] == nil {
				currentVersion[currentType] = []string{}
			}
			currentVersion[currentType] = append(currentVersion[currentType].([]string), entry)
		}
	}

	if currentVersion != nil {
		versions = append(versions, currentVersion)
	}

	return versions
}
