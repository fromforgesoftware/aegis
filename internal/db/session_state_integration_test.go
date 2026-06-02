//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/aegis/internal/db"
	"github.com/fromforgesoftware/aegis/internal/domain"
	"github.com/fromforgesoftware/aegis/internal/internaltest"
)

func TestSessionStateRepository_TrackPurgeRoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "sess-realm").Error)
	accountID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.account (id, realm_id, type, status) VALUES (?, ?, 'USER', 'ENABLED')`, accountID, realmID).Error)
	sessionID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.session (id, realm_id, account_id, expires_at) VALUES (?, ?, ?, NOW() + INTERVAL '1 hour')`, sessionID, realmID, accountID).Error)

	repo, err := db.NewSessionStateRepository(client)
	require.NoError(t, err)

	// Upsert is idempotent on session_id and tracks shard transitions.
	_, err = repo.Upsert(ctx, domain.NewSessionState(sessionID, accountID,
		domain.WithSessionStateCurrentShard("madoran"), domain.WithSessionStateLastActive(time.Now())))
	require.NoError(t, err)
	_, err = repo.Upsert(ctx, domain.NewSessionState(sessionID, accountID,
		domain.WithSessionStateCurrentShard("silvermoon"), domain.WithSessionStateLastActive(time.Now())))
	require.NoError(t, err)

	got, err := repo.Get(ctx, byID(sessionID))
	require.NoError(t, err)
	assert.Equal(t, "silvermoon", got.CurrentShard(), "shard transition persisted")

	// Idle purge removes rows older than the cutoff, keeps fresh ones.
	require.NoError(t, repo.Touch(ctx, sessionID, time.Now().Add(-time.Hour)))
	removed, err := repo.PurgeIdle(ctx, time.Now().Add(-30*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(1), removed)

	_, err = repo.Get(ctx, byID(sessionID))
	require.Error(t, err, "purged session state is gone")
}

func TestQuotaPolicyRepository_GetByRealmResourceType(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "quota-realm").Error)

	repo, err := db.NewQuotaPolicyRepository(client)
	require.NoError(t, err)

	created, err := repo.Create(ctx, domain.NewQuotaPolicy(realmID, "character", 5))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())

	got, err := repo.GetByRealmResourceType(ctx, realmID, "character")
	require.NoError(t, err)
	assert.Equal(t, 5, got.MaxCount())
}
