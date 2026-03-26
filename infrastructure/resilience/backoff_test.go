package resilience

import (
	"testing"
	"time"
)

func TestExponentialBackoff_Delays(t *testing.T) {
	t.Parallel()

	b := &ExponentialBackoff{
		InitialDelay: 100 * time.Millisecond,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
	}

	for _, tc := range tests {
		delay, ok := b.Next(tc.attempt)
		if !ok {
			t.Fatalf("attempt %d: expected ok=true", tc.attempt)
		}
		if delay != tc.want {
			t.Errorf("attempt %d: got %v, want %v", tc.attempt, delay, tc.want)
		}
	}
}

func TestExponentialBackoff_DefaultMultiplier(t *testing.T) {
	t.Parallel()

	b := &ExponentialBackoff{InitialDelay: 50 * time.Millisecond}

	delay, ok := b.Next(1)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Default multiplier is 2.0: 50ms * 2^1 = 100ms
	if delay != 100*time.Millisecond {
		t.Errorf("got %v, want 100ms", delay)
	}
}

func TestExponentialBackoff_MaxDelay(t *testing.T) {
	t.Parallel()

	b := &ExponentialBackoff{
		InitialDelay: 100 * time.Millisecond,
		Multiplier:   2.0,
		MaxDelay:     500 * time.Millisecond,
	}

	// attempt 3 would be 800ms, but capped at 500ms
	delay, ok := b.Next(3)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if delay != 500*time.Millisecond {
		t.Errorf("got %v, want 500ms", delay)
	}
}

func TestExponentialBackoff_MaxRetries(t *testing.T) {
	t.Parallel()

	b := &ExponentialBackoff{
		InitialDelay: 10 * time.Millisecond,
		MaxRetries:   3,
	}

	for i := 0; i < 3; i++ {
		_, ok := b.Next(i)
		if !ok {
			t.Fatalf("attempt %d: should be ok", i)
		}
	}

	_, ok := b.Next(3)
	if ok {
		t.Error("attempt 3: should return ok=false when MaxRetries=3")
	}
}

func TestExponentialBackoff_MaxDuration(t *testing.T) {
	t.Parallel()

	b := &ExponentialBackoff{
		InitialDelay: 100 * time.Millisecond,
		Multiplier:   2.0,
		MaxDuration:  350 * time.Millisecond,
	}

	// attempt 0: 100ms, elapsed=100ms -> ok
	_, ok := b.Next(0)
	if !ok {
		t.Fatal("attempt 0: should be ok")
	}

	// attempt 1: 200ms, elapsed=300ms -> ok
	_, ok = b.Next(1)
	if !ok {
		t.Fatal("attempt 1: should be ok")
	}

	// attempt 2: 400ms, elapsed would be 700ms > 350ms -> not ok
	_, ok = b.Next(2)
	if ok {
		t.Error("attempt 2: should return ok=false (exceeds MaxDuration)")
	}
}

func TestExponentialBackoff_Reset(t *testing.T) {
	t.Parallel()

	b := &ExponentialBackoff{
		InitialDelay: 100 * time.Millisecond,
		MaxDuration:  150 * time.Millisecond,
	}

	// consume the budget
	b.Next(0) // 100ms elapsed

	// attempt 1 would be 200ms, total 300ms > 150ms
	_, ok := b.Next(1)
	if ok {
		t.Error("should be exhausted before reset")
	}

	b.Reset()

	// after reset, attempt 0 should work again
	_, ok = b.Next(0)
	if !ok {
		t.Error("should be ok after Reset")
	}
}

func TestLinearBackoff_Delays(t *testing.T) {
	t.Parallel()

	b := &LinearBackoff{
		InitialDelay: 100 * time.Millisecond,
		Increment:    50 * time.Millisecond,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 150 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 250 * time.Millisecond},
	}

	for _, tc := range tests {
		delay, ok := b.Next(tc.attempt)
		if !ok {
			t.Fatalf("attempt %d: expected ok=true", tc.attempt)
		}
		if delay != tc.want {
			t.Errorf("attempt %d: got %v, want %v", tc.attempt, delay, tc.want)
		}
	}
}

