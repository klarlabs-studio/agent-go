package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	domainconfig "go.klarlabs.de/agent/domain/config"
)

// envExpander expands environment variables in configuration strings.
type envExpander struct {
	// strict fails if a referenced variable is not set.
	strict bool
	// missing tracks missing environment variables.
	missing []string
}

// Expand expands environment variables in the input string.
// Supported patterns:
//   - ${VAR} - expands to the value of VAR
//   - ${VAR:-default} - expands to VAR or "default" if not set
//   - ${VAR:?error message} - fails if VAR is not set
//   - $VAR - simple expansion (not recommended, use ${VAR})
func (e *envExpander) Expand(input string) (string, error) {
	e.missing = nil

	// Pattern for ${VAR}, ${VAR:-default}, ${VAR:?error}
	bracketPattern := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-[^}]*|:\?[^}]*)?\}`)

	// Pattern for $VAR (simple)
	simplePattern := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)

	// First, handle bracketed patterns
	result := bracketPattern.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name and modifier
		inner := match[2 : len(match)-1] // Remove ${ and }

		parts := strings.SplitN(inner, ":", 2)
		varName := parts[0]
		var modifier string
		if len(parts) > 1 {
			modifier = parts[1]
		}

		value, exists := os.LookupEnv(varName)

		// Handle modifiers
		if modifier != "" {
			if strings.HasPrefix(modifier, "-") {
				// Default value: ${VAR:-default}
				if !exists || value == "" {
					return modifier[1:] // Return default value
				}
			} else if strings.HasPrefix(modifier, "?") {
				// Required: ${VAR:?error message}
				if !exists || value == "" {
					e.missing = append(e.missing, fmt.Sprintf("%s: %s", varName, modifier[1:]))
					return match // Keep original for error reporting
				}
			}
		} else {
			// No modifier
			if !exists {
				if e.strict {
					e.missing = append(e.missing, varName)
				}
				return "" // Empty string for unset variables
			}
		}

		return value
	})

	// Then, handle simple $VAR patterns (only outside of brackets)
	result = simplePattern.ReplaceAllStringFunc(result, func(match string) string {
		// Skip if this is part of a ${...} pattern (already processed)
		varName := match[1:]
		value, exists := os.LookupEnv(varName)
		if !exists {
			if e.strict {
				e.missing = append(e.missing, varName)
			}
			return ""
		}
		return value
	})

	// Check for missing variables
	if len(e.missing) > 0 {
		return "", fmt.Errorf("%w: %s", domainconfig.ErrMissingEnvVar, strings.Join(e.missing, ", "))
	}

	return result, nil
}

// ExpandEnv is a convenience function that expands environment variables.
func ExpandEnv(input string) string {
	e := &envExpander{strict: false}
	result, _ := e.Expand(input)
	return result
}

// ExpandEnvStrict expands environment variables and returns an error for missing vars.
func ExpandEnvStrict(input string) (string, error) {
	e := &envExpander{strict: true}
	return e.Expand(input)
}
