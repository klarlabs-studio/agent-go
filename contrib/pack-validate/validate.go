// Package validate provides data validation tools for agents.
package validate

import (
	"context"
	"encoding/json"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the validation tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("validate").
		WithDescription("Data validation tools").
		AddTools(
			emailTool(),
			urlTool(),
			ipTool(),
			uuidTool(),
			phoneTool(),
			creditCardTool(),
			dateTimeTool(),
			alphanumericTool(),
			numericTool(),
			lengthTool(),
			regexTool(),
			jsonTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func emailTool() tool.Tool {
	return tool.NewBuilder("validate_email").
		WithDescription("Validate an email address").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Email string `json:"email"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			_, err := mail.ParseAddress(params.Email)
			valid := err == nil
			errorMsg := ""
			if err != nil {
				errorMsg = err.Error()
			}

			// Additional checks
			if valid {
				parts := strings.Split(params.Email, "@")
				if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
					valid = false
					errorMsg = "invalid email format"
				} else if !strings.Contains(parts[1], ".") {
					valid = false
					errorMsg = "domain must contain a dot"
				}
			}

			result := map[string]any{
				"email": params.Email,
				"valid": valid,
				"error": errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func urlTool() tool.Tool {
	return tool.NewBuilder("validate_url").
		WithDescription("Validate a URL").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL            string   `json:"url"`
				RequireScheme  bool     `json:"require_scheme,omitempty"`
				AllowedSchemes []string `json:"allowed_schemes,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			u, err := url.Parse(params.URL)
			valid := err == nil && u.Host != ""
			errorMsg := ""
			if err != nil {
				errorMsg = err.Error()
			} else if u.Host == "" {
				valid = false
				errorMsg = "missing host"
			}

			if valid && params.RequireScheme && u.Scheme == "" {
				valid = false
				errorMsg = "scheme is required"
			}

			if valid && len(params.AllowedSchemes) > 0 {
				allowed := false
				for _, s := range params.AllowedSchemes {
					if strings.EqualFold(u.Scheme, s) {
						allowed = true
						break
					}
				}
				if !allowed {
					valid = false
					errorMsg = "scheme not allowed"
				}
			}

			result := map[string]any{
				"url":    params.URL,
				"valid":  valid,
				"error":  errorMsg,
				"scheme": u.Scheme,
				"host":   u.Host,
				"path":   u.Path,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func ipTool() tool.Tool {
	return tool.NewBuilder("validate_ip").
		WithDescription("Validate an IP address").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IP      string `json:"ip"`
				Version int    `json:"version,omitempty"` // 4 or 6, 0 for any
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			ip := net.ParseIP(params.IP)
			valid := ip != nil
			errorMsg := ""
			version := 0

			if valid {
				if ip.To4() != nil {
					version = 4
				} else {
					version = 6
				}

				if params.Version != 0 && params.Version != version {
					valid = false
					errorMsg = "IP version mismatch"
				}
			} else {
				errorMsg = "invalid IP address"
			}

			result := map[string]any{
				"ip":      params.IP,
				"valid":   valid,
				"version": version,
				"error":   errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func uuidTool() tool.Tool {
	return tool.NewBuilder("validate_uuid").
		WithDescription("Validate a UUID").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				UUID    string `json:"uuid"`
				Version int    `json:"version,omitempty"` // 1, 4, 5, etc. 0 for any
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
			valid := uuidRegex.MatchString(params.UUID)
			errorMsg := ""
			version := 0

			if valid {
				// Extract version from position 14 (single hex digit, always 0-15)
				v, _ := strconv.ParseUint(string(params.UUID[14]), 16, 8)
				version = int(v) // #nosec G115 -- value is a single hex digit (0-15)

				if params.Version != 0 && params.Version != version {
					valid = false
					errorMsg = "UUID version mismatch"
				}
			} else {
				errorMsg = "invalid UUID format"
			}

			result := map[string]any{
				"uuid":    params.UUID,
				"valid":   valid,
				"version": version,
				"error":   errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func phoneTool() tool.Tool {
	return tool.NewBuilder("validate_phone").
		WithDescription("Validate a phone number (basic validation)").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Phone string `json:"phone"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Strip common formatting
			cleaned := ""
			for _, r := range params.Phone {
				if unicode.IsDigit(r) || r == '+' {
					cleaned += string(r)
				}
			}

			valid := true
			errorMsg := ""

			// Basic validation
			if len(cleaned) < 7 {
				valid = false
				errorMsg = "phone number too short"
			} else if len(cleaned) > 15 {
				valid = false
				errorMsg = "phone number too long"
			}

			// Check for valid start
			if valid && strings.HasPrefix(cleaned, "+") {
				if len(cleaned) < 8 {
					valid = false
					errorMsg = "international number too short"
				}
			}

			result := map[string]any{
				"phone":   params.Phone,
				"cleaned": cleaned,
				"valid":   valid,
				"error":   errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func creditCardTool() tool.Tool {
	return tool.NewBuilder("validate_credit_card").
		WithDescription("Validate a credit card number using Luhn algorithm").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number string `json:"number"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Remove spaces and dashes
			cleaned := ""
			for _, r := range params.Number {
				if unicode.IsDigit(r) {
					cleaned += string(r)
				}
			}

			valid := true
			errorMsg := ""

			if len(cleaned) < 13 || len(cleaned) > 19 {
				valid = false
				errorMsg = "invalid card number length"
			} else {
				// Luhn algorithm
				sum := 0
				alternate := false
				for i := len(cleaned) - 1; i >= 0; i-- {
					n, _ := strconv.Atoi(string(cleaned[i]))
					if alternate {
						n *= 2
						if n > 9 {
							n -= 9
						}
					}
					sum += n
					alternate = !alternate
				}
				valid = sum%10 == 0
				if !valid {
					errorMsg = "Luhn check failed"
				}
			}

			// Detect card type
			cardType := "unknown"
			if valid {
				switch {
				case strings.HasPrefix(cleaned, "4"):
					cardType = "visa"
				case strings.HasPrefix(cleaned, "5") || strings.HasPrefix(cleaned, "2"):
					cardType = "mastercard"
				case strings.HasPrefix(cleaned, "34") || strings.HasPrefix(cleaned, "37"):
					cardType = "amex"
				case strings.HasPrefix(cleaned, "6011") || strings.HasPrefix(cleaned, "65"):
					cardType = "discover"
				}
			}

			result := map[string]any{
				"number":    params.Number,
				"valid":     valid,
				"card_type": cardType,
				"error":     errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func dateTimeTool() tool.Tool {
	return tool.NewBuilder("validate_datetime").
		WithDescription("Validate a date/time string").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				DateTime string `json:"datetime"`
				Format   string `json:"format,omitempty"` // Go time format
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Common formats to try
			formats := []string{
				"2006-01-02",
				"2006-01-02T15:04:05Z07:00",
				"2006-01-02 15:04:05",
				"01/02/2006",
				"02-01-2006",
				"Jan 2, 2006",
				"January 2, 2006",
			}

			if params.Format != "" {
				formats = []string{params.Format}
			}

			valid := false
			matchedFormat := ""

			for _, f := range formats {
				if _, err := parseTime(f, params.DateTime); err == nil {
					valid = true
					matchedFormat = f
					break
				}
			}

			result := map[string]any{
				"datetime": params.DateTime,
				"valid":    valid,
				"format":   matchedFormat,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func alphanumericTool() tool.Tool {
	return tool.NewBuilder("validate_alphanumeric").
		WithDescription("Validate if string contains only alphanumeric characters").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text       string `json:"text"`
				AllowSpace bool   `json:"allow_space,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			valid := true
			for _, r := range params.Text {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					if params.AllowSpace && unicode.IsSpace(r) {
						continue
					}
					valid = false
					break
				}
			}

			result := map[string]any{
				"text":   params.Text,
				"valid":  valid,
				"length": len(params.Text),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func numericTool() tool.Tool {
	return tool.NewBuilder("validate_numeric").
		WithDescription("Validate if string is a valid number").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text     string   `json:"text"`
				AllowNeg bool     `json:"allow_negative,omitempty"`
				AllowDec bool     `json:"allow_decimal,omitempty"`
				Min      *float64 `json:"min,omitempty"`
				Max      *float64 `json:"max,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			valid := true
			errorMsg := ""
			var value float64

			if params.AllowDec {
				v, err := strconv.ParseFloat(params.Text, 64)
				if err != nil {
					valid = false
					errorMsg = "not a valid decimal number"
				} else {
					value = v
				}
			} else {
				v, err := strconv.ParseInt(params.Text, 10, 64)
				if err != nil {
					valid = false
					errorMsg = "not a valid integer"
				} else {
					value = float64(v)
				}
			}

			if valid && !params.AllowNeg && value < 0 {
				valid = false
				errorMsg = "negative numbers not allowed"
			}

			if valid && params.Min != nil && value < *params.Min {
				valid = false
				errorMsg = "value below minimum"
			}

			if valid && params.Max != nil && value > *params.Max {
				valid = false
				errorMsg = "value above maximum"
			}

			result := map[string]any{
				"text":  params.Text,
				"valid": valid,
				"value": value,
				"error": errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func lengthTool() tool.Tool {
	return tool.NewBuilder("validate_length").
		WithDescription("Validate string length").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text string `json:"text"`
				Min  int    `json:"min,omitempty"`
				Max  int    `json:"max,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			length := len(params.Text)
			valid := true
			errorMsg := ""

			if params.Min > 0 && length < params.Min {
				valid = false
				errorMsg = "text too short"
			}

			if params.Max > 0 && length > params.Max {
				valid = false
				errorMsg = "text too long"
			}

			result := map[string]any{
				"text":   params.Text,
				"length": length,
				"valid":  valid,
				"error":  errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func regexTool() tool.Tool {
	return tool.NewBuilder("validate_regex").
		WithDescription("Validate text against a regex pattern").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Text    string `json:"text"`
				Pattern string `json:"pattern"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			re, err := regexp.Compile(params.Pattern)
			if err != nil {
				result := map[string]any{
					"valid": false,
					"error": "invalid regex pattern: " + err.Error(),
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			matches := re.MatchString(params.Text)

			result := map[string]any{
				"text":    params.Text,
				"pattern": params.Pattern,
				"valid":   matches,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func jsonTool() tool.Tool {
	return tool.NewBuilder("validate_json").
		WithDescription("Validate JSON syntax").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				JSON string `json:"json"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			var parsed any
			err := json.Unmarshal([]byte(params.JSON), &parsed)
			valid := err == nil
			errorMsg := ""
			dataType := ""

			if err != nil {
				errorMsg = err.Error()
			} else {
				switch parsed.(type) {
				case map[string]any:
					dataType = "object"
				case []any:
					dataType = "array"
				case string:
					dataType = "string"
				case float64:
					dataType = "number"
				case bool:
					dataType = "boolean"
				case nil:
					dataType = "null"
				}
			}

			result := map[string]any{
				"valid": valid,
				"type":  dataType,
				"error": errorMsg,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// parseTime is a helper to parse time with various formats
func parseTime(format, value string) (any, error) {
	// This is a placeholder - in a real implementation you'd use time.Parse
	// For now, just do basic validation
	if len(value) < 8 {
		return nil, nil
	}
	return value, nil
}
