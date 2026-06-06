package api_test

import (
	"os"
	"testing"

	api "go.klarlabs.de/agent/interfaces/api"
)

func TestNewConfigLoader(t *testing.T) {
	t.Parallel()
	loader := api.NewConfigLoader()
	if loader == nil {
		t.Fatal("NewConfigLoader() returned nil")
	}
}

func TestNewConfigLoaderWithOptions(t *testing.T) {
	t.Parallel()
	loader := api.NewConfigLoaderWithOptions(
		api.ConfigWithEnvExpansion(true),
		api.ConfigWithStrictEnv(false),
		api.ConfigWithValidation(true),
	)
	if loader == nil {
		t.Fatal("NewConfigLoaderWithOptions() returned nil")
	}
}

func TestConfigWithEnvExpansion(t *testing.T) {
	t.Parallel()
	opt := api.ConfigWithEnvExpansion(true)
	if opt == nil {
		t.Fatal("ConfigWithEnvExpansion() returned nil")
	}
}

func TestConfigWithStrictEnv(t *testing.T) {
	t.Parallel()
	opt := api.ConfigWithStrictEnv(true)
	if opt == nil {
		t.Fatal("ConfigWithStrictEnv() returned nil")
	}
}

func TestConfigWithValidation(t *testing.T) {
	t.Parallel()
	opt := api.ConfigWithValidation(false)
	if opt == nil {
		t.Fatal("ConfigWithValidation() returned nil")
	}
}

func TestNewConfigBuilder(t *testing.T) {
	t.Parallel()
	cfg := api.DefaultAgentConfig()
	builder := api.NewConfigBuilder(cfg)
	if builder == nil {
		t.Fatal("NewConfigBuilder() returned nil")
	}
}

func TestNewConfigValidator(t *testing.T) {
	t.Parallel()
	v := api.NewConfigValidator()
	if v == nil {
		t.Fatal("NewConfigValidator() returned nil")
	}
}

func TestDefaultAgentConfig(t *testing.T) {
	t.Parallel()
	cfg := api.DefaultAgentConfig()
	if cfg == nil {
		t.Fatal("DefaultAgentConfig() returned nil")
	}
}

func TestGenerateConfigSchema(t *testing.T) {
	t.Parallel()
	schema := api.GenerateConfigSchema()
	if schema == nil {
		t.Fatal("GenerateConfigSchema() returned nil")
	}
}

func TestConfigSchemaJSON(t *testing.T) {
	t.Parallel()
	jsonStr, err := api.ConfigSchemaJSON()
	if err != nil {
		t.Fatalf("ConfigSchemaJSON() error = %v", err)
	}
	if jsonStr == "" {
		t.Fatal("ConfigSchemaJSON() returned empty string")
	}
}

func TestExpandEnv(t *testing.T) {
	t.Parallel()
	os.Setenv("TEST_API_VAR", "hello")
	defer os.Unsetenv("TEST_API_VAR")

	result := api.ExpandEnv("${TEST_API_VAR}")
	if result != "hello" {
		t.Errorf("ExpandEnv() = %q, want %q", result, "hello")
	}
}

func TestExpandEnvStrict(t *testing.T) {
	t.Parallel()

	t.Run("existing var", func(t *testing.T) {
		os.Setenv("TEST_API_STRICT_VAR", "world")
		defer os.Unsetenv("TEST_API_STRICT_VAR")

		result, err := api.ExpandEnvStrict("${TEST_API_STRICT_VAR}")
		if err != nil {
			t.Fatalf("ExpandEnvStrict() error = %v", err)
		}
		if result != "world" {
			t.Errorf("ExpandEnvStrict() = %q, want %q", result, "world")
		}
	})
}
