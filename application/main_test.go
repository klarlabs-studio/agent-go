package application

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain installs goleak so a leaked per-run governance goroutine — e.g. a
// full-delegation kernel governor whose held axi session is never Closed —
// fails CI instead of silently leaking across runs.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
