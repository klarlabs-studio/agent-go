// Package middleware provides pre-built middleware implementations.
package middleware

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// ValidationConfig configures the input validation middleware.
type ValidationConfig struct {
	// ValidateInput controls whether to validate tool inputs against schemas.
	// Default: true
	ValidateInput bool

	// ValidateOutput controls whether to validate tool outputs against schemas.
	// Default: false (outputs are typically validated by the consuming code)
	ValidateOutput bool

	// RejectEmpty controls whether to reject empty/nil inputs.
	// Default: false (empty inputs may be valid for some tools)
	RejectEmpty bool

	// MaxInputBytes, when > 0, rejects tool inputs whose raw JSON exceeds this
	// many bytes. This is an invocation-time input-validation guard that bounds
	// untrusted/LLM-produced payloads before they reach tool handlers,
	// limiting resource-exhaustion and oversized-injection vectors.
	// Default: 0 (no size limit).
	MaxInputBytes int
}

// DefaultValidationConfig returns a sensible default configuration.
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: false,
		RejectEmpty:    false,
	}
}

// Validation returns middleware that validates tool inputs and outputs
// against their declared JSON schemas.
//
// This provides a security layer that ensures:
// - Input is valid JSON
// - Input conforms to the tool's declared input schema (when defined)
// - Output conforms to the tool's declared output schema (when enabled)
//
// Security Considerations:
// - Prevents malformed JSON from reaching tool handlers
// - Enforces schema contracts defined by tools
// - Bounds untrusted/LLM-produced input size when MaxInputBytes is set
// - Helps detect LLM hallucinations that produce invalid tool inputs
//
// This is the framework's invocation-time input-validation guard. It performs
// structural and schema validation; it is not a prompt-injection content
// detector. The primary defense against malicious tool use is structural —
// the act-gate, tool eligibility, and governance/approval — not content
// heuristics. For callable, field-level validators (email, url, uuid, credit
// card, etc.) see contrib/pack-validate.
func Validation(cfg ValidationConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			t := execCtx.Tool
			input := execCtx.Input

			// Size guard runs regardless of schema validation: it bounds the
			// untrusted payload before any further processing.
			if cfg.MaxInputBytes > 0 && len(input) > cfg.MaxInputBytes {
				return tool.Result{}, fmt.Errorf("%w: input size %d exceeds limit %d bytes",
					tool.ErrInvalidInput, len(input), cfg.MaxInputBytes)
			}

			// Validate input
			if cfg.ValidateInput {
				if err := validateInput(t, input, cfg.RejectEmpty); err != nil {
					return tool.Result{}, fmt.Errorf("%w: %v", tool.ErrInvalidInput, err)
				}
			}

			// Execute the tool
			result, err := next(ctx, execCtx)
			if err != nil {
				return result, err
			}

			// Validate output (optional)
			if cfg.ValidateOutput {
				if err := validateOutput(t, result.Output); err != nil {
					return tool.Result{}, fmt.Errorf("%w: %v", tool.ErrInvalidOutput, err)
				}
			}

			return result, nil
		}
	}
}

// validateInput validates tool input against the tool's input schema.
func validateInput(t tool.Tool, input json.RawMessage, rejectEmpty bool) error {
	// Check for empty input
	if len(input) == 0 || string(input) == "null" {
		if rejectEmpty {
			return fmt.Errorf("input is empty or null")
		}
		// Empty input is allowed - some tools don't require input
		return nil
	}

	// Ensure input is valid JSON
	if !json.Valid(input) {
		return fmt.Errorf("input is not valid JSON")
	}

	// Validate against schema if defined
	schema := t.InputSchema()
	if !schema.IsEmpty() {
		if err := schema.Validate(input); err != nil {
			return fmt.Errorf("input schema validation failed: %w", err)
		}
	}

	return nil
}

// validateOutput validates tool output against the tool's output schema.
func validateOutput(t tool.Tool, output json.RawMessage) error {
	// Allow empty output
	if len(output) == 0 || string(output) == "null" {
		return nil
	}

	// Ensure output is valid JSON
	if !json.Valid(output) {
		return fmt.Errorf("output is not valid JSON")
	}

	// Validate against schema if defined
	schema := t.OutputSchema()
	if !schema.IsEmpty() {
		if err := schema.Validate(output); err != nil {
			return fmt.Errorf("output schema validation failed: %w", err)
		}
	}

	return nil
}
