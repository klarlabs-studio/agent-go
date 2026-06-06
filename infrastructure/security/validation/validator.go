// Package validation provides input validation for tool execution.
package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// Validator validates input against a schema with rules.
type Validator interface {
	// Validate validates input against the schema.
	Validate(input json.RawMessage) error

	// AddRule adds a validation rule for a field.
	AddRule(field string, rule Rule)
}

// Rule defines a validation rule.
type Rule interface {
	// Name returns the rule name.
	Name() string

	// Validate validates a value against the rule.
	Validate(value interface{}) error
}

// Schema defines the validation schema for a tool.
type Schema struct {
	rules map[string][]Rule
}

// NewSchema creates a new validation schema.
func NewSchema() *Schema {
	return &Schema{
		rules: make(map[string][]Rule),
	}
}

// AddRule adds a validation rule for a field.
func (s *Schema) AddRule(field string, rule Rule) *Schema {
	s.rules[field] = append(s.rules[field], rule)
	return s
}

// Validate validates input against the schema.
func (s *Schema) Validate(input json.RawMessage) error {
	var data map[string]interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		return fmt.Errorf("invalid JSON input: %w", err)
	}

	var errs []string
	for field, rules := range s.rules {
		value, exists := data[field]
		for _, rule := range rules {
			// Required rule handles missing fields
			if !exists {
				if _, ok := rule.(*RequiredRule); ok {
					if err := rule.Validate(nil); err != nil {
						errs = append(errs, fmt.Sprintf("%s: %s", field, err.Error()))
					}
				}
				continue
			}

			if err := rule.Validate(value); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %s", field, err.Error()))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// RequiredRule validates that a field is present and non-empty.
type RequiredRule struct{}

func (r *RequiredRule) Name() string { return "required" }

func (r *RequiredRule) Validate(value interface{}) error {
	if value == nil {
		return errors.New("field is required")
	}
	if str, ok := value.(string); ok && str == "" {
		return errors.New("field cannot be empty")
	}
	return nil
}

// Required creates a required rule.
func Required() Rule {
	return &RequiredRule{}
}

// MaxLengthRule validates that a string does not exceed a maximum length.
type MaxLengthRule struct {
	max int
}

func (r *MaxLengthRule) Name() string { return "max_length" }

func (r *MaxLengthRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil // Only applies to strings
	}
	if utf8.RuneCountInString(str) > r.max {
		return fmt.Errorf("exceeds maximum length of %d", r.max)
	}
	return nil
}

// MaxLength creates a max length rule.
func MaxLength(max int) Rule {
	return &MaxLengthRule{max: max}
}

// MinLengthRule validates that a string meets a minimum length.
type MinLengthRule struct {
	min int
}

func (r *MinLengthRule) Name() string { return "min_length" }

func (r *MinLengthRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}
	if utf8.RuneCountInString(str) < r.min {
		return fmt.Errorf("must be at least %d characters", r.min)
	}
	return nil
}

// MinLength creates a min length rule.
func MinLength(min int) Rule {
	return &MinLengthRule{min: min}
}

// PatternRule validates that a string matches a regex pattern.
type PatternRule struct {
	pattern *regexp.Regexp
	name    string
}

func (r *PatternRule) Name() string { return r.name }

func (r *PatternRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}
	if !r.pattern.MatchString(str) {
		return fmt.Errorf("does not match required pattern")
	}
	return nil
}

// Pattern creates a pattern rule.
func Pattern(pattern string) Rule {
	return &PatternRule{
		pattern: regexp.MustCompile(pattern),
		name:    "pattern",
	}
}

// AllowedValuesRule validates that a value is in a list of allowed values.
type AllowedValuesRule struct {
	values []string
}

func (r *AllowedValuesRule) Name() string { return "allowed_values" }

