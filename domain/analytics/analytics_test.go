package analytics

import "testing"

func TestAnalyticsInterface(t *testing.T) {
	// Verify Analytics interface is defined and usable as a type constraint
	var _ Analytics = nil
	_ = t // suppress unused
}
