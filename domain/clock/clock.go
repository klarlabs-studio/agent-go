// Package clock provides a small time abstraction so the runtime can be made
// deterministic in tests and during replay.
//
// The runtime stamps run IDs, run start times, and event timestamps from an
// injected Clock instead of calling time.Now directly. Production uses
// System(); tests inject a fixed or fake clock (statekit's *FakeClock
// satisfies this interface) so replayed and forked runs are reproducible.
package clock

import "time"

// Clock abstracts the source of the current wall-clock time.
//
// It is intentionally minimal — a single Now method — so that statekit's
// *FakeClock (which exposes Now) can be injected directly, unifying the
// timer clock and the timestamp clock used across the runtime.
type Clock interface {
	// Now returns the current time according to the clock.
	Now() time.Time
}

// systemClock is the default production clock backed by time.Now.
type systemClock struct{}

// Now returns the current wall-clock time.
func (systemClock) Now() time.Time { return time.Now() }

// System returns the default Clock backed by time.Now.
func System() Clock { return systemClock{} }

// FixedClock is a deterministic Clock that always returns the same instant.
// It is useful for tests that assert exact timestamps.
type FixedClock struct {
	t time.Time
}

// Fixed returns a Clock that always reports t.
func Fixed(t time.Time) *FixedClock { return &FixedClock{t: t} }

// Now returns the fixed instant.
func (c *FixedClock) Now() time.Time { return c.t }

// Set updates the instant the clock reports.
func (c *FixedClock) Set(t time.Time) { c.t = t }

// Advance moves the fixed clock forward by d and returns the new instant.
func (c *FixedClock) Advance(d time.Duration) time.Time {
	c.t = c.t.Add(d)
	return c.t
}
