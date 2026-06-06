// Package logging provides structured logging using bolt.
package logging

import (
	"os"
	"sync"

	"go.klarlabs.de/bolt"
)

var (
	defaultLogger *bolt.Logger
	once          sync.Once
)

// Config configures the logger.
type Config struct {
	// Level is the minimum log level (trace, debug, info, warn, error).
	Level string

	// Format is the output format (json or console).
	Format string

	// NoColor disables color output for console format.
	NoColor bool

	// Output is the output destination.
	Output *os.File
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Level:  "info",
		Format: "console",
		Output: os.Stdout,
	}
}

// ProductionConfig returns a production-ready configuration.
func ProductionConfig() Config {
	return Config{
		Level:  "info",
		Format: "json",
		Output: os.Stdout,
	}
}

// parseLevel converts a string level to bolt.Level.
func parseLevel(s string) bolt.Level {
	switch s {
	case "trace":
		return bolt.TRACE
	case "debug":
		return bolt.DEBUG
	case "info":
		return bolt.INFO
	case "warn":
		return bolt.WARN
	case "error":
		return bolt.ERROR
	default:
		return bolt.INFO
	}
}

// Init initializes the default logger with the given configuration.
func Init(config Config) {
	once.Do(func() {
		output := config.Output
		if output == nil {
			output = os.Stdout
		}

		var handler bolt.Handler
		if config.Format == "json" {
			handler = bolt.NewJSONHandler(output)
		} else {
			handler = bolt.NewConsoleHandler(output)
		}

		defaultLogger = bolt.New(handler).SetLevel(parseLevel(config.Level))
	})
}

// Get returns the default logger, initializing if necessary.
func Get() *bolt.Logger {
	// Always call Init - it's safe because of sync.Once
	Init(DefaultConfig())
	return defaultLogger
}

// SetLevel changes the log level of the default logger.
func SetLevel(level string) {
	Get().SetLevel(parseLevel(level))
}

// LogEvent is a wrapper that allows adding Fields to a bolt.Event.
type LogEvent struct {
	event *bolt.Event
}

// NewEvent wraps a bolt.Event for field application.
func NewEvent(e *bolt.Event) *LogEvent {
	return &LogEvent{event: e}
}

// Add applies a field to the event and returns the wrapper for chaining.
func (l *LogEvent) Add(f Field) *LogEvent {
	l.event = f(l.event)
	return l
}

// Msg sends the log event with a message.
func (l *LogEvent) Msg(msg string) {
	l.event.Msg(msg)
}

// Send sends the log event without a message.
func (l *LogEvent) Send() {
	l.event.Send()
}

// Convenience methods that return LogEvent for field chaining.

// Trace returns a LogEvent wrapper for trace level logging.
func Trace() *LogEvent {
	return &LogEvent{event: Get().Trace()}
}

// Debug returns a LogEvent wrapper for debug level logging.
func Debug() *LogEvent {
	return &LogEvent{event: Get().Debug()}
}

// Info returns a LogEvent wrapper for info level logging.
func Info() *LogEvent {
	return &LogEvent{event: Get().Info()}
}

// Warn returns a LogEvent wrapper for warn level logging.
func Warn() *LogEvent {
	return &LogEvent{event: Get().Warn()}
}

// Error returns a LogEvent wrapper for error level logging.
func Error() *LogEvent {
	return &LogEvent{event: Get().Error()}
}

// Fatal returns a LogEvent wrapper for fatal level logging.
func Fatal() *LogEvent {
	return &LogEvent{event: Get().Fatal()}
}
