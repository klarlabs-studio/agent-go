package tool_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestToolBuilder_Basic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		toolName    string
		description string
		wantErr     error
	}{
		{
			name:        "valid tool",
			toolName:    "test_tool",
			description: "A test tool",
			wantErr:     nil,
		},
		{
			name:        "empty name fails",
			toolName:    "",
			description: "Should fail",
			wantErr:     tool.ErrEmptyName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := tool.NewBuilder(tt.toolName).
				WithDescription(tt.description).
				WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
					return tool.Result{Output: input}, nil
				})

			built, err := builder.Build()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr == nil {
				if built.Name() != tt.toolName {
					t.Errorf("Name() = %v, want %v", built.Name(), tt.toolName)
				}
				if built.Description() != tt.description {
					t.Errorf("Description() = %v, want %v", built.Description(), tt.description)
				}
			}
		})
	}
}

func TestToolBuilder_ReadOnly(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("read_only_tool").
		WithDescription("A read-only tool").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	annotations := built.Annotations()
	if !annotations.ReadOnly {
		t.Error("ReadOnly should be true")
	}
	if annotations.RiskLevel != tool.RiskNone {
		t.Errorf("RiskLevel = %v, want %v", annotations.RiskLevel, tool.RiskNone)
	}
}

func TestToolBuilder_Destructive(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("destructive_tool").
		WithDescription("A destructive tool").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	annotations := built.Annotations()
	if !annotations.Destructive {
		t.Error("Destructive should be true")
	}
	if !annotations.RequiresApproval {
		t.Error("RequiresApproval should be true for destructive tools")
	}
	if annotations.RiskLevel < tool.RiskHigh {
		t.Errorf("RiskLevel = %v, want >= %v", annotations.RiskLevel, tool.RiskHigh)
	}
}

func TestToolBuilder_Idempotent(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("idempotent_tool").
		WithDescription("An idempotent tool").
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	annotations := built.Annotations()
	if !annotations.Idempotent {
		t.Error("Idempotent should be true")
	}
}

func TestToolBuilder_Cacheable(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("cacheable_tool").
		WithDescription("A cacheable tool").
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	annotations := built.Annotations()
	if !annotations.Cacheable {
		t.Error("Cacheable should be true")
	}
}

func TestToolBuilder_WithRiskLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		riskLevel tool.RiskLevel
	}{
		{"none", tool.RiskNone},
		{"low", tool.RiskLow},
		{"medium", tool.RiskMedium},
		{"high", tool.RiskHigh},
		{"critical", tool.RiskCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			built := tool.NewBuilder("risk_tool").
				WithDescription("A risky tool").
				WithRiskLevel(tt.riskLevel).
				WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
					return tool.Result{Output: input}, nil
				}).
				MustBuild()

			if built.Annotations().RiskLevel != tt.riskLevel {
				t.Errorf("RiskLevel = %v, want %v", built.Annotations().RiskLevel, tt.riskLevel)
			}
		})
	}
}

func TestToolBuilder_RequiresApproval(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("approval_tool").
		WithDescription("A tool requiring approval").
		RequiresApproval().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	annotations := built.Annotations()
	if !annotations.RequiresApproval {
		t.Error("RequiresApproval should be true")
	}
}

func TestToolBuilder_WithTags(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("tagged_tool").
		WithDescription("A tagged tool").
		WithTags("database", "read", "sql").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	tags := built.Annotations().Tags
	if len(tags) != 3 {
		t.Errorf("Tags count = %d, want 3", len(tags))
	}
	expectedTags := map[string]bool{"database": true, "read": true, "sql": true}
	for _, tag := range tags {
		if !expectedTags[tag] {
			t.Errorf("Unexpected tag: %s", tag)
		}
	}
}

func TestToolBuilder_MustBuild_Panics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustBuild should panic on error")
		}
	}()

	tool.NewBuilder("").MustBuild()
}

