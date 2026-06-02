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

func TestRealmRepository_RoundTrip(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	repo, err := db.NewRealmRepository(client)
	require.NoError(t, err)

	created, err := repo.Create(ctx, domain.NewRealm("acme", domain.WithRealmDisplayName("Acme Inc")))
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())

	got, err := repo.Get(ctx, byID(created.ID()))
	require.NoError(t, err)
	assert.Equal(t, "acme", got.Name())
	assert.Equal(t, "Acme Inc", got.DisplayName())
}

func TestInvitationRepository_TokenLookupAndAccept(t *testing.T) {
	client := internaltest.GetDB(t)
	t.Cleanup(func() { internaltest.TruncateTables(t, client) })

	ctx := context.Background()
	realmID := uuid.NewString()
	require.NoError(t, client.WithContext(ctx).
		Exec(`INSERT INTO aegis.realm (id, name) VALUES (?, ?)`, realmID, "inv-realm").Error)

	repo, err := db.NewInvitationRepository(client)
	require.NoError(t, err)

	created, err := repo.Create(ctx, domain.NewInvitation(realmID, "new@x.com", time.Now().Add(time.Hour),
		domain.WithInvitationTokenHash("hash-xyz")))
	require.NoError(t, err)

	got, err := repo.GetByTokenHash(ctx, "hash-xyz")
	require.NoError(t, err)
	assert.Equal(t, created.ID(), got.ID())
	assert.Equal(t, domain.InvitationStatusPending, got.Status())

	require.NoError(t, repo.MarkAccepted(ctx, created.ID(), time.Now()))
	got, err = repo.Get(ctx, byID(created.ID()))
	require.NoError(t, err)
	assert.Equal(t, domain.InvitationStatusAccepted, got.Status())
}
