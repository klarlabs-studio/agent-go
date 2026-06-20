package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	domainmw "go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
	mw "go.klarlabs.de/agent/infrastructure/middleware"
)

// mockToolWithSchemas creates a mock tool with specific input/output schemas.
type mockToolWithSchemas struct {
	name         string
	inputSchema  tool.Schema
	outputSchema tool.Schema
	handler      func(ctx context.Context, input json.RawMessage) (tool.Result, error)
}

func (m *mockToolWithSchemas) Name() string              { return m.name }
func (m *mockToolWithSchemas) Description() string       { return "mock tool with schemas" }
func (m *mockToolWithSchemas) InputSchema() tool.Schema  { return m.inputSchema }
func (m *mockToolWithSchemas) OutputSchema() tool.Schema { return m.outputSchema }
func (m *mockToolWithSchemas) Annotations() tool.Annotations {
	return tool.Annotations{}
}
func (m *mockToolWithSchemas) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if m.handler != nil {
		return m.handler(ctx, input)
	}
	return tool.Result{Output: json.RawMessage(`{"status":"ok"}`)}, nil
}

func TestValidation_ValidInputPassesThrough(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: false,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:        "test_tool",
		inputSchema: tool.EmptySchema(), // Accept any valid JSON
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{"key":"value"}`),
	}

	expected := tool.Result{Output: json.RawMessage(`{"result":"success"}`)}
	handler := middleware(createTestHandler(expected, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Output) != string(expected.Output) {
		t.Errorf("got output %s, want %s", result.Output, expected.Output)
	}
}

func TestValidation_EmptyInputAcceptedByDefault(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: false,
		RejectEmpty:    false, // Default: allow empty
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:        "no_input_tool",
		inputSchema: tool.EmptySchema(),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(``), // Empty input
	}

	expected := tool.Result{Output: json.RawMessage(`{"ok":true}`)}
	handler := middleware(createTestHandler(expected, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error for empty input: %v", err)
	}
	if string(result.Output) != string(expected.Output) {
		t.Errorf("got output %s, want %s", result.Output, expected.Output)
	}
}

func TestValidation_EmptyInputRejectedWhenConfigured(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: false,
		RejectEmpty:    true, // Reject empty input
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:        "requires_input_tool",
		inputSchema: tool.EmptySchema(),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(``), // Empty input
	}

	handler := middleware(createTestHandler(tool.Result{}, nil))

	_, err := handler(context.Background(), execCtx)
	if err == nil {
		t.Fatal("expected error for empty input with RejectEmpty=true")
	}
	if !errors.Is(err, tool.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestValidation_NullInputRejectedWhenConfigured(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: false,
		RejectEmpty:    true,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:        "requires_input_tool",
		inputSchema: tool.EmptySchema(),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`null`), // Null input
	}

	handler := middleware(createTestHandler(tool.Result{}, nil))

	_, err := handler(context.Background(), execCtx)
	if err == nil {
		t.Fatal("expected error for null input with RejectEmpty=true")
	}
	if !errors.Is(err, tool.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestValidation_InvalidJSONInputRejected(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: false,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:        "test_tool",
		inputSchema: tool.EmptySchema(),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{invalid json`), // Malformed JSON
	}

	handler := middleware(createTestHandler(tool.Result{}, nil))

	_, err := handler(context.Background(), execCtx)
	if err == nil {
		t.Fatal("expected error for invalid JSON input")
	}
	if !errors.Is(err, tool.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestValidation_SchemaValidationPass(t *testing.T) {
	t.Parallel()

	// Create a schema (note: basic validation only checks valid JSON for now)
	schema := tool.NewSchema(json.RawMessage(`{"type":"object"}`))

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: false,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:        "schema_tool",
		inputSchema: schema,
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{"field":"value"}`), // Valid JSON matching schema
	}

	expected := tool.Result{Output: json.RawMessage(`{"validated":true}`)}
	handler := middleware(createTestHandler(expected, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Output) != string(expected.Output) {
		t.Errorf("got output %s, want %s", result.Output, expected.Output)
	}
}

func TestValidation_SchemaValidationFailsOnInvalidJSON(t *testing.T) {
	t.Parallel()

	schema := tool.NewSchema(json.RawMessage(`{"type":"object"}`))

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: false,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:        "schema_tool",
		inputSchema: schema,
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`not json`), // Invalid JSON
	}

	handler := middleware(createTestHandler(tool.Result{}, nil))

	_, err := handler(context.Background(), execCtx)
	if err == nil {
		t.Fatal("expected error for invalid JSON with schema")
	}
	if !errors.Is(err, tool.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestValidation_OutputValidationDisabledByDefault(t *testing.T) {
	t.Parallel()

	cfg := mw.DefaultValidationConfig() // Output validation disabled
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "test_tool",
		inputSchema:  tool.EmptySchema(),
		outputSchema: tool.NewSchema(json.RawMessage(`{"type":"object"}`)),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	// Return invalid output - should pass because validation is disabled
	invalidOutput := tool.Result{Output: json.RawMessage(`invalid json`)}
	handler := middleware(createTestHandler(invalidOutput, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error when output validation disabled: %v", err)
	}
	if string(result.Output) != string(invalidOutput.Output) {
		t.Errorf("got output %s, want %s", result.Output, invalidOutput.Output)
	}
}

func TestValidation_ValidOutputPassesWhenEnabled(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: true, // Enable output validation
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "test_tool",
		inputSchema:  tool.EmptySchema(),
		outputSchema: tool.NewSchema(json.RawMessage(`{"type":"object"}`)),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	validOutput := tool.Result{Output: json.RawMessage(`{"result":"ok"}`)}
	handler := middleware(createTestHandler(validOutput, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error for valid output: %v", err)
	}
	if string(result.Output) != string(validOutput.Output) {
		t.Errorf("got output %s, want %s", result.Output, validOutput.Output)
	}
}

func TestValidation_InvalidOutputRejectedWhenEnabled(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: true, // Enable output validation
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "test_tool",
		inputSchema:  tool.EmptySchema(),
		outputSchema: tool.NewSchema(json.RawMessage(`{"type":"object"}`)),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	// Return invalid JSON output
	invalidOutput := tool.Result{Output: json.RawMessage(`{invalid`)}
	handler := middleware(createTestHandler(invalidOutput, nil))

	_, err := handler(context.Background(), execCtx)
	if err == nil {
		t.Fatal("expected error for invalid output JSON")
	}
	if !errors.Is(err, tool.ErrInvalidOutput) {
		t.Errorf("expected ErrInvalidOutput, got %v", err)
	}
}

func TestValidation_EmptyOutputAllowedEvenWithValidationEnabled(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: true,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "test_tool",
		inputSchema:  tool.EmptySchema(),
		outputSchema: tool.NewSchema(json.RawMessage(`{"type":"object"}`)),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	// Empty output should be allowed
	emptyOutput := tool.Result{Output: json.RawMessage(``)}
	handler := middleware(createTestHandler(emptyOutput, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error for empty output: %v", err)
	}
	if string(result.Output) != "" {
		t.Errorf("got output %s, want empty", result.Output)
	}
}

func TestValidation_NullOutputAllowedEvenWithValidationEnabled(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: true,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "test_tool",
		inputSchema:  tool.EmptySchema(),
		outputSchema: tool.NewSchema(json.RawMessage(`{"type":"object"}`)),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	// Null output should be allowed
	nullOutput := tool.Result{Output: json.RawMessage(`null`)}
	handler := middleware(createTestHandler(nullOutput, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error for null output: %v", err)
	}
	if string(result.Output) != "null" {
		t.Errorf("got output %s, want null", result.Output)
	}
}

func TestValidation_EmptySchemaAcceptsAnyValidJSON(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: true,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "flexible_tool",
		inputSchema:  tool.EmptySchema(), // No schema constraints
		outputSchema: tool.EmptySchema(),
	}

	testCases := []struct {
		name   string
		input  json.RawMessage
		output json.RawMessage
	}{
		{"object", json.RawMessage(`{"key":"value"}`), json.RawMessage(`{"result":"ok"}`)},
		{"array", json.RawMessage(`[1,2,3]`), json.RawMessage(`[4,5,6]`)},
		{"string", json.RawMessage(`"text"`), json.RawMessage(`"response"`)},
		{"number", json.RawMessage(`42`), json.RawMessage(`100`)},
		{"boolean", json.RawMessage(`true`), json.RawMessage(`false`)},
		{"null", json.RawMessage(`null`), json.RawMessage(`null`)},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			execCtx := &domainmw.ExecutionContext{
				CurrentState: agent.StateExplore,
				Tool:         mockT,
				Input:        tc.input,
			}

			expected := tool.Result{Output: tc.output}
			handler := middleware(createTestHandler(expected, nil))

			result, err := handler(context.Background(), execCtx)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.name, err)
			}
			if string(result.Output) != string(expected.Output) {
				t.Errorf("got output %s, want %s", result.Output, expected.Output)
			}
		})
	}
}

func TestValidation_InputValidationDisabled(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  false, // Disable input validation
		ValidateOutput: false,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:        "no_validation_tool",
		inputSchema: tool.NewSchema(json.RawMessage(`{"type":"object"}`)),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`invalid json`), // Should pass through
	}

	expected := tool.Result{Output: json.RawMessage(`{"ok":true}`)}
	handler := middleware(createTestHandler(expected, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error when validation disabled: %v", err)
	}
	if string(result.Output) != string(expected.Output) {
		t.Errorf("got output %s, want %s", result.Output, expected.Output)
	}
}

func TestValidation_HandlerErrorPassedThrough(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: true,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "failing_tool",
		inputSchema:  tool.EmptySchema(),
		outputSchema: tool.EmptySchema(),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	expectedErr := errors.New("handler execution failed")
	handler := middleware(createTestHandler(tool.Result{}, expectedErr))

	_, err := handler(context.Background(), execCtx)
	if err == nil {
		t.Fatal("expected error from handler")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected handler error, got %v", err)
	}
}

func TestValidation_NoValidationPerformedWhenBothDisabled(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  false,
		ValidateOutput: false,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "passthrough_tool",
		inputSchema:  tool.NewSchema(json.RawMessage(`{"type":"object"}`)),
		outputSchema: tool.NewSchema(json.RawMessage(`{"type":"object"}`)),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`totally invalid`), // Should pass through
	}

	invalidOutput := tool.Result{Output: json.RawMessage(`also invalid`)}
	handler := middleware(createTestHandler(invalidOutput, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error when all validation disabled: %v", err)
	}
	if string(result.Output) != string(invalidOutput.Output) {
		t.Errorf("got output %s, want %s", result.Output, invalidOutput.Output)
	}
}

func TestDefaultValidationConfig(t *testing.T) {
	t.Parallel()

	t.Run("returns expected defaults", func(t *testing.T) {
		t.Parallel()

		cfg := mw.DefaultValidationConfig()

		if !cfg.ValidateInput {
			t.Error("expected ValidateInput to be true by default")
		}
		if cfg.ValidateOutput {
			t.Error("expected ValidateOutput to be false by default")
		}
		if cfg.RejectEmpty {
			t.Error("expected RejectEmpty to be false by default")
		}
	})
}

func TestValidation_PreservesResultMetadata(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: true,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "metadata_tool",
		inputSchema:  tool.EmptySchema(),
		outputSchema: tool.EmptySchema(),
	}

	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	// Result with metadata
	expectedResult := tool.Result{
		Output:   json.RawMessage(`{"data":"value"}`),
		Cached:   true,
		Duration: 100,
		Artifacts: []tool.ArtifactRef{
			{ID: "artifact-1", Name: "test.txt"},
		},
	}
	handler := middleware(createTestHandler(expectedResult, nil))

	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result.Output) != string(expectedResult.Output) {
		t.Errorf("got output %s, want %s", result.Output, expectedResult.Output)
	}
	if result.Cached != expectedResult.Cached {
		t.Errorf("got Cached %v, want %v", result.Cached, expectedResult.Cached)
	}
	if result.Duration != expectedResult.Duration {
		t.Errorf("got Duration %v, want %v", result.Duration, expectedResult.Duration)
	}
	if len(result.Artifacts) != len(expectedResult.Artifacts) {
		t.Errorf("got %d artifacts, want %d", len(result.Artifacts), len(expectedResult.Artifacts))
	}
}

func TestValidation_ConcurrentExecution(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput:  true,
		ValidateOutput: true,
		RejectEmpty:    false,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{
		name:         "concurrent_tool",
		inputSchema:  tool.EmptySchema(),
		outputSchema: tool.EmptySchema(),
	}

	// Run multiple validations concurrently
	const numGoroutines = 100
	done := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			execCtx := &domainmw.ExecutionContext{
				CurrentState: agent.StateExplore,
				Tool:         mockT,
				Input:        json.RawMessage(`{"id":` + string(rune('0'+id%10)) + `}`),
			}

			expected := tool.Result{Output: json.RawMessage(`{"processed":true}`)}
			handler := middleware(createTestHandler(expected, nil))

			_, err := handler(context.Background(), execCtx)
			done <- err
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		if err := <-done; err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}
}

func TestValidation_MaxInputBytes_RejectsOversized(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput: true,
		MaxInputBytes: 16,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{name: "sized_tool", inputSchema: tool.EmptySchema()}
	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{"k":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), // > 16 bytes
	}

	handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))
	_, err := handler(context.Background(), execCtx)
	if !errors.Is(err, tool.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for oversized input, got %v", err)
	}
}

func TestValidation_MaxInputBytes_AllowsWithinLimit(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{
		ValidateInput: true,
		MaxInputBytes: 64,
	}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{name: "sized_tool", inputSchema: tool.EmptySchema()}
	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(`{"k":"v"}`),
	}

	expected := tool.Result{Output: json.RawMessage(`{"ok":true}`)}
	handler := middleware(createTestHandler(expected, nil))
	result, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error within limit: %v", err)
	}
	if string(result.Output) != string(expected.Output) {
		t.Errorf("got %s, want %s", result.Output, expected.Output)
	}
}

func TestValidation_MaxInputBytes_ZeroMeansUnbounded(t *testing.T) {
	t.Parallel()

	cfg := mw.ValidationConfig{ValidateInput: true, MaxInputBytes: 0}
	middleware := mw.Validation(cfg)

	mockT := &mockToolWithSchemas{name: "sized_tool", inputSchema: tool.EmptySchema()}
	big := `{"k":"` + string(make([]byte, 0)) + `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	execCtx := &domainmw.ExecutionContext{
		CurrentState: agent.StateExplore,
		Tool:         mockT,
		Input:        json.RawMessage(big),
	}
	handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))
	if _, err := handler(context.Background(), execCtx); err != nil {
		t.Fatalf("zero limit must be unbounded, got %v", err)
	}
}
