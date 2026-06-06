// Package api provides the public API for the agent-go library.
// This file provides configuration-related exports.
package api

import (
	domainconfig "go.klarlabs.de/agent/domain/config"
	infraconfig "go.klarlabs.de/agent/infrastructure/config"
)

// Re-export domain configuration types.
type (
	// AgentConfig represents the complete agent configuration.
	AgentConfig = domainconfig.AgentConfig
	// AgentSettings contains core agent behavior settings.
	AgentSettings = domainconfig.AgentSettings
	// ToolsConfig contains tool-related configuration.
	ToolsConfig = domainconfig.ToolsConfig
	// ToolPackConfig configures a tool pack.
	ToolPackConfig = domainconfig.ToolPackConfig
	// InlineToolConfig defines an inline tool.
	InlineToolConfig = domainconfig.InlineToolConfig
	// ToolAnnotationsConfig configures tool annotations.
	ToolAnnotationsConfig = domainconfig.ToolAnnotationsConfig
	// ToolHandlerConfig specifies how to execute a tool.
	ToolHandlerConfig = domainconfig.ToolHandlerConfig
	// PolicyConfig contains policy settings.
	PolicyConfig = domainconfig.PolicyConfig
	// ApprovalConfig configures approval behavior.
	ApprovalConfig = domainconfig.ApprovalConfig
	// TransitionConfig defines a state transition.
	TransitionConfig = domainconfig.TransitionConfig
	// RateLimitConfigSpec configures rate limiting.
	RateLimitConfigSpec = domainconfig.RateLimitConfig
	// ToolRateLimitConfigSpec configures per-tool rate limiting.
	ToolRateLimitConfigSpec = domainconfig.ToolRateLimitConfig
	// ResilienceConfig contains resilience settings.
	ResilienceConfig = domainconfig.ResilienceConfig
	// RetryConfigSpec configures retry behavior.
	RetryConfigSpec = domainconfig.RetryConfig
	// CircuitBreakerConfigSpec configures circuit breaker behavior.
	CircuitBreakerConfigSpec = domainconfig.CircuitBreakerConfig
	// BulkheadConfigSpec configures bulkhead behavior.
	BulkheadConfigSpec = domainconfig.BulkheadConfig
	// NotificationConfigSpec contains notification settings.
	NotificationConfigSpec = domainconfig.NotificationConfig
	// EndpointConfigSpec configures a webhook endpoint.
	EndpointConfigSpec = domainconfig.EndpointConfig
	// BatchingConfigSpec configures event batching.
	BatchingConfigSpec = domainconfig.BatchingConfig
	// ConfigDuration is a time.Duration that supports JSON/YAML string representation.
	ConfigDuration = domainconfig.Duration

	// ValidationError represents a configuration validation error.
	ValidationError = domainconfig.ValidationError
	// ValidationErrors is a collection of validation errors.
	ValidationErrors = domainconfig.ValidationErrors
)

// Re-export infrastructure configuration types.
type (
	// ConfigLoader loads agent configuration from files.
	ConfigLoader = infraconfig.Loader
	// ConfigBuilder builds engine options from configuration.
	ConfigBuilder = infraconfig.Builder
	// ConfigBuildResult contains the built components from configuration.
	ConfigBuildResult = infraconfig.BuildResult
	// ConfigLoaderOption configures the loader.
	ConfigLoaderOption = infraconfig.LoaderOption
	// JSONSchema represents a JSON Schema document.
	JSONSchema = infraconfig.JSONSchema
)

// Configuration format constants.
const (
	// ConfigFormatYAML is the YAML format.
	ConfigFormatYAML = infraconfig.FormatYAML
	// ConfigFormatJSON is the JSON format.
	ConfigFormatJSON = infraconfig.FormatJSON
)

// Configuration errors.
var (
	// ErrConfigNotFound indicates the configuration file was not found.
	ErrConfigNotFound = domainconfig.ErrConfigNotFound
	// ErrInvalidFormat indicates the configuration format is invalid.
	ErrInvalidFormat = domainconfig.ErrInvalidFormat
	// ErrUnsupportedFormat indicates the file format is not supported.
	ErrUnsupportedFormat = domainconfig.ErrUnsupportedFormat
	// ErrValidationFailed indicates configuration validation failed.
	ErrValidationFailed = domainconfig.ErrValidationFailed
	// ErrEnvExpansionFailed indicates environment variable expansion failed.
	ErrEnvExpansionFailed = domainconfig.ErrEnvExpansionFailed
	// ErrMissingEnvVar indicates a required environment variable is not set.
	ErrMissingEnvVar = domainconfig.ErrMissingEnvVar
	// ErrBuildFailed indicates engine building from config failed.
	ErrBuildFailed = domainconfig.ErrBuildFailed
	// ErrSchemaGenerationFailed indicates JSON schema generation failed.
	ErrSchemaGenerationFailed = domainconfig.ErrSchemaGenerationFailed
)

// NewConfigLoader creates a new configuration loader with default settings.
func NewConfigLoader() *ConfigLoader {
	return infraconfig.NewLoader()
}

// NewConfigLoaderWithOptions creates a loader with the specified options.
func NewConfigLoaderWithOptions(opts ...ConfigLoaderOption) *ConfigLoader {
	return infraconfig.NewLoaderWithOptions(opts...)
}

// ConfigWithEnvExpansion enables or disables environment variable expansion.
func ConfigWithEnvExpansion(enabled bool) ConfigLoaderOption {
	return infraconfig.WithEnvExpansion(enabled)
}

// ConfigWithStrictEnv enables strict environment variable checking.
func ConfigWithStrictEnv(enabled bool) ConfigLoaderOption {
	return infraconfig.WithStrictEnv(enabled)
}

// ConfigWithValidation enables or disables configuration validation.
func ConfigWithValidation(enabled bool) ConfigLoaderOption {
	return infraconfig.WithValidation(enabled)
}

// NewConfigBuilder creates a new configuration builder.
func NewConfigBuilder(config *AgentConfig) *ConfigBuilder {
	return infraconfig.NewBuilder(config)
}

// NewConfigValidator creates a new configuration validator.
func NewConfigValidator() *domainconfig.Validator {
	return domainconfig.NewValidator()
}

// DefaultAgentConfig returns a minimal default configuration.
func DefaultAgentConfig() *AgentConfig {
	return infraconfig.DefaultConfig()
}

// GenerateConfigSchema generates a JSON Schema for the AgentConfig.
func GenerateConfigSchema() *JSONSchema {
	return infraconfig.GenerateSchema()
}

// ConfigSchemaJSON returns the configuration JSON Schema as a JSON string.
func ConfigSchemaJSON() (string, error) {
	return infraconfig.SchemaJSON()
}

// ExpandEnv expands environment variables in a string.
// Supported patterns: ${VAR}, ${VAR:-default}, ${VAR:?error}
func ExpandEnv(input string) string {
	return infraconfig.ExpandEnv(input)
}

// ExpandEnvStrict expands environment variables and returns an error for missing vars.
func ExpandEnvStrict(input string) (string, error) {
	return infraconfig.ExpandEnvStrict(input)
}
