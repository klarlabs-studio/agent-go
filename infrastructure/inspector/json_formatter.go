// Package inspector provides inspector infrastructure implementations.
package inspector

import (
	"encoding/json"

	"go.klarlabs.de/agent/domain/inspector"
)

// JSONFormatter formats data as JSON.
type JSONFormatter struct {
	pretty bool
}

// JSONFormatterOption configures the JSON formatter.
type JSONFormatterOption func(*JSONFormatter)

// WithPrettyPrint enables pretty-printed output.
func WithPrettyPrint() JSONFormatterOption {
	return func(f *JSONFormatter) {
		f.pretty = true
	}
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter(opts ...JSONFormatterOption) *JSONFormatter {
	f := &JSONFormatter{}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Format formats the data as JSON.
func (f *JSONFormatter) Format(data any) ([]byte, error) {
	if f.pretty {
		return json.MarshalIndent(data, "", "  ")
	}
	return json.Marshal(data)
}

// FormatType returns the format type.
func (f *JSONFormatter) FormatType() inspector.ExportFormat {
	return inspector.FormatJSON
}

// Ensure JSONFormatter implements inspector.Formatter
var _ inspector.Formatter = (*JSONFormatter)(nil)
