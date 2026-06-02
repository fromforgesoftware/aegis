//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/fields"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestSessionRepository_RoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	accountID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "sess-realm").Error)
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, accountID, realmID).Error)

	repo, err := db.NewSessionRepository(client)
	require.NoError(t, err)

	created, err := repo.Create(ctx, domain.NewSession(realmID, accountID, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())

	got, err := repo.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, created.ID())))
	require.NoError(t, err)
	assert.Equal(t, accountID, got.AccountID())
	assert.Equal(t, realmID, got.RealmID())

	_, err = repo.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, uuid.NewString())))
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound), "want NOT_FOUND, got %v", err)
}

// TestRefreshTokenRepository_RotationAndReuse exercises the rotation chain:
// create + look up by hash, atomically claim once (a second claim is a no-op),
// and confirm sessions.Revoke kills the chain by revoking its session.
func TestRefreshTokenRepository_RotationAndReuse(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	accountID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "rt-realm").Error)
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, accountID, realmID).Error)

	sessions, err := db.NewSessionRepository(client)
	require.NoError(t, err)
	sess, err := sessions.Create(ctx, domain.NewSession(realmID, accountID, time.Now().Add(time.Hour)))
	require.NoError(t, err)

	repo, err := db.NewRefreshTokenRepository(client)
	require.NoError(t, err)

	require.NoError(t, repo.Create(ctx, domain.RefreshToken{
		SessionID: sess.ID(), ClientID: "web", TokenHash: "hash-1",
		Scopes: []string{"openid"}, ExpiresAt: time.Now().Add(24 * time.Hour),
	}))

	got, err := repo.GetByHash(ctx, "hash-1")
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	assert.Equal(t, sess.ID(), got.SessionID)
	assert.Nil(t, got.UsedAt)

	// First claim succeeds, second is a no-op (already used ⇒ concurrent reuse).
	claimed, err := repo.MarkUsed(ctx, got.ID, time.Now())
	require.NoError(t, err)
	assert.True(t, claimed)
	claimedAgain, err := repo.MarkUsed(ctx, got.ID, time.Now())
	require.NoError(t, err)
	assert.False(t, claimedAgain, "an already-used token cannot be claimed again")

	used, err := repo.GetByHash(ctx, "hash-1")
	require.NoError(t, err)
	require.NotNil(t, used.UsedAt)

	// Revoking the session kills the chain: ResolveSession-style reads see it
	// as revoked. Revocation lives on the session repo now, not on refresh.
	require.NoError(t, sessions.Revoke(ctx, sess.ID(), time.Now()))
	revoked, err := sessions.Get(ctx, search.WithQueryOpts(query.FilterBy(filter.OpEq, fields.ID, sess.ID())))
	require.NoError(t, err)
	require.NotNil(t, revoked.RevokedAt())
	assert.False(t, domain.SessionActive(revoked, time.Now()))
}

// TestAuthorizationCodeRepository_ConsumeOnce proves the code grant's
// single-use guarantee: the first Consume returns the code, a second Consume
// of the same code is NotFound (replay rejected), and an expired code is also
// NotFound.
func TestAuthorizationCodeRepository_ConsumeOnce(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	accountID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "code-realm").Error)
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, accountID, realmID).Error)

	repo, err := db.NewAuthorizationCodeRepository(client)
	require.NoError(t, err)

	code := domain.AuthorizationCode{
		Code:          "the-code",
		RealmID:       realmID,
		ClientID:      "web",
		AccountID:     accountID,
		RedirectURI:   "https://app/cb",
		Scopes:        []string{"openid"},
		PKCEChallenge: "ch",
		ExpiresAt:     time.Now().Add(time.Minute),
	}
	require.NoError(t, repo.Create(ctx, code))

	consumed, err := repo.Consume(ctx, "the-code", time.Now())
	require.NoError(t, err)
	assert.Equal(t, accountID, consumed.AccountID)
	assert.Equal(t, []string{"openid"}, consumed.Scopes)

	// Replay of the same code is rejected.
	_, err = repo.Consume(ctx, "the-code", time.Now())
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound), "want NOT_FOUND on replay, got %v", err)

	// An expired code is also not consumable.
	expired := code
	expired.Code = "expired-code"
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	require.NoError(t, repo.Create(ctx, expired))
	_, err = repo.Consume(ctx, "expired-code", time.Now())
	require.Error(t, err)
	assert.True(t, apierrors.Is(err, apierrors.CodeNotFound), "want NOT_FOUND on expired, got %v", err)
}
