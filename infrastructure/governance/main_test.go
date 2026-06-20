package governance

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain installs goleak so a Governor whose held run-session goroutine is
// never released (an un-Closed full-delegation kernel governor) fails CI
// instead of silently leaking.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
