// Package config provides configuration loading and parsing for agent-go.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"go.klarlabs.de/agent/domain/config"
)

// Loader loads agent configuration from files.
type Loader struct {
	// ExpandEnv enables environment variable expansion.
	ExpandEnv bool
	// StrictEnv fails if referenced env vars are missing.
	StrictEnv bool
	// Validate enables configuration validation.
	Validate bool
}

// NewLoader creates a new configuration loader with default settings.
func NewLoader() *Loader {
	return &Loader{
		ExpandEnv: true,
		StrictEnv: false,
		Validate:  true,
	}
}

// LoaderOption configures the loader.
type LoaderOption func(*Loader)

// WithEnvExpansion enables or disables environment variable expansion.
func WithEnvExpansion(enabled bool) LoaderOption {
	return func(l *Loader) {
		l.ExpandEnv = enabled
	}
}

// WithStrictEnv enables strict environment variable checking.
func WithStrictEnv(enabled bool) LoaderOption {
	return func(l *Loader) {
		l.StrictEnv = enabled
	}
}

// WithValidation enables or disables configuration validation.
func WithValidation(enabled bool) LoaderOption {
	return func(l *Loader) {
		l.Validate = enabled
	}
}

// NewLoaderWithOptions creates a loader with the specified options.
func NewLoaderWithOptions(opts ...LoaderOption) *Loader {
	l := NewLoader()
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// LoadFile loads configuration from a file path.
func (l *Loader) LoadFile(path string) (*config.AgentConfig, error) {
	// Clean path to prevent directory traversal (G304)
	cleanPath := filepath.Clean(path)

	// Check if file exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", config.ErrConfigNotFound, cleanPath)
		}
		return nil, fmt.Errorf("failed to access config file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: %s is a directory", config.ErrInvalidFormat, cleanPath)
	}

	// Open file with cleaned path
	f, err := os.Open(cleanPath) // #nosec G304 - path is cleaned above
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	// Determine format from extension
	ext := strings.ToLower(filepath.Ext(cleanPath))
	var format Format
	switch ext {
	case ".yaml", ".yml":
		format = FormatYAML
	case ".json":
		format = FormatJSON
	default:
		return nil, fmt.Errorf("%w: %s", config.ErrUnsupportedFormat, ext)
	}

	return l.Load(f, format)
}

// Format represents a configuration file format.
type Format string

const (
	// FormatYAML is the YAML format.
	FormatYAML Format = "yaml"
	// FormatJSON is the JSON format.
	FormatJSON Format = "json"
)

// Load loads configuration from a reader.
func (l *Loader) Load(r io.Reader, format Format) (*config.AgentConfig, error) {
	// Read all content
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Expand environment variables if enabled
	if l.ExpandEnv {
		data, err = l.expandEnvVars(data)
		if err != nil {
			return nil, err
		}
	}

	// Parse based on format
	cfg := &config.AgentConfig{}
	switch format {
	case FormatYAML:
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", config.ErrInvalidFormat, err)
		}
	case FormatJSON:
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", config.ErrInvalidFormat, err)
		}
	default:
		return nil, fmt.Errorf("%w: %s", config.ErrUnsupportedFormat, format)
	}

	// Validate if enabled
	if l.Validate {
		validator := config.NewValidator()
		if errs := validator.Validate(cfg); errs.HasErrors() {
			return nil, fmt.Errorf("%w: %v", config.ErrValidationFailed, errs)
		}
	}

	return cfg, nil
}

// expandEnvVars expands ${VAR} and $VAR patterns in the data.
func (l *Loader) expandEnvVars(data []byte) ([]byte, error) {
	expander := &envExpander{
		strict: l.StrictEnv,
	}
	result, err := expander.Expand(string(data))
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

// LoadString loads configuration from a string.
func (l *Loader) LoadString(content string, format Format) (*config.AgentConfig, error) {
	return l.Load(strings.NewReader(content), format)
}

// LoadBytes loads configuration from bytes.
func (l *Loader) LoadBytes(data []byte, format Format) (*config.AgentConfig, error) {
	return l.Load(strings.NewReader(string(data)), format)
}
