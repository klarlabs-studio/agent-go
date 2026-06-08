// Package license provides license detection and generation tools for agents.
package license

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the license tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("license").
		WithDescription("License detection and generation tools").
		AddTools(
			detectTool(),
			generateTool(),
			validateTool(),
			compareTool(),
			listTool(),
			infoTool(),
			compatibilityTool(),
			extractCopyrightTool(),
			spdxParseTool(),
			checkDependenciesTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Common license identifiers and their patterns
var licensePatterns = map[string]struct {
	patterns []string
	spdx     string
	name     string
	osi      bool
}{
	"mit": {
		patterns: []string{"MIT License", "Permission is hereby granted, free of charge"},
		spdx:     "MIT",
		name:     "MIT License",
		osi:      true,
	},
	"apache-2.0": {
		patterns: []string{"Apache License", "Version 2.0", "Apache-2.0"},
		spdx:     "Apache-2.0",
		name:     "Apache License 2.0",
		osi:      true,
	},
	"gpl-3.0": {
		patterns: []string{"GNU GENERAL PUBLIC LICENSE", "Version 3"},
		spdx:     "GPL-3.0",
		name:     "GNU General Public License v3.0",
		osi:      true,
	},
	"gpl-2.0": {
		patterns: []string{"GNU GENERAL PUBLIC LICENSE", "Version 2"},
		spdx:     "GPL-2.0",
		name:     "GNU General Public License v2.0",
		osi:      true,
	},
	"bsd-3-clause": {
		patterns: []string{"BSD 3-Clause", "Redistribution and use in source and binary forms"},
		spdx:     "BSD-3-Clause",
		name:     "BSD 3-Clause License",
		osi:      true,
	},
	"bsd-2-clause": {
		patterns: []string{"BSD 2-Clause", "Simplified BSD License"},
		spdx:     "BSD-2-Clause",
		name:     "BSD 2-Clause License",
		osi:      true,
	},
	"isc": {
		patterns: []string{"ISC License", "Permission to use, copy, modify, and/or distribute"},
		spdx:     "ISC",
		name:     "ISC License",
		osi:      true,
	},
	"mpl-2.0": {
		patterns: []string{"Mozilla Public License", "Version 2.0"},
		spdx:     "MPL-2.0",
		name:     "Mozilla Public License 2.0",
		osi:      true,
	},
	"lgpl-3.0": {
		patterns: []string{"GNU LESSER GENERAL PUBLIC LICENSE", "Version 3"},
		spdx:     "LGPL-3.0",
		name:     "GNU Lesser General Public License v3.0",
		osi:      true,
	},
	"agpl-3.0": {
		patterns: []string{"GNU AFFERO GENERAL PUBLIC LICENSE", "Version 3"},
		spdx:     "AGPL-3.0",
		name:     "GNU Affero General Public License v3.0",
		osi:      true,
	},
	"unlicense": {
		patterns: []string{"This is free and unencumbered software", "Unlicense"},
		spdx:     "Unlicense",
		name:     "The Unlicense",
		osi:      true,
	},
	"cc0-1.0": {
		patterns: []string{"CC0 1.0", "Creative Commons Zero"},
		spdx:     "CC0-1.0",
		name:     "Creative Commons Zero v1.0 Universal",
		osi:      false,
	},
}

func detectTool() tool.Tool {
	return tool.NewBuilder("license_detect").
		WithDescription("Detect license type from license text").
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

			content := strings.ToUpper(params.Content)
			var detected []map[string]any
			var confidence float64

			for id, info := range licensePatterns {
				matches := 0
				for _, pattern := range info.patterns {
					if strings.Contains(content, strings.ToUpper(pattern)) {
						matches++
					}
				}
				if matches > 0 {
					conf := float64(matches) / float64(len(info.patterns))
					detected = append(detected, map[string]any{
						"id":         id,
						"spdx":       info.spdx,
						"name":       info.name,
						"confidence": conf,
						"osi":        info.osi,
					})
					if conf > confidence {
						confidence = conf
					}
				}
			}

			result := map[string]any{
				"detected":   detected,
				"count":      len(detected),
				"confidence": confidence,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateTool() tool.Tool {
	return tool.NewBuilder("license_generate").
		WithDescription("Generate license text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				License string `json:"license"` // mit, apache-2.0, etc.
				Name    string `json:"name,omitempty"`
				Year    int    `json:"year,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			year := params.Year
			if year == 0 {
				year = time.Now().Year()
			}

			var text string
			switch strings.ToLower(params.License) {
			case "mit":
				text = generateMIT(params.Name, year)
			case "apache-2.0":
				text = generateApache2(params.Name, year)
			case "isc":
				text = generateISC(params.Name, year)
			case "bsd-3-clause":
				text = generateBSD3(params.Name, year)
			case "unlicense":
				text = generateUnlicense()
			default:
				text = ""
			}

			result := map[string]any{
				"license": text,
				"spdx":    strings.ToUpper(params.License),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("license_validate").
		WithDescription("Validate a license file").
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

			// Check for copyright notice
			if !strings.Contains(strings.ToLower(params.Content), "copyright") {
				issues = append(issues, "Missing copyright notice")
			}

			// Check for year
			yearRe := regexp.MustCompile(`\d{4}`)
			if !yearRe.MatchString(params.Content) {
				issues = append(issues, "Missing year in copyright notice")
			}

			// Try to detect license
			content := strings.ToUpper(params.Content)
			detected := false
			for _, info := range licensePatterns {
				for _, pattern := range info.patterns {
					if strings.Contains(content, strings.ToUpper(pattern)) {
						detected = true
						break
					}
				}
				if detected {
					break
				}
			}

			if !detected {
				issues = append(issues, "Could not identify license type")
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

func compareTool() tool.Tool {
	return tool.NewBuilder("license_compare").
		WithDescription("Compare two licenses for compatibility").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				License1 string `json:"license1"`
				License2 string `json:"license2"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			l1 := strings.ToLower(params.License1)
			l2 := strings.ToLower(params.License2)

			// Simple compatibility matrix
			permissive := map[string]bool{
				"mit": true, "apache-2.0": true, "bsd-2-clause": true,
				"bsd-3-clause": true, "isc": true, "unlicense": true, "cc0-1.0": true,
			}
			copyleft := map[string]bool{
				"gpl-2.0": true, "gpl-3.0": true, "agpl-3.0": true,
			}
			weakCopyleft := map[string]bool{
				"lgpl-3.0": true, "mpl-2.0": true,
			}

			var compatibility string
			switch {
			case permissive[l1] && permissive[l2]:
				compatibility = "compatible"
			case copyleft[l1] && copyleft[l2] && l1 == l2:
				compatibility = "compatible"
			case permissive[l1] && (copyleft[l2] || weakCopyleft[l2]):
				compatibility = "compatible"
			case (copyleft[l1] || weakCopyleft[l1]) && permissive[l2]:
				compatibility = "may-require-relicense"
			default:
				compatibility = "potentially-incompatible"
			}

			result := map[string]any{
				"license1":      params.License1,
				"license2":      params.License2,
				"compatibility": compatibility,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("license_list").
		WithDescription("List available license templates").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var licenses []map[string]any
			for id, info := range licensePatterns {
				licenses = append(licenses, map[string]any{
					"id":   id,
					"spdx": info.spdx,
					"name": info.name,
					"osi":  info.osi,
				})
			}

			result := map[string]any{
				"licenses": licenses,
				"count":    len(licenses),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func infoTool() tool.Tool {
	return tool.NewBuilder("license_info").
		WithDescription("Get information about a specific license").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				License string `json:"license"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			l := strings.ToLower(params.License)
			if info, ok := licensePatterns[l]; ok {
				permissions := []string{}
				conditions := []string{}
				limitations := []string{}

				switch l {
				case "mit", "isc":
					permissions = []string{"commercial-use", "modification", "distribution", "private-use"}
					conditions = []string{"license-and-copyright-notice"}
					limitations = []string{"liability", "warranty"}
				case "apache-2.0":
					permissions = []string{"commercial-use", "modification", "distribution", "patent-use", "private-use"}
					conditions = []string{"license-and-copyright-notice", "state-changes"}
					limitations = []string{"trademark-use", "liability", "warranty"}
				case "gpl-3.0":
					permissions = []string{"commercial-use", "modification", "distribution", "patent-use", "private-use"}
					conditions = []string{"disclose-source", "license-and-copyright-notice", "same-license", "state-changes"}
					limitations = []string{"liability", "warranty"}
				}

				result := map[string]any{
					"id":          l,
					"spdx":        info.spdx,
					"name":        info.name,
					"osi":         info.osi,
					"permissions": permissions,
					"conditions":  conditions,
					"limitations": limitations,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"error": "License not found",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compatibilityTool() tool.Tool {
	return tool.NewBuilder("license_compatibility_matrix").
		WithDescription("Get license compatibility matrix").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			// Simplified compatibility matrix
			matrix := map[string]map[string]string{
				"mit": {
					"mit": "yes", "apache-2.0": "yes", "gpl-2.0": "yes", "gpl-3.0": "yes",
					"bsd-3-clause": "yes", "lgpl-3.0": "yes", "mpl-2.0": "yes",
				},
				"apache-2.0": {
					"mit": "yes", "apache-2.0": "yes", "gpl-2.0": "no", "gpl-3.0": "yes",
					"bsd-3-clause": "yes", "lgpl-3.0": "yes", "mpl-2.0": "yes",
				},
				"gpl-3.0": {
					"mit": "yes", "apache-2.0": "yes", "gpl-2.0": "no", "gpl-3.0": "yes",
					"bsd-3-clause": "yes", "lgpl-3.0": "yes", "mpl-2.0": "yes",
				},
			}

			result := map[string]any{
				"matrix": matrix,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractCopyrightTool() tool.Tool {
	return tool.NewBuilder("license_extract_copyright").
		WithDescription("Extract copyright information from license text").
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

			// Find copyright lines
			lines := strings.Split(params.Content, "\n")
			var copyrights []string
			for _, line := range lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "copyright") || strings.Contains(lower, "(c)") {
					copyrights = append(copyrights, strings.TrimSpace(line))
				}
			}

			// Extract years
			yearRe := regexp.MustCompile(`\d{4}`)
			var years []string
			for _, c := range copyrights {
				years = append(years, yearRe.FindAllString(c, -1)...)
			}

			result := map[string]any{
				"copyrights": copyrights,
				"years":      years,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func spdxParseTool() tool.Tool {
	return tool.NewBuilder("license_spdx_parse").
		WithDescription("Parse SPDX license expression").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expression string `json:"expression"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Simple SPDX expression parsing
			expr := params.Expression
			hasAnd := strings.Contains(expr, " AND ")
			hasOr := strings.Contains(expr, " OR ")
			hasWith := strings.Contains(expr, " WITH ")

			// Extract license IDs
			expr = strings.ReplaceAll(expr, "(", " ")
			expr = strings.ReplaceAll(expr, ")", " ")
			parts := strings.Fields(expr)

			var licenses []string
			var exceptions []string
			inWith := false
			for _, p := range parts {
				switch p {
				case "AND", "OR":
					inWith = false
				case "WITH":
					inWith = true
				default:
					if inWith {
						exceptions = append(exceptions, p)
					} else {
						licenses = append(licenses, p)
					}
				}
			}

			result := map[string]any{
				"expression":      params.Expression,
				"licenses":        licenses,
				"exceptions":      exceptions,
				"has_and":         hasAnd,
				"has_or":          hasOr,
				"has_exception":   hasWith,
				"is_compound":     hasAnd || hasOr,
				"requires_choice": hasOr,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func checkDependenciesTool() tool.Tool {
	return tool.NewBuilder("license_check_dependencies").
		WithDescription("Check license compatibility of dependencies").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ProjectLicense string `json:"project_license"`
				Dependencies   []struct {
					Name    string `json:"name"`
					License string `json:"license"`
				} `json:"dependencies"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			projectL := strings.ToLower(params.ProjectLicense)

			copyleft := map[string]bool{
				"gpl-2.0": true, "gpl-3.0": true, "agpl-3.0": true,
			}

			var compatible []map[string]any
			var incompatible []map[string]any

			for _, dep := range params.Dependencies {
				depL := strings.ToLower(dep.License)
				isCompatible := true
				reason := "compatible"

				if copyleft[depL] && !copyleft[projectL] {
					isCompatible = false
					reason = "copyleft dependency requires project to use same license"
				} else if depL == "agpl-3.0" && projectL != "agpl-3.0" {
					isCompatible = false
					reason = "AGPL requires all derivative works to be AGPL"
				}

				entry := map[string]any{
					"name":       dep.Name,
					"license":    dep.License,
					"compatible": isCompatible,
					"reason":     reason,
				}

				if isCompatible {
					compatible = append(compatible, entry)
				} else {
					incompatible = append(incompatible, entry)
				}
			}

			result := map[string]any{
				"project_license": params.ProjectLicense,
				"compatible":      compatible,
				"incompatible":    incompatible,
				"all_compatible":  len(incompatible) == 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// License text generators
func generateMIT(name string, year int) string {
	return `MIT License

Copyright (c) ` + formatYear(year) + ` ` + name + `

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`
}

func generateApache2(name string, year int) string {
	return `                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   Copyright ` + formatYear(year) + ` ` + name + `

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.`
}

func generateISC(name string, year int) string {
	return `ISC License

Copyright (c) ` + formatYear(year) + ` ` + name + `

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.`
}

func generateBSD3(name string, year int) string {
	return `BSD 3-Clause License

Copyright (c) ` + formatYear(year) + `, ` + name + `
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its
   contributors may be used to endorse or promote products derived from
   this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.`
}

func formatYear(year int) string {
	b, _ := json.Marshal(year)
	return string(b)
}

func generateUnlicense() string {
	return `This is free and unencumbered software released into the public domain.

Anyone is free to copy, modify, publish, use, compile, sell, or
distribute this software, either in source code form or as a compiled
binary, for any purpose, commercial or non-commercial, and by any
means.

In jurisdictions that recognize copyright laws, the author or authors
of this software dedicate any and all copyright interest in the
software to the public domain. We make this dedication for the benefit
of the public at large and to the detriment of our heirs and
successors. We intend this dedication to be an overt act of
relinquishment in perpetuity of all present and future rights to this
software under copyright law.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
OTHER DEALINGS IN THE SOFTWARE.

For more information, please refer to <https://unlicense.org>`
}