func TestToolDefinition_Execute(t *testing.T) {
	t.Parallel()

	t.Run("successful execution", func(t *testing.T) {
		t.Parallel()

		input := json.RawMessage(`{"message": "hello"}`)
		expectedOutput := json.RawMessage(`{"result": "world"}`)

		built := tool.NewBuilder("exec_tool").
			WithHandler(func(ctx context.Context, in json.RawMessage) (tool.Result, error) {
				return tool.Result{Output: expectedOutput}, nil
			}).
			MustBuild()

		result, err := built.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if string(result.Output) != string(expectedOutput) {
			t.Errorf("Output = %s, want %s", result.Output, expectedOutput)
		}
	})

	t.Run("execution with error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("execution failed")

		built := tool.NewBuilder("error_tool").
			WithHandler(func(ctx context.Context, in json.RawMessage) (tool.Result, error) {
				return tool.Result{}, expectedErr
			}).
			MustBuild()

		_, err := built.Execute(context.Background(), nil)
		if !errors.Is(err, expectedErr) {
			t.Errorf("Execute() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()

		built := tool.NewBuilder("ctx_tool").
			WithHandler(func(ctx context.Context, in json.RawMessage) (tool.Result, error) {
				select {
				case <-ctx.Done():
					return tool.Result{}, ctx.Err()
				default:
					return tool.Result{Output: json.RawMessage(`{}`)}, nil
				}
			}).
			MustBuild()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := built.Execute(ctx, nil)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Execute() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestToolBuilder_ChainedConfiguration(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("chained_tool").
		WithDescription("A fully configured tool").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithTags("test", "example").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	annotations := built.Annotations()

	if !annotations.ReadOnly {
		t.Error("ReadOnly should be true")
	}
	if !annotations.Idempotent {
		t.Error("Idempotent should be true")
	}
	if !annotations.Cacheable {
		t.Error("Cacheable should be true")
	}
	if len(annotations.Tags) != 2 {
		t.Errorf("Tags count = %d, want 2", len(annotations.Tags))
	}
}

func TestToolBuilder_WithAnnotations(t *testing.T) {
	t.Parallel()

	customAnnotations := tool.Annotations{
		ReadOnly:         true,
		Destructive:      false,
		Idempotent:       true,
		Cacheable:        true,
		RiskLevel:        tool.RiskMedium,
		RequiresApproval: false,
		Timeout:          30,
		Tags:             []string{"custom"},
	}

	built := tool.NewBuilder("custom_annotations_tool").
		WithAnnotations(customAnnotations).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	got := built.Annotations()
	if got.ReadOnly != customAnnotations.ReadOnly {
		t.Errorf("ReadOnly = %v, want %v", got.ReadOnly, customAnnotations.ReadOnly)
	}
	if got.RiskLevel != customAnnotations.RiskLevel {
		t.Errorf("RiskLevel = %v, want %v", got.RiskLevel, customAnnotations.RiskLevel)
	}
	if got.Timeout != customAnnotations.Timeout {
		t.Errorf("Timeout = %v, want %v", got.Timeout, customAnnotations.Timeout)
	}
}

func TestToolBuilder_WithInputSchema(t *testing.T) {
	t.Parallel()

	inputSchema := tool.ObjectSchema(map[string]json.RawMessage{
		"name": json.RawMessage(`{"type": "string"}`),
		"age":  json.RawMessage(`{"type": "integer"}`),
	}, []string{"name"})

	built := tool.NewBuilder("schema_tool").
		WithInputSchema(inputSchema).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: input}, nil
		}).
		MustBuild()

	gotSchema := built.InputSchema()
	if string(gotSchema.Raw()) != string(inputSchema.Raw()) {
		t.Errorf("InputSchema() = %s, want %s", gotSchema.Raw(), inputSchema.Raw())
	}
}

func TestToolBuilder_WithOutputSchema(t *testing.T) {
	t.Parallel()

	outputSchema := tool.ObjectSchema(map[string]json.RawMessage{
		"result": json.RawMessage(`{"type": "boolean"}`),
	}, nil)

	built := tool.NewBuilder("output_schema_tool").
		WithOutputSchema(outputSchema).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"result": true}`)}, nil
		}).
		MustBuild()

	gotSchema := built.OutputSchema()
	if string(gotSchema.Raw()) != string(outputSchema.Raw()) {
		t.Errorf("OutputSchema() = %s, want %s", gotSchema.Raw(), outputSchema.Raw())
	}
}

func TestToolDefinition_InputSchema_Default(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("no_schema_tool").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, nil
		}).
		MustBuild()

	schema := built.InputSchema()
	if !schema.IsEmpty() {
		t.Errorf("Default InputSchema should be empty, got %s", schema.Raw())
	}
}

func TestToolDefinition_OutputSchema_Default(t *testing.T) {
	t.Parallel()

	built := tool.NewBuilder("no_output_schema_tool").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{}, nil
		}).
		MustBuild()

	schema := built.OutputSchema()
	if !schema.IsEmpty() {
		t.Errorf("Default OutputSchema should be empty, got %s", schema.Raw())
	}
}

func TestToolDefinition_Execute_NoHandler(t *testing.T) {
	t.Parallel()

	// Create a tool definition without a handler by using Build directly
	builder := tool.NewBuilder("no_handler_tool")
	built, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	_, err = built.Execute(context.Background(), nil)
	if !errors.Is(err, tool.ErrNoHandler) {
		t.Errorf("Execute() error = %v, want %v", err, tool.ErrNoHandler)
	}
}

func TestToolBuilder_WithSchemas_Combined(t *testing.T) {
	t.Parallel()

	inputSchema := tool.ObjectSchema(map[string]json.RawMessage{
		"query": json.RawMessage(`{"type": "string"}`),
	}, []string{"query"})

	outputSchema := tool.ObjectSchema(map[string]json.RawMessage{
		"results": json.RawMessage(`{"type": "array", "items": {"type": "string"}}`),
	}, nil)

	built := tool.NewBuilder("full_schema_tool").
		WithDescription("Tool with full schema").
		WithInputSchema(inputSchema).
		WithOutputSchema(outputSchema).
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"results": ["a", "b"]}`)}, nil
		}).
		MustBuild()

	if !built.Annotations().ReadOnly {
		t.Error("ReadOnly should be true")
	}
	if !built.Annotations().Cacheable {
		t.Error("Cacheable should be true")
	}
	if string(built.InputSchema().Raw()) != string(inputSchema.Raw()) {
		t.Errorf("InputSchema mismatch")
	}
	if string(built.OutputSchema().Raw()) != string(outputSchema.Raw()) {
		t.Errorf("OutputSchema mismatch")
	}
}
