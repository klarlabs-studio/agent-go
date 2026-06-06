package tool_test

import (
	"encoding/json"
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestNewSchema(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"type": "string"}`)
	schema := tool.NewSchema(raw)

	if string(schema.Raw()) != string(raw) {
		t.Errorf("Raw() = %s, want %s", schema.Raw(), raw)
	}
}

func TestEmptySchema(t *testing.T) {
	t.Parallel()

	schema := tool.EmptySchema()

	if string(schema.Raw()) != "{}" {
		t.Errorf("Raw() = %s, want {}", schema.Raw())
	}
	if !schema.IsEmpty() {
		t.Error("IsEmpty() should return true for empty schema")
	}
}

func TestObjectSchema(t *testing.T) {
	t.Parallel()

	t.Run("with properties and required", func(t *testing.T) {
		t.Parallel()

		properties := map[string]json.RawMessage{
			"name": json.RawMessage(`{"type": "string"}`),
			"age":  json.RawMessage(`{"type": "integer"}`),
		}
		required := []string{"name"}

		schema := tool.ObjectSchema(properties, required)

		var parsed map[string]interface{}
		if err := json.Unmarshal(schema.Raw(), &parsed); err != nil {
			t.Fatalf("Failed to parse schema: %v", err)
		}

		if parsed["type"] != "object" {
			t.Errorf("type = %v, want object", parsed["type"])
		}

		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("properties should be a map")
		}
		if len(props) != 2 {
			t.Errorf("properties count = %d, want 2", len(props))
		}

		req, ok := parsed["required"].([]interface{})
		if !ok {
			t.Fatal("required should be an array")
		}
		if len(req) != 1 {
			t.Errorf("required count = %d, want 1", len(req))
		}
	})

	t.Run("without required", func(t *testing.T) {
		t.Parallel()

		properties := map[string]json.RawMessage{
			"value": json.RawMessage(`{"type": "string"}`),
		}

		schema := tool.ObjectSchema(properties, nil)

		var parsed map[string]interface{}
		if err := json.Unmarshal(schema.Raw(), &parsed); err != nil {
			t.Fatalf("Failed to parse schema: %v", err)
		}

		if _, exists := parsed["required"]; exists {
			t.Error("required should not be present when empty")
		}
	})
}

func TestSchema_Raw(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"type": "number", "minimum": 0}`)
	schema := tool.NewSchema(raw)

	got := schema.Raw()
	if string(got) != string(raw) {
		t.Errorf("Raw() = %s, want %s", got, raw)
	}
}

func TestSchema_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema tool.Schema
		want   bool
	}{
		{
			name:   "empty braces",
			schema: tool.NewSchema(json.RawMessage(`{}`)),
			want:   true,
		},
		{
			name:   "null",
			schema: tool.NewSchema(json.RawMessage(`null`)),
			want:   true,
		},
		{
			name:   "nil raw",
			schema: tool.NewSchema(nil),
			want:   true,
		},
		{
			name:   "non-empty schema",
			schema: tool.NewSchema(json.RawMessage(`{"type": "string"}`)),
			want:   false,
		},
		{
			name:   "object schema",
			schema: tool.ObjectSchema(map[string]json.RawMessage{"a": json.RawMessage(`{}`)}, nil),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.schema.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSchema_Validate(t *testing.T) {
	t.Parallel()

	t.Run("empty schema accepts anything", func(t *testing.T) {
		t.Parallel()

		schema := tool.EmptySchema()
		if err := schema.Validate(json.RawMessage(`{"any": "data"}`)); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("valid json passes", func(t *testing.T) {
		t.Parallel()

		schema := tool.NewSchema(json.RawMessage(`{"type": "object"}`))
		if err := schema.Validate(json.RawMessage(`{"valid": "json"}`)); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("invalid json fails", func(t *testing.T) {
		t.Parallel()

		schema := tool.NewSchema(json.RawMessage(`{"type": "object"}`))
		err := schema.Validate(json.RawMessage(`{invalid`))
		if err == nil {
			t.Error("Validate() should return error for invalid JSON")
		}
	})
}

func TestSchema_MarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("with data", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"type":"string"}`)
		schema := tool.NewSchema(raw)

		data, err := json.Marshal(schema)
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}
		if string(data) != string(raw) {
			t.Errorf("MarshalJSON() = %s, want %s", data, raw)
		}
	})

	t.Run("nil schema", func(t *testing.T) {
		t.Parallel()

		schema := tool.NewSchema(nil)

		data, err := json.Marshal(schema)
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}
		if string(data) != "{}" {
			t.Errorf("MarshalJSON() = %s, want {}", data)
		}
	})
}

func TestSchema_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("valid json", func(t *testing.T) {
		t.Parallel()

		var schema tool.Schema
		data := []byte(`{"type": "integer"}`)

		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}
		if string(schema.Raw()) != string(data) {
			t.Errorf("Raw() = %s, want %s", schema.Raw(), data)
		}
	})

	t.Run("empty object", func(t *testing.T) {
		t.Parallel()

		var schema tool.Schema
		data := []byte(`{}`)

		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}
		if !schema.IsEmpty() {
			t.Error("IsEmpty() should return true after unmarshaling {}")
		}
	})
}

func TestSchema_InStruct(t *testing.T) {
	t.Parallel()

	type Config struct {
		Name   string      `json:"name"`
		Schema tool.Schema `json:"schema"`
	}

	t.Run("marshal and unmarshal", func(t *testing.T) {
		t.Parallel()

		config := Config{
			Name:   "test",
			Schema: tool.NewSchema(json.RawMessage(`{"type":"boolean"}`)),
		}

		data, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}

		var parsed Config
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}

		if parsed.Name != config.Name {
			t.Errorf("Name = %s, want %s", parsed.Name, config.Name)
		}
		if string(parsed.Schema.Raw()) != string(config.Schema.Raw()) {
			t.Errorf("Schema = %s, want %s", parsed.Schema.Raw(), config.Schema.Raw())
		}
	})
}