func TestLinearBackoff_MaxDelay(t *testing.T) {
	t.Parallel()

	b := &LinearBackoff{
		InitialDelay: 100 * time.Millisecond,
		Increment:    100 * time.Millisecond,
		MaxDelay:     250 * time.Millisecond,
	}

	// attempt 2: 100 + 200 = 300ms, capped at 250ms
	delay, _ := b.Next(2)
	if delay != 250*time.Millisecond {
		t.Errorf("got %v, want 250ms", delay)
	}
}

func TestLinearBackoff_MaxRetries(t *testing.T) {
	t.Parallel()

	b := &LinearBackoff{
		InitialDelay: 10 * time.Millisecond,
		Increment:    10 * time.Millisecond,
		MaxRetries:   2,
	}

	_, ok := b.Next(0)
	if !ok {
		t.Fatal("attempt 0: should be ok")
	}
	_, ok = b.Next(1)
	if !ok {
		t.Fatal("attempt 1: should be ok")
	}
	_, ok = b.Next(2)
	if ok {
		t.Error("attempt 2: should return ok=false")
	}
}

func TestLinearBackoff_MaxDuration(t *testing.T) {
	t.Parallel()

	b := &LinearBackoff{
		InitialDelay: 100 * time.Millisecond,
		Increment:    100 * time.Millisecond,
		MaxDuration:  250 * time.Millisecond,
	}

	// attempt 0: 100ms, elapsed=100ms
	_, ok := b.Next(0)
	if !ok {
		t.Fatal("attempt 0: should be ok")
	}

	// attempt 1: 200ms, elapsed would be 300ms > 250ms
	_, ok = b.Next(1)
	if ok {
		t.Error("attempt 1: should return ok=false (exceeds MaxDuration)")
	}
}

func TestLinearBackoff_Reset(t *testing.T) {
	t.Parallel()

	b := &LinearBackoff{
		InitialDelay: 100 * time.Millisecond,
		MaxDuration:  150 * time.Millisecond,
	}

	b.Next(0) // 100ms elapsed
	_, ok := b.Next(1)
	if ok {
		t.Error("should be exhausted before reset")
	}

	b.Reset()

	_, ok = b.Next(0)
	if !ok {
		t.Error("should be ok after Reset")
	}
}

func TestFixedBackoff_Delays(t *testing.T) {
	t.Parallel()

	b := &FixedBackoff{Delay: 200 * time.Millisecond}

	for i := 0; i < 5; i++ {
		delay, ok := b.Next(i)
		if !ok {
			t.Fatalf("attempt %d: expected ok=true", i)
		}
		if delay != 200*time.Millisecond {
			t.Errorf("attempt %d: got %v, want 200ms", i, delay)
		}
	}
}

func TestFixedBackoff_MaxRetries(t *testing.T) {
	t.Parallel()

	b := &FixedBackoff{
		Delay:      100 * time.Millisecond,
		MaxRetries: 1,
	}

	_, ok := b.Next(0)
	if !ok {
		t.Fatal("attempt 0: should be ok")
	}
	_, ok = b.Next(1)
	if ok {
		t.Error("attempt 1: should return ok=false")
	}
}

func TestFixedBackoff_MaxDuration(t *testing.T) {
	t.Parallel()

	b := &FixedBackoff{
		Delay:       100 * time.Millisecond,
		MaxDuration: 250 * time.Millisecond,
	}

	_, ok := b.Next(0) // elapsed=100ms
	if !ok {
		t.Fatal("attempt 0: should be ok")
	}
	_, ok = b.Next(1) // elapsed=200ms
	if !ok {
		t.Fatal("attempt 1: should be ok")
	}
	_, ok = b.Next(2) // would be 300ms > 250ms
	if ok {
		t.Error("attempt 2: should return ok=false")
	}
}

