package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/fromforgesoftware/go-kit/cache"
)

// AccountResolver resolves an upstream token to an account. *Client satisfies
// it; the caching resolver wraps any implementation.
type AccountResolver interface {
	ResolveAccount(ctx context.Context, realmID, idpName, token string) (ResolvedAccount, error)
}

// CachingResolver memoises ResolveAccount so the edge resolves a token →
// account in microseconds after the first call. Entries expire by TTL and can
// be invalidated explicitly when an account unlinks an IdP.
type CachingResolver struct {
	upstream AccountResolver
	cache    cache.Cache[ResolvedAccount]
	ttl      time.Duration

	mu        sync.Mutex
	byAccount map[string]map[string]struct{} // accountID → cache keys, for InvalidateAccount
}

func NewCachingResolver(upstream AccountResolver, ttl time.Duration) *CachingResolver {
	return &CachingResolver{
		upstream:  upstream,
		cache:     cache.NewMemory[ResolvedAccount](),
		ttl:       ttl,
		byAccount: map[string]map[string]struct{}{},
	}
}

func resolveKey(realmID, idpName, token string) string {
	sum := sha256.Sum256([]byte(token))
	return realmID + "|" + idpName + "|" + hex.EncodeToString(sum[:])
}

// Resolve returns the cached account or loads it; concurrent loads for the same
// token are coalesced by the underlying cache.
func (r *CachingResolver) Resolve(ctx context.Context, realmID, idpName, token string) (ResolvedAccount, error) {
	key := resolveKey(realmID, idpName, token)
	acct, err := r.cache.GetOrLoad(ctx, key, r.ttl, func(ctx context.Context) (ResolvedAccount, error) {
		return r.upstream.ResolveAccount(ctx, realmID, idpName, token)
	})
	if err != nil {
		return ResolvedAccount{}, err
	}
	r.track(acct.AccountID, key)
	return acct, nil
}

func (r *CachingResolver) track(accountID, key string) {
	if accountID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	keys := r.byAccount[accountID]
	if keys == nil {
		keys = map[string]struct{}{}
		r.byAccount[accountID] = keys
	}
	keys[key] = struct{}{}
}

// Invalidate drops a single token's cached resolution.
func (r *CachingResolver) Invalidate(ctx context.Context, realmID, idpName, token string) error {
	return r.cache.Delete(ctx, resolveKey(realmID, idpName, token))
}

// InvalidateAccount drops every cached token that resolved to accountID — the
// hook to call when an account unlinks an IdP or is disabled.
func (r *CachingResolver) InvalidateAccount(ctx context.Context, accountID string) error {
	r.mu.Lock()
	keys := r.byAccount[accountID]
	delete(r.byAccount, accountID)
	r.mu.Unlock()
	for key := range keys {
		if err := r.cache.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
