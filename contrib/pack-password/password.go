// Package password provides password utilities for agents.
package password

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
	"unicode"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the password utilities pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("password").
		WithDescription("Password utilities").
		AddTools(
			generateTool(),
			strengthTool(),
			validateTool(),
			entropyTool(),
			passphraseTool(),
			commonCheckTool(),
			suggestTool(),
			maskTool(),
			requirementsTool(),
			hashInfoTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

const (
	lowercase    = "abcdefghijklmnopqrstuvwxyz"
	uppercase    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits       = "0123456789"
	symbols      = "!@#$%^&*()_+-=[]{}|;:,.<>?"
	ambiguous    = "l1IoO0"
	similarChars = "Il1O0"
)

// Common weak passwords (subset for checking)
var commonPasswords = map[string]bool{
	"password": true, "123456": true, "12345678": true, "qwerty": true,
	"abc123": true, "password1": true, "admin": true, "letmein": true,
	"welcome": true, "monkey": true, "dragon": true, "master": true,
	"login": true, "princess": true, "solo": true, "qwerty123": true,
	"passw0rd": true, "football": true, "iloveyou": true, "shadow": true,
	"sunshine": true, "12345": true, "123456789": true, "1234567890": true,
}

func generateTool() tool.Tool {
	return tool.NewBuilder("password_generate").
		WithDescription("Generate secure random password").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Length           int  `json:"length,omitempty"`
				IncludeUpper     bool `json:"include_upper,omitempty"`
				IncludeLower     bool `json:"include_lower,omitempty"`
				IncludeDigits    bool `json:"include_digits,omitempty"`
				IncludeSymbols   bool `json:"include_symbols,omitempty"`
				ExcludeAmbiguous bool `json:"exclude_ambiguous,omitempty"`
				Count            int  `json:"count,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			length := params.Length
			if length <= 0 {
				length = 16
			}
			if length > 128 {
				length = 128
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}
			if count > 10 {
				count = 10
			}

			// Default: all character types
			if !params.IncludeUpper && !params.IncludeLower && !params.IncludeDigits && !params.IncludeSymbols {
				params.IncludeUpper = true
				params.IncludeLower = true
				params.IncludeDigits = true
				params.IncludeSymbols = true
			}

			// Build character set
			var charset string
			if params.IncludeLower {
				charset += lowercase
			}
			if params.IncludeUpper {
				charset += uppercase
			}
			if params.IncludeDigits {
				charset += digits
			}
			if params.IncludeSymbols {
				charset += symbols
			}

			if params.ExcludeAmbiguous {
				for _, ch := range ambiguous {
					charset = strings.ReplaceAll(charset, string(ch), "")
				}
			}

			if len(charset) == 0 {
				result := map[string]any{"error": "no characters available for password generation"}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			passwords := make([]string, count)
			for i := 0; i < count; i++ {
				password, err := generatePassword(length, charset)
				if err != nil {
					result := map[string]any{"error": err.Error()}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
				passwords[i] = password
			}

			var resultPassword any
			if count == 1 {
				resultPassword = passwords[0]
			} else {
				resultPassword = passwords
			}

			result := map[string]any{
				"password":     resultPassword,
				"length":       length,
				"charset_size": len(charset),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func generatePassword(length int, charset string) (string, error) {
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = charset[idx.Int64()]
	}

	return string(result), nil
}

func strengthTool() tool.Tool {
	return tool.NewBuilder("password_strength").
		WithDescription("Analyze password strength").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Password string `json:"password"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			password := params.Password
			length := len(password)

			// Check character classes
			hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
			hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
			hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
			hasSymbol := regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password)

			// Count character classes
			charClasses := 0
			if hasLower {
				charClasses++
			}
			if hasUpper {
				charClasses++
			}
			if hasDigit {
				charClasses++
			}
			if hasSymbol {
				charClasses++
			}

			// Calculate score (0-100)
			score := 0

			// Length score (up to 40 points)
			if length >= 8 {
				score += 10
			}
			if length >= 12 {
				score += 10
			}
			if length >= 16 {
				score += 10
			}
			if length >= 20 {
				score += 10
			}

			// Character class score (up to 40 points)
			score += charClasses * 10

			// Bonus for mixing (up to 20 points)
			if length >= 12 && charClasses >= 3 {
				score += 10
			}
			if length >= 16 && charClasses >= 4 {
				score += 10
			}

			// Penalties
			if commonPasswords[strings.ToLower(password)] {
				score = 0
			}

			// Determine strength label
			var strength string
			switch {
			case score >= 80:
				strength = "very_strong"
			case score >= 60:
				strength = "strong"
			case score >= 40:
				strength = "medium"
			case score >= 20:
				strength = "weak"
			default:
				strength = "very_weak"
			}

			result := map[string]any{
				"score":             score,
				"strength":          strength,
				"length":            length,
				"has_lowercase":     hasLower,
				"has_uppercase":     hasUpper,
				"has_digits":        hasDigit,
				"has_symbols":       hasSymbol,
				"character_classes": charClasses,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func validateTool() tool.Tool {
	return tool.NewBuilder("password_validate").
		WithDescription("Validate password against requirements").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Password      string `json:"password"`
				MinLength     int    `json:"min_length,omitempty"`
				MaxLength     int    `json:"max_length,omitempty"`
				RequireUpper  bool   `json:"require_upper,omitempty"`
				RequireLower  bool   `json:"require_lower,omitempty"`
				RequireDigit  bool   `json:"require_digit,omitempty"`
				RequireSymbol bool   `json:"require_symbol,omitempty"`
				MinClasses    int    `json:"min_classes,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			password := params.Password
			var errors []string
			valid := true

			// Length checks
			if params.MinLength > 0 && len(password) < params.MinLength {
				errors = append(errors, "password too short")
				valid = false
			}
			if params.MaxLength > 0 && len(password) > params.MaxLength {
				errors = append(errors, "password too long")
				valid = false
			}

			// Character class checks
			hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
			hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
			hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
			hasSymbol := regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password)

			if params.RequireLower && !hasLower {
				errors = append(errors, "missing lowercase letter")
				valid = false
			}
			if params.RequireUpper && !hasUpper {
				errors = append(errors, "missing uppercase letter")
				valid = false
			}
			if params.RequireDigit && !hasDigit {
				errors = append(errors, "missing digit")
				valid = false
			}
			if params.RequireSymbol && !hasSymbol {
				errors = append(errors, "missing symbol")
				valid = false
			}

			// Character class count
			charClasses := 0
			if hasLower {
				charClasses++
			}
			if hasUpper {
				charClasses++
			}
			if hasDigit {
				charClasses++
			}
			if hasSymbol {
				charClasses++
			}

			if params.MinClasses > 0 && charClasses < params.MinClasses {
				errors = append(errors, "insufficient character variety")
				valid = false
			}

			result := map[string]any{
				"valid":             valid,
				"character_classes": charClasses,
			}
			if len(errors) > 0 {
				result["errors"] = errors
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func entropyTool() tool.Tool {
	return tool.NewBuilder("password_entropy").
		WithDescription("Calculate password entropy").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Password string `json:"password"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			password := params.Password

			// Determine charset size
			charsetSize := 0
			if regexp.MustCompile(`[a-z]`).MatchString(password) {
				charsetSize += 26
			}
			if regexp.MustCompile(`[A-Z]`).MatchString(password) {
				charsetSize += 26
			}
			if regexp.MustCompile(`[0-9]`).MatchString(password) {
				charsetSize += 10
			}
			if regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password) {
				charsetSize += 32 // Approximate
			}

			// Entropy = log2(charset^length) = length * log2(charset)
			entropy := float64(len(password)) * bitsPerChar(charsetSize)

			var strength string
			switch {
			case entropy >= 128:
				strength = "very_strong"
			case entropy >= 80:
				strength = "strong"
			case entropy >= 60:
				strength = "reasonable"
			case entropy >= 40:
				strength = "weak"
			default:
				strength = "very_weak"
			}

			result := map[string]any{
				"entropy_bits": entropy,
				"charset_size": charsetSize,
				"length":       len(password),
				"strength":     strength,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func bitsPerChar(charsetSize int) float64 {
	if charsetSize <= 0 {
		return 0
	}
	// log2(charsetSize)
	bits := 0.0
	n := charsetSize
	for n > 1 {
		bits++
		n /= 2
	}
	return bits
}

func passphraseTool() tool.Tool {
	return tool.NewBuilder("password_passphrase").
		WithDescription("Generate passphrase from words").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				WordCount  int    `json:"word_count,omitempty"`
				Separator  string `json:"separator,omitempty"`
				Capitalize bool   `json:"capitalize,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			wordCount := params.WordCount
			if wordCount <= 0 {
				wordCount = 4
			}
			if wordCount > 10 {
				wordCount = 10
			}

			separator := params.Separator
			if separator == "" {
				separator = "-"
			}

			// Simple word list (subset)
			wordList := []string{
				"apple", "banana", "cherry", "dragon", "elephant",
				"falcon", "garden", "harbor", "island", "jungle",
				"kettle", "lemon", "mango", "nectar", "orange",
				"panda", "quartz", "river", "sunset", "tiger",
				"umbrella", "violet", "winter", "xenon", "yellow", "zebra",
				"anchor", "bridge", "castle", "desert", "empire",
				"forest", "glacier", "horizon", "iceberg", "journey",
			}

			words := make([]string, wordCount)
			for i := 0; i < wordCount; i++ {
				idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(wordList))))
				if err != nil {
					result := map[string]any{"error": err.Error()}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
				word := wordList[idx.Int64()]
				if params.Capitalize {
					word = strings.ToUpper(string(word[0])) + word[1:]
				}
				words[i] = word
			}

			passphrase := strings.Join(words, separator)

			// Calculate entropy (assuming wordlist of ~36 words)
			entropy := float64(wordCount) * 5.17 // log2(36) ≈ 5.17

			result := map[string]any{
				"passphrase":   passphrase,
				"word_count":   wordCount,
				"entropy_bits": entropy,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func commonCheckTool() tool.Tool {
	return tool.NewBuilder("password_common_check").
		WithDescription("Check if password is commonly used").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Password string `json:"password"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			lower := strings.ToLower(params.Password)
			isCommon := commonPasswords[lower]

			// Check patterns
			patterns := []struct {
				Name    string
				Pattern string
			}{
				{"sequential_numbers", `^(012|123|234|345|456|567|678|789|890)+$`},
				{"repeated_chars", `^(.)\1+$`},
				{"keyboard_pattern", `^(qwerty|asdf|zxcv)+`},
			}

			var matchedPatterns []string
			for _, p := range patterns {
				if regexp.MustCompile(p.Pattern).MatchString(lower) {
					matchedPatterns = append(matchedPatterns, p.Name)
				}
			}

			result := map[string]any{
				"is_common":        isCommon,
				"is_safe":          !isCommon && len(matchedPatterns) == 0,
				"matched_patterns": matchedPatterns,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func suggestTool() tool.Tool {
	return tool.NewBuilder("password_suggest").
		WithDescription("Suggest improvements to weak password").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Password string `json:"password"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			password := params.Password
			var suggestions []string

			if len(password) < 12 {
				suggestions = append(suggestions, "increase length to at least 12 characters")
			}
			if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
				suggestions = append(suggestions, "add uppercase letters")
			}
			if !regexp.MustCompile(`[a-z]`).MatchString(password) {
				suggestions = append(suggestions, "add lowercase letters")
			}
			if !regexp.MustCompile(`[0-9]`).MatchString(password) {
				suggestions = append(suggestions, "add numbers")
			}
			if !regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password) {
				suggestions = append(suggestions, "add special characters")
			}
			if commonPasswords[strings.ToLower(password)] {
				suggestions = append(suggestions, "avoid common passwords")
			}
			if regexp.MustCompile(`^(.)\1+$`).MatchString(password) {
				suggestions = append(suggestions, "avoid repeated characters")
			}

			result := map[string]any{
				"suggestions":       suggestions,
				"suggestion_count":  len(suggestions),
				"needs_improvement": len(suggestions) > 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func maskTool() tool.Tool {
	return tool.NewBuilder("password_mask").
		WithDescription("Mask password for display").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Password  string `json:"password"`
				ShowFirst int    `json:"show_first,omitempty"`
				ShowLast  int    `json:"show_last,omitempty"`
				MaskChar  string `json:"mask_char,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			maskChar := params.MaskChar
			if maskChar == "" {
				maskChar = "*"
			}

			password := params.Password
			length := len(password)

			if params.ShowFirst+params.ShowLast >= length {
				// Show all
				result := map[string]any{
					"masked": password,
					"length": length,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			masked := ""
			if params.ShowFirst > 0 {
				masked += password[:params.ShowFirst]
			}
			masked += strings.Repeat(maskChar, length-params.ShowFirst-params.ShowLast)
			if params.ShowLast > 0 {
				masked += password[length-params.ShowLast:]
			}

			result := map[string]any{
				"masked": masked,
				"length": length,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func requirementsTool() tool.Tool {
	return tool.NewBuilder("password_requirements").
		WithDescription("Get common password requirement presets").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Preset string `json:"preset,omitempty"` // basic, moderate, strong, enterprise
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			presets := map[string]map[string]any{
				"basic": {
					"min_length":     8,
					"require_upper":  false,
					"require_lower":  true,
					"require_digit":  true,
					"require_symbol": false,
					"min_classes":    2,
				},
				"moderate": {
					"min_length":     10,
					"require_upper":  true,
					"require_lower":  true,
					"require_digit":  true,
					"require_symbol": false,
					"min_classes":    3,
				},
				"strong": {
					"min_length":     12,
					"require_upper":  true,
					"require_lower":  true,
					"require_digit":  true,
					"require_symbol": true,
					"min_classes":    4,
				},
				"enterprise": {
					"min_length":     14,
					"max_length":     128,
					"require_upper":  true,
					"require_lower":  true,
					"require_digit":  true,
					"require_symbol": true,
					"min_classes":    4,
					"no_username":    true,
					"no_dictionary":  true,
					"no_common":      true,
					"history_count":  12,
					"max_age_days":   90,
				},
			}

			if params.Preset != "" {
				if preset, ok := presets[params.Preset]; ok {
					result := map[string]any{
						"preset":       params.Preset,
						"requirements": preset,
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}
				result := map[string]any{
					"error":             "unknown preset",
					"available_presets": []string{"basic", "moderate", "strong", "enterprise"},
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"presets": presets,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func hashInfoTool() tool.Tool {
	return tool.NewBuilder("password_hash_info").
		WithDescription("Get information about password hashing algorithms").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			algorithms := map[string]map[string]any{
				"bcrypt": {
					"recommended":   true,
					"cost_factor":   "10-12 recommended",
					"output_length": 60,
					"notes":         "Widely used, built-in salt, adjustable cost",
				},
				"argon2id": {
					"recommended": true,
					"parameters":  "memory, iterations, parallelism",
					"notes":       "Winner of Password Hashing Competition, recommended for new systems",
				},
				"scrypt": {
					"recommended": true,
					"parameters":  "N, r, p (CPU/memory cost)",
					"notes":       "Memory-hard, good alternative to bcrypt",
				},
				"pbkdf2": {
					"recommended": "acceptable",
					"iterations":  "minimum 100,000",
					"notes":       "NIST approved, widely supported",
				},
				"sha256": {
					"recommended": false,
					"notes":       "Too fast for passwords, use only with proper KDF",
				},
				"md5": {
					"recommended": false,
					"notes":       "NEVER use for passwords - broken and too fast",
				},
			}

			result := map[string]any{
				"algorithms":     algorithms,
				"recommendation": "Use bcrypt or argon2id for new applications",
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Helper to check for unicode categories
func hasUnicodeCategory(s string, category func(rune) bool) bool {
	for _, r := range s {
		if category(r) {
			return true
		}
	}
	return false
}

var _ = unicode.IsLower // Ensure import is used
