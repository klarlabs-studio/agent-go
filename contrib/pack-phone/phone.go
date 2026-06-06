// Package phone provides phone number utilities for agents.
package phone

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the phone utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("phone").
		WithDescription("Phone number utilities").
		AddTools(
			parseTool(),
			validateTool(),
			formatTool(),
			normalizeTool(),
			countryCodeTool(),
			typeTool(),
			compareTool(),
			maskTool(),
			extractTool(),
			generateTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Common country calling codes
var countryCodes = map[string]string{
	"US": "1", "CA": "1",
	"GB": "44", "UK": "44",
	"DE": "49",
	"FR": "33",
	"IT": "39",
	"ES": "34",
	"AU": "61",
	"JP": "81",
	"CN": "86",
	"IN": "91",
	"BR": "55",
	"MX": "52",
	"RU": "7",
	"KR": "82",
}

// Reverse lookup
var codeToCountry = map[string]string{
	"1":  "US/CA",
	"44": "GB",
	"49": "DE",
	"33": "FR",
	"39": "IT",
	"34": "ES",
	"61": "AU",
	"81": "JP",
	"86": "CN",
	"91": "IN",
	"55": "BR",
	"52": "MX",
	"7":  "RU",
	"82": "KR",
}

func parseTool() tool.Tool {
	return tool.NewBuilder("phone_parse").
		WithDescription("Parse phone number into components").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number  string `json:"number"`
				Country string `json:"country,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			number := params.Number
			// Remove all non-digit characters except +
			cleaned := regexp.MustCompile(`[^\d+]`).ReplaceAllString(number, "")

			hasPlus := strings.HasPrefix(cleaned, "+")
			digits := strings.TrimPrefix(cleaned, "+")

			var countryCode, nationalNumber string
			var country string

			// Try to detect country code
			if hasPlus && len(digits) > 0 {
				// Check 1-3 digit codes
				for i := 1; i <= 3 && i <= len(digits); i++ {
					if c, ok := codeToCountry[digits[:i]]; ok {
						countryCode = digits[:i]
						nationalNumber = digits[i:]
						country = c
						break
					}
				}
				if countryCode == "" {
					countryCode = digits[:1]
					nationalNumber = digits[1:]
				}
			} else if params.Country != "" {
				if code, ok := countryCodes[strings.ToUpper(params.Country)]; ok {
					countryCode = code
					nationalNumber = digits
					country = params.Country
				}
			} else {
				nationalNumber = digits
			}

			result := map[string]any{
				"original":        params.Number,
				"country_code":    countryCode,
				"national_number": nationalNumber,
				"digits_only":     digits,
				"has_plus":        hasPlus,
			}
			if country != "" {
				result["country"] = country
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("phone_validate").
		WithDescription("Validate phone number format").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number  string `json:"number"`
				Country string `json:"country,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			number := params.Number
			digits := regexp.MustCompile(`\d`).FindAllString(number, -1)
			digitCount := len(digits)

			var errors []string
			valid := true

			// Basic validation
			if digitCount < 7 {
				errors = append(errors, "too few digits (minimum 7)")
				valid = false
			}
			if digitCount > 15 {
				errors = append(errors, "too many digits (maximum 15)")
				valid = false
			}

			// Check for invalid characters
			invalidChars := regexp.MustCompile(`[^\d\s\-\(\)\+\.]`).FindAllString(number, -1)
			if len(invalidChars) > 0 {
				errors = append(errors, "contains invalid characters")
				valid = false
			}

			// Country-specific validation
			if params.Country != "" {
				switch strings.ToUpper(params.Country) {
				case "US", "CA":
					if digitCount != 10 && digitCount != 11 {
						errors = append(errors, "US/CA numbers should have 10 or 11 digits")
						valid = false
					}
				case "GB", "UK":
					if digitCount < 10 || digitCount > 11 {
						errors = append(errors, "UK numbers should have 10-11 digits")
						valid = false
					}
				}
			}

			result := map[string]any{
				"number":      params.Number,
				"valid":       valid,
				"digit_count": digitCount,
			}
			if len(errors) > 0 {
				result["errors"] = errors
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatTool() tool.Tool {
	return tool.NewBuilder("phone_format").
		WithDescription("Format phone number in various styles").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number  string `json:"number"`
				Format  string `json:"format,omitempty"` // e164, national, international, rfc3966
				Country string `json:"country,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			digits := regexp.MustCompile(`\d`).FindAllString(params.Number, -1)
			digitStr := strings.Join(digits, "")

			// Detect country code
			countryCode := ""
			nationalNum := digitStr
			if strings.HasPrefix(params.Number, "+") && len(digitStr) > 0 {
				for i := 1; i <= 3 && i <= len(digitStr); i++ {
					if _, ok := codeToCountry[digitStr[:i]]; ok {
						countryCode = digitStr[:i]
						nationalNum = digitStr[i:]
						break
					}
				}
			} else if params.Country != "" {
				if code, ok := countryCodes[strings.ToUpper(params.Country)]; ok {
					countryCode = code
				}
			}

			formats := make(map[string]string)

			// E.164 format: +12025551234
			if countryCode != "" {
				formats["e164"] = "+" + countryCode + nationalNum
			}

			// National format for US: (202) 555-1234
			if len(nationalNum) == 10 {
				formats["national"] = "(" + nationalNum[:3] + ") " + nationalNum[3:6] + "-" + nationalNum[6:]
			} else if len(nationalNum) == 7 {
				formats["national"] = nationalNum[:3] + "-" + nationalNum[3:]
			}

			// International format: +1 202-555-1234
			if countryCode != "" && len(nationalNum) >= 7 {
				if len(nationalNum) == 10 {
					formats["international"] = "+" + countryCode + " " + nationalNum[:3] + "-" + nationalNum[3:6] + "-" + nationalNum[6:]
				} else {
					formats["international"] = "+" + countryCode + " " + nationalNum
				}
			}

			// RFC 3966 format: tel:+1-202-555-1234
			if countryCode != "" && len(nationalNum) >= 7 {
				if len(nationalNum) == 10 {
					formats["rfc3966"] = "tel:+" + countryCode + "-" + nationalNum[:3] + "-" + nationalNum[3:6] + "-" + nationalNum[6:]
				} else {
					formats["rfc3966"] = "tel:+" + countryCode + "-" + nationalNum
				}
			}

			result := map[string]any{
				"original":        params.Number,
				"formats":         formats,
				"country_code":    countryCode,
				"national_number": nationalNum,
			}

			if params.Format != "" {
				if formatted, ok := formats[params.Format]; ok {
					result["formatted"] = formatted
				}
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func normalizeTool() tool.Tool {
	return tool.NewBuilder("phone_normalize").
		WithDescription("Normalize phone number to digits only").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number     string `json:"number"`
				KeepPlus   bool   `json:"keep_plus,omitempty"`
				AddCountry string `json:"add_country,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			hasPlus := strings.HasPrefix(params.Number, "+")
			digits := regexp.MustCompile(`\d`).FindAllString(params.Number, -1)
			normalized := strings.Join(digits, "")

			// Add country code if specified
			if params.AddCountry != "" {
				if code, ok := countryCodes[strings.ToUpper(params.AddCountry)]; ok {
					if !strings.HasPrefix(normalized, code) {
						normalized = code + normalized
						hasPlus = true
					}
				}
			}

			if params.KeepPlus && hasPlus {
				normalized = "+" + normalized
			}

			result := map[string]any{
				"original":   params.Number,
				"normalized": normalized,
				"length":     len(strings.Join(digits, "")),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func countryCodeTool() tool.Tool {
	return tool.NewBuilder("phone_country_code").
		WithDescription("Get or lookup country calling codes").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Country string `json:"country,omitempty"`
				Code    string `json:"code,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			result := make(map[string]any)

			if params.Country != "" {
				if code, ok := countryCodes[strings.ToUpper(params.Country)]; ok {
					result["country"] = strings.ToUpper(params.Country)
					result["code"] = code
					result["dial_format"] = "+" + code
				} else {
					result["error"] = "country not found"
					result["country"] = params.Country
				}
			} else if params.Code != "" {
				code := strings.TrimPrefix(params.Code, "+")
				if country, ok := codeToCountry[code]; ok {
					result["code"] = code
					result["country"] = country
					result["dial_format"] = "+" + code
				} else {
					result["error"] = "code not found"
					result["code"] = params.Code
				}
			} else {
				// Return all codes
				result["country_codes"] = countryCodes
				result["code_to_country"] = codeToCountry
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func typeTool() tool.Tool {
	return tool.NewBuilder("phone_type").
		WithDescription("Detect phone number type").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number  string `json:"number"`
				Country string `json:"country,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			digits := regexp.MustCompile(`\d`).FindAllString(params.Number, -1)
			digitStr := strings.Join(digits, "")

			phoneType := "unknown"
			confidence := "low"

			// US-specific detection
			country := strings.ToUpper(params.Country)
			if country == "US" || country == "CA" || country == "" {
				if len(digitStr) >= 10 {
					// Get last 10 digits
					last10 := digitStr
					if len(last10) > 10 {
						last10 = last10[len(last10)-10:]
					}
					areaCode := last10[:3]

					// Toll-free prefixes
					tollFree := []string{"800", "888", "877", "866", "855", "844", "833"}
					for _, tf := range tollFree {
						if areaCode == tf {
							phoneType = "toll_free"
							confidence = "high"
							break
						}
					}

					// Premium rate
					if areaCode == "900" {
						phoneType = "premium_rate"
						confidence = "high"
					}
				}
			}

			result := map[string]any{
				"number":     params.Number,
				"type":       phoneType,
				"confidence": confidence,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func compareTool() tool.Tool {
	return tool.NewBuilder("phone_compare").
		WithDescription("Compare two phone numbers for equality").
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

			digitsA := regexp.MustCompile(`\d`).FindAllString(params.A, -1)
			digitsB := regexp.MustCompile(`\d`).FindAllString(params.B, -1)

			strA := strings.Join(digitsA, "")
			strB := strings.Join(digitsB, "")

			// Exact match
			exactMatch := strA == strB

			// Suffix match (ignoring country code)
			suffixMatch := false
			if len(strA) >= 10 && len(strB) >= 10 {
				suffixA := strA
				suffixB := strB
				if len(suffixA) > 10 {
					suffixA = suffixA[len(suffixA)-10:]
				}
				if len(suffixB) > 10 {
					suffixB = suffixB[len(suffixB)-10:]
				}
				suffixMatch = suffixA == suffixB
			}

			result := map[string]any{
				"a":            params.A,
				"b":            params.B,
				"exact_match":  exactMatch,
				"suffix_match": suffixMatch,
				"match":        exactMatch || suffixMatch,
				"a_digits":     strA,
				"b_digits":     strB,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func maskTool() tool.Tool {
	return tool.NewBuilder("phone_mask").
		WithDescription("Mask phone number for privacy").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number   string `json:"number"`
				ShowLast int    `json:"show_last,omitempty"` // default 4
				MaskChar string `json:"mask_char,omitempty"` // default *
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			showLast := params.ShowLast
			if showLast <= 0 {
				showLast = 4
			}

			maskChar := params.MaskChar
			if maskChar == "" {
				maskChar = "*"
			}

			digits := regexp.MustCompile(`\d`).FindAllString(params.Number, -1)

			var masked strings.Builder
			digitIdx := 0

			for _, ch := range params.Number {
				if ch >= '0' && ch <= '9' {
					if digitIdx < len(digits)-showLast {
						masked.WriteString(maskChar)
					} else {
						masked.WriteRune(ch)
					}
					digitIdx++
				} else {
					masked.WriteRune(ch)
				}
			}

			result := map[string]any{
				"original": params.Number,
				"masked":   masked.String(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func extractTool() tool.Tool {
	return tool.NewBuilder("phone_extract").
		WithDescription("Extract phone numbers from text").
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

			// Various phone patterns
			patterns := []string{
				`\+?1?\s*\(?[2-9]\d{2}\)?[\s.-]*\d{3}[\s.-]*\d{4}`,
				`\+\d{1,3}[\s.-]?\d{1,14}`,
				`\b\d{3}[\s.-]\d{3}[\s.-]\d{4}\b`,
				`\b\d{10,11}\b`,
			}

			seen := make(map[string]bool)
			var phones []string

			for _, pattern := range patterns {
				re := regexp.MustCompile(pattern)
				matches := re.FindAllString(params.Text, -1)
				for _, match := range matches {
					// Normalize for dedup
					digits := regexp.MustCompile(`\d`).FindAllString(match, -1)
					key := strings.Join(digits, "")
					if len(key) >= 7 && !seen[key] {
						seen[key] = true
						phones = append(phones, match)
					}
				}
			}

			result := map[string]any{
				"phones": phones,
				"count":  len(phones),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateTool() tool.Tool {
	return tool.NewBuilder("phone_generate").
		WithDescription("Generate sample phone numbers").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Country string `json:"country,omitempty"`
				Count   int    `json:"count,omitempty"`
				Format  string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}
			if count > 10 {
				count = 10
			}

			country := strings.ToUpper(params.Country)
			if country == "" {
				country = "US"
			}

			// Generate fake numbers using 555 prefix (reserved for fiction)
			var phones []string
			for i := 0; i < count; i++ {
				var phone string
				switch country {
				case "US", "CA":
					// US format: +1 (555) 555-XXXX
					suffix := 1000 + i
					if params.Format == "e164" {
						phone = "+1555555" + strings.Repeat("0", 4-len(string(rune('0'+suffix%10)))) + string(rune('0'+suffix%10))
					} else {
						phone = "(555) 555-" + pad(suffix, 4)
					}
				case "GB", "UK":
					phone = "+44 20 5555 " + pad(1000+i, 4)
				default:
					if code, ok := countryCodes[country]; ok {
						phone = "+" + code + " 555 555 " + pad(1000+i, 4)
					} else {
						phone = "(555) 555-" + pad(1000+i, 4)
					}
				}
				phones = append(phones, phone)
			}

			result := map[string]any{
				"phones":  phones,
				"count":   len(phones),
				"country": country,
				"note":    "These are sample numbers using reserved prefixes",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func pad(n, width int) string {
	s := strings.Repeat("0", width) + string(rune('0'+n%10000))
	if n >= 1000 {
		s = string(rune('0'+n/1000%10)) + string(rune('0'+n/100%10)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
	}
	return s[len(s)-width:]
}
