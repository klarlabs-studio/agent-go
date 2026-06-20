package logging

import "go.klarlabs.de/bolt"

// Logger is an injectable structured logger. It wraps an optional *bolt.Logger
// so the runtime never depends on the package-level global singleton.
//
// A nil *Logger, or a Logger whose underlying bolt.Logger is nil, is a safe
// no-op: every level method returns a LogEvent that discards its fields and
// message. This makes the logger fully injectable — the engine holds a
// *Logger field and callers opt in via api.WithLogger; when unset the engine
// constructs a no-op logger and emits nothing.
type Logger struct {
	bolt *bolt.Logger
}

// NewLogger wraps a bolt.Logger for injection. A nil bolt.Logger yields a
// no-op logger.
func NewLogger(b *bolt.Logger) *Logger {
	return &Logger{bolt: b}
}

// NewNopLogger returns a logger that discards all output. This is the default
// used by the engine when no logger is injected, keeping the execution path
// free of the package-level global.
func NewNopLogger() *Logger {
	return &Logger{bolt: nil}
}

// NewLoggerFromConfig builds a bolt-backed logger from a Config without
// touching the package-level singleton. Useful for api.WithLogger callers who
// want a configured logger rather than supplying their own bolt.Logger.
func NewLoggerFromConfig(config Config) *Logger {
	return &Logger{bolt: buildLogger(config)}
}

// SetGlobalSinkForTest replaces the package-level default logger (the singleton
// returned by Get and used by the package-level Info/Error/Debug/Warn helpers)
// with the given bolt.Logger, returning a function that restores the previous
// default. It exists so tests can deterministically capture whether ANYTHING
// reaches the global sink — the production non-negotiable is that the execution
// path never does. It is a test-only seam; production code must never call it.
//
// It also marks the init sync.Once as consumed so a later Get does not rebuild
// over the test sink.
func SetGlobalSinkForTest(b *bolt.Logger) (restore func()) {
	once.Do(func() {}) // consume the Once so Get won't overwrite the test sink
	prev := defaultLogger
	defaultLogger = b
	return func() { defaultLogger = prev }
}

// event returns a LogEvent for the given level, or a no-op LogEvent when the
// logger has no underlying bolt.Logger.
func (l *Logger) event(level bolt.Level) *LogEvent {
	if l == nil || l.bolt == nil {
		return &LogEvent{event: nil}
	}
	switch level {
	case bolt.TRACE:
		return &LogEvent{event: l.bolt.Trace()}
	case bolt.DEBUG:
		return &LogEvent{event: l.bolt.Debug()}
	case bolt.INFO:
		return &LogEvent{event: l.bolt.Info()}
	case bolt.WARN:
		return &LogEvent{event: l.bolt.Warn()}
	case bolt.ERROR:
		return &LogEvent{event: l.bolt.Error()}
	case bolt.FATAL:
		return &LogEvent{event: l.bolt.Fatal()}
	default:
		return &LogEvent{event: l.bolt.Info()}
	}
}

// Trace returns a trace-level LogEvent.
func (l *Logger) Trace() *LogEvent { return l.event(bolt.TRACE) }

// Debug returns a debug-level LogEvent.
func (l *Logger) Debug() *LogEvent { return l.event(bolt.DEBUG) }

// Info returns an info-level LogEvent.
func (l *Logger) Info() *LogEvent { return l.event(bolt.INFO) }

// Warn returns a warn-level LogEvent.
func (l *Logger) Warn() *LogEvent { return l.event(bolt.WARN) }

// Error returns an error-level LogEvent.
func (l *Logger) Error() *LogEvent { return l.event(bolt.ERROR) }

// Fatal returns a fatal-level LogEvent.
func (l *Logger) Fatal() *LogEvent { return l.event(bolt.FATAL) }
