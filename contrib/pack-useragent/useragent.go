// Package useragent provides user agent parsing utilities for agents.
package useragent

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the user agent utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("useragent").
		WithDescription("User agent parsing utilities").
		AddTools(
			parseTool(),
			detectBrowserTool(),
			detectOSTool(),
			detectDeviceTool(),
			isBotTool(),
			isMobileTool(),
			buildTool(),
			compareTool(),
			categoryTool(),
			commonListTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Browser patterns
var browserPatterns = []struct {
	Name    string
	Pattern *regexp.Regexp
	Version *regexp.Regexp
}{
	{"Chrome", regexp.MustCompile(`Chrome/`), regexp.MustCompile(`Chrome/(\d+\.?\d*\.?\d*\.?\d*)`)},
	{"Firefox", regexp.MustCompile(`Firefox/`), regexp.MustCompile(`Firefox/(\d+\.?\d*\.?\d*)`)},
	{"Safari", regexp.MustCompile(`Safari/.*Version/`), regexp.MustCompile(`Version/(\d+\.?\d*\.?\d*)`)},
	{"Edge", regexp.MustCompile(`Edg/`), regexp.MustCompile(`Edg/(\d+\.?\d*\.?\d*)`)},
	{"Opera", regexp.MustCompile(`OPR/`), regexp.MustCompile(`OPR/(\d+\.?\d*\.?\d*)`)},
	{"IE", regexp.MustCompile(`MSIE|Trident`), regexp.MustCompile(`(?:MSIE |rv:)(\d+\.?\d*)`)},
}

// OS patterns
var osPatterns = []struct {
	Name    string
	Pattern *regexp.Regexp
	Version *regexp.Regexp
}{
	{"Windows", regexp.MustCompile(`Windows`), regexp.MustCompile(`Windows NT (\d+\.?\d*)`)},
	{"macOS", regexp.MustCompile(`Mac OS X`), regexp.MustCompile(`Mac OS X (\d+[._]\d+[._]?\d*)`)},
	{"iOS", regexp.MustCompile(`iPhone|iPad|iPod`), regexp.MustCompile(`OS (\d+[._]\d+[._]?\d*)`)},
	{"Android", regexp.MustCompile(`Android`), regexp.MustCompile(`Android (\d+\.?\d*\.?\d*)`)},
	{"Linux", regexp.MustCompile(`Linux`), nil},
	{"Chrome OS", regexp.MustCompile(`CrOS`), nil},
}

// Device patterns
var devicePatterns = []struct {
	Type    string
	Pattern *regexp.Regexp
}{
	{"tablet", regexp.MustCompile(`(?i)iPad|Tablet`)},
	{"mobile", regexp.MustCompile(`(?i)Mobile|iPhone|iPod|Android.*Mobile|Windows Phone`)},
	{"tv", regexp.MustCompile(`(?i)Smart-?TV|BRAVIA|GoogleTV|Apple\s?TV`)},
	{"console", regexp.MustCompile(`(?i)PlayStation|Xbox|Nintendo`)},
}

// Bot patterns
var botPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)bot|crawl|spider|slurp|search|fetch|archive`),
	regexp.MustCompile(`(?i)google|bing|yahoo|baidu|yandex|duckduck`),
	regexp.MustCompile(`(?i)facebook|twitter|linkedin|pinterest`),
	regexp.MustCompile(`(?i)curl|wget|python|java|ruby|php|perl`),
}

func parseTool() tool.Tool {
	return tool.NewBuilder("useragent_parse").
		WithDescription("Parse user agent string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UserAgent string `json:"user_agent"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ua := params.UserAgent

			// Detect browser
			browser := ""
			browserVersion := ""
			for _, bp := range browserPatterns {
				if bp.Pattern.MatchString(ua) {
					browser = bp.Name
					if bp.Version != nil {
						if matches := bp.Version.FindStringSubmatch(ua); len(matches) > 1 {
							browserVersion = matches[1]
						}
					}
					break
				}
			}

			// Detect OS
			os := ""
			osVersion := ""
			for _, op := range osPatterns {
				if op.Pattern.MatchString(ua) {
					os = op.Name
					if op.Version != nil {
						if matches := op.Version.FindStringSubmatch(ua); len(matches) > 1 {
							osVersion = strings.ReplaceAll(matches[1], "_", ".")
						}
					}
					break
				}
			}

			// Detect device type
			deviceType := "desktop"
			for _, dp := range devicePatterns {
				if dp.Pattern.MatchString(ua) {
					deviceType = dp.Type
					break
				}
			}
			// Android without "Mobile" is typically a tablet
			if deviceType == "desktop" && strings.Contains(ua, "Android") && !strings.Contains(ua, "Mobile") {
				deviceType = "tablet"
			}

			// Check if bot
			isBot := false
			for _, bp := range botPatterns {
				if bp.MatchString(ua) {
					isBot = true
					break
				}
			}

			result := map[string]any{
				"user_agent":      ua,
				"browser":         browser,
				"browser_version": browserVersion,
				"os":              os,
				"os_version":      osVersion,
				"device_type":     deviceType,
				"is_bot":          isBot,
				"is_mobile":       deviceType == "mobile",
				"is_tablet":       deviceType == "tablet",
				"is_desktop":      deviceType == "desktop",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func detectBrowserTool() tool.Tool {
	return tool.NewBuilder("useragent_detect_browser").
		WithDescription("Detect browser from user agent").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UserAgent string `json:"user_agent"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ua := params.UserAgent

			for _, bp := range browserPatterns {
				if bp.Pattern.MatchString(ua) {
					version := ""
					majorVersion := ""
					if bp.Version != nil {
						if matches := bp.Version.FindStringSubmatch(ua); len(matches) > 1 {
							version = matches[1]
							parts := strings.Split(version, ".")
							if len(parts) > 0 {
								majorVersion = parts[0]
							}
						}
					}

					result := map[string]any{
						"browser":       bp.Name,
						"version":       version,
						"major_version": majorVersion,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]any{
				"browser": "unknown",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func detectOSTool() tool.Tool {
	return tool.NewBuilder("useragent_detect_os").
		WithDescription("Detect operating system from user agent").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UserAgent string `json:"user_agent"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ua := params.UserAgent

			for _, op := range osPatterns {
				if op.Pattern.MatchString(ua) {
					version := ""
					if op.Version != nil {
						if matches := op.Version.FindStringSubmatch(ua); len(matches) > 1 {
							version = strings.ReplaceAll(matches[1], "_", ".")
						}
					}

					// Map Windows versions
					osName := op.Name
					if osName == "Windows" && version != "" {
						osName = mapWindowsVersion(version)
					}

					result := map[string]any{
						"os":      osName,
						"version": version,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]any{
				"os": "unknown",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func mapWindowsVersion(version string) string {
	versions := map[string]string{
		"10.0": "Windows 10/11",
		"6.3":  "Windows 8.1",
		"6.2":  "Windows 8",
		"6.1":  "Windows 7",
		"6.0":  "Windows Vista",
		"5.1":  "Windows XP",
		"5.0":  "Windows 2000",
	}
	if name, ok := versions[version]; ok {
		return name
	}
	return "Windows"
}

func detectDeviceTool() tool.Tool {
	return tool.NewBuilder("useragent_detect_device").
		WithDescription("Detect device type from user agent").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UserAgent string `json:"user_agent"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ua := params.UserAgent

			deviceType := "desktop"
			deviceBrand := ""

			for _, dp := range devicePatterns {
				if dp.Pattern.MatchString(ua) {
					deviceType = dp.Type
					break
				}
			}

			// Detect brand
			brandPatterns := map[string]*regexp.Regexp{
				"Apple":    regexp.MustCompile(`(?i)iPhone|iPad|iPod|Macintosh`),
				"Samsung":  regexp.MustCompile(`(?i)Samsung|SM-|GT-`),
				"Google":   regexp.MustCompile(`(?i)Pixel|Nexus`),
				"Huawei":   regexp.MustCompile(`(?i)Huawei|HUAWEI`),
				"Xiaomi":   regexp.MustCompile(`(?i)Xiaomi|MI\s|Redmi`),
				"OnePlus":  regexp.MustCompile(`(?i)OnePlus`),
				"Sony":     regexp.MustCompile(`(?i)Sony|Xperia`),
				"LG":       regexp.MustCompile(`(?i)LG-|LG\s`),
				"HTC":      regexp.MustCompile(`(?i)HTC`),
				"Motorola": regexp.MustCompile(`(?i)Motorola|Moto\s`),
			}

			for brand, pattern := range brandPatterns {
				if pattern.MatchString(ua) {
					deviceBrand = brand
					break
				}
			}

			result := map[string]any{
				"device_type": deviceType,
				"brand":       deviceBrand,
				"is_mobile":   deviceType == "mobile",
				"is_tablet":   deviceType == "tablet",
				"is_desktop":  deviceType == "desktop",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isBotTool() tool.Tool {
	return tool.NewBuilder("useragent_is_bot").
		WithDescription("Check if user agent is a bot").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UserAgent string `json:"user_agent"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ua := params.UserAgent
			isBot := false
			botType := ""

			for _, bp := range botPatterns {
				if bp.MatchString(ua) {
					isBot = true
					break
				}
			}

			// Identify specific bots
			botNames := map[string]*regexp.Regexp{
				"Googlebot":   regexp.MustCompile(`(?i)Googlebot`),
				"Bingbot":     regexp.MustCompile(`(?i)bingbot`),
				"YahooBot":    regexp.MustCompile(`(?i)Slurp`),
				"BaiduSpider": regexp.MustCompile(`(?i)Baiduspider`),
				"DuckDuckBot": regexp.MustCompile(`(?i)DuckDuckBot`),
				"FacebookBot": regexp.MustCompile(`(?i)facebookexternalhit`),
				"TwitterBot":  regexp.MustCompile(`(?i)Twitterbot`),
				"curl":        regexp.MustCompile(`(?i)^curl/`),
				"wget":        regexp.MustCompile(`(?i)^Wget/`),
			}

			for name, pattern := range botNames {
				if pattern.MatchString(ua) {
					botType = name
					isBot = true
					break
				}
			}

			result := map[string]any{
				"is_bot":   isBot,
				"bot_type": botType,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func isMobileTool() tool.Tool {
	return tool.NewBuilder("useragent_is_mobile").
		WithDescription("Check if user agent is mobile").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UserAgent string `json:"user_agent"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ua := params.UserAgent

			isMobile := false
			isTablet := false

			for _, dp := range devicePatterns {
				if dp.Pattern.MatchString(ua) {
					if dp.Type == "mobile" {
						isMobile = true
					} else if dp.Type == "tablet" {
						isTablet = true
					}
					break
				}
			}

			result := map[string]any{
				"is_mobile":     isMobile,
				"is_tablet":     isTablet,
				"is_mobile_any": isMobile || isTablet,
				"is_desktop":    !isMobile && !isTablet,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func buildTool() tool.Tool {
	return tool.NewBuilder("useragent_build").
		WithDescription("Build a user agent string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Browser        string `json:"browser"`
				BrowserVersion string `json:"browser_version,omitempty"`
				OS             string `json:"os,omitempty"`
				OSVersion      string `json:"os_version,omitempty"`
				Device         string `json:"device,omitempty"` // desktop, mobile, tablet
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Build a realistic user agent
			var ua string

			os := params.OS
			if os == "" {
				os = "Windows"
			}
			osVersion := params.OSVersion
			if osVersion == "" {
				osVersion = "10.0"
			}

			browserVersion := params.BrowserVersion
			if browserVersion == "" {
				browserVersion = "120.0.0.0"
			}

			switch strings.ToLower(params.Browser) {
			case "chrome":
				if params.Device == "mobile" {
					ua = "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + browserVersion + " Mobile Safari/537.36"
				} else {
					ua = "Mozilla/5.0 (Windows NT " + osVersion + "; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + browserVersion + " Safari/537.36"
				}
			case "firefox":
				ua = "Mozilla/5.0 (Windows NT " + osVersion + "; Win64; x64; rv:" + browserVersion + ") Gecko/20100101 Firefox/" + browserVersion
			case "safari":
				ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X " + strings.ReplaceAll(osVersion, ".", "_") + ") AppleWebKit/537.36 (KHTML, like Gecko) Version/" + browserVersion + " Safari/537.36"
			case "edge":
				ua = "Mozilla/5.0 (Windows NT " + osVersion + "; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + browserVersion + " Safari/537.36 Edg/" + browserVersion
			default:
				ua = "Mozilla/5.0 (Windows NT " + osVersion + "; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + browserVersion + " Safari/537.36"
			}

			result := map[string]any{
				"user_agent": ua,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareTool() tool.Tool {
	return tool.NewBuilder("useragent_compare").
		WithDescription("Compare two user agents").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				A string `json:"a"`
				B string `json:"b"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Parse both
			parseUA := func(ua string) map[string]any {
				browser := ""
				browserVersion := ""
				for _, bp := range browserPatterns {
					if bp.Pattern.MatchString(ua) {
						browser = bp.Name
						if bp.Version != nil {
							if matches := bp.Version.FindStringSubmatch(ua); len(matches) > 1 {
								browserVersion = matches[1]
							}
						}
						break
					}
				}

				os := ""
				for _, op := range osPatterns {
					if op.Pattern.MatchString(ua) {
						os = op.Name
						break
					}
				}

				deviceType := "desktop"
				for _, dp := range devicePatterns {
					if dp.Pattern.MatchString(ua) {
						deviceType = dp.Type
						break
					}
				}

				return map[string]any{
					"browser":         browser,
					"browser_version": browserVersion,
					"os":              os,
					"device_type":     deviceType,
				}
			}

			infoA := parseUA(params.A)
			infoB := parseUA(params.B)

			sameBrowser := infoA["browser"] == infoB["browser"]
			sameOS := infoA["os"] == infoB["os"]
			sameDevice := infoA["device_type"] == infoB["device_type"]

			result := map[string]any{
				"a":            infoA,
				"b":            infoB,
				"same_browser": sameBrowser,
				"same_os":      sameOS,
				"same_device":  sameDevice,
				"identical":    params.A == params.B,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func categoryTool() tool.Tool {
	return tool.NewBuilder("useragent_category").
		WithDescription("Categorize user agent").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UserAgent string `json:"user_agent"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ua := params.UserAgent

			// Determine category
			category := "browser"
			subcategory := "desktop"

			// Check if bot
			for _, bp := range botPatterns {
				if bp.MatchString(ua) {
					category = "bot"
					switch {
					case regexp.MustCompile(`(?i)search|google|bing`).MatchString(ua):
						subcategory = "search_engine"
					case regexp.MustCompile(`(?i)social|facebook|twitter`).MatchString(ua):
						subcategory = "social"
					case regexp.MustCompile(`(?i)curl|wget|python`).MatchString(ua):
						subcategory = "tool"
					default:
						subcategory = "crawler"
					}
					break
				}
			}

			if category != "bot" {
				for _, dp := range devicePatterns {
					if dp.Pattern.MatchString(ua) {
						subcategory = dp.Type
						break
					}
				}
			}

			result := map[string]any{
				"category":    category,
				"subcategory": subcategory,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func commonListTool() tool.Tool {
	return tool.NewBuilder("useragent_common_list").
		WithDescription("Get list of common user agents").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Type string `json:"type,omitempty"` // desktop, mobile, bot
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			commonUA := map[string][]string{
				"desktop": {
					"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
					"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
					"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
					"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
				},
				"mobile": {
					"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
					"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
					"Mozilla/5.0 (Linux; Android 13; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
				},
				"bot": {
					"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
					"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
					"curl/8.4.0",
				},
			}

			if params.Type != "" {
				if uas, ok := commonUA[params.Type]; ok {
					result := map[string]any{
						"type":        params.Type,
						"user_agents": uas,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]any{
				"user_agents": commonUA,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