func (r *AllowedValuesRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}
	for _, v := range r.values {
		if str == v {
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", strings.Join(r.values, ", "))
}

// AllowedValues creates an allowed values rule.
func AllowedValues(values ...string) Rule {
	return &AllowedValuesRule{values: values}
}

// RangeRule validates that a number is within a range.
type RangeRule struct {
	min, max float64
}

func (r *RangeRule) Name() string { return "range" }

func (r *RangeRule) Validate(value interface{}) error {
	var num float64
	switch v := value.(type) {
	case float64:
		num = v
	case int:
		num = float64(v)
	case int64:
		num = float64(v)
	default:
		return nil
	}
	if num < r.min || num > r.max {
		return fmt.Errorf("must be between %.2f and %.2f", r.min, r.max)
	}
	return nil
}

// Range creates a range rule.
func Range(min, max float64) Rule {
	return &RangeRule{min: min, max: max}
}

// NoSQLInjectionRule detects potential SQL injection patterns.
type NoSQLInjectionRule struct{}

func (r *NoSQLInjectionRule) Name() string { return "no_sql_injection" }

func (r *NoSQLInjectionRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}

	// Common SQL injection patterns
	patterns := []string{
		`(?i)\b(union\s+select|select\s+.*\s+from|insert\s+into|update\s+.*\s+set|delete\s+from|drop\s+table|drop\s+database)\b`,
		`(?i)('\s*or\s+'|"\s*or\s+"|'\s*or\s+1|"\s*or\s+1)`,
		`(?i)(--\s*$|;\s*--)`,
		`(?i)\bexec\s*\(|\bexecute\s*\(`,
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if re.MatchString(str) {
			return errors.New("potential SQL injection detected")
		}
	}
	return nil
}

// NoSQLInjection creates a SQL injection detection rule.
func NoSQLInjection() Rule {
	return &NoSQLInjectionRule{}
}

// NoPathTraversalRule detects path traversal attempts.
type NoPathTraversalRule struct{}

func (r *NoPathTraversalRule) Name() string { return "no_path_traversal" }

func (r *NoPathTraversalRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}

	// Path traversal patterns
	patterns := []string{
		`\.\.[\\/]`,
		`%2e%2e[\\/]`,
		`%252e%252e[\\/]`,
		`\.\.%2f`,
		`%2e%2e%2f`,
	}

	strLower := strings.ToLower(str)
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if re.MatchString(strLower) {
			return errors.New("potential path traversal detected")
		}
	}
	return nil
}

// NoPathTraversal creates a path traversal detection rule.
func NoPathTraversal() Rule {
	return &NoPathTraversalRule{}
}

// NoCommandInjectionRule detects potential command injection patterns.
type NoCommandInjectionRule struct{}

func (r *NoCommandInjectionRule) Name() string { return "no_command_injection" }

func (r *NoCommandInjectionRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}

	// Command injection patterns
	dangerous := []string{
		"|", ";", "&", "$", "`", "(", ")", "{", "}", "[", "]",
		"\n", "\r",
	}

	for _, d := range dangerous {
		if strings.Contains(str, d) {
			return errors.New("potential command injection detected")
		}
	}
	return nil
}

// NoCommandInjection creates a command injection detection rule.
func NoCommandInjection() Rule {
	return &NoCommandInjectionRule{}
}

// EmailRule validates email format.
type EmailRule struct{}

func (r *EmailRule) Name() string { return "email" }

func (r *EmailRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}

	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(pattern)
	if !re.MatchString(str) {
		return errors.New("invalid email format")
	}
	return nil
}

// Email creates an email validation rule.
func Email() Rule {
	return &EmailRule{}
}

// URLRule validates URL format.
type URLRule struct {
	allowedSchemes []string
}

func (r *URLRule) Name() string { return "url" }

func (r *URLRule) Validate(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return nil
	}

	// Basic URL validation
	pattern := `^(https?|ftp)://[^\s/$.?#].[^\s]*$`
	re := regexp.MustCompile(pattern)
	if !re.MatchString(str) {
		return errors.New("invalid URL format")
	}

	if len(r.allowedSchemes) > 0 {
		schemeValid := false
		for _, scheme := range r.allowedSchemes {
			if strings.HasPrefix(str, scheme+"://") {
				schemeValid = true
				break
			}
		}
		if !schemeValid {
			return fmt.Errorf("URL scheme must be one of: %s", strings.Join(r.allowedSchemes, ", "))
		}
	}

	return nil
}

// URL creates a URL validation rule.
func URL(allowedSchemes ...string) Rule {
	return &URLRule{allowedSchemes: allowedSchemes}
}

// CustomRule allows custom validation logic.
type CustomRule struct {
	name     string
	validate func(value interface{}) error
}

func (r *CustomRule) Name() string { return r.name }

func (r *CustomRule) Validate(value interface{}) error {
	return r.validate(value)
}

// Custom creates a custom validation rule.
func Custom(name string, validate func(value interface{}) error) Rule {
	return &CustomRule{name: name, validate: validate}
}

// ValidationMiddleware creates middleware that validates tool input.
func ValidationMiddleware(schemas map[string]*Schema) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			toolName := execCtx.Tool.Name()
			schema, exists := schemas[toolName]
			if exists {
				if err := schema.Validate(execCtx.Input); err != nil {
					return tool.Result{}, fmt.Errorf("input validation failed for %s: %w", toolName, err)
				}
			}
			return next(ctx, execCtx)
		}
	}
}
