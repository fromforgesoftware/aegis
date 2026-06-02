package domain_test

import (
	"testing"
	"time"

	"github.com/fromforgesoftware/aegis/internal/domain"
)

func TestLockoutPolicy_IsLocked(t *testing.T) {
	p := domain.LockoutPolicy{MaxFailures: 3, Duration: 15 * time.Minute}
	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	assert := func(got, want bool, msg string) {
		if got != want {
			t.Fatalf("%s: got %v want %v", msg, got, want)
		}
	}
	assert(p.IsLocked(nil, now), false, "nil lockedUntil")
	assert(p.IsLocked(&past, now), false, "past lockedUntil")
	assert(p.IsLocked(&future, now), true, "future lockedUntil")
}

func TestLockoutPolicy_OnFailure(t *testing.T) {
	p := domain.LockoutPolicy{MaxFailures: 3, Duration: 15 * time.Minute}
	now := time.Now()

	// Below threshold: counter increments, no lock yet.
	if count, until := p.OnFailure(0, now); count != 1 || until != nil {
		t.Fatalf("1st failure: count=%d until=%v, want 1/nil", count, until)
	}
	if count, until := p.OnFailure(1, now); count != 2 || until != nil {
		t.Fatalf("2nd failure: count=%d until=%v, want 2/nil", count, until)
	}

	// Reaching the threshold sets the lock expiry.
	count, until := p.OnFailure(2, now)
	if count != 3 {
		t.Fatalf("3rd failure count=%d, want 3", count)
	}
	if until == nil || !until.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("3rd failure lockedUntil=%v, want now+15m", until)
	}
}
