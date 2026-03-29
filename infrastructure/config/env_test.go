package config

import (
	"os"
	"testing"
)

func TestEnvExpander_SimpleExpansion(t *testing.T) {
	os.Setenv("TEST_VAR", "hello")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bracket syntax",
			input: "${TEST_VAR}",
			want:  "hello",
		},
		{
			name:  "dollar syntax",
			input: "$TEST_VAR",
			want:  "hello",
		},
		{
			name:  "embedded in text",
			input: "prefix-${TEST_VAR}-suffix",
			want:  "prefix-hello-suffix",
		},
		{
			name:  "multiple variables",
			input: "${TEST_VAR} ${TEST_VAR}",
			want:  "hello hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandEnv(tt.input)
			if got != tt.want {
				t.Errorf("ExpandEnv(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEnvExpander_DefaultValue(t *testing.T) {
	os.Unsetenv("UNSET_VAR")
	os.Setenv("SET_VAR", "set-value")
	defer os.Unsetenv("SET_VAR")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unset with default",
			input: "${UNSET_VAR:-default}",
			want:  "default",
		},
		{
			name:  "set with default",
			input: "${SET_VAR:-default}",
			want:  "set-value",
		},
		{
			name:  "empty string default",
			input: "${UNSET_VAR:-}",
			want:  "",
		},
		{
			name:  "complex default",
			input: "${UNSET_VAR:-http://localhost:8080}",
			want:  "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandEnv(tt.input)
			if got != tt.want {
				t.Errorf("ExpandEnv(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEnvExpander_RequiredVariable(t *testing.T) {
	os.Unsetenv("REQUIRED_VAR")

	input := "${REQUIRED_VAR:?variable is required}"
	_, err := ExpandEnvStrict(input)
	if err == nil {
		t.Error("ExpandEnvStrict() should return error for required unset variable")
	}
}

func TestEnvExpander_StrictMode(t *testing.T) {
	os.Unsetenv("MISSING_VAR")

	input := "${MISSING_VAR}"
	_, err := ExpandEnvStrict(input)
	if err == nil {
		t.Error("ExpandEnvStrict() should return error for missing variable")
	}
}

func TestEnvExpander_NonStrictMode(t *testing.T) {
	os.Unsetenv("MISSING_VAR")

	input := "${MISSING_VAR}"
	got := ExpandEnv(input)
	if got != "" {
		t.Errorf("ExpandEnv(%q) = %q, want empty string", input, got)
	}
}

func TestEnvExpander_NoExpansion(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "no variables",
			input: "plain text",
		},
		{
			name:  "escaped dollar",
			input: "price: $100",
		},
		{
			name:  "invalid syntax",
			input: "${incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandEnv(tt.input)
			if got != tt.input {
				t.Errorf("ExpandEnv(%q) = %q, want %q (unchanged)", tt.input, got, tt.input)
			}
		})
	}
}

func TestEnvExpander_YAMLConfig(t *testing.T) {
	os.Setenv("WEBHOOK_URL", "https://hooks.example.com")
	os.Setenv("WEBHOOK_SECRET", "my-secret")
	defer os.Unsetenv("WEBHOOK_URL")
	defer os.Unsetenv("WEBHOOK_SECRET")

	input := `
name: agent
notification:
  endpoints:
    - url: ${WEBHOOK_URL}
      secret: ${WEBHOOK_SECRET}
`
	expected := `
name: agent
notification:
  endpoints:
    - url: https://hooks.example.com
      secret: my-secret
`
	got := ExpandEnv(input)
	if got != expected {
		t.Errorf("ExpandEnv() =\n%s\nwant:\n%s", got, expected)
	}
}
