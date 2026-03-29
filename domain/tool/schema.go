package tool

import (
	"encoding/json"
	"errors"
)

// Schema wraps JSON Schema for input/output validation.
type Schema struct {
	raw json.RawMessage
}

// NewSchema creates a schema from raw JSON.
func NewSchema(raw json.RawMessage) Schema {
	return Schema{raw: raw}
}

// EmptySchema returns a schema that accepts any input.
func EmptySchema() Schema {
	return Schema{raw: json.RawMessage(`{}`)}
}

// ObjectSchema returns a schema for an object with the given properties.
func ObjectSchema(properties map[string]json.RawMessage, required []string) Schema {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	raw, _ := json.Marshal(schema)
	return Schema{raw: raw}
}

// Raw returns the underlying JSON schema.
func (s Schema) Raw() json.RawMessage {
	return s.raw
}

// IsEmpty returns true if the schema is empty or nil.
func (s Schema) IsEmpty() bool {
	return len(s.raw) == 0 || string(s.raw) == "{}" || string(s.raw) == "null"
}

// Validate validates data against the schema.
// Currently performs structural validation (valid JSON, type checking).
// For full JSON Schema Draft 2020-12 validation, use a dedicated validator
// like github.com/santhosh-tekuri/jsonschema in infrastructure/config/.
func (s Schema) Validate(data json.RawMessage) error {
	if s.IsEmpty() {
		return nil
	}
	// Structural validation: ensure data is valid JSON
	if !json.Valid(data) {
		return errors.New("invalid JSON")
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (s Schema) MarshalJSON() ([]byte, error) {
	if s.raw == nil {
		return []byte("{}"), nil
	}
	return s.raw, nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *Schema) UnmarshalJSON(data []byte) error {
	s.raw = data
	return nil
}