func TestFixedBackoff_Reset(t *testing.T) {
	t.Parallel()

	b := &FixedBackoff{
		Delay:       100 * time.Millisecond,
		MaxDuration: 150 * time.Millisecond,
	}

	b.Next(0) // 100ms elapsed
	_, ok := b.Next(1)
	if ok {
		t.Error("should be exhausted before reset")
	}
	b.Reset()
	_, ok = b.Next(0)
	if !ok {
		t.Error("should be ok after Reset")
	}
}

func TestJitteredBackoff_AddsJitter(t *testing.T) {
	t.Parallel()

	inner := &FixedBackoff{Delay: 100 * time.Millisecond}
	b := &JitteredBackoff{
		Inner:  inner,
		Factor: 0.5,
	}

	// Run many iterations to check jitter is within bounds.
	for i := 0; i < 100; i++ {
		delay, ok := b.Next(0)
		if !ok {
			t.Fatal("expected ok=true")
		}
		// delay should be in [100ms, 150ms]
		if delay < 100*time.Millisecond || delay > 150*time.Millisecond {
			t.Errorf("delay %v out of expected range [100ms, 150ms]", delay)
		}
	}
}

func TestJitteredBackoff_DefaultFactor(t *testing.T) {
	t.Parallel()

	inner := &FixedBackoff{Delay: 100 * time.Millisecond}
	b := &JitteredBackoff{Inner: inner} // Factor=0 -> defaults to 0.5

	delay, ok := b.Next(0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if delay < 100*time.Millisecond || delay > 150*time.Millisecond {
		t.Errorf("delay %v out of expected range [100ms, 150ms]", delay)
	}
}

func TestJitteredBackoff_FactorClampedToOne(t *testing.T) {
	t.Parallel()

	inner := &FixedBackoff{Delay: 100 * time.Millisecond}
	b := &JitteredBackoff{Inner: inner, Factor: 2.0} // clamped to 1.0

	for i := 0; i < 50; i++ {
		delay, _ := b.Next(0)
		// max jitter = 100% of delay = 100ms, so total <= 200ms
		if delay < 100*time.Millisecond || delay > 200*time.Millisecond {
			t.Errorf("delay %v out of expected range [100ms, 200ms]", delay)
		}
	}
}

func TestJitteredBackoff_PropagatesStop(t *testing.T) {
	t.Parallel()

	inner := &FixedBackoff{Delay: 10 * time.Millisecond, MaxRetries: 1}
	b := &JitteredBackoff{Inner: inner, Factor: 0.1}

	_, ok := b.Next(0)
	if !ok {
		t.Fatal("attempt 0: should be ok")
	}
	_, ok = b.Next(1)
	if ok {
		t.Error("attempt 1: should propagate inner stop")
	}
}

func TestJitteredBackoff_Reset(t *testing.T) {
	t.Parallel()

	inner := &FixedBackoff{Delay: 100 * time.Millisecond, MaxDuration: 150 * time.Millisecond}
	b := &JitteredBackoff{Inner: inner, Factor: 0.0}

	// Factor=0 means default 0.5, but we want exact test, so use a small
	// fixed that consumes budget.
	// Actually let's just use MaxRetries for clarity.
	inner2 := &FixedBackoff{Delay: 10 * time.Millisecond, MaxRetries: 1}
	b2 := &JitteredBackoff{Inner: inner2}

	b2.Next(0)
	_, ok := b2.Next(1)
	if ok {
		t.Error("should be exhausted")
	}

	b2.Reset()
	_, ok = b2.Next(0)
	if !ok {
		t.Error("should be ok after Reset")
	}

	// suppress unused
	_ = b
}

func TestBackoff_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// Compile-time check that all types implement Backoff.
	var _ Backoff = &ExponentialBackoff{}
	var _ Backoff = &LinearBackoff{}
	var _ Backoff = &FixedBackoff{}
	var _ Backoff = &JitteredBackoff{}
}
