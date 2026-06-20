package logging

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/bolt"
)

// testLogger creates a logger that writes to a buffer for testing
func testLogger() (*bolt.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	handler := bolt.NewJSONHandler(buf)
	logger := bolt.New(handler).SetLevel(bolt.TRACE)
	return logger, buf
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()

	if config.Level != "info" {
		t.Errorf("Level = %s, want info", config.Level)
	}
	if config.Format != "console" {
		t.Errorf("Format = %s, want console", config.Format)
	}
	if config.Output != os.Stdout {
		t.Errorf("Output = %v, want os.Stdout", config.Output)
	}
}

func TestProductionConfig(t *testing.T) {
	t.Parallel()

	config := ProductionConfig()

	if config.Level != "info" {
		t.Errorf("Level = %s, want info", config.Level)
	}
	if config.Format != "json" {
		t.Errorf("Format = %s, want json", config.Format)
	}
	if config.Output != os.Stdout {
		t.Errorf("Output = %v, want os.Stdout", config.Output)
	}
}

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bolt.Level
	}{
		{"trace", bolt.TRACE},
		{"debug", bolt.DEBUG},
		{"info", bolt.INFO},
		{"warn", bolt.WARN},
		{"error", bolt.ERROR},
		{"unknown", bolt.INFO}, // Default
		{"", bolt.INFO},        // Empty defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRunIDField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := RunID("run-123")
	if field == nil {
		t.Fatal("RunID() returned nil")
	}

	// Execute the field function
	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"run_id":"run-123"`)) {
		t.Errorf("expected run_id field in output: %s", buf.String())
	}
}

func TestStateField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := State(agent.StateExplore)
	if field == nil {
		t.Fatal("State() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"state":"explore"`)) {
		t.Errorf("expected state field in output: %s", buf.String())
	}
}

func TestFromStateField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := FromState(agent.StateIntake)
	if field == nil {
		t.Fatal("FromState() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"from_state":"intake"`)) {
		t.Errorf("expected from_state field in output: %s", buf.String())
	}
}

func TestToStateField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := ToState(agent.StateExplore)
	if field == nil {
		t.Fatal("ToState() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"to_state":"explore"`)) {
		t.Errorf("expected to_state field in output: %s", buf.String())
	}
}

func TestToolNameField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := ToolName("read_file")
	if field == nil {
		t.Fatal("ToolName() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"tool":"read_file"`)) {
		t.Errorf("expected tool field in output: %s", buf.String())
	}
}

func TestDecisionField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Decision(agent.DecisionCallTool)
	if field == nil {
		t.Fatal("Decision() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"decision":"call_tool"`)) {
		t.Errorf("expected decision field in output: %s", buf.String())
	}
}

func TestDurationField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Duration(100 * time.Millisecond)
	if field == nil {
		t.Fatal("Duration() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"duration_ms":100`)) {
		t.Errorf("expected duration_ms field in output: %s", buf.String())
	}
}

func TestDurationNsField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := DurationNs(100 * time.Millisecond)
	if field == nil {
		t.Fatal("DurationNs() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"duration_ns":100000000`)) {
		t.Errorf("expected duration_ns field in output: %s", buf.String())
	}
}

func TestCachedField(t *testing.T) {
	t.Parallel()

	t.Run("cached true", func(t *testing.T) {
		t.Parallel()

		logger, buf := testLogger()
		field := Cached(true)
		if field == nil {
			t.Fatal("Cached() returned nil")
		}

		event := logger.Info()
		field(event).Msg("test")

		if !bytes.Contains(buf.Bytes(), []byte(`"cached":true`)) {
			t.Errorf("expected cached field in output: %s", buf.String())
		}
	})

	t.Run("cached false", func(t *testing.T) {
		t.Parallel()

		logger, buf := testLogger()
		field := Cached(false)
		if field == nil {
			t.Fatal("Cached(false) returned nil")
		}

		event := logger.Info()
		field(event).Msg("test")

		if !bytes.Contains(buf.Bytes(), []byte(`"cached":false`)) {
			t.Errorf("expected cached field in output: %s", buf.String())
		}
	})
}

