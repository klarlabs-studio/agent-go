// Package resilience provides resilient execution patterns using fortify.
package resilience

import (
	"math"
	"math/rand/v2"
	"time"
)

// Backoff computes successive delay durations for retry logic.
type Backoff interface {
	// Next returns the delay for the given attempt (0-indexed) and whether
	// the caller should continue retrying.  When ok is false the caller
	// must stop retrying.
	Next(attempt int) (delay time.Duration, ok bool)

	// Reset restores the backoff to its initial state (if stateful).
	Reset()
}

// --- ExponentialBackoff ---

// ExponentialBackoff produces delays that grow exponentially:
//
//	delay = InitialDelay * Multiplier^attempt
//
// The delay is capped at MaxDelay when set.
type ExponentialBackoff struct {
	// InitialDelay is the delay for the first retry (attempt 0).
	InitialDelay time.Duration

	// Multiplier scales the delay on each successive attempt.
	// Defaults to 2.0 when zero.
	Multiplier float64

	// MaxDelay caps the computed delay. Zero means no cap.
	MaxDelay time.Duration

	// MaxRetries is the maximum number of retries. Zero means unlimited.
	MaxRetries int

	// MaxDuration is the total time budget across all retries.
	// Zero means no limit.
	MaxDuration time.Duration

	elapsed time.Duration
}

// Next implements Backoff.
func (b *ExponentialBackoff) Next(attempt int) (time.Duration, bool) {
	if b.MaxRetries > 0 && attempt >= b.MaxRetries {
		return 0, false
	}

	mult := b.Multiplier
	if mult == 0 {
		mult = 2.0
	}

	delay := time.Duration(float64(b.InitialDelay) * math.Pow(mult, float64(attempt)))
	if b.MaxDelay > 0 && delay > b.MaxDelay {
		delay = b.MaxDelay
	}

	if b.MaxDuration > 0 {
		if b.elapsed+delay > b.MaxDuration {
			return 0, false
		}
		b.elapsed += delay
	}

	return delay, true
}

// Reset implements Backoff.
func (b *ExponentialBackoff) Reset() {
	b.elapsed = 0
}

// --- LinearBackoff ---

// LinearBackoff produces delays that grow linearly:
//
//	delay = InitialDelay + Increment * attempt
type LinearBackoff struct {
	// InitialDelay is the delay for the first retry.
	InitialDelay time.Duration

	// Increment is added for each successive attempt.
	Increment time.Duration

	// MaxDelay caps the computed delay. Zero means no cap.
	MaxDelay time.Duration

	// MaxRetries is the maximum number of retries. Zero means unlimited.
	MaxRetries int

	// MaxDuration is the total time budget. Zero means no limit.
	MaxDuration time.Duration

	elapsed time.Duration
}

// Next implements Backoff.
func (b *LinearBackoff) Next(attempt int) (time.Duration, bool) {
	if b.MaxRetries > 0 && attempt >= b.MaxRetries {
		return 0, false
	}

	delay := b.InitialDelay + b.Increment*time.Duration(attempt)
	if b.MaxDelay > 0 && delay > b.MaxDelay {
		delay = b.MaxDelay
	}

	if b.MaxDuration > 0 {
		if b.elapsed+delay > b.MaxDuration {
			return 0, false
		}
		b.elapsed += delay
	}

	return delay, true
}

// Reset implements Backoff.
func (b *LinearBackoff) Reset() {
	b.elapsed = 0
}

// --- FixedBackoff ---

// FixedBackoff produces the same delay on every attempt.
type FixedBackoff struct {
	// Delay is returned on every attempt.
	Delay time.Duration

	// MaxRetries is the maximum number of retries. Zero means unlimited.
	MaxRetries int

	// MaxDuration is the total time budget. Zero means no limit.
	MaxDuration time.Duration

	elapsed time.Duration
}

// Next implements Backoff.
func (b *FixedBackoff) Next(attempt int) (time.Duration, bool) {
	if b.MaxRetries > 0 && attempt >= b.MaxRetries {
		return 0, false
	}

	if b.MaxDuration > 0 {
		if b.elapsed+b.Delay > b.MaxDuration {
			return 0, false
		}
		b.elapsed += b.Delay
	}

	return b.Delay, true
}

// Reset implements Backoff.
func (b *FixedBackoff) Reset() {
	b.elapsed = 0
}

// --- JitteredBackoff ---

// JitteredBackoff wraps any Backoff and adds random jitter to the delay.
// The jitter is uniformly distributed in [0, Factor * delay].
type JitteredBackoff struct {
	// Inner is the underlying backoff strategy.
	Inner Backoff

	// Factor controls the maximum jitter as a proportion of the computed delay.
	// For example, 0.5 adds up to 50% extra delay.
	// Must be in [0, 1]. Defaults to 0.5 when zero.
	Factor float64
}

// Next implements Backoff.
func (b *JitteredBackoff) Next(attempt int) (time.Duration, bool) {
	delay, ok := b.Inner.Next(attempt)
	if !ok {
		return 0, false
	}

	factor := b.Factor
	if factor <= 0 {
		factor = 0.5
	}
	if factor > 1 {
		factor = 1.0
	}

	jitter := time.Duration(rand.Float64() * factor * float64(delay))
	return delay + jitter, true
}

// Reset implements Backoff.
func (b *JitteredBackoff) Reset() {
	b.Inner.Reset()
}
