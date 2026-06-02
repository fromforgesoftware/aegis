package domain

import "time"

// LockoutPolicy is the brute-force lockout rule for an account: after
// MaxFailures consecutive failed logins the account is locked for
// Duration. It is a pure value object (no I/O), so the rule lives in the
// domain and is trivially testable. Per-realm policy can replace the
// global default in a later slice.
type LockoutPolicy struct {
	MaxFailures int
	Duration    time.Duration
}

// DefaultLockoutPolicy is the global default lockout rule.
func DefaultLockoutPolicy() LockoutPolicy {
	return LockoutPolicy{MaxFailures: 5, Duration: 15 * time.Minute}
}

// IsLocked reports whether an account whose lock expires at lockedUntil is
// currently locked.
func (p LockoutPolicy) IsLocked(lockedUntil *time.Time, now time.Time) bool {
	return lockedUntil != nil && lockedUntil.After(now)
}

// OnFailure returns the account's new failed-login count and the resulting
// lock expiry after one more failed attempt — lockedUntil is nil until the
// count reaches MaxFailures.
func (p LockoutPolicy) OnFailure(currentCount int, now time.Time) (count int, lockedUntil *time.Time) {
	count = currentCount + 1
	if count >= p.MaxFailures {
		until := now.Add(p.Duration)
		return count, &until
	}
	return count, nil
}