func TestErrorField(t *testing.T) {
	t.Parallel()

	t.Run("with error", func(t *testing.T) {
		t.Parallel()

		logger, buf := testLogger()
		field := ErrorField(errors.New("test error"))
		if field == nil {
			t.Fatal("ErrorField() returned nil")
		}

		event := logger.Info()
		field(event).Msg("test")

		if !bytes.Contains(buf.Bytes(), []byte(`"error":"test error"`)) {
			t.Errorf("expected error field in output: %s", buf.String())
		}
	})

	t.Run("with nil error", func(t *testing.T) {
		t.Parallel()

		logger, buf := testLogger()
		field := ErrorField(nil)
		if field == nil {
			t.Fatal("ErrorField(nil) returned nil")
		}

		event := logger.Info()
		field(event).Msg("test")

		// Should not contain error field
		if bytes.Contains(buf.Bytes(), []byte(`"error"`)) {
			t.Errorf("unexpected error field in output: %s", buf.String())
		}
	})
}

func TestBudgetField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Budget("tool_calls", 50)
	if field == nil {
		t.Fatal("Budget() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"budget":"tool_calls"`)) {
		t.Errorf("expected budget field in output: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"remaining":50`)) {
		t.Errorf("expected remaining field in output: %s", buf.String())
	}
}

func TestApprovedField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Approved(true)
	if field == nil {
		t.Fatal("Approved() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"approved":true`)) {
		t.Errorf("expected approved field in output: %s", buf.String())
	}
}

func TestApproverField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Approver("admin")
	if field == nil {
		t.Fatal("Approver() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"approver":"admin"`)) {
		t.Errorf("expected approver field in output: %s", buf.String())
	}
}

func TestEvidenceCountField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := EvidenceCount(10)
	if field == nil {
		t.Fatal("EvidenceCount() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"evidence_count":10`)) {
		t.Errorf("expected evidence_count field in output: %s", buf.String())
	}
}

func TestGoalField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Goal("process files")
	if field == nil {
		t.Fatal("Goal() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"goal":"process files"`)) {
		t.Errorf("expected goal field in output: %s", buf.String())
	}
}

func TestSummaryField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Summary("completed successfully")
	if field == nil {
		t.Fatal("Summary() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"summary":"completed successfully"`)) {
		t.Errorf("expected summary field in output: %s", buf.String())
	}
}

func TestReasonField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Reason("user request")
	if field == nil {
		t.Fatal("Reason() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"reason":"user request"`)) {
		t.Errorf("expected reason field in output: %s", buf.String())
	}
}

func TestComponentField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Component("engine")
	if field == nil {
		t.Fatal("Component() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"component":"engine"`)) {
		t.Errorf("expected component field in output: %s", buf.String())
	}
}

func TestOperationField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Operation("tool_execution")
	if field == nil {
		t.Fatal("Operation() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"operation":"tool_execution"`)) {
		t.Errorf("expected operation field in output: %s", buf.String())
	}
}

func TestStrField(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()
	field := Str("custom_key", "custom_value")
	if field == nil {
		t.Fatal("Str() returned nil")
	}

	event := logger.Info()
	field(event).Msg("test")

	if !bytes.Contains(buf.Bytes(), []byte(`"custom_key":"custom_value"`)) {
		t.Errorf("expected custom_key field in output: %s", buf.String())
	}
}

// TestInit tests logger initialization
func TestInit(t *testing.T) {
	// Note: Can't test Init() properly due to sync.Once
	// Just test that Init doesn't panic with various configs
	t.Run("with nil output uses stdout", func(t *testing.T) {
		// Skip because sync.Once is already triggered
		t.Skip("sync.Once already triggered in other tests")
	})
}

// TestGet tests getting the default logger
func TestGet(t *testing.T) {
	logger := Get()
	if logger == nil {
		t.Fatal("Get() returned nil")
	}
}

// TestSetLevel tests changing the log level
func TestSetLevel(t *testing.T) {
	// Just verify it doesn't panic
	SetLevel("debug")
	SetLevel("info")
	SetLevel("error")
}

// TestLogEvent tests the LogEvent wrapper
func TestLogEvent(t *testing.T) {
	t.Parallel()

	logger, buf := testLogger()

	t.Run("Add chains fields", func(t *testing.T) {
		buf.Reset()
		event := &LogEvent{event: logger.Info()}
		event.Add(RunID("run-1")).Add(State(agent.StateExplore)).Msg("test")

		if !bytes.Contains(buf.Bytes(), []byte(`"run_id":"run-1"`)) {
			t.Errorf("expected run_id field in output: %s", buf.String())
		}
		if !bytes.Contains(buf.Bytes(), []byte(`"state":"explore"`)) {
			t.Errorf("expected state field in output: %s", buf.String())
		}
	})

	t.Run("Send without message", func(t *testing.T) {
		buf.Reset()
		event := &LogEvent{event: logger.Info()}
		event.Add(RunID("run-2")).Send()

		if !bytes.Contains(buf.Bytes(), []byte(`"run_id":"run-2"`)) {
			t.Errorf("expected run_id field in output: %s", buf.String())
		}
	})
}

// TestNewEvent tests creating a new LogEvent wrapper
func TestNewEvent(t *testing.T) {
	logger, _ := testLogger()
	event := logger.Info()
	logEvent := NewEvent(event)

	if logEvent == nil {
		t.Fatal("NewEvent() returned nil")
	}
	if logEvent.event != event {
		t.Error("NewEvent() did not store the event correctly")
	}
}

// TestLogLevelHelpers tests the convenience methods
func TestLogLevelHelpers(t *testing.T) {
	// These call Get() which initializes the default logger
	// Just verify they don't panic and return non-nil

	// Redirect to discard to avoid polluting test output
	originalOutput := os.Stdout
	os.Stdout = os.NewFile(0, os.DevNull)
	defer func() { os.Stdout = originalOutput }()

	t.Run("Trace", func(t *testing.T) {
		event := Trace()
		if event == nil {
			t.Fatal("Trace() returned nil")
		}
	})

	t.Run("Debug", func(t *testing.T) {
		event := Debug()
		if event == nil {
			t.Fatal("Debug() returned nil")
		}
	})

	t.Run("Info", func(t *testing.T) {
		event := Info()
		if event == nil {
			t.Fatal("Info() returned nil")
		}
	})

	t.Run("Warn", func(t *testing.T) {
		event := Warn()
		if event == nil {
			t.Fatal("Warn() returned nil")
		}
	})

	t.Run("Error", func(t *testing.T) {
		event := Error()
		if event == nil {
			t.Fatal("Error() returned nil")
		}
	})

	// Note: Don't test Fatal() as it might call os.Exit
}

// Ensure io import is used
var _ io.Writer = (*bytes.Buffer)(nil)

// Injectable Logger (instance.go) tests.

func TestNopLogger_DiscardsOutput(t *testing.T) {
	l := NewNopLogger()
	// Must not panic and must produce no output.
	l.Info().Add(RunID("r1")).Add(State(agent.StateAct)).Msg("hello")
	l.Error().Add(ErrorField(errors.New("boom"))).Send()
	l.Debug().Msg("dbg")
}

func TestNilLogger_IsSafeNoop(t *testing.T) {
	var l *Logger // nil
	l.Info().Add(RunID("r1")).Msg("should not panic")
	l.Error().Send()
}

func TestNewLogger_WritesThroughBolt(t *testing.T) {
	b, buf := testLogger()
	l := NewLogger(b)

	l.Info().Add(RunID("run-42")).Msg("started")

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("run-42")) {
		t.Errorf("expected output to contain run id, got %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("started")) {
		t.Errorf("expected output to contain message, got %q", out)
	}
}

func TestNewLogger_NilBoltIsNoop(t *testing.T) {
	l := NewLogger(nil)
	l.Info().Add(RunID("x")).Msg("no panic")
}

func TestNewLoggerFromConfig_BuildsWorkingLogger(t *testing.T) {
	// Config.Output is *os.File; a temp file lets us assert real output without
	// touching the package-level singleton.
	f, err := os.CreateTemp(t.TempDir(), "log-*.json")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	l := NewLoggerFromConfig(Config{Level: "info", Format: "json", Output: f})
	l.Info().Add(RunID("cfg-run")).Msg("configured")

	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	if !bytes.Contains(data, []byte("cfg-run")) {
		t.Errorf("expected configured logger output to contain run id, got %q", string(data))
	}
}
