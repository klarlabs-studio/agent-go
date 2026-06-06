// Package country provides country and region utilities for agents.
package country

import (
	"context"
	"encoding/json"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the country utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("country").
		WithDescription("Country and region utilities").
		AddTools(
			lookupTool(),
			listTool(),
			byCodeTool(),
			byRegionTool(),
			currencyTool(),
			timezonesTool(),
			languagesTool(),
			neighborsTool(),
			searchTool(),
			validateCodeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Country represents country information
type Country struct {
	Name        string   `json:"name"`
	Alpha2      string   `json:"alpha2"`
	Alpha3      string   `json:"alpha3"`
	Numeric     string   `json:"numeric"`
	Region      string   `json:"region"`
	Subregion   string   `json:"subregion"`
	Capital     string   `json:"capital"`
	Currency    string   `json:"currency"`
	Languages   []string `json:"languages"`
	CallingCode string   `json:"calling_code"`
	Flag        string   `json:"flag"`
}

// Countries database (subset)
var countries = map[string]Country{
	"US": {
		Name: "United States", Alpha2: "US", Alpha3: "USA", Numeric: "840",
		Region: "Americas", Subregion: "Northern America", Capital: "Washington D.C.",
		Currency: "USD", Languages: []string{"en"}, CallingCode: "+1", Flag: "🇺🇸",
	},
	"GB": {
		Name: "United Kingdom", Alpha2: "GB", Alpha3: "GBR", Numeric: "826",
		Region: "Europe", Subregion: "Northern Europe", Capital: "London",
		Currency: "GBP", Languages: []string{"en"}, CallingCode: "+44", Flag: "🇬🇧",
	},
	"DE": {
		Name: "Germany", Alpha2: "DE", Alpha3: "DEU", Numeric: "276",
		Region: "Europe", Subregion: "Western Europe", Capital: "Berlin",
		Currency: "EUR", Languages: []string{"de"}, CallingCode: "+49", Flag: "🇩🇪",
	},
	"FR": {
		Name: "France", Alpha2: "FR", Alpha3: "FRA", Numeric: "250",
		Region: "Europe", Subregion: "Western Europe", Capital: "Paris",
		Currency: "EUR", Languages: []string{"fr"}, CallingCode: "+33", Flag: "🇫🇷",
	},
	"IT": {
		Name: "Italy", Alpha2: "IT", Alpha3: "ITA", Numeric: "380",
		Region: "Europe", Subregion: "Southern Europe", Capital: "Rome",
		Currency: "EUR", Languages: []string{"it"}, CallingCode: "+39", Flag: "🇮🇹",
	},
	"ES": {
		Name: "Spain", Alpha2: "ES", Alpha3: "ESP", Numeric: "724",
		Region: "Europe", Subregion: "Southern Europe", Capital: "Madrid",
		Currency: "EUR", Languages: []string{"es"}, CallingCode: "+34", Flag: "🇪🇸",
	},
	"CA": {
		Name: "Canada", Alpha2: "CA", Alpha3: "CAN", Numeric: "124",
		Region: "Americas", Subregion: "Northern America", Capital: "Ottawa",
		Currency: "CAD", Languages: []string{"en", "fr"}, CallingCode: "+1", Flag: "🇨🇦",
	},
	"AU": {
		Name: "Australia", Alpha2: "AU", Alpha3: "AUS", Numeric: "036",
		Region: "Oceania", Subregion: "Australia and New Zealand", Capital: "Canberra",
		Currency: "AUD", Languages: []string{"en"}, CallingCode: "+61", Flag: "🇦🇺",
	},
	"JP": {
		Name: "Japan", Alpha2: "JP", Alpha3: "JPN", Numeric: "392",
		Region: "Asia", Subregion: "Eastern Asia", Capital: "Tokyo",
		Currency: "JPY", Languages: []string{"ja"}, CallingCode: "+81", Flag: "🇯🇵",
	},
	"CN": {
		Name: "China", Alpha2: "CN", Alpha3: "CHN", Numeric: "156",
		Region: "Asia", Subregion: "Eastern Asia", Capital: "Beijing",
		Currency: "CNY", Languages: []string{"zh"}, CallingCode: "+86", Flag: "🇨🇳",
	},
	"IN": {
		Name: "India", Alpha2: "IN", Alpha3: "IND", Numeric: "356",
		Region: "Asia", Subregion: "Southern Asia", Capital: "New Delhi",
		Currency: "INR", Languages: []string{"hi", "en"}, CallingCode: "+91", Flag: "🇮🇳",
	},
	"BR": {
		Name: "Brazil", Alpha2: "BR", Alpha3: "BRA", Numeric: "076",
		Region: "Americas", Subregion: "South America", Capital: "Brasília",
		Currency: "BRL", Languages: []string{"pt"}, CallingCode: "+55", Flag: "🇧🇷",
	},
	"MX": {
		Name: "Mexico", Alpha2: "MX", Alpha3: "MEX", Numeric: "484",
		Region: "Americas", Subregion: "Central America", Capital: "Mexico City",
		Currency: "MXN", Languages: []string{"es"}, CallingCode: "+52", Flag: "🇲🇽",
	},
	"RU": {
		Name: "Russia", Alpha2: "RU", Alpha3: "RUS", Numeric: "643",
		Region: "Europe", Subregion: "Eastern Europe", Capital: "Moscow",
		Currency: "RUB", Languages: []string{"ru"}, CallingCode: "+7", Flag: "🇷🇺",
	},
	"KR": {
		Name: "South Korea", Alpha2: "KR", Alpha3: "KOR", Numeric: "410",
		Region: "Asia", Subregion: "Eastern Asia", Capital: "Seoul",
		Currency: "KRW", Languages: []string{"ko"}, CallingCode: "+82", Flag: "🇰🇷",
	},
	"NL": {
		Name: "Netherlands", Alpha2: "NL", Alpha3: "NLD", Numeric: "528",
		Region: "Europe", Subregion: "Western Europe", Capital: "Amsterdam",
		Currency: "EUR", Languages: []string{"nl"}, CallingCode: "+31", Flag: "🇳🇱",
	},
	"CH": {
		Name: "Switzerland", Alpha2: "CH", Alpha3: "CHE", Numeric: "756",
		Region: "Europe", Subregion: "Western Europe", Capital: "Bern",
		Currency: "CHF", Languages: []string{"de", "fr", "it", "rm"}, CallingCode: "+41", Flag: "🇨🇭",
	},
	"SE": {
		Name: "Sweden", Alpha2: "SE", Alpha3: "SWE", Numeric: "752",
		Region: "Europe", Subregion: "Northern Europe", Capital: "Stockholm",
		Currency: "SEK", Languages: []string{"sv"}, CallingCode: "+46", Flag: "🇸🇪",
	},
	"NO": {
		Name: "Norway", Alpha2: "NO", Alpha3: "NOR", Numeric: "578",
		Region: "Europe", Subregion: "Northern Europe", Capital: "Oslo",
		Currency: "NOK", Languages: []string{"no"}, CallingCode: "+47", Flag: "🇳🇴",
	},
	"DK": {
		Name: "Denmark", Alpha2: "DK", Alpha3: "DNK", Numeric: "208",
		Region: "Europe", Subregion: "Northern Europe", Capital: "Copenhagen",
		Currency: "DKK", Languages: []string{"da"}, CallingCode: "+45", Flag: "🇩🇰",
	},
}

// Alpha3 to Alpha2 mapping
var alpha3ToAlpha2 = make(map[string]string)

func init() {
	for alpha2, country := range countries {
		alpha3ToAlpha2[country.Alpha3] = alpha2
	}
}

func lookupTool() tool.Tool {
	return tool.NewBuilder("country_lookup").
		WithDescription("Look up country by code or name").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			query := strings.ToUpper(params.Query)

			// Try Alpha2
			if country, ok := countries[query]; ok {
				output, _ := json.Marshal(country)
				return tool.Result{Output: output}, nil
			}

			// Try Alpha3
			if alpha2, ok := alpha3ToAlpha2[query]; ok {
				output, _ := json.Marshal(countries[alpha2])
				return tool.Result{Output: output}, nil
			}

			// Try name search
			queryLower := strings.ToLower(params.Query)
			for _, country := range countries {
				if strings.ToLower(country.Name) == queryLower {
					output, _ := json.Marshal(country)
					return tool.Result{Output: output}, nil
				}
			}

			result := map[string]any{
				"error": "country not found",
				"query": params.Query,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("country_list").
		WithDescription("List all countries").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			countryList := make([]map[string]string, 0, len(countries))
			for _, country := range countries {
				countryList = append(countryList, map[string]string{
					"name":   country.Name,
					"alpha2": country.Alpha2,
					"alpha3": country.Alpha3,
					"flag":   country.Flag,
				})
			}

			result := map[string]any{
				"countries": countryList,
				"count":     len(countryList),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func byCodeTool() tool.Tool {
	return tool.NewBuilder("country_by_code").
		WithDescription("Get country by ISO code").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			code := strings.ToUpper(params.Code)

			// Try Alpha2
			if country, ok := countries[code]; ok {
				output, _ := json.Marshal(country)
				return tool.Result{Output: output}, nil
			}

			// Try Alpha3
			if alpha2, ok := alpha3ToAlpha2[code]; ok {
				output, _ := json.Marshal(countries[alpha2])
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"error": "country not found",
				"code":  params.Code,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func byRegionTool() tool.Tool {
	return tool.NewBuilder("country_by_region").
		WithDescription("Get countries by region").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Region    string `json:"region,omitempty"`
				Subregion string `json:"subregion,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var matches []Country
			for _, country := range countries {
				if params.Region != "" && strings.EqualFold(country.Region, params.Region) {
					if params.Subregion == "" || strings.EqualFold(country.Subregion, params.Subregion) {
						matches = append(matches, country)
					}
				} else if params.Subregion != "" && strings.EqualFold(country.Subregion, params.Subregion) {
					matches = append(matches, country)
				}
			}

			// Get unique regions
			regions := make(map[string][]string)
			for _, country := range countries {
				if _, ok := regions[country.Region]; !ok {
					regions[country.Region] = make([]string, 0)
				}
				found := false
				for _, s := range regions[country.Region] {
					if s == country.Subregion {
						found = true
						break
					}
				}
				if !found {
					regions[country.Region] = append(regions[country.Region], country.Subregion)
				}
			}

			if len(matches) > 0 {
				result := map[string]any{
					"countries": matches,
					"count":     len(matches),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"available_regions": regions,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func currencyTool() tool.Tool {
	return tool.NewBuilder("country_currency").
		WithDescription("Get countries by currency").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Currency string `json:"currency,omitempty"`
				Country  string `json:"country,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Country != "" {
				code := strings.ToUpper(params.Country)
				if country, ok := countries[code]; ok {
					result := map[string]any{
						"country":  country.Name,
						"currency": country.Currency,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			if params.Currency != "" {
				currency := strings.ToUpper(params.Currency)
				var matches []string
				for _, country := range countries {
					if country.Currency == currency {
						matches = append(matches, country.Name)
					}
				}
				result := map[string]any{
					"currency":  currency,
					"countries": matches,
					"count":     len(matches),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// List all currencies
			currencies := make(map[string][]string)
			for _, country := range countries {
				currencies[country.Currency] = append(currencies[country.Currency], country.Name)
			}
			result := map[string]any{
				"currencies": currencies,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func timezonesTool() tool.Tool {
	return tool.NewBuilder("country_timezones").
		WithDescription("Get common timezones for country").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Country string `json:"country"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Simplified timezone mapping
			timezones := map[string][]string{
				"US": {"America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles", "America/Anchorage", "Pacific/Honolulu"},
				"GB": {"Europe/London"},
				"DE": {"Europe/Berlin"},
				"FR": {"Europe/Paris"},
				"JP": {"Asia/Tokyo"},
				"CN": {"Asia/Shanghai"},
				"IN": {"Asia/Kolkata"},
				"AU": {"Australia/Sydney", "Australia/Melbourne", "Australia/Perth"},
				"BR": {"America/Sao_Paulo", "America/Manaus"},
				"CA": {"America/Toronto", "America/Vancouver", "America/Edmonton"},
				"RU": {"Europe/Moscow", "Asia/Vladivostok", "Asia/Yekaterinburg"},
			}

			code := strings.ToUpper(params.Country)
			if tzs, ok := timezones[code]; ok {
				result := map[string]any{
					"country":   code,
					"timezones": tzs,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Default to capital-based single timezone
			if country, ok := countries[code]; ok {
				result := map[string]any{
					"country": code,
					"note":    "timezone data not available, check capital: " + country.Capital,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"error": "country not found",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func languagesTool() tool.Tool {
	return tool.NewBuilder("country_languages").
		WithDescription("Get languages for country").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Country  string `json:"country,omitempty"`
				Language string `json:"language,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Country != "" {
				code := strings.ToUpper(params.Country)
				if country, ok := countries[code]; ok {
					result := map[string]any{
						"country":   country.Name,
						"languages": country.Languages,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
			}

			if params.Language != "" {
				lang := strings.ToLower(params.Language)
				var matches []string
				for _, country := range countries {
					for _, l := range country.Languages {
						if l == lang {
							matches = append(matches, country.Name)
							break
						}
					}
				}
				result := map[string]any{
					"language":  lang,
					"countries": matches,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"error": "specify country or language",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func neighborsTool() tool.Tool {
	return tool.NewBuilder("country_neighbors").
		WithDescription("Get neighboring countries").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Country string `json:"country"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Simplified neighbor mapping
			neighbors := map[string][]string{
				"US": {"CA", "MX"},
				"CA": {"US"},
				"MX": {"US"},
				"DE": {"FR", "NL", "CH", "DK"},
				"FR": {"DE", "IT", "ES", "CH"},
				"IT": {"FR", "CH"},
				"ES": {"FR"},
				"GB": {},
				"AU": {},
				"JP": {},
				"CN": {"RU", "IN", "KR"},
				"IN": {"CN"},
				"RU": {"CN", "NO"},
				"KR": {"CN"},
				"NL": {"DE"},
				"CH": {"DE", "FR", "IT"},
				"SE": {"NO", "DK"},
				"NO": {"SE", "RU"},
				"DK": {"DE", "SE"},
				"BR": {},
			}

			code := strings.ToUpper(params.Country)
			if neighborCodes, ok := neighbors[code]; ok {
				neighborCountries := make([]map[string]string, 0)
				for _, nc := range neighborCodes {
					if c, ok := countries[nc]; ok {
						neighborCountries = append(neighborCountries, map[string]string{
							"name":   c.Name,
							"alpha2": c.Alpha2,
							"flag":   c.Flag,
						})
					}
				}
				result := map[string]any{
					"country":   code,
					"neighbors": neighborCountries,
					"count":     len(neighborCountries),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"error": "country not found",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func searchTool() tool.Tool {
	return tool.NewBuilder("country_search").
		WithDescription("Search countries by name").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			query := strings.ToLower(params.Query)
			var matches []Country

			for _, country := range countries {
				if strings.Contains(strings.ToLower(country.Name), query) ||
					strings.Contains(strings.ToLower(country.Capital), query) {
					matches = append(matches, country)
				}
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

func validateCodeTool() tool.Tool {
	return tool.NewBuilder("country_validate_code").
		WithDescription("Validate country code").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			code := strings.ToUpper(params.Code)
			isAlpha2 := len(code) == 2
			isAlpha3 := len(code) == 3

			var valid bool
			var country *Country

			if isAlpha2 {
				if c, ok := countries[code]; ok {
					valid = true
					country = &c
				}
			} else if isAlpha3 {
				if alpha2, ok := alpha3ToAlpha2[code]; ok {
					valid = true
					c := countries[alpha2]
					country = &c
				}
			}

			result := map[string]any{
				"code":      params.Code,
				"valid":     valid,
				"is_alpha2": isAlpha2,
				"is_alpha3": isAlpha3,
			}
			if country != nil {
				result["country"] = country.Name
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
