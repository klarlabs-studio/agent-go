// Package creditcard provides credit card utilities for agents.
package creditcard

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the credit card utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("creditcard").
		WithDescription("Credit card utilities").
		AddTools(
			validateTool(),
			detectTypeTool(),
			maskTool(),
			formatTool(),
			parseTool(),
			luhnCheckTool(),
			expiryValidateTool(),
			generateTestTool(),
			binLookupTool(),
			sanitizeTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

// Card types and their patterns
type CardType struct {
	Name     string
	Pattern  *regexp.Regexp
	Lengths  []int
	CVVLen   int
	TestCard string // Test card number for sandbox
}

var cardTypes = []CardType{
	{
		Name:     "visa",
		Pattern:  regexp.MustCompile(`^4`),
		Lengths:  []int{13, 16, 19},
		CVVLen:   3,
		TestCard: "4111111111111111",
	},
	{
		Name:     "mastercard",
		Pattern:  regexp.MustCompile(`^(5[1-5]|2[2-7])`),
		Lengths:  []int{16},
		CVVLen:   3,
		TestCard: "5555555555554444",
	},
	{
		Name:     "amex",
		Pattern:  regexp.MustCompile(`^3[47]`),
		Lengths:  []int{15},
		CVVLen:   4,
		TestCard: "378282246310005",
	},
	{
		Name:     "discover",
		Pattern:  regexp.MustCompile(`^(6011|65|64[4-9])`),
		Lengths:  []int{16, 19},
		CVVLen:   3,
		TestCard: "6011111111111117",
	},
	{
		Name:     "diners",
		Pattern:  regexp.MustCompile(`^(36|30[0-5])`),
		Lengths:  []int{14, 16, 19},
		CVVLen:   3,
		TestCard: "30569309025904",
	},
	{
		Name:     "jcb",
		Pattern:  regexp.MustCompile(`^35(2[89]|[3-8])`),
		Lengths:  []int{16, 17, 18, 19},
		CVVLen:   3,
		TestCard: "3530111333300000",
	},
	{
		Name:     "unionpay",
		Pattern:  regexp.MustCompile(`^62`),
		Lengths:  []int{16, 17, 18, 19},
		CVVLen:   3,
		TestCard: "6200000000000005",
	},
}

func validateTool() tool.Tool {
	return tool.NewBuilder("creditcard_validate").
		WithDescription("Validate credit card number").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number string `json:"number"`
				CVV    string `json:"cvv,omitempty"`
				Expiry string `json:"expiry,omitempty"` // MM/YY or MM/YYYY
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			number := sanitizeNumber(params.Number)
			var errors []string
			valid := true

			// Check Luhn
			if !luhnValid(number) {
				errors = append(errors, "failed Luhn check")
				valid = false
			}

			// Detect type
			cardType := detectType(number)
			if cardType == nil {
				errors = append(errors, "unknown card type")
				valid = false
			} else {
				// Check length
				validLength := false
				for _, l := range cardType.Lengths {
					if len(number) == l {
						validLength = true
						break
					}
				}
				if !validLength {
					errors = append(errors, "invalid length for "+cardType.Name)
					valid = false
				}

				// Check CVV
				if params.CVV != "" {
					if len(params.CVV) != cardType.CVVLen {
						errors = append(errors, "invalid CVV length")
						valid = false
					}
					if !regexp.MustCompile(`^\d+$`).MatchString(params.CVV) {
						errors = append(errors, "CVV must be numeric")
						valid = false
					}
				}
			}

			// Check expiry
			if params.Expiry != "" {
				expiryValid, expiryError := validateExpiry(params.Expiry)
				if !expiryValid {
					errors = append(errors, expiryError)
					valid = false
				}
			}

			result := map[string]any{
				"valid": valid,
			}
			if cardType != nil {
				result["card_type"] = cardType.Name
			}
			if len(errors) > 0 {
				result["errors"] = errors
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sanitizeNumber(number string) string {
	return regexp.MustCompile(`\D`).ReplaceAllString(number, "")
}

func luhnValid(number string) bool {
	if len(number) == 0 {
		return false
	}

	sum := 0
	double := false

	for i := len(number) - 1; i >= 0; i-- {
		digit, err := strconv.Atoi(string(number[i]))
		if err != nil {
			return false
		}

		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
		double = !double
	}

	return sum%10 == 0
}

func detectType(number string) *CardType {
	for i := range cardTypes {
		if cardTypes[i].Pattern.MatchString(number) {
			return &cardTypes[i]
		}
	}
	return nil
}

func validateExpiry(expiry string) (bool, string) {
	// Parse MM/YY or MM/YYYY
	parts := strings.Split(expiry, "/")
	if len(parts) != 2 {
		return false, "invalid expiry format"
	}

	month, err := strconv.Atoi(parts[0])
	if err != nil || month < 1 || month > 12 {
		return false, "invalid month"
	}

	year, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, "invalid year"
	}

	// Convert 2-digit year to 4-digit
	if year < 100 {
		year += 2000
	}

	now := time.Now()
	expiryDate := time.Date(year, time.Month(month)+1, 0, 23, 59, 59, 0, time.UTC)

	if expiryDate.Before(now) {
		return false, "card expired"
	}

	return true, ""
}

func detectTypeTool() tool.Tool {
	return tool.NewBuilder("creditcard_detect_type").
		WithDescription("Detect credit card type from number").
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

			number := sanitizeNumber(params.Number)
			cardType := detectType(number)

			if cardType != nil {
				result := map[string]any{
					"type":          cardType.Name,
					"cvv_length":    cardType.CVVLen,
					"valid_lengths": cardType.Lengths,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"type":  "unknown",
				"error": "could not detect card type",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func maskTool() tool.Tool {
	return tool.NewBuilder("creditcard_mask").
		WithDescription("Mask credit card number for display").
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

			number := sanitizeNumber(params.Number)

			showLast := params.ShowLast
			if showLast <= 0 {
				showLast = 4
			}

			maskChar := params.MaskChar
			if maskChar == "" {
				maskChar = "*"
			}

			if len(number) <= showLast {
				result := map[string]any{
					"masked": number,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			masked := strings.Repeat(maskChar, len(number)-showLast) + number[len(number)-showLast:]

			// Format with spaces
			formatted := formatNumber(masked)

			result := map[string]any{
				"masked":           masked,
				"masked_formatted": formatted,
				"last_digits":      number[len(number)-showLast:],
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatNumber(number string) string {
	var formatted strings.Builder
	for i, ch := range number {
		if i > 0 && i%4 == 0 {
			formatted.WriteString(" ")
		}
		formatted.WriteRune(ch)
	}
	return formatted.String()
}

func formatTool() tool.Tool {
	return tool.NewBuilder("creditcard_format").
		WithDescription("Format credit card number").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number    string `json:"number"`
				Separator string `json:"separator,omitempty"` // default space
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			number := sanitizeNumber(params.Number)
			cardType := detectType(number)

			separator := params.Separator
			if separator == "" {
				separator = " "
			}

			// Format based on card type
			var formatted string
			if cardType != nil && cardType.Name == "amex" {
				// AMEX: 4-6-5
				if len(number) >= 15 {
					formatted = number[:4] + separator + number[4:10] + separator + number[10:]
				} else {
					formatted = formatWithSeparator(number, separator, 4)
				}
			} else {
				// Standard: 4-4-4-4
				formatted = formatWithSeparator(number, separator, 4)
			}

			result := map[string]any{
				"original":  params.Number,
				"formatted": formatted,
			}
			if cardType != nil {
				result["card_type"] = cardType.Name
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func formatWithSeparator(number, separator string, groupSize int) string {
	var formatted strings.Builder
	for i, ch := range number {
		if i > 0 && i%groupSize == 0 {
			formatted.WriteString(separator)
		}
		formatted.WriteRune(ch)
	}
	return formatted.String()
}

func parseTool() tool.Tool {
	return tool.NewBuilder("creditcard_parse").
		WithDescription("Parse credit card details").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Number string `json:"number"`
				Expiry string `json:"expiry,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			number := sanitizeNumber(params.Number)
			cardType := detectType(number)

			result := map[string]any{
				"number":     number,
				"length":     len(number),
				"luhn_valid": luhnValid(number),
			}

			if cardType != nil {
				result["card_type"] = cardType.Name
				result["issuer"] = getIssuerName(cardType.Name)
			}

			if len(number) >= 6 {
				result["bin"] = number[:6]
				result["last_four"] = number[len(number)-4:]
			}

			if params.Expiry != "" {
				parts := strings.Split(params.Expiry, "/")
				if len(parts) == 2 {
					result["expiry_month"] = parts[0]
					result["expiry_year"] = parts[1]
				}
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func getIssuerName(cardType string) string {
	issuers := map[string]string{
		"visa":       "Visa Inc.",
		"mastercard": "Mastercard Inc.",
		"amex":       "American Express",
		"discover":   "Discover Financial",
		"diners":     "Diners Club International",
		"jcb":        "JCB Co., Ltd.",
		"unionpay":   "China UnionPay",
	}
	return issuers[cardType]
}

func luhnCheckTool() tool.Tool {
	return tool.NewBuilder("creditcard_luhn_check").
		WithDescription("Perform Luhn algorithm check").
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

			number := sanitizeNumber(params.Number)
			valid := luhnValid(number)

			// Calculate check digit
			checkDigit := ""
			if len(number) > 0 {
				// Remove last digit and calculate what it should be
				withoutCheck := number[:len(number)-1]
				for d := 0; d <= 9; d++ {
					test := withoutCheck + strconv.Itoa(d)
					if luhnValid(test) {
						checkDigit = strconv.Itoa(d)
						break
					}
				}
			}

			result := map[string]any{
				"number":         number,
				"valid":          valid,
				"expected_check": checkDigit,
				"actual_check":   string(number[len(number)-1]),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func expiryValidateTool() tool.Tool {
	return tool.NewBuilder("creditcard_expiry_validate").
		WithDescription("Validate expiry date").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Expiry string `json:"expiry"` // MM/YY or MM/YYYY
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			valid, errMsg := validateExpiry(params.Expiry)

			result := map[string]any{
				"expiry": params.Expiry,
				"valid":  valid,
			}
			if errMsg != "" {
				result["error"] = errMsg
			}

			// Parse and show details
			parts := strings.Split(params.Expiry, "/")
			if len(parts) == 2 {
				month, _ := strconv.Atoi(parts[0])
				year, _ := strconv.Atoi(parts[1])
				if year < 100 {
					year += 2000
				}

				expiryDate := time.Date(year, time.Month(month)+1, 0, 23, 59, 59, 0, time.UTC)
				result["expiry_date"] = expiryDate.Format("2006-01-02")
				result["days_until_expiry"] = int(time.Until(expiryDate).Hours() / 24)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generateTestTool() tool.Tool {
	return tool.NewBuilder("creditcard_generate_test").
		WithDescription("Generate test credit card numbers").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Type string `json:"type,omitempty"` // visa, mastercard, amex, discover
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Type != "" {
				for _, ct := range cardTypes {
					if ct.Name == strings.ToLower(params.Type) {
						result := map[string]any{
							"type":        ct.Name,
							"test_number": ct.TestCard,
							"cvv_length":  ct.CVVLen,
							"note":        "This is a test card number for sandbox environments only",
						}
						output, _ := json.Marshal(result)
						return tool.Result{Output: output}, nil
					}
				}
				result := map[string]any{
					"error": "unknown card type",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			// Return all test cards
			testCards := make([]map[string]any, 0)
			for _, ct := range cardTypes {
				testCards = append(testCards, map[string]any{
					"type":        ct.Name,
					"test_number": ct.TestCard,
					"cvv_length":  ct.CVVLen,
				})
			}

			result := map[string]any{
				"test_cards": testCards,
				"note":       "These are test card numbers for sandbox environments only",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func binLookupTool() tool.Tool {
	return tool.NewBuilder("creditcard_bin_lookup").
		WithDescription("Look up card details from BIN").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				BIN string `json:"bin"` // First 6 digits
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			bin := sanitizeNumber(params.BIN)
			if len(bin) < 6 {
				result := map[string]any{
					"error": "BIN must be at least 6 digits",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}
			bin = bin[:6]

			// Detect type from BIN
			cardType := detectType(bin)

			result := map[string]any{
				"bin": bin,
			}

			if cardType != nil {
				result["card_type"] = cardType.Name
				result["issuer"] = getIssuerName(cardType.Name)
				result["cvv_length"] = cardType.CVVLen
			} else {
				result["card_type"] = "unknown"
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func sanitizeTool() tool.Tool {
	return tool.NewBuilder("creditcard_sanitize").
		WithDescription("Sanitize credit card input").
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

			sanitized := sanitizeNumber(params.Number)

			result := map[string]any{
				"original":   params.Number,
				"sanitized":  sanitized,
				"length":     len(sanitized),
				"is_numeric": len(sanitized) > 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
