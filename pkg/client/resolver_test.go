package client

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	calls   atomic.Int64
	account ResolvedAccount
	err     error
}

func (f *fakeResolver) ResolveAccount(context.Context, string, string, string) (ResolvedAccount, error) {
	f.calls.Add(1)
	return f.account, f.err
}

func TestCachingResolver_CachesAfterFirstLoad(t *testing.T) {
	up := &fakeResolver{account: ResolvedAccount{AccountID: "acc-1"}}
	r := NewCachingResolver(up, time.Minute)

	for range 3 {
		got, err := r.Resolve(context.Background(), "realm", "google", "tok")
		require.NoError(t, err)
		assert.Equal(t, "acc-1", got.AccountID)
	}
	assert.Equal(t, int64(1), up.calls.Load(), "upstream hit once, then served from cache")
}

func TestCachingResolver_DoesNotCacheErrors(t *testing.T) {
	up := &fakeResolver{err: errors.New("upstream down")}
	r := NewCachingResolver(up, time.Minute)

	_, err1 := r.Resolve(context.Background(), "realm", "google", "tok")
	_, err2 := r.Resolve(context.Background(), "realm", "google", "tok")
	require.Error(t, err1)
	require.Error(t, err2)
	assert.Equal(t, int64(2), up.calls.Load(), "a failed resolve is retried, not cached")
}

func TestCachingResolver_Invalidate(t *testing.T) {
	up := &fakeResolver{account: ResolvedAccount{AccountID: "acc-1"}}
	r := NewCachingResolver(up, time.Minute)

	_, _ = r.Resolve(context.Background(), "realm", "google", "tok")
	require.NoError(t, r.Invalidate(context.Background(), "realm", "google", "tok"))
	_, _ = r.Resolve(context.Background(), "realm", "google", "tok")
	assert.Equal(t, int64(2), up.calls.Load(), "invalidation forces a reload")
}

func TestCachingResolver_InvalidateAccount(t *testing.T) {
	up := &fakeResolver{account: ResolvedAccount{AccountID: "acc-1"}}
	r := NewCachingResolver(up, time.Minute)

	// Two distinct tokens both resolve to acc-1.
	_, _ = r.Resolve(context.Background(), "realm", "google", "tok-a")
	_, _ = r.Resolve(context.Background(), "realm", "google", "tok-b")
	require.Equal(t, int64(2), up.calls.Load())

	require.NoError(t, r.InvalidateAccount(context.Background(), "acc-1"))

	_, _ = r.Resolve(context.Background(), "realm", "google", "tok-a")
	_, _ = r.Resolve(context.Background(), "realm", "google", "tok-b")
	assert.Equal(t, int64(4), up.calls.Load(), "every token for the account reloads after unlink")
}

func TestCachingResolver_TTLExpiry(t *testing.T) {
	up := &fakeResolver{account: ResolvedAccount{AccountID: "acc-1"}}
	r := NewCachingResolver(up, time.Minute)
	clock := time.Unix(0, 0)
	r.cache.(*cache.Memory[ResolvedAccount]).SetClock(func() time.Time { return clock })

	_, _ = r.Resolve(context.Background(), "realm", "google", "tok")
	clock = clock.Add(2 * time.Minute) // past the TTL
	_, _ = r.Resolve(context.Background(), "realm", "google", "tok")
	assert.Equal(t, int64(2), up.calls.Load(), "expired entry reloads")
}
