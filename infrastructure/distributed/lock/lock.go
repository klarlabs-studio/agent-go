// Package lock provides distributed locking abstractions.
package lock

import (
	"context"
	"errors"
	"time"
)

// Lock defines the interface for distributed locks.
type Lock interface {
	// Acquire attempts to acquire the lock.
	// Returns true if the lock was acquired, false if already held.
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// Release releases the lock.
	Release(ctx context.Context, key string) error

	// Extend extends the TTL of a held lock.
	Extend(ctx context.Context, key string, ttl time.Duration) error

	// IsHeld checks if the lock is currently held.
	IsHeld(ctx context.Context, key string) (bool, error)
}

// LockInfo contains metadata about a lock.
type LockInfo struct {
	Key        string    `json:"key"`
	HolderID   string    `json:"holder_id"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// Locker provides lock acquisition with automatic ID tracking.
type Locker interface {
	Lock

	// ID returns the unique identifier for this locker.
	ID() string

	// WithLock executes a function while holding the lock.
	// Automatically releases the lock when the function returns.
	WithLock(ctx context.Context, key string, ttl time.Duration, fn func(ctx context.Context) error) error
}

// Common errors.
var (
	ErrLockNotHeld = errors.New("lock not held")
	ErrLockHeld    = errors.New("lock already held by another owner")
	ErrLockExpired = errors.New("lock has expired")
	ErrInvalidTTL  = errors.New("invalid TTL")
)

// LockOption configures lock behavior.
type LockOption func(*lockOptions)

type lockOptions struct {
	retryInterval time.Duration
	maxRetries    int
	onAcquire     func(key string)
	onRelease     func(key string)
}

// WithRetryInterval sets the interval between lock acquisition retries.
func WithRetryInterval(interval time.Duration) LockOption {
	return func(o *lockOptions) {
		o.retryInterval = interval
	}
}

// WithMaxRetries sets the maximum number of acquisition retries.
func WithMaxRetries(max int) LockOption {
	return func(o *lockOptions) {
		o.maxRetries = max
	}
}

// WithOnAcquire sets a callback for successful lock acquisition.
func WithOnAcquire(fn func(key string)) LockOption {
	return func(o *lockOptions) {
		o.onAcquire = fn
	}
}

// WithOnRelease sets a callback for lock release.
func WithOnRelease(fn func(key string)) LockOption {
	return func(o *lockOptions) {
		o.onRelease = fn
	}
}

// AcquireWithRetry attempts to acquire a lock with retries.
func AcquireWithRetry(ctx context.Context, lock Lock, key string, ttl time.Duration, opts ...LockOption) (bool, error) {
	options := &lockOptions{
		retryInterval: 100 * time.Millisecond,
		maxRetries:    10,
	}
	for _, opt := range opts {
		opt(options)
	}

	for i := 0; i <= options.maxRetries; i++ {
		acquired, err := lock.Acquire(ctx, key, ttl)
		if err != nil {
			return false, err
		}
		if acquired {
			if options.onAcquire != nil {
				options.onAcquire(key)
			}
			return true, nil
		}

		if i < options.maxRetries {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(options.retryInterval):
			}
		}
	}
	return false, nil
}
