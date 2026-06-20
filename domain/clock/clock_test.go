package clock_test

import (
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/clock"
	"go.klarlabs.de/statekit"
)

func TestSystemClock_NowAdvances(t *testing.T) {
	c := clock.System()
	a := c.Now()
	if a.IsZero() {
		t.Fatal("system clock returned zero time")
	}
}

func TestFixedClock_IsDeterministic(t *testing.T) {
	anchor := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	c := clock.Fixed(anchor)

	if !c.Now().Equal(anchor) {
		t.Errorf("expected %v, got %v", anchor, c.Now())
	}
	// Repeated reads return the same instant.
	if !c.Now().Equal(c.Now()) {
		t.Error("fixed clock must be stable across reads")
	}
}

func TestFixedClock_AdvanceAndSet(t *testing.T) {
	anchor := time.Unix(1_000, 0).UTC()
	c := clock.Fixed(anchor)

	got := c.Advance(5 * time.Second)
	want := anchor.Add(5 * time.Second)
	if !got.Equal(want) {
		t.Errorf("Advance: expected %v, got %v", want, got)
	}
	if !c.Now().Equal(want) {
		t.Errorf("Now after Advance: expected %v, got %v", want, c.Now())
	}

	c.Set(anchor)
	if !c.Now().Equal(anchor) {
		t.Errorf("Set: expected %v, got %v", anchor, c.Now())
	}
}

// TestStatekitFakeClock_SatisfiesClock proves a statekit *FakeClock can be
// injected directly as a clock.Clock, unifying the timer and timestamp clocks.
func TestStatekitFakeClock_SatisfiesClock(t *testing.T) {
	anchor := time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)
	var c clock.Clock = statekit.NewFakeClock(anchor)
	if !c.Now().Equal(anchor) {
		t.Errorf("expected fake clock now %v, got %v", anchor, c.Now())
	}
}
